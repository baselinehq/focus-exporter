package sink_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"testing"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/focus"
	"github.com/baselinehq/focus-exporter/pkg/sink"
)

var (
	_ sink.Sink = sink.NewJSONSink(nil)
	_ sink.Sink = sink.NewCSVSink(nil)
)

func sampleRecords() []focus.Record {
	t0 := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(24 * time.Hour)
	return []focus.Record{
		{
			BilledCost:         "0.14",
			EffectiveCost:      "0.14",
			ListCost:           "0.14",
			ContractedCost:     "0.14",
			BillingAccountId:   "acct-1",
			BillingCurrency:    "USD",
			BillingPeriodStart: t0,
			BillingPeriodEnd:   t1,
			ChargeCategory:     "Usage",
			ChargePeriodStart:  t0,
			ChargePeriodEnd:    t1,
			ServiceName:        "claude-sonnet-4-5",
			Provider:           "anthropic",
			Extensions: map[string]any{
				"x_TokenType":    "input",
				"x_PromptTokens": float64(1000000),
			},
		},
		{
			BilledCost:         "2.00",
			EffectiveCost:      "2.00",
			ListCost:           "2.00",
			ContractedCost:     "2.00",
			BillingAccountId:   "acct-1",
			BillingCurrency:    "USD",
			BillingPeriodStart: t0,
			BillingPeriodEnd:   t1,
			ChargeCategory:     "Usage",
			ChargePeriodStart:  t0,
			ChargePeriodEnd:    t1,
			ServiceName:        "PlanetScale",
			Provider:           "PlanetScale",
			Extensions: map[string]any{
				"x_InfraProvider": "aws",
			},
		},
	}
}

func TestJSONSinkInlinesExtensions(t *testing.T) {
	var buf bytes.Buffer
	if err := sink.NewJSONSink(&buf).Write(sampleRecords()); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 records, got %d", len(out))
	}
	if out[0]["ServiceName"] != "claude-sonnet-4-5" {
		t.Errorf("FOCUS key missing: %+v", out[0])
	}
	if out[0]["x_TokenType"] != "input" {
		t.Errorf("x_ key not inlined: %+v", out[0])
	}
	if out[1]["x_InfraProvider"] != "aws" {
		t.Errorf("x_ key not inlined: %+v", out[1])
	}
}

func TestCSVSinkHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := sink.NewCSVSink(&buf).Write(sampleRecords()); err != nil {
		t.Fatalf("write: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want header + 2 rows, got %d", len(rows))
	}
	header := rows[0]
	if header[0] != "BilledCost" || header[12] != "Provider" {
		t.Errorf("declared FOCUS order wrong: %v", header[:13])
	}
	xCols := header[len(header)-3:]
	want := []string{"x_InfraProvider", "x_PromptTokens", "x_TokenType"}
	for i, w := range want {
		if xCols[i] != w {
			t.Fatalf("x_ columns not sorted/appended: got %v want %v", xCols, want)
		}
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[h] = i
	}
	if rows[1][idx["ServiceName"]] != "claude-sonnet-4-5" {
		t.Errorf("row value misaligned: %v", rows[1])
	}
	if rows[1][idx["x_TokenType"]] != "input" {
		t.Errorf("x_ value misaligned: %v", rows[1])
	}
	if rows[2][idx["x_InfraProvider"]] != "aws" {
		t.Errorf("x_ value misaligned: %v", rows[2])
	}
	if rows[1][idx["x_InfraProvider"]] != "" {
		t.Errorf("absent x_ should be empty: %v", rows[1])
	}
}
