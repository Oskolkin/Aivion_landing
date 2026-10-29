package actions

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type panicRetryActionsClient struct {
	FakeActionAIClient
	ComposeCalls int
	RetryCalls   int
}

func (c *panicRetryActionsClient) ComposeAIFirstActions(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	c.ComposeCalls++
	c.ComposeAIFirstCalls = append(c.ComposeAIFirstCalls, input)
	if c.ComposeAIFirstFn != nil {
		return c.ComposeAIFirstFn(ctx, input)
	}
	return nil, nil, errors.New("compose not configured")
}

func (c *panicRetryActionsClient) RetryAIFirstActions(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	c.RetryCalls++
	panic("production retry must not be called")
}

func TestGenerateActionsAIFirstV2_NoRetryOnPartial(t *testing.T) {
	pack := testV2CatalogPack()
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2}, nil, nil, DefaultGenerateActionsOptions())
	client := &panicRetryActionsClient{
		FakeActionAIClient: FakeActionAIClient{
			ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
				good := ValidDetailedPilotDraftFromCandidateForTests(c1)
				bad := ValidDetailedPilotDraftFromCandidateForTests(c2)
				bad.Title = ""
				return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{good, bad}}, &AIRawResponse{}, nil
			},
		},
	}
	svc := &GenerationService{ai: client, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.ComposeAIFirstCalls) != 1 || client.RetryCalls != 0 {
		t.Fatalf("compose=%d retry=%d", len(client.ComposeAIFirstCalls), client.RetryCalls)
	}
	if res.FinalStatus != GenerationFinalStatusCompletedPartial || res.AcceptedCount != 1 || res.DroppedCount != 1 {
		t.Fatalf("status=%s accepted=%d dropped=%d", res.FinalStatus, res.AcceptedCount, res.DroppedCount)
	}
}

func TestGenerateActionsAIFirstV2_AllInvalidFailedNoRetry(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	client := &panicRetryActionsClient{
		FakeActionAIClient: FakeActionAIClient{
			ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
				bad := ValidMinimalPilotDraftFromCandidateForTests(c)
				bad.Title = ""
				return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{bad}}, &AIRawResponse{}, nil
			},
		},
	}
	svc := &GenerationService{ai: client, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if client.RetryCalls != 0 || res.FinalStatus != GenerationFinalStatusFailed || res.FailureSubreason != ActionsErrorAllDropped {
		t.Fatalf("retry=%d status=%s reason=%s", client.RetryCalls, res.FinalStatus, res.FailureSubreason)
	}
}

func TestGenerateActionsAIFirstV2_PreservesAIOrder(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{
		testStockCandidate("stock:candidate:01"),
		testStockCandidate("stock:candidate:02"),
		testStockCandidate("stock:candidate:03"),
		testStockCandidate("stock:candidate:04"),
		testStockCandidate("stock:candidate:05"),
	}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	drafts := []AIFirstDraftAction{
		makeOrderedPilotDraft(candidates[0], ActionPriorityWatch, "A"),
		makeOrderedPilotDraft(candidates[1], ActionPriorityCritical, ""),
		makeOrderedPilotDraft(candidates[2], ActionPriorityImportant, "C"),
		makeOrderedPilotDraft(candidates[3], ActionPriorityCritical, "D"),
		makeOrderedPilotDraft(candidates[4], ActionPriorityWatch, "E"),
	}
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: drafts}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Actions) != 4 {
		t.Fatalf("actions=%d", len(res.Actions))
	}
	wantTitles := []string{"A", "C", "D", "E"}
	for i, want := range wantTitles {
		if res.Actions[i].Title != want {
			t.Fatalf("idx=%d got=%q want=%q", i, res.Actions[i].Title, want)
		}
	}
	if len(res.TopActionKeys) != 3 || res.TopActionKeys[0] != res.Actions[0].ActionKey {
		t.Fatalf("top=%v", res.TopActionKeys)
	}
}

func makeOrderedPilotDraft(c AIFirstActionCandidate, priority ActionPriority, title string) AIFirstDraftAction {
	d := ValidDetailedPilotDraftFromCandidateForTests(c)
	d.Priority = priority
	d.Title = title
	return d
}

func testStockCandidateForEntity(id, skuEntityID string) AIFirstActionCandidate {
	c := testStockCandidate(id)
	c.AffectedEntities = []AIFirstAffectedEntityRef{
		{EntityType: "sku", EntityID: skuEntityID, FactIDs: []string{"fact_2"}},
	}
	return c
}

func TestGenerateActionsAIFirstV2_AIUnavailableFailedNotEmpty(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: nil, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusFailed || res.FailureSubreason != ActionsErrorAIClientUnavailable {
		t.Fatalf("status=%s reason=%s", res.FinalStatus, res.FailureSubreason)
	}
}

func TestGenerateActionsAIFirstV2_ZeroCandidatesEmptyWithoutClient(t *testing.T) {
	pack := testV2CatalogPack()
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", nil, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: nil, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: false, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusEmpty {
		t.Fatalf("status=%s", res.FinalStatus)
	}
}

func TestGenerateActionsAIFirstV1_NoRetry(t *testing.T) {
	pack := testAIFirstDataPack()
	input := ActionsAIFirstInput{
		SchemaVersion:      ActionsAIFirstSchemaVersionV1,
		SellerAccountID:    1,
		MetricDate:         "2026-06-24",
		DataPack:           &pack,
		AllowedActionTypes: AIFirstAllowedActionTypes,
		AllowedCategories:  AIFirstAllowedCategories,
		AllowedPriorities:  []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch},
	}
	client := &panicRetryActionsClient{
		FakeActionAIClient: FakeActionAIClient{
			ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
				invalid := validAIFirstAction()
				invalid.EvidenceFactIDs = []string{"unknown.fact"}
				return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{invalid}}, &AIRawResponse{}, nil
			},
		},
	}
	svc := &GenerationService{ai: client, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.ComposeAIFirstCalls) != 1 || client.RetryCalls != 0 {
		t.Fatalf("compose=%d retry=%d", len(client.ComposeAIFirstCalls), client.RetryCalls)
	}
	if res.FinalStatus == GenerationFinalStatusCompletedAfterRetry {
		t.Fatalf("unexpected retry status %s", res.FinalStatus)
	}
}

func TestGenerateActionsAIFirstV1_AIUnavailableFailedNotEmpty(t *testing.T) {
	pack := testAIFirstDataPack()
	input := ActionsAIFirstInput{
		SchemaVersion:      ActionsAIFirstSchemaVersionV1,
		SellerAccountID:    1,
		MetricDate:         "2026-06-24",
		DataPack:           &pack,
		AllowedActionTypes: AIFirstAllowedActionTypes,
		AllowedCategories:  AIFirstAllowedCategories,
		AllowedPriorities:  []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch},
	}
	svc := &GenerationService{ai: nil, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusFailed || res.FailureSubreason != ActionsErrorAIClientUnavailable {
		t.Fatalf("status=%s reason=%s", res.FinalStatus, res.FailureSubreason)
	}
}

func TestGenerateActionsAIFirstV2_EmptyProviderOutputFailed(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{}}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusFailed || res.FailureSubreason != ActionsErrorEmptyOutputFromNonEmptyCandidates {
		t.Fatalf("status=%s reason=%s", res.FinalStatus, res.FailureSubreason)
	}
}

func TestGenerateActionsAIFirstV2_OptionalFieldsMapWithoutPanic(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	draft := ValidDetailedPilotDraftFromCandidateForTests(c)
	draft.Summary = ""
	draft.ExpectedEffect = ""
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{draft}}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Actions) != 1 {
		t.Fatalf("actions=%d", len(res.Actions))
	}
	if res.Actions[0].Summary != "" || res.Actions[0].ExpectedEffect != "" {
		t.Fatalf("optional fields should be empty: %+v", res.Actions[0])
	}
}

func TestGenerateActionsAIFirstV2_PartialCounts(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := make([]AIFirstActionCandidate, 0, 10)
	for i := 0; i < 10; i++ {
		candidates = append(candidates, testStockCandidate(fmt.Sprintf("stock:candidate:%02d", i+1)))
	}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			actions := make([]AIFirstDraftAction, 0, 10)
			for i, c := range candidates {
				d := ValidDetailedPilotDraftFromCandidateForTests(c)
				d.Title = fmt.Sprintf("Action %02d", i+1)
				if i == 1 || i == 7 {
					d.Title = ""
				}
				actions = append(actions, d)
			}
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: actions}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReturnedCount != 10 || res.AcceptedCount != 8 || res.DroppedCount != 2 || res.FinalStatus != GenerationFinalStatusCompletedPartial {
		t.Fatalf("returned=%d accepted=%d dropped=%d status=%s", res.ReturnedCount, res.AcceptedCount, res.DroppedCount, res.FinalStatus)
	}
}

func TestGenerateActionsAIFirstV2_AIDisabledBeforeOversizedInput(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := make([]AIFirstActionCandidate, 0, 200)
	for i := 0; i < 200; i++ {
		candidates = append(candidates, testStockCandidate(fmt.Sprintf("stock:candidate:%03d", i+1)))
	}
	input, err := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatal(err)
	}
	svc := &GenerationService{ai: &FakeActionAIClient{}, repo: newFakeGenerationRepo()}
	res, genErr := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: false, DryRun: true})
	if genErr != nil {
		t.Fatal(genErr)
	}
	if res.FinalStatus != GenerationFinalStatusFailed || res.FailureSubreason != ActionsErrorAIDisabled || res.ComposeCalls != 0 {
		t.Fatalf("status=%s reason=%s compose=%d", res.FinalStatus, res.FailureSubreason, res.ComposeCalls)
	}
}

func pilotFunnelDraftWithoutFunnelEntity(c AIFirstActionCandidate) AIFirstDraftAction {
	d := ValidDetailedPilotDraftFromCandidateForTests(c)
	d.Type = ActionTypeFunnelWeakTransition
	d.Category = ActionCategoryFunnel
	d.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}}
	d.ObjectActions = nil
	return d
}

func TestGenerateActionsAIFirstV2_PersistGuardDropPartial(t *testing.T) {
	pack := testV2CatalogPack()
	c1 := testStockCandidate("stock:candidate:01")
	c2 := testStockCandidate("stock:candidate:02")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c1, c2}, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			good := ValidDetailedPilotDraftFromCandidateForTests(c1)
			good.Title = "Good"
			bad := ValidDetailedPilotDraftFromCandidateForTests(c2)
			bad.Title = ""
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{good, bad}}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusCompletedPartial || res.AcceptedCount != 1 || res.DroppedCount != 1 {
		t.Fatalf("status=%s accepted=%d dropped=%d", res.FinalStatus, res.AcceptedCount, res.DroppedCount)
	}
	if len(res.TopActionKeys) != 1 || res.TopActionKeys[0] != res.Actions[0].ActionKey {
		t.Fatalf("top=%v actions=%d", res.TopActionKeys, len(res.Actions))
	}
}

func TestGenerateActionsAIFirstV2_AllPersistGuardDroppedFailed(t *testing.T) {
	pack := testV2CatalogPack()
	c := testStockCandidate("stock:candidate:01")
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", []AIFirstActionCandidate{c}, nil, nil, DefaultGenerateActionsOptions())
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			bad := pilotFunnelDraftWithoutFunnelEntity(c)
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: []AIFirstDraftAction{bad}}, &AIRawResponse{}, nil
		},
	}, repo: newFakeGenerationRepo()}
	res, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalStatus != GenerationFinalStatusFailed || res.AcceptedCount != 0 || res.DroppedCount != 1 {
		t.Fatalf("status=%s accepted=%d dropped=%d", res.FinalStatus, res.AcceptedCount, res.DroppedCount)
	}
	if res.FailureSubreason != ActionsErrorAllDropped {
		t.Fatalf("reason=%s", res.FailureSubreason)
	}
}

func TestGenerateActionsAIFirstV2_ListActionsPreservesPersistedOrder(t *testing.T) {
	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{
		testStockCandidateForEntity("stock:candidate:01", "sku_1"),
		testStockCandidateForEntity("stock:candidate:02", "sku_2"),
		testAdvertisingCandidate("ad:candidate:01"),
		testStockCandidateForEntity("stock:candidate:03", "sku_2"),
		func() AIFirstActionCandidate {
			c := testStockCandidateForEntity("stock:candidate:04", "sku_1")
			c.Type = ActionTypeSKUOrdersDrop
			c.Category = ActionCategorySKU
			return c
		}(),
	}
	input, _ := BuildActionsAIFirstInputFromCandidates(pack, "h", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions())
	drafts := []AIFirstDraftAction{
		makeOrderedPilotDraft(candidates[0], ActionPriorityWatch, "A"),
		makeOrderedPilotDraft(candidates[1], ActionPriorityCritical, ""),
		makeOrderedPilotDraft(candidates[2], ActionPriorityImportant, "C"),
		makeOrderedPilotDraft(candidates[3], ActionPriorityCritical, "D"),
		makeOrderedPilotDraft(candidates[4], ActionPriorityWatch, "E"),
	}
	repo := newFakeGenerationRepo()
	svc := &GenerationService{ai: &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, in ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			return &ActionsAIFirstOutput{SchemaVersion: in.SchemaVersion, Actions: drafts}, &AIRawResponse{}, nil
		},
	}, repo: repo}
	genRes, err := svc.GenerateActionsAIFirstFromInput(context.Background(), input, GenerateActionsOptions{AIEnabled: true, DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	wantTitles := []string{"A", "C", "D", "E"}
	if len(genRes.Actions) != len(wantTitles) {
		t.Fatalf("generated actions=%d", len(genRes.Actions))
	}
	for i, want := range wantTitles {
		if genRes.Actions[i].Title != want {
			t.Fatalf("gen idx=%d got=%q want=%q", i, genRes.Actions[i].Title, want)
		}
	}
	if len(genRes.TopActionKeys) != 3 {
		t.Fatalf("top keys=%d", len(genRes.TopActionKeys))
	}
	for i := 0; i < 3; i++ {
		if genRes.TopActionKeys[i] != genRes.Actions[i].ActionKey {
			t.Fatalf("top[%d]=%q action=%q", i, genRes.TopActionKeys[i], genRes.Actions[i].ActionKey)
		}
	}

	readSvc := NewService(repo)
	listRes, err := readSvc.ListActions(context.Background(), input.SellerAccountID, ListActionsFilters{Status: string(ActionStatusOpen), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(listRes.Actions) != len(wantTitles) {
		t.Fatalf("listed actions=%d", len(listRes.Actions))
	}
	for i, want := range wantTitles {
		if listRes.Actions[i].Title != want {
			t.Fatalf("list idx=%d got=%q want=%q", i, listRes.Actions[i].Title, want)
		}
	}
}
