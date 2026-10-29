package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const recentDemandDays = 7

type SKUMetricsService struct {
	db      *pgxpool.Pool
	queries *dbgen.Queries
}

func NewSKUMetricsService(db *pgxpool.Pool) *SKUMetricsService {
	return &SKUMetricsService{
		db:      db,
		queries: dbgen.New(db),
	}
}

func (s *SKUMetricsService) RebuildDailySKUMetricsForSellerAccount(ctx context.Context, sellerAccountID int64) error {
	bounds, err := s.queries.GetDailySKUMetricSourceDateBoundsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return fmt.Errorf("get sku source date bounds: %w", err)
	}
	if !bounds.MinDate.Valid || !bounds.MaxDate.Valid {
		return nil
	}

	_, err = s.RebuildDailySKUMetricsForDateRange(ctx, sellerAccountID, bounds.MinDate.Time, bounds.MaxDate.Time)
	return err
}

func (s *SKUMetricsService) RebuildDailySKUMetricsForDateRange(
	ctx context.Context,
	sellerAccountID int64,
	dateFrom time.Time,
	dateTo time.Time,
) (rowsUpserted int, err error) {
	fromDate := normalizeDate(dateFrom)
	toDate := normalizeDate(dateTo)
	if toDate.Before(fromDate) {
		return 0, fmt.Errorf("invalid date range: to (%s) is before from (%s)", toDate.Format("2006-01-02"), fromDate.Format("2006-01-02"))
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	if _, err := qtx.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		return 0, fmt.Errorf("get seller account: %w", err)
	}

	sourceRows, err := qtx.ListDailySKUSourcesBySellerAndDateRange(ctx, dbgen.ListDailySKUSourcesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		Column2:         dateValue(fromDate),
		Column3:         dateValue(toDate),
	})
	if err != nil {
		return 0, fmt.Errorf("list sku metric sources: %w", err)
	}

	stockRows, err := qtx.ListCurrentStockBySellerAccountAndProduct(ctx, sellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("list current stock by product: %w", err)
	}
	stockByProduct := make(map[int64]int32, len(stockRows))
	for _, row := range stockRows {
		stockByProduct[row.OzonProductID] = row.StockAvailable
	}

	ordersByProductDay := make(map[int64]map[string]int32)
	for _, row := range sourceRows {
		dateKey := row.MetricDate.Time.Format("2006-01-02")
		byDay := ordersByProductDay[row.OzonProductID]
		if byDay == nil {
			byDay = make(map[string]int32)
			ordersByProductDay[row.OzonProductID] = byDay
		}
		byDay[dateKey] = row.OrdersCount
	}

	if err := qtx.DeleteDailySKUMetricsBySellerAndDateRange(ctx, dbgen.DeleteDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(fromDate),
		MetricDate_2:    dateValue(toDate),
	}); err != nil {
		return 0, fmt.Errorf("delete sku metrics in range: %w", err)
	}

	for _, row := range sourceRows {
		stockAvailable := stockByProduct[row.OzonProductID]
		daysOfCover := calcDaysOfCover(row.OzonProductID, row.MetricDate.Time, stockAvailable, ordersByProductDay)

		if _, err := qtx.UpsertDailySKUMetric(ctx, dbgen.UpsertDailySKUMetricParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      row.MetricDate,
			OzonProductID:   row.OzonProductID,
			OfferID:         row.OfferID,
			Sku:             row.Sku,
			ProductName:     row.ProductName,
			Revenue:         row.Revenue,
			OrdersCount:     row.OrdersCount,
			UnitsSold:       pgtype.Int4{Int32: row.UnitsSold, Valid: true},
			StockAvailable:  stockAvailable,
			DaysOfCover:     daysOfCover,
		}); err != nil {
			return 0, fmt.Errorf("upsert daily sku metric for date %s product %d: %w", row.MetricDate.Time.Format("2006-01-02"), row.OzonProductID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return len(sourceRows), nil
}

func calcDaysOfCover(productID int64, metricDate time.Time, stockAvailable int32, ordersByProductDay map[int64]map[string]int32) pgtype.Numeric {
	byDay := ordersByProductDay[productID]
	if len(byDay) == 0 && stockAvailable > 0 {
		return pgtype.Numeric{Valid: false}
	}

	var demandSum float64
	for i := 0; i < recentDemandDays; i++ {
		d := metricDate.AddDate(0, 0, -i).Format("2006-01-02")
		demandSum += float64(byDay[d])
	}
	avgDailyDemand := demandSum / recentDemandDays
	if avgDailyDemand <= 0 {
		return pgtype.Numeric{Valid: false}
	}
	// stock=0 with confirmed demand → coverage 0 (not null / "нет данных")
	if stockAvailable <= 0 {
		return numericValue(0)
	}

	value := math.Round((float64(stockAvailable)/avgDailyDemand)*100) / 100
	return numericValue(value)
}

func numericValue(v float64) pgtype.Numeric {
	formatted := fmt.Sprintf("%.2f", v)
	var n pgtype.Numeric
	if err := n.Scan(formatted); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}
