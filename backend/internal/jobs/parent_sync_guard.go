package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

var (
	ErrParentSyncPending        = errors.New("parent sync pending")
	ErrParentSyncRunning        = errors.New("parent sync running")
	ErrParentSyncFailed         = errors.New("parent sync failed")
	ErrParentSyncCancelled      = errors.New("parent sync cancelled")
	ErrParentSyncNotFound       = errors.New("parent sync not found")
	ErrParentSyncSellerMismatch = errors.New("parent sync seller mismatch")
	ErrParentSyncMetricMismatch = errors.New("parent sync metric date mismatch")
	ErrParentSyncUnknownStatus  = errors.New("parent sync unknown status")
)

// ParentSyncGuardError is the structured guard failure used for handler mapping.
type ParentSyncGuardError struct {
	Code      string
	Retryable bool
	SyncJobID int64
	SellerID  int64
	Actual    string
	Expected  string
	Err       error
}

func (e *ParentSyncGuardError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *ParentSyncGuardError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ValidatedParentSync is the canonical parent sync context for downstream work.
type ValidatedParentSync struct {
	SyncJobID       int64
	SellerAccountID int64
	SyncType        string
	StartedAt       time.Time
	FinishedAt      time.Time
	MetricDate      time.Time
}

// ParentSyncGuard validates parent sync before any downstream business side effects.
type ParentSyncGuard interface {
	RequireCompletedParentSync(
		ctx context.Context,
		syncJobID int64,
		sellerAccountID int64,
		requestedMetricDate *time.Time,
	) (ValidatedParentSync, error)
}

// SQLCParentSyncGuard loads sync jobs and canonical metric dates from PostgreSQL.
type SQLCParentSyncGuard struct {
	queries *dbgen.Queries
	log     *zap.Logger
	metrics *metrics.Metrics
	handler string
}

func NewSQLCParentSyncGuard(q *dbgen.Queries, log *zap.Logger, m *metrics.Metrics, handler string) *SQLCParentSyncGuard {
	if log == nil {
		log = zap.NewNop()
	}
	return &SQLCParentSyncGuard{queries: q, log: log, metrics: m, handler: handler}
}

func (g *SQLCParentSyncGuard) RequireCompletedParentSync(
	ctx context.Context,
	syncJobID int64,
	sellerAccountID int64,
	requestedMetricDate *time.Time,
) (ValidatedParentSync, error) {
	if g == nil || g.queries == nil {
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncNotFound, syncJobID, sellerAccountID, "", "")
	}
	if syncJobID <= 0 {
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncNotFound, syncJobID, sellerAccountID, "", "")
	}
	if sellerAccountID <= 0 {
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncSellerMismatch, syncJobID, sellerAccountID, "", "")
	}

	job, err := g.queries.GetSyncJobByID(ctx, syncJobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			g.recordGuard("blocked", "missing")
			g.logGuard(ctx, syncJobID, sellerAccountID, "", "blocked", ErrParentSyncNotFound, false)
			return ValidatedParentSync{}, guardPermanent(ErrParentSyncNotFound, syncJobID, sellerAccountID, "", "")
		}
		return ValidatedParentSync{}, err
	}

	if job.SellerAccountID != sellerAccountID {
		g.recordGuard("blocked", "seller_mismatch")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "blocked", ErrParentSyncSellerMismatch, false)
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncSellerMismatch, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	}

	switch job.Status {
	case syncdomain.SyncStatusCompleted.String():
	case syncdomain.SyncStatusPending.String():
		g.recordGuard("retry", "pending")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "retry", ErrParentSyncPending, true)
		return ValidatedParentSync{}, guardRetry(ErrParentSyncPending, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	case syncdomain.SyncStatusRunning.String():
		g.recordGuard("retry", "running")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "retry", ErrParentSyncRunning, true)
		return ValidatedParentSync{}, guardRetry(ErrParentSyncRunning, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	case syncdomain.SyncStatusFailed.String():
		g.recordGuard("blocked", "failed")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "blocked", ErrParentSyncFailed, false)
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncFailed, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	case "cancelled":
		g.recordGuard("blocked", "cancelled")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "blocked", ErrParentSyncCancelled, false)
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncCancelled, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	default:
		g.recordGuard("blocked", "unknown")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "blocked", ErrParentSyncUnknownStatus, false)
		return ValidatedParentSync{}, guardPermanent(ErrParentSyncUnknownStatus, syncJobID, sellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	}

	canonical, err := g.resolveCanonicalMetricDate(ctx, sellerAccountID)
	if err != nil {
		return ValidatedParentSync{}, err
	}
	if requestedMetricDate != nil && !sameCalendarDayUTC(*requestedMetricDate, canonical) {
		g.recordGuard("blocked", "metric_mismatch")
		g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "blocked", ErrParentSyncMetricMismatch, false)
		return ValidatedParentSync{}, guardPermanent(
			ErrParentSyncMetricMismatch,
			syncJobID,
			sellerAccountID,
			requestedMetricDate.UTC().Format("2006-01-02"),
			canonical.Format("2006-01-02"),
		)
	}

	out := ValidatedParentSync{
		SyncJobID:       job.ID,
		SellerAccountID: job.SellerAccountID,
		SyncType:        job.Type,
		MetricDate:      canonical,
	}
	if job.StartedAt.Valid {
		out.StartedAt = job.StartedAt.Time.UTC()
	}
	if job.FinishedAt.Valid {
		out.FinishedAt = job.FinishedAt.Time.UTC()
	}

	g.recordGuard("allowed", "completed")
	g.logGuard(ctx, syncJobID, sellerAccountID, job.Status, "allowed", nil, false)
	return out, nil
}

func (g *SQLCParentSyncGuard) resolveCanonicalMetricDate(ctx context.Context, sellerAccountID int64) (time.Time, error) {
	latest, err := g.queries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return time.Time{}, err
	}
	if !latest.Valid || latest.Time.Year() < 1970 {
		return postSyncCalendarDayUTC(time.Now().UTC()), nil
	}
	return postSyncCalendarDayUTC(latest.Time), nil
}

func (g *SQLCParentSyncGuard) recordGuard(result, reason string) {
	if g == nil || g.metrics == nil || g.metrics.ParentSyncGuardTotal == nil {
		return
	}
	handler := g.handler
	if handler == "" {
		handler = "unknown"
	}
	g.metrics.ParentSyncGuardTotal.WithLabelValues(handler, result, reason).Inc()
}

func (g *SQLCParentSyncGuard) logGuard(
	ctx context.Context,
	syncJobID, sellerAccountID int64,
	parentStatus, guardResult string,
	err error,
	retryable bool,
) {
	if g == nil || g.log == nil {
		return
	}
	fields := []zap.Field{
		zap.Int64("sync_job_id", syncJobID),
		zap.Int64("seller_account_id", sellerAccountID),
		zap.String("parent_sync_status", parentStatus),
		zap.String("guard_result", guardResult),
		zap.Bool("retryable", retryable),
	}
	if err != nil {
		fields = append(fields, zap.String("guard_error_code", guardErrorCode(err)), zap.Error(err))
	}
	g.log.Info("parent sync guard evaluated", fields...)
	_ = ctx
}

func guardErrorCode(err error) string {
	var ge *ParentSyncGuardError
	if errors.As(err, &ge) && ge.Code != "" {
		return ge.Code
	}
	switch {
	case errors.Is(err, ErrParentSyncPending):
		return "pending"
	case errors.Is(err, ErrParentSyncRunning):
		return "running"
	case errors.Is(err, ErrParentSyncFailed):
		return "failed"
	case errors.Is(err, ErrParentSyncCancelled):
		return "cancelled"
	case errors.Is(err, ErrParentSyncNotFound):
		return "missing"
	case errors.Is(err, ErrParentSyncSellerMismatch):
		return "seller_mismatch"
	case errors.Is(err, ErrParentSyncMetricMismatch):
		return "metric_mismatch"
	default:
		return "unknown"
	}
}

func guardRetry(err error, syncJobID, sellerID int64, actual, expected string) error {
	return &ParentSyncGuardError{
		Code:      guardErrorCode(err),
		Retryable: true,
		SyncJobID: syncJobID,
		SellerID:  sellerID,
		Actual:    actual,
		Expected:  expected,
		Err:       err,
	}
}

func guardPermanent(err error, syncJobID, sellerID int64, actual, expected string) error {
	return &ParentSyncGuardError{
		Code:      guardErrorCode(err),
		Retryable: false,
		SyncJobID: syncJobID,
		SellerID:  sellerID,
		Actual:    actual,
		Expected:  expected,
		Err:       err,
	}
}

func sameCalendarDayUTC(a, b time.Time) bool {
	au := a.UTC()
	bu := b.UTC()
	return au.Year() == bu.Year() && au.Month() == bu.Month() && au.Day() == bu.Day()
}

// IsRetryableParentSyncGuardError reports temporary parent sync states.
func IsRetryableParentSyncGuardError(err error) bool {
	var ge *ParentSyncGuardError
	if errors.As(err, &ge) {
		return ge.Retryable
	}
	return errors.Is(err, ErrParentSyncPending) || errors.Is(err, ErrParentSyncRunning)
}

// IsTerminalParentSyncGuardError reports integrity/terminal guard failures.
func IsTerminalParentSyncGuardError(err error) bool {
	if err == nil {
		return false
	}
	var ge *ParentSyncGuardError
	if errors.As(err, &ge) {
		return !ge.Retryable
	}
	return errors.Is(err, ErrParentSyncFailed) ||
		errors.Is(err, ErrParentSyncCancelled) ||
		errors.Is(err, ErrParentSyncNotFound) ||
		errors.Is(err, ErrParentSyncSellerMismatch) ||
		errors.Is(err, ErrParentSyncMetricMismatch) ||
		errors.Is(err, ErrParentSyncUnknownStatus)
}

// MapParentSyncGuardErrorToAsynq maps guard failures to Asynq retry semantics.
func MapParentSyncGuardErrorToAsynq(err error) error {
	if err == nil {
		return nil
	}
	if IsRetryableParentSyncGuardError(err) {
		return fmt.Errorf("parent sync not ready: %w", err)
	}
	return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
}

func parseOptionalMetricDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("metric_date: %w", err)
	}
	d := postSyncCalendarDayUTC(parsed)
	return &d, nil
}
