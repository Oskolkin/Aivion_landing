package contract

// UnifiedAnalysisOutputV11 is the AI-facing unified analysis response
// contract for schema_version aivion_unified_analysis_v11. It replaces the
// v1-v10 dashboard shape (account_summary/what_changed/... free-text blocks)
// with a findings-first model: every narrative/dashboard block references
// shared Finding records by finding_id, and every factual claim is grounded
// via MetricClaim/ThresholdClaim rather than embedded ad hoc in prose.
//
// UnifiedAnalysisOutputV1 (and its nested V1/V2 types in output_v1.go) are
// left unchanged; this is an independent, self-contained struct tree.
type UnifiedAnalysisOutputV11 struct {
	SchemaVersion   string                     `json:"schema_version"`
	AnalysisMeta    UnifiedAnalysisMetaV11     `json:"analysis_meta"`
	AnalysisOutcome string                     `json:"analysis_outcome"`
	MetricClaims    []UnifiedMetricClaimV11    `json:"metric_claims"`
	ThresholdClaims []UnifiedThresholdClaimV11 `json:"threshold_claims"`
	Findings        []UnifiedFindingV11        `json:"findings"`
	Dashboard       UnifiedDashboardV11        `json:"dashboard"`
	Actions         []UnifiedActionV11         `json:"actions"`
	Limitations     []UnifiedLimitationV11     `json:"limitations"`
}

// UnifiedAnalysisMetaV11 references the Analysis Pack used for the run.
type UnifiedAnalysisMetaV11 struct {
	PackID     string `json:"pack_id"`
	PackHash   string `json:"pack_hash"`
	MetricDate string `json:"metric_date"`
}

// UnifiedMetricClaimV11 is one numeric assertion grounded in exactly one
// current Pack fact, with an optional previous-period fact for delta
// rendering. ScopeType/EntityID identify what the claim is about;
// RenderFormat/DeltaMode tell the frontend how to render current vs delta.
type UnifiedMetricClaimV11 struct {
	ClaimID    string `json:"claim_id"`
	MetricCode string `json:"metric_code"`
	// ScopeType is one of: account, funnel, sku, campaign, category.
	ScopeType      string  `json:"scope_type"`
	EntityID       *string `json:"entity_id"`
	CurrentFactID  string  `json:"current_fact_id"`
	PreviousFactID *string `json:"previous_fact_id"`
	// RenderFormat is one of: money, integer, percent, days, units, ratio, decimal.
	RenderFormat string `json:"render_format"`
	// DeltaMode is one of: none, absolute, percent, percentage_points.
	DeltaMode string `json:"delta_mode"`
}

// UnifiedThresholdClaimV11 is a policy-backed comparative threshold cited by
// a Finding, grounded in the versioned business policy registry via
// PolicyID and the underlying MetricClaim via MetricClaimID. The AI must
// copy PolicyID verbatim — it never invents thresholds.
type UnifiedThresholdClaimV11 struct {
	ClaimID       string `json:"claim_id"`
	PolicyID      string `json:"policy_id"`
	MetricClaimID string `json:"metric_claim_id"`
}

// UnifiedFindingScopeV11 identifies what a Finding is about.
type UnifiedFindingScopeV11 struct {
	// ScopeType is one of: account, funnel, sku, campaign, category.
	ScopeType string   `json:"scope_type"`
	EntityIDs []string `json:"entity_ids"`
}

// UnifiedFindingV11 is one atomic analytical finding. Findings are the
// single source of truth referenced by dashboard blocks (via finding_id)
// and by Actions (via finding_ids) — dashboard prose and actions never
// restate facts independently of a Finding.
type UnifiedFindingV11 struct {
	FindingID string `json:"finding_id"`
	// Domain is one of: sales, funnel, stock, advertising, sku, category,
	// data_quality.
	Domain string `json:"domain"`
	// FindingType is a closed enum of all finding types (contract section 11).
	FindingType string `json:"finding_type"`
	// PrimaryBlock is the dashboard block that owns primary rendering of
	// this finding: account, what_changed, category, funnel, stock,
	// advertising, seller_pulse, sku_focus.
	PrimaryBlock string `json:"primary_block"`
	// Direction is one of: positive, negative, neutral.
	Direction string `json:"direction"`
	// Severity is one of: critical, important, watch, none.
	Severity string `json:"severity"`
	// Interpretation is one of: observed, derived, hypothesis.
	Interpretation         string                 `json:"interpretation"`
	Title                  string                 `json:"title"`
	ExplanationTemplate    string                 `json:"explanation_template"`
	BusinessImpactTemplate *string                `json:"business_impact_template"`
	Scope                  UnifiedFindingScopeV11 `json:"scope"`
	MetricClaimIDs         []string               `json:"metric_claim_ids"`
	ThresholdClaimIDs      []string               `json:"threshold_claim_ids"`
	// Confidence is one of: high, medium, low.
	Confidence      string   `json:"confidence"`
	LimitationCodes []string `json:"limitation_codes"`
}

// UnifiedImprovementLeverV11 links an account-level lever to the Finding it
// addresses and, optionally, to the Action that resolves it.
type UnifiedImprovementLeverV11 struct {
	Title               string  `json:"title"`
	ExplanationTemplate string  `json:"explanation_template"`
	FindingID           string  `json:"finding_id"`
	LinkedActionID      *string `json:"linked_action_id"`
	// ExpectedDirection is one of: risk_decrease, revenue_support,
	// conversion_improvement, cost_efficiency, stock_stability,
	// data_quality_improvement.
	ExpectedDirection string `json:"expected_direction"`
}

// UnifiedAccountDiagnosisV11 is the dashboard.account_diagnosis block.
type UnifiedAccountDiagnosisV11 struct {
	// Status is one of: available, limited, unavailable.
	Status                   string                       `json:"status"`
	Headline                 *string                      `json:"headline"`
	ExecutiveSummaryTemplate *string                      `json:"executive_summary_template"`
	PrimaryFindingIDs        []string                     `json:"primary_finding_ids"`
	PositiveFindingIDs       []string                     `json:"positive_finding_ids"`
	ImprovementLevers        []UnifiedImprovementLeverV11 `json:"improvement_levers"`
	LimitationCodes          []string                     `json:"limitation_codes"`
}

// UnifiedChangeItemV11 is one entry inside a WhatChanged group, referencing
// the Finding that changed and, optionally, the Action that addresses it.
type UnifiedChangeItemV11 struct {
	ChangeID       string  `json:"change_id"`
	FindingID      string  `json:"finding_id"`
	LinkedActionID *string `json:"linked_action_id"`
}

// UnifiedWhatChangedV11 is the dashboard.what_changed block.
type UnifiedWhatChangedV11 struct {
	// Status is one of: available, limited, unavailable.
	Status          string                 `json:"status"`
	SummaryTemplate *string                `json:"summary_template"`
	Worsened        []UnifiedChangeItemV11 `json:"worsened"`
	NewRisks        []UnifiedChangeItemV11 `json:"new_risks"`
	UnchangedRisks  []UnifiedChangeItemV11 `json:"unchanged_risks"`
	Improved        []UnifiedChangeItemV11 `json:"improved"`
	LimitationCodes []string               `json:"limitation_codes"`
}

// UnifiedCategoryInsightItemV11 is one category diagnosis card.
type UnifiedCategoryInsightItemV11 struct {
	CategoryID        string   `json:"category_id"`
	Headline          string   `json:"headline"`
	DiagnosisTemplate string   `json:"diagnosis_template"`
	FindingIDs        []string `json:"finding_ids"`
	LinkedActionID    *string  `json:"linked_action_id"`
	// Confidence is one of: high, medium, low.
	Confidence string `json:"confidence"`
}

// UnifiedCategoryInsightsV11 is the dashboard.category_insights block.
type UnifiedCategoryInsightsV11 struct {
	// Status is one of: available, limited, unavailable.
	Status          string                          `json:"status"`
	Items           []UnifiedCategoryInsightItemV11 `json:"items"`
	LimitationCodes []string                        `json:"limitation_codes"`
}

// UnifiedRecommendedCheckV11 is a suggested manual check surfaced by
// FunnelInsight, StockInsight, or AdvertisingInsight.
type UnifiedRecommendedCheckV11 struct {
	CheckType      string   `json:"check_type"`
	Title          string   `json:"title"`
	ReasonTemplate string   `json:"reason_template"`
	FindingIDs     []string `json:"finding_ids"`
	LinkedActionID *string  `json:"linked_action_id"`
}

// UnifiedFunnelInsightV11 is the dashboard.funnel_insight block.
type UnifiedFunnelInsightV11 struct {
	// Status is one of: available, limited, unavailable.
	Status                  string                       `json:"status"`
	WeakestTransitionKey    *string                      `json:"weakest_transition_key"`
	Headline                *string                      `json:"headline"`
	DiagnosisTemplate       *string                      `json:"diagnosis_template"`
	BusinessMeaningTemplate *string                      `json:"business_meaning_template"`
	PositiveContextTemplate *string                      `json:"positive_context_template"`
	RecommendedChecks       []UnifiedRecommendedCheckV11 `json:"recommended_checks"`
	FindingIDs              []string                     `json:"finding_ids"`
	// Confidence is one of: high, medium, low; nullable when unavailable.
	Confidence      *string  `json:"confidence"`
	LimitationCodes []string `json:"limitation_codes"`
}

// UnifiedStockInsightV11 is the dashboard.stock_insight block.
type UnifiedStockInsightV11 struct {
	// Status is one of: available, limited, unavailable.
	Status             string                       `json:"status"`
	Headline           *string                      `json:"headline"`
	DiagnosisTemplate  *string                      `json:"diagnosis_template"`
	PrimaryFindingIDs  []string                     `json:"primary_finding_ids"`
	PositiveFindingIDs []string                     `json:"positive_finding_ids"`
	RecommendedChecks  []UnifiedRecommendedCheckV11 `json:"recommended_checks"`
	// Confidence is one of: high, medium, low; nullable when unavailable.
	Confidence      *string  `json:"confidence"`
	LimitationCodes []string `json:"limitation_codes"`
}

// UnifiedAdvertisingInsightV11 is the dashboard.advertising_insight block
// (same shape as UnifiedStockInsightV11, kept as an independent type since
// the two blocks evolve separately in the contract).
type UnifiedAdvertisingInsightV11 struct {
	// Status is one of: available, limited, unavailable.
	Status             string                       `json:"status"`
	Headline           *string                      `json:"headline"`
	DiagnosisTemplate  *string                      `json:"diagnosis_template"`
	PrimaryFindingIDs  []string                     `json:"primary_finding_ids"`
	PositiveFindingIDs []string                     `json:"positive_finding_ids"`
	RecommendedChecks  []UnifiedRecommendedCheckV11 `json:"recommended_checks"`
	// Confidence is one of: high, medium, low; nullable when unavailable.
	Confidence      *string  `json:"confidence"`
	LimitationCodes []string `json:"limitation_codes"`
}

// UnifiedSellerPulseInsightV11 is the dashboard.seller_pulse_insight block.
type UnifiedSellerPulseInsightV11 struct {
	// Status is one of: available, limited, unavailable.
	Status                  string   `json:"status"`
	Headline                *string  `json:"headline"`
	SummaryTemplate         *string  `json:"summary_template"`
	KeyPointFindingIDs      []string `json:"key_point_finding_ids"`
	ImportantFactFindingIDs []string `json:"important_fact_finding_ids"`
	LimitationCodes         []string `json:"limitation_codes"`
}

// UnifiedSKUFocusItemV11 is one SKU reference inside SKUFocusSelection.
type UnifiedSKUFocusItemV11 struct {
	SKUID             string `json:"sku_id"`
	FindingID         string `json:"finding_id"`
	PrimaryMetricCode string `json:"primary_metric_code"`
}

// UnifiedSKUFocusSelectionV11 is the dashboard.sku_focus_selection block.
type UnifiedSKUFocusSelectionV11 struct {
	// Status is one of: available, limited, unavailable.
	Status           string                   `json:"status"`
	OrderDrop        []UnifiedSKUFocusItemV11 `json:"order_drop"`
	StockRisk        []UnifiedSKUFocusItemV11 `json:"stock_risk"`
	NoSalesWithStock []UnifiedSKUFocusItemV11 `json:"no_sales_with_stock"`
	SlowTurnover     []UnifiedSKUFocusItemV11 `json:"slow_turnover"`
	KeySKUs          []UnifiedSKUFocusItemV11 `json:"key_skus"`
	LimitationCodes  []string                 `json:"limitation_codes"`
}

// UnifiedDashboardV11 carries exactly the eight v11 dashboard blocks.
type UnifiedDashboardV11 struct {
	AccountDiagnosis   UnifiedAccountDiagnosisV11   `json:"account_diagnosis"`
	WhatChanged        UnifiedWhatChangedV11        `json:"what_changed"`
	CategoryInsights   UnifiedCategoryInsightsV11   `json:"category_insights"`
	FunnelInsight      UnifiedFunnelInsightV11      `json:"funnel_insight"`
	StockInsight       UnifiedStockInsightV11       `json:"stock_insight"`
	AdvertisingInsight UnifiedAdvertisingInsightV11 `json:"advertising_insight"`
	SellerPulseInsight UnifiedSellerPulseInsightV11 `json:"seller_pulse_insight"`
	SKUFocusSelection  UnifiedSKUFocusSelectionV11  `json:"sku_focus_selection"`
}

// UnifiedActionScopeV11 identifies what an Action applies to. Unlike
// UnifiedActionScopeV1, "mixed" is not a valid ScopeType in v11 — every
// Action targets exactly one scope kind.
type UnifiedActionScopeV11 struct {
	// ScopeType is one of: account, sku, campaign, funnel, category.
	ScopeType string   `json:"scope_type"`
	EntityIDs []string `json:"entity_ids"`
}

// UnifiedPlanStepV11 is one ordered execution step inside an Action's plan.
type UnifiedPlanStepV11 struct {
	Order             int      `json:"order"`
	Instruction       string   `json:"instruction"`
	RationaleTemplate string   `json:"rationale_template"`
	ExpectedResult    string   `json:"expected_result"`
	EntityIDs         []string `json:"entity_ids"`
}

// UnifiedCompletionConditionV11 is a flattened representation of the three
// completion-condition kinds (metric_improvement | policy_threshold |
// manual_confirmation). Type is the discriminator; fields not applicable to
// a given Type are left nil. This mirrors the JSON schema's anyOf of
// MetricImprovementCondition / PolicyThresholdCondition /
// ManualConfirmationCondition without requiring a custom (un)marshaler.
type UnifiedCompletionConditionV11 struct {
	ConditionID string `json:"condition_id"`
	// Type is one of: metric_improvement, policy_threshold, manual_confirmation.
	Type  string `json:"type"`
	Title string `json:"title"`

	// MetricCode/EntityID/PeriodDays apply to metric_improvement and
	// policy_threshold conditions only.
	MetricCode *string `json:"metric_code,omitempty"`
	EntityID   *string `json:"entity_id,omitempty"`
	PeriodDays *int    `json:"period_days,omitempty"`

	// BaselineClaimID/Direction apply to metric_improvement only.
	// Direction is one of: increase, decrease.
	BaselineClaimID *string `json:"baseline_claim_id,omitempty"`
	Direction       *string `json:"direction,omitempty"`

	// PolicyID applies to policy_threshold only.
	PolicyID *string `json:"policy_id,omitempty"`

	// ConfirmationText applies to manual_confirmation only.
	ConfirmationText *string `json:"confirmation_text,omitempty"`
}

// UnifiedCompletionV11 defines how an Action is considered complete.
type UnifiedCompletionV11 struct {
	// Mode is one of: all, any.
	Mode       string                          `json:"mode"`
	Conditions []UnifiedCompletionConditionV11 `json:"conditions"`
}

// UnifiedObjectInstructionV11 is a per-entity instruction attached to an Action.
type UnifiedObjectInstructionV11 struct {
	EntityID       string `json:"entity_id"`
	Instruction    string `json:"instruction"`
	ReasonTemplate string `json:"reason_template"`
	ExpectedResult string `json:"expected_result"`
}

// UnifiedActionV11 is one seller-facing action in the v11 unified AI output.
// Unlike UnifiedActionV1, there is no Category field — Type plus Scope fully
// classify the action.
type UnifiedActionV11 struct {
	AnalysisActionID string   `json:"analysis_action_id"`
	FindingIDs       []string `json:"finding_ids"`
	// Type is one of the 13 canonical action types (unchanged from v10):
	// stock_replenishment, stock_redistribution, slow_turnover_review,
	// no_sales_with_stock, sku_orders_drop, key_sku_protection,
	// ad_high_drr_review, ad_spend_no_orders, ad_budget_reallocation,
	// funnel_weak_transition, card_conversion_review,
	// category_underperformance, sales_drop_review.
	Type string `json:"type"`
	// Priority is one of: critical, important, watch.
	Priority string `json:"priority"`
	// ExecutionTier is one of: required, recommended.
	ExecutionTier          string                        `json:"execution_tier"`
	Title                  string                        `json:"title"`
	SituationTemplate      string                        `json:"situation_template"`
	Goal                   string                        `json:"goal"`
	Scope                  UnifiedActionScopeV11         `json:"scope"`
	EntityIDs              []string                      `json:"entity_ids"`
	Plan                   []UnifiedPlanStepV11          `json:"plan"`
	ExpectedEffectTemplate string                        `json:"expected_effect_template"`
	Completion             UnifiedCompletionV11          `json:"completion"`
	ObjectInstructions     []UnifiedObjectInstructionV11 `json:"object_instructions"`
	LimitationCodes        []string                      `json:"limitation_codes"`
	// Confidence is one of: high, medium, low.
	Confidence string `json:"confidence"`
}

// UnifiedLimitationV11 is a structured data-availability limitation
// (replacing the free-text v1-v10 limitations string list).
type UnifiedLimitationV11 struct {
	Code string `json:"code"`
	// Scope is one of: account, history, funnel, stock, advertising, sku, category.
	Scope          string   `json:"scope"`
	AffectedBlocks []string `json:"affected_blocks"`
}
