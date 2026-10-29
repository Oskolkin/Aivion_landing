package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
)

// repair-stock-cohorts-and-publish re-applies stock cohort reconcile + domain
// backfill on an already-validated run output, saves a new validated artifact,
// and publishes it as current.
func runRepairStockCohortsAndPublish(args []string) int {
	fs := flag.NewFlagSet("repair-stock-cohorts-and-publish", flag.ContinueOnError)
	var (
		runID      int64
		visibility string
	)
	fs.Int64Var(&runID, "run-id", 0, "source unified analysis run id with validated output")
	fs.StringVar(&visibility, "visibility", "visible", "visible|shadow")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if runID <= 0 {
		fmt.Fprintln(os.Stderr, "run-id is required")
		return 2
	}
	if err := devseed.AssertDevAIPipelineAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{ForceEnabled: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer env.Postgres.Close()

	uaRepo := unifiedanalysis.NewRepository(env.Queries)
	artifacts := unifiedanalysis.NewArtifactStore(uaRepo)
	payload, _, ok := artifacts.LoadValidatedOutputJSON(ctx, runID)
	if !ok || len(payload) == 0 {
		fmt.Fprintf(os.Stderr, "validated output missing for run %d\n", runID)
		return 1
	}
	out, err := outputextract.ParseOutput(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse output: %v\n", err)
		return 1
	}

	run, err := uaRepo.GetRun(ctx, runID)
	if err != nil || run == nil {
		fmt.Fprintf(os.Stderr, "get run %d: %v\n", runID, err)
		return 1
	}

	packStore := uapack.NewS3PackObjectStore(env.S3)
	packKey := strings.TrimSpace(run.PackStorageKey)
	if packKey == "" {
		fmt.Fprintln(os.Stderr, "run has no pack_storage_key")
		return 1
	}
	packBytes, err := packStore.Get(ctx, packKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load pack: %v\n", err)
		return 1
	}
	idx, err := outputvalidator.BuildPackReferenceIndex(packBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack index: %v\n", err)
		return 1
	}

	before := len(out.Actions)
	dropped := outputvalidator.ReconcileStockActionCohorts(out, idx)
	ensure := outputvalidator.EnsureMaterialDomainActions(out, idx)
	cats := outputvalidator.EnsureCategorySummaries(out, idx)
	scrubbed := outputvalidator.ScrubInternalFieldNames(out)
	_ = outputvalidator.SyncActionEntityCounts(out)
	_ = outputvalidator.FinalizeTopActionIDs(out)

	meta := outputvalidator.ExpectedRunMeta{
		PackID:     run.PackID,
		PackHash:   run.PackHash,
		MetricDate: run.MetricDate.UTC().Format("2006-01-02"),
	}
	report := outputvalidator.Validate(out, meta, idx)
	fmt.Printf("repair: before_actions=%d dropped_stock=%d ensured=%v category_summaries=%d scrubbed=%d after_actions=%d valid=%v errors=%d\n",
		before, dropped, ensure.AddedActionIDs, cats, scrubbed, len(out.Actions), report.Valid, len(report.Errors))
	if !report.Valid {
		raw, _ := json.MarshalIndent(report.Errors, "", "  ")
		fmt.Fprintln(os.Stderr, string(raw))
		return 1
	}

	canonical, err := outputvalidator.CanonicalJSON(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical: %v\n", err)
		return 1
	}
	outputHash := outputvalidator.OutputHash(canonical)
	if err := artifacts.SaveOutputValidated(ctx, runID, canonical, outputHash); err != nil {
		fmt.Fprintf(os.Stderr, "save validated: %v\n", err)
		return 1
	}

	vis := publication.Visibility(strings.TrimSpace(visibility))
	pubAsCurrent := true

	// Already-published runs short-circuit PublishRun; mark the existing
	// publication retryable and clear published pointer so CompleteAIDashboardNarrative
	// can update the narrative row linked to this run.
	_, _ = env.Postgres.Pool.Exec(ctx, `
		UPDATE unified_analysis_publications
		SET status = 'publication_failed',
		    error_code = 'repair_republish',
		    error_message = 'stock cohort repair republish',
		    updated_at = NOW()
		WHERE run_id = $1 AND status IN ('published', 'publication_failed', 'publishing')
	`, runID)
	_, _ = env.Postgres.Pool.Exec(ctx, `
		UPDATE ai_pipeline_runs
		SET status = 'publication_ready',
		    published_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, runID)

	pub := buildPublicationService(env)
	result, err := pub.PublishRun(ctx, publication.PublishRequest{
		RunID:                 runID,
		Actor:                 "devtool:repair-stock-cohorts-and-publish",
		PublishAsCurrent:      &pubAsCurrent,
		AllowNonCurrentPrompt: true,
		ForceVisibility:       &vis,
		AllowOlderMetricDate:  true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "publish: %v\n", err)
		return 1
	}
	printJSON(publishResultSummary(result))

	fmt.Println("stock_actions_audit:")
	for _, a := range out.Actions {
		if a.Category != "stock" && a.Type != "stock_replenishment" && a.Type != "stock_redistribution" {
			continue
		}
		fmt.Printf("  - id=%s type=%s title=%q entities=%d ids=%v\n",
			a.AnalysisActionID, a.Type, a.Title, len(a.Scope.EntityIDs), a.Scope.EntityIDs)
	}
	return 0
}
