//go:build integration

package runtimeintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type snapshotFingerprint struct {
	PipelineRunID    int64
	PublishedAt      time.Time
	DashboardPayload []byte
	DashboardHash    string
	ActionsRunID     int64
	ActionTitle      string
}

func TestPreviousSnapshotSurvival_DashboardFailure(t *testing.T) {
	runSurvivalScenario(t, survivalScenario{
		name:      "dashboard_failure",
		dashFail:  true,
		expectPub: false,
	})
}

func TestPreviousSnapshotSurvival_ActionsFailure(t *testing.T) {
	runSurvivalScenario(t, survivalScenario{
		name:      "actions_failure",
		actFail:   true,
		expectPub: false,
	})
}

func TestPreviousSnapshotSurvival_HashMismatchBlocksPublication(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, ctx, h, sellerID, p1)

	p2 := createCompletedPipelineAttempt(t, ctx, h, sellerID, metricDate, "p2", false, false, false)
	if _, err := h.Pool.Exec(ctx, `UPDATE action_generation_runs SET data_pack_hash = 'sha256:broken' WHERE ai_pipeline_run_id = $1`, p2); err != nil {
		t.Fatalf("break hash: %v", err)
	}
	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	if _, err := publisher.PublishPipelineSnapshot(ctx, p2); err == nil {
		t.Fatal("expected publication rejection")
	}
	assertCurrentPipeline(t, ctx, h, sellerID, p1)
	assertFingerprintUnchanged(t, ctx, h, sellerID, before)
}

func TestPreviousSnapshotSurvival_ProviderFailure(t *testing.T) {
	runSurvivalScenario(t, survivalScenario{
		name:         "provider_failure",
		dataPackFail: true,
		expectPub:    false,
	})
}

func TestPreviousSnapshotSurvival_PublicationFailure(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, ctx, h, sellerID, p1)

	svc := buildSurvivalPipelineService(h, survivalOptions{})
	svc.SetSnapshotPublisher(failingSnapshotPublisher{})
	_, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		ForceRegenerate:   true,
		AIEnabled:         false,
		Trigger:           "integration",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	assertCurrentPipeline(t, ctx, h, sellerID, p1)
	assertFingerprintUnchanged(t, ctx, h, sellerID, before)
}

func TestPreviousSnapshotSurvival_ContextCancellation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	metricDate := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, context.Background(), h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, context.Background(), h, sellerID, p1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := buildSurvivalPipelineService(h, survivalOptions{})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	_, _ = svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		ForceRegenerate:   true,
		AIEnabled:         false,
		Trigger:           "integration",
	})
	assertCurrentPipeline(t, context.Background(), h, sellerID, p1)
	assertFingerprintUnchanged(t, context.Background(), h, sellerID, before)
}

func TestPreviousSnapshotSurvival_ForceRerunFailure(t *testing.T) {
	runSurvivalScenario(t, survivalScenario{
		name:      "force_rerun_failure",
		dashFail:  true,
		force:     true,
		expectPub: false,
	})
}

func TestPreviousSnapshotSurvival_SuccessfulSwitch(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, ctx, h, sellerID, p1)

	svc := buildSurvivalPipelineService(h, survivalOptions{})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		ForceRegenerate:   true,
		AIEnabled:         false,
		Trigger:           "integration",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if !res.Published {
		t.Fatalf("expected published pipeline, status=%s", res.Status)
	}
	assertCurrentPipeline(t, ctx, h, sellerID, res.PipelineRunID)
	assertFingerprintUnchanged(t, ctx, h, sellerID, before)
}

func TestPreviousSnapshotSurvival_EmptyActionsSwitch(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	if countActionItemsByRun(t, ctx, h.Pool, beforeActionsRunID(t, ctx, h, p1)) == 0 {
		t.Fatal("p1 must contain actions")
	}

	svc := buildSurvivalPipelineService(h, survivalOptions{actionsEmpty: true})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		ForceRegenerate:   true,
		AIEnabled:         false,
		Trigger:           "integration",
	})
	if err != nil || !res.Published {
		t.Fatalf("run=%+v err=%v", res, err)
	}
	cur, err := publishedsnapshot.NewSQLCResolver(h.Queries).GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	if cur.PipelineRunID != res.PipelineRunID {
		t.Fatalf("current=%d want %d", cur.PipelineRunID, res.PipelineRunID)
	}
	p2Actions, err := h.Queries.ListActionItemsByRunID(ctx, pgtype.Int8{Int64: cur.ActionsRunID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p2Actions) != 0 {
		t.Fatalf("expected empty actions feed, got %d", len(p2Actions))
	}
	if countActionItemsByRun(t, ctx, h.Pool, beforeActionsRunID(t, ctx, h, p1)) == 0 {
		t.Fatal("historical p1 action rows must remain in database")
	}
}

func TestPreviousSnapshotSurvival_RetryDoesNotOverwritePrevious(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, ctx, h, sellerID, p1)

	svc := buildSurvivalPipelineService(h, survivalOptions{dashboardFail: true})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	if _, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: "attempt-1",
	}); err != nil {
		t.Fatal(err)
	}
	svc2 := buildSurvivalPipelineService(h, survivalOptions{})
	svc2.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc2.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: "attempt-2",
	})
	if err != nil || !res.Published {
		t.Fatalf("second run must publish, res=%+v err=%v", res, err)
	}
	assertFingerprintUnchanged(t, ctx, h, sellerID, before)
	if res.PipelineRunID == p1 {
		t.Fatal("new published pipeline must be a new run")
	}
}

func TestPreviousSnapshotSurvival_PublishedRowsImmutableAfterFailedP2(t *testing.T) {
	runSurvivalScenario(t, survivalScenario{
		name:      "published_rows_immutable",
		dashFail:  true,
		expectPub: false,
		checkP1:   true,
	})
}

func TestPreviousSnapshotSurvival_CleanupProtectsCurrentSnapshot(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	protected, err := publishedsnapshot.LoadCurrentProtectedSnapshotIDs(ctx, h.Queries, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	if !protected.ContainsPipelineRun(p1) {
		t.Fatal("protected set must include current pipeline")
	}
	// No production cleanup job exists yet; verify delete guards block published rows.
	if err := h.Queries.DeleteAIDashboardNarrativesBySellerMetricDate(ctx, dbgen.DeleteAIDashboardNarrativesBySellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
	}); err != nil {
		t.Fatalf("delete attempt: %v", err)
	}
	if _, err := h.Queries.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              survivalParseUUID(protected.DashboardRunID),
		SellerAccountID: sellerID,
	}); err != nil {
		t.Fatalf("published dashboard must remain: %v", err)
	}
}

type survivalScenario struct {
	name         string
	dashFail     bool
	actFail      bool
	dataPackFail bool
	force        bool
	expectPub    bool
	checkP1      bool
}

func runSurvivalScenario(t *testing.T, sc survivalScenario) {
	t.Helper()
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSnapshotFingerprint(t, ctx, h, sellerID, p1)

	opts := survivalOptions{
		dashboardFail: sc.dashFail,
		actionsFail:   sc.actFail,
		dataPackFail:  sc.dataPackFail,
	}
	svc := buildSurvivalPipelineService(h, opts)
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		ForceRegenerate:   sc.force || true,
		AIEnabled:         false,
		Trigger:           "integration",
	})
	if err != nil {
		t.Fatalf("%s: run failed: %v", sc.name, err)
	}
	if res.Published != sc.expectPub {
		t.Fatalf("%s: published=%v want %v status=%s", sc.name, res.Published, sc.expectPub, res.Status)
	}
	assertCurrentPipeline(t, ctx, h, sellerID, p1)
	if sc.checkP1 || true {
		assertFingerprintUnchanged(t, ctx, h, sellerID, before)
	}
}

func publishFixturePipeline(t *testing.T, ctx context.Context, h *Harness, sellerID int64, metricDate time.Time, tag string) int64 {
	t.Helper()
	createdAt := time.Now().UTC()
	runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, metricDate, createdAt)
	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err != nil {
		t.Fatalf("publish %s: %v", tag, err)
	}
	return runID
}

func loadSnapshotFingerprint(t *testing.T, ctx context.Context, h *Harness, sellerID, pipelineID int64) snapshotFingerprint {
	t.Helper()
	run, err := h.Queries.GetAIPipelineRun(ctx, pipelineID)
	if err != nil {
		t.Fatal(err)
	}
	dash, err := h.Queries.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              run.DashboardRunID,
		SellerAccountID: sellerID,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := h.Queries.ListActionItemsByRunID(ctx, run.ActionsRunID)
	if err != nil || len(items) == 0 {
		t.Fatalf("actions items: %v", err)
	}
	return snapshotFingerprint{
		PipelineRunID:    pipelineID,
		PublishedAt:      run.PublishedAt.Time.UTC(),
		DashboardPayload: append([]byte(nil), dash.ValidatedPayload...),
		DashboardHash:    dash.ContextHash,
		ActionsRunID:     run.ActionsRunID.Int64,
		ActionTitle:      items[0].Title,
	}
}

func assertCurrentPipeline(t *testing.T, ctx context.Context, h *Harness, sellerID, wantPipelineID int64) {
	t.Helper()
	cur, err := publishedsnapshot.NewSQLCResolver(h.Queries).GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatalf("current snapshot: %v", err)
	}
	if cur.PipelineRunID != wantPipelineID {
		t.Fatalf("current pipeline=%d want %d", cur.PipelineRunID, wantPipelineID)
	}
}

func assertFingerprintUnchanged(t *testing.T, ctx context.Context, h *Harness, sellerID int64, before snapshotFingerprint) {
	t.Helper()
	after := loadSnapshotFingerprint(t, ctx, h, sellerID, before.PipelineRunID)
	if !after.PublishedAt.Equal(before.PublishedAt) {
		t.Fatalf("published_at changed")
	}
	if string(after.DashboardPayload) != string(before.DashboardPayload) {
		t.Fatalf("dashboard payload changed")
	}
	if after.DashboardHash != before.DashboardHash {
		t.Fatalf("dashboard hash changed")
	}
	if after.ActionsRunID != before.ActionsRunID {
		t.Fatalf("actions run id changed")
	}
	if after.ActionTitle != before.ActionTitle {
		t.Fatalf("action content changed")
	}
	_ = sellerID
}

func beforeActionsRunID(t *testing.T, ctx context.Context, h *Harness, pipelineID int64) int64 {
	t.Helper()
	run, err := h.Queries.GetAIPipelineRun(ctx, pipelineID)
	if err != nil {
		t.Fatal(err)
	}
	return run.ActionsRunID.Int64
}

func countActionItemsByRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, runID int64) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_items WHERE run_id = $1`, runID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

type survivalOptions struct {
	dashboardFail bool
	actionsFail   bool
	actionsEmpty  bool
	dataPackFail  bool
}

func buildSurvivalPipelineService(h *Harness, opts survivalOptions) *aipipeline.SellerAIPipelineService {
	store := aipipeline.NewStore(h.Queries)
	var packBuilder aipipeline.DataPackBuilder = &staticDataPackBuilder{pack: survivalSamplePack(1, h.MetricDate)}
	if opts.dataPackFail {
		packBuilder = &failingDataPackBuilder{err: errors.New("data_pack_build_failed")}
	}
	svc := aipipeline.NewSellerAIPipelineService(
		packBuilder,
		&survivalDashboardRunner{store: store, fail: opts.dashboardFail},
		&survivalActionsRunner{queries: h.Queries, pool: h.Pool, fail: opts.actionsFail, empty: opts.actionsEmpty},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	return svc
}

type staticDataPackBuilder struct {
	pack *aidatapack.SellerAIDataPack
}

func (b *staticDataPackBuilder) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error) {
	if b.pack == nil {
		return nil, fmt.Errorf("missing pack")
	}
	out := *b.pack
	out.SellerContext.SellerAccountID = sellerAccountID
	out.SellerContext.MetricDate = metricDate.Format("2006-01-02")
	return &out, nil
}

type failingDataPackBuilder struct{ err error }

func (b *failingDataPackBuilder) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error) {
	return nil, b.err
}

type survivalDashboardRunner struct {
	store     *aipipeline.Store
	fail      bool
	failFirst bool
	failed    bool
}

func (r *survivalDashboardRunner) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
) (*aipipeline.DashboardRunOutcome, error) {
	if r.fail || (r.failFirst && !r.failed) {
		r.failed = true
		return &aipipeline.DashboardRunOutcome{
			Result: &dashboardhome.NarrativeGenerationResult{Status: "failed", ErrorMessage: "dashboard_failed"},
		}, nil
	}
	runner := aipipeline.NewFakeDashboardRunner(r.store)
	if opts.DataPackOverride == nil {
		opts.DataPackOverride = survivalSamplePack(sellerAccountID, metricDate)
	}
	if opts.GlobalContext == nil {
		opts.GlobalContext = &aicontext.GlobalSemanticContext{SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion}
	}
	return runner.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, metricDate, opts)
}

type survivalActionsRunner struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
	fail    bool
	empty   bool
}

func (r *survivalActionsRunner) BuildActionsAIFirstInputFromDataPack(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (r *survivalActionsRunner) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromCandidates(
		pack, dataPackHash, globalContext, globalContextHash, candidates, failedChunkIDs, dataLimitations, opts,
	)
}

func (r *survivalActionsRunner) GenerateActionsAIFirstFromInput(
	ctx context.Context,
	input actions.ActionsAIFirstInput,
	opts actions.GenerateActionsOptions,
) (*actions.GenerateActionsResult, error) {
	metricDate, _ := time.Parse("2006-01-02", input.MetricDate)
	repo := actions.NewSQLCGenerationRepository(r.pool, r.queries)
	runID, err := repo.CreateActionGenerationRun(ctx, input.SellerAccountID, metricDate, actions.GenerationRunStatusRunning)
	if err != nil {
		return nil, err
	}
	if opts.PipelineRunID != nil {
		_ = repo.LinkActionGenerationRunToPipeline(ctx, *opts.PipelineRunID, runID, input.DataPackHash, opts.GlobalContextHash)
	}
	finalStatus := actions.GenerationFinalStatusCompleted
	if r.fail {
		finalStatus = actions.GenerationFinalStatusFailed
	}
	if r.empty {
		finalStatus = actions.GenerationFinalStatusEmpty
	}
	_ = repo.UpdateActionGenerationRun(ctx, actions.UpdateGenerationRunParams{
		RunID: runID, Status: actions.GenerationRunStatusCompleted, FinalStatus: finalStatus,
	})
	if finalStatus == actions.GenerationFinalStatusCompleted {
		evidence, _ := json.Marshal([]actions.ActionEvidence{})
		ctas, _ := json.Marshal([]actions.ActionCTA{})
		_ = repo.UpsertActionItem(ctx, actions.UpsertActionItemParams{
			SellerAccountID: input.SellerAccountID, RunID: runID, ActionKey: "action:survival",
			Type: actions.ActionTypeStockReplenishment, Category: actions.ActionCategoryStock,
			Priority: actions.ActionPriorityImportant, Title: "Survival action", Reason: "r", NextStep: "n",
			EvidenceJSON: evidence, CTAJSON: ctas, LimitationsJSON: []byte("[]"),
			AffectedEntitiesJSON: []byte("{}"), SourceCandidateIDs: []byte("[]"),
			EvidenceFactIDs: []byte("[]"), PayloadJSON: []byte("{}"), MetricDate: metricDate,
		})
	}
	return &actions.GenerateActionsResult{RunID: runID, FinalStatus: finalStatus}, nil
}

func survivalSamplePack(sellerID int64, metricDate time.Time) *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: sellerID, MetricDate: metricDate.Format("2006-01-02")},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
			"campaign.1.last_7d.drr":      {FactID: "campaign.1.last_7d.drr", FormattedValue: "12%"},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			Campaigns: map[string]aidatapack.CampaignEntity{"campaign:1": {EntityID: "campaign:1", Name: "Test"}},
			SKUs:      map[string]aidatapack.SKUEntity{"sku:1": {EntityID: "sku:1", SKU: "1", Name: "SKU"}},
		},
		Tables: aidatapack.Tables{
			CampaignRollups: []aidatapack.CampaignRollupRow{{EntityID: "campaign:1", Name: "Test"}},
		},
	}
}

func createCompletedPipelineAttempt(
	t *testing.T,
	ctx context.Context,
	h *Harness,
	sellerID int64,
	metricDate time.Time,
	tag string,
	dashFail, actFail, actEmpty bool,
) int64 {
	t.Helper()
	svc := buildSurvivalPipelineService(h, survivalOptions{dashboardFail: dashFail, actionsFail: actFail, actionsEmpty: actEmpty})
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: tag,
	})
	if err != nil {
		t.Fatalf("create attempt %s: %v", tag, err)
	}
	return res.PipelineRunID
}

type failingSnapshotPublisher struct{}

func (failingSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	return aipipeline.PublishResult{}, errors.New("publication_failed")
}

func survivalParseUUID(raw string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(raw)
	return id
}
