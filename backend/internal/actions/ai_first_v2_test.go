package actions

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func testV2CatalogPack() *aidatapack.SellerAIDataPack {
	pack := testAIFirstDataPack()
	pack.FactRegistry["fact_1"] = aidatapack.Fact{FactID: "fact_1", Value: 1}
	pack.FactRegistry["fact_2"] = aidatapack.Fact{FactID: "fact_2", Value: 2}
	pack.FactRegistry["fact_3"] = aidatapack.Fact{FactID: "fact_3", Value: 3}
	pack.FactRegistry["fact_unrelated"] = aidatapack.Fact{FactID: "fact_unrelated", Value: 99}
	pack.EntityRegistry.SKUs["sku_1"] = aidatapack.SKUEntity{EntityID: "sku_1", EntityType: "sku", SKU: "sku_1", Name: "SKU 1"}
	pack.EntityRegistry.SKUs["sku_2"] = aidatapack.SKUEntity{EntityID: "sku_2", EntityType: "sku", SKU: "sku_2", Name: "SKU 2"}
	pack.EntityRegistry.Campaigns["campaign_1"] = aidatapack.CampaignEntity{EntityID: "campaign_1", EntityType: "campaign", CampaignExternalID: "c1", Name: "Campaign 1"}
	pack.EntityRegistry.SKUs["entity_unrelated"] = aidatapack.SKUEntity{EntityID: "entity_unrelated", EntityType: "sku", SKU: "x"}
	return &pack
}

func testGlobalContext() *aicontext.GlobalSemanticContext {
	return &aicontext.GlobalSemanticContext{
		OverallStory: "Тестовая история",
	}
}

func testStockCandidate(id string) AIFirstActionCandidate {
	return AIFirstActionCandidate{
		CandidateID:       id,
		ChunkID:           "stock_positions_001",
		Type:              ActionTypeStockReplenishment,
		Category:          ActionCategoryStock,
		LocalPriority:     ActionPriorityCritical,
		Problem:           "Низкий остаток SKU",
		RecommendedAction: "Пополнить остаток",
		WhyActionable:     "Риск out-of-stock",
		AffectedEntities: []AIFirstAffectedEntityRef{
			{EntityType: "sku", EntityID: "sku_1", FactIDs: []string{"fact_2"}},
		},
		EvidenceFactIDs: []string{"fact_1"},
		SourceSignalIDs: []string{"signal_1"},
	}
}

func testAdvertisingCandidate(id string) AIFirstActionCandidate {
	return AIFirstActionCandidate{
		CandidateID:       id,
		ChunkID:           "campaign_rollups_001",
		Type:              ActionTypeAdHighDRRReview,
		Category:          ActionCategoryAdvertising,
		LocalPriority:     ActionPriorityImportant,
		Problem:           "Высокий DRR",
		RecommendedAction: "Снизить ставки",
		WhyActionable:     "Перерасход бюджета",
		AffectedEntities: []AIFirstAffectedEntityRef{
			{EntityType: "campaign", EntityID: "campaign_1", FactIDs: []string{"fact_3"}},
		},
		EvidenceFactIDs: []string{"fact_3"},
		SourceSignalIDs: []string{"signal_2"},
	}
}

func TestMarshalActionsAIFirstInputWire_V2ExcludesDataPack(t *testing.T) {
	pack := testV2CatalogPack()
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc_hash",
		[]AIFirstActionCandidate{testStockCandidate("stock_positions_001:candidate:01")},
		[]string{}, []string{"limit"}, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := MarshalActionsAIFirstInputWire(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{`"data_pack"`, `"fact_registry"`, `"entity_registry"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("forbidden field %s in wire json: %s", forbidden, body)
		}
	}
	for _, required := range []string{`"action_candidates"`, `"fact_catalog"`, `"entity_catalog"`} {
		if !strings.Contains(body, required) {
			t.Fatalf("missing %s in wire json", required)
		}
	}
	if input.DataPack == nil {
		t.Fatal("backend data pack must remain on input struct")
	}
}

func TestBuildActionsAIFirstInputFromCandidates_FactCatalogScope(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{{
		CandidateID: "c1", ChunkID: "chunk1",
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		LocalPriority: ActionPriorityCritical,
		Problem:       "p", RecommendedAction: "r", WhyActionable: "w",
		EvidenceFactIDs: []string{"fact_1"},
		AffectedEntities: []AIFirstAffectedEntityRef{
			{EntityType: "sku", EntityID: "sku_1", FactIDs: []string{"fact_2", "fact_3"}},
		},
		SourceSignalIDs: []string{"s1"},
	}}
	input, err := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, id := range []string{"fact_1", "fact_2", "fact_3"} {
		if _, ok := input.FactCatalog[id]; !ok {
			t.Fatalf("missing fact %s", id)
		}
	}
	if _, ok := input.FactCatalog["fact_unrelated"]; ok {
		t.Fatal("unrelated fact must not be included")
	}
}

func TestBuildActionsAIFirstInputFromCandidates_UnknownFactErrors(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{testStockCandidate("c1")}
	candidates[0].EvidenceFactIDs = []string{"missing_fact"}
	_, err := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	if err == nil {
		t.Fatal("expected unknown fact error")
	}
}

func TestBuildActionsAIFirstInputFromCandidates_DuplicateCandidateID(t *testing.T) {
	pack := testV2CatalogPack()
	c1 := testStockCandidate("dup")
	c2 := testStockCandidate("dup")
	c2.ChunkID = "other"
	_, err := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2}, nil, nil, DefaultGenerateActionsOptions())
	if err == nil || ActionsErrorCode(err) != ActionsErrorInputDuplicateCandidateID {
		t.Fatalf("want duplicate candidate error, got %v", err)
	}
}

func TestValidateAIFirstV2_UnknownSourceCandidateID(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc",
		[]AIFirstActionCandidate{testStockCandidate("stock:candidate:01")},
		nil, nil, DefaultGenerateActionsOptions(),
	)
	output := ActionsAIFirstOutput{
		SchemaVersion: ActionsAIFirstSchemaVersionV2,
		Actions: []AIFirstDraftAction{{
			Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
			Title: "t", Reason: "r", NextStep: "n", CompletionCriteria: "c",
			EvidenceFactIDs:    []string{"fact_1"},
			AffectedEntities:   []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}},
			ObjectActions:      []AIFirstObjectAction{{EntityType: "sku", EntityID: "sku_1", RecommendedAction: "a", EvidenceFactIDs: []string{"fact_1"}}},
			SourceCandidateIDs: []string{"unknown:candidate:99"},
		}},
	}
	res := ValidateActionsAIFirstOutput(input, output)
	if res.Status != ValidationStatusInvalid {
		t.Fatalf("status=%s want invalid", res.Status)
	}
	if !issueCodePresent(res.Issues, ActionsErrorUnknownSourceCandidateID) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateAIFirstV2_FactNotGroundedInCandidates(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc",
		[]AIFirstActionCandidate{testStockCandidate("stock:candidate:01")},
		nil, nil, DefaultGenerateActionsOptions(),
	)
	output := ActionsAIFirstOutput{
		SchemaVersion: ActionsAIFirstSchemaVersionV2,
		Actions: []AIFirstDraftAction{{
			Type: ActionTypeStockReplenishment, Category: ActionCategoryStock, Priority: ActionPriorityCritical,
			Title: "t", Reason: "r", NextStep: "n", CompletionCriteria: "c",
			EvidenceFactIDs:    []string{"fact_3"},
			AffectedEntities:   []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}},
			ObjectActions:      []AIFirstObjectAction{{EntityType: "sku", EntityID: "sku_1", RecommendedAction: "a", EvidenceFactIDs: []string{"fact_3"}}},
			SourceCandidateIDs: []string{"stock:candidate:01"},
		}},
	}
	res := ValidateActionsAIFirstOutput(input, output)
	if !issueCodePresent(res.Issues, ActionsErrorFactNotGroundedInCandidates) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestGenerateActionsAIFirstV2_ZeroCandidatesNoCompose(t *testing.T) {
	pack := testV2CatalogPack()
	input, err := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", nil, nil, nil, DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	composeCalls := 0
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			composeCalls++
			return &ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{}}, &AIRawResponse{}, nil
		},
	}}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if composeCalls != 0 {
		t.Fatalf("compose calls=%d want 0", composeCalls)
	}
	if res.FinalStatus != GenerationFinalStatusEmpty {
		t.Fatalf("status=%s want empty", res.FinalStatus)
	}
}

func TestFakeProviderV2_StockGrouping(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{
		testStockCandidate("stock_positions_001:candidate:01"),
		testStockCandidate("stock_positions_002:candidate:02"),
		testStockCandidate("stock_positions_003:candidate:03"),
		testAdvertisingCandidate("campaign_001:candidate:01"),
	}
	candidates[1].AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_2", FactIDs: []string{"fact_2"}}}
	candidates[1].EvidenceFactIDs = []string{"fact_2"}
	candidates[2].AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1", FactIDs: []string{"fact_1"}}}
	input, err := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	client := NewDeterministicAIFirstFakeClient()
	out, _, err := client.ComposeAIFirstActions(context.Background(), input)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(out.Actions) >= len(candidates) {
		t.Fatalf("expected grouped actions < candidates, got %d", len(out.Actions))
	}
	stockAction := findActionWithType(out.Actions, ActionTypeStockReplenishment)
	if stockAction == nil {
		t.Fatal("missing stock action")
	}
	if len(stockAction.SourceCandidateIDs) < 3 {
		t.Fatalf("stock source ids=%v", stockAction.SourceCandidateIDs)
	}
	adv := findActionWithType(out.Actions, ActionTypeAdHighDRRReview)
	if adv == nil || len(adv.SourceCandidateIDs) != 1 {
		t.Fatalf("advertising action=%v", adv)
	}
}

func TestFakeProviderV2_Deterministic(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{testStockCandidate("c1"), testAdvertisingCandidate("c2")}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	client := NewDeterministicAIFirstFakeClient()
	out1, _, _ := client.ComposeAIFirstActions(context.Background(), input)
	out2, _, _ := client.ComposeAIFirstActions(context.Background(), input)
	b1, _ := json.Marshal(out1)
	b2, _ := json.Marshal(out2)
	if string(b1) != string(b2) {
		t.Fatal("non-deterministic fake output")
	}
}

func TestGuardActionsAIFirstInputTokens_UsesWirePayload(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc",
		[]AIFirstActionCandidate{testStockCandidate("c1")},
		nil, nil, DefaultGenerateActionsOptions(),
	)
	est, limit, err := guardActionsAIFirstInputTokens(input)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	if est > limit {
		t.Fatalf("unexpected too large: est=%d limit=%d", est, limit)
	}
	if est <= 0 {
		t.Fatalf("expected positive estimate, got %d", est)
	}
}

func TestGuardActionsAIFirstInputTokens_AtLimitAllowed(t *testing.T) {
	limit := aitokens.MaxInputTokensPerRequest()
	payload := strings.Repeat("а", limit*4)
	est, err := aitokens.EstimateJSONTokens(json.RawMessage(`{"x":"` + payload + `"}`))
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est > limit {
		t.Skip("cannot synthesize at-limit payload in this environment")
	}
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "hash", testGlobalContext(), "gc", []AIFirstActionCandidate{testStockCandidate("c1")}, nil, nil, DefaultGenerateActionsOptions())
	_, gotLimit, err := guardActionsAIFirstInputTokens(input)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	if gotLimit != limit {
		t.Fatalf("limit=%d want %d", gotLimit, limit)
	}
}

func TestAIFirstV2Prompt_ContainsKeyRules(t *testing.T) {
	prompt := aiFirstV2ComposeSystemPrompt()
	for _, fragment := range []string{
		"Action Candidates",
		"ФАЗА A",
		"object_actions",
		"source_candidate",
		"Global Context",
		"5–15",
		"max_actions",
		"object_detail_brief",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt missing fragment %q", fragment)
		}
	}
}

func TestOpenAIActionClient_V2WirePayloadExcludesDataPack(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc",
		[]AIFirstActionCandidate{testStockCandidate("c1")},
		nil, nil, DefaultGenerateActionsOptions(),
	)
	raw, err := marshalActionsAIFirstWirePayload(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"data_pack"`) {
		t.Fatal("wire payload contains data_pack")
	}
}

func issueCodePresent(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func findActionWithType(actions []AIFirstDraftAction, typ ActionType) *AIFirstDraftAction {
	for i := range actions {
		if actions[i].Type == typ {
			return &actions[i]
		}
	}
	return nil
}
