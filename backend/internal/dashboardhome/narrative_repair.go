package dashboardhome

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	narrativeTimingNumberPattern  = regexp.MustCompile(`(?i)\b\d+\s*(?:–|-)\s*\d+\s*(?:час|дн)`)
	narrativeResultPromisePattern = regexp.MustCompile(`(?i)(увелич|сэконом|снизит|подним|выраст|сократит|верн[её]т).{0,24}\d`)
)

type narrativeRepairResult struct {
	Payload           *dashboardNarrativePayload
	HardErrors        []string
	RepairableErrors  []string
	RepairActions     []string
	PartialBlocks     []string
	FallbackBlocks    []string
	AIActionsReceived int
	AIActionsAccepted int
	DroppedActions    int
	Repaired          bool
	Partial           bool
}

func trimNarrativeText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func repairDashboardNarrativePayload(
	payload *dashboardNarrativePayload,
	ctx dashboardNarrativeValidationContext,
) narrativeRepairResult {
	result := narrativeRepairResult{
		Payload:           payload,
		AIActionsReceived: len(payload.TopActions),
	}
	if payload == nil {
		result.HardErrors = append(result.HardErrors, "nil_dashboard_narrative_payload")
		return result
	}

	payload.Headline = trimNarrativeText(payload.Headline)
	payload.ExecutiveSummary = trimNarrativeText(payload.ExecutiveSummary)
	payload.ImprovementSummary = trimNarrativeText(payload.ImprovementSummary)
	payload.WhatChanged = repairWhatChangedPayload(payload.WhatChanged, ctx.WhatChanged, &result)
	if payload.SellerPulseNarrative != nil {
		if raw, err := json.Marshal(payload.SellerPulseNarrative); err == nil && sellerPulseNarrativeLooksNested(string(raw)) {
			if unwrapped := unwrapSellerPulseNarrativeRaw(raw); unwrapped != nil {
				payload.SellerPulseNarrative = unwrapped
				result.RepairActions = append(result.RepairActions, "seller_pulse_narrative_unwrapped")
				result.Repaired = true
			}
		}
	}

	allowedNumbers := buildAllowedNarrativeNumbers(ctx)
	repairedActions := make([]dashboardNarrativeTopAction, 0, len(payload.TopActions))
	for i, action := range payload.TopActions {
		repaired, hardErrs, softErrs, repairs := repairDashboardNarrativeTopAction(i, action, ctx, allowedNumbers)
		result.HardErrors = append(result.HardErrors, hardErrs...)
		result.RepairableErrors = append(result.RepairableErrors, softErrs...)
		result.RepairActions = append(result.RepairActions, repairs...)
		if repaired == nil {
			continue
		}
		repairedActions = append(repairedActions, *repaired)
	}
	payload.TopActions = repairedActions
	result.AIActionsAccepted = len(repairedActions)
	result.DroppedActions = result.AIActionsReceived - result.AIActionsAccepted
	if result.DroppedActions < 0 {
		result.DroppedActions = 0
	}
	result.Repaired = len(result.RepairActions) > 0
	if result.AIActionsReceived > 0 && result.AIActionsAccepted < result.AIActionsReceived {
		result.Partial = true
	}

	if payload.StockCoverageNarrative != nil && ctx.Stock != nil {
		forceStockNarrativeRevenueFromBackend(payload.StockCoverageNarrative, ctx.Stock.RevenueBackend)
	}
	if ctx.StockRequired && payload.StockCoverageNarrative == nil {
		result.RepairableErrors = append(result.RepairableErrors, "missing_stock_coverage_narrative")
		result.PartialBlocks = append(result.PartialBlocks, "stock_coverage_narrative")
		result.FallbackBlocks = append(result.FallbackBlocks, "stock_coverage_narrative")
		result.Partial = true
	}
	if ctx.SellerPulseRequired && payload.SellerPulseNarrative == nil {
		result.RepairableErrors = append(result.RepairableErrors, "missing_seller_pulse_narrative")
		result.PartialBlocks = append(result.PartialBlocks, "seller_pulse_narrative")
		result.FallbackBlocks = append(result.FallbackBlocks, "seller_pulse_narrative")
		result.Partial = true
	}
	if payload.AdEfficiencyNarrative != nil && ctx.AdEfficiency != nil {
		repairAdEfficiencyNarrative(payload.AdEfficiencyNarrative)
	}
	if ctx.AdEfficiencyRequired && payload.AdEfficiencyNarrative == nil {
		result.RepairableErrors = append(result.RepairableErrors, "missing_ad_efficiency_narrative")
		result.PartialBlocks = append(result.PartialBlocks, "ad_efficiency_narrative")
		result.Partial = true
	}

	result.HardErrors = append(result.HardErrors, validateDashboardNarrativeHard(payload, ctx)...)
	result.HardErrors = compactStringSlice(result.HardErrors)
	result.RepairableErrors = compactStringSlice(result.RepairableErrors)
	result.RepairActions = compactStringSlice(result.RepairActions)
	return result
}

func repairDashboardNarrativeTopAction(
	index int,
	action dashboardNarrativeTopAction,
	ctx dashboardNarrativeValidationContext,
	allowedNumbers map[string]struct{},
) (*dashboardNarrativeTopAction, []string, []string, []string) {
	var hardErrs []string
	var softErrs []string
	var repairs []string

	driver, driverOK := ctx.DriversByID[action.DriverID]
	if !driverOK {
		softErrs = append(softErrs, fmt.Sprintf("top_action_%d_unknown_driver_id", index))
		return nil, hardErrs, softErrs, repairs
	}

	action.DriverID = driver.ID
	action.Priority = normalizeTopActionPriority(firstNonEmpty(action.Priority, driver.Severity))
	expectedCategory := topActionCategoryForDriver(driver.Category)
	if action.Category == "" {
		action.Category = expectedCategory
		repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_category", index))
	} else if action.Category != expectedCategory && action.Category != driver.Category {
		action.Category = expectedCategory
		repairs = append(repairs, fmt.Sprintf("top_action_%d_normalized_category", index))
	}

	candidate, candidateOK := ctx.CandidatesByID[action.SourceCandidateID]
	if !candidateOK {
		if resolved, ok := resolveNarrativeCandidateForAction(action, ctx); ok {
			candidate = resolved
			action.SourceCandidateID = candidate.ID
			repairs = append(repairs, fmt.Sprintf("top_action_%d_repaired_source_candidate_id", index))
		} else {
			softErrs = append(softErrs, fmt.Sprintf("top_action_%d_unknown_source_candidate_id", index))
		}
	}

	if containsForbiddenNarrativeRecommendation(driver, action) {
		softErrs = append(softErrs, fmt.Sprintf("top_action_%d_forbidden_recommendation", index))
		return nil, hardErrs, softErrs, repairs
	}

	action.Title = trimNarrativeText(action.Title)
	action.Reason = trimNarrativeText(action.Reason)
	action.NextStep = trimNarrativeText(action.NextStep)
	action.ExpectedEffect = trimNarrativeText(action.ExpectedEffect)

	if action.Title == "" {
		if candidateOK || candidate.ID != "" {
			action.Title = trimNarrativeText(firstNonEmpty(candidate.Title, fallbackActionTitle(action.Category)))
		} else {
			action.Title = fallbackActionTitle(action.Category)
		}
		repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_title", index))
	}
	if action.Reason == "" {
		action.Reason = trimNarrativeText(firstNonEmpty(candidate.Reason, driver.Reason))
		repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_reason", index))
	}
	if action.NextStep == "" {
		action.NextStep = trimNarrativeText(firstNonEmpty(candidate.NextStep, fallbackActionForDriver(driver).NextStep))
		repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_next_step", index))
	}

	if action.ExpectedEffect == "" || isTooGenericNarrativeText(action.ExpectedEffect) {
		action.ExpectedEffect = fallbackExpectedEffect(action.Category)
		repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_expected_effect", index))
	}
	if containsResultPromiseWithNumber(action.ExpectedEffect) || containsUnbackedFactualNumber(action.ExpectedEffect, allowedNumbers) {
		action.ExpectedEffect = fallbackExpectedEffect(action.Category)
		repairs = append(repairs, fmt.Sprintf("top_action_%d_replaced_expected_effect_number", index))
	}

	for _, field := range []struct {
		name string
		ptr  *string
	}{
		{"title", &action.Title},
		{"reason", &action.Reason},
		{"next_step", &action.NextStep},
	} {
		if isBackendSourcedNarrativeField(action, driver, candidate, field.name, *field.ptr) {
			continue
		}
		if containsResultPromiseWithNumber(*field.ptr) {
			softErrs = append(softErrs, fmt.Sprintf("top_action_%d_unrepairable_%s_promise", index, field.name))
			return nil, hardErrs, softErrs, repairs
		}
		if repaired, ok := repairUnbackedNumbersInText(*field.ptr, allowedNumbers); ok {
			*field.ptr = repaired
			repairs = append(repairs, fmt.Sprintf("top_action_%d_repaired_%s_number", index, field.name))
		} else if containsUnbackedFactualNumber(*field.ptr, allowedNumbers) {
			softErrs = append(softErrs, fmt.Sprintf("top_action_%d_unbacked_number_in_%s", index, field.name))
			return nil, hardErrs, softErrs, repairs
		}
	}

	knownRefs := make([]string, 0, len(action.SupportingMetricRefs))
	for _, ref := range action.SupportingMetricRefs {
		if _, ok := ctx.MetricsByKey[ref]; ok {
			knownRefs = append(knownRefs, ref)
			continue
		}
		softErrs = append(softErrs, fmt.Sprintf("top_action_%d_unknown_metric_ref_%s", index, ref))
		repairs = append(repairs, fmt.Sprintf("top_action_%d_removed_unknown_metric_ref_%s", index, ref))
	}
	action.SupportingMetricRefs = knownRefs
	if len(action.SupportingMetricRefs) == 0 && candidate.ID != "" {
		for _, metric := range candidate.SupportingMetrics {
			if metric.Key != "" {
				action.SupportingMetricRefs = append(action.SupportingMetricRefs, metric.Key)
			}
		}
		if len(action.SupportingMetricRefs) > 0 {
			repairs = append(repairs, fmt.Sprintf("top_action_%d_filled_metric_refs_from_candidate", index))
		}
	}

	if isTooGenericNarrativeText(action.Title) && isTooGenericNarrativeText(action.Reason) && isTooGenericNarrativeText(action.NextStep) {
		hardErrs = append(hardErrs, fmt.Sprintf("top_action_%d_too_generic", index))
		return nil, hardErrs, softErrs, repairs
	}

	if action.Title == "" || action.Reason == "" || action.NextStep == "" || action.ExpectedEffect == "" {
		hardErrs = append(hardErrs, fmt.Sprintf("top_action_%d_empty_text_after_repair", index))
		return nil, hardErrs, softErrs, repairs
	}

	_ = validateNarrativeQuotedNames(fmt.Sprintf("top_action_%d_unknown_name", index), ctx.AllowedQuotedName, action.Title, action.Reason, action.NextStep, action.ExpectedEffect)
	return &action, hardErrs, softErrs, repairs
}

func resolveNarrativeCandidateForAction(
	action dashboardNarrativeTopAction,
	ctx dashboardNarrativeValidationContext,
) (dashboardNarrativeCandidate, bool) {
	if candidate, ok := ctx.CandidatesByID[action.SourceCandidateID]; ok {
		return candidate, true
	}
	if candidate, ok := ctx.CandidatesByID[action.DriverID+"-action"]; ok {
		return candidate, true
	}
	targetCategory := topActionCategoryForDriver(ctx.DriversByID[action.DriverID].Category)
	var best *dashboardNarrativeCandidate
	for _, candidate := range ctx.CandidatesByID {
		copyCandidate := candidate
		if candidateSupportsDriver(copyCandidate, action.DriverID) {
			return copyCandidate, true
		}
		if copyCandidate.Category == targetCategory && best == nil {
			best = &copyCandidate
		}
	}
	if best != nil {
		return *best, true
	}
	return dashboardNarrativeCandidate{}, false
}

func validateDashboardNarrativeHard(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	if payload == nil {
		return []string{"nil_dashboard_narrative_payload"}
	}
	if !hasUsableNarrativeSummary(payload) && len(payload.TopActions) == 0 {
		return []string{"no_usable_narrative_content"}
	}
	if len(ctx.DriversByID) > 0 && len(payload.TopActions) == 0 && !hasUsableNarrativeSummary(payload) {
		return []string{"missing_top_actions"}
	}
	return nil
}

func hasUsableNarrativeSummary(payload *dashboardNarrativePayload) bool {
	if payload == nil {
		return false
	}
	return trimNarrativeText(payload.Headline) != "" ||
		trimNarrativeText(payload.ExecutiveSummary) != "" ||
		trimNarrativeText(payload.ImprovementSummary) != ""
}

func buildAllowedNarrativeNumbers(ctx dashboardNarrativeValidationContext) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(value string) {
		for _, token := range extractNumericTokens(value) {
			out[token] = struct{}{}
		}
		for _, token := range extractNumericTokens(strings.ReplaceAll(value, " ", "")) {
			out[token] = struct{}{}
		}
	}
	for _, metric := range ctx.MetricsByKey {
		add(metric.Value)
	}
	for _, driver := range ctx.DriversByID {
		add(driver.Title)
		add(driver.Reason)
		for _, value := range driver.Evidence {
			add(fmt.Sprintf("%v", value))
		}
	}
	for _, candidate := range ctx.CandidatesByID {
		add(candidate.Title)
		add(candidate.Reason)
		add(candidate.NextStep)
		for _, metric := range candidate.SupportingMetrics {
			add(metric.Value)
		}
	}
	return out
}

func isBackendSourcedNarrativeField(
	action dashboardNarrativeTopAction,
	driver RiskDriver,
	candidate dashboardNarrativeCandidate,
	fieldName string,
	value string,
) bool {
	normalized := trimNarrativeText(value)
	if normalized == "" {
		return false
	}
	switch fieldName {
	case "title":
		return normalized == trimNarrativeText(driver.Title) ||
			normalized == trimNarrativeText(candidate.Title) ||
			normalized == trimNarrativeText(fallbackActionTitle(action.Category))
	case "reason":
		return normalized == trimNarrativeText(driver.Reason) ||
			normalized == trimNarrativeText(candidate.Reason)
	case "next_step":
		return normalized == trimNarrativeText(candidate.NextStep) ||
			normalized == trimNarrativeText(fallbackActionForDriver(driver).NextStep)
	default:
		return false
	}
}

func extractNumericTokens(text string) []string {
	var out []string
	for _, field := range strings.Fields(text) {
		token := strings.Trim(field, ".,%₽()")
		if token == "" {
			continue
		}
		hasDigit := false
		for _, r := range token {
			if unicode.IsDigit(r) {
				hasDigit = true
				break
			}
		}
		if hasDigit {
			out = append(out, token)
		}
	}
	return out
}

func containsResultPromiseWithNumber(text string) bool {
	return narrativeResultPromisePattern.MatchString(strings.ToLower(text))
}

func containsUnbackedFactualNumber(text string, allowed map[string]struct{}) bool {
	if narrativeTimingNumberPattern.MatchString(text) {
		return false
	}
	for _, token := range extractNumericTokens(text) {
		if _, ok := allowed[token]; ok {
			continue
		}
		// IDs and small ordinals are allowed.
		if len(strings.Trim(token, "0123456789")) == 0 && len(token) <= 4 {
			continue
		}
		return true
	}
	return false
}

func repairUnbackedNumbersInText(text string, allowed map[string]struct{}) (string, bool) {
	if !containsUnbackedFactualNumber(text, allowed) {
		return text, false
	}
	if containsResultPromiseWithNumber(text) {
		return text, false
	}
	return text, false
}
