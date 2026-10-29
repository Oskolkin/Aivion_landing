package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSyncAlreadyRunning = errors.New("sync job is already running")

// TaskEnqueuer enqueues Asynq tasks (implemented by *asynq.Client in production).
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type OrchestrationService struct {
	db       *pgxpool.Pool
	queries  *dbgen.Queries
	enqueuer TaskEnqueuer
}

func NewOrchestrationService(db *pgxpool.Pool, enqueuer TaskEnqueuer) *OrchestrationService {
	return &OrchestrationService{
		db:       db,
		queries:  dbgen.New(db),
		enqueuer: enqueuer,
	}
}

type ozonSyncCoordinatorPayload struct {
	SellerAccountID int64  `json:"seller_account_id"`
	SyncJobID       int64  `json:"sync_job_id"`
	SyncType        string `json:"sync_type"`
}

func (s *OrchestrationService) StartSync(ctx context.Context, sellerAccountID int64, syncType string) (dbgen.SyncJob, error) {
	normalized := syncdomain.NormalizeSyncType(syncType)

	job, err := s.queries.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sellerAccountID,
		Type:            normalized.String(),
		Status:          syncdomain.SyncStatusPending.String(),
		StartedAt:       pgtype.Timestamptz{Valid: false},
		FinishedAt:      pgtype.Timestamptz{Valid: false},
		ErrorMessage:    pgtype.Text{Valid: false},
	})
	if err != nil {
		if isActiveSyncUniqueViolation(err) {
			return dbgen.SyncJob{}, ErrSyncAlreadyRunning
		}
		return dbgen.SyncJob{}, fmt.Errorf("create sync job: %w", err)
	}

	payload, err := json.Marshal(ozonSyncCoordinatorPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       job.ID,
		SyncType:        normalized.String(),
	})
	if err != nil {
		_, _ = s.queries.MarkSyncJobFailedIfNonTerminal(ctx, dbgen.MarkSyncJobFailedIfNonTerminalParams{
			ID: job.ID,
			ErrorMessage: pgtype.Text{
				String: "failed to build sync coordinator task",
				Valid:  true,
			},
		})
		return dbgen.SyncJob{}, fmt.Errorf("create sync coordinator task: %w", err)
	}
	task := asynq.NewTask(syncdomain.TaskTypeOzonSyncCoordinator, payload)

	taskID := "ozon-sync:" + strconv.FormatInt(job.ID, 10)
	if _, err := s.enqueuer.Enqueue(task, asynq.TaskID(taskID)); err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			return job, nil
		}
		_, _ = s.queries.MarkSyncJobFailedIfNonTerminal(ctx, dbgen.MarkSyncJobFailedIfNonTerminalParams{
			ID: job.ID,
			ErrorMessage: pgtype.Text{
				String: "failed to enqueue sync coordinator task",
				Valid:  true,
			},
		})
		return dbgen.SyncJob{}, fmt.Errorf("enqueue sync coordinator task: %w", err)
	}

	return job, nil
}

func (s *OrchestrationService) StartInitialSync(ctx context.Context, sellerAccountID int64) (dbgen.SyncJob, error) {
	return s.StartSync(ctx, sellerAccountID, syncdomain.SyncTypeInitial.String())
}

func isActiveSyncUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "idx_sync_jobs_one_active_per_seller"
}

// EnsureImportJobsForSync guarantees one import job per domain for a sync job.
func EnsureImportJobsForSync(
	ctx context.Context,
	q *dbgen.Queries,
	cursorResolver func(ctx context.Context, sellerAccountID int64, domain string) (pgtype.Text, error),
	sellerAccountID, syncJobID int64,
) ([]dbgen.ImportJob, error) {
	out := make([]dbgen.ImportJob, 0, len(syncdomain.AllImportDomains))
	for _, domain := range syncdomain.AllImportDomains {
		sourceCursor, err := cursorResolver(ctx, sellerAccountID, domain.String())
		if err != nil {
			return nil, fmt.Errorf("resolve source cursor for %s: %w", domain, err)
		}
		row, err := q.EnsureImportJob(ctx, dbgen.EnsureImportJobParams{
			SellerAccountID: sellerAccountID,
			SyncJobID:       syncJobID,
			Domain:          domain.String(),
			SourceCursor:    sourceCursor,
		})
		if err != nil {
			return nil, fmt.Errorf("ensure import job for %s: %w", domain, err)
		}
		out = append(out, row)
	}
	return out, nil
}

func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
