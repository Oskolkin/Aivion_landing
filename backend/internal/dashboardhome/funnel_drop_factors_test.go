package dashboardhome

import "testing"

func TestFilterDropFactorsByEvidence_downgradesWhenSignalsMissing(t *testing.T) {
	in := []FunnelDropFactor{
		{
			Key:           "ad",
			Label:         "Слабая рекламная отдача",
			EvidenceKey:   "advertising",
			EvidenceLevel: "confirmed",
		},
	}
	out := filterDropFactorsByEvidence(in, FunnelEvidenceSnapshot{})
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].EvidenceLevel != "hypothesis" && out[0].EvidenceLevel != "limited" {
		t.Fatalf("unexpected evidence_level=%q", out[0].EvidenceLevel)
	}
	if out[0].Limitation == "" {
		t.Fatal("expected limitation for weak evidence")
	}
}

func TestClassifyDropFactorGroupByKey(t *testing.T) {
	cases := map[string]string{
		"views_decline":      DropFactorGroupFunnelSignal,
		"cart_add_decline":   DropFactorGroupFunnelSignal,
		"aov_drop":           DropFactorGroupBusinessSignal,
		"revenue_drop":       DropFactorGroupBusinessSignal,
		"stock_pressure":     DropFactorGroupPossibleAmplifier,
		"inefficient_ads":    DropFactorGroupContextHypothesis,
		"unknown_factor_xyz": DropFactorGroupContextHypothesis,
	}
	for key, want := range cases {
		if got := classifyDropFactorGroupByKey(key); got != want {
			t.Fatalf("key=%q got=%q want=%q", key, got, want)
		}
	}
}

func TestEnrichDropFactorOverridesWrongAIGroup(t *testing.T) {
	f := enrichDropFactorFallback(FunnelDropFactor{
		Key:   "inefficient_ads",
		Group: "funnel_signal",
	}, nil)
	if f.Group != DropFactorGroupContextHypothesis {
		t.Fatalf("group=%q", f.Group)
	}
	if f.EvidenceLevel != "hypothesis" {
		t.Fatalf("evidence=%q", f.EvidenceLevel)
	}
}

func TestEnrichDropFactorAovDropFinancialLimitation(t *testing.T) {
	f := enrichDropFactorFallback(FunnelDropFactor{Key: "aov_drop", EvidenceLevel: "likely"}, nil)
	if f.Group != DropFactorGroupBusinessSignal {
		t.Fatalf("group=%q", f.Group)
	}
	if f.EvidenceLevel != "confirmed" {
		t.Fatalf("evidence=%q", f.EvidenceLevel)
	}
	if f.Limitation == "" {
		t.Fatal("expected financial limitation")
	}
}

func TestEnrichDropFactorViewsDeclineLikelyWithLimitation(t *testing.T) {
	f := enrichDropFactorFallback(FunnelDropFactor{Key: "views_decline", EvidenceLevel: "confirmed"}, nil)
	if f.Group != DropFactorGroupFunnelSignal {
		t.Fatalf("group=%q", f.Group)
	}
	if f.EvidenceLevel != "likely" {
		t.Fatalf("evidence=%q", f.EvidenceLevel)
	}
	if f.Limitation == "" {
		t.Fatal("expected views limitation")
	}
}

func TestApplyFunnelAIGuards_sortsDropFactorsByGroup(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DropFactors: []FunnelDropFactor{
			{Key: "inefficient_ads", Label: "Реклама"},
			{Key: "views_decline", Label: "Просмотры"},
			{Key: "aov_drop", Label: "Средний чек"},
			{Key: "cart_add_decline", Label: "Корзина"},
		},
	}, nil, FunnelEvidenceSnapshot{HasStock: true, HasAdvertising: true}, nil, nil, nil, nil)
	if len(ai.DropFactors) != 4 {
		t.Fatalf("drop_factors=%+v", ai.DropFactors)
	}
	if ai.DropFactors[0].Key != "views_decline" || ai.DropFactors[1].Key != "cart_add_decline" {
		t.Fatalf("expected funnel signals first, got %+v", ai.DropFactors[:2])
	}
	if ai.DropFactors[2].Key != "aov_drop" {
		t.Fatalf("expected business third, got %q", ai.DropFactors[2].Key)
	}
}

func TestDropFactorFallbackAddsDeltaForCartAddDecline(t *testing.T) {
	v1 := int64(100)
	v2 := int64(30)
	p1 := int64(120)
	p2 := int64(60)
	d1 := -16.67
	d2 := -50.0
	stages := []FunnelStage{
		{Key: "product_views", Label: "Просмотры карточки", Value: &v1, PreviousDayValue: &p1, ChangeVsPreviousDayPct: &d1},
		{Key: "add_to_cart", Label: "Добавления в корзину", Value: &v2, PreviousDayValue: &p2, ChangeVsPreviousDayPct: &d2},
	}
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DropFactors: []FunnelDropFactor{{Key: "cart_add_decline", Label: "Падение корзины", EvidenceLevel: "likely"}},
	}, stages, FunnelEvidenceSnapshot{HasStock: true, HasAdvertising: true}, nil, nil, nil, nil)
	if len(ai.DropFactors) != 1 || ai.DropFactors[0].Delta == "" {
		t.Fatalf("drop_factors=%+v", ai.DropFactors)
	}
	if ai.DropFactors[0].Group != DropFactorGroupFunnelSignal {
		t.Fatalf("group=%q", ai.DropFactors[0].Group)
	}
}

func TestDropFactorFallbackAddsLimitationsForStockAndAds(t *testing.T) {
	ai := applyFunnelAIGuards(FunnelAIAnalysisWidget{
		DropFactors: []FunnelDropFactor{
			{Key: "stock_pressure", Label: "Остатки"},
			{Key: "inefficient_ads", Label: "Реклама"},
		},
	}, nil, FunnelEvidenceSnapshot{}, nil, nil, nil, nil)
	if len(ai.DropFactors) != 2 {
		t.Fatalf("drop_factors=%+v", ai.DropFactors)
	}
	if ai.DropFactors[0].Limitation == "" || ai.DropFactors[0].Group != DropFactorGroupPossibleAmplifier {
		t.Fatalf("stock unexpected: %+v", ai.DropFactors[0])
	}
	if ai.DropFactors[1].Limitation == "" || ai.DropFactors[1].EvidenceLevel == "confirmed" || ai.DropFactors[1].EvidenceLevel == "likely" {
		t.Fatalf("ads fallback unexpected: %+v", ai.DropFactors[1])
	}
	if ai.DropFactors[1].Group != DropFactorGroupContextHypothesis {
		t.Fatalf("ads group=%q", ai.DropFactors[1].Group)
	}
}
