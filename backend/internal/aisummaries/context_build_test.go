package aisummaries

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSyncDashboardAccountOrdersFromMetrics_usesDailyAccountMetrics(t *testing.T) {
	asOf := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:      101721.33,
			OrdersToday:       76,
			OrdersYesterday:   83,
			CancelsToday:      12,
			AverageOrderValue: analytics.RatioOrNil(101721.33, 76),
		},
	}

	rows := []dbgen.DailyAccountMetric{
		{
			MetricDate:   pgtype.Date{Time: asOf.AddDate(0, 0, -1), Valid: true},
			OrdersCount:  87,
			CancelCount:  8,
			ReturnsCount: 3,
		},
		{
			MetricDate:   pgtype.Date{Time: asOf, Valid: true},
			OrdersCount:  88,
			CancelCount:  12,
			ReturnsCount: 4,
		},
	}

	syncDashboardAccountOrdersFromMetrics(&dto, rows, asOf)

	if dto.Account.OrdersToday != 88 {
		t.Fatalf("orders_today=%d", dto.Account.OrdersToday)
	}
	if dto.Account.OrdersYesterday != 87 {
		t.Fatalf("orders_yesterday=%d", dto.Account.OrdersYesterday)
	}
	if dto.Account.AverageOrderValue == nil {
		t.Fatal("average_order_value should be recomputed")
	}
	if got := *dto.Account.AverageOrderValue; got < 1155 || got > 1157 {
		t.Fatalf("average_order_value=%v", got)
	}
	if dto.Account.CancellationRate == nil {
		t.Fatal("cancellation_rate should be recomputed")
	}
	if got := *dto.Account.CancellationRate; got < 0.136 || got > 0.137 {
		t.Fatalf("cancellation_rate=%v", got)
	}
}

func TestBuildAccountFunnelStageMaps_includesPreviousDayValuesAndChanges(t *testing.T) {
	current := &dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 120, Valid: true},
		ProductViews: pgtype.Int8{Int64: 60, Valid: true},
		AddToCart:    pgtype.Int8{Int64: 30, Valid: true},
		OrdersCount:  pgtype.Int8{Int64: 10, Valid: true},
	}
	previous := &dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 100, Valid: true},
		ProductViews: pgtype.Int8{Int64: 50, Valid: true},
		AddToCart:    pgtype.Int8{Int64: 20, Valid: true},
		OrdersCount:  pgtype.Int8{Int64: 8, Valid: true},
	}

	stages := buildAccountFunnelStageMaps(current, previous)
	if len(stages) != 4 {
		t.Fatalf("stages=%d", len(stages))
	}
	if stages[0]["stage_key"] != "impressions" {
		t.Fatalf("first stage=%v", stages[0]["stage_key"])
	}
	if stages[0]["previous_day_value"] != int64(100) {
		t.Fatalf("previous_day_value=%v", stages[0]["previous_day_value"])
	}
	if stages[0]["change_vs_previous_day_pct"] == nil {
		t.Fatal("expected change_vs_previous_day_pct")
	}
}

func TestBuildFunnel7DContext_includesStageAveragesAndConversionDeltas(t *testing.T) {
	currentStages := []map[string]any{
		{"stage_key": "impressions", "stage_label": "Показы", "value": int64(100), "conversion_from_top": float64(100)},
		{"stage_key": "product_views", "stage_label": "Просмотры карточки", "value": int64(20), "conversion_from_top": float64(20), "conversion_from_previous": float64(20)},
		{"stage_key": "add_to_cart", "stage_label": "Добавления в корзину", "value": int64(5), "conversion_from_top": float64(5), "conversion_from_previous": float64(25)},
	}
	rows := []dbgen.ProductAnalyticsDaily{
		{
			MetricDate:   pgtype.Date{Time: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC), Valid: true},
			Impressions:  pgtype.Int8{Int64: 90, Valid: true},
			ProductViews: pgtype.Int8{Int64: 18, Valid: true},
			AddToCart:    pgtype.Int8{Int64: 6, Valid: true},
			ScopeType:    "account",
		},
		{
			MetricDate:   pgtype.Date{Time: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), Valid: true},
			Impressions:  pgtype.Int8{Int64: 110, Valid: true},
			ProductViews: pgtype.Int8{Int64: 22, Valid: true},
			AddToCart:    pgtype.Int8{Int64: 4, Valid: true},
			ScopeType:    "account",
		},
	}

	ctx := buildFunnel7DContext(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), currentStages, rows, "available")
	stageAverages, ok := ctx["stage_averages"].([]map[string]any)
	if !ok || len(stageAverages) == 0 {
		t.Fatalf("stage_averages=%T %v", ctx["stage_averages"], ctx["stage_averages"])
	}
	transitions, ok := ctx["transition_averages"].([]map[string]any)
	if !ok || len(transitions) == 0 {
		t.Fatalf("transition_averages=%T %v", ctx["transition_averages"], ctx["transition_averages"])
	}
	if stageAverages[0]["average_value"] == nil {
		t.Fatalf("missing average_value: %+v", stageAverages[0])
	}
	if transitions[0]["conversion_diff_pp"] == nil {
		t.Fatalf("missing conversion_diff_pp: %+v", transitions[0])
	}
}

func TestBuildFunnelRelatedAdsContext_limitsWeakCampaigns(t *testing.T) {
	rows := make([]dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow, 0, 7)
	for i := 0; i < 7; i++ {
		rows = append(rows, dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
			CampaignExternalID: int64(i + 1),
			CampaignName:       "Кампания",
			SpendTotal:         mustNumeric(float64(100 + i*10)),
			OrdersTotal:        0,
			RevenueTotal:       mustNumeric(0),
		})
	}
	ctx := buildFunnelRelatedAdsContext(rows, nil)
	weak, ok := ctx["weak_campaigns"].([]map[string]any)
	if !ok {
		t.Fatalf("weak_campaigns=%T", ctx["weak_campaigns"])
	}
	if len(weak) != maxFunnelSignalItems {
		t.Fatalf("weak_campaigns=%d", len(weak))
	}
	zeroOrders, ok := ctx["campaigns_with_spend_zero_orders"].([]map[string]any)
	if !ok || len(zeroOrders) == 0 {
		t.Fatalf("campaigns_with_spend_zero_orders=%v", ctx["campaigns_with_spend_zero_orders"])
	}
}

func TestBuildFunnelRelatedAdsContext_limitsAndSortsCampaignLinks(t *testing.T) {
	rows := []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
		{
			CampaignExternalID: 10,
			CampaignName:       "Risky active",
			SpendTotal:         mustNumeric(5000),
			OrdersTotal:        0,
			RevenueTotal:       mustNumeric(0),
		},
		{
			CampaignExternalID: 20,
			CampaignName:       "Lower risk active",
			SpendTotal:         mustNumeric(900),
			OrdersTotal:        10,
			RevenueTotal:       mustNumeric(10000),
		},
		{
			CampaignExternalID: 30,
			CampaignName:       "Risky inactive",
			SpendTotal:         mustNumeric(4000),
			OrdersTotal:        0,
			RevenueTotal:       mustNumeric(0),
		},
	}
	links := []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{
		{CampaignExternalID: 30, CampaignName: pgtype.Text{String: "Risky inactive", Valid: true}, OzonProductID: 3001, IsActive: false},
		{CampaignExternalID: 20, CampaignName: pgtype.Text{String: "Lower risk active", Valid: true}, OzonProductID: 2001, IsActive: true},
		{CampaignExternalID: 10, CampaignName: pgtype.Text{String: "Risky active", Valid: true}, OzonProductID: 1001, IsActive: true},
		{CampaignExternalID: 10, CampaignName: pgtype.Text{String: "Risky active", Valid: true}, OzonProductID: 1002, IsActive: true},
		{CampaignExternalID: 20, CampaignName: pgtype.Text{String: "Lower risk active", Valid: true}, OzonProductID: 2002, IsActive: true},
		{CampaignExternalID: 30, CampaignName: pgtype.Text{String: "Risky inactive", Valid: true}, OzonProductID: 3002, IsActive: false},
		{CampaignExternalID: 20, CampaignName: pgtype.Text{String: "Lower risk active", Valid: true}, OzonProductID: 2003, IsActive: true},
	}

	ctx := buildFunnelRelatedAdsContext(rows, links)
	got, ok := ctx["campaign_sku_links"].([]map[string]any)
	if !ok {
		t.Fatalf("campaign_sku_links=%T", ctx["campaign_sku_links"])
	}
	if len(got) != maxFunnelSignalItems {
		t.Fatalf("campaign_sku_links len=%d", len(got))
	}
	if got[0]["campaign_external_id"] != int64(10) {
		t.Fatalf("expected risky active campaign first, got %+v", got[0])
	}
	if got[len(got)-1]["campaign_external_id"] == int64(30) {
		t.Fatalf("inactive risky campaign should be deprioritized, got %+v", got)
	}
}

func TestBuildFunnelRelatedSKUContext_includesTopSignals(t *testing.T) {
	share := 0.12
	days := 2.0
	items := []analytics.DashboardSKUMetricsItem{
		{
			OfferID:             strPtr("SKU-1"),
			ProductName:         strPtr("Товар 1"),
			StockAvailable:      0,
			DaysOfCover:         &days,
			OrdersCount:         3,
			OrdersDeltaDayToDay: -2,
			ShareOfRevenue:      &share,
		},
		{
			OfferID:             strPtr("SKU-2"),
			ProductName:         strPtr("Товар 2"),
			StockAvailable:      10,
			OrdersCount:         0,
			OrdersDeltaDayToDay: -1,
		},
	}
	ctx := buildFunnelRelatedSKUContext(items)
	stockRisk, ok := ctx["stock_risk_skus"].([]map[string]any)
	if !ok || len(stockRisk) == 0 {
		t.Fatalf("stock_risk_skus=%v", ctx["stock_risk_skus"])
	}
	orderDrop, ok := ctx["order_drop_skus"].([]map[string]any)
	if !ok || len(orderDrop) == 0 {
		t.Fatalf("order_drop_skus=%v", ctx["order_drop_skus"])
	}
	keyProblems, ok := ctx["key_sku_problems"].([]map[string]any)
	if !ok || len(keyProblems) == 0 {
		t.Fatalf("key_sku_problems=%v", ctx["key_sku_problems"])
	}
}

func TestComposeFunnelRelatedRisksContext_includesCountsAndSignals(t *testing.T) {
	ctx := composeFunnelRelatedRisksContext(
		[]dbgen.CountOpenAlertsByGroupRow{{AlertGroup: "stock", AlertsCount: 2}},
		[]dbgen.CountOpenAlertsBySeverityRow{{Severity: "high", AlertsCount: 2}},
		[]dbgen.CountOpenRecommendationsByPriorityRow{{PriorityLevel: "high", RecommendationsCount: 1}},
		[]dbgen.Alert{{ID: 1, Severity: "high", AlertGroup: "stock", Title: "Риск остатков", Message: "Проверьте товар"}},
		[]dbgen.Recommendation{{ID: 7, PriorityLevel: "high", Title: "Проверить кампанию", RecommendedAction: "Отключить слабую кампанию"}},
	)
	groupCounts, ok := ctx["open_alert_counts_by_group"].([]map[string]any)
	if !ok || len(groupCounts) != 1 {
		t.Fatalf("open_alert_counts_by_group=%v", ctx["open_alert_counts_by_group"])
	}
	alerts, ok := ctx["top_open_alerts"].([]map[string]any)
	if !ok || len(alerts) != 1 {
		t.Fatalf("top_open_alerts=%v", ctx["top_open_alerts"])
	}
	actions, ok := ctx["top_recommended_actions"].([]map[string]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("top_recommended_actions=%v", ctx["top_recommended_actions"])
	}
}

func strPtr(v string) *string { return &v }
