package aicost

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func TestNormalizeDailyLimits_invalidUsesSafeDefaults(t *testing.T) {
	cfg := normalizeDailyLimits(DailyLimitsConfig{})
	if cfg.ChatMaxQuestionsPerDay != 50 {
		t.Fatalf("chat=%d", cfg.ChatMaxQuestionsPerDay)
	}
	if cfg.RecommendationMaxRunsPerDay != 3 {
		t.Fatalf("rec=%d", cfg.RecommendationMaxRunsPerDay)
	}
	if cfg.SummaryMaxRunsPerDay != 3 {
		t.Fatalf("summary=%d", cfg.SummaryMaxRunsPerDay)
	}
	if cfg.MaxInputTokensPerRequest != aitokens.DefaultMaxInputTokensPerRequest {
		t.Fatalf("in=%d want %d", cfg.MaxInputTokensPerRequest, aitokens.DefaultMaxInputTokensPerRequest)
	}
	if cfg.MaxOutputTokensPerRequest != 4000 {
		t.Fatalf("out=%d", cfg.MaxOutputTokensPerRequest)
	}
}

func TestNormalizeDailyLimits_preservesPositive(t *testing.T) {
	cfg := normalizeDailyLimits(DailyLimitsConfig{
		ChatMaxQuestionsPerDay:      7,
		RecommendationMaxRunsPerDay: 2,
		SummaryMaxRunsPerDay:        1,
		MaxInputTokensPerRequest:    1000,
		MaxOutputTokensPerRequest:   500,
	})
	if cfg.ChatMaxQuestionsPerDay != 7 || cfg.RecommendationMaxRunsPerDay != 2 {
		t.Fatalf("unexpected %+v", cfg)
	}
}
