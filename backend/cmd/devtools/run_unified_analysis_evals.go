package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval/scenarios"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const evalSellerAccountID = scenarios.SellerAccountID // reserved fictional seller for Stage 4 eval packs

var mandatoryEvalScenarioNames = []string{
	"scenario_healthy_no_actions",
	"scenario_insufficient_data",
}

func runUnifiedAnalysisEvals(args []string) int {
	fs := flag.NewFlagSet("run-unified-analysis-evals", flag.ContinueOnError)
	var (
		scenarioRaw    string
		force          bool
		wait           bool
		timeoutRaw     string
		cleanup        bool
		debugArtifacts bool
		execution      string
		model          string
	)
	fs.StringVar(&scenarioRaw, "scenario", "", "scenario name: healthy_no_actions|insufficient_data|all (required)")
	fs.BoolVar(&force, "force", false, "force new run when active run exists")
	fs.BoolVar(&wait, "wait", true, "wait until publication_ready / publication_blocked / failed")
	fs.StringVar(&timeoutRaw, "timeout", "45m", "wait timeout")
	fs.BoolVar(&cleanup, "cleanup", true, "enable provider cleanup (worker queue handles cleanup in queue mode)")
	fs.BoolVar(&debugArtifacts, "debug-artifacts", true, "persist debug artifacts")
	fs.StringVar(&execution, "execution", "queue", "queue only for evals (default queue)")
	fs.StringVar(&model, "model", "", "optional model override (devtools only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	scenarioNames, err := resolveEvalScenarioNames(scenarioRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
	if err != nil || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeout: expected positive duration, e.g. 45m")
		return 2
	}
	execution = strings.ToLower(strings.TrimSpace(execution))
	if execution != "queue" {
		fmt.Fprintln(os.Stderr, "execution: evals support queue only (do not call StartRun/PollRun/ValidateOutput/CleanupRun directly)")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{
		ForceEnabled:   true,
		DebugArtifacts: &debugArtifacts,
		CleanupEnabled: &cleanup,
		WithAsynq:      true,
		PromptProfile:  "analysis",
		ModelOverride:  model,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	warnIfProviderDisabledInEnv()

	if err := ensureEvalSellerAccount(ctx, env.Postgres.Pool); err != nil {
		fmt.Fprintf(os.Stderr, "ensure eval seller: %v\n", err)
		return 1
	}

	metricDate := scenarios.EvalMetricDate()
	packStorage := uapack.NewS3PackStorage(env.S3)
	builder := uapack.NewPackBuilder(uapack.DefaultBuildOptions())
	validator := uapack.NewValidator(uapack.DefaultBuildOptions())
	repo := unifiedanalysis.NewRepository(env.Queries)

	exitCode := 0
	for i, scenarioName := range scenarioNames {
		def, ok := scenarios.ByName(scenarioName)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown scenario %q\n", scenarioName)
			return 2
		}
		runForce := force || i > 0
		code, err := runOneUnifiedAnalysisEval(ctx, evalRunParams{
			env:         env,
			repo:        repo,
			packStorage: packStorage,
			builder:     builder,
			validator:   validator,
			def:         def,
			metricDate:  metricDate,
			force:       runForce,
			wait:        wait,
			timeout:     timeout,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenario %s: %v\n", scenarioName, err)
		}
		if code != 0 {
			exitCode = code
		}
	}
	return exitCode
}

type evalRunParams struct {
	env         *unifiedDevtoolEnv
	repo        *unifiedanalysis.Repository
	packStorage uapack.PackStorage
	builder     *uapack.PackBuilder
	validator   *uapack.Validator
	def         scenarios.Definition
	metricDate  time.Time
	force       bool
	wait        bool
	timeout     time.Duration
}

func buildScenarioSnapshot(def scenarios.Definition, tb *scenarioTB) (*analysisdata.AnalysisDataSnapshot, error) {
	var snap *analysisdata.AnalysisDataSnapshot
	panicked := true
	func() {
		defer func() {
			if recover() != nil {
				return
			}
			panicked = false
		}()
		snap = def.Snapshot(tb)
	}()
	if tb.failed() || snap == nil {
		return nil, fmt.Errorf("build scenario snapshot %s", def.Name)
	}
	if panicked {
		return nil, fmt.Errorf("build scenario snapshot %s", def.Name)
	}
	return snap, nil
}

func runOneUnifiedAnalysisEval(ctx context.Context, p evalRunParams) (int, error) {
	tb := &scenarioTB{name: p.def.Name}
	snap, snapErr := buildScenarioSnapshot(p.def, tb)
	if snapErr != nil {
		return 1, snapErr
	}

	result, err := p.builder.Build(snap)
	if err != nil {
		return 1, fmt.Errorf("build pack: %w", err)
	}
	report, err := p.validator.ValidateZip(result.ZipBytes)
	if err != nil {
		return 1, fmt.Errorf("validate pack zip: %w", err)
	}
	result.Validation = report
	if !report.Valid {
		return 1, fmt.Errorf("pack validation failed (%d issues)", len(report.Issues))
	}

	storageKey, err := p.packStorage.Put(ctx, result)
	if err != nil {
		return 1, fmt.Errorf("store pack: %w", err)
	}

	run, err := p.env.Service.CreateRun(ctx, evalSellerAccountID, p.metricDate, p.force)
	if err != nil {
		return 1, fmt.Errorf("create run: %w", err)
	}
	fmt.Fprintf(os.Stderr, "scenario=%s seller_id=%d run_id=%d execution=queue prompt_profile=analysis\n",
		p.def.Name, evalSellerAccountID, run.ID)

	if err := injectScenarioPack(ctx, p.repo, run.ID, result, storageKey); err != nil {
		return 1, fmt.Errorf("inject pack: %w", err)
	}

	if err := enqueueUnifiedAnalysisStart(p.env.Asynq, run.ID); err != nil {
		return 1, fmt.Errorf("enqueue start: %w", err)
	}

	var finalRun *unifiedanalysis.Run
	if !p.wait {
		finalRun = run
		printUnifiedEvalSummary(finalRun, p.def, nil, nil)
		return 0, nil
	}

	finalRun, err = waitUnifiedRunDBProfile(ctx, p.repo, run.ID, p.timeout, true)
	if err != nil {
		printUnifiedEvalSummary(finalRun, p.def, nil, nil)
		return 1, err
	}

	var (
		output     *contract.UnifiedAnalysisOutputV1
		evalReport *eval.EvalReport
	)
	output, err = loadValidatedOutput(ctx, p.repo, run.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load validated output: %v\n", err)
	} else if output != nil {
		report := eval.Evaluate(output, p.def.Checks)
		evalReport = &report
		if report.Passed {
			fmt.Fprintf(os.Stderr, "eval %s: PASS\n", p.def.Name)
		} else {
			fmt.Fprintf(os.Stderr, "eval %s: FAIL (%d checks)\n", p.def.Name, len(report.Failures))
			for _, f := range report.Failures {
				fmt.Fprintf(os.Stderr, "  [%s] %s\n", f.Check, f.Message)
			}
		}
	}

	printUnifiedEvalSummary(finalRun, p.def, output, evalReport)

	code := 0
	if finalRun.Status == unifiedanalysis.StatusFailed || finalRun.Status == unifiedanalysis.StatusPublicationBlocked {
		code = 1
	}
	if evalReport != nil && !evalReport.Passed {
		code = 1
	}
	return code, nil
}

func injectScenarioPack(ctx context.Context, repo *unifiedanalysis.Repository, runID int64, result *uapack.BuildResult, storageKey string) error {
	if _, err := repo.TransitionStatus(ctx, runID, unifiedanalysis.StatusCreated, unifiedanalysis.StatusPackBuilding); err != nil {
		return fmt.Errorf("transition created->pack_building: %w", err)
	}
	if _, err := repo.TransitionStatus(ctx, runID, unifiedanalysis.StatusPackBuilding, unifiedanalysis.StatusPackValidating); err != nil {
		return fmt.Errorf("transition pack_building->pack_validating: %w", err)
	}

	archiveSHA := sha256.Sum256(result.ZipBytes)
	compressed := int64(len(result.ZipBytes))
	uncompressed := result.Validation.UncompressedBytes
	if uncompressed == 0 {
		uncompressed = compressed
	}
	if _, err := repo.SavePack(ctx, runID, dbgen.SaveUnifiedAnalysisPackParams{
		PackID:                nullablePGText(result.PackID),
		PackHash:              nullablePGText(result.PackHash),
		PackStorageKey:        nullablePGText(storageKey),
		PackArchiveSha256:     nullablePGText(hex.EncodeToString(archiveSHA[:])),
		PackCompressedBytes:   pgtype.Int8{Int64: compressed, Valid: true},
		PackUncompressedBytes: pgtype.Int8{Int64: uncompressed, Valid: true},
		DataPackSchemaVersion: nullablePGText(result.SchemaVersion),
	}); err != nil {
		return fmt.Errorf("save pack: %w", err)
	}
	if _, err := repo.TransitionStatus(ctx, runID, unifiedanalysis.StatusPackValidating, unifiedanalysis.StatusPackReady); err != nil {
		return fmt.Errorf("transition pack_validating->pack_ready: %w", err)
	}
	return nil
}

func loadValidatedOutput(ctx context.Context, repo *unifiedanalysis.Repository, runID int64) (*contract.UnifiedAnalysisOutputV1, error) {
	arts, err := repo.ListArtifacts(ctx, runID)
	if err != nil {
		return nil, err
	}
	for _, art := range arts {
		if art.Kind != unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated {
			continue
		}
		var out contract.UnifiedAnalysisOutputV1
		if err := json.Unmarshal(art.PayloadJson, &out); err != nil {
			return nil, fmt.Errorf("unmarshal validated output: %w", err)
		}
		return &out, nil
	}
	return nil, nil
}

func printUnifiedEvalSummary(run *unifiedanalysis.Run, def scenarios.Definition, output *contract.UnifiedAnalysisOutputV1, evalReport *eval.EvalReport) {
	summary := unifiedRunSummary(run)
	summary["scenario"] = def.Name
	summary["eval_seller_account_id"] = evalSellerAccountID
	if output != nil {
		summary["analysis_outcome"] = output.AnalysisOutcome
		summary["actions_count"] = len(output.Actions)
	}
	if evalReport != nil {
		summary["eval_passed"] = evalReport.Passed
		if len(evalReport.Failures) > 0 {
			summary["eval_failures"] = evalReport.Failures
		}
	}
	raw, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(raw))
}

func resolveEvalScenarioNames(raw string) ([]string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return nil, errors.New("scenario is required (healthy_no_actions|insufficient_data|all)")
	}
	if raw == "all" {
		return append([]string(nil), mandatoryEvalScenarioNames...), nil
	}
	name := normalizeEvalScenarioName(raw)
	if _, ok := scenarios.ByName(name); !ok {
		return nil, fmt.Errorf("unknown scenario %q (supported short names: healthy_no_actions, insufficient_data; --scenario=all runs mandatory B/C scenarios)", raw)
	}
	return []string{name}, nil
}

func normalizeEvalScenarioName(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "scenario_") {
		return raw
	}
	return "scenario_" + raw
}

func ensureEvalSellerAccount(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seller_accounts WHERE id = $1)`, evalSellerAccountID).Scan(&exists); err != nil {
		return fmt.Errorf("check seller: %w", err)
	}
	if exists {
		return nil
	}

	var userID int64
	email := fmt.Sprintf("unified-analysis-eval-%d@dev.local", evalSellerAccountID)
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash) VALUES ($1, 'eval-dev-hash') RETURNING id`, email).Scan(&userID); err != nil {
		return fmt.Errorf("insert eval user: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO seller_accounts (id, user_id, name, status)
VALUES ($1, $2, 'unified-analysis-eval', 'active')`, evalSellerAccountID, userID); err != nil {
		return fmt.Errorf("insert eval seller: %w", err)
	}
	_, _ = pool.Exec(ctx, `
SELECT setval(pg_get_serial_sequence('seller_accounts', 'id'), GREATEST((SELECT COALESCE(MAX(id), 1) FROM seller_accounts), $1))`, evalSellerAccountID)
	return nil
}

func nullablePGText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

type scenarioTB struct {
	name    string
	failed_ bool
}

func (t *scenarioTB) failed() bool { return t.failed_ }

func (t *scenarioTB) Helper() {}

func (t *scenarioTB) Fatalf(format string, args ...any) {
	t.failed_ = true
	panic(fmt.Sprintf(format, args...))
}
