package ingestion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrStatusConnectionNotFound = errors.New("ozon connection not found")

type StatusService struct {
	queries *dbgen.Queries
}

func NewStatusService(db *pgxpool.Pool) *StatusService {
	return &StatusService{
		queries: dbgen.New(db),
	}
}

type SyncJobStatusDTO struct {
	ID           int64   `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	StartedAt    *string `json:"started_at"`
	FinishedAt   *string `json:"finished_at"`
	ErrorMessage *string `json:"error_message"`
}

type ImportJobStatusDTO struct {
	ID              int64   `json:"id"`
	Domain          string  `json:"domain"`
	Status          string  `json:"status"`
	SourceCursor    *string `json:"source_cursor"`
	RecordsReceived int32   `json:"records_received"`
	RecordsImported int32   `json:"records_imported"`
	RecordsFailed   int32   `json:"records_failed"`
	StartedAt       *string `json:"started_at"`
	FinishedAt      *string `json:"finished_at"`
	ErrorMessage    *string `json:"error_message"`
}

type StatusResult struct {
	ConnectionStatus            string               `json:"connection_status"`
	LastCheckAt                 *string              `json:"last_check_at"`
	LastCheckResult             *string              `json:"last_check_result"`
	LastError                   *string              `json:"last_error"`
	PerformanceConnectionStatus string               `json:"performance_connection_status"`
	PerformanceTokenSet         bool                 `json:"performance_token_set"`
	PerformanceLastCheckAt      *string              `json:"performance_last_check_at"`
	PerformanceLastCheckResult  *string              `json:"performance_last_check_result"`
	PerformanceLastError        *string              `json:"performance_last_error"`
	CurrentSync                 *SyncJobStatusDTO    `json:"current_sync"`
	LastSuccessfulSyncAt        *string              `json:"last_successful_sync_at"`
	LatestImportJobs            []ImportJobStatusDTO `json:"latest_import_jobs"`
}

func (s *StatusService) GetStatus(ctx context.Context, sellerAccountID int64) (StatusResult, error) {
	connection, err := s.queries.GetOzonConnectionBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StatusResult{}, ErrStatusConnectionNotFound
		}
		return StatusResult{}, fmt.Errorf("get ozon connection: %w", err)
	}

	result := StatusResult{
		ConnectionStatus:            connection.Status,
		LastCheckAt:                 pgTimePtr(connection.LastCheckAt),
		LastCheckResult:             pgTextPtr(connection.LastCheckResult),
		LastError:                   pgTextPtr(connection.LastError),
		PerformanceConnectionStatus: connection.PerformanceStatus,
		PerformanceTokenSet: connection.PerformanceTokenEncrypted.Valid &&
			strings.TrimSpace(connection.PerformanceTokenEncrypted.String) != "",
		PerformanceLastCheckAt:     pgTimePtr(connection.PerformanceLastCheckAt),
		PerformanceLastCheckResult: pgTextPtr(connection.PerformanceLastCheckResult),
		PerformanceLastError:       pgTextPtr(connection.PerformanceLastError),
		LatestImportJobs:           []ImportJobStatusDTO{},
	}

	latestSyncJob, err := s.queries.GetLatestSyncJobBySellerAccountID(ctx, sellerAccountID)
	if err == nil {
		result.CurrentSync = &SyncJobStatusDTO{
			ID:           latestSyncJob.ID,
			Type:         latestSyncJob.Type,
			Status:       latestSyncJob.Status,
			StartedAt:    pgTimePtr(latestSyncJob.StartedAt),
			FinishedAt:   pgTimePtr(latestSyncJob.FinishedAt),
			ErrorMessage: pgTextPtr(latestSyncJob.ErrorMessage),
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return StatusResult{}, fmt.Errorf("get latest sync job: %w", err)
	}

	lastCompletedSyncJob, err := s.queries.GetLatestCompletedSyncJobBySellerAccountID(ctx, sellerAccountID)
	if err == nil {
		result.LastSuccessfulSyncAt = pgTimePtr(lastCompletedSyncJob.FinishedAt)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return StatusResult{}, fmt.Errorf("get latest completed sync job: %w", err)
	}

	importJobs, err := s.queries.ListLatestImportJobsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return StatusResult{}, fmt.Errorf("list latest import jobs: %w", err)
	}

	result.LatestImportJobs = make([]ImportJobStatusDTO, 0, len(importJobs))
	for _, job := range importJobs {
		result.LatestImportJobs = append(result.LatestImportJobs, ImportJobStatusDTO{
			ID:              job.ID,
			Domain:          job.Domain,
			Status:          job.Status,
			SourceCursor:    pgTextPtr(job.SourceCursor),
			RecordsReceived: job.RecordsReceived,
			RecordsImported: job.RecordsImported,
			RecordsFailed:   job.RecordsFailed,
			StartedAt:       pgTimePtr(job.StartedAt),
			FinishedAt:      pgTimePtr(job.FinishedAt),
			ErrorMessage:    pgTextPtr(job.ErrorMessage),
		})
	}

	return result, nil
}

func pgTextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}

	s := v.String
	return &s
}

func pgTimePtr(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}

	s := v.Time.Format(time.RFC3339)
	return &s
}
