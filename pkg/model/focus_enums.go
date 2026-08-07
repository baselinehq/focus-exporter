package model

type ChargeCategory string

const (
	ChargeUsage      ChargeCategory = "Usage"
	ChargePurchase   ChargeCategory = "Purchase"
	ChargeTax        ChargeCategory = "Tax"
	ChargeCredit     ChargeCategory = "Credit"
	ChargeAdjustment ChargeCategory = "Adjustment"
)

func (c ChargeCategory) Valid() bool {
	switch c {
	case ChargeUsage, ChargePurchase, ChargeTax, ChargeCredit, ChargeAdjustment:
		return true
	}
	return false
}

type ChargeFrequency string

const (
	ChargeFrequencyOneTime    ChargeFrequency = "One-Time"
	ChargeFrequencyRecurring  ChargeFrequency = "Recurring"
	ChargeFrequencyUsageBased ChargeFrequency = "Usage-Based"
)

func (f ChargeFrequency) Valid() bool {
	switch f {
	case ChargeFrequencyOneTime, ChargeFrequencyRecurring, ChargeFrequencyUsageBased:
		return true
	}
	return false
}

type PricingCategory string

const (
	PricingStandard  PricingCategory = "Standard"
	PricingDynamic   PricingCategory = "Dynamic"
	PricingCommitted PricingCategory = "Committed"
	PricingOther     PricingCategory = "Other"
)

func (p PricingCategory) Valid() bool {
	switch p {
	case PricingStandard, PricingDynamic, PricingCommitted, PricingOther:
		return true
	}
	return false
}

type ServiceCategory string

const (
	ServiceCategoryAIAndMachineLearning ServiceCategory = "AI and Machine Learning"
	ServiceCategoryAnalytics            ServiceCategory = "Analytics"
	ServiceCategoryBusinessApplications ServiceCategory = "Business Applications"
	ServiceCategoryCompute              ServiceCategory = "Compute"
	ServiceCategoryDatabases            ServiceCategory = "Databases"
	ServiceCategoryDeveloperTools       ServiceCategory = "Developer Tools"
	ServiceCategoryIdentity             ServiceCategory = "Identity"
	ServiceCategoryIntegration          ServiceCategory = "Integration"
	ServiceCategoryInternetOfThings     ServiceCategory = "Internet of Things"
	ServiceCategoryManagementAndGov     ServiceCategory = "Management and Governance"
	ServiceCategoryMedia                ServiceCategory = "Media"
	ServiceCategoryMigration            ServiceCategory = "Migration"
	ServiceCategoryMobile               ServiceCategory = "Mobile"
	ServiceCategoryMulticloud           ServiceCategory = "Multicloud"
	ServiceCategoryNetworking           ServiceCategory = "Networking"
	ServiceCategorySecurity             ServiceCategory = "Security"
	ServiceCategoryStorage              ServiceCategory = "Storage"
	ServiceCategoryWeb                  ServiceCategory = "Web"
	ServiceCategoryOther                ServiceCategory = "Other"
)

func (s ServiceCategory) Valid() bool {
	switch s {
	case ServiceCategoryAIAndMachineLearning, ServiceCategoryAnalytics, ServiceCategoryBusinessApplications,
		ServiceCategoryCompute, ServiceCategoryDatabases, ServiceCategoryDeveloperTools, ServiceCategoryIdentity,
		ServiceCategoryIntegration, ServiceCategoryInternetOfThings, ServiceCategoryManagementAndGov,
		ServiceCategoryMedia, ServiceCategoryMigration, ServiceCategoryMobile, ServiceCategoryMulticloud,
		ServiceCategoryNetworking, ServiceCategorySecurity, ServiceCategoryStorage, ServiceCategoryWeb,
		ServiceCategoryOther:
		return true
	}
	return false
}

type ServiceSubcategory string

const (
	ServiceSubcategoryGenerativeAI       ServiceSubcategory = "Generative AI"
	ServiceSubcategoryManagedDatabase    ServiceSubcategory = "Managed Database"
	ServiceSubcategoryStreamingAnalytics ServiceSubcategory = "Streaming Analytics"
)
