package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval"
)

func evalStore(ctx context.Context) (*eval.Repository, func(), error) {
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		return nil, nil, err
	}
	return &eval.Repository{Pool: pg.Pool}, func() { pg.Close() }, nil
}

func runEvalSuite(args []string) int {
	fs := flag.NewFlagSet("run-eval-suite", flag.ContinueOnError)
	suite := fs.String("suite", eval.KindFactualGolden, "suite key")
	split := fs.String("split", eval.SplitHoldout, "DEV|HOLDOUT|CALIBRATION|REPLAY")
	provider := fs.String("provider", "fake", "fake|openai")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	budget := eval.LoadBudget()
	r := &eval.Runner{Budget: budget, Put: eval.NewMemoryArtifacts().Put}
	switch strings.ToLower(*provider) {
	case "fake":
		r.Executor = eval.NewFrozenFixtureExecutor()
		r.Live = false
	case "openai":
		key, err := eval.MustLiveAPIKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		r.Live = true
		r.Executor = eval.NewFrozenLiveExecutor(key, &r.Budget)
	default:
		fmt.Fprintf(os.Stderr, "unknown provider %q\n", *provider)
		return 2
	}
	ctx := context.Background()
	if st, closer, err := evalStore(ctx); err == nil {
		r.Store = st
		defer closer()
	}
	run, cases, err := r.RunSuite(ctx, *suite, *split, eval.PurposeAcceptance, os.Getenv("GIT_SHA"), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep := eval.BuildReport(eval.Suite{SuiteKey: *suite, Version: "v1"}, run)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"run_id": run.ID, "suite": *suite, "split": *split, "provider": *provider, "models": run.ResolvedModels,
		"hard_verdict": run.Verdict, "semantic_verdict": run.Verdict,
		"critical_failures": run.CriticalFailuresCount, "tokens": rep.Tokens,
		"openai_requests": r.Budget.Requests, "duration_ms": rep.DurationMS,
		"report_artifact": run.ReportArtifactKey, "status": run.Status, "cases": len(cases), "human_review": rep.HumanReview,
	})
	if run.Verdict == eval.VerdictFail {
		return 1
	}
	return 0
}

func runCompareEvalRuns(args []string) int {
	fs := flag.NewFlagSet("compare-eval-runs", flag.ContinueOnError)
	baseline := fs.Int64("baseline", 0, "baseline run id")
	candidate := fs.Int64("candidate", 0, "candidate run id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *baseline == 0 || *candidate == 0 {
		fmt.Fprintln(os.Stderr, "--baseline and --candidate required")
		return 2
	}
	ctx := context.Background()
	st, closer, err := evalStore(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closer()
	b, err := st.GetRun(ctx, *baseline)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	c, err := st.GetRun(ctx, *candidate)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cmp := eval.CompareRuns(b, c)
	baseCases, _ := st.ListCaseRuns(ctx, b.ID)
	candCases, _ := st.ListCaseRuns(ctx, c.ID)
	out := map[string]any{"verdict": cmp, "baseline_cases": len(baseCases), "candidate_cases": len(candCases), "critical_regression": cmp.CriticalRegression}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	if cmp.CriticalRegression {
		return 1
	}
	return 0
}

func runPromoteEvalBaseline(args []string) int {
	fs := flag.NewFlagSet("promote-eval-baseline", flag.ContinueOnError)
	runID := fs.Int64("run-id", 0, "eval run id")
	by := fs.String("by", "", "promoter identity")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *runID == 0 || strings.TrimSpace(*by) == "" {
		fmt.Fprintln(os.Stderr, "--run-id and --by are required; promotion is never automatic")
		return 2
	}
	ctx := context.Background()
	st, closer, err := evalStore(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closer()
	run, err := st.GetRun(ctx, *runID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := eval.PromoteIfGuarded(ctx, st, run, *by); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("promoted run %d as baseline for suite %d split %s by %s\n", run.ID, run.SuiteID, run.Split, *by)
	return 0
}

func runExportEvalReport(args []string) int {
	fs := flag.NewFlagSet("export-eval-report", flag.ContinueOnError)
	out := fs.String("out", "eval-report.json", "output path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	arts := eval.NewMemoryArtifacts()
	r := &eval.Runner{Executor: eval.NewFrozenFixtureExecutor(), Put: arts.Put, Store: eval.NewMemoryStore()}
	run, _, err := r.RunSuite(context.Background(), eval.KindFactualGolden, eval.SplitHoldout, eval.PurposeAcceptance, "", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep := eval.BuildReport(eval.Suite{SuiteKey: eval.KindFactualGolden, Version: "v1"}, run)
	b, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(*out)
	return 0
}

func runRecordHumanReview(args []string) int {
	fs := flag.NewFlagSet("record-human-review", flag.ContinueOnError)
	caseRun := fs.Int64("case-run-id", 0, "eval_case_runs.id")
	reviewer := fs.String("reviewer", "", "human reviewer")
	status := fs.String("status", "PENDING", "PENDING|PASS|FAIL|NEEDS_DISCUSSION")
	notes := fs.String("notes", "", "notes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *caseRun == 0 || strings.TrimSpace(*reviewer) == "" {
		fmt.Fprintln(os.Stderr, "--case-run-id and --reviewer required")
		return 2
	}
	if err := eval.RejectAgentHumanPass(*reviewer, *status); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx := context.Background()
	st, closer, err := evalStore(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closer()
	h, err := eval.RecordHumanReview(ctx, st, eval.HumanReview{CaseRunID: *caseRun, Reviewer: *reviewer, Status: *status, Notes: *notes})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("human review persisted case_run=%d reviewer=%s status=%s id=%d\n", h.CaseRunID, h.Reviewer, h.Status, h.ID)
	return 0
}
