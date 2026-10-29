package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PipelineExecutionClaim is the result of an atomic execution claim attempt.
type PipelineExecutionClaim struct {
	DispatchID    int64
	PipelineRunID int64
	LeaseToken    string

	Claimed          bool
	AlreadyRunning   bool
	AlreadyCompleted bool
	AlreadyPublished bool
	RetryNotDue      bool
	TerminalFailed   bool
	AwaitingRecovery bool
}

// ClaimSource distinguishes handler vs recovery reclaim semantics.
type ClaimSource string

const (
	ClaimSourceHandler  ClaimSource = "handler"
	ClaimSourceRecovery ClaimSource = "recovery"
)

// IdempotentSuccess reports whether the handler should exit without error or AI work.
func (c PipelineExecutionClaim) IdempotentSuccess() bool {
	return c.AlreadyRunning || c.AlreadyCompleted || c.AlreadyPublished || c.AwaitingRecovery
}

// LeaseHash returns a stable non-secret log label for a lease token.
func LeaseHash(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// ClaimPipelineExecution atomically links a logical pipeline run and issues an execution lease.
func (s *AIPipelineDispatchService) ClaimPipelineExecution(
	ctx context.Context,
	dispatchID int64,
	leaseToken string,
	metricDate time.Time,
	source ClaimSource,
) (PipelineExecutionClaim, error) {
	var zero PipelineExecutionClaim
	if s == nil || s.queries == nil {
		return zero, fmt.Errorf("dispatch service not configured")
	}
	if s.pool == nil {
		return zero, fmt.Errorf("dispatch claim requires database pool")
	}
	if dispatchID <= 0 {
		return zero, fmt.Errorf("invalid dispatch_id")
	}
	if leaseToken == "" {
		return zero, fmt.Errorf("lease token is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)
	now := time.Now().UTC()
	leaseUntil := now.Add(s.cfg.LeaseDuration)

	dispatch, err := qtx.GetAIPipelineDispatchByIDForUpdate(ctx, dispatchID)
	if err != nil {
		return zero, fmt.Errorf("lock dispatch: %w", err)
	}

	if claim := classifyDispatchClaim(dispatch, now, source); claim.IdempotentSuccess() || claim.RetryNotDue || claim.TerminalFailed {
		if err := tx.Commit(ctx); err != nil {
			return zero, err
		}
		return claim, nil
	}

	if dispatch.PipelineRunID.Valid && dispatch.PipelineRunID.Int64 > 0 {
		run, rerr := qtx.GetAIPipelineRunForUpdate(ctx, dispatch.PipelineRunID.Int64)
		if rerr != nil {
			if errors.Is(rerr, pgx.ErrNoRows) {
				if _, ferr := qtx.MarkAIPipelineDispatchFailed(ctx, dbgen.MarkAIPipelineDispatchFailedParams{
					ID:            dispatch.ID,
					LastError:     pgtype.Text{String: fmt.Sprintf("missing pipeline run %d", dispatch.PipelineRunID.Int64), Valid: true},
					LastErrorCode: pgtype.Text{String: "pipeline_run_missing", Valid: true},
				}); ferr != nil {
					return zero, fmt.Errorf("mark dispatch failed: %w", ferr)
				}
				if err := tx.Commit(ctx); err != nil {
					return zero, err
				}
				return PipelineExecutionClaim{DispatchID: dispatch.ID, TerminalFailed: true}, nil
			}
			return zero, fmt.Errorf("load pipeline run: %w", rerr)
		}
		if run.PublishedAt.Valid {
			completed, cerr := qtx.MarkAIPipelineDispatchCompletedByID(ctx, dbgen.MarkAIPipelineDispatchCompletedByIDParams{
				ID:            dispatch.ID,
				PipelineRunID: pgtype.Int8{Int64: run.ID, Valid: true},
			})
			if cerr != nil {
				return zero, fmt.Errorf("complete published dispatch: %w", cerr)
			}
			if err := tx.Commit(ctx); err != nil {
				return zero, err
			}
			_ = completed
			return PipelineExecutionClaim{
				DispatchID:       dispatch.ID,
				PipelineRunID:    run.ID,
				AlreadyPublished: true,
			}, nil
		}
	}

	pipelineRunID, err := s.ensureLogicalPipelineRun(ctx, qtx, dispatch, metricDate, now)
	if err != nil {
		return zero, err
	}

	claimed, err := qtx.ClaimAIPipelineDispatchExecution(ctx, dbgen.ClaimAIPipelineDispatchExecutionParams{
		ID:                  dispatch.ID,
		PipelineRunID:       pgtype.Int8{Int64: pipelineRunID, Valid: true},
		ExecutionLeaseToken: pgtype.Text{String: leaseToken, Valid: true},
		LeaseAcquiredAt:     pgTimestamptz(now),
		LeaseExpiresAt:      pgTimestamptz(leaseUntil),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			refreshed, rerr := qtx.GetAIPipelineDispatchByID(ctx, dispatch.ID)
			if rerr != nil {
				return zero, rerr
			}
			if claim := classifyDispatchClaim(refreshed, now, source); claim.IdempotentSuccess() || claim.RetryNotDue || claim.TerminalFailed {
				if err := tx.Commit(ctx); err != nil {
					return zero, err
				}
				return claim, nil
			}
			return zero, fmt.Errorf("dispatch not claimable: status=%s", refreshed.Status)
		}
		return zero, fmt.Errorf("claim dispatch execution: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}

	s.log.Info("pipeline execution claimed",
		zap.Int64("dispatch_id", claimed.ID),
		zap.Int64("pipeline_run_id", pipelineRunID),
		zap.String("lease_token_hash", LeaseHash(leaseToken)),
		zap.String("claim_source", string(source)),
	)

	return PipelineExecutionClaim{
		DispatchID:    claimed.ID,
		PipelineRunID: pipelineRunID,
		LeaseToken:    leaseToken,
		Claimed:       true,
	}, nil
}

func classifyDispatchClaim(dispatch dbgen.AiPipelineDispatch, now time.Time, source ClaimSource) PipelineExecutionClaim {
	claim := PipelineExecutionClaim{DispatchID: dispatch.ID}
	if dispatch.PipelineRunID.Valid {
		claim.PipelineRunID = dispatch.PipelineRunID.Int64
	}

	switch dispatch.Status {
	case DispatchStatusCompleted:
		claim.AlreadyCompleted = true
	case DispatchStatusFailed:
		claim.TerminalFailed = true
	case DispatchStatusPipelineStarted:
		if hasFreshLease(dispatch, now) {
			claim.AlreadyRunning = true
			return claim
		}
		if source == ClaimSourceHandler {
			claim.AwaitingRecovery = true
			return claim
		}
	case DispatchStatusRetryPending:
		if dispatch.NextAttemptAt.Valid && dispatch.NextAttemptAt.Time.After(now) {
			claim.RetryNotDue = true
		}
	case DispatchStatusEnqueued:
		// claimable
	default:
		if dispatchTerminal(dispatch.Status) {
			claim.TerminalFailed = true
		}
	}
	return claim
}

func hasFreshLease(dispatch dbgen.AiPipelineDispatch, now time.Time) bool {
	return dispatch.ExecutionLeaseToken.Valid &&
		dispatch.ExecutionLeaseToken.String != "" &&
		dispatch.LeaseExpiresAt.Valid &&
		dispatch.LeaseExpiresAt.Time.After(now)
}

func (s *AIPipelineDispatchService) ensureLogicalPipelineRun(
	ctx context.Context,
	qtx *dbgen.Queries,
	dispatch dbgen.AiPipelineDispatch,
	metricDate time.Time,
	now time.Time,
) (int64, error) {
	if dispatch.PipelineRunID.Valid && dispatch.PipelineRunID.Int64 > 0 {
		return dispatch.PipelineRunID.Int64, nil
	}

	existing, err := qtx.GetAIPipelineRunByDispatchID(ctx, pgtype.Int8{Int64: dispatch.ID, Valid: true})
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	metric := metricDate
	if dispatch.MetricDate.Valid {
		metric = dispatch.MetricDate.Time
	}
	if metric.IsZero() {
		metric = now
	}
	metric = time.Date(metric.Year(), metric.Month(), metric.Day(), 0, 0, 0, 0, time.UTC)

	created, err := qtx.CreateAIPipelineRun(ctx, dbgen.CreateAIPipelineRunParams{
		SellerAccountID: dispatch.SellerAccountID,
		MetricDate:      pgDate(metric),
		Mode:            aipipeline.PipelineModeUnified,
		Trigger:         "post_sync",
		Status:          aipipeline.PipelineStatusRunning,
		DataPackHash:    fmt.Sprintf("dispatch:%d:pending", dispatch.ID),
		StartedAt:       pgTimestamptz(now),
		DispatchID:      pgtype.Int8{Int64: dispatch.ID, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create pipeline run for dispatch: %w", err)
	}
	return created.ID, nil
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// ValidateExecutionLease ensures the caller still owns the active execution lease.
func (s *AIPipelineDispatchService) ValidateExecutionLease(ctx context.Context, dispatchID, pipelineRunID int64, leaseToken string) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("dispatch service not configured")
	}
	now := time.Now().UTC()
	_, err := s.queries.ValidateAIPipelineDispatchLease(ctx, dbgen.ValidateAIPipelineDispatchLeaseParams{
		ID:                  dispatchID,
		PipelineRunID:       pgtype.Int8{Int64: pipelineRunID, Valid: true},
		ExecutionLeaseToken: pgtype.Text{String: leaseToken, Valid: true},
		LeaseExpiresAt:      pgTimestamptz(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("execution lease lost or expired")
		}
		return err
	}
	return nil
}

// RenewExecutionLease extends the active lease for long-running pipelines.
func (s *AIPipelineDispatchService) RenewExecutionLease(ctx context.Context, dispatchID int64, leaseToken string) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("dispatch service not configured")
	}
	now := time.Now().UTC()
	_, err := s.queries.RenewAIPipelineDispatchLease(ctx, dbgen.RenewAIPipelineDispatchLeaseParams{
		ID:                  dispatchID,
		LeaseExpiresAt:      pgTimestamptz(now.Add(s.cfg.LeaseDuration)),
		ExecutionLeaseToken: pgtype.Text{String: leaseToken, Valid: true},
		NowTs:               pgTimestamptz(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("execution lease renew rejected")
		}
		return err
	}
	return nil
}

// ReleaseStaleExecution transitions expired leases to retry_pending for recovery enqueue.
func (s *AIPipelineDispatchService) ReleaseStaleExecution(ctx context.Context, dispatchID int64, now time.Time) (dbgen.AiPipelineDispatch, error) {
	if s == nil || s.queries == nil {
		return dbgen.AiPipelineDispatch{}, fmt.Errorf("dispatch service not configured")
	}
	return s.queries.ReleaseStaleAIPipelineDispatchExecution(ctx, dbgen.ReleaseStaleAIPipelineDispatchExecutionParams{
		ID:             dispatchID,
		NowTs:          pgTimestamptz(now),
		StartedTimeout: durationInterval(s.cfg.StartedTimeout),
		LastError:      pgtype.Text{String: "stale pipeline execution lease", Valid: true},
		LastErrorCode:  pgtype.Text{String: "pipeline_stale_lease", Valid: true},
	})
}

// WithPool returns a copy of the dispatch service bound to a pgx pool for transactional claims.
func (s *AIPipelineDispatchService) WithPool(pool *pgxpool.Pool) *AIPipelineDispatchService {
	if s == nil {
		return nil
	}
	copy := *s
	copy.pool = pool
	return &copy
}
