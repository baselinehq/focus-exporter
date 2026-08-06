package focus

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGoldenRoundTripsWithoutLoss(t *testing.T) {
	raw, err := os.ReadFile("testdata/focus-1.2-full-example.json")
	if err != nil {
		t.Fatal(err)
	}

	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("golden does not fit focus.Record (missing/mistyped column): %v", err)
	}
	out, err := json.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}

	var a, b map[string]any
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &b); err != nil {
		t.Fatal(err)
	}

	for k := range a {
		if _, ok := b[k]; !ok {
			t.Errorf("FOCUS column %q lost in round-trip - focus.Record is missing it", k)
		}
	}
}
