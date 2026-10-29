package dashboardhome

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

const (
	DashboardNarrativePromptVersion       = "dashboard_narrative_v9_ai_only_chunks"
	DashboardNarrativePromptVersionLegacy = "dashboard_narrative_v8_datapack_chunks"
)

type dashboardNarrativePayload struct {
	Headline               string                           `json:"headline"`
	ExecutiveSummary       string                           `json:"executive_summary"`
	HealthReasons          []dashboardNarrativeHealthReason `json:"health_reasons"`
	ImprovementSummary     string                           `json:"improvement_summary"`
	TopActions             []dashboardNarrativeTopAction    `json:"top_actions"`
	RecommendedActions     []ActionItem                     `json:"recommended_actions,omitempty"`
	WhatChanged            *dashboardNarrativeWhatChanged   `json:"what_changed,omitempty"`
	DataQualityNotes       []string                         `json:"data_quality_notes"`
	StockCoverageNarrative *StockCoverageNarrative          `json:"stock_coverage_narrative,omitempty"`
	AdEfficiencyNarrative  *AdEfficiencyNarrative           `json:"ad_efficiency_narrative,omitempty"`
	SellerPulseNarrative   *SellerPulseNarrative            `json:"seller_pulse_narrative,omitempty"`
	FunnelAIAnalysis       *FunnelAIAnalysisWidget          `json:"funnel_ai_analysis,omitempty"`
	CategorySummaries      []CategorySummaryCard            `json:"category_summaries,omitempty"`
	RiskOverview           *RiskDistributionWidget          `json:"risk_overview,omitempty"`
	AccountHealth          *dashboardAccountHealthBlock     `json:"account_health,omitempty"`
}

type dashboardNarrativeHealthReason struct {
	DriverID string   `json:"driver_id"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Reason   string   `json:"reason"`
	FactIDs  []string `json:"fact_ids,omitempty"`
}

type dashboardNarrativeTopAction struct {
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

type dashboardNarrativeMetric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type dashboardNarrativeCandidate struct {
	ID                      string                     `json:"id"`
	Source                  string                     `json:"source"`
	Priority                string                     `json:"priority"`
	Category                string                     `json:"category"`
	CategoryLabel           string                     `json:"category_label"`
	Title                   string                     `json:"title"`
	Reason                  string                     `json:"reason"`
	NextStep                string                     `json:"next_step"`
	CompatibleDriverIDs     []string                   `json:"compatible_driver_ids,omitempty"`
	LinkedAlertIDs          []int64                    `json:"linked_alert_ids,omitempty"`
	LinkedRecommendationIDs []int64                    `json:"linked_recommendation_ids,omitempty"`
	SupportingMetrics       []dashboardNarrativeMetric `json:"supporting_metrics,omitempty"`
}

type dashboardNarrativeRiskDriverView struct {
	ID                      string         `json:"id"`
	Category                string         `json:"category"`
	CategoryLabel           string         `json:"category_label"`
	Severity                string         `json:"severity"`
	ImpactScore             int            `json:"impact_score"`
	Title                   string         `json:"title"`
	Reason                  string         `json:"reason"`
	Evidence                map[string]any `json:"evidence,omitempty"`
	LinkedAlertIDs          []int64        `json:"linked_alert_ids,omitempty"`
	LinkedRecommendationIDs []int64        `json:"linked_recommendation_ids,omitempty"`
}

type dashboardNarrativeContext struct {
	MetricDate               string                               `json:"metric_date"`
	HealthScore              int32                                `json:"health_score"`
	Severity                 string                               `json:"severity"`
	RiskDrivers              []dashboardNarrativeRiskDriverView   `json:"risk_drivers"`
	RecommendationCandidates []dashboardNarrativeCandidate        `json:"recommendation_candidates"`
	Alerts                   []map[string]any                     `json:"alerts"`
	SupportingMetricsCatalog map[string]string                    `json:"supporting_metrics_catalog"`
	SalesSummary             map[string]any                       `json:"sales_summary"`
	CancellationsSummary     map[string]any                       `json:"cancellations_summary"`
	FunnelSummary            map[string]any                       `json:"funnel_summary"`
	StockRisks               map[string]any                       `json:"stock_risks"`
	AdvertisingEfficiency    map[string]any                       `json:"advertising_efficiency"`
	AdEfficiencyContext      map[string]any                       `json:"ad_efficiency_context"`
	SupportingMetrics        map[string]any                       `json:"supporting_metrics"`
	DataFreshness            map[string]any                       `json:"data_freshness"`
	WhatChangedContext       dashboardNarrativeWhatChangedContext `json:"what_changed_context"`
	StockCoverageContext     map[string]any                       `json:"stock_coverage_context"`
	SellerPulseContext       map[string]any                       `json:"seller_pulse_context"`
	Constraints              map[string]bool                      `json:"constraints"`
}

type dashboardNarrativeBuildResult struct {
	Context                       dashboardNarrativeContext
	ContextSource                 string
	AIContext                     any    // v8 datapack context or nil (legacy uses Context)
	ContextHash                   string // rendered context hash (debug only)
	FactualContextHash            string // stable facts hash used for freshness
	RiskDriversHash               string
	CandidateByID                 map[string]dashboardNarrativeCandidate
	MetricCatalogByKey            map[string]dashboardNarrativeMetric
	AllowedQuotedNames            map[string]struct{}
	WhatChanged                   whatChangedValidationBundle
	StockNarrativeCtx             stockCoverageNarrativeValidationContext
	StockNarrativeRequired        bool
	AdEfficiencyNarrativeCtx      adEfficiencyNarrativeValidationContext
	AdEfficiencyNarrativeRequired bool
	SellerPulseCtx                sellerPulseNarrativeValidationContext
	SellerPulseRequired           bool
	DataPackFactIDs               []string
	FinanceAvailable              bool
	PriceConstraintsAvailable     bool
	DataPackAuditValid            bool
	GlobalSemanticContext         *aicontext.GlobalSemanticContext
	GlobalContextHash             string
	DataPackHash                  string
}

type storedDashboardNarrative struct {
	ID                  string
	MetricDate          string
	Status              string
	PromptVersion       string
	ContextHash         string // stored factual context hash
	RenderedContextHash string // stored rendered context hash (risk_drivers_hash column)
	RiskDriversHash     string // legacy risk drivers hash for old rows
	AiPipelineRunID     int64
	DataPackHash        string
	GlobalContextHash   string
	Payload             *dashboardNarrativePayload
	ValidationErrors    []string
}

// NarrativeDebug is returned in GET /dashboard/home when DASHBOARD_NARRATIVE_DEBUG=1.
type NarrativeDebug struct {
	SelectedRunID              string   `json:"selected_run_id,omitempty"`
	RowStatus                  string   `json:"row_status,omitempty"`
	FreshnessDecision          string   `json:"freshness_decision,omitempty"`
	FreshnessReason            string   `json:"freshness_reason,omitempty"`
	HardValidationErrors       []string `json:"hard_validation_errors,omitempty"`
	RepairableValidationErrors []string `json:"repairable_validation_errors,omitempty"`
	RepairActions              []string `json:"repair_actions,omitempty"`
	AIActionsReceived          int      `json:"ai_actions_received,omitempty"`
	AIActionsAccepted          int      `json:"ai_actions_accepted,omitempty"`
	FallbackActionsFilled      int      `json:"fallback_actions_filled,omitempty"`
	DroppedActions             int      `json:"dropped_actions,omitempty"`
	PartialBlocks              []string `json:"partial_blocks,omitempty"`
	FallbackBlocks             []string `json:"fallback_blocks,omitempty"`
	FallbackReason             string   `json:"fallback_reason,omitempty"`
	StoredFactualContextHash   string   `json:"stored_factual_context_hash,omitempty"`
	CurrentFactualContextHash  string   `json:"current_factual_context_hash,omitempty"`
	StoredRenderedContextHash  string   `json:"stored_rendered_context_hash,omitempty"`
	CurrentRenderedContextHash string   `json:"current_rendered_context_hash,omitempty"`
	ContextSource              string   `json:"context_source,omitempty"`
	PromptVersion              string   `json:"prompt_version,omitempty"`
	FactCount                  int      `json:"fact_count,omitempty"`
	DataPackAuditValid         bool     `json:"data_pack_audit_valid,omitempty"`
}

type dashboardNarrativeValidationContext struct {
	DriversByID               map[string]RiskDriver
	CandidatesByID            map[string]dashboardNarrativeCandidate
	MetricsByKey              map[string]dashboardNarrativeMetric
	AllowedQuotedName         map[string]struct{}
	WhatChanged               whatChangedValidationBundle
	Stock                     *stockCoverageNarrativeValidationContext
	StockRequired             bool
	AdEfficiency              *adEfficiencyNarrativeValidationContext
	AdEfficiencyRequired      bool
	SellerPulse               *sellerPulseNarrativeValidationContext
	SellerPulseRequired       bool
	ContextSource             string
	DataPackFactIDs           map[string]struct{}
	FinanceAvailable          bool
	PriceConstraintsAvailable bool
	AllowedFormattedValues    map[string]struct{}
}

func parseDashboardNarrativePayload(raw string) (*dashboardNarrativePayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty dashboard narrative json")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}
	if spRaw, ok := root["seller_pulse_narrative"]; ok && sellerPulseNarrativeLooksNested(string(spRaw)) {
		if unwrapped := unwrapSellerPulseNarrativeRaw(spRaw); unwrapped != nil {
			if encoded, err := json.Marshal(unwrapped); err == nil {
				root["seller_pulse_narrative"] = encoded
			}
		}
	}
	coerceStringNarrativeObject(root, "ad_efficiency_narrative", "summary")
	coerceStringNarrativeObject(root, "stock_coverage_narrative", "summary")
	normalizeDashboardPayloadRoot(root)
	normalized, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	var payload dashboardNarrativePayload
	if err := json.Unmarshal(normalized, &payload); err != nil {
		return nil, err
	}
	payload.Headline = cleanupSellerFacingText(payload.Headline)
	payload.ExecutiveSummary = normalizeSellerFacingSentences(cleanupSellerFacingText(payload.ExecutiveSummary))
	payload.ImprovementSummary = cleanupSellerFacingText(payload.ImprovementSummary)
	payload.DataQualityNotes = compactStringSlice(payload.DataQualityNotes)
	if payload.HealthReasons == nil {
		payload.HealthReasons = []dashboardNarrativeHealthReason{}
	}
	for i := range payload.HealthReasons {
		payload.HealthReasons[i].DriverID = strings.TrimSpace(payload.HealthReasons[i].DriverID)
		payload.HealthReasons[i].Category = strings.ToLower(strings.TrimSpace(payload.HealthReasons[i].Category))
		payload.HealthReasons[i].Title = cleanupSellerFacingText(payload.HealthReasons[i].Title)
		payload.HealthReasons[i].Reason = cleanupSellerFacingText(payload.HealthReasons[i].Reason)
	}
	if payload.TopActions == nil {
		payload.TopActions = []dashboardNarrativeTopAction{}
	}
	if dashboardAIOnlyEnabled() && payload.WhatChanged != nil {
		payload.WhatChanged = ensureWhatChangedRequiredGroups(payload.WhatChanged, whatChangedValidationBundle{}, nil)
	}
	payload.WhatChanged = normalizeDashboardNarrativeWhatChanged(payload.WhatChanged)
	for i := range payload.TopActions {
		payload.TopActions[i].DriverID = strings.TrimSpace(payload.TopActions[i].DriverID)
		payload.TopActions[i].SourceCandidateID = strings.TrimSpace(payload.TopActions[i].SourceCandidateID)
		payload.TopActions[i].Priority = normalizeTopActionPriority(payload.TopActions[i].Priority)
		payload.TopActions[i].Category = strings.ToLower(strings.TrimSpace(payload.TopActions[i].Category))
		payload.TopActions[i].Title = cleanupSellerFacingText(payload.TopActions[i].Title)
		payload.TopActions[i].Reason = cleanupSellerFacingText(payload.TopActions[i].Reason)
		payload.TopActions[i].NextStep = cleanupSellerFacingText(payload.TopActions[i].NextStep)
		payload.TopActions[i].ExpectedEffect = cleanupSellerFacingText(payload.TopActions[i].ExpectedEffect)
		payload.TopActions[i].SupportingMetricRefs = compactStringSlice(payload.TopActions[i].SupportingMetricRefs)
	}
	normalizeStockCoverageNarrativePayload(payload.StockCoverageNarrative)
	normalizeAdEfficiencyNarrativePayload(payload.AdEfficiencyNarrative)
	normalizeSellerPulseNarrativePayload(payload.SellerPulseNarrative)
	return &payload, nil
}

func compactStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func normalizeSellerFacingSentences(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	capitalizeAt := func(index int) {
		if index < 0 || index >= len(runes) || !unicode.IsLetter(runes[index]) {
			return
		}
		runes[index] = unicode.ToUpper(runes[index])
	}
	capitalizeAt(0)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '.' {
			continue
		}
		j := i + 1
		for j < len(runes) && unicode.IsSpace(runes[j]) {
			j++
		}
		capitalizeAt(j)
	}
	return string(runes)
}
