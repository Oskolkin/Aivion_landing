package dashboardhome

import (
	"strings"
	"testing"
)

func TestAIPrimarySourceForFunnelAnalysis(t *testing.T) {
	aiLabel := "Уникальный AI фактор просадки XYZ"
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		DropFactors: []FunnelDropFactor{
			{Key: "custom", Label: aiLabel, Metric: "1 156 ₽ против 1 409 ₽", Explanation: "Выручка просела сильнее заказов.", Tone: "negative"},
		},
		CheckDirections: []FunnelTextItem{
			{Key: "x", Label: "SKU и категории", Text: "Сопоставить просадку по SKU без императива.", Tone: "neutral"},
		},
		WeakestStage: FunnelWeakestStage{
			Status: "available", Key: "add_to_cart", Label: "Переход в корзину", Metric: "Корзина",
			Explanation: "AI explanation weakest XYZ", DeltaPrevious: "-10%",
		},
		RelatedSignals: []string{"Сигнал который не должен стать фактором"},
	}, []FunnelStage{
		stage("add_to_cart", "Добавления в корзину", 129, ptr(-27.9)),
	}, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)

	if len(ai.DropFactors) != 1 || ai.DropFactors[0].Label != aiLabel {
		t.Fatalf("drop factors replaced: %+v", ai.DropFactors)
	}
	if !strings.Contains(ai.WeakestStage.Explanation, "AI explanation weakest") {
		t.Fatalf("weakest explanation overwritten: %q", ai.WeakestStage.Explanation)
	}
	if strings.Contains(ai.DropFactors[0].Label, "signal_") {
		t.Fatal("backend synthesized factors from signals")
	}
}

func TestDirectionsAreStructuredAndNotImperative(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		CheckDirections: []FunnelTextItem{
			{Key: "a", Label: "Направление 1", Text: "Срочно остановите кампании", Tone: "neutral"},
			{Key: "b", Label: "Проверить остатки", Text: "Проверить доступность товаров", Tone: "neutral"},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	for _, d := range ai.CheckDirections {
		l := strings.ToLower(strings.TrimSpace(d.Label))
		tx := strings.ToLower(strings.TrimSpace(d.Text))
		if strings.HasPrefix(l, "направление") {
			t.Fatalf("placeholder label: %q", d.Label)
		}
		for _, bad := range []string{"срочно ", "проверить ", "разобрать ", "сверить "} {
			if strings.HasPrefix(tx, strings.TrimSpace(bad)) {
				t.Fatalf("imperative text: %q", d.Text)
			}
		}
		if strings.TrimSpace(d.Label) == "" || strings.TrimSpace(d.Text) == "" {
			t.Fatalf("missing label/text: %+v", d)
		}
	}
}

func TestDropFactorsRequireEvidence(t *testing.T) {
	items := []FunnelDropFactor{
		{Key: "ad", Label: "Реклама", Explanation: "Высокий ДРР", EvidenceKey: "advertising", EvidenceLevel: "confirmed"},
		{Key: "ok", Label: "Трафик", Explanation: "Меньше показов"},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{DropFactors: items}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DropFactors) != 2 {
		t.Fatalf("got %+v", ai.DropFactors)
	}
	if ai.DropFactors[0].EvidenceLevel != "hypothesis" && ai.DropFactors[0].EvidenceLevel != "limited" {
		t.Fatalf("expected downgraded evidence level, got %+v", ai.DropFactors[0])
	}
}

func TestDetailsSectionsValid(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DetailsSections: []FunnelDetailsSection{
			{Title: "Где просадка", Items: []FunnelDetailsSectionItem{
				{Text: "Корзина просела.", Metrics: []string{"−12%"}},
			}},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DetailsSections) != 1 || ai.DetailsSections[0].Title == "" {
		t.Fatalf("sections=%+v", ai.DetailsSections)
	}
}
