package dashboardhome

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) persistFrozenDashboardSnapshot(ctx context.Context,
	pipelineRunID int64,
	sellerAccountID int64,
	metricDate time.Time,
	dataPackHash string,
	globalContextHash string,
	dto analytics.DashboardMetricsDTO,
	resp Response,
) (int64, error) {
	if trackPersistFrozenDashboardSnapshotCalls.Load() {
		persistFrozenDashboardSnapshotCalls.Add(1)
	}
	if s == nil || s.q == nil {
		return 0, fmt.Errorf("dashboard service is not configured")
	}
	if pipelineRunID <= 0 {
		return 0, fmt.Errorf("pipeline_run_id is required")
	}
	dataPackHash = strings.TrimSpace(dataPackHash)
	globalContextHash = strings.TrimSpace(globalContextHash)
	if dataPackHash == "" || globalContextHash == "" {
		return 0, fmt.Errorf("dashboard snapshot hashes are required")
	}
	frozen := NewFrozenDashboardData(metricDate.UTC().Format("2006-01-02"), dto, &resp)
	payloadHash, err := frozen.CanonicalHash()
	if err != nil {
		return 0, err
	}
	payloadJSON, err := json.Marshal(frozen)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	row, err := s.q.InsertAIDashboardDataSnapshot(ctx, dbgen.InsertAIDashboardDataSnapshotParams{
		AiPipelineRunID:   pipelineRunID,
		SellerAccountID:   sellerAccountID,
		MetricDate:        dateValue(metricDate),
		DataPackHash:      dataPackHash,
		GlobalContextHash: globalContextHash,
		SchemaVersion:     FrozenDashboardDataSchemaVersion,
		PayloadJson:       payloadJSON,
		PayloadHash:       payloadHash,
		Status:            "completed",
		FinishedAt:        pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("insert dashboard data snapshot: %w", err)
	}
	if _, err := s.q.LinkAIPipelineRunDashboardDataSnapshot(ctx, dbgen.LinkAIPipelineRunDashboardDataSnapshotParams{
		ID:                      pipelineRunID,
		DashboardDataSnapshotID: pgtype.Int8{Int64: row.ID, Valid: true},
	}); err != nil {
		return 0, fmt.Errorf("link dashboard data snapshot: %w", err)
	}
	return row.ID, nil
}

func (s *Service) loadFrozenDashboardSnapshot(ctx context.Context, pipelineRunID int64) (FrozenDashboardData, string, error) {
	if s == nil || s.q == nil {
		return FrozenDashboardData{}, "", fmt.Errorf("dashboard service is not configured")
	}
	row, err := s.q.GetAIDashboardDataSnapshotByPipelineRunID(ctx, pipelineRunID)
	if err != nil {
		return FrozenDashboardData{}, "", fmt.Errorf("load dashboard data snapshot: %w", err)
	}
	frozen, err := ParseFrozenDashboardData(row.PayloadJson)
	if err != nil {
		return FrozenDashboardData{}, "", err
	}
	if strings.TrimSpace(row.PayloadHash) == "" {
		return FrozenDashboardData{}, "", fmt.Errorf("dashboard data snapshot payload_hash is missing")
	}
	if strings.TrimSpace(row.DataPackHash) == "" || strings.TrimSpace(row.GlobalContextHash) == "" {
		return FrozenDashboardData{}, "", fmt.Errorf("dashboard data snapshot hashes are missing")
	}
	return frozen, row.PayloadHash, nil
}
