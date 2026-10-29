package dashboardhome

import (
	"regexp"
	"strings"
)

var internalFieldNamePattern = regexp.MustCompile(`(?i)\bsku_focus\.|ai_summary\.|fallback|legacy`)

func finalizeSellerFacingDashboardResponse(resp *Response) {
	if resp == nil {
		return
	}
	applyHealthStatusFromScore(resp)
	applyPeriodContextFromMetricDate(resp)
	localizeSellerPulseKPILabels(resp)
	reconcileSellerPulseQuickActionsWithTopActions(resp)
	reconcileStockCoverageFromSKUFocus(resp)
	reconcileFunnelAnalysisConsistency(&resp.AIDiagnostics.FunnelAnalysis)
	// Unified publications already carry validated What Changed text in the
	// frozen narrative. Do not re-derive/strengthen conclusions on GET.
	unified := IsUnifiedPublicationPromptVersion(resp.AIDiagnostics.AccountSummary.PromptVersion)
	if !unified {
		repairSellerFacingWhatChangedConclusion(&resp.AIDiagnostics.WhatChanged)
		omitDeprecatedSKUFocusSurplus(resp)
	}
	sanitizeSellerFacingRiskDrivers(resp)
	polishSellerFacingStockCoverageMessage(resp)
	if n := resp.SellerPulse.Narrative; n != nil {
		// Keep AI thesis headline available for the structured AI-резюме card.
		n.DisplayHeadline = strings.TrimSpace(n.Headline) != ""
	}
}

// localizeSellerPulseKPILabels applies presentation RU labels on read so frozen
// publication facts (and older snapshots) stay consistent with the UI contract.
func localizeSellerPulseKPILabels(resp *Response) {
	if resp == nil {
		return
	}
	for i := range resp.SellerPulse.KPIs {
		kpi := &resp.SellerPulse.KPIs[i]
		switch strings.TrimSpace(strings.ToLower(kpi.Key)) {
		case "average_order_value", "aov":
			kpi.Label = "Средний чек"
			kpi.Key = "average_order_value"
		case "ad_spend_share_total_revenue", "drr":
			kpi.Label = "Рекламные расходы / общая выручка"
			kpi.Key = "ad_spend_share_total_revenue"
		case "ctr":
			// Legacy frozen publications may still carry CTR; keep a clear label
			// but do not promote CTR as a pulse KPI for new facts.
			kpi.Label = "Кликабельность карточек"
		}
	}
}

func reconcileSellerPulseQuickActionsWithTopActions(resp *Response) {
	n := resp.SellerPulse.Narrative
	if n == nil {
		return
	}
	if len(resp.AIDiagnostics.TopActions) > 0 {
		n.QuickActions = buildSellerPulseQuickActionsFromTopActions(resp)
		return
	}
	topByLabel := map[string]TopActionItem{}
	for _, top := range resp.AIDiagnostics.TopActions {
		key := strings.ToLower(strings.TrimSpace(top.Title))
		if key != "" {
			topByLabel[key] = top
		}
	}
	for i := range n.QuickActions {
		qa := &n.QuickActions[i]
		labelKey := strings.ToLower(strings.TrimSpace(qa.Label))
		if top, ok := topByLabel[labelKey]; ok {
			qa.Kind = "action"
			qa.ActionID = top.ActionID
			qa.ActionKey = top.ActionKey
			qa.Href = actionDetailHref(top.ActionKey, top.ActionID)
			qa.Priority = normalizeTopActionPriority(top.Priority)
			continue
		}
		if isLegacySellerPulseHref(qa.Href) {
			qa.Href = "/app/actions"
		}
	}
}

func reconcileStockCoverageFromSKUFocus(resp *Response) {
	if resp.SKUFocus.NoSalesWithStockTotalCount <= 0 {
		return
	}
	n := resp.AIDiagnostics.StockCoverage.Narrative
	if n == nil {
		return
	}
	ctx := stockCoverageNarrativeValidationContext{NoSalesCount: resp.SKUFocus.NoSalesWithStockTotalCount}
	enforceStockCoverageNoSalesFactual(n, ctx, resp)
}

func omitDeprecatedSKUFocusSurplus(resp *Response) {
	if len(resp.SKUFocus.SlowTurnover) == 0 && len(resp.SKUFocus.Surplus) > 0 {
		resp.SKUFocus.SlowTurnover = append([]SKUFocusCard(nil), resp.SKUFocus.Surplus...)
	}
	resp.SKUFocus.Surplus = nil
}

func polishSellerFacingStockCoverageMessage(resp *Response) {
	msg := strings.TrimSpace(resp.AIDiagnostics.StockCoverage.Message)
	if msg == "" {
		return
	}
	if strings.Contains(strings.ToLower(msg), "sku_focus.") {
		count := len(resp.SKUFocus.SlowTurnover)
		resp.AIDiagnostics.StockCoverage.Message = formatSlowTurnoverStockMessage(count)
		resp.AIDiagnostics.StockCoverage.SlowTurnoverLink = &StockCoverageSKUFocusLink{
			Label:  "Открыть SKU с риском хранения",
			Target: "sku-focus-slow-turnover",
		}
	}
}

func sanitizeSellerFacingRiskDrivers(resp *Response) {
	for i := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		d := &resp.AIDiagnostics.AccountSummary.RiskDrivers[i]
		if d.Category == "slow_turnover" || d.ID == driverSlowTurnover {
			d.CategoryLabel = riskDriverCategoryLabel("slow_turnover")
		}
		d.Title = stripInternalFieldNames(d.Title)
		d.Reason = stripInternalFieldNames(d.Reason)
	}
}

func stripInternalFieldNames(text string) string {
	if !internalFieldNamePattern.MatchString(text) {
		return text
	}
	return strings.TrimSpace(internalFieldNamePattern.ReplaceAllString(text, ""))
}

func containsInternalFieldReference(text string) bool {
	return internalFieldNamePattern.MatchString(text) || strings.Contains(strings.ToLower(text), "sku_focus.")
}
