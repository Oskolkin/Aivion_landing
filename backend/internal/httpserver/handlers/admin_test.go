package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/admin"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type fakeAdminService struct {
	listClientsFn                func(ctx context.Context, filter admin.ClientListFilter) (*admin.ClientListResult, error)
	getClientDetailFn            func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error)
	listSyncJobsFn               func(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error)
	listImportJobsFn             func(ctx context.Context, sellerAccountID int64, filter admin.ImportJobFilter) (*admin.ImportJobListResult, error)
	listImportErrorsFn           func(ctx context.Context, sellerAccountID int64, filter admin.ImportErrorFilter) (*admin.ImportErrorListResult, error)
	listSyncCursorsFn            func(ctx context.Context, sellerAccountID int64, filter admin.SyncCursorFilter) (*admin.SyncCursorListResult, error)
	rerunSyncFn                  func(ctx context.Context, actor admin.AdminActor, input admin.RerunSyncInput) (*admin.AdminActionLog, error)
	resetCursorFn                func(ctx context.Context, actor admin.AdminActor, input admin.ResetCursorInput) (*admin.AdminActionLog, error)
	rerunMetricsFn               func(ctx context.Context, actor admin.AdminActor, input admin.RerunMetricsInput) (*admin.AdminActionLog, error)
	rerunAlertsFn                func(ctx context.Context, actor admin.AdminActor, input admin.RerunAlertsInput) (*admin.AdminActionLog, error)
	rerunRecommendationsFn       func(ctx context.Context, actor admin.AdminActor, input admin.RerunRecommendationsInput) (*admin.AdminActionLog, error)
	getAIRecommendationLogsFn    func(ctx context.Context, sellerAccountID int64, filter admin.RecommendationRunLogFilter) (*admin.RecommendationRunLogListResult, error)
	getRecommendationRunDetailFn func(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error)
	getRecommendationRawAIFn     func(ctx context.Context, actor admin.AdminActor, sellerAccountID, recommendationID int64) (*admin.RecommendationRawAI, error)
	getAIChatLogsFn              func(ctx context.Context, sellerAccountID int64, filter admin.ChatTraceFilter) (*admin.ChatTraceListResult, error)
	getChatTraceDetailFn         func(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error)
	listChatSessionsFn           func(ctx context.Context, sellerAccountID int64, filter admin.ChatSessionFilter) (*admin.ChatSessionListResult, error)
	listChatMessagesFn           func(ctx context.Context, sellerAccountID, sessionID int64, filter admin.ChatMessageFilter) (*admin.ChatMessageListResult, error)
	listChatFeedbackFn           func(ctx context.Context, filter admin.ChatFeedbackFilter) (*admin.ChatFeedbackListResult, error)
	listRecommendationFeedbackFn func(ctx context.Context, sellerAccountID int64, filter admin.RecommendationFeedbackFilter) (*admin.RecommendationFeedbackListResult, error)
	getBillingStateFn            func(ctx context.Context, sellerAccountID int64) (*admin.BillingState, error)
	listBillingStatesFn          func(ctx context.Context, filter admin.BillingStateFilter) ([]admin.BillingState, error)
	updateBillingStateFn         func(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error)
}

func (f *fakeAdminService) ListClients(ctx context.Context, filter admin.ClientListFilter) (*admin.ClientListResult, error) {
	return f.listClientsFn(ctx, filter)
}
func (f *fakeAdminService) GetClientDetail(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
	return f.getClientDetailFn(ctx, sellerAccountID)
}
func (f *fakeAdminService) ListSyncJobs(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error) {
	return f.listSyncJobsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) ListImportJobs(ctx context.Context, sellerAccountID int64, filter admin.ImportJobFilter) (*admin.ImportJobListResult, error) {
	return f.listImportJobsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) ListImportErrors(ctx context.Context, sellerAccountID int64, filter admin.ImportErrorFilter) (*admin.ImportErrorListResult, error) {
	return f.listImportErrorsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) ListSyncCursors(ctx context.Context, sellerAccountID int64, filter admin.SyncCursorFilter) (*admin.SyncCursorListResult, error) {
	return f.listSyncCursorsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) RerunSync(ctx context.Context, actor admin.AdminActor, input admin.RerunSyncInput) (*admin.AdminActionLog, error) {
	return f.rerunSyncFn(ctx, actor, input)
}
func (f *fakeAdminService) ResetCursor(ctx context.Context, actor admin.AdminActor, input admin.ResetCursorInput) (*admin.AdminActionLog, error) {
	return f.resetCursorFn(ctx, actor, input)
}
func (f *fakeAdminService) RerunMetrics(ctx context.Context, actor admin.AdminActor, input admin.RerunMetricsInput) (*admin.AdminActionLog, error) {
	return f.rerunMetricsFn(ctx, actor, input)
}
func (f *fakeAdminService) RerunAlerts(ctx context.Context, actor admin.AdminActor, input admin.RerunAlertsInput) (*admin.AdminActionLog, error) {
	return f.rerunAlertsFn(ctx, actor, input)
}
func (f *fakeAdminService) RerunRecommendations(ctx context.Context, actor admin.AdminActor, input admin.RerunRecommendationsInput) (*admin.AdminActionLog, error) {
	return f.rerunRecommendationsFn(ctx, actor, input)
}
func (f *fakeAdminService) GetAIRecommendationLogs(ctx context.Context, sellerAccountID int64, filter admin.RecommendationRunLogFilter) (*admin.RecommendationRunLogListResult, error) {
	return f.getAIRecommendationLogsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) GetRecommendationRunDetail(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error) {
	return f.getRecommendationRunDetailFn(ctx, actor, sellerAccountID, runID)
}
func (f *fakeAdminService) GetRecommendationRawAI(ctx context.Context, actor admin.AdminActor, sellerAccountID, recommendationID int64) (*admin.RecommendationRawAI, error) {
	return f.getRecommendationRawAIFn(ctx, actor, sellerAccountID, recommendationID)
}
func (f *fakeAdminService) GetAIChatLogs(ctx context.Context, sellerAccountID int64, filter admin.ChatTraceFilter) (*admin.ChatTraceListResult, error) {
	return f.getAIChatLogsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) GetChatTraceDetail(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error) {
	return f.getChatTraceDetailFn(ctx, actor, sellerAccountID, traceID)
}
func (f *fakeAdminService) ListChatSessions(ctx context.Context, sellerAccountID int64, filter admin.ChatSessionFilter) (*admin.ChatSessionListResult, error) {
	return f.listChatSessionsFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, filter admin.ChatMessageFilter) (*admin.ChatMessageListResult, error) {
	return f.listChatMessagesFn(ctx, sellerAccountID, sessionID, filter)
}
func (f *fakeAdminService) ListChatFeedback(ctx context.Context, filter admin.ChatFeedbackFilter) (*admin.ChatFeedbackListResult, error) {
	return f.listChatFeedbackFn(ctx, filter)
}
func (f *fakeAdminService) ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter admin.RecommendationFeedbackFilter) (*admin.RecommendationFeedbackListResult, error) {
	return f.listRecommendationFeedbackFn(ctx, sellerAccountID, filter)
}
func (f *fakeAdminService) GetBillingState(ctx context.Context, sellerAccountID int64) (*admin.BillingState, error) {
	return f.getBillingStateFn(ctx, sellerAccountID)
}
func (f *fakeAdminService) ListBillingStates(ctx context.Context, filter admin.BillingStateFilter) ([]admin.BillingState, error) {
	return f.listBillingStatesFn(ctx, filter)
}
func (f *fakeAdminService) UpdateBillingState(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error) {
	return f.updateBillingStateFn(ctx, actor, input)
}

func TestAdminHandler(t *testing.T) {
	h := NewAdminHandler(&fakeAdminService{})

	t.Run("returns unauthorized when user missing in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
		rr := httptest.NewRecorder()

		h.Me(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("returns admin payload for authenticated user", func(t *testing.T) {
		user := dbgen.User{Email: "admin@example.com"}
		seller := dbgen.SellerAccount{}
		ctx := auth.WithAuthContext(context.Background(), user, &seller)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		h.Me(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			IsAdmin bool   `json:"is_admin"`
			Email   string `json:"email"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !body.IsAdmin {
			t.Fatalf("is_admin = %v, want true", body.IsAdmin)
		}
		if body.Email != "admin@example.com" {
			t.Fatalf("email = %q, want %q", body.Email, "admin@example.com")
		}
	})

	t.Run("list clients success", func(t *testing.T) {
		var got admin.ClientListFilter
		now := time.Now().UTC()
		h := NewAdminHandler(&fakeAdminService{
			listClientsFn: func(ctx context.Context, filter admin.ClientListFilter) (*admin.ClientListResult, error) {
				got = filter
				status := "connected"
				return &admin.ClientListResult{
					Items: []admin.ClientListItem{{
						SellerAccountID: 1, SellerName: "Demo", UserEmail: "seller@example.com",
						SellerStatus: "active", ConnectionStatus: &status, CreatedAt: now, UpdatedAt: now,
					}},
					Limit: 50, Offset: 0,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients?search=demo&status=active&connection_status=connected&limit=50&offset=0", nil)
		rr := httptest.NewRecorder()
		h.ListClients(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Search != "demo" || got.SellerStatus != "active" || got.ConnectionStatus != "connected" || got.Limit != 50 || got.Offset != 0 {
			t.Fatalf("unexpected filter: %+v", got)
		}
		var body struct {
			Items  []map[string]any `json:"items"`
			Limit  int32            `json:"limit"`
			Offset int32            `json:"offset"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 || body.Limit != 50 || body.Offset != 0 {
			t.Fatalf("unexpected body: %+v", body)
		}
	})

	t.Run("list clients invalid limit", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listClientsFn: func(ctx context.Context, filter admin.ClientListFilter) (*admin.ClientListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients?limit=abc", nil)
		rr := httptest.NewRecorder()
		h.ListClients(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("get client detail success", func(t *testing.T) {
		now := time.Now().UTC()
		h := NewAdminHandler(&fakeAdminService{
			getClientDetailFn: func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
				return &admin.ClientDetail{
					Overview: admin.ClientOverview{
						SellerAccountID: sellerAccountID,
						SellerName:      "Demo",
						SellerStatus:    "active",
						CreatedAt:       now,
						UpdatedAt:       now,
					},
					Connections: []admin.ClientConnection{{Provider: "ozon", ConnectionStatus: "connected"}},
					OperationalStatus: admin.OperationalStatus{
						LatestImportJobs: []admin.ImportJobSummary{},
						Limitations:      []string{},
					},
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.GetClientDetail(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("get client detail invalid id", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getClientDetailFn: func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/x", nil)
		req = withChiURLParam(req, "seller_account_id", "x")
		rr := httptest.NewRecorder()
		h.GetClientDetail(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("get client detail not found", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getClientDetailFn: func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
				return nil, pgx.ErrNoRows
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/999", nil)
		req = withChiURLParam(req, "seller_account_id", "999")
		rr := httptest.NewRecorder()
		h.GetClientDetail(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("get client detail seller validation error", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getClientDetailFn: func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
				return nil, admin.ErrSellerAccountIDRequired
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/0", nil)
		req = withChiURLParam(req, "seller_account_id", "0")
		rr := httptest.NewRecorder()
		h.GetClientDetail(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("get client detail internal error", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getClientDetailFn: func(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error) {
				return nil, errors.New("boom")
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.GetClientDetail(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("sync-jobs success and filters", func(t *testing.T) {
		var got admin.SyncJobFilter
		h := NewAdminHandler(&fakeAdminService{
			listSyncJobsFn: func(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error) {
				got = filter
				return &admin.SyncJobListResult{
					Items:  []admin.SyncJobSummary{{ID: 1, Type: "initial_sync", Status: "completed"}},
					Limit:  20,
					Offset: 10,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/sync-jobs?status=completed&limit=20&offset=10", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListSyncJobs(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Status != "completed" || got.Limit != 20 || got.Offset != 10 {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("sync-jobs invalid seller id", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listSyncJobsFn: func(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/x/sync-jobs", nil)
		req = withChiURLParam(req, "seller_account_id", "x")
		rr := httptest.NewRecorder()
		h.ListSyncJobs(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("sync-jobs invalid limit", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listSyncJobsFn: func(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/sync-jobs?limit=bad", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListSyncJobs(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("import-jobs success with filters", func(t *testing.T) {
		var got admin.ImportJobFilter
		h := NewAdminHandler(&fakeAdminService{
			listImportJobsFn: func(ctx context.Context, sellerAccountID int64, filter admin.ImportJobFilter) (*admin.ImportJobListResult, error) {
				got = filter
				return &admin.ImportJobListResult{Items: []admin.ImportJobSummary{{ID: 1}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/import-jobs?status=failed&domain=products", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListImportJobs(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Status != "failed" || got.Domain != "products" {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("import-errors success with domain", func(t *testing.T) {
		var got admin.ImportErrorFilter
		h := NewAdminHandler(&fakeAdminService{
			listImportErrorsFn: func(ctx context.Context, sellerAccountID int64, filter admin.ImportErrorFilter) (*admin.ImportErrorListResult, error) {
				got = filter
				return &admin.ImportErrorListResult{Items: []admin.ImportErrorItem{{ImportJobID: 1}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/import-errors?domain=orders", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListImportErrors(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Domain != "orders" {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("sync-cursors success with domain", func(t *testing.T) {
		var got admin.SyncCursorFilter
		h := NewAdminHandler(&fakeAdminService{
			listSyncCursorsFn: func(ctx context.Context, sellerAccountID int64, filter admin.SyncCursorFilter) (*admin.SyncCursorListResult, error) {
				got = filter
				return &admin.SyncCursorListResult{Items: []admin.SyncCursorItem{{Domain: "ads", CursorType: "since"}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/sync-cursors?domain=ads", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListSyncCursors(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Domain != "ads" {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("sync-cursors service error returns 500", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listSyncCursorsFn: func(ctx context.Context, sellerAccountID int64, filter admin.SyncCursorFilter) (*admin.SyncCursorListResult, error) {
				return nil, errors.New("boom")
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/sync-cursors", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListSyncCursors(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

func withChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx, _ := r.Context().Value(chi.RouteCtxKey).(*chi.Context)
	if rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withAdminUser(req *http.Request) *http.Request {
	ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 99, Email: "admin@example.com"}, nil)
	return req.WithContext(ctx)
}

func TestAdminHandlerActions(t *testing.T) {
	adminCtx := func(req *http.Request) *http.Request {
		ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 99, Email: "admin@example.com"}, nil)
		return req.WithContext(ctx)
	}

	t.Run("rerun-sync success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			rerunSyncFn: func(ctx context.Context, actor admin.AdminActor, input admin.RerunSyncInput) (*admin.AdminActionLog, error) {
				return &admin.AdminActionLog{ID: 1, AdminEmail: actor.Email, SellerAccountID: input.SellerAccountID, ActionType: admin.AdminActionRerunSync, Status: admin.AdminActionStatusCompleted, RequestPayload: map[string]any{}, ResultPayload: map[string]any{}, CreatedAt: time.Now()}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/rerun-sync", strings.NewReader(`{"sync_type":"initial_sync"}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.RerunSync(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("reset-cursor missing domain", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{resetCursorFn: func(ctx context.Context, actor admin.AdminActor, input admin.ResetCursorInput) (*admin.AdminActionLog, error) {
			return nil, nil
		}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/reset-cursor", strings.NewReader(`{"cursor_type":"since"}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.ResetCursor(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("rerun-metrics invalid date", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{rerunMetricsFn: func(ctx context.Context, actor admin.AdminActor, input admin.RerunMetricsInput) (*admin.AdminActionLog, error) {
			return nil, nil
		}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/rerun-metrics", strings.NewReader(`{"date_from":"bad","date_to":"2026-04-30"}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.RerunMetrics(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("rerun-metrics partial date range rejected", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{rerunMetricsFn: func(ctx context.Context, actor admin.AdminActor, input admin.RerunMetricsInput) (*admin.AdminActionLog, error) {
			t.Fatal("service should not be called")
			return nil, nil
		}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/rerun-metrics", strings.NewReader(`{"date_from":"2026-01-01","date_to":""}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.RerunMetrics(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("rerun-alerts success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			rerunAlertsFn: func(ctx context.Context, actor admin.AdminActor, input admin.RerunAlertsInput) (*admin.AdminActionLog, error) {
				return &admin.AdminActionLog{ID: 2, AdminEmail: actor.Email, SellerAccountID: input.SellerAccountID, ActionType: admin.AdminActionRerunAlerts, Status: admin.AdminActionStatusCompleted, RequestPayload: map[string]any{}, ResultPayload: map[string]any{}, CreatedAt: time.Now()}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/rerun-alerts", strings.NewReader(`{"as_of_date":"2026-04-30"}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.RerunAlerts(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("rerun-recommendations not configured", func(t *testing.T) {
		msg := admin.ErrAdminActionNotConfigured.Error()
		h := NewAdminHandler(&fakeAdminService{
			rerunRecommendationsFn: func(ctx context.Context, actor admin.AdminActor, input admin.RerunRecommendationsInput) (*admin.AdminActionLog, error) {
				return &admin.AdminActionLog{ID: 3, AdminEmail: actor.Email, SellerAccountID: input.SellerAccountID, ActionType: admin.AdminActionRerunRecommendations, Status: admin.AdminActionStatusFailed, ErrorMessage: &msg, RequestPayload: map[string]any{}, ResultPayload: map[string]any{}, CreatedAt: time.Now()}, admin.ErrAdminActionNotConfigured
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/1/actions/rerun-recommendations", strings.NewReader(`{"as_of_date":"2026-04-30"}`))
		req = adminCtx(withChiURLParam(req, "seller_account_id", "1"))
		rr := httptest.NewRecorder()
		h.RerunRecommendations(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestAdminRecommendationLogsHandlers(t *testing.T) {
	t.Run("list recommendation runs success", func(t *testing.T) {
		var got admin.RecommendationRunLogFilter
		h := NewAdminHandler(&fakeAdminService{
			getAIRecommendationLogsFn: func(ctx context.Context, sellerAccountID int64, filter admin.RecommendationRunLogFilter) (*admin.RecommendationRunLogListResult, error) {
				got = filter
				return &admin.RecommendationRunLogListResult{
					Items: []admin.RecommendationRunLogItem{{ID: 101, RunType: "manual", Status: "completed"}},
					Limit: 50, Offset: 0,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs?status=completed&run_type=manual&limit=50&offset=0", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListRecommendationRuns(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Status != "completed" || got.RunType != "manual" || got.Limit != 50 || got.Offset != 0 {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("list recommendation runs invalid limit", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getAIRecommendationLogsFn: func(ctx context.Context, sellerAccountID int64, filter admin.RecommendationRunLogFilter) (*admin.RecommendationRunLogListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs?limit=bad", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListRecommendationRuns(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("run detail success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRunDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error) {
				return &admin.RecommendationRunDetail{
					Run:             admin.RecommendationRunSummary{ID: runID, RunType: "manual", Status: "completed"},
					Recommendations: []admin.RecommendationItem{{ID: 500, RecommendationType: "pricing", Title: "t", Status: "open"}},
					Diagnostics:     []admin.RecommendationRunDiagnosticItem{{ID: 1}},
					Limitations:     []string{"Rejected item payloads are unavailable for historical runs."},
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs/10", nil)
		req = withAdminUser(withChiURLParam(req, "seller_account_id", "1"))
		req = withChiURLParam(req, "run_id", "10")
		rr := httptest.NewRecorder()
		h.GetRecommendationRunDetail(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("run detail not found", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRunDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error) {
				return nil, pgx.ErrNoRows
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs/10", nil)
		req = withAdminUser(withChiURLParam(req, "seller_account_id", "1"))
		req = withChiURLParam(req, "run_id", "10")
		rr := httptest.NewRecorder()
		h.GetRecommendationRunDetail(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("run detail requires admin actor", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRunDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs/10", nil)
		req = withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "run_id", "10")
		rr := httptest.NewRecorder()
		h.GetRecommendationRunDetail(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("run detail audit log failure returns 503", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRunDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error) {
				return nil, admin.ErrAdminAuditLogWriteFailed
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendation-runs/10", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "run_id", "10"))
		rr := httptest.NewRecorder()
		h.GetRecommendationRunDetail(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("recommendation raw ai success", func(t *testing.T) {
		now := time.Now().UTC()
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRawAIFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, recommendationID int64) (*admin.RecommendationRawAI, error) {
				return &admin.RecommendationRawAI{
					Recommendation: admin.RecommendationItem{
						ID:                       recommendationID,
						RecommendationType:       "pricing",
						Title:                    "x",
						Status:                   "open",
						SupportingMetricsPayload: map[string]any{},
						ConstraintsPayload:       map[string]any{},
						RawAIResponse:            map[string]any{"admin_only": true},
						CreatedAt:                now,
						UpdatedAt:                now,
					},
					RelatedAlerts: []admin.RecommendationAlertItem{{ID: 10, AlertType: "stockout"}},
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendations/1", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "id", "1"))
		rr := httptest.NewRecorder()
		h.GetRecommendationRawAI(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		rec, ok := body["recommendation"].(map[string]any)
		if !ok {
			t.Fatalf("expected recommendation object, got %#v", body["recommendation"])
		}
		raw, ok := rec["raw_ai_response"].(map[string]any)
		if !ok || raw["admin_only"] != true {
			t.Fatalf("admin raw endpoint must include raw_ai_response; got %#v", rec["raw_ai_response"])
		}
		// Audit log write is enforced in admin.TestViewRawAIRawSuccessWritesAuditLog (service layer).
	})

	t.Run("recommendation raw ai invalid id", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getRecommendationRawAIFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, recommendationID int64) (*admin.RecommendationRawAI, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/recommendations/x", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "id", "x"))
		rr := httptest.NewRecorder()
		h.GetRecommendationRawAI(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestAdminChatLogsHandlers(t *testing.T) {
	t.Run("list chat traces success", func(t *testing.T) {
		var got admin.ChatTraceFilter
		h := NewAdminHandler(&fakeAdminService{
			getAIChatLogsFn: func(ctx context.Context, sellerAccountID int64, filter admin.ChatTraceFilter) (*admin.ChatTraceListResult, error) {
				got = filter
				return &admin.ChatTraceListResult{Items: []admin.ChatTraceLogItem{{ID: 1, SessionID: 10}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces?status=completed&intent=priorities&session_id=10&limit=50&offset=0", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListChatTraces(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if got.Status != "completed" || got.Intent != "priorities" || got.SessionID == nil || *got.SessionID != 10 {
			t.Fatalf("unexpected filter: %+v", got)
		}
	})

	t.Run("list chat traces invalid session query", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getAIChatLogsFn: func(ctx context.Context, sellerAccountID int64, filter admin.ChatTraceFilter) (*admin.ChatTraceListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces?session_id=bad", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListChatTraces(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("chat trace detail success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getChatTraceDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error) {
				return &admin.ChatTraceDetail{
					ID: traceID, SessionID: 10, Status: "completed",
					Messages: []admin.ChatMessageItem{{ID: 1001, SessionID: 10, Role: "user", MessageType: "question", Content: "q", CreatedAt: time.Now()}},
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces/1", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "trace_id", "1"))
		rr := httptest.NewRecorder()
		h.GetChatTraceDetail(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("chat trace detail not found", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getChatTraceDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error) {
				return nil, pgx.ErrNoRows
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces/1", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "trace_id", "1"))
		rr := httptest.NewRecorder()
		h.GetChatTraceDetail(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("chat trace detail requires admin actor", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getChatTraceDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces/1", nil)
		req = withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "trace_id", "1")
		rr := httptest.NewRecorder()
		h.GetChatTraceDetail(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("chat trace detail audit log failure returns 503", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getChatTraceDetailFn: func(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error) {
				return nil, admin.ErrAdminAuditLogWriteFailed
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/ai/chat-traces/1", nil)
		req = withAdminUser(withChiURLParam(withChiURLParam(req, "seller_account_id", "1"), "trace_id", "1"))
		rr := httptest.NewRecorder()
		h.GetChatTraceDetail(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("list chat sessions success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listChatSessionsFn: func(ctx context.Context, sellerAccountID int64, filter admin.ChatSessionFilter) (*admin.ChatSessionListResult, error) {
				return &admin.ChatSessionListResult{Items: []admin.ChatSessionItem{{ID: 10, Title: "t", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/chat/sessions?status=active", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListChatSessions(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("list chat messages success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listChatMessagesFn: func(ctx context.Context, sellerAccountID, sessionID int64, filter admin.ChatMessageFilter) (*admin.ChatMessageListResult, error) {
				return &admin.ChatMessageListResult{Items: []admin.ChatMessageItem{{ID: 1, SessionID: sessionID, Role: "user", MessageType: "question", Content: "q", CreatedAt: time.Now()}}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/chat/sessions/10/messages", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		req = withChiURLParam(req, "session_id", "10")
		rr := httptest.NewRecorder()
		h.ListChatMessages(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("list chat messages invalid session id", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listChatMessagesFn: func(ctx context.Context, sellerAccountID, sessionID int64, filter admin.ChatMessageFilter) (*admin.ChatMessageListResult, error) {
				t.Fatalf("service should not be called")
				return nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/chat/sessions/x/messages", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		req = withChiURLParam(req, "session_id", "x")
		rr := httptest.NewRecorder()
		h.ListChatMessages(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestAdminFeedbackHandlers(t *testing.T) {
	t.Run("client chat feedback success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listChatFeedbackFn: func(ctx context.Context, filter admin.ChatFeedbackFilter) (*admin.ChatFeedbackListResult, error) {
				return &admin.ChatFeedbackListResult{
					Items: []admin.ChatFeedbackItem{{ID: 1, SellerAccountID: 1, SessionID: 2, MessageID: 3, Rating: "negative"}},
					Limit: 50, Offset: 0,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/feedback/chat?rating=negative", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListChatFeedbackByClient(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("global chat feedback success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listChatFeedbackFn: func(ctx context.Context, filter admin.ChatFeedbackFilter) (*admin.ChatFeedbackListResult, error) {
				return &admin.ChatFeedbackListResult{Items: []admin.ChatFeedbackItem{}, Limit: 50, Offset: 0}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/feedback/chat?seller_account_id=1", nil)
		rr := httptest.NewRecorder()
		h.ListAllChatFeedback(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("recommendation feedback success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listRecommendationFeedbackFn: func(ctx context.Context, sellerAccountID int64, filter admin.RecommendationFeedbackFilter) (*admin.RecommendationFeedbackListResult, error) {
				return &admin.RecommendationFeedbackListResult{
					Items: []admin.RecommendationFeedbackItem{{ID: 1, SellerAccountID: sellerAccountID, RecommendationID: 10, Rating: "positive"}},
					Limit: 50, Offset: 0,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/feedback/recommendations", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.ListRecommendationFeedbackByClient(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}

func TestAdminBillingHandlers(t *testing.T) {
	adminCtx := func(req *http.Request) *http.Request {
		ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 99, Email: "admin@example.com"}, nil)
		return req.WithContext(ctx)
	}
	t.Run("get client billing success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getBillingStateFn: func(ctx context.Context, sellerAccountID int64) (*admin.BillingState, error) {
				return &admin.BillingState{SellerAccountID: sellerAccountID, PlanCode: "trial", Status: admin.BillingStatusTrial, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/billing", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.GetBillingStateByClient(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
	t.Run("get client billing not found", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			getBillingStateFn: func(ctx context.Context, sellerAccountID int64) (*admin.BillingState, error) { return nil, nil },
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients/1/billing", nil)
		req = withChiURLParam(req, "seller_account_id", "1")
		rr := httptest.NewRecorder()
		h.GetBillingStateByClient(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
	t.Run("put billing success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			updateBillingStateFn: func(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error) {
				return &admin.BillingState{SellerAccountID: input.SellerAccountID, PlanCode: input.PlanCode, Status: input.Status, CreatedAt: time.Now(), UpdatedAt: time.Now()},
					&admin.AdminActionLog{ID: 1, AdminEmail: actor.Email, SellerAccountID: input.SellerAccountID, ActionType: admin.AdminActionUpdateBillingState, Status: admin.AdminActionStatusCompleted, CreatedAt: time.Now()}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/clients/1/billing", strings.NewReader(`{"plan_code":"trial","status":"trial","ai_tokens_used_month":0,"estimated_ai_cost_month":0}`))
		req = withChiURLParam(req, "seller_account_id", "1")
		req = adminCtx(req)
		rr := httptest.NewRecorder()
		h.UpdateBillingStateByClient(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
	t.Run("put billing invalid timestamp", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			updateBillingStateFn: func(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error) {
				t.Fatalf("service should not be called")
				return nil, nil, nil
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/clients/1/billing", strings.NewReader(`{"plan_code":"trial","status":"trial","trial_ends_at":"bad"}`))
		req = withChiURLParam(req, "seller_account_id", "1")
		req = adminCtx(req)
		rr := httptest.NewRecorder()
		h.UpdateBillingStateByClient(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
	t.Run("put billing invalid status", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			updateBillingStateFn: func(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error) {
				return nil, nil, errors.New("invalid billing status")
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/clients/1/billing", strings.NewReader(`{"plan_code":"trial","status":"bad"}`))
		req = withChiURLParam(req, "seller_account_id", "1")
		req = adminCtx(req)
		rr := httptest.NewRecorder()
		h.UpdateBillingStateByClient(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
	t.Run("get billing list success", func(t *testing.T) {
		h := NewAdminHandler(&fakeAdminService{
			listBillingStatesFn: func(ctx context.Context, filter admin.BillingStateFilter) ([]admin.BillingState, error) {
				return []admin.BillingState{{SellerAccountID: 1, PlanCode: "trial", Status: admin.BillingStatusTrial, CreatedAt: time.Now(), UpdatedAt: time.Now()}}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing?status=trial&limit=10&offset=0", nil)
		rr := httptest.NewRecorder()
		h.ListBillingStates(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})
}
