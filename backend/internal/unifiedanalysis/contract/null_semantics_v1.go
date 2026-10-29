package contract

// NullSemanticsV1 documents contract-level null vs zero rules for Analysis Pack CSV.
// Serialization happens in stage 2; these definitions are frozen for v1.
type NullSemanticsV1 struct {
	ZeroMeansObservedAbsence string
	NullMeansUnknownOrNA     string
	DivisionByZero           string
}

// MetricNullSemanticsV1 is the frozen null-semantics catalog for key metrics.
var MetricNullSemanticsV1 = map[string]string{
	"orders":                  "0 = observed zero orders; never null when row exists",
	"revenue":                 "0 = observed zero revenue; never null when row exists",
	"orders_delta_abs":        "0 = no change; never null when both period values known",
	"orders_delta_pct":        "null when previous period denominator is 0; else percent",
	"revenue_delta_pct":       "null when previous period denominator is 0; else percent",
	"drr_pct":                 "null when revenue is 0 or unavailable (division by zero)",
	"ctr_pct":                 "null when impressions are 0; else clicks/impressions*100",
	"conversion_pct":          "null when from-stage is 0; else to/from*100",
	"stock_coverage_days":     "null when average daily sales is 0/unavailable; else days",
	"average_daily_sales":     "0 = observed zero sales; null only if source window unavailable",
	"market_price_index":      "null when enrichment/price index source unavailable",
	"average_daily_sales_30d": "null until stage-2 loader provides 30d history; column frozen in v1",
	"stock_coverage_days_30d": "null until stage-2 loader provides 30d history; column frozen in v1",
}

// AnalysisPackDailyHistoryDaysV1 is the target raw daily history for Analysis Pack v1.
// Stage-1 SQL loader still loads 14d for legacy parity; stage 2 expands snapshot range to 30d.
// Legacy SellerAIDataPack continues to use 14d windows regardless.
const AnalysisPackDailyHistoryDaysV1 = 30

// LegacyDataPackDailyHistoryDays remains 14 and must not change.
const LegacyDataPackDailyHistoryDays = 14
