package focus

import (
	"time"

	"github.com/baselinehq/focus-exporter/pkg/model"
)

func FromUsage(u model.UsageRecord) Record {
	start, end := chargePeriod(u)

	chargeCategory := u.ChargeCategory
	if chargeCategory == "" {
		chargeCategory = "Usage"
	}

	r := Record{
		BillingCurrency:    u.Currency,
		BillingPeriodStart: start,
		BillingPeriodEnd:   end,
		ChargeCategory:     chargeCategory,
		ChargePeriodStart:  start,
		ChargePeriodEnd:    end,
		ServiceName:        u.ServiceName,
		Provider:           u.Provider,
		Extensions:         u.Extensions,
	}

	if u.Cost != nil {
		cost := string(*u.Cost)
		r.BilledCost = cost
		r.EffectiveCost = cost
		r.ListCost = cost
	}

	if u.ConsumedQty != nil {
		qty := string(*u.ConsumedQty)
		r.ConsumedQuantity = &qty
	}

	r.ServiceCategory = optional(u.ServiceCategory)
	r.ServiceSubcategory = optional(u.ServiceSubcategory)
	r.ChargeDescription = optional(u.ChargeDescription)
	r.ConsumedUnit = optional(u.ConsumedUnit)
	r.ResourceId = optional(u.ResourceID)
	r.ResourceName = optional(u.ResourceName)
	r.ResourceType = optional(u.ResourceType)
	r.RegionId = optional(u.RegionID)
	r.RegionName = optional(u.RegionName)
	r.SkuId = optional(u.SkuID)
	r.SkuMeter = optional(u.SkuMeter)
	r.SkuPriceId = optional(u.SkuPriceID)

	return r
}

func chargePeriod(u model.UsageRecord) (time.Time, time.Time) {
	if u.PeriodStart != nil && u.PeriodEnd != nil {
		return *u.PeriodStart, *u.PeriodEnd
	}
	return u.Day, u.Day.Add(24 * time.Hour)
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
