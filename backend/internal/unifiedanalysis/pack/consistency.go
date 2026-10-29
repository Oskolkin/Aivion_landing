package pack

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

// BusinessConsistencyIssue is one business-level error or warning.
type BusinessConsistencyIssue struct {
	Code    string `json:"code"`
	SKU     string `json:"sku,omitempty"`
	Message string `json:"message"`
}

// BusinessConsistencyReport summarizes snapshot/Pack business coherence.
type BusinessConsistencyReport struct {
	EligibleSKUs              int                        `json:"eligible_skus"`
	ExcludedSKUs              int                        `json:"excluded_skus"`
	Errors                    []BusinessConsistencyIssue `json:"errors"`
	Warnings                  []BusinessConsistencyIssue `json:"warnings"`
	AccountSKUOrdersDeltaPct  float64                    `json:"account_sku_orders_delta_pct"`
	AccountSKURevenueDeltaPct float64                    `json:"account_sku_revenue_delta_pct"`
	StockConflictsResolved    int                        `json:"stock_conflicts_resolved"`
	Eligibility               SKUEligibilityDiagnostics  `json:"eligibility"`
}

// HasErrors reports whether the pack must not be sent to OpenAI.
func (r BusinessConsistencyReport) HasErrors() bool {
	return len(r.Errors) > 0
}

// SummaryLine returns a compact human-readable summary.
func (r BusinessConsistencyReport) SummaryLine() string {
	return fmt.Sprintf(
		"Business consistency: eligible_skus=%d excluded_skus=%d errors=%d warnings=%d account_sku_orders_delta_pct=%.2f account_sku_revenue_delta_pct=%.2f stock_conflicts_resolved=%d",
		r.EligibleSKUs, r.ExcludedSKUs, len(r.Errors), len(r.Warnings),
		r.AccountSKUOrdersDeltaPct, r.AccountSKURevenueDeltaPct, r.StockConflictsResolved,
	)
}

// ValidateBusinessConsistency checks Pack indexes + skuFacts for critical contradictions.
func ValidateBusinessConsistency(idx *Indexes, facts map[string]skuFacts, periods Periods, metricDate string) BusinessConsistencyReport {
	report := BusinessConsistencyReport{
		EligibleSKUs: len(idx.SKUUniverse),
		ExcludedSKUs: idx.Eligibility.TotalDiscovered - idx.Eligibility.AnalysisEligible,
		Eligibility:  idx.Eligibility,
	}
	addErr := func(code, sku, msg string) {
		report.Errors = append(report.Errors, BusinessConsistencyIssue{Code: code, SKU: sku, Message: msg})
	}
	addWarn := func(code, sku, msg string) {
		report.Warnings = append(report.Warnings, BusinessConsistencyIssue{Code: code, SKU: sku, Message: msg})
	}

	// Period windows.
	if periods.Current7D.From == "" || periods.Current7D.To == "" || periods.Previous7D.From == "" {
		addErr("invalid_periods", "", "current/previous 7d windows are incomplete")
	} else {
		if periods.Current7D.From <= periods.Previous7D.To && periods.Previous7D.From <= periods.Current7D.To {
			// Adjacent non-overlapping is required: previous.To must be day before current.From.
			prevTo, err1 := time.Parse("2006-01-02", periods.Previous7D.To)
			curFrom, err2 := time.Parse("2006-01-02", periods.Current7D.From)
			if err1 == nil && err2 == nil {
				if !prevTo.AddDate(0, 0, 1).Equal(curFrom) {
					addErr("overlapping_windows", "", fmt.Sprintf("previous_7d.to=%s must be day before current_7d.from=%s", periods.Previous7D.To, periods.Current7D.From))
				}
			}
		}
		if periods.Current7D.To > metricDate {
			addErr("data_after_metric_date", "", fmt.Sprintf("current_7d.to=%s > metric_date=%s", periods.Current7D.To, metricDate))
		}
	}

	var skuOrders, skuRevenue float64
	for _, sku := range idx.SKUUniverse {
		f := facts[sku]
		m := idx.SKUMeta[sku]

		if m == nil || strings.TrimSpace(m.Title) == "" {
			addErr("sku_missing_title", sku, "analysis-eligible SKU has empty title")
		}
		if m == nil || strings.TrimSpace(m.OfferID) == "" {
			addErr("sku_missing_offer_id", sku, "analysis-eligible SKU has empty offer_id")
		}
		if _, ok := idx.ProductBySKU[sku]; !ok {
			addErr("sku_missing_product", sku, "analysis-eligible SKU has no seller product record")
		}
		id, err := strconv.ParseInt(sku, 10, 64)
		if err != nil || id <= 0 {
			addErr("sku_invalid_id", sku, "SKU id must be a positive integer")
		}

		if f.Orders7D > 0 && f.DaysWithoutSales > 6 {
			addErr("sales_vs_no_sales", sku, fmt.Sprintf("orders_7d=%d but days_without_sales=%d (>6)", f.Orders7D, f.DaysWithoutSales))
		}
		if f.HasStockData && f.AvailableStock < 0 {
			addErr("negative_stock", sku, "available stock is negative")
		}
		if f.Coverage7D != nil && *f.Coverage7D < 0 {
			addErr("negative_coverage", sku, "coverage_7d is negative")
		}
		if f.Coverage30D != nil && *f.Coverage30D < 0 {
			addErr("negative_coverage", sku, "coverage_30d is negative")
		}

		if f.HasStockData && f.AvailableStock == 0 {
			if f.StockStatus == stockpolicy.StatusHealthy || f.StockStatus == stockpolicy.StatusSurplus120D {
				addErr("zero_stock_healthy", sku, fmt.Sprintf("stock=0 but status=%s", f.StockStatus))
			}
		}
		if f.Coverage7D != nil {
			c := *f.Coverage7D
			if c <= 3 && f.StockStatus != "" && !stockpolicy.IsDeficitStatus(f.StockStatus) && f.StockStatus != stockpolicy.StatusOutOfStock && f.Orders7D > 0 {
				addErr("coverage_vs_status", sku, fmt.Sprintf("coverage_7d=%.2f but status=%s", c, f.StockStatus))
			}
			if c > 14 && stockpolicy.IsDeficitStatus(f.StockStatus) && f.StockStatus != stockpolicy.StatusOutOfStock {
				addErr("coverage_vs_critical", sku, fmt.Sprintf("coverage_7d=%.2f but deficit status=%s", c, f.StockStatus))
			}
		}
		if f.Orders7D > 0 && f.StockStatus == stockpolicy.StatusNoSales {
			addErr("active_sales_no_sales_status", sku, "orders_7d>0 with no_sales status")
		}

		// Daily vs summary reconciliation for current window.
		byDate := idx.SKUMetricsByDate[sku]
		var dailyOrders int64
		var dailyRevenue float64
		for _, d := range dateRangeInclusive(periods.Current7D.From, periods.Current7D.To) {
			if day, ok := byDate[d]; ok {
				dailyOrders += day.Orders
				dailyRevenue += day.Revenue
			}
		}
		if dailyOrders != f.Orders7D {
			addErr("summary_vs_daily_orders", sku, fmt.Sprintf("orders_7d=%d daily_sum=%d", f.Orders7D, dailyOrders))
		}
		if math.Abs(dailyRevenue-f.Revenue7D) > 0.02 {
			addErr("summary_vs_daily_revenue", sku, fmt.Sprintf("revenue_7d=%.2f daily_sum=%.2f", f.Revenue7D, dailyRevenue))
		}

		if len(f.StockConflictFlags) > 0 {
			report.StockConflictsResolved += len(f.StockConflictFlags)
			addWarn("stock_signal_conflict", sku, strings.Join(f.StockConflictFlags, ","))
		}
	}

	// Account ↔ SKU reconciliation uses ALL seller product-backed SKUs (including
	// archived/ineligible), because daily_account_metrics is built from the full catalog.
	// Analysis-eligible filtering must not be mistaken for an account source mismatch.
	for sku := range idx.ProductBySKU {
		byDate := idx.SKUMetricsByDate[sku]
		for _, d := range dateRangeInclusive(periods.Current7D.From, periods.Current7D.To) {
			if day, ok := byDate[d]; ok {
				skuOrders += float64(day.Orders)
				skuRevenue += day.Revenue
			}
		}
	}

	account := sumAccountWindow(idx, periods.Current7D.From, periods.Current7D.To)
	if account.Orders > 0 {
		report.AccountSKUOrdersDeltaPct = math.Round(((skuOrders-float64(account.Orders))/float64(account.Orders)*100)*100) / 100
	} else if skuOrders > 0 {
		report.AccountSKUOrdersDeltaPct = 100
	}
	if account.Revenue > 0 {
		report.AccountSKURevenueDeltaPct = math.Round(((skuRevenue-account.Revenue)/account.Revenue*100)*100) / 100
	} else if skuRevenue > 0 {
		report.AccountSKURevenueDeltaPct = 100
	}
	// Demo seed writes one order == one unit with SKU sales matching account;
	// tolerate tiny float noise but flag material mismatch.
	if math.Abs(report.AccountSKUOrdersDeltaPct) > 1.0 {
		addErr("account_sku_orders_mismatch", "", fmt.Sprintf("delta_pct=%.2f account=%d sku_sum=%.0f (product-backed SKUs)", report.AccountSKUOrdersDeltaPct, account.Orders, skuOrders))
	}
	if math.Abs(report.AccountSKURevenueDeltaPct) > 1.0 {
		addErr("account_sku_revenue_mismatch", "", fmt.Sprintf("delta_pct=%.2f account=%.2f sku_sum=%.2f (product-backed SKUs)", report.AccountSKURevenueDeltaPct, account.Revenue, skuRevenue))
	}

	if report.ExcludedSKUs > 0 {
		addWarn("excluded_skus_present", "", fmt.Sprintf("%d discovered SKUs excluded from AI universe (archived/technical/historical/incomplete)", report.ExcludedSKUs))
	}

	for _, cid := range idx.CampaignUniverse {
		m := idx.CampaignMeta[cid]
		if m == nil {
			addErr("campaign_missing_meta", strconv.FormatInt(cid, 10), "campaign missing metadata")
			continue
		}
		if cid <= 0 {
			addErr("campaign_invalid_id", strconv.FormatInt(cid, 10), "campaign id must be positive")
		}
		status := strings.ToUpper(strings.TrimSpace(m.Status))
		if status == "" {
			addWarn("campaign_missing_status", strconv.FormatInt(cid, 10), "campaign status empty")
		}
	}

	return report
}
