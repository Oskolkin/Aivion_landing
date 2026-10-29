package aipipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// DashboardCloneStore clones dashboard resources for derived pipeline runs.
type DashboardCloneStore interface {
	CloneDashboardNarrativeForPipeline(
		ctx context.Context,
		sourceDashboardRunID string,
		sourcePipelineRunID int64,
		targetPipelineRunID int64,
		sellerID int64,
		dataPackHash string,
		globalContextHash string,
	) (newDashboardRunID string, err error)
	CloneDashboardDataSnapshotForPipeline(
		ctx context.Context,
		sourceSnapshotID int64,
		sourcePipelineRunID int64,
		targetPipelineRunID int64,
		sellerID int64,
	) (newSnapshotID int64, err error)
	GetDashboardDataSnapshotByPipelineRunID(ctx context.Context, pipelineRunID int64) (dbgen.AiDashboardDataSnapshot, error)
}

func (s *Store) GetDashboardDataSnapshotByPipelineRunID(ctx context.Context, pipelineRunID int64) (dbgen.AiDashboardDataSnapshot, error) {
	if s == nil || s.q == nil {
		return dbgen.AiDashboardDataSnapshot{}, fmt.Errorf("store is not configured")
	}
	return s.q.GetAIDashboardDataSnapshotByPipelineRunID(ctx, pipelineRunID)
}

func (s *Store) CloneDashboardNarrativeForPipeline(
	ctx context.Context,
	sourceDashboardRunID string,
	sourcePipelineRunID int64,
	targetPipelineRunID int64,
	sellerID int64,
	dataPackHash string,
	globalContextHash string,
) (string, error) {
	if s == nil || s.q == nil {
		return "", fmt.Errorf("store is not configured")
	}
	var sourceID pgtype.UUID
	if err := sourceID.Scan(strings.TrimSpace(sourceDashboardRunID)); err != nil {
		return "", fmt.Errorf("parse source dashboard_run_id: %w", err)
	}
	source, err := s.q.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              sourceID,
		SellerAccountID: sellerID,
	})
	if err != nil {
		return "", fmt.Errorf("load source dashboard narrative: %w", err)
	}
	if !source.AiPipelineRunID.Valid || source.AiPipelineRunID.Int64 != sourcePipelineRunID {
		return "", fmt.Errorf("source dashboard narrative is not owned by pipeline %d", sourcePipelineRunID)
	}
	if strings.TrimSpace(textOrEmpty(source.DataPackHash)) != strings.TrimSpace(dataPackHash) {
		return "", fmt.Errorf("source dashboard data_pack_hash mismatch")
	}
	if strings.TrimSpace(textOrEmpty(source.GlobalContextHash)) != strings.TrimSpace(globalContextHash) {
		return "", fmt.Errorf("source dashboard global_context_hash mismatch")
	}

	now := time.Now().UTC()
	row, err := s.q.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
		SellerAccountID: sellerID,
		MetricDate:      source.MetricDate,
		Status:          "running",
		HealthScore:     source.HealthScore,
		Severity:        source.Severity,
		RiskDriversHash: source.RiskDriversHash,
		ContextHash:     source.ContextHash,
		PromptVersion:   source.PromptVersion,
		Model:           source.Model,
		StartedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("insert cloned dashboard narrative: %w", err)
	}

	status := strings.TrimSpace(source.Status)
	if status == "" {
		status = StepStatusCompleted
	}
	completed, err := s.q.CompleteAIDashboardNarrative(ctx, dbgen.CompleteAIDashboardNarrativeParams{
		ID:               row.ID,
		SellerAccountID:  sellerID,
		Status:           status,
		RawAiResponse:    source.RawAiResponse,
		ValidatedPayload: append([]byte(nil), source.ValidatedPayload...),
		ValidationErrors: append([]byte(nil), source.ValidationErrors...),
		InputTokens:      source.InputTokens,
		OutputTokens:     source.OutputTokens,
		EstimatedCost:    source.EstimatedCost,
		ErrorMessage:     source.ErrorMessage,
		FinishedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("complete cloned dashboard narrative: %w", err)
	}

	if err := s.LinkDashboardRun(ctx, uuidString(completed.ID), sellerID, targetPipelineRunID, dataPackHash, globalContextHash); err != nil {
		return "", fmt.Errorf("link cloned dashboard narrative: %w", err)
	}
	return uuidString(completed.ID), nil
}

func (s *Store) CloneDashboardDataSnapshotForPipeline(
	ctx context.Context,
	sourceSnapshotID int64,
	sourcePipelineRunID int64,
	targetPipelineRunID int64,
	sellerID int64,
) (int64, error) {
	if s == nil || s.q == nil {
		return 0, fmt.Errorf("store is not configured")
	}
	source, err := s.q.GetAIDashboardDataSnapshotByID(ctx, sourceSnapshotID)
	if err != nil {
		return 0, fmt.Errorf("load source dashboard data snapshot: %w", err)
	}
	if source.AiPipelineRunID != sourcePipelineRunID || source.SellerAccountID != sellerID {
		return 0, fmt.Errorf("source dashboard data snapshot ownership mismatch")
	}
	if strings.TrimSpace(source.PayloadHash) == "" {
		return 0, fmt.Errorf("source dashboard data snapshot payload_hash is missing")
	}
	status := strings.TrimSpace(source.Status)
	if status == "" {
		status = "completed"
	}
	now := time.Now().UTC()
	row, err := s.q.InsertAIDashboardDataSnapshot(ctx, dbgen.InsertAIDashboardDataSnapshotParams{
		AiPipelineRunID:   targetPipelineRunID,
		SellerAccountID:   sellerID,
		MetricDate:        source.MetricDate,
		DataPackHash:      source.DataPackHash,
		GlobalContextHash: source.GlobalContextHash,
		SchemaVersion:     source.SchemaVersion,
		PayloadJson:       append([]byte(nil), source.PayloadJson...),
		PayloadHash:       source.PayloadHash,
		Status:            status,
		FinishedAt:        pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("insert cloned dashboard data snapshot: %w", err)
	}
	if _, err := s.q.LinkAIPipelineRunDashboardDataSnapshot(ctx, dbgen.LinkAIPipelineRunDashboardDataSnapshotParams{
		ID:                      targetPipelineRunID,
		DashboardDataSnapshotID: pgtype.Int8{Int64: row.ID, Valid: true},
	}); err != nil {
		return 0, fmt.Errorf("link cloned dashboard data snapshot: %w", err)
	}
	return row.ID, nil
}
