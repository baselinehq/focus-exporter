package keywordsai

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name       = "keywordsai"
	baseURL    = "https://api.keywordsai.co"
	pageSize   = 1000
	maxPages   = 1000
	dateLayout = "2006-01-02"
)

type source struct {
	get       integrations.HTTPGet
	key       string
	accountID string
}

func New(get integrations.HTTPGet, apiKey, accountID string) integrations.Source {
	return &source{get: get, key: apiKey, accountID: accountID}
}

func (s *source) Name() string { return Name }

type logsPage struct {
	Count   int      `json:"count"`
	Next    string   `json:"next"`
	Results []logRow `json:"results"`
}

type logRow struct {
	Cost             json.Number `json:"cost"`
	PromptTokens     int64       `json:"prompt_tokens"`
	CompletionTokens int64       `json:"completion_tokens"`
	Model            string      `json:"model"`
	ProviderID       string      `json:"provider_id"`
	Timestamp        string      `json:"timestamp"`
}

type bucket struct {
	cost             *big.Rat
	promptTokens     int64
	completionTokens int64
	model            string
	provider         string
	day              time.Time
}

func bucketKey(day time.Time, modelName, provider string) string {
	return day.Format(dateLayout) + "|" + modelName + "|" + provider
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	if s.key == "" {
		return nil, fmt.Errorf("keywordsai: missing API key")
	}

	endpoint, err := s.firstURL(start, end)
	if err != nil {
		return nil, err
	}

	buckets := map[string]*bucket{}
	for page := 0; endpoint != "" && page < maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var body logsPage
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		for _, row := range body.Results {
			accumulate(buckets, row)
		}
		endpoint = body.Next
	}

	return records(buckets, s.accountID), nil
}

func accumulate(buckets map[string]*bucket, row logRow) {
	if row.Model == "" {
		return
	}
	ts, err := time.Parse(time.RFC3339, row.Timestamp)
	if err != nil {
		return
	}
	day := ts.UTC().Truncate(24 * time.Hour)
	key := bucketKey(day, row.Model, row.ProviderID)
	b := buckets[key]
	if b == nil {
		b = &bucket{cost: new(big.Rat), model: row.Model, provider: row.ProviderID, day: day}
		buckets[key] = b
	}
	if c, ok := new(big.Rat).SetString(row.Cost.String()); ok {
		b.cost.Add(b.cost, c)
	}
	b.promptTokens += row.PromptTokens
	b.completionTokens += row.CompletionTokens
}

func records(buckets map[string]*bucket, accountID string) []model.UsageRecord {
	out := []model.UsageRecord{}
	for _, b := range buckets {
		out = append(out, toRecord(b, accountID))
	}
	return out
}

func toRecord(b *bucket, accountID string) model.UsageRecord {
	rec := model.UsageRecord{
		Provider:           "KeywordsAI",
		Publisher:          "KeywordsAI",
		InvoiceIssuer:      "KeywordsAI",
		ServiceName:        b.model,
		ServiceCategory:    model.ServiceCategoryAIAndMachineLearning,
		ServiceSubcategory: model.ServiceSubcategoryGenerativeAI,
		ChargeCategory:     model.ChargeUsage,
		ChargeFrequency:    model.ChargeFrequencyUsageBased,
		PricingCategory:    model.PricingStandard,
		ChargeDescription:  b.model + " via " + b.provider,
		Currency:           "USD",
		PricingCurrency:    "USD",
		BillingAccountID:   accountID,
		Day:                b.day,
		ResourceID:         b.model,
		ResourceName:       b.model,
		ResourceType:       "Model",
		SkuID:              b.model,
		SkuMeter:           b.provider,
		SkuPriceID:         b.model + "|" + b.provider,
	}

	cost := model.Dec(integrations.TrimDecimal(b.cost))
	rec.Cost = &cost

	total := b.promptTokens + b.completionTokens
	if total > 0 {
		q := model.Dec(strconv.FormatInt(total, 10))
		rec.ConsumedQty = &q
		rec.ConsumedUnit = "tokens"
	}

	rec.Extensions = map[string]any{}
	if b.provider != "" {
		rec.Extensions["x_UpstreamProvider"] = b.provider
	}
	if b.promptTokens > 0 {
		rec.Extensions["x_PromptTokens"] = b.promptTokens
	}
	if b.completionTokens > 0 {
		rec.Extensions["x_CompletionTokens"] = b.completionTokens
	}
	if len(rec.Extensions) == 0 {
		rec.Extensions = nil
	}
	return rec
}

func (s *source) firstURL(start, end time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("keywordsai: invalid base url: %w", err)
	}
	u = u.JoinPath("api", "request-logs", "list")
	q := u.Query()
	q.Set("page_size", strconv.Itoa(pageSize))
	q.Set("start_time", start.UTC().Format(time.RFC3339))
	if !end.IsZero() {
		q.Set("end_time", end.UTC().Format(time.RFC3339))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *source) getJSON(ctx context.Context, endpoint string, out any) error {
	headers := map[string]string{
		"Authorization": "Bearer " + s.key,
		"Accept":        "application/json",
	}
	body, err := s.get(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("keywordsai: decode %s: %w", endpoint, err)
	}
	return nil
}

var Provider = integrations.Provider{
	Name:         Name,
	Capabilities: integrations.Capabilities{RequiresTimeRange: true},
	New: func(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
		key := env("KEYWORDSAI_API_KEY")
		orgID := env("KEYWORDSAI_ORG_ID")
		if key == "" || orgID == "" {
			return nil, fmt.Errorf("missing KEYWORDSAI_API_KEY / KEYWORDSAI_ORG_ID env")
		}
		return New(get, key, orgID), nil
	},
}
