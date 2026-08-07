package confluent

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func fixtureGet(t *testing.T) integrations.HTTPGet {
	t.Helper()
	p1, err := os.ReadFile("testdata/costs_page1.json")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := os.ReadFile("testdata/costs_page2.json")
	if err != nil {
		t.Fatal(err)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("key:secret"))
	return func(_ context.Context, u string, h map[string]string) ([]byte, error) {
		if h["Authorization"] != wantAuth {
			t.Fatalf("bad basic auth header: %q", h["Authorization"])
		}
		if strings.Contains(u, "page_token=TOK2") {
			return p2, nil
		}
		if strings.Contains(u, "/billing/v1/costs") {
			return p1, nil
		}
		t.Fatalf("unexpected url %q", u)
		return nil, nil
	}
}

func byID(recs []model.UsageRecord, cost string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.Cost != nil && string(*r.Cost) == cost {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	recs, err := New(fixtureGet(t), "key", "secret", "org-1").Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records across two pages, got %d", len(recs))
	}

	t.Run("kafka storage line maps to invoice-cost FOCUS", func(t *testing.T) {
		r, ok := byID(recs, "0.15")
		if !ok {
			t.Fatal("kafka record missing")
		}
		if r.ChargeCategory != model.ChargeUsage {
			t.Fatalf("charge category: %q", r.ChargeCategory)
		}
		if r.Provider != "Confluent" || r.ServiceCategory != model.ServiceCategoryAnalytics || r.ServiceSubcategory != model.ServiceSubcategoryStreamingAnalytics {
			t.Fatalf("focus identity: %+v", r)
		}
		if r.ResourceID != "lkc-123" || r.ResourceName != "prod-kafka" || r.ResourceType != "KAFKA" {
			t.Fatalf("resource: %+v", r)
		}
		if r.SkuPriceID != "KAFKA|KAFKA_STORAGE" || r.SkuMeter != "KAFKA_STORAGE" {
			t.Fatalf("sku: %+v", r)
		}
		if r.ConsumedUnit != "GB-hour" || r.ConsumedQty == nil || string(*r.ConsumedQty) != "1000" {
			t.Fatalf("consumed: %v %q", r.ConsumedQty, r.ConsumedUnit)
		}
		if r.ListUnitPrice == nil || string(*r.ListUnitPrice) != "0.00013" {
			t.Fatalf("unit price: %v", r.ListUnitPrice)
		}
		if r.BillingAccountID != "org-1" {
			t.Fatalf("account: %q", r.BillingAccountID)
		}
		if r.Extensions["x_Environment"] != "env-9" || r.Extensions["x_NetworkAccessType"] != "INTERNET" {
			t.Fatalf("extensions: %+v", r.Extensions)
		}
		if r.Extensions["x_OriginalAmount"] != "0.20" || r.Extensions["x_DiscountAmount"] != "0.05" {
			t.Fatalf("discount extensions: %+v", r.Extensions)
		}
		if r.PeriodStart == nil || !r.PeriodStart.Equal(start) {
			t.Fatalf("period start: %v", r.PeriodStart)
		}
	})

	t.Run("promo credit is a Credit with negative cost and no resource", func(t *testing.T) {
		r, ok := byID(recs, "-10.0")
		if !ok {
			t.Fatal("promo credit record missing")
		}
		if r.ChargeCategory != model.ChargeCredit {
			t.Fatalf("negative/promo line must be Credit: %q", r.ChargeCategory)
		}
		if r.ResourceID != "" {
			t.Fatalf("support/promo line has no resource: %q", r.ResourceID)
		}
		if r.ServiceName != "Confluent Cloud" {
			t.Fatalf("service name: %q", r.ServiceName)
		}
	})
}
