package analytics

import (
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// skuStockAnalyticsAgg is SKU-level aggregation of warehouse stock_analytics_snapshots rows.
// Count fields are summed; idc/ads/turnover_grade/days_without_sales use documented pick rules (never summed).
type skuStockAnalyticsAgg struct {
	available        int32
	excess           int32
	ads              *float64
	idc              *int32
	turnoverGrade    *string
	daysWithoutSales *int32
}

// turnover grade priority: higher index = more severe / riskier for seller (worst wins).
var turnoverGradePriority = map[string]int{
	"SURPLUS":    6,
	"NO_SALES":   5,
	"DEFICIT":    4,
	"POPULAR":    3,
	"ACTUAL":     2,
	"COLLECTING": 1,
}

func aggregateStockAnalyticsBySKU(rows []dbgen.StockAnalyticsSnapshot) map[int64]skuStockAnalyticsAgg {
	type agg struct {
		available        int32
		excess           int32
		adsSum           float64
		adsCount         int
		idc              int32
		idcSet           bool
		turnoverGrade    string
		daysWithoutSales int32
		dwsSet           bool
	}
	bySKU := make(map[int64]*agg)
	for _, row := range rows {
		cur := bySKU[row.Sku]
		if cur == nil {
			cur = &agg{}
			bySKU[row.Sku] = cur
		}
		cur.available += row.AvailableStockCount
		cur.excess += row.ExcessStockCount
		if row.Idc.Valid && (!cur.idcSet || row.Idc.Int32 > cur.idc) {
			cur.idc = row.Idc.Int32
			cur.idcSet = true
		}
		if row.DaysWithoutSales.Valid && (!cur.dwsSet || row.DaysWithoutSales.Int32 > cur.daysWithoutSales) {
			cur.daysWithoutSales = row.DaysWithoutSales.Int32
			cur.dwsSet = true
		}
		if row.Ads.Valid {
			if v, err := row.Ads.Float64Value(); err == nil && v.Valid {
				cur.adsSum += v.Float64
				cur.adsCount++
			}
		}
		cur.turnoverGrade = pickWorseTurnoverGrade(cur.turnoverGrade, textValue(row.TurnoverGrade))
	}

	out := make(map[int64]skuStockAnalyticsAgg, len(bySKU))
	for sku, cur := range bySKU {
		item := skuStockAnalyticsAgg{
			available: cur.available,
			excess:    cur.excess,
		}
		if cur.idcSet {
			v := cur.idc
			item.idc = &v
		}
		if cur.dwsSet {
			v := cur.daysWithoutSales
			item.daysWithoutSales = &v
		}
		if cur.adsCount > 0 {
			ads := math.Round((cur.adsSum/float64(cur.adsCount))*10000) / 10000
			item.ads = &ads
		}
		if cur.turnoverGrade != "" {
			g := cur.turnoverGrade
			item.turnoverGrade = &g
		}
		out[sku] = item
	}
	return out
}

func pickWorseTurnoverGrade(current, candidate string) string {
	candidate = strings.TrimSpace(strings.ToUpper(candidate))
	if candidate == "" || isUnknownTurnoverGrade(candidate) {
		return current
	}
	if current == "" {
		return candidate
	}
	if turnoverGradeRank(candidate) > turnoverGradeRank(current) {
		return candidate
	}
	return current
}

func turnoverGradeRank(grade string) int {
	if r, ok := turnoverGradePriority[grade]; ok {
		return r
	}
	return 0
}

func isUnknownTurnoverGrade(grade string) bool {
	switch grade {
	case "UNKNOWN", "UNSPECIFIED", "N/A", "NA":
		return true
	default:
		return false
	}
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
