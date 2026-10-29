package aisummaries

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ValidationOutcome struct {
	Payload          *SummaryPayload
	ValidationErrors []string
}

func ParseAndValidateSummaryJSON(raw string) (*ValidationOutcome, error) {
	return ParseAndValidateSummaryJSONWithScope(raw, "", "", "")
}

// ParseAndValidateSummaryJSONWithScope validates JSON and optional scope rules.
// summaryType: "account_daily_summary" | "category_daily_summary" | "" (skip scope checks).
// scopeID: category Ozon description_category_id as decimal string for category summaries.
// inferredCategory: output of classifyCategoryDataRichness ("low"|"medium"|"high") for category runs;
// when "low", enforces data_richness=low and summary_depth=short.
func ParseAndValidateSummaryJSONWithScope(raw, summaryType, scopeID, inferredCategory string) (*ValidationOutcome, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty json")
	}
	p, err := parseAndNormalizeSummaryJSONWithScope(raw, summaryType, scopeID, "")
	if err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	if strings.TrimSpace(p.DataRichness) == "" {
		if summaryType == "category_daily_summary" && inferredCategory == "low" {
			p.DataRichness = "low"
		} else {
			p.DataRichness = "medium"
		}
	}
	if strings.TrimSpace(p.SummaryDepth) == "" {
		if summaryType == "category_daily_summary" && inferredCategory == "low" {
			p.SummaryDepth = "short"
		} else {
			p.SummaryDepth = "normal"
		}
	}
	errs := validateSummaryPayload(p)
	if summaryType != "" {
		errs = append(errs, validateSummaryScope(summaryType, scopeID, p)...)
	}
	if summaryType == "category_daily_summary" && inferredCategory == "low" {
		errs = append(errs, validateLowDataCategoryOutput(p)...)
	}
	return &ValidationOutcome{Payload: p, ValidationErrors: errs}, nil
}

func validateSummaryScope(summaryType, scopeID string, p *SummaryPayload) []string {
	var errs []string
	switch summaryType {
	case "account_daily_summary":
		if p.CategoryID != nil {
			errs = append(errs, "account_summary_must_not_include_category_id")
		}
		if p.CategoryName != nil && strings.TrimSpace(*p.CategoryName) != "" {
			errs = append(errs, "account_summary_must_not_include_category_name")
		}
	case "category_daily_summary":
		if p.CategoryID == nil {
			errs = append(errs, "category_summary_missing_category_id")
		} else if strconv.FormatInt(*p.CategoryID, 10) != scopeID {
			errs = append(errs, "category_id_scope_mismatch")
		}
		if p.CategoryName == nil || strings.TrimSpace(*p.CategoryName) == "" {
			errs = append(errs, "category_summary_missing_category_name")
		}
	default:
		errs = append(errs, "unknown_summary_type_for_scope_validation")
	}
	return errs
}

func validateLowDataCategoryOutput(p *SummaryPayload) []string {
	var errs []string
	if p.DataRichness != "low" {
		errs = append(errs, "low_data_category_requires_data_richness_low")
	}
	if p.SummaryDepth != "short" {
		errs = append(errs, "low_data_category_requires_summary_depth_short")
	}
	return errs
}

func validateSummaryPayload(p *SummaryPayload) []string {
	var errs []string
	if p == nil {
		return []string{"nil payload"}
	}
	if strings.TrimSpace(p.Headline) == "" {
		errs = append(errs, "missing_headline")
	}
	if p.HealthScore < 0 || p.HealthScore > 100 {
		errs = append(errs, "health_score_out_of_range")
	}
	if !oneOf(p.Severity, "low", "medium", "high", "critical") {
		errs = append(errs, "invalid_severity")
	}
	if !oneOf(p.DataRichness, "low", "medium", "high") {
		errs = append(errs, "invalid_data_richness")
	}
	if !oneOf(p.SummaryDepth, "short", "normal", "deep") {
		errs = append(errs, "invalid_summary_depth")
	}
	if strings.TrimSpace(p.ExecutiveSummary) == "" {
		errs = append(errs, "missing_executive_summary")
	}
	if p.FunnelAIAnalysis != nil {
		funnel := p.FunnelAIAnalysis
		if !oneOf(funnel.Status, "available", "not_enough_data", "source_not_available", "source_pending", "error") {
			errs = append(errs, "invalid_funnel_ai_analysis_status")
		}
		if funnel.Status == "available" && !funnelHasStructuredPrimaryContent(funnel) {
			errs = append(errs, "funnel_ai_analysis_missing_primary_content")
		}
		if len([]rune(strings.TrimSpace(funnel.Summary))) > 700 {
			errs = append(errs, "funnel_ai_analysis_summary_too_long")
		}
		if len(funnel.DropFactors) > 5 {
			errs = append(errs, "funnel_ai_analysis_too_many_drop_factors")
		}
		for i, factor := range funnel.DropFactors {
			if strings.TrimSpace(factor.EvidenceLevel) != "" && !oneOf(factor.EvidenceLevel, "confirmed", "likely", "hypothesis", "limited") {
				errs = append(errs, fmt.Sprintf("funnel_drop_factor_%d_invalid_evidence_level", i))
			}
		}
		if funnel.WeakestStage != nil {
			if !oneOf(funnel.WeakestStage.Status, "available", "stable", "unavailable") {
				errs = append(errs, "funnel_ai_analysis_invalid_weakest_stage_status")
			}
			if strings.TrimSpace(funnel.WeakestStage.Tone) != "" && !oneOf(funnel.WeakestStage.Tone, "negative", "warning", "neutral", "positive") {
				errs = append(errs, "funnel_ai_analysis_invalid_weakest_stage_tone")
			}
		}
		if len(funnel.InfluenceFactors) > 5 {
			errs = append(errs, "funnel_ai_analysis_too_many_influence_factors")
		}
		if len(funnel.CheckDirections) > 3 {
			errs = append(errs, "funnel_ai_analysis_too_many_check_directions")
		}
	}
	for i, kf := range p.KeyFindings {
		if strings.TrimSpace(kf.Impact) != "" && !oneOf(kf.Impact, "low", "medium", "high") {
			errs = append(errs, fmt.Sprintf("key_finding_%d_invalid_impact", i))
		}
	}
	for i, c := range p.LikelyCauses {
		if strings.TrimSpace(c.Confidence) != "" && !oneOf(c.Confidence, "low", "medium", "high") {
			errs = append(errs, fmt.Sprintf("likely_cause_%d_invalid_confidence", i))
		}
	}
	for i, a := range p.RecommendedActions {
		if strings.TrimSpace(a.Priority) != "" && !oneOf(a.Priority, "low", "medium", "high", "critical") {
			errs = append(errs, fmt.Sprintf("action_%d_invalid_priority", i))
		}
		if strings.TrimSpace(a.RelatedEntityType) != "" && !oneOf(a.RelatedEntityType, "account", "category", "sku", "campaign", "alert", "recommendation") {
			errs = append(errs, fmt.Sprintf("action_%d_invalid_related_entity_type", i))
		}
	}
	for _, w := range p.Watchlist {
		if strings.TrimSpace(w.EntityType) != "" && !oneOf(w.EntityType, "sku", "category", "campaign", "metric") {
			errs = append(errs, "invalid_watchlist_entity_type")
			break
		}
	}
	if hasNaNInfJSON(p) {
		errs = append(errs, "nan_or_inf_in_payload")
	}
	return errs
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func hasNaNInfJSON(p *SummaryPayload) bool {
	b, err := json.Marshal(p)
	if err != nil {
		return true
	}
	s := string(b)
	return strings.Contains(s, "NaN") || strings.Contains(s, "Infinity") || strings.Contains(s, "-Infinity")
}

func IsValidationClean(out *ValidationOutcome) bool {
	return out != nil && len(out.ValidationErrors) == 0
}

// SanitizeForPriceAds applies product guardrails on free-text fields (MVP string scan).
func SanitizeForPriceAds(p *SummaryPayload) []string {
	var warns []string
	if p == nil {
		return warns
	}
	combined := strings.ToLower(p.ExecutiveSummary + " " + p.Headline)
	for _, a := range p.RecommendedActions {
		combined += " " + strings.ToLower(a.Action) + " " + strings.ToLower(a.Reason)
	}
	if strings.Contains(combined, "цен") && (strings.Contains(combined, "сниз") || strings.Contains(combined, "повыс") || strings.Contains(combined, "price")) {
		warns = append(warns, "possible_price_action_language_detected")
	}
	return warns
}

func numericFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func funnelHasStructuredPrimaryContent(f *FunnelAIAnalysis) bool {
	if f == nil {
		return false
	}
	if f.WeakestStage != nil && strings.TrimSpace(f.WeakestStage.Status) != "" {
		return true
	}
	if strings.TrimSpace(f.MainConclusion) != "" {
		return true
	}
	if len(f.DropFactors) > 0 || len(f.CheckDirections) > 0 || len(f.DetailsSections) > 0 {
		return true
	}
	if len(f.InfluenceFactors) > 0 {
		return true
	}
	return strings.TrimSpace(f.Summary) != "" ||
		strings.TrimSpace(f.ShortSummary) != "" ||
		strings.TrimSpace(f.FullAnalysis) != "" ||
		strings.TrimSpace(f.Details) != ""
}
