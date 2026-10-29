package adapter

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

func TestProjectActions_MaterialActionsFixture(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	projected := ProjectActions(out, 501, 42, "pack_example_v1_extra_long", metricDate, nil)

	if len(projected) != len(out.Actions) {
		t.Fatalf("expected %d projected actions, got %d", len(out.Actions), len(projected))
	}

	byID := make(map[string]ProjectedAction, len(projected))
	for i, pa := range projected {
		if pa.SortOrder != i {
			t.Fatalf("expected sort order %d to be preserved, got %d", i, pa.SortOrder)
		}
		if pa.Params.RunID != 501 {
			t.Fatalf("expected run id 501 on projected action %q, got %d", pa.AnalysisActionID, pa.Params.RunID)
		}
		if pa.Params.SellerAccountID != 42 {
			t.Fatalf("expected seller account id 42 on projected action %q, got %d", pa.AnalysisActionID, pa.Params.SellerAccountID)
		}
		byID[pa.AnalysisActionID] = pa
	}

	a1, ok := byID["a1"]
	if !ok {
		t.Fatal("expected action a1 to be projected")
	}
	if !a1.IsTop {
		t.Fatal("expected a1 to be flagged as top action")
	}
	if !a1.IsRecommended {
		t.Fatal("expected a1 to be flagged as recommended action")
	}
	if a1.Params.Type != actions.ActionType("stock_replenishment") {
		t.Fatalf("expected type stock_replenishment, got %q", a1.Params.Type)
	}
	if a1.Params.Category != actions.ActionCategory("stock") {
		t.Fatalf("expected category stock, got %q", a1.Params.Category)
	}
	if a1.Params.Priority != actions.ActionPriority("critical") {
		t.Fatalf("expected priority critical, got %q", a1.Params.Priority)
	}
	if a1.Params.ActionKey == "" {
		t.Fatal("expected non-empty action key")
	}
	wantKey := ActionKeyFor(shortPackID("pack_example_v1_extra_long"), "a1")
	if a1.Params.ActionKey != wantKey {
		t.Fatalf("expected action key %q, got %q", wantKey, a1.Params.ActionKey)
	}
	if a1.Params.MetricDate != metricDate {
		t.Fatalf("expected metric date %v, got %v", metricDate, a1.Params.MetricDate)
	}

	a4, ok := byID["a4"]
	if !ok {
		t.Fatal("expected action a4 to be projected")
	}
	if a4.IsTop {
		t.Fatal("expected a4 to not be a top action")
	}
	if !a4.IsRecommended {
		t.Fatal("expected a4 to be flagged as recommended action")
	}
}

func TestProjectActions_NoActionsFixture(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.no_actions.json")
	projected := ProjectActions(out, 1, 7, "pack_example_v1", time.Now(), nil)
	if len(projected) != 0 {
		t.Fatalf("expected no projected actions, got %d", len(projected))
	}
}

func TestProjectActions_HydratesObjectActionMetricsFromFacts(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	var targetSKU string
	for _, a := range out.Actions {
		if a.Scope.ScopeType == "sku" && len(a.Scope.EntityIDs) > 0 {
			targetSKU = a.Scope.EntityIDs[0]
			break
		}
	}
	if targetSKU == "" {
		t.Fatal("fixture has no sku-scoped action")
	}
	bareSKU := targetSKU
	if i := strings.IndexByte(bareSKU, ':'); i >= 0 {
		bareSKU = bareSKU[i+1:]
	}

	skuID, err := strconv.ParseInt(bareSKU, 10, 64)
	if err != nil {
		// Non-numeric fixture ids are still resolvable via offer_id path.
		skuID = 0
	}
	name := "Test lamp"
	stock := int32(12)
	cover := 2.5
	rec := dashboardfacts.SKUFactRecord{
		OzonProductID:  1,
		SKU:            &skuID,
		OfferID:        &bareSKU,
		ProductName:    &name,
		StockAvailable: stock,
		Orders7d:       7,
		DaysOfCover:    &cover,
	}
	facts := dashboardfacts.DashboardFactsV1{
		SKUFactsIndex: dashboardfacts.SKUFactsIndex{
			ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{"1": rec},
		},
	}

	projected := ProjectActions(out, 1, 1, "pack_test", metricDate, EntityFactsLookupFromDashboardFacts(facts))
	found := false
	for _, pa := range projected {
		detail, _ := actions.ParseActionPayload(pa.Params.PayloadJSON)
		if detail == nil {
			continue
		}
		for _, obj := range detail.ObjectActions {
			bareTarget := targetSKU
			if i := strings.IndexByte(bareTarget, ':'); i >= 0 {
				bareTarget = bareTarget[i+1:]
			}
			if obj.EntityID != targetSKU && obj.EntityID != bareTarget {
				continue
			}
			found = true
			if obj.Name != name {
				t.Fatalf("name=%q want %q", obj.Name, name)
			}
			if len(obj.Metrics) < 3 {
				t.Fatalf("metrics=%d want >=3: %+v", len(obj.Metrics), obj.Metrics)
			}
		}
	}
	if !found {
		t.Fatal("expected hydrated object_action for target SKU")
	}
}

func TestActionKeyFor_StableAndShortensPackID(t *testing.T) {
	k1 := ActionKeyFor(shortPackID("a_very_long_pack_identifier_value"), "a1")
	k2 := ActionKeyFor(shortPackID("a_very_long_pack_identifier_value"), "a1")
	if k1 != k2 {
		t.Fatalf("expected ActionKeyFor to be deterministic, got %q vs %q", k1, k2)
	}
	if k1 != "unified:a_very_long_:a1" {
		t.Fatalf("expected shortened pack id in action key, got %q", k1)
	}
}
