package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/jackc/pgx/v5/pgtype"
)

// runInspectUnifiedBundle compares facts artifact, frozen snapshot, narrative,
// actions mapping, and current pointers for one publication or run.
func runInspectUnifiedBundle(args []string) int {
	fs := flag.NewFlagSet("inspect-unified-bundle", flag.ContinueOnError)
	var publicationID, runID, sellerID int64
	var dumpDir string
	fs.Int64Var(&publicationID, "publication-id", 0, "unified_analysis_publications.id")
	fs.Int64Var(&runID, "run-id", 0, "ai_pipeline_runs.id (used when publication-id is omitted)")
	fs.Int64Var(&sellerID, "seller-id", 0, "optional seller_account_id for pointer inspection")
	fs.StringVar(&dumpDir, "dump-dir", "", "optional directory to write facts/frozen/narrative JSON dumps")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if publicationID <= 0 && runID <= 0 {
		fmt.Fprintln(os.Stderr, "publication-id or run-id is required")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	out := map[string]any{}
	if publicationID > 0 {
		row, err := env.Queries.GetUnifiedAnalysisPublicationByID(ctx, publicationID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "get publication: %v\n", err)
			return 1
		}
		runID = row.RunID
		sellerID = row.SellerAccountID
		out["publication"] = publicationRowSummary(row)
	}
	out["run_id"] = runID
	out["seller_account_id"] = sellerID

	artStore := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(env.Queries))
	if facts, ok := artStore.LoadDashboardFacts(ctx, runID); ok {
		out["facts"] = map[string]any{
			"present":         true,
			"facts_hash":      facts.FactsHash,
			"pack_hash":       facts.PackHash,
			"pack_id":         facts.PackID,
			"metric_date":     facts.MetricDate,
			"kpi_count":       len(facts.SellerPulse.KPIs),
			"trend_day_count": len(facts.SellerPulse.Trend7D),
			"content_counts":  facts.ContentCounts,
			"seller_pulse":    facts.SellerPulse,
			"account_today": map[string]any{
				"revenue_today": facts.DashboardMetrics.Account.RevenueToday,
				"orders_today":  facts.DashboardMetrics.Account.OrdersToday,
			},
		}
		if dumpDir != "" {
			_ = os.MkdirAll(dumpDir, 0o755)
			if b, err := json.MarshalIndent(facts, "", "  "); err == nil {
				_ = os.WriteFile(dumpDir+"/dashboard_facts.json", b, 0o644)
			}
		}
	} else {
		out["facts"] = map[string]any{"present": false}
	}

	if snap, err := env.Queries.GetAIDashboardDataSnapshotByPipelineRunID(ctx, runID); err == nil {
		var frozen dashboardhome.FrozenDashboardData
		if json.Unmarshal(snap.PayloadJson, &frozen) == nil {
			out["frozen"] = map[string]any{
				"kpi_count":           len(frozen.SellerPulse.KPIs),
				"trend_day_count":     len(frozen.SellerPulse.Trend7D),
				"stock_segment_count": len(frozen.StockCoverage.Segments),
				"funnel_stage_count":  len(frozen.SalesFunnel.Stages),
				"ad_point_count":      len(frozen.AdEfficiency.Points),
				"historical":          frozen.HistoricalAnalysis,
				"data_is_stale":       frozen.DataIsStale,
				"metric_date":         frozen.MetricDate,
				"seller_pulse_kpis":   frozen.SellerPulse.KPIs,
				"seller_pulse_trend":  frozen.SellerPulse.Trend7D,
			}
			if dumpDir != "" {
				_ = os.MkdirAll(dumpDir, 0o755)
				if b, err := json.MarshalIndent(frozen, "", "  "); err == nil {
					_ = os.WriteFile(dumpDir+"/frozen_dashboard.json", b, 0o644)
				}
			}
		}
	}

	if sellerID > 0 {
		if narr, err := env.Queries.GetAIDashboardNarrativeByPipelineRunID(ctx, dbgen.GetAIDashboardNarrativeByPipelineRunIDParams{
			AiPipelineRunID: pgtype.Int8{Int64: runID, Valid: true},
			SellerAccountID: sellerID,
		}); err == nil {
			var payload map[string]any
			_ = json.Unmarshal(narr.ValidatedPayload, &payload)
			headline, _ := payload["headline"].(string)
			summary, _ := payload["executive_summary"].(string)
			out["narrative"] = map[string]any{
				"status":                    narr.Status,
				"prompt_version":            narr.PromptVersion,
				"headline_present":          strings.TrimSpace(headline) != "",
				"executive_summary_present": strings.TrimSpace(summary) != "",
			}
			if dumpDir != "" {
				_ = os.MkdirAll(dumpDir, 0o755)
				if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
					_ = os.WriteFile(dumpDir+"/narrative_payload.json", b, 0o644)
				}
			}
		}
		if maps, err := env.Queries.ListUnifiedPublicationActionMaps(ctx, publicationID); err == nil && publicationID > 0 {
			out["actions_map_count"] = len(maps)
		}
		if cur, err := env.Queries.GetSellerCurrentUnifiedPublication(ctx, sellerID); err == nil {
			out["current_unified_publication_id"] = cur.PublicationID
			out["is_current"] = cur.PublicationID == publicationID || cur.RunID == runID
		}
		if compat, err := env.Queries.GetSellerCurrentPipelineRunID(ctx, sellerID); err == nil {
			out["compat_pipeline_run_id"] = compat
		}
	}

	printJSON(out)
	return 0
}
