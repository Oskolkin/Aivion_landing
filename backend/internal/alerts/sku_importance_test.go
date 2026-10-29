package alerts

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestLargeSellerSKUWith10000RevenueNotKey(t *testing.T) {
	const activeCount = 1000
	const targetProductID int64 = 500
	metrics := make([]SKUDailyMetric, 0, activeCount)
	for i := 0; i < activeCount; i++ {
		pid := int64(i + 1)
		revenue := 1_000_000.0
		orders := int32(100)
		if pid == targetProductID {
			revenue = 10_000
			orders = 5
		}
		metrics = append(metrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[targetProductID]
	if ctx.ActiveSKUCount != activeCount {
		t.Fatalf("active_sku_count=%d, want %d", ctx.ActiveSKUCount, activeCount)
	}
	if ctx.RevenueShare7D >= keySKURevenueShareMin {
		t.Fatalf("revenue_share_7d=%f, expected < 5%%", ctx.RevenueShare7D)
	}
	if ctx.OrdersShare7D >= keySKUOrdersShareMin {
		t.Fatalf("orders_share_7d=%f, expected < 5%%", ctx.OrdersShare7D)
	}
	if ctx.RevenueRank7D <= keySKUTopActivePercentMax {
		t.Fatalf("revenue_rank_7d=%d, expected > top-30 cap", ctx.RevenueRank7D)
	}
	if ctx.OrdersRank7D <= keySKUTopActivePercentMax {
		t.Fatalf("orders_rank_7d=%d, expected > top-30 cap", ctx.OrdersRank7D)
	}
	if ctx.IsKeySKU {
		t.Fatalf("expected non-key SKU for large seller, reasons=%v", ctx.KeyReasons)
	}
}

func TestLargeSellerDoesNotTriggerKeySKUAlerts(t *testing.T) {
	const targetProductID int64 = 500
	metrics := make([]SKUDailyMetric, 0, 1000)
	for i := 0; i < 1000; i++ {
		pid := int64(i + 1)
		revenue := 1_000_000.0
		orders := int32(100)
		if pid == targetProductID {
			revenue = 10_000
			orders = 5
		}
		metrics = append(metrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)

	days := 5.0
	stockSKU := baseStockSKU()
	stockSKU.OzonProductID = targetProductID
	stockSKU.CurrentStock = 0
	stockSKU.DaysOfCover = &days
	if _, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{stockSKU}, importance), stockSKU); ok {
		t.Fatal("stock_critical_sku_low_stock must not fire for non-key SKU in large catalog")
	}

	priceCtx := basePriceContext()
	priceCtx.OzonProductID = targetProductID
	priceCtx.HasEffectiveConstraint = false
	if _, ok := evaluateMissingPricingConstraintsForKeySKU(basePriceInput([]ProductPricingContext{priceCtx}, importance), priceCtx); ok {
		t.Fatal("missing_pricing_constraints_for_key_sku must not fire for non-key SKU in large catalog")
	}
}

func TestSmallSellerTop5RevenueIsKey(t *testing.T) {
	metrics := make([]SKUDailyMetric, 0, 20)
	for i := 0; i < 20; i++ {
		pid := int64(i + 1)
		metrics = append(metrics, dailyMetric(pid, float64(20-i), int32(i+1)))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatalf("expected top-5 revenue SKU to be key, rank=%d reasons=%v", ctx.RevenueRank7D, ctx.KeyReasons)
	}
	if !containsReason(ctx.KeyReasons, keySKUReasonTopRevenueSmall) {
		t.Fatalf("expected small-account revenue reason, got %v", ctx.KeyReasons)
	}
}

func TestSmallSellerTop5OrdersIsKey(t *testing.T) {
	metrics := make([]SKUDailyMetric, 0, 20)
	for i := 0; i < 20; i++ {
		pid := int64(i + 1)
		metrics = append(metrics, dailyMetric(pid, 100, int32(20-i)))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatalf("expected top-5 orders SKU to be key, rank=%d reasons=%v", ctx.OrdersRank7D, ctx.KeyReasons)
	}
	if !containsReason(ctx.KeyReasons, keySKUReasonTopOrdersSmall) {
		t.Fatalf("expected small-account orders reason, got %v", ctx.KeyReasons)
	}
}

func TestSmallSellerRank6NotKeyWithoutShare(t *testing.T) {
	metrics := make([]SKUDailyMetric, 0, 20)
	for i := 0; i < 20; i++ {
		pid := int64(i + 1)
		revenue := 1.0
		orders := int32(1)
		if pid == 1 {
			revenue = 1000
			orders = 1000
		}
		metrics = append(metrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[6]
	if ctx.RevenueRank7D != 6 {
		t.Fatalf("revenue_rank_7d=%d, want 6", ctx.RevenueRank7D)
	}
	if ctx.RevenueShare7D >= keySKURevenueShareMin || ctx.OrdersShare7D >= keySKUOrdersShareMin {
		t.Fatalf("rank-6 SKU should be below 5%% share, rev_share=%f orders_share=%f",
			ctx.RevenueShare7D, ctx.OrdersShare7D)
	}
	if ctx.IsKeySKU {
		t.Fatalf("rank-6 SKU must not be key without >=5%% share or top-5 rank, reasons=%v", ctx.KeyReasons)
	}
}

func TestRevenueShareAtLeastFivePercentIsKey(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 50_000, 0),
		dailyMetric(2, 950_000, 10),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatal("expected key SKU by revenue share with zero orders")
	}
	if !containsReason(ctx.KeyReasons, keySKUReasonRevenueShare) {
		t.Fatalf("expected revenue share reason, got %v", ctx.KeyReasons)
	}
}

func TestOrdersShareAtLeastFivePercentIsKey(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 0, 50),
		dailyMetric(2, 100, 950),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatal("expected key SKU by orders share with zero revenue")
	}
	if !containsReason(ctx.KeyReasons, keySKUReasonOrdersShare) {
		t.Fatalf("expected orders share reason, got %v", ctx.KeyReasons)
	}
}

func TestSKUWithRevenueOnlyCanBeKeyByRevenueRank(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 100_000, 0),
		dailyMetric(2, 100, 5),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatalf("SKU with revenue only should be key, reasons=%v", ctx.KeyReasons)
	}
}

func TestSKUWithOrdersOnlyCanBeKeyByOrdersRank(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 0, 100),
		dailyMetric(2, 100, 5),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatalf("SKU with orders only should be key, reasons=%v", ctx.KeyReasons)
	}
}

func TestSKUWithoutSalesNotKey(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 0, 0),
		dailyMetric(2, 100, 5),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	if importance[1].IsKeySKU {
		t.Fatal("SKU without revenue and orders must not be key")
	}
	if importance[1].RevenueRank7D != 0 || importance[1].OrdersRank7D != 0 {
		t.Fatal("inactive SKU must not receive ranks")
	}
}

func TestActiveSKUCountExcludesZeroOnly(t *testing.T) {
	metrics := make([]SKUDailyMetric, 0, 120)
	for i := 0; i < 120; i++ {
		pid := int64(i + 1)
		revenue := 0.0
		orders := int32(0)
		if i < 20 {
			revenue = float64(20 - i)
			orders = 1
		}
		metrics = append(metrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	var ctx SKUImportanceContext
	for _, c := range importance {
		ctx = c
		break
	}
	if ctx.ActiveSKUCount != 20 {
		t.Fatalf("active_sku_count=%d, want 20", ctx.ActiveSKUCount)
	}
}

func TestKeySKUTopPercentileCutoffExamples(t *testing.T) {
	cases := []struct {
		active int
		want   int
	}{
		{0, 0},
		{1, 1},
		{5, 1},
		{20, 2},
		{80, 8},
		{200, 20},
		{1000, 30},
	}
	for _, tc := range cases {
		if got := keySKUTopPercentileCutoff(tc.active); got != tc.want {
			t.Fatalf("active=%d: got topLimit=%d, want %d", tc.active, got, tc.want)
		}
	}
}

func TestRankTieBreaksByProductID(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(2, 100, 1),
		dailyMetric(1, 100, 1),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	if importance[1].RevenueRank7D != 1 {
		t.Fatalf("lower product_id should win tie, rank=%d", importance[1].RevenueRank7D)
	}
	if importance[2].RevenueRank7D != 2 {
		t.Fatalf("higher product_id should be second on tie, rank=%d", importance[2].RevenueRank7D)
	}
}

func TestKeySKUReasonsCanAccumulate(t *testing.T) {
	metrics := []SKUDailyMetric{
		dailyMetric(1, 60_000, 60),
		dailyMetric(2, 40_000, 40),
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	ctx := importance[1]
	if !ctx.IsKeySKU {
		t.Fatal("expected key SKU")
	}
	if len(ctx.KeyReasons) < 2 {
		t.Fatalf("expected multiple reasons, got %v", ctx.KeyReasons)
	}
}

func TestEvidencePayloadNoNaNOrInf(t *testing.T) {
	ctx := SKUImportanceContext{
		Revenue7D:      0,
		Orders7D:       0,
		TotalRevenue7D: 0,
		TotalOrders7D:  0,
		ActiveSKUCount: 0,
		IsKeySKU:       false,
		KeyReasons:     nil,
	}
	payload := MergeKeySKUImportanceEvidence(EvidencePayload{}, ctx)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if strings.Contains(string(raw), "NaN") || strings.Contains(string(raw), "Infinity") {
		t.Fatalf("evidence must not contain NaN/Inf: %s", raw)
	}
}

func TestNilOptionalFieldsDoNotPanic(t *testing.T) {
	metrics := []SKUDailyMetric{{
		SellerAccountID: 1,
		MetricDate:      time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		OzonProductID:   1,
		SKU:             nil,
		OfferID:         nil,
		ProductName:     nil,
		CurrentStock:    0,
		DaysOfCover:     nil,
		Revenue:         100,
		OrdersCount:     1,
	}}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	if math.IsNaN(importance[1].RevenueShare7D) || math.IsInf(importance[1].RevenueShare7D, 0) {
		t.Fatal("share must be finite")
	}
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

func dailyMetric(productID int64, revenue float64, orders int32) SKUDailyMetric {
	return SKUDailyMetric{
		SellerAccountID: 1,
		MetricDate:      time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		OzonProductID:   productID,
		Revenue:         revenue,
		OrdersCount:     orders,
	}
}
