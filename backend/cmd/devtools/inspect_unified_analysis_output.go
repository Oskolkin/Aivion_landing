package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/jackc/pgx/v5/pgtype"
)

func runInspectUnifiedAnalysisOutput(args []string) int {
	fs := flag.NewFlagSet("inspect-unified-analysis-output", flag.ContinueOnError)
	var (
		runID      int64
		showOutput bool
	)
	fs.Int64Var(&runID, "run-id", 0, "unified analysis run id (required)")
	fs.BoolVar(&showOutput, "show-output", false, "print validated output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if runID <= 0 {
		fmt.Fprintln(os.Stderr, "run-id is required")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	repo := unifiedanalysis.NewRepository(env.Queries)
	run, err := repo.GetRun(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get run: %v\n", err)
		return 1
	}

	summary := unifiedRunSummary(run)
	arts, err := repo.ListArtifacts(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list artifacts: %v\n", err)
		return 1
	}
	var validation any
	var outputSummary any
	var validated json.RawMessage
	for _, art := range arts {
		switch art.Kind {
		case unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation:
			_ = json.Unmarshal(art.PayloadJson, &validation)
		case unifiedanalysis.ArtifactUnifiedAnalysisOutputSummary:
			_ = json.Unmarshal(art.PayloadJson, &outputSummary)
		case unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated:
			validated = art.PayloadJson
			summary["output_artifact_hash"] = pgText(art.Hash)
		}
	}
	summary["output_validation"] = validation
	summary["output_summary"] = outputSummary
	raw, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(raw))
	if showOutput && len(validated) > 0 {
		fmt.Println(string(validated))
	}
	return 0
}

func pgText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}
