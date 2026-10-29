package actions

import (
	"fmt"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func pilotInput(candidates ...AIFirstActionCandidate) ActionsAIFirstInput {
	pack := testV2CatalogPack()
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		panic(err)
	}
	return input
}

func minimalPilotDraft(c AIFirstActionCandidate) AIFirstDraftAction {
	return ValidDetailedPilotDraftFromCandidateForTests(c)
}

func TestValidateActionsAIFirstOutputPilot_MinimalActionAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{
		SchemaVersion: input.SchemaVersion,
		Actions:       []AIFirstDraftAction{minimalPilotDraft(c)},
	})
	if res.AcceptedCount != 1 || res.DroppedCount != 0 {
		t.Fatalf("accepted=%d dropped=%d", res.AcceptedCount, res.DroppedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_ActionPlanInsteadOfNextStep(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.NextStep = ""
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_MissingBothNextStepAndPlanDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.NextStep = ""
	draft.ActionPlan = nil
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingNextStepOrPlan) {
		t.Fatalf("dropped=%d issues=%v", res.DroppedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_InvalidTypeDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Type = ActionType("unknown_type")
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueInvalidActionType) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_InvalidCategoryDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Category = ActionCategory("unknown")
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueInvalidActionCategory) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_InvalidPriorityDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Priority = ActionPriority("urgent_now")
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueInvalidActionPriority) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptyTitleDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Title = "  "
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingTitle) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptyReasonDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Reason = ""
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingReason) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptyCompletionCriteriaDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.CompletionCriteria = ""
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingCompletion) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptyAffectedEntitiesDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.AffectedEntities = nil
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingAffected) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptyEvidenceFactsDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	compose := ActionsAIFirstComposeOutput{Actions: []AIFirstComposeAction{
		evidenceComposeAction("a1", []string{c.CandidateID}, nil),
	}}
	replay := ReplayTwoPhaseActionsFromCompose(input, compose, nil)
	if replay.PersistableCount != 1 {
		t.Fatalf("persistable=%d drops=%v", replay.PersistableCount, replay.DropIssueCounts)
	}
	if replay.DropIssueCounts[pilotIssueMissingEvidenceFacts] > 0 {
		t.Fatalf("legacy missing_evidence drop should not occur after derivation: %v", replay.DropIssueCounts)
	}
}

func TestValidateActionsAIFirstOutputPilot_EmptySourceCandidatesDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.SourceCandidateIDs = nil
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueMissingSourceIDs) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_UnknownSKUDroppedOthersKept(t *testing.T) {
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	input := pilotInput(c1, c2)
	bad := minimalPilotDraft(c1)
	bad.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "unknown_sku"}}
	good := minimalPilotDraft(c2)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{
		SchemaVersion: input.SchemaVersion,
		Actions:       []AIFirstDraftAction{bad, good},
	})
	if res.AcceptedCount != 1 || res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueUnknownEntity) {
		t.Fatalf("accepted=%d dropped=%d", res.AcceptedCount, res.DroppedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_UnknownCampaignDropped(t *testing.T) {
	c := testAdvertisingCandidate("ad:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "campaign", EntityID: "missing_campaign"}}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueUnknownEntity) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_UnknownFactDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.EvidenceFactIDs = []string{"missing_fact"}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueUnknownFact) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_UnknownCandidateDropped(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.SourceCandidateIDs = []string{"missing:candidate"}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueUnknownSourceID) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_FactOnlyInDataPackDropped(t *testing.T) {
	pack := testV2CatalogPack()
	pack.FactRegistry["pack_only_fact"] = aidatapack.Fact{FactID: "pack_only_fact", Value: 1}
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := minimalPilotDraft(c)
	draft.EvidenceFactIDs = []string{"pack_only_fact"}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.DroppedCount != 1 || !pilotHasIssue(res, pilotIssueUnknownFact) {
		t.Fatalf("issues=%v", res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_MissingExpectedEffectAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_MissingSummaryAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.Summary = ""
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_MissingObjectActionsWarningOnly(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidMinimalPilotDraftFromCandidateForTests(c)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 || !pilotHasIssue(res, "missing_object_actions") {
		t.Fatalf("accepted=%d issues=%v", res.AcceptedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_InsufficientActionPlanWarningOnly(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.ActionPlan = draft.ActionPlan[:1]
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 || !pilotHasIssue(res, "insufficient_action_plan") {
		t.Fatalf("accepted=%d issues=%v", res.AcceptedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_GenericActionStepWarningOnly(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.ActionPlan[0].Instruction = "Провести анализ ситуации"
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 || !pilotHasIssue(res, "generic_action_step") {
		t.Fatalf("accepted=%d issues=%v", res.AcceptedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_DetailedActionAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 || res.DroppedCount != 0 {
		t.Fatalf("accepted=%d dropped=%d", res.AcceptedCount, res.DroppedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_MissingObjectActionsAccepted(t *testing.T) {
	t.Skip("superseded by TestValidateActionsAIFirstOutputPilot_MissingObjectActionsDropped")
}

func TestValidateActionsAIFirstOutputPilot_GenericPhraseWarningOnlyWhenWeak(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidMinimalPilotDraftFromCandidateForTests(c)
	draft.Reason = "Проверьте остатки по SKU"
	draft.NextStep = "Обратите внимание на остатки"
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_NoLimitationsAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := minimalPilotDraft(c)
	draft.VisibleLimitations = nil
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_UnusedCandidatesAccepted(t *testing.T) {
	candidates := make([]AIFirstActionCandidate, 0, 5)
	for i := 0; i < 5; i++ {
		candidates = append(candidates, testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1)))
	}
	input := pilotInput(candidates...)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{
		SchemaVersion: input.SchemaVersion,
		Actions:       []AIFirstDraftAction{minimalPilotDraft(candidates[0]), minimalPilotDraft(candidates[1])},
	})
	if res.AcceptedCount != 2 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_SemanticDuplicatesAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	d1 := minimalPilotDraft(c)
	d2 := minimalPilotDraft(c)
	d2.Title = d1.Title
	d2.Reason = d1.Reason
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{d1, d2}})
	if res.AcceptedCount != 2 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_ReusedCandidateAccepted(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	d1 := minimalPilotDraft(c)
	d2 := minimalPilotDraft(c)
	d2.Title = "Вторая action"
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{d1, d2}})
	if res.AcceptedCount != 2 {
		t.Fatalf("accepted=%d", res.AcceptedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_FactNotTiedToSourceCandidateAccepted(t *testing.T) {
	stock := testStockCandidate("stock:candidate:01")
	ad := testAdvertisingCandidate("ad:candidate:01")
	input := pilotInput(stock, ad)
	draft := minimalPilotDraft(stock)
	draft.EvidenceFactIDs = []string{"fact_3"}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d issues=%v", res.AcceptedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_DifferentCandidateTypesAccepted(t *testing.T) {
	stock := testStockCandidate("stock:candidate:01")
	ad := testAdvertisingCandidate("ad:candidate:01")
	input := pilotInput(stock, ad)
	draft := minimalPilotDraft(stock)
	draft.SourceCandidateIDs = []string{stock.CandidateID, ad.CandidateID}
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if res.AcceptedCount != 1 {
		t.Fatalf("accepted=%d issues=%v", res.AcceptedCount, res.Issues)
	}
}

func TestValidateActionsAIFirstOutputPilot_CapsAt20(t *testing.T) {
	candidates := make([]AIFirstActionCandidate, 0, 25)
	actions := make([]AIFirstDraftAction, 0, 25)
	for i := 0; i < 25; i++ {
		c := testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1))
		candidates = append(candidates, c)
		actions = append(actions, minimalPilotDraft(c))
	}
	input := pilotInput(candidates...)
	res := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: actions})
	if res.ValidBeforeLimit != 25 || res.AcceptedCount != 20 || res.TruncatedCount != 5 {
		t.Fatalf("valid=%d accepted=%d truncated=%d", res.ValidBeforeLimit, res.AcceptedCount, res.TruncatedCount)
	}
}

func TestValidateActionsAIFirstOutputPilot_StrictRejectsMissingExpectedEffect(t *testing.T) {
	c := testStockCandidate("stock:candidate:01")
	input := pilotInput(c)
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.ExpectedEffect = ""
	strict := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	pilot := ValidateActionsAIFirstOutputPilot(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{draft}})
	if pilot.AcceptedCount != 1 {
		t.Fatalf("pilot accepted=%d", pilot.AcceptedCount)
	}
	if len(strict.AcceptedActions) != 0 {
		t.Fatalf("strict should drop without expected_effect, accepted=%d", len(strict.AcceptedActions))
	}
}

func pilotHasIssue(res PilotActionsValidationResult, code string) bool {
	for _, issue := range res.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
