package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
)

type OzonEnrichmentHandler struct {
	service *ozonenrichmentsync.Service
}

func NewOzonEnrichmentHandler(service *ozonenrichmentsync.Service) *OzonEnrichmentHandler {
	return &OzonEnrichmentHandler{service: service}
}

type enrichmentSyncRequest struct {
	SellerID   int64  `json:"seller_id"`
	MetricDate string `json:"metric_date"`
	Scope      string `json:"scope"`
	Force      bool   `json:"force"`
}

func (h *OzonEnrichmentHandler) Sync(w http.ResponseWriter, r *http.Request) {
	var req enrichmentSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if req.SellerID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "seller_id is required"})
		return
	}
	metricDate, err := parseMetricDate(req.MetricDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.service.SyncOzonEnrichmentP0(r.Context(), ozonenrichmentsync.SyncInput{
		SellerAccountID: req.SellerID,
		MetricDate:      metricDate,
		Scope:           req.Scope,
		Force:           req.Force,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *OzonEnrichmentHandler) Status(w http.ResponseWriter, r *http.Request) {
	sellerID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("seller_id")), 10, 64)
	if err != nil || sellerID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "seller_id is required"})
		return
	}
	metricDate, err := parseMetricDate(r.URL.Query().Get("metric_date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	scope := r.URL.Query().Get("scope")

	status, err := h.service.GetEnrichmentStatus(r.Context(), sellerID, metricDate, scope)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func parseMetricDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
}
