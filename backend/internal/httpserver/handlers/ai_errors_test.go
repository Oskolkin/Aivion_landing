package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func TestMapAIHandlerError_dailyLimit429(t *testing.T) {
	rr := httptest.NewRecorder()
	if !mapAIHandlerError(rr, aicost.ErrDailyLimitExceeded) {
		t.Fatal("expected handled")
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRecommendationsHandlerGenerate_dailyLimitNot500(t *testing.T) {
	svc := recommendations.NewService(
		&mockRecommendationsRepo{},
		nil,
		nil,
		nil,
		recommendations.ServiceConfig{},
		stubRecDailyUsage{recErr: aicost.ErrDailyLimitExceeded},
	)
	h := NewRecommendationsHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/generate", strings.NewReader(`{}`))
	seller := dbgen.SellerAccount{ID: 7}
	ctx := auth.WithAuthContext(req.Context(), dbgen.User{ID: 1}, &seller)
	rr := httptest.NewRecorder()
	h.GenerateRecommendations(rr, req.WithContext(ctx))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

type stubRecDailyUsage struct {
	recErr error
}

func (s stubRecDailyUsage) CheckChatQuestion(context.Context, int64) error      { return nil }
func (s stubRecDailyUsage) CheckRecommendationRun(context.Context, int64) error { return s.recErr }
func (s stubRecDailyUsage) CheckSummaryRun(context.Context, int64) error        { return nil }
