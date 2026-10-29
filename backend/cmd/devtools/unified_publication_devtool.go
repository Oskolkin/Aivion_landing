package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// buildPublicationService wires a Stage 5 publication.Service from a devtool env.
func buildPublicationService(env *unifiedDevtoolEnv) *publication.Service {
	uaRepo := unifiedanalysis.NewRepository(env.Queries)
	uaArtifacts := unifiedanalysis.NewArtifactStore(uaRepo)
	return publication.NewService(
		env.Postgres.Pool,
		env.Queries,
		uaArtifacts,
		publication.DefaultPublicationPolicy(),
		publication.GlobalFlagsFromEnv(),
	)
}

func publishResultSummary(result *publication.PublishResult) map[string]any {
	if result == nil {
		return map[string]any{"error": "result is nil"}
	}
	out := map[string]any{
		"outcome":                    result.Outcome,
		"publication_id":             result.PublicationID,
		"run_id":                     result.RunID,
		"seller_account_id":          result.SellerAccountID,
		"metric_date":                result.MetricDate,
		"mode":                       result.Mode,
		"visibility":                 result.Visibility,
		"status":                     result.Status,
		"analysis_outcome":           result.AnalysisOutcome,
		"output_hash":                result.OutputHash,
		"bundle_hash":                result.BundleHash,
		"dashboard_run_id":           result.DashboardRunID,
		"actions_run_id":             result.ActionsRunID,
		"dashboard_data_snapshot_id": result.DashboardDataSnapshotID,
		"dashboard_record_count":     result.DashboardRecordCount,
		"actions_record_count":       result.ActionsRecordCount,
		"top_actions_count":          result.TopActionsCount,
		"recommended_actions_count":  result.RecommendedActionsCount,
		"error_code":                 result.ErrorCode,
		"error_message":              result.ErrorMessage,
	}
	if !result.PublishedAt.IsZero() {
		out["published_at"] = result.PublishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func printJSON(v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(raw))
}

// runPublishUnifiedAnalysis implements `publish-unified-analysis`.
func runPublishUnifiedAnalysis(args []string) int {
	fs := flag.NewFlagSet("publish-unified-analysis", flag.ContinueOnError)
	var (
		runID                 int64
		visibilityRaw         string
		execution             string
		wait                  bool
		allowNonCurrentPrompt bool
		timeoutRaw            string
	)
	fs.Int64Var(&runID, "run-id", 0, "unified analysis run id (required)")
	fs.StringVar(&visibilityRaw, "visibility", "", "optional force visibility: visible|shadow")
	fs.StringVar(&execution, "execution", "queue", "queue|direct (default queue)")
	fs.BoolVar(&wait, "wait", true, "wait for a terminal publication status (queue execution only)")
	fs.BoolVar(&allowNonCurrentPrompt, "allow-non-current-prompt", false, "bypass prompt/schema pinning (admin/devtools republish only)")
	fs.StringVar(&timeoutRaw, "timeout", "2m", "wait timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if runID <= 0 {
		fmt.Fprintln(os.Stderr, "run-id is required and must be > 0")
		return 2
	}
	execution = strings.ToLower(strings.TrimSpace(execution))
	if execution != "queue" && execution != "direct" {
		fmt.Fprintln(os.Stderr, "execution: use queue or direct")
		return 2
	}
	var forceVisibility *publication.Visibility
	if v := strings.ToLower(strings.TrimSpace(visibilityRaw)); v != "" {
		vv := publication.Visibility(v)
		if !vv.IsValid() {
			fmt.Fprintln(os.Stderr, "visibility: use visible or shadow")
			return 2
		}
		forceVisibility = &vv
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
	if err != nil || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeout: expected positive duration, e.g. 2m")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{WithAsynq: execution == "queue"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	req := publication.PublishRequest{
		RunID:                 runID,
		Actor:                 "devtool:publish-unified-analysis",
		AllowNonCurrentPrompt: allowNonCurrentPrompt,
		ForceVisibility:       forceVisibility,
	}

	if execution == "direct" {
		pubSvc := buildPublicationService(env)
		result, pubErr := pubSvc.PublishRun(ctx, req)
		if pubErr != nil {
			fmt.Fprintf(os.Stderr, "publish: %v\n", pubErr)
			printJSON(publishResultSummary(result))
			return 1
		}
		printJSON(publishResultSummary(result))
		return 0
	}

	enqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: env.Asynq}
	if err := enqueue.EnqueuePublish(ctx, runID); err != nil {
		fmt.Fprintf(os.Stderr, "enqueue publish: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "run_id=%d execution=queue enqueued\n", runID)
	if !wait {
		return 0
	}
	pub, waitErr := waitUnifiedAnalysisPublicationTerminal(ctx, env.Queries, runID, timeout)
	if waitErr != nil {
		fmt.Fprintf(os.Stderr, "wait: %v\n", waitErr)
		if pub != nil {
			printJSON(publicationRowSummary(*pub))
		}
		return 1
	}
	printJSON(publicationRowSummary(*pub))
	if publication.Status(pub.Status) == publication.StatusPublicationFailed {
		return 1
	}
	return 0
}

func waitUnifiedAnalysisPublicationTerminal(ctx context.Context, queries *dbgen.Queries, runID int64, timeout time.Duration) (*dbgen.UnifiedAnalysisPublication, error) {
	deadline := time.Now().Add(timeout)
	for {
		row, err := queries.GetUnifiedAnalysisPublicationByRunID(ctx, runID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			switch publication.Status(row.Status) {
			case publication.StatusPublished, publication.StatusPublicationFailed,
				publication.StatusRolledBack, publication.StatusSuperseded:
				return &row, nil
			}
		}
		if time.Now().After(deadline) {
			if err == nil {
				return &row, fmt.Errorf("timeout after %s waiting for terminal publication status (status=%q)", timeout, row.Status)
			}
			return nil, fmt.Errorf("timeout after %s waiting for publication row to be created", timeout)
		}
		timer := time.NewTimer(1 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func publicationRowSummary(row dbgen.UnifiedAnalysisPublication) map[string]any {
	out := map[string]any{
		"publication_id":            row.ID,
		"seller_account_id":         row.SellerAccountID,
		"metric_date":               row.MetricDate.Time.UTC().Format("2006-01-02"),
		"run_id":                    row.RunID,
		"mode":                      row.Mode,
		"visibility":                row.Visibility,
		"status":                    row.Status,
		"analysis_outcome":          row.AnalysisOutcome,
		"pack_id":                   row.PackID,
		"pack_hash":                 row.PackHash,
		"prompt_version":            row.PromptVersion,
		"schema_version":            row.SchemaVersion,
		"output_hash":               row.OutputHash,
		"bundle_hash":               row.BundleHash,
		"dashboard_record_count":    row.DashboardRecordCount,
		"actions_record_count":      row.ActionsRecordCount,
		"top_actions_count":         row.TopActionsCount,
		"recommended_actions_count": row.RecommendedActionsCount,
	}
	if row.ErrorCode.Valid {
		out["error_code"] = row.ErrorCode.String
	}
	if row.ErrorMessage.Valid {
		out["error_message"] = row.ErrorMessage.String
	}
	if row.PublishedAt.Valid {
		out["published_at"] = row.PublishedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.SupersededAt.Valid {
		out["superseded_at"] = row.SupersededAt.Time.UTC().Format(time.RFC3339)
	}
	return out
}

// runInspectUnifiedPublication implements `inspect-unified-publication`.
func runInspectUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("inspect-unified-publication", flag.ContinueOnError)
	var publicationID int64
	fs.Int64Var(&publicationID, "publication-id", 0, "unified_analysis_publications.id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if publicationID <= 0 {
		fmt.Fprintln(os.Stderr, "publication-id is required and must be > 0")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	row, err := env.Queries.GetUnifiedAnalysisPublicationByID(ctx, publicationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get publication: %v\n", err)
		return 1
	}
	maps, err := env.Queries.ListUnifiedPublicationActionMaps(ctx, publicationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list action maps: %v\n", err)
		return 1
	}
	summary := publicationRowSummary(row)
	summary["action_map_count"] = len(maps)
	topCount := 0
	recCount := 0
	for _, m := range maps {
		if m.IsTop {
			topCount++
		}
		if m.IsRecommended {
			recCount++
		}
	}
	summary["action_map_top_count"] = topCount
	summary["action_map_recommended_count"] = recCount
	summary["actions_count"] = row.ActionsRecordCount

	// Enrich with real frozen dashboard content (not just dashboard_record_count).
	if snap, snapErr := env.Queries.GetAIDashboardDataSnapshotByPipelineRunID(ctx, row.RunID); snapErr == nil {
		var frozen dashboardhome.FrozenDashboardData
		if uerr := json.Unmarshal(snap.PayloadJson, &frozen); uerr == nil {
			summary["kpi_count"] = len(frozen.SellerPulse.KPIs)
			summary["trend_day_count"] = len(frozen.SellerPulse.Trend7D)
			summary["stock_segment_count"] = len(frozen.StockCoverage.Segments)
			summary["funnel_stage_count"] = len(frozen.SalesFunnel.Stages)
			summary["ad_point_count"] = len(frozen.AdEfficiency.Points)
			skuCount := len(frozen.SKUFocus.OrderDrop) + len(frozen.SKUFocus.StockRisk) +
				len(frozen.SKUFocus.NoSalesWithStock) + len(frozen.SKUFocus.Surplus) +
				len(frozen.SKUFocus.SlowTurnover) + len(frozen.SKUFocus.KeySKUs)
			summary["sku_focus_count"] = skuCount
			contentStatus := "empty"
			if len(frozen.SellerPulse.KPIs) > 0 || len(frozen.SellerPulse.Trend7D) > 0 {
				contentStatus = "ready"
			}
			summary["dashboard_content_status"] = contentStatus
		}
	}
	if narr, narrErr := env.Queries.GetAIDashboardNarrativeByPipelineRunID(ctx, dbgen.GetAIDashboardNarrativeByPipelineRunIDParams{
		AiPipelineRunID: pgtype.Int8{Int64: row.RunID, Valid: true},
		SellerAccountID: row.SellerAccountID,
	}); narrErr == nil {
		var payload map[string]any
		if json.Unmarshal(narr.ValidatedPayload, &payload) == nil {
			headline, _ := payload["headline"].(string)
			execSummary, _ := payload["executive_summary"].(string)
			summary["headline_present"] = strings.TrimSpace(headline) != ""
			summary["executive_summary_present"] = strings.TrimSpace(execSummary) != ""
		}
	}
	// Facts artifact (may be absent on pre-facts publications).
	artStore := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(env.Queries))
	if facts, ok := artStore.LoadDashboardFacts(ctx, row.RunID); ok {
		summary["facts_hash"] = facts.FactsHash
		summary["facts_pack_hash"] = facts.PackHash
		summary["facts_kpi_count"] = len(facts.SellerPulse.KPIs)
		summary["facts_trend_day_count"] = len(facts.SellerPulse.Trend7D)
	} else {
		summary["facts_hash"] = nil
		summary["facts_missing"] = true
	}

	// Current pointers for this seller.
	if cur, curErr := env.Queries.GetSellerCurrentUnifiedPublication(ctx, row.SellerAccountID); curErr == nil {
		summary["current_unified_publication_id"] = cur.PublicationID
		summary["is_current_unified"] = cur.PublicationID == row.ID
	}
	if compatRun, compatErr := env.Queries.GetSellerCurrentPipelineRunID(ctx, row.SellerAccountID); compatErr == nil {
		summary["compat_pipeline_run_id"] = compatRun
		summary["is_current_compat"] = compatRun == row.RunID
	}

	printJSON(summary)
	return 0
}

// runListUnifiedPublications implements `list-unified-publications`.
func runListUnifiedPublications(args []string) int {
	fs := flag.NewFlagSet("list-unified-publications", flag.ContinueOnError)
	var (
		sellerID int64
		limit    int64
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.Int64Var(&limit, "limit", 20, "max rows to return")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if limit <= 0 {
		limit = 20
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	rows, err := env.Queries.ListUnifiedAnalysisPublicationsBySeller(ctx, dbgen.ListUnifiedAnalysisPublicationsBySellerParams{
		SellerAccountID: sellerID,
		Limit:           int32(limit),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list publications: %v\n", err)
		return 1
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicationRowSummary(row))
	}
	printJSON(out)
	return 0
}

// runRollbackUnifiedPublication implements `rollback-unified-publication`.
func runRollbackUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("rollback-unified-publication", flag.ContinueOnError)
	var (
		sellerID            int64
		targetPublicationID int64
		reason              string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.Int64Var(&targetPublicationID, "target-publication-id", 0, "previously published publication to restore as current (required)")
	fs.StringVar(&reason, "reason", "", "free-text rollback reason")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if targetPublicationID <= 0 {
		fmt.Fprintln(os.Stderr, "target-publication-id is required and must be > 0")
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
	result, rbErr := pubSvc.RollbackPublication(ctx, publication.RollbackRequest{
		SellerAccountID:     sellerID,
		TargetPublicationID: targetPublicationID,
		Actor:               "devtool:rollback-unified-publication",
		Reason:              reason,
	})
	if rbErr != nil {
		fmt.Fprintf(os.Stderr, "rollback: %v\n", rbErr)
		return 1
	}
	printJSON(publishResultSummary(result))
	return 0
}

// runPromoteShadowPublication implements `promote-shadow-publication`.
func runPromoteShadowPublication(args []string) int {
	fs := flag.NewFlagSet("promote-shadow-publication", flag.ContinueOnError)
	var (
		publicationID int64
		reason        string
	)
	fs.Int64Var(&publicationID, "publication-id", 0, "unified_analysis_publications.id (required, must be published+shadow)")
	fs.StringVar(&reason, "reason", "", "free-text promotion reason")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if publicationID <= 0 {
		fmt.Fprintln(os.Stderr, "publication-id is required and must be > 0")
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
	result, promErr := pubSvc.PromoteShadowPublication(ctx, publication.PromoteRequest{
		PublicationID: publicationID,
		Actor:         "devtool:promote-shadow-publication",
		Reason:        reason,
	})
	if promErr != nil {
		fmt.Fprintf(os.Stderr, "promote: %v\n", promErr)
		return 1
	}
	printJSON(publishResultSummary(result))
	return 0
}

func reconcileResultSummary(result publication.ReconcileResult) map[string]any {
	return map[string]any{
		"seller_account_id": result.SellerAccountID,
		"dry_run":           result.DryRun,
		"checked":           result.Checked,
		"publication_id":    result.PublicationID,
		"expected_run_id":   result.ExpectedRunID,
		"actual_run_id":     result.ActualRunID,
		"mismatched":        result.Mismatched,
		"repaired":          result.Repaired,
	}
}

// runReconcileUnifiedPublication implements `reconcile-unified-publication`.
//
// It detects (and, unless --dry-run, repairs) a seller_current_ai_snapshots
// (compat) pointer that has drifted from the seller's authoritative
// seller_current_unified_publications pointer. See
// publication.Service.ReconcileCurrentPublicationPointers doc for exactly
// what is (and, deliberately, is not) touched.
func runReconcileUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("reconcile-unified-publication", flag.ContinueOnError)
	var (
		sellerID int64
		dryRun   bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.BoolVar(&dryRun, "dry-run", true, "report drift without repairing it (default true; pass --dry-run=false to repair)")
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
	result, recErr := pubSvc.ReconcileCurrentPublicationPointers(ctx, sellerID, dryRun)
	if recErr != nil {
		fmt.Fprintf(os.Stderr, "reconcile: %v\n", recErr)
		return 1
	}
	printJSON(reconcileResultSummary(result))
	if result.Mismatched && dryRun {
		// Non-zero exit on dry-run drift so operators/cron can alert on it,
		// without this devtool itself mutating anything.
		return 1
	}
	return 0
}
