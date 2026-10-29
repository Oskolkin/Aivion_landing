package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

func runInspectAIPipeline(args []string) int {
	fs := flag.NewFlagSet("inspect-ai-pipeline", flag.ContinueOnError)
	var pipelineRunID int64
	var includePayload bool
	fs.Int64Var(&pipelineRunID, "pipeline-run-id", 0, "ai_pipeline_runs.id (required)")
	fs.BoolVar(&includePayload, "include-payload", false, "include full artifact payloads (default false)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if pipelineRunID <= 0 {
		fmt.Fprintln(os.Stderr, "pipeline-run-id is required and must be > 0")
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

	store := aipipeline.NewStore(dbgen.New(postgres.Pool))
	summary, err := aipipeline.InspectPipelineRun(ctx, store, pipelineRunID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if includePayload {
		artifacts, err := store.ListArtifacts(ctx, pipelineRunID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		out := map[string]any{"summary": summary, "artifact_payloads": artifacts}
		raw, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(raw))
		return 0
	}
	raw, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(raw))
	return 0
}
