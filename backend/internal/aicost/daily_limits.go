package aicost

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrDailyLimitExceeded is returned when a seller exceeds configured AI usage limits.
var ErrDailyLimitExceeded = errors.New("ai daily usage limit exceeded")

// DailyUsageChecker enforces per-day AI usage before OpenAI calls.
type DailyUsageChecker interface {
	CheckChatQuestion(ctx context.Context, sellerAccountID int64) error
	CheckRecommendationRun(ctx context.Context, sellerAccountID int64) error
	CheckSummaryRun(ctx context.Context, sellerAccountID int64) error
}

var _ DailyUsageChecker = (*DailyUsageGuard)(nil)

// DailyLimitsConfig holds per-day and per-request AI caps (MVP technical guards, not billing).
type DailyLimitsConfig struct {
	ChatMaxQuestionsPerDay      int
	RecommendationMaxRunsPerDay int
	SummaryMaxRunsPerDay        int
	MaxInputTokensPerRequest    int
	MaxOutputTokensPerRequest   int
}

// DailyUsageGuard enforces daily AI usage limits using persisted run/message counts.
type DailyUsageGuard struct {
	q   *dbgen.Queries
	cfg DailyLimitsConfig
}

func NewDailyUsageGuard(q *dbgen.Queries, cfg DailyLimitsConfig) *DailyUsageGuard {
	return &DailyUsageGuard{q: q, cfg: normalizeDailyLimits(cfg)}
}

func normalizeDailyLimits(cfg DailyLimitsConfig) DailyLimitsConfig {
	if cfg.ChatMaxQuestionsPerDay <= 0 {
		cfg.ChatMaxQuestionsPerDay = 50
	}
	if cfg.RecommendationMaxRunsPerDay <= 0 {
		cfg.RecommendationMaxRunsPerDay = 3
	}
	if cfg.SummaryMaxRunsPerDay <= 0 {
		cfg.SummaryMaxRunsPerDay = 3
	}
	if cfg.MaxInputTokensPerRequest <= 0 {
		cfg.MaxInputTokensPerRequest = aitokens.MaxInputTokensPerRequest()
	}
	if cfg.MaxOutputTokensPerRequest <= 0 {
		cfg.MaxOutputTokensPerRequest = 4000
	}
	return cfg
}

func utcDayStart(now time.Time) pgtype.Timestamptz {
	u := now.UTC()
	t := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func (g *DailyUsageGuard) CheckChatQuestion(ctx context.Context, sellerAccountID int64) error {
	if g == nil || g.q == nil {
		return nil
	}
	since := utcDayStart(time.Now())
	n, err := g.q.CountChatUserQuestionsSince(ctx, dbgen.CountChatUserQuestionsSinceParams{
		SellerAccountID: sellerAccountID,
		CreatedAt:       since,
	})
	if err != nil {
		return fmt.Errorf("count chat questions: %w", err)
	}
	if n >= int64(g.cfg.ChatMaxQuestionsPerDay) {
		return fmt.Errorf("%w: chat questions per day (%d)", ErrDailyLimitExceeded, g.cfg.ChatMaxQuestionsPerDay)
	}
	return nil
}

func (g *DailyUsageGuard) CheckRecommendationRun(ctx context.Context, sellerAccountID int64) error {
	if g == nil || g.q == nil {
		return nil
	}
	since := utcDayStart(time.Now())
	n, err := g.q.CountRecommendationRunsSince(ctx, dbgen.CountRecommendationRunsSinceParams{
		SellerAccountID: sellerAccountID,
		StartedAt:       since,
	})
	if err != nil {
		return fmt.Errorf("count recommendation runs: %w", err)
	}
	if n >= int64(g.cfg.RecommendationMaxRunsPerDay) {
		return fmt.Errorf("%w: recommendation runs per day (%d)", ErrDailyLimitExceeded, g.cfg.RecommendationMaxRunsPerDay)
	}
	return nil
}

func (g *DailyUsageGuard) CheckSummaryRun(ctx context.Context, sellerAccountID int64) error {
	if g == nil || g.q == nil {
		return nil
	}
	since := utcDayStart(time.Now())
	n, err := g.q.CountAIDailySummaryRunsSince(ctx, dbgen.CountAIDailySummaryRunsSinceParams{
		SellerAccountID: sellerAccountID,
		CreatedAt:       since,
	})
	if err != nil {
		return fmt.Errorf("count ai summary runs: %w", err)
	}
	if n >= int64(g.cfg.SummaryMaxRunsPerDay) {
		return fmt.Errorf("%w: ai summary runs per day (%d)", ErrDailyLimitExceeded, g.cfg.SummaryMaxRunsPerDay)
	}
	return nil
}
