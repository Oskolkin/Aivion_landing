package actions

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func testRun69LikePack() *aidatapack.SellerAIDataPack {
	pack := testV2CatalogPack()
	pack.FactRegistry["fact_1"] = aidatapack.Fact{
		FactID: "fact_1", Metric: "idc_days", Window: "current_day",
		Value: 5, Unit: aidatapack.UnitDays, FormattedValue: "5 дн.",
	}
	for i := 1; i <= 12; i++ {
		skuID := "sku_" + itoa(i)
		pack.EntityRegistry.SKUs[skuID] = aidatapack.SKUEntity{
			EntityID: skuID, EntityType: "sku", SKU: skuID, Name: "SKU " + itoa(i),
		}
		pack.FactRegistry["sku."+skuID+".stock.current_day"] = aidatapack.Fact{
			FactID: "sku." + skuID + ".stock.current_day", Metric: "stock_available", Window: "current_day",
			Value: 10, Unit: aidatapack.UnitCount, FormattedValue: "10",
		}
		pack.FactRegistry["sku."+skuID+".idc_days.current_day"] = aidatapack.Fact{
			FactID: "sku." + skuID + ".idc_days.current_day", Metric: "idc_days", Window: "current_day",
			Value: 5, Unit: aidatapack.UnitDays, FormattedValue: "5 дн.",
		}
	}
	pack.FactRegistry["campaign.1.spend.last_7d"] = aidatapack.Fact{
		FactID: "campaign.1.spend.last_7d", Metric: "spend", Window: "last_7d",
		Value: 4200, Unit: aidatapack.UnitRUB, FormattedValue: "4 200 ₽",
	}
	pack.FactRegistry["campaign.2.spend.last_7d"] = aidatapack.Fact{
		FactID: "campaign.2.spend.last_7d", Metric: "spend", Window: "last_7d",
		Value: 4200, Unit: aidatapack.UnitRUB, FormattedValue: "4 200 ₽",
	}
	pack.FactRegistry["campaign.3.spend.last_7d"] = aidatapack.Fact{
		FactID: "campaign.3.spend.last_7d", Metric: "spend", Window: "last_7d",
		Value: 4200, Unit: aidatapack.UnitRUB, FormattedValue: "4 200 ₽",
	}
	return pack
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func buildRun69LikeInput(t *testing.T) ActionsAIFirstInput {
	t.Helper()
	pack := testRun69LikePack()
	candidates := []AIFirstActionCandidate{
		run69Candidate("repl:crit", ActionTypeStockReplenishment, ActionCategoryStock, "sku_1", "sku_2"),
		run69Candidate("repl:prev", ActionTypeStockReplenishment, ActionCategoryStock, "sku_1", "sku_3"),
		run69Candidate("no_sales", ActionTypeNoSalesWithStock, ActionCategorySlowTurnover, "sku_4", "sku_5"),
		run69Candidate("surplus", ActionTypeSlowTurnoverReview, ActionCategorySlowTurnover, "sku_6"),
		run69Candidate("ad_spend", ActionTypeAdSpendNoOrders, ActionCategoryAdvertising, "campaign_1"),
	}
	candidates[4].EvidenceFactIDs = []string{
		"campaign.1.spend.last_7d", "campaign.2.spend.last_7d", "campaign.3.spend.last_7d",
	}
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build input: %v", err)
	}
	return input
}

func run69Candidate(id string, typ ActionType, cat ActionCategory, entityIDs ...string) AIFirstActionCandidate {
	entities := make([]AIFirstAffectedEntityRef, 0, len(entityIDs))
	for _, eid := range entityIDs {
		entType := "sku"
		if strings.HasPrefix(eid, "campaign") {
			entType = "campaign"
		}
		entities = append(entities, AIFirstAffectedEntityRef{EntityType: entType, EntityID: eid, FactIDs: []string{"fact_1"}})
	}
	return AIFirstActionCandidate{
		CandidateID: id, ChunkID: "chunk", Type: typ, Category: cat,
		LocalPriority: ActionPriorityCritical, Problem: "problem", RecommendedAction: "act",
		WhyActionable: "why", AffectedEntities: entities, EvidenceFactIDs: []string{"fact_1"},
	}
}

func TestNormalizeActions_TitleTemplatePreservesAIWording(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := AIFirstDraftAction{
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
		OperationalGoal:    "replenish_stock",
		TitleTemplate:      "Срочно пополнить {object_count} SKU с дефицитом",
		Reason:             strings.Repeat("reason ", 60),
		NextStep:           "step",
		CompletionCriteria: "done",
		EvidenceFactIDs:    []string{"fact_1"},
		SourceCandidateIDs: []string{"repl:crit"},
		AffectedEntities: []AIFirstAffectedEntityRef{
			{EntityType: "sku", EntityID: "sku_1"},
			{EntityType: "sku", EntityID: "sku_2"},
		},
		ActionPlan: []AIFirstActionStep{
			{Instruction: "Проверить остаток sku_1 и sku_2 по складу."},
			{Instruction: "Сформировать поставку по sku с idc_days ниже порога."},
			{Instruction: "Подтвердить дату следующего поступления."},
		},
	}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	if out.Actions[0].Title != "Срочно пополнить 2 SKU с дефицитом" {
		t.Fatalf("title=%q", out.Actions[0].Title)
	}
}

func TestNormalizeActions_CandidateScopeUnionOnlySelected(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := AIFirstDraftAction{
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
		OperationalGoal: "replenish_stock", TitleTemplate: "Пополнить {object_count} SKU",
		Reason: strings.Repeat("r ", 80), NextStep: "s", CompletionCriteria: "c",
		EvidenceFactIDs:    []string{"fact_1"},
		SourceCandidateIDs: []string{"repl:crit", "repl:prev"},
		AffectedEntities:   []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}},
		ActionPlan:         []AIFirstActionStep{{Instruction: "a"}, {Instruction: "b"}, {Instruction: "c"}},
	}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	ids := normalizeTestEntityIDsFromAction(out.Actions[0])
	if len(ids) != 3 || !normalizeTestContainsString(ids, "sku_3") {
		t.Fatalf("entities=%v want union of repl:crit+repl:prev", ids)
	}
	if normalizeTestContainsString(ids, "sku_4") {
		t.Fatalf("must not include entity from unselected candidate")
	}
}

func TestNormalizeActions_TechnicalDuplicateByOperationalGoal(t *testing.T) {
	input := buildRun69LikeInput(t)
	makeAction := func(title string, sku string) AIFirstDraftAction {
		return AIFirstDraftAction{
			Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
			OperationalGoal: "replenish_stock", TitleTemplate: title,
			Reason: strings.Repeat("r ", 80), NextStep: "s", CompletionCriteria: "c",
			EvidenceFactIDs: []string{"fact_1"}, SourceCandidateIDs: []string{"repl:crit"},
			AffectedEntities: []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: sku}},
			ActionPlan:       []AIFirstActionStep{{Instruction: "a"}, {Instruction: "b"}, {Instruction: "c"}},
		}
	}
	out, result := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{
		makeAction("Критическое пополнение {object_count} SKU", "sku_1"),
		makeAction("Превентивное пополнение {object_count} SKU", "sku_1"),
	}})
	if len(out.Actions) != 1 {
		t.Fatalf("actions=%d want=1 after empty duplicate removed", len(out.Actions))
	}
	if !normalizeTestContainsString(normalizeTestEntityIDsFromAction(out.Actions[0]), "sku_1") {
		t.Fatalf("first action must keep sku_1")
	}
	if !normalizeTestHasIssueCode(result.Issues, "duplicate_entity_operational_goal_removed") {
		t.Fatalf("issues=%v", result.Issues)
	}
}

func TestNormalizeActions_DifferentGoalsKeepSameSKU(t *testing.T) {
	input := buildRun69LikeInput(t)
	base := func(goal, cid string) AIFirstDraftAction {
		return AIFirstDraftAction{
			Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
			OperationalGoal: goal, TitleTemplate: "Действие {object_count} SKU",
			Reason: strings.Repeat("r ", 80), NextStep: "s", CompletionCriteria: "c",
			EvidenceFactIDs: []string{"fact_1"}, SourceCandidateIDs: []string{cid},
			AffectedEntities: []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}},
			ActionPlan:       []AIFirstActionStep{{Instruction: "a"}, {Instruction: "b"}, {Instruction: "c"}},
		}
	}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{
		base("replenish_stock", "repl:crit"),
		base("improve_ad_efficiency", "ad_spend"),
	}})
	if len(out.Actions) != 2 {
		t.Fatalf("actions=%d", len(out.Actions))
	}
}

func TestNormalizeActions_MetricHydrationIDC(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.ObjectActions = []AIFirstObjectAction{{
		EntityType: "sku", EntityID: "sku_1", RecommendedAction: "Пополнить", Reason: "risk",
		Metrics: []AIFirstObjectMetric{{FactID: "fact_1"}},
	}}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	m := out.Actions[0].ObjectActions[0].Metrics[0]
	if m.DisplayLabel != "Покрытие запасом" {
		t.Fatalf("label=%q", m.DisplayLabel)
	}
}

func TestNormalizeActions_ComputedSum(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "ad_spend", "campaign_1")
	action.Type = ActionTypeAdSpendNoOrders
	action.Category = ActionCategoryAdvertising
	action.OperationalGoal = "stop_ad_waste"
	action.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "campaign", EntityID: "campaign_1"}}
	action.ComputedValues = []AIFirstComputedValue{
		{Key: "total_spend_7d", Op: "sum", FactIDs: []string{
			"campaign.1.spend.last_7d", "campaign.2.spend.last_7d", "campaign.3.spend.last_7d",
		}},
	}
	action.SummaryTemplate = "{object_count} кампании потратили {computed:total_spend_7d} за 7 дней без заказов."
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	if !strings.Contains(out.Actions[0].Summary, "12 600") && !strings.Contains(out.Actions[0].Summary, "12 600") {
		t.Fatalf("summary=%q", out.Actions[0].Summary)
	}
}

func TestNormalizeActions_CategoryTaxonomyOnly(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "no_sales", "sku_4")
	action.Type = ActionTypeNoSalesWithStock
	action.Category = ActionCategoryStock
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	if out.Actions[0].Category != ActionCategorySlowTurnover {
		t.Fatalf("category=%q", out.Actions[0].Category)
	}
}

func TestNormalizeActions_PartialObjectActionsCounts(t *testing.T) {
	pack := testRun69LikePack()
	entities := make([]AIFirstAffectedEntityRef, 0, 12)
	for i := 1; i <= 12; i++ {
		id := "sku_" + itoa(i)
		entities = append(entities, AIFirstAffectedEntityRef{EntityType: "sku", EntityID: id, FactIDs: []string{"fact_1"}})
	}
	candidate := AIFirstActionCandidate{
		CandidateID: "repl:12", ChunkID: "chunk", Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		LocalPriority: ActionPriorityCritical, Problem: "p", RecommendedAction: "a", WhyActionable: "w",
		AffectedEntities: entities, EvidenceFactIDs: []string{"fact_1"},
	}
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", []AIFirstActionCandidate{candidate}, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build input: %v", err)
	}
	objects := make([]AIFirstObjectAction, 0, 8)
	for i := 1; i <= 8; i++ {
		id := "sku_" + itoa(i)
		objects = append(objects, AIFirstObjectAction{
			EntityType: "sku", EntityID: id, RecommendedAction: "act", Reason: "why",
			Metrics: []AIFirstObjectMetric{{FactID: "fact_1"}},
		})
	}
	action := detailedNormalizedAction(input, "repl:12", "sku_1")
	action.AffectedEntities = entities
	action.ObjectActions = objects
	action.TitleTemplate = "Пополнить {object_count} SKU"
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	a := out.Actions[0]
	if a.AffectedObjectsTotal != 12 || a.AffectedObjectsDisplayed != 8 || !a.AffectedObjectsTruncated {
		t.Fatalf("total=%d displayed=%d truncated=%v", a.AffectedObjectsTotal, a.AffectedObjectsDisplayed, a.AffectedObjectsTruncated)
	}
}

func TestPilotValidator_QualityWarningDoesNotDrop(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.ObjectActions = []AIFirstObjectAction{{
		EntityType: "sku", EntityID: "sku_1", RecommendedAction: "Пополнить", Reason: "why",
		Metrics: []AIFirstObjectMetric{{FactID: "sku.sku_1.stock.current_day"}},
	}}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	pilot := ValidateActionsAIFirstOutputPilot(input, out)
	if pilot.AcceptedCount != 1 {
		t.Fatalf("accepted=%d issues=%v", pilot.AcceptedCount, pilot.Issues)
	}
	if !normalizeTestPilotHasIssue(pilot, "insufficient_object_metrics") {
		t.Fatalf("expected quality warning, issues=%v", pilot.Issues)
	}
}

func TestPilotValidator_UnknownFactDrops(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.EvidenceFactIDs = []string{"missing_fact"}
	pilot := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	if pilot.AcceptedCount != 0 {
		t.Fatalf("accepted=%d", pilot.AcceptedCount)
	}
}

func detailedNormalizedAction(input ActionsAIFirstInput, candidateID, entityID string) AIFirstDraftAction {
	entType := "sku"
	if strings.HasPrefix(entityID, "campaign") {
		entType = "campaign"
	}
	return AIFirstDraftAction{
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
		OperationalGoal: "replenish_stock", TitleTemplate: "Пополнить {object_count} SKU",
		ScopeType: AIFirstActionScopeEntity,
		Reason:    strings.Repeat("reason with facts ", 20), NextStep: "step", CompletionCriteria: "criteria",
		EvidenceFactIDs: []string{"fact_1"}, SourceCandidateIDs: []string{candidateID},
		AffectedEntities: []AIFirstAffectedEntityRef{{EntityType: entType, EntityID: entityID}},
		ActionPlan: []AIFirstActionStep{
			{Instruction: "Проверить остаток и заказы sku."},
			{Instruction: "Согласовать поставку по sku."},
			{Instruction: "Назначить контроль через 7 дней."},
		},
	}
}

func normalizeTestEntityIDsFromAction(a AIFirstDraftAction) []string {
	out := make([]string, 0, len(a.AffectedEntities))
	for _, e := range a.AffectedEntities {
		out = append(out, e.EntityID)
	}
	return out
}

func normalizeTestContainsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func normalizeTestHasIssueCode(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func normalizeTestPilotHasIssue(pilot PilotActionsValidationResult, code string) bool {
	for _, i := range pilot.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
