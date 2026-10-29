package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	var sellerAccountID int64
	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
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

	service := analytics.NewStocksViewService(postgres.Pool)
	rows, err := service.ListCurrentStocksBySellerAccount(ctx, sellerAccountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list current stock metrics failed: %v\n", err)
		os.Exit(1)
	}

	payload, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
