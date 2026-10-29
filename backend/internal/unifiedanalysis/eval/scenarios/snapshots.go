package scenarios

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// SnapshotStockoutKeySKU builds a pack where the key revenue SKU is out of stock.
func SnapshotStockoutKeySKU(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, asOf := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		dayIdx := i
		keyOrders := int32(12)
		if dayIdx >= 23 {
			keyOrders = 14
		}
		healthyOrders := int32(3)

		accountDays = append(accountDays, dailyAccountRow(t, d, keyOrders+healthyOrders, float64((keyOrders+healthyOrders)*100)))
		skuDays = append(skuDays,
			dailySKURow(t, d, ProductKeyOut, SKUKeyOut, "eval-key-out", "Eval Key SKU", keyOrders, float64(keyOrders*100), 0),
			dailySKURow(t, d, ProductHealthy, SKUHealthy, "eval-healthy", "Eval Healthy SKU", healthyOrders, float64(healthyOrders*80), 120),
		)
		funnelDays = append(funnelDays, dailyFunnelRow(d, 5000, 1200, 120, int64(keyOrders+healthyOrders)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products: []dbgen.Product{
			productRow(t, ProductKeyOut, SKUKeyOut, "eval-key-out", "Eval Key SKU", "999.00"),
			productRow(t, ProductHealthy, SKUHealthy, "eval-healthy", "Eval Healthy SKU", "799.00"),
		},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{
			stockRow(t, SKUKeyOut, "eval-key-out", "Eval Key SKU", 0, "DEFICIT", 0),
			stockRow(t, SKUHealthy, "eval-healthy", "Eval Healthy SKU", 120, "ACTUAL", 0),
		},
		Campaigns:    []dbgen.AdCampaign{campaignRow(CampaignHealthy, "Eval Healthy Campaign")},
		AdMetrics:    []dbgen.AdMetricsDaily{},
		CampaignSKUs: []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{campaignSKU(CampaignHealthy, SKUHealthy)},
		FunnelRows:   funnelDays,
	}
	_ = asOf
	return finishSnapshot(t, snap)
}

// SnapshotAdSpendWithoutRevenue builds a pack with ad spend but zero orders/revenue on metric date.
func SnapshotAdSpendWithoutRevenue(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var adDays []dbgen.AdMetricsDaily
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		orders := int32(8)
		isMetricDay := d.Equal(from.AddDate(0, 0, 29))

		spend := 250.0
		adRevenue := 400.0
		adOrders := int32(2)
		if isMetricDay {
			spend = 500.0
			adRevenue = 0
			adOrders = 0
		}

		accountDays = append(accountDays, dailyAccountRow(t, d, orders, float64(orders*120)))
		skuDays = append(skuDays, dailySKURow(t, d, ProductHealthy, SKUHealthy, "eval-ad-sku", "Eval Ad SKU", orders, float64(orders*120), 80))
		adDays = append(adDays, dailyAdRow(t, d, CampaignSpendNoOrders, "Eval Spend No Orders", 3000, 150, spend, adRevenue, adOrders))
		funnelDays = append(funnelDays, dailyFunnelRow(d, 4000, 900, 90, int64(orders)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products:       []dbgen.Product{productRow(t, ProductHealthy, SKUHealthy, "eval-ad-sku", "Eval Ad SKU", "899.00")},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{stockRow(t, SKUHealthy, "eval-ad-sku", "Eval Ad SKU", 80, "ACTUAL", 0)},
		Campaigns:      []dbgen.AdCampaign{campaignRow(CampaignSpendNoOrders, "Eval Spend No Orders")},
		AdMetrics:      adDays,
		CampaignSKUs:   []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{campaignSKU(CampaignSpendNoOrders, SKUHealthy)},
		FunnelRows:     funnelDays,
	}
	return finishSnapshot(t, snap)
}

// SnapshotFunnelConversionDrop builds a pack with a sharp view→cart conversion drop in current 7d.
func SnapshotFunnelConversionDrop(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		orders := int32(10)
		if i >= 23 {
			orders = 6
		}

		views := int64(1000)
		cart := int64(100)
		if i >= 23 {
			cart = 20
		}

		accountDays = append(accountDays, dailyAccountRow(t, d, orders, float64(orders*100)))
		skuDays = append(skuDays, dailySKURow(t, d, ProductHealthy, SKUHealthy, "eval-funnel", "Eval Funnel SKU", orders, float64(orders*100), 60))
		funnelDays = append(funnelDays, dailyFunnelRow(d, 5000, views, cart, int64(orders)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products:       []dbgen.Product{productRow(t, ProductHealthy, SKUHealthy, "eval-funnel", "Eval Funnel SKU", "850.00")},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{stockRow(t, SKUHealthy, "eval-funnel", "Eval Funnel SKU", 60, "ACTUAL", 0)},
		Campaigns:      []dbgen.AdCampaign{campaignRow(CampaignHealthy, "Eval Funnel Campaign")},
		AdMetrics:      []dbgen.AdMetricsDaily{},
		CampaignSKUs:   []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{campaignSKU(CampaignHealthy, SKUHealthy)},
		FunnelRows:     funnelDays,
	}
	return finishSnapshot(t, snap)
}

// SnapshotHealthyNoActions builds a stable healthy account with no material issues.
func SnapshotHealthyNoActions(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var adDays []dbgen.AdMetricsDaily
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		orders := int32(10 + i%2)

		accountDays = append(accountDays, dailyAccountRow(t, d, orders, float64(orders*150)))
		skuDays = append(skuDays, dailySKURow(t, d, ProductHealthy, SKUHealthy, "eval-healthy", "Eval Healthy SKU", orders, float64(orders*150), 100))
		adDays = append(adDays, dailyAdRow(t, d, CampaignHealthy, "Eval Healthy Campaign", 2000, 100, 80, 600, 3))
		funnelDays = append(funnelDays, dailyFunnelRow(d, 5000, 1200, 150, int64(orders)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products:       []dbgen.Product{productRow(t, ProductHealthy, SKUHealthy, "eval-healthy", "Eval Healthy SKU", "950.00")},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{stockRow(t, SKUHealthy, "eval-healthy", "Eval Healthy SKU", 100, "ACTUAL", 0)},
		Campaigns:      []dbgen.AdCampaign{campaignRow(CampaignHealthy, "Eval Healthy Campaign")},
		AdMetrics:      adDays,
		CampaignSKUs:   []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{campaignSKU(CampaignHealthy, SKUHealthy)},
		FunnelRows:     funnelDays,
	}
	return finishSnapshot(t, snap)
}

// SnapshotInsufficientData builds a sparse snapshot with almost no source rows.
func SnapshotInsufficientData(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	asOf := analysisdata.DateOnlyUTC(metricDate)
	from := asOf.AddDate(0, 0, -29)
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: SellerAccountID,
		MetricDate:      pgDate(metricDate),
		Windows:         baseWindows(),
		LoadedHistory:   baseLoadedHistory(from, asOf),
	}
}

// SnapshotMultiSKUGrouping builds a pack with multiple distinct SKU problem signals.
func SnapshotMultiSKUGrouping(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		keyOrders := int32(10)
		dropOrders := int32(8)
		if i >= 23 {
			dropOrders = 2
		}
		noSalesOrders := int32(0)
		slowOrders := int32(1)

		total := keyOrders + dropOrders + noSalesOrders + slowOrders
		accountDays = append(accountDays, dailyAccountRow(t, d, total, float64(total*100)))
		skuDays = append(skuDays,
			dailySKURow(t, d, ProductKeyOut, SKUKeyOut, "eval-multi-key", "Eval Multi Key", keyOrders, float64(keyOrders*120), 0),
			dailySKURow(t, d, ProductOrderDrop, SKUOrderDrop, "eval-multi-drop", "Eval Multi Drop", dropOrders, float64(dropOrders*90), 50),
			dailySKURow(t, d, ProductNoSales, SKUNoSales, "eval-multi-nosales", "Eval Multi No Sales", noSalesOrders, 0, 40),
			dailySKURow(t, d, ProductSlowTurnover, SKUSlowTurnover, "eval-multi-slow", "Eval Multi Slow", slowOrders, float64(slowOrders*70), 200),
		)
		funnelDays = append(funnelDays, dailyFunnelRow(d, 5000, 1000, 100, int64(total)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products: []dbgen.Product{
			productRow(t, ProductKeyOut, SKUKeyOut, "eval-multi-key", "Eval Multi Key", "999.00"),
			productRow(t, ProductOrderDrop, SKUOrderDrop, "eval-multi-drop", "Eval Multi Drop", "799.00"),
			productRow(t, ProductNoSales, SKUNoSales, "eval-multi-nosales", "Eval Multi No Sales", "599.00"),
			productRow(t, ProductSlowTurnover, SKUSlowTurnover, "eval-multi-slow", "Eval Multi Slow", "499.00"),
		},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{
			stockRow(t, SKUKeyOut, "eval-multi-key", "Eval Multi Key", 0, "DEFICIT", 0),
			stockRow(t, SKUOrderDrop, "eval-multi-drop", "Eval Multi Drop", 50, "ACTUAL", 0),
			stockRow(t, SKUNoSales, "eval-multi-nosales", "Eval Multi No Sales", 40, "ACTUAL", 14),
			stockRow(t, SKUSlowTurnover, "eval-multi-slow", "Eval Multi Slow", 200, "SURPLUS", 0),
		},
		Campaigns:    []dbgen.AdCampaign{campaignRow(CampaignHealthy, "Eval Multi Campaign")},
		AdMetrics:    []dbgen.AdMetricsDaily{},
		CampaignSKUs: []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{campaignSKU(CampaignHealthy, SKUKeyOut)},
		FunnelRows:   funnelDays,
	}
	return finishSnapshot(t, snap)
}

// SnapshotSlowTurnoverSurplus builds a pack dominated by slow-turnover surplus SKUs.
func SnapshotSlowTurnoverSurplus(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		orders := int32(1)
		accountDays = append(accountDays, dailyAccountRow(t, d, orders, float64(orders*80)))
		skuDays = append(skuDays, dailySKURow(t, d, ProductSlowTurnover, SKUSlowTurnover, "eval-slow", "Eval Slow SKU", orders, float64(orders*80), 500))
		funnelDays = append(funnelDays, dailyFunnelRow(d, 2000, 400, 10, int64(orders)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products:       []dbgen.Product{productRow(t, ProductSlowTurnover, SKUSlowTurnover, "eval-slow", "Eval Slow SKU", "450.00")},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{stockRow(t, SKUSlowTurnover, "eval-slow", "Eval Slow SKU", 500, "SURPLUS", 7)},
		Campaigns:      []dbgen.AdCampaign{},
		AdMetrics:      []dbgen.AdMetricsDaily{},
		CampaignSKUs:   []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{},
		FunnelRows:     funnelDays,
	}
	return finishSnapshot(t, snap)
}

// SnapshotMixedAccount combines stock, ad, and funnel stress signals at account level.
func SnapshotMixedAccount(t SnapshotHarness) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	from, _ := dateRange30(t)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var adDays []dbgen.AdMetricsDaily
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		keyOrders := int32(8)
		healthyOrders := int32(4)
		total := keyOrders + healthyOrders

		spend := 120.0
		adRev := 200.0
		adOrd := int32(1)
		if i >= 23 {
			spend = 350.0
			adRev = 100.0
		}

		cart := int64(80)
		if i >= 23 {
			cart = 25
		}

		accountDays = append(accountDays, dailyAccountRow(t, d, total, float64(total*110)))
		skuDays = append(skuDays,
			dailySKURow(t, d, ProductKeyOut, SKUKeyOut, "eval-mixed-key", "Eval Mixed Key", keyOrders, float64(keyOrders*110), 0),
			dailySKURow(t, d, ProductHealthy, SKUHealthy, "eval-mixed-ok", "Eval Mixed OK", healthyOrders, float64(healthyOrders*90), 90),
		)
		adDays = append(adDays, dailyAdRow(t, d, CampaignHighDRR, "Eval Mixed High DRR", 2500, 120, spend, adRev, adOrd))
		funnelDays = append(funnelDays, dailyFunnelRow(d, 4500, 900, cart, int64(total)))
	}

	snap := &analysisdata.AnalysisDataSnapshot{
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products: []dbgen.Product{
			productRow(t, ProductKeyOut, SKUKeyOut, "eval-mixed-key", "Eval Mixed Key", "990.00"),
			productRow(t, ProductHealthy, SKUHealthy, "eval-mixed-ok", "Eval Mixed OK", "790.00"),
		},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{
			stockRow(t, SKUKeyOut, "eval-mixed-key", "Eval Mixed Key", 0, "DEFICIT", 0),
			stockRow(t, SKUHealthy, "eval-mixed-ok", "Eval Mixed OK", 90, "ACTUAL", 0),
		},
		Campaigns: []dbgen.AdCampaign{
			campaignRow(CampaignHighDRR, "Eval Mixed High DRR"),
			campaignRow(CampaignSpendNoOrders, "Eval Mixed Spend"),
		},
		AdMetrics: adDays,
		CampaignSKUs: []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{
			campaignSKU(CampaignHighDRR, SKUKeyOut),
			campaignSKU(CampaignSpendNoOrders, SKUHealthy),
		},
		FunnelRows: funnelDays,
	}
	return finishSnapshot(t, snap)
}
