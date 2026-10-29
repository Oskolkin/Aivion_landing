package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
)

func runCancelUnifiedProviderRun(args []string) int {
	fs := flag.NewFlagSet("cancel-unified-provider-run", flag.ContinueOnError)
	var (
		runID     int64
		execution string
	)
	fs.Int64Var(&runID, "run-id", 0, "unified analysis run id (required)")
	fs.StringVar(&execution, "execution", "direct", "queue|direct cleanup dispatch (default direct)")
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

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{
		ForceEnabled: true,
		WithAsynq:    execution == "queue",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	if err := env.Service.CancelRun(ctx, runID); err != nil {
		fmt.Fprintf(os.Stderr, "cancel run: %v\n", err)
		return 1
	}

	switch execution {
	case "queue":
		if err := enqueueUnifiedAnalysisCleanup(env.Asynq, runID); err != nil {
			fmt.Fprintf(os.Stderr, "enqueue cleanup: %v\n", err)
			return 1
		}
	default:
		if err := env.Service.CleanupRun(ctx, runID); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup run: %v\n", err)
			return 1
		}
	}

	repo := unifiedanalysis.NewRepository(env.Queries)
	final, getErr := repo.GetRun(ctx, runID)
	if getErr != nil {
		fmt.Fprintf(os.Stderr, "get run after cancel: %v\n", getErr)
		return 1
	}
	printUnifiedRunSummary(final)
	return 0
}
