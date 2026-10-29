package devseed

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReconciliationOptions configures the read-only source→metrics reconciliation used by
// `dev-seed-mvp --validate-only` (and devtools validate-seller-analysis-data) to prove
// the source-first seed is internally consistent.
type ReconciliationOptions struct {
	SellerAccountID int64
	MetricDate      time.Time
	HistoryDays     int
}

// ReconciliationReport and ReconciliationIssue are re-exported from the shared
// sourcerecon package so callers (CLI, devtools, dashboardfacts) work with a
// single canonical report shape.
type ReconciliationReport = sourcerecon.ReconciliationReport
type ReconciliationIssue = sourcerecon.ReconciliationIssue

// ReconcileSellerAnalysisData loads the seller's analysis-data snapshot and runs the
// shared snapshot reconciliation (sourcerecon.ReconcileSnapshot), then layers the
// DB-only source-of-truth checks that cannot be expressed on the snapshot alone:
//
//   - Account ↔ source (orders / sales revenue / units) per day, recomputed from
//     the raw orders + sales tables.
//   - Raw stocks aggregate ↔ daily SKU stock aggregate on the anchor date.
//
// The returned report is finalized (sorted, Valid set, Hash computed).
func ReconcileSellerAnalysisData(ctx context.Context, pool *pgxpool.Pool, opts ReconciliationOptions) (ReconciliationReport, error) {
	if opts.SellerAccountID <= 0 {
		return ReconciliationReport{}, fmt.Errorf("reconciliation: seller_account_id must be > 0")
	}

	anchor := MVPDateUTC(opts.MetricDate)
	if opts.MetricDate.IsZero() {
		var maxDate *time.Time
		if err := pool.QueryRow(ctx,
			`SELECT MAX(metric_date)::timestamptz FROM daily_account_metrics WHERE seller_account_id = $1`,
			opts.SellerAccountID).Scan(&maxDate); err != nil {
			return ReconciliationReport{}, fmt.Errorf("reconciliation: resolve anchor: %w", err)
		}
		if maxDate == nil {
			return ReconciliationReport{}, fmt.Errorf("reconciliation: no daily_account_metrics for seller %d", opts.SellerAccountID)
		}
		anchor = MVPDateUTC(*maxDate)
	}

	history := opts.HistoryDays
	if history <= 0 {
		history = 30
	}

	loader := analysisdata.NewSQLLoader(pool)
	snap, err := loader.LoadWithOptions(ctx, analysisdata.LoadOptions{
		SellerAccountID: opts.SellerAccountID,
		MetricDate:      anchor,
		HistoryDays:     history,
	})
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("reconciliation: load snapshot: %w", err)
	}

	report, err := sourcerecon.ReconcileSnapshot(snap)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("reconciliation: reconcile snapshot: %w", err)
	}

	if err := appendAccountVsSourceChecks(ctx, pool, &report, snap, anchor, history); err != nil {
		return ReconciliationReport{}, err
	}
	if err := appendRawStockChecks(ctx, pool, &report, snap, anchor); err != nil {
		return ReconciliationReport{}, err
	}

	return sourcerecon.Finalize(report), nil
}

// appendAccountVsSourceChecks recomputes the account-level metrics directly from the
// raw orders + sales tables and compares them, per day, against the stored
// daily_account_metrics carried by the snapshot.
func appendAccountVsSourceChecks(
	ctx context.Context,
	pool *pgxpool.Pool,
	report *ReconciliationReport,
	snap *analysisdata.AnalysisDataSnapshot,
	anchor time.Time,
	history int,
) error {
	from := anchor.AddDate(0, 0, -(history - 1))
	queries := dbgen.New(pool)
	sources, err := queries.ListDailyAccountMetricSourcesBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricSourcesBySellerAndDateRangeParams{
		SellerAccountID: snap.SellerAccountID,
		Column2:         pgtype.Date{Time: from, Valid: true},
		Column3:         pgtype.Date{Time: anchor, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("reconciliation: list account metric sources: %w", err)
	}

	type srcDay struct {
		revenue float64
		orders  int64
		returns int64
		cancels int64
		units   int64
	}
	sourceByDay := make(map[string]srcDay, len(sources))
	for _, s := range sources {
		if !s.MetricDate.Valid {
			continue
		}
		if analysisdata.DateOnlyUTC(s.MetricDate.Time).After(anchor) {
			continue
		}
		sourceByDay[analysisdata.FormatDate(s.MetricDate.Time)] = srcDay{
			revenue: numericFloat(s.Revenue),
			orders:  int64(s.OrdersCount),
			returns: int64(s.ReturnsCount),
			cancels: int64(s.CancelCount),
			units:   int64(s.UnitsSold),
		}
	}

	accByDay := make(map[string]dbgen.DailyAccountMetric, len(snap.AccountMetrics))
	for _, r := range snap.AccountMetrics {
		if !r.MetricDate.Valid {
			continue
		}
		accByDay[analysisdata.FormatDate(r.MetricDate.Time)] = r
	}

	for day, src := range sourceByDay {
		acc, ok := accByDay[day]
		if !ok {
			// The source produced business activity for this day but the stored
			// account metric is missing — a trimmed/incomplete rebuild.
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_missing_for_source_day", Path: "daily_account_metrics", Date: day,
				Expected: src.orders, Actual: nil,
			})
			continue
		}
		if relDiff(src.revenue, numericFloat(acc.Revenue)) > sourcerecon.RevenueTolerance {
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_source_revenue_mismatch", Path: "daily_account_metrics.revenue", Date: day,
				Expected: round2(src.revenue), Actual: round2(numericFloat(acc.Revenue)),
			})
		}
		if src.orders != int64(acc.OrdersCount) {
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_source_orders_mismatch", Path: "daily_account_metrics.orders_count", Date: day,
				Expected: src.orders, Actual: int64(acc.OrdersCount),
			})
		}
		if src.cancels != int64(acc.CancelCount) {
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_source_cancels_mismatch", Path: "daily_account_metrics.cancel_count", Date: day,
				Expected: src.cancels, Actual: int64(acc.CancelCount),
			})
		}
		if src.returns != int64(acc.ReturnsCount) {
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_source_returns_mismatch", Path: "daily_account_metrics.returns_count", Date: day,
				Expected: src.returns, Actual: int64(acc.ReturnsCount),
			})
		}
		if accUnits := int64(int32FromInt4(acc.UnitsSold)); src.units != accUnits {
			report.Warnings = append(report.Warnings, ReconciliationIssue{
				Code: "account_source_units_mismatch", Path: "daily_account_metrics.units_sold", Date: day,
				Expected: src.units, Actual: accUnits,
			})
		}
	}

	// Stored account rows with no matching source day were fabricated.
	for day := range accByDay {
		if _, ok := sourceByDay[day]; !ok {
			report.Errors = append(report.Errors, ReconciliationIssue{
				Code: "account_without_source", Path: "daily_account_metrics", Date: day,
			})
		}
	}

	if report.Counts == nil {
		report.Counts = map[string]int64{}
	}
	report.Counts["source_account_days"] = int64(len(sourceByDay))
	return nil
}

// appendRawStockChecks aggregates the raw stocks table on the anchor and compares it
// to the daily SKU stock aggregate and the stock-analytics aggregate (three-way).
func appendRawStockChecks(
	ctx context.Context,
	pool *pgxpool.Pool,
	report *ReconciliationReport,
	snap *analysisdata.AnalysisDataSnapshot,
	anchor time.Time,
) error {
	queries := dbgen.New(pool)
	rawRows, err := queries.ListCurrentStockBySellerAccountAndProduct(ctx, snap.SellerAccountID)
	if err != nil {
		return fmt.Errorf("reconciliation: list current stock: %w", err)
	}
	var rawStock int64
	for _, r := range rawRows {
		rawStock += int64(r.StockAvailable)
	}

	anchorKey := analysisdata.FormatDate(anchor)
	var skuStock int64
	for _, r := range snap.SKUMetrics {
		if r.MetricDate.Valid && analysisdata.FormatDate(r.MetricDate.Time) == anchorKey {
			skuStock += int64(r.StockAvailable)
		}
	}
	var analyticsStock int64
	analyticsRows := 0
	for _, r := range snap.StockSnapshots {
		if r.SnapshotDate.Valid && analysisdata.FormatDate(r.SnapshotDate.Time) == anchorKey {
			analyticsStock += int64(r.AvailableStockCount)
			analyticsRows++
		}
	}

	if report.Counts == nil {
		report.Counts = map[string]int64{}
	}
	report.Counts["raw_stock_available"] = rawStock

	// Raw stocks and the daily SKU stock aggregate both derive from the stocks
	// table; the "current" raw view can include multiple snapshot times so a
	// divergence is surfaced as a warning rather than a hard failure.
	if len(rawRows) > 0 && rawStock != skuStock {
		report.Warnings = append(report.Warnings, ReconciliationIssue{
			Code: "stock_raw_vs_sku_mismatch", Path: "stocks", Date: anchorKey,
			Expected: skuStock, Actual: rawStock,
		})
	}
	if analyticsRows > 0 && len(rawRows) > 0 && rawStock != analyticsStock {
		report.Warnings = append(report.Warnings, ReconciliationIssue{
			Code: "stock_raw_vs_analytics_mismatch", Path: "stock_analytics_snapshots", Date: anchorKey,
			Expected: analyticsStock, Actual: rawStock,
		})
	}
	return nil
}

func numericFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}

func int32FromInt4(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func relDiff(a, b float64) float64 {
	denom := math.Max(1, math.Abs(b))
	return math.Abs(a-b) / denom
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
