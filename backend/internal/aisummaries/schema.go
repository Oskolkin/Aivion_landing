package aisummaries

// SummaryPayload mirrors the structured JSON contract for account and category daily summaries.
type SummaryPayload struct {
	Headline          string            `json:"headline"`
	HealthScore       int               `json:"health_score"`
	Severity          string            `json:"severity"`
	DataRichness      string            `json:"data_richness"`
	SummaryDepth      string            `json:"summary_depth"`
	ExecutiveSummary  string            `json:"executive_summary"`
	KeyRisk           string            `json:"key_risk,omitempty"`
	RecommendedAction string            `json:"recommended_action,omitempty"`
	FunnelAIAnalysis  *FunnelAIAnalysis `json:"funnel_ai_analysis,omitempty"`

	CategoryID   *int64  `json:"category_id,omitempty"`
	CategoryName *string `json:"category_name,omitempty"`

	WhatChangedSincePreviousSummary struct {
		Improved       []string `json:"improved"`
		Worsened       []string `json:"worsened"`
		UnchangedRisks []string `json:"unchanged_risks"`
		ResolvedRisks  []string `json:"resolved_risks"`
		NewRisks       []string `json:"new_risks"`
	} `json:"what_changed_since_previous_summary"`

	KeyFindings []struct {
		Title           string `json:"title"`
		Metric          string `json:"metric"`
		CurrentValue    any    `json:"current_value"`
		ComparisonValue any    `json:"comparison_value"`
		Delta           string `json:"delta"`
		Evidence        string `json:"evidence"`
		Impact          string `json:"impact"`
		Interpretation  string `json:"interpretation"`
	} `json:"key_findings"`

	LikelyCauses []struct {
		Cause        string `json:"cause"`
		Confidence   string `json:"confidence"`
		Evidence     string `json:"evidence"`
		IsHypothesis bool   `json:"is_hypothesis"`
	} `json:"likely_causes"`

	RecommendedActions []struct {
		Priority          string  `json:"priority"`
		Action            string  `json:"action"`
		Reason            string  `json:"reason"`
		ExpectedEffect    string  `json:"expected_effect"`
		RelatedEntityType string  `json:"related_entity_type"`
		RelatedEntityID   *string `json:"related_entity_id"`
	} `json:"recommended_actions"`

	Watchlist []struct {
		EntityType     string `json:"entity_type"`
		EntityID       string `json:"entity_id"`
		Name           string `json:"name"`
		Reason         string `json:"reason"`
		MetricSnapshot string `json:"metric_snapshot"`
	} `json:"watchlist"`

	LinkedAlertIDs          []int64  `json:"linked_alert_ids"`
	LinkedRecommendationIDs []int64  `json:"linked_recommendation_ids"`
	DataQualityNotes        []string `json:"data_quality_notes"`
}

type FunnelAIAnalysis struct {
	Status string `json:"status,omitempty"`
	// Legacy compact narrative (optional; compact UI should not depend on summary).
	ShortSummary     string   `json:"short_summary,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	TrendVsYesterday string   `json:"trend_vs_yesterday,omitempty"`
	TrendVs7D        string   `json:"trend_vs_7d,omitempty"`
	RelatedSignals   []string `json:"related_signals"`
	RecommendedFocus []string `json:"recommended_focus"`
	FullAnalysis     string   `json:"full_analysis,omitempty"`
	Details          string   `json:"details,omitempty"`

	WeakestStage          *FunnelWeakestStage          `json:"weakest_stage,omitempty"`
	MainConclusion        string                       `json:"main_conclusion,omitempty"`
	ActionItems           []string                     `json:"action_items,omitempty"`
	Comparisons           *FunnelComparisons           `json:"comparisons,omitempty"`
	ComparisonsCommentary *FunnelComparisonsCommentary `json:"comparisons_commentary,omitempty"`
	// Optional legacy fields; no longer requested in new AI prompts (UI removed).
	DropFactors     []FunnelDropFactor     `json:"drop_factors,omitempty"`
	CheckDirections []FunnelTextItem       `json:"check_directions,omitempty"`
	DetailsSections []FunnelDetailsSection `json:"details_sections,omitempty"`
	// Legacy alias; prefer drop_factors.
	InfluenceFactors []FunnelTextItem `json:"influence_factors"`
}

type FunnelWeakestStage struct {
	Status        string `json:"status,omitempty"`
	Key           string `json:"key,omitempty"`
	Label         string `json:"label,omitempty"`
	Metric        string `json:"metric,omitempty"`
	Value         string `json:"value,omitempty"`
	Explanation   string `json:"explanation,omitempty"`
	Reason        string `json:"reason,omitempty"`
	DeltaPrevious string `json:"delta_previous,omitempty"`
	Delta7D       string `json:"delta_7d,omitempty"`
	Tone          string `json:"tone,omitempty"`
}

type FunnelComparisonsCommentary struct {
	PreviousPeriod  string `json:"previous_period,omitempty"`
	SevenDayAverage string `json:"seven_day_average,omitempty"`
}

type FunnelDropFactor struct {
	Key           string `json:"key,omitempty"`
	Label         string `json:"label,omitempty"`
	Group         string `json:"group,omitempty"`
	EvidenceLevel string `json:"evidence_level,omitempty"`
	Metric        string `json:"metric,omitempty"`
	Delta         string `json:"delta,omitempty"`
	Explanation   string `json:"explanation,omitempty"`
	Limitation    string `json:"limitation,omitempty"`
	Tone          string `json:"tone,omitempty"`
	EvidenceKey   string `json:"evidence_key,omitempty"`
}

type FunnelDetailsSectionItem struct {
	Text    string   `json:"text,omitempty"`
	Metrics []string `json:"metrics,omitempty"`
}

type FunnelDetailsSection struct {
	Title string                     `json:"title,omitempty"`
	Items []FunnelDetailsSectionItem `json:"items,omitempty"`
}

type FunnelComparisonItem struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
	Delta string `json:"delta,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

type FunnelComparisons struct {
	PreviousPeriod    []FunnelComparisonItem `json:"previous_period"`
	SevenDayAverage   []FunnelComparisonItem `json:"seven_day_average"`
	StageMetrics      []FunnelComparisonItem `json:"stage_metrics,omitempty"`
	ConversionMetrics []FunnelComparisonItem `json:"conversion_metrics,omitempty"`
	BusinessMetrics   []FunnelComparisonItem `json:"business_metrics,omitempty"`
	ContextSignals    []FunnelComparisonItem `json:"context_signals,omitempty"`
}

type FunnelTextItem struct {
	Key        string `json:"key,omitempty"`
	Label      string `json:"label,omitempty"`
	Priority   string `json:"priority,omitempty"`
	DataNeeded string `json:"data_needed,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Tone       string `json:"tone,omitempty"`
	Text       string `json:"text,omitempty"`
}
