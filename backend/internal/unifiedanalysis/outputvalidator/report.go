package outputvalidator

// Issue is one technical validation finding.
type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Report is the deterministic technical validation result.
type Report struct {
	Valid    bool    `json:"valid"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`

	ActionCount         int `json:"action_count"`
	TopActionCount      int `json:"top_action_count"`
	ReferencedSKUs      int `json:"referenced_skus"`
	ReferencedCampaigns int `json:"referenced_campaigns"`
}

func (r *Report) addError(code, path, message string) {
	r.Errors = append(r.Errors, Issue{Code: code, Path: path, Message: message})
}

func (r *Report) addWarning(code, path, message string) {
	r.Warnings = append(r.Warnings, Issue{Code: code, Path: path, Message: message})
}

const (
	CodeRootMissing             = "root_missing"
	CodeWrongSchemaVersion      = "wrong_schema_version"
	CodeWrongPackHash           = "wrong_pack_hash"
	CodeTooManyTopActions       = "too_many_top_actions"
	CodeOutcomeActionsMismatch  = "outcome_actions_mismatch"
	CodeInsufficientLimitations = "insufficient_limitations"
	CodeInvalidActionIDFormat   = "invalid_action_id_format"
	CodeUnknownEnum             = "unknown_enum"
	CodeEmptyRequiredText       = "empty_required_text"
	CodeEmptySteps              = "empty_steps"
	CodeDuplicateStep           = "duplicate_step"
	CodeDuplicateEntityInAction = "duplicate_entity_in_action"
	CodeDuplicateActionRef      = "duplicate_action_ref"
	CodeDuplicateLimitation     = "duplicate_limitation"
	CodePlaceholderText         = "placeholder_text"
	CodeTextTooLong             = "text_too_long"

	CodeOutputPackIDMismatch                = "output_pack_id_mismatch"
	CodeOutputMetricDateMismatch            = "output_metric_date_mismatch"
	CodeOutputInvalidOutcome                = "output_invalid_outcome"
	CodeOutputActionLimitExceeded           = "actions_count_above_maximum"
	CodeActionsCountBelowMinimum            = "actions_count_below_minimum"
	CodeRequiredActionsCountBelowMinimum    = "required_actions_count_below_minimum"
	CodeRequiredActionIncomplete            = "required_action_incomplete"
	CodeActionDuplicate                     = "action_duplicate"
	CodeActionNotGrounded                   = "action_not_grounded"
	CodeQuickActionRequiredReferenceMissing = "quick_action_required_reference_missing"
	CodeRiskDriverUnknownID                 = "risk_driver_unknown_id"
	CodeRiskDriverDuplicateInstance         = "risk_driver_duplicate_instance"
	CodeRiskDriverUnresolvableReference     = "risk_driver_unresolvable_reference"
	CodeOutputDuplicateActionID             = "output_duplicate_action_id"
	CodeOutputUnknownActionReference        = "output_unknown_action_reference"
	CodeOutputUnknownSKU                    = "output_unknown_sku"
	CodeOutputUnknownCampaign               = "output_unknown_campaign"
	CodeOutputDuplicateSKUAcrossActions     = "output_duplicate_sku_across_actions"
	CodeOutputUnknownLimitation             = "output_unknown_limitation_reference"
	CodeOutputMissingScopeEntities          = "output_missing_scope_entities"

	CodeActionTitleEntityCountMismatch   = "action_title_entity_count_mismatch"
	CodeActionSummaryEntityCountMismatch = "action_summary_entity_count_mismatch"
	CodeActionExpectedResultTooGeneric   = "action_expected_result_too_generic"
	CodeActionPlanTooShort               = "action_plan_too_short"
	CodeActionSituationTooShort          = "action_situation_too_short"
	CodeActionImportanceTooShort         = "action_importance_too_short"

	// AI analytical layer completeness (material outcomes only). These never
	// validate factual KPI/array content — that is backend-owned.
	CodeMissingAccountSummaryHeadline     = "missing_account_summary_headline"
	CodeMissingExecutiveSummary           = "missing_executive_summary"
	CodeMissingHealthScore                = "health_score_missing"
	CodeInvalidHealthScore                = "health_score_out_of_range"
	CodeHealthScoreDefaultedToZero        = "health_score_defaulted_to_zero"
	CodeHealthScoreFactorsMissing         = "health_score_factors_missing"
	CodeHealthScoreStatusMismatch         = "health_score_status_mismatch"
	CodeMissingWhatChanged                = "missing_what_changed"
	CodeInvalidWhatChangedKeys            = "invalid_what_changed_keys"
	CodeMissingSellerPulseNarrative       = "missing_seller_pulse_narrative"
	CodeInvalidSellerPulseNarrativeStatus = "invalid_seller_pulse_narrative_status"
	CodeMissingSellerPulseNarrativeText   = "missing_seller_pulse_narrative_text"

	// CodeMissingAdvertisingChecklist/CodeMissingCategorySummaries/
	// CodeIncompleteCategorySummary enforce that the AI actually emits its
	// advertising checklist / category analysis whenever the Pack carries
	// the underlying data. The backend never invents checklist templates or
	// category scores — these codes only ever reject an output for
	// omitting content the AI was required to produce itself.
	CodeMissingAdvertisingChecklist               = "missing_advertising_checklist"
	CodeAdvertisingHeadlineMissing                = "advertising_headline_missing"
	CodeAdvertisingHeadlineTruncated              = "advertising_headline_truncated"
	CodeAdvertisingHeadlineTooGeneric             = "advertising_headline_too_generic"
	CodeAdvertisingExplanationMissing             = "advertising_explanation_missing"
	CodeAdvertisingExplanationFragmented          = "advertising_explanation_fragmented"
	CodeAdvertisingExplanationTooShallow          = "advertising_explanation_too_shallow"
	CodeAdvertisingHeadlineExplanationDuplicate   = "advertising_headline_explanation_duplicate"
	CodeAdvertisingRecommendationMissing          = "advertising_recommendation_missing"
	CodeAdvertisingRecommendationNotGrounded      = "advertising_recommendation_not_grounded"
	CodeAdvertisingNarrativeFactMismatch          = "advertising_narrative_fact_mismatch"
	CodeAdvertisingBareDRRForStoreShare           = "advertising_bare_drr_for_store_share"
	CodeAdvertisingRevenueDenominatorMixed        = "advertising_revenue_denominator_mixed"
	CodeAdvertisingTotalRevenueShareMismatch      = "advertising_total_revenue_share_mismatch"
	CodeAdvertisingAttributedRevenueShareMismatch = "advertising_attributed_revenue_share_mismatch"

	// Advertising claim-based grounding (Wave 2): ad_efficiency.narrative.
	// metric_claims replace the old adapter-side text-rewrite heuristics.
	// Every claim must use a canonical metric_code with a valid period and
	// (when Pack account metrics are indexed) match the Pack fact value;
	// every free-text percentage in headline/summary must be backed by a
	// matching claim. Mismatches are hard errors that block publication —
	// the AI text itself is never rewritten.
	CodeAdvertisingClaimMetricMissing         = "advertising_claim_metric_missing"
	CodeAdvertisingClaimUnknownMetricCode     = "advertising_claim_unknown_metric_code"
	CodeAdvertisingClaimFactMissing           = "advertising_claim_fact_missing"
	CodeAdvertisingClaimFactUnknown           = "advertising_claim_fact_unknown"
	CodeAdvertisingClaimPeriodMissing         = "advertising_claim_period_missing"
	CodeAdvertisingClaimMetricMismatch        = "advertising_claim_metric_mismatch"
	CodeAdvertisingClaimValueMismatch         = "advertising_claim_value_mismatch"
	CodeAdvertisingRevenueDenominatorConfused = "advertising_revenue_denominator_confused"
	CodeAdvertisingClaimPercentWithoutClaim   = "advertising_claim_percent_without_claim"
	// CodeAdvertisingPercentClaimAmbiguous: free-text percents and claims
	// exist, but the sentence does not uniquely map each percent to one
	// metric (or one claim would cover multiple distinct percents).
	CodeAdvertisingPercentClaimAmbiguous = "advertising_percent_claim_ambiguous"
	CodeMissingCategorySummaries         = "missing_category_summaries"
	CodeIncompleteCategorySummary        = "incomplete_category_summary"
	CodeMissingFunnelAnalysis            = "missing_funnel_analysis"
	CodeMissingRiskDistribution          = "missing_risk_distribution"
	CodeMissingSKUFocus                  = "missing_sku_focus"

	// AI-number-vs-facts mismatch is a warning-only signal (never a hard
	// failure): the AI analytical layer may disagree with backend-computed
	// KPI numbers without blocking publication.
	CodeAINumberFactMismatch = "ai_number_fact_mismatch"

	// Per-action evidence + completion-condition grounding (schema v8+).
	// Every action must carry at least one evidence_fact_id, each
	// evidence/baseline fact_id that references a Pack entity/account
	// metric must resolve in the Pack facts registry, and each machine
	// completion condition must carry a loadable baseline that is not
	// already satisfied and stays consistent with the text criterion.
	CodeActionEvidenceMissing               = "action_evidence_missing"
	CodeActionEvidenceFactUnknown           = "action_evidence_fact_unknown"
	CodeActionEvidenceScopeInconsistent     = "action_evidence_scope_inconsistent"
	CodeActionCompletionBaselineMissing     = "action_completion_baseline_missing"
	CodeActionCompletionTargetAlreadyMet    = "action_completion_target_already_met"
	CodeActionCompletionTextMachineMismatch = "action_completion_text_machine_mismatch"

	// CodeActionDomainCoverageIncomplete fires when a hard-required material
	// domain (stock OOS/critical, high-DRR advertising, spend-without-orders,
	// material sales decline) has Pack-grounded candidates that remain
	// completely uncovered — no Action of the matching type exists and no
	// candidate entity is owned by any other Action — after
	// EnsureMaterialDomainActions ran. This is a safety net for the rare
	// case where the contract.MaxActionsV1 cap was reached before every
	// material domain could be represented; it never fires when idx is nil.
	CodeActionDomainCoverageIncomplete = "action_domain_coverage_incomplete"

	// Stock cohort / metric publication gates (P0).
	CodeStockCohortOverlap              = "stock_cohort_overlap"
	CodeStockDeficitSurplusOverlap      = "stock_deficit_surplus_overlap"
	CodeStockUmbrellaSubgroupDuplicate  = "stock_umbrella_subgroup_duplicate"
	CodeActionContainsInternalFieldName = "action_contains_internal_field_name"
)
