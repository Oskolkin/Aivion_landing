package dashboardhome

import (
	"math"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func funnelHistoryRow(impressions, views, cart, orders int64) dbgen.ProductAnalyticsDaily {
	return dbgen.ProductAnalyticsDaily{
		ScopeType:       "account",
		ScopeID:         "",
		Impressions:     pgtype.Int8{Valid: true, Int64: impressions},
		ProductViews:    pgtype.Int8{Valid: true, Int64: views},
		AddToCart:       pgtype.Int8{Valid: true, Int64: cart},
		OrdersCount:     pgtype.Int8{Valid: true, Int64: orders},
		CompletedOrders: pgtype.Int8{Valid: false},
	}
}

func TestBuildSevenDayAverageComparisons(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		funnelHistoryRow(4000, 575, 147, 56),
		funnelHistoryRow(4000, 575, 147, 56),
	}
	cart := int64(129)
	ord := int64(60)
	imp := int64(4025)
	views := int64(533)
	conv := math.Round(float64(ord)/float64(cart)*10000) / 100
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &imp},
		{Key: "product_views", Label: "Просмотры карточки", Value: &views},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &cart},
		{Key: "orders", Label: "Заказанные единицы", Value: &ord, ConversionFromPrevious: &conv},
	}
	out := buildSevenDayAverageComparisons(stages, hist, nil)
	if len(out) < 5 {
		t.Fatalf("expected stage rows + transition, got %d: %+v", len(out), out)
	}
	var pvDelta, cartDelta, transition string
	for _, it := range out {
		switch it.Key {
		case "product_views":
			pvDelta = it.Delta
		case "add_to_cart":
			cartDelta = it.Delta
		case "add_to_cart_to_orders":
			transition = it.Delta
		}
	}
	if pvDelta != "-7.3%" {
		t.Fatalf("product_views delta=%q", pvDelta)
	}
	if cartDelta != "-12.2%" {
		t.Fatalf("add_to_cart delta=%q", cartDelta)
	}
	if !strings.Contains(transition, "п.п.") {
		t.Fatalf("expected pp delta, got %q", transition)
	}
}

func TestSevenDayHistoryFillsComparisonsInEnsure(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		funnelHistoryRow(4000, 575, 147, 56),
		funnelHistoryRow(4000, 575, 147, 56),
	}
	wantSummary := "Просадка к среднему за 7 дней по корзине."
	ord := int64(60)
	conv := math.Round(float64(ord)/float64(129)*10000) / 100
	stages := []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		{Key: "orders", Label: "Заказанные единицы", Value: &ord, ChangeVsPreviousDayPct: ptr(7.46), ConversionFromPrevious: &conv},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		Summary:    wantSummary,
		Comparisons: FunnelComparisons{
			SevenDayAverage: nil,
		},
	}, stages, FunnelEvidenceSnapshot{}, hist, nil, nil, nil)
	if len(ai.Comparisons.SevenDayAverage) == 0 {
		t.Fatal("expected seven_day_average from history")
	}
	if strings.TrimSpace(ai.Summary) != wantSummary {
		t.Fatalf("summary changed unexpectedly: %q", ai.Summary)
	}
}

func TestDirectionsStripInfinitiveCheckVerbsInLabel(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		CheckDirections: []FunnelTextItem{
			{Key: "x", Label: "Проверить карточки и категории", Text: "какие дали основное снижение просмотров.", Tone: "neutral"},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.CheckDirections) != 1 {
		t.Fatalf("len=%d", len(ai.CheckDirections))
	}
	l := strings.ToLower(strings.TrimSpace(ai.CheckDirections[0].Label))
	if strings.HasPrefix(l, "проверить") || strings.HasPrefix(l, "разобрать") || strings.HasPrefix(l, "сверить") {
		t.Fatalf("label still imperative: %q", ai.CheckDirections[0].Label)
	}
}
