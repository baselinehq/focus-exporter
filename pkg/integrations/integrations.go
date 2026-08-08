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

type HTTPPost func(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, error)

type Factory func(get HTTPGet, post HTTPPost, env func(string) string) (Source, error)

type Capabilities struct {
	RequiresTimeRange bool
}

type Provider struct {
	Name         string
	Capabilities Capabilities
	New          Factory
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

func (r *Registry) Add(p Provider) {
	r.RegisterWithCapabilities(p.Name, p.New, p.Capabilities)
}

func (r *Registry) RegisterWithCapabilities(name string, f Factory, caps Capabilities) {
	r.entries[name] = entry{factory: f, caps: caps}
}

func (r *Registry) Capabilities(name string) (Capabilities, bool) {
	e, ok := r.entries[name]
	return e.caps, ok
}

func (r *Registry) Build(name string, get HTTPGet, post HTTPPost, env func(string) string) (Source, error) {
	e, ok := r.entries[name]
	if !ok {
		return nil, fmt.Errorf("integrations: unknown provider %q", name)
	}
	return e.factory(get, post, env)
}
