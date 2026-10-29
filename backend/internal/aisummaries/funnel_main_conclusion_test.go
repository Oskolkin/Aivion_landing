package aisummaries

import (
	"strings"
	"testing"
)

const goodWithMarker = `Основная просадка к 16.06.2026 находится в верхней части воронки: показы снизились на 8,75%, а просмотры карточек — на 16,03%. Ключевое слабое место — переход «Показы → Просмотры карточки»: конверсия составила 14,36% и снизилась на 1,24 п.п. Нижние этапы не выглядят главной причиной проблемы. Но этой компенсации недостаточно. Что сделать: найти карточки с просадкой; разобрать рекламу; обновить фото и цену.`

const goodWithoutMarker = `Основная просадка находится на переходе «Показы → Просмотры карточки»: показы снизились на 8,75%, а просмотры карточек — на 16,03%. Конверсия перехода составила 14,36% и снизилась на 1,24 п.п. Нижние этапы не выглядят главной причиной, потому что переходы ниже не ухудшились. Это ограничивает поток покупателей в корзину и заказы. Дальше нужно выгрузить SKU с максимальной просадкой просмотров, разобрать кампании с показами без переходов и обновить первый экран карточек у проблемных товаров.`

const goodTwoActions = `Основная просадка на переходе «Карточка → Корзина»: конверсия 24,2% и снизилась на 4,81 п.п. Это ограничивает поток в заказы. Что сделать: проверить контент карточек с высоким трафиком; сравнить цену и скидку с конкурентами.`

const goodFourSentences = `Просадка на этапе «Показы → Просмотры карточки»: показы -8,75%, просмотры -16,03%. Нижние этапы держатся лучше. Это ограничивает поток в корзину. Нужно разобрать кампании и обновить фото у проблемных SKU.`

func TestFunnelMainConclusionQualityWeak_acceptsGoodAI(t *testing.T) {
	for name, text := range map[string]string{
		"with_marker":    goodWithMarker,
		"without_marker": goodWithoutMarker,
		"two_actions":    goodTwoActions,
		"four_sentences": goodFourSentences,
	} {
		if FunnelMainConclusionQualityWeak(text) {
			t.Fatalf("%s should pass quality check", name)
		}
		if FunnelMainConclusionSignalScore(text) < 3 {
			t.Fatalf("%s expected signal score >= 3, got %d", name, FunnelMainConclusionSignalScore(text))
		}
	}
}

func TestFunnelMainConclusionQualityWeak_rejectsWeakAI(t *testing.T) {
	weak := []string{
		"",
		"Просел верх воронки.",
		"Воронка просела, нужно проанализировать ситуацию.",
		"Главное ухудшение в верхней части воронки без конкретных цифр и без действий для продавца.",
		"Что сделать: проверьте трафик и карточки.",
		"Показы выросли, всё хорошо.",
	}
	for _, text := range weak {
		if !FunnelMainConclusionQualityWeak(text) {
			t.Fatalf("expected weak conclusion: %q", text)
		}
	}
}

func TestNormalizeFunnelMainConclusionText_deltaPP(t *testing.T) {
	in := "Конверсия составила 14,36% и изменилась на -1,24 п.п. к прошлой сводке."
	out, reason := NormalizeFunnelMainConclusionText(in)
	if reason != "delta_pp_normalized" {
		t.Fatalf("expected delta repair, got %q", reason)
	}
	if strings.Contains(out, "на -") {
		t.Fatalf("expected normalized delta, got %q", out)
	}
	if !strings.Contains(out, "снизилась на 1,24 п.п.") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestResolveFunnelMainConclusion_keepsGoodAI(t *testing.T) {
	funnel := &FunnelAIAnalysis{
		Status:         "available",
		MainConclusion: goodWithoutMarker,
	}
	res := ResolveFunnelMainConclusion(funnel)
	if res.Source != "ai" {
		t.Fatalf("expected ai source, got %q", res.Source)
	}
	if res.Source == "fallback" {
		t.Fatal("good AI must not be replaced by fallback")
	}
	if !strings.Contains(res.Text, "выгрузить SKU") {
		t.Fatalf("expected preserved AI text, got %q", res.Text)
	}
}

func TestResolveFunnelMainConclusion_repairsNotFallback(t *testing.T) {
	funnel := &FunnelAIAnalysis{
		Status:         "available",
		MainConclusion: "Конверсия перехода «Показы → Просмотры карточки» составила 14,36% и изменилась на -1,24 п.п. Нижние этапы не выглядят главной причиной. Это ограничивает поток в корзину. Нужно разобрать кампании и обновить фото.",
	}
	res := ResolveFunnelMainConclusion(funnel)
	if res.Source == "fallback" {
		t.Fatal("repairable AI should not go to fallback")
	}
	if res.Source != "ai" && res.Source != "ai_repaired" {
		t.Fatalf("unexpected source %q", res.Source)
	}
}

func TestResolveFunnelMainConclusion_fallbackWhenGeneric(t *testing.T) {
	funnel := &FunnelAIAnalysis{
		Status:         "available",
		MainConclusion: "Воронка просела, нужно проанализировать ситуацию.",
		WeakestStage: &FunnelWeakestStage{
			Key:   "impressions_to_product_views",
			Label: "Показы → Просмотры карточки",
			Value: "14,36%",
		},
		Comparisons: &FunnelComparisons{
			StageMetrics: []FunnelComparisonItem{
				{Key: "impressions", Label: "Показы", Delta: "-8,75%"},
			},
		},
	}
	res := ResolveFunnelMainConclusion(funnel)
	if res.Source != "fallback" {
		t.Fatalf("expected fallback for generic text, got %q (%q)", res.Source, res.Text)
	}
}

func TestBuildFunnelMainConclusionFallback_upperFunnel(t *testing.T) {
	built := BuildFunnelMainConclusionFallback(&FunnelAIAnalysis{
		Status: "available",
		WeakestStage: &FunnelWeakestStage{
			Key:           "impressions_to_product_views",
			Label:         "Показы → Просмотры карточки",
			Value:         "14,36%",
			DeltaPrevious: "-1,24 п.п.",
		},
		Comparisons: &FunnelComparisons{
			StageMetrics: []FunnelComparisonItem{
				{Key: "impressions", Label: "Показы", Delta: "-8,75%"},
				{Key: "product_views", Label: "Просмотры карточки", Delta: "-16,03%"},
			},
			ConversionMetrics: []FunnelComparisonItem{
				{Key: "product_views_to_add_to_cart", Label: "Карточка → Корзина", Delta: "+0,5 п.п."},
			},
		},
	})
	lower := strings.ToLower(built)
	for _, want := range []string{
		"показы → просмотры карточки",
		"8,75%",
		"что сделать:",
	} {
		if !strings.Contains(lower, want) {
			t.Fatalf("fallback missing %q in %q", want, built)
		}
	}
}

func TestRepairFunnelMainConclusion_fillsEmptyWeakText(t *testing.T) {
	funnel := &FunnelAIAnalysis{
		Status: "available",
		WeakestStage: &FunnelWeakestStage{
			Status: "available",
			Key:    "impressions_to_product_views",
			Label:  "Показы → Просмотры карточки",
			Value:  "14,36%",
		},
		ComparisonsCommentary: &FunnelComparisonsCommentary{
			PreviousPeriod: "Просел верх воронки.",
		},
		Comparisons: &FunnelComparisons{
			StageMetrics: []FunnelComparisonItem{
				{Key: "impressions", Label: "Показы", Delta: "-8,75%"},
			},
		},
	}
	result := repairFunnelMainConclusion(funnel)
	if !result.Repaired {
		t.Fatalf("expected repair, got %+v", result)
	}
	if !strings.Contains(strings.Join(result.RepairActions, ","), "funnel_main_conclusion_fallback:") {
		t.Fatalf("expected fallback audit action, got %+v", result.RepairActions)
	}
	if FunnelMainConclusionQualityWeak(funnel.MainConclusion) {
		t.Fatalf("repaired conclusion still weak: %q", funnel.MainConclusion)
	}
}

func TestRepairFunnelMainConclusion_preservesGoodAI(t *testing.T) {
	original := goodWithoutMarker
	funnel := &FunnelAIAnalysis{
		Status:         "available",
		MainConclusion: original,
	}
	result := repairFunnelMainConclusion(funnel)
	if funnel.MainConclusion != original {
		t.Fatalf("good AI replaced: %q", funnel.MainConclusion)
	}
	if strings.Contains(strings.Join(result.RepairActions, ","), "funnel_main_conclusion_fallback:") {
		t.Fatalf("good AI should not trigger fallback, actions=%v", result.RepairActions)
	}
}
