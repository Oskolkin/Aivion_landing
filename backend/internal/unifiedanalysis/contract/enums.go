package contract

// Allowed action taxonomy for Unified Analysis Output v1.
// Values must stay aligned with actions.ActionsAIFirstActive* (see contract_test.go).
// growth priority is intentionally excluded — frontend/backend do not support it.

var AllowedActionTypesV1 = []string{
	"stock_replenishment",
	"stock_redistribution",
	"slow_turnover_review",
	"no_sales_with_stock",
	"sku_orders_drop",
	"key_sku_protection",
	"ad_high_drr_review",
	"ad_spend_no_orders",
	"ad_budget_reallocation",
	"funnel_weak_transition",
	"card_conversion_review",
	"category_underperformance",
	"sales_drop_review",
}

var AllowedActionCategoriesV1 = []string{
	"stock",
	"slow_turnover",
	"sku",
	"sales",
	"advertising",
	"funnel",
	"category",
}

var AllowedActionPrioritiesV1 = []string{
	"critical",
	"important",
	"watch",
}

// Execution tiers for UnifiedActionV1 (schema v6+).
const (
	ExecutionTierRequired    = "required"
	ExecutionTierRecommended = "recommended"
)

// AllowedExecutionTiersV1 lists valid execution_tier values.
var AllowedExecutionTiersV1 = []string{
	ExecutionTierRequired,
	ExecutionTierRecommended,
}

// Analysis outcomes for UnifiedAnalysisOutputV1.
const (
	AnalysisOutcomeMaterialActionsFound = "material_actions_found"
	AnalysisOutcomeNoMaterialActions    = "no_material_actions"
	AnalysisOutcomeInsufficientData     = "insufficient_data"
	AnalysisOutcomeAnalysisFailed       = "analysis_failed"
)

// AllowedAnalysisOutcomesV1 lists valid analysis_outcome values.
var AllowedAnalysisOutcomesV1 = []string{
	AnalysisOutcomeMaterialActionsFound,
	AnalysisOutcomeNoMaterialActions,
	AnalysisOutcomeInsufficientData,
	AnalysisOutcomeAnalysisFailed,
}

// Action scope types for UnifiedActionScopeV1.
var AllowedActionScopeTypesV1 = []string{
	"account",
	"sku",
	"campaign",
	"funnel",
	"category",
	"mixed",
}

// Confidence levels for UnifiedActionV1.
var AllowedActionConfidenceV1 = []string{
	"high",
	"medium",
	"low",
}
