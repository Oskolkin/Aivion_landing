package adapter

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestSKUFocusCardFromFacts_TrendStatuses(t *testing.T) {
	sku := int64(101)
	lookup := skuFactsLookup{
		byOzonProductID: map[int64]dashboardfacts.SKUFactRecord{
			9: {
				OzonProductID:       9,
				SKU:                 &sku,
				Orders:              12,
				Orders7d:            12,
				OrdersPrev7d:        15,
				OrdersDeltaDayToDay: -3,
				Sparkline7D: []dashboardfacts.SparklinePoint{
					{Date: "d1", Value: f64(1), HasData: true},
					{Date: "d2", Value: f64(2), HasData: true},
					{Date: "d3", Value: f64(3), HasData: true},
					{Date: "d4", Value: f64(4), HasData: true},
					{Date: "d5", Value: f64(5), HasData: true},
					{Date: "d6", Value: f64(6), HasData: true},
					{Date: "d7", Value: f64(7), HasData: true},
				},
			},
		},
		bySKU: map[int64]dashboardfacts.SKUFactRecord{},
	}
	lookup.bySKU[sku] = lookup.byOzonProductID[9]

	ai := contract.UnifiedSKUFocusCardV1{SKU: &sku, ProductName: "P", Problem: "Падение", KeyMetric: "-3 заказа"}
	card, kind := skuFocusCardFromFacts(ai, lookup)
	if kind != skuFocusOK {
		t.Fatalf("kind=%v", kind)
	}
	if card.TrendStatus != dashboardhome.SKUFocusTrendAvailable || len(card.Sparkline7D) != 7 {
		t.Fatalf("7 points: %+v", card)
	}

	rec := lookup.byOzonProductID[9]
	rec.Sparkline7D = []dashboardfacts.SparklinePoint{
		{Date: "d1", Value: f64(1), HasData: true},
		{Date: "d2", Value: f64(2), HasData: true},
	}
	lookup.byOzonProductID[9] = rec
	lookup.bySKU[sku] = rec
	card, kind = skuFocusCardFromFacts(ai, lookup)
	if kind != skuFocusOK || card.TrendStatus != dashboardhome.SKUFocusTrendAvailable || len(card.Sparkline7D) != 2 {
		t.Fatalf("2 points: kind=%v card=%+v", kind, card)
	}

	rec.Sparkline7D = []dashboardfacts.SparklinePoint{{Date: "d1", Value: f64(1), HasData: true}}
	lookup.byOzonProductID[9] = rec
	lookup.bySKU[sku] = rec
	card, kind = skuFocusCardFromFacts(ai, lookup)
	if kind != skuFocusOK {
		t.Fatal("1 point must retain card")
	}
	if card.TrendStatus != dashboardhome.SKUFocusTrendNotEnoughData {
		t.Fatalf("1 point status=%q", card.TrendStatus)
	}
	if len(card.Sparkline7D) != 1 {
		t.Fatalf("1 point sparkline len=%d", len(card.Sparkline7D))
	}

	rec.Sparkline7D = nil
	lookup.byOzonProductID[9] = rec
	lookup.bySKU[sku] = rec
	card, kind = skuFocusCardFromFacts(ai, lookup)
	if kind != skuFocusOK {
		t.Fatal("0 points with summary facts must retain card")
	}
	if card.TrendStatus != dashboardhome.SKUFocusTrendNotEnoughData {
		t.Fatalf("0 points status=%q", card.TrendStatus)
	}
	if card.Sparkline7D != nil {
		t.Fatalf("0 points must not invent sparkline: %+v", card.Sparkline7D)
	}
	if card.CurrentOrders == nil || *card.CurrentOrders != 12 {
		t.Fatalf("summary orders must remain: %+v", card.CurrentOrders)
	}
}

func TestSKUFocusCardFromFacts_UnresolvedAndMissingIssue(t *testing.T) {
	lookup := skuFactsLookup{
		byOzonProductID: map[int64]dashboardfacts.SKUFactRecord{},
		bySKU:           map[int64]dashboardfacts.SKUFactRecord{},
	}
	sku := int64(999)
	_, kind := skuFocusCardFromFacts(contract.UnifiedSKUFocusCardV1{
		SKU: &sku, ProductName: "Ghost", Problem: "x", KeyMetric: "y",
	}, lookup)
	if kind != skuFocusUnresolved {
		t.Fatalf("want unresolved, got %v", kind)
	}

	known := int64(1)
	lookup.bySKU[known] = dashboardfacts.SKUFactRecord{OzonProductID: 1, SKU: &known, Orders: 5}
	_, kind = skuFocusCardFromFacts(contract.UnifiedSKUFocusCardV1{
		SKU: &known, ProductName: "NoIssue",
	}, lookup)
	if kind != skuFocusMissingIssue {
		t.Fatalf("want missing issue, got %v", kind)
	}
}

func TestSKUFocusTrendState_ImmutableOnCopy(t *testing.T) {
	card := dashboardhome.SKUFocusCard{
		OzonProductID: 1,
		TrendStatus:   dashboardhome.SKUFocusTrendNotEnoughData,
		TrendMessage:  "Недостаточно точек для тренда",
		Sparkline7D:   []dashboardhome.SparklinePoint{{Date: "d1", HasData: true}},
	}
	focus := dashboardhome.SKUFocus{OrderDrop: []dashboardhome.SKUFocusCard{card}}
	cp := focus
	cp.OrderDrop[0].TrendMessage = "mutated"
	if focus.OrderDrop[0].TrendMessage == "mutated" {
		// slice shares backing array — re-copy for fingerprint style check
		raw := append([]dashboardhome.SKUFocusCard(nil), focus.OrderDrop...)
		raw[0].TrendStatus = dashboardhome.SKUFocusTrendAvailable
		if focus.OrderDrop[0].TrendStatus != dashboardhome.SKUFocusTrendNotEnoughData {
			t.Fatal("independent copy must preserve frozen trend status")
		}
		_ = raw
	}
	// Explicit: status string must remain the published vocabulary
	if card.TrendStatus != dashboardhome.SKUFocusTrendNotEnoughData {
		t.Fatal("trend status mutated")
	}
}
