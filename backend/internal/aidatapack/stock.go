package aidatapack

import (
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type aggregatedStock struct {
	AvailableStockCount            int32
	ValidStockCount                int32
	RequestedStockCount            int32
	TransitStockCount              int32
	WaitingDocsStockCount          int32
	ExcessStockCount               int32
	ExpiringStockCount             int32
	ReturnFromCustomerStockCount   int32
	ReturnToSellerStockCount       int32
	StockDefectStockCount          int32
	TransitDefectStockCount        int32
	DaysWithoutSales               int32
	Ads28d                         *float64
	IDCDays                        *float64
	TurnoverGrade                  string
	SourceRowsCount                int
	SourceTurnoverGrades           []string
	TurnoverGradeAggregationMethod string
}

func buildStockPositions(b *Builder, ctx *loadContext, known map[string]*skuMeta) []StockPositionRow {
	lites := liteStockSnapshots(ctx)
	bySKU := map[string]aggregatedStock{}
	for sku := range known {
		agg := aggregateStockFromSnapshots(lites, sku)
		if agg.TurnoverGrade != "" || agg.AvailableStockCount > 0 {
			bySKU[sku] = agg
		}
	}

	w := b.Windows()
	source := ctx.EnrichmentSource
	if source == "" {
		source = "unknown"
	}

	rows := make([]StockPositionRow, 0, len(known))
	for sku := range known {
		a, ok := bySKU[sku]
		if !ok {
			rows = append(rows, StockPositionRow{
				EntityID:           skuEntityID(sku),
				SnapshotDate:       w.CurrentDay,
				DataStatus:         "not_available",
				WarehouseBreakdown: []any{},
			})
			continue
		}
		row := StockPositionRow{
			EntityID:                       skuEntityID(sku),
			SnapshotDate:                   w.CurrentDay,
			DataStatus:                     "available",
			AvailableStockCount:            a.AvailableStockCount,
			ValidStockCount:                a.ValidStockCount,
			RequestedStockCount:            a.RequestedStockCount,
			TransitStockCount:              a.TransitStockCount,
			WaitingDocsStockCount:          a.WaitingDocsStockCount,
			ExcessStockCount:               a.ExcessStockCount,
			ExpiringStockCount:             a.ExpiringStockCount,
			ReturnFromCustomerStockCount:   a.ReturnFromCustomerStockCount,
			ReturnToSellerStockCount:       a.ReturnToSellerStockCount,
			StockDefectStockCount:          a.StockDefectStockCount,
			TransitDefectStockCount:        a.TransitDefectStockCount,
			DaysWithoutSales:               a.DaysWithoutSales,
			Ads28d:                         a.Ads28d,
			IDCDays:                        a.IDCDays,
			TurnoverGrade:                  a.TurnoverGrade,
			SourceRowsCount:                a.SourceRowsCount,
			SourceTurnoverGrades:           a.SourceTurnoverGrades,
			TurnoverGradeAggregationMethod: a.TurnoverGradeAggregationMethod,
			WarehouseBreakdown:             []any{},
			Source:                         source,
		}

		var factIDs []string
		if a.IDCDays != nil {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".stock.idc_days", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "idc_days", Date: w.CurrentDay, Value: *a.IDCDays, Unit: UnitDays,
				Source: FactSource{Table: "stock_analytics_snapshots", Field: "idc"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if a.TurnoverGrade != "" {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".stock.turnover_grade", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "turnover_grade", Date: w.CurrentDay, Value: a.TurnoverGrade, Unit: UnitEnum,
				Source: FactSource{Table: "stock_analytics_snapshots", Field: "turnover_grade"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		row.FactIDs = factIDs
		rows = append(rows, row)
	}
	return rows
}

func aggregateStockRows(rows []struct {
	Sku                          int64
	AvailableStockCount          int32
	ValidStockCount              int32
	RequestedStockCount          int32
	TransitStockCount            int32
	WaitingDocsStockCount        int32
	ExcessStockCount             int32
	ExpiringStockCount           int32
	ReturnFromCustomerStockCount int32
	ReturnToSellerStockCount     int32
	StockDefectStockCount        int32
	TransitDefectStockCount      int32
	DaysWithoutSales             int32
	Ads                          float64
	AdsValid                     bool
	IDC                          int32
	IDCValid                     bool
	TurnoverGrade                string
}, sku string) aggregatedStock {
	var out aggregatedStock
	var grades []string
	var minIDC *float64
	var maxADS *float64
	matched := 0
	target := sku
	for _, row := range rows {
		if int64ToString(row.Sku) != target {
			continue
		}
		matched++
		out.AvailableStockCount += row.AvailableStockCount
		out.ValidStockCount += row.ValidStockCount
		out.RequestedStockCount += row.RequestedStockCount
		out.TransitStockCount += row.TransitStockCount
		out.WaitingDocsStockCount += row.WaitingDocsStockCount
		out.ExcessStockCount += row.ExcessStockCount
		out.ExpiringStockCount += row.ExpiringStockCount
		out.ReturnFromCustomerStockCount += row.ReturnFromCustomerStockCount
		out.ReturnToSellerStockCount += row.ReturnToSellerStockCount
		out.StockDefectStockCount += row.StockDefectStockCount
		out.TransitDefectStockCount += row.TransitDefectStockCount
		if row.DaysWithoutSales > out.DaysWithoutSales {
			out.DaysWithoutSales = row.DaysWithoutSales
		}
		if row.AdsValid {
			v := row.Ads
			if maxADS == nil || v > *maxADS {
				maxADS = &v
			}
		}
		if row.IDCValid && row.IDC > 0 {
			v := float64(row.IDC)
			if minIDC == nil || v < *minIDC {
				minIDC = &v
			}
		}
		if row.TurnoverGrade != "" {
			grades = append(grades, row.TurnoverGrade)
		}
	}
	out.Ads28d = maxADS
	out.IDCDays = minIDC
	out.TurnoverGrade = mostCriticalTurnoverGrade(grades)
	out.SourceRowsCount = matched
	out.SourceTurnoverGrades = uniqueSortedStrings(grades)
	out.TurnoverGradeAggregationMethod = turnoverGradeAggregationMethod(len(grades), out.SourceTurnoverGrades)
	return out
}

func turnoverGradeAggregationMethod(rowCount int, uniqueGrades []string) string {
	if rowCount <= 1 {
		return "single_row"
	}
	if len(uniqueGrades) <= 1 {
		return "uniform_across_rows"
	}
	return "most_critical_across_rows"
}

func uniqueSortedStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// aggregateStockFromSnapshots aggregates stock analytics rows for one SKU.
func aggregateStockFromSnapshots(all []stockSnapshotLite, sku string) aggregatedStock {
	rows := make([]struct {
		Sku                          int64
		AvailableStockCount          int32
		ValidStockCount              int32
		RequestedStockCount          int32
		TransitStockCount            int32
		WaitingDocsStockCount        int32
		ExcessStockCount             int32
		ExpiringStockCount           int32
		ReturnFromCustomerStockCount int32
		ReturnToSellerStockCount     int32
		StockDefectStockCount        int32
		TransitDefectStockCount      int32
		DaysWithoutSales             int32
		Ads                          float64
		AdsValid                     bool
		IDC                          int32
		IDCValid                     bool
		TurnoverGrade                string
	}, 0)
	for _, r := range all {
		if int64ToString(r.Sku) != sku {
			continue
		}
		rows = append(rows, struct {
			Sku                          int64
			AvailableStockCount          int32
			ValidStockCount              int32
			RequestedStockCount          int32
			TransitStockCount            int32
			WaitingDocsStockCount        int32
			ExcessStockCount             int32
			ExpiringStockCount           int32
			ReturnFromCustomerStockCount int32
			ReturnToSellerStockCount     int32
			StockDefectStockCount        int32
			TransitDefectStockCount      int32
			DaysWithoutSales             int32
			Ads                          float64
			AdsValid                     bool
			IDC                          int32
			IDCValid                     bool
			TurnoverGrade                string
		}{
			Sku: r.Sku, AvailableStockCount: r.AvailableStockCount, ValidStockCount: r.ValidStockCount,
			RequestedStockCount: r.RequestedStockCount, TransitStockCount: r.TransitStockCount,
			WaitingDocsStockCount: r.WaitingDocsStockCount, ExcessStockCount: r.ExcessStockCount,
			ExpiringStockCount: r.ExpiringStockCount, ReturnFromCustomerStockCount: r.ReturnFromCustomerStockCount,
			ReturnToSellerStockCount: r.ReturnToSellerStockCount, StockDefectStockCount: r.StockDefectStockCount,
			TransitDefectStockCount: r.TransitDefectStockCount, DaysWithoutSales: r.DaysWithoutSales,
			Ads: r.Ads, AdsValid: r.AdsValid, IDC: r.IDC, IDCValid: r.IDCValid, TurnoverGrade: r.TurnoverGrade,
		})
	}
	return aggregateStockRows(rows, sku)
}

type stockSnapshotLite struct {
	Sku                          int64
	AvailableStockCount          int32
	ValidStockCount              int32
	RequestedStockCount          int32
	TransitStockCount            int32
	WaitingDocsStockCount        int32
	ExcessStockCount             int32
	ExpiringStockCount           int32
	ReturnFromCustomerStockCount int32
	ReturnToSellerStockCount     int32
	StockDefectStockCount        int32
	TransitDefectStockCount      int32
	DaysWithoutSales             int32
	Ads                          float64
	AdsValid                     bool
	IDC                          int32
	IDCValid                     bool
	TurnoverGrade                string
}

func int32OrZero(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func liteStockSnapshots(ctx *loadContext) []stockSnapshotLite {
	out := make([]stockSnapshotLite, 0, len(ctx.StockSnapshots))
	for _, row := range ctx.StockSnapshots {
		lite := stockSnapshotLite{
			Sku: row.Sku, AvailableStockCount: row.AvailableStockCount,
			ValidStockCount: int32OrZero(row.ValidStockCount), RequestedStockCount: row.RequestedStockCount.Int32,
			TransitStockCount: row.TransitStockCount.Int32, WaitingDocsStockCount: row.WaitingDocsStockCount.Int32,
			ExcessStockCount: row.ExcessStockCount, ExpiringStockCount: row.ExpiringStockCount.Int32,
			ReturnFromCustomerStockCount: row.ReturnFromCustomerStockCount.Int32,
			ReturnToSellerStockCount:     row.ReturnToSellerStockCount.Int32,
			StockDefectStockCount:        row.StockDefectStockCount.Int32,
			TransitDefectStockCount:      row.TransitDefectStockCount.Int32,
			DaysWithoutSales:             row.DaysWithoutSales.Int32,
			TurnoverGrade:                textValue(row.TurnoverGrade),
		}
		if row.Ads.Valid {
			lite.Ads = numericToFloat(row.Ads)
			lite.AdsValid = true
		}
		if row.Idc.Valid {
			lite.IDC = row.Idc.Int32
			lite.IDCValid = true
		}
		out = append(out, lite)
	}
	return out
}
