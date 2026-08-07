package model

import "time"

type Decimal string

func Dec(s string) Decimal { return Decimal(s) }

type TokenBucket string

const (
	BucketInput         TokenBucket = "input"
	BucketOutput        TokenBucket = "output"
	BucketCacheRead     TokenBucket = "cache_read"
	BucketCacheCreation TokenBucket = "cache_creation"
)

type UsageRecord struct {
	Provider           string
	ServiceName        string
	ServiceCategory    ServiceCategory
	ServiceSubcategory ServiceSubcategory
	ChargeCategory     ChargeCategory
	ChargeDescription  string

	Day         time.Time
	PeriodStart *time.Time
	PeriodEnd   *time.Time

	Cost           *Decimal
	ContractedCost *Decimal
	Currency       string

	BillingAccountID   string
	BillingAccountName string
	Publisher          string
	InvoiceIssuer      string
	ChargeFrequency    ChargeFrequency
	PricingCategory    PricingCategory
	PricingCurrency    string

	ConsumedQty  *Decimal
	ConsumedUnit string

	PricingQty          *Decimal
	PricingUnit         string
	ListUnitPrice       *Decimal
	ContractedUnitPrice *Decimal

	ResourceID   string
	ResourceName string
	ResourceType string
	RegionID     string
	RegionName   string

	SkuID           string
	SkuMeter        string
	SkuPriceID      string
	SkuPriceDetails map[string]any

	Extensions map[string]any
}
