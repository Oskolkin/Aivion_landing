package main

import (
	"context"
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
	var dateFrom string
	var dateTo string

	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.StringVar(&dateFrom, "from", "", "optional start date in YYYY-MM-DD")
	flag.StringVar(&dateTo, "to", "", "optional end date in YYYY-MM-DD")
	flag.Parse()

	if sellerAccountID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-account-id is required and must be > 0")
		os.Exit(2)
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

	service := analytics.NewAccountMetricsService(postgres.Pool)
	if dateFrom != "" || dateTo != "" {
		if dateFrom == "" || dateTo == "" {
			fmt.Fprintln(os.Stderr, "both --from and --to are required when using date range mode")
			os.Exit(2)
		}

		from, err := time.Parse("2006-01-02", dateFrom)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --from date: %v\n", err)
			os.Exit(2)
		}
		to, err := time.Parse("2006-01-02", dateTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --to date: %v\n", err)
			os.Exit(2)
		}

		if _, err := service.RebuildDailyAccountMetricsForDateRange(ctx, sellerAccountID, from, to); err != nil {
			fmt.Fprintf(os.Stderr, "rebuild account metrics by range failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("daily_account_metrics rebuilt for seller_account_id=%d in range %s..%s\n", sellerAccountID, dateFrom, dateTo)
		return
	}

	if err := service.RebuildDailyAccountMetricsForSellerAccount(ctx, sellerAccountID); err != nil {
		fmt.Fprintf(os.Stderr, "rebuild account metrics failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("daily_account_metrics rebuilt for seller_account_id=%d across all source dates\n", sellerAccountID)
}
