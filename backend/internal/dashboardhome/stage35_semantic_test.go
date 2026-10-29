package dashboardhome

import (
	"strings"
	"testing"
)

func TestActionSemanticCategoryAdvertisingNotStock(t *testing.T) {
	item := TopActionItem{
		Title:    "Сократить неэффективный рекламный расход",
		Reason:   "Кампании с ДРР выше 100% тратят бюджет без окупаемости",
		Category: "stock",
		DriverID: driverStock,
	}
	repairTopActionItemSemantic(&item)
	if item.Category != "advertising" || item.DriverID != driverAdvertising {
		t.Fatalf("want advertising/driver-advertising, got %q/%q", item.Category, item.DriverID)
	}
	if !actionCategoryConsistentWithText(item) {
		t.Fatal("action should be semantically consistent after repair")
	}
}

func TestQuickActionKindMatchesTopActionCategory(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			TopActions: []TopActionItem{{
				Title:     "Сократить неэффективный рекламный расход",
				Category:  "advertising",
				ActionID:  42,
				ActionKey: "unified:pack:1:action_001",
			}},
		},
		SellerPulse: SellerPulse{
			Narrative: &SellerPulseNarrative{
				QuickActions: []SellerPulseQuickAction{{
					Label: "Сократить неэффективный рекламный расход",
					Kind:  "stock",
				}},
			},
		},
	}
	reconcileSellerPulseQuickActionsWithTopActions(resp)
	qa := resp.SellerPulse.Narrative.QuickActions[0]
	if qa.Kind != "action" {
		t.Fatalf("want kind=action, got %q", qa.Kind)
	}
	if !strings.HasPrefix(qa.Href, "/app/actions") {
		t.Fatalf("want actions href, got %q", qa.Href)
	}
}

func TestWhatChangedNoDuplicateLabels(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		Groups: []dashboardNarrativeWhatChangedGroup{{
			Key: "unchanged_risks",
			Items: []dashboardNarrativeWhatChangedItem{
				{Label: "Остатки", Text: "4 SKU в критичной зоне."},
				{Label: "Остатки", Text: "Ещё 4 SKU с коротким запасом."},
			},
		}},
	}
	repairWhatChangedSemantic(wc)
	labels := map[string]int{}
	for _, item := range wc.Groups[0].Items {
		labels[strings.ToLower(item.Label)]++
	}
	for label, count := range labels {
		if count > 1 && !strings.Contains(label, "(") {
			t.Fatalf("duplicate label remains: %q", label)
		}
	}
}

func TestWhatChangedMalformedTextRejected(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		Groups: []dashboardNarrativeWhatChangedGroup{{
			Items: []dashboardNarrativeWhatChangedItem{{
				Label: "Остатки",
				Text:  "8, сохраняются 4 критичных и 4 коротких остатков.",
			}},
		}},
	}
	repairWhatChangedSemantic(wc)
	if len(wc.Groups[0].Items) == 0 {
		t.Fatal("expected repaired item, got empty group")
	}
	text := wc.Groups[0].Items[0].Text
	if actionMalformedWhatChangedStart.MatchString(text) {
		t.Fatalf("malformed text not repaired: %q", text)
	}
	if strings.Contains(text, "коротких остатков") {
		t.Fatalf("bad grammar remains: %q", text)
	}
}

func TestFunnelNoContradictoryComparisonAvailability(t *testing.T) {
	analysis := FunnelAnalysisWidget{
		PrimaryDropMessage: "Просмотры карточки → Добавления в корзину: конверсия снизилась на −4.79 п.п.",
		AIAnalysis: FunnelAIAnalysisWidget{
			WidgetBase:   WidgetBase{Status: WidgetAvailable},
			ShortSummary: "Данных о крупнейшем ухудшении этапа к прошлому периоду нет.",
		},
	}
	reconcileFunnelAnalysisConsistency(&analysis)
	if funnelHasContradictoryComparison(analysis) {
		t.Fatal("funnel still contradictory after reconcile")
	}
}

func TestStockCoverageNoSalesWithStockAvailableWhenSKUFocusHasData(t *testing.T) {
	resp := &Response{
		SKUFocus: SKUFocus{
			NoSalesWithStockTotalCount: 59,
			NoSalesWithStock:           make([]SKUFocusCard, 6),
		},
		AIDiagnostics: AIDiagnostics{
			StockCoverage: StockCoverageWidget{
				Narrative: &StockCoverageNarrative{
					NoSalesWithStock: StockCoverageNoSalesWithStock{Status: "unavailable"},
				},
			},
		},
	}
	finalizeSellerFacingDashboardResponse(resp)
	n := resp.AIDiagnostics.StockCoverage.Narrative
	if n.NoSalesWithStock.Status != "available" || n.NoSalesWithStock.Count != 59 {
		t.Fatalf("want available count=59, got %s/%d", n.NoSalesWithStock.Status, n.NoSalesWithStock.Count)
	}
}

func TestNoInternalFieldNamesInSellerFacingText(t *testing.T) {
	msg := formatSlowTurnoverStockMessage(6)
	if containsInternalFieldReference(msg) {
		t.Fatalf("slow turnover message must not contain internal refs: %q", msg)
	}
}

func TestSlowTurnoverCategoryLabel(t *testing.T) {
	if got := riskDriverCategoryLabel("slow_turnover"); got != "Медленная оборачиваемость" {
		t.Fatalf("want Медленная оборачиваемость, got %q", got)
	}
}

func TestSurplusSlowTurnoverSeparation(t *testing.T) {
	resp := &Response{
		SKUFocus: SKUFocus{
			Surplus:      []SKUFocusCard{{ProductName: "A"}},
			SlowTurnover: []SKUFocusCard{{ProductName: "A"}},
		},
	}
	omitDeprecatedSKUFocusSurplus(resp)
	if len(resp.SKUFocus.Surplus) != 0 {
		t.Fatal("surplus should be omitted from seller-facing response")
	}
	if len(resp.SKUFocus.SlowTurnover) == 0 {
		t.Fatal("slow_turnover should remain")
	}
}

func TestDashboardAIFirstSourceMapRequired(t *testing.T) {
	resp := &Response{Status: StatusReady, AIDiagnostics: AIDiagnostics{
		AccountSummary: AccountSummaryWidget{NarrativeSource: "ai"},
	}}
	m := buildDashboardNarrativeSourceMap(resp)
	if len(m) == 0 {
		t.Fatal("source_map must not be empty")
	}
	if m["seller_pulse.kpis"] != "backend_factual_metrics" {
		t.Fatalf("unexpected kpis source: %q", m["seller_pulse.kpis"])
	}
}

func TestDashboardAIFirstRejectsHiddenFallbackSources(t *testing.T) {
	resp := &Response{
		Status: StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{NarrativeSource: "ai"},
			TopActions:     []TopActionItem{{Source: "fallback", Title: "x"}},
		},
	}
	report := RunDashboardAIFirstAudit(resp)
	if report.OverallStatus == "passed" {
		t.Fatal("expected audit failure for fallback top_action source")
	}
}

func TestDashboardAIFirstAllowsBackendFactualMetrics(t *testing.T) {
	resp := &Response{
		Status: StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{NarrativeSource: "ai"},
		},
	}
	report := RunDashboardAIFirstAudit(resp)
	if report.SourceMap["funnel_analysis.stages"] != "backend_factual_metrics" {
		t.Fatalf("unexpected stages source: %q", report.SourceMap["funnel_analysis.stages"])
	}
}

func TestRepairAIOnlyTopActionAdvertisingBeforeStock(t *testing.T) {
	payload := &dashboardNarrativePayload{
		TopActions: []dashboardNarrativeTopAction{{
			Title:    "Сократить неэффективный рекламный расход",
			Reason:   "Высокий расход по кампаниям с низкой окупаемостью",
			Category: "stock",
			DriverID: driverStock,
		}},
	}
	repairDashboardNarrativePayloadAIOnly(payload)
	a := payload.TopActions[0]
	if a.Category != "stock" || a.DriverID != driverStock {
		t.Fatalf("ai-only path must not semantically rewrite action category/driver, got %q/%q", a.Category, a.DriverID)
	}
}
