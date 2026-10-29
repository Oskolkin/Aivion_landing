package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestReconcileStockActionCohorts_DropsMixedUmbrella(t *testing.T) {
	cov2 := 2.0
	cov90 := 90.0
	idx := &PackReferenceIndex{
		SKUIds: map[string]struct{}{"1": {}, "2": {}, "3": {}},
		SKUFacts: map[string]SKUFactRow{
			"1": {StockStatus: stockpolicy.StatusOutOfStock, HasStockData: true, AvailableStock: 0, Orders7D: 5},
			"2": {StockStatus: stockpolicy.StatusCriticalShortage, HasStockData: true, AvailableStock: 2, CoverageDays7D: &cov2, Orders7D: 7},
			"3": {StockStatus: stockpolicy.StatusHealthy, HasStockData: true, AvailableStock: 90, CoverageDays7D: &cov90, Orders7D: 7},
		},
	}
	out := &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Actions: []contract.UnifiedActionV1{
			{
				AnalysisActionID:   "action_001",
				Type:               "stock_replenishment",
				Category:           "stock",
				Title:              "Пополнить 3 дефицитных SKU",
				Scope:              contract.UnifiedActionScopeV1{ScopeType: "sku", EntityIDs: []string{"1", "2", "3"}},
				Steps:              []contract.UnifiedActionStepV1{{Order: 1, Title: "x", Description: "y"}},
				CompletionCriteria: "done",
			},
			{
				AnalysisActionID:   "action_002",
				Type:               "ad_high_drr_review",
				Category:           "advertising",
				Title:              "Реклама",
				Scope:              contract.UnifiedActionScopeV1{ScopeType: "campaign", EntityIDs: []string{"9"}},
				Steps:              []contract.UnifiedActionStepV1{{Order: 1, Title: "x", Description: "y"}},
				CompletionCriteria: "done",
			},
		},
	}

	dropped := ReconcileStockActionCohorts(out, idx)
	if dropped != 1 {
		t.Fatalf("dropped=%d want 1", dropped)
	}
	if len(out.Actions) != 1 || out.Actions[0].AnalysisActionID != "action_002" {
		t.Fatalf("actions=%+v", out.Actions)
	}

	res := EnsureMaterialDomainActions(out, idx)
	if len(res.AddedActionIDs) < 2 {
		t.Fatalf("AddedActionIDs=%v want OOS+critical", res.AddedActionIDs)
	}
	stockTitles := 0
	for _, a := range out.Actions {
		if a.Type != "stock_replenishment" {
			continue
		}
		stockTitles++
		bands := stockBandsForEntities(a.Scope.EntityIDs, idx)
		if len(bands) != 1 {
			t.Fatalf("stock action %s mixed bands=%v", a.AnalysisActionID, bands)
		}
		for _, id := range a.Scope.EntityIDs {
			if id == "3" {
				t.Fatal("healthy SKU 3 must not enter deficit band")
			}
		}
	}
	if stockTitles != 2 {
		t.Fatalf("stock replenishment actions=%d want 2 exclusive bands", stockTitles)
	}
}
