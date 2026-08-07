package integrations

import (
	"context"
	"fmt"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/model"
)

type Source interface {
	Name() string
	Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error)
}

type HTTPGet func(ctx context.Context, url string, headers map[string]string) ([]byte, error)

type Factory func(get HTTPGet, env func(string) string) (Source, error)

// Capabilities declares provider traits the CLI can act on without building the
// source (which would need credentials).
type Capabilities struct {
	// RequiresTimeRange is true when the provider's API mandates an explicit
	// window (a start time), so an open-ended default export is invalid.
	RequiresTimeRange bool
}

type entry struct {
	factory Factory
	caps    Capabilities
}

type Registry struct {
	entries map[string]entry
}

func NewRegistry() *Registry {
	return &Registry{entries: map[string]entry{}}
}

func (r *Registry) Register(name string, f Factory) {
	r.RegisterWithCapabilities(name, f, Capabilities{})
}

func (r *Registry) RegisterWithCapabilities(name string, f Factory, caps Capabilities) {
	r.entries[name] = entry{factory: f, caps: caps}
}

func (r *Registry) Capabilities(name string) (Capabilities, bool) {
	e, ok := r.entries[name]
	return e.caps, ok
}

func (r *Registry) Build(name string, get HTTPGet, env func(string) string) (Source, error) {
	e, ok := r.entries[name]
	if !ok {
		return nil, fmt.Errorf("integrations: unknown provider %q", name)
	}
	return e.factory(get, env)
}
