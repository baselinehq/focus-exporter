package confluent

import (
	"context"
	"encoding/base64"
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
	Name     = "confluent"
	baseURL  = "https://api.confluent.cloud"
	pageSize = 500
)

type source struct {
	get       integrations.HTTPGet
	keyID     string
	secret    string
	accountID string
}

var Provider = integrations.Provider{
	Name:         Name,
	Capabilities: integrations.Capabilities{RequiresTimeRange: true},
	New: func(get integrations.HTTPGet, _ integrations.HTTPPost, env func(string) string) (integrations.Source, error) {
		keyID := env("CONFLUENT_CLOUD_API_KEY")
		secret := env("CONFLUENT_CLOUD_API_SECRET")
		orgID := env("CONFLUENT_ORG_ID")
		if keyID == "" || secret == "" || orgID == "" {
			return nil, fmt.Errorf("missing CONFLUENT_CLOUD_API_KEY / CONFLUENT_CLOUD_API_SECRET / CONFLUENT_ORG_ID env")
		}
		return New(get, keyID, secret, orgID), nil
	},
}

func New(get integrations.HTTPGet, keyID, secret, accountID string) integrations.Source {
	return &source{get: get, keyID: keyID, secret: secret, accountID: accountID}
}

func (s *source) Name() string { return Name }

type costList struct {
	Data     []costItem `json:"data"`
	Metadata struct {
		Next *string `json:"next"`
	} `json:"metadata"`
}

type costItem struct {
	ID                string      `json:"id"`
	StartDate         string      `json:"start_date"`
	EndDate           string      `json:"end_date"`
	Product           string      `json:"product"`
	LineType          string      `json:"line_type"`
	NetworkAccessType string      `json:"network_access_type"`
	Price             json.Number `json:"price"`
	Unit              string      `json:"unit"`
	Quantity          json.Number `json:"quantity"`
	OriginalAmount    json.Number `json:"original_amount"`
	DiscountAmount    json.Number `json:"discount_amount"`
	Amount            json.Number `json:"amount"`
	Resource          *struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Environment *struct {
			ID string `json:"id"`
		} `json:"environment"`
	} `json:"resource"`
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	endpoint, err := s.costsURL(start, end)
	if err != nil {
		return nil, err
	}

	out := []model.UsageRecord{}
	for endpoint != "" {
		var body costList
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		for _, c := range body.Data {
			rec, ok := s.toRecord(c)
			if !ok {
				continue
			}
			out = append(out, rec)
		}
		endpoint = ""
		if body.Metadata.Next != nil {
			endpoint = *body.Metadata.Next
		}
	}
	return out, nil
}

func (s *source) toRecord(c costItem) (model.UsageRecord, bool) {
	periodStart, err := parseDate(c.StartDate)
	if err != nil {
		log.Printf("confluent: skipping cost %q: invalid start_date %q", c.ID, c.StartDate)
		return model.UsageRecord{}, false
	}
	periodEnd, err := parseDate(c.EndDate)
	if err != nil {
		log.Printf("confluent: skipping cost %q: invalid end_date %q", c.ID, c.EndDate)
		return model.UsageRecord{}, false
	}
	if _, ok := new(big.Rat).SetString(c.Amount.String()); !ok {
		log.Printf("confluent: skipping cost %q: invalid amount %q", c.ID, c.Amount)
		return model.UsageRecord{}, false
	}

	rec := s.baseRecord()
	rec.ChargeCategory = chargeCategory(c)
	rec.PeriodStart = &periodStart
	rec.PeriodEnd = &periodEnd
	rec.SkuID = c.Product
	rec.SkuMeter = c.LineType
	rec.SkuPriceID = skuPriceID(c.Product, c.LineType)
	rec.ChargeDescription = strings.TrimSpace(c.Product + " " + c.LineType)

	cost := model.Dec(c.Amount.String())
	rec.Cost = &cost

	if c.Resource != nil {
		rec.ResourceID = c.Resource.ID
		rec.ResourceName = c.Resource.DisplayName
		rec.ResourceType = c.Product
	}

	if c.Quantity != "" {
		q := model.Dec(c.Quantity.String())
		rec.ConsumedQty = &q
		rec.ConsumedUnit = c.Unit
		rec.PricingQty = &q
		rec.PricingUnit = c.Unit
	}
	if c.Price != "" {
		p := model.Dec(c.Price.String())
		rec.ListUnitPrice = &p
		rec.ContractedUnitPrice = &p
	}

	rec.Extensions = map[string]any{}
	if c.NetworkAccessType != "" {
		rec.Extensions["x_NetworkAccessType"] = c.NetworkAccessType
	}
	if c.Resource != nil && c.Resource.Environment != nil && c.Resource.Environment.ID != "" {
		rec.Extensions["x_Environment"] = c.Resource.Environment.ID
	}
	if integrations.NonZero(c.OriginalAmount) {
		rec.Extensions["x_OriginalAmount"] = c.OriginalAmount.String()
	}
	if integrations.NonZero(c.DiscountAmount) {
		rec.Extensions["x_DiscountAmount"] = c.DiscountAmount.String()
	}
	if len(rec.Extensions) == 0 {
		rec.Extensions = nil
	}
	return rec, true
}

func (s *source) baseRecord() model.UsageRecord {
	return model.UsageRecord{
		Provider:           "Confluent",
		Publisher:          "Confluent",
		InvoiceIssuer:      "Confluent",
		ServiceName:        "Confluent Cloud",
		ServiceCategory:    model.ServiceCategoryAnalytics,
		ServiceSubcategory: model.ServiceSubcategoryStreamingAnalytics,
		ChargeCategory:     model.ChargeUsage,
		ChargeFrequency:    model.ChargeFrequencyUsageBased,
		PricingCategory:    model.PricingStandard,
		Currency:           "USD",
		PricingCurrency:    "USD",
		BillingAccountID:   s.accountID,
	}
}

func chargeCategory(c costItem) model.ChargeCategory {
	if f, err := c.Amount.Float64(); err == nil && f < 0 {
		return model.ChargeCredit
	}
	if strings.Contains(c.LineType, "PROMO_CREDIT") {
		return model.ChargeCredit
	}
	return model.ChargeUsage
}

func skuPriceID(product, lineType string) string {
	return product + "|" + lineType
}

func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func (s *source) costsURL(start, end time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("confluent: invalid base url: %w", err)
	}
	u = u.JoinPath("billing", "v1", "costs")
	q := u.Query()
	q.Set("start_date", start.UTC().Format("2006-01-02"))
	q.Set("end_date", end.UTC().Format("2006-01-02"))
	q.Set("page_size", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *source) getJSON(ctx context.Context, endpoint string, out any) error {
	auth := base64.StdEncoding.EncodeToString([]byte(s.keyID + ":" + s.secret))
	headers := map[string]string{
		"Authorization": "Basic " + auth,
		"Accept":        "application/json",
	}
	body, err := s.get(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("confluent: decode %s: %w", endpoint, err)
	}
	return nil
}
