package aisummaries

import (
	"fmt"
	"strconv"
	"strings"
)

type SummaryRepairContext struct {
	SummaryType      string
	ScopeID          string
	CategoryName     string
	InferredCategory string
}

type SummaryRepairResult struct {
	Repaired         bool
	Partial          bool
	HardErrors       []string
	RepairableErrors []string
	RepairActions    []string
	FallbackBlocks   []string
	DroppedActions   int
}

func repairSummaryPayload(payload *SummaryPayload, ctx SummaryRepairContext) SummaryRepairResult {
	result := SummaryRepairResult{}
	if payload == nil {
		result.HardErrors = append(result.HardErrors, "nil_payload")
		return result
	}

	if payload.HealthScore < 0 || payload.HealthScore > 100 {
		old := payload.HealthScore
		if payload.HealthScore < 0 {
			payload.HealthScore = 0
		} else {
			payload.HealthScore = 100
		}
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, fmt.Sprintf("health_score_clamped_from_%d", old))
	}

	if strings.TrimSpace(payload.Severity) == "" || !oneOf(payload.Severity, "low", "medium", "high", "critical") {
		payload.Severity = "medium"
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "severity_defaulted_medium")
	}
	if strings.TrimSpace(payload.DataRichness) == "" || !oneOf(payload.DataRichness, "low", "medium", "high") {
		payload.DataRichness = "medium"
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "data_richness_defaulted_medium")
	}
	if strings.TrimSpace(payload.SummaryDepth) == "" || !oneOf(payload.SummaryDepth, "short", "normal", "deep") {
		payload.SummaryDepth = "normal"
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "summary_depth_defaulted_normal")
	}

	if strings.TrimSpace(payload.Headline) == "" && strings.TrimSpace(payload.ExecutiveSummary) != "" {
		payload.Headline = truncateRunes(payload.ExecutiveSummary, 120)
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "headline_filled_from_executive_summary")
	}
	if strings.TrimSpace(payload.ExecutiveSummary) == "" && strings.TrimSpace(payload.Headline) != "" {
		payload.ExecutiveSummary = payload.Headline
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "executive_summary_filled_from_headline")
	}

	switch ctx.SummaryType {
	case "account_daily_summary":
		payload.CategoryID = nil
		payload.CategoryName = nil
	case "category_daily_summary":
		result = mergeRepairResult(result, repairCategorySummaryFields(payload, ctx))
	}

	result = mergeRepairResult(result, repairRecommendedActionsList(payload))
	if payload.FunnelAIAnalysis != nil {
		result = mergeRepairResult(result, repairFunnelAIAnalysis(payload.FunnelAIAnalysis))
	}

	if ctx.SummaryType == "category_daily_summary" && ctx.InferredCategory == "low" {
		if payload.DataRichness != "low" {
			payload.DataRichness = "low"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "low_data_category_data_richness_forced_low")
		}
		if payload.SummaryDepth != "short" {
			payload.SummaryDepth = "short"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "low_data_category_summary_depth_forced_short")
		}
	}

	return result
}

func repairCategorySummaryFields(payload *SummaryPayload, ctx SummaryRepairContext) SummaryRepairResult {
	result := SummaryRepairResult{}
	if ctx.ScopeID != "" {
		if id, err := strconv.ParseInt(ctx.ScopeID, 10, 64); err == nil {
			if payload.CategoryID == nil || *payload.CategoryID != id {
				payload.CategoryID = &id
				result.Repaired = true
				result.RepairActions = append(result.RepairActions, "category_id_aligned_to_scope")
			}
		}
	}
	name := strings.TrimSpace(ctx.CategoryName)
	if name == "" && payload.CategoryName != nil {
		name = strings.TrimSpace(*payload.CategoryName)
	}
	if name == "" {
		name = "Категория без названия"
		result.RepairableErrors = append(result.RepairableErrors, "category_name_missing_used_default")
	}
	payload.CategoryName = &name

	if strings.TrimSpace(payload.KeyRisk) == "" {
		if len(payload.WhatChangedSincePreviousSummary.NewRisks) > 0 {
			payload.KeyRisk = payload.WhatChangedSincePreviousSummary.NewRisks[0]
		} else if len(payload.LikelyCauses) > 0 {
			payload.KeyRisk = payload.LikelyCauses[0].Cause
		} else if len(payload.KeyFindings) > 0 {
			payload.KeyRisk = firstNonEmptyString(payload.KeyFindings[0].Title, payload.KeyFindings[0].Evidence)
		} else {
			payload.KeyRisk = fmt.Sprintf("Требует внимания: %s", name)
		}
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "key_risk_filled_from_context")
	}

	if strings.TrimSpace(payload.RecommendedAction) == "" {
		if len(payload.RecommendedActions) > 0 && strings.TrimSpace(payload.RecommendedActions[0].Action) != "" {
			payload.RecommendedAction = payload.RecommendedActions[0].Action
		} else {
			payload.RecommendedAction = fmt.Sprintf("Проверьте динамику и риски в категории «%s».", name)
		}
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "recommended_action_filled_from_context")
	}

	if strings.TrimSpace(payload.Headline) == "" {
		payload.Headline = fmt.Sprintf("Категория «%s»: ключевые риски и действия", name)
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "category_headline_filled")
	}

	return result
}

func repairRecommendedActionsList(payload *SummaryPayload) SummaryRepairResult {
	result := SummaryRepairResult{}
	if payload == nil {
		return result
	}
	kept := payload.RecommendedActions[:0]
	for i := range payload.RecommendedActions {
		action := &payload.RecommendedActions[i]
		action.Action = strings.TrimSpace(action.Action)
		if action.Action == "" {
			result.DroppedActions++
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("action_%d_empty", i))
			continue
		}
		combined := strings.ToLower(action.Action + " " + action.Reason)
		if isForbiddenRecommendedAction(combined) {
			result.DroppedActions++
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("action_%d_forbidden_recommendation", i))
			continue
		}
		if strings.TrimSpace(action.Priority) != "" && !oneOf(action.Priority, "low", "medium", "high", "critical") {
			action.Priority = "medium"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, fmt.Sprintf("action_%d_priority_defaulted", i))
		}
		if action.Priority == "" {
			action.Priority = "medium"
		}
		if strings.TrimSpace(action.RelatedEntityType) != "" && !oneOf(action.RelatedEntityType, "account", "category", "sku", "campaign", "alert", "recommendation") {
			action.RelatedEntityType = "account"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, fmt.Sprintf("action_%d_entity_type_defaulted", i))
		}
		if action.RelatedEntityType == "" {
			action.RelatedEntityType = "account"
		}
		if strings.TrimSpace(action.ExpectedEffect) == "" {
			action.ExpectedEffect = "Снизит риск и поможет стабилизировать показатели."
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, fmt.Sprintf("action_%d_expected_effect_filled", i))
		}
		kept = append(kept, *action)
	}
	payload.RecommendedActions = kept
	if result.DroppedActions > 0 {
		result.Partial = true
	}
	return result
}

func repairFunnelAIAnalysis(funnel *FunnelAIAnalysis) SummaryRepairResult {
	result := SummaryRepairResult{}
	if funnel == nil {
		return result
	}

	if strings.TrimSpace(funnel.Status) == "" {
		funnel.Status = "available"
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_status_defaulted_available")
	}
	if !oneOf(funnel.Status, "available", "not_enough_data", "source_not_available", "source_pending", "error") {
		funnel.Status = "available"
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_status_normalized")
	}

	if funnel.WeakestStage != nil {
		ws := funnel.WeakestStage
		if strings.TrimSpace(ws.Status) != "" && !oneOf(ws.Status, "available", "stable", "unavailable") {
			ws.Status = "available"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "funnel_weakest_stage_status_normalized")
		}
		if strings.TrimSpace(ws.Tone) != "" && !oneOf(ws.Tone, "negative", "warning", "neutral", "positive") {
			ws.Tone = "neutral"
			result.Repaired = true
		}
	}

	if len(funnel.DropFactors) > 5 {
		funnel.DropFactors = funnel.DropFactors[:5]
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_drop_factors_truncated")
	}
	cleanFactors := make([]FunnelDropFactor, 0, len(funnel.DropFactors))
	for i, factor := range funnel.DropFactors {
		if strings.TrimSpace(factor.Label) == "" && strings.TrimSpace(factor.Explanation) == "" {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("funnel_drop_factor_%d_empty", i))
			continue
		}
		if strings.TrimSpace(factor.EvidenceLevel) != "" && !oneOf(factor.EvidenceLevel, "confirmed", "likely", "hypothesis", "limited") {
			factor.EvidenceLevel = "limited"
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, fmt.Sprintf("funnel_drop_factor_%d_evidence_level_repaired", i))
		}
		cleanFactors = append(cleanFactors, factor)
	}
	funnel.DropFactors = cleanFactors

	if len(funnel.CheckDirections) > 3 {
		funnel.CheckDirections = funnel.CheckDirections[:3]
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_check_directions_truncated")
	}

	funnel.DetailsSections = reorderFunnelDetailsSections(funnel.DetailsSections)
	if len(funnel.DetailsSections) == 0 && funnelHasStructuredPrimaryContent(funnel) {
		if text := firstNonEmptyString(funnel.Summary, funnel.ShortSummary, funnel.FullAnalysis); text != "" {
			funnel.DetailsSections = []FunnelDetailsSection{{Title: "Анализ воронки", Items: []FunnelDetailsSectionItem{{Text: text}}}}
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "funnel_details_sections_filled_from_summary")
		}
	}

	if funnel.Comparisons == nil {
		funnel.Comparisons = &FunnelComparisons{
			PreviousPeriod:  []FunnelComparisonItem{},
			SevenDayAverage: []FunnelComparisonItem{},
		}
	}

	if funnel.Status == "available" && !funnelHasStructuredPrimaryContent(funnel) {
		funnel.Status = "not_enough_data"
		result.Partial = true
		result.FallbackBlocks = append(result.FallbackBlocks, "funnel_ai_analysis_primary_content")
		result.RepairableErrors = append(result.RepairableErrors, "funnel_ai_analysis_missing_primary_content")
	}

	result = mergeRepairResult(result, repairFunnelMainConclusion(funnel))

	return result
}

func reorderFunnelDetailsSections(sections []FunnelDetailsSection) []FunnelDetailsSection {
	if len(sections) <= 1 {
		return sections
	}
	order := []string{"основные потери", "конверсия", "сигналы", "рекомендации", "детали"}
	byTitle := make(map[string]FunnelDetailsSection, len(sections))
	rest := make([]FunnelDetailsSection, 0, len(sections))
	for _, sec := range sections {
		title := strings.ToLower(strings.TrimSpace(sec.Title))
		if title == "" {
			rest = append(rest, sec)
			continue
		}
		byTitle[title] = sec
	}
	out := make([]FunnelDetailsSection, 0, len(sections))
	used := map[string]struct{}{}
	for _, key := range order {
		for title, sec := range byTitle {
			if strings.Contains(title, key) {
				out = append(out, sec)
				used[title] = struct{}{}
			}
		}
	}
	for _, sec := range sections {
		title := strings.ToLower(strings.TrimSpace(sec.Title))
		if title != "" {
			if _, ok := used[title]; ok {
				continue
			}
		}
		out = append(out, sec)
	}
	out = append(out, rest...)
	return out
}

func isForbiddenRecommendedAction(text string) bool {
	if strings.Contains(text, "цен") && (strings.Contains(text, "сниз") || strings.Contains(text, "повыс") || strings.Contains(text, "price")) {
		return true
	}
	return false
}

func mergeRepairResult(base, extra SummaryRepairResult) SummaryRepairResult {
	if extra.Repaired {
		base.Repaired = true
	}
	if extra.Partial {
		base.Partial = true
	}
	base.HardErrors = append(base.HardErrors, extra.HardErrors...)
	base.RepairableErrors = append(base.RepairableErrors, extra.RepairableErrors...)
	base.RepairActions = append(base.RepairActions, extra.RepairActions...)
	base.FallbackBlocks = append(base.FallbackBlocks, extra.FallbackBlocks...)
	base.DroppedActions += extra.DroppedActions
	return base
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max-1]) + "…"
}
