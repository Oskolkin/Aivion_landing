// Package bundlevalidator validates a fully-projected Unified Analysis
// publication bundle (backend Dashboard Facts + merged DashboardProjection +
// projected actions) before it is persisted by Stage 5 publication. It is
// the last gate before publish: a bundle that fails validation must never
// be persisted, and the run's publication must be recorded as
// publication_failed with the returned error code rather than silently
// publishing incomplete or inconsistent data.
//
// See docs/current/dashboard-field-ownership.md for the field-ownership
// rules this validator enforces.
package bundlevalidator

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/healthstatus"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
)

// Stable error codes surfaced on unified_analysis_publications.error_code
// when ValidatePublicationBundle rejects a bundle. Keep these frozen once
// shipped.
const (
	CodeDashboardFactsMissing       = "unified_dashboard_facts_missing"
	CodeDashboardFactsHashMismatch  = "unified_dashboard_facts_hash_mismatch"
	CodeDashboardKPIsMissing        = "unified_dashboard_kpis_missing"
	CodeDashboardTrendMissing       = "unified_dashboard_trend_missing"
	CodeDashboardNarrativeMissing   = "unified_dashboard_narrative_missing"
	CodeDashboardFunnelMissing      = "unified_dashboard_funnel_missing"
	CodeDashboardStockMissing       = "unified_dashboard_stock_missing"
	CodeDashboardAdsMissing         = "unified_dashboard_ads_missing"
	CodeDashboardBundleInconsistent = "unified_dashboard_bundle_inconsistent"
	CodeActionsBundleInconsistent   = "unified_actions_bundle_inconsistent"

	// Completeness codes for the AI-analytical layer (account summary,
	// what_changed, seller pulse narrative, funnel analysis, risk
	// distribution, top actions) and for the projected actions/widgets
	// consistency. Material outcomes (material_actions_found,
	// no_material_actions) describe a completed analysis of a real account
	// state and must carry a fully-populated analytical layer; these codes
	// are only ever emitted for material bundles unless noted otherwise.
	CodeDashboardAccountSummaryMissing = "unified_dashboard_account_summary_missing"
	CodeDashboardHealthScoreMissing    = "unified_dashboard_health_score_missing"
	CodeDashboardWhatChangedMissing    = "unified_dashboard_what_changed_missing"
	CodeDashboardSellerPulseMissing    = "unified_dashboard_seller_pulse_missing"
	// CodeDashboardCategoriesMissing gates on dashboardfacts's
	// category-availability signal (DataCoverage.CategoryCount): the
	// seller's catalog is categorized but the Categories facts block came
	// out empty. This is a defense-in-depth check — dashboardfacts already
	// fails the run earlier (validateDashboardFacts) for the same
	// condition, but this catches facts artifacts persisted before that
	// rule existed.
	CodeDashboardCategoriesMissing       = "unified_dashboard_categories_missing"
	CodeDashboardSKUFocusMissing         = "unified_dashboard_sku_focus_missing"
	CodeDashboardCrossBlockInconsistent  = "unified_dashboard_cross_block_inconsistent"
	CodeDashboardFunnelAnalysisMissing   = "unified_dashboard_funnel_analysis_missing"
	CodeDashboardRiskDistributionMissing = "unified_dashboard_risk_distribution_missing"
	CodeDashboardTopActionsMissing       = "unified_dashboard_top_actions_missing"
	CodeDashboardTopActionsTooMany       = "unified_dashboard_top_actions_too_many"
	CodeDashboardTopActionUnresolved     = "unified_dashboard_top_action_unresolved"
	CodeDashboardTopActionDuplicate      = "unified_dashboard_top_action_duplicate"
	CodeDashboardTopActionArtificialFill = "unified_dashboard_top_actions_artificial_fill"
	// CodeDashboardAdvertisingChecklistMissing/CodeDashboardCategoryAnalysisMissing/
	// CodeDashboardSKUTrendMissing are the last-gate (publish-time) checks
	// for the AI advertising checklist / category analysis / SKU trend
	// completeness rules also enforced earlier by outputvalidator. The
	// backend never invents this content — these codes only ever reject a
	// bundle for omitting AI-authored content the Pack's data justifies.
	CodeDashboardAdvertisingChecklistMissing          = "unified_dashboard_advertising_checklist_missing"
	CodeDashboardAdvertisingHeadlineMissing           = "unified_dashboard_advertising_headline_missing"
	CodeDashboardAdvertisingExplanationMissing        = "unified_dashboard_advertising_explanation_missing"
	CodeDashboardAdvertisingHeadlineTruncated         = "unified_dashboard_advertising_headline_truncated"
	CodeDashboardAdvertisingExplanationFragmented     = "unified_dashboard_advertising_explanation_fragmented"
	CodeDashboardAdvertisingRecommendationMissing     = "unified_dashboard_advertising_recommendation_missing"
	CodeDashboardAdvertisingRecommendationNotGrounded = "unified_dashboard_advertising_recommendation_not_grounded"
	CodeDashboardCategoryAnalysisMissing              = "unified_dashboard_category_analysis_missing"
	CodeDashboardSKUTrendMissing                      = "unified_dashboard_sku_trend_missing"
	// CodeDashboardRiskDriverUnknownID rejects a bundle whose AI output
	// referenced a risk_drivers[].id outside the backend healthpolicy
	// candidate registry (see adapter.ReasonAIRiskDriverUnknownID). The AI
	// may only select/order/explain known registry driver ids — it must
	// never invent new ones, since health_score is computed solely from the
	// registry and an invented id would otherwise silently vanish from
	// display without anyone noticing the AI drifted from the contract.
	CodeDashboardRiskDriverUnknownID  = "unified_dashboard_risk_driver_unknown_id"
	CodeDashboardSKUFocusUnresolved   = "unified_dashboard_sku_focus_unresolved"
	CodeDashboardSKUFocusMissingIssue = "unified_dashboard_sku_focus_missing_issue"
	CodeDashboardKeySKUCohortMismatch = "unified_dashboard_key_sku_cohort_mismatch"
	// CodeActionsMissing is emitted specifically when a material_actions_found
	// bundle has zero actions (an empty-actions signal distinct from the
	// broader consistency mismatch code above).
	CodeActionsMissing                      = "unified_actions_missing"
	CodeActionsCountBelowMinimum            = "actions_count_below_minimum"
	CodeActionsCountAboveMaximum            = "actions_count_above_maximum"
	CodeRequiredActionsCountBelowMinimum    = "required_actions_count_below_minimum"
	CodeActionDetailIncomplete              = "unified_action_detail_incomplete"
	CodeDashboardHealthScoreOutOfRange      = "health_score_out_of_range"
	CodeDashboardHealthScoreDefaultedToZero = "health_score_defaulted_to_zero"
	CodeDashboardHealthScoreFactorsMissing  = "health_score_factors_missing"
	CodeDashboardHealthScoreStatusMismatch  = "health_score_status_mismatch"
	CodeBundlePartialNotPublishable         = "unified_bundle_partial_not_publishable"
	CodeBundleContainsPlaceholder           = "unified_bundle_contains_placeholder"
	// CodeBundleCrossSourceMismatch is reserved for a future identity check
	// across the bundle's constituent sources (facts/output/actions). No
	// check currently emits it.
	CodeBundleCrossSourceMismatch = "unified_bundle_cross_source_mismatch"

	// Account-level DRR consistency between Seller Pulse and Advertising.
	CodeAccountDRRSourceMismatch          = "account_drr_source_mismatch"
	CodeAccountDRRCampaignScopeIncomplete = "account_drr_campaign_scope_incomplete"
	CodeAccountDRRInvalidAggregation      = "account_drr_invalid_aggregation"
	CodeAccountDRRDailySeriesMismatch     = "account_drr_daily_series_mismatch"
	CodeAccountDRRRepeatedAggregateSeries = "account_drr_repeated_aggregate_series"
	CodeAdvertisingAIDRRFactMismatch      = "advertising_ai_drr_fact_mismatch"
)

// placeholderRe flags English filler markers plus common Russian
// non-committal placeholder phrases the prompt explicitly forbids (see
// prompt/analyst_v1.txt): "нет данных", "данные отсутствуют", "нужно/
// требуется провести/дополнительный анализ", "информация отсутствует".
var placeholderRe = regexp.MustCompile(`(?i)(\bTODO\b|\bTBD\b|\bN/?A\b|lorem ipsum|\bplaceholder\b|` +
	`нет данных|данные отсутствуют|информация отсутствует|` +
	`(нужно|требуется)\s+(провести\s+)?дополнительн\S*\s+анализ)`)

// Issue is one validation failure.
type Issue struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Report is the result of ValidatePublicationBundle. A bundle is publishable
// iff Valid is true (Errors is empty).
type Report struct {
	Valid  bool    `json:"valid"`
	Errors []Issue `json:"errors"`
}

// firstErrorCode returns the code of the first recorded error, or "" if the
// report is valid. Publication callers use this as the single
// error_code stamped on a publication_failed row.
func (r Report) FirstErrorCode() string {
	if len(r.Errors) == 0 {
		return ""
	}
	return r.Errors[0].Code
}

// Input bundles everything ValidatePublicationBundle needs to check a
// publication bundle for internal consistency and completeness before it is
// persisted.
type Input struct {
	// FactsPresent is false when the run has no dashboardfacts.DashboardFactsV1
	// artifact at all (e.g. artifacts.LoadDashboardFacts found nothing).
	// A missing-facts bundle is always invalid: there is nothing to
	// validate the projection against, and Dashboard Facts are the
	// authoritative source for every factual field.
	FactsPresent bool
	// FactsIdentityOK is false when the loaded facts do not match the run
	// being published (pack_hash/seller_account_id/metric_date mismatch, or
	// facts_hash verification failure). Never publish facts that do not
	// provably belong to this run's Analysis Pack.
	FactsIdentityOK bool

	Facts      dashboardfacts.DashboardFactsV1
	Projection adapter.DashboardProjection
	Actions    []adapter.ProjectedAction

	// AnalysisOutcome is the validated AI output's analysis_outcome
	// (contract.AnalysisOutcome*). Drives which completeness rules apply.
	AnalysisOutcome string

	// MismatchWarnings are the adapter.MismatchWarning list ProjectDashboard
	// returned alongside Projection. Most reasons are informational only,
	// but adapter.ReasonAIRiskDriverUnknownID is a contract violation (the
	// AI invented a risk driver_id outside the backend healthpolicy
	// registry) and is rejected by checkNoUnknownAIRiskDriverIDs below.
	MismatchWarnings []adapter.MismatchWarning
}

// isMaterialOutcome reports whether outcome represents a completed analysis
// with a real (positive or negative) account state — as opposed to
// insufficient_data/analysis_failed, where the AI analytical layer is
// allowed to stay minimal/absent.
func isMaterialOutcome(outcome string) bool {
	switch outcome {
	case contract.AnalysisOutcomeMaterialActionsFound, contract.AnalysisOutcomeNoMaterialActions:
		return true
	default:
		return false
	}
}

// ValidatePublicationBundle checks a fully-projected publication bundle
// against coverage-aware completeness/consistency rules derived from
// docs/current/dashboard-field-ownership.md. It never mutates its input.
func ValidatePublicationBundle(in Input) Report {
	var issues []Issue

	if !in.FactsPresent {
		return Report{Valid: false, Errors: []Issue{{
			Code:    CodeDashboardFactsMissing,
			Field:   "facts",
			Message: "dashboard facts artifact is missing for this run; cannot publish without backend facts",
		}}}
	}
	if !in.FactsIdentityOK {
		return Report{Valid: false, Errors: []Issue{{
			Code:    CodeDashboardFactsHashMismatch,
			Field:   "facts.facts_hash",
			Message: "dashboard facts identity (pack_hash/seller_account_id/metric_date/facts_hash) does not match the run being published",
		}}}
	}

	coverage := in.Facts.DataCoverage
	frozen := in.Projection.Frozen

	issues = append(issues, checkSellerPulse(coverage, frozen)...)
	issues = append(issues, checkFunnel(coverage, frozen)...)
	issues = append(issues, checkStock(coverage, frozen)...)
	issues = append(issues, checkAds(coverage, frozen)...)
	issues = append(issues, checkSKUFocus(in.Facts, frozen)...)
	issues = append(issues, checkFunnelAnalysis(frozen)...)
	issues = append(issues, checkOutcomeConsistency(in)...)
	issues = append(issues, checkAccountSummaryCompleteness(in)...)
	issues = append(issues, checkHealthScore(in)...)
	issues = append(issues, checkWhatChanged(in)...)
	issues = append(issues, checkSellerPulseNarrativePresent(in)...)
	issues = append(issues, checkRiskDistribution(in)...)
	issues = append(issues, checkTopActions(in)...)
	issues = append(issues, checkActionDetailCompleteness(in.Actions)...)
	issues = append(issues, checkNoPartialWidgets(in)...)
	issues = append(issues, checkPlaceholderText(in)...)
	issues = append(issues, checkAdvertisingChecklist(coverage, frozen)...)
	issues = append(issues, checkAdvertisingNarrativeFields(coverage, frozen)...)
	issues = append(issues, checkCategoriesFacts(coverage, in.Facts)...)
	issues = append(issues, checkCategoryAnalysis(in)...)
	issues = append(issues, checkSKUTrend(coverage, frozen)...)
	issues = append(issues, checkAccountDRRConsistency(in)...)
	issues = append(issues, checkNoUnknownAIRiskDriverIDs(in)...)
	issues = append(issues, checkNoUnresolvedSKUFocus(in)...)
	issues = append(issues, checkKeySKUCohortMembership(in)...)
	issues = append(issues, checkCrossBlockConsistency(in)...)

	return Report{Valid: len(issues) == 0, Errors: issues}
}

// checkSellerPulse: if account metrics coverage is present, require
// revenue+orders KPIs (keys revenue, orders) and a non-empty observed
// Trend7D.
func checkSellerPulse(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.AccountMetricsDays <= 0 {
		return nil
	}
	var issues []Issue
	hasRevenue, hasOrders := false, false
	for _, kpi := range frozen.SellerPulse.KPIs {
		switch kpi.Key {
		case "revenue":
			hasRevenue = true
		case "orders":
			hasOrders = true
		}
	}
	if !hasRevenue || !hasOrders {
		issues = append(issues, Issue{
			Code:    CodeDashboardKPIsMissing,
			Field:   "seller_pulse.kpis",
			Message: "account metrics coverage is present but revenue/orders KPIs are missing from the projected dashboard",
		})
	}
	if len(frozen.SellerPulse.Trend7D) == 0 {
		issues = append(issues, Issue{
			Code:    CodeDashboardTrendMissing,
			Field:   "seller_pulse.trend_7d",
			Message: "account metrics coverage is present but seller_pulse.trend_7d is empty",
		})
	}
	return issues
}

// checkFunnel: if funnel data is available, require funnel stages.
func checkFunnel(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if !coverage.FunnelAvailable {
		return nil
	}
	if len(frozen.SalesFunnel.Stages) == 0 {
		return []Issue{{
			Code:    CodeDashboardFunnelMissing,
			Field:   "sales_funnel.stages",
			Message: "funnel data is available but sales_funnel.stages is empty",
		}}
	}
	return nil
}

// checkStock: if stock snapshot rows are present, require a stock factual
// block with segments, or an explicit not_enough_data/source_not_available
// status (never an invented empty "ok" block).
func checkStock(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.StockSnapshotRows <= 0 {
		return nil
	}
	if len(frozen.StockCoverage.Segments) > 0 {
		return nil
	}
	if isExplicitNoDataStatus(string(frozen.StockCoverage.Status)) {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardStockMissing,
		Field:   "stock_coverage.segments",
		Message: "stock snapshot data is available but stock_coverage has no segments and no explicit not_enough_data status",
	}}
}

// checkAds: if ad metrics days are present, require an ad factual summary
// block.
func checkAds(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.AdMetricsDays <= 0 {
		return nil
	}
	if frozen.AdEfficiency.Summary != nil {
		return nil
	}
	if isExplicitNoDataStatus(string(frozen.AdEfficiency.Status)) {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardAdsMissing,
		Field:   "ad_efficiency.summary",
		Message: "ad metrics data is available but ad_efficiency has no factual summary and no explicit not_enough_data status",
	}}
}

// minAdvertisingChecklistItems mirrors outputvalidator's minimum — kept in
// sync so the last publish-time gate agrees with the earlier contract check.
const minAdvertisingChecklistItems = 2

// checkAdvertisingChecklist: if ad metrics days are present, the AI must
// have emitted at least minAdvertisingChecklistItems concrete advertising
// checklist items on ad_efficiency.narrative.checklist. The backend never
// invents checklist templates — this only enforces presence.
func checkAdvertisingChecklist(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.AdMetricsDays <= 0 {
		return nil
	}
	narrative := frozen.AdEfficiency.Narrative
	if narrative != nil && len(narrative.Checklist) >= minAdvertisingChecklistItems {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardAdvertisingChecklistMissing,
		Field:   "ad_efficiency.narrative.checklist",
		Message: fmt.Sprintf("ad metrics data is available but ad_efficiency.narrative.checklist has fewer than %d items", minAdvertisingChecklistItems),
	}}
}

var advertisingHeadlineTruncatedRe = regexp.MustCompile(`(?i)(?:дrr|дрр)?\s*\d+\.\s*$`)

// checkAdvertisingNarrativeFields requires separate headline + summary when
// ad metrics exist. Rejects decimal-truncated headlines / fragmented summaries.
func checkAdvertisingNarrativeFields(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.AdMetricsDays <= 0 {
		return nil
	}
	narrative := frozen.AdEfficiency.Narrative
	if narrative == nil {
		return []Issue{{
			Code:    CodeDashboardAdvertisingHeadlineMissing,
			Field:   "ad_efficiency.narrative.headline",
			Message: "ad metrics data is available but ad_efficiency.narrative is missing",
		}}
	}
	var issues []Issue
	headline := strings.TrimSpace(narrative.Headline)
	summary := strings.TrimSpace(narrative.Summary)
	recommendation := strings.TrimSpace(narrative.Recommendation)
	if headline == "" {
		issues = append(issues, Issue{
			Code:    CodeDashboardAdvertisingHeadlineMissing,
			Field:   "ad_efficiency.narrative.headline",
			Message: "ad_efficiency.narrative.headline is required (must not be derived by splitting summary)",
		})
	} else if advertisingHeadlineTruncatedRe.MatchString(headline) {
		issues = append(issues, Issue{
			Code:    CodeDashboardAdvertisingHeadlineTruncated,
			Field:   "ad_efficiency.narrative.headline",
			Message: "headline appears truncated on a decimal point",
		})
	}
	if summary == "" {
		issues = append(issues, Issue{
			Code:    CodeDashboardAdvertisingExplanationMissing,
			Field:   "ad_efficiency.narrative.summary",
			Message: "ad_efficiency.narrative.summary (explanation) is required",
		})
	} else {
		s := summary
		if strings.HasPrefix(s, "4%") || strings.HasPrefix(s, "4%)") || strings.HasPrefix(s, "),") ||
			strings.HasPrefix(s, "%") || strings.HasPrefix(s, ")") {
			issues = append(issues, Issue{
				Code:    CodeDashboardAdvertisingExplanationFragmented,
				Field:   "ad_efficiency.narrative.summary",
				Message: "summary starts with a sentence fragment (likely split on decimal point)",
			})
		}
	}
	if recommendation != "" {
		issues = append(issues, Issue{
			Code:    "advertising_recommendation_not_allowed",
			Field:   "ad_efficiency.narrative.recommendation",
			Message: "ad_efficiency.narrative.recommendation is not allowed; actions belong in Actions",
		})
	}
	return issues
}

// checkCategoriesFacts: the seller's catalog is categorized
// (DataCoverage.CategoryCount > 0) but the Categories facts block is
// empty. dashboardfacts.validateDashboardFacts already fails the run for
// this earlier; this is a defense-in-depth check for facts persisted
// before that rule existed.
func checkCategoriesFacts(coverage dashboardfacts.DataCoverage, facts dashboardfacts.DashboardFactsV1) []Issue {
	if coverage.CategoryCount <= 0 || len(facts.Categories) > 0 {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardCategoriesMissing,
		Field:   "categories",
		Message: "seller catalog has categories but dashboard facts categories block is empty",
	}}
}

// checkCategoryAnalysis: when the backend Categories facts block is
// non-empty, the AI must have produced a category_summaries entry per
// category with a non-nil category_score and non-empty headline. The
// backend never invents category scores — this only enforces presence of
// AI-authored analysis when the Pack's category facts justify it.
func checkCategoryAnalysis(in Input) []Issue {
	if len(in.Facts.Categories) == 0 {
		return nil
	}
	summaries := in.Projection.Narrative.CategorySummaries
	if len(summaries) == 0 {
		return []Issue{{
			Code:    CodeDashboardCategoryAnalysisMissing,
			Field:   "category_summaries",
			Message: "category facts are present but category_summaries is empty",
		}}
	}
	var issues []Issue
	for i, c := range summaries {
		if c.CategoryScore == nil || strings.TrimSpace(c.Headline) == "" {
			issues = append(issues, Issue{
				Code:    CodeDashboardCategoryAnalysisMissing,
				Field:   fmt.Sprintf("category_summaries[%d]", i),
				Message: "category_score/headline is required for each category summary when category facts are present",
			})
		}
	}
	return issues
}

// skuTrendMinObservedPoints is the minimum number of HasData sparkline
// points a surfaced SKU focus card must carry once the account has 2+ days
// of SKU metrics.
const skuTrendMinObservedPoints = 2

// checkSKUTrend: once SKU metrics span 2+ days, every surfaced sku_focus
// card must declare an explicit trend_status. Sparse sparklines are allowed
// with not_enough_data / source_not_available — cards are not dropped.
func checkSKUTrend(coverage dashboardfacts.DataCoverage, frozen dashboardhome.FrozenDashboardData) []Issue {
	if coverage.SKUMetricsDays < skuTrendMinObservedPoints {
		return nil
	}
	sf := frozen.SKUFocus
	lists := []struct {
		field string
		cards []dashboardhome.SKUFocusCard
	}{
		{"order_drop", sf.OrderDrop},
		{"stock_risk", sf.StockRisk},
		{"no_sales_with_stock", sf.NoSalesWithStock},
		{"slow_turnover", sf.SlowTurnover},
	}
	var issues []Issue
	for _, l := range lists {
		for i, card := range l.cards {
			status := strings.TrimSpace(card.TrendStatus)
			observed := countObservedSparklinePoints(card.Sparkline7D)
			switch status {
			case dashboardhome.SKUFocusTrendAvailable:
				if observed < skuTrendMinObservedPoints {
					issues = append(issues, Issue{
						Code:    CodeDashboardSKUTrendMissing,
						Field:   fmt.Sprintf("sku_focus.%s[%d].trend_status", l.field, i),
						Message: "trend_status=available requires at least 2 observed sparkline points",
					})
				}
			case dashboardhome.SKUFocusTrendNotEnoughData, dashboardhome.SKUFocusTrendSourceNotAvailable:
				// Explicit sparse / unavailable trend — card may remain.
			case "":
				// Legacy publications without trend_status: require ≥2 points
				// or fail closed so new publishes always stamp the field.
				if observed < skuTrendMinObservedPoints {
					issues = append(issues, Issue{
						Code:    CodeDashboardSKUTrendMissing,
						Field:   fmt.Sprintf("sku_focus.%s[%d].trend_status", l.field, i),
						Message: "sku_focus card missing trend_status with fewer than 2 observed sparkline points",
					})
				}
			default:
				issues = append(issues, Issue{
					Code:    CodeDashboardSKUTrendMissing,
					Field:   fmt.Sprintf("sku_focus.%s[%d].trend_status", l.field, i),
					Message: fmt.Sprintf("unknown trend_status %q", status),
				})
			}
		}
	}
	return issues
}

func countObservedSparklinePoints(points []dashboardhome.SparklinePoint) int {
	n := 0
	for _, p := range points {
		if p.HasData {
			n++
		}
	}
	return n
}

// checkSKUFocus: if the backend SKU facts index has entries, the projected
// sku_focus block must surface at least one of them somewhere (AI selects
// which entities matter; an entirely empty sku_focus while facts have SKUs
// to choose from indicates a dropped/failed projection rather than a
// deliberate "nothing to show" state).
func checkSKUFocus(facts dashboardfacts.DashboardFactsV1, frozen dashboardhome.FrozenDashboardData) []Issue {
	if len(facts.SKUFactsIndex.ByOzonProductID) == 0 {
		return nil
	}
	sf := frozen.SKUFocus
	if len(sf.OrderDrop) > 0 || len(sf.StockRisk) > 0 || len(sf.NoSalesWithStock) > 0 || len(sf.SlowTurnover) > 0 || len(sf.KeySKUs) > 0 {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardSKUFocusMissing,
		Field:   "sku_focus",
		Message: "sku facts index is non-empty but sku_focus has no entries in any list",
	}}
}

// checkCrossBlockConsistency blocks publish when Dashboard surfaces disagree
// on the same canonical backend facts (stock risk counts, SKU Focus top-N,
// key share totals, technical ID leakage).
func checkCrossBlockConsistency(in Input) []Issue {
	var issues []Issue
	frozen := in.Projection.Frozen
	candidates := dashboardfacts.BuildSKUFocusCandidates(in.Facts.SKUFactsIndex)

	if len(frozen.SKUFocus.OrderDrop) > dashboardfacts.SKUFocusDashboardOrderDropLimit {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.order_drop",
			Message: fmt.Sprintf("order_drop has %d cards; dashboard limit is %d", len(frozen.SKUFocus.OrderDrop), dashboardfacts.SKUFocusDashboardOrderDropLimit),
		})
	}
	// stock_risk list includes attention + planning_8_14 for filter navigation.
	// stock_risk_total_count is the all-risk (attention) total used by CTA.
	if frozen.SKUFocus.StockRiskTotalCount > 0 && len(frozen.SKUFocus.StockRisk) < frozen.SKUFocus.StockRiskTotalCount {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.stock_risk",
			Message: fmt.Sprintf("stock_risk has %d cards but stock_risk_total_count=%d", len(frozen.SKUFocus.StockRisk), frozen.SKUFocus.StockRiskTotalCount),
		})
	}
	attentionInList := 0
	planningInList := 0
	for _, card := range frozen.SKUFocus.StockRisk {
		switch card.CoverageBucket {
		case "out_of_stock", "critical_0_3", "shortage_4_7":
			attentionInList++
		case "planning_8_14":
			planningInList++
		}
	}
	if frozen.SKUFocus.StockRiskTotalCount > 0 && attentionInList > 0 && attentionInList != frozen.SKUFocus.StockRiskTotalCount {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.stock_risk",
			Message: fmt.Sprintf("attention cards in stock_risk=%d but stock_risk_total_count=%d", attentionInList, frozen.SKUFocus.StockRiskTotalCount),
		})
	}
	if candidates.StockRiskTotal > 0 && frozen.SKUFocus.StockRiskTotalCount > 0 && frozen.SKUFocus.StockRiskTotalCount != candidates.StockRiskTotal {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.stock_risk_total_count",
			Message: fmt.Sprintf("stock_risk_total_count=%d disagrees with backend candidates=%d", frozen.SKUFocus.StockRiskTotalCount, candidates.StockRiskTotal),
		})
	}
	if len(frozen.SKUFocus.NoSalesWithStock) > dashboardfacts.SKUFocusDashboardNoSalesLimit {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.no_sales_with_stock",
			Message: fmt.Sprintf("no_sales_with_stock has %d cards; dashboard limit is %d", len(frozen.SKUFocus.NoSalesWithStock), dashboardfacts.SKUFocusDashboardNoSalesLimit),
		})
	}
	if len(frozen.SKUFocus.KeySKUs) > dashboardfacts.SKUFocusDashboardKeySKULimit {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.key_skus",
			Message: fmt.Sprintf("key_skus has %d cards; dashboard limit is %d", len(frozen.SKUFocus.KeySKUs), dashboardfacts.SKUFocusDashboardKeySKULimit),
		})
	}

	if candidates.StockRiskTotal > 0 && frozen.SKUFocus.StockRiskTotalCount > 0 &&
		frozen.SKUFocus.StockRiskTotalCount != candidates.StockRiskTotal {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.stock_risk_total_count",
			Message: fmt.Sprintf("stock_risk_total_count %d != candidate total %d", frozen.SKUFocus.StockRiskTotalCount, candidates.StockRiskTotal),
		})
	}
	if candidates.NoSalesTotal > 0 && frozen.SKUFocus.NoSalesWithStockTotalCount > 0 &&
		frozen.SKUFocus.NoSalesWithStockTotalCount != candidates.NoSalesTotal {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.no_sales_with_stock_total_count",
			Message: fmt.Sprintf("no_sales total %d != candidate total %d", frozen.SKUFocus.NoSalesWithStockTotalCount, candidates.NoSalesTotal),
		})
	}

	var oos, critical, short, planning int
	for _, seg := range frozen.StockCoverage.Segments {
		switch seg.Key {
		case "out_of_stock":
			oos = seg.SKUCount
		case "critical_0_3":
			critical = seg.SKUCount
		case "shortage_4_7":
			short = seg.SKUCount
		case "planning_8_14":
			planning = seg.SKUCount
		}
	}
	segmentRisk := oos + critical + short
	if segmentRisk > 0 && candidates.StockRiskTotal > 0 && segmentRisk != candidates.StockRiskTotal {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "stock_coverage.segments",
			Message: fmt.Sprintf("stock risk segments sum %d != sku focus stock-risk universe %d", segmentRisk, candidates.StockRiskTotal),
		})
	}
	if planning > 0 && planningInList > 0 && planning != planningInList {
		issues = append(issues, Issue{
			Code:    CodeDashboardCrossBlockInconsistent,
			Field:   "sku_focus.stock_risk.planning_8_14",
			Message: fmt.Sprintf("planning_8_14 coverage=%d but sku focus list has %d", planning, planningInList),
		})
	}

	forbidden := []string{
		"high_drr_campaign",
		"sales_decline",
		"key_sku_stock_risk",
		"stock_replenishment",
	}
	parts := []string{}
	if frozen.SellerPulse.Narrative != nil {
		parts = append(parts, frozen.SellerPulse.Narrative.Headline, frozen.SellerPulse.Narrative.Summary)
	}
	if frozen.StockCoverage.Narrative != nil {
		parts = append(parts, frozen.StockCoverage.Narrative.Headline, frozen.StockCoverage.Narrative.Summary)
	}
	parts = append(parts,
		frozen.FunnelAnalysis.AIAnalysis.ShortSummary,
		frozen.FunnelAnalysis.AIAnalysis.MainConclusion,
	)
	if frozen.AdEfficiency.Narrative != nil {
		parts = append(parts, frozen.AdEfficiency.Narrative.Headline, frozen.AdEfficiency.Narrative.Summary)
	}
	scan := strings.ToLower(strings.Join(parts, "\n"))
	for _, token := range forbidden {
		if strings.Contains(scan, token) {
			issues = append(issues, Issue{
				Code:    CodeDashboardCrossBlockInconsistent,
				Field:   "user_facing_copy",
				Message: fmt.Sprintf("user-facing copy contains technical token %q", token),
			})
		}
	}

	return issues
}

// checkFunnelAnalysis: if funnel stages are present, the AI funnel analysis
// narrative must carry a short_summary or main_conclusion — stages without
// any AI interpretation indicate a dropped/failed narrative projection.
func checkFunnelAnalysis(frozen dashboardhome.FrozenDashboardData) []Issue {
	if len(frozen.SalesFunnel.Stages) == 0 {
		return nil
	}
	ai := frozen.FunnelAnalysis.AIAnalysis
	if strings.TrimSpace(ai.ShortSummary) != "" || strings.TrimSpace(ai.MainConclusion) != "" {
		return nil
	}
	return []Issue{{
		Code:    CodeDashboardFunnelAnalysisMissing,
		Field:   "funnel_analysis.ai_analysis",
		Message: "funnel stages are present but funnel_analysis AI short_summary/main_conclusion is empty",
	}}
}

// checkAccountSummaryCompleteness: material outcomes require a non-empty
// improvement_summary in addition to the headline/executive_summary already
// enforced by checkOutcomeConsistency.
func checkAccountSummaryCompleteness(in Input) []Issue {
	if !isMaterialOutcome(in.AnalysisOutcome) {
		return nil
	}
	if strings.TrimSpace(in.Projection.Narrative.ImprovementSummary) == "" {
		return []Issue{{
			Code:    CodeDashboardAccountSummaryMissing,
			Field:   "account_summary.improvement_summary",
			Message: "material outcome requires a non-empty account_summary.improvement_summary",
		}}
	}
	return nil
}

// checkHealthScore: material outcomes require a non-nil health_score within
// the valid 0..100 range, with factors that explain the score when material
// actions were found.
func checkHealthScore(in Input) []Issue {
	if !isMaterialOutcome(in.AnalysisOutcome) {
		return nil
	}
	score := in.Projection.HealthScore
	if score == nil {
		return []Issue{{
			Code:    CodeDashboardHealthScoreMissing,
			Field:   "account_summary.health_score",
			Message: "material outcome requires a non-nil health_score",
		}}
	}
	if *score < 0 || *score > 100 {
		return []Issue{{
			Code:    CodeDashboardHealthScoreOutOfRange,
			Field:   "account_summary.health_score",
			Message: fmt.Sprintf("health_score %d is out of the valid 0..100 range", *score),
		}}
	}
	var issues []Issue
	ah := in.Projection.Narrative.AccountHealth
	if in.AnalysisOutcome == contract.AnalysisOutcomeMaterialActionsFound {
		if ah == nil || len(ah.RiskDrivers) == 0 {
			issues = append(issues, Issue{
				Code:    CodeDashboardHealthScoreFactorsMissing,
				Field:   "account_summary.risk_drivers",
				Message: "material_actions_found requires risk_drivers that explain health_score",
			})
		}
	}
	if *score == 0 {
		hasCriticalHigh := false
		if ah != nil {
			for _, d := range ah.RiskDrivers {
				switch strings.ToLower(strings.TrimSpace(d.Severity)) {
				case "critical", "high":
					hasCriticalHigh = true
				}
			}
		}
		if !hasCriticalHigh {
			issues = append(issues, Issue{
				Code:    CodeDashboardHealthScoreDefaultedToZero,
				Field:   "account_summary.health_score",
				Message: "health_score 0 requires critical/high risk_drivers",
			})
		}
	}
	if ah != nil {
		if sev := strings.TrimSpace(ah.Severity); sev != "" {
			want := healthstatus.FromScore(score).Severity
			got := healthstatus.NormalizeSeverity(sev)
			if want != "" && got != want {
				issues = append(issues, Issue{
					Code:    CodeDashboardHealthScoreStatusMismatch,
					Field:   "account_summary.severity",
					Message: fmt.Sprintf("severity %q does not match health_score %d band (want %q)", sev, *score, want),
				})
			}
		}
	}
	return issues
}

// requiredWhatChangedKeys is the home-compatible what_changed contract.
// Thematic keys (revenue/ads/sku_*) are not publishable: home apply rejects
// them after positional remap. See docs/current/dashboard-completeness-contract.md.
var requiredWhatChangedKeys = []string{"worsened", "new_risks", "unchanged_risks", "improved"}

// checkWhatChanged: material outcomes require a what_changed narrative with
// a summary/main_conclusion, the four home-compatible group keys in order,
// and at least one group carrying at least one item.
func checkWhatChanged(in Input) []Issue {
	if !isMaterialOutcome(in.AnalysisOutcome) {
		return nil
	}
	wc := in.Projection.Narrative.WhatChanged
	if wc == nil {
		return []Issue{{
			Code:    CodeDashboardWhatChangedMissing,
			Field:   "what_changed",
			Message: "material outcome requires a non-nil what_changed narrative",
		}}
	}
	hasSummary := strings.TrimSpace(wc.Summary) != "" || strings.TrimSpace(wc.MainConclusion) != ""
	if !hasSummary {
		return []Issue{{
			Code:    CodeDashboardWhatChangedMissing,
			Field:   "what_changed",
			Message: "material outcome requires what_changed summary/main_conclusion",
		}}
	}
	if len(wc.Groups) != len(requiredWhatChangedKeys) {
		return []Issue{{
			Code:    CodeDashboardWhatChangedMissing,
			Field:   "what_changed.groups",
			Message: fmt.Sprintf("what_changed must have exactly %d home-compatible groups (%v), got %d", len(requiredWhatChangedKeys), requiredWhatChangedKeys, len(wc.Groups)),
		}}
	}
	hasGroupItem := false
	var issues []Issue
	for i, want := range requiredWhatChangedKeys {
		got := strings.TrimSpace(wc.Groups[i].Key)
		if got != want {
			issues = append(issues, Issue{
				Code:    CodeDashboardWhatChangedMissing,
				Field:   fmt.Sprintf("what_changed.groups[%d].key", i),
				Message: fmt.Sprintf("got %q want %q (thematic keys are not publishable)", got, want),
			})
		}
		if len(wc.Groups[i].Items) > 0 {
			hasGroupItem = true
			if strings.TrimSpace(wc.Groups[i].Summary) == "" {
				issues = append(issues, Issue{
					Code:    CodeDashboardWhatChangedMissing,
					Field:   fmt.Sprintf("what_changed.groups[%d].summary", i),
					Message: "group with items requires a non-empty summary (home apply rejects missing_summary_when_items)",
				})
			}
		}
	}
	if !hasGroupItem {
		issues = append(issues, Issue{
			Code:    CodeDashboardWhatChangedMissing,
			Field:   "what_changed.groups",
			Message: "material outcome requires at least one what_changed group with an item",
		})
	}
	return issues
}

// checkSellerPulseNarrativePresent: material outcomes require a resolved
// seller_pulse narrative with a non-empty headline or summary.
func checkSellerPulseNarrativePresent(in Input) []Issue {
	if !isMaterialOutcome(in.AnalysisOutcome) {
		return nil
	}
	narrative := in.Projection.Narrative.SellerPulseNarrative
	if narrative == nil || (strings.TrimSpace(narrative.Headline) == "" && strings.TrimSpace(narrative.Summary) == "") {
		return []Issue{{
			Code:    CodeDashboardSellerPulseMissing,
			Field:   "seller_pulse.narrative",
			Message: "material outcome requires a resolved seller_pulse narrative headline/summary",
		}}
	}
	return nil
}

// checkRiskDistribution: material_actions_found requires a non-empty
// risk_distribution.groups — a decision to surface material actions implies
// at least one identified risk group behind it. no_material_actions is
// intentionally excluded: a healthy account can legitimately have zero risk
// groups.
func checkRiskDistribution(in Input) []Issue {
	if in.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return nil
	}
	if len(in.Projection.Frozen.RiskDistribution.Groups) == 0 {
		return []Issue{{
			Code:    CodeDashboardRiskDistributionMissing,
			Field:   "risk_distribution.groups",
			Message: "material_actions_found requires non-empty risk_distribution.groups",
		}}
	}
	return nil
}

// checkTopActions: material_actions_found requires at least one top action
// surfaced in the narrative. Also enforces max 3, every DriverID resolves to
// a projected action, no duplicate IDs, and no artificial padding beyond the
// available required-tier action set.
func checkTopActions(in Input) []Issue {
	if in.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return nil
	}
	tops := in.Projection.Narrative.TopActions
	if len(tops) == 0 {
		return []Issue{{
			Code:    CodeDashboardTopActionsMissing,
			Field:   "top_actions",
			Message: "material_actions_found requires at least one top action in the narrative",
		}}
	}
	var issues []Issue
	maxTop := contract.MaxTopActionsV1
	if maxTop <= 0 {
		maxTop = 3
	}
	if len(tops) > maxTop {
		issues = append(issues, Issue{
			Code:    CodeDashboardTopActionsTooMany,
			Field:   "top_actions",
			Message: fmt.Sprintf("top_actions=%d max=%d", len(tops), maxTop),
		})
	}

	actionIDs := make(map[string]struct{}, len(in.Actions))
	requiredCount := 0
	for _, a := range in.Actions {
		id := strings.TrimSpace(a.AnalysisActionID)
		if id == "" {
			continue
		}
		actionIDs[id] = struct{}{}
		if strings.EqualFold(strings.TrimSpace(a.ExecutionTier), contract.ExecutionTierRequired) {
			requiredCount++
		}
	}

	seen := make(map[string]struct{}, len(tops))
	for i, top := range tops {
		id := strings.TrimSpace(top.DriverID)
		field := fmt.Sprintf("top_actions[%d]", i)
		if id == "" {
			issues = append(issues, Issue{
				Code:    CodeDashboardTopActionUnresolved,
				Field:   field,
				Message: "top action driver_id is empty",
			})
			continue
		}
		if _, ok := actionIDs[id]; !ok {
			issues = append(issues, Issue{
				Code:    CodeDashboardTopActionUnresolved,
				Field:   field,
				Message: fmt.Sprintf("top action %q does not resolve to a projected action", id),
			})
		}
		if _, dup := seen[id]; dup {
			issues = append(issues, Issue{
				Code:    CodeDashboardTopActionDuplicate,
				Field:   field,
				Message: fmt.Sprintf("duplicate top action id %q", id),
			})
		} else {
			seen[id] = struct{}{}
		}
	}

	// Artificial fill: more Top Actions than required-tier actions available
	// in this run (publication must not pad to 3 when fewer required exist).
	if requiredCount > 0 && len(tops) > requiredCount {
		issues = append(issues, Issue{
			Code:    CodeDashboardTopActionArtificialFill,
			Field:   "top_actions",
			Message: fmt.Sprintf("top_actions=%d exceeds required actions=%d (no artificial fill)", len(tops), requiredCount),
		})
	}
	return issues
}

// checkActionDetailCompleteness: every projected action must carry at least
// one step and a non-empty completion_criteria.
func checkActionDetailCompleteness(projActions []adapter.ProjectedAction) []Issue {
	var issues []Issue
	for _, a := range projActions {
		if a.StepCount == 0 || strings.TrimSpace(a.CompletionCriteria) == "" {
			issues = append(issues, Issue{
				Code:    CodeActionDetailIncomplete,
				Field:   fmt.Sprintf("actions[%s]", a.AnalysisActionID),
				Message: "action is missing steps or completion_criteria",
			})
		}
	}
	return issues
}

// checkNoPartialWidgets: material outcomes must never publish a widget
// (stock/ads/funnel/funnel_analysis/seller_pulse narrative) whose status is
// "partial" — a partial widget is an in-progress state, not a publishable
// factual/narrative block.
func checkNoPartialWidgets(in Input) []Issue {
	if !isMaterialOutcome(in.AnalysisOutcome) {
		return nil
	}
	frozen := in.Projection.Frozen
	var issues []Issue
	partialWidgets := []struct {
		field  string
		status dashboardhome.WidgetStatus
	}{
		{"stock_coverage.status", frozen.StockCoverage.Status},
		{"ad_efficiency.status", frozen.AdEfficiency.Status},
		{"sales_funnel.status", frozen.SalesFunnel.Status},
		{"funnel_analysis.status", frozen.FunnelAnalysis.Status},
	}
	for _, w := range partialWidgets {
		if w.status == dashboardhome.WidgetPartial {
			issues = append(issues, Issue{
				Code:    CodeBundlePartialNotPublishable,
				Field:   w.field,
				Message: fmt.Sprintf("%s is partial; material outcome must not publish a partial widget", w.field),
			})
		}
	}
	if narrative := in.Projection.Narrative.SellerPulseNarrative; narrative != nil &&
		strings.EqualFold(strings.TrimSpace(narrative.Status), string(dashboardhome.WidgetPartial)) {
		issues = append(issues, Issue{
			Code:    CodeBundlePartialNotPublishable,
			Field:   "seller_pulse.narrative.status",
			Message: "seller_pulse.narrative.status is partial; material outcome must not publish a partial widget",
		})
	}
	return issues
}

// checkPlaceholderText flags obvious placeholder markers (TODO/TBD/N-A/lorem
// ipsum/placeholder) leaking into the seller-facing headline/executive
// summary.
func checkPlaceholderText(in Input) []Issue {
	var issues []Issue
	texts := []struct {
		field string
		text  string
	}{
		{"account_summary.headline", in.Projection.Narrative.Headline},
		{"account_summary.executive_summary", in.Projection.Narrative.ExecutiveSummary},
	}
	for _, t := range texts {
		if placeholderRe.MatchString(t.text) {
			issues = append(issues, Issue{
				Code:    CodeBundleContainsPlaceholder,
				Field:   t.field,
				Message: "text contains a placeholder marker",
			})
		}
	}
	return issues
}

// checkNoUnknownAIRiskDriverIDs rejects a bundle whose AI output invented a
// risk_drivers[].id outside the backend healthpolicy candidate registry.
// health_score is always computed from the full backend registry (Wave 2)
// — the AI may only pick which known driver ids to explain and in what
// order, never add its own. ProjectDashboard already drops unknown ids from
// the published projection and reports them as
// adapter.ReasonAIRiskDriverUnknownID MismatchWarnings; this is the
// publish-time gate that turns that signal into a hard rejection instead of
// a silently-swallowed warning.
func checkNoUnknownAIRiskDriverIDs(in Input) []Issue {
	var issues []Issue
	for _, w := range in.MismatchWarnings {
		if w.Reason != adapter.ReasonAIRiskDriverUnknownID {
			continue
		}
		issues = append(issues, Issue{
			Code:    CodeDashboardRiskDriverUnknownID,
			Field:   w.Field,
			Message: fmt.Sprintf("AI risk_drivers referenced unknown driver_id %q outside the backend healthpolicy registry", w.AIValue),
		})
	}
	return issues
}

// checkNoUnresolvedSKUFocus rejects silent drops of AI-selected sku_focus
// entities that do not resolve against Pack/facts, and cards without a
// confirmable issue statement.
func checkNoUnresolvedSKUFocus(in Input) []Issue {
	var issues []Issue
	for _, w := range in.MismatchWarnings {
		switch w.Reason {
		case adapter.ReasonSKUFocusUnresolvedEntity:
			issues = append(issues, Issue{
				Code:    CodeDashboardSKUFocusUnresolved,
				Field:   w.Field,
				Message: fmt.Sprintf("sku_focus entity %q did not resolve against Pack/facts; publication blocked", w.AIValue),
			})
		case adapter.ReasonSKUFocusMissingIssue:
			issues = append(issues, Issue{
				Code:    CodeDashboardSKUFocusMissingIssue,
				Field:   w.Field,
				Message: fmt.Sprintf("sku_focus entity %q has no problem/key_metric; publication blocked", w.AIValue),
			})
		}
	}
	return issues
}

// checkKeySKUCohortMembership requires published KeySKUs to be a prefix of the
// backend revenue cohort (same order). Dashboard may show top-N while
// key_skus_total_count / store totals remain cohort-owned.
func checkKeySKUCohortMembership(in Input) []Issue {
	cohort := in.Facts.KeySKUCohort
	published := in.Projection.Frozen.SKUFocus.KeySKUs
	if cohort.Status != "available" && len(cohort.Items) == 0 {
		if len(published) == 0 {
			return nil
		}
		return []Issue{{
			Code:    CodeDashboardKeySKUCohortMismatch,
			Field:   "sku_focus.key_skus",
			Message: "published key_skus present but backend cohort is empty/not_enough_data",
		}}
	}
	var issues []Issue
	if len(published) > len(cohort.Items) {
		issues = append(issues, Issue{
			Code:    CodeDashboardKeySKUCohortMismatch,
			Field:   "sku_focus.key_skus",
			Message: fmt.Sprintf("published key_skus count %d exceeds backend cohort %d", len(published), len(cohort.Items)),
		})
		return issues
	}
	if in.Projection.Frozen.SKUFocus.KeySKUsTotalCount > 0 &&
		in.Projection.Frozen.SKUFocus.KeySKUsTotalCount != len(cohort.Items) {
		issues = append(issues, Issue{
			Code:    CodeDashboardKeySKUCohortMismatch,
			Field:   "sku_focus.key_skus_total_count",
			Message: fmt.Sprintf("key_skus_total_count %d != backend cohort %d", in.Projection.Frozen.SKUFocus.KeySKUsTotalCount, len(cohort.Items)),
		})
	}
	seen := make(map[int64]struct{}, len(published))
	for i, card := range published {
		item := cohort.Items[i]
		if card.OzonProductID != item.OzonProductID {
			issues = append(issues, Issue{
				Code:    CodeDashboardKeySKUCohortMismatch,
				Field:   fmt.Sprintf("sku_focus.key_skus[%d]", i),
				Message: fmt.Sprintf("ozon_product_id %d != cohort %d at rank %d", card.OzonProductID, item.OzonProductID, item.Rank),
			})
		}
		if _, dup := seen[card.OzonProductID]; dup {
			issues = append(issues, Issue{
				Code:    CodeDashboardKeySKUCohortMismatch,
				Field:   fmt.Sprintf("sku_focus.key_skus[%d]", i),
				Message: fmt.Sprintf("duplicate ozon_product_id %d in published key_skus", card.OzonProductID),
			})
		}
		seen[card.OzonProductID] = struct{}{}
		if math.Abs(card.RevenueSharePct-item.RevenueSharePct) > 0.15 {
			issues = append(issues, Issue{
				Code:    CodeDashboardKeySKUCohortMismatch,
				Field:   fmt.Sprintf("sku_focus.key_skus[%d].revenue_share_pct", i),
				Message: fmt.Sprintf("revenue_share_pct %.2f diverges from cohort %.2f", card.RevenueSharePct, item.RevenueSharePct),
			})
		}
	}
	storeDenom := in.Projection.Frozen.SKUFocus.StoreTotalRevenue7d
	if math.Abs(storeDenom-cohort.StoreTotalRevenue7d) > 0.01 {
		issues = append(issues, Issue{
			Code:    CodeDashboardKeySKUCohortMismatch,
			Field:   "sku_focus.store_total_revenue_7d",
			Message: fmt.Sprintf("store denominator %.2f != cohort %.2f", storeDenom, cohort.StoreTotalRevenue7d),
		})
	}
	return issues
}

func isExplicitNoDataStatus(status string) bool {
	switch dashboardhome.WidgetStatus(strings.TrimSpace(status)) {
	case dashboardhome.WidgetNotEnoughData, dashboardhome.WidgetSourceNotAvailable, dashboardhome.WidgetSourceAccessDenied, dashboardhome.WidgetSourcePending:
		return true
	default:
		return false
	}
}

// checkOutcomeConsistency enforces the analysis_outcome-specific rules:
//   - material_actions_found: require headline, executive summary, and at
//     least 1 action.
//   - no_material_actions: require headline+summary, and exactly 0 actions
//     (a non-empty action list would contradict the declared outcome).
//   - insufficient_data: allow fewer populated blocks, but require an
//     explicit status/limitations trail (no invented zeros) — enforced by
//     checkSellerPulse/checkFunnel/checkStock/checkAds already requiring
//     either data or an explicit no-data status.
func checkOutcomeConsistency(in Input) []Issue {
	headline := strings.TrimSpace(in.Projection.Narrative.Headline)
	summary := strings.TrimSpace(in.Projection.Narrative.ExecutiveSummary)
	actionCount := len(in.Actions)

	switch in.AnalysisOutcome {
	case contract.AnalysisOutcomeMaterialActionsFound:
		var issues []Issue
		if headline == "" || summary == "" {
			issues = append(issues, Issue{
				Code:    CodeDashboardNarrativeMissing,
				Field:   "account_summary",
				Message: "analysis_outcome=material_actions_found requires a non-empty headline and executive_summary",
			})
		}
		if actionCount == 0 {
			issues = append(issues, Issue{
				Code:    CodeActionsMissing,
				Field:   "actions",
				Message: "analysis_outcome=material_actions_found requires at least 1 action, got 0",
			})
		}
		if actionCount > 0 && actionCount < contract.MinActionsMaterialV1 {
			issues = append(issues, Issue{
				Code:    CodeActionsCountBelowMinimum,
				Field:   "actions",
				Message: fmt.Sprintf("material_actions_found requires at least %d actions, got %d", contract.MinActionsMaterialV1, actionCount),
			})
		}
		if actionCount > contract.MaxActionsV1 {
			issues = append(issues, Issue{
				Code:    CodeActionsCountAboveMaximum,
				Field:   "actions",
				Message: fmt.Sprintf("material_actions_found allows at most %d actions, got %d", contract.MaxActionsV1, actionCount),
			})
		}
		requiredCount := 0
		for _, a := range in.Actions {
			if strings.TrimSpace(a.ExecutionTier) == contract.ExecutionTierRequired {
				requiredCount++
			}
		}
		if actionCount > 0 && requiredCount < contract.MinRequiredActionsMaterialV1 {
			issues = append(issues, Issue{
				Code:  CodeRequiredActionsCountBelowMinimum,
				Field: "actions",
				Message: fmt.Sprintf("material_actions_found requires at least %d execution_tier=required actions, got %d",
					contract.MinRequiredActionsMaterialV1, requiredCount),
			})
		}
		return issues

	case contract.AnalysisOutcomeNoMaterialActions:
		var issues []Issue
		if headline == "" || summary == "" {
			issues = append(issues, Issue{
				Code:    CodeDashboardNarrativeMissing,
				Field:   "account_summary",
				Message: "analysis_outcome=no_material_actions requires a non-empty headline and executive_summary",
			})
		}
		if actionCount != 0 {
			issues = append(issues, Issue{
				Code:    CodeActionsBundleInconsistent,
				Field:   "actions",
				Message: "analysis_outcome=no_material_actions requires 0 actions",
			})
		}
		return issues

	case contract.AnalysisOutcomeInsufficientData:
		// Fewer populated blocks are expected/allowed; the per-block checks
		// above already require an explicit no-data status rather than
		// invented zeros wherever coverage exists.
		return nil

	default:
		return []Issue{{
			Code:    CodeDashboardBundleInconsistent,
			Field:   "analysis_outcome",
			Message: "unrecognized analysis_outcome value: " + in.AnalysisOutcome,
		}}
	}
}
