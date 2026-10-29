package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
)

// DashboardHomeConsistencyIssue describes a post-build dashboard response inconsistency.
type DashboardHomeConsistencyIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // warning | failed
	Message  string `json:"message"`
}

// DashboardNarrativeDebug is returned when DASHBOARD_NARRATIVE_DEBUG=1.
type DashboardNarrativeDebug struct {
	AIOnly                  bool                            `json:"ai_only,omitempty"`
	LatestRunID             string                          `json:"latest_run_id,omitempty"`
	PromptVersion           string                          `json:"prompt_version,omitempty"`
	Status                  string                          `json:"status,omitempty"`
	FinalPayloadSource      string                          `json:"final_payload_source,omitempty"`
	SubstitutionUsed        bool                            `json:"substitution_used,omitempty"`
	RequiredBlocks          map[string]string               `json:"required_blocks,omitempty"`
	SubstitutionBlocks      []string                        `json:"substitution_blocks,omitempty"`
	DeprecatedBlocks        []string                        `json:"deprecated_blocks,omitempty"`
	QueuedBlocks            []string                        `json:"queued_blocks,omitempty"`
	OutdatedBlocks          []string                        `json:"outdated_blocks,omitempty"`
	SourceMap               map[string]string               `json:"source_map,omitempty"`
	AppliedBlocks           map[string]string               `json:"applied_blocks,omitempty"`
	ApplyRepairs            []string                        `json:"apply_repairs,omitempty"`
	AcceptanceRepairLog     []string                        `json:"acceptance_repair_log,omitempty"`
	OverwrittenByDeprecated []string                        `json:"overwritten_by_deprecated,omitempty"`
	ConsistencyStatus       string                          `json:"consistency_status,omitempty"`
	ConsistencyIssues       []DashboardHomeConsistencyIssue `json:"consistency_issues,omitempty"`
}

var (
	noRisksMessagePattern = regexp.MustCompile(`(?i)рисков не обнаружено|открытых рисков нет`)
	factIDInTextPattern   = regexp.MustCompile(`\bfact_id\b|\bentity_id\b`)
)

// ValidateDashboardHomeResponseConsistency runs response-level checks after BuildHome composition.
func ValidateDashboardHomeResponseConsistency(resp *Response) []DashboardHomeConsistencyIssue {
	if resp == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0, 8)
	stored := resp.AIDiagnostics.AccountSummary.storedNarrative
	summary := resp.AIDiagnostics.AccountSummary

	if stored != nil &&
		stored.PromptVersion == DashboardNarrativePromptVersion &&
		stored.Status == "completed" &&
		!dashboardAIOnlyEnabled() {
		if summary.Freshness == "fallback" || summary.NarrativeSource == "fallback" || summary.ValidationStatus == "fallback_stale" {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "account_summary_fallback_after_chunks",
				Severity: "failed",
				Message:  "accepted datapack chunks narrative is fallback/stale in account_summary",
			})
		}
		if pulse := resp.SellerPulse.Narrative; pulse != nil && pulse.Status == "stale" {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "seller_pulse_stale_after_chunks",
				Severity: "failed",
				Message:  "seller_pulse.narrative is stale after completed chunks narrative",
			})
		}
		if ad := resp.AIDiagnostics.AdEfficiency.Narrative; ad != nil && ad.Status == "pending" {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "ad_efficiency_narrative_pending",
				Severity: "failed",
				Message:  "ad_efficiency_narrative must not be pending after completed chunks narrative",
			})
		}
	}

	if len(summary.RiskDrivers) > 0 {
		msg := strings.TrimSpace(resp.AIDiagnostics.RiskDistribution.Message)
		if noRisksMessagePattern.MatchString(msg) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "risk_distribution_contradicts_drivers",
				Severity: "warning",
				Message:  "risk_distribution claims no risks while account_summary has risk drivers",
			})
		}
	}

	if issue := validateCancellationRateDenominator(summary.RiskDrivers); issue != nil {
		issues = append(issues, *issue)
	}
	if issue := validateSurplusCountsConsistency(resp); issue != nil {
		issues = append(issues, *issue)
	}
	issues = append(issues, validateAdvertisingActionsConsistency(resp)...)
	issues = append(issues, validateSellerFacingTextHygiene(resp)...)

	return issues
}

func validateCancellationRateDenominator(drivers []RiskDriver) *DashboardHomeConsistencyIssue {
	for _, driver := range drivers {
		if driver.ID != "driver-cancellations" {
			continue
		}
		cancels, okC := evidenceInt(driver.Evidence, "cancels_today")
		orders, okO := evidenceInt(driver.Evidence, "orders_today")
		rate, okR := evidenceFloat(driver.Evidence, "cancellation_rate")
		if !okC || !okO || !okR || orders <= 0 {
			continue
		}
		expected := float64(cancels) / float64(orders) * 100
		if mathAbs(expected-rate) > 0.6 {
			return &DashboardHomeConsistencyIssue{
				Code:     "cancellation_rate_denominator_mismatch",
				Severity: "failed",
				Message: fmt.Sprintf(
					"cancellation_rate %.2f%% does not match %d/%d orders (expected %.2f%%)",
					rate, cancels, orders, expected,
				),
			}
		}
		if !strings.Contains(driver.Reason, fmt.Sprintf("%d отмен при %d заказах", cancels, orders)) {
			return &DashboardHomeConsistencyIssue{
				Code:     "cancellation_rate_text_denominator_mismatch",
				Severity: "warning",
				Message:  "cancellation driver text does not reference the same cancels/orders counts as evidence",
			}
		}
	}
	return nil
}

func validateSurplusCountsConsistency(resp *Response) *DashboardHomeConsistencyIssue {
	slowRisk := len(resp.SKUFocus.Surplus)
	var ozonSurplus int
	for _, seg := range resp.AIDiagnostics.StockCoverage.Segments {
		if seg.Key == "surplus" {
			ozonSurplus = seg.SKUCount
			break
		}
	}
	for _, driver := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		if driver.ID != "driver-surplus" {
			continue
		}
		count, ok := evidenceInt(driver.Evidence, "surplus_sku_count")
		if !ok {
			continue
		}
		if count > 0 && int(count) != slowRisk {
			return &DashboardHomeConsistencyIssue{
				Code:     "surplus_count_mismatch",
				Severity: "warning",
				Message: fmt.Sprintf(
					"driver-surplus count %d does not match sku_focus.surplus %d (stock_coverage.surplus ozon=%d)",
					count, slowRisk, ozonSurplus,
				),
			}
		}
		if count > 0 && ozonSurplus == 0 && slowRisk == 0 {
			return &DashboardHomeConsistencyIssue{
				Code:     "surplus_count_missing_support",
				Severity: "warning",
				Message:  "surplus driver is active but neither sku_focus.surplus nor stock_coverage.surplus reflect it",
			}
		}
	}
	return nil
}

func validateAdvertisingActionsConsistency(resp *Response) []DashboardHomeConsistencyIssue {
	campaignSpend := map[string]float64{}
	for _, row := range resp.AIDiagnostics.AdEfficiency.HighDRRCampaigns {
		if name := strings.TrimSpace(row.CampaignName); name != "" {
			campaignSpend[name] = row.Spend
		}
	}
	for _, row := range resp.AIDiagnostics.AdEfficiency.NoOrderCampaigns {
		if name := strings.TrimSpace(row.CampaignName); name != "" {
			campaignSpend[name] = row.Spend
		}
	}
	if len(campaignSpend) == 0 {
		return nil
	}

	issues := make([]DashboardHomeConsistencyIssue, 0)
	checkActions := func(codePrefix string, actions []TopActionItem) {
		for _, action := range actions {
			if action.Category != "advertising" {
				continue
			}
			for name, spend := range campaignSpend {
				if !strings.Contains(action.Reason, name) && !strings.Contains(action.Title, name) {
					continue
				}
				if conflictingSpendInText(action.Reason, spend) || conflictingSpendInText(action.Title, spend) {
					issues = append(issues, DashboardHomeConsistencyIssue{
						Code:     codePrefix + "_ad_spend_conflict",
						Severity: "warning",
						Message:  fmt.Sprintf("action for campaign %q may cite spend that conflicts with ad_efficiency rows", name),
					})
				}
			}
		}
	}
	checkActions("top_actions", resp.AIDiagnostics.TopActions)
	return issues
}

func conflictingSpendInText(text string, expectedSpend float64) bool {
	// Heuristic: if text mentions a substantially different ₽ amount near "расход"/"потрач", flag it.
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "₽") && !strings.Contains(lower, "руб") {
		return false
	}
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '(' || r == ')'
	}) {
		token = strings.TrimSuffix(strings.TrimSpace(token), "₽")
		token = strings.TrimSuffix(token, "руб")
		token = strings.TrimSuffix(token, "руб.")
		if token == "" {
			continue
		}
		var value float64
		if _, err := fmt.Sscanf(strings.ReplaceAll(token, " ", ""), "%f", &value); err != nil {
			continue
		}
		if value > 100 && mathAbs(value-expectedSpend) > expectedSpend*0.25 && mathAbs(value-expectedSpend) > 500 {
			return true
		}
	}
	return false
}

func validateSellerFacingTextHygiene(resp *Response) []DashboardHomeConsistencyIssue {
	issues := make([]DashboardHomeConsistencyIssue, 0)
	check := func(code, text string) {
		if factIDInTextPattern.MatchString(strings.ToLower(text)) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     code,
				Severity: "warning",
				Message:  "seller-facing text contains fact_id/entity_id tokens",
			})
		}
	}
	summary := resp.AIDiagnostics.AccountSummary
	check("account_headline_fact_id", summary.Headline)
	check("account_executive_summary_fact_id", summary.ExecutiveSummary)
	if pulse := resp.SellerPulse.Narrative; pulse != nil {
		check("seller_pulse_headline_fact_id", pulse.Headline)
		check("seller_pulse_summary_fact_id", pulse.Summary)
	}
	return issues
}

func consistencyStatusFromIssues(issues []DashboardHomeConsistencyIssue) string {
	status := "passed"
	for _, issue := range issues {
		if issue.Severity == "failed" {
			return "failed"
		}
		if issue.Severity == "warning" {
			status = "warning"
		}
	}
	return status
}

func buildDashboardNarrativeDebug(resp *Response, issues []DashboardHomeConsistencyIssue) *DashboardNarrativeDebug {
	if resp == nil {
		return nil
	}
	stored := resp.AIDiagnostics.AccountSummary.storedNarrative
	debug := &DashboardNarrativeDebug{
		AIOnly:                  dashboardAIOnlyEnabled(),
		AppliedBlocks:           map[string]string{},
		RequiredBlocks:          map[string]string{},
		OverwrittenByDeprecated: []string{},
		ConsistencyIssues:       issues,
		ConsistencyStatus:       consistencyStatusFromIssues(issues),
		SubstitutionBlocks:      []string{},
		DeprecatedBlocks:        []string{},
		QueuedBlocks:            []string{},
		OutdatedBlocks:          []string{},
	}
	if stored != nil {
		debug.LatestRunID = stored.ID
		debug.PromptVersion = stored.PromptVersion
		debug.Status = stored.Status
		if stored.Payload != nil {
			for _, block := range classifyAIOnlyRequiredBlocks(stored.Payload) {
				debug.RequiredBlocks[block.name] = block.presence
				switch block.kind {
				case "substitution":
					debug.SubstitutionBlocks = append(debug.SubstitutionBlocks, block.name)
				case "deprecated":
					debug.DeprecatedBlocks = append(debug.DeprecatedBlocks, block.name)
				case "queued":
					debug.QueuedBlocks = append(debug.QueuedBlocks, block.name)
				case "outdated":
					debug.OutdatedBlocks = append(debug.OutdatedBlocks, block.name)
				}
			}
		}
	}
	summary := resp.AIDiagnostics.AccountSummary
	accountSource := appliedBlockSource(summary.NarrativeSource == "ai")
	debug.AppliedBlocks["account_summary"] = accountSource
	if pulse := resp.SellerPulse.Narrative; pulse != nil {
		debug.AppliedBlocks["seller_pulse_narrative"] = sellerPulseAppliedSourceAIOnly(pulse.Status)
	}
	if ad := resp.AIDiagnostics.AdEfficiency.Narrative; ad != nil {
		debug.AppliedBlocks["ad_efficiency_narrative"] = narrativeBlockAppliedSourceAIOnly(ad.Status)
	}
	if resp.AIDiagnostics.StockCoverage.Narrative != nil {
		debug.AppliedBlocks["stock_coverage_narrative"] = narrativeBlockAppliedSourceAIOnly(resp.AIDiagnostics.StockCoverage.Narrative.Status)
	}
	if resp.AIDiagnostics.WhatChanged.Status == WidgetAvailable && summary.NarrativeSource == "ai" {
		if src := blockApplySource(resp, "what_changed"); src != "" {
			debug.AppliedBlocks["what_changed"] = src
		} else {
			debug.AppliedBlocks["what_changed"] = "ai"
		}
	} else if src := blockApplySource(resp, "what_changed"); src != "" {
		debug.AppliedBlocks["what_changed"] = src
	} else if summary.NarrativeSource == "ai" && len(resp.AIDiagnostics.WhatChanged.Groups) > 0 {
		debug.AppliedBlocks["what_changed"] = "ai"
	} else {
		debug.AppliedBlocks["what_changed"] = appliedBlockSource(false)
	}
	if len(resp.AIDiagnostics.TopActions) > 0 && summary.NarrativeSource == "ai" {
		debug.AppliedBlocks["top_actions"] = "ai"
	} else {
		debug.AppliedBlocks["top_actions"] = appliedBlockSource(false)
	}
	debug.AppliedBlocks["recommended_actions"] = "deprecated_dashboard_block"
	if resp.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status == WidgetAvailable && summary.NarrativeSource == "ai" {
		debug.AppliedBlocks["funnel_ai_analysis"] = "ai"
	}
	if len(resp.AIDiagnostics.CategorySummaries) > 0 && summary.NarrativeSource == "ai" {
		debug.AppliedBlocks["category_summaries"] = "ai"
	}
	if resp.AIDiagnostics.RiskDistribution.Status == WidgetAvailable && summary.NarrativeSource == "ai" {
		debug.AppliedBlocks["risk_overview"] = "ai"
	}
	debug.FinalPayloadSource = FinalPayloadSourceDatapackChunksSynthesis
	debug.SubstitutionUsed = hasDeterministicSubstitution(debug.AppliedBlocks)
	if dashboardAIOnlyEnabled() && debug.SubstitutionUsed {
		debug.ConsistencyStatus = "failed"
		issues = append(issues, DashboardHomeConsistencyIssue{
			Code: "debug_applied_blocks_misreport", Severity: "failed",
			Message: "AI-only dashboard has deterministic block substitution",
		})
		debug.ConsistencyIssues = issues
	}
	debug.SourceMap = buildDashboardNarrativeSourceMap(resp)
	debug.ApplyRepairs = narrativeApplyRepairs(resp)
	if nd := resp.AIDiagnostics.AccountSummary.NarrativeDebug; nd != nil && len(nd.RepairActions) > 0 {
		debug.AcceptanceRepairLog = append([]string(nil), nd.RepairActions...)
	}
	return debug
}

func hasDeterministicSubstitution(blocks map[string]string) bool {
	for _, src := range blocks {
		switch src {
		case "substituted_by_deterministic", "deterministic_substitution", "substitution":
			return true
		}
	}
	return false
}

func appliedBlockSource(isAI bool) string {
	if isAI {
		return "ai"
	}
	if dashboardAIOnlyEnabled() {
		return "missing"
	}
	return "substitution"
}

func sellerPulseAppliedSourceAIOnly(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available":
		return "ai"
	default:
		return appliedBlockSource(false)
	}
}

func narrativeBlockAppliedSourceAIOnly(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "available", "ready":
		return "ai"
	default:
		return appliedBlockSource(false)
	}
}

type aiOnlyBlockPresence struct {
	name     string
	presence string
	kind     string
}

func classifyAIOnlyRequiredBlocks(payload *dashboardNarrativePayload) []aiOnlyBlockPresence {
	out := []aiOnlyBlockPresence{
		{name: "account_summary", presence: presenceFromBool(strings.TrimSpace(payload.Headline) != "" && strings.TrimSpace(payload.ExecutiveSummary) != "")},
		{name: "seller_pulse_narrative", presence: presenceFromBool(payload.SellerPulseNarrative != nil), kind: blockKindFromStatus(sellerPulseStatus(payload))},
		{name: "what_changed", presence: presenceFromBool(payload.WhatChanged != nil && len(payload.WhatChanged.Groups) > 0)},
		{name: "top_actions", presence: presenceFromBool(len(payload.TopActions) >= 3)},
		{name: "recommended_actions", presence: "deprecated", kind: "deprecated"},
		{name: "stock_coverage_narrative", presence: presenceFromBool(payload.StockCoverageNarrative != nil), kind: blockKindFromStatus(blockStatus(payload.StockCoverageNarrative))},
		{name: "ad_efficiency_narrative", presence: presenceFromBool(payload.AdEfficiencyNarrative != nil), kind: blockKindFromStatus(blockStatus(payload.AdEfficiencyNarrative))},
		{name: "funnel_ai_analysis", presence: presenceFromBool(payload.FunnelAIAnalysis != nil), kind: funnelAIBlockKind(payload)},
		{name: "category_summaries", presence: presenceFromBool(len(payload.CategorySummaries) > 0)},
		{name: "risk_overview", presence: presenceFromBool(payload.RiskOverview != nil), kind: riskOverviewBlockKind(payload)},
	}
	return out
}

func funnelAIBlockKind(payload *dashboardNarrativePayload) string {
	if payload == nil || payload.FunnelAIAnalysis == nil {
		return ""
	}
	return blockKindFromStatus(string(payload.FunnelAIAnalysis.Status))
}

func riskOverviewBlockKind(payload *dashboardNarrativePayload) string {
	if payload == nil || payload.RiskOverview == nil {
		return ""
	}
	return blockKindFromStatus(string(payload.RiskOverview.Status))
}

func sellerPulseStatus(payload *dashboardNarrativePayload) string {
	if payload == nil || payload.SellerPulseNarrative == nil {
		return ""
	}
	return payload.SellerPulseNarrative.Status
}

func blockStatus(v any) string {
	switch n := v.(type) {
	case *StockCoverageNarrative:
		if n == nil {
			return ""
		}
		return n.Status
	case *AdEfficiencyNarrative:
		if n == nil {
			return ""
		}
		return n.Status
	default:
		return ""
	}
}

func presenceFromBool(ok bool) string {
	if ok {
		return "present"
	}
	return "missing"
}

func blockKindFromStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "fallback", "fallback_stale":
		return "substitution"
	case "legacy":
		return "deprecated"
	case "pending", "source_pending":
		return "queued"
	case "stale":
		return "outdated"
	default:
		return ""
	}
}

func sellerPulseAppliedSource(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available":
		return "ai"
	case "fallback", "stale", "pending":
		return status
	default:
		return status
	}
}

func narrativeBlockAppliedSource(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "available", "ready":
		return "ai"
	case "fallback", "stale", "pending":
		return status
	default:
		if status == "" {
			return "missing"
		}
		return status
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
