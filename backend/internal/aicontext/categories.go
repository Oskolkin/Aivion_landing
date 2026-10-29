package aicontext

// AllowedCategories are valid Global Semantic Context categories.
var AllowedCategories = []string{
	"stock",
	"slow_turnover",
	"sku",
	"sales",
	"advertising",
	"funnel",
	"category",
}

// ForbiddenCategories must never appear in Global Semantic Context.
var ForbiddenCategories = []string{
	"data_quality",
	"price",
	"margin",
	"profit",
	"repricing",
}

// AllowedConfidenceValues for global context confidence field.
var AllowedConfidenceValues = []string{
	"high",
	"medium",
	"low",
}

// AllowedSeverityValues for semantic risks.
var AllowedSeverityValues = []string{
	"critical",
	"high",
	"medium",
	"low",
	"watch",
}

func IsAllowedCategory(category string) bool {
	for _, c := range AllowedCategories {
		if c == category {
			return true
		}
	}
	return false
}

func IsForbiddenCategory(category string) bool {
	for _, c := range ForbiddenCategories {
		if c == category {
			return true
		}
	}
	return false
}
