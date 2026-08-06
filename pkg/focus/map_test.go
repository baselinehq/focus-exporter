package focus_test

import (
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/focus"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func TestFromUsage(t *testing.T) {
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	modelCost := model.Dec("0.14")
	modelQty := model.Dec("1000000")

	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	infraCost := model.Dec("42.50")

	cases := []struct {
		name   string
		in     model.UsageRecord
		verify func(t *testing.T, r focus.Record)
	}{
		{
			name: "model shaped record with AI categories and token extensions",
			in: model.UsageRecord{
				Provider:           "anthropic",
				ServiceName:        "claude-sonnet-4-5",
				ServiceCategory:    "AI and Machine Learning",
				ServiceSubcategory: "Generative AI",
				ChargeCategory:     "Usage",
				Day:                day,
				Cost:               &modelCost,
				Currency:           "USD",
				ConsumedQty:        &modelQty,
				ConsumedUnit:       "tokens",
				SkuID:              "claude-sonnet-4-5",
				SkuPriceID:         "claude-sonnet-4-5|input",
				Extensions: map[string]any{
					"x_TokenType":    string(model.BucketInput),
					"x_PromptTokens": int64(1000000),
				},
			},
			verify: func(t *testing.T, r focus.Record) {
				if r.ServiceCategory == nil || *r.ServiceCategory != "AI and Machine Learning" {
					t.Fatalf("ServiceCategory wrong: %+v", r.ServiceCategory)
				}
				if r.ServiceSubcategory == nil || *r.ServiceSubcategory != "Generative AI" {
					t.Fatalf("ServiceSubcategory wrong: %+v", r.ServiceSubcategory)
				}
				if r.ServiceName != "claude-sonnet-4-5" {
					t.Fatalf("ServiceName wrong: %q", r.ServiceName)
				}
				if r.SkuPriceId == nil || *r.SkuPriceId != "claude-sonnet-4-5|input" {
					t.Fatalf("SkuPriceId wrong: %+v", r.SkuPriceId)
				}
				if r.BilledCost != "0.14" || r.EffectiveCost != "0.14" || r.ListCost != "0.14" {
					t.Fatalf("cost columns wrong: %q %q %q", r.BilledCost, r.EffectiveCost, r.ListCost)
				}
				if r.BillingCurrency != "USD" {
					t.Fatalf("BillingCurrency wrong: %q", r.BillingCurrency)
				}
				if r.ConsumedQuantity == nil || *r.ConsumedQuantity != "1000000" {
					t.Fatalf("ConsumedQuantity wrong: %+v", r.ConsumedQuantity)
				}
				if r.ConsumedUnit == nil || *r.ConsumedUnit != "tokens" {
					t.Fatalf("ConsumedUnit wrong: %+v", r.ConsumedUnit)
				}
				if !r.ChargePeriodStart.Equal(day) {
					t.Fatalf("ChargePeriodStart wrong: %v", r.ChargePeriodStart)
				}
				if !r.ChargePeriodEnd.Equal(day.Add(24 * time.Hour)) {
					t.Fatalf("ChargePeriodEnd wrong: %v", r.ChargePeriodEnd)
				}
				if !r.BillingPeriodStart.Equal(day) || !r.BillingPeriodEnd.Equal(day.Add(24*time.Hour)) {
					t.Fatalf("billing period wrong: %v %v", r.BillingPeriodStart, r.BillingPeriodEnd)
				}
				if r.Extensions["x_TokenType"] != string(model.BucketInput) {
					t.Fatalf("extensions dropped: %+v", r.Extensions)
				}
			},
		},
		{
			name: "infra shaped record with resource region and explicit period",
			in: model.UsageRecord{
				Provider:           "PlanetScale",
				ServiceName:        "PlanetScale",
				ServiceCategory:    "Databases",
				ServiceSubcategory: "Managed Database",
				ChargeCategory:     "Usage",
				ChargeDescription:  "row_reads",
				Day:                day,
				PeriodStart:        &periodStart,
				PeriodEnd:          &periodEnd,
				Cost:               &infraCost,
				Currency:           "USD",
				ResourceID:         "db-123",
				ResourceName:       "prod-db",
				ResourceType:       "Database",
				RegionID:           "us-east-1",
				RegionName:         "US East",
				SkuID:              "scaler_pro",
				SkuMeter:           "row_reads",
				SkuPriceID:         "scaler_pro|row_reads",
			},
			verify: func(t *testing.T, r focus.Record) {
				if r.ServiceCategory == nil || *r.ServiceCategory != "Databases" {
					t.Fatalf("ServiceCategory wrong: %+v", r.ServiceCategory)
				}
				if !r.ChargePeriodStart.Equal(periodStart) || !r.ChargePeriodEnd.Equal(periodEnd) {
					t.Fatalf("period should come from PeriodStart/End: %v %v", r.ChargePeriodStart, r.ChargePeriodEnd)
				}
				if !r.BillingPeriodStart.Equal(periodStart) || !r.BillingPeriodEnd.Equal(periodEnd) {
					t.Fatalf("billing period should come from PeriodStart/End: %v %v", r.BillingPeriodStart, r.BillingPeriodEnd)
				}
				if r.ResourceId == nil || *r.ResourceId != "db-123" {
					t.Fatalf("ResourceId wrong: %+v", r.ResourceId)
				}
				if r.ResourceName == nil || *r.ResourceName != "prod-db" {
					t.Fatalf("ResourceName wrong: %+v", r.ResourceName)
				}
				if r.ResourceType == nil || *r.ResourceType != "Database" {
					t.Fatalf("ResourceType wrong: %+v", r.ResourceType)
				}
				if r.RegionId == nil || *r.RegionId != "us-east-1" {
					t.Fatalf("RegionId wrong: %+v", r.RegionId)
				}
				if r.RegionName == nil || *r.RegionName != "US East" {
					t.Fatalf("RegionName wrong: %+v", r.RegionName)
				}
				if r.SkuId == nil || *r.SkuId != "scaler_pro" {
					t.Fatalf("SkuId wrong: %+v", r.SkuId)
				}
				if r.SkuMeter == nil || *r.SkuMeter != "row_reads" {
					t.Fatalf("SkuMeter wrong: %+v", r.SkuMeter)
				}
				if r.ChargeDescription == nil || *r.ChargeDescription != "row_reads" {
					t.Fatalf("ChargeDescription wrong: %+v", r.ChargeDescription)
				}
				if r.Extensions != nil {
					t.Fatalf("Extensions should be nil: %+v", r.Extensions)
				}
			},
		},
		{
			name: "edge case empty charge category and nil cost",
			in: model.UsageRecord{
				Provider:    "PlanetScale",
				ServiceName: "PlanetScale",
				Day:         day,
				Currency:    "USD",
			},
			verify: func(t *testing.T, r focus.Record) {
				if r.ChargeCategory != "Usage" {
					t.Fatalf("ChargeCategory should default to Usage: %q", r.ChargeCategory)
				}
				if r.BilledCost != "" || r.EffectiveCost != "" || r.ListCost != "" {
					t.Fatalf("cost columns should be empty for nil Cost: %q %q %q", r.BilledCost, r.EffectiveCost, r.ListCost)
				}
				if r.ServiceCategory != nil {
					t.Fatalf("ServiceCategory should be nil when unset: %+v", r.ServiceCategory)
				}
				if r.ResourceId != nil {
					t.Fatalf("ResourceId should be nil when unset: %+v", r.ResourceId)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.verify(t, focus.FromUsage(tc.in))
		})
	}
}
