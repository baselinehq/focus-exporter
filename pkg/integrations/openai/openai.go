package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name        = "openai"
	baseURL     = "https://api.openai.com"
	bucketWidth = "1d"
	usageLimit  = 31
	costLimit   = 180
)

type source struct {
	get       integrations.HTTPGet
	adminKey  string
	accountID string
}

func New(get integrations.HTTPGet, adminKey, accountID string) integrations.Source {
	return &source{get: get, adminKey: adminKey, accountID: accountID}
}

func (s *source) Name() string { return Name }

type timeBucket struct {
	StartTime int64             `json:"start_time"`
	EndTime   int64             `json:"end_time"`
	Results   []json.RawMessage `json:"results"`
}

type page struct {
	Data     []timeBucket `json:"data"`
	HasMore  bool         `json:"has_more"`
	NextPage string       `json:"next_page"`
}

type usageResult struct {
	Model                 string `json:"model"`
	InputTokens           int64  `json:"input_tokens"`
	InputCachedTokens     int64  `json:"input_cached_tokens"`
	InputUncachedTokens   int64  `json:"input_uncached_tokens"`
	InputCacheWriteTokens int64  `json:"input_cache_write_tokens"`
	InputAudioTokens      int64  `json:"input_audio_tokens"`
	OutputTokens          int64  `json:"output_tokens"`
	OutputAudioTokens     int64  `json:"output_audio_tokens"`
	NumModelRequests      int64  `json:"num_model_requests"`
}

type costResult struct {
	Amount struct {
		Value    json.Number `json:"value"`
		Currency string      `json:"currency"`
	} `json:"amount"`
	LineItem string `json:"line_item"`
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	usage, err := fetchAll(ctx, s, "usage/completions", "model", usageLimit, start, end)
	if err != nil {
		return nil, err
	}
	costs, err := fetchAll(ctx, s, "costs", "line_item", costLimit, start, end)
	if err != nil {
		return nil, err
	}

	out := []model.UsageRecord{}
	out = append(out, s.tokenRecords(usage, start, end)...)
	out = append(out, s.costRecords(costs, start, end)...)
	return out, nil
}

func (s *source) tokenRecords(buckets []timeBucket, start, end time.Time) []model.UsageRecord {
	var out []model.UsageRecord
	for _, b := range buckets {
		day, ok := bucketDay(b.StartTime, start, end)
		if !ok {
			continue
		}
		for _, raw := range b.Results {
			var r usageResult
			if err := json.Unmarshal(raw, &r); err != nil {
				log.Printf("openai: skipping unparseable usage row: %v", err)
				continue
			}
			if r.Model == "" {
				continue
			}
			if v := uncachedInput(r); v > 0 {
				rec := s.tokenRecord(day, r.Model, model.BucketInput, v, r.NumModelRequests)
				if r.InputAudioTokens > 0 {
					rec.Extensions["x_InputAudioTokens"] = r.InputAudioTokens
				}
				out = append(out, rec)
			}
			if r.InputCachedTokens > 0 {
				out = append(out, s.tokenRecord(day, r.Model, model.BucketCacheRead, r.InputCachedTokens, r.NumModelRequests))
			}
			if r.InputCacheWriteTokens > 0 {
				out = append(out, s.tokenRecord(day, r.Model, model.BucketCacheCreation, r.InputCacheWriteTokens, r.NumModelRequests))
			}
			if r.OutputTokens > 0 {
				rec := s.tokenRecord(day, r.Model, model.BucketOutput, r.OutputTokens, r.NumModelRequests)
				if r.OutputAudioTokens > 0 {
					rec.Extensions["x_OutputAudioTokens"] = r.OutputAudioTokens
				}
				out = append(out, rec)
			}
		}
	}
	return out
}

// uncachedInput is input tokens billed at the standard uncached rate. OpenAI's
// input_uncached_tokens already excludes both cached reads and cache writes; the
// fallback subtracts both so cache-write tokens are not double counted into input.
func uncachedInput(r usageResult) int64 {
	if r.InputUncachedTokens > 0 {
		return r.InputUncachedTokens
	}
	if n := r.InputTokens - r.InputCachedTokens - r.InputCacheWriteTokens; n > 0 {
		return n
	}
	return 0
}

func (s *source) tokenRecord(day time.Time, modelName string, bucket model.TokenBucket, tokens, requests int64) model.UsageRecord {
	rec := s.baseRecord()
	rec.ServiceName = modelName
	rec.Day = day
	rec.ResourceID = modelName
	rec.ResourceName = modelName
	rec.ResourceType = "Model"
	rec.SkuID = modelName
	rec.SkuMeter = string(bucket)
	rec.SkuPriceID = modelName + "|" + string(bucket)
	rec.ChargeDescription = modelName + " " + string(bucket) + " tokens"
	rec.Currency = "USD"
	rec.PricingCurrency = "USD"

	q := model.Dec(strconv.FormatInt(tokens, 10))
	rec.ConsumedQty = &q
	rec.ConsumedUnit = "tokens"
	pq := model.Dec(perMTok(tokens))
	rec.PricingQty = &pq
	rec.PricingUnit = "1M tokens"

	rec.Extensions = map[string]any{"x_TokenType": string(bucket)}
	if requests > 0 {
		rec.Extensions["x_ModelRequests"] = requests
	}
	return rec
}

func (s *source) costRecords(buckets []timeBucket, start, end time.Time) []model.UsageRecord {
	var out []model.UsageRecord
	for _, b := range buckets {
		day, ok := bucketDay(b.StartTime, start, end)
		if !ok {
			continue
		}
		for _, raw := range b.Results {
			var r costResult
			if err := json.Unmarshal(raw, &r); err != nil {
				log.Printf("openai: skipping unparseable cost row: %v", err)
				continue
			}
			amount, err := r.Amount.Value.Float64()
			if err != nil {
				log.Printf("openai: skipping cost row with bad amount %q: %v", r.Amount.Value, err)
				continue
			}

			rec := s.baseRecord()
			rec.ServiceName = r.LineItem
			if rec.ServiceName == "" {
				rec.ServiceName = "OpenAI"
			}
			rec.ChargeDescription = r.LineItem
			rec.SkuMeter = r.LineItem
			rec.Day = day
			if amount < 0 {
				rec.ChargeCategory = model.ChargeCredit
			}

			cost := model.Dec(r.Amount.Value.String())
			rec.Cost = &cost
			currency := strings.ToUpper(r.Amount.Currency)
			if currency == "" {
				currency = "USD"
			}
			rec.Currency = currency
			rec.PricingCurrency = currency
			out = append(out, rec)
		}
	}
	return out
}

func (s *source) baseRecord() model.UsageRecord {
	return model.UsageRecord{
		Provider:           "OpenAI",
		Publisher:          "OpenAI",
		InvoiceIssuer:      "OpenAI",
		ServiceCategory:    model.ServiceCategoryAIAndMachineLearning,
		ServiceSubcategory: model.ServiceSubcategoryGenerativeAI,
		ChargeCategory:     model.ChargeUsage,
		ChargeFrequency:    model.ChargeFrequencyUsageBased,
		PricingCategory:    model.PricingStandard,
		Currency:           "USD",
		PricingCurrency:    "USD",
		BillingAccountID:   s.accountID,
	}
}

func perMTok(tokens int64) string {
	return strconv.FormatFloat(float64(tokens)/1_000_000, 'f', -1, 64)
}

func bucketDay(unix int64, start, end time.Time) (time.Time, bool) {
	t := time.Unix(unix, 0).UTC()
	if !start.IsZero() && t.Before(start) {
		return time.Time{}, false
	}
	if !end.IsZero() && !t.Before(end) {
		return time.Time{}, false
	}
	return t, true
}

func fetchAll(ctx context.Context, s *source, report, groupBy string, limit int, start, end time.Time) ([]timeBucket, error) {
	var out []timeBucket
	nextPage := ""
	for {
		endpoint, err := s.reportURL(report, groupBy, limit, start, end, nextPage)
		if err != nil {
			return nil, err
		}
		var body page
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

func (s *source) reportURL(report, groupBy string, limit int, start, end time.Time, nextPage string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("openai: invalid base url: %w", err)
	}
	u = u.JoinPath("v1", "organization", report)
	q := u.Query()
	q.Set("bucket_width", bucketWidth)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("start_time", strconv.FormatInt(start.UTC().Unix(), 10))
	if !end.IsZero() {
		q.Set("end_time", strconv.FormatInt(end.UTC().Unix(), 10))
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
		"Authorization": "Bearer " + s.adminKey,
		"Accept":        "application/json",
	}
	body, err := s.get(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("openai: decode %s: %w", endpoint, err)
	}
	return nil
}
