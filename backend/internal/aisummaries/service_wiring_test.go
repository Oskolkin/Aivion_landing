package aisummaries

import (
	"errors"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewServiceFromDeps_missingPool(t *testing.T) {
	_, err := NewServiceFromDeps(ServiceDeps{OpenAIKey: "sk-test"})
	if err == nil || !strings.Contains(err.Error(), "postgres pool") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewServiceFromDeps_missingOpenAIClient(t *testing.T) {
	pool := &pgxpool.Pool{}
	dashboard := analytics.NewDashboardService(pool)
	_, err := NewServiceFromDeps(ServiceDeps{
		Pool:        pool,
		Dashboard:   dashboard,
		AI:          nil,
		OzonSvc:     &ozon.Service{},
		OpenAIKey:   "sk-test",
		OpenAIModel: "gpt-4.1-mini",
	})
	if err == nil || !strings.Contains(err.Error(), "openai client") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewServiceFromDeps_missingOpenAIKey(t *testing.T) {
	pool := &pgxpool.Pool{}
	ai, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewServiceFromDeps(ServiceDeps{
		Pool:        pool,
		Dashboard:   analytics.NewDashboardService(pool),
		AI:          ai,
		OzonSvc:     &ozon.Service{},
		OpenAIKey:   "",
		OpenAIModel: "gpt-4.1-mini",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrOpenAIKeyRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestService_validateRuntime_missingAI(t *testing.T) {
	pool := &pgxpool.Pool{}
	s := &Service{
		pool:      pool,
		q:         dbgen.New(pool),
		dashboard: analytics.NewDashboardService(pool),
		ai:        nil,
		ozonSvc:   &ozon.Service{},
		ozonHTTP:  ozon.NewClient(),
		openAIKey: "sk-test",
	}
	err := s.validateRuntime()
	if err == nil || !strings.Contains(err.Error(), "openai client") {
		t.Fatalf("err=%v", err)
	}
}
