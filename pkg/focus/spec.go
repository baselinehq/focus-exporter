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

var validCurrencyCodes = map[string]struct{}{
	"AED": {}, "AFN": {}, "ALL": {}, "AMD": {}, "ANG": {}, "AOA": {}, "ARS": {}, "AUD": {}, "AWG": {}, "AZN": {},
	"BAM": {}, "BBD": {}, "BDT": {}, "BGN": {}, "BHD": {}, "BIF": {}, "BMD": {}, "BND": {}, "BOB": {}, "BRL": {},
	"BSD": {}, "BTN": {}, "BWP": {}, "BYN": {}, "BZD": {}, "CAD": {}, "CDF": {}, "CHF": {}, "CLP": {}, "CNY": {},
	"COP": {}, "CRC": {}, "CUP": {}, "CVE": {}, "CZK": {}, "DJF": {}, "DKK": {}, "DOP": {}, "DZD": {}, "EGP": {},
	"ERN": {}, "ETB": {}, "EUR": {}, "FJD": {}, "FKP": {}, "GBP": {}, "GEL": {}, "GHS": {}, "GIP": {}, "GMD": {},
	"GNF": {}, "GTQ": {}, "GYD": {}, "HKD": {}, "HNL": {}, "HRK": {}, "HTG": {}, "HUF": {}, "IDR": {}, "ILS": {},
	"INR": {}, "IQD": {}, "IRR": {}, "ISK": {}, "JMD": {}, "JOD": {}, "JPY": {}, "KES": {}, "KGS": {}, "KHR": {},
	"KMF": {}, "KPW": {}, "KRW": {}, "KWD": {}, "KYD": {}, "KZT": {}, "LAK": {}, "LBP": {}, "LKR": {}, "LRD": {},
	"LSL": {}, "LYD": {}, "MAD": {}, "MDL": {}, "MGA": {}, "MKD": {}, "MMK": {}, "MNT": {}, "MOP": {}, "MRU": {},
	"MUR": {}, "MVR": {}, "MWK": {}, "MXN": {}, "MYR": {}, "MZN": {}, "NAD": {}, "NGN": {}, "NIO": {}, "NOK": {},
	"NPR": {}, "NZD": {}, "OMR": {}, "PAB": {}, "PEN": {}, "PGK": {}, "PHP": {}, "PKR": {}, "PLN": {}, "PYG": {},
	"QAR": {}, "RON": {}, "RSD": {}, "RUB": {}, "RWF": {}, "SAR": {}, "SBD": {}, "SCR": {}, "SDG": {}, "SEK": {},
	"SGD": {}, "SHP": {}, "SLE": {}, "SOS": {}, "SRD": {}, "SSP": {}, "STN": {}, "SVC": {}, "SYP": {}, "SZL": {},
	"THB": {}, "TJS": {}, "TMT": {}, "TND": {}, "TOP": {}, "TRY": {}, "TTD": {}, "TWD": {}, "TZS": {}, "UAH": {},
	"UGX": {}, "USD": {}, "UYU": {}, "UZS": {}, "VED": {}, "VES": {}, "VND": {}, "VUV": {}, "WST": {}, "XAF": {},
	"XCD": {}, "XOF": {}, "XPF": {}, "YER": {}, "ZAR": {}, "ZMW": {}, "ZWL": {},
}

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
