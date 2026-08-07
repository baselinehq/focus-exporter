package anthropic

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
	usage, err := os.ReadFile("testdata/usage_report.json")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := os.ReadFile("testdata/cost_report.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, u string, h map[string]string) ([]byte, error) {
		if h["x-api-key"] != "admin-key" || h["anthropic-version"] != anthropicVersion {
			t.Fatalf("bad auth headers: %v", h)
		}
		if strings.Contains(u, "organizations/me") {
			return []byte(`{"id":"acct-fetched","type":"organization","name":"Acme Org"}`), nil
		}
		if strings.Contains(u, "usage_report/messages") {
			return usage, nil
		}
		if strings.Contains(u, "cost_report") {
			return cost, nil
		}
		t.Fatalf("unexpected url %q", u)
		return nil, nil
	}
}

func find(recs []model.UsageRecord, name string, bucket model.TokenBucket) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.ServiceName == name && r.SkuMeter == string(bucket) {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	start := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 8, 2, 0, 0, 0, 0, time.UTC)

	recs, err := New(fixtureGet(t), "admin-key", "").Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		model   string
		bucket  model.TokenBucket
		wantCst string
		wantQty string
	}{
		{"opus input joins cost+tokens", "claude-opus-4-6", model.BucketInput, "3", "1500"},
		{"opus output", "claude-opus-4-6", model.BucketOutput, "7.5", "500"},
		{"opus cache_read", "claude-opus-4-6", model.BucketCacheRead, "0.3", "200"},
		{"opus cache_creation sums 1h+5m cost and tokens", "claude-opus-4-6", model.BucketCacheCreation, "1.8", "1500"},
		{"haiku input", "claude-haiku-4", model.BucketInput, "1", "1000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := find(recs, tc.model, tc.bucket)
			if !ok {
				t.Fatalf("record %s/%s missing", tc.model, tc.bucket)
			}
			if r.Cost == nil || string(*r.Cost) != tc.wantCst {
				t.Fatalf("cost = %v, want %s", r.Cost, tc.wantCst)
			}
			if r.ConsumedQty == nil || string(*r.ConsumedQty) != tc.wantQty {
				t.Fatalf("qty = %v, want %s", r.ConsumedQty, tc.wantQty)
			}
			if r.Provider != "Anthropic" || r.ServiceCategory != "AI and Machine Learning" || r.ServiceSubcategory != "Generative AI" {
				t.Fatalf("focus identity: %+v", r)
			}
			if r.SkuPriceID != tc.model+"|"+string(tc.bucket) {
				t.Fatalf("skuPriceID = %q", r.SkuPriceID)
			}
			if r.Currency != "USD" || r.ConsumedUnit != "tokens" {
				t.Fatalf("currency/unit: %+v", r)
			}
			if r.Extensions["x_TokenType"] != string(tc.bucket) {
				t.Fatalf("x_TokenType: %+v", r.Extensions)
			}
		})
	}

	t.Run("cost row without usage is still surfaced", func(t *testing.T) {
		r, ok := find(recs, "claude-sonnet-4", model.BucketInput)
		if !ok {
			t.Fatal("sonnet cost-only record dropped")
		}
		if r.Cost == nil || string(*r.Cost) != "9.99" {
			t.Fatalf("sonnet cost = %v, want 9.99", r.Cost)
		}
		if r.ConsumedQty != nil {
			t.Fatalf("sonnet has no usage, qty must be nil: %v", r.ConsumedQty)
		}
	})

	t.Run("non-token cost surfaced with cost_type meter", func(t *testing.T) {
		r, ok := find(recs, "Anthropic", "web_search")
		if !ok {
			t.Fatal("web_search cost dropped")
		}
		if r.Cost == nil || string(*r.Cost) != "2.5" {
			t.Fatalf("web_search cost = %v, want 2.5", r.Cost)
		}
		if r.ChargeDescription != "Web search requests" {
			t.Fatalf("description = %q", r.ChargeDescription)
		}
	})

	t.Run("service tier and context window carried as extensions", func(t *testing.T) {
		r, _ := find(recs, "claude-opus-4-6", model.BucketInput)
		if r.Extensions["x_ServiceTier"] != "standard" || r.Extensions["x_ContextWindow"] != "0-200k" {
			t.Fatalf("dims extensions: %+v", r.Extensions)
		}
	})

	t.Run("focus enrichment fields populated", func(t *testing.T) {
		r, _ := find(recs, "claude-opus-4-6", model.BucketInput)
		if r.BillingAccountID != "acct-fetched" || r.BillingAccountName != "Acme Org" {
			t.Fatalf("billing account fetched from /organizations/me: id=%q name=%q", r.BillingAccountID, r.BillingAccountName)
		}
		if r.Publisher != "Anthropic" || r.InvoiceIssuer != "Anthropic" {
			t.Fatalf("publisher/issuer: %+v", r)
		}
		if r.ChargeFrequency != "Usage-Based" || r.PricingCategory != "Standard" || r.PricingCurrency != "USD" {
			t.Fatalf("charge/pricing meta: %+v", r)
		}
		if r.ResourceID != "claude-opus-4-6" || r.ChargeDescription != "claude-opus-4-6 input tokens" {
			t.Fatalf("resource/description: %+v", r)
		}
		if r.PricingUnit != "1M tokens" || r.PricingQty == nil || string(*r.PricingQty) != "0.0015" {
			t.Fatalf("pricing quantity: %v", r.PricingQty)
		}
		if r.ListUnitPrice == nil || string(*r.ListUnitPrice) != "2000" {
			t.Fatalf("list unit price = %v, want 2000 ($3 / 0.0015 MTok)", r.ListUnitPrice)
		}
		if r.SkuPriceDetails["token_type"] != "input" || r.SkuPriceDetails["service_tier"] != "standard" {
			t.Fatalf("sku price details: %+v", r.SkuPriceDetails)
		}
	})
}
