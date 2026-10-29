package stockpolicy

import (
	"math"
	"time"
)

// Policy defaults for target coverage when seller-specific lead time is unknown.
// Always labelled as policy_default — never presented as an individual calculation.
const (
	DefaultLeadTimeDays        = 7.0
	DefaultReceivingBufferDays = 2.0
	DefaultSafetyStockDays     = 5.0
	TargetSourcePolicyDefault  = "policy_default"
	TargetSourceCalculated     = "calculated"
	TargetSourceUserConfigured = "user_configured"
	TargetSourceUnknown        = "unknown"
)

// CoverageDays7D is the canonical coverage formula:
//
//	avg_daily_sales_7d = sales_7d_units / 7
//	stock_coverage_days_7d = available_stock_units / avg_daily_sales_7d
//
// Rules:
//   - stock=0 and sales>0 → 0
//   - sales=0 → nil (no velocity; never treat as 0 coverage)
//   - never substitute sales into the coverage field
func CoverageDays7D(availableStock, sales7D int64) *float64 {
	if sales7D <= 0 {
		return nil
	}
	avg := float64(sales7D) / 7.0
	return CoverageFromStockAndADS(availableStock, avg)
}

// DefaultTargetCoverageDays returns lead_time + receiving_buffer + safety_stock
// using policy defaults when seller-specific values are unavailable.
func DefaultTargetCoverageDays() (days float64, source string) {
	return DefaultLeadTimeDays + DefaultReceivingBufferDays + DefaultSafetyStockDays, TargetSourcePolicyDefault
}

// TargetCoverageDays computes an individual target when inputs are present;
// otherwise falls back to the labelled policy default.
func TargetCoverageDays(leadTime, receivingBuffer, safetyStock *float64) (days float64, source string) {
	if leadTime == nil && receivingBuffer == nil && safetyStock == nil {
		return DefaultTargetCoverageDays()
	}
	lt := DefaultLeadTimeDays
	rb := DefaultReceivingBufferDays
	ss := DefaultSafetyStockDays
	source = TargetSourcePolicyDefault
	if leadTime != nil && *leadTime >= 0 {
		lt = *leadTime
		source = TargetSourceCalculated
	}
	if receivingBuffer != nil && *receivingBuffer >= 0 {
		rb = *receivingBuffer
		if source != TargetSourceCalculated {
			source = TargetSourceCalculated
		}
	}
	if safetyStock != nil && *safetyStock >= 0 {
		ss = *safetyStock
		if source != TargetSourceCalculated {
			source = TargetSourceCalculated
		}
	}
	return lt + rb + ss, source
}

// RecommendedSupplyUnits computes
//
//	max(0, ceil(avg_daily * target_days) - (stock + in_transit + planned))
func RecommendedSupplyUnits(availableStock, inTransit, planned int64, avgDailySales, targetCoverageDays float64) int64 {
	if avgDailySales <= 0 || targetCoverageDays <= 0 {
		return 0
	}
	targetUnits := avgDailySales * targetCoverageDays
	netAvailable := float64(availableStock + inTransit + planned)
	need := targetUnits - netAvailable
	if need <= 0 {
		return 0
	}
	return int64(math.Ceil(need))
}

// EstimatedStockoutDate projects when stock reaches zero at the current
// average daily sales rate. Nil when there is no velocity or stock is already
// non-positive (caller should treat stock=0 as "already stocked out").
func EstimatedStockoutDate(asOf time.Time, availableStock int64, avgDailySales float64) *time.Time {
	if availableStock <= 0 {
		t := asOf
		return &t
	}
	if avgDailySales <= 0 {
		return nil
	}
	days := float64(availableStock) / avgDailySales
	t := asOf.Add(time.Duration(days*24) * time.Hour)
	return &t
}

// RecommendedOrderDate suggests ordering by (stockout_date - lead_time),
// floored at asOf when the window has already passed.
func RecommendedOrderDate(asOf time.Time, stockout *time.Time, leadTimeDays float64) *time.Time {
	if stockout == nil {
		return nil
	}
	if leadTimeDays < 0 {
		leadTimeDays = DefaultLeadTimeDays
	}
	orderBy := stockout.Add(-time.Duration(leadTimeDays*24) * time.Hour)
	if orderBy.Before(asOf) {
		orderBy = asOf
	}
	return &orderBy
}

// SurplusExcessUnits estimates units above the surplus threshold coverage.
func SurplusExcessUnits(availableStock int64, avgDailySales float64, surplusThresholdDays float64) int64 {
	if availableStock <= 0 || avgDailySales <= 0 || surplusThresholdDays <= 0 {
		return 0
	}
	target := avgDailySales * surplusThresholdDays
	excess := float64(availableStock) - target
	if excess <= 0 {
		return 0
	}
	return int64(math.Floor(excess))
}
