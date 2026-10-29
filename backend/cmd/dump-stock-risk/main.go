package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	sellerID := int64(1)
	svc := dashboardhome.NewService(
		pool,
		analytics.NewDashboardService(pool),
		analytics.NewCriticalSKUService(pool),
		analytics.NewReplenishmentService(pool),
		analytics.NewAdvertisingService(pool),
		alerts.NewService(alerts.NewSQLCRepository(dbgen.New(pool))),
	)

	resp, err := svc.BuildHome(ctx, sellerID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("metric_date=%s seller_account_id=%d stock_risk_count=%d\n", resp.MetricDate, sellerID, len(resp.SKUFocus.StockRisk))
	for i, card := range resp.SKUFocus.StockRisk {
		fmt.Printf("\n--- stock_risk[%d] ---\n", i)
		b, _ := json.MarshalIndent(card, "", "  ")
		fmt.Println(string(b))
	}
}
