package modal

import (
	"context"
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

func byResource(recs []model.UsageRecord, id string) (model.UsageRecord, bool) {
	for _, r := range recs {
		if r.ResourceID == id {
			return r, true
		}
	}
	return model.UsageRecord{}, false
}

func TestFetch(t *testing.T) {
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	items := []*modalpb.WorkspaceBillingReportItem{
		{
			ObjectId:        "ap-abc123",
			Description:     "moondream3",
			EnvironmentName: "main",
			Interval:        timestamppb.New(day),
			Cost:            "12.50",
			Tags:            map[string]string{"team": "vision"},
			CostByResource:  map[string]string{"GPU:H100": "11.00", "CPU": "1.50"},
		},
		{
			ObjectId: "fu-def456",
			Cost:     "",
		},
	}

	recs, err := newSource(fakeReport(items, nil), "revyl-ws").Fetch(context.Background(), day, day.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("zero-cost row must be skipped: got %d records", len(recs))
	}

	t.Run("gpu app cost mapped with resource breakdown and focus identity", func(t *testing.T) {
		r, ok := byResource(recs, "ap-abc123")
		if !ok {
			t.Fatal("app record missing")
		}
		if r.Provider != "Modal" || r.ServiceCategory != model.ServiceCategoryCompute {
			t.Fatalf("focus identity: %+v", r)
		}
		if r.Cost == nil || string(*r.Cost) != "12.50" {
			t.Fatalf("cost = %v, want 12.50", r.Cost)
		}
		if r.ResourceType != "App" || r.ResourceName != "moondream3" {
			t.Fatalf("resource: type=%q name=%q", r.ResourceType, r.ResourceName)
		}
		if r.BillingAccountID != "revyl-ws" {
			t.Fatalf("account: %q", r.BillingAccountID)
		}
		if r.SkuPriceDetails["GPU:H100"] != "11.00" {
			t.Fatalf("cost_by_resource lost: %+v", r.SkuPriceDetails)
		}
		if r.Extensions["x_Environment"] != "main" {
			t.Fatalf("environment: %+v", r.Extensions)
		}
		tags, ok := r.Extensions["x_Tags"].(map[string]any)
		if !ok || tags["team"] != "vision" {
			t.Fatalf("tags: %+v", r.Extensions)
		}
	})

	t.Run("emitted record is focus-valid", func(t *testing.T) {
		r, _ := byResource(recs, "ap-abc123")
		if err := focus.Validate(focus.FromUsage(r)); err != nil {
			t.Fatalf("record not FOCUS-compliant: %v", err)
		}
	})
}
