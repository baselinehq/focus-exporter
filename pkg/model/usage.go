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
	ServiceCategory    string
	ServiceSubcategory string
	ChargeCategory     string
	ChargeDescription  string

	Day         time.Time
	PeriodStart *time.Time
	PeriodEnd   *time.Time

	Cost     *Decimal
	Currency string

	ConsumedQty  *Decimal
	ConsumedUnit string

	ResourceID   string
	ResourceName string
	ResourceType string
	RegionID     string
	RegionName   string

	SkuID      string
	SkuMeter   string
	SkuPriceID string

	Extensions map[string]any
}
