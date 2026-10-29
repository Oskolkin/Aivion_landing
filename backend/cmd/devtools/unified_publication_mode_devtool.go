package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5"
)

// runSetUnifiedAnalysisMode implements `set-unified-analysis-mode`.
func runSetUnifiedAnalysisMode(args []string) int {
	fs := flag.NewFlagSet("set-unified-analysis-mode", flag.ContinueOnError)
	var (
		sellerID       int64
		mode           string
		autoRunRaw     string
		autoPublishRaw string
		legacyFallback bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&mode, "mode", "", "legacy|shadow|unified (required)")
	fs.StringVar(&autoRunRaw, "auto-run", "false", "true|false")
	fs.StringVar(&autoPublishRaw, "auto-publish", "false", "true|false")
	fs.BoolVar(&legacyFallback, "legacy-fallback", true, "allow legacy fallback when unified publication is unavailable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	m := publication.AnalysisMode(mode)
	if !m.IsValid() {
		fmt.Fprintln(os.Stderr, "mode: use legacy, shadow or unified")
		return 2
	}
	autoRun, err := strconv.ParseBool(autoRunRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auto-run: use true or false")
		return 2
	}
	autoPublish, err := strconv.ParseBool(autoPublishRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auto-publish: use true or false")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	row, err := env.Queries.UpsertSellerAnalysisSettings(ctx, dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          string(m),
		AutoRunEnabled:        autoRun,
		AutoPublishEnabled:    autoPublish,
		LegacyFallbackEnabled: legacyFallback,
		UpdatedBy:             "devtool:set-unified-analysis-mode",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "upsert seller analysis settings: %v\n", err)
		return 1
	}

	restoredPipeline := int64(0)
	restoredActionsRun := int64(0)
	if m == publication.AnalysisModeUnified {
		// Legacy e2e seed retargets ai_pipeline_runs.actions_run_id onto a
		// seed generation run. Switching back to unified must restore the
		// published projection or Actions API serves the wrong/empty set.
		var pipelineID, actionsRunID int64
		err := env.Postgres.Pool.QueryRow(ctx, `
SELECT u.run_id, u.actions_run_id
FROM seller_current_unified_publications c
JOIN unified_analysis_publications u ON u.id = c.publication_id
WHERE c.seller_account_id = $1
  AND u.status = 'published'
  AND u.visibility = 'visible'
  AND u.actions_run_id IS NOT NULL`, sellerID).Scan(&pipelineID, &actionsRunID)
		if err == nil && pipelineID > 0 && actionsRunID > 0 {
			if _, execErr := env.Postgres.Pool.Exec(ctx, `
UPDATE action_generation_runs
SET ai_pipeline_run_id = NULL, updated_at = now()
WHERE ai_pipeline_run_id = $1 AND id <> $2`, pipelineID, actionsRunID); execErr != nil {
				fmt.Fprintf(os.Stderr, "unlink foreign actions runs: %v\n", execErr)
				return 1
			}
			if _, execErr := env.Postgres.Pool.Exec(ctx, `
UPDATE action_generation_runs
SET ai_pipeline_run_id = $1, updated_at = now()
WHERE id = $2 AND seller_account_id = $3`, pipelineID, actionsRunID, sellerID); execErr != nil {
				fmt.Fprintf(os.Stderr, "relink publication actions run: %v\n", execErr)
				return 1
			}
			if _, execErr := env.Postgres.Pool.Exec(ctx, `
UPDATE ai_pipeline_runs
SET actions_run_id = $1, updated_at = now()
WHERE id = $2 AND seller_account_id = $3`, actionsRunID, pipelineID, sellerID); execErr != nil {
				fmt.Fprintf(os.Stderr, "restore pipeline actions_run_id: %v\n", execErr)
				return 1
			}
			restoredPipeline = pipelineID
			restoredActionsRun = actionsRunID
		}
	}

	global := publication.GlobalFlagsFromEnv()
	effective := publication.EffectiveSettings(&row, global)
	printJSON(map[string]any{
		"seller_account_id":       row.SellerAccountID,
		"analysis_mode":           row.AnalysisMode,
		"auto_run_enabled":        row.AutoRunEnabled,
		"auto_publish_enabled":    row.AutoPublishEnabled,
		"legacy_fallback_enabled": row.LegacyFallbackEnabled,
		"updated_by":              row.UpdatedBy,
		"restored_pipeline_id":    restoredPipeline,
		"restored_actions_run_id": restoredActionsRun,
		"global_flags": map[string]any{
			"rollout_enabled":         global.RolloutEnabled,
			"auto_run_enabled":        global.AutoRunEnabled,
			"auto_publish_enabled":    global.AutoPublishEnabled,
			"legacy_fallback_enabled": global.LegacyFallbackEnabled,
		},
		"effective": map[string]any{
			"effective_auto_run":     effective.EffectiveAutoRun,
			"effective_auto_publish": effective.EffectiveAutoPublish,
			"visibility":             effective.Visibility,
		},
	})
	return 0
}

// runCheckUnifiedPublicationReadiness implements `check-unified-publication-readiness`.
func runCheckUnifiedPublicationReadiness(args []string) int {
	fs := flag.NewFlagSet("check-unified-publication-readiness", flag.ContinueOnError)
	var (
		sellerID int64
		live     bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "optional seller_account_id for per-seller checks")
	fs.BoolVar(&live, "live", false, "reserved for parity with check-unified-analysis-readiness; unused here")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_ = live

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	out := map[string]any{}
	ok56, msg56 := checkUnifiedAnalysisMigration056(ctx, env)
	out["migration_000056"] = ok56
	out["migration_000056_note"] = msg56

	global := publication.GlobalFlagsFromEnv()
	out["global_flags"] = map[string]any{
		"rollout_enabled":         global.RolloutEnabled,
		"auto_run_enabled":        global.AutoRunEnabled,
		"auto_publish_enabled":    global.AutoPublishEnabled,
		"legacy_fallback_enabled": global.LegacyFallbackEnabled,
	}

	exitCode := 0
	if !ok56 {
		exitCode = 1
	}

	if sellerID > 0 {
		sellerInfo := map[string]any{}
		var settingsPtr *dbgen.SellerAnalysisSetting
		row, sErr := env.Queries.GetSellerAnalysisSettings(ctx, sellerID)
		if sErr == nil {
			settingsPtr = &row
			sellerInfo["settings_found"] = true
			sellerInfo["analysis_mode"] = row.AnalysisMode
			sellerInfo["auto_run_enabled"] = row.AutoRunEnabled
			sellerInfo["auto_publish_enabled"] = row.AutoPublishEnabled
			sellerInfo["legacy_fallback_enabled"] = row.LegacyFallbackEnabled
		} else if errors.Is(sErr, pgx.ErrNoRows) {
			sellerInfo["settings_found"] = false
		} else {
			sellerInfo["settings_error"] = sErr.Error()
		}
		effective := publication.EffectiveSettings(settingsPtr, global)
		sellerInfo["effective_auto_run"] = effective.EffectiveAutoRun
		sellerInfo["effective_auto_publish"] = effective.EffectiveAutoPublish
		sellerInfo["visibility"] = effective.Visibility

		if current, curErr := env.Queries.GetSellerCurrentUnifiedPublication(ctx, sellerID); curErr == nil {
			sellerInfo["current_unified_publication_id"] = current.PublicationID
			sellerInfo["current_unified_publication_status"] = current.Status
			sellerInfo["current_unified_metric_date"] = current.MetricDate.Time.UTC().Format("2006-01-02")
		} else if errors.Is(curErr, pgx.ErrNoRows) {
			sellerInfo["current_unified_publication_id"] = nil
		} else {
			sellerInfo["current_unified_publication_error"] = curErr.Error()
		}
		out["seller"] = sellerInfo
	}

	printJSON(out)
	return exitCode
}

func checkUnifiedAnalysisMigration056(ctx context.Context, env *unifiedDevtoolEnv) (bool, string) {
	var exists bool
	err := env.Postgres.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables
  WHERE table_schema = 'public' AND table_name = 'unified_analysis_publications'
)`).Scan(&exists)
	if err != nil {
		return false, err.Error()
	}
	if !exists {
		return false, "migration 000056_unified_analysis_publication not applied"
	}
	var checkDef string
	if err := env.Postgres.Pool.QueryRow(ctx, `
SELECT pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conname = 'chk_ai_pipeline_runs_status'
`).Scan(&checkDef); err != nil {
		return false, "chk_ai_pipeline_runs_status not found: " + err.Error()
	}
	if !strings.Contains(checkDef, "publication_failed") {
		return false, "chk_ai_pipeline_runs_status missing publication_failed"
	}
	return true, "publication tables and publication_failed status present"
}

// runCompareAnalysisSources implements `compare-analysis-sources`. Reports technical
// field presence/counts only — never prints narrative text or action content.
func runCompareAnalysisSources(args []string) int {
	fs := flag.NewFlagSet("compare-analysis-sources", flag.ContinueOnError)
	var sellerID int64
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	pubSvc := buildPublicationService(env)
	resolved, resErr := pubSvc.ResolveCurrentAnalysisSource(ctx, sellerID)
	out := map[string]any{"seller_account_id": sellerID}
	if resErr != nil {
		out["resolve_error"] = resErr.Error()
	} else {
		out["resolved_source"] = resolved.Source
		out["resolved_metric_date"] = resolved.MetricDate
		out["resolved_fallback_reason"] = resolved.FallbackReason
	}

	legacy := map[string]any{}
	if legacyRunID, legErr := env.Queries.GetSellerCurrentPipelineRunID(ctx, sellerID); legErr == nil {
		legacy["pipeline_run_id"] = legacyRunID
		if run, runErr := env.Queries.GetAIPipelineRun(ctx, legacyRunID); runErr == nil {
			legacy["status"] = run.Status
			legacy["metric_date"] = run.MetricDate.Time.UTC().Format("2006-01-02")
			legacy["has_dashboard_run_id"] = run.DashboardRunID.Valid
			legacy["has_actions_run_id"] = run.ActionsRunID.Valid
			legacy["has_dashboard_data_snapshot_id"] = run.DashboardDataSnapshotID.Valid
			legacy["has_data_pack_hash"] = strings.TrimSpace(run.DataPackHash) != ""
			legacy["has_global_context_hash"] = run.GlobalContextHash.Valid && strings.TrimSpace(run.GlobalContextHash.String) != ""
			legacy["published"] = run.PublishedAt.Valid
		} else if !errors.Is(runErr, pgx.ErrNoRows) {
			legacy["run_error"] = runErr.Error()
		}
	} else if errors.Is(legErr, pgx.ErrNoRows) {
		legacy["pipeline_run_id"] = nil
	} else {
		legacy["error"] = legErr.Error()
	}
	out["legacy"] = legacy

	unified := map[string]any{}
	if current, curErr := env.Queries.GetSellerCurrentUnifiedPublication(ctx, sellerID); curErr == nil {
		unified["publication_id"] = current.PublicationID
		unified["run_id"] = current.RunID
		unified["status"] = current.Status
		unified["visibility"] = current.Visibility
		unified["analysis_outcome"] = current.AnalysisOutcome
		unified["metric_date"] = current.MetricDate.Time.UTC().Format("2006-01-02")
		unified["has_dashboard_run_id"] = current.DashboardRunID.Valid
		unified["has_actions_run_id"] = current.ActionsRunID.Valid
		unified["has_dashboard_data_snapshot_id"] = current.DashboardDataSnapshotID.Valid
		unified["has_output_hash"] = strings.TrimSpace(current.OutputHash) != ""
		unified["has_bundle_hash"] = strings.TrimSpace(current.BundleHash) != ""
		if pub, pubErr := env.Queries.GetUnifiedAnalysisPublicationByID(ctx, current.PublicationID); pubErr == nil {
			unified["dashboard_record_count"] = pub.DashboardRecordCount
			unified["actions_record_count"] = pub.ActionsRecordCount
			unified["top_actions_count"] = pub.TopActionsCount
			unified["recommended_actions_count"] = pub.RecommendedActionsCount
		}
	} else if errors.Is(curErr, pgx.ErrNoRows) {
		unified["publication_id"] = nil
	} else {
		unified["error"] = curErr.Error()
	}
	out["unified"] = unified

	printJSON(out)
	return 0
}
