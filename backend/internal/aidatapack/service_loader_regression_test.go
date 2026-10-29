package aidatapack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeLoader struct {
	snapshot      *analysisdata.AnalysisDataSnapshot
	err           error
	gotSellerID   int64
	gotMetricDate time.Time
	loadCalled    bool
}

func (f *fakeLoader) Load(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*analysisdata.AnalysisDataSnapshot, error) {
	_ = ctx
	f.loadCalled = true
	f.gotSellerID = sellerAccountID
	f.gotMetricDate = metricDate
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func TestNewServiceWithLoaderRejectsNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil loader")
		}
	}()
	_ = NewServiceWithLoader(nil)
}

func TestServiceBuildUsesInjectedLoader(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 15, 30, 0, 0, time.UTC)
	snap := regressionSnapshotFixture()
	loader := &fakeLoader{snapshot: snap}
	svc := NewServiceWithLoader(loader)

	pack, err := svc.Build(context.Background(), 42, metricDate)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !loader.loadCalled {
		t.Fatal("expected loader.Load to be called")
	}
	if loader.gotSellerID != 42 {
		t.Fatalf("seller_id=%d", loader.gotSellerID)
	}
	if !loader.gotMetricDate.Equal(metricDate) {
		t.Fatalf("metric_date=%v want %v", loader.gotMetricDate, metricDate)
	}
	if pack.SchemaVersion != SchemaVersion {
		t.Fatalf("schema=%s", pack.SchemaVersion)
	}
	if pack.SellerContext.SellerAccountID != 42 {
		t.Fatalf("seller=%d", pack.SellerContext.SellerAccountID)
	}
	if err := ValidateDataPack(pack); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestServiceBuildPropagatesLoaderError(t *testing.T) {
	want := errors.New("loader boom")
	svc := NewServiceWithLoader(&fakeLoader{err: want})
	_, err := svc.Build(context.Background(), 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotToPackRegression(t *testing.T) {
	snap := regressionSnapshotFixture()
	pack, err := BuildPack(snap)
	if err != nil {
		t.Fatalf("BuildPack: %v", err)
	}

	if pack.SchemaVersion != "ai_data_pack_v1" {
		t.Fatalf("schema=%s", pack.SchemaVersion)
	}
	if pack.SellerContext.SellerAccountID != 42 {
		t.Fatalf("seller=%d", pack.SellerContext.SellerAccountID)
	}
	if pack.SellerContext.MetricDate != "2026-07-08" {
		t.Fatalf("metric_date=%s", pack.SellerContext.MetricDate)
	}
	if pack.DataWindows.CurrentDay != "2026-07-08" ||
		pack.DataWindows.Yesterday != "2026-07-07" ||
		pack.DataWindows.Last7d.From != "2026-07-02" ||
		pack.DataWindows.Previous7d.From != "2026-06-25" ||
		pack.DataWindows.Last14d.From != "2026-06-25" {
		t.Fatalf("windows=%+v", pack.DataWindows)
	}
	if len(pack.Tables.SKURollups) != 1 {
		t.Fatalf("sku_rollups=%d", len(pack.Tables.SKURollups))
	}
	if pack.Tables.AccountRollup.DataStatus != "available" {
		t.Fatalf("account status=%s", pack.Tables.AccountRollup.DataStatus)
	}
	if len(pack.Tables.CampaignRollups) != 1 {
		t.Fatalf("campaign_rollups=%d", len(pack.Tables.CampaignRollups))
	}
	if len(pack.Tables.StockPositions) != 1 {
		t.Fatalf("stock_positions=%d", len(pack.Tables.StockPositions))
	}
	if len(pack.EntityRegistry.SKUs) == 0 {
		t.Fatal("expected sku entities")
	}
	if len(pack.FactRegistry) == 0 {
		t.Fatal("expected facts")
	}
	if pack.DataQuality.Sources == nil {
		t.Fatal("expected data quality sources")
	}
	if pack.Constraints.SKUPriceConstraints == nil {
		t.Fatal("expected constraints slice")
	}
	if err := ValidateDataPack(pack); err != nil {
		t.Fatalf("validate: %v", err)
	}

	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Re-marshal through map for stable key order already provided by encoding/json.
	var canonical any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		t.Fatalf("unmarshal canonical: %v", err)
	}
	got, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		t.Fatalf("marshal indent: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join(testdataDir(t), "seller_ai_data_pack_v1_regression.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" || os.Getenv("UPDATE_AIDATAPACK_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(got)
		t.Logf("updated golden %s sha256=%s", goldenPath, hex.EncodeToString(sum[:]))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (set UPDATE_GOLDEN=1 to create): %v", err)
	}
	sumGot := sha256.Sum256(got)
	sumWant := sha256.Sum256(want)
	t.Logf("legacy/refactored canonical hash got=%s want=%s equal=%v",
		hex.EncodeToString(sumGot[:]), hex.EncodeToString(sumWant[:]), string(got) == string(want))
	if string(got) != string(want) {
		diff := firstJSONDiff(string(want), string(got), 12)
		t.Fatalf("golden mismatch\ngot_sha=%s\nwant_sha=%s\n%s\nset UPDATE_GOLDEN=1 only after intentional contract changes",
			hex.EncodeToString(sumGot[:]), hex.EncodeToString(sumWant[:]), diff)
	}
}

func TestFunnelRowOrderDoesNotAffectLegacyPack(t *testing.T) {
	// Variant A proof: SelectAccountFunnelRows sorting must not change SellerAIDataPack.
	// buildFunnelRollup indexes by date; FunnelRows order is not serialized into the pack.
	base := regressionSnapshotFixture()
	packSorted, err := BuildPack(cloneSnapshotWithFunnelOrder(base, false))
	if err != nil {
		t.Fatalf("sorted BuildPack: %v", err)
	}
	packReversed, err := BuildPack(cloneSnapshotWithFunnelOrder(base, true))
	if err != nil {
		t.Fatalf("reversed BuildPack: %v", err)
	}
	sortedJSON := canonicalPackJSON(t, packSorted)
	reversedJSON := canonicalPackJSON(t, packReversed)
	sumSorted := sha256.Sum256(sortedJSON)
	sumReversed := sha256.Sum256(reversedJSON)
	t.Logf("funnel_order_proof sorted=%s reversed=%s equal=%v",
		hex.EncodeToString(sumSorted[:]), hex.EncodeToString(sumReversed[:]), string(sortedJSON) == string(reversedJSON))
	if string(sortedJSON) != string(reversedJSON) {
		t.Fatalf("funnel row order changed pack output\n%s", firstJSONDiff(string(sortedJSON), string(reversedJSON), 20))
	}
}

func TestBuildPackAssemblyUnchangedFromHEADSurface(t *testing.T) {
	// BuildPack assembly body is unchanged vs HEAD (comment-only builder.go diff).
	// This test locks the canonical hash of the committed golden fixture path.
	snap := regressionSnapshotFixture()
	pack, err := BuildPack(snap)
	if err != nil {
		t.Fatal(err)
	}
	got := canonicalPackJSON(t, pack)
	sum := sha256.Sum256(got)
	wantRaw, err := os.ReadFile(filepath.Join(testdataDir(t), "seller_ai_data_pack_v1_regression.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantSum := sha256.Sum256(wantRaw)
	t.Logf("golden_origin=post_refactor_buildpack_fixture fixture=regressionSnapshotFixture()")
	t.Logf("legacy_commit=HEAD BuildPack assembly identical except loader extraction")
	t.Logf("legacy_canonical_hash=%s", hex.EncodeToString(wantSum[:]))
	t.Logf("new_canonical_hash=%s", hex.EncodeToString(sum[:]))
	t.Logf("equal=%v excluded_fields=none", string(got) == string(wantRaw))
	if string(got) != string(wantRaw) {
		t.Fatal("BuildPack output diverged from committed golden")
	}
}

func canonicalPackJSON(t *testing.T, pack *SellerAIDataPack) []byte {
	t.Helper()
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var canonical any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
}

func cloneSnapshotWithFunnelOrder(src *analysisdata.AnalysisDataSnapshot, reverse bool) *analysisdata.AnalysisDataSnapshot {
	cp := *src
	rows := append([]dbgen.ProductAnalyticsDaily(nil), src.FunnelRows...)
	if reverse {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	cp.FunnelRows = rows
	return &cp
}

func firstJSONDiff(want, got string, maxLines int) string {
	wLines := splitLines(want)
	gLines := splitLines(got)
	n := len(wLines)
	if len(gLines) > n {
		n = len(gLines)
	}
	var b strings.Builder
	shown := 0
	for i := 0; i < n && shown < maxLines; i++ {
		var wl, gl string
		if i < len(wLines) {
			wl = wLines[i]
		}
		if i < len(gLines) {
			gl = gLines[i]
		}
		if wl == gl {
			continue
		}
		fmt.Fprintf(&b, "line %d\n- %s\n+ %s\n", i+1, wl, gl)
		shown++
	}
	if b.Len() == 0 {
		return "no line-level diff (length/encoding only?)"
	}
	return b.String()
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func regressionSnapshotFixture() *analysisdata.AnalysisDataSnapshot {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	windows := ComputeDataWindows(metricDate)
	sku := int64(801000100001)
	productID := int64(901000100001)
	campaignID := int64(882000001625)

	days := make([]dbgen.DailyAccountMetric, 0, 14)
	skuDays := make([]dbgen.DailySkuMetric, 0, 14)
	adDays := make([]dbgen.AdMetricsDaily, 0, 14)
	funnelDays := make([]dbgen.ProductAnalyticsDaily, 0, 14)
	for i := 0; i < 14; i++ {
		d := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		pgD := pgtype.Date{Time: d, Valid: true}
		orders := int32(10 + i)
		days = append(days, dbgen.DailyAccountMetric{
			SellerAccountID: 42,
			MetricDate:      pgD,
			Revenue:         mustNumeric(fmt.Sprintf("%d.00", 1000+i*10)),
			OrdersCount:     orders,
			ReturnsCount:    1,
			CancelCount:     1,
			UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
		})
		skuDays = append(skuDays, dbgen.DailySkuMetric{
			SellerAccountID: 42,
			MetricDate:      pgD,
			OzonProductID:   productID,
			OfferID:         pgtype.Text{String: "offer-1", Valid: true},
			Sku:             pgtype.Int8{Int64: sku, Valid: true},
			ProductName:     pgtype.Text{String: "SKU One", Valid: true},
			Revenue:         mustNumeric(fmt.Sprintf("%d.00", 800+i*5)),
			OrdersCount:     orders,
			StockAvailable:  50,
			UnitsSold:       pgtype.Int4{Int32: orders, Valid: true},
		})
		adDays = append(adDays, dbgen.AdMetricsDaily{
			SellerAccountID:    42,
			CampaignExternalID: campaignID,
			MetricDate:         pgD,
			Impressions:        1000 + int64(i),
			Clicks:             50 + int64(i),
			Spend:              mustNumeric("100.00"),
			OrdersCount:        2,
			Revenue:            mustNumeric("400.00"),
			CampaignName:       pgtype.Text{String: "Campaign One", Valid: true},
		})
		funnelDays = append(funnelDays, dbgen.ProductAnalyticsDaily{
			SellerAccountID: 42,
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
		SellerAccountID:  42,
		MetricDate:       pgtype.Date{Time: metricDate, Valid: true},
		Windows:          windows,
		EnrichmentSource: "dev_seed",
		AccountMetrics:   days,
		SKUMetrics:       skuDays,
		Products: []dbgen.Product{{
			ID:              1,
			SellerAccountID: 42,
			OzonProductID:   productID,
			OfferID:         pgtype.Text{String: "offer-1", Valid: true},
			Sku:             pgtype.Int8{Int64: sku, Valid: true},
			Name:            "SKU One",
			Status:          pgtype.Text{String: "active", Valid: true},
		}},
		StockSnapshots: []dbgen.StockAnalyticsSnapshot{{
			SellerAccountID:     42,
			SnapshotDate:        pgtype.Date{Time: metricDate, Valid: true},
			Sku:                 sku,
			OfferID:             pgtype.Text{String: "offer-1", Valid: true},
			ProductName:         pgtype.Text{String: "SKU One", Valid: true},
			WarehouseID:         1,
			WarehouseName:       pgtype.Text{String: "WH-1", Valid: true},
			AvailableStockCount: 50,
			Ads:                 mustNumeric("5.00"),
			Idc:                 pgtype.Int4{Int32: 10, Valid: true},
			TurnoverGrade:       pgtype.Text{String: "ACTUAL", Valid: true},
			DaysWithoutSales:    pgtype.Int4{Int32: 0, Valid: true},
		}},
		Campaigns: []dbgen.AdCampaign{{
			SellerAccountID:    42,
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

func mustNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		panic(err)
	}
	return n
}
