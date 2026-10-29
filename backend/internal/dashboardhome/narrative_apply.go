package dashboardhome

import (
	"fmt"
	"strings"
)

func applyDashboardNarrativeHealthReasons(
	drivers []RiskDriver,
	payload *dashboardNarrativePayload,
) []RiskDriver {
	if payload == nil || len(payload.HealthReasons) == 0 {
		return drivers
	}
	reasonsByDriver := make(map[string]dashboardNarrativeHealthReason, len(payload.HealthReasons))
	for _, reason := range payload.HealthReasons {
		reasonsByDriver[reason.DriverID] = reason
	}
	out := make([]RiskDriver, 0, len(drivers))
	for _, driver := range drivers {
		if override, ok := reasonsByDriver[driver.ID]; ok {
			if override.Title != "" {
				driver.Title = override.Title
			}
			if override.Reason != "" {
				driver.Reason = override.Reason
			}
		}
		out = append(out, driver)
	}
	return out
}

func buildTopActionsFromDashboardNarrative(
	payload *dashboardNarrativePayload,
	build dashboardNarrativeBuildResult,
	drivers []RiskDriver,
	fallbackPool []TopActionItem,
	limit int,
) []TopActionItem {
	if payload == nil || limit <= 0 {
		return selectTopActionsFromDrivers(drivers, fallbackPool, limit)
	}
	driverMap := make(map[string]RiskDriver, len(drivers))
	for _, driver := range drivers {
		driverMap[driver.ID] = driver
	}

	out := make([]TopActionItem, 0, limit)
	used := map[string]struct{}{}
	for _, action := range payload.TopActions {
		if len(out) >= limit {
			break
		}
		driver, ok := driverMap[action.DriverID]
		if !ok {
			continue
		}
		candidate, ok := build.CandidateByID[action.SourceCandidateID]
		if !ok {
			validationCtx := dashboardNarrativeValidationContext{
				DriversByID:    driverMap,
				CandidatesByID: build.CandidateByID,
			}
			if resolved, resolvedOK := resolveNarrativeCandidateForAction(action, validationCtx); resolvedOK {
				candidate = resolved
				ok = true
			}
		}
		if !ok {
			continue
		}
		item := TopActionItem{
			ID:                      candidate.ID,
			DriverID:                driver.ID,
			Source:                  "ai_summary",
			Priority:                candidate.Priority,
			PriorityLabel:           topActionPriorityLabel(candidate.Priority),
			Category:                candidate.Category,
			CategoryLabel:           topActionCategoryLabel(candidate.Category),
			Title:                   cleanupSellerFacingText(action.Title),
			Reason:                  cleanupSellerFacingText(action.Reason),
			NextStep:                cleanupSellerFacingText(action.NextStep),
			ExpectedEffect:          cleanupSellerFacingText(action.ExpectedEffect),
			LinkedAlertIDs:          compactInt64s(candidate.LinkedAlertIDs),
			LinkedRecommendationIDs: compactInt64s(candidate.LinkedRecommendationIDs),
			SupportingMetrics:       selectDashboardNarrativeSupportingMetrics(action.SupportingMetricRefs, candidate),
		}
		if item.Priority == "" {
			item.Priority = driver.Severity
			item.PriorityLabel = topActionPriorityLabel(item.Priority)
		}
		if item.Category == "" {
			item.Category = topActionCategoryForDriver(driver.Category)
			item.CategoryLabel = topActionCategoryLabel(item.Category)
		}
		if item.ID == "" {
			item.ID = driver.ID + "-ai-action"
		}
		if _, exists := used[item.ID]; exists {
			continue
		}
		used[item.ID] = struct{}{}
		out = append(out, item)
	}

	return out
}

func buildTopActionsFromDashboardNarrativeAIOnly(
	payload *dashboardNarrativePayload,
	drivers []RiskDriver,
	limit int,
) []TopActionItem {
	if payload == nil || limit <= 0 {
		return nil
	}
	driverMap := make(map[string]RiskDriver, len(drivers))
	for _, driver := range drivers {
		driverMap[driver.ID] = driver
	}
	out := make([]TopActionItem, 0, limit)
	used := map[string]struct{}{}
	for i, action := range payload.TopActions {
		if len(out) >= limit {
			break
		}
		driver, ok := driverMap[action.DriverID]
		category := strings.TrimSpace(action.Category)
		priority := strings.TrimSpace(action.Priority)
		if ok {
			if category == "" {
				category = topActionCategoryForDriver(driver.Category)
			}
			if priority == "" {
				priority = driver.Severity
			}
		}
		if category == "" {
			category = categoryFromDriverID(action.DriverID)
		}
		if priority == "" {
			priority = "medium"
		}
		id := strings.TrimSpace(action.SourceCandidateID)
		if id == "" {
			id = fmt.Sprintf("ai-only-top-action-%d", i+1)
		} else {
			id = fmt.Sprintf("%s-%d", id, i+1)
		}
		if _, exists := used[id]; exists {
			continue
		}
		used[id] = struct{}{}
		out = append(out, TopActionItem{
			ID:             id,
			DriverID:       action.DriverID,
			Source:         "dashboard_narrative_ai_only",
			Priority:       priority,
			PriorityLabel:  topActionPriorityLabel(priority),
			Category:       category,
			CategoryLabel:  topActionCategoryLabel(category),
			Title:          cleanupSellerFacingText(action.Title),
			Reason:         cleanupSellerFacingText(action.Reason),
			NextStep:       cleanupSellerFacingText(action.NextStep),
			ExpectedEffect: cleanupSellerFacingText(action.ExpectedEffect),
		})
	}
	return out
}

func selectDashboardNarrativeSupportingMetrics(
	refs []string,
	candidate dashboardNarrativeCandidate,
) []TopActionSupportingMetric {
	if len(candidate.SupportingMetrics) == 0 {
		return nil
	}
	if len(refs) == 0 {
		return narrativeSupportingMetricsToTopAction(candidate.SupportingMetrics)
	}
	selected := make([]dashboardNarrativeMetric, 0, len(refs))
	byKey := make(map[string]dashboardNarrativeMetric, len(candidate.SupportingMetrics))
	for _, metric := range candidate.SupportingMetrics {
		byKey[metric.Key] = metric
	}
	for _, ref := range refs {
		if metric, ok := byKey[ref]; ok {
			selected = append(selected, metric)
		}
	}
	if len(selected) == 0 {
		return narrativeSupportingMetricsToTopAction(candidate.SupportingMetrics)
	}
	return narrativeSupportingMetricsToTopAction(selected)
}

func narrativeSupportingMetricsToTopAction(metrics []dashboardNarrativeMetric) []TopActionSupportingMetric {
	out := make([]TopActionSupportingMetric, 0, len(metrics))
	for _, metric := range metrics {
		if metric.Label == "" || metric.Value == "" {
			continue
		}
		out = append(out, TopActionSupportingMetric{
			Label: metric.Label,
			Value: metric.Value,
		})
	}
	return compactSupportingMetrics(out)
}
