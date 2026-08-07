package openai

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
	usage, err := os.ReadFile("testdata/usage_completions.json")
	if err != nil {
		t.Fatal(err)
	}
	costs, err := os.ReadFile("testdata/costs.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, u string, h map[string]string) ([]byte, error) {
		if h["Authorization"] != "Bearer admin-key" {
			t.Fatalf("bad auth header: %v", h)
		}
		switch {
		case strings.Contains(u, "usage/completions"):
			return usage, nil
		case strings.Contains(u, "costs"):
			return costs, nil
		}
		t.Fatalf("unexpected url %q", u)
		return nil, nil
	}
}

func find(recs []model.UsageRecord, name, meter string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.ServiceName == name && r.SkuMeter == meter {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	start := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 8, 2, 0, 0, 0, 0, time.UTC)

	recs, err := New(fixtureGet(t), "admin-key", "org-123").Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}

	tokenTests := []struct {
		name    string
		model   string
		bucket  model.TokenBucket
		wantQty string
	}{
		{"gpt-4o uncached input", "gpt-4o", model.BucketInput, "800"},
		{"gpt-4o cache_read", "gpt-4o", model.BucketCacheRead, "200"},
		{"gpt-4o output", "gpt-4o", model.BucketOutput, "500"},
		{"gpt-4o-mini input derived from total minus cached", "gpt-4o-mini", model.BucketInput, "300"},
	}
	for _, tc := range tokenTests {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := find(recs, tc.model, string(tc.bucket))
			if !ok {
				t.Fatalf("token record %s/%s missing", tc.model, tc.bucket)
			}
			if r.ConsumedQty == nil || string(*r.ConsumedQty) != tc.wantQty {
				t.Fatalf("qty = %v, want %s", r.ConsumedQty, tc.wantQty)
			}
			if r.Cost != nil {
				t.Fatalf("token records must have no cost (separate grain), got %v", r.Cost)
			}
			if r.Provider != "OpenAI" || r.ServiceCategory != model.ServiceCategoryAIAndMachineLearning {
				t.Fatalf("focus identity: %+v", r)
			}
			if r.SkuPriceID != tc.model+"|"+string(tc.bucket) {
				t.Fatalf("skuPriceID = %q", r.SkuPriceID)
			}
			if r.BillingAccountID != "org-123" {
				t.Fatalf("account id: %q", r.BillingAccountID)
			}
		})
	}

	t.Run("gpt-4o-mini cache_read skipped when zero", func(t *testing.T) {
		if _, ok := find(recs, "gpt-4o-mini", string(model.BucketCacheRead)); ok {
			t.Fatal("zero cache_read must not emit a record")
		}
	})

	t.Run("num_model_requests carried as extension", func(t *testing.T) {
		r, _ := find(recs, "gpt-4o", string(model.BucketInput))
		if r.Extensions["x_ModelRequests"] != int64(7) {
			t.Fatalf("x_ModelRequests: %+v", r.Extensions)
		}
	})

	t.Run("line-item cost records emitted with real dollars", func(t *testing.T) {
		r, ok := find(recs, "gpt-4o, input", "gpt-4o, input")
		if !ok {
			t.Fatal("cost line-item record missing")
		}
		if r.Cost == nil || string(*r.Cost) != "12.34" {
			t.Fatalf("cost = %v, want 12.34 (dollars, not cents)", r.Cost)
		}
		if r.Currency != "USD" {
			t.Fatalf("currency usd must uppercase to USD: %q", r.Currency)
		}
		if r.ConsumedQty != nil {
			t.Fatalf("line-item cost has no token quantity: %v", r.ConsumedQty)
		}
		img, ok := find(recs, "Image models", "Image models")
		if !ok || img.Cost == nil || string(*img.Cost) != "0.06" {
			t.Fatalf("image cost: %+v", img)
		}
	})
}
