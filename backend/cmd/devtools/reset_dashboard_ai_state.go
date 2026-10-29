package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

type resetDashboardAIStateResult struct {
	SellerID   int64    `json:"seller_id"`
	MetricDate string   `json:"metric_date"`
	Deleted    []string `json:"deleted"`
}

func runResetDashboardAIState(args []string) int {
	fs := flag.NewFlagSet("reset-dashboard-ai-state", flag.ContinueOnError)
	var sellerID int64
	var metricDateRaw string
	var confirm bool
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.BoolVar(&confirm, "confirm", false, "confirm destructive reset")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || metricDateRaw == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}
	if !confirm {
		fmt.Fprintln(os.Stderr, "--confirm is required")
		return 2
	}
	if err := devseed.AssertDevSeedAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	md, err := time.Parse("2006-01-02", metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	tx, err := postgres.Pool.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "begin tx: %v\n", err)
		return 1
	}
	defer tx.Rollback(ctx)

	metricDate := md.Format("2006-01-02")
	deleted := make([]string, 0, 8)
	if _, err := tx.Exec(ctx, `DELETE FROM ai_dashboard_narratives WHERE seller_account_id=$1 AND metric_date=$2::date`, sellerID, metricDate); err == nil {
		deleted = append(deleted, "ai_dashboard_narratives")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ai_daily_summaries WHERE seller_account_id=$1 AND metric_date=$2::date`, sellerID, metricDate); err == nil {
		deleted = append(deleted, "ai_daily_summaries")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM ai_daily_summary_runs WHERE seller_account_id=$1 AND metric_date=$2::date`, sellerID, metricDate); err == nil {
		deleted = append(deleted, "ai_daily_summary_runs")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM recommendations WHERE seller_account_id=$1 AND (source IN ('ai_summary','dashboard_narrative','dashboard_narrative_ai_only','risk_driver') OR ai_prompt_version LIKE 'dashboard_narrative%')`, sellerID); err == nil {
		deleted = append(deleted, "recommendations(ai-generated)")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM recommendation_runs WHERE seller_account_id=$1 AND ai_prompt_version LIKE 'dashboard_narrative%'`, sellerID); err == nil {
		deleted = append(deleted, "recommendation_runs(dashboard_narrative)")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM recommendation_alert_links WHERE seller_account_id=$1 AND recommendation_id NOT IN (SELECT id FROM recommendations WHERE seller_account_id=$1)`, sellerID); err == nil {
		deleted = append(deleted, "recommendation_alert_links(orphaned)")
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "commit: %v\n", err)
		return 1
	}

	raw, _ := json.MarshalIndent(resetDashboardAIStateResult{
		SellerID:   sellerID,
		MetricDate: metricDate,
		Deleted:    deleted,
	}, "", "  ")
	fmt.Println(string(raw))
	return 0
}
