package scenarios

import (
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// Deterministic fictional IDs — no PII, no dependency on seller 1 seed DB.
const (
	SellerAccountID = int64(9042)

	SKUKeyOut       int64 = 901042000001
	SKUHealthy      int64 = 901042000002
	SKUOrderDrop    int64 = 901042000003
	SKUNoSales      int64 = 901042000004
	SKUSlowTurnover int64 = 901042000005

	ProductKeyOut       int64 = 911042000001
	ProductHealthy      int64 = 911042000002
	ProductOrderDrop    int64 = 911042000003
	ProductNoSales      int64 = 911042000004
	ProductSlowTurnover int64 = 911042000005

	CampaignSpendNoOrders int64 = 882042000001
	CampaignHealthy       int64 = 882042000002
	CampaignHighDRR       int64 = 882042000003
)

var metricDate = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

// EvalMetricDate returns the fixed metric date shared by all eval scenario packs.
func EvalMetricDate() time.Time {
	return analysisdata.DateOnlyUTC(metricDate)
}

func mustNumeric(t SnapshotHarness, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("numeric %q: %v", s, err)
	}
	return n
}

func pgDate(d time.Time) pgtype.Date {
	return pgtype.Date{Time: analysisdata.DateOnlyUTC(d), Valid: true}
}

func baseWindows() analysisdata.DataWindows {
	return analysisdata.ComputeDataWindows(metricDate)
}

func baseLoadedHistory(from, to time.Time) analysisdata.LoadedHistory {
	return analysisdata.LoadedHistory{
		From: analysisdata.FormatDate(from),
		To:   analysisdata.FormatDate(to),
		Days: int(to.Sub(from).Hours()/24) + 1,
	}
}

func dailyAccountRow(t SnapshotHarness, d time.Time, orders int32, revenue float64) dbgen.DailyAccountMetric {
	t.Helper()
	return dbgen.DailyAccountMetric{
		SellerAccountID: SellerAccountID,
		MetricDate:      pgDate(d),
		Revenue:         mustNumeric(t, fmt.Sprintf("%.2f", revenue)),
		OrdersCount:     orders,
		ReturnsCount:    0,
		CancelCount:     0,
		UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
	}
}

func dailySKURow(t SnapshotHarness, d time.Time, productID, sku int64, offer, name string, orders int32, revenue float64, stock int32) dbgen.DailySkuMetric {
	t.Helper()
	return dbgen.DailySkuMetric{
		SellerAccountID: SellerAccountID,
		MetricDate:      pgDate(d),
		OzonProductID:   productID,
		OfferID:         pgtype.Text{String: offer, Valid: true},
		Sku:             pgtype.Int8{Int64: sku, Valid: true},
		ProductName:     pgtype.Text{String: name, Valid: true},
		Revenue:         mustNumeric(t, fmt.Sprintf("%.2f", revenue)),
		OrdersCount:     orders,
		StockAvailable:  stock,
		UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
	}
}

func dailyAdRow(t SnapshotHarness, d time.Time, campaignID int64, name string, impressions, clicks int64, spend, revenue float64, orders int32) dbgen.AdMetricsDaily {
	t.Helper()
	return dbgen.AdMetricsDaily{
		SellerAccountID:    SellerAccountID,
		CampaignExternalID: campaignID,
		MetricDate:         pgDate(d),
		Impressions:        impressions,
		Clicks:             clicks,
		Spend:              mustNumeric(t, fmt.Sprintf("%.2f", spend)),
		OrdersCount:        orders,
		Revenue:            mustNumeric(t, fmt.Sprintf("%.2f", revenue)),
		CampaignName:       pgtype.Text{String: name, Valid: true},
	}
}

func dailyFunnelRow(d time.Time, impressions, views, cart, orders int64) dbgen.ProductAnalyticsDaily {
	return dbgen.ProductAnalyticsDaily{
		SellerAccountID: SellerAccountID,
		MetricDate:      pgDate(d),
		ScopeType:       "account",
		ScopeID:         "",
		Impressions:     pgtype.Int8{Int64: impressions, Valid: true},
		ProductViews:    pgtype.Int8{Int64: views, Valid: true},
		AddToCart:       pgtype.Int8{Int64: cart, Valid: true},
		OrdersCount:     pgtype.Int8{Int64: orders, Valid: true},
		CompletedOrders: pgtype.Int8{Int64: orders, Valid: true},
		SourceStatus:    pgtype.Text{String: "ok", Valid: true},
	}
}

func productRow(t SnapshotHarness, productID, sku int64, offer, name, price string) dbgen.Product {
	t.Helper()
	return dbgen.Product{
		ID:              productID - 910000000000,
		SellerAccountID: SellerAccountID,
		OzonProductID:   productID,
		OfferID:         pgtype.Text{String: offer, Valid: true},
		Sku:             pgtype.Int8{Int64: sku, Valid: true},
		Name:            name,
		Status:          pgtype.Text{String: "active", Valid: true},
		ReferencePrice:  mustNumeric(t, price),
	}
}

func stockRow(t SnapshotHarness, sku int64, offer, name string, available int32, grade string, daysWithoutSales int32) dbgen.StockAnalyticsSnapshot {
	t.Helper()
	return dbgen.StockAnalyticsSnapshot{
		SellerAccountID:     SellerAccountID,
		SnapshotDate:        pgDate(metricDate),
		Sku:                 sku,
		OfferID:             pgtype.Text{String: offer, Valid: true},
		ProductName:         pgtype.Text{String: name, Valid: true},
		WarehouseID:         1,
		WarehouseName:       pgtype.Text{String: "WH-SCENARIO", Valid: true},
		AvailableStockCount: available,
		Ads:                 mustNumeric(t, "1.00"),
		Idc:                 pgtype.Int4{Int32: 30, Valid: true},
		TurnoverGrade:       pgtype.Text{String: grade, Valid: true},
		DaysWithoutSales:    pgtype.Int4{Int32: daysWithoutSales, Valid: true},
	}
}

func campaignRow(id int64, name string) dbgen.AdCampaign {
	return dbgen.AdCampaign{
		SellerAccountID:    SellerAccountID,
		CampaignExternalID: id,
		CampaignName:       name,
		CampaignType:       pgtype.Text{String: "SKU", Valid: true},
		Status:             pgtype.Text{String: "CAMPAIGN_STATE_RUNNING", Valid: true},
	}
}

func campaignSKU(campaignID, sku int64) dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow {
	return dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{
		CampaignExternalID: campaignID,
		Sku:                pgtype.Int8{Int64: sku, Valid: true},
	}
}

func enrichmentRun() *dbgen.OzonEnrichmentSyncRun {
	return &dbgen.OzonEnrichmentSyncRun{
		Scope:  "eval_scenario",
		Status: "succeeded",
	}
}

func finishSnapshot(t SnapshotHarness, snap *analysisdata.AnalysisDataSnapshot) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	asOf := analysisdata.DateOnlyUTC(metricDate)
	from := asOf.AddDate(0, 0, -29)
	snap.SellerAccountID = SellerAccountID
	snap.MetricDate = pgDate(metricDate)
	snap.Windows = baseWindows()
	snap.LoadedHistory = baseLoadedHistory(from, asOf)
	snap.EnrichmentSource = "eval_scenario"
	snap.EnrichmentRun = enrichmentRun()
	snap.Categories = []dbgen.DescriptionCategory{{
		DescriptionCategoryID: 10,
		CategoryName:          "Scenario Category",
	}}
	snap.PriceConstraints = []dbgen.SkuEffectiveConstraint{}
	snap.APICapabilities = []dbgen.OzonApiCapability{}
	return snap
}

func dateRange30(t SnapshotHarness) (from, asOf time.Time) {
	t.Helper()
	asOf = analysisdata.DateOnlyUTC(metricDate)
	from = asOf.AddDate(0, 0, -29)
	return from, asOf
}
