package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestCalculateRevenueAtRiskOnlyForDemandSKUs(t *testing.T) {
	d0 := 2.0
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, ProductName: ptrS("A"), StockAvailable: 1, DaysOfCover: &d0, RevenueLast7Days: 700, OrdersLast7Days: 0, ActiveSalesDays7D: 7},
		{OzonProductID: 2, ProductName: ptrS("B"), StockAvailable: 5, DaysOfCover: &d0, RevenueLast7Days: 0, OrdersLast7Days: 0, ActiveSalesDays7D: 0},
	}
	rev := computeStockCoverageRevenueAtRisk(skus)
	if rev.Status != "available" || rev.TotalDaily <= 0 {
		t.Fatalf("expected available positive revenue, got %+v", rev)
	}
}

func TestRevenueAtRiskExcludesNoSalesStock(t *testing.T) {
	d10 := 10.0
	skus := []analytics.DashboardSKUMetricsItem{
		{OzonProductID: 1, ProductName: ptrS("Idle"), StockAvailable: 100, DaysOfCover: &d10, RevenueLast7Days: 0, OrdersLast7Days: 0, ActiveSalesDays7D: 0},
	}
	rev := computeStockCoverageRevenueAtRisk(skus)
	if rev.Status != "unavailable" {
		t.Fatalf("expected unavailable, got %s", rev.Status)
	}
}

func TestSKUFocusLinkTargetIsValid(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.SKUFocusLink.Target = "wrong"
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	if errs := validateStockCoverageNarrativePayload(n, ctx); len(errs) == 0 {
		t.Fatal("expected bad target error")
	}
}

func TestAdsMentionWithoutEvidenceFailsValidation(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.Summary = "Есть связь с рекламой по трафику и коротким покрытием по SKU с подтверждённым спросом."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	if errs := validateStockCoverageNarrativePayload(n, ctx); len(errs) == 0 {
		t.Fatal("expected ads evidence error")
	}
}

func TestStockCoverageSummaryDoesNotContainProductNames(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{
		SKUs: []analytics.DashboardSKUMetricsItem{
			{OzonProductID: 1, ProductName: ptrS("Светильник напольный торшер"), StockAvailable: 2, RevenueLast7Days: 700, OrdersLast7Days: 1, ActiveSalesDays7D: 7},
		},
	}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.Summary = "Светильник напольный торшер находится в риске по короткому покрытию."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	errs := validateStockCoverageNarrativePayload(n, ctx)
	for _, err := range errs {
		if err == "stock_narrative_summary_mentions_product_names" {
			return
		}
	}
	t.Fatalf("expected product-name validation error, got %v", errs)
}

func TestStockCoverageSummaryIncludesNoSalesWhenAvailable(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{
		SKUs: []analytics.DashboardSKUMetricsItem{
			{OzonProductID: 2, ProductName: ptrS("Idle"), StockAvailable: 51, RevenueLast7Days: 0, OrdersLast7Days: 0, ActiveSalesDays7D: 0},
		},
	}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.NoSalesWithStock = StockCoverageNoSalesWithStock{Status: "available", Count: 1, StockUnits: 51}
	n.Summary = "Основной риск связан с коротким покрытием по SKU с подтверждённым спросом за последнюю неделю."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	if errs := validateStockCoverageNarrativePayload(n, ctx); len(errs) == 0 {
		t.Fatal("expected missing no_sales signal error")
	}
	n.NoSalesInsight = "Есть сигнал по остатку без продаж за последние 7 дней."
	if errs := validateStockCoverageNarrativePayload(n, ctx); len(errs) != 0 {
		t.Fatalf("expected no errors after no-sales insight, got %v", errs)
	}
}

func TestStockCoverageSummaryNoActionVerbs(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.Summary = "Проверьте остатки и пополните склад по SKU с коротким покрытием."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	errs := validateStockCoverageNarrativePayload(n, ctx)
	for _, err := range errs {
		if err == "stock_narrative_forbidden_action_verb" {
			return
		}
	}
	t.Fatalf("expected forbidden action verb error, got %v", errs)
}

func TestFactualNarrativeDoesNotGenerateInterpretation(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{
		SKUs: []analytics.DashboardSKUMetricsItem{
			{OzonProductID: 1, ProductName: ptrS("Idle"), StockAvailable: 51, RevenueLast7Days: 0, OrdersLast7Days: 0, ActiveSalesDays7D: 0},
		},
	}
	n := buildStockCoverageFactualNarrative(dto, resp, nil)
	if n == nil {
		t.Fatal("expected factual narrative")
	}
	if strings.TrimSpace(n.Headline) == "" {
		t.Fatal("expected factual headline")
	}
	if strings.TrimSpace(n.Summary) == "" {
		t.Fatal("expected factual summary")
	}
	if strings.TrimSpace(n.RiskComment) != "" {
		t.Fatalf("factual narrative must not include risk_comment: %q", n.RiskComment)
	}
	if len(n.Actions) > 0 {
		t.Fatalf("factual narrative must not include actions: %+v", n.Actions)
	}
	if n.NoSalesWithStock.Count != 1 || strings.TrimSpace(n.NoSalesWithStock.Comment) != "" {
		t.Fatalf("no_sales factual only: %+v", n.NoSalesWithStock)
	}
}

func TestStockCoverageSummaryUsesSKU(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.Summary = "Покрытие показывает, что 6 артикул в зоне риска по короткому покрытию."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	polishStockCoverageNarrativeForSeller(n)
	if strings.Contains(strings.ToLower(n.Summary), "артикул") {
		t.Fatalf("expected seller summary with SKU, got %q", n.Summary)
	}
}

func TestStockCoverageSummaryDoesNotUseInternalZoneTerm(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := minimalValidStockNarrative(resp, dto, nil)
	n.Summary = "Основной риск: 6 SKU в зоне 0–3 дня с подтверждённым спросом."
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, nil)
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	errs := validateStockCoverageNarrativePayload(n, ctx)
	found := false
	for _, e := range errs {
		if e == "stock_narrative_summary_uses_internal_zone_term" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected internal-zone-term error, got %v", errs)
	}
}

func TestStockCoverageSellerFacingTextsUseSKU(t *testing.T) {
	n := &StockCoverageNarrative{
		Status:           "available",
		Summary:          "6 артикул в зоне риска по короткому покрытию.",
		RiskComment:      "5 артикул в коротком покрытии.",
		NoSalesWithStock: StockCoverageNoSalesWithStock{Status: "available", Comment: "1 артикул без продаж."},
		SKUFocusLink:     StockCoverageSKUFocusLink{Label: "Перейти к артикул с риском по остаткам", Target: stockSKUFocusTarget},
	}
	polishStockCoverageNarrativeForSeller(n)
	all := strings.ToLower(n.Summary + " " + n.RiskComment + " " + n.NoSalesWithStock.Comment + " " + n.SKUFocusLink.Label)
	if strings.Contains(all, "артикул") {
		t.Fatalf("expected SKU-only seller text, got %q", all)
	}
	if n.SKUFocusLink.Label != defaultSKUFocusLinkLabel {
		t.Fatalf("unexpected sku link label: %q", n.SKUFocusLink.Label)
	}
}

func TestStockCoverageGenericSummaryNotSubstantive(t *testing.T) {
	if isStockCoverageSummarySubstantive("") {
		t.Fatal("empty summary should not be substantive")
	}
	if isStockCoverageSummarySubstantive("Покрытие показывает, на сколько дней хватит текущего запаса.") {
		t.Fatal("template summary should not be substantive")
	}
	if !isStockCoverageSummarySubstantive("Основной риск сосредоточен в коротком покрытии по SKU с подтверждённым спросом.") {
		t.Fatal("expected substantive AI summary")
	}
}

func TestStockCoverageBadActionDroppedNotFatal(t *testing.T) {
	n := &StockCoverageNarrative{
		Status:  "available",
		Summary: "Основной риск сосредоточен в коротком покрытии по SKU с подтверждённым спросом.",
		Actions: []StockCoverageAction{
			{Title: "", Text: ""},
			{Title: "что сделать", Text: ""},
			{Title: "Проверить критичные SKU", Text: "Проверьте товары с покрытием 0–3 дня."},
		},
	}
	repairStockCoverageNarrativeActions(n)
	if len(n.Actions) != 1 || n.Actions[0].Title != "Проверить критичные SKU" {
		t.Fatalf("unexpected repaired actions: %+v", n.Actions)
	}
}

func TestFactualCountsIndependentOfAI(t *testing.T) {
	resp := baseStockResp()
	dto := analytics.DashboardMetricsDTO{SKUs: []analytics.DashboardSKUMetricsItem{}}
	n := buildStockCoverageFactualNarrative(dto, resp, nil)
	if n == nil || len(n.Segments) == 0 || n.Segments[0].Count != 1 {
		t.Fatalf("unexpected factual segments: %+v", n)
	}
}

func ptrS(s string) *string { return &s }

func baseStockResp() *Response {
	return &Response{
		AIDiagnostics: AIDiagnostics{
			StockCoverage: StockCoverageWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Segments: []StockCoverageSegment{
					{Key: "critical_0_3", Label: "0–3 дня", SKUCount: 1, Severity: "critical"},
					{Key: "shortage_4_7", Label: "4–7 дней", SKUCount: 0, Severity: "high"},
					{Key: "planning_8_14", Label: "8–14 дней", SKUCount: 0, Severity: "medium"},
					{Key: "healthy_14plus", Label: ">14 дней", SKUCount: 0, Severity: "low"},
					{Key: "surplus", Label: "Излишки / хранение", SKUCount: 0, Severity: "high"},
				},
				TotalSKUs: 1,
			},
		},
	}
}

func minimalValidStockNarrative(resp *Response, dto analytics.DashboardMetricsDTO, recs []dbgen.Recommendation) *StockCoverageNarrative {
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, recs)
	n := buildStockCoverageFactualNarrative(dto, resp, recs)
	n.Summary = "Основной риск связан с коротким покрытием по SKU с подтверждённым спросом за последнюю неделю."
	n.RiskComment = "Короткое покрытие концентрируется в нескольких SKU с активными продажами."
	forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
	n.NoSalesWithStock = StockCoverageNoSalesWithStock{Status: "unavailable"}
	n.AdvertisingLink = StockCoverageAdvertisingLink{Status: "unavailable"}
	n.RiskExamples = nil
	return n
}
