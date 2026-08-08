package keywordsai

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/focus"
	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

func fixtureGet(t *testing.T, body []byte, err error) integrations.HTTPGet {
	t.Helper()
	return func(_ context.Context, u string, h map[string]string) ([]byte, error) {
		if h["Authorization"] != "Bearer key-1" {
			t.Fatalf("bad auth header: %q", h["Authorization"])
		}
		if !strings.Contains(u, "start_time=") {
			t.Fatalf("missing time window in %q", u)
		}
		return body, err
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
	logs, err := os.ReadFile("testdata/logs.json")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		body    []byte
		getErr  error
		wantErr bool
		verify  func(t *testing.T, recs []model.UsageRecord)
	}{
		{
			name: "per-day per-model spend aggregated across log rows",
			body: logs,
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 2 {
					t.Fatalf("want 2 buckets (model+provider), got %d", len(recs))
				}
				gpt, ok := byModel(recs, "openai/gpt-4.1")
				if !ok {
					t.Fatal("gpt-4.1 bucket missing")
				}
				if gpt.Provider != "KeywordsAI" || gpt.ServiceCategory != model.ServiceCategoryAIAndMachineLearning {
					t.Fatalf("focus identity: %+v", gpt)
				}
				if gpt.Cost == nil || string(*gpt.Cost) != "0.15" {
					t.Fatalf("cost should sum two rows to 0.15, got %v", gpt.Cost)
				}
				if gpt.ConsumedQty == nil || string(*gpt.ConsumedQty) != "1800" {
					t.Fatalf("tokens should sum to 1800, got %v", gpt.ConsumedQty)
				}
				if gpt.SkuMeter != "openai" || gpt.Extensions["x_UpstreamProvider"] != "openai" {
					t.Fatalf("upstream provider: %+v", gpt)
				}
				if gpt.BillingAccountID != "org-1" {
					t.Fatalf("account: %q", gpt.BillingAccountID)
				}
				if err := focus.Validate(focus.FromUsage(gpt)); err != nil {
					t.Fatalf("record not FOCUS-compliant: %v", err)
				}
			},
		},
		{
			name: "empty results yield no records",
			body: []byte(`{"count":0,"next":null,"results":[]}`),
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 0 {
					t.Fatalf("want 0 records, got %d", len(recs))
				}
			},
		},
		{
			name:    "http error propagates",
			getErr:  errors.New("boom"),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := New(fixtureGet(t, tc.body, tc.getErr), "key-1", "org-1").Fetch(context.Background(), start, end)
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
