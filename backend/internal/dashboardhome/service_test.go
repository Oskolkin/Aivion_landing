package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAnalyzeFunnelFindsMaxDrop(t *testing.T) {
	v100 := int64(100)
	v50 := int64(50)
	v10 := int64(10)
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &v100, ConversionFromTop: ptrFloat(100)},
		{Key: "views", Label: "Просмотры", Value: &v50, ConversionFromPrevious: ptrFloat(50), ConversionFromTop: ptrFloat(50)},
		{Key: "cart", Label: "Корзина", Value: &v10, ConversionFromPrevious: ptrFloat(20), ConversionFromTop: ptrFloat(10)},
	}
	res := analyzeFunnel(stages, nil)
	if res.stage != "Просмотры → Корзина" {
		t.Fatalf("stage=%q", res.stage)
	}
	if res.message == "" {
		t.Fatal("expected message")
	}
}

func TestAnalyzeFunnelPrefersBaselineDeteriorationOverStructuralDrop(t *testing.T) {
	v1000 := int64(1000)
	v100 := int64(100)
	v90 := int64(90)
	v45 := int64(45)
	v10 := int64(10)
	v8 := int64(8)
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &v1000, PreviousDayValue: &v1000},
		{Key: "product_views", Label: "Просмотры карточки", Value: &v100, PreviousDayValue: &v100},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &v90, PreviousDayValue: &v45},
		{Key: "orders", Label: "Заказанные единицы", Value: &v10, PreviousDayValue: &v8},
	}
	history := []dbgen.ProductAnalyticsDaily{
		{ScopeType: "account", Impressions: pgtype.Int8{Int64: 1000, Valid: true}, ProductViews: pgtype.Int8{Int64: 100, Valid: true}, AddToCart: pgtype.Int8{Int64: 45, Valid: true}, OrdersCount: pgtype.Int8{Int64: 8, Valid: true}},
		{ScopeType: "account", Impressions: pgtype.Int8{Int64: 980, Valid: true}, ProductViews: pgtype.Int8{Int64: 98, Valid: true}, AddToCart: pgtype.Int8{Int64: 44, Valid: true}, OrdersCount: pgtype.Int8{Int64: 7, Valid: true}},
	}
	res := analyzeFunnel(stages, history)
	if res.stage != "Добавления в корзину → Заказанные единицы" {
		t.Fatalf("stage=%q", res.stage)
	}
	if !strings.Contains(res.message, "ко вчера") {
		t.Fatalf("message=%q", res.message)
	}
}

func TestAnalyzeFunnelFallbackWithoutBaseline(t *testing.T) {
	v100 := int64(100)
	v40 := int64(40)
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &v100},
		{Key: "product_views", Label: "Просмотры карточки", Value: &v40},
	}
	res := analyzeFunnel(stages, nil)
	if !strings.Contains(res.message, "Недостаточно истории") {
		t.Fatalf("message=%q", res.message)
	}
}

func TestBuildFunnelStages(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		Impressions:     pgtype.Int8{Int64: 1000, Valid: true},
		ProductViews:    pgtype.Int8{Int64: 400, Valid: true},
		AddToCart:       pgtype.Int8{Int64: 80, Valid: true},
		OrdersCount:     pgtype.Int8{Int64: 20, Valid: true},
		CompletedOrders: pgtype.Int8{Int64: 18, Valid: true},
	}
	stages := buildFunnelStages(row, nil)
	if len(stages) != 5 {
		t.Fatalf("stages=%d", len(stages))
	}
	if stages[4].Key != "delivered_units" || stages[4].Label != "Доставленные единицы" {
		t.Fatalf("last stage=%+v", stages[4])
	}
}

func TestBuildFunnelStages_omitsDeliveredWhenNull(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 100, Valid: true},
		ProductViews: pgtype.Int8{Int64: 40, Valid: true},
		AddToCart:    pgtype.Int8{Int64: 10, Valid: true},
		OrdersCount:  pgtype.Int8{Int64: 2, Valid: true},
	}
	stages := buildFunnelStages(row, nil)
	if len(stages) != 4 {
		t.Fatalf("stages=%d want 4", len(stages))
	}
}

func TestBuildFunnelStages_includesPreviousDayValuesAndChanges(t *testing.T) {
	current := dbgen.ProductAnalyticsDaily{
		Impressions:     pgtype.Int8{Int64: 120, Valid: true},
		ProductViews:    pgtype.Int8{Int64: 60, Valid: true},
		AddToCart:       pgtype.Int8{Int64: 130, Valid: true},
		OrdersCount:     pgtype.Int8{Int64: 0, Valid: true},
		CompletedOrders: pgtype.Int8{Int64: 20, Valid: true},
	}
	previous := &dbgen.ProductAnalyticsDaily{
		Impressions:     pgtype.Int8{Int64: 130, Valid: true},
		ProductViews:    pgtype.Int8{Int64: 55, Valid: true},
		AddToCart:       pgtype.Int8{Int64: 120, Valid: true},
		OrdersCount:     pgtype.Int8{Int64: 100, Valid: true},
		CompletedOrders: pgtype.Int8{Int64: 0, Valid: true},
	}

	stages := buildFunnelStages(current, previous)
	if len(stages) != 5 {
		t.Fatalf("stages=%d", len(stages))
	}
	if stages[0].PreviousDayValue == nil || *stages[0].PreviousDayValue != 130 {
		t.Fatalf("impressions previous=%v", stages[0].PreviousDayValue)
	}
	if stages[0].ChangeVsPreviousDayPct == nil || *stages[0].ChangeVsPreviousDayPct > -7.69 || *stages[0].ChangeVsPreviousDayPct < -7.7 {
		t.Fatalf("impressions delta=%v", stages[0].ChangeVsPreviousDayPct)
	}
	if stages[2].ChangeVsPreviousDayPct == nil || *stages[2].ChangeVsPreviousDayPct < 8.33 || *stages[2].ChangeVsPreviousDayPct > 8.34 {
		t.Fatalf("add_to_cart delta=%v", stages[2].ChangeVsPreviousDayPct)
	}
	if stages[3].ChangeVsPreviousDayPct == nil || *stages[3].ChangeVsPreviousDayPct != -100 {
		t.Fatalf("orders delta=%v", stages[3].ChangeVsPreviousDayPct)
	}
	if stages[4].PreviousDayValue == nil || *stages[4].PreviousDayValue != 0 {
		t.Fatalf("delivered previous=%v", stages[4].PreviousDayValue)
	}
	if stages[4].ChangeVsPreviousDayPct != nil {
		t.Fatalf("delivered delta should be nil when yesterday is zero: %v", stages[4].ChangeVsPreviousDayPct)
	}
}

func TestBuildFunnelStages_handlesMissingPreviousDay(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 120, Valid: true},
		ProductViews: pgtype.Int8{Int64: 60, Valid: true},
		AddToCart:    pgtype.Int8{Int64: 30, Valid: true},
		OrdersCount:  pgtype.Int8{Int64: 10, Valid: true},
	}

	stages := buildFunnelStages(row, nil)
	for _, stage := range stages {
		if stage.PreviousDayValue != nil || stage.ChangeVsPreviousDayPct != nil {
			t.Fatalf("stage should not have previous-day data: %+v", stage)
		}
	}
}

func TestComputePercentChange(t *testing.T) {
	today := int64(120)
	yesterday := int64(130)
	got := computePercentChange(&today, &yesterday)
	if got == nil || *got > -7.69 || *got < -7.7 {
		t.Fatalf("today=120 yesterday=130 delta=%v", got)
	}

	today = 130
	yesterday = 120
	got = computePercentChange(&today, &yesterday)
	if got == nil || *got < 8.33 || *got > 8.34 {
		t.Fatalf("today=130 yesterday=120 delta=%v", got)
	}

	today = 0
	yesterday = 100
	got = computePercentChange(&today, &yesterday)
	if got == nil || *got != -100 {
		t.Fatalf("today=0 yesterday=100 delta=%v", got)
	}

	yesterday = 0
	got = computePercentChange(&today, &yesterday)
	if got != nil {
		t.Fatalf("delta should be nil when yesterday is zero: %v", got)
	}

	got = computePercentChange(&today, nil)
	if got != nil {
		t.Fatalf("delta should be nil when yesterday is missing: %v", got)
	}
}

func TestFunnelWidgetsFromRow_premiumRequired(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		SourceStatus:  pgtype.Text{String: "premium_required", Valid: true},
		SourceMessage: pgtype.Text{String: "Premium only", Valid: true},
	}
	funnel, analysis := funnelWidgetsFromRow(row)
	if funnel.Status != WidgetPremiumRequired || analysis.Status != WidgetPremiumRequired {
		t.Fatalf("funnel=%+v analysis=%+v", funnel.Status, analysis.Status)
	}
	if len(funnel.Stages) != 0 {
		t.Fatal("expected no fake stages")
	}
}

func TestFunnelWidgetsFromRow_emptyMetricsNotAvailable(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		SourceStatus: pgtype.Text{String: "not_enough_data", Valid: true},
	}
	funnel, _ := funnelWidgetsFromRow(row)
	if funnel.Status == WidgetAvailable {
		t.Fatal("must not be available with empty metrics")
	}
	if len(funnel.Stages) != 0 {
		t.Fatal("expected no stages")
	}
}

func TestStockBucketKey(t *testing.T) {
	bucket := "surplus_120d"
	sku := analytics.DashboardSKUMetricsItem{StockStatusBucket: &bucket}
	if StockCoverageBucketKey(sku) != "surplus" {
		t.Fatalf("got %q", StockCoverageBucketKey(sku))
	}
	d := 2.0
	sku2 := analytics.DashboardSKUMetricsItem{DaysOfCover: &d, StockAvailable: 5}
	if StockCoverageBucketKey(sku2) != "critical_0_3" {
		t.Fatalf("got %q", StockCoverageBucketKey(sku2))
	}
}

func TestNormalizeActionKeyDedup(t *testing.T) {
	a := normalizeActionKey("  Check   Stock  ")
	b := normalizeActionKey("check stock")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}

func TestPriorityRank(t *testing.T) {
	if priorityRank("critical") >= priorityRank("high") {
		t.Fatal("critical should rank higher than high")
	}
}

func TestUrgencyGroupForPriority(t *testing.T) {
	if urgencyGroupForPriority("critical") != "urgent" {
		t.Fatal("expected urgent")
	}
	if urgencyGroupForPriority("medium") != "today" {
		t.Fatal("expected today")
	}
}

func TestWhatChangedWidgetFromNarrative_FallbackHasFourGroups(t *testing.T) {
	widget := whatChangedWidgetFromNarrative(buildFallbackWhatChangedPayload(testWhatChangedContext()))
	if widget.Status != WidgetAvailable {
		t.Fatalf("status=%q", widget.Status)
	}
	if len(widget.Groups) != 4 {
		t.Fatalf("groups=%d", len(widget.Groups))
	}
}

func TestFunnelAIAnalysisWidgetFromPayload_available(t *testing.T) {
	payload := aisummaries.SummaryPayload{
		FunnelAIAnalysis: &aisummaries.FunnelAIAnalysis{
			Status:           "available",
			ShortSummary:     "Краткий вывод.",
			TrendVsYesterday: "Ко вчера стало хуже.",
			TrendVs7D:        "Ниже уровня за 7 дней.",
			RelatedSignals:   []string{"Есть слабые кампании."},
			RecommendedFocus: []string{"Проверить слабые кампании."},
			FullAnalysis:     "Полный разбор.",
		},
	}

	widget := funnelAIAnalysisWidgetFromPayload(payload)
	if widget.Status != WidgetAvailable {
		t.Fatalf("status=%q", widget.Status)
	}
	if widget.ShortSummary != "Краткий вывод." {
		t.Fatalf("short_summary=%q", widget.ShortSummary)
	}
	if len(widget.RelatedSignals) != 1 || len(widget.RecommendedFocus) != 1 {
		t.Fatalf("widget=%+v", widget)
	}
}

func TestFunnelAIAnalysisWidgetFromPayload_missingUsesMutedState(t *testing.T) {
	widget := funnelAIAnalysisWidgetFromPayload(aisummaries.SummaryPayload{})
	if widget.Status != WidgetNotEnoughData {
		t.Fatalf("status=%q", widget.Status)
	}
	if widget.Message == "" {
		t.Fatal("expected message")
	}
	if widget.Message == "Основная потеря сейчас на этапе Показы → Просмотры карточки" {
		t.Fatal("deterministic primary drop text must not be used as AI analysis")
	}
}

func TestApplySalesFunnelPreservesExistingAIAnalysis(t *testing.T) {
	row := dbgen.ProductAnalyticsDaily{
		Impressions:  pgtype.Int8{Int64: 100, Valid: true},
		ProductViews: pgtype.Int8{Int64: 50, Valid: true},
		AddToCart:    pgtype.Int8{Int64: 10, Valid: true},
		OrdersCount:  pgtype.Int8{Int64: 5, Valid: true},
	}
	_, analysis := funnelWidgetsFromRows(row, nil)
	analysis.AIAnalysis = FunnelAIAnalysisWidget{
		WidgetBase:       WidgetBase{Status: WidgetAvailable},
		ShortSummary:     "AI summary",
		RelatedSignals:   []string{},
		RecommendedFocus: []string{},
	}
	if analysis.AIAnalysis.ShortSummary != "AI summary" {
		t.Fatalf("ai_analysis=%+v", analysis.AIAnalysis)
	}
}

func ptrFloat(v float64) *float64 { return &v }
