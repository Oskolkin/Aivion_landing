package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SQLCCandidateRepository loads candidate data via sqlc queries.
type SQLCCandidateRepository struct {
	queries      *dbgen.Queries
	actionsRunID int64
}

func NewSQLCCandidateRepository(queries *dbgen.Queries) *SQLCCandidateRepository {
	return &SQLCCandidateRepository{queries: queries}
}

func (r *SQLCCandidateRepository) BindActionsRunID(runID int64) {
	r.actionsRunID = runID
}

func (r *SQLCCandidateRepository) GetAccountSnapshot(ctx context.Context, sellerAccountID int64, metricDate time.Time) (AccountSnapshotRow, error) {
	var out AccountSnapshotRow
	prev := metricDate.AddDate(0, 0, -1)
	from7 := metricDate.AddDate(0, 0, -6)

	today, err := r.queries.GetDailyAccountMetricBySellerAndDate(ctx, dbgen.GetDailyAccountMetricBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(metricDate),
	})
	if err != nil && err != pgx.ErrNoRows {
		return out, fmt.Errorf("account metric today: %w", err)
	}
	if err == nil {
		rev := numericToFloat(today.Revenue)
		ord := int64(today.OrdersCount)
		out.Revenue = &rev
		out.Orders = &ord
	}

	yesterday, err := r.queries.GetDailyAccountMetricBySellerAndDate(ctx, dbgen.GetDailyAccountMetricBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(prev),
	})
	if err != nil && err != pgx.ErrNoRows {
		return out, fmt.Errorf("account metric yesterday: %w", err)
	}
	if err == nil {
		rev := numericToFloat(yesterday.Revenue)
		ord := int64(yesterday.OrdersCount)
		out.RevenuePrev = &rev
		out.OrdersPrev = &ord
	}

	spend7, err := r.queries.SumAdSpendBySellerAndDateRange(ctx, dbgen.SumAdSpendBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(from7),
		MetricDate_2:    pgDate(metricDate),
	})
	if err != nil {
		return out, fmt.Errorf("ad spend 7d: %w", err)
	}
	sp := numericToFloat(spend7)
	if sp > 0 {
		out.AdSpend7d = &sp
	}

	accountRows, err := r.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(from7),
		MetricDate_2:    pgDate(metricDate),
	})
	if err != nil {
		return out, fmt.Errorf("account metrics 7d: %w", err)
	}
	var rev7 float64
	for _, row := range accountRows {
		rev7 += numericToFloat(row.Revenue)
	}
	if rev7 > 0 {
		rev7 = round2(rev7)
		out.Revenue7d = &rev7
	}

	return out, nil
}

func (r *SQLCCandidateRepository) LoadSKUMetricsBundle(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*SKUMetricsBundle, error) {
	from := metricDate.AddDate(0, 0, -13)
	rows, err := r.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(from),
		MetricDate_2:    pgDate(metricDate),
	})
	if err != nil {
		return nil, fmt.Errorf("list sku metrics: %w", err)
	}
	return buildSKUMetricsBundle(metricDate, rows), nil
}

func (r *SQLCCandidateRepository) ListStockReplenishmentCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]StockReplenishmentRow, error) {
	bundle, err := r.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterStockReplenishmentRows(bundle, cfg), nil
}

func (r *SQLCCandidateRepository) ListNoSalesWithStockCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]NoSalesWithStockRow, error) {
	bundle, err := r.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterNoSalesWithStockRows(bundle), nil
}

func (r *SQLCCandidateRepository) ListSKUOrdersDropCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUOrdersDropRow, error) {
	bundle, err := r.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterSKUOrdersDropRows(bundle), nil
}

func (r *SQLCCandidateRepository) ListKeySKURiskCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]KeySKURiskRow, error) {
	bundle, err := r.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	replenish := filterStockReplenishmentRows(bundle, cfg)
	replenishSet := make(map[int64]bool, len(replenish))
	for _, row := range replenish {
		replenishSet[row.OzonProductID] = true
	}
	drops := filterSKUOrdersDropRows(bundle)
	dropSet := make(map[int64]bool, len(drops))
	for _, row := range drops {
		dropSet[row.OzonProductID] = true
	}
	return filterKeySKURiskRows(bundle, cfg, replenishSet, dropSet), nil
}

func (r *SQLCCandidateRepository) ListAdCampaignMetrics7d(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]AdCampaignMetricRow, error) {
	from := metricDate.AddDate(0, 0, -6)
	rows, err := r.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        pgDate(from),
		DateTo:          pgDate(metricDate),
	})
	if err != nil {
		return nil, fmt.Errorf("list ad campaigns: %w", err)
	}
	out := make([]AdCampaignMetricRow, 0, len(rows))
	for _, row := range rows {
		spend := numericToFloat(row.SpendTotal)
		revenue := numericToFloat(row.RevenueTotal)
		item := AdCampaignMetricRow{
			CampaignExternalID: strconv.FormatInt(row.CampaignExternalID, 10),
			CampaignName:       row.CampaignName,
			Spend7d:            spend,
			Revenue7d:          revenue,
			Orders7d:           row.OrdersTotal,
			HasMetricData:      row.LatestMetricDate.Valid,
		}
		item.DRRPct = analytics.CampaignDRRPct(spend, revenue)
		out = append(out, item)
	}
	return out, nil
}

func (r *SQLCCandidateRepository) CountAdCampaigns(ctx context.Context, sellerAccountID int64) (int64, error) {
	count, err := r.queries.CountAdCampaignsBySeller(ctx, sellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("count ad campaigns: %w", err)
	}
	return count, nil
}

func (r *SQLCCandidateRepository) GetFunnelData(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*FunnelDataBundle, error) {
	prev := metricDate.AddDate(0, 0, -1)
	from7 := metricDate.AddDate(0, 0, -6)

	current, err := r.queries.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(metricDate),
		ScopeType:       "account",
		ScopeID:         "account",
	})
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("funnel current: %w", err)
	}

	bundle := &FunnelDataBundle{}
	if err == nil {
		bundle.HasAccountScope = true
		bundle.SourceStatus = textOrEmpty(current.SourceStatus)
		cur := funnelDayFromRow(current)
		bundle.Current = &cur
	}

	previous, err := r.queries.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(prev),
		ScopeType:       "account",
		ScopeID:         "account",
	})
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("funnel previous: %w", err)
	}
	if err == nil {
		prevDay := funnelDayFromRow(previous)
		bundle.Previous = &prevDay
	}

	historyRows, err := r.queries.ListProductAnalyticsDailyBySellerAndDateRange(ctx, dbgen.ListProductAnalyticsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(from7),
		MetricDate_2:    pgDate(metricDate),
	})
	if err != nil {
		return nil, fmt.Errorf("funnel history: %w", err)
	}
	for _, row := range historyRows {
		if row.ScopeType != "account" {
			continue
		}
		bundle.History7d = append(bundle.History7d, funnelDayFromRow(row))
	}
	return bundle, nil
}

func (r *SQLCCandidateRepository) UpsertActionEntityGroup(ctx context.Context, sellerAccountID int64, metricDate time.Time, groupKey, entityType string, entityIDs []string, summary map[string]any) (*ActionEntityGroupRecord, error) {
	idsJSON, err := json.Marshal(entityIDs)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = map[string]any{}
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	if r.actionsRunID <= 0 {
		return nil, fmt.Errorf("actions run id is required for entity group upsert")
	}
	row, err := r.queries.UpsertActionEntityGroup(ctx, dbgen.UpsertActionEntityGroupParams{
		SellerAccountID:       sellerAccountID,
		MetricDate:            pgDate(metricDate),
		GroupKey:              groupKey,
		EntityType:            entityType,
		EntityIdsJson:         idsJSON,
		SummaryJson:           summaryJSON,
		ActionGenerationRunID: r.actionsRunID,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert entity group: %w", err)
	}
	return &ActionEntityGroupRecord{
		ID:              row.ID,
		SellerAccountID: row.SellerAccountID,
		MetricDate:      datePtr(row.MetricDate),
		GroupKey:        row.GroupKey,
		EntityType:      row.EntityType,
		EntityIDsJSON:   row.EntityIdsJson,
		SummaryJSON:     row.SummaryJson,
	}, nil
}

func buildSKUMetricsBundle(metricDate time.Time, rows []dbgen.DailySkuMetric) *SKUMetricsBundle {
	metricKey := formatMetricDate(metricDate)
	prevKey := formatMetricDate(metricDate.AddDate(0, 0, -1))
	from7 := formatMetricDate(metricDate.AddDate(0, 0, -6))

	type agg struct {
		revenue float64
		orders  int64
	}
	byProduct := make(map[int64]*SKUMetricRow)
	agg7 := make(map[int64]*agg)

	for _, row := range rows {
		pid := row.OzonProductID
		item, ok := byProduct[pid]
		if !ok {
			item = &SKUMetricRow{OzonProductID: pid}
			byProduct[pid] = item
		}
		if row.OfferID.Valid {
			s := row.OfferID.String
			item.OfferID = &s
		}
		if row.Sku.Valid {
			v := row.Sku.Int64
			item.SKU = &v
		}
		if row.ProductName.Valid {
			item.ProductName = row.ProductName.String
		}

		dayKey := formatMetricDate(row.MetricDate.Time)
		if dayKey == metricKey {
			item.HasMetricRow = true
			item.OrdersCount = row.OrdersCount
			item.Revenue = numericToFloat(row.Revenue)
			item.StockAvailable = row.StockAvailable
			item.DaysOfCover = numericToFloatPtr(row.DaysOfCover)
		}
		if dayKey == prevKey {
			item.HasPrevMetricRow = true
			item.PrevOrdersCount = row.OrdersCount
			item.PrevRevenue = numericToFloat(row.Revenue)
		}
		if dayKey >= from7 && dayKey <= metricKey {
			a := agg7[pid]
			if a == nil {
				a = &agg{}
				agg7[pid] = a
			}
			a.revenue += numericToFloat(row.Revenue)
			a.orders += int64(row.OrdersCount)
		}
	}

	outRows := make([]SKUMetricRow, 0, len(byProduct))
	var storeRev7 float64
	for _, item := range byProduct {
		if a := agg7[item.OzonProductID]; a != nil {
			item.Revenue7d = round2(a.revenue)
			item.Orders7d = a.orders
			storeRev7 += a.revenue
		}
		if item.HasMetricRow && item.HasPrevMetricRow {
			item.OrdersDelta = item.OrdersCount - item.PrevOrdersCount
			item.RevenueDelta = round2(item.Revenue - item.PrevRevenue)
		}
		outRows = append(outRows, *item)
	}

	storeRev7 = round2(storeRev7)
	keySet := selectKeySKUIDs(outRows, storeRev7)
	for i := range outRows {
		if keySet[outRows[i].OzonProductID] {
			outRows[i].IsKeySKU = true
		}
		if storeRev7 > 0 && outRows[i].Revenue7d > 0 {
			share := round1((outRows[i].Revenue7d / storeRev7) * 100)
			outRows[i].RevenueSharePct = &share
		}
	}

	return &SKUMetricsBundle{
		MetricDate:     metricDate,
		Rows:           outRows,
		StoreRevenue7d: storeRev7,
		TotalScanned:   len(outRows),
	}
}

func filterStockReplenishmentRows(bundle *SKUMetricsBundle, cfg CandidateBuilderConfig) []StockReplenishmentRow {
	out := make([]StockReplenishmentRow, 0)
	for _, row := range bundle.Rows {
		if !row.HasMetricRow {
			continue
		}
		if row.StockAvailable <= 0 {
			continue
		}
		if row.DaysOfCover == nil || *row.DaysOfCover <= 0 {
			continue
		}
		if *row.DaysOfCover > cfg.StockWarningCoverageDays {
			continue
		}
		if row.Orders7d <= 0 && row.Revenue7d <= 0 {
			continue
		}
		avgDaily := avgDailyRevenue7d(row.Revenue7d)
		risk := calcRevenueAtRisk7d(avgDaily, *row.DaysOfCover)
		critical := *row.DaysOfCover <= cfg.StockCriticalCoverageDays
		out = append(out, StockReplenishmentRow{
			SKUMetricRow:     row,
			RevenueAtRisk7d:  risk,
			PriorityCritical: critical || row.IsKeySKU,
		})
	}
	sortStockReplenishmentRows(out)
	return out
}

func filterNoSalesWithStockRows(bundle *SKUMetricsBundle) []NoSalesWithStockRow {
	out := make([]NoSalesWithStockRow, 0)
	for _, row := range bundle.Rows {
		if !row.HasMetricRow {
			continue
		}
		if row.OrdersCount != 0 {
			continue
		}
		if row.StockAvailable <= 0 {
			continue
		}
		price := unitPriceFromRow(row)
		frozen := frozenStockValue(row.StockAvailable, price)
		out = append(out, NoSalesWithStockRow{
			SKUMetricRow:     row,
			FrozenStockValue: frozen,
		})
	}
	sortNoSalesRows(out)
	return out
}

func filterSKUOrdersDropRows(bundle *SKUMetricsBundle) []SKUOrdersDropRow {
	out := make([]SKUOrdersDropRow, 0)
	for _, row := range bundle.Rows {
		if !row.HasMetricRow || !row.HasPrevMetricRow {
			continue
		}
		if row.OrdersDelta >= 0 {
			continue
		}
		out = append(out, SKUOrdersDropRow{SKUMetricRow: row})
	}
	sortOrdersDropRows(out)
	return out
}

func filterKeySKURiskRows(bundle *SKUMetricsBundle, cfg CandidateBuilderConfig, replenishOnly, dropOnly map[int64]bool) []KeySKURiskRow {
	out := make([]KeySKURiskRow, 0)
	for _, row := range bundle.Rows {
		if !row.IsKeySKU || !row.HasMetricRow {
			continue
		}
		risks := make([]string, 0, 4)
		score := 0
		if row.HasPrevMetricRow && row.OrdersDelta < 0 {
			risks = append(risks, "orders_drop")
			score += 2
		}
		if row.DaysOfCover != nil && *row.DaysOfCover > 0 && *row.DaysOfCover <= cfg.StockWarningCoverageDays && row.StockAvailable > 0 {
			risks = append(risks, "low_coverage")
			score += 2
		}
		if row.OrdersCount == 0 && row.StockAvailable > 0 {
			risks = append(risks, "no_sales_with_stock")
			score += 1
		}
		if len(risks) < 2 && score < 3 {
			continue
		}
		onlyReplenish := len(risks) == 1 && risks[0] == "low_coverage" && replenishOnly[row.OzonProductID]
		onlyDrop := len(risks) == 1 && risks[0] == "orders_drop" && dropOnly[row.OzonProductID]
		if onlyReplenish || onlyDrop {
			continue
		}
		out = append(out, KeySKURiskRow{
			SKUMetricRow: row,
			RiskTypes:    risks,
			RiskScore:    score,
		})
	}
	sortKeySKURiskRows(out)
	return out
}

func funnelDayFromRow(row dbgen.ProductAnalyticsDaily) FunnelDayMetrics {
	return FunnelDayMetrics{
		MetricDate:      row.MetricDate.Time,
		Impressions:     pgInt8Ptr(row.Impressions),
		ProductViews:    pgInt8Ptr(row.ProductViews),
		AddToCart:       pgInt8Ptr(row.AddToCart),
		Orders:          pgInt8Ptr(row.OrdersCount),
		CompletedOrders: pgInt8Ptr(row.CompletedOrders),
	}
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
