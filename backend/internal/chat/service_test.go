package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/google/uuid"
)

func TestServiceAskHappyPath(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:     ChatIntentPriorities,
				Confidence: 0.9,
				Language:   "ru",
				ToolCalls:  []ToolCall{{Name: ToolGetDashboardSummary, Args: map[string]any{}}},
			},
			InputTokens:  10,
			OutputTokens: 20,
		},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{
				Answer:          "По dashboard есть риск.",
				Summary:         "Нужна ручная проверка.",
				Intent:          ChatIntentPriorities,
				ConfidenceLevel: ConfidenceLevelHigh,
				SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "revenue trend"}},
				Limitations:     []string{"данные ограничены"},
			},
			InputTokens:  30,
			OutputTokens: 40,
		},
	}
	toolExec := &fakeToolExecutor{
		results: []ToolResult{
			{Name: ToolGetDashboardSummary, Data: map[string]any{"kpi": map[string]any{"revenue": 100}, "data_freshness": "fresh"}},
		},
	}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:              repo,
		AIClient:          ai,
		ToolRegistry:      NewDefaultToolRegistry(),
		ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ToolExecutor:      toolExec,
		ContextAssembler:  NewContextAssembler(),
		AnswerValidator:   NewAnswerValidator(),
		Config: ServiceConfig{
			PlannerModel: "gpt-p",
			AnswerModel:  "gpt-a",
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	out, err := svc.Ask(context.Background(), AskInput{
		SellerAccountID: 1,
		Question:        "Что делать?",
	})
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if out == nil || out.Answer == nil || out.AssistantMessageID == nil {
		t.Fatal("expected full ask result")
	}
	if repo.createSessionCalls != 1 || repo.createMessageCalls < 2 || repo.completeTraceCalls != 1 {
		t.Fatalf("unexpected repo calls: %+v", repo)
	}
	if repo.failTraceCalls != 0 {
		t.Fatalf("fail trace should not be called")
	}
	if repo.lastCompleteTrace.InputTokens != 40 || repo.lastCompleteTrace.OutputTokens != 60 {
		t.Fatalf("token sum mismatch")
	}
	if repo.lastCompleteTrace.DetectedIntent != ChatIntentPriorities {
		t.Fatalf("detected intent mismatch")
	}
}

func TestServiceAskStreamEmitsStatusDeltaAndDone(t *testing.T) {
	repo := newFakeRepo()
	registry := NewDefaultToolRegistry()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{Plan: ToolPlan{
			Intent: ChatIntentPriorities, Confidence: 0.9, Language: "ru",
			ToolCalls: []ToolCall{{Name: ToolGetDashboardSummary, Args: map[string]any{}}},
		}},
		answerOut: &GenerateAnswerOutput{Answer: ChatAnswer{
			Answer:  "Проверьте остатки по ключевым SKU и создайте поставку.",
			Summary: "Нужна поставка", Intent: ChatIntentPriorities, ConfidenceLevel: ConfidenceLevelHigh,
			SupportingFacts: []SupportingFact{{Source: "tool_result", Fact: "Использованы данные инструмента: get_dashboard_summary"}},
		}},
	}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:              repo,
		AIClient:          ai,
		ToolRegistry:      registry,
		ToolPlanValidator: NewToolPlanValidator(registry),
		ToolExecutor: &fakeToolExecutor{results: []ToolResult{
			{Name: ToolGetDashboardSummary, Data: map[string]any{"kpi": map[string]any{"revenue": 100}}},
		}},
		ContextAssembler: NewContextAssembler(),
		AnswerValidator:  NewAnswerValidator(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var events []AskStreamEvent
	result, err := svc.AskStream(context.Background(), AskInput{
		SellerAccountID: 1, Question: "Что делать?", ClientRequestID: "stream-test",
	}, func(event AskStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("ask stream failed: %v", err)
	}
	if result == nil || result.Answer == nil {
		t.Fatal("expected ask result")
	}
	if len(events) < 5 || events[0].Status != "queued" || events[1].Status != "running" || events[2].Status != "streaming" {
		t.Fatalf("unexpected status events: %+v", events)
	}
	if events[len(events)-1].Type != "done" || events[len(events)-1].Status != "completed" || events[len(events)-1].Result != result {
		t.Fatalf("unexpected done event: %+v", events[len(events)-1])
	}
	var deltas strings.Builder
	for _, event := range events {
		if event.Type == "delta" {
			deltas.WriteString(event.Delta)
		}
	}
	if deltas.String() != result.Answer.Answer {
		t.Fatalf("streamed answer = %q, want %q", deltas.String(), result.Answer.Answer)
	}
	if ai.streamCalls != 1 || ai.ansCalls != 0 {
		t.Fatalf("expected StreamAnswer only, stream=%d answer=%d", ai.streamCalls, ai.ansCalls)
	}
}

type fakeActionContextProvider struct {
	context *ActionPromptContext
	err     error
}

func (f fakeActionContextProvider) GetActionForChatContext(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionPromptContext, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.context, nil
}

func TestServiceAskActionContextInPlannerPrompt(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{Plan: ToolPlan{Intent: ChatIntentGeneralOverview, Confidence: 0.9, Language: "ru"}},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{Answer: "ok", Summary: "ok", Intent: ChatIntentGeneralOverview, ConfidenceLevel: ConfidenceLevelMedium},
		},
	}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:                  repo,
		AIClient:              ai,
		ToolRegistry:          NewDefaultToolRegistry(),
		ToolPlanValidator:     NewToolPlanValidator(NewDefaultToolRegistry()),
		ToolExecutor:          &fakeToolExecutor{},
		ContextAssembler:      NewContextAssembler(),
		AnswerValidator:       NewAnswerValidator(),
		ActionContextProvider: fakeActionContextProvider{context: &ActionPromptContext{Title: "Пополнить ключевые SKU", Reason: "Низкий DOC", NextStep: "Сделайте поставку", Evidence: []string{"DOC: 2.1"}, Limitations: []string{"Нет прогноза поставок"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Ask(context.Background(), AskInput{
		SellerAccountID: 1,
		Question:        "Что делать?",
		ActionKey:       "stock:2026-06-24:key_skus",
	})
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if !strings.Contains(ai.lastPlannerUserPrompt, "Action context from Actions Engine") {
		t.Fatalf("planner prompt must include action context, got: %s", ai.lastPlannerUserPrompt)
	}
	if strings.Contains(ai.lastPlannerUserPrompt, "entity_group_json") {
		t.Fatalf("planner prompt leaks technical fields: %s", ai.lastPlannerUserPrompt)
	}
}

func TestServiceAskActionContextProviderError_GracefulFallback(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{Plan: ToolPlan{Intent: ChatIntentGeneralOverview, Confidence: 0.9, Language: "ru"}},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{Answer: "ok", Summary: "ok", Intent: ChatIntentGeneralOverview, ConfidenceLevel: ConfidenceLevelMedium},
		},
	}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:                  repo,
		AIClient:              ai,
		ToolRegistry:          NewDefaultToolRegistry(),
		ToolPlanValidator:     NewToolPlanValidator(NewDefaultToolRegistry()),
		ToolExecutor:          &fakeToolExecutor{},
		ContextAssembler:      NewContextAssembler(),
		AnswerValidator:       NewAnswerValidator(),
		ActionContextProvider: fakeActionContextProvider{err: errors.New("not found")},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Ask(context.Background(), AskInput{
		SellerAccountID: 1,
		Question:        "Что делать?",
		ActionKey:       "foreign.key",
	})
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if strings.Contains(ai.lastPlannerUserPrompt, "Action context from Actions Engine") {
		t.Fatalf("action context must be skipped on provider error: %s", ai.lastPlannerUserPrompt)
	}
}

func TestServiceAskEmptyQuestion(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	_, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "  "})
	if !errors.Is(err, ErrQuestionRequired) {
		t.Fatalf("expected ErrQuestionRequired, got %v", err)
	}
	if repo.createSessionCalls != 0 && repo.createMessageCalls != 0 {
		t.Fatal("repo should not be called")
	}
}

type stubChatDailyUsage struct {
	chatErr error
}

func (s stubChatDailyUsage) CheckChatQuestion(context.Context, int64) error      { return s.chatErr }
func (s stubChatDailyUsage) CheckRecommendationRun(context.Context, int64) error { return nil }
func (s stubChatDailyUsage) CheckSummaryRun(context.Context, int64) error        { return nil }

func TestServiceAskDailyLimitControlledResponse(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: &fakeToolExecutor{},
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
		DailyUsage: stubChatDailyUsage{chatErr: aicost.ErrDailyLimitExceeded},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Почему упали продажи?"})
	if err != nil {
		t.Fatalf("expected controlled response: %v", err)
	}
	if ai.planCalls != 0 || ai.ansCalls != 0 {
		t.Fatal("OpenAI must not be called when daily limit exceeded")
	}
	if out == nil || out.Answer == nil {
		t.Fatal("expected answer")
	}
}

func TestServiceAskPlannerErrorControlledResponse(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{planErr: errors.New("planner down")}
	svc, _ := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: &fakeToolExecutor{},
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Почему упали продажи?"})
	if err != nil {
		t.Fatalf("expected controlled response, got err %v", err)
	}
	if out == nil || out.Answer == nil || out.AssistantMessageID == nil {
		t.Fatal("expected assistant answer")
	}
	if repo.completeTraceCalls != 1 || repo.failTraceCalls != 0 {
		t.Fatalf("trace should complete, not fail: complete=%d fail=%d", repo.completeTraceCalls, repo.failTraceCalls)
	}
	if ai.planCalls != 1 || ai.ansCalls != 0 {
		t.Fatalf("planner called once, answerer not called")
	}
}

func TestServiceAskInvalidToolPlanFailsBeforeToolExecution(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:     ChatIntentPriorities,
				Confidence: 0.9,
				Language:   "ru",
				ToolCalls:  []ToolCall{{Name: "unknown_tool", Args: map[string]any{}}},
			},
		},
	}
	toolExec := &fakeToolExecutor{}
	svc, _ := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: toolExec,
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Почему упали продажи?"})
	if err != nil {
		t.Fatalf("expected controlled response: %v", err)
	}
	if out == nil || out.Answer == nil {
		t.Fatal("expected answer")
	}
	if toolExec.executePlanCalls != 0 {
		t.Fatal("tool executor must not be called")
	}
	if repo.completeTraceCalls != 1 {
		t.Fatal("trace should complete with controlled answer")
	}
}

func TestServiceAskToolExecutionFullFailure(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:     ChatIntentPriorities,
				Confidence: 0.9,
				Language:   "ru",
				ToolCalls:  []ToolCall{{Name: ToolGetDashboardSummary, Args: map[string]any{}}},
			},
		},
	}
	toolExec := &fakeToolExecutor{err: errors.New("tools failed"), results: nil}
	svc, _ := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: toolExec,
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Почему упали продажи?"})
	if err != nil {
		t.Fatalf("expected controlled response: %v", err)
	}
	if out == nil || out.Answer == nil {
		t.Fatal("expected answer")
	}
	if repo.completeTraceCalls != 1 {
		t.Fatal("trace should complete")
	}
}

func TestServiceAskAnswerValidationFailure(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:     ChatIntentPriorities,
				Confidence: 0.9,
				Language:   "ru",
				ToolCalls:  []ToolCall{{Name: ToolGetOpenAlerts, Args: map[string]any{"limit": 1}}},
			},
		},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{
				Answer:          "alert 999 требует внимания",
				Summary:         "S",
				ConfidenceLevel: ConfidenceLevelHigh,
				SupportingFacts: []SupportingFact{{Source: "alert", Fact: "f"}},
			},
		},
	}
	toolExec := &fakeToolExecutor{
		results: []ToolResult{{Name: ToolGetOpenAlerts, Data: map[string]any{"items": []any{map[string]any{"id": 101, "alert_type": "stock"}}}}},
	}
	svc, _ := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: toolExec,
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Покажи открытые alert'ы"})
	if err != nil {
		t.Fatalf("expected controlled response: %v", err)
	}
	if out == nil || out.Answer == nil {
		t.Fatal("expected answer")
	}
	if repo.createAssistantMessageCount != 1 {
		t.Fatal("assistant message should be saved for controlled validation failure")
	}
	if repo.completeTraceCalls != 1 {
		t.Fatal("trace should complete")
	}
}

type fakeAIClient struct {
	planOut               *PlanToolsOutput
	answerOut             *GenerateAnswerOutput
	planErr               error
	answerErr             error
	planCalls             int
	ansCalls              int
	streamCalls           int
	lastPlannerUserPrompt string
}

func (f *fakeAIClient) PlanTools(ctx context.Context, input PlanToolsInput) (*PlanToolsOutput, error) {
	f.planCalls++
	f.lastPlannerUserPrompt = input.UserPrompt
	if f.planErr != nil {
		return nil, f.planErr
	}
	return f.planOut, nil
}

func (f *fakeAIClient) GenerateAnswer(ctx context.Context, input GenerateAnswerInput) (*GenerateAnswerOutput, error) {
	f.ansCalls++
	if f.answerErr != nil {
		return nil, f.answerErr
	}
	return f.answerOut, nil
}

func (f *fakeAIClient) StreamAnswer(ctx context.Context, input GenerateAnswerInput, onDelta func(delta string) error) (*GenerateAnswerOutput, error) {
	f.streamCalls++
	if f.answerErr != nil {
		return nil, f.answerErr
	}
	if f.answerOut == nil {
		return nil, errors.New("stream answer is not configured")
	}
	answer := []rune(f.answerOut.Answer.Answer)
	split := len(answer) / 2
	if split == 0 {
		split = len(answer)
	}
	for _, delta := range []string{string(answer[:split]), string(answer[split:])} {
		if delta != "" {
			if err := onDelta(delta); err != nil {
				return nil, err
			}
		}
	}
	return f.answerOut, nil
}

type fakeToolExecutor struct {
	results          []ToolResult
	err              error
	executePlanCalls int
}

func (f *fakeToolExecutor) Execute(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeToolExecutor) ExecutePlan(ctx context.Context, sellerAccountID int64, plan ValidatedToolPlan) ([]ToolResult, error) {
	f.executePlanCalls++
	return f.results, f.err
}

type fakeRepo struct {
	nextSessionID int64
	nextMessageID int64
	nextTraceID   int64
	sessions      map[int64]*ChatSession

	createSessionCalls          int
	createMessageCalls          int
	createAssistantMessageCount int
	createTraceCalls            int
	completeTraceCalls          int
	failTraceCalls              int
	touchSessionCalls           int

	lastCompleteTrace CompleteTraceInput
	lastFailTrace     FailTraceInput
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		nextSessionID: 1, nextMessageID: 1, nextTraceID: 1,
		sessions: map[int64]*ChatSession{},
	}
}

func (f *fakeRepo) CreateSession(ctx context.Context, input CreateSessionInput) (*ChatSession, error) {
	f.createSessionCalls++
	id := f.nextSessionID
	f.nextSessionID++
	s := &ChatSession{ID: id, SellerAccountID: input.SellerAccountID, UserID: input.UserID, Title: input.Title, ActionKey: input.ActionKey, Status: ChatSessionStatusActive}
	f.sessions[id] = s
	return s, nil
}
func (f *fakeRepo) GetSessionByActionKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ChatSession, error) {
	for _, s := range f.sessions {
		if s.SellerAccountID == sellerAccountID && s.ActionKey != nil && *s.ActionKey == actionKey {
			return s, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) ReactivateSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	s := f.sessions[sessionID]
	if s != nil {
		s.Status = ChatSessionStatusActive
	}
	return s, nil
}
func (f *fakeRepo) GetSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	s := f.sessions[sessionID]
	if s == nil || s.SellerAccountID != sellerAccountID {
		return nil, nil
	}
	return s, nil
}
func (f *fakeRepo) ListSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error) {
	return nil, nil
}
func (f *fakeRepo) ListActiveSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error) {
	return nil, nil
}
func (f *fakeRepo) ArchiveSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	return nil, nil
}
func (f *fakeRepo) SoftDeleteSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	return nil, nil
}
func (f *fakeRepo) TouchSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	f.touchSessionCalls++
	return f.sessions[sessionID], nil
}
func (f *fakeRepo) UpdateSessionTitle(ctx context.Context, sellerAccountID, sessionID int64, title string) (*ChatSession, error) {
	return f.sessions[sessionID], nil
}
func (f *fakeRepo) CreateMessage(ctx context.Context, input CreateMessageInput) (*ChatMessage, error) {
	f.createMessageCalls++
	if input.Role == ChatMessageRoleAssistant {
		f.createAssistantMessageCount++
	}
	id := f.nextMessageID
	f.nextMessageID++
	return &ChatMessage{
		ID: id, SessionID: input.SessionID, SellerAccountID: input.SellerAccountID, Role: input.Role, Content: input.Content, MessageType: input.MessageType,
		CreatedAt: time.Now().UTC(),
	}, nil
}
func (f *fakeRepo) GetMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatMessage, error) {
	return nil, nil
}
func (f *fakeRepo) GetMessageByClientRequestID(ctx context.Context, sellerAccountID int64, clientRequestID string) (*ChatMessage, error) {
	return nil, nil
}

type fakeAttachmentRepo struct{}

func (f *fakeAttachmentRepo) CreateAttachment(ctx context.Context, input CreateAttachmentInput) (*ChatAttachment, error) {
	return &ChatAttachment{ID: input.ID, SellerAccountID: input.SellerAccountID, FileName: input.FileName, Status: input.Status, AttachmentKind: input.AttachmentKind}, nil
}
func (f *fakeAttachmentRepo) GetAttachmentForSeller(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) (*ChatAttachment, error) {
	return nil, nil
}
func (f *fakeAttachmentRepo) ListAttachmentsByMessageIDs(ctx context.Context, sellerAccountID int64, messageIDs []int64) ([]ChatAttachment, error) {
	return nil, nil
}
func (f *fakeAttachmentRepo) ListDraftAttachmentsByIDs(ctx context.Context, sellerAccountID int64, attachmentIDs []uuid.UUID) ([]ChatAttachment, error) {
	return nil, nil
}
func (f *fakeAttachmentRepo) CountDraftInputAttachments(ctx context.Context, sellerAccountID int64) (int64, error) {
	return 0, nil
}
func (f *fakeAttachmentRepo) AttachAttachmentToMessage(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, sessionID, messageID int64, openAIFileID string) (*ChatAttachment, error) {
	return nil, nil
}
func (f *fakeAttachmentRepo) DeleteDraftAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) error {
	return nil
}
func (f *fakeRepo) ListMessages(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatMessage, error) {
	return nil, nil
}
func (f *fakeRepo) ListRecentMessages(ctx context.Context, sellerAccountID, sessionID int64, limit int32) ([]ChatMessage, error) {
	return nil, nil
}
func (f *fakeRepo) CreateTrace(ctx context.Context, input CreateTraceInput) (*ChatTrace, error) {
	f.createTraceCalls++
	id := f.nextTraceID
	f.nextTraceID++
	return &ChatTrace{ID: id, SessionID: input.SessionID, SellerAccountID: input.SellerAccountID, Status: ChatTraceStatusRunning}, nil
}
func (f *fakeRepo) CompleteTrace(ctx context.Context, input CompleteTraceInput) (*ChatTrace, error) {
	f.completeTraceCalls++
	f.lastCompleteTrace = input
	return &ChatTrace{ID: input.TraceID, Status: ChatTraceStatusCompleted}, nil
}
func (f *fakeRepo) FailTrace(ctx context.Context, input FailTraceInput) (*ChatTrace, error) {
	f.failTraceCalls++
	f.lastFailTrace = input
	return &ChatTrace{ID: input.TraceID, Status: ChatTraceStatusFailed}, nil
}
func (f *fakeRepo) GetTrace(ctx context.Context, sellerAccountID, traceID int64) (*ChatTrace, error) {
	return nil, nil
}
func (f *fakeRepo) GetLatestTraceBySession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatTrace, error) {
	return nil, nil
}
func (f *fakeRepo) ListTracesBySession(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatTrace, error) {
	return nil, nil
}
func (f *fakeRepo) CreateFeedback(ctx context.Context, input AddFeedbackInput) (*ChatFeedback, error) {
	return nil, nil
}
func (f *fakeRepo) GetFeedbackByMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatFeedback, error) {
	return nil, nil
}
func (f *fakeRepo) ListFeedback(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatFeedback, error) {
	return nil, nil
}

func TestServiceAskUnsupportedQuestionCompletesWithoutTools(t *testing.T) {
	repo := newFakeRepo()
	reason := "Запрос требует auto-action."
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:            ChatIntentUnsupported,
				Confidence:        0.95,
				Language:          "ru",
				ToolCalls:         []ToolCall{},
				Assumptions:       []string{},
				UnsupportedReason: &reason,
			},
		},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{
				Answer:          "Я не могу выполнить это действие автоматически.",
				Summary:         "Запрос требует действия, которое AI-чат не выполняет.",
				Intent:          ChatIntentUnsupported,
				ConfidenceLevel: ConfidenceLevelHigh,
				SupportingFacts: []SupportingFact{{Source: "limitation", Fact: "AI-чат не выполняет auto-actions."}},
				Limitations:     []string{"Запрос требует auto-action, который запрещен в AI Chat MVP."},
			},
		},
	}
	toolExec := &fakeToolExecutor{}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: toolExec,
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Объясни рекомендацию: можно ли выполнить это действие автоматически в кабинете?"})
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if out == nil || out.Answer == nil || out.AssistantMessageID == nil {
		t.Fatal("expected answer and assistant message")
	}
	if toolExec.executePlanCalls != 0 {
		t.Fatal("tools must not be executed for unsupported intent")
	}
	if ai.ansCalls != 1 {
		t.Fatal("answerer must be called for unsupported intent")
	}
	if repo.completeTraceCalls != 1 || repo.failTraceCalls != 0 {
		t.Fatalf("trace should complete successfully")
	}
	if repo.createAssistantMessageCount != 1 {
		t.Fatal("assistant message should be saved")
	}
	if len(out.Answer.Limitations) == 0 || len(out.Answer.SupportingFacts) == 0 {
		t.Fatal("unsupported answer should include limitations and supporting facts")
	}
}

func TestSanitizeTracePayloadRedactsSecrets(t *testing.T) {
	payload := map[string]any{
		"api_key":       "sk-test-secret",
		"authorization": "Bearer super-secret-token",
		"nested": map[string]any{
			"access_token": "tok123",
			"note":         "OPENAI_API_KEY=sk-live-abc",
		},
		"items": []any{
			map[string]any{"cookie": "abc"},
			"authorization: bearer token-123",
		},
	}
	sanitized := sanitizeTracePayload(payload)
	if sanitized["api_key"] != "[REDACTED]" {
		t.Fatalf("expected api_key redaction")
	}
	if sanitized["authorization"] != "[REDACTED]" {
		t.Fatalf("expected authorization redaction")
	}
	nested := sanitized["nested"].(map[string]any)
	if nested["access_token"] != "[REDACTED]" {
		t.Fatalf("expected nested token redaction")
	}
	if !strings.Contains(nested["note"].(string), "[REDACTED]") {
		t.Fatalf("expected OPENAI_API_KEY marker redaction")
	}
	items := sanitized["items"].([]any)
	if items[0].(map[string]any)["cookie"] != "[REDACTED]" {
		t.Fatalf("expected cookie redaction")
	}
	if items[1] != "authorization: Bearer [REDACTED]" {
		t.Fatalf("expected bearer text redaction")
	}
}

func TestServiceAskSanitizesTracePayloads(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{
		planOut: &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:     ChatIntentPriorities,
				Confidence: 0.9,
				Language:   "ru",
				ToolCalls:  []ToolCall{{Name: ToolGetDashboardSummary, Args: map[string]any{}}},
			},
			RawResponse: []byte(`{"token":"secret","nested":{"authorization":"Bearer abc"},"text":"sk-test-secret"}`),
		},
		answerOut: &GenerateAnswerOutput{
			Answer: ChatAnswer{
				Answer:          "Ответ",
				Summary:         "Кратко",
				Intent:          ChatIntentPriorities,
				ConfidenceLevel: ConfidenceLevelMedium,
				SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "f"}},
			},
			RawResponse: []byte(`{"api_key":"sk-live-123","payload":"authorization: bearer ttt"}`),
		},
	}
	toolExec := &fakeToolExecutor{
		results: []ToolResult{
			{Name: ToolGetDashboardSummary, Data: map[string]any{"kpi": map[string]any{"revenue": 100}}},
		},
	}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo: repo, AIClient: ai, ToolExecutor: toolExec,
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if _, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Какие товары с риском остатка?"}); err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if strings.Contains(fmt.Sprint(repo.lastCompleteTrace.RawPlannerResponse), "sk-test-secret") {
		t.Fatalf("planner raw response contains unredacted secret")
	}
	if strings.Contains(fmt.Sprint(repo.lastCompleteTrace.RawAnswerResponse), "sk-live-123") {
		t.Fatalf("answer raw response contains unredacted secret")
	}
	if !strings.Contains(fmt.Sprint(repo.lastCompleteTrace.RawPlannerResponse), "[REDACTED") {
		t.Fatalf("expected redaction marker in planner trace payload")
	}
	if !strings.Contains(fmt.Sprint(repo.lastCompleteTrace.RawAnswerResponse), "[REDACTED") {
		t.Fatalf("expected redaction marker in answer trace payload")
	}
}
