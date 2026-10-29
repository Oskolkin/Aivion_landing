package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type AlertsHandler struct {
	service *alerts.Service
}

func NewAlertsHandler(service *alerts.Service) *AlertsHandler {
	return &AlertsHandler{service: service}
}

type alertResponse struct {
	ID              int64                  `json:"id"`
	AlertType       string                 `json:"alert_type"`
	AlertGroup      string                 `json:"alert_group"`
	EntityType      string                 `json:"entity_type"`
	EntityID        *string                `json:"entity_id"`
	EntitySKU       *int64                 `json:"entity_sku"`
	EntityOfferID   *string                `json:"entity_offer_id"`
	Title           string                 `json:"title"`
	Message         string                 `json:"message"`
	Severity        string                 `json:"severity"`
	Urgency         string                 `json:"urgency"`
	Status          string                 `json:"status"`
	EvidencePayload alerts.EvidencePayload `json:"evidence_payload"`
	FirstSeenAt     string                 `json:"first_seen_at"`
	LastSeenAt      string                 `json:"last_seen_at"`
	ResolvedAt      *string                `json:"resolved_at"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
}

type alertsSummaryResponse struct {
	OpenTotal int64 `json:"open_total"`
	Critical  int64 `json:"critical_count"`
	High      int64 `json:"high_count"`
	Medium    int64 `json:"medium_count"`
	Low       int64 `json:"low_count"`
	ByGroup   struct {
		Sales          int64 `json:"sales"`
		Stock          int64 `json:"stock"`
		Advertising    int64 `json:"advertising"`
		PriceEconomics int64 `json:"price_economics"`
	} `json:"by_group"`
	LatestRun *latestRunResponse `json:"latest_run"`
}

type latestRunResponse struct {
	ID               int64   `json:"id"`
	RunType          string  `json:"run_type"`
	Status           string  `json:"status"`
	StartedAt        string  `json:"started_at"`
	FinishedAt       *string `json:"finished_at"`
	SalesAlertsCount int32   `json:"sales_alerts_count"`
	StockAlertsCount int32   `json:"stock_alerts_count"`
	AdAlertsCount    int32   `json:"ad_alerts_count"`
	PriceAlertsCount int32   `json:"price_alerts_count"`
	TotalAlertsCount int32   `json:"total_alerts_count"`
	ErrorMessage     *string `json:"error_message"`
}

type runAlertsRequest struct {
	AsOfDate *string `json:"as_of_date"`
	RunType  *string `json:"run_type"`
}

type runAlertsResponse struct {
	SellerAccountID      int64                                 `json:"seller_account_id"`
	AsOfDate             string                                `json:"as_of_date"`
	RunID                int64                                 `json:"run_id"`
	Status               string                                `json:"status"`
	Sales                alerts.RunSalesAlertsSummary          `json:"sales"`
	Stock                alerts.RunStockAlertsSummary          `json:"stock"`
	Advertising          alerts.RunAdvertisingAlertsSummary    `json:"advertising"`
	PriceEconomics       alerts.RunPriceEconomicsAlertsSummary `json:"price_economics"`
	TotalGeneratedAlerts int                                   `json:"total_generated_alerts"`
	TotalUpsertedAlerts  int                                   `json:"total_upserted_alerts"`
	TotalSkippedRules    int                                   `json:"total_skipped_rules"`
	StartedAt            string                                `json:"started_at"`
	FinishedAt           string                                `json:"finished_at"`
}

func (h *AlertsHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	filter, err := parseAlertsFilter(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.service.ListAlerts(r.Context(), sellerAccount.ID, filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list alerts")
		return
	}
	resp := make([]alertResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, mapAlertResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  resp,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *AlertsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	summary, err := h.service.GetSummary(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get alerts summary")
		return
	}
	resp := alertsSummaryResponse{
		OpenTotal: summary.OpenTotal,
	}
	for _, row := range summary.BySeverity {
		switch row.Severity {
		case alerts.SeverityCritical:
			resp.Critical = row.Count
		case alerts.SeverityHigh:
			resp.High = row.Count
		case alerts.SeverityMedium:
			resp.Medium = row.Count
		case alerts.SeverityLow:
			resp.Low = row.Count
		}
	}
	for _, row := range summary.ByGroup {
		switch row.Group {
		case alerts.AlertGroupSales:
			resp.ByGroup.Sales = row.Count
		case alerts.AlertGroupStock:
			resp.ByGroup.Stock = row.Count
		case alerts.AlertGroupAdvertising:
			resp.ByGroup.Advertising = row.Count
		case alerts.AlertGroupPriceEconomics:
			resp.ByGroup.PriceEconomics = row.Count
		}
	}
	if summary.LatestRun != nil {
		resp.LatestRun = mapLatestRun(*summary.LatestRun)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AlertsHandler) RunAlerts(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req runAlertsRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	asOf := time.Now().UTC()
	if req.AsOfDate != nil && strings.TrimSpace(*req.AsOfDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.AsOfDate))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
			return
		}
		asOf = parsed
	}

	runType := alerts.RunTypeManual
	if req.RunType != nil && strings.TrimSpace(*req.RunType) != "" {
		switch alerts.RunType(strings.TrimSpace(*req.RunType)) {
		case alerts.RunTypeManual, alerts.RunTypeScheduled, alerts.RunTypePostSync, alerts.RunTypeBackfill:
			runType = alerts.RunType(strings.TrimSpace(*req.RunType))
		default:
			writeJSONError(w, http.StatusBadRequest, "invalid run_type")
			return
		}
	}

	result, err := h.service.RunForAccountWithType(r.Context(), sellerAccount.ID, asOf, runType)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to run alerts engine")
		return
	}
	writeJSON(w, http.StatusOK, runAlertsResponse{
		SellerAccountID:      result.SellerAccountID,
		AsOfDate:             result.AsOfDate.UTC().Format("2006-01-02"),
		RunID:                result.RunID,
		Status:               string(result.Status),
		Sales:                result.Sales,
		Stock:                result.Stock,
		Advertising:          result.Advertising,
		PriceEconomics:       result.PriceEconomics,
		TotalGeneratedAlerts: result.TotalGeneratedAlerts,
		TotalUpsertedAlerts:  result.TotalUpsertedAlerts,
		TotalSkippedRules:    result.TotalSkippedRules,
		StartedAt:            result.StartedAt.UTC().Format(time.RFC3339),
		FinishedAt:           result.FinishedAt.UTC().Format(time.RFC3339),
	})
}

func (h *AlertsHandler) DismissAlert(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, true)
}

func (h *AlertsHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, false)
}

func (h *AlertsHandler) handleAction(w http.ResponseWriter, r *http.Request, dismiss bool) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	idRaw := strings.TrimSpace(chi.URLParam(r, "id"))
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid alert id")
		return
	}
	var updated alerts.Alert
	if dismiss {
		updated, err = h.service.DismissAlert(r.Context(), sellerAccount.ID, id)
	} else {
		updated, err = h.service.ResolveAlert(r.Context(), sellerAccount.ID, id)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "alert not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update alert")
		return
	}
	writeJSON(w, http.StatusOK, mapAlertResponse(updated))
}

func parseAlertsFilter(r *http.Request) (alerts.ListFilter, error) {
	filter := alerts.ListFilter{
		Limit:  50,
		Offset: 0,
	}
	q := r.URL.Query()
	if raw := strings.TrimSpace(q.Get("status")); raw != "" {
		status := alerts.AlertStatus(raw)
		switch status {
		case alerts.AlertStatusOpen, alerts.AlertStatusResolved, alerts.AlertStatusDismissed:
			filter.Status = &status
		default:
			return alerts.ListFilter{}, errors.New("invalid status")
		}
	}
	if raw := strings.TrimSpace(q.Get("group")); raw != "" {
		group := alerts.AlertGroup(raw)
		switch group {
		case alerts.AlertGroupSales, alerts.AlertGroupStock, alerts.AlertGroupAdvertising, alerts.AlertGroupPriceEconomics:
			filter.Group = &group
		default:
			return alerts.ListFilter{}, errors.New("invalid group")
		}
	}
	if raw := strings.TrimSpace(q.Get("severity")); raw != "" {
		sev := alerts.Severity(raw)
		switch sev {
		case alerts.SeverityLow, alerts.SeverityMedium, alerts.SeverityHigh, alerts.SeverityCritical:
			filter.Severity = &sev
		default:
			return alerts.ListFilter{}, errors.New("invalid severity")
		}
	}
	if raw := strings.TrimSpace(q.Get("entity_type")); raw != "" {
		entityType := alerts.EntityType(raw)
		switch entityType {
		case alerts.EntityTypeAccount, alerts.EntityTypeSKU, alerts.EntityTypeProduct, alerts.EntityTypeCampaign, alerts.EntityTypePricingConstraint:
			filter.EntityType = &entityType
		default:
			return alerts.ListFilter{}, errors.New("invalid entity_type")
		}
	}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return alerts.ListFilter{}, errors.New("invalid limit")
		}
		if v > 200 {
			v = 200
		}
		filter.Limit = v
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return alerts.ListFilter{}, errors.New("invalid offset")
		}
		filter.Offset = v
	}
	return filter, nil
}

func mapAlertResponse(a alerts.Alert) alertResponse {
	return alertResponse{
		ID:              a.ID,
		AlertType:       string(a.AlertType),
		AlertGroup:      string(a.AlertGroup),
		EntityType:      string(a.EntityType),
		EntityID:        a.EntityID,
		EntitySKU:       a.EntitySKU,
		EntityOfferID:   a.EntityOfferID,
		Title:           a.Title,
		Message:         a.Message,
		Severity:        string(a.Severity),
		Urgency:         string(a.Urgency),
		Status:          string(a.Status),
		EvidencePayload: a.EvidencePayload,
		FirstSeenAt:     a.FirstSeenAt.UTC().Format(time.RFC3339),
		LastSeenAt:      a.LastSeenAt.UTC().Format(time.RFC3339),
		ResolvedAt:      timePtrRFC3339(a.ResolvedAt),
		CreatedAt:       a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func mapLatestRun(run alerts.AlertRun) *latestRunResponse {
	resp := &latestRunResponse{
		ID:               run.ID,
		RunType:          string(run.RunType),
		Status:           string(run.Status),
		StartedAt:        run.StartedAt.UTC().Format(time.RFC3339),
		SalesAlertsCount: run.SalesAlertsCount,
		StockAlertsCount: run.StockAlertsCount,
		AdAlertsCount:    run.AdAlertsCount,
		PriceAlertsCount: run.PriceAlertsCount,
		TotalAlertsCount: run.TotalAlertsCount,
		ErrorMessage:     run.ErrorMessage,
	}
	if run.FinishedAt != nil {
		v := run.FinishedAt.UTC().Format(time.RFC3339)
		resp.FinishedAt = &v
	}
	return resp
}
