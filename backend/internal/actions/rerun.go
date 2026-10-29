package actions

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewGenerationServiceFromPool wires the real generation pipeline from a database pool.
func NewGenerationServiceFromPool(pool *pgxpool.Pool, ai ActionAIClient) *GenerationService {
	queries := dbgen.New(pool)
	candRepo := NewSQLCCandidateRepository(queries)
	builder := NewCandidateBuilder(candRepo, DefaultCandidateBuilderConfig())
	genRepo := NewSQLCGenerationRepository(pool, queries)
	if ai == nil {
		ai = &FakeActionAIClient{}
	}
	return NewGenerationService(builder, ai, genRepo).
		WithAIDataPackBuilder(aidatapack.NewService(pool))
}

// RerunActionsRequest configures a manual actions generation rerun.
type RerunActionsRequest struct {
	SellerAccountID int64
	MetricDate      *time.Time
	AIEnabled       *bool
	Force           bool
	DryRun          bool
}

// RerunActionsResponse is the JSON summary for rerun entrypoints.
type RerunActionsResponse struct {
	RunID         int64    `json:"run_id"`
	Status        string   `json:"status"`
	FinalStatus   string   `json:"final_status"`
	AcceptedCount int      `json:"accepted_count"`
	FallbackCount int      `json:"fallback_count"`
	ActionKeys    []string `json:"action_keys"`
}

// RerunService triggers real actions generation outside the seller feed.
type RerunService struct {
	generation *GenerationService
}

func NewRerunService(generation *GenerationService) *RerunService {
	return &RerunService{generation: generation}
}

// ActionsRerunRunner is implemented by RerunService for admin/dev rerun entrypoints.
type ActionsRerunRunner interface {
	RerunActions(ctx context.Context, req RerunActionsRequest) (*RerunActionsResponse, error)
}

func (s *RerunService) RerunActions(ctx context.Context, req RerunActionsRequest) (*RerunActionsResponse, error) {
	if s == nil || s.generation == nil {
		return nil, ErrGenerationUnavailable
	}
	if req.SellerAccountID <= 0 {
		return nil, ErrInvalidSellerID
	}
	metricDate := time.Now().UTC()
	if req.MetricDate != nil {
		metricDate = req.MetricDate.UTC()
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	opts := GenerateOptionsFromEnv()
	if req.AIEnabled != nil {
		opts.AIEnabled = *req.AIEnabled
	}
	opts.DryRun = req.DryRun
	_ = req.Force // reserved for future stale-delete policy overrides

	result, err := s.generation.GenerateActions(ctx, req.SellerAccountID, metricDate, opts)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(result.Actions))
	for _, item := range result.Actions {
		keys = append(keys, item.ActionKey)
	}
	status := GenerationRunStatusCompleted
	if result.FinalStatus == GenerationFinalStatusFailed {
		status = GenerationRunStatusFailed
	}
	return &RerunActionsResponse{
		RunID:         result.RunID,
		Status:        status,
		FinalStatus:   result.FinalStatus,
		AcceptedCount: len(result.Actions),
		FallbackCount: result.FallbackCount,
		ActionKeys:    keys,
	}, nil
}
