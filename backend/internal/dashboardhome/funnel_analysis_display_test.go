package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func stage(key, label string, value int64, delta *float64) FunnelStage {
	return FunnelStage{Key: key, Label: label, Value: &value, ChangeVsPreviousDayPct: delta}
}

func ptr(v float64) *float64 { return &v }

func TestFunnelAnalysisCurrentCase(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Открытия карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		stage("orders", "Заказанные единицы", 24, ptr(-9.1)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Key != "add_to_cart" {
		t.Fatalf("weakest key=%q", ai.WeakestStage.Key)
	}
	if ai.WeakestStage.Label != "Переход в корзину" || ai.WeakestStage.Metric != "Добавления в корзину" {
		t.Fatalf("weakest label/metric=%q / %q", ai.WeakestStage.Label, ai.WeakestStage.Metric)
	}
	if ai.WeakestStage.DeltaPrevious != "-27.9%" {
		t.Fatalf("delta_prev=%q", ai.WeakestStage.DeltaPrevious)
	}
}

func TestFunnelAnalysisImpressionsDrop(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("impressions", "Показы", 100, ptr(-40)),
		stage("product_views", "Открытия карточки", 50, ptr(-5)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Label != "Входящий трафик (показы)" {
		t.Fatalf("weakest=%q", ai.WeakestStage.Label)
	}
}

func TestFunnelAnalysisCartToOrderDrop(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("add_to_cart", "Добавления в корзину", 100, ptr(-2)),
		stage("orders", "Заказанные единицы", 20, ptr(-35)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Label != "Оформление заказа" {
		t.Fatalf("weakest=%q", ai.WeakestStage.Label)
	}
}

func TestFunnelAnalysisCardToCartDrop(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("product_views", "Открытия карточки", 100, ptr(-1)),
		stage("add_to_cart", "Добавления в корзину", 20, ptr(-33)),
		stage("orders", "Заказанные единицы", 18, ptr(-2)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Label != "Переход в корзину" {
		t.Fatalf("weakest=%q", ai.WeakestStage.Label)
	}
}

func TestFunnelAnalysisDeliveredDrop(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("orders", "Заказанные единицы", 100, ptr(2)),
		stage("delivered_units", "Доставленные единицы", 60, ptr(-30)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Label != "Доставка" {
		t.Fatalf("weakest=%q", ai.WeakestStage.Label)
	}
}

func TestFunnelAnalysisStable(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, []FunnelStage{
		stage("impressions", "Показы", 100, ptr(1)),
		stage("product_views", "Открытия карточки", 55, ptr(0.5)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Status != "stable" {
		t.Fatalf("status=%q", ai.WeakestStage.Status)
	}
}

func TestFunnelAnalysisAllStagesDropSummary(t *testing.T) {
	summary := "AI: заметное падение на всех этапах воронки относительно прошлой сводки."
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		Summary:    summary,
	}, []FunnelStage{
		stage("impressions", "Показы", 100, ptr(-15)),
		stage("product_views", "Открытия карточки", 55, ptr(-16)),
		stage("add_to_cart", "Добавления в корзину", 20, ptr(-18)),
		stage("orders", "Заказанные единицы", 8, ptr(-20)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if strings.TrimSpace(ai.Summary) != summary {
		t.Fatalf("summary mutated: %q", ai.Summary)
	}
	if ai.WeakestStage.Key != "orders" {
		t.Fatalf("weakest key=%q", ai.WeakestStage.Key)
	}
}

func TestFunnelAnalysisInsufficientData(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Status != "unavailable" {
		t.Fatalf("status=%q", ai.WeakestStage.Status)
	}
}

func TestAIWeakestStagePreservedAndEnrich7D(t *testing.T) {
	w := FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		WeakestStage: FunnelWeakestStage{
			Status: "available", Key: "add_to_cart", Label: "Переход в корзину", Metric: "Добавления в корзину",
			Explanation: "AI weakest explanation", DeltaPrevious: "-27.9%",
		},
		Comparisons: FunnelComparisons{
			SevenDayAverage: []FunnelComparisonItem{
				{Key: "add_to_cart", Label: "Добавления в корзину", Delta: "-12.2%", Tone: "negative"},
			},
		},
	}
	ai := applyFunnelAIGuards(w, []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.WeakestStage.Explanation != "AI weakest explanation" {
		t.Fatalf("explanation=%q", ai.WeakestStage.Explanation)
	}
	if ai.WeakestStage.Delta7D != "-12.2%" {
		t.Fatalf("delta7d=%q", ai.WeakestStage.Delta7D)
	}
}

func TestSevenDayConsistency(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Summary: "Просадка к среднему за 7 дней по корзине и за последние 7 дней по карточке.",
		Comparisons: FunnelComparisons{
			SevenDayAverage: nil,
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	lower := strings.ToLower(ai.Summary)
	if strings.Contains(lower, "7 дн") || strings.Contains(lower, "7дн") {
		t.Fatalf("7d language should be stripped without rows: %q", ai.Summary)
	}

	keep := "Просадка к среднему за 7 дней по корзине."
	ai2 := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Summary: keep,
		Comparisons: FunnelComparisons{
			SevenDayAverage: []FunnelComparisonItem{{Key: "add_to_cart", Label: "Корзина", Delta: "-1%", Tone: "negative"}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if strings.TrimSpace(ai2.Summary) != keep {
		t.Fatalf("expected summary kept, got %q", ai2.Summary)
	}
}

func limitedHistoryStages() []FunnelStage {
	ord := int64(25)
	conv := 41.0
	return []FunnelStage{
		stage("impressions", "Показы", 1300, ptr(-1)),
		stage("product_views", "Просмотры карточки", 130, ptr(-2)),
		stage("add_to_cart", "Добавления в корзину", 55, ptr(-3)),
		{Key: "orders", Label: "Заказанные единицы", Value: &ord, ConversionFromPrevious: &conv},
	}
}

func limitedHistoryRows() []dbgen.ProductAnalyticsDaily {
	return []dbgen.ProductAnalyticsDaily{
		funnelHistoryRow(1000, 100, 50, 20),
		funnelHistoryRow(1200, 120, 60, 24),
	}
}

func hasLimitedHistorySignal(items []FunnelComparisonItem) bool {
	for _, it := range items {
		if it.Key == "history_status" && strings.Contains(it.Value, "доступно меньше 7 дней истории") {
			return true
		}
	}
	return false
}

func TestSevenDayLimitedHistoryNoteIsSeparateSignalNotInSummary(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
	}, limitedHistoryStages(), FunnelEvidenceSnapshot{}, limitedHistoryRows(), nil, nil, nil)

	// Note must live as a separate context signal (rendered as an InfoNote)...
	if !hasLimitedHistorySignal(ai.Comparisons.ContextSignals) {
		t.Fatalf("limited-history signal missing from context_signals: %+v", ai.Comparisons.ContextSignals)
	}
	// ...and must NOT be duplicated inside the 7-day commentary.
	if strings.Contains(ai.ComparisonsCommentary.SevenDayAverage, "Сравнение ограничено") {
		t.Fatalf("limited-history note leaked into commentary: %q", ai.ComparisonsCommentary.SevenDayAverage)
	}
}

func TestLimitedHistoryRephrasesSevenDayWording(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{
			SevenDayAverage: "Заказы держатся выше среднего за 7 дней, просадка локальна.",
		},
	}, limitedHistoryStages(), FunnelEvidenceSnapshot{}, limitedHistoryRows(), nil, nil, nil)

	lower := strings.ToLower(ai.ComparisonsCommentary.SevenDayAverage)
	if strings.Contains(lower, "за 7 дней") || strings.Contains(lower, "за последние 7 дней") {
		t.Fatalf("7-day wording not rephrased under limited history: %q", ai.ComparisonsCommentary.SevenDayAverage)
	}
	if !strings.Contains(lower, "доступн") {
		t.Fatalf("expected period-safe wording, got: %q", ai.ComparisonsCommentary.SevenDayAverage)
	}
}

func TestCommentaryStripsInlineLimitedHistoryNote(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{
			SevenDayAverage: "Слабее работает середина воронки. Сравнение ограничено: доступно меньше 7 дней истории.",
		},
	}, limitedHistoryStages(), FunnelEvidenceSnapshot{}, limitedHistoryRows(), nil, nil, nil)
	if strings.Contains(ai.ComparisonsCommentary.SevenDayAverage, "Сравнение ограничено") {
		t.Fatalf("inline note not stripped: %q", ai.ComparisonsCommentary.SevenDayAverage)
	}
	if !strings.Contains(ai.ComparisonsCommentary.SevenDayAverage, "середина воронки") {
		t.Fatalf("analytical part dropped: %q", ai.ComparisonsCommentary.SevenDayAverage)
	}
}

func TestEmptyCommentaryGetsAnalyticalFallback(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
	}, []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		stage("orders", "Заказанные единицы", 60, ptr(-9.1)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	prev := ai.ComparisonsCommentary.PreviousPeriod
	if strings.TrimSpace(prev) == "" {
		t.Fatalf("previous commentary fallback is empty")
	}
	if !strings.Contains(prev, "воронк") {
		t.Fatalf("fallback is not analytical: %q", prev)
	}
}

func TestNumberDumpCommentaryReplacedByFallback(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{
			PreviousPeriod: "Показы -4,2%, просмотры -13,6%, добавления -27,9%, заказы -9,1%.",
		},
	}, []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		stage("orders", "Заказанные единицы", 60, ptr(-9.1)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	prev := ai.ComparisonsCommentary.PreviousPeriod
	if strings.Contains(prev, "-27,9%") || strings.Contains(prev, "-13,6%") {
		t.Fatalf("number dump not replaced: %q", prev)
	}
	if !strings.Contains(prev, "воронк") {
		t.Fatalf("fallback not analytical: %q", prev)
	}
}

func TestCommentaryReplacesLegacyTerms(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{
			PreviousPeriod: "Оформленные заказы держатся, но выкуп просел сильнее остальных этапов.",
		},
	}, []FunnelStage{
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-5)),
		stage("orders", "Заказанные единицы", 60, ptr(-9.1)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	prev := strings.ToLower(ai.ComparisonsCommentary.PreviousPeriod)
	if strings.Contains(prev, "оформленные заказы") || strings.Contains(prev, "выкуп") {
		t.Fatalf("legacy terms not replaced: %q", ai.ComparisonsCommentary.PreviousPeriod)
	}
}

func TestOverlyGenericPreviousCommentaryReplaced(t *testing.T) {
	// Real stored AI text that only describes movement without interpreting it.
	generic := "По сравнению с 20.05.2026 верх воронки просел умеренно, а самое резкое ухудшение произошло на этапе добавления в корзину. При этом переход из корзины в заказ, наоборот, стал сильнее."
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase:            WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{PreviousPeriod: generic},
		Comparisons: FunnelComparisons{
			ConversionMetrics: []FunnelComparisonItem{
				{Key: "add_to_cart_to_orders", Label: "Корзина → заказ", Delta: "+8,9 п.п.", Tone: "positive"},
			},
		},
	}, []FunnelStage{
		stage("impressions", "Показы", 4025, ptr(-4.2)),
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
		stage("orders", "Заказанные единицы", 60, ptr(-9.1)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)

	prev := ai.ComparisonsCommentary.PreviousPeriod
	if prev == generic {
		t.Fatalf("overly-generic commentary was not replaced: %q", prev)
	}
	if !strings.Contains(prev, "середине воронки") {
		t.Fatalf("fallback should localize to middle of funnel: %q", prev)
	}
	if !strings.Contains(prev, "не компенсирует") {
		t.Fatalf("fallback should add compensation caveat when lower transition improved: %q", prev)
	}
}

func TestInterpretivePreviousCommentaryKept(t *testing.T) {
	good := "Основная просадка в середине воронки: после просмотра карточек реже добавляют товары в корзину, поэтому слабее наполняется заказ."
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase:            WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{PreviousPeriod: good},
	}, []FunnelStage{
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.ComparisonsCommentary.PreviousPeriod != good {
		t.Fatalf("interpretive AI commentary must be kept, got %q", ai.ComparisonsCommentary.PreviousPeriod)
	}
}

func TestGenericFallbackUniversalRegions(t *testing.T) {
	cases := []struct {
		name   string
		stages []FunnelStage
		want   string
	}{
		{"upper", []FunnelStage{stage("impressions", "Показы", 100, ptr(-40)), stage("product_views", "Просмотры", 50, ptr(-5))}, "верхней части воронки"},
		{"lower", []FunnelStage{stage("add_to_cart", "Корзина", 100, ptr(-2)), stage("orders", "Заказанные единицы", 20, ptr(-35))}, "нижний переход"},
		{"delivery", []FunnelStage{stage("orders", "Заказанные единицы", 100, ptr(-2)), stage("delivered_units", "Доставленные единицы", 20, ptr(-40))}, "выполнение заказов"},
	}
	for _, c := range cases {
		ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
			WidgetBase:            WidgetBase{Status: WidgetAvailable},
			ComparisonsCommentary: FunnelComparisonsCommentary{PreviousPeriod: "Показатели в целом просели."},
		}, c.stages, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
		if !strings.Contains(ai.ComparisonsCommentary.PreviousPeriod, c.want) {
			t.Fatalf("%s: expected %q in %q", c.name, c.want, ai.ComparisonsCommentary.PreviousPeriod)
		}
	}
}

func TestGoodCommentaryKeptAsPrimary(t *testing.T) {
	good := "Основная просадка в середине воронки: после просмотра карточек реже добавляют товары в корзину."
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase:            WidgetBase{Status: WidgetAvailable},
		ComparisonsCommentary: FunnelComparisonsCommentary{PreviousPeriod: good},
	}, []FunnelStage{
		stage("product_views", "Просмотры карточки", 533, ptr(-13.6)),
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.ComparisonsCommentary.PreviousPeriod != good {
		t.Fatalf("good AI commentary should be kept verbatim, got %q", ai.ComparisonsCommentary.PreviousPeriod)
	}
}

func TestCausalLanguageSoftened(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Summary: "Падение вызвало просадку и привело к проблемам из-за рекламы.",
		DropFactors: []FunnelDropFactor{
			{Label: "Реклама", Explanation: "Снижение вызвало падение."},
		},
		CheckDirections: []FunnelTextItem{
			{Label: "Диагностика", Text: "Из-за рекламы проверьте кампании."},
		},
		DetailsSections: []FunnelDetailsSection{
			{Title: "Итог", Items: []FunnelDetailsSectionItem{{Text: "Это привело к просадке."}}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)

	blob := strings.ToLower(ai.Summary + " " + ai.DropFactors[0].Explanation + " " + ai.CheckDirections[0].Text + " " + ai.DetailsSections[0].Items[0].Text)
	for _, bad := range []string{"вызвало", "привело", "из-за"} {
		if strings.Contains(blob, bad) {
			t.Fatalf("causal term remained: %q in %q", bad, blob)
		}
	}
}

func TestSoftenCausalClaims_readableAndKeepsFinancialLinkVerb(t *testing.T) {
	in := "Снижение среднего чека привело к падению выручки, а это влияет на маржу из-за роста скидок."
	out := softenCausalClaims(in)
	if strings.Contains(strings.ToLower(out), "привело к") || strings.Contains(strings.ToLower(out), "из-за") {
		t.Fatalf("hard causality should be softened: %q", out)
	}
	if !strings.Contains(strings.ToLower(out), "влияет на маржу") {
		t.Fatalf("correct financial relation should remain readable: %q", out)
	}
	if strings.Contains(strings.ToLower(out), "связано с к") {
		t.Fatalf("grammar degraded: %q", out)
	}
}

func TestDetailsSectionsNormalizedOrderAndSingleConclusion(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DetailsSections: []FunnelDetailsSection{
			{Title: "summary", Items: []FunnelDetailsSectionItem{{Text: "Итог 1"}}},
			{Title: "supporting_evidence", Items: []FunnelDetailsSectionItem{{Text: "Факт 1"}}},
			{Title: "related_signals", Items: []FunnelDetailsSectionItem{{Text: "Сигнал 1"}}},
			{Title: "conclusion", Items: []FunnelDetailsSectionItem{{Text: "Итог 1"}}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DetailsSections) < 3 {
		t.Fatalf("sections=%+v", ai.DetailsSections)
	}
	titles := make([]string, 0, len(ai.DetailsSections))
	for _, s := range ai.DetailsSections {
		titles = append(titles, s.Title)
	}
	if titles[len(titles)-1] != "Итог" {
		t.Fatalf("conclusion must be last, got %v", titles)
	}
	conclusions := 0
	for _, t := range titles {
		if t == "Итог" {
			conclusions++
		}
	}
	if conclusions != 1 {
		t.Fatalf("expected single conclusion, got %v", titles)
	}
}

func TestDetailsSectionsAliasMappingRussianAndEnglish(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DetailsSections: []FunnelDetailsSection{
			{Title: "Supporting evidence", Items: []FunnelDetailsSectionItem{{Text: "Факт 1"}}},
			{Title: "Что смягчает вывод", Items: []FunnelDetailsSectionItem{{Text: "Факт 2"}}},
			{Title: "Related signals", Items: []FunnelDetailsSectionItem{{Text: "Факт 3"}}},
			{Title: "Ограничения анализа", Items: []FunnelDetailsSectionItem{{Text: "Факт 4"}}},
			{Title: "Conclusion", Items: []FunnelDetailsSectionItem{{Text: "Факт 5"}}},
			{Title: "Вывод", Items: []FunnelDetailsSectionItem{{Text: "Факт 6"}}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DetailsSections) != 5 {
		t.Fatalf("sections=%+v", ai.DetailsSections)
	}
	titles := []string{
		ai.DetailsSections[0].Title,
		ai.DetailsSections[1].Title,
		ai.DetailsSections[2].Title,
		ai.DetailsSections[3].Title,
		ai.DetailsSections[4].Title,
	}
	expect := []string{
		"Что подтверждает вывод",
		"Что смягчает вывод",
		"Связанные сигналы",
		"Ограничения анализа",
		"Итог",
	}
	for i := range expect {
		if titles[i] != expect[i] {
			t.Fatalf("titles=%v", titles)
		}
	}
}

func TestSKUFunnelClaimSoftenedInDirectionsAndDetails(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		CheckDirections: []FunnelTextItem{
			{Label: "Проверка", Text: "Проверьте товары, где просмотры есть, а добавления в корзину снизились."},
		},
		DetailsSections: []FunnelDetailsSection{
			{Title: "Связанные сигналы", Items: []FunnelDetailsSectionItem{{Text: "SKU с просмотрами и падением добавлений в корзину требуют проверки."}}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)

	dir := strings.ToLower(ai.CheckDirections[0].Text)
	if !strings.Contains(dir, "если sku-детализация") {
		t.Fatalf("direction must be softened: %q", ai.CheckDirections[0].Text)
	}
	detail := strings.ToLower(ai.DetailsSections[0].Items[0].Text)
	if !strings.Contains(detail, "в mvp начните с товаров с падением заказов") {
		t.Fatalf("details must be softened: %q", ai.DetailsSections[0].Items[0].Text)
	}
}

func TestAIPayloadLegacyLabelsAreNormalized(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		Comparisons: FunnelComparisons{
			StageMetrics: []FunnelComparisonItem{
				{Key: "orders", Label: "Оформленные заказы", Value: "10"},
				{Key: "delivered_units", Label: "Выкупили товар", Value: "9"},
			},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if ai.Comparisons.StageMetrics[0].Label != "Заказанные единицы" {
		t.Fatalf("orders label=%q", ai.Comparisons.StageMetrics[0].Label)
	}
	if ai.Comparisons.StageMetrics[1].Label != "Доставленные единицы" {
		t.Fatalf("delivered label=%q", ai.Comparisons.StageMetrics[1].Label)
	}
}

func TestConversionMetricsFallbackBuiltWhenMissing(t *testing.T) {
	v1 := int64(1000)
	v2 := int64(200)
	v3 := int64(50)
	v4 := int64(20)
	p1 := int64(1200)
	p2 := int64(240)
	p3 := int64(60)
	p4 := int64(24)
	c2 := 20.0
	c3 := 25.0
	c4 := 40.0
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &v1, PreviousDayValue: &p1},
		{Key: "product_views", Label: "Просмотры карточки", Value: &v2, PreviousDayValue: &p2, ConversionFromPrevious: &c2},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &v3, PreviousDayValue: &p3, ConversionFromPrevious: &c3},
		{Key: "orders", Label: "Заказанные единицы", Value: &v4, PreviousDayValue: &p4, ConversionFromPrevious: &c4},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{}, stages, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.Comparisons.ConversionMetrics) < 3 {
		t.Fatalf("conversion_metrics=%+v", ai.Comparisons.ConversionMetrics)
	}
	if !strings.Contains(ai.Comparisons.ConversionMetrics[0].Delta, "п.п.") {
		t.Fatalf("expected pp delta, got %q", ai.Comparisons.ConversionMetrics[0].Delta)
	}
}

func TestConversionMetricsFallbackOmitsDeliveredWhenUnavailable(t *testing.T) {
	v1 := int64(1000)
	v2 := int64(200)
	v3 := int64(50)
	c2 := 20.0
	c3 := 25.0
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: &v1},
		{Key: "product_views", Label: "Просмотры карточки", Value: &v2, ConversionFromPrevious: &c2},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &v3, ConversionFromPrevious: &c3},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{}, stages, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	for _, it := range ai.Comparisons.ConversionMetrics {
		if strings.Contains(it.Key, "delivered_units") {
			t.Fatalf("unexpected delivered conversion %+v", it)
		}
	}
}

func TestContextSignalsFallbackIncludesLimitedHistory(t *testing.T) {
	hist := []dbgen.ProductAnalyticsDaily{
		funnelHistoryRow(1000, 100, 50, 20),
		funnelHistoryRow(1100, 110, 44, 18),
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{}, []FunnelStage{
		stage("impressions", "Показы", 1300, ptr(-1)),
		stage("product_views", "Просмотры карточки", 130, ptr(-2)),
	}, FunnelEvidenceSnapshot{}, hist, nil, nil, nil)
	if len(ai.Comparisons.ContextSignals) == 0 {
		t.Fatalf("context_signals empty")
	}
	if !strings.Contains(strings.ToLower(ai.Comparisons.ContextSignals[0].Value), "меньше 7 дней") {
		t.Fatalf("context_signal=%+v", ai.Comparisons.ContextSignals[0])
	}
}

func TestContextSignalsFallbackIncludesLimitedHistoryWhenZeroDays(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{}, []FunnelStage{
		stage("impressions", "Показы", 1300, ptr(-1)),
		stage("product_views", "Просмотры карточки", 130, ptr(-2)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.Comparisons.ContextSignals) == 0 {
		t.Fatalf("context_signals empty")
	}
	got := strings.ToLower(ai.Comparisons.ContextSignals[0].Value)
	if !strings.Contains(got, "меньше 7 дней") {
		t.Fatalf("expected limited history signal, got %+v", ai.Comparisons.ContextSignals[0])
	}
	if strings.Contains(got, "история сравнения доступна") {
		t.Fatalf("unexpected available history text: %+v", ai.Comparisons.ContextSignals[0])
	}
}

func TestCheckDirectionFallbackFillsFields(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		CheckDirections: []FunnelTextItem{
			{Key: "card_quality", Label: "Проверка", Text: "Проверьте товары, где просмотры есть, а добавления в корзину снизились."},
		},
	}, []FunnelStage{
		stage("product_views", "Просмотры карточки", 100, ptr(-2)),
		stage("add_to_cart", "Добавления в корзину", 20, ptr(-10)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	d := ai.CheckDirections[0]
	if d.Priority == "" || d.DataNeeded == "" || d.Reason == "" {
		t.Fatalf("direction fields not filled: %+v", d)
	}
	if !strings.Contains(strings.ToLower(d.Text), "если sku-детализация") {
		t.Fatalf("text not softened: %q", d.Text)
	}
}

func TestLegacyVykuplennyeNormalizedWithoutBrokenWordsInDetails(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DetailsSections: []FunnelDetailsSection{
			{
				Title: "Итог выкупа",
				Items: []FunnelDetailsSectionItem{
					{
						Text:    "По выкупленным позициям и выкупленных заказах есть просадка.",
						Metrics: []string{"Доля выкупленных единиц"},
					},
				},
			},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DetailsSections) == 0 || len(ai.DetailsSections[0].Items) == 0 {
		t.Fatalf("details missing: %+v", ai.DetailsSections)
	}
	blob := strings.ToLower(ai.DetailsSections[0].Title + " " + ai.DetailsSections[0].Items[0].Text + " " + strings.Join(ai.DetailsSections[0].Items[0].Metrics, " "))
	for _, bad := range []string{"выкуп", "выкупленные", "доставкал"} {
		if strings.Contains(blob, bad) {
			t.Fatalf("unexpected token %q in %q", bad, blob)
		}
	}
	if !strings.Contains(blob, "доставлен") {
		t.Fatalf("expected readable delivered-form wording in %q", blob)
	}
}

func TestDropFactorsPreservedFromAI(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DropFactors: []FunnelDropFactor{
			{Key: "aov", Label: "Средний чек снизился", Metric: "1 156 ₽", Explanation: "Выручка просела сильнее заказов.", Tone: "negative"},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DropFactors) != 1 || ai.DropFactors[0].Label != "Средний чек снизился" {
		t.Fatalf("factors=%+v", ai.DropFactors)
	}
}

func TestDirectionsAreNotTopActions(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		CheckDirections: []FunnelTextItem{
			{Key: "a", Label: "Направление 1", Text: "Срочно остановите кампании с низкой эффективностью", Tone: "neutral"},
			{Key: "b", Label: "Диагностика", Text: "Сократите расход на рекламу до целевого уровня", Tone: "neutral"},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	for _, d := range ai.CheckDirections {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(d.Label)), "направление") {
			t.Fatalf("placeholder direction label: %q", d.Label)
		}
		lower := strings.ToLower(strings.TrimSpace(d.Text))
		for _, bad := range []string{"срочно ", "остановите ", "сократите ", "пополните ", "уберите "} {
			if strings.HasPrefix(lower, strings.TrimSpace(bad)) {
				t.Fatalf("imperative opener leaked: %q", d.Text)
			}
		}
	}
}

func TestFactorsRequireEvidence(t *testing.T) {
	items := []FunnelTextItem{
		{Key: "ad", Label: "Реклама", Text: "Высокий ДРР", Tone: "neutral"},
		{Key: "st", Label: "Остатки", Text: "Мало на складе", Tone: "neutral"},
		{Key: "cn", Label: "Отмены", Text: "Рост отмен", Tone: "neutral"},
		{Key: "ok", Label: "Трафик", Text: "Меньше показов", Tone: "neutral"},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{InfluenceFactors: items}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.InfluenceFactors) != 4 {
		t.Fatalf("got %+v", ai.InfluenceFactors)
	}
	evAll := FunnelEvidenceSnapshot{HasAdvertising: true, HasStock: true, HasCancellations: true}
	ai2 := applyFunnelAIGuards(FunnelAIAnalysisWidget{InfluenceFactors: items}, nil, evAll, nil, nil, nil, nil)
	if len(ai2.InfluenceFactors) != 4 {
		t.Fatalf("len=%d", len(ai2.InfluenceFactors))
	}
}
