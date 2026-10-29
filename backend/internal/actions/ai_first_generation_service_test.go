package actions

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateActions_AIFirstFalse_UsesLegacyCandidatePath(t *testing.T) {
	repo := newFakeGenerationRepo()
	builder := buildTestCandidateBuilder(t)
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn:  defaultFakeReviewFn(),
	}
	svc := NewGenerationService(builder, fakeAI, repo)

	opts := DefaultGenerateActionsOptions()
	opts.AIFirst = false
	_, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(fakeAI.ComposeCalls) == 0 {
		t.Fatal("expected legacy ComposeDomainActions call when AIFirst=false")
	}
	if len(fakeAI.ComposeAIFirstCalls) != 0 {
		t.Fatal("ai-first compose must not be called when AIFirst=false")
	}
}

func TestGenerateActions_AIFirstTrue_UsesAIFirstPathWithoutCandidateBuilder(t *testing.T) {
	pack := testAIFirstDataPack()
	dataPackBuilder := &fakeAIDataPackBuilder{pack: &pack}
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			return &ActionsAIFirstOutput{
				SchemaVersion: input.SchemaVersion,
				Actions:       []AIFirstDraftAction{validAIFirstAction()},
			}, &AIRawResponse{Model: "fake-ai-first"}, nil
		},
	}
	svc := NewGenerationService(nil, fakeAI, repo).WithAIDataPackBuilder(dataPackBuilder)

	opts := DefaultGenerateActionsOptions()
	opts.AIFirst = true
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Fatal("expected AI-first actions")
	}
	if len(fakeAI.ComposeCalls) != 0 {
		t.Fatalf("legacy ComposeDomainActions should not be called in AI-first mode")
	}
	if len(fakeAI.ComposeAIFirstCalls) != 1 {
		t.Fatalf("ComposeAIFirstActions calls=%d want 1", len(fakeAI.ComposeAIFirstCalls))
	}
}

func TestGenerateActions_AIFirstTrue_NoRetryOnRepairable(t *testing.T) {
	pack := testAIFirstDataPack()
	dataPackBuilder := &fakeAIDataPackBuilder{pack: &pack}
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			invalid := validAIFirstAction()
			invalid.EvidenceFactIDs = []string{"unknown.fact"}
			return &ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{invalid}}, &AIRawResponse{Model: "first"}, nil
		},
		RetryAIFirstFn: func(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			t.Fatal("retry must not be called")
			return nil, nil, nil
		},
	}
	svc := NewGenerationService(nil, fakeAI, repo).WithAIDataPackBuilder(dataPackBuilder)

	opts := DefaultGenerateActionsOptions()
	opts.AIFirst = true
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("retry calls=%d want 0", len(fakeAI.RetryAIFirstCalls))
	}
	if result.FinalStatus == GenerationFinalStatusCompletedAfterRetry {
		t.Fatalf("final status=%q", result.FinalStatus)
	}
}

func TestGenerateActions_AIFirstTrue_DeterministicFakeE2E(t *testing.T) {
	pack := testAIFirstDataPack()
	dataPackBuilder := &fakeAIDataPackBuilder{pack: &pack}
	repo := newFakeGenerationRepo()
	fakeAI := NewDeterministicAIFirstFakeClient()
	svc := NewGenerationService(nil, fakeAI, repo).WithAIDataPackBuilder(dataPackBuilder)

	opts := DefaultGenerateActionsOptions()
	opts.AIFirst = true
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) < 2 {
		t.Fatalf("expected at least 2 persisted actions, got %d", len(result.Actions))
	}
	if len(fakeAI.ComposeCalls) != 0 {
		t.Fatalf("candidate-first compose must not be called in AI-first mode")
	}
	if len(fakeAI.ComposeAIFirstCalls) != 1 {
		t.Fatalf("ComposeAIFirstActions calls=%d want 1", len(fakeAI.ComposeAIFirstCalls))
	}
	in := fakeAI.ComposeAIFirstCalls[0]
	if in.DataPack.SchemaVersion == "" || in.DataPack.SellerContext.SellerAccountID == 0 {
		t.Fatal("expected AI-first input built from Seller AI Data Pack")
	}
	if result.FinalStatus != GenerationFinalStatusCompleted && result.FinalStatus != GenerationFinalStatusCompletedPartial {
		t.Fatalf("final status=%q", result.FinalStatus)
	}

	seenStock := false
	seenAdReview := false
	for _, item := range result.Actions {
		if len(item.SourceCandidateIDs) != 0 {
			t.Fatalf("ai-first action must not require source_candidate_ids: %+v", item.SourceCandidateIDs)
		}
		for _, cta := range item.CTAs {
			k := strings.TrimSpace(strings.ToLower(cta.Type))
			if k == ActionCTAOpenSKUList || k == ActionCTAOpenCampaignList ||
				k == "download_xlsx" || k == "export" || k == "copy" || k == "snooze" {
				t.Fatalf("disallowed cta found: %q", k)
			}
		}
		if item.Type == ActionTypeStockReplenishment {
			seenStock = true
			if item.OperationalDetail == nil || len(item.OperationalDetail.ObjectActions) == 0 {
				t.Fatal("stock_replenishment must include object_actions")
			}
		}
		if item.Type == ActionTypeAdHighDRRReview {
			seenAdReview = true
			if item.OperationalDetail == nil || len(item.OperationalDetail.ObjectActions) == 0 {
				t.Fatal("ad_high_drr_review must include object_actions")
			}
			if len(item.VisibleLimitations) == 0 {
				t.Fatal("ad_high_drr_review must include visible limitations")
			}
		}
	}
	if !seenStock {
		t.Fatal("expected stock_replenishment action from deterministic fake provider")
	}
	if !seenAdReview {
		t.Fatal("expected ad_high_drr_review action from deterministic fake provider")
	}
}
