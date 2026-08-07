package focus

const ChargeClassCorrection = "Correction"

const (
	ChargeCategoryUsage      = "Usage"
	ChargeCategoryPurchase   = "Purchase"
	ChargeCategoryTax        = "Tax"
	ChargeCategoryCredit     = "Credit"
	ChargeCategoryAdjustment = "Adjustment"
)

const (
	ServiceCategoryAI                   = "AI and Machine Learning"
	ServiceCategoryAnalytics            = "Analytics"
	ServiceCategoryBusinessApplications = "Business Applications"
	ServiceCategoryCompute              = "Compute"
	ServiceCategoryDatabases            = "Databases"
	ServiceCategoryDeveloperTools       = "Developer Tools"
	ServiceCategoryIdentity             = "Identity"
	ServiceCategoryIntegration          = "Integration"
	ServiceCategoryInternetOfThings     = "Internet of Things"
	ServiceCategoryManagementGovernance = "Management and Governance"
	ServiceCategoryMedia                = "Media"
	ServiceCategoryMigration            = "Migration"
	ServiceCategoryMobile               = "Mobile"
	ServiceCategoryMulticloud           = "Multicloud"
	ServiceCategoryNetworking           = "Networking"
	ServiceCategorySecurity             = "Security"
	ServiceCategoryStorage              = "Storage"
	ServiceCategoryWeb                  = "Web"
	ServiceCategoryOther                = "Other"
)

var validChargeCategories = map[string]struct{}{
	ChargeCategoryUsage: {}, ChargeCategoryPurchase: {}, ChargeCategoryTax: {},
	ChargeCategoryCredit: {}, ChargeCategoryAdjustment: {},
}

var validServiceCategories = map[string]struct{}{
	ServiceCategoryAI: {}, ServiceCategoryAnalytics: {}, ServiceCategoryBusinessApplications: {},
	ServiceCategoryCompute: {}, ServiceCategoryDatabases: {}, ServiceCategoryDeveloperTools: {},
	ServiceCategoryIdentity: {}, ServiceCategoryIntegration: {}, ServiceCategoryInternetOfThings: {},
	ServiceCategoryManagementGovernance: {}, ServiceCategoryMedia: {}, ServiceCategoryMigration: {},
	ServiceCategoryMobile: {}, ServiceCategoryMulticloud: {}, ServiceCategoryNetworking: {},
	ServiceCategorySecurity: {}, ServiceCategoryStorage: {}, ServiceCategoryWeb: {},
	ServiceCategoryOther: {},
}
