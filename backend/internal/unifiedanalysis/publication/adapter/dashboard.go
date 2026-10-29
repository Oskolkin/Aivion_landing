package adapter

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/healthpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/weeklychangepolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/healthstatus"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// NarrativePayload mirrors the (unexported) dashboardhome dashboardNarrativePayload
// JSON shape so it can be marshaled directly into ai_dashboard_narratives.validated_payload
// without importing an unexported type. Field names/JSON tags MUST stay in
// sync with backend/internal/dashboardhome/narrative_types.go.
type NarrativePayload struct {
	Headline               string                                `json:"headline"`
	ExecutiveSummary       string                                `json:"executive_summary"`
	HealthReasons          []NarrativeHealthReason               `json:"health_reasons"`
	ImprovementSummary     string                                `json:"improvement_summary"`
	TopActions             []NarrativeTopAction                  `json:"top_actions"`
	RecommendedActions     []dashboardhome.ActionItem            `json:"recommended_actions,omitempty"`
	WhatChanged            *NarrativeWhatChanged                 `json:"what_changed,omitempty"`
	DataQualityNotes       []string                              `json:"data_quality_notes"`
	StockCoverageNarrative *dashboardhome.StockCoverageNarrative `json:"stock_coverage_narrative,omitempty"`
	AdEfficiencyNarrative  *dashboardhome.AdEfficiencyNarrative  `json:"ad_efficiency_narrative,omitempty"`
	SellerPulseNarrative   *dashboardhome.SellerPulseNarrative   `json:"seller_pulse_narrative,omitempty"`
	FunnelAIAnalysis       *dashboardhome.FunnelAIAnalysisWidget `json:"funnel_ai_analysis,omitempty"`
	CategorySummaries      []dashboardhome.CategorySummaryCard   `json:"category_summaries,omitempty"`
	RiskOverview           *dashboardhome.RiskDistributionWidget `json:"risk_overview,omitempty"`
	AccountHealth          *NarrativeAccountHealth               `json:"account_health,omitempty"`
}

// NarrativeHealthReason mirrors dashboardNarrativeHealthReason.
type NarrativeHealthReason struct {
	DriverID string   `json:"driver_id"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Reason   string   `json:"reason"`
	FactIDs  []string `json:"fact_ids,omitempty"`
}

// NarrativeTopAction mirrors dashboardNarrativeTopAction.
type NarrativeTopAction struct {
	DriverID             string   `json:"driver_id"`
	SourceCandidateID    string   `json:"source_candidate_id"`
	Priority             string   `json:"priority"`
	Category             string   `json:"category"`
	Title                string   `json:"title"`
	Reason               string   `json:"reason"`
	NextStep             string   `json:"next_step"`
	ExpectedEffect       string   `json:"expected_effect"`
	SupportingMetricRefs []string `json:"supporting_metric_refs"`
	FactIDs              []string `json:"fact_ids,omitempty"`
}

// NarrativeWhatChanged mirrors dashboardNarrativeWhatChanged.
type NarrativeWhatChanged struct {
	Summary        string                      `json:"summary"`
	MainConclusion string                      `json:"main_conclusion,omitempty"`
	Groups         []NarrativeWhatChangedGroup `json:"groups"`
}

// NarrativeWhatChangedGroup mirrors dashboardNarrativeWhatChangedGroup.
type NarrativeWhatChangedGroup struct {
	Key       string                     `json:"key"`
	Label     string                     `json:"label"`
	Tone      string                     `json:"tone"`
	Summary   string                     `json:"summary"`
	Items     []NarrativeWhatChangedItem `json:"items"`
	EmptyText string                     `json:"empty_text"`
}

// NarrativeWhatChangedItem mirrors dashboardNarrativeWhatChangedItem.
type NarrativeWhatChangedItem struct {
	Label             string   `json:"label"`
	Text              string   `json:"text"`
	Highlights        []string `json:"highlights"`
	Domain            string   `json:"domain,omitempty"`
	Severity          string   `json:"severity,omitempty"`
	CurrentValueText  string   `json:"current_value_text,omitempty"`
	PreviousValueText string   `json:"previous_value_text,omitempty"`
	DeltaText         string   `json:"delta_text,omitempty"`
	ImpactText        string   `json:"impact_text,omitempty"`
	EntityCount       int      `json:"entity_count,omitempty"`
	EntityNames       []string `json:"entity_names,omitempty"`
	LinkedActionID    string   `json:"linked_action_id,omitempty"`
	ActionLinkLabel   string   `json:"action_link_label,omitempty"`
	DeepLink          string   `json:"deep_link,omitempty"`
	ChangeID          string   `json:"change_id,omitempty"`
	EvidenceFactIDs   []string `json:"evidence_fact_ids,omitempty"`
}

// NarrativeAccountHealth mirrors dashboardAccountHealthBlock.
type NarrativeAccountHealth struct {
	HealthScore          *int32                              `json:"health_score,omitempty"`
	Severity             string                              `json:"severity"`
	HealthStatusCode     string                              `json:"health_status_code,omitempty"`
	HealthStatusLabel    string                              `json:"health_status_label,omitempty"`
	HealthStatusExplain  string                              `json:"health_status_explanation,omitempty"`
	Headline             string                              `json:"headline,omitempty"`
	ExecutiveSummary     string                              `json:"executive_summary,omitempty"`
	ImprovementSummary   string                              `json:"improvement_summary,omitempty"`
	ImprovementLevers    []dashboardhome.ImprovementLever    `json:"improvement_levers,omitempty"`
	RiskDrivers          []NarrativeRiskDriverView           `json:"risk_drivers"`
	HealthScoreBreakdown *dashboardhome.HealthScoreBreakdown `json:"health_score_breakdown,omitempty"`
	// HealthPolicyVersion/HealthFactsHash pin health_score to the exact
	// backend candidate-registry policy and Dashboard Facts snapshot it was
	// computed from (see healthpolicy.HealthPolicyVersion /
	// dashboardfacts.DashboardFactsV1.FactsHash). Wave 2: health_score is
	// always grounded in the full backend registry, never in the AI's
	// risk_drivers subset — these two fields make that provenance auditable.
	HealthPolicyVersion string `json:"health_policy_version,omitempty"`
	HealthFactsHash     string `json:"health_facts_hash,omitempty"`
}

// NarrativeRiskDriverView mirrors dashboardNarrativeRiskDriverView.
type NarrativeRiskDriverView struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	Severity      string `json:"severity"`
	ImpactScore   int    `json:"impact_score"`
	Title         string `json:"title"`
	Reason        string `json:"reason"`
}

// DashboardProjection is the combined output of ProjectDashboard: the frozen
// factual snapshot payload plus the narrative payload persisted on
// ai_dashboard_narratives.validated_payload.
type DashboardProjection struct {
	Frozen    dashboardhome.FrozenDashboardData
	Narrative NarrativePayload
	// HealthScore/Severity are extracted for convenience (used to populate
	// ai_dashboard_narratives.health_score/severity columns).
	HealthScore *int32
	Severity    string
}

// MismatchWarning records a factual mismatch between an AI-reported value
// and the corresponding backend Dashboard Fact for a merged field (e.g. a
// seller_pulse KPI where the AI output and the facts disagree on the
// number). Backend facts always win in ProjectDashboard's merge — most of
// these warnings are informational only, for QA/monitoring, and never block
// or alter publication. The exception is ReasonAIRiskDriverUnknownID: an AI
// output that invents a risk driver_id outside the healthpolicy registry is
// a contract violation, and bundlevalidator rejects the bundle for it (see
// CodeDashboardRiskDriverUnknownID).
type MismatchWarning struct {
	Field     string `json:"field"`
	Reason    string `json:"reason"`
	AIValue   string `json:"ai_value,omitempty"`
	FactValue string `json:"fact_value,omitempty"`
}

// ReasonAIRiskDriverUnknownID is the MismatchWarning.Reason emitted when the
// AI output references a risk_drivers[].id that is not part of the backend
// healthpolicy.KnownDriverIDs registry. The AI may only select/order/explain
// known registry driver ids — it must never invent new ones. bundlevalidator
// treats this reason as a publish-blocking error.
const ReasonAIRiskDriverUnknownID = "ai_risk_driver_unknown_id"

// ReasonSKUFocusUnresolvedEntity is emitted when an AI-selected sku_focus
// card cannot be resolved against Pack/facts. Bundle validation treats this
// as publication_blocked (no silent drop).
const ReasonSKUFocusUnresolvedEntity = "sku_focus_unresolved_entity"

// ReasonSKUFocusMissingIssue is emitted when identity resolves but the AI
// card carries neither problem nor key_metric (issue cannot be confirmed).
const ReasonSKUFocusMissingIssue = "sku_focus_missing_issue"

// ProjectDashboard merges backend Dashboard Facts (dashboardfacts.DashboardFactsV1)
// with the validated AI UnifiedAnalysisOutputV1 into the frozen dashboard
// snapshot + narrative payload shapes expected by the existing dashboard
// persistence/render pipeline.
//
// Merge policy (see docs/current/dashboard-field-ownership.md for the full
// field-ownership table):
//   - Backend facts ALWAYS win for: DashboardMetrics, SellerPulse
//     KPIs/Trend7D, StockCoverage segments/totals/status, AdEfficiency
//     factual summary/points/campaigns, SalesFunnel stages/status. AI values
//     for these fields are never used, even when facts are empty (no
//     fallback to AI numbers).
//   - AI wins for: narratives, account_summary texts, funnel AI analysis,
//     risk drivers, what_changed, category summaries, action-derived top
//     actions.
//   - SKUFocus: AI selects which entities to surface for order_drop/stock_risk/
//     no_sales/slow_turnover (and supplies problem copy); factual fields are
//     joined from facts.SKUFactsIndex. KeySKUs membership is backend-owned via
//     facts.KeySKUCohort (80% revenue cohort); AI may only overlay copy for
//     cohort members matched by sku id. Unresolvable AI focus SKUs emit
//     ReasonSKUFocusUnresolvedEntity (publication-blocking).
func ProjectDashboard(facts dashboardfacts.DashboardFactsV1, out *contract.UnifiedAnalysisOutputV1, sellerAccountID int64, metricDate string) (DashboardProjection, []MismatchWarning) {
	return ProjectDashboardWithPrevious(facts, nil, out, sellerAccountID, metricDate)
}

// ProjectDashboardWithPrevious is ProjectDashboard plus an optional previous
// Dashboard Facts snapshot used by weeklychangepolicy to classify
// new_risk / active_problem / worsened continuity.
func ProjectDashboardWithPrevious(
	facts dashboardfacts.DashboardFactsV1,
	previous *dashboardfacts.DashboardFactsV1,
	out *contract.UnifiedAnalysisOutputV1,
	sellerAccountID int64,
	metricDate string,
) (DashboardProjection, []MismatchWarning) {
	frozen := frozenFromFacts(facts, sellerAccountID, metricDate)

	if out == nil {
		return DashboardProjection{Frozen: frozen}, nil
	}
	dash := out.Dashboard

	warnings := detectKPIMismatches(dash.SellerPulse.KPIs, facts.SellerPulse.KPIs)

	skuLookup := buildSKUFactsLookup(facts.SKUFactsIndex)
	candidates := dashboardfacts.BuildSKUFocusCandidates(facts.SKUFactsIndex)
	var skuFocusWarnings []MismatchWarning
	frozen.SKUFocus, skuFocusWarnings = projectSKUFocus(dash.SKUFocus, skuLookup, facts.KeySKUCohort, candidates)
	warnings = append(warnings, skuFocusWarnings...)

	// AI narrative overlays onto the backend-owned factual widgets built by
	// frozenFromFacts above; AI never overwrites the factual fields already
	// set there (segments/totals/status/kpis/trend/stages/summary/points).
	frozen.SellerPulse.Narrative = projectSellerPulseNarrative(dash.SellerPulse.Narrative)
	frozen.StockCoverage.Narrative = projectStockCoverageNarrative(dash.StockCoverage.Narrative)
	frozen.AdEfficiency.Narrative = projectAdEfficiencyNarrative(dash.AdEfficiency.Narrative)
	frozen.FunnelAnalysis = projectFunnelAnalysis(dash.FunnelAnalysis)
	frozen.RiskDistribution = projectRiskDistribution(dash.RiskDistribution)

	narrative := NarrativePayload{
		Headline:               dash.AccountSummary.Headline,
		ExecutiveSummary:       dash.AccountSummary.ExecutiveSummary,
		HealthReasons:          projectHealthReasons(dash.AccountSummary.RiskDrivers),
		ImprovementSummary:     dash.AccountSummary.ImprovementSummary,
		TopActions:             projectTopActions(out.Actions, dash.TopActionIDs),
		DataQualityNotes:       append([]string{}, out.Limitations...),
		StockCoverageNarrative: frozen.StockCoverage.Narrative,
		AdEfficiencyNarrative:  frozen.AdEfficiency.Narrative,
		SellerPulseNarrative:   frozen.SellerPulse.Narrative,
		FunnelAIAnalysis:       projectFunnelAIAnalysisPtr(dash.FunnelAnalysis.AIAnalysis),
		CategorySummaries:      projectCategorySummariesWithFacts(dash.CategorySummaries, facts.Categories),
		RiskOverview:           projectRiskDistributionPtr(dash.RiskDistribution),
		WhatChanged:            projectWhatChangedFromRegistry(facts, previous, dash.WhatChanged, out.Actions),
	}

	// health_score is ALWAYS computed from the full backend risk candidate
	// registry (healthpolicy.BuildCandidateRegistry, grounded in Dashboard
	// Facts), never from the AI's risk_drivers subset — see
	// healthpolicy package doc and docs/current/dashboard-field-ownership.md.
	// The AI may only pick which registry candidates to explain first and
	// supply Title/Reason copy for them (projectRiskDriverViewsFromRegistry);
	// it can never invent a driver_id outside the registry or change a
	// candidate's Severity/ImpactScore/Penalty.
	registry := healthpolicy.BuildCandidateRegistry(facts)
	var healthScore *int32
	var healthBreakdown *dashboardhome.HealthScoreBreakdown
	if healthpolicy.HasSufficientCoreData(facts) {
		breakdown := healthpolicy.ComputeHealthScore(registry)
		healthBreakdown = &breakdown
		score := breakdown.FinalScore
		healthScore = &score
		if dash.AccountSummary.HealthScore != nil && *dash.AccountSummary.HealthScore != score {
			warnings = append(warnings, MismatchWarning{
				Field:     "account_summary.health_score",
				Reason:    "backend_score_overrides_ai",
				AIValue:   fmt.Sprintf("%d", *dash.AccountSummary.HealthScore),
				FactValue: fmt.Sprintf("%d", score),
			})
		}
	}
	riskDriverViews, unknownDriverIDs := projectRiskDriverViewsFromRegistry(registry, dash.AccountSummary.RiskDrivers)
	for _, id := range unknownDriverIDs {
		warnings = append(warnings, MismatchWarning{
			Field:   "account_summary.risk_drivers",
			Reason:  ReasonAIRiskDriverUnknownID,
			AIValue: id,
		})
	}
	// Prefer deterministic score→severity mapping so the API never surfaces
	// AI-only values like "warning" when a numeric index exists.
	mappedSeverity := mapDashboardSeverity(dash.AccountSummary.Severity)
	if st := healthstatus.FromScore(healthScore); st.Severity != "" {
		mappedSeverity = st.Severity
	}
	if healthScore != nil || mappedSeverity != "" {
		st := healthstatus.FromScore(healthScore)
		narrative.AccountHealth = &NarrativeAccountHealth{
			Severity:             mappedSeverity,
			HealthStatusCode:     st.Code,
			HealthStatusLabel:    st.Label,
			HealthStatusExplain:  st.Explanation,
			Headline:             dash.AccountSummary.Headline,
			ExecutiveSummary:     dash.AccountSummary.ExecutiveSummary,
			ImprovementSummary:   dash.AccountSummary.ImprovementSummary,
			ImprovementLevers:    projectImprovementLevers(dash.AccountSummary.ImprovementLevers),
			RiskDrivers:          riskDriverViews,
			HealthScoreBreakdown: healthBreakdown,
			HealthPolicyVersion:  healthpolicy.HealthPolicyVersion,
			HealthFactsHash:      facts.FactsHash,
		}
		if healthScore != nil {
			v := *healthScore
			narrative.AccountHealth.HealthScore = &v
		}
	}

	return DashboardProjection{
		Frozen:      frozen,
		Narrative:   narrative,
		HealthScore: healthScore,
		Severity:    mappedSeverity,
	}, warnings
}

// maxDisplayRiskDrivers caps the seller-facing risk_drivers list, matching
// the legacy dashboardhome riskDriverResponseLimit.
const maxDisplayRiskDrivers = 6

// projectRiskDriverViewsFromRegistry builds the seller-facing risk_drivers
// display list from the backend candidate registry, overlaying AI-authored
// Title/Reason copy where the AI referenced a matching (known) driver_id.
// The AI may only choose which candidates to explain first (display order)
// and provide their copy — it never adds, removes, or rescales a candidate.
// AI risk_drivers whose id is not in the registry are dropped from display
// (never scored) and returned as unknownDriverIDs for the caller to warn on.
func projectRiskDriverViewsFromRegistry(
	registry []healthpolicy.RiskCandidate,
	aiDrivers []contract.UnifiedRiskDriverV1,
) (views []NarrativeRiskDriverView, unknownDriverIDs []string) {
	byID := make(map[string]healthpolicy.RiskCandidate, len(registry))
	for _, c := range registry {
		byID[c.DriverID] = c
	}

	used := make(map[string]struct{}, len(aiDrivers))
	views = make([]NarrativeRiskDriverView, 0, len(registry))
	for _, ai := range aiDrivers {
		id := strings.TrimSpace(ai.ID)
		if id == "" {
			continue
		}
		cand, ok := byID[id]
		if !ok {
			// Only invented ids (outside the closed healthpolicy registry)
			// are publication blockers. A known registry code that is not
			// active for this facts snapshot is skipped for display without
			// failing the bundle — score already comes from the full registry.
			if !healthpolicy.IsKnownDriverID(id) {
				unknownDriverIDs = append(unknownDriverIDs, id)
			}
			continue
		}
		if _, dup := used[id]; dup {
			continue
		}
		used[id] = struct{}{}
		views = append(views, riskDriverViewFromCandidate(cand, ai.Title, ai.Reason))
	}

	remaining := make([]healthpolicy.RiskCandidate, 0, len(registry))
	for _, c := range registry {
		if c.DriverID == healthpolicy.DriverDataLimitation {
			// Informational-only candidate: never displayed as a scored risk.
			continue
		}
		if _, ok := used[c.DriverID]; ok {
			continue
		}
		remaining = append(remaining, c)
	}
	for _, c := range healthpolicy.SortCandidates(remaining) {
		if len(views) >= maxDisplayRiskDrivers {
			break
		}
		views = append(views, riskDriverViewFromCandidate(c, "", ""))
	}
	if len(views) > maxDisplayRiskDrivers {
		views = views[:maxDisplayRiskDrivers]
	}
	return views, unknownDriverIDs
}

func riskDriverViewFromCandidate(c healthpolicy.RiskCandidate, aiTitle, aiReason string) NarrativeRiskDriverView {
	title := strings.TrimSpace(aiTitle)
	if title == "" {
		title = healthpolicy.DefaultTitle(c.DriverID)
	}
	reason := strings.TrimSpace(aiReason)
	if reason == "" {
		reason = healthpolicy.DefaultReason(c)
	}
	return NarrativeRiskDriverView{
		ID:            c.DriverID,
		Category:      c.Domain,
		CategoryLabel: healthpolicy.DomainLabel(c.Domain),
		Severity:      healthpolicy.NormalizeSeverity(c.Severity),
		ImpactScore:   c.ImpactScore,
		Title:         title,
		Reason:        reason,
	}
}

// frozenFromFacts builds the backend-owned portion of FrozenDashboardData
// directly from dashboardfacts.DashboardFactsV1: DashboardMetrics,
// SellerPulse KPIs/Trend7D, StockCoverage segments/totals/status,
// AdEfficiency factual summary/points/campaigns, and SalesFunnel
// stages/status. None of these fields are ever taken from AI output — if
// facts are empty for a block, the projection stays empty rather than
// falling back to an AI-reported number.
func frozenFromFacts(facts dashboardfacts.DashboardFactsV1, sellerAccountID int64, metricDate string) dashboardhome.FrozenDashboardData {
	return dashboardhome.FrozenDashboardData{
		SchemaVersion:    dashboardhome.FrozenDashboardDataSchemaVersion,
		MetricDate:       metricDate,
		DashboardMetrics: factsDashboardMetrics(facts, sellerAccountID, metricDate),
		SellerPulse:      projectSellerPulseFromFacts(facts.SellerPulse),
		StockCoverage:    projectStockCoverageFromFacts(facts.StockCoverage),
		AdEfficiency:     projectAdEfficiencyFromFacts(facts.AdEfficiency),
		SalesFunnel:      projectSalesFunnelFromFacts(facts.Funnel),
	}
}

func factsDashboardMetrics(facts dashboardfacts.DashboardFactsV1, sellerAccountID int64, metricDate string) analytics.DashboardMetricsDTO {
	metrics := facts.DashboardMetrics
	if metrics.SellerAccountID == 0 {
		metrics.SellerAccountID = sellerAccountID
	}
	if metrics.AsOfDate == "" {
		metrics.AsOfDate = metricDate
	}
	return metrics
}

// detectKPIMismatches compares AI-reported seller_pulse KPI values against
// the corresponding backend fact for the same key and returns an
// informational warning for every disagreement. Facts always win regardless
// of these warnings; callers may log/monitor them.
func detectKPIMismatches(aiKPIs []contract.UnifiedKPICardV1, factKPIs []dashboardfacts.KPICardFact) []MismatchWarning {
	if len(aiKPIs) == 0 || len(factKPIs) == 0 {
		return nil
	}
	factByKey := make(map[string]dashboardfacts.KPICardFact, len(factKPIs))
	for _, f := range factKPIs {
		factByKey[f.Key] = f
	}
	var warnings []MismatchWarning
	for _, ai := range aiKPIs {
		fact, ok := factByKey[ai.Key]
		if !ok {
			continue
		}
		aiValue := strings.TrimSpace(ai.Value)
		factValue := strings.TrimSpace(fact.Value)
		if aiValue != "" && aiValue != factValue {
			warnings = append(warnings, MismatchWarning{
				Field:     fmt.Sprintf("seller_pulse.kpis.%s", ai.Key),
				Reason:    "ai_value_differs_from_facts",
				AIValue:   ai.Value,
				FactValue: fact.Value,
			})
		}
	}
	return warnings
}

// ApplyDateFreshness stamps Unified Analysis date-resolution metadata onto the
// frozen dashboard bundle and adds a seller-facing freshness note.
//
// A fixed-date (historical) analysis is expected to have a large gap
// between EffectiveMetricDate and today — that alone must never produce a
// "data not updated" note (resolution.FreshnessLagDays/IsStale are always
// SOURCE freshness, never analysis age; see analysisdata.MetricDateResolution
// doc comment). Historical analyses instead get a neutral
// "Исторический анализ по данным на DD.MM.YYYY" note, unless the underlying
// sources are themselves genuinely stale, in which case the staleness note
// still applies.
func ApplyDateFreshness(proj *DashboardProjection, resolution analysisdata.MetricDateResolution) {
	if proj == nil {
		return
	}
	effective := resolution.EffectiveMetricDate
	if effective == "" {
		effective = proj.Frozen.MetricDate
	}
	proj.Frozen.RequestedDate = resolution.RequestedDate
	proj.Frozen.EffectiveMetricDate = effective
	proj.Frozen.LatestCompleteDataDate = resolution.LatestCompleteDataDate
	proj.Frozen.DataFreshnessLagDays = resolution.FreshnessLagDays
	proj.Frozen.DataIsStale = resolution.IsStale
	proj.Frozen.HistoricalAnalysis = resolution.HistoricalAnalysis
	proj.Frozen.AnalysisAgeDays = resolution.AnalysisAgeDays
	proj.Frozen.SourceFreshnessLagDays = resolution.SourceFreshnessLagDays
	if effective != "" {
		proj.Frozen.MetricDate = effective
	}

	note := dashboardhome.DateFreshnessNote(resolution.HistoricalAnalysis, resolution.IsStale, resolution.FreshnessLagDays, effective)
	if note == "" {
		return
	}
	for _, existing := range proj.Narrative.DataQualityNotes {
		if existing == note {
			return
		}
	}
	proj.Narrative.DataQualityNotes = append(proj.Narrative.DataQualityNotes, note)
}

// mapDashboardSeverity converts Unified Analysis severity vocabulary into the
// values accepted by chk_ai_dashboard_narratives_severity and the existing
// frontend health-severity labels. Unknown values become "" (NULL in DB).
func mapDashboardSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return "critical"
	case "high", "important", "warning", "risk":
		// Unified / legacy AI variants; closest dashboard tone.
		return "high"
	case "medium":
		return "medium"
	case "attention":
		return "attention"
	case "low", "stable", "positive":
		return "low"
	default:
		return ""
	}
}

func widgetStatus(status string) dashboardhome.WidgetStatus {
	switch strings.TrimSpace(status) {
	case "ok", "ready":
		return dashboardhome.WidgetAvailable
	default:
		return dashboardhome.WidgetStatus(status)
	}
}

// projectSellerPulseFromFacts builds the factual seller_pulse KPIs/trend
// from backend Dashboard Facts. Backend facts always win here; there is no
// AI-driven equivalent of this function.
func projectSellerPulseFromFacts(facts dashboardfacts.SellerPulseFacts) dashboardhome.SellerPulse {
	out := dashboardhome.SellerPulse{
		KPIs:    make([]dashboardhome.KPICard, 0, len(facts.KPIs)),
		Trend7D: make([]dashboardhome.TrendDayPoint, 0, len(facts.Trend7D)),
	}
	for _, k := range facts.KPIs {
		card := dashboardhome.KPICard{Key: k.Key, Label: k.Label, Value: k.Value, Hint: k.Hint}
		if k.Delta != nil {
			card.Delta = &dashboardhome.KPICardDelta{Abs: k.Delta.Abs, Pct: k.Delta.Pct, Label: k.Delta.Label, Unit: k.Delta.Unit, Display: k.Delta.Display}
		}
		for _, sp := range k.Sparkline7D {
			card.Sparkline7D = append(card.Sparkline7D, dashboardhome.SparklinePoint{Date: sp.Date, Value: sp.Value, Revenue: sp.Revenue, HasData: sp.HasData})
		}
		out.KPIs = append(out.KPIs, card)
	}
	for _, t := range facts.Trend7D {
		out.Trend7D = append(out.Trend7D, dashboardhome.TrendDayPoint{Date: t.Date, Revenue: t.Revenue, Orders: t.Orders})
	}
	return out
}

func projectSellerPulseNarrative(in *contract.UnifiedSellerPulseNarrativeV1) *dashboardhome.SellerPulseNarrative {
	if in == nil {
		return nil
	}
	out := &dashboardhome.SellerPulseNarrative{
		Status:          in.Status,
		Tone:            in.Tone,
		Headline:        stripUnauthorizedHealthClaims(in.Headline),
		DisplayHeadline: strings.TrimSpace(in.Headline) != "",
		Summary:         stripUnauthorizedHealthClaims(in.Summary),
		DataQualityNote: in.DataQualityNote,
	}
	for _, fact := range in.KeyPoints {
		note := projectSellerPulseNote(fact)
		if note == nil {
			continue
		}
		note.Text = stripUnauthorizedHealthClaims(note.Text)
		out.KeyPoints = append(out.KeyPoints, *note)
	}
	for _, fact := range in.ImportantFacts {
		note := projectSellerPulseNote(fact)
		if note == nil {
			continue
		}
		note.Text = stripUnauthorizedHealthClaims(note.Text)
		out.ImportantNotes = append(out.ImportantNotes, *note)
	}
	return out
}

// stripUnauthorizedHealthClaims removes AI-invented health labels that may
// contradict the backend health score. Pulse must describe sales/drivers/risks,
// not declare the cabinet "high health" or "critical" on its own.
func stripUnauthorizedHealthClaims(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	banned := []string{
		"высокий индекс здоровья",
		"высоким индексом здоровья",
		"высокое здоровье",
		"высоком здоровье",
		"критическое состояние",
		"критическом состоянии",
		"кабинет в критическом",
		"здоровье кабинета высокое",
		"здоровье кабинета критическое",
	}
	for _, b := range banned {
		if strings.Contains(lower, b) {
			// Drop the sentence containing the claim rather than leave a contradiction.
			parts := strings.Split(s, ".")
			kept := make([]string, 0, len(parts))
			for _, p := range parts {
				pTrim := strings.TrimSpace(p)
				if pTrim == "" {
					continue
				}
				if strings.Contains(strings.ToLower(pTrim), b) {
					continue
				}
				kept = append(kept, pTrim)
			}
			s = strings.Join(kept, ". ")
			if s != "" && !strings.HasSuffix(s, ".") {
				s += "."
			}
			lower = strings.ToLower(s)
		}
	}
	return strings.TrimSpace(s)
}

func projectSellerPulseNote(fact contract.UnifiedSellerPulsePointV1) *dashboardhome.SellerPulseImportantNote {
	title := strings.TrimSpace(fact.Title)
	text := strings.TrimSpace(fact.Explanation)
	if text == "" {
		return nil
	}
	// Never surface internal driver/factor codes as note titles.
	if looksLikeInternalCode(title) {
		title = ""
	}
	return &dashboardhome.SellerPulseImportantNote{
		Title:      title,
		Text:       text,
		Tone:       mapSignalSeverityToTone(fact.Severity),
		MetricRefs: append([]string(nil), fact.FactIDs...),
		FactIDs:    append([]string(nil), fact.FactIDs...),
		EntityIDs:  append([]string(nil), fact.EntityIDs...),
	}
}

func looksLikeInternalCode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, " ") {
		return false
	}
	return strings.Contains(s, "_") && s == strings.ToLower(s)
}

// skuFactsLookup is a reverse index over dashboardfacts.SKUFactsIndex (keyed
// by ozon_product_id) built by SKU/offer_id, the identifiers the AI output
// actually carries for sku_focus/key_skus selections.
type skuFactsLookup struct {
	byOzonProductID map[int64]dashboardfacts.SKUFactRecord
	bySKU           map[int64]dashboardfacts.SKUFactRecord
	byOfferID       map[string]dashboardfacts.SKUFactRecord
}

func buildSKUFactsLookup(idx dashboardfacts.SKUFactsIndex) skuFactsLookup {
	lookup := skuFactsLookup{
		byOzonProductID: make(map[int64]dashboardfacts.SKUFactRecord, len(idx.ByOzonProductID)),
		bySKU:           make(map[int64]dashboardfacts.SKUFactRecord, len(idx.ByOzonProductID)),
		byOfferID:       make(map[string]dashboardfacts.SKUFactRecord, len(idx.ByOzonProductID)),
	}
	for _, rec := range idx.ByOzonProductID {
		lookup.byOzonProductID[rec.OzonProductID] = rec
		if rec.SKU != nil {
			lookup.bySKU[*rec.SKU] = rec
		}
		if rec.OfferID != nil && strings.TrimSpace(*rec.OfferID) != "" {
			lookup.byOfferID[*rec.OfferID] = rec
		}
	}
	return lookup
}

// find resolves an AI-selected SKU/offer_id against the facts index. Returns
// ok=false when neither identifier resolves — callers must drop the card
// rather than substitute AI-provided numbers (never fallback).
func (l skuFactsLookup) find(sku *int64, offerID *string) (dashboardfacts.SKUFactRecord, bool) {
	if sku != nil {
		if rec, ok := l.bySKU[*sku]; ok {
			return rec, true
		}
	}
	if offerID != nil && strings.TrimSpace(*offerID) != "" {
		if rec, ok := l.byOfferID[*offerID]; ok {
			return rec, true
		}
	}
	return dashboardfacts.SKUFactRecord{}, false
}

// resolveAISelectedSKU recovers a Pack sku_id when the model leaves sku/offer_id
// null but embeds the identifier in product_name (e.g. "SKU 801000100003").
func resolveAISelectedSKU(sku *int64, offerID *string, productName string) (*int64, *string) {
	if sku != nil || (offerID != nil && strings.TrimSpace(*offerID) != "") {
		return sku, offerID
	}
	name := strings.TrimSpace(productName)
	if name == "" {
		return sku, offerID
	}
	re := regexp.MustCompile(`(?i)(?:^|\bsku[\s#:*-]*)(\d{6,})`)
	if m := re.FindStringSubmatch(name); len(m) == 2 {
		if id, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return &id, offerID
		}
	}
	if id, err := strconv.ParseInt(name, 10, 64); err == nil && id >= 100000 {
		return &id, offerID
	}
	return sku, offerID
}

func factProductName(rec dashboardfacts.SKUFactRecord, aiProductName string) string {
	if rec.ProductName != nil && strings.TrimSpace(*rec.ProductName) != "" {
		return *rec.ProductName
	}
	return aiProductName
}

// skuFocusCardFromFacts joins an AI-selected sku_focus entity against
// backend facts: AI supplies problem/key_metric copy (entity selection +
// text), facts supply name/offer_id/stock/coverage/orders.
// Sparse sparklines never drop the card — they set TrendStatus.
func skuFocusCardFromFacts(ai contract.UnifiedSKUFocusCardV1, lookup skuFactsLookup) (dashboardhome.SKUFocusCard, skuFocusResolveKind) {
	sku, offerID := resolveAISelectedSKU(ai.SKU, ai.OfferID, ai.ProductName)
	rec, ok := lookup.find(sku, offerID)
	if !ok {
		return dashboardhome.SKUFocusCard{}, skuFocusUnresolved
	}
	problem := strings.TrimSpace(ai.Problem)
	keyMetric := strings.TrimSpace(ai.KeyMetric)
	if problem == "" && keyMetric == "" {
		return dashboardhome.SKUFocusCard{}, skuFocusMissingIssue
	}
	spark := sparklinePointsFromFacts(rec.Sparkline7D)
	observed := 0
	for _, pt := range spark {
		if pt.HasData {
			observed++
		}
	}
	trendStatus, trendMessage, sparkOut := skuFocusTrendFromObserved(spark, observed)

	stock := rec.StockAvailable
	curOrders := int32(rec.Orders7d)
	prevOrders := int32(rec.OrdersPrev7d)
	ordersDelta := curOrders - prevOrders
	revenueDelta := rec.Revenue7d - rec.RevenuePrev7d
	card := dashboardhome.SKUFocusCard{
		OzonProductID:        rec.OzonProductID,
		SKU:                  rec.SKU,
		OfferID:              rec.OfferID,
		ProductName:          factProductName(rec, ai.ProductName),
		Problem:              problem,
		KeyMetric:            keyMetric,
		TrendStatus:          trendStatus,
		TrendMessage:         trendMessage,
		Sparkline7D:          sparkOut,
		StockTrend7D:         stockTrendFromFacts(rec),
		CurrentStock:         &stock,
		CoverageDays:         rec.DaysOfCover,
		CoverageBucket:       rec.CoverageBucket,
		CurrentOrders:        &curOrders,
		PreviousOrders:       &prevOrders,
		OrdersDeltaDayToDay:  &ordersDelta,
		RevenueDeltaDayToDay: &revenueDelta,
		NoSalesDays:          rec.DaysWithoutSales,
		AvgDailyRevenue7d:    avgDailyRevenuePtr(rec),
		RevenueAtRisk7d:      revenueAtRisk7dPtr(rec),
	}
	return card, skuFocusOK
}

func skuFocusTrendFromObserved(spark []dashboardhome.SparklinePoint, observed int) (status, message string, out []dashboardhome.SparklinePoint) {
	switch {
	case observed >= 2:
		return dashboardhome.SKUFocusTrendAvailable, "", spark
	case observed == 1:
		// Keep the single observed point; UI must not invent a second point.
		return dashboardhome.SKUFocusTrendNotEnoughData, "Недостаточно точек для тренда", spark
	default:
		return dashboardhome.SKUFocusTrendNotEnoughData, "Тренд недоступен: нет дневных точек", nil
	}
}

// sparklinePointsFromFacts converts a dashboardfacts sparkline (identical
// shape) into the dashboardhome projection type used by SKUFocusCard.
func sparklinePointsFromFacts(in []dashboardfacts.SparklinePoint) []dashboardhome.SparklinePoint {
	if len(in) == 0 {
		return nil
	}
	out := make([]dashboardhome.SparklinePoint, 0, len(in))
	for _, sp := range in {
		out = append(out, dashboardhome.SparklinePoint{Date: sp.Date, Value: sp.Value, Revenue: sp.Revenue, HasData: sp.HasData})
	}
	return out
}

func stockTrendFromFacts(rec dashboardfacts.SKUFactRecord) []dashboardhome.StockTrendPoint {
	if len(rec.Sparkline7D) == 0 {
		return nil
	}
	out := make([]dashboardhome.StockTrendPoint, 0, len(rec.Sparkline7D))
	observed := 0
	for _, sp := range rec.Sparkline7D {
		pt := dashboardhome.StockTrendPoint{Date: sp.Date}
		if sp.HasData && sp.Stock != nil {
			pt.HasData = true
			pt.Stock = *sp.Stock
			observed++
		}
		out = append(out, pt)
	}
	if observed < 2 {
		return nil
	}
	return out
}

func avgDailyRevenuePtr(rec dashboardfacts.SKUFactRecord) *float64 {
	if rec.Revenue7d <= 0 {
		return nil
	}
	v := math.Round(rec.Revenue7d/7*100) / 100
	return &v
}

func revenueAtRisk7dPtr(rec dashboardfacts.SKUFactRecord) *float64 {
	switch rec.CoverageBucket {
	case "out_of_stock", "critical_0_3", "shortage_4_7":
		if rec.Revenue7d <= 0 {
			return nil
		}
		v := math.Round(rec.Revenue7d/7*100) / 100
		return &v
	default:
		return nil
	}
}

// keySkuRevenueTrendFromFacts converts a dashboardfacts sparkline into the
// KeySkuRevenueTrendPoint shape used by KeySkuCard.RevenueTrend7D. Missing
// days (HasData=false) stay HasData=nil/Orders=0/Revenue=0, matching the
// existing "no observed data" representation for this field.
func keySkuRevenueTrendFromFacts(in []dashboardfacts.SparklinePoint) []dashboardhome.KeySkuRevenueTrendPoint {
	if len(in) == 0 {
		return nil
	}
	out := make([]dashboardhome.KeySkuRevenueTrendPoint, 0, len(in))
	for _, sp := range in {
		pt := dashboardhome.KeySkuRevenueTrendPoint{Date: sp.Date}
		if sp.HasData {
			hasData := true
			pt.HasData = &hasData
			if sp.Value != nil {
				pt.Orders = int32(*sp.Value)
			}
			if sp.Revenue != nil {
				pt.Revenue = *sp.Revenue
			}
		}
		out = append(out, pt)
	}
	return out
}

func projectSKUFocusCards(in []contract.UnifiedSKUFocusCardV1, lookup skuFactsLookup, tab string) ([]dashboardhome.SKUFocusCard, []MismatchWarning) {
	out := make([]dashboardhome.SKUFocusCard, 0, len(in))
	var warnings []MismatchWarning
	for i, c := range in {
		card, errKind := skuFocusCardFromFacts(c, lookup)
		switch errKind {
		case skuFocusOK:
			out = append(out, card)
		case skuFocusUnresolved:
			warnings = append(warnings, MismatchWarning{
				Field:   fmt.Sprintf("sku_focus.%s[%d]", tab, i),
				Reason:  ReasonSKUFocusUnresolvedEntity,
				AIValue: formatAISelectedSKU(c.SKU, c.OfferID, c.ProductName),
			})
		case skuFocusMissingIssue:
			warnings = append(warnings, MismatchWarning{
				Field:   fmt.Sprintf("sku_focus.%s[%d]", tab, i),
				Reason:  ReasonSKUFocusMissingIssue,
				AIValue: formatAISelectedSKU(c.SKU, c.OfferID, c.ProductName),
			})
		default:
			// No silent sparse drops — sparse is handled inside skuFocusOK path.
		}
	}
	return out, warnings
}

type skuFocusResolveKind int

const (
	skuFocusOK skuFocusResolveKind = iota
	skuFocusUnresolved
	skuFocusMissingIssue
)

func formatAISelectedSKU(sku *int64, offerID *string, productName string) string {
	parts := make([]string, 0, 3)
	if sku != nil {
		parts = append(parts, fmt.Sprintf("sku=%d", *sku))
	}
	if offerID != nil && strings.TrimSpace(*offerID) != "" {
		parts = append(parts, "offer="+strings.TrimSpace(*offerID))
	}
	if strings.TrimSpace(productName) != "" {
		parts = append(parts, "name="+strings.TrimSpace(productName))
	}
	return strings.Join(parts, "/")
}

// keySkuCardFromCohortItem builds a KeySkuCard from a backend cohort member,
// joining sparkline/stock from SKUFactsIndex. AI may only supply product-name
// fallback when the SKU is already in the cohort (matched by sku id).
func keySkuCardFromCohortItem(item dashboardfacts.KeySKUCohortItem, rec dashboardfacts.SKUFactRecord, aiProductName string) dashboardhome.KeySkuCard {
	stock := rec.StockAvailable
	return dashboardhome.KeySkuCard{
		OzonProductID:             item.OzonProductID,
		ProductName:               factProductName(rec, aiProductName),
		SKU:                       firstNonNilSKU(item.SKU, rec.SKU),
		OfferID:                   firstNonNilOffer(item.OfferID, rec.OfferID),
		Revenue7d:                 item.Revenue7d,
		Orders7d:                  item.Orders7d,
		RevenueSharePct:           item.RevenueSharePct,
		CumulativeRevenueSharePct: item.CumulativeRevenueSharePct,
		CoverageDays:              rec.DaysOfCover,
		CurrentStock:              &stock,
		RevenueTrend7D:            keySkuRevenueTrendFromFacts(rec.Sparkline7D),
	}
}

func firstNonNilSKU(a, b *int64) *int64 {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilOffer(a, b *string) *string {
	if a != nil && strings.TrimSpace(*a) != "" {
		return a
	}
	return b
}

// projectKeySKUCardsFromCohort uses backend KeySKUCohortFacts as membership.
// AI key_skus outside the cohort are dropped; cohort members without AI text
// are still included (empty AI overlay). AI reason text is not a membership
// gate — only sku-id match against the cohort is.
func projectKeySKUCardsFromCohort(aiCards []contract.UnifiedKeySKUCardV1, lookup skuFactsLookup, cohort dashboardfacts.KeySKUCohortFacts) []dashboardhome.KeySkuCard {
	aiBySKU := make(map[int64]contract.UnifiedKeySKUCardV1, len(aiCards))
	for _, c := range aiCards {
		sku, _ := resolveAISelectedSKU(c.SKU, c.OfferID, c.ProductName)
		if sku == nil {
			continue
		}
		aiBySKU[*sku] = c
	}

	out := make([]dashboardhome.KeySkuCard, 0, len(cohort.Items))
	for _, item := range cohort.Items {
		rec, ok := lookup.byOzonProductID[item.OzonProductID]
		if !ok {
			rec, ok = lookup.find(item.SKU, item.OfferID)
		}
		if !ok {
			// Cohort items are built from the same SKUFactsIndex; synthesize a
			// minimal record so membership is never lost when identity is sparse.
			rec = dashboardfacts.SKUFactRecord{
				OzonProductID: item.OzonProductID,
				SKU:           item.SKU,
				OfferID:       item.OfferID,
				ProductName:   item.ProductName,
				Revenue7d:     item.Revenue7d,
				Orders7d:      item.Orders7d,
			}
		}
		aiName := ""
		if item.SKU != nil {
			if ai, matched := aiBySKU[*item.SKU]; matched {
				aiName = ai.ProductName
			}
		}
		out = append(out, keySkuCardFromCohortItem(item, rec, aiName))
	}
	return out
}

// projectSKUFocus joins AI-selected sku_focus entities against backend facts.
// When an AI tab is empty but backend candidates exist, cards are filled from
// facts (deterministic problem/key_metric). KeySKUs membership stays cohort-owned.
func projectSKUFocus(in contract.UnifiedSKUFocusV1, lookup skuFactsLookup, cohort dashboardfacts.KeySKUCohortFacts, candidates dashboardfacts.SKUFocusCandidates) (dashboardhome.SKUFocus, []MismatchWarning) {
	var warnings []MismatchWarning
	orderDrop, w := projectSKUFocusCards(in.OrderDrop, lookup, "order_drop")
	warnings = append(warnings, w...)
	stockRisk, w := projectSKUFocusCards(in.StockRisk, lookup, "stock_risk")
	warnings = append(warnings, w...)
	noSales, w := projectSKUFocusCards(in.NoSalesWithStock, lookup, "no_sales_with_stock")
	warnings = append(warnings, w...)
	slow, w := projectSKUFocusCards(in.SlowTurnover, lookup, "slow_turnover")
	warnings = append(warnings, w...)

	if len(orderDrop) == 0 {
		orderDrop = factualSKUFocusCards(candidates.OrderDrop, "Падение заказов за неделю", orderDropKeyMetric)
	}
	if len(stockRisk) == 0 {
		stockRisk = factualSKUFocusCards(candidates.StockRisk, "Риск по остатку", stockRiskKeyMetric)
	} else {
		// Prefer AI reasons for selected cards, then append remaining factual
		// stock-risk SKUs so Stock Coverage CTA can open the full list.
		// Totals stay owned by backend candidates (never inflate from AI extras).
		stockRisk = mergeStockRiskCards(stockRisk, candidates.StockRisk)
	}
	if len(noSales) == 0 {
		noSales = factualSKUFocusCards(candidates.NoSales, "Есть остаток без продаж", noSalesKeyMetric)
	}

	orderDropTotal := candidates.OrderDropTotal
	if orderDropTotal < len(orderDrop) {
		orderDropTotal = len(orderDrop)
	}
	stockRiskTotal := candidates.StockRiskTotal
	noSalesTotal := candidates.NoSalesTotal
	if noSalesTotal < len(noSales) {
		noSalesTotal = len(noSales)
	}

	orderDrop = dashboardfacts.ClampSKUFocusCards(orderDrop, dashboardfacts.SKUFocusDashboardOrderDropLimit)
	// Do not clamp stock_risk: FE shows top-N by default and expands on show=all.
	noSales = dashboardfacts.ClampSKUFocusCards(noSales, dashboardfacts.SKUFocusDashboardNoSalesLimit)
	keySKUs := projectKeySKUCardsFromCohort(in.KeySKUs, lookup, cohort)
	keySKUs = dashboardfacts.ClampSKUFocusCards(keySKUs, dashboardfacts.SKUFocusDashboardKeySKULimit)

	return dashboardhome.SKUFocus{
		OrderDrop:                  orderDrop,
		StockRisk:                  stockRisk,
		StockRiskTotalCount:        stockRiskTotal,
		NoSalesWithStock:           noSales,
		NoSalesWithStockTotalCount: noSalesTotal,
		SlowTurnover:               slow,
		KeySKUs:                    keySKUs,
		KeySKUsTotalCount:          len(cohort.Items),
		StoreTotalRevenue7d:        cohort.StoreTotalRevenue7d,
		KeySKUsTotalRevenue7d:      cohort.KeySKUsTotalRevenue7d,
	}, warnings
}

func factualSKUFocusCards(recs []dashboardfacts.SKUFactRecord, problem string, keyMetricFn func(dashboardfacts.SKUFactRecord) string) []dashboardhome.SKUFocusCard {
	out := make([]dashboardhome.SKUFocusCard, 0, len(recs))
	for _, rec := range recs {
		out = append(out, factualSKUFocusCard(rec, problem, keyMetricFn))
	}
	return out
}

func factualSKUFocusCard(rec dashboardfacts.SKUFactRecord, problem string, keyMetricFn func(dashboardfacts.SKUFactRecord) string) dashboardhome.SKUFocusCard {
	spark := sparklinePointsFromFacts(rec.Sparkline7D)
	observed := 0
	for _, pt := range spark {
		if pt.HasData {
			observed++
		}
	}
	trendStatus, trendMessage, sparkOut := skuFocusTrendFromObserved(spark, observed)
	stock := rec.StockAvailable
	curOrders := int32(rec.Orders7d)
	prevOrders := int32(rec.OrdersPrev7d)
	ordersDelta := curOrders - prevOrders
	revenueDelta := rec.Revenue7d - rec.RevenuePrev7d
	return dashboardhome.SKUFocusCard{
		OzonProductID:        rec.OzonProductID,
		SKU:                  rec.SKU,
		OfferID:              rec.OfferID,
		ProductName:          factProductName(rec, ""),
		Problem:              problem,
		KeyMetric:            keyMetricFn(rec),
		TrendStatus:          trendStatus,
		TrendMessage:         trendMessage,
		Sparkline7D:          sparkOut,
		StockTrend7D:         stockTrendFromFacts(rec),
		CurrentStock:         &stock,
		CoverageDays:         rec.DaysOfCover,
		CoverageBucket:       rec.CoverageBucket,
		CurrentOrders:        &curOrders,
		PreviousOrders:       &prevOrders,
		OrdersDeltaDayToDay:  &ordersDelta,
		RevenueDeltaDayToDay: &revenueDelta,
		NoSalesDays:          rec.DaysWithoutSales,
		AvgDailyRevenue7d:    avgDailyRevenuePtr(rec),
		RevenueAtRisk7d:      revenueAtRisk7dPtr(rec),
	}
}

// mergeStockRiskCards keeps AI-authored reasons for cards that are in the
// backend stock-risk candidate universe, then appends any remaining factual
// candidates. AI cards outside the candidate set are dropped so totals stay
// owned by BuildSKUFocusCandidates.
func mergeStockRiskCards(aiCards []dashboardhome.SKUFocusCard, candidates []dashboardfacts.SKUFactRecord) []dashboardhome.SKUFocusCard {
	candidateByID := make(map[int64]dashboardfacts.SKUFactRecord, len(candidates))
	for _, rec := range candidates {
		candidateByID[rec.OzonProductID] = rec
	}
	seen := make(map[int64]struct{}, len(candidates))
	out := make([]dashboardhome.SKUFocusCard, 0, len(candidates))
	for _, card := range aiCards {
		if card.OzonProductID == 0 {
			continue
		}
		if _, ok := candidateByID[card.OzonProductID]; !ok {
			continue
		}
		if _, ok := seen[card.OzonProductID]; ok {
			continue
		}
		seen[card.OzonProductID] = struct{}{}
		out = append(out, card)
	}
	for _, rec := range candidates {
		if _, ok := seen[rec.OzonProductID]; ok {
			continue
		}
		seen[rec.OzonProductID] = struct{}{}
		out = append(out, factualSKUFocusCard(rec, "Риск по остатку", stockRiskKeyMetric))
	}
	return out
}

func orderDropKeyMetric(rec dashboardfacts.SKUFactRecord) string {
	if rec.OrdersPrev7d <= 0 {
		return "Падение заказов за 7 дней"
	}
	pct := float64(rec.OrdersPrev7d-rec.Orders7d) / float64(rec.OrdersPrev7d) * 100
	return fmt.Sprintf("Заказы 7д: %d → %d (−%.0f%%)", rec.OrdersPrev7d, rec.Orders7d, pct)
}

func stockRiskKeyMetric(rec dashboardfacts.SKUFactRecord) string {
	if rec.DaysOfCover != nil {
		return fmt.Sprintf("Покрытие: %.1f дн · остаток %d", *rec.DaysOfCover, rec.StockAvailable)
	}
	if rec.StockAvailable <= 0 {
		return "Нет остатка"
	}
	return fmt.Sprintf("Остаток %d", rec.StockAvailable)
}

func noSalesKeyMetric(rec dashboardfacts.SKUFactRecord) string {
	days := int32(0)
	if rec.DaysWithoutSales != nil {
		days = *rec.DaysWithoutSales
	}
	return fmt.Sprintf("Без продаж %d дн · остаток %d", days, rec.StockAvailable)
}

// projectStockCoverageFromFacts builds the factual stock_coverage
// segments/totals/status from backend Dashboard Facts. Backend facts
// always win here; there is no AI-driven equivalent of this function.
func projectStockCoverageFromFacts(facts dashboardfacts.StockCoverageFacts) dashboardhome.StockCoverageWidget {
	out := dashboardhome.StockCoverageWidget{
		WidgetBase: dashboardhome.WidgetBase{Status: widgetStatus(facts.Status)},
		TotalSKUs:  facts.TotalSKUs,
		Segments:   make([]dashboardhome.StockCoverageSegment, 0, len(facts.Segments)),
	}
	for _, s := range facts.Segments {
		out.Segments = append(out.Segments, dashboardhome.StockCoverageSegment{Key: s.Key, Label: s.Label, SKUCount: s.SKUCount, Severity: s.Severity})
	}
	return out
}

func projectStockCoverageNarrative(in *contract.UnifiedStockCoverageNarrativeV1) *dashboardhome.StockCoverageNarrative {
	if in == nil {
		return nil
	}
	return &dashboardhome.StockCoverageNarrative{
		Status:   in.Status,
		Headline: in.Headline,
		Summary:  in.Summary,
		NextStep: in.NextStep,
		Tone:     in.Tone,
	}
}

// projectAdEfficiencyFromFacts builds the factual ad_efficiency
// summary/points/campaigns from backend Dashboard Facts. Backend facts
// always win here; there is no AI-driven equivalent of this function.
func projectAdEfficiencyFromFacts(facts dashboardfacts.AdEfficiencyFacts) dashboardhome.AdEfficiencyWidget {
	out := dashboardhome.AdEfficiencyWidget{
		WidgetBase: dashboardhome.WidgetBase{Status: widgetStatus(facts.Status)},
	}
	if facts.Period != nil {
		out.Period = &dashboardhome.AdEfficiencyPeriod{
			MetricDate: facts.Period.MetricDate,
			DateFrom:   facts.Period.DateFrom,
			DateTo:     facts.Period.DateTo,
			SampleDays: facts.Period.SampleDays,
		}
	}
	if facts.Summary != nil {
		out.Summary = &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct:              facts.Summary.CurrentDRRPct,
			PreviousDRRPct:             facts.Summary.PreviousDRRPct,
			AverageDRR7dPct:            facts.Summary.AverageDRR7dPct,
			DeltaVsPreviousPp:          facts.Summary.DeltaVsPreviousPp,
			DeltaVsAverage7dPp:         facts.Summary.DeltaVsAverage7dPp,
			TotalSpendToday:            facts.Summary.TotalSpendToday,
			TotalRevenueToday:          facts.Summary.TotalRevenueToday,
			TotalStoreRevenueToday:     facts.Summary.TotalStoreRevenueToday,
			TotalOrdersToday:           facts.Summary.TotalOrdersToday,
			TotalSpend7d:               facts.Summary.TotalSpend7d,
			TotalAttributedRevenue7d:   facts.Summary.TotalAttributedRevenue7d,
			TotalStoreRevenue7d:        facts.Summary.TotalStoreRevenue7d,
			CampaignsHighDRRCount:      facts.Summary.CampaignsHighDRRCount,
			CampaignsNoOrdersCount:     facts.Summary.CampaignsNoOrdersCount,
			HighDRRSpend:               facts.Summary.HighDRRSpend,
			SpendWithoutOrders:         facts.Summary.SpendWithoutOrders,
			SpendWithoutOrdersDateFrom: facts.Summary.SpendWithoutOrdersDateFrom,
			SpendWithoutOrdersDateTo:   facts.Summary.SpendWithoutOrdersDateTo,
			HighDRRThresholdPct:        facts.Summary.HighDRRThresholdPct,
		}
	}
	for _, t := range facts.Trend7D {
		out.Trend7D = append(out.Trend7D, dashboardhome.AdEfficiencyTrendPoint{
			Date:        t.Date,
			Spend:       t.Spend,
			Revenue:     t.Revenue,
			Orders:      t.Orders,
			DRRPct:      t.DRRPct,
			IsToday:     t.IsToday,
			IsYesterday: t.IsYesterday,
			DataStatus:  t.DataStatus,
		})
	}
	for _, c := range facts.HighDRRCampaigns {
		out.HighDRRCampaigns = append(out.HighDRRCampaigns, adCampaignDisplayRowFromFacts(c))
	}
	for _, c := range facts.NoOrderCampaigns {
		out.NoOrderCampaigns = append(out.NoOrderCampaigns, adCampaignDisplayRowFromFacts(c))
	}
	for _, p := range facts.Points {
		out.Points = append(out.Points, dashboardhome.AdEfficiencyPoint{
			CampaignExternalID: p.CampaignExternalID,
			CampaignName:       p.CampaignName,
			Spend:              p.Spend,
			DRR:                p.DRR,
			Revenue:            p.Revenue,
			Orders:             p.Orders,
			EfficiencySignal:   p.EfficiencySignal,
		})
	}
	return out
}

func adCampaignDisplayRowFromFacts(c dashboardfacts.AdCampaignDisplayRow) dashboardhome.AdCampaignDisplayRow {
	return dashboardhome.AdCampaignDisplayRow{
		CampaignExternalID: c.CampaignExternalID,
		CampaignName:       c.CampaignName,
		DRRPct:             c.DRRPct,
		Spend:              c.Spend,
		Orders:             c.Orders,
		Revenue:            c.Revenue,
		ReasonCode:         c.ReasonCode,
	}
}

// projectAdEfficiencyNarrative copies AI advertising narrative as-is.
// Percent grounding / denominator fixes are rejected by outputvalidator and
// bundlevalidator — the adapter must never silently rewrite seller-facing text.
// MetricClaims / ThresholdClaims are preserved so bundle validation can use
// structured semantics instead of re-interpreting Russian narrative prose.
func projectAdEfficiencyNarrative(
	in *contract.UnifiedAdEfficiencyNarrativeV1,
) *dashboardhome.AdEfficiencyNarrative {
	if in == nil {
		return nil
	}
	return &dashboardhome.AdEfficiencyNarrative{
		Status:          in.Status,
		Headline:        in.Headline,
		Summary:         in.Summary,
		Recommendation:  in.Recommendation, // retained for reading old publications; new schema omits it
		Checklist:       projectAdEfficiencyChecklist(in.Checklist),
		Limitations:     append([]string{}, in.Limitations...),
		MetricClaims:    projectAdEfficiencyMetricClaims(in.MetricClaims),
		ThresholdClaims: projectAdEfficiencyThresholdClaims(in.ThresholdClaims),
	}
}

func projectAdEfficiencyMetricClaims(in []contract.UnifiedMetricClaimV1) []dashboardhome.AdEfficiencyMetricClaim {
	if len(in) == 0 {
		return nil
	}
	out := make([]dashboardhome.AdEfficiencyMetricClaim, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.AdEfficiencyMetricClaim{
			FactID:      c.FactID,
			MetricCode:  c.MetricCode,
			Value:       c.Value,
			PeriodType:  c.PeriodType,
			PeriodStart: c.PeriodStart,
			PeriodEnd:   c.PeriodEnd,
		})
	}
	return out
}

func projectAdEfficiencyThresholdClaims(in []contract.UnifiedThresholdClaimV1) []dashboardhome.AdEfficiencyThresholdClaim {
	if len(in) == 0 {
		return nil
	}
	out := make([]dashboardhome.AdEfficiencyThresholdClaim, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.AdEfficiencyThresholdClaim{
			PolicyID:   c.PolicyID,
			MetricCode: c.MetricCode,
			Scope:      c.Scope,
			Operator:   c.Operator,
			Value:      c.Value,
		})
	}
	return out
}

// projectAdEfficiencyChecklist copies the AI-authored advertising checklist
// as-is. The backend never invents/templates checklist items — the AI must
// emit them, and outputvalidator/bundlevalidator enforce presence when the
// Pack has advertising data.
func projectAdEfficiencyChecklist(in []contract.UnifiedAdEfficiencyChecklistItemV1) []dashboardhome.AdEfficiencyCheckItem {
	if len(in) == 0 {
		return nil
	}
	out := make([]dashboardhome.AdEfficiencyCheckItem, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.AdEfficiencyCheckItem{
			Title:  c.Title,
			Text:   c.Text,
			Target: c.Target,
		})
	}
	return out
}

// projectSalesFunnelFromFacts builds the factual sales_funnel
// stages/status from backend Dashboard Facts. Backend facts always win
// here; there is no AI-driven equivalent of this function. (The separate
// AI-owned funnel_analysis.ai_analysis narrative is projected by
// projectFunnelAnalysis.)
func projectSalesFunnelFromFacts(facts dashboardfacts.FunnelFacts) dashboardhome.SalesFunnelWidget {
	out := dashboardhome.SalesFunnelWidget{
		WidgetBase:            dashboardhome.WidgetBase{Status: widgetStatus(facts.Status)},
		CurrentCompletedDate:  facts.CurrentCompletedDate,
		PreviousCompletedDate: facts.PreviousCompletedDate,
		Stages:                make([]dashboardhome.FunnelStage, 0, len(facts.Stages)),
		DailySeries:           make([]dashboardhome.FunnelDailyPoint, 0, len(facts.DailySeries)),
		Overview7D: dashboardhome.FunnelWeeklyWidget{
			WidgetBase: dashboardhome.WidgetBase{Status: widgetStatus(facts.Overview7D.Status)},
			Stages:     make([]dashboardhome.FunnelWeeklyStage, 0, len(facts.Overview7D.Stages)),
		},
	}
	for _, s := range facts.Stages {
		out.Stages = append(out.Stages, dashboardhome.FunnelStage{
			Key:                            s.Key,
			Label:                          s.Label,
			Value:                          s.Value,
			ConversionFromPrevious:         s.ConversionFromPrevious,
			ConversionFromTop:              s.ConversionFromTop,
			PreviousDayValue:               s.PreviousDayValue,
			ChangeVsPreviousDayPct:         s.ChangeVsPreviousDayPct,
			PreviousConversionFromPrevious: s.PreviousConversionFromPrevious,
			ConversionDeltaPp:              s.ConversionDeltaPp,
			PeriodType:                     s.PeriodType,
		})
	}
	for _, point := range facts.DailySeries {
		out.DailySeries = append(out.DailySeries, dashboardhome.FunnelDailyPoint{
			Date:           point.Date,
			Impressions:    point.Impressions,
			ProductViews:   point.ProductViews,
			AddToCart:      point.AddToCart,
			OrderedUnits:   point.OrderedUnits,
			DeliveredUnits: point.DeliveredUnits,
		})
	}
	for _, stage := range facts.Overview7D.Stages {
		out.Overview7D.Stages = append(out.Overview7D.Stages, dashboardhome.FunnelWeeklyStage{
			Key:                            stage.Key,
			Label:                          stage.Label,
			Value:                          stage.Value,
			PreviousPeriodValue:            stage.PreviousPeriodValue,
			ChangeVsPreviousPeriodPct:      stage.ChangeVsPreviousPeriodPct,
			ConversionFromPrevious:         stage.ConversionFromPrevious,
			PreviousConversionFromPrevious: stage.PreviousConversionFromPrevious,
			ConversionDeltaPp:              stage.ConversionDeltaPp,
		})
	}
	return out
}

func projectFunnelAIAnalysis(in contract.UnifiedFunnelAIAnalysisV1) dashboardhome.FunnelAIAnalysisWidget {
	return dashboardhome.FunnelAIAnalysisWidget{
		WidgetBase:       dashboardhome.WidgetBase{Status: widgetStatus(in.Status), Message: in.Message},
		ShortSummary:     in.ShortSummary,
		TrendVsYesterday: in.TrendVsYesterday,
		TrendVs7D:        in.TrendVs7D,
		RelatedSignals:   append([]string{}, in.RelatedSignals...),
		RecommendedFocus: append([]string{}, in.RecommendedFocus...),
		FullAnalysis:     in.FullAnalysis,
		Summary:          in.Summary,
		Details:          in.Details,
		MainConclusion:   in.MainConclusion,
		ActionItems:      append([]string{}, in.ActionItems...),
	}
}

func projectFunnelAIAnalysisPtr(in contract.UnifiedFunnelAIAnalysisV1) *dashboardhome.FunnelAIAnalysisWidget {
	out := projectFunnelAIAnalysis(in)
	return &out
}

func projectFunnelAnalysis(in contract.UnifiedFunnelAnalysisV1) dashboardhome.FunnelAnalysisWidget {
	return dashboardhome.FunnelAnalysisWidget{
		WidgetBase:         dashboardhome.WidgetBase{Status: widgetStatus(in.Status), Message: in.Message},
		PrimaryDropStage:   in.PrimaryDropStage,
		PrimaryDropMessage: in.PrimaryDropMessage,
		AIAnalysis:         projectFunnelAIAnalysis(in.AIAnalysis),
	}
}

func projectRiskDistribution(in contract.UnifiedRiskDistributionV1) dashboardhome.RiskDistributionWidget {
	out := dashboardhome.RiskDistributionWidget{
		WidgetBase: dashboardhome.WidgetBase{Status: widgetStatus(in.Status), Message: in.Message},
		Total:      in.Total,
		Groups:     make([]dashboardhome.RiskGroupCount, 0, len(in.Groups)),
	}
	for _, g := range in.Groups {
		out.Groups = append(out.Groups, dashboardhome.RiskGroupCount{Group: g.Group, Label: g.Label, Count: g.Count, Severity: g.Severity})
	}
	return out
}

func projectRiskDistributionPtr(in contract.UnifiedRiskDistributionV1) *dashboardhome.RiskDistributionWidget {
	out := projectRiskDistribution(in)
	return &out
}

func projectCategorySummaries(in []contract.UnifiedCategorySummaryV1) []dashboardhome.CategorySummaryCard {
	out := make([]dashboardhome.CategorySummaryCard, 0, len(in))
	for _, c := range in {
		out = append(out, dashboardhome.CategorySummaryCard{
			CategoryID:        c.CategoryID,
			CategoryName:      c.CategoryName,
			CategoryScore:     c.CategoryScore,
			ScoreMax:          c.ScoreMax,
			HealthScore:       c.HealthScore,
			Severity:          c.Severity,
			Headline:          c.Headline,
			KeyRisk:           c.KeyRisk,
			RecommendedAction: c.RecommendedAction,
		})
	}
	return out
}

// projectCategorySummariesWithFacts merges AI category cards with backend
// CategoryFacts. Missing material categories get factual fallback cards so the
// UI never shows a single card with a large empty grid when several categories
// have sales. Backend ComputeCategoryScore overrides arbitrary AI scores.
func projectCategorySummariesWithFacts(ai []contract.UnifiedCategorySummaryV1, facts []dashboardfacts.CategoryFacts) []dashboardhome.CategorySummaryCard {
	projected := projectCategorySummaries(ai)
	if len(facts) == 0 {
		return projected
	}
	byID := make(map[string]dashboardhome.CategorySummaryCard, len(projected))
	order := make([]string, 0, len(facts))
	for _, c := range projected {
		id := strings.TrimSpace(c.CategoryID)
		if id == "" {
			id = strings.TrimSpace(c.CategoryName)
		}
		byID[id] = c
	}
	out := make([]dashboardhome.CategorySummaryCard, 0, len(facts))
	for _, f := range facts {
		id := fmt.Sprintf("%d", f.CategoryID)
		order = append(order, id)
		card, ok := byID[id]
		if !ok {
			card = factualCategoryCard(f)
		} else {
			score := int32(math.Round(dashboardfacts.ComputeCategoryScore(f)))
			card.CategoryScore = &score
			if card.ScoreMax == 0 {
				card.ScoreMax = 10
			}
			if strings.TrimSpace(card.CategoryName) == "" {
				card.CategoryName = f.CategoryName
			}
			if strings.TrimSpace(card.Headline) == "" || strings.TrimSpace(card.KeyRisk) == "" || strings.TrimSpace(card.RecommendedAction) == "" {
				fb := factualCategoryCard(f)
				if strings.TrimSpace(card.Headline) == "" {
					card.Headline = fb.Headline
				}
				if strings.TrimSpace(card.KeyRisk) == "" {
					card.KeyRisk = fb.KeyRisk
				}
				if strings.TrimSpace(card.RecommendedAction) == "" {
					card.RecommendedAction = fb.RecommendedAction
				}
				if strings.TrimSpace(card.Severity) == "" {
					card.Severity = fb.Severity
				}
			}
		}
		out = append(out, card)
	}
	_ = order
	return out
}

func factualCategoryCard(f dashboardfacts.CategoryFacts) dashboardhome.CategorySummaryCard {
	score := int32(math.Round(dashboardfacts.ComputeCategoryScore(f)))
	severity := "medium"
	switch {
	case score >= 8:
		severity = "low"
	case score >= 5:
		severity = "medium"
	case score >= 3:
		severity = "high"
	default:
		severity = "critical"
	}
	share := 0.0
	if f.RevenueShare != nil {
		share = *f.RevenueShare
	}
	headline := fmt.Sprintf("Формирует %.0f%% выручки", share)
	keyRisk := "Существенных рисков по категории не выявлено."
	action := "Держать текущую динамику и контролировать остатки."
	if f.StockRiskSKUCount > 0 {
		keyRisk = fmt.Sprintf("%d из %d SKU в зоне риска по остатку", f.StockRiskSKUCount, f.SKUCount)
		action = "Приоритизировать пополнение SKU с дефицитом в этой категории."
	}
	if f.PreviousRevenue > 0 && f.Revenue < f.PreviousRevenue*0.9 {
		keyRisk = fmt.Sprintf("Выручка дня ниже предыдущего дня на %.0f ₽", f.PreviousRevenue-f.Revenue)
		action = "Разобрать SKU с просадкой заказов внутри категории."
	}
	if f.SKUCount < 3 {
		keyRisk = "Недостаточно объёма данных по категории."
		action = "Наблюдать динамику; выводы по категории ограничены."
	}
	return dashboardhome.CategorySummaryCard{
		CategoryID:        fmt.Sprintf("%d", f.CategoryID),
		CategoryName:      f.CategoryName,
		CategoryScore:     &score,
		ScoreMax:          10,
		Severity:          severity,
		Headline:          headline,
		KeyRisk:           keyRisk,
		RecommendedAction: action,
	}
}

func projectHealthReasons(in []contract.UnifiedRiskDriverV1) []NarrativeHealthReason {
	out := make([]NarrativeHealthReason, 0, len(in))
	for _, d := range in {
		out = append(out, NarrativeHealthReason{
			DriverID: d.ID,
			Category: d.Category,
			Title:    d.Title,
			Reason:   d.Reason,
		})
	}
	return out
}

func projectImprovementLevers(in []contract.UnifiedImprovementLeverV2) []dashboardhome.ImprovementLever {
	out := make([]dashboardhome.ImprovementLever, 0, len(in))
	for _, lever := range in {
		title := strings.TrimSpace(lever.Title)
		explanation := strings.TrimSpace(lever.Explanation)
		if title == "" && explanation == "" {
			continue
		}
		out = append(out, dashboardhome.ImprovementLever{
			Title:             title,
			Explanation:       explanation,
			FactorID:          strings.TrimSpace(lever.FactorID),
			LinkedActionID:    strings.TrimSpace(lever.LinkedActionID),
			ExpectedDirection: strings.TrimSpace(lever.ExpectedDirection),
		})
	}
	return out
}

func projectWhatChanged(in contract.UnifiedWhatChangedV1) *NarrativeWhatChanged {
	return projectWhatChangedFromRegistry(dashboardfacts.DashboardFactsV1{}, nil, in, nil)
}

func projectWhatChangedFromRegistry(
	facts dashboardfacts.DashboardFactsV1,
	previous *dashboardfacts.DashboardFactsV1,
	in contract.UnifiedWhatChangedV1,
	actions []contract.UnifiedActionV1,
) *NarrativeWhatChanged {
	if strings.TrimSpace(in.Status) == "" && len(in.Groups) == 0 && strings.TrimSpace(in.Summary) == "" {
		return nil
	}

	prevCtx := weeklychangepolicy.PreviousContext{}
	// Fingerprints from AI previous_state / active claims help continuity when
	// a prior facts snapshot is not threaded into ProjectDashboard.
	var fingerprints []weeklychangepolicy.Problem
	for _, g := range in.Groups {
		key := strings.TrimSpace(g.Key)
		for _, it := range g.Items {
			metric := strings.TrimSpace(it.Metric)
			if metric == "" {
				metric = strings.TrimSpace(it.ChangeID)
			}
			if metric == "" {
				metric = strings.TrimSpace(it.SignalID)
			}
			if key == weeklychangepolicy.GroupKeyUnchangedRisks || strings.TrimSpace(it.PreviousState) != "" {
				fingerprints = append(fingerprints, weeklychangepolicy.Problem{
					MetricCode: metric,
					DriverID:   strings.TrimSpace(it.SignalID),
					EntityIDs:  append([]string{}, it.EntityIDs...),
					Present:    true,
				})
			}
		}
	}
	if len(fingerprints) > 0 {
		prevCtx = weeklychangepolicy.MergePreviousContext(prevCtx, weeklychangepolicy.BuildPreviousContextFromFingerprints("", fingerprints))
	}

	reg := weeklychangepolicy.BuildCandidateRegistry(facts, previous, prevCtx)
	actionRefs := make([]weeklychangepolicy.ActionRef, 0, len(actions))
	for _, a := range actions {
		actionRefs = append(actionRefs, weeklychangepolicy.ActionRef{
			AnalysisActionID: a.AnalysisActionID,
			Category:         a.Category,
			ScopeType:        a.Scope.ScopeType,
			EntityIDs:        append([]string{}, a.Scope.EntityIDs...),
			SourceSignalIDs:  append([]string{}, a.SourceSignalIDs...),
			Title:            a.Title,
		})
	}
	reg.Candidates = weeklychangepolicy.AttachActionLinks(reg.Candidates, actionRefs)
	// Rebuild ByChangeType after linking.
	byType := map[weeklychangepolicy.ChangeType][]weeklychangepolicy.ChangeCandidate{
		weeklychangepolicy.ChangeWorsened:      {},
		weeklychangepolicy.ChangeNewRisk:       {},
		weeklychangepolicy.ChangeActiveProblem: {},
		weeklychangepolicy.ChangeImproved:      {},
	}
	for _, c := range reg.Candidates {
		byType[c.ChangeType] = append(byType[c.ChangeType], c)
	}
	reg.ByChangeType = byType

	summary, mainConclusion, projected := weeklychangepolicy.BuildGroupsFromRegistry(reg, &in, 3)
	if strings.TrimSpace(summary) == "" {
		summary = strings.TrimSpace(in.Summary)
	}
	if strings.TrimSpace(mainConclusion) == "" {
		mainConclusion = strings.TrimSpace(in.MainConclusion)
	}
	// Sanitize campaign IDs from summary/conclusion.
	summary = weeklychangepolicy.StripNumericCampaignIDs(summary, nil)
	mainConclusion = weeklychangepolicy.StripNumericCampaignIDs(mainConclusion, nil)

	out := &NarrativeWhatChanged{
		Summary:        summary,
		MainConclusion: mainConclusion,
		Groups:         make([]NarrativeWhatChangedGroup, 0, len(projected)),
	}
	for _, g := range projected {
		group := NarrativeWhatChangedGroup{
			Key:       g.Key,
			Label:     g.Label,
			Tone:      g.Tone,
			Summary:   g.Summary,
			EmptyText: g.EmptyText,
			Items:     make([]NarrativeWhatChangedItem, 0, len(g.Items)),
		}
		for _, it := range g.Items {
			group.Items = append(group.Items, NarrativeWhatChangedItem{
				Label:             it.Title,
				Text:              it.Description,
				Highlights:        append([]string{}, it.Highlights...),
				Domain:            it.Domain,
				Severity:          it.Severity,
				CurrentValueText:  it.CurrentValueText,
				PreviousValueText: it.PreviousValueText,
				DeltaText:         it.DeltaText,
				ImpactText:        it.ImpactText,
				EntityCount:       it.EntityCount,
				EntityNames:       append([]string{}, it.EntityNames...),
				LinkedActionID:    it.LinkedActionID,
				ActionLinkLabel:   it.ActionLinkLabel,
				DeepLink:          it.DeepLink,
				ChangeID:          it.ChangeID,
				EvidenceFactIDs:   append([]string{}, it.EvidenceFactIDs...),
			})
		}
		if strings.TrimSpace(group.Summary) == "" && len(group.Items) > 0 {
			parts := make([]string, 0, len(group.Items))
			for _, it := range group.Items {
				label := strings.TrimSpace(it.Label)
				text := strings.TrimSpace(it.Text)
				switch {
				case label != "" && text != "":
					parts = append(parts, label+": "+text)
				case text != "":
					parts = append(parts, text)
				case label != "":
					parts = append(parts, label)
				}
			}
			if len(parts) > 0 {
				group.Summary = strings.Join(parts, " ")
			}
		}
		out.Groups = append(out.Groups, group)
	}

	// Fallback: if registry produced no items at all but AI had content, keep
	// sanitized AI groups (still strip campaign IDs / fix empty texts).
	totalItems := 0
	for _, g := range out.Groups {
		totalItems += len(g.Items)
	}
	if totalItems == 0 {
		return projectWhatChangedLegacySanitized(in, reg)
	}
	return out
}

func projectWhatChangedLegacySanitized(in contract.UnifiedWhatChangedV1, reg weeklychangepolicy.RegistryResult) *NarrativeWhatChanged {
	out := &NarrativeWhatChanged{
		Summary:        weeklychangepolicy.StripNumericCampaignIDs(in.Summary, nil),
		MainConclusion: weeklychangepolicy.StripNumericCampaignIDs(in.MainConclusion, nil),
		Groups:         make([]NarrativeWhatChangedGroup, 0, 4),
	}
	byKey := map[string]contract.UnifiedWhatChangedGroupV1{}
	for _, g := range in.Groups {
		byKey[strings.TrimSpace(g.Key)] = g
	}
	for _, key := range []string{
		weeklychangepolicy.GroupKeyWorsened,
		weeklychangepolicy.GroupKeyNewRisks,
		weeklychangepolicy.GroupKeyUnchangedRisks,
		weeklychangepolicy.GroupKeyImproved,
	} {
		g := byKey[key]
		empty := reg.EmptyTextByGroup[key]
		if empty == "" {
			empty = weeklychangepolicy.EmptyTextForGroup(key, reg.HistoryAvailable)
		}
		group := NarrativeWhatChangedGroup{
			Key:       key,
			Label:     weeklychangepolicy.DefaultGroupLabel(key),
			Tone:      weeklychangepolicy.DefaultGroupTone(key),
			Summary:   weeklychangepolicy.StripNumericCampaignIDs(g.Summary, nil),
			EmptyText: empty,
			Items:     make([]NarrativeWhatChangedItem, 0, len(g.Items)),
		}
		for _, it := range g.Items {
			names := append([]string{}, it.EntityNames...)
			group.Items = append(group.Items, NarrativeWhatChangedItem{
				Label:             weeklychangepolicy.StripNumericCampaignIDs(it.Label, names),
				Text:              weeklychangepolicy.StripNumericCampaignIDs(it.Text, names),
				Highlights:        append([]string{}, it.Highlights...),
				Domain:            it.Domain,
				Severity:          it.Severity,
				CurrentValueText:  it.CurrentValueText,
				PreviousValueText: it.PreviousValueText,
				DeltaText:         it.DeltaText,
				ImpactText:        weeklychangepolicy.StripNumericCampaignIDs(it.ImpactText, names),
				EntityCount:       it.EntityCount,
				EntityNames:       names,
				LinkedActionID:    it.LinkedActionID,
				ActionLinkLabel:   it.ActionLinkLabel,
				DeepLink:          it.DeepLink,
				ChangeID:          firstNonEmpty(it.ChangeID, it.SignalID),
			})
		}
		if len(group.Items) == 0 {
			group.Summary = empty
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}

// projectTopActions builds the narrative top_actions list from the unified
// actions array, ordered by dash.TopActionIDs (capped by contract.MaxTopActionsV1).
// Unresolved IDs are skipped (never artificially filled); bundlevalidator rejects
// unresolved / over-limit Top 3 before publish.
func projectTopActions(actions []contract.UnifiedActionV1, topActionIDs []string) []NarrativeTopAction {
	byID := make(map[string]contract.UnifiedActionV1, len(actions))
	for _, a := range actions {
		byID[a.AnalysisActionID] = a
	}
	limit := contract.MaxTopActionsV1
	if limit <= 0 {
		limit = 3
	}
	out := make([]NarrativeTopAction, 0, min(len(topActionIDs), limit))
	for _, id := range topActionIDs {
		if len(out) >= limit {
			break
		}
		a, ok := byID[id]
		if !ok {
			continue
		}
		nextStep := ""
		if len(a.Steps) > 0 {
			nextStep = a.Steps[0].Description
			if nextStep == "" {
				nextStep = a.Steps[0].Title
			}
		}
		out = append(out, NarrativeTopAction{
			DriverID:             a.AnalysisActionID,
			Priority:             a.Priority,
			Category:             a.Category,
			Title:                a.Title,
			Reason:               a.Problem,
			NextStep:             nextStep,
			ExpectedEffect:       a.ExpectedEffect,
			SupportingMetricRefs: []string{},
		})
	}
	return out
}

func mapSignalSeverityToTone(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "high", "negative":
		return "negative"
	case "risk":
		return "risk"
	case "positive", "low", "stable":
		return "positive"
	case "medium", "warning", "attention":
		return "risk"
	default:
		return "neutral"
	}
}
