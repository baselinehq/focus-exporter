package sink

import "github.com/baselinehq/focus-exporter/pkg/focus"

type Sink interface {
	Write(recs []focus.Record) error
}
