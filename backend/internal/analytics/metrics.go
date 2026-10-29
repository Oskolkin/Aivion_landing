package analytics

import (
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

// RatioOrNil returns numerator/denominator rounded to 4 decimals, or nil when denominator <= 0.
func RatioOrNil(numerator, denominator float64) *float64 {
	if denominator <= 0 {
		return nil
	}
	v := math.Round((numerator/denominator)*10000) / 10000
	return &v
}

// PercentDeltaOrNil returns (current-previous)/previous*100, or nil when previous <= 0.
func PercentDeltaOrNil(current, previous float64) *float64 {
	if previous <= 0 {
		return nil
	}
	v := math.Round(((current-previous)/previous*100)*100) / 100
	return &v
}

// AverageDailyVelocity returns sum/days as a daily average (missing days in sum treated as 0).
func AverageDailyVelocity(sum int64, days int) *float64 {
	if days <= 0 {
		return nil
	}
	v := math.Round((float64(sum)/float64(days))*100) / 100
	return &v
}

// DiscountDepth returns (old_price - current_price) / old_price, or nil when invalid.
func DiscountDepth(oldPrice, currentPrice float64) *float64 {
	if oldPrice <= 0 || currentPrice <= 0 {
		return nil
	}
	return RatioOrNil(oldPrice-currentPrice, oldPrice)
}

// CampaignDRR is spend / revenue; nil when revenue <= 0 (never 100% at zero revenue).
func CampaignDRR(spend, revenue float64) *float64 {
	return RatioOrNil(spend, revenue)
}

// CampaignDRRPct is spend / revenue * 100; nil when revenue <= 0.
func CampaignDRRPct(spend, revenue float64) *float64 {
	if revenue <= 0 {
		return nil
	}
	v := math.Round((spend/revenue*100)*100) / 100
	return &v
}

// PercentagePointsDelta returns current-previous in percentage points; nil when either operand is nil.
func PercentagePointsDelta(current, previous *float64) *float64 {
	if current == nil || previous == nil {
		return nil
	}
	v := math.Round((*current-*previous)*100) / 100
	return &v
}

// CampaignROAS is revenue / spend.
func CampaignROAS(revenue, spend float64) *float64 {
	return RatioOrNil(revenue, spend)
}

// CampaignCPO is spend / orders_count.
func CampaignCPO(spend float64, orders int64) *float64 {
	return RatioOrNil(spend, float64(orders))
}

// CampaignCTR is clicks / impressions.
func CampaignCTR(clicks, impressions int64) *float64 {
	return RatioOrNil(float64(clicks), float64(impressions))
}

// CampaignCPC is spend / clicks.
func CampaignCPC(spend float64, clicks int64) *float64 {
	return RatioOrNil(spend, float64(clicks))
}

// CampaignClickToOrderRate is orders_count / clicks.
func CampaignClickToOrderRate(orders, clicks int64) *float64 {
	return RatioOrNil(float64(orders), float64(clicks))
}

// CampaignSpendShare is campaign spend / total ad spend for period.
func CampaignSpendShare(campaignSpend, totalSpend float64) *float64 {
	return RatioOrNil(campaignSpend, totalSpend)
}

// AccountAdSpendToRevenue is total ad spend / account (cabinet) revenue (0–1 ratio).
// Prefer AccountDRRPct for percent display fields; this is the ratio form of the
// same canonical cabinet DRR formula.
func AccountAdSpendToRevenue(adSpend, accountRevenue float64) *float64 {
	return RatioOrNil(adSpend, accountRevenue)
}

// CancellationRate is cancel_count / orders_count where:
//   - orders_count = COUNT(*) of all order postings on metric_date (any status);
//   - cancel_count = postings with status exactly 'cancelled' (case-insensitive);
//   - returned postings are excluded from cancel_count (counted separately as returns_count).
//
// Denominator is total postings, not (orders - cancelled). Returns nil when orders_count <= 0.
func CancellationRate(cancelled, totalOrders int32) *float64 {
	return RatioOrNil(float64(cancelled), float64(totalOrders))
}

// ExcessStockShare is excess / available stock counts from Ozon analytics.
func ExcessStockShare(excess, available int32) *float64 {
	return RatioOrNil(float64(excess), float64(available))
}

type StockStatusBucketResult struct {
	Bucket string
	Source string
}

// ComputeStockStatusBucket derives inventory health bucket from canonical Aivion
// stock policy. Raw Ozon turnover_grade / IDC never unconditionally override
// calculated coverage; they are recorded only as conflict signals.
func ComputeStockStatusBucket(stockAvailable int32, daysOfCover *float64, ozonIDC *int32, turnoverGrade *string, excessShare *float64, daysWithoutSales *int32) StockStatusBucketResult {
	hasInternal := stockAvailable > 0 || daysOfCover != nil
	hasOzon := ozonIDC != nil || turnoverGrade != nil || excessShare != nil || (daysWithoutSales != nil && *daysWithoutSales > 0)

	grade := ""
	if turnoverGrade != nil {
		grade = strings.TrimSpace(*turnoverGrade)
	}

	// Prefer internal days-of-cover. When only Ozon IDC is present, use it as
	// coverage proxy so dashboards stay populated for enrichment-only rows.
	coverage := daysOfCover
	if coverage == nil && ozonIDC != nil {
		v := float64(*ozonIDC)
		coverage = &v
	}

	var dws *int64
	if daysWithoutSales != nil {
		v := int64(*daysWithoutSales)
		dws = &v
	}

	// Approximate Orders7D from velocity signals when available: if days without
	// sales is 0 and stock/coverage imply demand, treat as active; if dws >= 7,
	// treat as zero recent orders. Dashboard enrichment does not always pass
	// orders_7d; status policy still prefers coverage over raw NO_SALES grade.
	orders7d := int64(0)
	if dws != nil && *dws == 0 && stockAvailable > 0 {
		orders7d = 1 // conservative: recent sale implied by zero streak
	}

	res := stockpolicy.ComputeCanonicalStockStatus(stockpolicy.Input{
		AvailableStock:    int64(stockAvailable),
		HasStockData:      hasInternal || hasOzon,
		CoverageDays:      coverage,
		Orders7D:          orders7d,
		DaysWithoutSales:  dws,
		HasSalesHistory:   daysOfCover != nil || ozonIDC != nil || daysWithoutSales != nil,
		OzonTurnoverGrade: grade,
		ExcessStockShare:  excessShare,
	})

	// Dashboard SKU rows always carry StockAvailable; treat zero as known stock data.
	if !hasInternal && !hasOzon && stockAvailable == 0 {
		return StockStatusBucketResult{Bucket: stockpolicy.StatusOutOfStock, Source: stockStatusSource(true, false)}
	}
	if !hasInternal && !hasOzon {
		if stockAvailable <= 3 {
			return StockStatusBucketResult{Bucket: stockpolicy.StatusCriticalShortage, Source: stockStatusSource(true, false)}
		}
		return StockStatusBucketResult{Bucket: stockpolicy.StatusHealthy, Source: stockStatusSource(true, false)}
	}
	source := res.Source
	if source == "" {
		source = stockStatusSource(hasInternal, hasOzon)
	}
	return StockStatusBucketResult{Bucket: res.Status, Source: source}
}

func stockStatusSource(hasInternal, hasOzon bool) string {
	switch {
	case hasInternal && hasOzon:
		return "combined"
	case hasOzon:
		return "ozon_stock_analytics"
	default:
		return "internal_days_of_cover"
	}
}
