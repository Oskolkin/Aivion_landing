package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type RecommendationsHandler struct {
	service *recommendations.Service
}

func NewRecommendationsHandler(service *recommendations.Service) *RecommendationsHandler {
	return &RecommendationsHandler{service: service}
}

type recommendationResponse struct {
	ID                       int64                  `json:"id"`
	Source                   string                 `json:"source"`
	RecommendationType       string                 `json:"recommendation_type"`
	Horizon                  string                 `json:"horizon"`
	EntityType               string                 `json:"entity_type"`
	EntityID                 *string                `json:"entity_id"`
	EntitySKU                *int64                 `json:"entity_sku"`
	EntityOfferID            *string                `json:"entity_offer_id"`
	Title                    string                 `json:"title"`
	WhatHappened             string                 `json:"what_happened"`
	WhyItMatters             string                 `json:"why_it_matters"`
	RecommendedAction        string                 `json:"recommended_action"`
	ExpectedEffect           *string                `json:"expected_effect"`
	PriorityScore            float64                `json:"priority_score"`
	PriorityLevel            string                 `json:"priority_level"`
	Urgency                  string                 `json:"urgency"`
	ConfidenceLevel          string                 `json:"confidence_level"`
	Status                   string                 `json:"status"`
	SupportingMetricsPayload map[string]any         `json:"supporting_metrics_payload"`
	ConstraintsPayload       map[string]any         `json:"constraints_payload"`
	AIModel                  *string                `json:"ai_model"`
	AIPromptVersion          *string                `json:"ai_prompt_version"`
	FirstSeenAt              string                 `json:"first_seen_at"`
	LastSeenAt               string                 `json:"last_seen_at"`
	AcceptedAt               *string                `json:"accepted_at"`
	DismissedAt              *string                `json:"dismissed_at"`
	ResolvedAt               *string                `json:"resolved_at"`
	CreatedAt                string                 `json:"created_at"`
	UpdatedAt                string                 `json:"updated_at"`
	RelatedAlerts            []relatedAlertResponse `json:"related_alerts,omitempty"`
}

type relatedAlertResponse struct {
	ID              int64          `json:"id"`
	AlertType       string         `json:"alert_type"`
	AlertGroup      string         `json:"alert_group"`
	EntityType      string         `json:"entity_type"`
	EntityID        *string        `json:"entity_id"`
	EntitySKU       *int64         `json:"entity_sku"`
	EntityOfferID   *string        `json:"entity_offer_id"`
	Title           string         `json:"title"`
	Message         string         `json:"message"`
	Severity        string         `json:"severity"`
	Urgency         string         `json:"urgency"`
	Status          string         `json:"status"`
	EvidencePayload map[string]any `json:"evidence_payload"`
	FirstSeenAt     string         `json:"first_seen_at"`
	LastSeenAt      string         `json:"last_seen_at"`
}

type generateRecommendationsRequest struct {
	AsOfDate *string `json:"as_of_date"`
	RunType  *string `json:"run_type"`
}

type recommendationsSummaryResponse struct {
	OpenTotal  int64 `json:"open_total"`
	ByPriority struct {
		Low      int64 `json:"low"`
		Medium   int64 `json:"medium"`
		High     int64 `json:"high"`
		Critical int64 `json:"critical"`
	} `json:"by_priority"`
	ByConfidence struct {
		Low    int64 `json:"low"`
		Medium int64 `json:"medium"`
		High   int64 `json:"high"`
	} `json:"by_confidence"`
	LatestRun *recommendationLatestRunResponse `json:"latest_run"`
}

type addRecommendationFeedbackRequest struct {
	Rating  string  `json:"rating"`
	Comment *string `json:"comment"`
}

type recommendationFeedbackResponse struct {
	ID               int64   `json:"id"`
	RecommendationID int64   `json:"recommendation_id"`
	Rating           string  `json:"rating"`
	Comment          *string `json:"comment"`
	CreatedAt        string  `json:"created_at"`
}

type recommendationLatestRunResponse struct {
	ID                            int64   `json:"id"`
	RunType                       string  `json:"run_type"`
	Status                        string  `json:"status"`
	StartedAt                     string  `json:"started_at"`
	FinishedAt                    *string `json:"finished_at"`
	AsOfDate                      *string `json:"as_of_date"`
	AIModel                       *string `json:"ai_model"`
	AIPromptVersion               *string `json:"ai_prompt_version"`
	InputTokens                   int64   `json:"input_tokens"`
	OutputTokens                  int64   `json:"output_tokens"`
	TotalTokens                   int64   `json:"total_tokens"`
	EstimatedCost                 float64 `json:"estimated_cost"`
	GeneratedRecommendationsCount int64   `json:"generated_recommendations_count"`
	ErrorMessage                  *string `json:"error_message"`
}

func (h *RecommendationsHandler) ListRecommendations(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	filter, err := parseRecommendationsFilter(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.service.ListRecommendations(r.Context(), sellerAccount.ID, filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list recommendations")
		return
	}
	respItems := make([]recommendationResponse, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, mapRecommendationResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  respItems,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *RecommendationsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	summary, err := h.service.GetSummary(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get recommendations summary")
		return
	}
	resp := recommendationsSummaryResponse{OpenTotal: summary.OpenTotal}
	for _, row := range summary.ByPriority {
		switch row.Name {
		case "low":
			resp.ByPriority.Low = row.Count
		case "medium":
			resp.ByPriority.Medium = row.Count
		case "high":
			resp.ByPriority.High = row.Count
		case "critical":
			resp.ByPriority.Critical = row.Count
		}
	}
	for _, row := range summary.ByConfidence {
		switch row.Name {
		case "low":
			resp.ByConfidence.Low = row.Count
		case "medium":
			resp.ByConfidence.Medium = row.Count
		case "high":
			resp.ByConfidence.High = row.Count
		}
	}
	if summary.LatestRun != nil {
		lr := summary.LatestRun
		resp.LatestRun = &recommendationLatestRunResponse{
			ID:                            lr.ID,
			RunType:                       lr.RunType,
			Status:                        lr.Status,
			StartedAt:                     lr.StartedAt.UTC().Format(time.RFC3339),
			FinishedAt:                    timePtrRFC3339(lr.FinishedAt),
			AsOfDate:                      lr.AsOfDate,
			AIModel:                       lr.AIModel,
			AIPromptVersion:               lr.AIPromptVersion,
			InputTokens:                   int64(lr.InputTokens),
			OutputTokens:                  int64(lr.OutputTokens),
			TotalTokens:                   int64(lr.InputTokens + lr.OutputTokens),
			EstimatedCost:                 lr.EstimatedCost,
			GeneratedRecommendationsCount: int64(lr.GeneratedRecommendationsCount),
			ErrorMessage:                  lr.ErrorMessage,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *RecommendationsHandler) GetRecommendationByID(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := parseRecommendationID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid recommendation id")
		return
	}
	detail, err := h.service.GetRecommendationDetailByID(r.Context(), sellerAccount.ID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "recommendation not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get recommendation")
		return
	}
	writeJSON(w, http.StatusOK, mapRecommendationDetailResponse(detail))
}

func (h *RecommendationsHandler) GenerateRecommendations(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req generateRecommendationsRequest
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
	runType := "manual"
	if req.RunType != nil && strings.TrimSpace(*req.RunType) != "" {
		rt := strings.TrimSpace(*req.RunType)
		switch rt {
		case "manual", "scheduled", "post_alerts", "backfill":
			runType = rt
		default:
			writeJSONError(w, http.StatusBadRequest, "invalid run_type")
			return
		}
	}
	result, err := h.service.GenerateForAccountWithType(r.Context(), sellerAccount.ID, asOf, runType)
	if err != nil {
		if mapAIHandlerError(w, err) {
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to generate recommendations")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *RecommendationsHandler) AcceptRecommendation(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, "accept")
}
func (h *RecommendationsHandler) DismissRecommendation(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, "dismiss")
}
func (h *RecommendationsHandler) ResolveRecommendation(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, "resolve")
}

func (h *RecommendationsHandler) AddFeedback(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := parseRecommendationID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid recommendation id")
		return
	}
	var req addRecommendationFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.AddFeedback(r.Context(), recommendations.AddRecommendationFeedbackInput{
		SellerAccountID:  sellerAccount.ID,
		RecommendationID: id,
		Rating:           recommendations.RecommendationFeedbackRating(strings.TrimSpace(req.Rating)),
		Comment:          req.Comment,
	})
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "recommendation not found")
		case strings.Contains(err.Error(), "invalid feedback rating"):
			writeJSONError(w, http.StatusBadRequest, "invalid rating")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to add recommendation feedback")
		}
		return
	}
	writeJSON(w, http.StatusOK, recommendationFeedbackResponse{
		ID:               item.ID,
		RecommendationID: item.RecommendationID,
		Rating:           string(item.Rating),
		Comment:          item.Comment,
		CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *RecommendationsHandler) handleAction(w http.ResponseWriter, r *http.Request, action string) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := parseRecommendationID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid recommendation id")
		return
	}
	var item recommendations.Recommendation
	switch action {
	case "accept":
		item, err = h.service.AcceptRecommendation(r.Context(), sellerAccount.ID, id)
	case "dismiss":
		item, err = h.service.DismissRecommendation(r.Context(), sellerAccount.ID, id)
	default:
		item, err = h.service.ResolveRecommendation(r.Context(), sellerAccount.ID, id)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "recommendation not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update recommendation")
		return
	}
	writeJSON(w, http.StatusOK, mapRecommendationResponse(item))
}

func parseRecommendationsFilter(r *http.Request) (recommendations.ListFilter, error) {
	filter := recommendations.ListFilter{Limit: 50, Offset: 0}
	q := r.URL.Query()
	set := func(v string) *string {
		if strings.TrimSpace(v) == "" {
			return nil
		}
		s := strings.TrimSpace(v)
		return &s
	}
	filter.Status = set(q.Get("status"))
	filter.RecommendationType = set(q.Get("recommendation_type"))
	filter.PriorityLevel = set(q.Get("priority_level"))
	filter.ConfidenceLevel = set(q.Get("confidence_level"))
	filter.Horizon = set(q.Get("horizon"))
	filter.EntityType = set(q.Get("entity_type"))
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return recommendations.ListFilter{}, errors.New("invalid limit")
		}
		if v > 200 {
			v = 200
		}
		filter.Limit = v
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return recommendations.ListFilter{}, errors.New("invalid offset")
		}
		filter.Offset = v
	}
	return filter, nil
}

func parseRecommendationID(r *http.Request) (int64, error) {
	idRaw := strings.TrimSpace(chi.URLParam(r, "id"))
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// MapPublicRecommendationJSON returns the seller-facing JSON object for a recommendation
// (same shape as list/detail/accept responses; omits raw_ai_response by design).
func MapPublicRecommendationJSON(item recommendations.Recommendation) (map[string]any, error) {
	resp := mapRecommendationResponse(item)
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapRecommendationResponse(item recommendations.Recommendation) recommendationResponse {
	return recommendationResponse{
		ID:                       item.ID,
		Source:                   item.Source,
		RecommendationType:       item.RecommendationType,
		Horizon:                  item.Horizon,
		EntityType:               item.EntityType,
		EntityID:                 item.EntityID,
		EntitySKU:                item.EntitySKU,
		EntityOfferID:            item.EntityOfferID,
		Title:                    item.Title,
		WhatHappened:             item.WhatHappened,
		WhyItMatters:             item.WhyItMatters,
		RecommendedAction:        item.RecommendedAction,
		ExpectedEffect:           item.ExpectedEffect,
		PriorityScore:            item.PriorityScore,
		PriorityLevel:            item.PriorityLevel,
		Urgency:                  item.Urgency,
		ConfidenceLevel:          item.ConfidenceLevel,
		Status:                   item.Status,
		SupportingMetricsPayload: item.SupportingMetrics,
		ConstraintsPayload:       item.Constraints,
		AIModel:                  item.AIModel,
		AIPromptVersion:          item.AIPromptVersion,
		FirstSeenAt:              item.FirstSeenAt.UTC().Format(time.RFC3339),
		LastSeenAt:               item.LastSeenAt.UTC().Format(time.RFC3339),
		AcceptedAt:               timePtrRFC3339(item.AcceptedAt),
		DismissedAt:              timePtrRFC3339(item.DismissedAt),
		ResolvedAt:               timePtrRFC3339(item.ResolvedAt),
		CreatedAt:                item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func mapRecommendationDetailResponse(detail recommendations.RecommendationDetail) recommendationResponse {
	resp := mapRecommendationResponse(detail.Recommendation)
	resp.RelatedAlerts = make([]relatedAlertResponse, 0, len(detail.RelatedAlerts))
	for _, a := range detail.RelatedAlerts {
		resp.RelatedAlerts = append(resp.RelatedAlerts, relatedAlertResponse{
			ID:              a.ID,
			AlertType:       a.AlertType,
			AlertGroup:      a.AlertGroup,
			EntityType:      a.EntityType,
			EntityID:        a.EntityID,
			EntitySKU:       a.EntitySKU,
			EntityOfferID:   a.EntityOfferID,
			Title:           a.Title,
			Message:         a.Message,
			Severity:        a.Severity,
			Urgency:         a.Urgency,
			Status:          a.Status,
			EvidencePayload: a.EvidencePayload,
			FirstSeenAt:     a.FirstSeenAt.UTC().Format(time.RFC3339),
			LastSeenAt:      a.LastSeenAt.UTC().Format(time.RFC3339),
		})
	}
	return resp
}
