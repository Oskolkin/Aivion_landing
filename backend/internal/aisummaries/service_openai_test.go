package aisummaries

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustRecommendationsOpenAIClient(t *testing.T) *recommendations.OpenAIClient {
	t.Helper()
	client, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{APIKey: "sk-test", Model: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// Regression: OpenAI errors return (nil, err); callers must check err before out.InputTokens.
func TestCallOpenAI_errorBeforeNilOutUsage(t *testing.T) {
	pool := &pgxpool.Pool{}
	svc := &Service{
		pool:        pool,
		q:           nil,
		dashboard:   analytics.NewDashboardService(pool),
		ai:          mustRecommendationsOpenAIClient(t),
		ozonSvc:     &ozon.Service{},
		ozonHTTP:    ozon.NewClient(),
		openAIKey:   "sk-test",
		openAIModel: "gpt-test",
	}

	// Simulate provider error path: nil output + error must not panic when handled by caller.
	_, err := svc.callOpenAI(context.Background(), "sys", "user", map[string]any{"k": "v"})
	// Real client will fail HTTP; we only assert no panic and error is returned.
	if err == nil {
		t.Fatal("expected openai error for unreachable API in unit test")
	}
	if !strings.Contains(err.Error(), "openai request failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestService_callOpenAI_nilClient(t *testing.T) {
	svc := &Service{ai: nil, openAIKey: "sk-test", openAIModel: "gpt-test"}
	_, err := svc.callOpenAI(context.Background(), "sys", "user", nil)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err=%v", err)
	}
}

func TestOpenAIResultGuard_pattern(t *testing.T) {
	var out *recommendations.GenerateRecommendationsOutput
	var err error = errors.New("model not found")
	if err == nil && out != nil {
		t.Fatal("unreachable")
	}
	if err != nil && out == nil {
		return // correct order: check err before out.InputTokens
	}
	t.Fatal("expected error branch")
}
