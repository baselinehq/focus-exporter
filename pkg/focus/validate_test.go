package focus

import (
	"testing"
	"time"
)

func ptr(s string) *string { return &s }

func TestValidate(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	base := func() Record {
		return Record{
			ChargeCategory:    ChargeCategoryUsage,
			ServiceCategory:   ptr(ServiceCategoryCompute),
			BillingCurrency:   "USD",
			BilledCost:        "1.23",
			EffectiveCost:     "1.23",
			ListCost:          "1.50",
			ContractedCost:    "1.00",
			ChargePeriodStart: start,
			ChargePeriodEnd:   start.Add(time.Hour),
		}
	}
	cases := []struct {
		name    string
		mutate  func(*Record)
		wantErr bool
	}{
		{"valid row", func(*Record) {}, false},
		{"nil service category allowed", func(r *Record) { r.ServiceCategory = nil }, false},
		{"missing charge category", func(r *Record) { r.ChargeCategory = "" }, true},
		{"unknown charge category", func(r *Record) { r.ChargeCategory = "Refund" }, true},
		{"unknown service category", func(r *Record) { r.ServiceCategory = ptr("Blockchain") }, true},
		{"nil charge class allowed", func(r *Record) { r.ChargeClass = nil }, false},
		{"correction charge class allowed", func(r *Record) { r.ChargeClass = ptr(ChargeClassCorrection) }, false},
		{"unknown charge class", func(r *Record) { r.ChargeClass = ptr("Estimated") }, true},
		{"empty currency allowed", func(r *Record) { r.BillingCurrency = "" }, false},
		{"lowercase currency rejected", func(r *Record) { r.BillingCurrency = "usd" }, true},
		{"non iso currency rejected", func(r *Record) { r.BillingCurrency = "Dollars" }, true},
		{"empty billed cost rejected", func(r *Record) { r.BilledCost = "" }, true},
		{"non numeric cost rejected", func(r *Record) { r.EffectiveCost = "abc" }, true},
		{"nan cost rejected", func(r *Record) { r.ListCost = "NaN" }, true},
		{"missing charge period start", func(r *Record) { r.ChargePeriodStart = time.Time{} }, true},
		{"missing charge period end", func(r *Record) { r.ChargePeriodEnd = time.Time{} }, true},
		{"end not after start", func(r *Record) { r.ChargePeriodEnd = r.ChargePeriodStart }, true},
		{"invalid contracted cost rejected", func(r *Record) { r.ContractedCost = "abc" }, true},
		{"empty contracted cost allowed", func(r *Record) { r.ContractedCost = "" }, false},
		{"hex cost rejected", func(r *Record) { r.EffectiveCost = "0x1p2" }, true},
		{"exponent cost allowed", func(r *Record) { r.EffectiveCost = "1e3" }, false},
		{"leading plus cost allowed", func(r *Record) { r.EffectiveCost = "+1" }, false},
		{"negative cost allowed", func(r *Record) { r.EffectiveCost = "-12.10" }, false},
		{"unassigned currency rejected", func(r *Record) { r.BillingCurrency = "ZZZ" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base()
			tc.mutate(&r)
			err := Validate(r)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
