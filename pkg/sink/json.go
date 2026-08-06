package sink

import (
	"encoding/json"
	"io"

	"github.com/baselinehq/focus-exporter/pkg/focus"
)

type jsonSink struct {
	w io.Writer
}

func NewJSONSink(w io.Writer) Sink {
	return &jsonSink{w: w}
}

func (s *jsonSink) Write(recs []focus.Record) error {
	enc := json.NewEncoder(s.w)
	enc.SetIndent("", "  ")
	return enc.Encode(recs)
}
