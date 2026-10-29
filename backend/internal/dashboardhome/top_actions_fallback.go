package dashboardhome

import (
	"fmt"
	"sort"
	"strings"
)

// buildAIDiagnosticsTopActions is the multilevel builder for ai_diagnostics.top_actions.
//
//  1. Use AI narrative top actions when a fresh narrative was applied.
//  2. Otherwise build fallback top actions from AI-summary recommended actions,
//     recommendation/alert candidates and risk drivers (so the block stays useful
//     even when the narrative is stale/missing/failed).
//  3. Only return [] when there are truly no candidates.
func buildAIDiagnosticsTopActions(
	narrativeApplied bool,
	aiNarrativeActions []TopActionItem,
	aiSummaryActions []TopActionItem,
	drivers []RiskDriver,
	actionPool []TopActionItem,
	limit int,
) []TopActionItem {
	if narrativeApplied && len(aiNarrativeActions) > 0 {
		return aiNarrativeActions
	}
	fallback := buildFallbackTopActionsForAIDiagnostics(aiSummaryActions, drivers, actionPool, limit)
	if len(fallback) > 0 {
		return fallback
	}
	return []TopActionItem{}
}

// buildFallbackTopActionsForAIDiagnostics assembles deterministic Top 3 cards when the
// AI narrative is not current. Candidate priority order:
//
//  1. AI-summary recommended actions that already carry expected_effect;
//  2. recommendation/alert candidates (already shaped via buildTopActionPool);
//  3. risk drivers.
func buildFallbackTopActionsForAIDiagnostics(
	aiSummaryActions []TopActionItem,
	drivers []RiskDriver,
	actionPool []TopActionItem,
	limit int,
) []TopActionItem {
	if limit <= 0 {
		return []TopActionItem{}
	}
	candidates := make([]TopActionItem, 0, len(aiSummaryActions)+len(actionPool)+len(drivers))
	for _, action := range aiSummaryActions {
		if strings.TrimSpace(action.ExpectedEffect) == "" {
			continue
		}
		candidates = append(candidates, action)
	}
	candidates = append(candidates, actionPool...)
	for _, driver := range drivers {
		candidates = append(candidates, topActionFromDriverFallback(driver))
	}
	return dedupeFallbackTopActions(candidates, limit)
}

// topActionFromDriverFallback turns a risk driver into a seller-facing Top action card
// with a backend-safe expected_effect.
func topActionFromDriverFallback(driver RiskDriver) TopActionItem {
	action := fallbackActionForDriver(driver)
	action.DriverID = driver.ID
	action.Source = "risk_driver"
	if action.Category == "" {
		action.Category = topActionCategoryForDriver(driver.Category)
	}
	if action.Priority == "" {
		action.Priority = driver.Severity
	}
	action.LinkedAlertIDs = compactInt64s(append(action.LinkedAlertIDs, driver.LinkedAlertIDs...))
	action.LinkedRecommendationIDs = compactInt64s(append(action.LinkedRecommendationIDs, driver.LinkedRecommendationIDs...))
	return action
}

// buildAISummaryFallbackTopActions converts AI-summary recommended actions (the
// recommended_actions feed source) into Top action candidates, preserving their
// existing expected_effect.
func buildAISummaryFallbackTopActions(aiItems []ActionItem) []TopActionItem {
	out := make([]TopActionItem, 0, len(aiItems))
	for i := range aiItems {
		item := aiItems[i]
		if !strings.EqualFold(strings.TrimSpace(item.Source), "ai_summary") {
			continue
		}
		nextStep := strings.TrimSpace(item.Action)
		if nextStep == "" {
			continue
		}
		category := detectActionItemCategory(item)
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = nextStep
		}
		action := TopActionItem{
			ID:             fmt.Sprintf("ai-summary-action-%d", i+1),
			Source:         "ai_summary",
			Priority:       normalizeTopActionPriority(item.Priority),
			Category:       category,
			Title:          fallbackActionTitle(category),
			Reason:         cleanupSellerFacingText(reason),
			NextStep:       cleanupSellerFacingText(nextStep),
			ExpectedEffect: cleanupSellerFacingText(item.ExpectedEffect),
		}
		if item.LinkedAlertID != nil {
			action.LinkedAlertIDs = []int64{*item.LinkedAlertID}
		}
		if item.LinkedRecommendationID != nil {
			action.LinkedRecommendationIDs = []int64{*item.LinkedRecommendationID}
		}
		out = append(out, action)
	}
	return out
}

func detectActionItemCategory(item ActionItem) string {
	return detectTopActionCategory(topActionCandidate{
		RelatedEntityType: item.RelatedEntityType,
		RelatedEntityID:   item.RelatedEntityID,
		Title:             item.Action,
		WhatHappened:      item.Reason,
		NextStep:          item.ExpectedEffect,
	})
}

// dedupeFallbackTopActions normalizes candidates, prefers one card per category (so the
// block is not three near-identical advertising cards when other directions exist),
// then fills any remaining slots regardless of category.
func dedupeFallbackTopActions(candidates []TopActionItem, limit int) []TopActionItem {
	if limit <= 0 {
		return []TopActionItem{}
	}
	normalized := make([]TopActionItem, 0, len(candidates))
	seenKey := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		item := ensureFallbackTopActionFields(candidate)
		if !isValidFallbackTopAction(item) {
			continue
		}
		key := fallbackTopActionDedupKey(item)
		if _, exists := seenKey[key]; exists {
			continue
		}
		seenKey[key] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return []TopActionItem{}
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return priorityRank(normalized[i].Priority) < priorityRank(normalized[j].Priority)
	})

	out := make([]TopActionItem, 0, limit)
	usedCategories := make(map[string]struct{}, limit)
	usedKeys := make(map[string]struct{}, limit)
	for _, item := range normalized {
		if len(out) >= limit {
			break
		}
		if _, exists := usedCategories[item.Category]; exists {
			continue
		}
		usedCategories[item.Category] = struct{}{}
		usedKeys[fallbackTopActionDedupKey(item)] = struct{}{}
		out = append(out, item)
	}
	if len(out) < limit {
		for _, item := range normalized {
			if len(out) >= limit {
				break
			}
			if _, exists := usedKeys[fallbackTopActionDedupKey(item)]; exists {
				continue
			}
			usedKeys[fallbackTopActionDedupKey(item)] = struct{}{}
			out = append(out, item)
		}
		sort.SliceStable(out, func(i, j int) bool {
			return priorityRank(out[i].Priority) < priorityRank(out[j].Priority)
		})
	}
	return out
}

func ensureFallbackTopActionFields(item TopActionItem) TopActionItem {
	item.Title = cleanupSellerFacingText(item.Title)
	item.Reason = cleanupSellerFacingText(item.Reason)
	item.NextStep = cleanupSellerFacingText(item.NextStep)
	if strings.TrimSpace(item.Category) == "" {
		item.Category = "account"
	}
	item.Category = strings.ToLower(strings.TrimSpace(item.Category))
	item.CategoryLabel = topActionCategoryLabel(item.Category)
	item.Priority = normalizeTopActionPriority(item.Priority)
	item.PriorityLabel = topActionPriorityLabel(item.Priority)
	if strings.TrimSpace(item.ExpectedEffect) == "" {
		item.ExpectedEffect = fallbackExpectedEffect(item.Category)
	} else {
		item.ExpectedEffect = cleanupSellerFacingText(item.ExpectedEffect)
	}
	if strings.TrimSpace(item.Source) == "" {
		item.Source = "recommendation"
	}
	if strings.TrimSpace(item.ID) == "" {
		item.ID = fallbackTopActionDedupKey(item)
	}
	item.SupportingMetrics = compactSupportingMetrics(item.SupportingMetrics)
	return item
}

func isValidFallbackTopAction(item TopActionItem) bool {
	return strings.TrimSpace(item.ID) != "" &&
		strings.TrimSpace(item.Source) != "" &&
		strings.TrimSpace(item.Priority) != "" &&
		strings.TrimSpace(item.Category) != "" &&
		strings.TrimSpace(item.Title) != "" &&
		strings.TrimSpace(item.Reason) != "" &&
		strings.TrimSpace(item.NextStep) != "" &&
		strings.TrimSpace(item.ExpectedEffect) != ""
}

func fallbackTopActionDedupKey(item TopActionItem) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return strings.ToLower(id)
	}
	return strings.ToLower(strings.TrimSpace(item.Category)) + "|" + normalizeActionKey(item.Title)
}

// fallbackActionTitle returns an action-oriented title per category for fallback cards
// that do not carry their own title.
func fallbackActionTitle(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "advertising":
		return "Проверьте рекламные расходы с низкой отдачей"
	case "stock":
		return "Проверьте товары с риском дефицита"
	case "surplus":
		return "Разберите товары с избыточным остатком"
	case "funnel":
		return "Проверьте слабое место воронки"
	case "cancellations":
		return "Проверьте рост отмен"
	case "sales":
		return "Проверьте причину просадки продаж"
	case "products":
		return "Проверьте товар, который требует внимания"
	default:
		return "Проверьте приоритетное действие по кабинету"
	}
}

// fallbackExpectedEffect provides a backend-safe expected effect for fallback cards that
// do not already carry one. Kept on the backend so the frontend only renders the value.
func fallbackExpectedEffect(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "advertising":
		return "Снизить риск рекламного перерасхода."
	case "stock":
		return "Снизить риск потери продаж из-за дефицита."
	case "surplus":
		return "Освободить склад и оборотные средства от излишков."
	case "funnel":
		return "Найти причину потери покупателей на этапе выбора."
	case "sales":
		return "Быстрее локализовать причину просадки продаж."
	case "cancellations":
		return "Снизить риск потери заказов из-за отмен."
	case "products":
		return "Не упустить проблему по ключевому товару."
	default:
		return "Сфокусироваться на действии с наибольшим влиянием."
	}
}
