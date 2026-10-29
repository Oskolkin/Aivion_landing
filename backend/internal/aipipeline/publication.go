package aipipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SnapshotPublisher interface {
	PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (PublishResult, error)
}

var ErrStalePipelinePublication = errors.New("stale pipeline publication rejected")

type PublishResult struct {
	Snapshot         publishedsnapshot.Snapshot
	AlreadyPublished bool
	IsCurrent        bool
}

type SQLCSnapshotPublisher struct {
	pool *pgxpool.Pool
}

func NewSQLCSnapshotPublisher(pool *pgxpool.Pool) *SQLCSnapshotPublisher {
	return &SQLCSnapshotPublisher{pool: pool}
}

func (p *SQLCSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (PublishResult, error) {
	if pipelineRunID <= 0 {
		return PublishResult{}, fmt.Errorf("invalid pipeline_run_id")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PublishResult{}, err
	}
	defer tx.Rollback(ctx)

	q := dbgen.New(tx)
	pipelineBase, err := q.GetAIPipelineRun(ctx, pipelineRunID)
	if err != nil {
		return PublishResult{}, err
	}
	if pipelineBase.SellerAccountID <= 0 {
		return PublishResult{}, fmt.Errorf("invalid seller_account_id in pipeline")
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", pipelineBase.SellerAccountID); err != nil {
		return PublishResult{}, fmt.Errorf("seller publication lock failed: %w", err)
	}
	pipeline, err := q.GetAIPipelineRunForUpdate(ctx, pipelineRunID)
	if err != nil {
		return PublishResult{}, err
	}
	resolver := publishedsnapshot.NewSQLCResolver(dbgen.New(tx))
	if pipeline.PublishedAt.Valid {
		existing, resErr := resolver.GetPublishedSnapshotByPipelineID(ctx, pipeline.SellerAccountID, pipeline.ID)
		if resErr != nil {
			return PublishResult{}, fmt.Errorf("pipeline already published but snapshot invalid: %w", resErr)
		}
		isCurrent, curErr := isPipelineCurrentForSeller(ctx, q, pipeline.SellerAccountID, pipeline.ID)
		if curErr != nil {
			return PublishResult{}, curErr
		}
		if err := tx.Commit(ctx); err != nil {
			return PublishResult{}, err
		}
		return PublishResult{
			Snapshot:         existing,
			AlreadyPublished: true,
			IsCurrent:        isCurrent,
		}, nil
	}
	if !pipeline.DashboardRunID.Valid || !pipeline.ActionsRunID.Valid || !pipeline.DashboardDataSnapshotID.Valid {
		return PublishResult{}, fmt.Errorf("pipeline has empty snapshot references")
	}
	if strings.TrimSpace(pipeline.DataPackHash) == "" {
		return PublishResult{}, fmt.Errorf("pipeline data_pack_hash is required")
	}
	if !pipeline.GlobalContextHash.Valid || strings.TrimSpace(pipeline.GlobalContextHash.String) == "" {
		return PublishResult{}, fmt.Errorf("pipeline global_context_hash is required")
	}
	pipelineGlobalHash := strings.TrimSpace(pipeline.GlobalContextHash.String)
	if current, curErr := q.GetLatestPublishedAIPipelineRunBySeller(ctx, pipeline.SellerAccountID); curErr == nil && current.ID != pipeline.ID {
		if isStalePipelineRelativeToCurrent(current, pipeline) {
			return PublishResult{}, ErrStalePipelinePublication
		}
	}

	dashSnap, err := q.GetAIDashboardDataSnapshotByID(ctx, pipeline.DashboardDataSnapshotID.Int64)
	if err != nil {
		return PublishResult{}, fmt.Errorf("load dashboard data snapshot for publication: %w", err)
	}
	if dashSnap.AiPipelineRunID != pipeline.ID || dashSnap.SellerAccountID != pipeline.SellerAccountID {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot ownership mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(dashSnap.Status), "completed") && !strings.EqualFold(strings.TrimSpace(dashSnap.Status), "available") {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot status is not publishable")
	}
	if strings.TrimSpace(dashSnap.PayloadHash) == "" {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot payload_hash is required")
	}
	if strings.TrimSpace(dashSnap.DataPackHash) != strings.TrimSpace(pipeline.DataPackHash) {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot data_pack_hash mismatch")
	}
	if strings.TrimSpace(dashSnap.GlobalContextHash) != pipelineGlobalHash {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot global_context_hash mismatch")
	}
	if !dashSnap.MetricDate.Valid || dashSnap.MetricDate.Time.UTC().Format("2006-01-02") != pipeline.MetricDate.Time.UTC().Format("2006-01-02") {
		return PublishResult{}, fmt.Errorf("dashboard data snapshot metric_date mismatch")
	}

	dash, err := q.GetAIDashboardNarrativeByIDForUpdate(ctx, dbgen.GetAIDashboardNarrativeByIDForUpdateParams{
		ID:              pipeline.DashboardRunID,
		SellerAccountID: pipeline.SellerAccountID,
	})
	if err != nil {
		return PublishResult{}, fmt.Errorf("load dashboard run for publication: %w", err)
	}
	act, err := q.GetActionGenerationRunForUpdate(ctx, pipeline.ActionsRunID.Int64)
	if err != nil {
		return PublishResult{}, fmt.Errorf("load actions run for publication: %w", err)
	}
	if act.SellerAccountID != pipeline.SellerAccountID {
		return PublishResult{}, fmt.Errorf("actions seller mismatch")
	}
	if !dash.AiPipelineRunID.Valid || dash.AiPipelineRunID.Int64 != pipeline.ID {
		return PublishResult{}, fmt.Errorf("dashboard run is not linked to pipeline")
	}
	if !act.AiPipelineRunID.Valid || act.AiPipelineRunID.Int64 != pipeline.ID {
		return PublishResult{}, fmt.Errorf("actions run is not linked to pipeline")
	}
	if !dash.MetricDate.Valid || !act.MetricDate.Valid || dash.MetricDate.Time.UTC().Format("2006-01-02") != pipeline.MetricDate.Time.UTC().Format("2006-01-02") || act.MetricDate.Time.UTC().Format("2006-01-02") != pipeline.MetricDate.Time.UTC().Format("2006-01-02") {
		return PublishResult{}, fmt.Errorf("metric_date mismatch across runs")
	}
	if !strings.EqualFold(strings.TrimSpace(dash.Status), "completed") {
		return PublishResult{}, fmt.Errorf("dashboard run status is not publishable")
	}
	finalStatus := strings.TrimSpace(textOrEmpty(act.FinalStatus))
	if finalStatus == "" {
		finalStatus = strings.TrimSpace(act.Status)
	}
	if !actions.IsPublishableActionFinalStatus(finalStatus) {
		return PublishResult{}, fmt.Errorf("actions run status is not publishable")
	}
	if !actions.IsEmptyActionFinalStatus(finalStatus) && !strings.EqualFold(strings.TrimSpace(act.Status), "completed") {
		return PublishResult{}, fmt.Errorf("actions run status is not publishable")
	}
	if !dash.DataPackHash.Valid || strings.TrimSpace(dash.DataPackHash.String) != strings.TrimSpace(pipeline.DataPackHash) {
		return PublishResult{}, fmt.Errorf("dashboard data_pack_hash mismatch")
	}
	if !act.DataPackHash.Valid || strings.TrimSpace(act.DataPackHash.String) != strings.TrimSpace(pipeline.DataPackHash) {
		return PublishResult{}, fmt.Errorf("actions data_pack_hash mismatch")
	}
	if !dash.GlobalContextHash.Valid || strings.TrimSpace(dash.GlobalContextHash.String) != pipelineGlobalHash {
		return PublishResult{}, fmt.Errorf("dashboard global_context_hash mismatch")
	}
	if !act.GlobalContextHash.Valid || strings.TrimSpace(act.GlobalContextHash.String) != pipelineGlobalHash {
		return PublishResult{}, fmt.Errorf("actions global_context_hash mismatch")
	}

	now := time.Now().UTC()
	_, err = q.SetAIPipelinePublishedAt(ctx, dbgen.SetAIPipelinePublishedAtParams{
		ID:          pipeline.ID,
		PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Status:      pgtype.Text{String: PipelineStatusCompleted, Valid: pipeline.Status != PipelineStatusCompleted},
		CompletedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return PublishResult{}, err
	}
	if err := q.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
		SellerAccountID: pipeline.SellerAccountID,
		PipelineRunID:   pipeline.ID,
	}); err != nil {
		return PublishResult{}, fmt.Errorf("set current seller snapshot pointer: %w", err)
	}
	snapshot, err := resolver.GetPublishedSnapshotByPipelineID(ctx, pipeline.SellerAccountID, pipeline.ID)
	if err != nil {
		return PublishResult{}, fmt.Errorf("resolve snapshot after publication: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, err
	}
	return PublishResult{
		Snapshot:         snapshot,
		AlreadyPublished: false,
		IsCurrent:        true,
	}, nil
}

func isPipelineCurrentForSeller(ctx context.Context, q *dbgen.Queries, sellerAccountID, pipelineRunID int64) (bool, error) {
	currentID, err := q.GetSellerCurrentPipelineRunID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return currentID == pipelineRunID, nil
}

func isStalePipelineRelativeToCurrent(current, candidate dbgen.AiPipelineRun) bool {
	currentDate := current.MetricDate.Time.UTC().Format("2006-01-02")
	candidateDate := candidate.MetricDate.Time.UTC().Format("2006-01-02")
	if currentDate > candidateDate {
		return true
	}
	if currentDate < candidateDate {
		return false
	}
	if current.CreatedAt.Time.After(candidate.CreatedAt.Time) {
		return true
	}
	if current.CreatedAt.Time.Equal(candidate.CreatedAt.Time) && current.ID > candidate.ID {
		return true
	}
	return false
}
