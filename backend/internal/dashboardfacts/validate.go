package dashboardfacts

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
)

func validateDashboardFacts(facts DashboardFactsV1) FactsValidationReport {
	report := FactsValidationReport{Valid: true}

	if facts.SchemaVersion != SchemaVersion {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "invalid_schema_version", Path: "schema_version",
			Message: fmt.Sprintf("expected %q", SchemaVersion),
		})
	}
	if facts.SellerAccountID <= 0 {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "invalid_seller_account_id", Path: "seller_account_id",
			Message: "seller_account_id must be positive",
		})
	}
	if strings.TrimSpace(facts.MetricDate) == "" {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_metric_date", Path: "metric_date",
			Message: "metric_date is required",
		})
	}
	if strings.TrimSpace(facts.PackID) == "" {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_pack_id", Path: "pack_id",
			Message: "pack_id is required",
		})
	}
	if strings.TrimSpace(facts.PackHash) == "" {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_pack_hash", Path: "pack_hash",
			Message: "pack_hash is required",
		})
	}
	if strings.TrimSpace(facts.FactsHash) == "" {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_facts_hash", Path: "facts_hash",
			Message: "facts_hash is required",
		})
	}

	hasRevenueKPI := false
	hasOrdersKPI := false
	for _, kpi := range facts.SellerPulse.KPIs {
		switch kpi.Key {
		case "revenue":
			hasRevenueKPI = true
		case "orders":
			hasOrdersKPI = true
		}
	}
	if facts.DashboardMetrics.Account.RevenueToday > 0 && !hasRevenueKPI {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_revenue_kpi", Path: "seller_pulse.kpis",
			Message: "revenue KPI required when account revenue is present",
		})
	}
	if facts.DashboardMetrics.Account.OrdersToday > 0 && !hasOrdersKPI {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_orders_kpi", Path: "seller_pulse.kpis",
			Message: "orders KPI required when account orders are present",
		})
	}

	for i, day := range facts.SellerPulse.Trend7D {
		if day.Revenue == nil && day.Orders == nil {
			report.Errors = append(report.Errors, FactsValidationIssue{
				Code: "empty_trend_day", Path: fmt.Sprintf("seller_pulse.trend_7d[%d]", i),
				Message: "trend day must include observed revenue or orders",
			})
		}
	}

	if !hasKPIKey(facts.SellerPulse.KPIs, "average_order_value") &&
		facts.DashboardMetrics.Account.OrdersToday > 0 {
		report.Warnings = append(report.Warnings, FactsValidationIssue{
			Code: "missing_aov_kpi", Path: "seller_pulse.kpis",
			Message: "average_order_value KPI omitted despite orders_today > 0",
		})
	}
	if !hasKPIKey(facts.SellerPulse.KPIs, "ad_spend_share_total_revenue") &&
		!hasKPIKey(facts.SellerPulse.KPIs, "drr") &&
		facts.AdEfficiency.Status == "available" {
		report.Warnings = append(report.Warnings, FactsValidationIssue{
			Code: "missing_ad_spend_share_kpi", Path: "seller_pulse.kpis",
			Message: "ad_spend_share_total_revenue KPI omitted despite ad efficiency availability",
		})
	}

	// The seller's catalog is categorized (DataCoverage.CategoryCount > 0)
	// but buildCategoryFacts produced nothing at all — a dropped/failed
	// build rather than a legitimate "no same-day category metrics" state,
	// which would instead leave Categories partially populated.
	if facts.DataCoverage.CategoryCount > 0 && len(facts.Categories) == 0 {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "missing_category_facts", Path: "categories",
			Message: "seller catalog has categories but categories facts are empty",
		})
	}

	if len(facts.SKUFactsIndex.ByOzonProductID) > 0 && facts.DataCoverage.SKUMetricsDays >= 2 && facts.ContentCounts.SKUsWithSparkline == 0 {
		report.Warnings = append(report.Warnings, FactsValidationIssue{
			Code: "missing_sku_sparklines", Path: "sku_facts_index",
			Message: "sku metrics span 2+ days but no sku carries a multi-day sparkline",
		})
	}

	for _, issue := range validateAccountDRRConsistency(facts) {
		report.Errors = append(report.Errors, issue)
	}

	if len(report.Errors) > 0 {
		report.Valid = false
	}
	return report
}

// appendSourceReconciliationIssues runs the shared snapshot reconciliation
// (account ↔ SKU ↔ category ↔ advertising ↔ funnel ↔ stock, plus seller
// identity / period / no-negatives) against the same snapshot the facts were
// built from, and folds its findings into the facts validation report. Any
// reconciliation error fails facts validation; reconciliation warnings are
// surfaced as facts warnings.
func appendSourceReconciliationIssues(report *FactsValidationReport, snapshot *analysisdata.AnalysisDataSnapshot) {
	if snapshot == nil {
		return
	}
	recon, err := sourcerecon.ReconcileSnapshot(snapshot)
	if err != nil {
		report.Errors = append(report.Errors, FactsValidationIssue{
			Code: "source_reconciliation_error", Path: "snapshot", Message: err.Error(),
		})
		report.Valid = false
		return
	}
	for _, issue := range recon.Errors {
		report.Errors = append(report.Errors, reconIssueToFacts(issue))
	}
	for _, issue := range recon.Warnings {
		report.Warnings = append(report.Warnings, reconIssueToFacts(issue))
	}
	if len(report.Errors) > 0 {
		report.Valid = false
	}
}

func reconIssueToFacts(issue sourcerecon.ReconciliationIssue) FactsValidationIssue {
	parts := make([]string, 0, 3)
	if issue.Date != "" {
		parts = append(parts, "date="+issue.Date)
	}
	if issue.EntityID != "" {
		parts = append(parts, "entity="+issue.EntityID)
	}
	if issue.Expected != nil || issue.Actual != nil {
		parts = append(parts, fmt.Sprintf("expected=%v actual=%v", issue.Expected, issue.Actual))
	}
	if issue.Message != "" {
		parts = append(parts, issue.Message)
	}
	return FactsValidationIssue{
		Code:    "recon_" + issue.Code,
		Path:    issue.Path,
		Message: strings.Join(parts, " "),
	}
}

func hasKPIKey(kpis []KPICardFact, key string) bool {
	for _, kpi := range kpis {
		if kpi.Key == key {
			return true
		}
	}
	return false
}

func buildContentCounts(facts DashboardFactsV1) ContentCounts {
	segmentsWithSKUs := 0
	for _, seg := range facts.StockCoverage.Segments {
		if seg.SKUCount > 0 {
			segmentsWithSKUs++
		}
	}
	return ContentCounts{
		SellerPulseKPIs:      len(facts.SellerPulse.KPIs),
		SellerPulseTrendDays: len(facts.SellerPulse.Trend7D),
		StockSegments:        segmentsWithSKUs,
		FunnelStages:         len(facts.Funnel.Stages),
		AdPoints:             countAdPointsWithData(facts.AdEfficiency.Points),
		SKUIndexEntries:      len(facts.SKUFactsIndex.ByOzonProductID),
		SKUsWithSparkline:    countSKUsWithSparkline(facts.SKUFactsIndex),
		CategoryCount:        len(facts.Categories),
	}
}
