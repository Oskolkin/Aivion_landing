package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	var sellerAccountID int64
	var asOfDate string
	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.StringVar(&asOfDate, "as-of-date", "", "optional metric date in YYYY-MM-DD; defaults to today (UTC)")
	flag.Parse()

	if sellerAccountID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-account-id is required and must be > 0")
		os.Exit(2)
	}

	var asOf *time.Time
	if asOfDate != "" {
		parsed, err := time.Parse("2006-01-02", asOfDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --as-of-date: %v\n", err)
			os.Exit(2)
		}
		asOf = &parsed
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer postgres.Close()

	service := analytics.NewDashboardService(postgres.Pool)
	dto, err := service.BuildDashboardMetrics(ctx, sellerAccountID, asOf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build dashboard metrics failed: %v\n", err)
		os.Exit(1)
	}

	payload, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
