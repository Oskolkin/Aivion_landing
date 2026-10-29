package outputvalidator_test

import (
	"fmt"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func ensureFloatPtr(v float64) *float64 { return &v }

// setOfIDs builds the map[string]struct{} shape used by
// PackReferenceIndex.SKUIds/CampaignIds.
func setOfIDs(ids ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func oosSKU(id string) outputvalidator.SKUFactRow {
	return outputvalidator.SKUFactRow{
		SKUID:          id,
		HasStockData:   true,
		AvailableStock: 0,
		StockStatus:    "out_of_stock",
		Orders7D:       0,
		OrdersPrev7D:   6,
		Revenue7D:      0,
		RevenuePrev7D:  9000,
	}
}

func criticalShortageSKU(id string) outputvalidator.SKUFactRow {
	return outputvalidator.SKUFactRow{
		SKUID:           id,
		HasStockData:    true,
		AvailableStock:  3,
		StockStatus:     "critical_shortage",
		Orders7D:        5,
		OrdersPrev7D:    5,
		AvgDailySales7D: ensureFloatPtr(1.2),
		CoverageDays7D:  ensureFloatPtr(2.5),
	}
}

func highDRRCampaign(id string, drr float64) outputvalidator.CampaignFactRow {
	return outputvalidator.CampaignFactRow{
		CampaignID: id,
		Spend7D:    5000,
		Revenue7D:  2000,
		Orders7D:   4,
		DRRPct:     ensureFloatPtr(drr),
	}
}

func spendNoOrdersCampaign(id string, spend float64) outputvalidator.CampaignFactRow {
	return outputvalidator.CampaignFactRow{
		CampaignID: id,
		Spend7D:    spend,
		Revenue7D:  0,
		Orders7D:   0,
	}
}

func salesDeclineSKU(id string, prevOrders, orders int64, prevRevenue, revenue float64) outputvalidator.SKUFactRow {
	return outputvalidator.SKUFactRow{
		SKUID:         id,
		OrdersPrev7D:  prevOrders,
		Orders7D:      orders,
		RevenuePrev7D: prevRevenue,
		Revenue7D:     revenue,
	}
}

func actionWithSKUScope(id, actionType, tier string, skuIDs ...string) contract.UnifiedActionV1 {
	a := minimalAction(id, tier, "important")
	a.Type = actionType
	entities := make([]string, 0, len(skuIDs))
	for _, s := range skuIDs {
		entities = append(entities, "sku:"+s)
	}
	a.Scope = contract.UnifiedActionScopeV1{ScopeType: "sku", EntityIDs: entities}
	return a
}

func materialOutcomeOutput(actions ...contract.UnifiedActionV1) *contract.UnifiedAnalysisOutputV1 {
	return &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Actions:         append([]contract.UnifiedActionV1{}, actions...),
	}
}

func TestEnsureMaterialDomainActions_BackfillsStockAndAds(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds:      setOfIDs("101", "102"),
		CampaignIds: setOfIDs("501"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"101": oosSKU("101"),
		},
		CampaignFacts: map[string]outputvalidator.CampaignFactRow{
			"501": highDRRCampaign("501", 150),
		},
	}
	// AI returned exactly one Action (a sales-drop action on an unrelated
	// SKU) — stock and advertising problems are completely uncovered.
	out := materialOutcomeOutput(actionWithSKUScope("action_001", "sku_orders_drop", contract.ExecutionTierRequired, "102"))

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)

	if len(res.AddedActionIDs) != 2 {
		t.Fatalf("AddedActionIDs=%v want 2 (stock + ads)", res.AddedActionIDs)
	}
	if len(out.Actions) != 3 {
		t.Fatalf("actions=%d want 3 (1 AI + 2 backfilled)", len(out.Actions))
	}

	var stockAction, adsAction *contract.UnifiedActionV1
	for i := range out.Actions {
		switch out.Actions[i].Type {
		case "stock_replenishment":
			stockAction = &out.Actions[i]
		case "ad_high_drr_review":
			adsAction = &out.Actions[i]
		}
	}
	if stockAction == nil {
		t.Fatal("expected a backfilled stock_replenishment action")
	}
	if adsAction == nil {
		t.Fatal("expected a backfilled ad_high_drr_review action")
	}

	if stockAction.Category != "stock" || stockAction.Priority != "critical" || stockAction.ExecutionTier != contract.ExecutionTierRequired {
		t.Fatalf("stock action shape unexpected: %+v", stockAction)
	}
	if len(stockAction.Scope.EntityIDs) != 1 || stockAction.Scope.EntityIDs[0] != "101" {
		t.Fatalf("stock action scope=%v want [101]", stockAction.Scope.EntityIDs)
	}
	if len(stockAction.Steps) < 3 {
		t.Fatalf("stock action steps=%d want >=3", len(stockAction.Steps))
	}
	if len(stockAction.EvidenceFactIDs) == 0 {
		t.Fatal("stock action must carry evidence_fact_ids")
	}

	if adsAction.Category != "advertising" || adsAction.ExecutionTier != contract.ExecutionTierRequired {
		t.Fatalf("ads action shape unexpected: %+v", adsAction)
	}
	if len(adsAction.Scope.EntityIDs) != 1 || adsAction.Scope.EntityIDs[0] != "501" {
		t.Fatalf("ads action scope=%v want [501]", adsAction.Scope.EntityIDs)
	}

	// Title must be a concrete, non-placeholder Russian string that cites
	// the real candidate count — never invented filler.
	if stockAction.Title == "" || stockAction.Summary == "" || stockAction.CompletionCriteria == "" {
		t.Fatalf("stock action missing required narrative fields: %+v", stockAction)
	}
	t.Logf("backfilled stock title: %q", stockAction.Title)
	t.Logf("backfilled ads title: %q", adsAction.Title)

	// analysis_action_id must not collide with the existing AI action and
	// must follow the action_NNN convention used elsewhere in the pipeline.
	seen := map[string]bool{}
	for _, a := range out.Actions {
		if seen[a.AnalysisActionID] {
			t.Fatalf("duplicate analysis_action_id %q", a.AnalysisActionID)
		}
		seen[a.AnalysisActionID] = true
	}
}

func TestEnsureMaterialDomainActions_DoesNotDuplicateOwnedSKUs(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds: setOfIDs("101", "103"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"101": oosSKU("101"), // already owned by an existing Action below
			"103": oosSKU("103"), // unowned — must be picked up
		},
	}
	// A pre-existing Action (of an unrelated type) already claims SKU 101.
	out := materialOutcomeOutput(actionWithSKUScope("action_001", "key_sku_protection", contract.ExecutionTierRequired, "101"))

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)

	if len(res.AddedActionIDs) != 1 {
		t.Fatalf("AddedActionIDs=%v want 1 (only SKU 103's domain)", res.AddedActionIDs)
	}
	var stockAction *contract.UnifiedActionV1
	for i := range out.Actions {
		if out.Actions[i].Type == "stock_replenishment" {
			stockAction = &out.Actions[i]
		}
	}
	if stockAction == nil {
		t.Fatal("expected a backfilled stock_replenishment action for SKU 103")
	}
	if len(stockAction.Scope.EntityIDs) != 1 || stockAction.Scope.EntityIDs[0] != "103" {
		t.Fatalf("scope=%v want [103] (SKU 101 already owned, must never be reused)", stockAction.Scope.EntityIDs)
	}

	// Hard rule: every sku_id must appear in at most one Action across the
	// whole result.
	owner := map[string]string{}
	for _, a := range out.Actions {
		for _, e := range a.Scope.EntityIDs {
			if prev, ok := owner[e]; ok {
				t.Fatalf("entity %q owned by both %q and %q", e, prev, a.AnalysisActionID)
			}
			owner[e] = a.AnalysisActionID
		}
	}
}

func TestEnsureMaterialDomainActions_CoveredEntitiesSkippedButOtherBandsBackfilled(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds: setOfIDs("101", "102"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"101": oosSKU("101"),
			"102": criticalShortageSKU("102"),
		},
	}
	// AI already owns OOS SKU 101 via stock_replenishment. Critical SKU 102
	// remains uncovered and must still be backfilled as a separate band.
	out := materialOutcomeOutput(actionWithSKUScope("action_001", "stock_replenishment", contract.ExecutionTierRequired, "101"))

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)

	if len(res.AddedActionIDs) < 1 {
		t.Fatalf("AddedActionIDs=%v want at least critical band backfill", res.AddedActionIDs)
	}
	foundCritical := false
	for _, a := range out.Actions {
		for _, e := range a.Scope.EntityIDs {
			if e == "102" {
				foundCritical = true
			}
			if e == "101" && a.AnalysisActionID != "action_001" {
				t.Fatalf("SKU 101 reused by %s", a.AnalysisActionID)
			}
		}
	}
	if !foundCritical {
		t.Fatalf("expected backfilled action covering 102, actions=%+v", out.Actions)
	}
	foundExclusion := false
	for _, ex := range res.Exclusions {
		if ex.Domain == "stock_oos" && ex.Code == outputvalidator.ExclusionCoveredByExisting {
			foundExclusion = true
		}
	}
	if !foundExclusion {
		t.Fatalf("expected covered_by_existing_action exclusion for stock_oos, got %+v", res.Exclusions)
	}
}

func TestEnsureMaterialDomainActions_RespectsMaxActionsAndFlagsIncompleteCoverage(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds: setOfIDs("101"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"101": oosSKU("101"),
			"201": salesDeclineSKU("201", 10, 2, 10000, 2000),
		},
	}
	idx.SKUIds["201"] = struct{}{}

	// Fourteen pre-existing Actions (unrelated type/scope) already fill most
	// of the contract.MaxActionsV1=15 budget.
	actions := make([]contract.UnifiedActionV1, 0, 14)
	for i := 1; i <= 14; i++ {
		a := minimalAction(fmt.Sprintf("action_%03d", i), contract.ExecutionTierRequired, "watch")
		a.Type = "category_underperformance"
		a.Scope = contract.UnifiedActionScopeV1{ScopeType: "account"}
		actions = append(actions, a)
	}
	out := materialOutcomeOutput(actions...)

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)

	if len(out.Actions) != contract.MaxActionsV1 {
		t.Fatalf("actions=%d want %d (hard cap)", len(out.Actions), contract.MaxActionsV1)
	}
	if len(res.AddedActionIDs) != 1 {
		t.Fatalf("AddedActionIDs=%v want exactly 1 (only room for one more)", res.AddedActionIDs)
	}
	capReached := false
	for _, ex := range res.Exclusions {
		if ex.Code == outputvalidator.ExclusionActionCapReached {
			capReached = true
		}
	}
	if !capReached {
		t.Fatalf("expected an action_cap_reached exclusion, got %+v", res.Exclusions)
	}

	// Completeness safety net: the domain that could not be backfilled due
	// to the cap must surface as a hard validation error.
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{}, idx)
	if !hasIssueCode(report.Errors, outputvalidator.CodeActionDomainCoverageIncomplete) {
		t.Fatalf("expected %s error, got errors=%+v", outputvalidator.CodeActionDomainCoverageIncomplete, report.Errors)
	}
}

func TestEnsureMaterialDomainActions_SplitsOOSAndCriticalIntoSeparateStockActions(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds: setOfIDs("101", "102"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"101": oosSKU("101"),
			"102": criticalShortageSKU("102"),
		},
	}
	out := materialOutcomeOutput()

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)

	if len(res.AddedActionIDs) != 2 {
		t.Fatalf("AddedActionIDs=%v want 2 (OOS and critical as separate operational Actions)", res.AddedActionIDs)
	}
	if len(out.Actions) != 2 {
		t.Fatalf("actions=%d want 2", len(out.Actions))
	}
	owned := map[string]string{}
	for _, a := range out.Actions {
		for _, e := range a.Scope.EntityIDs {
			if prev, ok := owned[e]; ok {
				t.Fatalf("entity %q owned by both %q and %q", e, prev, a.AnalysisActionID)
			}
			owned[e] = a.AnalysisActionID
		}
	}
	if _, ok := owned["101"]; !ok {
		t.Fatalf("missing OOS 101 in actions=%+v", out.Actions)
	}
	if _, ok := owned["102"]; !ok {
		t.Fatalf("missing critical 102 in actions=%+v", out.Actions)
	}
}

func TestEnsureMaterialDomainActions_NoopWithoutIndexOrNonMaterialOutcome(t *testing.T) {
	out := materialOutcomeOutput()
	if res := outputvalidator.EnsureMaterialDomainActions(out, nil); len(res.AddedActionIDs) != 0 {
		t.Fatalf("expected no-op with nil index, got %+v", res)
	}

	idx := &outputvalidator.PackReferenceIndex{
		SKUIds:   setOfIDs("101"),
		SKUFacts: map[string]outputvalidator.SKUFactRow{"101": oosSKU("101")},
	}
	noActionsOut := &contract.UnifiedAnalysisOutputV1{AnalysisOutcome: contract.AnalysisOutcomeNoMaterialActions}
	if res := outputvalidator.EnsureMaterialDomainActions(noActionsOut, idx); len(res.AddedActionIDs) != 0 {
		t.Fatalf("expected no-op for non-material outcome, got %+v", res)
	}
	if len(noActionsOut.Actions) != 0 {
		t.Fatal("no_material_actions must keep an empty actions array")
	}
}

func TestEnsureMaterialDomainActions_SpendNoOrdersAndSalesDecline(t *testing.T) {
	idx := &outputvalidator.PackReferenceIndex{
		CampaignIds: setOfIDs("601"),
		SKUIds:      setOfIDs("301"),
		CampaignFacts: map[string]outputvalidator.CampaignFactRow{
			"601": spendNoOrdersCampaign("601", 3000),
		},
		SKUFacts: map[string]outputvalidator.SKUFactRow{
			"301": salesDeclineSKU("301", 10, 2, 10000, 2000),
		},
	}
	out := materialOutcomeOutput()

	res := outputvalidator.EnsureMaterialDomainActions(out, idx)
	if len(res.AddedActionIDs) != 2 {
		t.Fatalf("AddedActionIDs=%v want 2 (spend-no-orders + sales-decline)", res.AddedActionIDs)
	}
	var spendAction, declineAction *contract.UnifiedActionV1
	for i := range out.Actions {
		switch out.Actions[i].Type {
		case "ad_spend_no_orders":
			spendAction = &out.Actions[i]
		case "sku_orders_drop":
			declineAction = &out.Actions[i]
		}
	}
	if spendAction == nil {
		t.Fatal("expected a backfilled ad_spend_no_orders action")
	}
	if declineAction == nil {
		t.Fatal("expected a backfilled sku_orders_drop action")
	}
	if len(spendAction.Scope.EntityIDs) != 1 || spendAction.Scope.EntityIDs[0] != "601" {
		t.Fatalf("spend action scope=%v want [601]", spendAction.Scope.EntityIDs)
	}
	if len(declineAction.Scope.EntityIDs) != 1 || declineAction.Scope.EntityIDs[0] != "301" {
		t.Fatalf("decline action scope=%v want [301]", declineAction.Scope.EntityIDs)
	}
}
