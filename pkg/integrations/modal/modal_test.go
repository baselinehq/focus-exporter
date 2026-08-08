package modal

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/baselinehq/focus-exporter/pkg/focus"
	"github.com/baselinehq/focus-exporter/pkg/integrations/modal/modalpb"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func fakeReport(items []*modalpb.WorkspaceBillingReportItem, err error) reporter {
	return func(context.Context, time.Time, time.Time) ([]*modalpb.WorkspaceBillingReportItem, error) {
		return items, err
	}
}

func fakeSummary(summary *modalpb.WorkspaceBillingSummaryResponse, err error) summarizer {
	return func(context.Context, time.Time) (*modalpb.WorkspaceBillingSummaryResponse, error) {
		return summary, err
	}
}

func noSummary() summarizer { return fakeSummary(nil, errors.New("no summary")) }

func find(recs []model.UsageRecord, pred func(model.UsageRecord) bool) (model.UsageRecord, bool) {
	for _, r := range recs {
		if pred(r) {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	gpuApp := &modalpb.WorkspaceBillingReportItem{
		ObjectId: "ap-abc123", Description: "moondream3", EnvironmentName: "main",
		Interval: timestamppb.New(day), Cost: "12.50",
		Tags:           map[string]string{"team": "vision"},
		CostByResource: map[string]string{"GPU:H100": "11.00", "CPU": "1.50"},
	}

	cases := []struct {
		name       string
		items      []*modalpb.WorkspaceBillingReportItem
		reportErr  error
		summary    *modalpb.WorkspaceBillingSummaryResponse
		summaryErr error
		wantErr    bool
		verify     func(t *testing.T, recs []model.UsageRecord)
	}{
		{
			name:       "gpu app cost mapped with resource breakdown and focus identity",
			items:      []*modalpb.WorkspaceBillingReportItem{gpuApp},
			summaryErr: errors.New("no summary"),
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 1 {
					t.Fatalf("want 1 record, got %d", len(recs))
				}
				r := recs[0]
				if r.Provider != "Modal" || r.ServiceCategory != model.ServiceCategoryCompute ||
					r.ServiceSubcategory != model.ServiceSubcategoryServerlessCompute {
					t.Fatalf("focus identity: %+v", r)
				}
				if r.Cost == nil || string(*r.Cost) != "12.50" {
					t.Fatalf("cost = %v, want 12.50", r.Cost)
				}
				if r.ResourceType != "App" || r.ResourceName != "moondream3" || r.BillingAccountID != "revyl-ws" {
					t.Fatalf("resource/account: %+v", r)
				}
				if r.SkuPriceDetails["GPU:H100"] != "11.00" {
					t.Fatalf("cost_by_resource lost: %+v", r.SkuPriceDetails)
				}
				if r.Extensions["x_Environment"] != "main" {
					t.Fatalf("environment: %+v", r.Extensions)
				}
				if tags, ok := r.Extensions["x_Tags"].(map[string]any); !ok || tags["team"] != "vision" {
					t.Fatalf("tags: %+v", r.Extensions)
				}
				if err := focus.Validate(focus.FromUsage(r)); err != nil {
					t.Fatalf("record not FOCUS-compliant: %v", err)
				}
			},
		},
		{
			name: "malformed and interval-less rows are skipped",
			items: []*modalpb.WorkspaceBillingReportItem{
				gpuApp,
				{ObjectId: "fu-bad", Interval: timestamppb.New(day), Cost: "not-a-number"},
				{ObjectId: "fu-nointerval", Cost: "1.00"},
			},
			summaryErr: errors.New("no summary"),
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 1 {
					t.Fatalf("only the valid row should survive, got %d", len(recs))
				}
			},
		},
		{
			name:      "reporter error propagates",
			reportErr: errors.New("boom"),
			wantErr:   true,
		},
		{
			name:  "non-usage charges emitted from billing summary",
			items: []*modalpb.WorkspaceBillingReportItem{gpuApp},
			summary: &modalpb.WorkspaceBillingSummaryResponse{
				StartTimestamp: timestamppb.New(monthStart),
				EndTimestamp:   timestamppb.New(monthStart.AddDate(0, 1, 0)),
				MeteredCost:    "12.50", BilledCost: "37.50",
				Adjustments: map[string]string{"plan": "30.00", "credits": "-5.00"},
			},
			verify: func(t *testing.T, recs []model.UsageRecord) {
				plan, ok := find(recs, func(r model.UsageRecord) bool { return r.SkuMeter == "plan" })
				if !ok || plan.ChargeCategory != model.ChargePurchase || plan.Cost == nil || string(*plan.Cost) != "30.00" {
					t.Fatalf("plan fee record wrong: %+v", plan)
				}
				credits, ok := find(recs, func(r model.UsageRecord) bool { return r.SkuMeter == "credits" })
				if !ok || credits.ChargeCategory != model.ChargeCredit || string(*credits.Cost) != "-5.00" {
					t.Fatalf("credit record wrong: %+v", credits)
				}
				if err := focus.Validate(focus.FromUsage(plan)); err != nil {
					t.Fatalf("plan fee not FOCUS-valid: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := newSource(fakeReport(tc.items, tc.reportErr), fakeSummary(tc.summary, tc.summaryErr), "revyl-ws")
			recs, err := src.Fetch(context.Background(), monthStart, monthStart.AddDate(0, 1, 0))
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tc.verify(t, recs)
		})
	}
}
