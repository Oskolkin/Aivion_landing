package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
)

var quotedNarrativeNamePattern = regexp.MustCompile(`«([^»]+)»|"([^"]+)"`)

func buildDashboardNarrativeValidationContext(
	drivers []RiskDriver,
	build dashboardNarrativeBuildResult,
) dashboardNarrativeValidationContext {
	driverMap := make(map[string]RiskDriver, len(drivers))
	for _, driver := range drivers {
		driverMap[driver.ID] = driver
	}
	sellerPulseCtx := build.SellerPulseCtx
	adEfficiencyCtx := build.AdEfficiencyNarrativeCtx
	return dashboardNarrativeValidationContext{
		DriversByID:          driverMap,
		CandidatesByID:       build.CandidateByID,
		MetricsByKey:         build.MetricCatalogByKey,
		AllowedQuotedName:    build.AllowedQuotedNames,
		WhatChanged:          build.WhatChanged,
		Stock:                copyStockNarrativeValidationCtx(build.StockNarrativeCtx),
		StockRequired:        build.StockNarrativeRequired,
		AdEfficiency:         &adEfficiencyCtx,
		AdEfficiencyRequired: build.AdEfficiencyNarrativeRequired,
		SellerPulse:          &sellerPulseCtx,
		SellerPulseRequired:  build.SellerPulseRequired,
	}
}

func copyStockNarrativeValidationCtx(v stockCoverageNarrativeValidationContext) *stockCoverageNarrativeValidationContext {
	c := v
	return &c
}

func validateDashboardNarrativePayload(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	var errs []string
	if payload == nil {
		return []string{"nil_dashboard_narrative_payload"}
	}
	if strings.TrimSpace(payload.Headline) == "" {
		errs = append(errs, "missing_headline")
	}
	if strings.TrimSpace(payload.ExecutiveSummary) == "" {
		errs = append(errs, "missing_executive_summary")
	}
	if strings.TrimSpace(payload.ImprovementSummary) == "" {
		errs = append(errs, "missing_improvement_summary")
	}
	if isImprovementSummaryAboutLackOfImprovements(payload.ImprovementSummary) {
		errs = append(errs, "improvement_summary_about_lack_of_improvements")
	}
	if isTooGenericNarrativeText(payload.ExecutiveSummary) {
		errs = append(errs, "executive_summary_too_generic")
	}
	errs = append(errs, validateNarrativeQuotedNames(
		"executive_summary_unknown_name",
		ctx.AllowedQuotedName,
		payload.ExecutiveSummary,
	)...)
	if len(ctx.DriversByID) > 0 && len(payload.HealthReasons) == 0 {
		errs = append(errs, "missing_health_reasons")
	}
	for i, reason := range payload.HealthReasons {
		driver, ok := ctx.DriversByID[reason.DriverID]
		if !ok {
			errs = append(errs, fmt.Sprintf("health_reason_%d_unknown_driver_id", i))
			continue
		}
		if strings.TrimSpace(reason.Title) == "" || strings.TrimSpace(reason.Reason) == "" {
			errs = append(errs, fmt.Sprintf("health_reason_%d_empty_text", i))
		}
		if reason.Category != "" && reason.Category != driver.Category {
			errs = append(errs, fmt.Sprintf("health_reason_%d_category_mismatch", i))
		}
		if isTooGenericNarrativeText(reason.Title) || isTooGenericNarrativeText(reason.Reason) {
			errs = append(errs, fmt.Sprintf("health_reason_%d_too_generic", i))
		}
		errs = append(errs, validateNarrativeQuotedNames(
			fmt.Sprintf("health_reason_%d_unknown_name", i),
			ctx.AllowedQuotedName,
			reason.Title,
			reason.Reason,
		)...)
	}
	if len(ctx.DriversByID) > 0 && len(payload.TopActions) == 0 {
		errs = append(errs, "missing_top_actions")
	}
	errs = append(errs, validateDataPackNarrativeExtras(payload, ctx)...)
	for i, action := range payload.TopActions {
		driver, driverOK := ctx.DriversByID[action.DriverID]
		if !driverOK {
			errs = append(errs, fmt.Sprintf("top_action_%d_unknown_driver_id", i))
			continue
		}
		candidate, candidateOK := ctx.CandidatesByID[action.SourceCandidateID]
		if !candidateOK {
			errs = append(errs, fmt.Sprintf("top_action_%d_unknown_source_candidate_id", i))
			continue
		}
		if strings.TrimSpace(action.Title) == "" ||
			strings.TrimSpace(action.Reason) == "" ||
			strings.TrimSpace(action.NextStep) == "" ||
			strings.TrimSpace(action.ExpectedEffect) == "" {
			errs = append(errs, fmt.Sprintf("top_action_%d_empty_text", i))
		}
		expectedCategory := topActionCategoryForDriver(driver.Category)
		if action.Category == "" {
			errs = append(errs, fmt.Sprintf("top_action_%d_missing_category", i))
		} else if action.Category != expectedCategory && action.Category != driver.Category {
			errs = append(errs, fmt.Sprintf("top_action_%d_category_mismatch", i))
		}
		if candidate.Category != "" && action.Category != "" && candidate.Category != action.Category {
			errs = append(errs, fmt.Sprintf("top_action_%d_candidate_category_mismatch", i))
		}
		if !candidateSupportsDriver(candidate, driver.ID) {
			errs = append(errs, fmt.Sprintf("top_action_%d_candidate_not_linked_to_driver", i))
		}
		if containsUnbackedNumber(action.Title) ||
			containsUnbackedNumber(action.Reason) ||
			containsUnbackedNumber(action.NextStep) ||
			containsUnbackedNumber(action.ExpectedEffect) {
			if len(action.SupportingMetricRefs) == 0 && len(candidate.SupportingMetrics) == 0 {
				errs = append(errs, fmt.Sprintf("top_action_%d_invented_metric_or_number", i))
			}
		}
		for _, ref := range action.SupportingMetricRefs {
			if _, ok := ctx.MetricsByKey[ref]; !ok {
				errs = append(errs, fmt.Sprintf("top_action_%d_unknown_metric_ref_%s", i, ref))
			}
		}
		if isTooGenericNarrativeText(action.Title) ||
			isTooGenericNarrativeText(action.Reason) ||
			isTooGenericNarrativeText(action.NextStep) ||
			isTooGenericNarrativeText(action.ExpectedEffect) {
			errs = append(errs, fmt.Sprintf("top_action_%d_too_generic", i))
		}
		if containsForbiddenNarrativeRecommendation(driver, action) {
			errs = append(errs, fmt.Sprintf("top_action_%d_forbidden_recommendation", i))
		}
		errs = append(errs, validateNarrativeQuotedNames(
			fmt.Sprintf("top_action_%d_unknown_name", i),
			ctx.AllowedQuotedName,
			action.Title,
			action.Reason,
			action.NextStep,
			action.ExpectedEffect,
		)...)
	}
	if ctx.StockRequired {
		if payload.StockCoverageNarrative == nil {
			errs = append(errs, "missing_stock_coverage_narrative")
		} else if ctx.Stock != nil {
			forceStockNarrativeRevenueFromBackend(payload.StockCoverageNarrative, ctx.Stock.RevenueBackend)
			errs = append(errs, validateStockCoverageNarrativePayload(payload.StockCoverageNarrative, *ctx.Stock)...)
		}
	}
	errs = append(errs, validateWhatChangedPayload(payload.WhatChanged, ctx.WhatChanged)...)
	if ctx.SellerPulseRequired {
		if payload.SellerPulseNarrative == nil {
			errs = append(errs, "missing_seller_pulse_narrative")
		} else if ctx.SellerPulse != nil {
			errs = append(errs, validateSellerPulseNarrativePayload(payload.SellerPulseNarrative, *ctx.SellerPulse)...)
		}
	}
	if ctx.AdEfficiencyRequired {
		if payload.AdEfficiencyNarrative == nil {
			errs = append(errs, "missing_ad_efficiency_narrative")
		} else if ctx.AdEfficiency != nil {
			errs = append(errs, validateAdEfficiencyNarrativePayload(payload.AdEfficiencyNarrative, *ctx.AdEfficiency)...)
		}
	}
	return compactStringSlice(errs)
}

func candidateSupportsDriver(candidate dashboardNarrativeCandidate, driverID string) bool {
	if len(candidate.CompatibleDriverIDs) == 0 {
		return true
	}
	for _, id := range candidate.CompatibleDriverIDs {
		if id == driverID {
			return true
		}
	}
	return false
}

func validateNarrativeQuotedNames(prefix string, allowed map[string]struct{}, values ...string) []string {
	if len(allowed) == 0 {
		return nil
	}
	var errs []string
	for _, value := range values {
		matches := quotedNarrativeNamePattern.FindAllStringSubmatch(value, -1)
		for _, match := range matches {
			name := strings.TrimSpace(match[1])
			if name == "" && len(match) > 2 {
				name = strings.TrimSpace(match[2])
			}
			if name == "" {
				continue
			}
			if _, ok := allowed[name]; !ok {
				errs = append(errs, prefix)
				break
			}
		}
	}
	return errs
}

func containsForbiddenNarrativeRecommendation(driver RiskDriver, action dashboardNarrativeTopAction) bool {
	text := strings.ToLower(strings.Join([]string{action.Title, action.Reason, action.NextStep, action.ExpectedEffect}, " "))
	if strings.Contains(text, "цен") && (strings.Contains(text, "сниз") || strings.Contains(text, "пониз") || strings.Contains(text, "повыс")) {
		return true
	}
	if driver.Category == "stock" || driver.Category == "surplus" {
		if strings.Contains(text, "реклам") && (strings.Contains(text, "увелич") || strings.Contains(text, "усиль") || strings.Contains(text, "подним")) {
			return true
		}
	}
	return false
}

func isImprovementSummaryAboutLackOfImprovements(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	for _, forbidden := range []string{
		"улучшений нет",
		"не выявлено",
		"не видно",
		"не компенсируются",
		"по сравнению с прошлой сводкой не",
		"заметных улучшений",
		"улучшений с прошлой сводки",
	} {
		if strings.Contains(normalized, forbidden) {
			return true
		}
	}
	return false
}

func isTooGenericNarrativeText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return true
	}
	if len(strings.Fields(normalized)) < 3 {
		return true
	}
	for _, forbidden := range []string{
		"проверьте всё",
		"проверьте все",
		"разберите ситуацию",
		"проверьте детали сигнала",
		"выберите следующее действие",
		"проанализируйте ситуацию",
	} {
		if strings.Contains(normalized, forbidden) {
			return true
		}
	}
	return false
}

func containsUnbackedNumber(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func narrativeFreshForCurrentContext(narrative *storedDashboardNarrative, build dashboardNarrativeBuildResult) bool {
	return evaluateNarrativeFreshness(narrative, build).Fresh
}
