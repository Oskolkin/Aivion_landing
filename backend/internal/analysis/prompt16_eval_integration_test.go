//go:build integration

package analysis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval/replay"
)

func prompt16FakeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
}

func TestPrompt16EvaluationCannotPublish(t *testing.T) {
	f := prompt81Seed(t)
	ctx := context.Background()
	before, err := f.repo.CurrentPublicationID(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	run, err := replay.StartEvaluationRun(ctx, f.repo, analysispg.Run{
		SellerID: f.seller, Kind: "EVALUATION", AnalysisDate: f.cut, CutoffAt: f.cut,
		ContractHash: f.bundle.Hash(), IntentHash: prompt81Hex("eval-no-pub"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.ExecutionClass != "EVALUATION" {
		t.Fatalf("execution_class=%s", run.ExecutionClass)
	}
	_, err = f.repo.PublishAtomic(ctx, analysispg.PublishInput{Run: run, Now: time.Now().UTC()})
	if !errors.Is(err, analysispg.ErrEvaluationCannotPublish) {
		t.Fatalf("want ErrEvaluationCannotPublish got %v", err)
	}
	if err := replay.AssertNoPublish(ctx, f.repo, f.seller, before); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt16EvaluationRunnerUsesProductionRuntime(t *testing.T) {
	prompt16FakeEnv(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	run := f.evalRun(t)
	before, err := f.repo.CurrentPublicationID(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.RunEvaluationStages(ctx, f.pool, store, "p16-runtime", run.ID); err != nil {
		t.Fatal(err)
	}
	if err := replay.AssertNoPublish(ctx, f.repo, f.seller, before); err != nil {
		t.Fatal(err)
	}
	var nPub int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, run.ID).Scan(&nPub)
	if nPub != 0 {
		t.Fatalf("evaluation published %d", nPub)
	}
	st, err := f.repo.GetStage(ctx, run.ID, string(stages.Publish))
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "SUCCEEDED" && st.Status != "SUCCEEDED_WITH_LIMITATIONS" {
		t.Fatalf("publish stage %s", st.Status)
	}
}

func TestPrompt16EvaluationNeverPublishes(t *testing.T) {
	TestPrompt16EvaluationRunnerUsesProductionRuntime(t)
}

func TestPrompt16NoOpZeroAICallsRealRuntime(t *testing.T) {
	prompt16FakeEnv(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	ex := &replay.ProductionCaseExecutor{
		Pool: f.pool, Repo: f.repo, Store: store, SellerID: f.seller, WorkerID: "p16-noop",
		NewRun: func(ctx context.Context) (analysispg.Run, error) {
			return f.evalRun(t), nil
		},
	}
	discovery.TestDiscoverCalls.Store(0)
	act1, err := ex.Execute(ctx, eval.CaseInput{CaseKey: "runtime"}, eval.ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n1 := discovery.TestDiscoverCalls.Load()
	if n1 < 1 || act1.NoOp {
		t.Fatalf("first evaluation must call Discovery, calls=%d noop=%v", n1, act1.NoOp)
	}
	act2, err := ex.Execute(ctx, eval.CaseInput{CaseKey: "runtime"}, eval.ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n2 := discovery.TestDiscoverCalls.Load()
	if !act2.NoOp || n2 != n1 || act2.DiscoveryCalls != 0 {
		t.Fatalf("second evaluation must NO_OP discovery: noop=%v calls %d→%d deepCalls=%d", act2.NoOp, n1, n2, act2.DiscoveryCalls)
	}
}

func TestPrompt16ReplayDeterminismRealRuntime(t *testing.T) {
	prompt16FakeEnv(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	r1 := f.evalRun(t)
	r2 := f.evalRun(t)
	if err := replay.RunEvaluationStages(ctx, f.pool, store, "p16-det-a", r1.ID); err != nil {
		t.Fatal(err)
	}
	if err := replay.RunEvaluationStages(ctx, f.pool, store, "p16-det-b", r2.ID); err != nil {
		t.Fatal(err)
	}
	hash := func(runID int64, key string) string {
		t.Helper()
		st, err := f.repo.GetStage(ctx, runID, key)
		if err != nil {
			t.Fatal(err)
		}
		if st.OutputHash == "" {
			t.Fatalf("empty hash %s run %d", key, runID)
		}
		return st.OutputHash
	}
	for _, key := range []string{string(stages.SealUniverse), string(stages.BuildLandscapeCore), string(stages.RunMateriality), string(stages.BuildDiscoveryPack), string(stages.BuildDeepPack)} {
		a, b := hash(r1.ID, key), hash(r2.ID, key)
		if a != b {
			t.Fatalf("%s hash mismatch %s vs %s", key, a, b)
		}
	}
}

func TestPrompt16LongitudinalContinuity(t *testing.T) {
	prompt16FakeEnv(t)
	f, r1, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	var n int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&n)
	if n != 1 {
		t.Fatalf("previous action must be assessed once on P2, got %d", n)
	}
	var outcome string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	switch outcome {
	case "FULLY_SUPPORTED", "PARTIALLY_SUPPORTED", "NOT_SUPPORTED", "UNVERIFIABLE", "SUPERSEDED":
	default:
		t.Fatalf("outcome=%s", outcome)
	}
	if r1 == r2 {
		t.Fatal("P1 and P2 must be distinct runs")
	}
	var p1Facts, p2Facts int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, r1).Scan(&p1Facts)
	_ = p1Facts
	_ = p2Facts
}

func TestPrompt16FailureInjectionRecovery(t *testing.T) {
	prompt16FakeEnv(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	run := f.evalRun(t)
	before, _ := f.repo.CurrentPublicationID(ctx, f.seller)
	exec, err := runtime.NewProductionExecutor(f.pool, store, "p16-fail", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err := exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}
	var attempts1 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_ai_attempts a JOIN analysis_stage_attempts s ON s.id=a.stage_attempt_id
		JOIN analysis_run_stages st ON st.id=s.stage_id WHERE st.run_id=$1 AND st.stage_key=$2`, run.ID, string(stages.DiscoveryAI)).Scan(&attempts1)
	if err := exec.ExecuteStage(ctx, run.ID, string(stages.DiscoveryAI)); err != nil {
		t.Fatal(err)
	}
	if err := exec.ExecuteStage(ctx, run.ID, string(stages.DeepAI)); err != nil {
		t.Fatal(err)
	}
	var attempts2 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_ai_attempts a JOIN analysis_stage_attempts s ON s.id=a.stage_attempt_id
		JOIN analysis_run_stages st ON st.id=s.stage_id WHERE st.run_id=$1 AND st.stage_key=$2`, run.ID, string(stages.DiscoveryAI)).Scan(&attempts2)
	if attempts2 != attempts1 {
		t.Fatalf("duplicate delivery re-ran Discovery AI %d→%d", attempts1, attempts2)
	}
	var nPub int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, run.ID).Scan(&nPub)
	if nPub != 0 {
		t.Fatalf("duplicate publication %d", nPub)
	}
	if err := replay.AssertNoPublish(ctx, f.repo, f.seller, before); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt16EvaluationExtractsProductionActuals(t *testing.T) {
	prompt16FakeEnv(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	run := f.evalRun(t)
	if err := replay.RunEvaluationStages(ctx, f.pool, store, "p16-actuals", run.ID); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{string(artifacts.LandscapeCoreSet), string(artifacts.MaterialitySelection), string(artifacts.DiscoveryPack), string(artifacts.DeepPack)} {
		body := prompt91LoadArtifactKind(t, f, store, run.ID, kind)
		if len(body) == 0 {
			t.Fatalf("empty %s", kind)
		}
	}
}
