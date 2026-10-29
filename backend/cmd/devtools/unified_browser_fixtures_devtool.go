package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// runPrepareUnifiedBrowserScenario seeds (if needed) material / healthy /
// insufficient Unified publications on one seller and switches the current
// visible pointer to the requested scenario via RollbackPublication / PublishRun.
//
// Used by Playwright Stage 5 acceptance without mocking Dashboard/Actions APIs.
func runPrepareUnifiedBrowserScenario(args []string) int {
	fs := flag.NewFlagSet("prepare-unified-browser-scenario", flag.ContinueOnError)
	var (
		sellerID int64
		scenario string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&scenario, "scenario", "material", "material|healthy|insufficient|shadow|promote")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required")
		return 2
	}
	scenario = strings.ToLower(strings.TrimSpace(scenario))

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	pubSvc := buildPublicationService(env)
	q := env.Queries

	if _, err := q.UpsertSellerAnalysisSettings(ctx, dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          "unified",
		AutoRunEnabled:        false,
		AutoPublishEnabled:    false,
		LegacyFallbackEnabled: true,
		UpdatedBy:             "devtool:prepare-unified-browser-scenario",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "set seller mode: %v\n", err)
		return 1
	}

	fixtures, err := ensureBrowserFixtures(ctx, env, pubSvc, sellerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ensure fixtures: %v\n", err)
		return 1
	}

	out := map[string]any{
		"seller_id":                sellerID,
		"scenario":                 scenario,
		"material_publication":     fixtures.Material,
		"healthy_publication":      fixtures.Healthy,
		"insufficient_publication": fixtures.Insufficient,
		"shadow_publication":       fixtures.Shadow,
	}

	switch scenario {
	case "material":
		if err := switchToPublication(ctx, env, pubSvc, sellerID, fixtures.Material, "browser_material"); err != nil {
			fmt.Fprintf(os.Stderr, "switch material: %v\n", err)
			return 1
		}
		out["current_publication_id"] = fixtures.Material.PublicationID
		out["current_run_id"] = fixtures.Material.RunID
		out["analysis_outcome"] = fixtures.Material.Outcome
	case "healthy":
		if err := switchToPublication(ctx, env, pubSvc, sellerID, fixtures.Healthy, "browser_healthy"); err != nil {
			fmt.Fprintf(os.Stderr, "switch healthy: %v\n", err)
			return 1
		}
		out["current_publication_id"] = fixtures.Healthy.PublicationID
		out["current_run_id"] = fixtures.Healthy.RunID
		out["analysis_outcome"] = fixtures.Healthy.Outcome
	case "insufficient":
		if err := switchToPublication(ctx, env, pubSvc, sellerID, fixtures.Insufficient, "browser_insufficient"); err != nil {
			fmt.Fprintf(os.Stderr, "switch insufficient: %v\n", err)
			return 1
		}
		out["current_publication_id"] = fixtures.Insufficient.PublicationID
		out["current_run_id"] = fixtures.Insufficient.RunID
		out["analysis_outcome"] = fixtures.Insufficient.Outcome
	case "shadow":
		// Keep visible pointer on material; ensure shadow exists but is not current.
		if err := switchToPublication(ctx, env, pubSvc, sellerID, fixtures.Material, "browser_shadow_base"); err != nil {
			fmt.Fprintf(os.Stderr, "switch material for shadow base: %v\n", err)
			return 1
		}
		out["current_publication_id"] = fixtures.Material.PublicationID
		out["current_run_id"] = fixtures.Material.RunID
		out["analysis_outcome"] = fixtures.Material.Outcome
		out["shadow_visible"] = false
	case "promote":
		if fixtures.Shadow.PublicationID == 0 {
			fmt.Fprintln(os.Stderr, "shadow publication missing")
			return 1
		}
		res, err := pubSvc.PromoteShadowPublication(ctx, publication.PromoteRequest{
			PublicationID: fixtures.Shadow.PublicationID,
			Actor:         "devtool:prepare-unified-browser-scenario",
			Reason:        "browser_promote",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "promote: %v\n", err)
			return 1
		}
		out["current_publication_id"] = res.PublicationID
		out["current_run_id"] = res.RunID
		out["analysis_outcome"] = res.AnalysisOutcome
		out["promoted"] = true
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", scenario)
		return 2
	}

	// Re-read pointers for evidence.
	var unifiedPubID, compatRunID int64
	_ = env.Postgres.Pool.QueryRow(ctx, `
SELECT publication_id FROM seller_current_unified_publications WHERE seller_account_id=$1`, sellerID).Scan(&unifiedPubID)
	_ = env.Postgres.Pool.QueryRow(ctx, `
SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRunID)
	out["unified_pointer_publication_id"] = unifiedPubID
	out["compat_pointer_run_id"] = compatRunID

	printJSON(out)
	return 0
}

type browserPubRef struct {
	PublicationID int64  `json:"publication_id"`
	RunID         int64  `json:"run_id"`
	Outcome       string `json:"outcome"`
	Visibility    string `json:"visibility"`
	MetricDate    string `json:"metric_date"`
	ActionsCount  int    `json:"actions_count"`
}

type browserFixtures struct {
	Material     browserPubRef
	Healthy      browserPubRef
	Insufficient browserPubRef
	Shadow       browserPubRef
}

func ensureBrowserFixtures(ctx context.Context, env *unifiedDevtoolEnv, pubSvc *publication.Service, sellerID int64) (browserFixtures, error) {
	var out browserFixtures
	var err error

	out.Material, err = findOrPublishFixture(ctx, env, pubSvc, sellerID,
		time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
		"unified_output_v1.valid.json",
		publication.VisibilityVisible,
		"material_actions_found",
	)
	if err != nil {
		return out, fmt.Errorf("material: %w", err)
	}
	out.Healthy, err = findOrPublishFixture(ctx, env, pubSvc, sellerID,
		time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		"unified_output_v1.no_actions.json",
		publication.VisibilityVisible,
		"no_material_actions",
	)
	if err != nil {
		return out, fmt.Errorf("healthy: %w", err)
	}
	out.Insufficient, err = findOrPublishFixture(ctx, env, pubSvc, sellerID,
		time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
		"unified_output_v1.insufficient_data.json",
		publication.VisibilityVisible,
		"insufficient_data",
	)
	if err != nil {
		return out, fmt.Errorf("insufficient: %w", err)
	}
	out.Shadow, err = findOrPublishFixture(ctx, env, pubSvc, sellerID,
		time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC),
		"unified_output_v1.valid.json",
		publication.VisibilityShadow,
		"material_actions_found",
	)
	if err != nil {
		return out, fmt.Errorf("shadow: %w", err)
	}
	return out, nil
}

func findOrPublishFixture(
	ctx context.Context,
	env *unifiedDevtoolEnv,
	pubSvc *publication.Service,
	sellerID int64,
	metricDate time.Time,
	fixtureName string,
	visibility publication.Visibility,
	wantOutcome string,
) (browserPubRef, error) {
	dateStr := metricDate.Format("2006-01-02")
	var existing browserPubRef
	err := env.Postgres.Pool.QueryRow(ctx, `
SELECT p.id, p.run_id, p.analysis_outcome, p.visibility, p.metric_date::text, p.actions_record_count
FROM unified_analysis_publications p
WHERE p.seller_account_id=$1
  AND p.metric_date=$2::date
  AND p.analysis_outcome=$3
  AND p.visibility=$4
  AND p.status IN ('published', 'superseded')
ORDER BY p.id ASC
LIMIT 1`, sellerID, dateStr, wantOutcome, string(visibility)).Scan(
		&existing.PublicationID, &existing.RunID, &existing.Outcome, &existing.Visibility, &existing.MetricDate, &existing.ActionsCount,
	)
	if err == nil {
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return browserPubRef{}, err
	}

	runID, err := seedPublicationReadyFixture(ctx, env, sellerID, metricDate, fixtureName)
	if err != nil {
		return browserPubRef{}, err
	}
	vis := visibility
	res, err := pubSvc.PublishRun(ctx, publication.PublishRequest{
		RunID:           runID,
		Actor:           "devtool:prepare-unified-browser-scenario",
		ForceVisibility: &vis,
	})
	if err != nil {
		return browserPubRef{}, fmt.Errorf("publish run %d (fixture=%s): %w", runID, fixtureName, err)
	}
	// PublishRun can return a non-error result whose Outcome still means the
	// publication was NOT actually persisted as published (e.g. bundle
	// validation failure, stale metric_date, or analysis_failed). Treating
	// any non-error return as success is the root cause of downstream
	// callers finding no current unified publication to switch/roll back
	// onto (see runPrepareUnifiedBrowserScenario/switchToPublication) — a
	// publication_failed row is not a usable fixture.
	if !isSuccessfulPublishOutcome(res.Outcome) {
		return browserPubRef{}, fmt.Errorf(
			"publish run %d (fixture=%s) did not succeed: outcome=%s error_code=%s error_message=%s",
			runID, fixtureName, res.Outcome, res.ErrorCode, res.ErrorMessage,
		)
	}
	return browserPubRef{
		PublicationID: res.PublicationID,
		RunID:         res.RunID,
		Outcome:       res.AnalysisOutcome,
		Visibility:    string(res.Visibility),
		MetricDate:    res.MetricDate,
		ActionsCount:  res.ActionsRecordCount,
	}, nil
}

// isSuccessfulPublishOutcome reports whether a publication.PublishOutcome
// represents a run that actually reached (or already was at)
// status=published, i.e. is safe to switch/roll back a seller's current
// pointer onto. Outcomes like bundle_validation_failed, stale, and
// skipped_analysis_failed never project dashboard/actions data and must
// never be treated as a usable fixture.
func isSuccessfulPublishOutcome(outcome publication.PublishOutcome) bool {
	switch outcome {
	case publication.OutcomePublished, publication.OutcomeAlreadyPublished:
		return true
	default:
		return false
	}
}

// switchToPublication makes target the seller's current visible unified
// publication. If the seller has no current unified publication at all yet
// (e.g. this is the very first publish for the seller), RollbackPublication
// always fails with "no current unified publication to roll back" — it is
// designed to switch FROM an existing current publication, not to
// bootstrap the very first one. In that case, set the current pointer
// directly instead of going through Rollback.
func switchToPublication(ctx context.Context, env *unifiedDevtoolEnv, pubSvc *publication.Service, sellerID int64, target browserPubRef, reason string) error {
	hasCurrent, err := sellerHasCurrentUnifiedPublication(ctx, env, sellerID)
	if err != nil {
		return fmt.Errorf("check seller current unified publication: %w", err)
	}
	if !hasCurrent {
		return bootstrapCurrentPublication(ctx, env, sellerID, target)
	}

	_, err = pubSvc.RollbackPublication(ctx, publication.RollbackRequest{
		SellerAccountID:     sellerID,
		TargetPublicationID: target.PublicationID,
		Actor:               "devtool:prepare-unified-browser-scenario",
		Reason:              reason,
	})
	if err != nil {
		var pubErr *publication.Error
		if errors.As(err, &pubErr) && strings.Contains(pubErr.Error(), "already current") {
			return nil
		}
		// Fallback: some wrapped forms only expose the message.
		if strings.Contains(err.Error(), "already current") {
			return nil
		}
		return err
	}
	return nil
}

// sellerHasCurrentUnifiedPublication reports whether the seller already has
// a row in seller_current_unified_publications (i.e. has published at least
// one visible unified publication before).
func sellerHasCurrentUnifiedPublication(ctx context.Context, env *unifiedDevtoolEnv, sellerID int64) (bool, error) {
	var exists bool
	if err := env.Postgres.Pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM seller_current_unified_publications WHERE seller_account_id=$1)`, sellerID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// bootstrapCurrentPublication sets the seller's current unified publication
// pointer (and the legacy-compat seller_current_ai_snapshots pointer)
// directly onto target, for the case where the seller has no current
// publication yet. This mirrors the pointer-setting side effect of
// PublishRun(visibility=visible) for a first-ever publish, without
// requiring RollbackPublication (which only supports switching FROM an
// existing current publication).
func bootstrapCurrentPublication(ctx context.Context, env *unifiedDevtoolEnv, sellerID int64, target browserPubRef) error {
	metricDate, err := time.Parse("2006-01-02", target.MetricDate)
	if err != nil {
		return fmt.Errorf("parse target metric_date %q: %w", target.MetricDate, err)
	}
	q := env.Queries
	if _, err := q.UpsertSellerCurrentUnifiedPublication(ctx, dbgen.UpsertSellerCurrentUnifiedPublicationParams{
		SellerAccountID: sellerID,
		PublicationID:   target.PublicationID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
	}); err != nil {
		return fmt.Errorf("bootstrap seller current unified publication: %w", err)
	}
	if err := q.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
		SellerAccountID: sellerID,
		PipelineRunID:   target.RunID,
	}); err != nil {
		return fmt.Errorf("bootstrap seller current ai snapshot: %w", err)
	}
	return nil
}

func seedPublicationReadyFixture(ctx context.Context, env *unifiedDevtoolEnv, sellerID int64, metricDate time.Time, fixtureName string) (int64, error) {
	policy := publication.DefaultPublicationPolicy()
	raw, err := readContractFixture(fixtureName)
	if err != nil {
		return 0, err
	}
	var parsed contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, err
	}
	packID := fmt.Sprintf("pack:%d:%s:browser", sellerID, metricDate.Format("2006-01-02"))
	packHash := fmt.Sprintf("browser_hash_%d_%s", sellerID, metricDate.Format("20060102"))
	for len(packHash) < 64 {
		packHash += "0"
	}
	packHash = packHash[:64]
	parsed.AnalysisMeta.PackID = packID
	parsed.AnalysisMeta.PackHash = packHash
	parsed.AnalysisMeta.MetricDate = metricDate.Format("2006-01-02")
	canonical, err := outputvalidator.CanonicalJSON(&parsed)
	if err != nil {
		return 0, err
	}
	outputHash := outputvalidator.OutputHash(canonical)

	q := env.Queries
	run, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               "browser_fixture",
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: pgtype.Text{String: contract.AnalysisPackSchemaVersionV1, Valid: true},
		DataPackHash:          packHash,
		PipelineVersion:       pgtype.Text{String: "browser", Valid: true},
		PromptVersion:         pgtype.Text{String: policy.AcceptedPromptVersion, Valid: true},
		Provider:              pgtype.Text{String: "openai", Valid: true},
		ProviderModel:         pgtype.Text{String: "gpt-test", Valid: true},
		ProviderAttemptCount:  1,
		StartedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create run: %w", err)
	}
	if _, err := env.Postgres.Pool.Exec(ctx, `
UPDATE ai_pipeline_runs SET
  pack_id=$2, pack_hash=$3, code_interpreter_calls=1,
  provider_status='completed', cleanup_status='completed'
WHERE id=$1`, run.ID, packID, packHash); err != nil {
		return 0, err
	}
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   canonical,
	}); err != nil {
		return 0, err
	}
	valPayload, _ := json.Marshal(map[string]any{
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
		PayloadJson:   valPayload,
	}); err != nil {
		return 0, err
	}
	summaryPayload, _ := json.Marshal(map[string]any{
		"outcome":      parsed.AnalysisOutcome,
		"action_count": len(parsed.Actions),
		"output_hash":  outputHash,
	})
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputSummary,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   summaryPayload,
	}); err != nil {
		return 0, err
	}

	// PublishRun requires a matching unified_dashboard_facts_v1 artifact
	// (Dashboard Facts are the sole source of truth for every factual
	// dashboard field — see docs/current/dashboard-field-ownership.md).
	// Without this, bundlevalidator.ValidatePublicationBundle always
	// rejects the bundle (CodeDashboardFactsMissing), PublishRun records a
	// publication_failed row instead of published, and the seller's
	// current pointer is never set — which is what previously made every
	// browser fixture scenario fail downstream in switchToPublication.
	// Derive facts from the same fixture used for the AI output artifact so
	// bundlevalidator's coverage-aware completeness rules pass, mirroring
	// buildTestDashboardFacts in publication_integration_test.go.
	artifacts := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(q))
	facts := buildBrowserDashboardFacts(sellerID, metricDate, packHash, &parsed)
	if err := artifacts.SaveDashboardFacts(ctx, run.ID, facts); err != nil {
		return 0, fmt.Errorf("save dashboard facts artifact: %w", err)
	}

	return run.ID, nil
}

// buildBrowserDashboardFacts derives a minimal-but-consistent
// dashboardfacts.DashboardFactsV1 fixture from a parsed AI output fixture,
// so browser/Playwright fixtures exercise PublishRun's facts-required path
// the same way publication_integration_test.go's buildTestDashboardFacts
// does for Go integration tests. Coverage flags mirror whichever blocks the
// AI fixture itself reports as available, and the factual content is
// copied 1:1 from the AI output so bundlevalidator.ValidatePublicationBundle's
// coverage-aware rules pass (including SKU focus entities resolving against
// this seeded facts index).
func buildBrowserDashboardFacts(sellerID int64, metricDate time.Time, packHash string, out *contract.UnifiedAnalysisOutputV1) dashboardfacts.DashboardFactsV1 {
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
		// Align factual store/attributed denominators with narrative metric_claims
		// so claim-aware bundle validation can publish browser fixtures that
		// historically kept deliberately divergent AI summary percentages.
		if narr := dash.AdEfficiency.Narrative; narr != nil {
			var storeShare, attrShare *float64
			for _, c := range narr.MetricClaims {
				if c.Value == nil {
					continue
				}
				switch strings.TrimSpace(c.MetricCode) {
				case "ad_spend_share_of_total_store_revenue_pct":
					v := *c.Value
					storeShare = &v
				case "attributed_campaign_drr_pct":
					v := *c.Value
					attrShare = &v
				}
			}
			if attrShare != nil {
				facts.AdEfficiency.Summary.CurrentDRRPct = attrShare
				facts.AdEfficiency.Summary.AverageDRR7dPct = attrShare
			}
			if storeShare != nil {
				facts.AdEfficiency.Summary.TotalSpend7d = *storeShare
				facts.AdEfficiency.Summary.TotalStoreRevenue7d = 100
				facts.AdEfficiency.Summary.TotalSpendToday = *storeShare
				facts.AdEfficiency.Summary.TotalStoreRevenueToday = 100
			}
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
	addSKU := func(sku *int64, offerID *string, productName string) int64 {
		id := nextProductID
		rec := dashboardfacts.SKUFactRecord{
			OzonProductID:  id,
			SKU:            sku,
			OfferID:        offerID,
			ProductName:    &productName,
			Revenue:        1000,
			Orders:         1,
			StockAvailable: 10,
		}
		facts.SKUFactsIndex.ByOzonProductID[fmt.Sprintf("%d", id)] = rec
		nextProductID++
		return id
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
		id := addSKU(c.SKU, c.OfferID, c.ProductName)
		facts.KeySKUCohort.Items = append(facts.KeySKUCohort.Items, dashboardfacts.KeySKUCohortItem{
			OzonProductID:             id,
			SKU:                       c.SKU,
			OfferID:                   c.OfferID,
			ProductName:               &c.ProductName,
			Revenue7d:                 1000,
			RevenueSharePct:           100,
			CumulativeRevenueSharePct: 100,
		})
	}
	if len(facts.KeySKUCohort.Items) > 0 {
		facts.KeySKUCohort.Status = "available"
		facts.KeySKUCohort.ThresholdPct = dashboardfacts.KeySKUCohortThresholdPct
		facts.KeySKUCohort.PolicyVersion = dashboardfacts.KeySKUCohortPolicyVersion
		facts.KeySKUCohort.StoreTotalRevenue7d = 1000
		facts.KeySKUCohort.KeySKUsTotalRevenue7d = 1000
		facts.KeySKUCohort.CumulativeRevenueSharePct = 100
		for i := range facts.KeySKUCohort.Items {
			facts.KeySKUCohort.Items[i].Rank = i + 1
			facts.KeySKUCohort.Items[i].Orders7d = 1
		}
	}

	return facts
}

func readContractFixture(name string) ([]byte, error) {
	candidates := []string{
		filepath.Join("internal", "unifiedanalysis", "contract", "testdata", name),
		filepath.Join("..", "internal", "unifiedanalysis", "contract", "testdata", name),
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		candidates = append([]string{
			filepath.Join(filepath.Dir(file), "..", "..", "internal", "unifiedanalysis", "contract", "testdata", name),
		}, candidates...)
	}
	var lastErr error
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err == nil {
			return raw, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("read fixture %s: %w", name, lastErr)
}
