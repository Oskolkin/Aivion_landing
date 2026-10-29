package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestRecommendationsHandlerAddFeedback(t *testing.T) {
	handler := NewRecommendationsHandler(recommendations.NewService(
		&mockRecommendationsRepo{},
		nil,
		nil,
		nil,
		recommendations.ServiceConfig{},
		nil,
	))

	makeReq := func(body string, id string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/"+id+"/feedback", strings.NewReader(body))
		seller := dbgen.SellerAccount{ID: 7}
		ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 1, Email: "u@x.y"}, &seller)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	}

	t.Run("success", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler.AddFeedback(rr, makeReq(`{"rating":"positive","comment":"ok"}`, "1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("invalid rating", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler.AddFeedback(rr, makeReq(`{"rating":"wrong"}`, "1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := recommendations.NewService(&mockRecommendationsRepo{getErr: pgx.ErrNoRows}, nil, nil, nil, recommendations.ServiceConfig{}, nil)
		h := NewRecommendationsHandler(svc)
		rr := httptest.NewRecorder()
		h.AddFeedback(rr, makeReq(`{"rating":"positive"}`, "1"))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

func TestRecommendationsHandlerPublicResponsesOmitRawAIResponse(t *testing.T) {
	now := time.Now().UTC()
	withRaw := recommendations.Recommendation{
		ID:                 12,
		Source:             "ai",
		RecommendationType: "replenish_sku",
		Horizon:            "short_term",
		EntityType:         "account",
		Title:              "t",
		WhatHappened:       "w",
		WhyItMatters:       "y",
		RecommendedAction:  "a",
		PriorityScore:      1,
		PriorityLevel:      "low",
		Urgency:            "low",
		ConfidenceLevel:    "low",
		Status:             "open",
		SupportingMetrics:  map[string]any{},
		Constraints:        map[string]any{},
		FirstSeenAt:        now,
		LastSeenAt:         now,
		CreatedAt:          now,
		UpdatedAt:          now,
		RawAIResponse:      map[string]any{"must_not_leak": true},
	}

	makeGET := func(id string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/"+id, nil)
		seller := dbgen.SellerAccount{ID: 7}
		ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 1, Email: "u@x.y"}, &seller)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	}

	makeAction := func(id string, action string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/"+id+"/"+action, nil)
		seller := dbgen.SellerAccount{ID: 7}
		ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 1, Email: "u@x.y"}, &seller)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	}

	t.Run("GET detail", func(t *testing.T) {
		repo := &mockRecommendationsRepo{getByID: &withRaw}
		svc := recommendations.NewService(repo, nil, nil, nil, recommendations.ServiceConfig{}, nil)
		h := NewRecommendationsHandler(svc)
		rr := httptest.NewRecorder()
		h.GetRecommendationByID(rr, makeGET("12"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if _, ok := out["raw_ai_response"]; ok {
			t.Fatalf("public GET /recommendations/{id} must not include raw_ai_response")
		}
	})

	t.Run("POST accept", func(t *testing.T) {
		accepted := withRaw
		accepted.Status = "accepted"
		repo := &mockRecommendationsRepo{actionRec: &accepted}
		svc := recommendations.NewService(repo, nil, nil, nil, recommendations.ServiceConfig{}, nil)
		h := NewRecommendationsHandler(svc)
		rr := httptest.NewRecorder()
		h.AcceptRecommendation(rr, makeAction("12", "accept"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if _, ok := out["raw_ai_response"]; ok {
			t.Fatalf("public POST accept must not include raw_ai_response")
		}
	})
}

type mockRecommendationsRepo struct {
	getErr    error
	getByID   *recommendations.Recommendation
	actionRec *recommendations.Recommendation
}

func (m *mockRecommendationsRepo) CreateRun(context.Context, recommendations.CreateRecommendationRunInput) (int64, error) {
	return 0, nil
}
func (m *mockRecommendationsRepo) CompleteRun(context.Context, recommendations.CompleteRecommendationRunInput) error {
	return nil
}
func (m *mockRecommendationsRepo) FailRun(context.Context, int64, int64, string) error { return nil }
func (m *mockRecommendationsRepo) CreateRunDiagnostic(context.Context, recommendations.CreateRunDiagnosticInput) error {
	return nil
}
func (m *mockRecommendationsRepo) UpsertRecommendation(context.Context, recommendations.UpsertRecommendationInput) (int64, error) {
	return 0, nil
}
func (m *mockRecommendationsRepo) DeleteRecommendationAlertLinks(context.Context, int64, int64) error {
	return nil
}
func (m *mockRecommendationsRepo) LinkRecommendationAlert(context.Context, int64, int64, int64) error {
	return nil
}
func (m *mockRecommendationsRepo) ListRecommendationsFiltered(context.Context, int64, recommendations.ListFilter) ([]recommendations.Recommendation, error) {
	return nil, nil
}
func (m *mockRecommendationsRepo) GetRecommendationByID(context.Context, int64, int64) (recommendations.Recommendation, error) {
	if m.getErr != nil {
		return recommendations.Recommendation{}, m.getErr
	}
	if m.getByID != nil {
		return *m.getByID, nil
	}
	return recommendations.Recommendation{ID: 1}, nil
}
func (m *mockRecommendationsRepo) ListAlertsByRecommendationID(context.Context, int64, int64) ([]recommendations.RelatedAlert, error) {
	return nil, nil
}
func (m *mockRecommendationsRepo) AcceptRecommendation(context.Context, int64, int64) (recommendations.Recommendation, error) {
	if m.actionRec != nil {
		return *m.actionRec, nil
	}
	return recommendations.Recommendation{}, nil
}
func (m *mockRecommendationsRepo) DismissRecommendation(context.Context, int64, int64) (recommendations.Recommendation, error) {
	if m.actionRec != nil {
		return *m.actionRec, nil
	}
	return recommendations.Recommendation{}, nil
}
func (m *mockRecommendationsRepo) ResolveRecommendation(context.Context, int64, int64) (recommendations.Recommendation, error) {
	if m.actionRec != nil {
		return *m.actionRec, nil
	}
	return recommendations.Recommendation{}, nil
}
func (m *mockRecommendationsRepo) CountOpenRecommendations(context.Context, int64) (int64, error) {
	return 0, nil
}
func (m *mockRecommendationsRepo) CountOpenRecommendationsByPriority(context.Context, int64) ([]recommendations.NamedCount, error) {
	return nil, nil
}
func (m *mockRecommendationsRepo) CountOpenRecommendationsByConfidence(context.Context, int64) ([]recommendations.NamedCount, error) {
	return nil, nil
}
func (m *mockRecommendationsRepo) GetLatestRecommendationRun(context.Context, int64) (*recommendations.RunInfo, error) {
	return nil, nil
}
func (m *mockRecommendationsRepo) CreateFeedback(ctx context.Context, input recommendations.AddRecommendationFeedbackInput) (*recommendations.RecommendationFeedback, error) {
	return &recommendations.RecommendationFeedback{
		ID:               1,
		RecommendationID: input.RecommendationID,
		SellerAccountID:  input.SellerAccountID,
		Rating:           input.Rating,
		Comment:          input.Comment,
		CreatedAt:        time.Now().UTC(),
	}, nil
}
