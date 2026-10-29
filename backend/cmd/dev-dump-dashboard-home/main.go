package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func main() {
	var sellerID int64
	var output string
	flag.Int64Var(&sellerID, "seller-account-id", 0, "seller_account_id (required)")
	flag.StringVar(&output, "output", "", "output JSON path (required)")
	flag.Parse()
	if sellerID <= 0 || output == "" {
		fmt.Fprintln(os.Stderr, "seller-account-id and output are required")
		os.Exit(2)
	}

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

	svc := dashboardhome.NewService(
		postgres.Pool,
		analytics.NewDashboardService(postgres.Pool),
		analytics.NewCriticalSKUService(postgres.Pool),
		analytics.NewReplenishmentService(postgres.Pool),
		analytics.NewAdvertisingService(postgres.Pool),
		alerts.NewService(alerts.NewSQLCRepository(dbgen.New(postgres.Pool))),
	)
	resp, err := svc.BuildHome(ctx, sellerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BuildHome: %v\n", err)
		os.Exit(1)
	}
	raw, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(output, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s metric_date=%s status=%s narrative_source=%s\n",
		output, resp.MetricDate, resp.Status, resp.AIDiagnostics.AccountSummary.NarrativeSource)
}
