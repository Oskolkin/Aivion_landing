package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
)

func runUnifiedAnalysis(args []string) int {
	fs := flag.NewFlagSet("run-unified-analysis", flag.ContinueOnError)
	var (
		sellerID            int64
		dateModeRaw         string
		metricDateRaw       string
		publishAsCurrentRaw string
		force               bool
		wait                bool
		timeoutRaw          string
		cleanup             bool
		debugArtifacts      bool
		execution           string
		model               string
		timezone            string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&dateModeRaw, "date-mode", "current", "current|fixed (default current)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required for fixed; ignored for current)")
	fs.StringVar(&publishAsCurrentRaw, "publish-as-current", "", "true|false (default: true for current, false for fixed)")
	fs.BoolVar(&force, "force", false, "force new run when active run exists")
	fs.BoolVar(&wait, "wait", true, "wait until publication_ready / publication_blocked / failed")
	fs.StringVar(&timeoutRaw, "timeout", "45m", "wait timeout")
	fs.BoolVar(&cleanup, "cleanup", true, "run provider cleanup after completion (direct mode)")
	fs.BoolVar(&debugArtifacts, "debug-artifacts", true, "persist debug artifacts")
	fs.StringVar(&execution, "execution", "queue", "queue|direct (default queue)")
	fs.StringVar(&model, "model", "", "optional model override (devtools only)")
	fs.StringVar(&timezone, "timezone", analysisdata.DefaultTimezone, "seller timezone for current-mode calendar date")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	dateMode := analysisdata.DateMode(strings.ToLower(strings.TrimSpace(dateModeRaw)))
	if dateMode == "" {
		dateMode = analysisdata.DateModeCurrent
	}
	if dateMode != analysisdata.DateModeCurrent && dateMode != analysisdata.DateModeFixed {
		fmt.Fprintln(os.Stderr, "date-mode: use current or fixed")
		return 2
	}
	if dateMode == analysisdata.DateModeFixed && strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "metric-date is required for date-mode=fixed (YYYY-MM-DD)")
		return 2
	}
	if dateMode == analysisdata.DateModeCurrent && strings.TrimSpace(metricDateRaw) != "" {
		fmt.Fprintln(os.Stderr, "warning: metric-date is ignored for date-mode=current; ResolveAnalysisMetricDate picks latest complete data")
	}

	var publishAsCurrent *bool
	switch strings.ToLower(strings.TrimSpace(publishAsCurrentRaw)) {
	case "":
		publishAsCurrent = nil
	case "true", "1", "yes":
		v := true
		publishAsCurrent = &v
	case "false", "0", "no":
		v := false
		publishAsCurrent = &v
	default:
		fmt.Fprintln(os.Stderr, "publish-as-current: use true or false")
		return 2
	}

	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
	if err != nil || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeout: expected positive duration, e.g. 45m")
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
		PromptProfile:  "analysis",
		ModelOverride:  model,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	coverage := analysisdata.NewSQLCoverageSource(env.Postgres.Pool)
	resolution, err := analysisdata.ResolveAnalysisMetricDate(ctx, coverage, analysisdata.ResolveAnalysisMetricDateInput{
		SellerAccountID:     sellerID,
		DateMode:            dateMode,
		RequestedMetricDate: metricDateRaw,
		Timezone:            timezone,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve metric date: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	fmt.Fprintln(os.Stderr, "date_resolution=")
	_ = enc.Encode(resolution)
	if !resolution.Allowed {
		fmt.Fprintf(os.Stderr, "date resolution rejected: %s (%s)\n", resolution.ResolutionReason, resolution.ErrorCode)
		return 1
	}
	metricDate, err := time.Parse("2006-01-02", resolution.EffectiveMetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "effective_metric_date: %v\n", err)
		return 1
	}

	if execution == "queue" {
		warnIfProviderDisabledInEnv()
	}

	run, err := env.Service.CreateRunWithOptions(ctx, sellerID, metricDate, unifiedanalysis.CreateRunOptions{
		Force:            force,
		DateResolution:   resolution,
		PublishAsCurrent: publishAsCurrent,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create run: %v\n", err)
		return 1
	}
	effectivePublish := resolution.PublishAsCurrentDefault
	if publishAsCurrent != nil {
		effectivePublish = *publishAsCurrent
	}
	fmt.Fprintf(os.Stderr, "run_id=%d execution=%s date_mode=%s effective_metric_date=%s publish_as_current=%v prompt_profile=analysis\n",
		run.ID, execution, dateMode, resolution.EffectiveMetricDate, effectivePublish)

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
		finalRun, err = waitUnifiedRunDBProfile(ctx, repo, run.ID, timeout, true)
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
		if err == nil && finalRun != nil && finalRun.Status == unifiedanalysis.StatusOutputValidating {
			if _, valErr := env.Service.ValidateOutput(ctx, run.ID); valErr != nil {
				fmt.Fprintf(os.Stderr, "validate: %v\n", valErr)
				err = valErr
			} else {
				repo := unifiedanalysis.NewRepository(env.Queries)
				finalRun, _ = repo.GetRun(ctx, run.ID)
			}
		}
		if err == nil && cleanup && finalRun != nil {
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
	if finalRun != nil && (finalRun.Status == unifiedanalysis.StatusFailed || finalRun.Status == unifiedanalysis.StatusPublicationBlocked) {
		return 1
	}

	// Explicit publish respecting publish_as_current (also covers cases where
	// auto-publish was disabled, left the run at publication_ready, or returned stale
	// for an older metric_date before AllowOlderMetricDate was applied).
	if wait && finalRun != nil {
		switch finalRun.Status {
		case unifiedanalysis.StatusPublicationReady, unifiedanalysis.StatusPublished:
			pubSvc := buildPublicationService(env)
			result, pubErr := pubSvc.PublishRun(ctx, publication.PublishRequest{
				RunID:                finalRun.ID,
				Actor:                "devtool:run-unified-analysis",
				PublishAsCurrent:     &effectivePublish,
				AllowOlderMetricDate: effectivePublish,
			})
			if pubErr != nil {
				fmt.Fprintf(os.Stderr, "publish: %v\n", pubErr)
				printJSON(publishResultSummary(result))
				return 1
			}
			fmt.Fprintln(os.Stderr, "publication=")
			printJSON(publishResultSummary(result))
			if result != nil && result.Outcome == publication.OutcomeStale {
				return 1
			}
		}
	}
	return 0
}
