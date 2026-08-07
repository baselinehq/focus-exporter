package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name       = "openrouter"
	baseURL    = "https://openrouter.ai"
	maxDays    = 400
	dateLayout = "2006-01-02"
)

type source struct {
	get       integrations.HTTPGet
	key       string
	accountID string
}

func New(get integrations.HTTPGet, managementKey, accountID string) integrations.Source {
	return &source{get: get, key: managementKey, accountID: accountID}
}

func (s *source) Name() string { return Name }

type activityResponse struct {
	Data []activityItem `json:"data"`
}

type activityItem struct {
	Date             string      `json:"date"`
	Model            string      `json:"model"`
	ModelPermaslug   string      `json:"model_permaslug"`
	ProviderName     string      `json:"provider_name"`
	Usage            json.Number `json:"usage"`
	BYOKUsage        json.Number `json:"byok_usage_inference"`
	Requests         int64       `json:"requests"`
	PromptTokens     int64       `json:"prompt_tokens"`
	CompletionTokens int64       `json:"completion_tokens"`
	ReasoningTokens  int64       `json:"reasoning_tokens"`
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	out := []model.UsageRecord{}
	days := 0
	for day := start.UTC().Truncate(24 * time.Hour); day.Before(end); day = day.AddDate(0, 0, 1) {
		if days++; days > maxDays {
			break
		}
		endpoint, err := s.activityURL(day)
		if err != nil {
			return nil, err
		}
		var body activityResponse
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			log.Printf("openrouter: skipping %s: %v", day.Format(dateLayout), err)
			continue
		}
		for _, item := range body.Data {
			out = append(out, s.toRecord(item, day))
		}
	}
	return out, nil
}

func (s *source) toRecord(item activityItem, day time.Time) model.UsageRecord {
	rec := model.UsageRecord{
		Provider:           "OpenRouter",
		Publisher:          "OpenRouter",
		InvoiceIssuer:      "OpenRouter",
		ServiceName:        item.Model,
		ServiceCategory:    model.ServiceCategoryAIAndMachineLearning,
		ServiceSubcategory: model.ServiceSubcategoryGenerativeAI,
		ChargeCategory:     model.ChargeUsage,
		ChargeFrequency:    model.ChargeFrequencyUsageBased,
		PricingCategory:    model.PricingStandard,
		ChargeDescription:  item.Model + " via " + item.ProviderName,
		Currency:           "USD",
		PricingCurrency:    "USD",
		BillingAccountID:   s.accountID,
		Day:                day,
		ResourceID:         item.Model,
		ResourceName:       item.Model,
		ResourceType:       "Model",
		SkuID:              item.Model,
		SkuMeter:           item.ProviderName,
		SkuPriceID:         item.Model + "|" + item.ProviderName,
	}

	cost := model.Dec(numberOrZero(item.Usage))
	rec.Cost = &cost

	total := item.PromptTokens + item.CompletionTokens
	if total > 0 {
		q := model.Dec(strconv.FormatInt(total, 10))
		rec.ConsumedQty = &q
		rec.ConsumedUnit = "tokens"
	}

	rec.Extensions = map[string]any{"x_UpstreamProvider": item.ProviderName}
	if item.ModelPermaslug != "" {
		rec.Extensions["x_ModelPermaslug"] = item.ModelPermaslug
	}
	if item.PromptTokens > 0 {
		rec.Extensions["x_PromptTokens"] = item.PromptTokens
	}
	if item.CompletionTokens > 0 {
		rec.Extensions["x_CompletionTokens"] = item.CompletionTokens
	}
	if item.ReasoningTokens > 0 {
		rec.Extensions["x_ReasoningTokens"] = item.ReasoningTokens
	}
	if item.Requests > 0 {
		rec.Extensions["x_ModelRequests"] = item.Requests
	}
	if nonZero(item.BYOKUsage) {
		rec.Extensions["x_ByokUsage"] = item.BYOKUsage.String()
	}
	return rec
}

func numberOrZero(n json.Number) string {
	if n == "" {
		return "0"
	}
	return n.String()
}

func nonZero(n json.Number) bool {
	if n == "" {
		return false
	}
	f, err := n.Float64()
	return err == nil && f != 0
}

func (s *source) activityURL(day time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("openrouter: invalid base url: %w", err)
	}
	u = u.JoinPath("api", "v1", "activity")
	q := u.Query()
	q.Set("date", day.UTC().Format(dateLayout))
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
		return fmt.Errorf("openrouter: decode %s: %w", endpoint, err)
	}
	return nil
}
