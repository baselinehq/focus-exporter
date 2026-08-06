package sink

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/baselinehq/focus-exporter/pkg/focus"
)

type csvSink struct {
	w io.Writer
}

func NewCSVSink(w io.Writer) Sink {
	return &csvSink{w: w}
}

func focusColumns() []string {
	t := reflect.TypeOf(focus.Record{})
	cols := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			cols = append(cols, name)
		}
	}
	return cols
}

func (s *csvSink) Write(recs []focus.Record) error {
	rows := make([]map[string]any, len(recs))
	xSet := map[string]struct{}{}
	for i, rec := range recs {
		raw, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		rows[i] = m
		for k := range rec.Extensions {
			xSet[k] = struct{}{}
		}
	}

	xCols := make([]string, 0, len(xSet))
	for k := range xSet {
		xCols = append(xCols, k)
	}
	sort.Strings(xCols)

	header := append(focusColumns(), xCols...)

	cw := csv.NewWriter(s.w)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, m := range rows {
		record := make([]string, len(header))
		for i, col := range header {
			record[i] = cellValue(m[col])
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func cellValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
