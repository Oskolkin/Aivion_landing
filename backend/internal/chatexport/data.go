package chatexport

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type DataService struct {
	queries     *dbgen.Queries
	criticalSKU *analytics.CriticalSKUService
	advertising *analytics.AdvertisingService
}

func NewDataService(
	queries *dbgen.Queries,
	criticalSKU *analytics.CriticalSKUService,
	advertising *analytics.AdvertisingService,
) *DataService {
	return &DataService{
		queries:     queries,
		criticalSKU: criticalSKU,
		advertising: advertising,
	}
}

func (d *DataService) ResolveAsOfDate(ctx context.Context, sellerAccountID int64, requested *time.Time) (time.Time, error) {
	if requested != nil {
		return requested.UTC().Truncate(24 * time.Hour), nil
	}
	latest, err := d.queries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return time.Time{}, err
	}
	if latest.Valid {
		return latest.Time.UTC().Truncate(24 * time.Hour), nil
	}
	return time.Now().UTC().Truncate(24 * time.Hour), nil
}

func (d *DataService) BuildTable(
	ctx context.Context,
	sellerAccountID int64,
	exportType string,
	dateFrom, dateTo time.Time,
) (ExportTable, error) {
	switch exportType {
	case TypeSalesSummary:
		return d.buildSalesSummary(ctx, sellerAccountID, dateFrom, dateTo)
	case TypeSKUPerformance:
		return d.buildSKUPerformance(ctx, sellerAccountID, dateTo)
	case TypeCriticalSKU:
		return d.buildCriticalSKU(ctx, sellerAccountID, dateTo)
	case TypeStockRisks:
		return d.buildStockRisks(ctx, sellerAccountID, dateTo)
	case TypeStockSurplus:
		return d.buildStockSurplus(ctx, sellerAccountID, dateTo)
	case TypeAdvertisingCampaigns, TypeCampaignPerformance:
		return d.buildCampaigns(ctx, sellerAccountID, dateFrom, dateTo)
	case TypeAlerts:
		return d.buildAlerts(ctx, sellerAccountID)
	case TypeRecommendations:
		return d.buildRecommendations(ctx, sellerAccountID)
	case TypeOrdersByStatus:
		return d.buildOrdersByStatus(ctx, sellerAccountID, dateFrom, dateTo)
	default:
		return ExportTable{}, fmt.Errorf("unsupported export type: %s", exportType)
	}
}

func (d *DataService) buildSalesSummary(ctx context.Context, sellerID int64, from, to time.Time) (ExportTable, error) {
	cols := []string{
		"date", "revenue", "orders_count", "units_sold", "average_order_value",
		"cancellation_rate", "account_ad_spend_to_revenue",
	}
	rows, err := d.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      toPgDate(from),
		MetricDate_2:    toPgDate(to),
	})
	if err != nil {
		return ExportTable{}, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].MetricDate.Time.Before(rows[j].MetricDate.Time)
	})
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		revenue := numericFloat(row.Revenue)
		orders := row.OrdersCount
		adSpend, _ := d.sumAdSpendForDate(ctx, sellerID, row.MetricDate.Time)
		out = append(out, []string{
			row.MetricDate.Time.Format("2006-01-02"),
			fmtFloat(revenue),
			fmtInt32(orders),
			fmtUnitsSold(row.UnitsSold),
			fmtFloat(ratio(revenue, float64(orders))),
			fmtFloat(cancelRate(row.CancelCount, orders)),
			fmtFloat(ratio(adSpend, revenue)),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildSKUPerformance(ctx context.Context, sellerID int64, asOf time.Time) (ExportTable, error) {
	cols := []string{
		"sku", "offer_id", "product_name", "revenue", "orders_count", "units_sold",
		"sku_orders_share", "sku_revenue_share", "sku_orders_delta_percent",
		"velocity_orders_7d", "velocity_orders_14d", "velocity_units_7d", "velocity_units_14d",
		"stock_available", "days_of_cover", "stock_status_bucket", "discount_depth",
	}
	result, err := d.criticalSKU.ListCriticalSKUsForSellerAccount(ctx, sellerID, asOf)
	if err != nil {
		return ExportTable{}, err
	}
	// Use all SKU metrics for the day, not only critical-ranked subset.
	metricRows, err := d.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      toPgDate(asOf),
		MetricDate_2:    toPgDate(asOf),
	})
	if err != nil {
		return ExportTable{}, err
	}
	cardByProduct := map[int64]analytics.CriticalSKUCard{}
	for _, c := range result.Items {
		cardByProduct[c.OzonProductID] = c
	}
	sort.Slice(metricRows, func(i, j int) bool {
		return numericFloat(metricRows[i].Revenue) > numericFloat(metricRows[j].Revenue)
	})
	out := make([][]string, 0, len(metricRows))
	for _, row := range metricRows {
		card := cardByProduct[row.OzonProductID]
		out = append(out, []string{
			fmtInt64Ptr(pgInt64Ptr(row.Sku)),
			fmtStrPtr(pgTextPtr(row.OfferID)),
			fmtStrPtr(pgTextPtr(row.ProductName)),
			fmtFloat(numericFloat(row.Revenue)),
			fmtInt32(row.OrdersCount),
			fmtUnitsSold(row.UnitsSold),
			fmtFloatPtr(card.SKUOrdersShare),
			fmtFloatPtr(card.SKURevenueShare),
			fmtFloatPtr(card.SKUOrdersDeltaPercent),
			fmtFloatPtr(card.SKUSalesVelocityOrders7D),
			fmtFloatPtr(card.SKUSalesVelocityOrders7D),
			fmtFloatPtr(card.SKUSalesVelocityOrders7D),
			fmtFloatPtr(card.SKUSalesVelocityOrders7D),
			fmtInt32(card.StockAvailable),
			fmtFloatPtr(card.DaysOfCover),
			fmtStrPtr(card.StockStatusBucket),
			fmtFloatPtr(card.DiscountDepth),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildCriticalSKU(ctx context.Context, sellerID int64, asOf time.Time) (ExportTable, error) {
	cols := []string{
		"sku", "offer_id", "product_name", "revenue_7d", "orders_7d", "revenue_share_7d",
		"orders_share_7d", "revenue_rank_7d", "orders_rank_7d", "days_of_cover",
		"stock_status_bucket", "key_reasons",
	}
	result, err := d.criticalSKU.ListCriticalSKUsForSellerAccount(ctx, sellerID, asOf)
	if err != nil {
		return ExportTable{}, err
	}
	out := make([][]string, 0, len(result.Items))
	for i, item := range result.Items {
		out = append(out, []string{
			fmtInt64Ptr(item.SKU),
			fmtStrPtr(item.OfferID),
			fmtStrPtr(item.ProductName),
			fmtFloat(item.Revenue),
			fmtInt32(item.SalesOps),
			fmtFloatPtr(item.SKURevenueShare),
			fmtFloatPtr(item.SKUOrdersShare),
			fmtInt(i + 1),
			fmtInt(i + 1),
			fmtFloatPtr(item.DaysOfCover),
			fmtStrPtr(item.StockStatusBucket),
			strings.Join(item.Signals, "; "),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildStockRisks(ctx context.Context, sellerID int64, asOf time.Time) (ExportTable, error) {
	cols := []string{
		"sku", "offer_id", "product_name", "stock_available", "days_of_cover",
		"stock_status_bucket", "ozon_idc", "ozon_ads", "turnover_grade", "risk_reason", "severity",
	}
	rows, err := d.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      toPgDate(asOf),
		MetricDate_2:    toPgDate(asOf),
	})
	if err != nil {
		return ExportTable{}, err
	}
	ozonBySKU, _ := d.loadStockAnalyticsBySKU(ctx, sellerID, asOf)

	type riskRow struct {
		rank      int
		cells     []string
		productID int64
	}
	candidates := make([]riskRow, 0)
	for _, row := range rows {
		days := pgNumericFloatPtr(row.DaysOfCover)
		depletion := classifyStockDepletionRisk(row.StockAvailable, days)
		if !isExportableStockRisk(depletion) {
			continue
		}
		var skuID int64
		if row.Sku.Valid {
			skuID = row.Sku.Int64
		}
		oz := ozonBySKU[skuID]
		bucket := ""
		if oz.available > 0 || days != nil {
			excessShare := (*float64)(nil)
			if oz.available > 0 && oz.excess > 0 {
				v := float64(oz.excess) / float64(oz.available)
				excessShare = &v
			}
			var turnover *string
			if oz.turnover != "" {
				tg := oz.turnover
				turnover = &tg
			}
			b := analytics.ComputeStockStatusBucket(row.StockAvailable, days, oz.idc, turnover, excessShare, oz.daysWithoutSales)
			bucket = b.Bucket
		}
		candidates = append(candidates, riskRow{
			rank:      stockRiskRank(depletion),
			productID: row.OzonProductID,
			cells: []string{
				fmtInt64Ptr(pgInt64Ptr(row.Sku)),
				fmtStrPtr(pgTextPtr(row.OfferID)),
				fmtStrPtr(pgTextPtr(row.ProductName)),
				fmtInt32(row.StockAvailable),
				fmtFloatPtr(days),
				bucket,
				fmtInt32Ptr(oz.idc),
				fmtFloatPtr(oz.ads),
				oz.turnover,
				stockRiskReasonText(row.StockAvailable, days, depletion),
				stockRiskSeverity(depletion),
			},
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rank == candidates[j].rank {
			return candidates[i].productID < candidates[j].productID
		}
		return candidates[i].rank > candidates[j].rank
	})
	out := make([][]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.cells)
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildStockSurplus(ctx context.Context, sellerID int64, asOf time.Time) (ExportTable, error) {
	cols := []string{
		"sku", "offer_id", "product_name", "available_stock_count", "excess_stock_count",
		"excess_stock_share", "ozon_idc", "ozon_ads", "turnover_grade", "days_without_sales", "warehouses_summary",
	}
	snapDate, err := d.queries.GetLatestStockAnalyticsSnapshotDateBySeller(ctx, sellerID)
	if err != nil {
		return ExportTable{}, err
	}
	if !snapDate.Valid {
		return ExportTable{Columns: cols, Rows: nil}, nil
	}
	rows, err := d.queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerID,
		SnapshotDate:    snapDate,
	})
	if err != nil {
		return ExportTable{}, err
	}
	type agg struct {
		sku              int64
		offerID          string
		productName      string
		available        int32
		excess           int32
		idc              *int32
		ads              *float64
		turnover         string
		daysWithoutSales *int32
		warehouses       []string
	}
	bySKU := map[int64]*agg{}
	for _, row := range rows {
		if row.Sku <= 0 {
			continue
		}
		sku := row.Sku
		a := bySKU[sku]
		if a == nil {
			a = &agg{sku: sku}
			bySKU[sku] = a
		}
		if row.OfferID.Valid {
			a.offerID = row.OfferID.String
		}
		if row.ProductName.Valid {
			a.productName = row.ProductName.String
		}
		a.available += row.AvailableStockCount
		a.excess += row.ExcessStockCount
		if row.Idc.Valid {
			v := row.Idc.Int32
			a.idc = &v
		}
		if row.Ads.Valid {
			f := numericFloat(row.Ads)
			a.ads = &f
		}
		if row.TurnoverGrade.Valid {
			a.turnover = row.TurnoverGrade.String
		}
		if row.DaysWithoutSales.Valid {
			v := row.DaysWithoutSales.Int32
			a.daysWithoutSales = &v
		}
		if row.WarehouseName.Valid {
			a.warehouses = append(a.warehouses, row.WarehouseName.String)
		}
	}
	out := make([][]string, 0)
	for _, a := range bySKU {
		if a.excess <= 0 && !strings.EqualFold(a.turnover, "SURPLUS") {
			continue
		}
		share := ""
		if a.available > 0 {
			share = fmtFloat(float64(a.excess) / float64(a.available))
		}
		out = append(out, []string{
			fmtInt64(a.sku),
			a.offerID,
			a.productName,
			fmtInt32(a.available),
			fmtInt32(a.excess),
			share,
			fmtInt32Ptr(a.idc),
			fmtFloatPtr(a.ads),
			a.turnover,
			fmtInt32Ptr(a.daysWithoutSales),
			strings.Join(a.warehouses, ", "),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][4] > out[j][4] })
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildCampaigns(ctx context.Context, sellerID int64, from, to time.Time) (ExportTable, error) {
	cols := []string{
		"campaign_external_id", "campaign_name", "date_from", "date_to",
		"impressions", "clicks", "spend", "orders_count", "revenue",
		"drr", "roas", "cpo", "ctr", "cpc", "click_to_order_rate", "spend_share",
	}
	if d.advertising == nil {
		return ExportTable{}, fmt.Errorf("advertising service is not configured")
	}
	dto, err := d.advertising.BuildAdvertisingAnalytics(ctx, sellerID, analytics.AdvertisingQuery{
		DateFrom: from,
		DateTo:   to,
		Limit:    MaxExportRows,
	})
	if err != nil {
		return ExportTable{}, err
	}
	fromS := from.Format("2006-01-02")
	toS := to.Format("2006-01-02")
	out := make([][]string, 0, len(dto.Campaign))
	for _, c := range dto.Campaign {
		out = append(out, []string{
			fmtInt64(c.CampaignExternalID),
			c.CampaignName,
			fromS,
			toS,
			fmtInt64(c.ImpressionsTotal),
			fmtInt64(c.ClicksTotal),
			fmtFloat(c.SpendTotal),
			fmtInt64(c.OrdersTotal),
			fmtFloat(c.RevenueTotal),
			fmtFloatPtr(c.CampaignDRR),
			fmtFloatPtr(c.CampaignROAS),
			fmtFloatPtr(c.CampaignCPO),
			fmtFloatPtr(c.CTR),
			fmtFloatPtr(c.CPC),
			fmtFloatPtr(c.CampaignClickToOrderRate),
			fmtFloatPtr(c.CampaignSpendShare),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildAlerts(ctx context.Context, sellerID int64) (ExportTable, error) {
	cols := []string{
		"created_at", "alert_type", "severity", "urgency", "entity_type", "entity_id",
		"title", "message", "status", "evidence_summary",
	}
	rows, err := d.queries.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           MaxExportRows,
		Offset:          0,
	})
	if err != nil {
		return ExportTable{}, err
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, []string{
			row.CreatedAt.Time.UTC().Format(time.RFC3339),
			row.AlertType,
			row.Severity,
			row.Urgency,
			row.EntityType,
			fmtStrPtr(pgTextPtr(row.EntityID)),
			row.Title,
			row.Message,
			row.Status,
			evidenceSummary(row.EvidencePayload),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildRecommendations(ctx context.Context, sellerID int64) (ExportTable, error) {
	cols := []string{
		"created_at", "recommendation_type", "title", "message", "priority", "status",
		"linked_alert_type", "evidence_summary",
	}
	rows, err := d.queries.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           MaxExportRows,
		Offset:          0,
	})
	if err != nil {
		return ExportTable{}, err
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, []string{
			row.CreatedAt.Time.UTC().Format(time.RFC3339),
			row.RecommendationType,
			row.Title,
			row.WhatHappened + " " + row.WhyItMatters,
			row.PriorityLevel,
			row.Status,
			"",
			evidenceSummary(row.SupportingMetricsPayload),
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) buildOrdersByStatus(ctx context.Context, sellerID int64, from, to time.Time) (ExportTable, error) {
	cols := []string{"status", "count", "share"}
	rows, err := d.queries.CountOrdersByStatusForDateRange(ctx, dbgen.CountOrdersByStatusForDateRangeParams{
		SellerAccountID: sellerID,
		Column2:         toPgDate(from),
		Column3:         toPgDate(to),
	})
	if err != nil {
		return ExportTable{}, err
	}
	var total int64
	for _, row := range rows {
		total += row.OrdersCount
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		share := ""
		if total > 0 {
			share = fmtFloat(float64(row.OrdersCount) / float64(total))
		}
		out = append(out, []string{
			fmt.Sprint(row.Status),
			fmtInt64(row.OrdersCount),
			share,
		})
	}
	return ExportTable{Columns: cols, Rows: capRows(out)}, nil
}

func (d *DataService) sumAdSpendForDate(ctx context.Context, sellerID int64, day time.Time) (float64, error) {
	rows, err := d.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        toPgDate(day),
		DateTo:          toPgDate(day),
	})
	if err != nil {
		return 0, err
	}
	var total float64
	for _, row := range rows {
		total += numericFloat(row.Spend)
	}
	return total, nil
}

type stockOzonAgg struct {
	available        int32
	excess           int32
	idc              *int32
	ads              *float64
	turnover         string
	daysWithoutSales *int32
}

func (d *DataService) loadStockAnalyticsBySKU(ctx context.Context, sellerID int64, asOf time.Time) (map[int64]stockOzonAgg, error) {
	snapDate, err := d.queries.GetLatestStockAnalyticsSnapshotDateBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if !snapDate.Valid {
		return map[int64]stockOzonAgg{}, nil
	}
	rows, err := d.queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerID,
		SnapshotDate:    snapDate,
	})
	if err != nil {
		return nil, err
	}
	bySKU := make(map[int64]*stockOzonAgg)
	for _, row := range rows {
		if row.Sku <= 0 {
			continue
		}
		a := bySKU[row.Sku]
		if a == nil {
			a = &stockOzonAgg{}
			bySKU[row.Sku] = a
		}
		a.available += row.AvailableStockCount
		a.excess += row.ExcessStockCount
		if row.Idc.Valid {
			v := row.Idc.Int32
			a.idc = &v
		}
		if row.Ads.Valid {
			f := numericFloat(row.Ads)
			a.ads = &f
		}
		if row.TurnoverGrade.Valid {
			a.turnover = row.TurnoverGrade.String
		}
		if row.DaysWithoutSales.Valid {
			v := row.DaysWithoutSales.Int32
			a.daysWithoutSales = &v
		}
	}
	out := make(map[int64]stockOzonAgg, len(bySKU))
	for sku, agg := range bySKU {
		out[sku] = *agg
	}
	return out, nil
}

func capRows(rows [][]string) [][]string {
	if len(rows) > MaxExportRows {
		return rows[:MaxExportRows]
	}
	return rows
}

func evidenceSummary(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	parts := make([]string, 0, 6)
	for _, key := range []string{"reasons", "summary", "risk", "signal"} {
		if v, ok := m[key]; ok {
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, "; ")
}

func toPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t.UTC(), Valid: true}
}

func pgTextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := strings.TrimSpace(v.String)
	if s == "" {
		return nil
	}
	return &s
}

func pgInt64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func numericFloat(v pgtype.Numeric) float64 {
	if !v.Valid {
		return 0
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return math.Round(f.Float64*100) / 100
}

func pgNumericFloatPtr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	out := math.Round(f.Float64*100) / 100
	return &out
}

func ratio(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return math.Round((num/den)*10000) / 10000
}

func cancelRate(cancels, orders int32) float64 {
	return ratio(float64(cancels), float64(orders))
}

func fmtFloat(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func fmtFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return fmtFloat(*v)
}

func fmtInt32(v int32) string {
	return fmt.Sprintf("%d", v)
}

func fmtInt32Ptr(v *int32) string {
	if v == nil {
		return ""
	}
	return fmtInt32(*v)
}

func fmtInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func fmtInt64Ptr(v *int64) string {
	if v == nil {
		return ""
	}
	return fmtInt64(*v)
}

func fmtInt(v int) string {
	return fmt.Sprintf("%d", v)
}

func fmtStrPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func fmtUnitsSold(v pgtype.Int4) string {
	if !v.Valid {
		return ""
	}
	return fmtInt32(v.Int32)
}
