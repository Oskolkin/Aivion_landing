package dashboardhome

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func funnelHistoryRowWithDelivered(impressions, views, cart, orders, delivered int64) dbgen.ProductAnalyticsDaily {
	row := funnelHistoryRow(impressions, views, cart, orders)
	row.CompletedOrders = pgtype.Int8{Valid: true, Int64: delivered}
	return row
}

func TestPreviousComparisonIncludesRevenue(t *testing.T) {
	cur := dbgen.ProductAnalyticsDaily{
		ScopeType: "account", ScopeID: "",
		Revenue:     mustNumeric(101721),
		OrdersCount: pgtype.Int8{Valid: true, Int64: 60},
	}
	prev := dbgen.ProductAnalyticsDaily{
		ScopeType: "account", ScopeID: "",
		Revenue:     mustNumeric(131000),
		OrdersCount: pgtype.Int8{Valid: true, Int64: 66},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Comparisons: FunnelComparisons{
			BusinessMetrics: []FunnelComparisonItem{
				{Key: "average_order_value", Label: "Средний чек", Delta: "-17.95%"},
			},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, &cur, &prev, nil)

	var revenue string
	for _, it := range ai.Comparisons.BusinessMetrics {
		if it.Key == "revenue" {
			revenue = it.Delta
		}
	}
	if revenue == "" || !strings.Contains(revenue, "%") {
		t.Fatalf("business_metrics missing revenue: %+v", ai.Comparisons.BusinessMetrics)
	}
}

func TestPreviousComparisonIncludesDeliveredUnits(t *testing.T) {
	del := int64(54)
	prevDel := int64(60)
	stages := []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		stage("orders", "Заказанные единицы", 60, ptr(-9.0)),
		{Key: "delivered_units", Label: "Доставленные единицы", Value: &del, PreviousDayValue: &prevDel, ChangeVsPreviousDayPct: ptr(-10.0)},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Comparisons: FunnelComparisons{
			PreviousPeriod: []FunnelComparisonItem{
				{Key: "impressions", Label: "Показы", Delta: "-4.2%"},
				{Key: "product_views", Label: "Просмотры карточки", Delta: "-13.6%"},
			},
			StageMetrics: []FunnelComparisonItem{
				{Key: "impressions", Label: "Показы", Delta: "-4.2%"},
			},
		},
	}, stages, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)

	found := false
	for _, it := range ai.Comparisons.StageMetrics {
		if it.Key == "delivered_units" && strings.Contains(it.Delta, "%") {
			found = true
		}
	}
	if !found {
		t.Fatalf("stage_metrics missing delivered_units: %+v", ai.Comparisons.StageMetrics)
	}
}

func TestSevenDayAverageIncludesAllStageMetrics(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		funnelHistoryRowWithDelivered(4000, 575, 147, 56, 50),
		funnelHistoryRowWithDelivered(4000, 575, 147, 56, 50),
	}
	ord := int64(60)
	del := int64(54)
	conv := 46.51
	stages := []FunnelStage{
		stage("impressions", "Показы", 4025, nil),
		stage("product_views", "Просмотры карточки", 533, nil),
		stage("add_to_cart", "Добавления в корзину", 129, nil),
		{Key: "orders", Label: "Заказанные единицы", Value: &ord, ConversionFromPrevious: &conv},
		{Key: "delivered_units", Label: "Доставленные единицы", Value: &del},
	}
	out := buildSevenDayAverageComparisons(stages, hist, nil)
	keys := map[string]bool{}
	for _, it := range out {
		keys[it.Key] = true
	}
	for _, want := range []string{"impressions", "product_views", "add_to_cart", "orders", "delivered_units"} {
		if !keys[want] {
			t.Fatalf("missing stage %s in %+v", want, out)
		}
	}
}

func TestSevenDayAverageIncludesConversionMetrics(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		funnelHistoryRow(4000, 575, 147, 56),
		funnelHistoryRow(4000, 575, 147, 56),
	}
	cart := int64(129)
	ord := int64(60)
	imp := int64(4025)
	views := int64(533)
	c2 := 13.24
	c3 := 24.2
	c4 := 46.51
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &imp},
		{Key: "product_views", Label: "Просмотры карточки", Value: &views, ConversionFromPrevious: &c2},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &cart, ConversionFromPrevious: &c3},
		{Key: "orders", Label: "Заказанные единицы", Value: &ord, ConversionFromPrevious: &c4},
	}
	out := buildSevenDayAverageComparisons(stages, hist, nil)
	wantConv := []string{
		"impressions_to_product_views",
		"product_views_to_add_to_cart",
		"add_to_cart_to_orders",
	}
	got := map[string]string{}
	for _, it := range out {
		if strings.Contains(it.Key, "_to_") {
			got[it.Key] = it.Delta
		}
	}
	for _, k := range wantConv {
		d, ok := got[k]
		if !ok || !strings.Contains(d, "п.п.") {
			t.Fatalf("missing conversion %s: %+v", k, out)
		}
	}
}

func TestSevenDayAverageIncludesAverageOrderValue(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		{
			ScopeType: "account", ScopeID: "",
			Impressions:  pgtype.Int8{Valid: true, Int64: 4000},
			ProductViews: pgtype.Int8{Valid: true, Int64: 575},
			AddToCart:    pgtype.Int8{Valid: true, Int64: 147},
			OrdersCount:  pgtype.Int8{Valid: true, Int64: 56},
			Revenue:      mustNumeric(80000),
		},
		{
			ScopeType: "account", ScopeID: "",
			Impressions:  pgtype.Int8{Valid: true, Int64: 4000},
			ProductViews: pgtype.Int8{Valid: true, Int64: 575},
			AddToCart:    pgtype.Int8{Valid: true, Int64: 147},
			OrdersCount:  pgtype.Int8{Valid: true, Int64: 56},
			Revenue:      mustNumeric(84000),
		},
	}
	cur := hist[0]
	cur.Revenue = mustNumeric(70000)
	cur.OrdersCount = pgtype.Int8{Valid: true, Int64: 60}
	ord := int64(60)
	stages := []FunnelStage{{Key: "orders", Label: "Заказанные единицы", Value: &ord}}
	out := buildSevenDayAverageComparisons(stages, hist, &cur)
	var aov string
	for _, it := range out {
		if it.Key == "average_order_value" {
			aov = it.Delta
		}
	}
	if aov == "" || !strings.Contains(aov, "%") {
		t.Fatalf("expected average_order_value delta, got %+v", out)
	}
}

func TestSevenDayConversionSkippedWhenDenominatorZero(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{funnelHistoryRow(0, 0, 0, 0)}
	imp := int64(100)
	views := int64(0)
	c := 0.0
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &imp},
		{Key: "product_views", Label: "Просмотры", Value: &views, ConversionFromPrevious: &c},
	}
	out := buildSevenDayAverageComparisons(stages, hist, nil)
	for _, it := range out {
		if it.Key == "impressions_to_product_views" {
			t.Fatalf("should not emit conversion with zero denominator: %+v", it)
		}
	}
}

func mustNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.2f", f)); err != nil {
		panic(err)
	}
	return n
}
