package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name             = "anthropic"
	baseURL          = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
	bucketWidth      = "1d"
	pageLimit        = 31
)

type source struct {
	get      integrations.HTTPGet
	adminKey string
}

func New(get integrations.HTTPGet, adminKey string) integrations.Source {
	return &source{get: get, adminKey: adminKey}
}

func (s *source) Name() string { return Name }

type timeBucket[T any] struct {
	StartingAt string `json:"starting_at"`
	EndingAt   string `json:"ending_at"`
	Results    []T    `json:"results"`
}

type page[T any] struct {
	Data     []timeBucket[T] `json:"data"`
	HasMore  bool            `json:"has_more"`
	NextPage string          `json:"next_page"`
}

type usageResult struct {
	Model                string `json:"model"`
	UncachedInputTokens  int64  `json:"uncached_input_tokens"`
	CacheReadInputTokens int64  `json:"cache_read_input_tokens"`
	OutputTokens         int64  `json:"output_tokens"`
	CacheCreation        struct {
		Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
		Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
	} `json:"cache_creation"`
	ServiceTier   string `json:"service_tier"`
	ContextWindow string `json:"context_window"`
	InferenceGeo  string `json:"inference_geo"`
}

type costResult struct {
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	CostType      string `json:"cost_type"`
	TokenType     string `json:"token_type"`
	Model         string `json:"model"`
	Description   string `json:"description"`
	ServiceTier   string `json:"service_tier"`
	ContextWindow string `json:"context_window"`
	InferenceGeo  string `json:"inference_geo"`
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	usage, err := fetchAll[usageResult](ctx, s, []string{"organizations", "usage_report", "messages"}, "model", start, end)
	if err != nil {
		return nil, err
	}
	costs, err := fetchAll[costResult](ctx, s, []string{"organizations", "cost_report"}, "description", start, end)
	if err != nil {
		return nil, err
	}

	tokens, meta := indexUsage(usage, start, end)
	return join(costs, tokens, meta, start, end), nil
}

type bucketKey struct {
	day    time.Time
	model  string
	bucket model.TokenBucket
}

type dims struct {
	serviceTier   string
	contextWindow string
	inferenceGeo  string
}

func indexUsage(buckets []timeBucket[usageResult], start, end time.Time) (map[bucketKey]int64, map[bucketKey]dims) {
	tokens := map[bucketKey]int64{}
	meta := map[bucketKey]dims{}
	for _, b := range buckets {
		day, ok := bucketDay(b.StartingAt, start, end)
		if !ok {
			continue
		}
		for _, r := range b.Results {
			if r.Model == "" {
				continue
			}
			add := func(bucket model.TokenBucket, n int64) {
				if n == 0 {
					return
				}
				k := bucketKey{day, r.Model, bucket}
				tokens[k] += n
				meta[k] = dims{r.ServiceTier, r.ContextWindow, r.InferenceGeo}
			}
			add(model.BucketInput, r.UncachedInputTokens)
			add(model.BucketOutput, r.OutputTokens)
			add(model.BucketCacheRead, r.CacheReadInputTokens)
			add(model.BucketCacheCreation, r.CacheCreation.Ephemeral1h+r.CacheCreation.Ephemeral5m)
		}
	}
	return tokens, meta
}

func join(buckets []timeBucket[costResult], tokens map[bucketKey]int64, meta map[bucketKey]dims, start, end time.Time) []model.UsageRecord {
	out := []model.UsageRecord{}
	costCents := map[bucketKey]*big.Rat{}
	costMeta := map[bucketKey]dims{}
	var loose []model.UsageRecord

	for _, b := range buckets {
		day, ok := bucketDay(b.StartingAt, start, end)
		if !ok {
			continue
		}
		for _, r := range b.Results {
			bucket, isToken := bucketForTokenType(r.TokenType)
			if !isToken || r.Model == "" {
				loose = append(loose, nonTokenCost(r, day))
				continue
			}
			k := bucketKey{day, r.Model, bucket}
			amt, ok := new(big.Rat).SetString(r.Amount)
			if !ok {
				log.Printf("anthropic: bad cost amount %q for %s/%s: skipping", r.Amount, r.Model, r.TokenType)
				continue
			}
			if costCents[k] == nil {
				costCents[k] = new(big.Rat)
			}
			costCents[k].Add(costCents[k], amt)
			costMeta[k] = dims{r.ServiceTier, r.ContextWindow, r.InferenceGeo}
		}
	}

	seen := map[bucketKey]bool{}
	for k, cents := range costCents {
		seen[k] = true
		d := costMeta[k]
		if isZeroDims(d) {
			d = meta[k]
		}
		out = append(out, tokenRecord(k, centsToDollars(cents), tokens[k], d))
	}
	for k, n := range tokens {
		if seen[k] {
			continue
		}
		out = append(out, tokenRecord(k, "", n, meta[k]))
	}
	return append(out, loose...)
}

func tokenRecord(k bucketKey, cost string, tokenCount int64, d dims) model.UsageRecord {
	rec := model.UsageRecord{
		Provider:           "Anthropic",
		ServiceName:        k.model,
		ServiceCategory:    "AI and Machine Learning",
		ServiceSubcategory: "Generative AI",
		ChargeCategory:     "Usage",
		Day:                k.day,
		Currency:           "USD",
		ResourceType:       "Model",
		SkuID:              k.model,
		SkuMeter:           string(k.bucket),
		SkuPriceID:         skuPriceID(k.model, k.bucket),
		Extensions:         map[string]any{"x_TokenType": string(k.bucket)},
	}
	if cost != "" {
		c := model.Dec(cost)
		rec.Cost = &c
	}
	if tokenCount != 0 {
		q := model.Dec(strconv.FormatInt(tokenCount, 10))
		rec.ConsumedQty = &q
		rec.ConsumedUnit = "tokens"
	}
	addDims(rec.Extensions, d)
	return rec
}

func nonTokenCost(r costResult, day time.Time) model.UsageRecord {
	name := r.Model
	if name == "" {
		name = "Anthropic"
	}
	rec := model.UsageRecord{
		Provider:           "Anthropic",
		ServiceName:        name,
		ServiceCategory:    "AI and Machine Learning",
		ServiceSubcategory: "Generative AI",
		ChargeCategory:     "Usage",
		ChargeDescription:  r.Description,
		Day:                day,
		Currency:           "USD",
		SkuMeter:           r.CostType,
		Extensions:         map[string]any{},
	}
	if amt, ok := new(big.Rat).SetString(r.Amount); ok {
		c := model.Dec(centsToDollars(amt))
		rec.Cost = &c
	}
	addDims(rec.Extensions, dims{r.ServiceTier, r.ContextWindow, r.InferenceGeo})
	if len(rec.Extensions) == 0 {
		rec.Extensions = nil
	}
	return rec
}

func addDims(ext map[string]any, d dims) {
	if d.serviceTier != "" {
		ext["x_ServiceTier"] = d.serviceTier
	}
	if d.contextWindow != "" {
		ext["x_ContextWindow"] = d.contextWindow
	}
	if d.inferenceGeo != "" {
		ext["x_InferenceGeo"] = d.inferenceGeo
	}
}

func isZeroDims(d dims) bool { return d == dims{} }

func bucketForTokenType(tt string) (model.TokenBucket, bool) {
	switch tt {
	case "uncached_input_tokens":
		return model.BucketInput, true
	case "output_tokens":
		return model.BucketOutput, true
	case "cache_read_input_tokens":
		return model.BucketCacheRead, true
	case "cache_creation.ephemeral_1h_input_tokens", "cache_creation.ephemeral_5m_input_tokens":
		return model.BucketCacheCreation, true
	}
	return "", false
}

func skuPriceID(m string, b model.TokenBucket) string {
	return m + "|" + string(b)
}

func centsToDollars(cents *big.Rat) string {
	dollars := new(big.Rat).Quo(cents, big.NewRat(100, 1))
	s := dollars.FloatString(12)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func bucketDay(value string, start, end time.Time) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Printf("anthropic: unparseable bucket start %q: skipping", value)
		return time.Time{}, false
	}
	t = t.UTC()
	if t.Before(start) || !t.Before(end) {
		return time.Time{}, false
	}
	return t, true
}

func fetchAll[T any](ctx context.Context, s *source, path []string, groupBy string, start, end time.Time) ([]timeBucket[T], error) {
	var out []timeBucket[T]
	nextPage := ""
	for {
		endpoint, err := s.reportURL(path, groupBy, start, end, nextPage)
		if err != nil {
			return nil, err
		}
		var body page[T]
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Data...)
		if !body.HasMore || body.NextPage == "" {
			return out, nil
		}
		nextPage = body.NextPage
	}
}

func (s *source) reportURL(path []string, groupBy string, start, end time.Time, nextPage string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("anthropic: invalid base url: %w", err)
	}
	u = u.JoinPath(append([]string{"v1"}, path...)...)
	q := u.Query()
	q.Set("bucket_width", bucketWidth)
	q.Set("limit", strconv.Itoa(pageLimit))
	q.Set("starting_at", start.UTC().Format(time.RFC3339))
	if !end.IsZero() {
		q.Set("ending_at", end.UTC().Format(time.RFC3339))
	}
	q.Add("group_by[]", groupBy)
	if nextPage != "" {
		q.Set("page", nextPage)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *source) getJSON(ctx context.Context, endpoint string, out any) error {
	headers := map[string]string{
		"x-api-key":         s.adminKey,
		"anthropic-version": anthropicVersion,
		"Accept":            "application/json",
	}
	body, err := s.get(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("anthropic: decode %s: %w", endpoint, err)
	}
	return nil
}
