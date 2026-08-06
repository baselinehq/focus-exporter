package planetscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func page(u string) int {
	parsed, err := url.Parse(u)
	if err != nil {
		return 1
	}
	p := parsed.Query().Get("page")
	switch p {
	case "", "1":
		return 1
	case "2":
		return 2
	}
	return 3
}

func fullLineItemPage(n int) []byte {
	items := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, map[string]any{
			"metric_name":   fmt.Sprintf("metric_%d", i),
			"subtotal":      1.00,
			"database_id":   "database_main",
			"database_name": "main-db",
		})
	}
	body, _ := json.Marshal(map[string]any{"data": items})
	return body
}

func recordByMeter(recs []model.UsageRecord, meter string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.SkuMeter == meter {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func recordByCost(recs []model.UsageRecord, cost string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.Cost != nil && string(*r.Cost) == cost {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		get    integrations.HTTPGet
		assert func(t *testing.T, recs []model.UsageRecord, err error)
	}{
		{
			name: "join overlap and missing-database surface",
			get: func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
				switch {
				case strings.Contains(u, "/databases"):
					return fixture(t, "databases.json"), nil
				case strings.Contains(u, "/invoices/invoice_july/line-items"):
					return fixture(t, "line_items.json"), nil
				case strings.Contains(u, "/invoices"):
					return fixture(t, "invoices.json"), nil
				}
				return nil, fmt.Errorf("unexpected url %q", u)
			},
			assert: func(t *testing.T, recs []model.UsageRecord, err error) {
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				if len(recs) != 3 {
					t.Fatalf("want 3 records (january invoice excluded), got %d", len(recs))
				}
				reads, ok := recordByMeter(recs, "reads")
				if !ok {
					t.Fatal("reads record missing")
				}
				if reads.Cost == nil || *reads.Cost != "12.34" {
					t.Fatalf("cost drift: %+v", reads.Cost)
				}
				if reads.ChargeCategory != "Usage" {
					t.Fatalf("positive line should be Usage: %q", reads.ChargeCategory)
				}
				if reads.ChargeDescription != "Row reads for branch 'main'" {
					t.Fatalf("charge description should come from description field: %q", reads.ChargeDescription)
				}
				credit, ok := recordByCost(recs, "-4.10")
				if !ok {
					t.Fatal("credit (negative) record missing")
				}
				if credit.ChargeCategory != "Credit" {
					t.Fatalf("negative line should be Credit: %q", credit.ChargeCategory)
				}
				if credit.ChargeDescription != "Prorated reads discount for branch 'main'" {
					t.Fatalf("credit description: %q", credit.ChargeDescription)
				}
				if credit.SkuMeter != "reads" {
					t.Fatalf("credit SkuMeter should stay metric_name: %q", credit.SkuMeter)
				}
				if reads.Currency != "USD" {
					t.Fatalf("currency: %q", reads.Currency)
				}
				if reads.ResourceID != "database_main" || reads.ResourceType != "Database" {
					t.Fatalf("resource: %+v", reads)
				}
				if reads.RegionID != "us-east" || reads.RegionName != "US East (Northern Virginia)" {
					t.Fatalf("region join failed: %+v", reads)
				}
				if reads.SkuID != "scaler_pro" || reads.SkuPriceID != "scaler_pro|reads" {
					t.Fatalf("sku: %+v", reads)
				}
				if reads.Provider != "PlanetScale" || reads.ServiceCategory != "Databases" || reads.ServiceSubcategory != "Managed Database" {
					t.Fatalf("focus identity: %+v", reads)
				}
				if reads.Extensions["x_InfraProvider"] != "aws" {
					t.Fatalf("infra provider ext: %+v", reads.Extensions)
				}
				if reads.PeriodStart == nil || !reads.PeriodStart.Equal(start) {
					t.Fatalf("period start: %+v", reads.PeriodStart)
				}
				ghost, ok := recordByMeter(recs, "storage")
				if !ok {
					t.Fatal("storage (missing-database) record dropped")
				}
				if ghost.ResourceID != "database_ghost" {
					t.Fatalf("ghost resource: %+v", ghost)
				}
				if ghost.RegionID != "" || ghost.RegionName != "" {
					t.Fatalf("missing-db region should be empty: %+v", ghost)
				}
				if ghost.SkuPriceID != "storage" {
					t.Fatalf("missing-db skupriceid should be metric only: %q", ghost.SkuPriceID)
				}
			},
		},
		{
			name: "pagination triggers next fetch",
			get: func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
				switch {
				case strings.Contains(u, "/databases"):
					return fixture(t, "databases.json"), nil
				case strings.Contains(u, "/invoices/invoice_july/line-items"):
					if page(u) == 1 {
						return fullLineItemPage(pageSize), nil
					}
					return fixture(t, "line_items.json"), nil
				case strings.Contains(u, "/invoices"):
					return fixture(t, "invoices.json"), nil
				}
				return nil, fmt.Errorf("unexpected url %q", u)
			},
			assert: func(t *testing.T, recs []model.UsageRecord, err error) {
				if err != nil {
					t.Fatalf("Fetch: %v", err)
				}
				if len(recs) != pageSize+3 {
					t.Fatalf("want %d records across two pages, got %d", pageSize+3, len(recs))
				}
				if _, ok := recordByMeter(recs, "reads"); !ok {
					t.Fatal("second page items missing")
				}
			},
		},
		{
			name: "line-item error skips invoice, others still emitted",
			get: func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
				switch {
				case strings.Contains(u, "/databases"):
					return fixture(t, "databases.json"), nil
				case strings.Contains(u, "/invoices/invoice_july/line-items"):
					return nil, fmt.Errorf("boom")
				case strings.Contains(u, "/invoices"):
					return fixture(t, "invoices.json"), nil
				}
				return nil, fmt.Errorf("unexpected url %q", u)
			},
			assert: func(t *testing.T, recs []model.UsageRecord, err error) {
				if err != nil {
					t.Fatalf("a single invoice failure must not be fatal: %v", err)
				}
				if len(recs) != 0 {
					t.Fatalf("only invoice failed, want 0 records, got %d", len(recs))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := New(tc.get, "my-org", "token-id", "token-secret")
			recs, err := src.Fetch(context.Background(), start, end)
			tc.assert(t, recs, err)
		})
	}
}

func TestFetchExcludesBoundaryInvoice(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	invoices := `{"data":[
		{"id":"invoice_july","billing_period_start":"2026-07-01T00:00:00Z","billing_period_end":"2026-08-01T00:00:00Z"},
		{"id":"invoice_august","billing_period_start":"2026-08-01T00:00:00Z","billing_period_end":"2026-09-01T00:00:00Z"}
	]}`

	get := func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
		switch {
		case strings.Contains(u, "/databases"):
			return fixture(t, "databases.json"), nil
		case strings.Contains(u, "/invoices/invoice_august/line-items"):
			return nil, fmt.Errorf("august invoice must not be fetched for a July window")
		case strings.Contains(u, "/invoices/invoice_july/line-items"):
			return fixture(t, "line_items.json"), nil
		case strings.Contains(u, "/invoices"):
			return []byte(invoices), nil
		}
		return nil, fmt.Errorf("unexpected url %q", u)
	}

	src := New(get, "my-org", "token-id", "token-secret")
	recs, err := src.Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for _, r := range recs {
		if r.PeriodStart != nil && !r.PeriodStart.Before(end) {
			t.Fatalf("adjacent invoice leaked into window: PeriodStart %v", r.PeriodStart)
		}
	}
}

func TestFetchSkipsInvalidInvoicePeriods(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	invoices := `{"data":[
		{"id":"invoice_malformed","billing_period_start":"not-a-date","billing_period_end":"2026-08-01T00:00:00Z"},
		{"id":"invoice_inverted","billing_period_start":"2026-08-01T00:00:00Z","billing_period_end":"2026-07-01T00:00:00Z"},
		{"id":"invoice_july","billing_period_start":"2026-07-01T00:00:00Z","billing_period_end":"2026-08-01T00:00:00Z"}
	]}`

	get := func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
		switch {
		case strings.Contains(u, "/databases"):
			return fixture(t, "databases.json"), nil
		case strings.Contains(u, "/invoices/invoice_malformed/line-items"),
			strings.Contains(u, "/invoices/invoice_inverted/line-items"):
			return nil, fmt.Errorf("invalid invoice must not be fetched")
		case strings.Contains(u, "/invoices/invoice_july/line-items"):
			return fixture(t, "line_items.json"), nil
		case strings.Contains(u, "/invoices"):
			return []byte(invoices), nil
		}
		return nil, fmt.Errorf("unexpected url %q", u)
	}

	src := New(get, "my-org", "token-id", "token-secret")
	recs, err := src.Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatalf("invalid invoice periods must not be fatal: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("want 3 records from the valid july invoice, got %d", len(recs))
	}
	if _, ok := recordByMeter(recs, "reads"); !ok {
		t.Fatal("valid invoice records dropped alongside invalid ones")
	}
}

func TestAuthHeaders(t *testing.T) {
	var seen map[string]string
	get := func(ctx context.Context, u string, h map[string]string) ([]byte, error) {
		seen = h
		if strings.Contains(u, "/databases") {
			return []byte(`{"data":[]}`), nil
		}
		return []byte(`{"data":[]}`), nil
	}
	src := New(get, "my-org", "token-id", "token-secret")
	if _, err := src.Fetch(context.Background(), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if seen["Authorization"] != "token-id:token-secret" {
		t.Fatalf("auth header: %q", seen["Authorization"])
	}
	if seen["Accept"] != "application/json" {
		t.Fatalf("accept header: %q", seen["Accept"])
	}
}
