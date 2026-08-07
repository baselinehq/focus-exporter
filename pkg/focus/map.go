package focus

import (
	"encoding/json"
	"time"

	"github.com/baselinehq/focus-exporter/pkg/model"
)

func FromUsage(u model.UsageRecord) Record {
	start, end := chargePeriod(u)

	chargeCategory := u.ChargeCategory
	if chargeCategory == "" {
		chargeCategory = model.ChargeUsage
	}

	r := Record{
		BillingCurrency:    u.Currency,
		BillingPeriodStart: start,
		BillingPeriodEnd:   end,
		ChargeCategory:     string(chargeCategory),
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
		r.ContractedCost = cost
	}
	if u.ContractedCost != nil {
		r.ContractedCost = string(*u.ContractedCost)
	}

	if u.ConsumedQty != nil {
		qty := string(*u.ConsumedQty)
		r.ConsumedQuantity = &qty
	}

	r.ServiceCategory = optional(string(u.ServiceCategory))
	r.ServiceSubcategory = optional(string(u.ServiceSubcategory))
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

	r.BillingAccountId = u.BillingAccountID
	r.Publisher = optional(u.Publisher)
	r.InvoiceIssuer = optional(u.InvoiceIssuer)
	r.ChargeFrequency = optional(string(u.ChargeFrequency))
	r.PricingCategory = optional(string(u.PricingCategory))
	r.PricingCurrency = optional(u.PricingCurrency)
	r.PricingUnit = optional(u.PricingUnit)
	r.PricingQuantity = decimalPtr(u.PricingQty)
	r.ListUnitPrice = decimalPtr(u.ListUnitPrice)
	r.ContractedUnitPrice = decimalPtr(u.ContractedUnitPrice)
	r.SkuPriceDetails = skuPriceDetails(u.SkuPriceDetails)

	return r
}

func decimalPtr(d *model.Decimal) *string {
	if d == nil {
		return nil
	}
	s := string(*d)
	return &s
}

func skuPriceDetails(m map[string]any) *json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	msg := json.RawMessage(raw)
	return &msg
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
