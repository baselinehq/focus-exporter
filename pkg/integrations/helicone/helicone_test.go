package helicone

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

func fixturePost(t *testing.T, body []byte, err error) integrations.HTTPPost {
	t.Helper()
	return func(_ context.Context, u string, h map[string]string, reqBody []byte) ([]byte, error) {
		if h["Authorization"] != "Bearer key-1" {
			t.Fatalf("bad auth header: %q", h["Authorization"])
		}
		if !strings.Contains(string(reqBody), "created_at") {
			t.Fatalf("request body missing time filter: %s", reqBody)
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
	rows, err := os.ReadFile("testdata/requests.json")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		body    []byte
		postErr error
		wantErr bool
		verify  func(t *testing.T, recs []model.UsageRecord)
	}{
		{
			name: "spend aggregated per model+provider, out-of-window rows dropped",
			body: rows,
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 2 {
					t.Fatalf("want 2 buckets (June row clamped out), got %d", len(recs))
				}
				gpt, ok := byModel(recs, "gpt-4o")
				if !ok {
					t.Fatal("gpt-4o bucket missing")
				}
				if gpt.Provider != "Helicone" || gpt.ServiceCategory != model.ServiceCategoryAIAndMachineLearning {
					t.Fatalf("focus identity: %+v", gpt)
				}
				if gpt.Cost == nil || string(*gpt.Cost) != "0.2" {
					t.Fatalf("cost should sum in-window rows to 0.2, got %v", gpt.Cost)
				}
				if gpt.ConsumedQty == nil || string(*gpt.ConsumedQty) != "2100" {
					t.Fatalf("tokens should sum to 2100, got %v", gpt.ConsumedQty)
				}
				if gpt.SkuMeter != "OPENAI" || gpt.Extensions["x_UpstreamProvider"] != "OPENAI" {
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
			name: "empty data yields no records",
			body: []byte(`{"data":[]}`),
			verify: func(t *testing.T, recs []model.UsageRecord) {
				if len(recs) != 0 {
					t.Fatalf("want 0 records, got %d", len(recs))
				}
			},
		},
		{
			name:    "post error propagates",
			postErr: errors.New("boom"),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := New(fixturePost(t, tc.body, tc.postErr), "key-1", "org-1").Fetch(context.Background(), start, end)
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
