package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestDeriveEvidenceFromCandidates_AbsentAIEvidence(t *testing.T) {
	c1 := candidateWithFacts("candidate-1", "f1", "f2")
	c2 := candidateWithFacts("candidate-2", "f2", "f3")
	input := evidenceTestInput(t, c1, c2)

	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"candidate-1", "candidate-2"}, nil),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)

	if replay.ComposeReturnedCount != 1 || replay.PersistableCount != 1 {
		t.Fatalf("returned=%d persistable=%d drops=%v", replay.ComposeReturnedCount, replay.PersistableCount, replay.DropIssueCounts)
	}
	if replay.DropIssueCounts[pilotIssueMissingEvidenceFacts] > 0 || replay.DropIssueCounts[pilotIssueNoValidEvidenceAfterDerivation] > 0 {
		t.Fatalf("unexpected evidence drops: %v", replay.DropIssueCounts)
	}
	evidence := replay.Output.Actions[0].EvidenceFactIDs
	if len(evidence) != 3 || evidence[0] != "f1" || evidence[1] != "f2" || evidence[2] != "f3" {
		t.Fatalf("evidence=%v", evidence)
	}
}

func TestDeriveEvidenceFromCandidates_EmptyAIEvidence(t *testing.T) {
	c1 := candidateWithFacts("candidate-1", "f1", "f2")
	input := evidenceTestInput(t, c1)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"candidate-1"}, []string{}),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)
	if replay.PersistableCount != 1 {
		t.Fatalf("persistable=%d drops=%v", replay.PersistableCount, replay.DropIssueCounts)
	}
}

func TestDeriveEvidenceFromCandidates_UnknownAIEvidenceRemoved(t *testing.T) {
	c1 := candidateWithFacts("candidate-1", "f1", "f2")
	input := evidenceTestInput(t, c1)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"candidate-1"}, []string{"f1", "invented-fact"}),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)
	if replay.PersistableCount != 1 {
		t.Fatalf("persistable=%d drops=%v issues=%v", replay.PersistableCount, replay.DropIssueCounts, replay.Pilot.Issues)
	}
	if !replayHasWarning(replay, "unknown_evidence_fact_id_removed") {
		t.Fatalf("expected unknown_evidence warning, issues=%v", replay.Pilot.Issues)
	}
	if got := replay.Output.Actions[0].EvidenceFactIDs; len(got) != 2 || got[0] != "f1" || got[1] != "f2" {
		t.Fatalf("canonical evidence=%v", got)
	}
}

func TestDeriveEvidenceFromCandidates_UnknownCandidateRemoved(t *testing.T) {
	c1 := candidateWithFacts("valid-1", "f1")
	c2 := candidateWithFacts("valid-2", "f2")
	input := evidenceTestInput(t, c1, c2)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"valid-1", "unknown", "valid-2"}, nil),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)
	if replay.PersistableCount != 1 {
		t.Fatalf("persistable=%d drops=%v", replay.PersistableCount, replay.DropIssueCounts)
	}
	if got := replay.Output.Actions[0].EvidenceFactIDs; len(got) != 2 {
		t.Fatalf("evidence=%v", got)
	}
}

func TestDeriveEvidenceFromCandidates_NoValidCandidateEvidenceDropped(t *testing.T) {
	c1 := candidateWithFacts("candidate-empty", "")
	input := evidenceTestInput(t, c1)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"candidate-empty"}, nil),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)
	if replay.PersistableCount != 0 {
		t.Fatalf("persistable=%d want 0", replay.PersistableCount)
	}
	if replay.ComposeNorm.DroppedActionCount != 1 {
		t.Fatalf("normalization drops=%d want 1", replay.ComposeNorm.DroppedActionCount)
	}
	if replay.DropIssueCounts[pilotIssueNoValidEvidenceAfterDerivation] == 0 &&
		!replayHasFatal(replay, pilotIssueNoValidEvidenceAfterDerivation) {
		t.Fatalf("expected %s drop, counts=%v issues=%v", pilotIssueNoValidEvidenceAfterDerivation, replay.DropIssueCounts, replay.ComposeNorm.Issues)
	}
}

func TestDeriveEvidenceFromCandidates_PartialObjectDetailPreservesBaseAction(t *testing.T) {
	c1 := candidateWithFacts("candidate-1", "f1", "f2")
	input := evidenceTestInput(t, c1)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{"candidate-1"}, nil),
	}}
	merges := []ObjectDetailMergeResult{{
		AuthoritativeTotal: 1, DisplayedCount: 0, DetailCoveragePartial: true,
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, merges)
	if replay.PersistableCount != 1 {
		t.Fatalf("persistable=%d drops=%v", replay.PersistableCount, replay.DropIssueCounts)
	}
	if len(replay.Output.Actions[0].EvidenceFactIDs) == 0 {
		t.Fatal("canonical evidence lost")
	}
}

func TestReplayRun74OfflineFixture(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "debug", "run-74")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		replay := replayRun74SyntheticFixture(t)
		assertRun74ReplayExpectations(t, replay)
		return
	}
	input, compose, merges, err := loadRun74ReplayArtifacts(fixtureDir)
	if err != nil {
		t.Fatalf("load run-74 artifacts: %v", err)
	}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, merges)
	assertRun74ReplayExpectations(t, replay)
}

func replayRun74SyntheticFixture(t *testing.T) TwoPhaseActionsReplayResult {
	t.Helper()
	input := buildRun74LikeInput(t)
	compose := buildRun74LikeComposeOutput(input)
	return ReplayTwoPhaseActionsFromCompose(input, compose, nil)
}

func assertRun74ReplayExpectations(t *testing.T, replay TwoPhaseActionsReplayResult) {
	t.Helper()
	if replay.ComposeReturnedCount != 6 {
		t.Fatalf("returned=%d want 6", replay.ComposeReturnedCount)
	}
	if replay.DropIssueCounts[pilotIssueMissingEvidenceFacts] > 0 {
		t.Fatalf("missing_evidence_fact_ids drops=%d", replay.DropIssueCounts[pilotIssueMissingEvidenceFacts])
	}
	for i, action := range replay.Output.Actions {
		if len(action.EvidenceFactIDs) == 0 {
			t.Fatalf("action %d missing derived evidence", i)
		}
	}
	if replay.PersistableCount == 0 {
		t.Fatalf("persistable=0 drops=%v pilot=%v", replay.DropIssueCounts, replay.Pilot.DropIssueCounts)
	}
	t.Logf("run74 replay: returned=%d accepted=%d persistable=%d drops=%v",
		replay.ComposeReturnedCount, replay.AcceptedCount, replay.PersistableCount, replay.DropIssueCounts)
}

func buildRun74LikeInput(t *testing.T) ActionsAIFirstInput {
	t.Helper()
	pack := testRun69LikePack()
	candidates := make([]AIFirstActionCandidate, 0, 11)
	specs := []struct {
		id     string
		typ    ActionType
		cat    ActionCategory
		entity string
		facts  []string
	}{
		{"run74:c01", ActionTypeStockReplenishment, ActionCategoryStock, "sku_1", []string{"fact_1"}},
		{"run74:c02", ActionTypeStockReplenishment, ActionCategoryStock, "sku_2", []string{"sku.sku_2.stock.current_day"}},
		{"run74:c03", ActionTypeNoSalesWithStock, ActionCategorySlowTurnover, "sku_3", []string{"sku.sku_3.stock.current_day"}},
		{"run74:c04", ActionTypeSlowTurnoverReview, ActionCategorySlowTurnover, "sku_4", []string{"sku.sku_4.stock.current_day"}},
		{"run74:c05", ActionTypeStockReplenishment, ActionCategoryStock, "sku_5", []string{"sku.sku_5.stock.current_day"}},
		{"run74:c06", ActionTypeAdSpendNoOrders, ActionCategoryAdvertising, "campaign_1", []string{"campaign.1.spend.last_7d"}},
		{"run74:c07", ActionTypeAdHighDRRReview, ActionCategoryAdvertising, "campaign_1", []string{"campaign.2.spend.last_7d"}},
		{"run74:c08", ActionTypeStockReplenishment, ActionCategoryStock, "sku_6", []string{"sku.sku_6.stock.current_day"}},
		{"run74:c09", ActionTypeNoSalesWithStock, ActionCategorySlowTurnover, "sku_7", []string{"sku.sku_7.stock.current_day"}},
		{"run74:c10", ActionTypeSlowTurnoverReview, ActionCategorySlowTurnover, "sku_8", []string{"sku.sku_8.stock.current_day"}},
		{"run74:c11", ActionTypeStockReplenishment, ActionCategoryStock, "sku_9", []string{"sku.sku_9.stock.current_day"}},
	}
	for _, spec := range specs {
		entType := "sku"
		if strings.HasPrefix(spec.entity, "campaign") {
			entType = "campaign"
		}
		candidates = append(candidates, AIFirstActionCandidate{
			CandidateID: spec.id, ChunkID: "chunk", Type: spec.typ, Category: spec.cat,
			LocalPriority: ActionPriorityCritical, Problem: "problem", RecommendedAction: "act",
			WhyActionable: "why",
			AffectedEntities: []AIFirstAffectedEntityRef{{
				EntityType: entType, EntityID: spec.entity, FactIDs: spec.facts,
			}},
			EvidenceFactIDs: append([]string(nil), spec.facts...),
		})
	}
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build input: %v", err)
	}
	return input
}

func buildRun74LikeComposeOutput(input ActionsAIFirstInput) ActionsAIFirstComposeOutput {
	goals := []string{
		"replenish_stock", "reduce_excess_stock", "review_surplus_scope",
		"stop_ad_waste", "rebalance_stock", "restore_availability",
	}
	actions := make([]AIFirstComposeAction, 0, 6)
	for i := 0; i < 6; i++ {
		c := input.ActionCandidates[i]
		actions = append(actions, AIFirstComposeAction{
			ActionKey:          fmt.Sprintf("run74_action_%d", i),
			Type:               c.Type,
			Category:           c.Category,
			Priority:           ActionPriorityCritical,
			ScopeType:          AIFirstActionScopeEntity,
			OperationalGoal:    goals[i],
			SourceCandidateIDs: []string{c.CandidateID},
			TitleTemplate:      "Разобрать {object_count} объектов",
			Reason:             fakeLongReason([]string{c.Problem}, []string{c.WhyActionable}),
			Steps:              []string{"Шаг 1", "Шаг 2", "Шаг 3"},
			CompletionCriteria: "Критерий выполнен по каждому объекту",
			ObjectDetailBrief:  "Сформируй object detail по каждому entity",
		})
	}
	return ActionsAIFirstComposeOutput{SchemaVersion: ActionsAIFirstSchemaVersionV2, Actions: actions}
}

func loadRun74ReplayArtifacts(dir string) (ActionsAIFirstInput, ActionsAIFirstComposeOutput, []ObjectDetailMergeResult, error) {
	inputRaw, err := os.ReadFile(filepath.Join(dir, "actions-input.json"))
	if err != nil {
		return ActionsAIFirstInput{}, ActionsAIFirstComposeOutput{}, nil, err
	}
	var input ActionsAIFirstInput
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		return ActionsAIFirstInput{}, ActionsAIFirstComposeOutput{}, nil, err
	}

	composePaths := []string{
		"actions-compose-parsed-output.json",
		"actions_compose_parsed_output.json",
		"actions-compose-output.json",
	}
	var compose ActionsAIFirstComposeOutput
	var composeErr error
	for _, name := range composePaths {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			composeErr = err
			continue
		}
		if err := json.Unmarshal(raw, &compose); err == nil && len(compose.Actions) > 0 {
			composeErr = nil
			break
		}
		var wrapped struct {
			Actions []AIFirstComposeAction `json:"actions"`
		}
		if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Actions) > 0 {
			compose = ActionsAIFirstComposeOutput{SchemaVersion: ActionsAIFirstSchemaVersionV2, Actions: wrapped.Actions}
			composeErr = nil
			break
		}
	}
	if composeErr != nil || len(compose.Actions) == 0 {
		if composeErr == nil {
			composeErr = fmt.Errorf("compose output empty")
		}
		return ActionsAIFirstInput{}, ActionsAIFirstComposeOutput{}, nil, composeErr
	}

	var merges []ObjectDetailMergeResult
	if raw, err := os.ReadFile(filepath.Join(dir, "actions-object-detail-merge.json")); err == nil {
		var wrapped struct {
			Merges []ObjectDetailMergeResult `json:"merges"`
		}
		_ = json.Unmarshal(raw, &wrapped)
		merges = wrapped.Merges
	}
	return input, compose, merges, nil
}

func evidenceTestInput(t *testing.T, candidates ...AIFirstActionCandidate) ActionsAIFirstInput {
	t.Helper()
	pack := testV2CatalogPack()
	for _, id := range []string{"f1", "f2", "f3"} {
		pack.FactRegistry[id] = aidatapackFact(id)
	}
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build input: %v", err)
	}
	return input
}

func candidateWithFacts(id string, facts ...string) AIFirstActionCandidate {
	c := testStockCandidate(id)
	c.EvidenceFactIDs = nil
	validFacts := make([]string, 0, len(facts))
	for _, factID := range facts {
		factID = strings.TrimSpace(factID)
		if factID == "" {
			continue
		}
		validFacts = append(validFacts, factID)
	}
	c.EvidenceFactIDs = validFacts
	if len(validFacts) > 0 {
		c.AffectedEntities = []AIFirstAffectedEntityRef{{
			EntityType: "sku", EntityID: "sku_1", FactIDs: append([]string(nil), validFacts...),
		}}
	} else {
		c.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}}
	}
	return c
}

func evidenceComposeAction(key string, sourceIDs, evidence []string) AIFirstComposeAction {
	return AIFirstComposeAction{
		ActionKey:          key,
		Type:               ActionTypeStockReplenishment,
		Category:           ActionCategoryStock,
		Priority:           ActionPriorityCritical,
		ScopeType:          AIFirstActionScopeEntity,
		OperationalGoal:    key + "_goal",
		SourceCandidateIDs: append([]string(nil), sourceIDs...),
		EvidenceFactIDs:    append([]string(nil), evidence...),
		TitleTemplate:      "Разобрать {object_count} SKU",
		Reason:             fakeLongReason([]string{"problem"}, []string{"why"}),
		Steps:              []string{"Шаг 1", "Шаг 2", "Шаг 3"},
		CompletionCriteria: "Критерий выполнен",
		ObjectDetailBrief:  "detail",
	}
}

func aidatapackFact(id string) aidatapack.Fact {
	return aidatapack.Fact{FactID: id, Value: 1}
}

func replayHasWarning(replay TwoPhaseActionsReplayResult, code string) bool {
	for _, issue := range replay.Pilot.Issues {
		if issue.Code == code {
			return true
		}
	}
	for _, issue := range replay.ComposeNorm.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func replayHasFatal(replay TwoPhaseActionsReplayResult, code string) bool {
	for _, issue := range replay.ComposeNorm.Issues {
		if issue.Code == code && issue.Severity == IssueSeverityFatal {
			return true
		}
	}
	return false
}

func baseDroppedWithReason(bases []AIFirstBaseAction, reason string) bool {
	for _, b := range bases {
		if b.Dropped && b.DropReason == reason {
			return true
		}
	}
	return false
}
