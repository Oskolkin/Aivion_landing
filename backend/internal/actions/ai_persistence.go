package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLCGenerationRepository implements GenerationRepository using sqlc + transactions.
type SQLCGenerationRepository struct {
	*SQLCRepository
	pool *pgxpool.Pool
}

// NewSQLCGenerationRepository creates a generation-aware repository.
func NewSQLCGenerationRepository(pool *pgxpool.Pool, queries *dbgen.Queries) *SQLCGenerationRepository {
	return &SQLCGenerationRepository{
		SQLCRepository: NewSQLCRepository(queries),
		pool:           pool,
	}
}

func (r *SQLCGenerationRepository) CreateActionGenerationRun(ctx context.Context, sellerAccountID int64, metricDate time.Time, status string) (int64, error) {
	row, err := r.queries.CreateActionGenerationRun(ctx, dbgen.CreateActionGenerationRunParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(&metricDate),
		Status:          status,
	})
	if err != nil {
		return 0, fmt.Errorf("create action generation run: %w", err)
	}
	return row.ID, nil
}

func (r *SQLCGenerationRepository) UpdateActionGenerationRun(ctx context.Context, params UpdateGenerationRunParams) error {
	_, err := r.queries.UpdateActionGenerationRun(ctx, dbgen.UpdateActionGenerationRunParams{
		ID:                   params.RunID,
		Status:               params.Status,
		ContextHash:          nullableText(params.ContextHash),
		FinalStatus:          nullableText(params.FinalStatus),
		AcceptedCount:        int32(params.AcceptedCount),
		RepairedCount:        int32(params.RepairedCount),
		DroppedCount:         int32(params.DroppedCount),
		FallbackCount:        int32(params.FallbackCount),
		AiFocusJson:          params.AIFocusJSON,
		TopActionKeysJson:    params.TopActionKeysJSON,
		DataQualityNotesJson: params.DataQualityNotes,
	})
	if err != nil {
		return fmt.Errorf("update action generation run: %w", err)
	}
	return nil
}

func (r *SQLCGenerationRepository) CreateActionGenerationCategoryRun(ctx context.Context, params CreateCategoryRunParams) (int64, error) {
	row, err := r.queries.CreateActionGenerationCategoryRun(ctx, dbgen.CreateActionGenerationCategoryRunParams{
		RunID:               params.RunID,
		SellerAccountID:     params.SellerAccountID,
		Domain:              params.Domain,
		Status:              params.Status,
		Model:               nullableText(params.Model),
		InputContextJson:    params.InputContextJSON,
		RawResponseJson:     params.RawResponseJSON,
		ValidationJson:      params.ValidationJSON,
		RepairResponseJson:  params.RepairResponseJSON,
		FinalValidationJson: params.FinalValidationJSON,
		AcceptedCount:       int32(params.AcceptedCount),
		RepairedCount:       int32(params.RepairedCount),
		DroppedCount:        int32(params.DroppedCount),
	})
	if err != nil {
		return 0, fmt.Errorf("create category run: %w", err)
	}
	return row.ID, nil
}

func (r *SQLCGenerationRepository) UpdateActionGenerationCategoryRun(ctx context.Context, params UpdateCategoryRunParams) error {
	_, err := r.queries.UpdateActionGenerationCategoryRun(ctx, dbgen.UpdateActionGenerationCategoryRunParams{
		ID:                  params.CategoryRunID,
		Status:              params.Status,
		Model:               nullableText(params.Model),
		InputContextJson:    params.InputContextJSON,
		RawResponseJson:     params.RawResponseJSON,
		ValidationJson:      params.ValidationJSON,
		RepairResponseJson:  params.RepairResponseJSON,
		FinalValidationJson: params.FinalValidationJSON,
		AcceptedCount:       int32(params.AcceptedCount),
		RepairedCount:       int32(params.RepairedCount),
		DroppedCount:        int32(params.DroppedCount),
	})
	if err != nil {
		return fmt.Errorf("update category run: %w", err)
	}
	return nil
}

func (r *SQLCGenerationRepository) UpsertActionItem(ctx context.Context, params UpsertActionItemParams) error {
	_, err := r.queries.UpsertActionItem(ctx, dbgen.UpsertActionItemParams{
		SellerAccountID:        params.SellerAccountID,
		RunID:                  pgtype.Int8{Int64: params.RunID, Valid: params.RunID > 0},
		ActionKey:              params.ActionKey,
		Type:                   string(params.Type),
		Category:               string(params.Category),
		Priority:               string(params.Priority),
		Title:                  params.Title,
		Summary:                nullableText(params.Summary),
		Reason:                 params.Reason,
		NextStep:               params.NextStep,
		ExpectedEffect:         nullableText(params.ExpectedEffect),
		EvidenceJson:           params.EvidenceJSON,
		LimitationsJson:        params.LimitationsJSON,
		CtaJson:                params.CTAJSON,
		AffectedEntitiesJson:   params.AffectedEntitiesJSON,
		EntityGroupRef:         nullableText(params.EntityGroupRef),
		SourceCandidateIdsJson: params.SourceCandidateIDs,
		EvidenceFactIdsJson:    params.EvidenceFactIDs,
		PayloadJson:            params.PayloadJSON,
		MetricDate:             toDate(&params.MetricDate),
	})
	if err != nil {
		return fmt.Errorf("upsert action item: %w", err)
	}
	return nil
}

func (r *SQLCGenerationRepository) DeleteStaleActionItemsForDate(ctx context.Context, sellerAccountID int64, metricDate time.Time, keepKeys []string) error {
	if len(keepKeys) == 0 {
		return nil
	}
	if err := r.queries.DeleteStaleActionItemsForDate(ctx, dbgen.DeleteStaleActionItemsForDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(&metricDate),
		Column3:         keepKeys,
	}); err != nil {
		return fmt.Errorf("delete stale action items: %w", err)
	}
	return nil
}

func (r *SQLCGenerationRepository) LinkActionGenerationRunToPipeline(
	ctx context.Context,
	pipelineRunID, actionsRunID int64,
	dataPackHash, globalContextHash string,
) error {
	if pipelineRunID <= 0 || actionsRunID <= 0 {
		return fmt.Errorf("pipeline_run_id and actions_run_id are required")
	}
	if err := r.queries.ClearActionGenerationRunPipelineLinkExcept(ctx, dbgen.ClearActionGenerationRunPipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: pipelineRunID, Valid: true},
		ID:              actionsRunID,
	}); err != nil {
		return fmt.Errorf("clear previous actions pipeline link: %w", err)
	}
	_, err := r.queries.UpdateActionGenerationRunPipelineFields(ctx, dbgen.UpdateActionGenerationRunPipelineFieldsParams{
		ID:                actionsRunID,
		AiPipelineRunID:   pgtype.Int8{Int64: pipelineRunID, Valid: true},
		DataPackHash:      nullableText(dataPackHash),
		GlobalContextHash: nullableText(globalContextHash),
	})
	if err != nil {
		return fmt.Errorf("link action generation run to pipeline: %w", err)
	}
	return nil
}

func (r *SQLCGenerationRepository) PersistGenerationAtomic(ctx context.Context, params PersistGenerationParams) error {
	if r.pool == nil {
		return r.persistWithoutTx(ctx, params)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.queries.WithTx(tx)
	if err := persistWithQueries(ctx, qtx, params); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SQLCGenerationRepository) persistWithoutTx(ctx context.Context, params PersistGenerationParams) error {
	return persistWithQueries(ctx, r.queries, params)
}

func persistWithQueries(ctx context.Context, q *dbgen.Queries, params PersistGenerationParams) error {
	p := params.RunUpdate
	if _, err := q.UpdateActionGenerationRun(ctx, dbgen.UpdateActionGenerationRunParams{
		ID:                   p.RunID,
		Status:               p.Status,
		ContextHash:          nullableText(p.ContextHash),
		FinalStatus:          nullableText(p.FinalStatus),
		AcceptedCount:        int32(p.AcceptedCount),
		RepairedCount:        int32(p.RepairedCount),
		DroppedCount:         int32(p.DroppedCount),
		FallbackCount:        int32(p.FallbackCount),
		AiFocusJson:          p.AIFocusJSON,
		TopActionKeysJson:    p.TopActionKeysJSON,
		DataQualityNotesJson: p.DataQualityNotes,
	}); err != nil {
		return fmt.Errorf("update run in tx: %w", err)
	}
	if params.PipelineRunID != nil && *params.PipelineRunID > 0 {
		if _, err := q.UpdateActionGenerationRunPipelineFields(ctx, dbgen.UpdateActionGenerationRunPipelineFieldsParams{
			ID:                p.RunID,
			AiPipelineRunID:   pgtype.Int8{Int64: *params.PipelineRunID, Valid: true},
			DataPackHash:      nullableText(params.DataPackHash),
			GlobalContextHash: nullableText(params.GlobalContextHash),
		}); err != nil {
			return fmt.Errorf("update run pipeline fields in tx: %w", err)
		}
	}
	for _, item := range params.ActionItems {
		if _, err := q.UpsertActionItem(ctx, dbgen.UpsertActionItemParams{
			SellerAccountID:        item.SellerAccountID,
			RunID:                  pgtype.Int8{Int64: item.RunID, Valid: item.RunID > 0},
			ActionKey:              item.ActionKey,
			Type:                   string(item.Type),
			Category:               string(item.Category),
			Priority:               string(item.Priority),
			Title:                  item.Title,
			Summary:                nullableText(item.Summary),
			Reason:                 item.Reason,
			NextStep:               item.NextStep,
			ExpectedEffect:         nullableText(item.ExpectedEffect),
			EvidenceJson:           item.EvidenceJSON,
			LimitationsJson:        item.LimitationsJSON,
			CtaJson:                item.CTAJSON,
			AffectedEntitiesJson:   item.AffectedEntitiesJSON,
			EntityGroupRef:         nullableText(item.EntityGroupRef),
			SourceCandidateIdsJson: item.SourceCandidateIDs,
			EvidenceFactIdsJson:    item.EvidenceFactIDs,
			PayloadJson:            item.PayloadJSON,
			MetricDate:             toDate(&item.MetricDate),
		}); err != nil {
			return fmt.Errorf("upsert action item in tx: %w", err)
		}
	}
	if params.StaleDelete != nil {
		sd := params.StaleDelete
		if sd.RunID <= 0 {
			return fmt.Errorf("stale delete requires run_id")
		}
		if err := q.DeleteStaleActionItemsForRun(ctx, dbgen.DeleteStaleActionItemsForRunParams{
			RunID:           pgtype.Int8{Int64: sd.RunID, Valid: true},
			SellerAccountID: sd.SellerAccountID,
			Column3:         sd.KeepKeys,
		}); err != nil {
			return fmt.Errorf("delete stale in tx: %w", err)
		}
	}
	return nil
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}
