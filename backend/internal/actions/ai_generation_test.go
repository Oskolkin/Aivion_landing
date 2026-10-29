package actions

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeGenerationRepo struct {
	mockRepo
	mu sync.Mutex

	runs         map[int64]*generationRunState
	categoryRuns []categoryRunState
	nextRunID    int64
	items        map[string]ActionItemRecord
	userStates   map[string]string
	itemOrder    []string
	createdSeq   int
	baseTime     time.Time

	persistCalls int
	persistErr   error
}

type generationRunState struct {
	ID              int64
	SellerAccountID int64
	MetricDate      time.Time
	Status          string
	ContextHash     string
	FinalStatus     string
	AcceptedCount   int
	RepairedCount   int
	DroppedCount    int
	FallbackCount   int
	AIFocusJSON     []byte
	TopActionKeys   []byte
	DataQuality     []byte
}

type categoryRunState struct {
	ID                  int64
	RunID               int64
	Domain              string
	Status              string
	Model               string
	InputContextJSON    []byte
	RawResponseJSON     []byte
	ValidationJSON      []byte
	FinalValidationJSON []byte
}

func newFakeGenerationRepo() *fakeGenerationRepo {
	return &fakeGenerationRepo{
		mockRepo:   mockRepo{items: []ActionItemRecord{}},
		runs:       make(map[int64]*generationRunState),
		items:      make(map[string]ActionItemRecord),
		userStates: make(map[string]string),
		baseTime:   time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeGenerationRepo) CreateActionGenerationRun(ctx context.Context, sellerAccountID int64, metricDate time.Time, status string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextRunID++
	id := f.nextRunID
	f.runs[id] = &generationRunState{
		ID:              id,
		SellerAccountID: sellerAccountID,
		MetricDate:      metricDate,
		Status:          status,
	}
	return id, nil
}

func (f *fakeGenerationRepo) UpdateActionGenerationRun(ctx context.Context, params UpdateGenerationRunParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[params.RunID]
	if !ok {
		return nil
	}
	run.Status = params.Status
	run.ContextHash = params.ContextHash
	run.FinalStatus = params.FinalStatus
	run.AcceptedCount = params.AcceptedCount
	run.RepairedCount = params.RepairedCount
	run.DroppedCount = params.DroppedCount
	run.FallbackCount = params.FallbackCount
	run.AIFocusJSON = params.AIFocusJSON
	run.TopActionKeys = params.TopActionKeysJSON
	run.DataQuality = params.DataQualityNotes
	if IsPublishableActionFinalStatus(params.FinalStatus) || IsEmptyActionFinalStatus(params.FinalStatus) {
		md := run.MetricDate
		f.latestRun = &ActionGenerationRunRecord{
			ID:              run.ID,
			SellerAccountID: run.SellerAccountID,
			MetricDate:      &md,
			Status:          params.Status,
			FinalStatus:     params.FinalStatus,
		}
		f.latestMetric = &md
	}
	return nil
}

func (f *fakeGenerationRepo) GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.latestRun != nil && f.latestRun.SellerAccountID == sellerAccountID {
		rec := *f.latestRun
		return &rec, nil
	}
	return nil, nil
}

func (f *fakeGenerationRepo) GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*ActionGenerationRunRecord, error) {
	run, err := f.GetLatestPublishableRun(ctx, sellerAccountID)
	if err != nil || run == nil {
		return run, err
	}
	if pipelineRunID > 0 {
		run.AiPipelineRunID = pipelineRunID
	}
	return run, nil
}

func (f *fakeGenerationRepo) ListActionsByRunID(ctx context.Context, runID int64, params ListActionsParams) ([]ActionItemRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ActionItemRecord, 0)
	for _, key := range f.itemOrder {
		item, ok := f.items[key]
		if !ok || item.RunID == nil || *item.RunID != runID {
			continue
		}
		if params.Category != nil && item.Category != *params.Category {
			continue
		}
		if params.Priority != nil && item.Priority != *params.Priority {
			continue
		}
		if params.Status != "" && params.Status != "all" && string(item.Status) != params.Status {
			continue
		}
		out = append(out, item)
	}
	if params.Offset >= len(out) {
		return []ActionItemRecord{}, nil
	}
	end := len(out)
	if params.Limit > 0 && params.Offset+params.Limit < end {
		end = params.Offset + params.Limit
	}
	return out[params.Offset:end], nil
}

func (f *fakeGenerationRepo) CreateActionGenerationCategoryRun(ctx context.Context, params CreateCategoryRunParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := int64(len(f.categoryRuns) + 1)
	f.categoryRuns = append(f.categoryRuns, categoryRunState{
		ID:                  id,
		RunID:               params.RunID,
		Domain:              params.Domain,
		Status:              params.Status,
		Model:               params.Model,
		InputContextJSON:    params.InputContextJSON,
		RawResponseJSON:     params.RawResponseJSON,
		ValidationJSON:      params.ValidationJSON,
		FinalValidationJSON: params.FinalValidationJSON,
	})
	return id, nil
}

func (f *fakeGenerationRepo) UpdateActionGenerationCategoryRun(ctx context.Context, params UpdateCategoryRunParams) error {
	return nil
}

func (f *fakeGenerationRepo) UpsertActionItem(ctx context.Context, params UpsertActionItemParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := params.ActionKey
	existing, exists := f.items[key]
	rec := ActionItemRecord{
		ID:                   existing.ID,
		SellerAccountID:      params.SellerAccountID,
		RunID:                &params.RunID,
		ActionKey:            params.ActionKey,
		Type:                 params.Type,
		Category:             params.Category,
		Priority:             params.Priority,
		Status:               ActionStatusOpen,
		Title:                params.Title,
		Summary:              params.Summary,
		Reason:               params.Reason,
		NextStep:             params.NextStep,
		ExpectedEffect:       params.ExpectedEffect,
		EvidenceJSON:         params.EvidenceJSON,
		LimitationsJSON:      params.LimitationsJSON,
		CTAJSON:              params.CTAJSON,
		AffectedEntitiesJSON: params.AffectedEntitiesJSON,
		EntityGroupRef:       params.EntityGroupRef,
		SourceCandidateIDs:   params.SourceCandidateIDs,
		EvidenceFactIDs:      params.EvidenceFactIDs,
		MetricDate:           &params.MetricDate,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	if !exists {
		rec.ID = int64(len(f.items) + 1)
		f.createdSeq++
		rec.CreatedAt = f.baseTime.Add(time.Duration(f.createdSeq) * time.Millisecond)
		f.itemOrder = append(f.itemOrder, key)
	}
	if st, ok := f.userStates[key]; ok {
		rec.Status = ActionStatus(st)
	}
	f.items[key] = rec
	// sync mockRepo items slice
	f.mockRepo.items = make([]ActionItemRecord, 0, len(f.items))
	for _, v := range f.items {
		f.mockRepo.items = append(f.mockRepo.items, v)
	}
	return nil
}

func (f *fakeGenerationRepo) DeleteStaleActionItemsForDate(ctx context.Context, sellerAccountID int64, metricDate time.Time, keepKeys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	keep := make(map[string]bool)
	for _, k := range keepKeys {
		keep[k] = true
	}
	for key, item := range f.items {
		if item.SellerAccountID != sellerAccountID || item.MetricDate == nil {
			continue
		}
		if !sameDate(*item.MetricDate, metricDate) {
			continue
		}
		if !keep[key] {
			delete(f.items, key)
		}
	}
	f.mockRepo.items = make([]ActionItemRecord, 0, len(f.items))
	for _, v := range f.items {
		f.mockRepo.items = append(f.mockRepo.items, v)
	}
	return nil
}

func (f *fakeGenerationRepo) LinkActionGenerationRunToPipeline(ctx context.Context, pipelineRunID, actionsRunID int64, dataPackHash, globalContextHash string) error {
	return nil
}

func (f *fakeGenerationRepo) PersistGenerationAtomic(ctx context.Context, params PersistGenerationParams) error {
	f.persistCalls++
	if f.persistErr != nil {
		return f.persistErr
	}
	if err := f.UpdateActionGenerationRun(ctx, params.RunUpdate); err != nil {
		return err
	}
	for _, item := range params.ActionItems {
		if err := f.UpsertActionItem(ctx, item); err != nil {
			return err
		}
	}
	if params.StaleDelete != nil {
		sd := params.StaleDelete
		if sd.RunID <= 0 {
			return fmt.Errorf("stale delete requires run_id")
		}
		keep := make(map[string]bool)
		for _, k := range sd.KeepKeys {
			keep[k] = true
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for key, item := range f.items {
			if item.RunID == nil || *item.RunID != sd.RunID {
				continue
			}
			if !keep[key] {
				delete(f.items, key)
			}
		}
	}
	return nil
}

var _ GenerationRepository = (*fakeGenerationRepo)(nil)

func (f *fakeGenerationRepo) categoryRunByDomain(domain string) *categoryRunState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.categoryRuns {
		if f.categoryRuns[i].Domain == domain {
			return &f.categoryRuns[i]
		}
	}
	return nil
}

func buildTestCandidateBuilder(t *testing.T) *CandidateBuilder {
	t.Helper()
	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle([]SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 10
			r.DaysOfCover = ptrCoverage(5)
			r.Revenue7d = 7000
			r.Orders7d = 7
		}),
	})
	repo.accountSnapshot = AccountSnapshotRow{}
	return NewCandidateBuilder(repo, DefaultCandidateBuilderConfig())
}

func TestGenerateActions_AIDisabledFallback(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{}
	svc := NewGenerationService(builder, fakeAI, repo)

	opts := DefaultGenerateActionsOptions()
	opts.AIEnabled = false

	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Fatalf("expected fallback actions")
	}
	if result.FinalStatus != GenerationFinalStatusFallback && result.FinalStatus != GenerationFinalStatusCompletedPartial {
		t.Fatalf("final_status=%q", result.FinalStatus)
	}
	if len(fakeAI.ComposeCalls) != 0 {
		t.Fatalf("AI should not be called when disabled")
	}
}

func TestGenerateActions_RepairFlow(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
			if len(input.DomainContext.Candidates) == 0 {
				return &DomainComposerOutput{Domain: input.Domain, Actions: nil}, &AIRawResponse{Model: "fake"}, nil
			}
			cand := input.DomainContext.Candidates[0]
			action, ok := buildFallbackActionForCandidate(cand)
			if !ok {
				return &DomainComposerOutput{Domain: input.Domain, Actions: nil}, &AIRawResponse{Model: "fake"}, nil
			}
			action.Title = "Проверьте остатки"
			return &DomainComposerOutput{Domain: input.Domain, Actions: []DomainAIAction{action}}, &AIRawResponse{Model: "fake"}, nil
		},
		RepairFn: func(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error) {
			if len(input.DomainContext.Candidates) == 0 {
				return &DomainComposerOutput{Domain: input.DomainContext.Domain, Actions: nil}, &AIRawResponse{Model: "fake-repair"}, nil
			}
			fixed, _ := buildFallbackActionForCandidate(input.DomainContext.Candidates[0])
			return &DomainComposerOutput{Domain: input.DomainContext.Domain, Actions: []DomainAIAction{fixed}}, &AIRawResponse{Model: "fake-repair"}, nil
		},
		ReviewFn: func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
			refs := make([]LegacyGlobalCheckActionRef, 0, len(input.DraftActions))
			top := make([]string, 0)
			for _, d := range input.DraftActions {
				refs = append(refs, LegacyGlobalCheckActionRef{TempActionID: d.TempActionID})
				top = append(top, d.TempActionID)
			}
			return &LegacyGlobalCheckOutput{
				AIFocus:           &ActionsAIFocus{Status: "available", Source: "ai", Headline: "Фокус", Summary: "Сводка"},
				OrderedActionRefs: refs,
				TopActionRefs:     top,
			}, &AIRawResponse{Model: "fake-review"}, nil
		},
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	opts := DefaultGenerateActionsOptions()

	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Fatalf("expected actions after repair")
	}
	if len(fakeAI.RepairCalls) == 0 {
		t.Fatalf("expected repair call")
	}
}

func TestGenerateActions_InvalidJSONFallback(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
			return nil, &AIRawResponse{RawText: "not json"}, nil
		},
		ReviewFn: defaultFakeReviewFn(),
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Fatalf("expected fallback actions after invalid JSON")
	}
}

func TestGenerateActions_LegacyGlobalCheckFailureDoesNotFailGeneration(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn: func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
			return nil, nil, context.DeadlineExceeded
		},
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Fatalf("expected actions despite legacy global check failure")
	}
	if result.AIFocus == nil || result.AIFocus.Source != "deterministic" {
		t.Fatalf("expected deterministic ai_focus, got %+v", result.AIFocus)
	}
}

func TestGenerateActions_EndToEndPersistsItems(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn:  defaultFakeReviewFn(),
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if result.FinalStatus != GenerationFinalStatusCompleted && result.FinalStatus != GenerationFinalStatusCompletedPartial {
		t.Fatalf("final_status=%q", result.FinalStatus)
	}
	if len(repo.items) == 0 {
		t.Fatalf("expected persisted action items")
	}
	if len(repo.categoryRuns) == 0 {
		t.Fatalf("expected category runs")
	}
	if repo.persistCalls != 1 {
		t.Fatalf("persistCalls=%d want 1", repo.persistCalls)
	}
}

func TestValidateLegacyGlobalCheck_UnknownActionRef(t *testing.T) {
	drafts := map[string]ScoredDraftAction{
		"a1": {TempActionID: "a1"},
	}
	out := &LegacyGlobalCheckOutput{
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "missing"}},
		TopActionRefs:     []string{"missing"},
	}
	opts := DefaultGenerateActionsOptions()
	gv := ValidateLegacyGlobalCheckOutput(out, drafts, opts, DefaultForbiddenClaims, AccountSnapshot{})
	if gv.Valid {
		t.Fatalf("expected invalid legacy global check")
	}
	_, _, focus := ApplyLegacyGlobalCheckOrdering([]ScoredDraftAction{drafts["a1"]}, gv, opts)
	if focus.Source != "deterministic" {
		t.Fatalf("expected deterministic fallback focus")
	}
}

func TestValidateLegacyGlobalCheck_ForbiddenAIFocus(t *testing.T) {
	drafts := map[string]ScoredDraftAction{"a1": {TempActionID: "a1", Action: DomainAIAction{Title: "Test"}}}
	out := &LegacyGlobalCheckOutput{
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "a1"}},
		TopActionRefs:     []string{"a1"},
		AIFocus:           &ActionsAIFocus{Headline: "Снизить цену срочно", Summary: "ok", Source: "ai"},
	}
	gv := ValidateLegacyGlobalCheckOutput(out, drafts, DefaultGenerateActionsOptions(), DefaultForbiddenClaims, AccountSnapshot{})
	if gv.Valid {
		t.Fatalf("expected invalid ai_focus")
	}
}

func defaultValidComposeFn() func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
	return func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
		if len(input.DomainContext.Candidates) == 0 {
			return &DomainComposerOutput{Domain: input.Domain, Actions: nil}, &AIRawResponse{Model: "fake"}, nil
		}
		actions := make([]DomainAIAction, 0, len(input.DomainContext.Candidates))
		for _, c := range input.DomainContext.Candidates {
			a, ok := buildFallbackActionForCandidate(c)
			if ok {
				actions = append(actions, a)
			}
		}
		return &DomainComposerOutput{Domain: input.Domain, Actions: actions}, &AIRawResponse{Model: "fake"}, nil
	}
}

func defaultFakeReviewFn() func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
	return func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
		refs := make([]LegacyGlobalCheckActionRef, 0, len(input.DraftActions))
		top := make([]string, 0)
		for i, d := range input.DraftActions {
			refs = append(refs, LegacyGlobalCheckActionRef{TempActionID: d.TempActionID})
			if i < 3 {
				top = append(top, d.TempActionID)
			}
		}
		return &LegacyGlobalCheckOutput{
			AIFocus:           &ActionsAIFocus{Status: "available", Source: "ai", Headline: "Фокус дня", Summary: "Сводка"},
			OrderedActionRefs: refs,
			TopActionRefs:     top,
		}, &AIRawResponse{Model: "fake-review"}, nil
	}
}

func TestFallbackActionsPassValidation(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	fallback := finalizeAcceptedActions(domainCtx, BuildFallbackActions(domainCtx))
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
		Domain:  ActionDomainStock,
		Actions: fallback,
	}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) == 0 {
		t.Fatalf("fallback should pass validation, issues=%+v", result.Issues)
	}
}

func TestBuildFallbackActionFromCandidate(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	fallback := BuildFallbackActions(domainCtx)
	if len(fallback) != 1 {
		t.Fatalf("fallback count=%d", len(fallback))
	}
	if !containsString(fallback[0].Title, "12") {
		t.Fatalf("title=%q", fallback[0].Title)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || jsonContains(s, sub))
}

func jsonContains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
