package pack

import (
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("numeric %q: %v", s, err)
	}
	return n
}

// minimalSnapshot returns a snapshot with only identity fields set (no rows at all).
func minimalSnapshot(sellerAccountID int64, metricDate time.Time) *analysisdata.AnalysisDataSnapshot {
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgtype.Date{Time: analysisdata.DateOnlyUTC(metricDate), Valid: true},
		Windows:         analysisdata.ComputeDataWindows(metricDate),
		LoadedHistory: analysisdata.LoadedHistory{
			From: analysisdata.FormatDate(analysisdata.DateOnlyUTC(metricDate).AddDate(0, 0, -29)),
			To:   analysisdata.FormatDate(analysisdata.DateOnlyUTC(metricDate)),
			Days: 30,
		},
	}
}

// populatedSnapshot builds a fixture snapshot with one SKU, one campaign, and 30 days
// of account/sku/ad/funnel history ending at metricDate, for builder/hash tests.
func populatedSnapshot(t *testing.T, metricDate time.Time) *analysisdata.AnalysisDataSnapshot {
	t.Helper()
	const sellerID = int64(42)
	sku := int64(801000100001)
	productID := int64(901000100001)
	campaignID := int64(882000001625)

	asOf := analysisdata.DateOnlyUTC(metricDate)
	from := asOf.AddDate(0, 0, -29)

	var accountDays []dbgen.DailyAccountMetric
	var skuDays []dbgen.DailySkuMetric
	var adDays []dbgen.AdMetricsDaily
	var funnelDays []dbgen.ProductAnalyticsDaily

	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		pgD := pgtype.Date{Time: d, Valid: true}
		orders := int32(5 + i%7)
		accountDays = append(accountDays, dbgen.DailyAccountMetric{
			SellerAccountID: sellerID,
			MetricDate:      pgD,
			Revenue:         mustNumeric(t, fmt.Sprintf("%d.00", 1000+i*10)),
			OrdersCount:     orders,
			ReturnsCount:    1,
			CancelCount:     1,
			UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
		})
		skuDays = append(skuDays, dbgen.DailySkuMetric{
			SellerAccountID: sellerID,
			MetricDate:      pgD,
			OzonProductID:   productID,
			OfferID:         pgtype.Text{String: "offer-1", Valid: true},
			Sku:             pgtype.Int8{Int64: sku, Valid: true},
			ProductName:     pgtype.Text{String: "SKU One", Valid: true},
			Revenue:         mustNumeric(t, fmt.Sprintf("%d.00", 800+i*5)),
			OrdersCount:     orders,
			StockAvailable:  50,
			UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
		})
		adDays = append(adDays, dbgen.AdMetricsDaily{
			SellerAccountID:    sellerID,
			CampaignExternalID: campaignID,
			MetricDate:         pgD,
			Impressions:        1000 + int64(i),
			Clicks:             50 + int64(i),
			Spend:              mustNumeric(t, "100.00"),
			OrdersCount:        2,
			Revenue:            mustNumeric(t, "400.00"),
			CampaignName:       pgtype.Text{String: "Campaign One", Valid: true},
		})
		funnelDays = append(funnelDays, dbgen.ProductAnalyticsDaily{
			SellerAccountID: sellerID,
			MetricDate:      pgD,
			ScopeType:       "account",
			ScopeID:         "",
			Impressions:     pgtype.Int8{Int64: 5000, Valid: true},
			ProductViews:    pgtype.Int8{Int64: 1000, Valid: true},
			AddToCart:       pgtype.Int8{Int64: 100, Valid: true},
			OrdersCount:     pgtype.Int8{Int64: int64(orders), Valid: true},
			CompletedOrders: pgtype.Int8{Int64: int64(orders - 1), Valid: true},
			SourceStatus:    pgtype.Text{String: "ok", Valid: true},
		})
	}

	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID:  sellerID,
		MetricDate:       pgtype.Date{Time: asOf, Valid: true},
		Windows:          analysisdata.ComputeDataWindows(metricDate),
		EnrichmentSource: "dev_seed",
		LoadedHistory: analysisdata.LoadedHistory{
			From: analysisdata.FormatDate(from),
			To:   analysisdata.FormatDate(asOf),
			Days: 30,
		},
		AccountMetrics: accountDays,
		SKUMetrics:     skuDays,
		Products: []dbgen.Product{{
			ID:              1,
			SellerAccountID: sellerID,
			OzonProductID:   productID,
			OfferID:         pgtype.Text{String: "offer-1", Valid: true},
			Sku:             pgtype.Int8{Int64: sku, Valid: true},
			Name:            "SKU Один / тестовый",
			Status:          pgtype.Text{String: "active", Valid: true},
			ReferencePrice:  mustNumeric(t, "999.00"),
		}},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{{
			SellerAccountID:     sellerID,
			SnapshotDate:        pgtype.Date{Time: asOf, Valid: true},
			Sku:                 sku,
			OfferID:             pgtype.Text{String: "offer-1", Valid: true},
			ProductName:         pgtype.Text{String: "SKU Один / тестовый", Valid: true},
			WarehouseID:         1,
			WarehouseName:       pgtype.Text{String: "WH-1", Valid: true},
			AvailableStockCount: 50,
			Ads:                 mustNumeric(t, "5.00"),
			Idc:                 pgtype.Int4{Int32: 10, Valid: true},
			TurnoverGrade:       pgtype.Text{String: "ACTUAL", Valid: true},
			DaysWithoutSales:    pgtype.Int4{Int32: 0, Valid: true},
		}},
		Campaigns: []dbgen.AdCampaign{{
			SellerAccountID:    sellerID,
			CampaignExternalID: campaignID,
			CampaignName:       "Campaign One",
			CampaignType:       pgtype.Text{String: "SKU", Valid: true},
			Status:             pgtype.Text{String: "CAMPAIGN_STATE_RUNNING", Valid: true},
		}},
		AdMetrics: adDays,
		CampaignSKUs: []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{{
			CampaignExternalID: campaignID,
			Sku:                pgtype.Int8{Int64: sku, Valid: true},
		}},
		FunnelRows: funnelDays,
		Categories: []dbgen.DescriptionCategory{{
			DescriptionCategoryID: 10,
			CategoryName:          "Category A",
		}},
		PriceConstraints: []dbgen.SkuEffectiveConstraint{},
		EnrichmentRun: &dbgen.OzonEnrichmentSyncRun{
			Scope:  "p0_dev_seed",
			Status: "succeeded",
		},
		APICapabilities: []dbgen.OzonApiCapability{},
	}
}

func reverseAccountMetrics(in []dbgen.DailyAccountMetric) []dbgen.DailyAccountMetric {
	out := append([]dbgen.DailyAccountMetric(nil), in...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func reverseSKUMetrics(in []dbgen.DailySkuMetric) []dbgen.DailySkuMetric {
	out := append([]dbgen.DailySkuMetric(nil), in...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func reverseAdMetrics(in []dbgen.AdMetricsDaily) []dbgen.AdMetricsDaily {
	out := append([]dbgen.AdMetricsDaily(nil), in...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
