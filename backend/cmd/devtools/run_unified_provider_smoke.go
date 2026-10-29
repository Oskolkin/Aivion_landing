package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
)

func runUnifiedProviderSmoke(args []string) int {
	fs := flag.NewFlagSet("run-unified-provider-smoke", flag.ContinueOnError)
	var (
		sellerID       int64
		metricDateRaw  string
		force          bool
		wait           bool
		timeoutRaw     string
		cleanup        bool
		debugArtifacts bool
		execution      string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.BoolVar(&force, "force", false, "force new run when active run exists")
	fs.BoolVar(&wait, "wait", true, "wait until run reaches terminal status")
	fs.StringVar(&timeoutRaw, "timeout", "30m", "wait timeout")
	fs.BoolVar(&cleanup, "cleanup", true, "run provider cleanup after completion (direct mode)")
	fs.BoolVar(&debugArtifacts, "debug-artifacts", true, "persist debug artifacts")
	fs.StringVar(&execution, "execution", "queue", "queue|direct (default queue)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "metric-date is required (YYYY-MM-DD)")
		return 2
	}
	metricDate, err := parseMetricDateCLI(metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
	if err != nil || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeout: expected positive duration, e.g. 30m")
		return 2
	}
	execution = strings.ToLower(strings.TrimSpace(execution))
	if execution != "queue" && execution != "direct" {
		fmt.Fprintln(os.Stderr, "execution: use queue or direct")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{
		ForceEnabled:   true,
		DebugArtifacts: &debugArtifacts,
		CleanupEnabled: &cleanup,
		WithAsynq:      execution == "queue",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	if execution == "queue" {
		warnIfProviderDisabledInEnv()
	}

	run, err := env.Service.CreateRun(ctx, sellerID, metricDate, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create run: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "run_id=%d execution=%s\n", run.ID, execution)

	var finalRun *unifiedanalysis.Run
	switch execution {
	case "queue":
		if err := enqueueUnifiedAnalysisStart(env.Asynq, run.ID); err != nil {
			fmt.Fprintf(os.Stderr, "enqueue start: %v\n", err)
			return 1
		}
		if !wait {
			printUnifiedRunSummary(run)
			return 0
		}
		repo := unifiedanalysis.NewRepository(env.Queries)
		finalRun, err = waitUnifiedRunDB(ctx, repo, run.ID, timeout)
	default:
		if err := env.Service.StartRun(ctx, run.ID); err != nil {
			fmt.Fprintf(os.Stderr, "start run: %v\n", err)
			return 1
		}
		if !wait {
			repo := unifiedanalysis.NewRepository(env.Queries)
			finalRun, _ = repo.GetRun(ctx, run.ID)
			printUnifiedRunSummary(finalRun)
			return 0
		}
		finalRun, err = directPollUnifiedRun(ctx, env.Service, env.UAConfig, run.ID, timeout)
		if err == nil && cleanup && finalRun != nil && finalRun.Status == unifiedanalysis.StatusProviderCompleted {
			if cleanupErr := env.Service.CleanupRun(ctx, run.ID); cleanupErr != nil {
				fmt.Fprintf(os.Stderr, "cleanup: %v\n", cleanupErr)
			} else {
				repo := unifiedanalysis.NewRepository(env.Queries)
				finalRun, _ = repo.GetRun(ctx, run.ID)
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "wait: %v\n", err)
		if finalRun != nil {
			printUnifiedRunSummary(finalRun)
		}
		return 1
	}
	printUnifiedRunSummary(finalRun)
	if finalRun != nil && finalRun.Status == unifiedanalysis.StatusFailed {
		return 1
	}
	return 0
}
