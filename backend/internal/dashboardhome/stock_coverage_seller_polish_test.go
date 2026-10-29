package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestPolishRussianArticulAndTovarCounts(t *testing.T) {
	cases := []struct{ in, out string }{
		{"6 артикул в зоне", "6 артикулов в зоне"},
		{"5 артикул при", "5 артикулов при"},
		{"1 артикул и", "1 артикул и"},
		{"2 артикул дня", "2 артикула дня"},
		{"69 артикул имеют", "69 артикулов имеют"},
		{"80 артикул.", "80 артикулов."},
		{"1 товаров без", "1 товар без"},
		{"11 товар в критике", "11 товаров в критике"},
	}
	for _, c := range cases {
		got := polishRussianArticulAndTovarCounts(c.in)
		if got != c.out {
			t.Fatalf("polishRussianArticulAndTovarCounts(%q) = %q want %q", c.in, got, c.out)
		}
	}
}

func TestReplaceArticulFormsWithSKU(t *testing.T) {
	in := "1 артикул, 2 артикула, 5 артикулов, к артикулу, с артикулом, об артикуле, артикулами, в артикулах"
	out := replaceArticulFormsWithSKU(in)
	if strings.Contains(strings.ToLower(out), "артикул") {
		t.Fatalf("expected all articul forms replaced, got %q", out)
	}
	if !strings.Contains(out, "SKU") {
		t.Fatalf("expected SKU in text, got %q", out)
	}
}

func TestPolishStockCoverageReplacesTechnicalMethodNote(t *testing.T) {
	n := &StockCoverageNarrative{
		Status:      "available",
		Summary:     "Кратко.",
		RiskComment: "Риск.",
		RevenueAtRisk: StockCoverageNarrativeRevenue{
			Status:     "available",
			Value:      "1 ₽/день",
			MethodNote: "Оценка рассчитана по weighted_avg_daily_revenue_7d.",
		},
		Segments:         nil,
		SKUFocusLink:     StockCoverageSKUFocusLink{Label: "Перейти к артикул с риском по остаткам", Target: stockSKUFocusTarget},
		NoSalesWithStock: StockCoverageNoSalesWithStock{Status: "unavailable"},
	}
	polishStockCoverageNarrativeForSeller(n)
	if strings.Contains(n.RevenueAtRisk.MethodNote, "weighted_avg") {
		t.Fatalf("method_note still technical: %q", n.RevenueAtRisk.MethodNote)
	}
	if n.RevenueAtRisk.MethodNote != sellerFacingRevenueMethodNote {
		t.Fatalf("method_note = %q want seller constant", n.RevenueAtRisk.MethodNote)
	}
	if n.SKUFocusLink.Label != defaultSKUFocusLinkLabel {
		t.Fatalf("sku label = %q", n.SKUFocusLink.Label)
	}
}

func TestNormalizeStockCoverageSummaryLanguageDoesNotPrependTemplate(t *testing.T) {
	in := "Основной риск связан с коротким покрытием."
	out := normalizeStockCoverageSummaryLanguage(in)
	if strings.Contains(out, "Покрытие показывает, на сколько дней") {
		t.Fatalf("should not prepend coverage template: %q", out)
	}
}

func TestPolishStockCoverageNormalizesLegacySKUFocusLabel(t *testing.T) {
	n := &StockCoverageNarrative{
		Status:           "available",
		Summary:          "Кратко.",
		SKUFocusLink:     StockCoverageSKUFocusLink{Label: "Смотреть товары в SKU-фокусе", Target: stockSKUFocusTarget},
		NoSalesWithStock: StockCoverageNoSalesWithStock{Status: "unavailable"},
		RevenueAtRisk:    StockCoverageNarrativeRevenue{Status: "unavailable"},
	}
	polishStockCoverageNarrativeForSeller(n)
	if n.SKUFocusLink.Label != defaultSKUFocusLinkLabel {
		t.Fatalf("expected default sku link label, got %q", n.SKUFocusLink.Label)
	}
}

func TestFactualNarrativeSKUFocusAndMethodNote(t *testing.T) {
	d0 := 1.0
	d20 := 20.0
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, ProductName: ptrS("A"), StockAvailable: 2, DaysOfCover: &d0, RevenueLast7Days: 700, OrdersLast7Days: 1, ActiveSalesDays7D: 7},
		{OzonProductID: 2, ProductName: ptrS("Idle"), StockAvailable: 51, DaysOfCover: &d20, RevenueLast7Days: 0, OrdersLast7Days: 0, ActiveSalesDays7D: 0},
	}
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			StockCoverage: StockCoverageWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Segments: []StockCoverageSegment{
					{Key: "critical_0_3", Label: "0–3 дня", SKUCount: 1, Severity: "critical"},
					{Key: "shortage_4_7", Label: "4–7 дней", SKUCount: 1, Severity: "high"},
					{Key: "planning_8_14", Label: "8–14 дней", SKUCount: 0, Severity: "medium"},
					{Key: "healthy_14plus", Label: ">14 дней", SKUCount: 0, Severity: "low"},
					{Key: "surplus", Label: "Излишки / хранение", SKUCount: 0, Severity: "high"},
				},
				TotalSKUs: 2,
			},
		},
	}
	n := BuildStockCoverageNarrativeFallbackExported(analytics.DashboardMetricsDTO{SKUs: skus}, resp, nil)
	polishStockCoverageNarrativeForSeller(n)
	if n == nil {
		t.Fatal("nil narrative")
	}
	if n.SKUFocusLink.Target != stockSKUFocusTarget || n.SKUFocusLink.Label != defaultSKUFocusLinkLabel {
		t.Fatalf("sku link: %+v", n.SKUFocusLink)
	}
	if n.RevenueAtRisk.Status == "available" && strings.Contains(n.RevenueAtRisk.MethodNote, "weighted_avg") {
		t.Fatalf("method: %q", n.RevenueAtRisk.MethodNote)
	}
	if n.NoSalesWithStock.Status != "available" || n.NoSalesWithStock.Count != 1 {
		t.Fatalf("no_sales: %+v", n.NoSalesWithStock)
	}
	if strings.TrimSpace(n.Summary) == "" || strings.TrimSpace(n.Headline) == "" {
		t.Fatalf("factual narrative should include seller-facing headline/summary: headline=%q summary=%q", n.Headline, n.Summary)
	}
	if strings.TrimSpace(n.NoSalesWithStock.Comment) != "" {
		t.Fatalf("factual narrative should not include no_sales comment: %q", n.NoSalesWithStock.Comment)
	}
	if n.NoSalesFocusLink == nil || n.NoSalesFocusLink.Target != stockNoSalesSKUFocusTarget {
		t.Fatalf("expected no_sales focus link: %+v", n.NoSalesFocusLink)
	}
}
