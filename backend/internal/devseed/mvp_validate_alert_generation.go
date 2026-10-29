package devseed

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MVPAlertGenerationValidateOptions configures --validate-alert-generation.
type MVPAlertGenerationValidateOptions struct {
	SellerAccountID int64
	ResetDerived    bool
	// AsOfDate if set is used as the engine as-of calendar day (UTC); otherwise MAX(daily_account_metrics.metric_date) (same anchor as seed validation).
	AsOfDate *time.Time
}

type mvpAlertGenRow struct {
	Group   string
	Count   int64
	OK      bool
	CheckID string
}

// CleanupMVPDerivedAlertsForSeller deletes alert_runs and alerts for one seller (dev tooling).
// Also removes recommendation_alert_links rows that reference those alerts (ON DELETE CASCADE from alerts).
func CleanupMVPDerivedAlertsForSeller(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	// Order: alerts first so CASCADE cleans recommendation_alert_links; alert_runs are independent.
	if err := q.DeleteAlertsBySellerAccountID(ctx, sellerAccountID); err != nil {
		return fmt.Errorf("delete alerts: %w", err)
	}
	if err := q.DeleteAlertRunsBySellerAccountID(ctx, sellerAccountID); err != nil {
		return fmt.Errorf("delete alert_runs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ValidateMVPAlertGeneration runs the production alerts service the same way as POST /api/v1/alerts/run
// (manual run type, as-of date resolved like the handler when body omits as_of_date: anchor from metrics).
// It does not insert alerts directly; it only invokes alerts.Service.RunForAccountWithType.
func ValidateMVPAlertGeneration(ctx context.Context, pool *pgxpool.Pool, opts MVPAlertGenerationValidateOptions) (bool, error) {
	if opts.SellerAccountID <= 0 {
		return false, fmt.Errorf("seller_account_id must be > 0")
	}

	if opts.ResetDerived {
		if err := CleanupMVPDerivedAlertsForSeller(ctx, pool, opts.SellerAccountID); err != nil {
			return false, err
		}
		fmt.Printf("reset-derived: deleted alerts + alert_runs for seller_account_id=%d\n\n", opts.SellerAccountID)
	}

	asOf, err := resolveMVPAlertAsOfDate(ctx, pool, opts.SellerAccountID, opts.AsOfDate)
	if err != nil {
		return false, err
	}

	svc := alerts.NewService(alerts.NewSQLCRepository(dbgen.New(pool)))
	fmt.Printf("Running alerts engine (RunForAccountWithType, run_type=%s, as_of_date=%s)…\n", alerts.RunTypeManual, asOf.Format("2006-01-02"))

	runSummary, err := svc.RunForAccountWithType(ctx, opts.SellerAccountID, asOf, alerts.RunTypeManual)
	if err != nil {
		return false, fmt.Errorf("alerts engine: %w", err)
	}
	if runSummary.Status != alerts.RunStatusCompleted {
		return false, fmt.Errorf("alert run finished with status %s (run_id=%d)", runSummary.Status, runSummary.RunID)
	}

	fmt.Printf("Run completed: run_id=%d total_upserted=%d total_generated=%d\n\n",
		runSummary.RunID, runSummary.TotalUpsertedAlerts, runSummary.TotalGeneratedAlerts)

	q := dbgen.New(pool)
	totalAll, err := q.CountAlertsBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, fmt.Errorf("count alerts: %w", err)
	}

	summary, err := svc.GetSummary(ctx, opts.SellerAccountID)
	if err != nil {
		return false, fmt.Errorf("alerts summary: %w", err)
	}

	openSales := openCountForGroup(summary, alerts.AlertGroupSales)
	openStock := openCountForGroup(summary, alerts.AlertGroupStock)
	openAds := openCountForGroup(summary, alerts.AlertGroupAdvertising)
	openPrice := openCountForGroup(summary, alerts.AlertGroupPriceEconomics)
	openSevHi := openHighOrCriticalCount(summary)

	rows := []mvpAlertGenRow{
		{Group: "alerts_total", Count: totalAll, OK: totalAll > 0, CheckID: "alerts_total"},
		{Group: "open_alerts", Count: summary.OpenTotal, OK: summary.OpenTotal > 0, CheckID: "open_alerts"},
		{Group: "open_sales", Count: openSales, OK: openSales > 0, CheckID: "open_sales"},
		{Group: "open_stock", Count: openStock, OK: openStock > 0, CheckID: "open_stock"},
		{Group: "open_advertising", Count: openAds, OK: openAds > 0, CheckID: "open_advertising"},
		{Group: "open_price_economics", Count: openPrice, OK: openPrice > 0, CheckID: "open_price_economics"},
		{Group: "open_high_or_critical", Count: openSevHi, OK: openSevHi > 0, CheckID: "open_high_or_critical"},
	}

	printMVPAlertGenerationTable(rows)

	allOK := true
	var failed []string
	for _, r := range rows {
		if !r.OK {
			allOK = false
			failed = append(failed, r.CheckID)
		}
	}
	if !allOK {
		fmt.Println()
		fmt.Println("Hints (усилить сиды / пороги):")
		for _, id := range failed {
			fmt.Printf("  • %s: %s\n", id, mvpAlertGenerationHint(id))
		}
	}
	return allOK, nil
}

func resolveMVPAlertAsOfDate(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, override *time.Time) (time.Time, error) {
	if override != nil {
		t := override.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return resolveMVPValidationAnchorDate(ctx, pool, sellerAccountID)
}

func openCountForGroup(s alerts.Summary, g alerts.AlertGroup) int64 {
	for _, row := range s.ByGroup {
		if row.Group == g {
			return row.Count
		}
	}
	return 0
}

func openHighOrCriticalCount(s alerts.Summary) int64 {
	var n int64
	for _, row := range s.BySeverity {
		if row.Severity == alerts.SeverityHigh || row.Severity == alerts.SeverityCritical {
			n += row.Count
		}
	}
	return n
}

func printMVPAlertGenerationTable(rows []mvpAlertGenRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Alert group\tCount\tStatus")
	for _, r := range rows {
		st := "OK"
		if !r.OK {
			st = "FAIL"
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\n", r.Group, r.Count, st)
	}
	_ = w.Flush()
}

func mvpAlertGenerationHint(checkID string) string {
	switch checkID {
	case "alerts_total":
		return "Проверьте, что MVP seed завершился и RebuildMVPAnalytics заполнил daily_account_metrics / daily_sku_metrics; движок не создаёт строки при пустых входных данных."
	case "open_alerts":
		return "Если алерты есть, но все resolved/dismissed, запустите с --reset-derived=true или очистите статусы; иначе усильте правила/входные метрики."
	case "open_sales":
		return "Нужен контраст метрик между as-of и предыдущим днём (выручка/заказы): см. MVP commerce + analytics; при необходимости пороги в internal/alerts/sales_rules.go."
	case "open_stock":
		return "Нужны SKU с низким days_of_cover / нулевым остатком в daily_sku_metrics и stocks: см. dev-seed commerce/stocks и internal/alerts/stock_rules.go."
	case "open_advertising":
		return "Нужны ad_metrics_daily + ad_campaign_skus с «плохими» кампаниями (ROAS, spend без результата): см. MVP ads seed и internal/alerts/advertising_rules.go."
	case "open_price_economics":
		return "Нужны products + sku_effective_constraints + цены вне min/max или margin risk: см. MVP pricing seed и internal/alerts/price_rules.go."
	case "open_high_or_critical":
		return "Нужны более жёсткие условия (крупное падение продаж, OOS, сильный margin risk): см. scoring в internal/alerts/scoring.go и сегменты declining/zero stock в сидах."
	default:
		return "См. internal/alerts и dev-seed MVP генераторы commerce/ads/pricing."
	}
}
