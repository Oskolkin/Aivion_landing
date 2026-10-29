//go:build integration

package runtimeintegration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublishedSnapshotPublication_IdempotentAndStaleGuard(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	resolver := publishedsnapshot.NewSQLCResolver(h.Queries)

	p1 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-2*time.Minute))
	p2 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-1*time.Minute))

	_, err := publisher.PublishPipelineSnapshot(ctx, p2)
	if err != nil {
		t.Fatalf("publish p2: %v", err)
	}
	// Idempotent re-publish.
	_, err = publisher.PublishPipelineSnapshot(ctx, p2)
	if err != nil {
		t.Fatalf("republish p2: %v", err)
	}
	if _, err := publisher.PublishPipelineSnapshot(ctx, p1); !errors.Is(err, aipipeline.ErrStalePipelinePublication) {
		t.Fatalf("expected stale publication error, got %v", err)
	}
	cur, err := resolver.GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatalf("current snapshot: %v", err)
	}
	if cur.PipelineRunID != p2 {
		t.Fatalf("current pipeline=%d want %d", cur.PipelineRunID, p2)
	}
}

func TestPublishedSnapshotPublication_ConcurrentOrderProtected(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	resolver := publishedsnapshot.NewSQLCResolver(h.Queries)

	pOld := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-2*time.Minute))
	pNew := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-1*time.Minute))

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		errs <- publishIgnoreStale(ctx, publisher, pOld)
	}()
	go func() {
		defer wg.Done()
		errs <- publishIgnoreStale(ctx, publisher, pNew)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent publish: %v", err)
		}
	}
	cur, err := resolver.GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatalf("resolve current snapshot: %v", err)
	}
	if cur.PipelineRunID != pNew {
		t.Fatalf("current pipeline=%d want newest %d", cur.PipelineRunID, pNew)
	}
}

func TestPublishedSnapshotPublication_HistoricalRepublishStaysNonCurrent(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	resolver := publishedsnapshot.NewSQLCResolver(h.Queries)

	p1 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-2*time.Minute))
	p2 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC), time.Now().UTC().Add(-1*time.Minute))

	if _, err := publisher.PublishPipelineSnapshot(ctx, p1); err != nil {
		t.Fatalf("publish p1: %v", err)
	}
	if _, err := publisher.PublishPipelineSnapshot(ctx, p2); err != nil {
		t.Fatalf("publish p2: %v", err)
	}
	republish, err := publisher.PublishPipelineSnapshot(ctx, p1)
	if err != nil {
		t.Fatalf("republish historical p1: %v", err)
	}
	if !republish.AlreadyPublished || republish.IsCurrent {
		t.Fatalf("historical republish must be AlreadyPublished=true IsCurrent=false, got %+v", republish)
	}
	cur, err := resolver.GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatalf("resolve current snapshot: %v", err)
	}
	if cur.PipelineRunID != p2 {
		t.Fatalf("current pipeline=%d want %d", cur.PipelineRunID, p2)
	}
}

func TestPublishedSnapshotPublication_ValidationMismatchesRejected(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)

	t.Run("hash_mismatch", func(t *testing.T) {
		runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), time.Now().UTC())
		if _, err := h.Pool.Exec(ctx, `UPDATE action_generation_runs SET data_pack_hash = 'sha256:broken' WHERE ai_pipeline_run_id = $1`, runID); err != nil {
			t.Fatalf("break hash: %v", err)
		}
		if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err == nil {
			t.Fatal("expected hash mismatch publication error")
		}
	})

	t.Run("seller_mismatch", func(t *testing.T) {
		runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), time.Now().UTC())
		otherSeller := h.SeedSeller(t)
		if _, err := h.Pool.Exec(ctx, `UPDATE action_generation_runs SET seller_account_id = $2 WHERE ai_pipeline_run_id = $1`, runID, otherSeller); err != nil {
			t.Fatalf("break seller ownership: %v", err)
		}
		if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err == nil {
			t.Fatal("expected seller mismatch publication error")
		}
	})

	t.Run("metric_date_mismatch", func(t *testing.T) {
		runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), time.Now().UTC())
		if _, err := h.Pool.Exec(ctx, `UPDATE action_generation_runs SET metric_date = ($2::date + interval '1 day')::date WHERE ai_pipeline_run_id = $1`, runID, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("break metric_date: %v", err)
		}
		if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err == nil {
			t.Fatal("expected metric_date mismatch publication error")
		}
	})
}

func TestPublishedSnapshotImmutability_GuardsPublishedRows(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), time.Now().UTC())
	if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err != nil {
		t.Fatalf("publish run: %v", err)
	}
	run, err := h.Queries.GetAIPipelineRun(ctx, runID)
	if err != nil {
		t.Fatalf("load pipeline run: %v", err)
	}
	if !run.DashboardRunID.Valid || !run.ActionsRunID.Valid {
		t.Fatal("pipeline must have dashboard/actions runs linked")
	}

	if _, err := h.Queries.UpdateAIDashboardNarrativeValidatedPayload(ctx, dbgen.UpdateAIDashboardNarrativeValidatedPayloadParams{
		ID:               run.DashboardRunID,
		SellerAccountID:  sellerID,
		ValidatedPayload: []byte(`{"summary":"mutated"}`),
		Column4:          []byte(`[]`),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("dashboard payload mutation must be blocked, got: %v", err)
	}

	if _, err := h.Queries.UpdateActionGenerationRun(ctx, dbgen.UpdateActionGenerationRunParams{
		ID:                   run.ActionsRunID.Int64,
		Status:               "completed",
		ContextHash:          pgtype.Text{String: "mutated", Valid: true},
		FinalStatus:          pgtype.Text{String: "completed", Valid: true},
		AcceptedCount:        1,
		RepairedCount:        0,
		DroppedCount:         0,
		FallbackCount:        0,
		AiFocusJson:          []byte(`{}`),
		TopActionKeysJson:    []byte(`[]`),
		DataQualityNotesJson: []byte(`[]`),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("actions run mutation must be blocked, got: %v", err)
	}
}

func publishIgnoreStale(ctx context.Context, publisher *aipipeline.SQLCSnapshotPublisher, runID int64) error {
	_, err := publisher.PublishPipelineSnapshot(ctx, runID)
	if errors.Is(err, aipipeline.ErrStalePipelinePublication) {
		return nil
	}
	return err
}

func createPublishablePipelineRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, q *dbgen.Queries, sellerID int64, metricDate, createdAt time.Time) int64 {
	t.Helper()
	run, err := q.CreateAIPipelineRun(ctx, dbgen.CreateAIPipelineRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  "unified",
		Trigger:               "integration",
		Status:                "completed",
		DataPackSchemaVersion: pgtype.Text{String: "v1", Valid: true},
		DataPackHash:          "sha256:test-hash-" + createdAt.Format("150405.000"),
		StartedAt:             pgtype.Timestamptz{Time: createdAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("create pipeline run: %v", err)
	}
	globalHash := "gh-" + createdAt.Format("150405.000")
	dashboardRow, err := q.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		Status:          "completed",
		HealthScore:     pgtype.Int4{Int32: 70, Valid: true},
		Severity:        pgtype.Text{String: "medium", Valid: true},
		RiskDriversHash: "rdh",
		ContextHash:     "ctx",
		PromptVersion:   "v9",
		Model:           pgtype.Text{String: "fake", Valid: true},
		StartedAt:       pgtype.Timestamptz{Time: createdAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("insert dashboard narrative: %v", err)
	}
	_, err = q.UpdateAIDashboardNarrativePipelineFields(ctx, dbgen.UpdateAIDashboardNarrativePipelineFieldsParams{
		ID:              dashboardRow.ID,
		SellerAccountID: sellerID,
		AiPipelineRunID: pgtype.Int8{Int64: run.ID, Valid: true},
		DataPackHash:    pgtype.Text{String: run.DataPackHash, Valid: true},
		GlobalContextHash: pgtype.Text{
			String: globalHash,
			Valid:  true,
		},
	})
	if err != nil {
		t.Fatalf("link dashboard narrative: %v", err)
	}
	dashSnap, err := q.InsertAIDashboardDataSnapshot(ctx, dbgen.InsertAIDashboardDataSnapshotParams{
		AiPipelineRunID:   run.ID,
		SellerAccountID:   sellerID,
		MetricDate:        pgtype.Date{Time: metricDate, Valid: true},
		DataPackHash:      run.DataPackHash,
		GlobalContextHash: globalHash,
		SchemaVersion:     "dashboard_data_snapshot_v1",
		PayloadJson:       []byte(`{"schema_version":"dashboard_data_snapshot_v1","metric_date":"` + metricDate.Format("2006-01-02") + `","dashboard_metrics":{"seller_account_id":` + fmt.Sprintf("%d", sellerID) + `,"as_of_date":"` + metricDate.Format("2006-01-02") + `","account":{"revenue_today":100,"orders_today":5},"skus":[]}}`),
		PayloadHash:       "sha256:test-payload-" + createdAt.Format("150405.000"),
		Status:            "completed",
		FinishedAt:        pgtype.Timestamptz{Time: createdAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("insert dashboard data snapshot: %v", err)
	}
	var actionsRunID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (
	seller_account_id, metric_date, status, final_status, ai_pipeline_run_id, data_pack_hash, global_context_hash, created_at, updated_at
) VALUES ($1, $2, 'completed', 'completed', $3, $4, $5, $6, $6)
RETURNING id
`, sellerID, pgtype.Date{Time: metricDate, Valid: true}, run.ID, run.DataPackHash, globalHash, createdAt).Scan(&actionsRunID); err != nil {
		t.Fatalf("insert action_generation_run: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_entity_groups (
	seller_account_id, metric_date, group_key, entity_type, entity_ids_json, summary_json, action_generation_run_id
) VALUES ($1, $2, 'g1', 'sku', '["sku-1"]'::jsonb, '{}'::jsonb, $3)
`, sellerID, pgtype.Date{Time: metricDate, Valid: true}, actionsRunID); err != nil {
		t.Fatalf("insert action entity group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (
	seller_account_id, run_id, action_key, type, category, priority, title, summary, reason, next_step, expected_effect,
	evidence_json, limitations_json, cta_json, affected_entities_json, entity_group_ref, source_candidate_ids_json, evidence_fact_ids_json, payload_json, metric_date
) VALUES (
	$1, $2, $3, 'inventory', 'stock', 'important', 'Title', 'Summary', 'Reason', 'Next', 'Effect',
	'[]'::jsonb, '[]'::jsonb, '[]'::jsonb, '{}'::jsonb, 'g1', '[]'::jsonb, '[]'::jsonb, '{}'::jsonb, $4
)`, sellerID, actionsRunID, "action:"+createdAt.Format("150405.000"), pgtype.Date{Time: metricDate, Valid: true}); err != nil {
		t.Fatalf("insert action item: %v", err)
	}
	_, err = q.UpdateAIPipelineRun(ctx, dbgen.UpdateAIPipelineRunParams{
		ID:                      run.ID,
		DashboardRunID:          dashboardRow.ID,
		ActionsRunID:            pgtype.Int8{Int64: actionsRunID, Valid: true},
		DashboardDataSnapshotID: pgtype.Int8{Int64: dashSnap.ID, Valid: true},
		Status:                  pgtype.Text{String: "completed", Valid: true},
		CompletedAt:             pgtype.Timestamptz{Time: createdAt, Valid: true},
		GlobalContextHash: pgtype.Text{
			String: globalHash,
			Valid:  true,
		},
	})
	if err != nil {
		t.Fatalf("update pipeline run links: %v", err)
	}
	return run.ID
}
