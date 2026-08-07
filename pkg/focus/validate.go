package focus

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"golang.org/x/text/currency"
)

func isISO4217(code string) bool {
	if code != strings.ToUpper(code) {
		return false
	}
	_, err := currency.ParseISO(code)
	return err == nil
}

func finiteDecimal(name, v string) error {
	if v == "" {
		return fmt.Errorf("focus: %s is required", name)
	}
	if _, err := decimal.NewFromString(v); err != nil {
		return fmt.Errorf("focus: %s %q is not a valid decimal", name, v)
	}
	return nil
}

func Validate(r Record) error {
	if _, ok := validChargeCategories[r.ChargeCategory]; !ok {
		return fmt.Errorf("focus: invalid ChargeCategory %q", r.ChargeCategory)
	}
	if r.ChargeClass != nil && *r.ChargeClass != ChargeClassCorrection {
		return fmt.Errorf("focus: invalid ChargeClass %q", *r.ChargeClass)
	}
	if r.ServiceCategory != nil {
		if _, ok := validServiceCategories[*r.ServiceCategory]; !ok {
			return fmt.Errorf("focus: invalid ServiceCategory %q", *r.ServiceCategory)
		}
	}
	for _, m := range []struct{ name, value string }{
		{"BillingAccountId", r.BillingAccountId},
		{"BillingCurrency", r.BillingCurrency},
		{"ServiceName", r.ServiceName},
		{"Provider", r.Provider},
	} {
		if m.value == "" {
			return fmt.Errorf("focus: %s is required", m.name)
		}
	}
	if r.ChargeDescription == nil || *r.ChargeDescription == "" {
		return fmt.Errorf("focus: ChargeDescription is required")
	}
	if !isISO4217(r.BillingCurrency) {
		return fmt.Errorf("focus: BillingCurrency %q is not an ISO 4217 code", r.BillingCurrency)
	}
	for _, c := range []struct{ name, value string }{
		{"BilledCost", r.BilledCost},
		{"EffectiveCost", r.EffectiveCost},
		{"ListCost", r.ListCost},
	} {
		if err := finiteDecimal(c.name, c.value); err != nil {
			return err
		}
	}
	if r.ContractedCost != "" {
		if err := finiteDecimal("ContractedCost", r.ContractedCost); err != nil {
			return err
		}
	}
	if r.ChargePeriodStart.IsZero() {
		return fmt.Errorf("focus: ChargePeriodStart is required")
	}
	if r.ChargePeriodEnd.IsZero() {
		return fmt.Errorf("focus: ChargePeriodEnd is required")
	}
	if !r.ChargePeriodEnd.After(r.ChargePeriodStart) {
		return fmt.Errorf("focus: ChargePeriodEnd %s must be after ChargePeriodStart %s", r.ChargePeriodEnd, r.ChargePeriodStart)
	}
	if r.BillingPeriodStart.IsZero() {
		return fmt.Errorf("focus: BillingPeriodStart is required")
	}
	if r.BillingPeriodEnd.IsZero() {
		return fmt.Errorf("focus: BillingPeriodEnd is required")
	}
	if !r.BillingPeriodEnd.After(r.BillingPeriodStart) {
		return fmt.Errorf("focus: BillingPeriodEnd %s must be after BillingPeriodStart %s", r.BillingPeriodEnd, r.BillingPeriodStart)
	}
	return nil
}
