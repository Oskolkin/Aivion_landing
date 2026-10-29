//go:build integration

package publication_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run publication integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

func createSeller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	email := fmt.Sprintf("ua-pub-%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status) VALUES ($1, $2, 'active') RETURNING id`,
		userID, "ua-publication").Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "contract", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func fixtureCanonical(t *testing.T, name string) (canonical []byte, hash string, out *contract.UnifiedAnalysisOutputV1) {
	t.Helper()
	raw := readFixture(t, name)
	var parsed contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	can, err := outputvalidator.CanonicalJSON(&parsed)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	return can, outputvalidator.OutputHash(can), &parsed
}

type seededRun struct {
	SellerID   int64
	RunID      int64
	MetricDate time.Time
	OutputHash string
	PackID     string
	PackHash   string
}

func seedPublicationReadyRun(t *testing.T, pool *pgxpool.Pool, sellerID int64, metricDate time.Time, fixtureName string) seededRun {
	t.Helper()
	ctx := context.Background()
	q := dbgen.New(pool)
	policy := publication.DefaultPublicationPolicy()
	canonical, outputHash, parsed := fixtureCanonical(t, fixtureName)

	packID := fmt.Sprintf("pack:%d:%s:test", sellerID, metricDate.Format("2006-01-02"))
	packHash := "testhash0123456789abcdef0123456789abcdef0123456789abcdef01234567"
	parsed.AnalysisMeta.PackID = packID
	parsed.AnalysisMeta.PackHash = packHash
	parsed.AnalysisMeta.MetricDate = metricDate.Format("2006-01-02")
	canonical, err := outputvalidator.CanonicalJSON(parsed)
	if err != nil {
		t.Fatalf("re-canonical: %v", err)
	}
	outputHash = outputvalidator.OutputHash(canonical)

	run, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               "test",
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: pgtype.Text{String: contract.AnalysisPackSchemaVersionV1, Valid: true},
		DataPackHash:          packHash,
		PipelineVersion:       pgtype.Text{String: "test", Valid: true},
		PromptVersion:         pgtype.Text{String: policy.AcceptedPromptVersion, Valid: true},
		Provider:              pgtype.Text{String: "openai", Valid: true},
		ProviderModel:         pgtype.Text{String: "gpt-test", Valid: true},
		ProviderAttemptCount:  1,
		StartedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateUnifiedAnalysisRun: %v", err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_runs SET
  pack_id=$2, pack_hash=$3, code_interpreter_calls=1,
  provider_status='completed', cleanup_status='completed'
WHERE id=$1`, run.ID, packID, packHash); err != nil {
		t.Fatalf("stamp pack fields: %v", err)
	}

	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		ObjectKey:     pgtype.Text{},
		PayloadJson:   canonical,
	}); err != nil {
		t.Fatalf("validated artifact: %v", err)
	}

	validationPayload, _ := json.Marshal(map[string]any{
		"prompt_version": policy.AcceptedPromptVersion,
		"prompt_hash":    policy.AcceptedPromptHash,
		"schema_version": policy.AcceptedSchemaVersion,
		"schema_hash":    policy.AcceptedSchemaHash,
		"output_hash":    outputHash,
		"valid":          true,
	})
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation,
		Hash:          pgtype.Text{String: "validation", Valid: true},
		Storage:       "db",
		PayloadJson:   validationPayload,
	}); err != nil {
		t.Fatalf("validation artifact: %v", err)
	}

	summaryPayload, _ := json.Marshal(map[string]any{
		"outcome":          parsed.AnalysisOutcome,
		"action_count":     len(parsed.Actions),
		"top_action_count": len(parsed.Dashboard.TopActionIDs),
		"output_hash":      outputHash,
	})
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputSummary,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   summaryPayload,
	}); err != nil {
		t.Fatalf("summary artifact: %v", err)
	}

	// PublishRun now requires a matching unified_dashboard_facts_v1 artifact
	// (facts are the sole source of truth for factual dashboard fields; see
	// docs/current/dashboard-field-ownership.md), so seed one derived from
	// the same fixture used for the AI output artifact above, mirroring
	// whatever the fixture reports as available so
	// bundlevalidator.ValidatePublicationBundle accepts it.
	artifacts := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(q))
	facts := buildTestDashboardFacts(sellerID, metricDate, packHash, parsed)
	if err := artifacts.SaveDashboardFacts(ctx, run.ID, facts); err != nil {
		t.Fatalf("save dashboard facts artifact: %v", err)
	}

	return seededRun{
		SellerID:   sellerID,
		RunID:      run.ID,
		MetricDate: metricDate,
		OutputHash: outputHash,
		PackID:     packID,
		PackHash:   packHash,
	}
}

// buildTestDashboardFacts derives a minimal-but-consistent
// dashboardfacts.DashboardFactsV1 fixture from a parsed AI output fixture,
// so integration tests exercise PublishRun's facts-required path without
// having to hand-author a separate facts fixture per AI output fixture.
// Coverage flags mirror whichever blocks the AI fixture itself reports as
// available, and the factual content is copied 1:1 from the AI output so
// bundlevalidator.ValidatePublicationBundle's coverage-aware rules pass.
func buildTestDashboardFacts(sellerID int64, metricDate time.Time, packHash string, out *contract.UnifiedAnalysisOutputV1) dashboardfacts.DashboardFactsV1 {
	dash := out.Dashboard
	facts := dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: sellerID,
		MetricDate:      metricDate.Format("2006-01-02"),
		PackID:          out.AnalysisMeta.PackID,
		PackHash:        packHash,
		SKUFactsIndex:   dashboardfacts.SKUFactsIndex{ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{}},
	}

	for _, kpi := range dash.SellerPulse.KPIs {
		facts.SellerPulse.KPIs = append(facts.SellerPulse.KPIs, dashboardfacts.KPICardFact{
			Key: kpi.Key, Label: kpi.Label, Value: kpi.Value, Hint: kpi.Hint,
		})
	}
	for _, p := range dash.SellerPulse.Trend7D {
		facts.SellerPulse.Trend7D = append(facts.SellerPulse.Trend7D, dashboardfacts.TrendDayPoint{
			Date: p.Date, Revenue: p.Revenue, Orders: p.Orders,
		})
	}
	if len(dash.SellerPulse.KPIs) > 0 {
		facts.DataCoverage.AccountMetricsDays = 7
	}

	facts.StockCoverage.Status = dash.StockCoverage.Status
	facts.StockCoverage.TotalSKUs = dash.StockCoverage.TotalSKUs
	for _, s := range dash.StockCoverage.Segments {
		facts.StockCoverage.Segments = append(facts.StockCoverage.Segments, dashboardfacts.StockCoverageSegment{
			Key: s.Key, Label: s.Label, SKUCount: s.SKUCount, Severity: s.Severity,
		})
	}
	if len(dash.StockCoverage.Segments) > 0 {
		facts.DataCoverage.StockSnapshotRows = 10
	}

	facts.AdEfficiency.Status = dash.AdEfficiency.Status
	if dash.AdEfficiency.Summary != nil {
		facts.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
			CurrentDRRPct:          dash.AdEfficiency.Summary.CurrentDRRPct,
			PreviousDRRPct:         dash.AdEfficiency.Summary.PreviousDRRPct,
			AverageDRR7dPct:        dash.AdEfficiency.Summary.AverageDRR7dPct,
			TotalSpendToday:        dash.AdEfficiency.Summary.TotalSpendToday,
			TotalRevenueToday:      dash.AdEfficiency.Summary.TotalRevenueToday,
			TotalOrdersToday:       dash.AdEfficiency.Summary.TotalOrdersToday,
			CampaignsHighDRRCount:  dash.AdEfficiency.Summary.CampaignsHighDRRCount,
			CampaignsNoOrdersCount: dash.AdEfficiency.Summary.CampaignsNoOrdersCount,
			HighDRRSpend:           dash.AdEfficiency.Summary.HighDRRSpend,
			SpendWithoutOrders:     dash.AdEfficiency.Summary.SpendWithoutOrders,
		}
		facts.DataCoverage.AdMetricsDays = 7
	}

	facts.Funnel.Status = dash.SalesFunnel.Status
	for _, s := range dash.SalesFunnel.Stages {
		facts.Funnel.Stages = append(facts.Funnel.Stages, dashboardfacts.FunnelStage{
			Key: s.Key, Label: s.Label, Value: s.Value,
			ConversionFromPrevious: s.ConversionFromPrevious, ConversionFromTop: s.ConversionFromTop,
			PreviousDayValue: s.PreviousDayValue, ChangeVsPreviousDayPct: s.ChangeVsPreviousDayPct,
		})
	}
	if len(dash.SalesFunnel.Stages) > 0 {
		facts.DataCoverage.FunnelAvailable = true
	}

	nextProductID := int64(1)
	addSKU := func(sku *int64, offerID *string, productName string) {
		rec := dashboardfacts.SKUFactRecord{
			OzonProductID:  nextProductID,
			SKU:            sku,
			OfferID:        offerID,
			ProductName:    &productName,
			Revenue:        1000,
			Revenue7d:      1000,
			Orders:         1,
			Orders7d:       1,
			StockAvailable: 10,
		}
		facts.SKUFactsIndex.ByOzonProductID[fmt.Sprintf("%d", nextProductID)] = rec
		nextProductID++
	}
	for _, c := range dash.SKUFocus.OrderDrop {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.StockRisk {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.NoSalesWithStock {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.SlowTurnover {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.KeySKUs {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	// Cohort membership owns KeySKU projection; seed Revenue7d above so healthy
	// accounts (key_skus only, empty risk tabs) still project sku_focus entries.
	facts.KeySKUCohort = dashboardfacts.BuildKeySKUCohortFacts(facts.SKUFactsIndex)
	if len(facts.SKUFactsIndex.ByOzonProductID) > 0 {
		facts.DataCoverage.SKUMetricsDays = 7
	}

	return facts
}

func setSellerMode(t *testing.T, q *dbgen.Queries, sellerID int64, mode string) {
	t.Helper()
	_, err := q.UpsertSellerAnalysisSettings(context.Background(), dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          mode,
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
		UpdatedBy:             "test",
	})
	if err != nil {
		t.Fatalf("UpsertSellerAnalysisSettings: %v", err)
	}
}

func newPubService(pool *pgxpool.Pool) *publication.Service {
	q := dbgen.New(pool)
	artifacts := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(q))
	return publication.NewService(pool, q, artifacts, publication.DefaultPublicationPolicy(), publication.GlobalFlags{
		RolloutEnabled:        true,
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	})
}

func currentPointers(t *testing.T, pool *pgxpool.Pool, sellerID int64) (unifiedPubID, compatRunID int64, hasUnified, hasCompat bool) {
	t.Helper()
	ctx := context.Background()
	err := pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_unified_publications WHERE seller_account_id=$1`, sellerID).Scan(&unifiedPubID)
	if err == nil {
		hasUnified = true
	} else if err != pgx.ErrNoRows {
		t.Fatalf("unified pointer: %v", err)
	}
	err = pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRunID)
	if err == nil {
		hasCompat = true
	} else if err != pgx.ErrNoRows {
		t.Fatalf("compat pointer: %v", err)
	}
	return
}

func assertPointersMatch(t *testing.T, pool *pgxpool.Pool, sellerID, wantRunID, wantPubID int64) {
	t.Helper()
	unifiedPubID, compatRunID, hasU, hasC := currentPointers(t, pool, sellerID)
	if !hasU || !hasC {
		t.Fatalf("missing pointers unified=%v compat=%v", hasU, hasC)
	}
	if wantPubID > 0 && unifiedPubID != wantPubID {
		t.Fatalf("unified pointer pub=%d want %d", unifiedPubID, wantPubID)
	}
	if compatRunID != wantRunID {
		t.Fatalf("compat pointer run=%d want %d", compatRunID, wantRunID)
	}
	var pubRunID int64
	if err := pool.QueryRow(context.Background(), `SELECT run_id FROM unified_analysis_publications WHERE id=$1`, unifiedPubID).Scan(&pubRunID); err != nil {
		t.Fatalf("load publication run: %v", err)
	}
	if pubRunID != wantRunID || pubRunID != compatRunID {
		t.Fatalf("pointer invariant broken: unified.pub.run=%d compat=%d want=%d", pubRunID, compatRunID, wantRunID)
	}
}

func TestPublishVisiblePointersAtomic(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "unified")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	svc := newPubService(pool)

	res, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("PublishRun: %v", err)
	}
	if res.Status != publication.StatusPublished {
		t.Fatalf("status=%s", res.Status)
	}
	assertPointersMatch(t, pool, sellerID, seed.RunID, res.PublicationID)
}

func TestPublishShadowPointersUnchanged(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "shadow")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	svc := newPubService(pool)

	res, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("PublishRun: %v", err)
	}
	if res.Visibility != publication.VisibilityShadow {
		t.Fatalf("visibility=%s", res.Visibility)
	}
	_, _, hasU, hasC := currentPointers(t, pool, sellerID)
	if hasU || hasC {
		t.Fatalf("shadow must not switch pointers unified=%v compat=%v", hasU, hasC)
	}
}

func TestFailureInjectionPoints(t *testing.T) {
	points := []publication.FailurePoint{
		publication.FailAfterPublicationCreated,
		publication.FailAfterDashboardInserted,
		publication.FailAfterFirstActionInserted,
		publication.FailAfterAllActionsInserted,
		publication.FailAfterActionMapInserted,
		publication.FailAfterRunPublishing,
		publication.FailAfterCompatPointer,
		publication.FailAfterUnifiedPointer,
		publication.FailBeforeAudit,
		publication.FailBeforeCommit,
	}
	pool := openPool(t)
	for _, point := range points {
		point := point
		t.Run(string(point), func(t *testing.T) {
			sellerID := createSeller(t, pool)
			q := dbgen.New(pool)
			setSellerMode(t, q, sellerID, "unified")
			seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
			base := newPubService(pool)

			_, err := base.WithTestFailurePoint(point).PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"})
			if err == nil {
				t.Fatal("expected injected failure")
			}
			_, _, hasU, hasC := currentPointers(t, pool, sellerID)
			if hasU || hasC {
				t.Fatalf("pointers must be unchanged after failure unified=%v compat=%v", hasU, hasC)
			}
			var runStatus string
			if err := pool.QueryRow(context.Background(), `SELECT status FROM ai_pipeline_runs WHERE id=$1`, seed.RunID).Scan(&runStatus); err != nil {
				t.Fatalf("run status: %v", err)
			}
			if runStatus == unifiedanalysis.StatusPublished {
				t.Fatal("run must not be published after injected failure")
			}
			// After rollback of TX that set publishing, run should be back to publication_ready
			// (transition is inside TX). Allow publishing only if somehow outside — expect publication_ready.
			if runStatus != unifiedanalysis.StatusPublicationReady && runStatus != unifiedanalysis.StatusPublishing {
				t.Fatalf("run status=%q want publication_ready (or publishing if two-phase)", runStatus)
			}

			res, err := base.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test-retry"})
			if err != nil {
				t.Fatalf("retry PublishRun: %v", err)
			}
			if res.Status != publication.StatusPublished {
				t.Fatalf("retry status=%s", res.Status)
			}
			assertPointersMatch(t, pool, sellerID, seed.RunID, res.PublicationID)
		})
	}
}

func TestConcurrentSameRunPublish(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "unified")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	svc := newPubService(pool)

	var wg sync.WaitGroup
	results := make([]*publication.PublishResult, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "concurrent"})
		}(i)
	}
	wg.Wait()

	okCount := 0
	var pubID int64
	var bundle string
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("goroutine %d nil result", i)
		}
		okCount++
		if pubID == 0 {
			pubID = results[i].PublicationID
			bundle = results[i].BundleHash
		} else if results[i].PublicationID != pubID {
			t.Fatalf("different publication ids %d vs %d", pubID, results[i].PublicationID)
		} else if results[i].BundleHash != bundle {
			t.Fatalf("bundle hash mismatch")
		}
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM unified_analysis_publications WHERE run_id=$1`, seed.RunID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("publication count=%d want 1", n)
	}
	assertPointersMatch(t, pool, sellerID, seed.RunID, pubID)
}

func TestStaleOlderMetricDate(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "unified")
	svc := newPubService(pool)

	newer := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC), "unified_output_v1.no_actions.json")
	resNew, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: newer.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("publish newer: %v", err)
	}
	if resNew.Outcome != publication.OutcomePublished {
		t.Fatalf("newer outcome=%s want published (code=%s)", resNew.Outcome, resNew.ErrorCode)
	}
	older := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	resOld, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: older.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("publish older: %v", err)
	}
	if resOld.Outcome != publication.OutcomeStale {
		t.Fatalf("outcome=%s want stale", resOld.Outcome)
	}
	assertPointersMatch(t, pool, sellerID, newer.RunID, resNew.PublicationID)
}

func TestSameSellerOutcomeSequence(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "unified")
	svc := newPubService(pool)
	d1 := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	mat := seedPublicationReadyRun(t, pool, sellerID, d1, "unified_output_v1.valid.json")
	r1, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: mat.RunID, Actor: "seq"})
	if err != nil {
		t.Fatalf("material: %v", err)
	}
	if r1.ActionsRecordCount != 5 {
		t.Fatalf("material actions=%d want 5 (fixture unified_output_v1.valid.json)", r1.ActionsRecordCount)
	}
	assertPointersMatch(t, pool, sellerID, mat.RunID, r1.PublicationID)

	healthy := seedPublicationReadyRun(t, pool, sellerID, d2, "unified_output_v1.no_actions.json")
	r2, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: healthy.RunID, Actor: "seq"})
	if err != nil {
		t.Fatalf("healthy: %v", err)
	}
	if r2.Outcome != publication.OutcomePublished {
		t.Fatalf("healthy outcome=%s want published (code=%s)", r2.Outcome, r2.ErrorCode)
	}
	if r2.ActionsRecordCount != 0 {
		t.Fatalf("healthy actions=%d", r2.ActionsRecordCount)
	}
	assertPointersMatch(t, pool, sellerID, healthy.RunID, r2.PublicationID)

	insuff := seedPublicationReadyRun(t, pool, sellerID, d3, "unified_output_v1.insufficient_data.json")
	r3, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: insuff.RunID, Actor: "seq"})
	if err != nil {
		t.Fatalf("insufficient: %v", err)
	}
	if r3.ActionsRecordCount != 0 {
		t.Fatalf("insufficient actions=%d", r3.ActionsRecordCount)
	}
	assertPointersMatch(t, pool, sellerID, insuff.RunID, r3.PublicationID)

	rb1, err := svc.RollbackPublication(context.Background(), publication.RollbackRequest{
		SellerAccountID: sellerID, TargetPublicationID: r1.PublicationID, Actor: "seq", Reason: "back to material",
	})
	if err != nil {
		t.Fatalf("rollback material: %v", err)
	}
	if rb1.PublicationID != r1.PublicationID {
		t.Fatalf("rollback target=%d", rb1.PublicationID)
	}
	assertPointersMatch(t, pool, sellerID, mat.RunID, r1.PublicationID)

	rb2, err := svc.RollbackPublication(context.Background(), publication.RollbackRequest{
		SellerAccountID: sellerID, TargetPublicationID: r3.PublicationID, Actor: "seq", Reason: "back to insufficient",
	})
	if err != nil {
		t.Fatalf("rollback insufficient: %v", err)
	}
	assertPointersMatch(t, pool, sellerID, insuff.RunID, rb2.PublicationID)
}

func TestPromoteAndRollbackPointers(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "shadow")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	svc := newPubService(pool)

	res, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("shadow publish: %v", err)
	}
	setSellerMode(t, q, sellerID, "unified")
	promoted, err := svc.PromoteShadowPublication(context.Background(), publication.PromoteRequest{
		PublicationID: res.PublicationID, Actor: "test",
	})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	assertPointersMatch(t, pool, sellerID, seed.RunID, promoted.PublicationID)
}

func TestReconcileRepairsCompatPointer(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerID, "unified")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	svc := newPubService(pool)
	res, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Corrupt compat pointer to a nonexistent-but-different run id by writing a fake row.
	// Use the same seller's run with a bogus pipeline id via direct update after inserting a dummy pipeline isn't easy;
	// instead set pipeline_run_id to an older legacy-looking value by creating another run and pointing there.
	other := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC), "unified_output_v1.no_actions.json")
	if err := q.UpsertSellerCurrentAISnapshot(context.Background(), dbgen.UpsertSellerCurrentAISnapshotParams{
		SellerAccountID: sellerID,
		PipelineRunID:   other.RunID,
	}); err != nil {
		t.Fatalf("corrupt compat: %v", err)
	}

	dry, err := svc.ReconcileCurrentPublicationPointers(context.Background(), sellerID, true)
	if err != nil {
		t.Fatalf("dry reconcile: %v", err)
	}
	if !dry.Mismatched || dry.Repaired {
		t.Fatalf("dry: mismatched=%v repaired=%v", dry.Mismatched, dry.Repaired)
	}
	live, err := svc.ReconcileCurrentPublicationPointers(context.Background(), sellerID, false)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !live.Repaired {
		t.Fatal("expected repair")
	}
	assertPointersMatch(t, pool, sellerID, seed.RunID, res.PublicationID)
}

func TestTenantIsolationRollbackAndPromote(t *testing.T) {
	pool := openPool(t)
	sellerA := createSeller(t, pool)
	sellerB := createSeller(t, pool)
	q := dbgen.New(pool)
	setSellerMode(t, q, sellerA, "unified")
	setSellerMode(t, q, sellerB, "unified")
	svc := newPubService(pool)

	seedA := seedPublicationReadyRun(t, pool, sellerA, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	seedB := seedPublicationReadyRun(t, pool, sellerB, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.no_actions.json")
	resA, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seedA.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("publish A: %v", err)
	}
	resB, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seedB.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("publish B: %v", err)
	}

	_, err = svc.RollbackPublication(context.Background(), publication.RollbackRequest{
		SellerAccountID: sellerA, TargetPublicationID: resB.PublicationID, Actor: "evil",
	})
	if err == nil {
		t.Fatal("seller A must not rollback to seller B publication")
	}

	// Shadow publish for B then try promote from A context is N/A; promote by id belonging to B from A seller settings:
	setSellerMode(t, q, sellerB, "shadow")
	seedB2 := seedPublicationReadyRun(t, pool, sellerB, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC), "unified_output_v1.insufficient_data.json")
	shadow, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seedB2.RunID, Actor: "test"})
	if err != nil {
		t.Fatalf("shadow B: %v", err)
	}
	// Promote is by publication id (admin/devtools); ownership is enforced against publication.seller_account_id.
	// Ensure ResolveCurrentAnalysisSource for A still points at A's run.
	src, err := svc.ResolveCurrentAnalysisSource(context.Background(), sellerA)
	if err != nil {
		t.Fatalf("resolve A: %v", err)
	}
	if src.Source != publication.SourceUnified || src.PipelineRunID != seedA.RunID {
		t.Fatalf("seller A source leaked: %+v", src)
	}
	srcB, err := svc.ResolveCurrentAnalysisSource(context.Background(), sellerB)
	if err != nil {
		t.Fatalf("resolve B: %v", err)
	}
	// B is shadow mode → must not surface unified even with shadow publication present.
	if srcB.Source == publication.SourceUnified {
		t.Fatalf("shadow seller B must not resolve unified: %+v", srcB)
	}
	_ = resA
	_ = shadow
}

func TestSelectorModes(t *testing.T) {
	pool := openPool(t)
	sellerID := createSeller(t, pool)
	q := dbgen.New(pool)
	svc := newPubService(pool)

	setSellerMode(t, q, sellerID, "legacy")
	src, err := svc.ResolveCurrentAnalysisSource(context.Background(), sellerID)
	if err != nil {
		t.Fatalf("legacy resolve: %v", err)
	}
	if src.Source != publication.SourceLegacy && src.Source != publication.SourceNone {
		t.Fatalf("legacy source=%s", src.Source)
	}

	setSellerMode(t, q, sellerID, "shadow")
	seed := seedPublicationReadyRun(t, pool, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "unified_output_v1.valid.json")
	if _, err := svc.PublishRun(context.Background(), publication.PublishRequest{RunID: seed.RunID, Actor: "test"}); err != nil {
		t.Fatalf("shadow publish: %v", err)
	}
	src, err = svc.ResolveCurrentAnalysisSource(context.Background(), sellerID)
	if err != nil {
		t.Fatalf("shadow resolve: %v", err)
	}
	if src.Source == publication.SourceUnified {
		t.Fatal("shadow must not resolve to unified")
	}

	setSellerMode(t, q, sellerID, "unified")
	vis := publication.VisibilityVisible
	if _, err := svc.PublishRun(context.Background(), publication.PublishRequest{
		RunID: seed.RunID, Actor: "test", ForceVisibility: &vis,
	}); err != nil {
		t.Fatalf("promote via force: %v", err)
	}
	src, err = svc.ResolveCurrentAnalysisSource(context.Background(), sellerID)
	if err != nil {
		t.Fatalf("unified resolve: %v", err)
	}
	if src.Source != publication.SourceUnified || src.PipelineRunID != seed.RunID {
		t.Fatalf("unified source=%s run=%d", src.Source, src.PipelineRunID)
	}
}
