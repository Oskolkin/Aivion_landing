package actions

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

type v2GenRun struct {
	result       *GenerateActionsResult
	err          error
	composeCalls int
	retryCalls   int
}

func runV2Generation(t *testing.T, input ActionsAIFirstInput, composeFn func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error)) v2GenRun {
	t.Helper()
	composeCalls := 0
	retryCalls := 0
	fake := &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			composeCalls++
			out, err := composeFn(ctx, in)
			if err != nil {
				return nil, &AIRawResponse{}, err
			}
			return out, &AIRawResponse{RawText: "fixture"}, nil
		},
		RetryAIFirstFn: func(ctx context.Context, in AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			retryCalls++
			return nil, nil, fmt.Errorf("retry must not be called")
		},
	}
	svc := &GenerationService{ai: fake, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	return v2GenRun{result: res, err: err, composeCalls: composeCalls, retryCalls: retryCalls}
}

func validV2DraftFromCandidate(c AIFirstActionCandidate) AIFirstDraftAction {
	return ValidV2DraftFromCandidateForTests(c)
}

func TestV2Traceability_EmptySourceCandidateIDs(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(c)
	draft.SourceCandidateIDs = nil
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.composeCalls != 1 || run.retryCalls != 0 {
		t.Fatalf("compose=%d retry=%d", run.composeCalls, run.retryCalls)
	}
	if run.result.FinalStatus != GenerationFinalStatusFailed {
		t.Fatalf("status=%s", run.result.FinalStatus)
	}
}

func TestV2Traceability_DuplicateSourceCandidateIDInAction(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(c)
	draft.SourceCandidateIDs = []string{c.CandidateID, c.CandidateID}
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.composeCalls != 1 || run.retryCalls != 0 || run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("compose=%d retry=%d status=%s", run.composeCalls, run.retryCalls, run.result.FinalStatus)
	}
}

func TestV2Traceability_UnknownSourceCandidateID(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(c)
	draft.SourceCandidateIDs = []string{"unknown:candidate:99"}
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.composeCalls != 1 || run.retryCalls != 0 || run.result.FinalStatus != GenerationFinalStatusFailed {
		t.Fatalf("compose=%d retry=%d status=%s", run.composeCalls, run.retryCalls, run.result.FinalStatus)
	}
}

func TestV2Traceability_DuplicateCandidateUsageAcrossActions(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	d1 := validV2DraftFromCandidate(c)
	d2 := validV2DraftFromCandidate(c)
	d2.Title = "Вторая action"
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{d1, d2}}, nil
	})
	if run.composeCalls != 1 || run.retryCalls != 0 || run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("compose=%d retry=%d status=%s", run.composeCalls, run.retryCalls, run.result.FinalStatus)
	}
}

func TestV2Traceability_FactInCatalogNotInSourceCandidates(t *testing.T) {
	pack := testV2CatalogPack()
	stock := testStockCandidate("stock:candidate:01")
	ad := testAdvertisingCandidate("ad:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{stock, ad}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(stock)
	draft.EvidenceFactIDs = []string{"fact_3"}
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.composeCalls != 1 || run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("compose=%d status=%s", run.composeCalls, run.result.FinalStatus)
	}
}

func TestStrictTraceability_DuplicateCandidateUsageAcrossActionsRejected(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	d1 := validV2DraftFromCandidate(c)
	d2 := validV2DraftFromCandidate(c)
	d2.Title = "Вторая action"
	output := ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{d1, d2}}
	res := ValidateActionsAIFirstOutput(input, output)
	if !hasFatalValidationIssues(res.Issues) {
		t.Fatalf("expected strict fatal issues, got %+v", res.Issues)
	}
}

func TestV2Traceability_EntityInCatalogNotInSourceCandidates(t *testing.T) {
	pack := testV2CatalogPack()
	stock := testStockCandidate("stock:candidate:01")
	ad := testAdvertisingCandidate("ad:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{stock, ad}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(stock)
	draft.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "campaign", EntityID: "campaign_1"}}
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.composeCalls != 1 || run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("compose=%d status=%s", run.composeCalls, run.result.FinalStatus)
	}
}

func TestV2Traceability_FactOnlyInFullDataPackRejected(t *testing.T) {
	pack := testV2CatalogPack()
	pack.FactRegistry["pack_only_fact"] = aidatapack.Fact{FactID: "pack_only_fact", Value: 1}
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(c)
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		draft.EvidenceFactIDs = []string{"pack_only_fact"}
		draft.ObjectActions[0].EvidenceFactIDs = []string{"pack_only_fact"}
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("status=%s", run.result.FinalStatus)
	}
	if len(run.result.Actions) != 1 || len(run.result.Actions[0].EvidenceFactIDs) == 0 {
		t.Fatalf("expected persisted action with derived evidence, actions=%d", len(run.result.Actions))
	}
	if run.result.Actions[0].EvidenceFactIDs[0] != "fact_1" {
		t.Fatalf("evidence=%v want candidate-derived fact_1", run.result.Actions[0].EvidenceFactIDs)
	}
}

func TestV2Traceability_IncompatibleSourceCandidateTypes(t *testing.T) {
	pack := testV2CatalogPack()
	stock := testStockCandidate("stock:candidate:01")
	ad := testAdvertisingCandidate("ad:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{stock, ad}, nil, nil, DefaultGenerateActionsOptions())
	draft := validV2DraftFromCandidate(stock)
	draft.SourceCandidateIDs = []string{stock.CandidateID, ad.CandidateID}
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, nil
	})
	if run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("status=%s", run.result.FinalStatus)
	}
}

func TestV2Traceability_Accepts20Actions(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := make([]AIFirstActionCandidate, 0, 20)
	actionsOut := make([]AIFirstDraftAction, 0, 20)
	for i := 0; i < 20; i++ {
		c := testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1))
		c.Problem = "Низкий остаток на складе"
		c.RecommendedAction = "Пополнить остатки"
		c.WhyActionable = "Риск out-of-stock"
		candidates = append(candidates, c)
		actionsOut = append(actionsOut, validV2DraftFromCandidate(c))
	}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: actionsOut}, nil
	})
	if run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("status=%s", run.result.FinalStatus)
	}
}

func TestV2Traceability_Caps21ActionsTo20(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := make([]AIFirstActionCandidate, 0, 21)
	actionsOut := make([]AIFirstDraftAction, 0, 21)
	for i := 0; i < 21; i++ {
		c := testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1))
		c.Problem = "Низкий остаток на складе"
		c.RecommendedAction = "Пополнить остатки"
		c.WhyActionable = "Риск out-of-stock"
		candidates = append(candidates, c)
		actionsOut = append(actionsOut, validV2DraftFromCandidate(c))
	}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: actionsOut}, nil
	})
	if run.composeCalls != 1 || run.retryCalls != 0 || run.result.FinalStatus != GenerationFinalStatusCompleted {
		t.Fatalf("compose=%d retry=%d status=%s", run.composeCalls, run.retryCalls, run.result.FinalStatus)
	}
	if run.result.AcceptedCount != 20 || run.result.TruncatedCount != 1 {
		t.Fatalf("accepted=%d truncated=%d", run.result.AcceptedCount, run.result.TruncatedCount)
	}
}

func TestGuardActionsAIFirstInputTokens_MatchesOutboundBytes(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{testStockCandidate("stock:candidate:01")}
	candidates[0].Problem = "Низкий остаток на складе для ключевых SKU"
	candidates[0].RecommendedAction = "Пополнить остатки по приоритетным SKU"
	candidates[0].WhyActionable = "Риск потери заказов из-за out-of-stock"
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates,
		[]string{"chunk_failed"},
		[]string{"Часть данных не была проанализирована из-за технической ошибки."},
		DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatal(err)
	}
	wireBytes, err := MarshalActionsAIFirstInputWire(input)
	if err != nil {
		t.Fatal(err)
	}
	expected := aitokens.EstimateBytesTokens(wireBytes)
	estimated, _, err := guardActionsAIFirstInputTokens(input)
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	if estimated != expected {
		t.Fatalf("guard estimated=%d expected bytes estimate=%d", estimated, expected)
	}
}

func TestGuardActionsAIFirstInputTokens_BelowLimitComposeOnce(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{testStockCandidate("c1")}, nil, nil, DefaultGenerateActionsOptions())
	wireBytes, _ := MarshalActionsAIFirstInputWire(input)
	est := aitokens.EstimateBytesTokens(wireBytes)
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est+10))
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		c := testStockCandidate("stock:candidate:01")
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{validV2DraftFromCandidate(c)}}, nil
	})
	if run.err != nil {
		t.Fatalf("unexpected err: %v", run.err)
	}
	if run.composeCalls != 1 || run.retryCalls != 0 {
		t.Fatalf("compose=%d retry=%d", run.composeCalls, run.retryCalls)
	}
}

func TestGuardActionsAIFirstInputTokens_EqualLimitComposeOnce(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{testStockCandidate("c1")}, nil, nil, DefaultGenerateActionsOptions())
	wireBytes, _ := MarshalActionsAIFirstInputWire(input)
	est := aitokens.EstimateBytesTokens(wireBytes)
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est))
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		c := testStockCandidate("stock:candidate:01")
		return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{validV2DraftFromCandidate(c)}}, nil
	})
	if run.err != nil {
		t.Fatalf("unexpected err: %v", run.err)
	}
	if run.composeCalls != 1 {
		t.Fatalf("compose=%d want 1", run.composeCalls)
	}
}

func TestGuardActionsAIFirstInputTokens_AboveLimitNoCompose(t *testing.T) {
	pack := testV2CatalogPack()
	long := strings.Repeat("пример кандидата ", 5000)
	candidates := make([]AIFirstActionCandidate, 0, 40)
	for i := 0; i < 40; i++ {
		c := testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1))
		c.Problem = long
		c.RecommendedAction = long
		c.WhyActionable = long
		candidates = append(candidates, c)
	}
	input, err := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatal(err)
	}
	wireBytes, _ := MarshalActionsAIFirstInputWire(input)
	est := aitokens.EstimateBytesTokens(wireBytes)
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est-1))
	run := runV2Generation(t, input, func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, error) {
		t.Fatal("compose must not be called")
		return nil, nil
	})
	if run.composeCalls != 0 || run.retryCalls != 0 {
		t.Fatalf("compose=%d retry=%d", run.composeCalls, run.retryCalls)
	}
	if ActionsErrorCode(run.err) != ActionsErrorInputTooLarge {
		t.Fatalf("err=%v", run.err)
	}
}

func TestFakeGrouping_CompatibleStockCandidatesMerge(t *testing.T) {
	pack := testV2CatalogPack()
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	c3 := testStockCandidate("stock:candidate:03")
	c2.Problem = c1.Problem
	c2.RecommendedAction = c1.RecommendedAction
	c3.Problem = c1.Problem
	c3.RecommendedAction = c1.RecommendedAction
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2, c3}, nil, nil, DefaultGenerateActionsOptions())
	out := buildDeterministicAIFirstV2Output(input)
	stock := findActionWithType(out.Actions, ActionTypeStockReplenishment)
	if stock == nil || len(stock.SourceCandidateIDs) != 3 {
		t.Fatalf("stock action=%v", stock)
	}
}

func TestFakeGrouping_DifferentProblemsStaySeparate(t *testing.T) {
	pack := testV2CatalogPack()
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	c2.Problem = "Другая проблема остатков"
	c2.RecommendedAction = c1.RecommendedAction
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2}, nil, nil, DefaultGenerateActionsOptions())
	out := buildDeterministicAIFirstV2Output(input)
	stockActions := 0
	for _, a := range out.Actions {
		if a.Type == ActionTypeStockReplenishment {
			stockActions++
		}
	}
	if stockActions != 2 {
		t.Fatalf("stock actions=%d want 2", stockActions)
	}
}

func TestFakeGrouping_StockAndAdvertisingSeparate(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(
		pack, "h", testGlobalContext(), "gc",
		[]AIFirstActionCandidate{testStockCandidate("s1"), testAdvertisingCandidate("a1")},
		nil, nil, DefaultGenerateActionsOptions(),
	)
	out := buildDeterministicAIFirstV2Output(input)
	if findActionWithType(out.Actions, ActionTypeStockReplenishment) == nil || findActionWithType(out.Actions, ActionTypeAdHighDRRReview) == nil {
		t.Fatalf("actions=%+v", out.Actions)
	}
}

func TestFakeGroupingKey_Deterministic(t *testing.T) {
	c := testStockCandidate("c1")
	k1 := fakeCandidateCompatibilityKey(c)
	k2 := fakeCandidateCompatibilityKey(c)
	if k1 != k2 {
		t.Fatalf("keys differ: %q %q", k1, k2)
	}
}
