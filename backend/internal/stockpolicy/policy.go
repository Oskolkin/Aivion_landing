// Package stockpolicy owns canonical Aivion stock-health facts.
//
// Raw Ozon signals (turnover_grade, ozon days_without_sales, IDC) are inputs for
// conflict detection only. Final stock_status / coverage buckets used by Analysis
// Pack, Dashboard Facts, and health counters must come from these helpers so the
// layers stay consistent.
package stockpolicy

import "math"

// Contract status strings used in Analysis Pack sku_summary.stock_status and
// analytics.StockStatusBucket. Prefer existing enum values to avoid prompt/schema migration.
const (
	StatusOutOfStock       = "out_of_stock"
	StatusCriticalShortage = "critical_shortage"
	StatusShortage7D       = "shortage_7d"
	StatusPlanning14D      = "planning_14d"
	StatusHealthy          = "healthy"
	StatusSurplus120D      = "surplus_120d"
	StatusNoSales          = "no_sales"
	StatusUnknown          = "unknown"
	StatusCollectingData   = "collecting_data"
)

// Coverage thresholds (days of cover).
const (
	CoverageCriticalMax = 3.0
	CoverageShortageMax = 7.0
	CoveragePlanningMax = 14.0
	CoverageSurplusMin  = 60.0
)

// PersistentNoSalesMinDays is the minimum consecutive zero-sales streak that
// qualifies as a no_sales stock status when stock remains available.
const PersistentNoSalesMinDays int64 = 14

// Input gathers the canonical signals needed to derive stock status.
type Input struct {
	AvailableStock    int64
	HasStockData      bool
	CoverageDays      *float64 // prefer 7d coverage when available; else 30d
	Orders7D          int64
	DaysWithoutSales  *int64 // canonical value calculated from daily history
	HasSalesHistory   bool   // true when enough daily observations exist to judge demand
	OzonTurnoverGrade string // raw; never unconditionally overrides coverage
	ExcessStockShare  *float64
}

// Result is the canonical stock status plus provenance / conflict flags.
type Result struct {
	Status        string
	Source        string
	ConflictFlags []string
}

// ComputeCanonicalStockStatus derives the Aivion stock_status from stock,
// coverage, and sales facts. Raw Ozon turnover_grade does not override coverage.
func ComputeCanonicalStockStatus(in Input) Result {
	var conflicts []string
	if in.OzonTurnoverGrade != "" {
		conflicts = appendConflictIfGradeDisagrees(conflicts, in)
	}

	if !in.HasStockData {
		return Result{Status: "", Source: "no_stock_data", ConflictFlags: conflicts}
	}

	if in.AvailableStock < 0 {
		return Result{Status: StatusUnknown, Source: "invalid_stock", ConflictFlags: append(conflicts, "negative_stock")}
	}

	if in.AvailableStock == 0 {
		return Result{Status: StatusOutOfStock, Source: "canonical_stock", ConflictFlags: conflicts}
	}

	// Stock present + prolonged no sales → no_sales (even if Ozon grade says otherwise).
	if in.DaysWithoutSales != nil && *in.DaysWithoutSales >= PersistentNoSalesMinDays && in.Orders7D == 0 {
		return Result{Status: StatusNoSales, Source: "canonical_days_without_sales", ConflictFlags: conflicts}
	}

	// Active sales must never resolve to no_sales, even when raw grade is NO_SALES.
	if in.Orders7D > 0 && (in.OzonTurnoverGrade == "NO_SALES" || in.OzonTurnoverGrade == "RESTRICTED_NO_SALES") {
		conflicts = appendUnique(conflicts, "ozon_no_sales_vs_active_sales")
	}

	if in.ExcessStockShare != nil && *in.ExcessStockShare >= 0.5 {
		return Result{Status: StatusSurplus120D, Source: "excess_stock_share", ConflictFlags: conflicts}
	}

	if in.CoverageDays == nil {
		if !in.HasSalesHistory {
			return Result{Status: StatusCollectingData, Source: "insufficient_sales_history", ConflictFlags: conflicts}
		}
		// Stock without measurable velocity: treat as unknown rather than healthy.
		return Result{Status: StatusUnknown, Source: "coverage_unavailable", ConflictFlags: conflicts}
	}

	c := *in.CoverageDays
	if c < 0 {
		return Result{Status: StatusUnknown, Source: "invalid_coverage", ConflictFlags: append(conflicts, "negative_coverage")}
	}

	switch {
	case c <= CoverageCriticalMax:
		return Result{Status: StatusCriticalShortage, Source: "canonical_coverage", ConflictFlags: conflicts}
	case c <= CoverageShortageMax:
		return Result{Status: StatusShortage7D, Source: "canonical_coverage", ConflictFlags: conflicts}
	case c <= CoveragePlanningMax:
		return Result{Status: StatusPlanning14D, Source: "canonical_coverage", ConflictFlags: conflicts}
	case c >= CoverageSurplusMin:
		return Result{Status: StatusSurplus120D, Source: "canonical_coverage", ConflictFlags: conflicts}
	default:
		return Result{Status: StatusHealthy, Source: "canonical_coverage", ConflictFlags: conflicts}
	}
}

// Canonical coverage / stock-focus segment keys (mutually exclusive).
const (
	SegmentOutOfStock    = "out_of_stock"
	SegmentCritical0_3   = "critical_0_3"
	SegmentShortage4_7   = "shortage_4_7"
	SegmentPlanning8_14  = "planning_8_14"
	SegmentHealthy14Plus = "healthy_14plus"
	SegmentSurplus       = "surplus"
	SegmentNoSales       = "no_sales"
	SegmentCollecting    = "collecting_data"
)

// MapToCoverageSegment maps a canonical status to dashboard StockCoverage segment keys.
// Segments are mutually exclusive: every status maps to exactly one key.
func MapToCoverageSegment(status string) string {
	switch status {
	case StatusOutOfStock:
		return SegmentOutOfStock
	case StatusCriticalShortage:
		return SegmentCritical0_3
	case StatusShortage7D:
		return SegmentShortage4_7
	case StatusPlanning14D:
		return SegmentPlanning8_14
	case StatusSurplus120D:
		return SegmentSurplus
	case StatusNoSales:
		return SegmentNoSales
	case StatusHealthy:
		return SegmentHealthy14Plus
	default:
		return SegmentCollecting
	}
}

// ClassifyCoverageSegment is the single canonical stock classification used by
// Stock Coverage counters and SKU Focus membership/filters.
func ClassifyCoverageSegment(in Input) string {
	return MapToCoverageSegment(ComputeCanonicalStockStatus(in).Status)
}

// IsStockFocusAttentionSegment reports risk segments shown under «Все риски».
// planning_8_14 is control/planning, not all-risk.
func IsStockFocusAttentionSegment(segment string) bool {
	switch segment {
	case SegmentOutOfStock, SegmentCritical0_3, SegmentShortage4_7:
		return true
	default:
		return false
	}
}

// IsStockFocusStockTabSegment reports segments included in SKU Focus → Остатки.
func IsStockFocusStockTabSegment(segment string) bool {
	switch segment {
	case SegmentOutOfStock, SegmentCritical0_3, SegmentShortage4_7, SegmentPlanning8_14:
		return true
	default:
		return false
	}
}

// IsDeficitStatus reports whether status counts toward deficit_sku_count.
func IsDeficitStatus(status string) bool {
	switch status {
	case StatusCriticalShortage, StatusOutOfStock, "DEFICIT", "WAITING_FOR_SUPPLY":
		return true
	default:
		return false
	}
}

// IsSurplusStatus reports whether status counts toward surplus_sku_count.
func IsSurplusStatus(status string) bool {
	switch status {
	case StatusSurplus120D, "SURPLUS":
		return true
	default:
		return false
	}
}

// ClampDaysWithoutSalesWithOrders7D enforces: orders_7d > 0 ⇒ days_without_sales ≤ 6.
func ClampDaysWithoutSalesWithOrders7D(days, orders7d int64) int64 {
	if days < 0 {
		days = 0
	}
	if orders7d > 0 && days > 6 {
		return 6
	}
	return days
}

// DaysWithoutSalesFromObserved walks back from metricDate over observed daily
// order counts. Missing calendar days end the streak (sparse gap ≠ confirmed zero).
// datesNewestFirst must be YYYY-MM-DD sorted descending, only ≤ metricDate.
func DaysWithoutSalesFromObserved(ordersByDate map[string]int64, datesNewestFirst []string, orders7d int64) int64 {
	var count int64
	for _, d := range datesNewestFirst {
		orders, ok := ordersByDate[d]
		if !ok {
			break
		}
		if orders > 0 {
			break
		}
		count++
	}
	return ClampDaysWithoutSalesWithOrders7D(count, orders7d)
}

// CoverageFromStockAndADS returns stock / ads when ads > 0.
func CoverageFromStockAndADS(stock int64, ads float64) *float64 {
	if ads <= 0 || stock < 0 {
		return nil
	}
	v := float64(stock) / ads
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return nil
	}
	return &v
}

func appendConflictIfGradeDisagrees(conflicts []string, in Input) []string {
	grade := in.OzonTurnoverGrade
	if grade == "" || in.CoverageDays == nil {
		return conflicts
	}
	c := *in.CoverageDays
	switch grade {
	case "DEFICIT", "WAITING_FOR_SUPPLY":
		if c > CoveragePlanningMax {
			return appendUnique(conflicts, "ozon_deficit_vs_healthy_coverage")
		}
	case "SURPLUS":
		if c <= CoverageShortageMax && in.Orders7D > 0 {
			return appendUnique(conflicts, "ozon_surplus_vs_shortage_coverage")
		}
	case "NO_SALES", "RESTRICTED_NO_SALES":
		if in.Orders7D > 0 {
			return appendUnique(conflicts, "ozon_no_sales_vs_active_sales")
		}
	case "ACTUAL":
		if in.AvailableStock == 0 {
			return appendUnique(conflicts, "ozon_actual_vs_zero_stock")
		}
	}
	return conflicts
}

func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}
