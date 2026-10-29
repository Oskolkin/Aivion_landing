package aidatapack

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestComputeDataWindows(t *testing.T) {
	metricDate := time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
	w := ComputeDataWindows(metricDate)

	if w.CurrentDay != "2026-06-25" {
		t.Fatalf("current_day=%s", w.CurrentDay)
	}
	if w.Yesterday != "2026-06-24" {
		t.Fatalf("yesterday=%s", w.Yesterday)
	}
	if w.Last7d.From != "2026-06-19" || w.Last7d.To != "2026-06-25" {
		t.Fatalf("last_7d=%+v", w.Last7d)
	}
	if w.Previous7d.From != "2026-06-12" || w.Previous7d.To != "2026-06-18" {
		t.Fatalf("previous_7d=%+v", w.Previous7d)
	}
	if w.Last14d.From != "2026-06-12" || w.Last14d.To != "2026-06-25" {
		t.Fatalf("last_14d=%+v", w.Last14d)
	}

	// previous_7d must not overlap last_7d
	if !parseDate(w.Previous7d.To).Before(parseDate(w.Last7d.From)) {
		t.Fatal("previous_7d overlaps last_7d")
	}
}

func TestEntityIDsDeterministic(t *testing.T) {
	cases := map[string]string{
		skuEntityID("801000100001"):      "sku:801000100001",
		campaignEntityID("882000001625"): "campaign:882000001625",
		skuGroupEntityID("901000100001"): "sku_group:901000100001",
		warehouseEntityID("42"):          "warehouse:42",
		categoryEntityID("123"):          "category:123",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestNormalizeFactID(t *testing.T) {
	id, err := normalizeFactID("Campaign.882.Last_7d.Spend")
	if err != nil || id != "campaign.882.last_7d.spend" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if _, err := normalizeFactID("bad:id"); err == nil {
		t.Fatal("expected error for colon")
	}
	if _, err := normalizeFactID("плохой"); err == nil {
		t.Fatal("expected error for cyrillic")
	}
}

func TestAddFactRejectsConflictingDuplicate(t *testing.T) {
	b := NewBuilder(ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)))
	src := FactSource{Table: "t", Field: "f"}
	_, err := b.AddFact(Fact{FactID: "sku.1.last_7d.revenue", EntityType: "sku", EntityID: "sku:1", Metric: "revenue", Value: 10.0, Unit: UnitRUB, Source: src})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.AddFact(Fact{FactID: "sku.1.last_7d.revenue", EntityType: "sku", EntityID: "sku:1", Metric: "revenue", Value: 20.0, Unit: UnitRUB, Source: src})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		unit string
		val  any
		want string
	}{
		{UnitRUB, 12345.0, "12 345 ₽"},
		{UnitRUB, 12345.67, "12 345,67 ₽"},
		{UnitPercent, 12.34, "12,34%"},
		{UnitCount, int64(123), "123"},
		{UnitDays, 12.3, "12,3 дн."},
		{UnitRatio, 1.23, "1,23"},
		{UnitBoolean, true, "да"},
		{UnitBoolean, false, "нет"},
		{UnitEnum, "GREEN", "GREEN"},
	}
	for _, tc := range tests {
		got := FormatValue(tc.unit, tc.val)
		if got != tc.want {
			t.Fatalf("unit=%s val=%v got=%q want=%q", tc.unit, tc.val, got, tc.want)
		}
	}
}

func TestAdMetricCalculationsNoNaN(t *testing.T) {
	agg := aggregateAdMetrics([]adMetricDay{{Spend: 100, Orders: 0, Revenue: 0, Impressions: 1000, Clicks: 10}})
	if agg.DRRPct != nil {
		t.Fatalf("drr should be nil when revenue=0, got %v", *agg.DRRPct)
	}
	if !agg.SpendWithoutOrders {
		t.Fatal("expected spend_without_orders")
	}
	if !agg.SpendWithoutRevenue {
		t.Fatal("expected spend_without_revenue")
	}
	if agg.ROAS != nil && *agg.ROAS != 0 {
		t.Fatalf("roas should be 0 or nil when revenue=0, got %v", *agg.ROAS)
	}
	w := adAggToWindow(agg)
	if w.DRRPct != nil {
		t.Fatal("window drr nil")
	}
	if math.IsNaN(w.Spend) {
		t.Fatal("nan spend")
	}
}

func TestCampaignDRRWhenRevenuePositive(t *testing.T) {
	agg := aggregateAdMetrics([]adMetricDay{{Spend: 100, Revenue: 200, Orders: 2}})
	if agg.DRRPct == nil || *agg.DRRPct != 50 {
		t.Fatalf("drr=%v", agg.DRRPct)
	}
}

func TestAutoCampaignBidApplicability(t *testing.T) {
	ctx := &loadContext{
		Campaigns: []dbgen.AdCampaign{{
			CampaignExternalID:       1,
			ProductAutopilotStrategy: pgtype.Text{String: "AUTO_STRATEGY", Valid: true},
		}},
	}
	link := dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{CampaignExternalID: 1}
	bid := buildCampaignBid(link, ctx)
	if bid == nil || bid.Applicability != "not_applicable_auto_strategy" {
		t.Fatalf("bid=%+v", bid)
	}
}

func TestPickAccountFunnelRowsPrefersEmptyScopeID(t *testing.T) {
	rows := []dbgen.ProductAnalyticsDaily{
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "account", Impressions: pgInt8v(1)},
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "", Impressions: pgInt8v(99)},
	}
	picked := pickAccountFunnelRows(rows)
	if len(picked) != 1 {
		t.Fatalf("len=%d", len(picked))
	}
	if picked[0].ScopeID != "" || picked[0].Impressions.Int64 != 99 {
		t.Fatalf("picked wrong row: scope_id=%q impressions=%d", picked[0].ScopeID, picked[0].Impressions.Int64)
	}
}

func TestDetectEnrichmentSourceDevSeed(t *testing.T) {
	run := &dbgen.OzonEnrichmentSyncRun{Scope: "p0_dev_seed"}
	if got := detectEnrichmentSource(run); got != "dev_seed" {
		t.Fatalf("source=%q", got)
	}
}

func TestFinanceRollupNotAvailable(t *testing.T) {
	f := buildFinanceRollup()
	if f.DataStatus != "not_available" {
		t.Fatalf("status=%s", f.DataStatus)
	}
}

func TestValidateDataPackMissingFact(t *testing.T) {
	pack := &SellerAIDataPack{
		SchemaVersion: SchemaVersion,
		SellerContext: SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)),
		DataQuality:   DataQuality{Sources: map[string]any{}},
		EntityRegistry: EntityRegistry{
			SKUs: map[string]SKUEntity{}, SKUGroups: map[string]SKUGroupEntity{},
			Campaigns: map[string]CampaignEntity{}, Warehouses: map[string]WarehouseEntity{},
			Categories: map[string]CategoryEntity{},
		},
		FactRegistry: map[string]Fact{
			"existing.fact": {
				FactID: "existing.fact", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: UnitRUB,
				Source: FactSource{Table: "t", Field: "f"},
			},
		},
		Tables: Tables{
			AccountRollup: AccountRollupTable{FactIDs: []string{"missing.fact"}},
		},
		Constraints: ConstraintsBlock{SKUPriceConstraints: []any{}},
	}
	if err := ValidateDataPack(pack); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateDataPackEmptyFactRegistry(t *testing.T) {
	pack := minimalValidPack()
	pack.FactRegistry = map[string]Fact{}
	if err := ValidateDataPack(pack); err == nil {
		t.Fatal("expected empty fact_registry error")
	}
}

func TestFactRegistryJSONIsObject(t *testing.T) {
	pack := minimalValidPack()
	pack.FactRegistry["account.test.revenue"] = Fact{
		FactID: "account.test.revenue", EntityType: "account", EntityID: "account:seller",
		Metric: "revenue", Value: 100.0, Unit: UnitRUB,
		Source: FactSource{Table: "daily_account_metrics", Field: "revenue"},
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertFactRegistryJSONShape(raw); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	fr, ok := root["fact_registry"].(map[string]any)
	if !ok {
		t.Fatalf("fact_registry type=%T", root["fact_registry"])
	}
	if len(fr) == 0 {
		t.Fatal("expected facts")
	}
}

func TestCampaignMetricsJSONPreservesZero(t *testing.T) {
	w := adAggToWindow(adMetricAgg{Spend: 12600, Orders: 0, Revenue: 0, SpendWithoutOrders: true, SpendWithoutRevenue: true})
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"spend", "orders", "revenue", "impressions", "clicks"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in %s", key, string(raw))
		}
	}
	if m["orders"].(float64) != 0 || m["revenue"].(float64) != 0 {
		t.Fatalf("orders/revenue should be 0: %v", m)
	}
	if m["drr_pct"] != nil {
		t.Fatalf("drr_pct should be omitted: %v", m["drr_pct"])
	}
}

func TestStockAggregationMetadata(t *testing.T) {
	lites := []stockSnapshotLite{
		{Sku: 1, TurnoverGrade: "ACTUAL", AvailableStockCount: 1},
		{Sku: 1, TurnoverGrade: "DEFICIT", AvailableStockCount: 2},
	}
	agg := aggregateStockFromSnapshots(lites, "1")
	if agg.SourceRowsCount != 2 {
		t.Fatalf("rows=%d", agg.SourceRowsCount)
	}
	if agg.TurnoverGradeAggregationMethod != "most_critical_across_rows" {
		t.Fatalf("method=%s", agg.TurnoverGradeAggregationMethod)
	}
	if agg.TurnoverGrade != "DEFICIT" {
		t.Fatalf("grade=%s", agg.TurnoverGrade)
	}
}

func TestValidateForbiddenBusinessKey(t *testing.T) {
	pack := minimalValidPack()
	pack.Tables.AccountRollup.CurrentDay = map[string]any{"pause_campaign": true}
	if err := ValidateDataPack(pack); err == nil {
		t.Fatal("expected forbidden business key error")
	}
}

func minimalValidPack() *SellerAIDataPack {
	return &SellerAIDataPack{
		SchemaVersion: SchemaVersion,
		SellerContext: SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)),
		DataQuality:   DataQuality{Sources: map[string]any{}, Limitations: []string{}},
		EntityRegistry: EntityRegistry{
			SKUs: map[string]SKUEntity{}, SKUGroups: map[string]SKUGroupEntity{},
			Campaigns: map[string]CampaignEntity{}, Warehouses: map[string]WarehouseEntity{},
			Categories: map[string]CategoryEntity{},
		},
		FactRegistry: map[string]Fact{
			"account.current_day.revenue": {
				FactID: "account.current_day.revenue", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: UnitRUB,
				Source: FactSource{Table: "daily_account_metrics", Field: "revenue"},
			},
		},
		Tables: Tables{
			AccountRollup: AccountRollupTable{DataStatus: "available", CurrentDay: map[string]any{"revenue": 1}},
			FinanceRollup: FinanceRollupTable{DataStatus: "not_available"},
		},
		Constraints: ConstraintsBlock{SKUPriceConstraints: []any{}},
	}
}

func TestValidateDataPackMissingEntity(t *testing.T) {
	pack := &SellerAIDataPack{
		SchemaVersion: SchemaVersion,
		SellerContext: SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)),
		DataQuality:   DataQuality{Sources: map[string]any{}},
		EntityRegistry: EntityRegistry{
			SKUs: map[string]SKUEntity{}, SKUGroups: map[string]SKUGroupEntity{},
			Campaigns: map[string]CampaignEntity{}, Warehouses: map[string]WarehouseEntity{},
			Categories: map[string]CategoryEntity{},
		},
		FactRegistry: map[string]Fact{
			"dummy": {
				FactID: "dummy", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: UnitRUB,
				Source: FactSource{Table: "t", Field: "f"},
			},
		},
		Tables: Tables{
			SKURollups: []SKURollupRow{{EntityID: "sku:missing"}},
		},
		Constraints: ConstraintsBlock{SKUPriceConstraints: []any{}},
	}
	if err := ValidateDataPack(pack); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAuditCampaignScenarioCountsSeparateStateAndActivity(t *testing.T) {
	pack := &SellerAIDataPack{
		Tables: Tables{
			CampaignRollups: []CampaignRollupRow{
				{
					State: "CAMPAIGN_STATE_STOPPED",
					Last7d: CampaignMetricsWindow{
						Impressions: 50, Spend: 0, Orders: 0, Revenue: 0,
					},
				},
				{
					State: "CAMPAIGN_STATE_RUNNING",
					Last7d: CampaignMetricsWindow{
						Impressions: 0, Spend: 0, Orders: 0, Revenue: 0,
					},
				},
				{
					State: "CAMPAIGN_STATE_INACTIVE",
					Last7d: CampaignMetricsWindow{
						Impressions: 100, Spend: 10, Orders: 0, Revenue: 0,
						SpendWithoutOrders: true, SpendWithoutRevenue: true,
					},
				},
			},
		},
	}
	report := AuditDataPack(pack)
	if report.CampaignScenarios.InactiveByStateCount != 2 {
		t.Fatalf("inactive_by_state=%d want 2", report.CampaignScenarios.InactiveByStateCount)
	}
	if report.CampaignScenarios.NoActivityLast7dCount != 1 {
		t.Fatalf("no_activity_last_7d=%d want 1", report.CampaignScenarios.NoActivityLast7dCount)
	}
}

func TestIsInactiveCampaignState(t *testing.T) {
	for _, state := range []string{"CAMPAIGN_STATE_STOPPED", "CAMPAIGN_STATE_INACTIVE", "CAMPAIGN_STATE_PAUSED"} {
		if !isInactiveCampaignState(state) {
			t.Fatalf("%s should be inactive", state)
		}
	}
	if isInactiveCampaignState("CAMPAIGN_STATE_RUNNING") {
		t.Fatal("running should not be inactive")
	}
}

func pgDate(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

func pgInt8v(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func TestBuildDataQuality_EnrichmentFailedStructuredLimitation(t *testing.T) {
	errorsJSON := []byte(`[{"source":"seller_api","message":"credentials missing"}]`)
	ctx := &loadContext{
		Windows: ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)),
		EnrichmentRun: &dbgen.OzonEnrichmentSyncRun{
			Status:                  "failed",
			ErrorsJson:              errorsJSON,
			PerformanceApiAvailable: false,
		},
	}
	dq := buildDataQuality(ctx, 1, 1, 0)
	if len(dq.StructuredLimitations) != 1 {
		t.Fatalf("structured_limitations=%d want 1", len(dq.StructuredLimitations))
	}
	lim := dq.StructuredLimitations[0]
	if lim.Code != limitationCodeOzonEnrichmentUnavailable {
		t.Fatalf("code=%q", lim.Code)
	}
	if lim.Severity != limitationSeverityHigh {
		t.Fatalf("severity=%q", lim.Severity)
	}
	wantScope := []string{"advertising", "ozon_api_capabilities", "product_enrichment", "stock_analytics"}
	if len(lim.Scope) != len(wantScope) {
		t.Fatalf("scope=%v want %v", lim.Scope, wantScope)
	}
	for i, item := range wantScope {
		if lim.Scope[i] != item {
			t.Fatalf("scope=%v want %v", lim.Scope, wantScope)
		}
	}
	if !strings.Contains(lim.Message, "seller_api: credentials missing") {
		t.Fatalf("message=%q", lim.Message)
	}
	enrich, ok := dq.Sources["product_enrichment"].(map[string]any)
	if !ok || enrich["status"] != "unavailable" {
		t.Fatalf("product_enrichment source=%v", enrich)
	}
	stock, ok := dq.Sources["stock_analytics"].(map[string]any)
	if !ok || stock["status"] != "unavailable" {
		t.Fatalf("stock_analytics source=%v", stock)
	}
}
