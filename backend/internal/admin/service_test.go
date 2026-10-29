package admin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testAdminActor() AdminActor {
	uid := int64(999)
	return AdminActor{UserID: &uid, Email: "audit-test@example.com"}
}

type fakeIngestion struct{}

func (f fakeIngestion) StartInitialSync(ctx context.Context, sellerAccountID int64) (int64, string, error) {
	return 101, "pending", nil
}

type fakeCursor struct{}

func (f fakeCursor) ResetCursor(ctx context.Context, sellerAccountID int64, domain, cursorType string, cursorValue *string) error {
	return nil
}

type fakeAlerts struct{}

func (f fakeAlerts) Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error) {
	return map[string]any{"alert_run_id": 77, "status": "completed"}, nil
}

type fakeRecommendations struct{}

func (f fakeRecommendations) Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error) {
	return map[string]any{"recommendation_run_id": 88, "status": "completed"}, nil
}

type fakeMetrics struct {
	lastSeller int64
	lastFrom   time.Time
	lastTo     time.Time
	result     map[string]any
	err        error
}

func (f *fakeMetrics) Rerun(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (map[string]any, error) {
	f.lastSeller = sellerAccountID
	f.lastFrom = dateFrom
	f.lastTo = dateTo
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return map[string]any{
		"status":                     "completed",
		"account_daily_metrics_rows": 4,
		"sku_daily_metrics_rows":     9,
	}, nil
}

type fakeRepo struct {
	listClientsFn                          func(ctx context.Context, filter ClientListFilter) (*ClientListResult, error)
	getClientOverviewFn                    func(ctx context.Context, sellerAccountID int64) (*ClientOverview, error)
	getClientConnectionsFn                 func(ctx context.Context, sellerAccountID int64) ([]ClientConnection, error)
	listSyncJobsFn                         func(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error)
	listImportJobsFn                       func(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error)
	listImportErrorsFn                     func(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter, page Page) ([]ImportErrorItem, error)
	listSyncCursorsFn                      func(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter, page Page) ([]SyncCursorItem, error)
	listAlertRunsFn                        func(ctx context.Context, sellerAccountID int64, page Page) ([]AlertRunSummary, error)
	listRecommendationRunsFn               func(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error)
	listChatTracesFn                       func(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error)
	listChatSessionsFn                     func(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter, page Page) ([]ChatSessionItem, error)
	getChatSessionFn                       func(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSessionItem, error)
	listChatMessagesFn                     func(ctx context.Context, sellerAccountID, sessionID int64, filter ChatMessageFilter, page Page) ([]ChatMessageItem, error)
	getChatTraceDetailFn                   func(ctx context.Context, sellerAccountID, traceID int64) (*ChatTraceDetail, error)
	getRecommendationRunDetailFn           func(ctx context.Context, sellerAccountID, runID int64) (*RecommendationRunDetail, error)
	getRecommendationRawAIFn               func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error)
	getBillingStateFn                      func(ctx context.Context, sellerAccountID int64) (*BillingState, error)
	listBillingStatesFn                    func(ctx context.Context, filter BillingStateFilter) ([]BillingState, error)
	createActionFn                         func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error)
	completeActionFn                       func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error)
	failActionFn                           func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error)
	upsertBillingStateFn                   func(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error)
	listChatFeedbackFn                     func(ctx context.Context, filter ChatFeedbackFilter, page Page) ([]ChatFeedbackItem, error)
	listRecommendationFeedbackFn           func(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter, page Page) ([]RecommendationFeedbackItem, error)
	getRecommendationProxyFeedbackCountsFn func(ctx context.Context, sellerAccountID int64) (RecommendationFeedbackProxyStatus, error)
	peekRecommendationForAuditFn           func(ctx context.Context, sellerAccountID, recommendationID int64) (RecommendationViewAuditMeta, error)
	peekChatTraceForAuditFn                func(ctx context.Context, sellerAccountID, traceID int64) (ChatTraceViewAuditMeta, error)
	peekRecommendationRunForAuditFn        func(ctx context.Context, sellerAccountID, runID int64) (RecommendationRunViewAuditMeta, error)
}

func (f *fakeRepo) ListClients(ctx context.Context, filter ClientListFilter) (*ClientListResult, error) {
	if f.listClientsFn != nil {
		return f.listClientsFn(ctx, filter)
	}
	return &ClientListResult{}, nil
}
func (f *fakeRepo) GetClientOverview(ctx context.Context, sellerAccountID int64) (*ClientOverview, error) {
	if f.getClientOverviewFn != nil {
		return f.getClientOverviewFn(ctx, sellerAccountID)
	}
	return nil, nil
}
func (f *fakeRepo) GetClientConnections(ctx context.Context, sellerAccountID int64) ([]ClientConnection, error) {
	if f.getClientConnectionsFn != nil {
		return f.getClientConnectionsFn(ctx, sellerAccountID)
	}
	return nil, nil
}
func (f *fakeRepo) ListSyncJobs(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error) {
	if f.listSyncJobsFn != nil {
		return f.listSyncJobsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListImportJobs(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error) {
	if f.listImportJobsFn != nil {
		return f.listImportJobsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListImportErrors(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter, page Page) ([]ImportErrorItem, error) {
	if f.listImportErrorsFn != nil {
		return f.listImportErrorsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListSyncCursors(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter, page Page) ([]SyncCursorItem, error) {
	if f.listSyncCursorsFn != nil {
		return f.listSyncCursorsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListAlertRuns(ctx context.Context, sellerAccountID int64, page Page) ([]AlertRunSummary, error) {
	if f.listAlertRunsFn != nil {
		return f.listAlertRunsFn(ctx, sellerAccountID, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListRecommendationRuns(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error) {
	if f.listRecommendationRunsFn != nil {
		return f.listRecommendationRunsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListChatTraces(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error) {
	if f.listChatTracesFn != nil {
		return f.listChatTracesFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListChatSessions(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter, page Page) ([]ChatSessionItem, error) {
	if f.listChatSessionsFn != nil {
		return f.listChatSessionsFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) GetChatSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSessionItem, error) {
	if f.getChatSessionFn != nil {
		return f.getChatSessionFn(ctx, sellerAccountID, sessionID)
	}
	return nil, nil
}
func (f *fakeRepo) ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, filter ChatMessageFilter, page Page) ([]ChatMessageItem, error) {
	if f.listChatMessagesFn != nil {
		return f.listChatMessagesFn(ctx, sellerAccountID, sessionID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListChatFeedback(ctx context.Context, filter ChatFeedbackFilter, page Page) ([]ChatFeedbackItem, error) {
	if f.listChatFeedbackFn != nil {
		return f.listChatFeedbackFn(ctx, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter, page Page) ([]RecommendationFeedbackItem, error) {
	if f.listRecommendationFeedbackFn != nil {
		return f.listRecommendationFeedbackFn(ctx, sellerAccountID, filter, page)
	}
	return nil, nil
}
func (f *fakeRepo) GetRecommendationProxyFeedbackCounts(ctx context.Context, sellerAccountID int64) (RecommendationFeedbackProxyStatus, error) {
	if f.getRecommendationProxyFeedbackCountsFn != nil {
		return f.getRecommendationProxyFeedbackCountsFn(ctx, sellerAccountID)
	}
	return RecommendationFeedbackProxyStatus{}, nil
}
func (f *fakeRepo) PeekRecommendationForAudit(ctx context.Context, sellerAccountID, recommendationID int64) (RecommendationViewAuditMeta, error) {
	if f.peekRecommendationForAuditFn != nil {
		return f.peekRecommendationForAuditFn(ctx, sellerAccountID, recommendationID)
	}
	return RecommendationViewAuditMeta{
		ID:              recommendationID,
		AIModel:         "gpt-test",
		AIPromptVersion: "pv-1",
	}, nil
}
func (f *fakeRepo) PeekChatTraceForAudit(ctx context.Context, sellerAccountID, traceID int64) (ChatTraceViewAuditMeta, error) {
	if f.peekChatTraceForAuditFn != nil {
		return f.peekChatTraceForAuditFn(ctx, sellerAccountID, traceID)
	}
	return ChatTraceViewAuditMeta{
		ID:                   traceID,
		SessionID:            1,
		PlannerModel:         "planner-m",
		AnswerModel:          "answer-m",
		PlannerPromptVersion: "planner-pv",
		AnswerPromptVersion:  "answer-pv",
		Status:               "completed",
	}, nil
}
func (f *fakeRepo) PeekRecommendationRunForAudit(ctx context.Context, sellerAccountID, runID int64) (RecommendationRunViewAuditMeta, error) {
	if f.peekRecommendationRunForAuditFn != nil {
		return f.peekRecommendationRunForAuditFn(ctx, sellerAccountID, runID)
	}
	return RecommendationRunViewAuditMeta{
		ID:              runID,
		RunType:         "manual",
		Status:          "completed",
		AIModel:         "run-model",
		AIPromptVersion: "run-pv",
	}, nil
}
func (f *fakeRepo) GetChatTraceDetail(ctx context.Context, sellerAccountID, traceID int64) (*ChatTraceDetail, error) {
	if f.getChatTraceDetailFn != nil {
		return f.getChatTraceDetailFn(ctx, sellerAccountID, traceID)
	}
	return nil, nil
}
func (f *fakeRepo) GetRecommendationRunDetail(ctx context.Context, sellerAccountID, runID int64) (*RecommendationRunDetail, error) {
	if f.getRecommendationRunDetailFn != nil {
		return f.getRecommendationRunDetailFn(ctx, sellerAccountID, runID)
	}
	return nil, nil
}
func (f *fakeRepo) GetRecommendationRawAI(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
	if f.getRecommendationRawAIFn != nil {
		return f.getRecommendationRawAIFn(ctx, sellerAccountID, recommendationID)
	}
	return nil, nil
}
func (f *fakeRepo) GetBillingState(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
	if f.getBillingStateFn != nil {
		return f.getBillingStateFn(ctx, sellerAccountID)
	}
	return nil, nil
}
func (f *fakeRepo) ListBillingStates(ctx context.Context, filter BillingStateFilter) ([]BillingState, error) {
	if f.listBillingStatesFn != nil {
		return f.listBillingStatesFn(ctx, filter)
	}
	return nil, nil
}
func (f *fakeRepo) CreateAdminActionLog(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
	if f.createActionFn != nil {
		return f.createActionFn(ctx, input)
	}
	return &AdminActionLog{
		ID:              999001,
		AdminUserID:     input.AdminUserID,
		AdminEmail:      input.AdminEmail,
		SellerAccountID: input.SellerAccountID,
		ActionType:      input.ActionType,
		TargetType:      input.TargetType,
		TargetID:        input.TargetID,
		RequestPayload:  input.RequestPayload,
		Status:          AdminActionStatusRunning,
	}, nil
}
func (f *fakeRepo) CompleteAdminActionLog(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
	if f.completeActionFn != nil {
		return f.completeActionFn(ctx, input)
	}
	return &AdminActionLog{ID: input.ID, Status: AdminActionStatusCompleted, ResultPayload: input.ResultPayload}, nil
}
func (f *fakeRepo) FailAdminActionLog(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
	if f.failActionFn != nil {
		return f.failActionFn(ctx, input)
	}
	return nil, nil
}
func (f *fakeRepo) UpsertBillingState(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error) {
	if f.upsertBillingStateFn != nil {
		return f.upsertBillingStateFn(ctx, input)
	}
	return nil, nil
}

func TestNewServiceRequiresRepo(t *testing.T) {
	_, err := NewService(ServiceDeps{})
	if !errors.Is(err, ErrRepositoryRequired) {
		t.Fatalf("expected ErrRepositoryRequired, got %v", err)
	}
}

func TestListClientsNormalizesPagination(t *testing.T) {
	var got ClientListFilter
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listClientsFn: func(ctx context.Context, filter ClientListFilter) (*ClientListResult, error) {
				got = filter
				return &ClientListResult{}, nil
			},
		},
	})
	_, err := svc.ListClients(context.Background(), ClientListFilter{Limit: 0, Offset: -5})
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 50 || got.Offset != 0 {
		t.Fatalf("unexpected normalized page: %+v", got)
	}
}

func TestGetClientDetailComposesData(t *testing.T) {
	now := time.Now()
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			getClientOverviewFn: func(ctx context.Context, sellerAccountID int64) (*ClientOverview, error) {
				return &ClientOverview{SellerAccountID: sellerAccountID, SellerName: "A", CreatedAt: now, UpdatedAt: now}, nil
			},
			getClientConnectionsFn: func(ctx context.Context, sellerAccountID int64) ([]ClientConnection, error) {
				return []ClientConnection{{Provider: "ozon", ConnectionStatus: "connected"}}, nil
			},
			listSyncJobsFn: func(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error) {
				return []SyncJobSummary{{ID: 1}}, nil
			},
			listImportJobsFn: func(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error) {
				return []ImportJobSummary{{ID: 2}}, nil
			},
			listAlertRunsFn: func(ctx context.Context, sellerAccountID int64, page Page) ([]AlertRunSummary, error) {
				return nil, nil
			},
			listRecommendationRunsFn: func(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error) {
				return nil, nil
			},
			listChatTracesFn: func(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error) {
				return nil, nil
			},
			getBillingStateFn: func(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
				return &BillingState{SellerAccountID: sellerAccountID}, nil
			},
		},
	})
	detail, err := svc.GetClientDetail(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Overview.SellerAccountID != 42 || len(detail.Connections) != 1 || detail.Billing == nil {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestGetBillingStateDelegatesRepo(t *testing.T) {
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			getBillingStateFn: func(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
				return &BillingState{SellerAccountID: sellerAccountID}, nil
			},
		},
	})
	state, err := svc.GetBillingState(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if state.SellerAccountID != 5 {
		t.Fatalf("unexpected seller account id: %d", state.SellerAccountID)
	}
}

func TestUpdateBillingStateAudited(t *testing.T) {
	created := false
	completed := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				created = true
				return &AdminActionLog{ID: 100, Status: AdminActionStatusRunning}, nil
			},
			upsertBillingStateFn: func(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error) {
				return &BillingState{SellerAccountID: input.SellerAccountID, Status: input.Status}, nil
			},
			completeActionFn: func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
				completed = true
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusCompleted}, nil
			},
			getBillingStateFn: func(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
				return &BillingState{SellerAccountID: sellerAccountID, Status: BillingStatusActive}, nil
			},
		},
	})
	state, log, err := svc.UpdateBillingState(context.Background(), AdminActor{Email: "admin@example.com"}, UpdateBillingStateInput{
		SellerAccountID: 7,
		PlanCode:        "internal",
		Status:          BillingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || !completed || state == nil || log == nil {
		t.Fatalf("audit flow not completed: created=%v completed=%v state=%v log=%v", created, completed, state, log)
	}
}

func TestUpdateBillingStateValidation(t *testing.T) {
	svc, _ := NewService(ServiceDeps{Repo: &fakeRepo{}})
	_, _, err := svc.UpdateBillingState(context.Background(), AdminActor{Email: "admin@example.com"}, UpdateBillingStateInput{
		SellerAccountID: 1,
		PlanCode:        "",
		Status:          BillingStatusTrial,
	})
	if err == nil {
		t.Fatalf("expected plan_code validation error")
	}
	_, _, err = svc.UpdateBillingState(context.Background(), AdminActor{Email: "admin@example.com"}, UpdateBillingStateInput{
		SellerAccountID: 1,
		PlanCode:        "trial",
		Status:          BillingStatus("bad"),
	})
	if err == nil {
		t.Fatalf("expected status validation error")
	}
	neg := int64(-1)
	_, _, err = svc.UpdateBillingState(context.Background(), AdminActor{Email: "admin@example.com"}, UpdateBillingStateInput{
		SellerAccountID:    1,
		PlanCode:           "trial",
		Status:             BillingStatusTrial,
		AITokensLimitMonth: &neg,
	})
	if err == nil {
		t.Fatalf("expected tokens validation error")
	}
}

func TestUpdateBillingStateFailAuditOnUpsertError(t *testing.T) {
	failed := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				return &AdminActionLog{ID: 10, Status: AdminActionStatusRunning}, nil
			},
			upsertBillingStateFn: func(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error) {
				return nil, errors.New("db fail")
			},
			failActionFn: func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
				failed = true
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusFailed}, nil
			},
		},
	})
	_, action, err := svc.UpdateBillingState(context.Background(), AdminActor{Email: "admin@example.com"}, UpdateBillingStateInput{
		SellerAccountID: 1,
		PlanCode:        "trial",
		Status:          BillingStatusTrial,
	})
	if err == nil || action == nil || !failed {
		t.Fatalf("expected audited failure, err=%v action=%v failed=%v", err, action, failed)
	}
}

func TestListBillingStatesNormalizesAndTrims(t *testing.T) {
	var got BillingStateFilter
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listBillingStatesFn: func(ctx context.Context, filter BillingStateFilter) ([]BillingState, error) {
				got = filter
				return []BillingState{}, nil
			},
		},
	})
	_, err := svc.ListBillingStates(context.Background(), BillingStateFilter{Status: " trial ", Limit: 0, Offset: -1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "trial" || got.Limit != 50 || got.Offset != 0 {
		t.Fatalf("unexpected filter: %+v", got)
	}
}

func TestActionMissingDependencyFailsAndAudited(t *testing.T) {
	failed := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				return &AdminActionLog{ID: 11, Status: AdminActionStatusRunning}, nil
			},
			failActionFn: func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
				failed = true
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusFailed}, nil
			},
		},
	})
	_, err := svc.RerunAlerts(context.Background(), AdminActor{Email: "admin@example.com"}, RerunAlertsInput{SellerAccountID: 9, AsOfDate: time.Now()})
	if !errors.Is(err, ErrAdminActionNotConfigured) {
		t.Fatalf("expected ErrAdminActionNotConfigured, got %v", err)
	}
	if !failed {
		t.Fatalf("expected action to be failed in audit log")
	}
}

func TestRerunMetricsNotConfiguredAudited(t *testing.T) {
	failed := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				return &AdminActionLog{ID: 12, Status: AdminActionStatusRunning}, nil
			},
			failActionFn: func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
				failed = true
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusFailed}, nil
			},
		},
	})
	_, err := svc.RerunMetrics(context.Background(), AdminActor{Email: "admin@example.com"}, RerunMetricsInput{SellerAccountID: 9})
	if !errors.Is(err, ErrAdminActionNotConfigured) {
		t.Fatalf("expected ErrAdminActionNotConfigured, got %v", err)
	}
	if !failed {
		t.Fatalf("expected action to be failed in audit log")
	}
}

func TestRerunMetricsInvalidDateRangeNoAudit(t *testing.T) {
	created := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				created = true
				return &AdminActionLog{ID: 1, Status: AdminActionStatusRunning}, nil
			},
		},
		MetricsService: &fakeMetrics{},
	})
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.RerunMetrics(context.Background(), AdminActor{Email: "admin@example.com"}, RerunMetricsInput{SellerAccountID: 9, DateFrom: from, DateTo: to})
	if err == nil {
		t.Fatal("expected error")
	}
	if created {
		t.Fatalf("did not expect audit log for pre-action validation failure")
	}
}

func TestRerunMetricsCallsRerunnerAndCompletesAudit(t *testing.T) {
	created := false
	completed := false
	m := &fakeMetrics{}
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				created = true
				return &AdminActionLog{ID: 50, Status: AdminActionStatusRunning}, nil
			},
			completeActionFn: func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
				completed = true
				if input.ResultPayload["account_daily_metrics_rows"] == nil {
					t.Fatalf("expected result payload from rerunner: %+v", input.ResultPayload)
				}
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusCompleted, ResultPayload: input.ResultPayload}, nil
			},
		},
		MetricsService: m,
	})
	df := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	dt := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	log, err := svc.RerunMetrics(context.Background(), AdminActor{Email: "admin@example.com"}, RerunMetricsInput{SellerAccountID: 3, DateFrom: df, DateTo: dt})
	if err != nil {
		t.Fatal(err)
	}
	if !created || !completed || log == nil || log.Status != AdminActionStatusCompleted {
		t.Fatalf("unexpected audit outcome: created=%v completed=%v log=%+v", created, completed, log)
	}
	if m.lastSeller != 3 {
		t.Fatalf("unexpected seller: %d", m.lastSeller)
	}
	if m.lastFrom.Format("2006-01-02") != "2026-01-10" || m.lastTo.Format("2006-01-02") != "2026-01-12" {
		t.Fatalf("unexpected date range passed to rerunner: %s..%s", m.lastFrom.Format("2006-01-02"), m.lastTo.Format("2006-01-02"))
	}
}

func TestActorValidation(t *testing.T) {
	svc, _ := NewService(ServiceDeps{Repo: &fakeRepo{}})
	_, err := svc.RerunMetrics(context.Background(), AdminActor{}, RerunMetricsInput{SellerAccountID: 1})
	if !errors.Is(err, ErrAdminActorRequired) {
		t.Fatalf("expected ErrAdminActorRequired, got %v", err)
	}
}

func TestSellerValidation(t *testing.T) {
	svc, _ := NewService(ServiceDeps{Repo: &fakeRepo{}})
	_, err := svc.GetBillingState(context.Background(), 0)
	if !errors.Is(err, ErrSellerAccountIDRequired) {
		t.Fatalf("expected ErrSellerAccountIDRequired, got %v", err)
	}
}

func TestAILogsDelegation(t *testing.T) {
	calledChat := false
	calledRec := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listChatTracesFn: func(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error) {
				calledChat = true
				return []ChatTraceLogItem{{ID: 1}}, nil
			},
			listRecommendationRunsFn: func(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error) {
				calledRec = true
				return []RecommendationRunLogItem{{ID: 2}}, nil
			},
		},
	})
	if _, err := svc.GetAIChatLogs(context.Background(), 3, ChatTraceFilter{Limit: 10, Offset: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetAIRecommendationLogs(context.Background(), 3, RecommendationRunLogFilter{Limit: 10, Offset: 0}); err != nil {
		t.Fatal(err)
	}
	if !calledChat || !calledRec {
		t.Fatalf("delegation did not call repository methods")
	}
}

func TestRecommendationLogsFilterAndDetailDelegation(t *testing.T) {
	var gotFilter RecommendationRunLogFilter
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listRecommendationRunsFn: func(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error) {
				gotFilter = filter
				if page.Limit != 50 || page.Offset != 0 {
					t.Fatalf("unexpected page: %+v", page)
				}
				return []RecommendationRunLogItem{{ID: 10}}, nil
			},
			getRecommendationRunDetailFn: func(ctx context.Context, sellerAccountID, runID int64) (*RecommendationRunDetail, error) {
				return &RecommendationRunDetail{Run: RecommendationRunSummary{ID: runID}, Diagnostics: []RecommendationRunDiagnosticItem{}}, nil
			},
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				return &RecommendationRawAI{Recommendation: RecommendationItem{ID: recommendationID}}, nil
			},
		},
	})

	res, err := svc.GetAIRecommendationLogs(context.Background(), 5, RecommendationRunLogFilter{Status: " completed ", RunType: " manual ", Limit: 0, Offset: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || gotFilter.Status != "completed" || gotFilter.RunType != "manual" {
		t.Fatalf("unexpected recommendation list result/filter: %+v %+v", res, gotFilter)
	}

	detail, err := svc.GetRecommendationRunDetail(context.Background(), testAdminActor(), 5, 11)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Limitations) == 0 {
		t.Fatalf("expected limitations for empty diagnostics/recommendations")
	}

	raw, err := svc.GetRecommendationRawAI(context.Background(), testAdminActor(), 5, 44)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Recommendation.ID != 44 {
		t.Fatalf("unexpected recommendation id: %d", raw.Recommendation.ID)
	}
}

func TestChatLogsFiltersSessionsAndMessages(t *testing.T) {
	var gotChatFilter ChatTraceFilter
	var gotSessionFilter ChatSessionFilter
	var gotMessageFilter ChatMessageFilter
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listChatTracesFn: func(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error) {
				gotChatFilter = filter
				return []ChatTraceLogItem{{ID: 1}}, nil
			},
			getChatTraceDetailFn: func(ctx context.Context, sellerAccountID, traceID int64) (*ChatTraceDetail, error) {
				return &ChatTraceDetail{ID: traceID}, nil
			},
			listChatSessionsFn: func(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter, page Page) ([]ChatSessionItem, error) {
				gotSessionFilter = filter
				return []ChatSessionItem{{ID: 10}}, nil
			},
			getChatSessionFn: func(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSessionItem, error) {
				return &ChatSessionItem{ID: sessionID}, nil
			},
			listChatMessagesFn: func(ctx context.Context, sellerAccountID, sessionID int64, filter ChatMessageFilter, page Page) ([]ChatMessageItem, error) {
				gotMessageFilter = filter
				return []ChatMessageItem{{ID: 100, SessionID: sessionID}}, nil
			},
		},
	})

	traces, err := svc.GetAIChatLogs(context.Background(), 5, ChatTraceFilter{Status: " completed ", Intent: " priorities ", Limit: 0, Offset: -1})
	if err != nil || len(traces.Items) != 1 || gotChatFilter.Status != "completed" || gotChatFilter.Intent != "priorities" {
		t.Fatalf("unexpected chat traces result/filter: %+v %+v %v", traces, gotChatFilter, err)
	}
	if _, err := svc.GetChatTraceDetail(context.Background(), testAdminActor(), 5, 1); err != nil {
		t.Fatal(err)
	}
	sessions, err := svc.ListChatSessions(context.Background(), 5, ChatSessionFilter{Status: " active ", Limit: 0, Offset: 0})
	if err != nil || len(sessions.Items) != 1 || gotSessionFilter.Status != "active" {
		t.Fatalf("unexpected sessions result/filter: %+v %+v %v", sessions, gotSessionFilter, err)
	}
	messages, err := svc.ListChatMessages(context.Background(), 5, 10, ChatMessageFilter{Limit: 10, Offset: 0})
	if err != nil || len(messages.Items) != 1 || gotMessageFilter.Limit != 10 {
		t.Fatalf("unexpected messages result/filter: %+v %+v %v", messages, gotMessageFilter, err)
	}
}

func TestListChatMessagesValidation(t *testing.T) {
	svc, _ := NewService(ServiceDeps{Repo: &fakeRepo{}})
	if _, err := svc.ListChatMessages(context.Background(), 0, 1, ChatMessageFilter{}); !errors.Is(err, ErrSellerAccountIDRequired) {
		t.Fatalf("expected ErrSellerAccountIDRequired, got %v", err)
	}
	if _, err := svc.ListChatMessages(context.Background(), 1, 0, ChatMessageFilter{}); !errors.Is(err, ErrAdminDataUnavailable) {
		t.Fatalf("expected ErrAdminDataUnavailable, got %v", err)
	}
}

func TestDiagnosticsMethodsNormalizeAndDelegate(t *testing.T) {
	var syncFilter SyncJobFilter
	var importFilter ImportJobFilter
	var errFilter ImportErrorFilter
	var cursorFilter SyncCursorFilter

	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listSyncJobsFn: func(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error) {
				syncFilter = filter
				if page.Limit != 50 || page.Offset != 0 {
					t.Fatalf("unexpected sync page: %+v", page)
				}
				return []SyncJobSummary{{ID: 1}}, nil
			},
			listImportJobsFn: func(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error) {
				importFilter = filter
				if page.Limit != 200 || page.Offset != 0 {
					t.Fatalf("unexpected import page: %+v", page)
				}
				return []ImportJobSummary{{ID: 2}}, nil
			},
			listImportErrorsFn: func(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter, page Page) ([]ImportErrorItem, error) {
				errFilter = filter
				return []ImportErrorItem{{ImportJobID: 3}}, nil
			},
			listSyncCursorsFn: func(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter, page Page) ([]SyncCursorItem, error) {
				cursorFilter = filter
				return []SyncCursorItem{{ID: 4}}, nil
			},
		},
	})

	if _, err := svc.ListSyncJobs(context.Background(), 1, SyncJobFilter{Status: " running ", Limit: 0, Offset: -1}); err != nil {
		t.Fatal(err)
	}
	if syncFilter.Status != "running" {
		t.Fatalf("unexpected sync status filter: %q", syncFilter.Status)
	}
	if _, err := svc.ListImportJobs(context.Background(), 1, ImportJobFilter{Status: "completed", Domain: " products ", Limit: 999, Offset: -1}); err != nil {
		t.Fatal(err)
	}
	if importFilter.Domain != "products" || importFilter.Status != "completed" {
		t.Fatalf("unexpected import filter: %+v", importFilter)
	}
	if _, err := svc.ListImportErrors(context.Background(), 1, ImportErrorFilter{Domain: "orders"}); err != nil {
		t.Fatal(err)
	}
	if errFilter.Domain != "orders" {
		t.Fatalf("unexpected import errors filter: %+v", errFilter)
	}
	if _, err := svc.ListSyncCursors(context.Background(), 1, SyncCursorFilter{Domain: "ads"}); err != nil {
		t.Fatal(err)
	}
	if cursorFilter.Domain != "ads" {
		t.Fatalf("unexpected cursor filter: %+v", cursorFilter)
	}
}

func TestFeedbackLists(t *testing.T) {
	var gotChat ChatFeedbackFilter
	var gotRec RecommendationFeedbackFilter
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			listChatFeedbackFn: func(ctx context.Context, filter ChatFeedbackFilter, page Page) ([]ChatFeedbackItem, error) {
				gotChat = filter
				return []ChatFeedbackItem{{ID: 1, SellerAccountID: 3, MessageID: 10, SessionID: 11, Rating: "negative"}}, nil
			},
			listRecommendationFeedbackFn: func(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter, page Page) ([]RecommendationFeedbackItem, error) {
				gotRec = filter
				return []RecommendationFeedbackItem{{ID: 1, SellerAccountID: sellerAccountID, RecommendationID: 9, Rating: "positive"}}, nil
			},
			getRecommendationProxyFeedbackCountsFn: func(ctx context.Context, sellerAccountID int64) (RecommendationFeedbackProxyStatus, error) {
				return RecommendationFeedbackProxyStatus{AcceptedCount: 1}, nil
			},
		},
	})
	chat, err := svc.ListChatFeedback(context.Background(), ChatFeedbackFilter{Rating: " negative ", Limit: 0})
	if err != nil || len(chat.Items) != 1 || gotChat.Rating != "negative" {
		t.Fatalf("unexpected chat feedback result/filter: %+v %+v %v", chat, gotChat, err)
	}
	rec, err := svc.ListRecommendationFeedback(context.Background(), 3, RecommendationFeedbackFilter{Rating: " positive ", Status: " accepted "})
	if err != nil || len(rec.Items) != 1 || gotRec.Rating != "positive" || gotRec.Status != "accepted" || rec.ProxyStatusFeedback.AcceptedCount != 1 {
		t.Fatalf("unexpected recommendation feedback result/filter: %+v %+v %v", rec, gotRec, err)
	}
}

func TestActionsWithDependencies(t *testing.T) {
	repo := &fakeRepo{
		createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
			return &AdminActionLog{ID: 1, Status: AdminActionStatusRunning}, nil
		},
		completeActionFn: func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
			return &AdminActionLog{ID: input.ID, Status: AdminActionStatusCompleted, ResultPayload: input.ResultPayload}, nil
		},
		failActionFn: func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
			return &AdminActionLog{ID: input.ID, Status: AdminActionStatusFailed}, nil
		},
	}
	svc, _ := NewService(ServiceDeps{
		Repo:                   repo,
		IngestionService:       fakeIngestion{},
		CursorService:          fakeCursor{},
		AlertsService:          fakeAlerts{},
		RecommendationsService: fakeRecommendations{},
	})

	if _, err := svc.RerunSync(context.Background(), AdminActor{Email: "admin@example.com"}, RerunSyncInput{SellerAccountID: 1, SyncType: "initial_sync"}); err != nil {
		t.Fatal(err)
	}
	cv := "2026-01-01T00:00:00Z"
	if _, err := svc.ResetCursor(context.Background(), AdminActor{Email: "admin@example.com"}, ResetCursorInput{SellerAccountID: 1, Domain: "orders", CursorType: "since", CursorValue: &cv}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RerunAlerts(context.Background(), AdminActor{Email: "admin@example.com"}, RerunAlertsInput{SellerAccountID: 1, AsOfDate: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RerunRecommendations(context.Background(), AdminActor{Email: "admin@example.com"}, RerunRecommendationsInput{SellerAccountID: 1, AsOfDate: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

var _ Repository = (*fakeRepo)(nil)
