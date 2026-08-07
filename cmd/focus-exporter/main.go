package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/focus"
	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/integrations/anthropic"
	"github.com/baselinehq/focus-exporter/pkg/integrations/confluent"
	"github.com/baselinehq/focus-exporter/pkg/integrations/openai"
	"github.com/baselinehq/focus-exporter/pkg/integrations/openrouter"
	"github.com/baselinehq/focus-exporter/pkg/integrations/planetscale"
	"github.com/baselinehq/focus-exporter/pkg/sink"
)

func main() {
	if err := run(nil, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "focus-exporter:", err)
		os.Exit(1)
	}
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func defaultRegistry() *integrations.Registry {
	reg := integrations.NewRegistry()
	reg.Register(planetscale.Name, planetscaleFactory)
	reg.RegisterWithCapabilities(anthropic.Name, anthropicFactory, integrations.Capabilities{RequiresTimeRange: true})
	reg.RegisterWithCapabilities(openai.Name, openaiFactory, integrations.Capabilities{RequiresTimeRange: true})
	reg.RegisterWithCapabilities(confluent.Name, confluentFactory, integrations.Capabilities{RequiresTimeRange: true})
	reg.RegisterWithCapabilities(openrouter.Name, openrouterFactory, integrations.Capabilities{RequiresTimeRange: true})
	return reg
}

func confluentFactory(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
	keyID := env("CONFLUENT_CLOUD_API_KEY")
	secret := env("CONFLUENT_CLOUD_API_SECRET")
	if keyID == "" || secret == "" {
		return nil, fmt.Errorf("missing CONFLUENT_CLOUD_API_KEY / CONFLUENT_CLOUD_API_SECRET env")
	}
	return confluent.New(get, keyID, secret, env("CONFLUENT_ORG_ID")), nil
}

func openrouterFactory(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
	key := env("OPENROUTER_MANAGEMENT_KEY")
	if key == "" {
		return nil, fmt.Errorf("missing OPENROUTER_MANAGEMENT_KEY env")
	}
	return openrouter.New(get, key, env("OPENROUTER_ORG_ID")), nil
}

func openaiFactory(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
	adminKey := env("OPENAI_ADMIN_KEY")
	if adminKey == "" {
		return nil, fmt.Errorf("missing OPENAI_ADMIN_KEY env")
	}
	return openai.New(get, adminKey, env("OPENAI_ORG_ID")), nil
}

func anthropicFactory(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
	adminKey := env("ANTHROPIC_ADMIN_KEY")
	if adminKey == "" {
		return nil, fmt.Errorf("missing ANTHROPIC_ADMIN_KEY env")
	}
	return anthropic.New(get, adminKey, env("ANTHROPIC_ORG_ID")), nil
}

func planetscaleFactory(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
	org := env("PLANETSCALE_ORG")
	tokenID := env("PLANETSCALE_SERVICE_TOKEN_ID")
	token := env("PLANETSCALE_SERVICE_TOKEN")
	if org == "" || tokenID == "" || token == "" {
		return nil, fmt.Errorf("missing PLANETSCALE_ORG / PLANETSCALE_SERVICE_TOKEN_ID / PLANETSCALE_SERVICE_TOKEN env")
	}
	return planetscale.New(get, org, tokenID, token), nil
}

func run(reg *integrations.Registry, args []string, env func(string) string, stdout io.Writer) error {
	if reg == nil {
		reg = defaultRegistry()
	}

	fs := flag.NewFlagSet("focus-exporter", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var providers stringSlice
	fs.Var(&providers, "provider", "provider to export (repeatable)")
	start := fs.String("start", "", "window start (RFC3339 or 2006-01-02)")
	end := fs.String("end", "", "window end (RFC3339 or 2006-01-02)")
	month := fs.String("month", "", "convenience window (YYYY-MM)")
	format := fs.String("format", "json", "output format: json|csv")
	out := fs.String("out", "", "output file (default stdout)")
	fs.StringVar(out, "o", "", "output file (default stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(providers) == 0 {
		return fmt.Errorf("at least one --provider is required")
	}

	windowStart, windowEnd, err := resolveWindow(*start, *end, *month)
	if err != nil {
		return err
	}

	if windowStart.IsZero() {
		for _, name := range providers {
			if caps, ok := reg.Capabilities(name); ok && caps.RequiresTimeRange {
				return fmt.Errorf("provider %q requires an explicit window: pass --start/--end or --month", name)
			}
		}
	}

	get := newHTTPGet(30 * time.Second)
	ctx := context.Background()

	records := []focus.Record{}
	for _, name := range providers {
		src, err := reg.Build(name, get, env)
		if err != nil {
			return err
		}
		usage, err := src.Fetch(ctx, windowStart, windowEnd)
		if err != nil {
			log.Printf("provider %s skipped: %v", name, err)
			continue
		}
		for _, u := range usage {
			records = append(records, focus.FromUsage(u))
		}
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				log.Printf("closing %s: %v", *out, cerr)
			}
		}()
		w = f
	}

	s, err := pickSink(*format, w)
	if err != nil {
		return err
	}
	return s.Write(records)
}

func pickSink(format string, w io.Writer) (sink.Sink, error) {
	switch format {
	case "json":
		return sink.NewJSONSink(w), nil
	case "csv":
		return sink.NewCSVSink(w), nil
	default:
		return nil, fmt.Errorf("unknown format %q (want json or csv)", format)
	}
}

func resolveWindow(start, end, month string) (time.Time, time.Time, error) {
	if month != "" {
		if start != "" || end != "" {
			return time.Time{}, time.Time{}, fmt.Errorf("--month cannot be combined with --start/--end")
		}
		m, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --month %q (want YYYY-MM): %w", month, err)
		}
		return m, m.AddDate(0, 1, 0), nil
	}
	if start == "" && end == "" {
		return time.Time{}, time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	if start == "" || end == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("provide both --start and --end, or neither (to export the full available period)")
	}
	s, err := parseDate(start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --start: %w", err)
	}
	e, err := parseDate(end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --end: %w", err)
	}
	if !e.After(s) {
		return time.Time{}, time.Time{}, fmt.Errorf("--end must be after --start")
	}
	return s, e, nil
}

func parseDate(v string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q (want RFC3339 or 2006-01-02)", v)
}

func newHTTPGet(timeout time.Duration) integrations.HTTPGet {
	client := &http.Client{Timeout: timeout}
	return func(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				log.Printf("closing response body for %s: %v", rawURL, cerr)
			}
		}()
		const maxBody = 64 << 20
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		if err != nil {
			return nil, err
		}
		if len(body) > maxBody {
			return nil, fmt.Errorf("GET %s: response exceeds %d bytes", rawURL, maxBody)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s: status %d: %s", rawURL, resp.StatusCode, snippet(body))
		}
		return body, nil
	}
}

func snippet(body []byte) string {
	const max = 200
	if len(body) > max {
		return string(body[:max]) + "..."
	}
	return string(body)
}
