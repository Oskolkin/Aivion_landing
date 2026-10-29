package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/admin"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type adminService interface {
	ListClients(ctx context.Context, filter admin.ClientListFilter) (*admin.ClientListResult, error)
	GetClientDetail(ctx context.Context, sellerAccountID int64) (*admin.ClientDetail, error)
	ListSyncJobs(ctx context.Context, sellerAccountID int64, filter admin.SyncJobFilter) (*admin.SyncJobListResult, error)
	ListImportJobs(ctx context.Context, sellerAccountID int64, filter admin.ImportJobFilter) (*admin.ImportJobListResult, error)
	ListImportErrors(ctx context.Context, sellerAccountID int64, filter admin.ImportErrorFilter) (*admin.ImportErrorListResult, error)
	ListSyncCursors(ctx context.Context, sellerAccountID int64, filter admin.SyncCursorFilter) (*admin.SyncCursorListResult, error)
	RerunSync(ctx context.Context, actor admin.AdminActor, input admin.RerunSyncInput) (*admin.AdminActionLog, error)
	ResetCursor(ctx context.Context, actor admin.AdminActor, input admin.ResetCursorInput) (*admin.AdminActionLog, error)
	RerunMetrics(ctx context.Context, actor admin.AdminActor, input admin.RerunMetricsInput) (*admin.AdminActionLog, error)
	RerunAlerts(ctx context.Context, actor admin.AdminActor, input admin.RerunAlertsInput) (*admin.AdminActionLog, error)
	RerunRecommendations(ctx context.Context, actor admin.AdminActor, input admin.RerunRecommendationsInput) (*admin.AdminActionLog, error)
	GetAIRecommendationLogs(ctx context.Context, sellerAccountID int64, filter admin.RecommendationRunLogFilter) (*admin.RecommendationRunLogListResult, error)
	GetRecommendationRunDetail(ctx context.Context, actor admin.AdminActor, sellerAccountID, runID int64) (*admin.RecommendationRunDetail, error)
	GetRecommendationRawAI(ctx context.Context, actor admin.AdminActor, sellerAccountID, recommendationID int64) (*admin.RecommendationRawAI, error)
	GetAIChatLogs(ctx context.Context, sellerAccountID int64, filter admin.ChatTraceFilter) (*admin.ChatTraceListResult, error)
	GetChatTraceDetail(ctx context.Context, actor admin.AdminActor, sellerAccountID, traceID int64) (*admin.ChatTraceDetail, error)
	ListChatSessions(ctx context.Context, sellerAccountID int64, filter admin.ChatSessionFilter) (*admin.ChatSessionListResult, error)
	ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, filter admin.ChatMessageFilter) (*admin.ChatMessageListResult, error)
	ListChatFeedback(ctx context.Context, filter admin.ChatFeedbackFilter) (*admin.ChatFeedbackListResult, error)
	ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter admin.RecommendationFeedbackFilter) (*admin.RecommendationFeedbackListResult, error)
	GetBillingState(ctx context.Context, sellerAccountID int64) (*admin.BillingState, error)
	ListBillingStates(ctx context.Context, filter admin.BillingStateFilter) ([]admin.BillingState, error)
	UpdateBillingState(ctx context.Context, actor admin.AdminActor, input admin.UpdateBillingStateInput) (*admin.BillingState, *admin.AdminActionLog, error)
}

type AdminHandler struct {
	service adminService
}

func NewAdminHandler(service adminService) *AdminHandler {
	return &AdminHandler{service: service}
}

type adminMeResponse struct {
	IsAdmin bool   `json:"is_admin"`
	Email   string `json:"email"`
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, adminMeResponse{
		IsAdmin: true,
		Email:   user.Email,
	})
}

func (h *AdminHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}

	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.ListClients(r.Context(), admin.ClientListFilter{
		Search:           strings.TrimSpace(r.URL.Query().Get("search")),
		SellerStatus:     strings.TrimSpace(r.URL.Query().Get("status")),
		ConnectionStatus: strings.TrimSpace(r.URL.Query().Get("connection_status")),
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list admin clients")
		return
	}

	items := make([]adminClientItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminClientItemResponse(item))
	}
	writeJSON(w, http.StatusOK, adminClientsResponse{
		Items:  items,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func (h *AdminHandler) GetClientDetail(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}

	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}

	detail, err := h.service.GetClientDetail(r.Context(), sellerAccountID)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrSellerAccountIDRequired):
			writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "client not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to get admin client detail")
		}
		return
	}

	writeJSON(w, http.StatusOK, toAdminClientDetailResponse(*detail))
}

func (h *AdminHandler) ListSyncJobs(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListSyncJobs(r.Context(), sellerAccountID, admin.SyncJobFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list sync jobs")
		return
	}
	items := make([]adminSyncJobSummaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminSyncJobResponse(item))
	}
	writeJSON(w, http.StatusOK, adminSyncJobsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) ListImportJobs(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListImportJobs(r.Context(), sellerAccountID, admin.ImportJobFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Domain: strings.TrimSpace(r.URL.Query().Get("domain")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list import jobs")
		return
	}
	items := make([]adminImportJobSummaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminImportJobResponse(item))
	}
	writeJSON(w, http.StatusOK, adminImportJobsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) ListImportErrors(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListImportErrors(r.Context(), sellerAccountID, admin.ImportErrorFilter{
		Domain: strings.TrimSpace(r.URL.Query().Get("domain")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list import errors")
		return
	}
	items := make([]adminImportErrorResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminImportErrorResponse(item))
	}
	writeJSON(w, http.StatusOK, adminImportErrorsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) ListSyncCursors(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListSyncCursors(r.Context(), sellerAccountID, admin.SyncCursorFilter{
		Domain: strings.TrimSpace(r.URL.Query().Get("domain")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list sync cursors")
		return
	}
	items := make([]adminSyncCursorResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminSyncCursorResponse(item))
	}
	writeJSON(w, http.StatusOK, adminSyncCursorsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) RerunSync(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req rerunSyncRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	log, actionErr := h.service.RerunSync(r.Context(), actor, admin.RerunSyncInput{
		SellerAccountID: sellerAccountID,
		SyncType:        strings.TrimSpace(req.SyncType),
	})
	writeAdminActionResult(w, log, actionErr, "failed to rerun sync")
}

func (h *AdminHandler) ResetCursor(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req resetCursorRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Domain) == "" || strings.TrimSpace(req.CursorType) == "" {
		writeJSONError(w, http.StatusBadRequest, "domain and cursor_type are required")
		return
	}
	log, actionErr := h.service.ResetCursor(r.Context(), actor, admin.ResetCursorInput{
		SellerAccountID: sellerAccountID,
		Domain:          strings.TrimSpace(req.Domain),
		CursorType:      strings.TrimSpace(req.CursorType),
		CursorValue:     req.CursorValue,
	})
	writeAdminActionResult(w, log, actionErr, "failed to reset cursor")
}

func (h *AdminHandler) RerunMetrics(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req rerunMetricsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dfTrim := strings.TrimSpace(req.DateFrom)
	dtTrim := strings.TrimSpace(req.DateTo)
	var dateFrom, dateTo time.Time
	if dfTrim == "" && dtTrim == "" {
		// Service applies default window (last 30 calendar days, UTC).
	} else if dfTrim == "" || dtTrim == "" {
		writeJSONError(w, http.StatusBadRequest, "date_from and date_to must both be set or both omitted")
		return
	} else {
		var err error
		dateFrom, err = parseISODate(dfTrim)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid date_from, expected YYYY-MM-DD")
			return
		}
		dateTo, err = parseISODate(dtTrim)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid date_to, expected YYYY-MM-DD")
			return
		}
	}
	log, actionErr := h.service.RerunMetrics(r.Context(), actor, admin.RerunMetricsInput{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateFrom,
		DateTo:          dateTo,
	})
	writeAdminActionResult(w, log, actionErr, "failed to rerun metrics")
}

func (h *AdminHandler) RerunAlerts(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req rerunByDateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asOfDate, err := parseISODate(req.AsOfDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}
	log, actionErr := h.service.RerunAlerts(r.Context(), actor, admin.RerunAlertsInput{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOfDate,
	})
	writeAdminActionResult(w, log, actionErr, "failed to rerun alerts")
}

func (h *AdminHandler) RerunRecommendations(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req rerunByDateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asOfDate, err := parseISODate(req.AsOfDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}
	log, actionErr := h.service.RerunRecommendations(r.Context(), actor, admin.RerunRecommendationsInput{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOfDate,
	})
	writeAdminActionResult(w, log, actionErr, "failed to rerun recommendations")
}

func (h *AdminHandler) ListRecommendationRuns(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.GetAIRecommendationLogs(r.Context(), sellerAccountID, admin.RecommendationRunLogFilter{
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		RunType: strings.TrimSpace(r.URL.Query().Get("run_type")),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		if errors.Is(err, admin.ErrSellerAccountIDRequired) {
			writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to list recommendation runs")
		return
	}
	items := make([]adminRecommendationRunResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminRecommendationRunResponse(item))
	}
	writeJSON(w, http.StatusOK, adminRecommendationRunsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) GetRecommendationRunDetail(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		return
	}
	runID, err := parseSellerAccountIDParam(r, "run_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid run_id")
		return
	}
	detail, err := h.service.GetRecommendationRunDetail(r.Context(), actor, sellerAccountID, runID)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrSellerAccountIDRequired) || errors.Is(err, admin.ErrAdminDataUnavailable):
			writeJSONError(w, http.StatusBadRequest, "invalid run_id")
		case errors.Is(err, admin.ErrAdminActorRequired):
			writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		case errors.Is(err, admin.ErrAdminAuditLogWriteFailed):
			writeJSONError(w, http.StatusServiceUnavailable, "audit log failed; raw AI payload withheld")
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "recommendation run not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to get recommendation run detail")
		}
		return
	}
	writeJSON(w, http.StatusOK, toAdminRecommendationRunDetailResponse(*detail))
}

func (h *AdminHandler) GetRecommendationRawAI(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		return
	}
	recommendationID, err := parseSellerAccountIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid recommendation id")
		return
	}
	detail, err := h.service.GetRecommendationRawAI(r.Context(), actor, sellerAccountID, recommendationID)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrSellerAccountIDRequired) || errors.Is(err, admin.ErrAdminDataUnavailable):
			writeJSONError(w, http.StatusBadRequest, "invalid recommendation id")
		case errors.Is(err, admin.ErrAdminActorRequired):
			writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		case errors.Is(err, admin.ErrAdminAuditLogWriteFailed):
			writeJSONError(w, http.StatusServiceUnavailable, "audit log failed; raw AI payload withheld")
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "recommendation not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to get recommendation detail")
		}
		return
	}
	writeJSON(w, http.StatusOK, toAdminRecommendationRawAIResponse(*detail))
}

func (h *AdminHandler) ListChatTraces(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := admin.ChatTraceFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Intent: strings.TrimSpace(r.URL.Query().Get("intent")),
		Limit:  limit,
		Offset: offset,
	}
	if rawSessionID := strings.TrimSpace(r.URL.Query().Get("session_id")); rawSessionID != "" {
		sessionID, parseErr := strconv.ParseInt(rawSessionID, 10, 64)
		if parseErr != nil || sessionID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		filter.SessionID = &sessionID
	}
	result, err := h.service.GetAIChatLogs(r.Context(), sellerAccountID, filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat traces")
		return
	}
	items := make([]adminChatTraceItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminChatTraceItemResponse(item))
	}
	writeJSON(w, http.StatusOK, adminChatTracesResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) GetChatTraceDetail(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		return
	}
	traceID, err := parseSellerAccountIDParam(r, "trace_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid trace_id")
		return
	}
	detail, err := h.service.GetChatTraceDetail(r.Context(), actor, sellerAccountID, traceID)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrSellerAccountIDRequired) || errors.Is(err, admin.ErrAdminDataUnavailable):
			writeJSONError(w, http.StatusBadRequest, "invalid trace_id")
		case errors.Is(err, admin.ErrAdminActorRequired):
			writeJSONError(w, http.StatusUnauthorized, "admin actor required")
		case errors.Is(err, admin.ErrAdminAuditLogWriteFailed):
			writeJSONError(w, http.StatusServiceUnavailable, "audit log failed; raw AI payload withheld")
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "chat trace not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to get chat trace detail")
		}
		return
	}
	writeJSON(w, http.StatusOK, toAdminChatTraceDetailResponse(*detail))
}

func (h *AdminHandler) ListChatSessions(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListChatSessions(r.Context(), sellerAccountID, admin.ChatSessionFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat sessions")
		return
	}
	items := make([]adminChatSessionItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminChatSessionItemResponse(item))
	}
	writeJSON(w, http.StatusOK, adminChatSessionsResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) ListChatMessages(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(w, http.StatusInternalServerError, "admin service is not configured")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	sessionID, err := parseSellerAccountIDParam(r, "session_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListChatMessages(r.Context(), sellerAccountID, sessionID, admin.ChatMessageFilter{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrSellerAccountIDRequired) || errors.Is(err, admin.ErrAdminDataUnavailable):
			writeJSONError(w, http.StatusBadRequest, "invalid session_id")
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "chat session not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to list chat messages")
		}
		return
	}
	items := make([]adminChatMessageResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAdminChatMessageResponse(item))
	}
	writeJSON(w, http.StatusOK, adminChatMessagesResponse{Items: items, Limit: result.Limit, Offset: result.Offset})
}

func (h *AdminHandler) ListChatFeedbackByClient(w http.ResponseWriter, r *http.Request) {
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListChatFeedback(r.Context(), admin.ChatFeedbackFilter{
		SellerAccountID: &sellerAccountID,
		Rating:          strings.TrimSpace(r.URL.Query().Get("rating")),
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat feedback")
		return
	}
	writeJSON(w, http.StatusOK, toAdminChatFeedbackResponse(*result))
}

func (h *AdminHandler) ListAllChatFeedback(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := admin.ChatFeedbackFilter{
		Rating: strings.TrimSpace(r.URL.Query().Get("rating")),
		Limit:  limit,
		Offset: offset,
	}
	if rawSellerID := strings.TrimSpace(r.URL.Query().Get("seller_account_id")); rawSellerID != "" {
		sellerID, parseErr := strconv.ParseInt(rawSellerID, 10, 64)
		if parseErr != nil || sellerID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
			return
		}
		filter.SellerAccountID = &sellerID
	}
	result, err := h.service.ListChatFeedback(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat feedback")
		return
	}
	writeJSON(w, http.StatusOK, toAdminChatFeedbackResponse(*result))
}

func (h *AdminHandler) ListRecommendationFeedbackByClient(w http.ResponseWriter, r *http.Request) {
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.ListRecommendationFeedback(r.Context(), sellerAccountID, admin.RecommendationFeedbackFilter{
		Rating: strings.TrimSpace(r.URL.Query().Get("rating")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list recommendation feedback")
		return
	}
	writeJSON(w, http.StatusOK, toAdminRecommendationFeedbackResponse(*result))
}

func (h *AdminHandler) GetBillingStateByClient(w http.ResponseWriter, r *http.Request) {
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	state, err := h.service.GetBillingState(r.Context(), sellerAccountID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get billing state")
		return
	}
	if state == nil {
		writeJSONError(w, http.StatusNotFound, "billing state not found")
		return
	}
	writeJSON(w, http.StatusOK, toAdminBillingStateResponse(*state))
}

func (h *AdminHandler) UpdateBillingStateByClient(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActorFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sellerAccountID, err := parseSellerAccountIDParam(r, "seller_account_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	var req updateBillingStateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	trialEndsAt, err := parseOptionalRFC3339(req.TrialEndsAt)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid trial_ends_at, expected RFC3339")
		return
	}
	periodStart, err := parseOptionalRFC3339(req.CurrentPeriodStart)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid current_period_start, expected RFC3339")
		return
	}
	periodEnd, err := parseOptionalRFC3339(req.CurrentPeriodEnd)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid current_period_end, expected RFC3339")
		return
	}
	state, action, updateErr := h.service.UpdateBillingState(r.Context(), actor, admin.UpdateBillingStateInput{
		SellerAccountID:      sellerAccountID,
		PlanCode:             strings.TrimSpace(req.PlanCode),
		Status:               admin.BillingStatus(strings.TrimSpace(req.Status)),
		TrialEndsAt:          trialEndsAt,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
		AITokensLimitMonth:   req.AITokensLimitMonth,
		AITokensUsedMonth:    req.AITokensUsedMonth,
		EstimatedAICostMonth: req.EstimatedAICostMonth,
		Notes:                req.Notes,
	})
	if updateErr != nil {
		lowerErr := strings.ToLower(updateErr.Error())
		if errors.Is(updateErr, admin.ErrSellerAccountIDRequired) || errors.Is(updateErr, admin.ErrAdminActorRequired) || strings.Contains(lowerErr, "required") || strings.Contains(lowerErr, "invalid") || strings.Contains(lowerErr, "non-negative") {
			writeJSONError(w, http.StatusBadRequest, updateErr.Error())
			return
		}
		if action != nil {
			writeJSON(w, http.StatusInternalServerError, updateBillingStateResponse{
				Billing: nil,
				Action:  toAdminActionResponse(action),
			})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update billing state")
		return
	}
	writeJSON(w, http.StatusOK, updateBillingStateResponse{
		Billing: billingPtr(state),
		Action:  toAdminActionResponse(action),
	})
}

func (h *AdminHandler) ListBillingStates(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parseAdminPagination(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.service.ListBillingStates(r.Context(), admin.BillingStateFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list billing states")
		return
	}
	respItems := make([]adminBillingStateResponse, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toAdminBillingStateResponse(item))
	}
	writeJSON(w, http.StatusOK, adminBillingListResponse{
		Items:  respItems,
		Limit:  limit,
		Offset: offset,
	})
}

type adminClientsResponse struct {
	Items  []adminClientItemResponse `json:"items"`
	Limit  int32                     `json:"limit"`
	Offset int32                     `json:"offset"`
}

type adminClientItemResponse struct {
	SellerAccountID               int64   `json:"seller_account_id"`
	SellerName                    string  `json:"seller_name"`
	UserEmail                     string  `json:"user_email"`
	SellerStatus                  string  `json:"seller_status"`
	ConnectionStatus              *string `json:"connection_status"`
	LastCheckAt                   *string `json:"last_check_at"`
	LatestSyncStatus              *string `json:"latest_sync_status"`
	LatestSyncStartedAt           *string `json:"latest_sync_started_at"`
	LatestSyncFinishedAt          *string `json:"latest_sync_finished_at"`
	OpenAlertsCount               int64   `json:"open_alerts_count"`
	OpenRecommendationsCount      int64   `json:"open_recommendations_count"`
	LatestRecommendationRunStatus *string `json:"latest_recommendation_run_status"`
	LatestChatTraceStatus         *string `json:"latest_chat_trace_status"`
	BillingStatus                 *string `json:"billing_status"`
	CreatedAt                     string  `json:"created_at"`
	UpdatedAt                     string  `json:"updated_at"`
}

type adminClientDetailResponse struct {
	Overview          adminClientOverviewResponse    `json:"overview"`
	Connections       []adminConnectionResponse      `json:"connections"`
	OperationalStatus adminOperationalStatusResponse `json:"operational_status"`
	Billing           *adminBillingStateResponse     `json:"billing"`
}

type adminClientOverviewResponse struct {
	SellerAccountID int64   `json:"seller_account_id"`
	SellerName      string  `json:"seller_name"`
	SellerStatus    string  `json:"seller_status"`
	OwnerUserID     *int64  `json:"owner_user_id"`
	OwnerEmail      *string `json:"owner_email"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type adminConnectionResponse struct {
	Provider                    string  `json:"provider"`
	Status                      string  `json:"status"`
	LastCheckAt                 *string `json:"last_check_at"`
	LastCheckResult             *string `json:"last_check_result"`
	LastError                   *string `json:"last_error"`
	UpdatedAt                   *string `json:"updated_at"`
	PerformanceConnectionStatus string  `json:"performance_connection_status"`
	PerformanceTokenSet         bool    `json:"performance_token_set"`
	PerformanceLastCheckAt      *string `json:"performance_last_check_at"`
	PerformanceLastCheckResult  *string `json:"performance_last_check_result"`
	PerformanceLastError        *string `json:"performance_last_error"`
}

type adminOperationalStatusResponse struct {
	LatestSyncJob            *adminSyncJobSummaryResponse           `json:"latest_sync_job"`
	LatestImportJobs         []adminImportJobSummaryResponse        `json:"latest_import_jobs"`
	LatestAlertRun           *adminAlertRunSummaryResponse          `json:"latest_alert_run"`
	LatestRecommendationRun  *adminRecommendationRunSummaryResponse `json:"latest_recommendation_run"`
	LatestChatTrace          *adminChatTraceSummaryResponse         `json:"latest_chat_trace"`
	OpenAlertsCount          int64                                  `json:"open_alerts_count"`
	OpenRecommendationsCount int64                                  `json:"open_recommendations_count"`
	Limitations              []string                               `json:"limitations"`
}

type adminSyncJobSummaryResponse struct {
	ID           int64   `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	StartedAt    *string `json:"started_at"`
	FinishedAt   *string `json:"finished_at"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    string  `json:"created_at"`
}

type adminImportJobSummaryResponse struct {
	ID              int64   `json:"id"`
	SyncJobID       int64   `json:"sync_job_id"`
	Domain          string  `json:"domain"`
	Status          string  `json:"status"`
	SourceCursor    *string `json:"source_cursor"`
	RecordsReceived int32   `json:"records_received"`
	RecordsImported int32   `json:"records_imported"`
	RecordsFailed   int32   `json:"records_failed"`
	StartedAt       *string `json:"started_at"`
	FinishedAt      *string `json:"finished_at"`
	ErrorMessage    *string `json:"error_message"`
	CreatedAt       string  `json:"created_at"`
}

type adminAlertRunSummaryResponse struct {
	ID               int64   `json:"id"`
	RunType          string  `json:"run_type"`
	Status           string  `json:"status"`
	StartedAt        *string `json:"started_at"`
	FinishedAt       *string `json:"finished_at"`
	TotalAlertsCount int32   `json:"total_alerts_count"`
	ErrorMessage     *string `json:"error_message"`
}

type adminRecommendationRunSummaryResponse struct {
	ID                            int64   `json:"id"`
	RunType                       string  `json:"run_type"`
	Status                        string  `json:"status"`
	StartedAt                     *string `json:"started_at"`
	FinishedAt                    *string `json:"finished_at"`
	InputTokens                   int32   `json:"input_tokens"`
	OutputTokens                  int32   `json:"output_tokens"`
	GeneratedRecommendationsCount int32   `json:"generated_recommendations_count"`
	AcceptedRecommendationsCount  int32   `json:"accepted_recommendations_count"`
	ErrorMessage                  *string `json:"error_message"`
}

type adminChatTraceSummaryResponse struct {
	ID             int64   `json:"id"`
	SessionID      int64   `json:"session_id"`
	Status         string  `json:"status"`
	DetectedIntent *string `json:"detected_intent"`
	StartedAt      *string `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	ErrorMessage   *string `json:"error_message"`
}

type adminBillingStateResponse struct {
	SellerAccountID      int64   `json:"seller_account_id"`
	PlanCode             string  `json:"plan_code"`
	Status               string  `json:"status"`
	TrialEndsAt          *string `json:"trial_ends_at"`
	CurrentPeriodStart   *string `json:"current_period_start"`
	CurrentPeriodEnd     *string `json:"current_period_end"`
	AITokensLimitMonth   *int64  `json:"ai_tokens_limit_month"`
	AITokensUsedMonth    int64   `json:"ai_tokens_used_month"`
	EstimatedAICostMonth float64 `json:"estimated_ai_cost_month"`
	Notes                *string `json:"notes"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type adminBillingListResponse struct {
	Items  []adminBillingStateResponse `json:"items"`
	Limit  int32                       `json:"limit"`
	Offset int32                       `json:"offset"`
}

type updateBillingStateRequest struct {
	PlanCode             string  `json:"plan_code"`
	Status               string  `json:"status"`
	TrialEndsAt          *string `json:"trial_ends_at"`
	CurrentPeriodStart   *string `json:"current_period_start"`
	CurrentPeriodEnd     *string `json:"current_period_end"`
	AITokensLimitMonth   *int64  `json:"ai_tokens_limit_month"`
	AITokensUsedMonth    int64   `json:"ai_tokens_used_month"`
	EstimatedAICostMonth float64 `json:"estimated_ai_cost_month"`
	Notes                *string `json:"notes"`
}

type updateBillingStateResponse struct {
	Billing *adminBillingStateResponse `json:"billing"`
	Action  *adminActionResponse       `json:"action"`
}

type adminSyncJobsResponse struct {
	Items  []adminSyncJobSummaryResponse `json:"items"`
	Limit  int32                         `json:"limit"`
	Offset int32                         `json:"offset"`
}

type adminImportJobsResponse struct {
	Items  []adminImportJobSummaryResponse `json:"items"`
	Limit  int32                           `json:"limit"`
	Offset int32                           `json:"offset"`
}

type adminImportErrorResponse struct {
	ImportJobID   int64   `json:"import_job_id"`
	SyncJobID     int64   `json:"sync_job_id"`
	Domain        string  `json:"domain"`
	Status        string  `json:"status"`
	ErrorMessage  string  `json:"error_message"`
	RecordsFailed int32   `json:"records_failed"`
	StartedAt     *string `json:"started_at"`
	FinishedAt    *string `json:"finished_at"`
}

type adminImportErrorsResponse struct {
	Items  []adminImportErrorResponse `json:"items"`
	Limit  int32                      `json:"limit"`
	Offset int32                      `json:"offset"`
}

type adminSyncCursorResponse struct {
	Domain      string  `json:"domain"`
	CursorType  string  `json:"cursor_type"`
	CursorValue *string `json:"cursor_value"`
	UpdatedAt   string  `json:"updated_at"`
}

type adminSyncCursorsResponse struct {
	Items  []adminSyncCursorResponse `json:"items"`
	Limit  int32                     `json:"limit"`
	Offset int32                     `json:"offset"`
}

type adminRecommendationRunsResponse struct {
	Items  []adminRecommendationRunResponse `json:"items"`
	Limit  int32                            `json:"limit"`
	Offset int32                            `json:"offset"`
}

type adminRecommendationRunResponse struct {
	ID                            int64   `json:"id"`
	RunType                       string  `json:"run_type"`
	Status                        string  `json:"status"`
	AsOfDate                      *string `json:"as_of_date"`
	AIModel                       *string `json:"ai_model"`
	AIPromptVersion               *string `json:"ai_prompt_version"`
	InputTokens                   int32   `json:"input_tokens"`
	OutputTokens                  int32   `json:"output_tokens"`
	EstimatedCost                 float64 `json:"estimated_cost"`
	GeneratedRecommendationsCount int32   `json:"generated_recommendations_count"`
	AcceptedRecommendationsCount  int32   `json:"accepted_recommendations_count"`
	RejectedRecommendationsCount  *int32  `json:"rejected_recommendations_count"`
	ErrorMessage                  *string `json:"error_message"`
	StartedAt                     *string `json:"started_at"`
	FinishedAt                    *string `json:"finished_at"`
}

type adminRecommendationRunDetailResponse struct {
	Run             adminRecommendationRunResponse          `json:"run"`
	Recommendations []adminRecommendationItemResponse       `json:"recommendations"`
	Diagnostics     []adminRecommendationDiagnosticResponse `json:"diagnostics"`
	Limitations     []string                                `json:"limitations"`
}

type adminRecommendationItemResponse struct {
	ID                       int64          `json:"id"`
	RecommendationType       string         `json:"recommendation_type"`
	Title                    string         `json:"title"`
	Status                   string         `json:"status"`
	PriorityLevel            string         `json:"priority_level"`
	ConfidenceLevel          string         `json:"confidence_level"`
	Horizon                  string         `json:"horizon"`
	EntityType               string         `json:"entity_type"`
	EntityID                 *string        `json:"entity_id"`
	EntitySKU                *int64         `json:"entity_sku"`
	EntityOfferID            *string        `json:"entity_offer_id"`
	WhatHappened             *string        `json:"what_happened"`
	WhyItMatters             *string        `json:"why_it_matters"`
	RecommendedAction        *string        `json:"recommended_action"`
	ExpectedEffect           *string        `json:"expected_effect"`
	SupportingMetricsPayload map[string]any `json:"supporting_metrics_payload"`
	ConstraintsPayload       map[string]any `json:"constraints_payload"`
	RawAIResponse            map[string]any `json:"raw_ai_response"`
	CreatedAt                string         `json:"created_at"`
	UpdatedAt                string         `json:"updated_at"`
}

type adminRecommendationDiagnosticResponse struct {
	ID                      int64          `json:"id"`
	OpenAIRequestID         *string        `json:"openai_request_id"`
	AIModel                 *string        `json:"ai_model"`
	PromptVersion           *string        `json:"prompt_version"`
	ContextPayloadSummary   map[string]any `json:"context_payload_summary"`
	RawOpenAIResponse       map[string]any `json:"raw_openai_response"`
	ValidationResultPayload map[string]any `json:"validation_result_payload"`
	RejectedItemsPayload    map[string]any `json:"rejected_items_payload"`
	ErrorStage              *string        `json:"error_stage"`
	ErrorMessage            *string        `json:"error_message"`
	InputTokens             int64          `json:"input_tokens"`
	OutputTokens            int64          `json:"output_tokens"`
	EstimatedCost           float64        `json:"estimated_cost"`
	CreatedAt               string         `json:"created_at"`
}

type adminRecommendationRawAIResponse struct {
	Recommendation adminRecommendationItemResponse         `json:"recommendation"`
	RelatedAlerts  []adminRecommendationRelatedAlertBrief  `json:"related_alerts"`
	Diagnostics    []adminRecommendationDiagnosticResponse `json:"diagnostics"`
	Limitations    []string                                `json:"limitations"`
}

type adminRecommendationRelatedAlertBrief struct {
	ID         int64  `json:"id"`
	AlertType  string `json:"alert_type"`
	AlertGroup string `json:"alert_group"`
	Severity   string `json:"severity"`
	Urgency    string `json:"urgency"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type adminChatTracesResponse struct {
	Items  []adminChatTraceItemResponse `json:"items"`
	Limit  int32                        `json:"limit"`
	Offset int32                        `json:"offset"`
}

type adminChatTraceItemResponse struct {
	ID                   int64   `json:"id"`
	SessionID            int64   `json:"session_id"`
	UserMessageID        *int64  `json:"user_message_id"`
	AssistantMessageID   *int64  `json:"assistant_message_id"`
	DetectedIntent       *string `json:"detected_intent"`
	Status               string  `json:"status"`
	PlannerModel         string  `json:"planner_model"`
	AnswerModel          string  `json:"answer_model"`
	PlannerPromptVersion string  `json:"planner_prompt_version"`
	AnswerPromptVersion  string  `json:"answer_prompt_version"`
	InputTokens          int32   `json:"input_tokens"`
	OutputTokens         int32   `json:"output_tokens"`
	EstimatedCost        float64 `json:"estimated_cost"`
	ErrorMessage         *string `json:"error_message"`
	StartedAt            *string `json:"started_at"`
	FinishedAt           *string `json:"finished_at"`
	CreatedAt            *string `json:"created_at,omitempty"`
}

type adminChatTraceDetailResponse struct {
	Trace       adminChatTraceItemResponse     `json:"trace"`
	Messages    []adminChatMessageResponse     `json:"messages"`
	Payloads    adminChatTracePayloadsResponse `json:"payloads"`
	Limitations []string                       `json:"limitations"`
}

type adminChatTracePayloadsResponse struct {
	ToolPlanPayload          map[string]any `json:"tool_plan_payload"`
	ValidatedToolPlanPayload map[string]any `json:"validated_tool_plan_payload"`
	ToolResultsPayload       map[string]any `json:"tool_results_payload"`
	FactContextPayload       map[string]any `json:"fact_context_payload"`
	RawPlannerResponse       map[string]any `json:"raw_planner_response"`
	RawAnswerResponse        map[string]any `json:"raw_answer_response"`
	AnswerValidationPayload  map[string]any `json:"answer_validation_payload"`
}

type adminChatSessionsResponse struct {
	Items  []adminChatSessionItemResponse `json:"items"`
	Limit  int32                          `json:"limit"`
	Offset int32                          `json:"offset"`
}

type adminChatSessionItemResponse struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	LastMessageAt *string `json:"last_message_at"`
}

type adminChatMessagesResponse struct {
	Items  []adminChatMessageResponse `json:"items"`
	Limit  int32                      `json:"limit"`
	Offset int32                      `json:"offset"`
}

type adminChatMessageResponse struct {
	ID          int64  `json:"id"`
	SessionID   int64  `json:"session_id"`
	Role        string `json:"role"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
}

type adminChatFeedbackResponse struct {
	Items  []adminChatFeedbackItemResponse `json:"items"`
	Limit  int32                           `json:"limit"`
	Offset int32                           `json:"offset"`
}

type adminChatFeedbackItemResponse struct {
	ID              int64                    `json:"id"`
	SellerAccountID int64                    `json:"seller_account_id"`
	SellerName      *string                  `json:"seller_name,omitempty"`
	SessionID       int64                    `json:"session_id"`
	MessageID       int64                    `json:"message_id"`
	Rating          string                   `json:"rating"`
	Comment         *string                  `json:"comment"`
	CreatedAt       string                   `json:"created_at"`
	TraceID         *int64                   `json:"trace_id"`
	Message         adminChatFeedbackMessage `json:"message"`
	Session         adminChatFeedbackSession `json:"session"`
}

type adminChatFeedbackMessage struct {
	ID          int64   `json:"id"`
	Role        *string `json:"role"`
	MessageType *string `json:"message_type"`
	Content     *string `json:"content"`
}

type adminChatFeedbackSession struct {
	ID    int64   `json:"id"`
	Title *string `json:"title"`
}

type adminRecommendationFeedbackResponse struct {
	Items               []adminRecommendationFeedbackItemResponse `json:"items"`
	ProxyStatusFeedback admin.RecommendationFeedbackProxyStatus   `json:"proxy_status_feedback"`
	Limitations         []string                                  `json:"limitations"`
	Limit               int32                                     `json:"limit"`
	Offset              int32                                     `json:"offset"`
}

type adminRecommendationFeedbackItemResponse struct {
	ID               int64                              `json:"id"`
	SellerAccountID  int64                              `json:"seller_account_id"`
	RecommendationID int64                              `json:"recommendation_id"`
	Rating           string                             `json:"rating"`
	Comment          *string                            `json:"comment"`
	CreatedAt        string                             `json:"created_at"`
	Recommendation   adminRecommendationFeedbackRecInfo `json:"recommendation"`
}

type adminRecommendationFeedbackRecInfo struct {
	ID                 int64   `json:"id"`
	RecommendationType string  `json:"recommendation_type"`
	Title              string  `json:"title"`
	PriorityLevel      string  `json:"priority_level"`
	ConfidenceLevel    string  `json:"confidence_level"`
	Status             string  `json:"status"`
	EntityType         string  `json:"entity_type"`
	EntityID           *string `json:"entity_id"`
	EntitySKU          *int64  `json:"entity_sku"`
	EntityOfferID      *string `json:"entity_offer_id"`
	CreatedAt          string  `json:"created_at"`
}

type rerunSyncRequest struct {
	SyncType string `json:"sync_type"`
}

type resetCursorRequest struct {
	Domain      string  `json:"domain"`
	CursorType  string  `json:"cursor_type"`
	CursorValue *string `json:"cursor_value"`
}

type rerunMetricsRequest struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

type rerunByDateRequest struct {
	AsOfDate string `json:"as_of_date"`
}

type adminActionResponse struct {
	ID              int64          `json:"id"`
	AdminUserID     *int64         `json:"admin_user_id,omitempty"`
	AdminEmail      string         `json:"admin_email"`
	SellerAccountID int64          `json:"seller_account_id"`
	ActionType      string         `json:"action_type"`
	TargetType      *string        `json:"target_type,omitempty"`
	TargetID        *int64         `json:"target_id,omitempty"`
	RequestPayload  map[string]any `json:"request_payload"`
	ResultPayload   map[string]any `json:"result_payload"`
	Status          string         `json:"status"`
	ErrorMessage    *string        `json:"error_message,omitempty"`
	CreatedAt       string         `json:"created_at"`
	FinishedAt      *string        `json:"finished_at,omitempty"`
}

func parseAdminPagination(r *http.Request) (int32, int32, error) {
	limit := int32(50)
	offset := int32(0)

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			return 0, 0, errors.New("invalid limit")
		}
		limit = int32(parsed)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("invalid offset")
		}
		offset = int32(parsed)
	}
	return limit, offset, nil
}

func parseSellerAccountIDParam(r *http.Request, key string) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, key))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func toAdminClientItemResponse(item admin.ClientListItem) adminClientItemResponse {
	return adminClientItemResponse{
		SellerAccountID:               item.SellerAccountID,
		SellerName:                    item.SellerName,
		UserEmail:                     item.UserEmail,
		SellerStatus:                  item.SellerStatus,
		ConnectionStatus:              item.ConnectionStatus,
		LastCheckAt:                   timePtrRFC3339(item.LastConnectionCheckAt),
		LatestSyncStatus:              item.LatestSyncStatus,
		LatestSyncStartedAt:           timePtrRFC3339(item.LatestSyncStartedAt),
		LatestSyncFinishedAt:          timePtrRFC3339(item.LatestSyncFinishedAt),
		OpenAlertsCount:               item.OpenAlertsCount,
		OpenRecommendationsCount:      item.OpenRecommendationsCount,
		LatestRecommendationRunStatus: item.LatestRecommendationRunStatus,
		LatestChatTraceStatus:         item.LatestChatTraceStatus,
		BillingStatus:                 item.BillingStatus,
		CreatedAt:                     item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                     item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminClientDetailResponse(detail admin.ClientDetail) adminClientDetailResponse {
	resp := adminClientDetailResponse{
		Overview: adminClientOverviewResponse{
			SellerAccountID: detail.Overview.SellerAccountID,
			SellerName:      detail.Overview.SellerName,
			SellerStatus:    detail.Overview.SellerStatus,
			OwnerUserID:     detail.Overview.OwnerUserID,
			OwnerEmail:      detail.Overview.OwnerEmail,
			CreatedAt:       detail.Overview.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:       detail.Overview.UpdatedAt.UTC().Format(time.RFC3339),
		},
		Connections: []adminConnectionResponse{},
		OperationalStatus: adminOperationalStatusResponse{
			LatestImportJobs:         []adminImportJobSummaryResponse{},
			Limitations:              detail.OperationalStatus.Limitations,
			OpenAlertsCount:          detail.OperationalStatus.OpenAlertsCount,
			OpenRecommendationsCount: detail.OperationalStatus.OpenRecommendationsCount,
		},
	}
	for _, conn := range detail.Connections {
		resp.Connections = append(resp.Connections, adminConnectionResponse{
			Provider:                    conn.Provider,
			Status:                      conn.ConnectionStatus,
			LastCheckAt:                 timePtrRFC3339(conn.LastCheckAt),
			LastCheckResult:             conn.LastCheckResult,
			LastError:                   conn.LastConnectionErr,
			UpdatedAt:                   timePtrRFC3339(conn.UpdatedAt),
			PerformanceConnectionStatus: conn.PerformanceConnectionStatus,
			PerformanceTokenSet:         conn.PerformanceTokenSet,
			PerformanceLastCheckAt:      timePtrRFC3339(conn.PerformanceLastCheckAt),
			PerformanceLastCheckResult:  conn.PerformanceLastCheckResult,
			PerformanceLastError:        conn.PerformanceLastError,
		})
	}
	if v := detail.OperationalStatus.LatestSyncJob; v != nil {
		resp.OperationalStatus.LatestSyncJob = &adminSyncJobSummaryResponse{
			ID: v.ID, Type: v.Type, Status: v.Status, StartedAt: timePtrRFC3339(v.StartedAt),
			FinishedAt: timePtrRFC3339(v.FinishedAt), ErrorMessage: v.ErrorMessage,
			CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	for _, v := range detail.OperationalStatus.LatestImportJobs {
		resp.OperationalStatus.LatestImportJobs = append(resp.OperationalStatus.LatestImportJobs, adminImportJobSummaryResponse{
			ID: v.ID, SyncJobID: v.SyncJobID, Domain: v.Domain, Status: v.Status, SourceCursor: v.SourceCursor,
			RecordsReceived: v.RecordsReceived, RecordsImported: v.RecordsImported, RecordsFailed: v.RecordsFailed,
			StartedAt: timePtrRFC3339(v.StartedAt), FinishedAt: timePtrRFC3339(v.FinishedAt), ErrorMessage: v.ErrorMessage,
			CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	if v := detail.OperationalStatus.LatestAlertRun; v != nil {
		resp.OperationalStatus.LatestAlertRun = &adminAlertRunSummaryResponse{
			ID: v.ID, RunType: v.RunType, Status: v.Status, StartedAt: timePtrRFC3339(v.StartedAt),
			FinishedAt: timePtrRFC3339(v.FinishedAt), TotalAlertsCount: v.TotalAlertsCount, ErrorMessage: v.ErrorMessage,
		}
	}
	if v := detail.OperationalStatus.LatestRecommendationRun; v != nil {
		resp.OperationalStatus.LatestRecommendationRun = &adminRecommendationRunSummaryResponse{
			ID: v.ID, RunType: v.RunType, Status: v.Status, StartedAt: timePtrRFC3339(v.StartedAt),
			FinishedAt: timePtrRFC3339(v.FinishedAt), InputTokens: v.InputTokens, OutputTokens: v.OutputTokens,
			GeneratedRecommendationsCount: v.GeneratedRecommendationsCount, AcceptedRecommendationsCount: v.AcceptedRecommendationsCount,
			ErrorMessage: v.ErrorMessage,
		}
	}
	if v := detail.OperationalStatus.LatestChatTrace; v != nil {
		resp.OperationalStatus.LatestChatTrace = &adminChatTraceSummaryResponse{
			ID: v.ID, SessionID: v.SessionID, Status: v.Status, DetectedIntent: v.DetectedIntent,
			StartedAt: timePtrRFC3339(v.StartedAt), FinishedAt: timePtrRFC3339(v.FinishedAt), ErrorMessage: v.ErrorMessage,
		}
	}
	if detail.Billing != nil {
		resp.Billing = &adminBillingStateResponse{
			SellerAccountID:      detail.Billing.SellerAccountID,
			PlanCode:             detail.Billing.PlanCode,
			Status:               string(detail.Billing.Status),
			TrialEndsAt:          timePtrRFC3339(detail.Billing.TrialEndsAt),
			CurrentPeriodStart:   timePtrRFC3339(detail.Billing.CurrentPeriodStart),
			CurrentPeriodEnd:     timePtrRFC3339(detail.Billing.CurrentPeriodEnd),
			AITokensLimitMonth:   detail.Billing.AITokensLimitMonth,
			AITokensUsedMonth:    detail.Billing.AITokensUsedMonth,
			EstimatedAICostMonth: detail.Billing.EstimatedAICostMonth,
			Notes:                detail.Billing.Notes,
			CreatedAt:            detail.Billing.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:            detail.Billing.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	return resp
}

func toAdminSyncJobResponse(item admin.SyncJobSummary) adminSyncJobSummaryResponse {
	return adminSyncJobSummaryResponse{
		ID:           item.ID,
		Type:         item.Type,
		Status:       item.Status,
		StartedAt:    timePtrRFC3339(item.StartedAt),
		FinishedAt:   timePtrRFC3339(item.FinishedAt),
		ErrorMessage: item.ErrorMessage,
		CreatedAt:    item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminImportJobResponse(item admin.ImportJobSummary) adminImportJobSummaryResponse {
	return adminImportJobSummaryResponse{
		ID:              item.ID,
		SyncJobID:       item.SyncJobID,
		Domain:          item.Domain,
		Status:          item.Status,
		SourceCursor:    item.SourceCursor,
		RecordsReceived: item.RecordsReceived,
		RecordsImported: item.RecordsImported,
		RecordsFailed:   item.RecordsFailed,
		StartedAt:       timePtrRFC3339(item.StartedAt),
		FinishedAt:      timePtrRFC3339(item.FinishedAt),
		ErrorMessage:    item.ErrorMessage,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminImportErrorResponse(item admin.ImportErrorItem) adminImportErrorResponse {
	return adminImportErrorResponse{
		ImportJobID:   item.ImportJobID,
		SyncJobID:     item.SyncJobID,
		Domain:        item.Domain,
		Status:        item.Status,
		ErrorMessage:  item.ErrorMessage,
		RecordsFailed: item.RecordsFailed,
		StartedAt:     timePtrRFC3339(item.StartedAt),
		FinishedAt:    timePtrRFC3339(item.FinishedAt),
	}
}

func toAdminSyncCursorResponse(item admin.SyncCursorItem) adminSyncCursorResponse {
	return adminSyncCursorResponse{
		Domain:      item.Domain,
		CursorType:  item.CursorType,
		CursorValue: item.CursorValue,
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminRecommendationRunResponse(item admin.RecommendationRunLogItem) adminRecommendationRunResponse {
	return adminRecommendationRunResponse{
		ID:                            item.ID,
		RunType:                       item.RunType,
		Status:                        item.Status,
		AsOfDate:                      datePtrISO(item.AsOfDate),
		AIModel:                       item.AIModel,
		AIPromptVersion:               item.AIPromptVersion,
		InputTokens:                   item.InputTokens,
		OutputTokens:                  item.OutputTokens,
		EstimatedCost:                 item.EstimatedCost,
		GeneratedRecommendationsCount: item.GeneratedRecommendationsCount,
		AcceptedRecommendationsCount:  item.AcceptedRecommendationsCount,
		RejectedRecommendationsCount:  item.RejectedRecommendationsCount,
		ErrorMessage:                  item.ErrorMessage,
		StartedAt:                     timePtrRFC3339(item.StartedAt),
		FinishedAt:                    timePtrRFC3339(item.FinishedAt),
	}
}

func toAdminRecommendationRunDetailResponse(detail admin.RecommendationRunDetail) adminRecommendationRunDetailResponse {
	recommendations := make([]adminRecommendationItemResponse, 0, len(detail.Recommendations))
	for _, rec := range detail.Recommendations {
		recommendations = append(recommendations, toAdminRecommendationItemResponse(rec))
	}
	diagnostics := make([]adminRecommendationDiagnosticResponse, 0, len(detail.Diagnostics))
	for _, d := range detail.Diagnostics {
		diagnostics = append(diagnostics, toAdminRecommendationDiagnosticResponse(d))
	}
	return adminRecommendationRunDetailResponse{
		Run:             toAdminRecommendationRunResponse(detail.Run),
		Recommendations: recommendations,
		Diagnostics:     diagnostics,
		Limitations:     detail.Limitations,
	}
}

func toAdminRecommendationRawAIResponse(detail admin.RecommendationRawAI) adminRecommendationRawAIResponse {
	alerts := make([]adminRecommendationRelatedAlertBrief, 0, len(detail.RelatedAlerts))
	for _, alert := range detail.RelatedAlerts {
		alerts = append(alerts, adminRecommendationRelatedAlertBrief{
			ID:         alert.ID,
			AlertType:  alert.AlertType,
			AlertGroup: alert.AlertGroup,
			Severity:   alert.Severity,
			Urgency:    alert.Urgency,
			Title:      alert.Title,
			Status:     alert.Status,
		})
	}
	diagnostics := make([]adminRecommendationDiagnosticResponse, 0, len(detail.Diagnostics))
	for _, d := range detail.Diagnostics {
		diagnostics = append(diagnostics, toAdminRecommendationDiagnosticResponse(d))
	}
	return adminRecommendationRawAIResponse{
		Recommendation: toAdminRecommendationItemResponse(detail.Recommendation),
		RelatedAlerts:  alerts,
		Diagnostics:    diagnostics,
		Limitations:    detail.Limitations,
	}
}

func toAdminRecommendationItemResponse(item admin.RecommendationItem) adminRecommendationItemResponse {
	return adminRecommendationItemResponse{
		ID:                       item.ID,
		RecommendationType:       item.RecommendationType,
		Title:                    item.Title,
		Status:                   item.Status,
		PriorityLevel:            item.PriorityLevel,
		ConfidenceLevel:          item.ConfidenceLevel,
		Horizon:                  item.Horizon,
		EntityType:               item.EntityType,
		EntityID:                 item.EntityID,
		EntitySKU:                item.EntitySKU,
		EntityOfferID:            item.EntityOfferID,
		WhatHappened:             item.WhatHappened,
		WhyItMatters:             item.WhyItMatters,
		RecommendedAction:        item.RecommendedAction,
		ExpectedEffect:           item.ExpectedEffect,
		SupportingMetricsPayload: item.SupportingMetricsPayload,
		ConstraintsPayload:       item.ConstraintsPayload,
		RawAIResponse:            item.RawAIResponse,
		CreatedAt:                item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminRecommendationDiagnosticResponse(item admin.RecommendationRunDiagnosticItem) adminRecommendationDiagnosticResponse {
	return adminRecommendationDiagnosticResponse{
		ID:                      item.ID,
		OpenAIRequestID:         item.OpenAIRequestID,
		AIModel:                 item.AIModel,
		PromptVersion:           item.PromptVersion,
		ContextPayloadSummary:   item.ContextPayloadSummary,
		RawOpenAIResponse:       item.RawOpenAIResponse,
		ValidationResultPayload: item.ValidationResultPayload,
		RejectedItemsPayload:    item.RejectedItemsPayload,
		ErrorStage:              item.ErrorStage,
		ErrorMessage:            item.ErrorMessage,
		InputTokens:             item.InputTokens,
		OutputTokens:            item.OutputTokens,
		EstimatedCost:           item.EstimatedCost,
		CreatedAt:               item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminChatTraceItemResponse(item admin.ChatTraceLogItem) adminChatTraceItemResponse {
	return adminChatTraceItemResponse{
		ID:                   item.ID,
		SessionID:            item.SessionID,
		UserMessageID:        item.UserMessageID,
		AssistantMessageID:   item.AssistantMessageID,
		DetectedIntent:       item.DetectedIntent,
		Status:               item.Status,
		PlannerModel:         item.PlannerModel,
		AnswerModel:          item.AnswerModel,
		PlannerPromptVersion: item.PlannerPromptVersion,
		AnswerPromptVersion:  item.AnswerPromptVersion,
		InputTokens:          item.InputTokens,
		OutputTokens:         item.OutputTokens,
		EstimatedCost:        item.EstimatedCost,
		ErrorMessage:         item.ErrorMessage,
		StartedAt:            timePtrRFC3339(item.StartedAt),
		FinishedAt:           timePtrRFC3339(item.FinishedAt),
	}
}

func toAdminChatTraceDetailResponse(detail admin.ChatTraceDetail) adminChatTraceDetailResponse {
	createdAt := detail.CreatedAt.UTC().Format(time.RFC3339)
	trace := toAdminChatTraceItemResponse(admin.ChatTraceLogItem{
		ID:                   detail.ID,
		SessionID:            detail.SessionID,
		UserMessageID:        detail.UserMessageID,
		AssistantMessageID:   detail.AssistantMessageID,
		DetectedIntent:       detail.DetectedIntent,
		Status:               detail.Status,
		PlannerModel:         detail.PlannerModel,
		AnswerModel:          detail.AnswerModel,
		PlannerPromptVersion: detail.PlannerPromptVersion,
		AnswerPromptVersion:  detail.AnswerPromptVersion,
		InputTokens:          detail.InputTokens,
		OutputTokens:         detail.OutputTokens,
		EstimatedCost:        detail.EstimatedCost,
		ErrorMessage:         detail.ErrorMessage,
		StartedAt:            detail.StartedAt,
		FinishedAt:           detail.FinishedAt,
	})
	trace.CreatedAt = &createdAt
	messages := make([]adminChatMessageResponse, 0, len(detail.Messages))
	for _, message := range detail.Messages {
		messages = append(messages, toAdminChatMessageResponse(message))
	}
	return adminChatTraceDetailResponse{
		Trace:    trace,
		Messages: messages,
		Payloads: adminChatTracePayloadsResponse{
			ToolPlanPayload:          detail.ToolPlanPayload,
			ValidatedToolPlanPayload: detail.ValidatedToolPlanPayload,
			ToolResultsPayload:       detail.ToolResultsPayload,
			FactContextPayload:       detail.FactContextPayload,
			RawPlannerResponse:       detail.RawPlannerResponse,
			RawAnswerResponse:        detail.RawAnswerResponse,
			AnswerValidationPayload:  detail.AnswerValidationPayload,
		},
		Limitations: detail.Limitations,
	}
}

func toAdminChatSessionItemResponse(item admin.ChatSessionItem) adminChatSessionItemResponse {
	return adminChatSessionItemResponse{
		ID:            item.ID,
		Title:         item.Title,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     item.UpdatedAt.UTC().Format(time.RFC3339),
		LastMessageAt: timePtrRFC3339(item.LastMessageAt),
	}
}

func toAdminChatMessageResponse(item admin.ChatMessageItem) adminChatMessageResponse {
	return adminChatMessageResponse{
		ID:          item.ID,
		SessionID:   item.SessionID,
		Role:        item.Role,
		MessageType: item.MessageType,
		Content:     item.Content,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminActionResponse(item *admin.AdminActionLog) *adminActionResponse {
	if item == nil {
		return nil
	}
	return &adminActionResponse{
		ID:              item.ID,
		AdminUserID:     item.AdminUserID,
		AdminEmail:      item.AdminEmail,
		SellerAccountID: item.SellerAccountID,
		ActionType:      string(item.ActionType),
		TargetType:      item.TargetType,
		TargetID:        item.TargetID,
		RequestPayload:  item.RequestPayload,
		ResultPayload:   item.ResultPayload,
		Status:          string(item.Status),
		ErrorMessage:    item.ErrorMessage,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		FinishedAt:      timePtrRFC3339(item.FinishedAt),
	}
}

func toAdminChatFeedbackResponse(result admin.ChatFeedbackListResult) adminChatFeedbackResponse {
	items := make([]adminChatFeedbackItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, adminChatFeedbackItemResponse{
			ID:              item.ID,
			SellerAccountID: item.SellerAccountID,
			SellerName:      item.SellerName,
			SessionID:       item.SessionID,
			MessageID:       item.MessageID,
			Rating:          item.Rating,
			Comment:         item.Comment,
			CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
			TraceID:         item.TraceID,
			Message: adminChatFeedbackMessage{
				ID:          item.MessageID,
				Role:        item.MessageRole,
				MessageType: item.MessageType,
				Content:     item.MessageContent,
			},
			Session: adminChatFeedbackSession{
				ID:    item.SessionID,
				Title: item.SessionTitle,
			},
		})
	}
	return adminChatFeedbackResponse{Items: items, Limit: result.Limit, Offset: result.Offset}
}

func toAdminRecommendationFeedbackResponse(result admin.RecommendationFeedbackListResult) adminRecommendationFeedbackResponse {
	items := make([]adminRecommendationFeedbackItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, adminRecommendationFeedbackItemResponse{
			ID:               item.ID,
			SellerAccountID:  item.SellerAccountID,
			RecommendationID: item.RecommendationID,
			Rating:           item.Rating,
			Comment:          item.Comment,
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
			Recommendation: adminRecommendationFeedbackRecInfo{
				ID:                 item.RecommendationID,
				RecommendationType: item.RecommendationType,
				Title:              item.Title,
				PriorityLevel:      item.PriorityLevel,
				ConfidenceLevel:    item.ConfidenceLevel,
				Status:             item.RecommendationStatus,
				EntityType:         item.EntityType,
				EntityID:           item.EntityID,
				EntitySKU:          item.EntitySKU,
				EntityOfferID:      item.EntityOfferID,
				CreatedAt:          item.RecommendationCreatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	return adminRecommendationFeedbackResponse{
		Items:               items,
		ProxyStatusFeedback: result.ProxyStatusFeedback,
		Limitations:         result.Limitations,
		Limit:               result.Limit,
		Offset:              result.Offset,
	}
}

func toAdminBillingStateResponse(item admin.BillingState) adminBillingStateResponse {
	return adminBillingStateResponse{
		SellerAccountID:      item.SellerAccountID,
		PlanCode:             item.PlanCode,
		Status:               string(item.Status),
		TrialEndsAt:          timePtrRFC3339(item.TrialEndsAt),
		CurrentPeriodStart:   timePtrRFC3339(item.CurrentPeriodStart),
		CurrentPeriodEnd:     timePtrRFC3339(item.CurrentPeriodEnd),
		AITokensLimitMonth:   item.AITokensLimitMonth,
		AITokensUsedMonth:    item.AITokensUsedMonth,
		EstimatedAICostMonth: item.EstimatedAICostMonth,
		Notes:                item.Notes,
		CreatedAt:            item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func adminActorFromRequest(r *http.Request) (admin.AdminActor, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		return admin.AdminActor{}, false
	}
	userID := user.ID
	return admin.AdminActor{
		UserID: &userID,
		Email:  user.Email,
	}, true
}

func parseISODate(raw string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(raw))
}

func parseOptionalRFC3339(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func datePtrISO(v *time.Time) *string {
	if v == nil {
		return nil
	}
	s := v.UTC().Format("2006-01-02")
	return &s
}

func decodeJSONBody(r *http.Request, out any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	return json.NewDecoder(r.Body).Decode(out)
}

func writeAdminActionResult(w http.ResponseWriter, actionLog *admin.AdminActionLog, actionErr error, fallbackMessage string) {
	if actionErr == nil {
		writeJSON(w, http.StatusOK, toAdminActionResponse(actionLog))
		return
	}

	statusCode := http.StatusInternalServerError
	if errors.Is(actionErr, admin.ErrAdminActionNotConfigured) {
		statusCode = http.StatusServiceUnavailable
	}
	lowerErr := strings.ToLower(actionErr.Error())
	if errors.Is(actionErr, admin.ErrSellerAccountIDRequired) || strings.Contains(lowerErr, "required") || strings.Contains(lowerErr, "invalid") || strings.Contains(lowerErr, "unsupported") {
		statusCode = http.StatusBadRequest
	}

	if actionLog != nil {
		writeJSON(w, statusCode, toAdminActionResponse(actionLog))
		return
	}
	writeJSONError(w, statusCode, fallbackMessage)
}

func billingPtr(state *admin.BillingState) *adminBillingStateResponse {
	if state == nil {
		return nil
	}
	resp := toAdminBillingStateResponse(*state)
	return &resp
}
