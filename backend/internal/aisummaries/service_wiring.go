package aisummaries

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceDeps are required dependencies for aisummaries.Service (same wiring as worker).
type ServiceDeps struct {
	Pool                 *pgxpool.Pool
	Dashboard            *analytics.DashboardService
	AI                   *recommendations.OpenAIClient
	OzonSvc              *ozon.Service
	OpenAIKey            string
	OpenAIModel          string
	MaxInputTokensApprox int
	MaxCategoriesPerRun  int
	DailyUsage           aicost.DailyUsageChecker
}

// NewServiceFromDeps constructs Service after validating dependencies.
func NewServiceFromDeps(d ServiceDeps) (*Service, error) {
	if d.Pool == nil {
		return nil, fmt.Errorf("postgres pool is required for ai summaries service")
	}
	if d.Dashboard == nil {
		return nil, fmt.Errorf("dashboard service is required for ai summaries service")
	}
	if d.AI == nil {
		return nil, fmt.Errorf("openai client is required for ai summaries service")
	}
	if d.OzonSvc == nil {
		return nil, fmt.Errorf("ozon service is required for ai summaries service")
	}
	if strings.TrimSpace(d.OpenAIKey) == "" {
		return nil, ErrOpenAIKeyRequired
	}
	maxCat := d.MaxCategoriesPerRun
	if maxCat <= 0 {
		maxCat = 10
	}
	maxInput := d.MaxInputTokensApprox
	if maxInput <= 0 {
		maxInput = 20000
	}
	return &Service{
		pool:                 d.Pool,
		q:                    dbgen.New(d.Pool),
		dashboard:            d.Dashboard,
		ai:                   d.AI,
		ozonSvc:              d.OzonSvc,
		ozonHTTP:             ozon.NewClient(),
		openAIKey:            strings.TrimSpace(d.OpenAIKey),
		openAIModel:          strings.TrimSpace(d.OpenAIModel),
		maxInputTokensApprox: maxInput,
		maxCategoriesPerRun:  maxCat,
		dailyUsage:           d.DailyUsage,
	}, nil
}

// BuildServiceForAcceptanceLoop is like BuildService but skips daily run limits (dev acceptance tooling).
func BuildServiceForAcceptanceLoop(cfg *config.Config, pool *pgxpool.Pool) (*Service, error) {
	svc, err := buildServiceDeps(cfg, pool)
	if err != nil {
		return nil, err
	}
	svc.dailyUsage = nil
	return svc, nil
}

// BuildService wires dependencies the same way as cmd/worker (config + postgres pool).
func BuildService(cfg *config.Config, pool *pgxpool.Pool) (*Service, error) {
	return buildServiceDeps(cfg, pool)
}

func buildServiceDeps(cfg *config.Config, pool *pgxpool.Pool) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	ozonSvc, err := ozon.NewService(
		pool,
		cfg.Auth.EncryptionKey,
		cfg.Ozon.PerformanceAuthBaseURL,
		cfg.Ozon.DisconnectRetentionDays,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize ozon service: %w", err)
	}
	openAIClient, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:               cfg.OpenAI.APIKey,
		Model:                cfg.OpenAI.Model,
		TimeoutSeconds:       cfg.OpenAI.TimeoutSeconds,
		MaxRetries:           cfg.OpenAI.MaxRetries,
		ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxOutputTokens:      cfg.AI.OpenAIMaxOutputTokens(),
	})
	if err != nil {
		return nil, fmt.Errorf("openai client: %w", err)
	}
	dashboardService := analytics.NewDashboardService(pool)
	return NewServiceFromDeps(ServiceDeps{
		Pool:                 pool,
		Dashboard:            dashboardService,
		AI:                   openAIClient,
		OzonSvc:              ozonSvc,
		OpenAIKey:            cfg.OpenAI.APIKey,
		OpenAIModel:          cfg.OpenAI.Model,
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxCategoriesPerRun:  cfg.AI.SummaryMaxCategoriesPerRun,
		DailyUsage: aicost.NewDailyUsageGuard(dbgen.New(pool), aicost.DailyLimitsConfig{
			ChatMaxQuestionsPerDay:      cfg.AI.ChatMaxQuestionsPerDay,
			RecommendationMaxRunsPerDay: cfg.AI.RecommendationMaxRunsPerDay,
			SummaryMaxRunsPerDay:        cfg.AI.SummaryMaxRunsPerDay,
			MaxInputTokensPerRequest:    cfg.AI.MaxInputTokensPerRequest,
			MaxOutputTokensPerRequest:   cfg.AI.MaxOutputTokensPerRequest,
		}),
	})
}
