package planetscale

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
	Name     = "planetscale"
	baseURL  = "https://api.planetscale.com"
	pageSize = 100
)

type source struct {
	get     integrations.HTTPGet
	org     string
	tokenID string
	token   string
}

func New(get integrations.HTTPGet, org, tokenID, token string) integrations.Source {
	return &source{get: get, org: org, tokenID: tokenID, token: token}
}

func (s *source) Name() string { return Name }

type invoiceListResponse struct {
	Data []invoiceItem `json:"data"`
}

type invoiceItem struct {
	ID                 string `json:"id"`
	BillingPeriodStart string `json:"billing_period_start"`
	BillingPeriodEnd   string `json:"billing_period_end"`
}

type lineItemListResponse struct {
	Data []lineItem `json:"data"`
}

type lineItem struct {
	MetricName   string      `json:"metric_name"`
	Subtotal     json.Number `json:"subtotal"`
	DatabaseID   string      `json:"database_id"`
	DatabaseName string      `json:"database_name"`
}

type databaseListResponse struct {
	Data []databaseItem `json:"data"`
}

type databaseItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Plan   string `json:"plan"`
	Region struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
		Provider    string `json:"provider"`
	} `json:"region"`
}

func (s *source) pageURL(page int, segments ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("planetscale: invalid base url: %w", err)
	}
	u = u.JoinPath(append([]string{"v1"}, segments...)...)
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *source) getJSON(ctx context.Context, endpoint string, out any) error {
	headers := map[string]string{
		"Authorization": s.tokenID + ":" + s.token,
		"Accept":        "application/json",
	}
	body, err := s.get(ctx, endpoint, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("planetscale: decode %s: %w", endpoint, err)
	}
	return nil
}

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	databases, err := s.listDatabases(ctx)
	if err != nil {
		return nil, err
	}
	invoices, err := s.listInvoices(ctx)
	if err != nil {
		return nil, err
	}

	out := []model.UsageRecord{}
	for _, inv := range invoices {
		periodStart, err := parsePlanetScaleDate(inv.BillingPeriodStart)
		if err != nil {
			return nil, fmt.Errorf("planetscale: invoice %s billing_period_start: %w", inv.ID, err)
		}
		periodEnd, err := parsePlanetScaleDate(inv.BillingPeriodEnd)
		if err != nil {
			return nil, fmt.Errorf("planetscale: invoice %s billing_period_end: %w", inv.ID, err)
		}
		if !overlaps(periodStart, periodEnd, start, end) {
			continue
		}
		items, err := s.listLineItems(ctx, inv.ID)
		if err != nil {
			log.Printf("planetscale: skipping invoice %s: line-items fetch failed: %v", inv.ID, err)
			continue
		}
		for _, li := range items {
			out = append(out, toUsageRecord(li, databases[li.DatabaseID], periodStart, periodEnd))
		}
	}
	return out, nil
}

func (s *source) listDatabases(ctx context.Context) (map[string]databaseItem, error) {
	byID := map[string]databaseItem{}
	for page := 1; ; page++ {
		endpoint, err := s.pageURL(page, "organizations", s.org, "databases")
		if err != nil {
			return nil, err
		}
		var body databaseListResponse
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		for _, db := range body.Data {
			byID[db.ID] = db
		}
		if len(body.Data) < pageSize {
			return byID, nil
		}
	}
}

func (s *source) listInvoices(ctx context.Context) ([]invoiceItem, error) {
	out := []invoiceItem{}
	for page := 1; ; page++ {
		endpoint, err := s.pageURL(page, "organizations", s.org, "invoices")
		if err != nil {
			return nil, err
		}
		var body invoiceListResponse
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Data...)
		if len(body.Data) < pageSize {
			return out, nil
		}
	}
}

func (s *source) listLineItems(ctx context.Context, invoiceID string) ([]lineItem, error) {
	out := []lineItem{}
	for page := 1; ; page++ {
		endpoint, err := s.pageURL(page, "organizations", s.org, "invoices", invoiceID, "line-items")
		if err != nil {
			return nil, err
		}
		var body lineItemListResponse
		if err := s.getJSON(ctx, endpoint, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Data...)
		if len(body.Data) < pageSize {
			return out, nil
		}
	}
}

func toUsageRecord(li lineItem, db databaseItem, periodStart, periodEnd time.Time) model.UsageRecord {
	cost := model.Dec(li.Subtotal.String())
	rec := model.UsageRecord{
		Provider:           "PlanetScale",
		ServiceName:        "PlanetScale",
		ServiceCategory:    "Databases",
		ServiceSubcategory: "Managed Database",
		ChargeCategory:     "Usage",
		ChargeDescription:  li.MetricName,
		PeriodStart:        &periodStart,
		PeriodEnd:          &periodEnd,
		Cost:               &cost,
		Currency:           "USD",
		ResourceID:         li.DatabaseID,
		ResourceName:       li.DatabaseName,
		ResourceType:       "Database",
		RegionID:           db.Region.Slug,
		RegionName:         db.Region.DisplayName,
		SkuID:              db.Plan,
		SkuMeter:           li.MetricName,
		SkuPriceID:         skuPriceID(db.Plan, li.MetricName),
	}
	if db.Region.Provider != "" {
		rec.Extensions = map[string]any{"x_InfraProvider": db.Region.Provider}
	}
	return rec
}

func skuPriceID(plan, metric string) string {
	if plan == "" {
		return metric
	}
	return plan + "|" + metric
}

func overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

func parsePlanetScaleDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty billing period date")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", value)
}
