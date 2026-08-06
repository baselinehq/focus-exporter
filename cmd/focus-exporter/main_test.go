package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

type fakeSource struct {
	name string
	recs []model.UsageRecord
	err  error
}

func (f fakeSource) Name() string { return f.name }

func (f fakeSource) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.recs, nil
}

func fakeFactory(src integrations.Source, err error) integrations.Factory {
	return func(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
		return src, err
	}
}

func testRegistry() *integrations.Registry {
	reg := integrations.NewRegistry()
	cost := model.Dec("0.14")
	good := fakeSource{name: "good", recs: []model.UsageRecord{{
		Provider: "good", ServiceName: "svc-a", ChargeCategory: "Usage",
		Day:  time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		Cost: &cost, Currency: "USD",
	}}}
	reg.Register("good", fakeFactory(good, nil))
	reg.Register("bad", fakeFactory(fakeSource{name: "bad", err: fmt.Errorf("boom")}, nil))
	reg.Register("noenv", fakeFactory(nil, fmt.Errorf("missing FAKE_KEY env")))
	return reg
}

func TestRun(t *testing.T) {
	env := func(string) string { return "" }
	cases := []struct {
		name    string
		args    []string
		wantErr string
		check   func(t *testing.T, out string)
	}{
		{
			name: "json good plus skipped bad",
			args: []string{"--provider", "good", "--provider", "bad", "--month", "2026-07"},
			check: func(t *testing.T, out string) {
				var recs []map[string]any
				if err := json.Unmarshal([]byte(out), &recs); err != nil {
					t.Fatalf("output not JSON array: %v\n%s", err, out)
				}
				if len(recs) != 1 {
					t.Fatalf("want 1 record from good provider, got %d", len(recs))
				}
				if recs[0]["ServiceName"] != "svc-a" {
					t.Fatalf("wrong record: %+v", recs[0])
				}
			},
		},
		{
			name: "csv format",
			args: []string{"--provider", "good", "--month", "2026-07", "--format", "csv"},
			check: func(t *testing.T, out string) {
				rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
				if err != nil {
					t.Fatalf("output not CSV: %v\n%s", err, out)
				}
				if len(rows) != 2 {
					t.Fatalf("want header + 1 data row, got %d rows", len(rows))
				}
			},
		},
		{
			name:    "unknown provider",
			args:    []string{"--provider", "nope", "--month", "2026-07"},
			wantErr: "unknown provider",
		},
		{
			name:    "missing env surfaces",
			args:    []string{"--provider", "noenv", "--month", "2026-07"},
			wantErr: "missing FAKE_KEY",
		},
		{
			name:    "unknown format",
			args:    []string{"--provider", "good", "--month", "2026-07", "--format", "xml"},
			wantErr: "unknown format",
		},
		{
			name:    "no provider",
			args:    []string{"--month", "2026-07"},
			wantErr: "provider is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := run(testRegistry(), tc.args, env, &buf)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, buf.String())
		})
	}
}
