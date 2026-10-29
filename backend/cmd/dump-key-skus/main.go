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

	dto, err := analytics.NewDashboardService(pool).BuildDashboardMetrics(ctx, sellerID, nil)
	if err != nil {
		panic(err)
	}

	focus := resp.SKUFocus
	_, _, allKeyCards := dashboardhome.KeySkuSelectionFromDashboard(dto.SKUs)

	var selectedRevenueSum float64
	for _, card := range allKeyCards {
		selectedRevenueSum += card.Revenue7d
	}

	lastCumulative := 0.0
	if len(allKeyCards) > 0 {
		lastCumulative = allKeyCards[len(allKeyCards)-1].CumulativeRevenueSharePct
	}

	fmt.Println("=== key SKU verification ===")
	fmt.Printf("metric_date=%s\n", resp.MetricDate)
	fmt.Printf("store_total_revenue_7d=%.2f\n", focus.StoreTotalRevenue7d)
	fmt.Printf("key_skus_total_count=%d\n", focus.KeySKUsTotalCount)
	fmt.Printf("key_skus_total_revenue_7d=%.2f\n", focus.KeySKUsTotalRevenue7d)
	fmt.Printf("sum_revenue_7d_selected_key_skus=%.2f\n", selectedRevenueSum)
	fmt.Printf("last_cumulative_revenue_share_pct=%.1f\n", lastCumulative)
	fmt.Printf("cumulative_gte_80=%v\n", lastCumulative >= 80)
	fmt.Printf("dashboard_preview_count=%d\n", len(focus.KeySKUs))
	fmt.Println()
	for i, card := range allKeyCards {
		fmt.Printf("%d. %s | revenue_7d=%.2f | share=%.1f%% | cumulative=%.1f%%\n",
			i+1, card.ProductName, card.Revenue7d, card.RevenueSharePct, card.CumulativeRevenueSharePct)
	}

	out := map[string]any{
		"metric_date":                  resp.MetricDate,
		"store_total_revenue_7d":       focus.StoreTotalRevenue7d,
		"key_skus_total_count":         focus.KeySKUsTotalCount,
		"key_skus_total_revenue_7d":    focus.KeySKUsTotalRevenue7d,
		"sum_revenue_7d_selected_keys": selectedRevenueSum,
		"verification": map[string]any{
			"last_cumulative_revenue_share_pct": lastCumulative,
			"cumulative_gte_80":                 lastCumulative >= 80,
		},
		"key_skus_all": allKeyCards,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println()
	fmt.Println(string(b))
}
