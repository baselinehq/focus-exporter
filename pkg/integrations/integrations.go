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

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}}
}

func (r *Registry) Register(name string, f Factory) {
	r.factories[name] = f
}

func (r *Registry) Build(name string, get HTTPGet, env func(string) string) (Source, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("integrations: unknown provider %q", name)
	}
	return f(get, env)
}
