package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
)

func runInspectUnifiedProviderRun(args []string) int {
	fs := flag.NewFlagSet("inspect-unified-provider-run", flag.ContinueOnError)
	var runID int64
	fs.Int64Var(&runID, "run-id", 0, "unified analysis run id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if runID <= 0 {
		fmt.Fprintln(os.Stderr, "run-id is required and must be > 0")
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
	if run.Mode != unifiedanalysis.ModeUnifiedAnalysis {
		fmt.Fprintf(os.Stderr, "run %d is not mode=%q (got %q)\n", runID, unifiedanalysis.ModeUnifiedAnalysis, run.Mode)
		return 1
	}

	out := unifiedRunSummary(run)
	raw, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(raw))
	return 0
}
