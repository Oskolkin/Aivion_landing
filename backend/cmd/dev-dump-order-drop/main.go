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
	flag.Int64Var(&sellerID, "seller-account-id", 1, "seller account id")
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

	if len(resp.SKUFocus.OrderDrop) == 0 {
		fmt.Println("order_drop is empty")
		os.Exit(1)
	}

	first := resp.SKUFocus.OrderDrop[0]
	out, _ := json.MarshalIndent(map[string]any{
		"product_name":             first.ProductName,
		"previous_orders":          first.PreviousOrders,
		"current_orders":           first.CurrentOrders,
		"orders_delta_day_to_day":  first.OrdersDeltaDayToDay,
		"revenue_delta_day_to_day": first.RevenueDeltaDayToDay,
		"sparkline_7d":             first.Sparkline7D,
	}, "", "  ")
	fmt.Println(string(out))

	var bad int64
	_ = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM daily_sku_metrics
WHERE seller_account_id = $1 AND orders_count > 0 AND COALESCE(revenue, 0) = 0`, sellerID).Scan(&bad)
	fmt.Printf("\nSQL orders>0 revenue=0: %d\n", bad)
	fmt.Printf("order_drop count: %d\n", len(resp.SKUFocus.OrderDrop))
	for i, card := range resp.SKUFocus.OrderDrop {
		fmt.Printf("%d. %s orders_delta=%v revenue_delta=%v\n", i+1, card.ProductName, derefI32(card.OrdersDeltaDayToDay), derefF64(card.RevenueDeltaDayToDay))
	}
}

func derefI32(v *int32) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefF64(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
