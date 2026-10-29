package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

func runInspectActionsQuality(args []string) int {
	fs := flag.NewFlagSet("inspect-actions-quality", flag.ContinueOnError)
	var (
		sellerID      int64
		runID         int64
		pipelineRunID int64
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id")
	fs.Int64Var(&runID, "run-id", 0, "action_generation_runs.id")
	fs.Int64Var(&pipelineRunID, "pipeline-run-id", 0, "ai_pipeline_runs.id (uses linked actions_run_id)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := devseed.AssertDevAIPipelineAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer postgres.Close()

	if runID <= 0 && pipelineRunID > 0 {
		var linked *int64
		err = postgres.Pool.QueryRow(ctx, `
			SELECT actions_run_id FROM ai_pipeline_runs WHERE id = $1
		`, pipelineRunID).Scan(&linked)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pipeline run %d: %v\n", pipelineRunID, err)
			return 1
		}
		if linked == nil || *linked <= 0 {
			fmt.Fprintf(os.Stderr, "pipeline run %d has no actions_run_id\n", pipelineRunID)
			return 1
		}
		runID = *linked
	}
	if runID <= 0 && sellerID > 0 {
		err = postgres.Pool.QueryRow(ctx, `
			SELECT COALESCE(p.actions_run_id, 0)
			FROM ai_pipeline_runs p
			WHERE p.seller_account_id = $1 AND p.published_at IS NOT NULL
			ORDER BY p.published_at DESC, p.id DESC
			LIMIT 1
		`, sellerID).Scan(&runID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "latest published pipeline: %v\n", err)
			return 1
		}
	}
	if runID <= 0 {
		fmt.Fprintln(os.Stderr, "run-id, pipeline-run-id, or seller-id with published pipeline is required")
		return 2
	}

	rows, err := postgres.Pool.Query(ctx, `
		SELECT id, seller_account_id, run_id, action_key, type, category, title, reason,
		       source_candidate_ids_json, evidence_fact_ids_json, affected_entities_json, payload_json, metric_date
		FROM action_items
		WHERE run_id = $1
		ORDER BY id
	`, runID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer rows.Close()

	inputs := make([]actions.ActionQualityInspectInput, 0)
	for rows.Next() {
		var (
			item      actions.ActionQualityInspectInput
			runRef    *int64
			srcRaw    []byte
			factRaw   []byte
			affectRaw []byte
			metric    *time.Time
		)
		if err := rows.Scan(
			&item.ID, &item.SellerAccountID, &runRef, &item.ActionKey,
			&item.Type, &item.Category, &item.Title, &item.Reason,
			&srcRaw, &factRaw, &affectRaw, &item.PayloadJSON, &metric,
		); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		item.RunID = runRef
		item.MetricDate = metric
		item.SourceCandidateIDs = parseStringJSONArray(srcRaw)
		item.EvidenceFactIDs = parseStringJSONArray(factRaw)
		item.AffectedEntitiesCount = actions.CountAffectedEntitiesFromJSON(affectRaw)
		inputs = append(inputs, item)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	report := actions.InspectPublishedActionQuality(inputs)
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
	if report.Failed > 0 {
		return 1
	}
	return 0
}

func parseStringJSONArray(raw []byte) []string {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
