package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/go-chi/chi/v5"
)

func withSeller(req *http.Request, sellerID int64) *http.Request {
	sa := dbgen.SellerAccount{ID: sellerID}
	ctx := auth.WithAuthContext(req.Context(), dbgen.User{}, &sa)
	return req.WithContext(ctx)
}

func withActionKey(req *http.Request, actionKey string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("action_key", actionKey)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestActionsHandler_ListActions_EmptyOK(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions", nil)
	req = withSeller(req, 1)
	rr := httptest.NewRecorder()
	h.ListActions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp actions.ActionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(actions.ActionResponseStatusNotGenerated) {
		t.Fatalf("status=%q", resp.Status)
	}
}

func TestActionsHandler_ListActions_InvalidFilter(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions?status=unknown", nil)
	req = withSeller(req, 1)
	rr := httptest.NewRecorder()
	h.ListActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestActionsHandler_GetAction_NotFound(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/missing.key", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "missing.key")
	rr := httptest.NewRecorder()
	h.GetAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_CheckAction_NoContent(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 1,
				ActionKey:       "do.check",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "t",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/do.check/check", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "do.check")
	rr := httptest.NewRecorder()
	h.CheckAction(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rr.Code)
	}
}

func TestActionsHandler_ExportAction_XLSX(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 1,
				ActionKey:       "exp.key",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Export test",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/exp.key/export?format=xlsx", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "exp.key")
	rr := httptest.NewRecorder()
	h.ExportAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got == "" {
		t.Fatalf("missing content-type")
	}
	if got := rr.Header().Get("Content-Disposition"); got == "" {
		t.Fatalf("missing content-disposition")
	}
	if !strings.Contains(rr.Header().Get("Content-Disposition"), "aivion-action-") {
		t.Fatalf("unexpected content-disposition: %s", rr.Header().Get("Content-Disposition"))
	}
	if rr.Body.Len() == 0 {
		t.Fatalf("empty export body")
	}
}

func TestActionsHandler_ExportAction_CSV(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 1,
				ActionKey:       "exp.csv",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Export csv",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/exp.csv/export?format=csv", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "exp.csv")
	rr := httptest.NewRecorder()
	h.ExportAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("content-type=%q", got)
	}
	if !strings.Contains(rr.Header().Get("Content-Disposition"), ".csv") {
		t.Fatalf("unexpected disposition: %s", rr.Header().Get("Content-Disposition"))
	}
}

func TestActionsHandler_ExportAction_UnsupportedFormat(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/x/export?format=pdf", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "x")
	rr := httptest.NewRecorder()
	h.ExportAction(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestActionsHandler_ExportAction_UnknownActionKey(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/missing/export?format=xlsx", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "missing")
	rr := httptest.NewRecorder()
	h.ExportAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_ExportAction_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 2,
				ActionKey:       "foreign.key",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/foreign.key/export?format=xlsx", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "foreign.key")
	rr := httptest.NewRecorder()
	h.ExportAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func withActionID(req *http.Request, actionID int64) *http.Request {
	rctx := chi.RouteContext(req.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add("id", strconv.FormatInt(actionID, 10))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestActionsHandler_GetAction_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 2,
				ActionKey:       "foreign.get",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/foreign.get", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "foreign.get")
	rr := httptest.NewRecorder()
	h.GetAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_GetActionByID_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              42,
				SellerAccountID: 2,
				ActionKey:       "foreign.by-id",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/actions/id/42", nil)
	req = withSeller(req, 1)
	req = withActionID(req, 42)
	rr := httptest.NewRecorder()
	h.GetActionByID(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_CheckAction_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 2,
				ActionKey:       "foreign.check",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/foreign.check/check", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "foreign.check")
	rr := httptest.NewRecorder()
	h.CheckAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_DismissAction_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 2,
				ActionKey:       "foreign.dismiss",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusOpen,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/foreign.dismiss/dismiss", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "foreign.dismiss")
	rr := httptest.NewRecorder()
	h.DismissAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_UncheckAction_ForeignSellerNotFound(t *testing.T) {
	repo := &actionsListRepo{
		items: []actions.ActionItemRecord{
			{
				ID:              1,
				SellerAccountID: 2,
				ActionKey:       "foreign.uncheck",
				Type:            actions.ActionTypeStockReplenishment,
				Category:        actions.ActionCategoryStock,
				Priority:        actions.ActionPriorityCritical,
				Status:          actions.ActionStatusChecked,
				Title:           "Foreign",
				Reason:          "r",
				NextStep:        "n",
			},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/foreign.uncheck/uncheck", nil)
	req = withSeller(req, 1)
	req = withActionKey(req, "foreign.uncheck")
	rr := httptest.NewRecorder()
	h.UncheckAction(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestActionsHandler_AdminListGenerationRuns_OK(t *testing.T) {
	repo := &actionsListRepo{
		adminRuns: []dbgen.ActionGenerationRun{
			{ID: 1001, SellerAccountID: 7, Status: "completed"},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs?seller_id=7", nil)
	rr := httptest.NewRecorder()
	h.AdminListGenerationRuns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "\"run_id\":1001") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestActionsHandler_AdminListGenerationRuns_FiltersAndPagination(t *testing.T) {
	repo := &actionsListRepo{
		adminRuns: []dbgen.ActionGenerationRun{
			{ID: 1001, SellerAccountID: 7, Status: "completed"},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs?seller_id=7&status=completed&final_status=completed&metric_date=2026-06-24&limit=10&offset=2", nil)
	rr := httptest.NewRecorder()
	h.AdminListGenerationRuns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if repo.lastAdminParams.SellerAccountID == nil || *repo.lastAdminParams.SellerAccountID != 7 {
		t.Fatalf("seller filter not passed: %+v", repo.lastAdminParams)
	}
	if repo.lastAdminParams.Status != "completed" || repo.lastAdminParams.FinalStatus != "completed" {
		t.Fatalf("status filters not passed: %+v", repo.lastAdminParams)
	}
	if repo.lastAdminParams.Limit != 10 || repo.lastAdminParams.Offset != 2 {
		t.Fatalf("pagination not passed: %+v", repo.lastAdminParams)
	}
}

func TestActionsHandler_AdminGetGenerationRun_OK(t *testing.T) {
	repo := &actionsListRepo{
		adminRuns: []dbgen.ActionGenerationRun{
			{ID: 55, SellerAccountID: 8, Status: "completed"},
		},
		adminCategoryRuns: []dbgen.ActionGenerationCategoryRun{
			{RunID: 55, Domain: "stock", Status: "completed"},
		},
	}
	h := &ActionsHandler{service: actions.NewService(repo)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs/55", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("run_id", "55")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.AdminGetGenerationRun(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "\"domain\":\"stock\"") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestActionsHandler_AdminGetGenerationRun_NotFound(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs/777", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("run_id", "777")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.AdminGetGenerationRun(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestActionsHandler_AdminListGenerationRuns_InvalidPagination(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs?limit=bad&offset=-1", nil)
	rr := httptest.NewRecorder()
	h.AdminListGenerationRuns(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminActionsGenerationRunsRoute_ForbidsNonAdminViaMiddleware(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{})}
	router := chi.NewRouter()
	router.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAdmin, _ := r.Context().Value("is_admin").(bool); !isAdmin {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}).Get("/api/v1/admin/actions/generation-runs", h.AdminListGenerationRuns)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/actions/generation-runs", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

type actionsListRepo struct {
	items             []actions.ActionItemRecord
	adminRuns         []dbgen.ActionGenerationRun
	adminCategoryRuns []dbgen.ActionGenerationCategoryRun
	lastAdminParams   actions.AdminListGenerationRunsParams
}

func (r *actionsListRepo) GetLatestRun(ctx context.Context, sellerAccountID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}

func (r *actionsListRepo) GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}

func (r *actionsListRepo) GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}

func (r *actionsListRepo) ListActionsByRunID(ctx context.Context, runID int64, params actions.ListActionsParams) ([]actions.ActionItemRecord, error) {
	return r.items, nil
}

func (r *actionsListRepo) GetLatestMetricDate(ctx context.Context, sellerAccountID int64) (*time.Time, error) {
	return nil, nil
}

func (r *actionsListRepo) ListActions(ctx context.Context, params actions.ListActionsParams) ([]actions.ActionItemRecord, error) {
	if len(r.items) == 0 {
		return nil, nil
	}
	return r.items, nil
}

func (r *actionsListRepo) GetActionByKey(ctx context.Context, sellerAccountID int64, actionKey string) (*actions.ActionItemRecord, error) {
	for i := range r.items {
		if r.items[i].SellerAccountID == sellerAccountID && r.items[i].ActionKey == actionKey {
			item := r.items[i]
			return &item, nil
		}
	}
	return nil, actions.ErrActionNotFound
}

func (r *actionsListRepo) GetActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*actions.ActionItemRecord, error) {
	for i := range r.items {
		if r.items[i].SellerAccountID == sellerAccountID && r.items[i].ID == actionID {
			item := r.items[i]
			return &item, nil
		}
	}
	return nil, actions.ErrActionNotFound
}

func (r *actionsListRepo) GetHistoricalActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*actions.ActionItemRecord, error) {
	return r.GetActionByID(ctx, sellerAccountID, actionID)
}

func (r *actionsListRepo) GetActionEntityGroup(ctx context.Context, sellerAccountID, actionsRunID int64, groupKey string) (*actions.ActionEntityGroup, error) {
	return nil, actions.ErrEntityGroupNotFound
}

func (r *actionsListRepo) SetActionChecked(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	return nil
}

func (r *actionsListRepo) SetActionOpen(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	return nil
}

func (r *actionsListRepo) SetActionDismissed(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	return nil
}

func (r *actionsListRepo) GetGenerationRunByID(ctx context.Context, runID int64) (*actions.ActionGenerationRunRecord, error) {
	row, err := r.GetGenerationRunForAdmin(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &actions.ActionGenerationRunRecord{
		ID:              row.ID,
		SellerAccountID: row.SellerAccountID,
		Status:          row.Status,
	}, nil
}

func (r *actionsListRepo) ListGenerationRunsForAdmin(ctx context.Context, params actions.AdminListGenerationRunsParams) ([]dbgen.ActionGenerationRun, error) {
	r.lastAdminParams = params
	return r.adminRuns, nil
}

func (r *actionsListRepo) GetGenerationRunForAdmin(ctx context.Context, runID int64) (*dbgen.ActionGenerationRun, error) {
	for i := range r.adminRuns {
		if r.adminRuns[i].ID == runID {
			row := r.adminRuns[i]
			return &row, nil
		}
	}
	return nil, actions.ErrActionNotFound
}

func (r *actionsListRepo) ListGenerationCategoryRunsForAdmin(ctx context.Context, runID int64) ([]dbgen.ActionGenerationCategoryRun, error) {
	return r.adminCategoryRuns, nil
}

type stubActionsRerun struct {
	req  actions.RerunActionsRequest
	resp *actions.RerunActionsResponse
	err  error
}

func (s *stubActionsRerun) RerunActions(ctx context.Context, req actions.RerunActionsRequest) (*actions.RerunActionsResponse, error) {
	s.req = req
	return s.resp, s.err
}

func TestActionsHandler_AdminRerunActions_ValidatesSellerID(t *testing.T) {
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{}), rerun: &stubActionsRerun{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/actions/rerun", strings.NewReader(`{"seller_id":0}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.AdminRerunActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestActionsHandler_AdminRerunActions_CallsRerunService(t *testing.T) {
	stub := &stubActionsRerun{
		resp: &actions.RerunActionsResponse{RunID: 9, Status: "completed", FinalStatus: "fallback", AcceptedCount: 2},
	}
	h := &ActionsHandler{service: actions.NewService(&actionsListRepo{}), rerun: stub}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/actions/rerun", strings.NewReader(`{"seller_id":1,"metric_date":"2026-06-24","ai_enabled":false,"force":true}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.AdminRerunActions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if stub.req.SellerAccountID != 1 {
		t.Fatalf("seller_id=%d", stub.req.SellerAccountID)
	}
	if stub.req.AIEnabled == nil || *stub.req.AIEnabled {
		t.Fatalf("expected ai_enabled=false override")
	}
}
