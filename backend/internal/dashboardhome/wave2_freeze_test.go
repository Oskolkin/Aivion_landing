package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestShouldOverrideTopActionsFromFinal_SkipsUnified(t *testing.T) {
	if shouldOverrideTopActionsFromFinal(UnifiedPublicationAdapterPromptVersion) {
		t.Fatal("unified publication must not override Top Actions on GET")
	}
	if !shouldOverrideTopActionsFromFinal(DashboardNarrativePromptVersion) {
		t.Fatal("legacy path may still override Top Actions")
	}
	if !shouldOverrideTopActionsFromFinal("") {
		t.Fatal("empty prompt version is treated as legacy override-eligible")
	}
}

func TestUnifiedPublicationKeepsPublishedTopActionsComposition(t *testing.T) {
	published := []TopActionItem{
		{ActionKey: "pub-a1", Title: "Published A", ActionID: 101},
		{ActionKey: "pub-a2", Title: "Published B", ActionID: 102},
		{ActionKey: "pub-a3", Title: "Published C", ActionID: 103},
	}
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				PromptVersion: UnifiedPublicationAdapterPromptVersion,
			},
			TopActions: append([]TopActionItem(nil), published...),
		},
		TopActions: append([]TopActionItem(nil), published...),
	}

	// Mirror applyStoredDashboardNarrativeToHome: Unified skips override.
	if shouldOverrideTopActionsFromFinal(resp.AIDiagnostics.AccountSummary.PromptVersion) {
		t.Fatal("guard failed: unified must skip overrideTopActionsFromFinalActions")
	}
	if len(resp.AIDiagnostics.TopActions) != 3 {
		t.Fatalf("composition length changed: %d", len(resp.AIDiagnostics.TopActions))
	}
	for i := range published {
		if resp.AIDiagnostics.TopActions[i].ActionKey != published[i].ActionKey ||
			resp.AIDiagnostics.TopActions[i].Title != published[i].Title ||
			resp.AIDiagnostics.TopActions[i].ActionID != published[i].ActionID {
			t.Fatalf("top action %d mutated: %+v", i, resp.AIDiagnostics.TopActions[i])
		}
	}
}

func TestOmitDeprecatedSKUFocusSurplus_SkippedForUnified(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				PromptVersion: UnifiedPublicationAdapterPromptVersion,
			},
		},
		SKUFocus: SKUFocus{
			Surplus: []SKUFocusCard{{ProductName: "SurplusOnly", OzonProductID: 9}},
		},
	}
	finalizeSellerFacingDashboardResponse(resp)
	if len(resp.SKUFocus.Surplus) != 1 {
		t.Fatalf("unified GET must keep published surplus taxonomy, got %+v", resp.SKUFocus.Surplus)
	}
	if len(resp.SKUFocus.SlowTurnover) != 0 {
		t.Fatalf("unified GET must not remap surplus→slow_turnover, got %+v", resp.SKUFocus.SlowTurnover)
	}
}

func TestOmitDeprecatedSKUFocusSurplus_StillRunsForLegacy(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				PromptVersion: DashboardNarrativePromptVersion,
			},
		},
		SKUFocus: SKUFocus{
			Surplus: []SKUFocusCard{{ProductName: "LegacySurplus", OzonProductID: 9}},
		},
	}
	finalizeSellerFacingDashboardResponse(resp)
	if len(resp.SKUFocus.Surplus) != 0 {
		t.Fatal("legacy path still omits surplus on GET")
	}
	if len(resp.SKUFocus.SlowTurnover) != 1 {
		t.Fatal("legacy path remaps surplus→slow_turnover")
	}
}

func TestApplyFrozenDashboardDataUnified_DoesNotEnrichSKUFocusDeltas(t *testing.T) {
	newFrozen := func() FrozenDashboardData {
		return FrozenDashboardData{
			SchemaVersion: FrozenDashboardDataSchemaVersion,
			MetricDate:    "2026-07-08",
			DashboardMetrics: analytics.DashboardMetricsDTO{
				SellerAccountID: 1,
				SKUs: []analytics.DashboardSKUMetricsItem{{
					OzonProductID:        42,
					OrdersCount:          20,
					OrdersDeltaDayToDay:  5,
					RevenueDeltaDayToDay: 100,
				}},
			},
			SKUFocus: SKUFocus{
				OrderDrop: []SKUFocusCard{{
					OzonProductID: 42,
					ProductName:   "Frozen SKU",
					// Deliberately leave deltas nil — Unified must not fill them.
				}},
			},
		}
	}

	resp := Response{}
	if err := ApplyFrozenDashboardDataUnified(&resp, newFrozen()); err != nil {
		t.Fatal(err)
	}
	if resp.SKUFocus.OrderDrop[0].CurrentOrders != nil ||
		resp.SKUFocus.OrderDrop[0].OrdersDeltaDayToDay != nil ||
		resp.SKUFocus.OrderDrop[0].RevenueDeltaDayToDay != nil {
		t.Fatalf("unified apply must not enrich deltas: %+v", resp.SKUFocus.OrderDrop[0])
	}

	legacy := Response{}
	if err := ApplyFrozenDashboardData(&legacy, newFrozen()); err != nil {
		t.Fatal(err)
	}
	if legacy.SKUFocus.OrderDrop[0].OrdersDeltaDayToDay == nil {
		t.Fatal("legacy apply may still enrich missing deltas from frozen metrics")
	}

	unifiedViaSource := Response{}
	if err := applyFrozenForAnalysisSource(&unifiedViaSource, newFrozen(), analysissource.SourceUnified); err != nil {
		t.Fatal(err)
	}
	if unifiedViaSource.SKUFocus.OrderDrop[0].OrdersDeltaDayToDay != nil {
		t.Fatal("analysis source=unified must skip SKU Focus enrichment")
	}
}
