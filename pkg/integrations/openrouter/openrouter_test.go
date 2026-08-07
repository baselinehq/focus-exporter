package openrouter

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func fixtureGet(t *testing.T) integrations.HTTPGet {
	t.Helper()
	day, err := os.ReadFile("testdata/activity_2026-07-01.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, u string, h map[string]string) ([]byte, error) {
		if h["Authorization"] != "Bearer mgmt-key" {
			t.Fatalf("bad auth header: %q", h["Authorization"])
		}
		if strings.Contains(u, "date=2026-07-01") {
			return day, nil
		}
		return []byte(`{"data":[]}`), nil
	}
}

func byModel(recs []model.UsageRecord, m string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.ServiceName == m {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	recs, err := New(fixtureGet(t), "mgmt-key", "acct-1").Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 activity rows, got %d", len(recs))
	}

	t.Run("gateway cost per model+provider with token detail", func(t *testing.T) {
		r, ok := byModel(recs, "openai/gpt-4.1")
		if !ok {
			t.Fatal("gpt-4.1 record missing")
		}
		if r.Provider != "OpenRouter" || r.ServiceCategory != model.ServiceCategoryAIAndMachineLearning {
			t.Fatalf("focus identity: %+v", r)
		}
		if r.Cost == nil || string(*r.Cost) != "12.0" {
			t.Fatalf("cost = %v, want 12.0 (real credit spend)", r.Cost)
		}
		if r.SkuMeter != "OpenAI" || r.SkuPriceID != "openai/gpt-4.1|OpenAI" {
			t.Fatalf("sku (upstream provider): %+v", r)
		}
		if r.ConsumedQty == nil || string(*r.ConsumedQty) != "60000" {
			t.Fatalf("consumed qty = %v, want 60000 (prompt+completion)", r.ConsumedQty)
		}
		if r.PricingQty == nil || string(*r.PricingQty) != "0.06" || r.PricingUnit != "1M tokens" {
			t.Fatalf("pricing quantity: %v %q", r.PricingQty, r.PricingUnit)
		}
		if r.ListUnitPrice == nil || string(*r.ListUnitPrice) != "200" {
			t.Fatalf("blended unit price = %v, want 200 ($12 / 0.06 MTok)", r.ListUnitPrice)
		}
		if r.Extensions["x_UpstreamProvider"] != "OpenAI" || r.Extensions["x_PromptTokens"] != int64(50000) || r.Extensions["x_ModelRequests"] != int64(100) || r.Extensions["x_EndpointId"] != "ep-1" {
			t.Fatalf("extensions: %+v", r.Extensions)
		}
		if _, ok := r.Extensions["x_ByokUsage"]; ok {
			t.Fatal("zero byok must not emit x_ByokUsage")
		}
		if r.BillingAccountID != "acct-1" {
			t.Fatalf("account: %q", r.BillingAccountID)
		}
	})

	t.Run("byok spend kept separate from billed cost", func(t *testing.T) {
		r, _ := byModel(recs, "anthropic/claude-sonnet-4.5")
		if r.Cost == nil || string(*r.Cost) != "3.25" {
			t.Fatalf("billed cost excludes byok: %v", r.Cost)
		}
		if r.Extensions["x_ByokUsage"] != "1.10" {
			t.Fatalf("x_ByokUsage: %+v", r.Extensions)
		}
		if r.Extensions["x_ReasoningTokens"] != int64(500) {
			t.Fatalf("x_ReasoningTokens: %+v", r.Extensions)
		}
		if r.Extensions["x_ByokRequests"] != int64(5) {
			t.Fatalf("x_ByokRequests: %+v", r.Extensions)
		}
	})
}
