package unifiedanalysis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository wraps sqlc queries for unified_analysis runs.
type Repository struct {
	q *dbgen.Queries
}

// NewRepository creates a Repository.
func NewRepository(q *dbgen.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) CreateRun(ctx context.Context, sellerID int64, metricDate pgtype.Date, cfg Config, startedAt pgtype.Timestamptz, trigger string) (*Run, error) {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = TriggerManual
	}
	row, err := r.q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            metricDate,
		Mode:                  ModeUnifiedAnalysis,
		Trigger:               trigger,
		Status:                StatusCreated,
		DataPackSchemaVersion: nullableText(AnalysisPackSchemaVersion),
		DataPackHash:          DataPackHashPending,
		PipelineVersion:       nullableText(PipelineVersionV1),
		PromptVersion:         nullableText(firstNonEmpty(cfg.PromptVersion, promptVersionForConfig(cfg))),
		Provider:              nullableText(cfg.Provider),
		ProviderModel:         nullableText(cfg.Model),
		ProviderAttemptCount:  0,
		StartedAt:             startedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create unified analysis run: %w", err)
	}
	run := RunFromDB(row)
	return run, nil
}

func promptVersionForConfig(cfg Config) string {
	switch strings.ToLower(strings.TrimSpace(cfg.PromptProfile)) {
	case "analysis", "production", "analyst":
		return PromptVersionAnalyst
	default:
		return PromptVersionSmoke
	}
}

func (r *Repository) GetRun(ctx context.Context, runID int64) (*Run, error) {
	row, err := r.q.GetUnifiedAnalysisRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("get unified analysis run: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) FindActiveRun(ctx context.Context, sellerID int64, metricDate pgtype.Date) (*Run, error) {
	row, err := r.q.FindActiveUnifiedAnalysisRun(ctx, dbgen.FindActiveUnifiedAnalysisRunParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find active unified analysis run: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) TransitionStatus(ctx context.Context, runID int64, from, to string) (*Run, error) {
	row, err := r.q.TransitionUnifiedAnalysisRunStatus(ctx, dbgen.TransitionUnifiedAnalysisRunStatusParams{
		ID:             runID,
		ExpectedStatus: from,
		NextStatus:     to,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCASConflict
		}
		return nil, fmt.Errorf("transition unified analysis run status: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SavePack(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisPackParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.SaveUnifiedAnalysisPack(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save unified analysis pack: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveContainer(ctx context.Context, runID int64, containerID string) (*Run, error) {
	row, err := r.q.SaveUnifiedAnalysisContainer(ctx, dbgen.SaveUnifiedAnalysisContainerParams{
		ID:                  runID,
		ProviderContainerID: nullableText(containerID),
	})
	if err != nil {
		return nil, fmt.Errorf("save unified analysis container: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveContainerFile(ctx context.Context, runID int64, fileID, filePath string) (*Run, error) {
	row, err := r.q.SaveUnifiedAnalysisContainerFile(ctx, dbgen.SaveUnifiedAnalysisContainerFileParams{
		ID:                        runID,
		ProviderContainerFileID:   nullableText(fileID),
		ProviderContainerFilePath: nullableText(filePath),
	})
	if err != nil {
		return nil, fmt.Errorf("save unified analysis container file: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveResponseStarted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseStartedParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.SaveUnifiedAnalysisResponseStarted(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save unified analysis response started: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveResponseCompleted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseCompletedParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.SaveUnifiedAnalysisResponseCompleted(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save unified analysis response completed: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) RequestCancellation(ctx context.Context, runID int64) (*Run, error) {
	row, err := r.q.RequestUnifiedAnalysisCancellation(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("request unified analysis cancellation: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) MarkFailed(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisFailedParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.MarkUnifiedAnalysisFailed(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("mark unified analysis failed: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveProviderTerminalFailure(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisProviderTerminalFailureParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.SaveUnifiedAnalysisProviderTerminalFailure(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save provider terminal failure: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) StartNextProviderAttempt(ctx context.Context, runID int64, params dbgen.StartNextUnifiedAnalysisProviderAttemptParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.StartNextUnifiedAnalysisProviderAttempt(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCASConflict
		}
		return nil, fmt.Errorf("start next provider attempt: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) MarkCancelled(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisCancelledParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.MarkUnifiedAnalysisCancelled(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("mark unified analysis cancelled: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) SaveCleanupResult(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisCleanupResultParams) (*Run, error) {
	params.ID = runID
	row, err := r.q.SaveUnifiedAnalysisCleanupResult(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save unified analysis cleanup result: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) UpdateProviderStatus(ctx context.Context, runID int64, providerStatus, status string) (*Run, error) {
	row, err := r.q.UpdateUnifiedAnalysisProviderStatus(ctx, dbgen.UpdateUnifiedAnalysisProviderStatusParams{
		ID:             runID,
		ProviderStatus: nullableText(providerStatus),
		Status:         nullableText(status),
	})
	if err != nil {
		return nil, fmt.Errorf("update unified analysis provider status: %w", err)
	}
	return RunFromDB(row), nil
}

func (r *Repository) ListRecoverableRuns(ctx context.Context, limit int32) ([]*Run, error) {
	rows, err := r.q.ListRecoverableUnifiedAnalysisRuns(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable unified analysis runs: %w", err)
	}
	out := make([]*Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, RunFromDB(row))
	}
	return out, nil
}

func (r *Repository) CreateArtifact(ctx context.Context, params dbgen.CreateAIPipelineArtifactParams) (int64, error) {
	row, err := r.q.CreateAIPipelineArtifact(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("create ai pipeline artifact: %w", err)
	}
	return row.ID, nil
}

func (r *Repository) ListArtifacts(ctx context.Context, runID int64) ([]dbgen.AiPipelineArtifact, error) {
	rows, err := r.q.ListAIPipelineArtifacts(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list ai pipeline artifacts: %w", err)
	}
	return rows, nil
}

// SavePublicationBlocked transitions to publication_blocked and stores error fields.
func (r *Repository) SavePublicationBlocked(ctx context.Context, runID int64, code, message string) (*Run, error) {
	row, err := r.q.SaveUnifiedAnalysisPublicationBlocked(ctx, dbgen.SaveUnifiedAnalysisPublicationBlockedParams{
		ID:           runID,
		ErrorCode:    nullableText(code),
		ErrorMessage: nullableText(truncateErr(message, 500)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCASConflict
		}
		return nil, fmt.Errorf("save publication blocked: %w", err)
	}
	return RunFromDB(row), nil
}
