package helicone

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name       = "helicone"
	queryURL   = "https://api.helicone.ai/v1/request/query"
	pageLimit  = 1000
	maxPages   = 1000
	dateLayout = "2006-01-02"
)

type source struct {
	post      integrations.HTTPPost
	key       string
	accountID string
}

func New(post integrations.HTTPPost, apiKey, accountID string) integrations.Source {
	return &source{post: post, key: apiKey, accountID: accountID}
}

func (s *source) Name() string { return Name }

type queryResponse struct {
	Data []requestRow `json:"data"`
}

type requestRow struct {
	CostUSD          json.Number `json:"costUSD"`
	Cost             json.Number `json:"cost"`
	PromptTokens     int64       `json:"prompt_tokens"`
	CompletionTokens int64       `json:"completion_tokens"`
	RequestModel     string      `json:"request_model"`
	Provider         string      `json:"provider"`
	RequestCreatedAt string      `json:"request_created_at"`
}

func (r requestRow) cost() json.Number {
	if r.CostUSD != "" {
		return r.CostUSD
	}
	return r.Cost
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
		return nil, fmt.Errorf("helicone: missing API key")
	}

	buckets := map[string]*bucket{}
	for page := 0; page < maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := s.query(ctx, start, end, page*pageLimit)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			accumulate(buckets, row, start, end)
		}
		if len(rows) < pageLimit {
			break
		}
	}

	return records(buckets, s.accountID), nil
}

func accumulate(buckets map[string]*bucket, row requestRow, start, end time.Time) {
	if row.RequestModel == "" {
		return
	}
	ts, err := time.Parse(time.RFC3339, row.RequestCreatedAt)
	if err != nil {
		return
	}
	ts = ts.UTC()
	if ts.Before(start) || !ts.Before(end) {
		return
	}
	day := ts.Truncate(24 * time.Hour)
	key := bucketKey(day, row.RequestModel, row.Provider)
	b := buckets[key]
	if b == nil {
		b = &bucket{cost: new(big.Rat), model: row.RequestModel, provider: row.Provider, day: day}
		buckets[key] = b
	}
	if c, ok := new(big.Rat).SetString(row.cost().String()); ok {
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
		Provider:           "Helicone",
		Publisher:          "Helicone",
		InvoiceIssuer:      "Helicone",
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

func (s *source) query(ctx context.Context, start, end time.Time, offset int) ([]requestRow, error) {
	body, err := json.Marshal(map[string]any{
		"filter": map[string]any{
			"left":     createdAtBound(start, "gte"),
			"operator": "and",
			"right":    createdAtBound(end, "lt"),
		},
		"offset": offset,
		"limit":  pageLimit,
		"sort":   map[string]any{"created_at": "desc"},
	})
	if err != nil {
		return nil, fmt.Errorf("helicone: encode query: %w", err)
	}

	raw, err := s.post(ctx, queryURL, map[string]string{
		"Authorization": "Bearer " + s.key,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}, body)
	if err != nil {
		return nil, err
	}

	var resp queryResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("helicone: decode response: %w", err)
	}
	return resp.Data, nil
}

func createdAtBound(t time.Time, op string) map[string]any {
	return map[string]any{
		"request": map[string]any{
			"created_at": map[string]any{op: t.UTC().Format(time.RFC3339)},
		},
	}
}

var Provider = integrations.Provider{
	Name:         Name,
	Capabilities: integrations.Capabilities{RequiresTimeRange: true},
	New: func(_ integrations.HTTPGet, post integrations.HTTPPost, env func(string) string) (integrations.Source, error) {
		key := env("HELICONE_API_KEY")
		orgID := env("HELICONE_ORG_ID")
		if key == "" || orgID == "" {
			return nil, fmt.Errorf("missing HELICONE_API_KEY / HELICONE_ORG_ID env")
		}
		return New(post, key, orgID), nil
	},
}
