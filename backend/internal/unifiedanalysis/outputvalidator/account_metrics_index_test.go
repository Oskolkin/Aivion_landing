package outputvalidator

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

func TestAccountSummaryCSVIndexesRevenueDespiteRecordReuse(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("account_summary.csv")
	if err != nil {
		t.Fatal(err)
	}
	csv := "row_id,seller_account_id,metric_date,orders_7d,orders_previous_7d,orders_delta_abs,orders_delta_pct,revenue_7d,revenue_previous_7d,revenue_delta_abs,revenue_delta_pct,delivered_7d,cancelled_7d,returns_7d,average_order_value_7d,impressions_7d,product_views_7d,add_to_cart_7d,funnel_orders_7d,impression_to_view_pct,view_to_cart_pct,cart_to_order_pct,order_to_delivered_pct,active_sku_count,stockout_sku_count,deficit_sku_count,surplus_sku_count,no_sales_with_stock_count,ad_spend_7d,ad_revenue_7d,ad_orders_7d,account_drr_pct\n" +
		"account:1:2026-07-08,1,2026-07-08,672,715,-43,-6.01,975389.79,1057742.52,-82352.73,-7.79,594,44,34,1451.47,24156,3481,892,336,14.41,25.62,37.67,90.18,80,16,16,16,42,163800.00,178500.00,105,91.76\n"
	if _, err := w.Write([]byte(csv)); err != nil {
		t.Fatal(err)
	}
	// Minimal companion files required by BuildPackReferenceIndex.
	for name, body := range map[string]string{
		"manifest.json":        `{"pack_id":"pack:1:2026-07-08:test"}`,
		"sku_summary.csv":      "sku_id\n801\n",
		"campaign_summary.csv": "campaign_id\nc1\n",
		"limitations.json":     `{"items":[]}`,
	} {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	idx, err := BuildPackReferenceIndex(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	f, ok := idx.LookupAccountMetric("account:revenue_7d:current_7d", "revenue_7d", "current_7d")
	if !ok {
		t.Fatalf("expected revenue_7d indexed, accountMetrics=%d", len(idx.accountMetrics))
	}
	if f.Value < 975000 || f.Value > 976000 {
		t.Fatalf("unexpected revenue value %.4f", f.Value)
	}
	drr, ok := idx.LookupAccountMetric("account:drr:current_7d", "drr", "current_7d")
	if !ok {
		t.Fatal("expected store-level drr indexed")
	}
	if drr.Value < 16 || drr.Value > 18 {
		t.Fatalf("unexpected store drr %.4f (want ~16.8)", drr.Value)
	}

	// Canonical ad_efficiency.narrative.metric_claims codes (Wave 2) must be
	// derived from the same account_summary.csv row.
	adSpend, ok := idx.LookupAccountMetric("", AdMetricAdSpend, "current_7d")
	if !ok || adSpend.Value != 163800 {
		t.Fatalf("expected ad_spend=163800, got ok=%v value=%.2f", ok, adSpend.Value)
	}
	storeRevenue, ok := idx.LookupAccountMetric("", AdMetricStoreTotalRevenue, "current_7d")
	if !ok || storeRevenue.Value < 975000 || storeRevenue.Value > 976000 {
		t.Fatalf("expected store_total_revenue~975389.79, got ok=%v value=%.2f", ok, storeRevenue.Value)
	}
	attrRevenue, ok := idx.LookupAccountMetric("", AdMetricAttributedAdRevenue, "current_7d")
	if !ok || attrRevenue.Value != 178500 {
		t.Fatalf("expected attributed_ad_revenue=178500, got ok=%v value=%.2f", ok, attrRevenue.Value)
	}
	adOrders, ok := idx.LookupAccountMetric("", AdMetricAdOrders, "current_7d")
	if !ok || adOrders.Value != 105 {
		t.Fatalf("expected ad_orders=105, got ok=%v value=%.2f", ok, adOrders.Value)
	}
	storeSharePct, ok := idx.LookupAccountMetric("", AdMetricAdSpendShareOfTotalStoreRevenuePct, "current_7d")
	if !ok || storeSharePct.Value < 16 || storeSharePct.Value > 18 {
		t.Fatalf("expected ad_spend_share_of_total_store_revenue_pct~16.8, got ok=%v value=%.2f", ok, storeSharePct.Value)
	}
	attrDRRPct, ok := idx.LookupAccountMetric("", AdMetricAttributedCampaignDRRPct, "current_7d")
	if !ok || attrDRRPct.Value < 91 || attrDRRPct.Value > 93 {
		t.Fatalf("expected attributed_campaign_drr_pct~91.8, got ok=%v value=%.2f", ok, attrDRRPct.Value)
	}
	_ = os.DevNull
}
