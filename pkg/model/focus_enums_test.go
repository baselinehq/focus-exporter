package model

import "testing"

func TestEnumConstantsAreValid(t *testing.T) {
	charge := []ChargeCategory{ChargeUsage, ChargePurchase, ChargeTax, ChargeCredit, ChargeAdjustment}
	for _, c := range charge {
		if !c.Valid() {
			t.Errorf("ChargeCategory %q reported invalid", c)
		}
	}
	if ChargeCategory("Bogus").Valid() {
		t.Error("unknown ChargeCategory must be invalid")
	}

	freq := []ChargeFrequency{ChargeFrequencyOneTime, ChargeFrequencyRecurring, ChargeFrequencyUsageBased}
	for _, f := range freq {
		if !f.Valid() {
			t.Errorf("ChargeFrequency %q reported invalid", f)
		}
	}

	pricing := []PricingCategory{PricingStandard, PricingDynamic, PricingCommitted, PricingOther}
	for _, p := range pricing {
		if !p.Valid() {
			t.Errorf("PricingCategory %q reported invalid", p)
		}
	}

	if !ServiceCategoryAIAndMachineLearning.Valid() || !ServiceCategoryDatabases.Valid() {
		t.Error("known ServiceCategory reported invalid")
	}
	if ServiceCategory("Databasesss").Valid() {
		t.Error("unknown ServiceCategory must be invalid")
	}
}
