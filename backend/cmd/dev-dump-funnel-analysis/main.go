package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func main() {
	var sellerID int64
	var outDir string
	flag.Int64Var(&sellerID, "seller-account-id", 1, "seller account id")
	flag.StringVar(&outDir, "out-dir", "tmp/funnel-live-dump", "output directory for JSON files")
	flag.Parse()

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer postgres.Close()

	q := dbgen.New(postgres.Pool)
	latest, err := q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerID)
	if err != nil || !latest.Valid {
		fmt.Fprintf(os.Stderr, "metric date: %v\n", err)
		os.Exit(1)
	}
	metricDate := latest.Time.UTC()

	rows, err := q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      latest,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list summaries: %v\n", err)
		os.Exit(1)
	}

	var accountSummaryJSON []byte
	for _, row := range rows {
		if row.SummaryType == "account_daily_summary" && row.Status == "completed" && len(row.SummaryJson) > 0 {
			accountSummaryJSON = row.SummaryJson
			break
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	meta := map[string]any{
		"seller_account_id": sellerID,
		"metric_date":       metricDate.Format("2006-01-02"),
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(filepath.Join(outDir, "meta.json"), meta)

	if len(accountSummaryJSON) == 0 {
		fmt.Println("no completed account_daily_summary found")
		os.Exit(1)
	}

	var payload aisummaries.SummaryPayload
	if err := json.Unmarshal(accountSummaryJSON, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal summary: %v\n", err)
		os.Exit(1)
	}

	// Validated funnel block as stored after ParseAndValidate at generation time.
	if payload.FunnelAIAnalysis != nil {
		writeJSON(filepath.Join(outDir, "validated_funnel_ai_analysis.json"), payload.FunnelAIAnalysis)
	}

	var root map[string]any
	if err := json.Unmarshal(accountSummaryJSON, &root); err == nil {
		if rawFunnel, ok := root["funnel_ai_analysis"]; ok {
			writeJSON(filepath.Join(outDir, "raw_funnel_from_summary_json.json"), rawFunnel)
		}
	}

	pool := postgres.Pool
	dashSvc := dashboardhome.NewService(
		pool,
		analytics.NewDashboardService(pool),
		analytics.NewCriticalSKUService(pool),
		analytics.NewReplenishmentService(pool),
		analytics.NewAdvertisingService(pool),
		alerts.NewService(alerts.NewSQLCRepository(dbgen.New(pool))),
	)
	resp, err := dashSvc.BuildHome(ctx, sellerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dashboard home: %v\n", err)
		os.Exit(1)
	}
	writeJSON(filepath.Join(outDir, "dashboard_home_funnel_ai_analysis.json"), resp.AIDiagnostics.FunnelAnalysis.AIAnalysis)

	fmt.Printf("wrote dumps to %s\n", outDir)
}

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal %s: %v\n", path, err)
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
		return
	}
	fmt.Println("wrote", path)
}
