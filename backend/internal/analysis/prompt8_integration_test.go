//go:build integration

package analysis_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type prompt8Clock struct{ now time.Time }

func (c *prompt8Clock) Now() time.Time { return c.now.UTC() }

type prompt8Rand struct{}

func (prompt8Rand) Int63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return 0
}

func prompt8Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func prompt8Seller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var user, seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'x','active')RETURNING id`,
		fmt.Sprintf("p8_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'p8','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	return seller
}

func prompt8SeedDomainRun(t *testing.T, pool *pgxpool.Pool, seller int64, domain string, at time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	var parent, domainRun int64
	if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
		VALUES($1,'MANUAL','COMPLETED','prompt8',$2,$2)RETURNING id`, seller, at).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at,finished_at)
		VALUES($1,$2,$3,'COMPLETED',$4,$4)RETURNING id`, parent, seller, domain, at).Scan(&domainRun); err != nil {
		t.Fatal(err)
	}
	return domainRun
}

func prompt8SetTestStageHashes(t *testing.T, repo *analysispg.Repository, runID int64) {
	t.Helper()
	ctx := context.Background()
	list, err := repo.ListStages(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	h := strings.Repeat("a", 64)
	for _, s := range list {
		if err = repo.SetStageInputHash(ctx, s.ID, h); err != nil {
			t.Fatal(err)
		}
	}
}

func prompt8ResolvedManifest(seller int64, cut time.Time, entityState string) resolve.Manifest {
	return resolve.Manifest{
		Version: resolve.ManifestVersion, SellerAccountID: seller,
		AnalysisDate: canonicaljson.Date(cut.AddDate(0, 0, -1)), CutoffAt: cut,
		RunKind: "EVALUATION", IntentHash: runtime.StandardIntentHash(),
		Policies: factualapi.CurrentPolicies(),
		Sources: []resolve.SourceBinding{{
			Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", SourceBindingKey: "CATALOG.SKU_UNIVERSE",
			SourceState: "SUCCESS", SourceContractKey: "CATALOG", SourceContractVersion: "v1",
			SourceContractHash: factualapi.StableHash("catalog-contract", entityState),
			WatermarkHash:      factualapi.StableHash("wm", entityState),
			SourceHash:         factualapi.StableHash("src", entityState),
			BindingHash:        factualapi.StableHash("bind", entityState),
		}},
		Capabilities: []resolve.CapabilitySnapshot{{
			Key: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY",
			ContractState: "ACTIVE", AccessState: "ACCESSIBLE", SourceState: "SUCCESS", SemanticState: "VALIDATED",
			CoverageState: "FULL", TemporalState: "CURRENT", HistoryState: "PARTIAL", PopulationState: "NON_EMPTY",
			PolicyVersion: "capability_policy_v3", AssessedAt: cut.Add(-time.Minute),
			AssessmentHash: factualapi.StableHash("cap", entityState),
		}},
	}
}

func TestPrompt8UniverseSealIdempotencyAndIntegrity(t *testing.T) {
	pool := prompt8Pool(t)
	ctx := context.Background()
	seller := prompt8Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	sealer := universe.Sealer{Repo: repo, Store: store}

	bundle, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := bundle.MarshalCanonical()
	if err = repo.PutContractBundle(ctx, bundle.Hash(), "", body); err != nil {
		t.Fatal(err)
	}
	run, err := repo.CreateProductionRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.EnsureStages(ctx, run.ID, stages.ProductionImplementedStageNames()); err != nil {
		t.Fatal(err)
	}

	resolver := resolve.Resolver{Pool: pool}
	resolved, _, resolvedHash, err := resolver.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: run.Kind, IntentHash: run.IntentHash, ContractBundleHash: bundle.Hash(),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Sources) == 0 {
		domainRunID := prompt8SeedDomainRun(t, pool, seller, "CATALOG", cut)
		wh := fmt.Sprintf("%064x", 5)
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermarks(seller_account_id,domain,fact_family,latest_observed_at,state_hash,assessed_at,last_successful_domain_run_id,source_contract_hash)
			VALUES($1,'CATALOG','CATALOG.SKU_UNIVERSE',$2,$3,$2,$4,$3)
			ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET latest_observed_at=EXCLUDED.latest_observed_at,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at,last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id`,
			seller, cut, wh, domainRunID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(seller_account_id,domain,fact_family,latest_observed_at,state_hash,assessed_at,knowledge_time,last_successful_domain_run_id,source_contract_hash)
			VALUES($1,'CATALOG','CATALOG.SKU_UNIVERSE',$2,$3,$2,$2,$4,$3)`,
			seller, cut, wh, domainRunID)
		if err != nil {
			t.Fatal(err)
		}
		resolved, _, resolvedHash, err = resolver.Resolve(ctx, resolve.Input{
			SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
			RunKind: run.Kind, IntentHash: run.IntentHash, ContractBundleHash: bundle.Hash(),
			Policies: factualapi.CurrentPolicies(),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	in := universe.Input{
		SellerID: seller, RunID: run.ID, ResolvedContextHash: resolvedHash, Resolved: &resolved,
		EntitySets: []universe.EntitySet{{EntityKind: "SKU", SetHash: fmt.Sprintf("%064x", 9), MemberCount: 2, Members: []string{"100", "200"}}},
	}
	b1, err := sealer.Seal(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := sealer.Seal(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if b1.ID != b2.ID || b1.Hash != b2.Hash {
		t.Fatalf("seal not idempotent: %s/%s vs %s/%s", b1.ID, b1.Hash, b2.ID, b2.Hash)
	}
	loaded, err := sealer.LoadValidated(ctx, b1.ID, seller)
	if err != nil || loaded.Hash != b1.Hash {
		t.Fatalf("reload failed: %v hash=%s", err, loaded.Hash)
	}
	store.Corrupt(b1.Record.ObjectKey, []byte(`{"tampered":true}`))
	if _, err = sealer.LoadValidated(ctx, b1.ID, seller); err == nil {
		t.Fatal("corrupt manifest accepted")
	}
}

func TestPrompt8ProductionRunUniquenessAndEvaluationConcurrency(t *testing.T) {
	pool := prompt8Pool(t)
	ctx := context.Background()
	seller := prompt8Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	bundle, _ := contracts.Current()
	body, _ := bundle.MarshalCanonical()
	_ = repo.PutContractBundle(ctx, bundle.Hash(), "", body)
	intent := runtime.StandardIntentHash()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CreateProductionRun(ctx, analysispg.Run{
				SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
				ContractHash: bundle.Hash(), IntentHash: intent,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	ok, conflict := 0, 0
	for err := range errs {
		if err == nil {
			ok++
		} else if err == analysispg.ErrActiveProductionRunExists {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("production uniqueness ok=%d conflict=%d", ok, conflict)
	}
	for i := 0; i < 2; i++ {
		if _, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
			SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
			ContractHash: bundle.Hash(), IntentHash: intent,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrompt8LeaseFencingHeartbeatRetryCancelRecovery(t *testing.T) {
	pool := prompt8Pool(t)
	ctx := context.Background()
	seller := prompt8Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	bundle, _ := contracts.Current()
	body, _ := bundle.MarshalCanonical()
	_ = repo.PutContractBundle(ctx, bundle.Hash(), "", body)
	run, err := repo.CreateProductionRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.EnsureStages(ctx, run.ID, []string{"TEST_STAGE_A", "TEST_STAGE_B"}); err != nil {
		t.Fatal(err)
	}
	prompt8SetTestStageHashes(t, repo, run.ID)

	clock := &prompt8Clock{now: cut}
	pol := policy.V1(clock, prompt8Rand{})
	attempts := 0
	exec := &runtime.Executor{
		Repo: repo, Store: store, Policy: pol, WorkerID: "worker-b",
		NextStages: []string{"TEST_STAGE_A", "TEST_STAGE_B"},
		Handlers: map[stages.Name]runtime.Handler{
			"TEST_STAGE_A": runtime.HandlerFunc(func(ctx context.Context, w runtime.Work) (runtime.Output, error) {
				attempts++
				if attempts == 1 {
					return runtime.Output{}, fmt.Errorf("transient")
				}
				return runtime.Output{Kind: artifacts.TestStageOutput, Body: []byte(`{"ok":true}`)}, nil
			}),
			"TEST_STAGE_B": runtime.HandlerFunc(func(ctx context.Context, w runtime.Work) (runtime.Output, error) {
				return runtime.Output{Kind: artifacts.TestStageOutput, Body: []byte(`{"b":true}`)}, nil
			}),
		},
	}

	aLease, err := repo.ClaimByRunStage(ctx, run.ID, "TEST_STAGE_A", "worker-a", time.Minute, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Heartbeat(ctx, aLease, clock.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	bad := aLease
	bad.Token = "00000000-0000-0000-0000-000000000000"
	if err = repo.Heartbeat(ctx, bad, clock.Now().Add(time.Minute)); err != analysispg.ErrLeaseLost {
		t.Fatalf("expected LEASE_LOST, got %v", err)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	bLease, err := repo.ClaimByRunStage(ctx, run.ID, "TEST_STAGE_A", "worker-b", time.Minute, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.FinalizeSuccess(ctx, aLease, nil, ""); err != analysispg.ErrLeaseLost {
		t.Fatalf("stale A accepted: %v", err)
	}
	if err = repo.FinalizeSuccess(ctx, bLease, nil, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}

	runRetry, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.EnsureStages(ctx, runRetry.ID, []string{"TEST_STAGE_A", "TEST_STAGE_B"})
	prompt8SetTestStageHashes(t, repo, runRetry.ID)
	attempts = 0
	exec.Repo = repo
	_ = exec.ExecuteStage(ctx, runRetry.ID, "TEST_STAGE_A")
	stage, _ := repo.GetStage(ctx, runRetry.ID, "TEST_STAGE_A")
	if stage.Status != "FAILED_RETRYABLE" {
		t.Fatalf("expected FAILED_RETRYABLE got %s attempts=%d", stage.Status, stage.AttemptCount)
	}
	clock.now = clock.now.Add(time.Minute)
	if err = exec.ExecuteStage(ctx, runRetry.ID, "TEST_STAGE_A"); err != nil {
		t.Fatal(err)
	}
	stage, _ = repo.GetStage(ctx, runRetry.ID, "TEST_STAGE_A")
	if stage.Status != "SUCCEEDED" || stage.AttemptCount < 2 {
		t.Fatalf("retry history incomplete: %+v attempts=%d", stage.Status, stage.AttemptCount)
	}
	if err = exec.ExecuteStage(ctx, runRetry.ID, "TEST_STAGE_A"); err != nil {
		t.Fatal(err)
	}
	if err = repo.MarkNextStageReady(ctx, runRetry.ID, "TEST_STAGE_A", []string{"TEST_STAGE_A", "TEST_STAGE_B"}); err != nil {
		t.Fatal(err)
	}
	prompt8SetTestStageHashes(t, repo, runRetry.ID)
	disp := runtime.RecoveryDispatcher{Enabled: true, Executor: exec}
	items, err := disp.Scan(ctx)
	if err != nil || len(items) == 0 {
		t.Fatalf("recovery scan empty: %v %#v", err, items)
	}
	if err = exec.ExecuteStage(ctx, runRetry.ID, "TEST_STAGE_B"); err != nil {
		t.Fatal(err)
	}

	run2, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.EnsureStages(ctx, run2.ID, []string{"TEST_STAGE_A", "TEST_STAGE_B"})
	prompt8SetTestStageHashes(t, repo, run2.ID)
	_ = repo.RequestCancel(ctx, run2.ID, "TEST")
	if err = repo.ApplyCancelAtBoundary(ctx, run2.ID); err != nil {
		t.Fatal(err)
	}
	r2, _ := repo.GetRun(ctx, run2.ID)
	if r2.Status != "CANCELLED" {
		t.Fatalf("cancel run status=%s", r2.Status)
	}
	stA, _ := repo.GetStage(ctx, run2.ID, "TEST_STAGE_A")
	stB, _ := repo.GetStage(ctx, run2.ID, "TEST_STAGE_B")
	if stA.Status != "CANCELLED" || stB.Status != "CANCELLED" {
		t.Fatalf("cancel stage statuses A=%s B=%s", stA.Status, stB.Status)
	}
}

func TestPrompt8CrashAfterArtifactAndHashDeterminism(t *testing.T) {
	pool := prompt8Pool(t)
	ctx := context.Background()
	seller := prompt8Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	sealer := universe.Sealer{Repo: repo, Store: store}

	resolved := prompt8ResolvedManifest(seller, cut, "same")
	resolvedHash := factualapi.StableHash("resolved", "same")
	sets := []universe.EntitySet{{EntityKind: "SKU", SetHash: fmt.Sprintf("%064x", 1), MemberCount: 1, Members: []string{"1"}}}
	m1 := universe.ManifestFromResolved(resolved, resolvedHash, sets, nil)
	m2 := universe.ManifestFromResolved(resolved, resolvedHash, sets, nil)
	if canonicaljson.Hash(universe.SemanticHashInput(m1)) != canonicaljson.Hash(universe.SemanticHashInput(m2)) {
		t.Fatal("hash nondeterministic")
	}
	sets3 := []universe.EntitySet{{EntityKind: "SKU", SetHash: fmt.Sprintf("%064x", 2), MemberCount: 1, Members: []string{"2"}}}
	m3 := universe.ManifestFromResolved(resolved, resolvedHash, sets3, nil)
	if canonicaljson.Hash(universe.SemanticHashInput(m1)) == canonicaljson.Hash(universe.SemanticHashInput(m3)) {
		t.Fatal("semantic change did not alter hash")
	}

	bundle, _ := contracts.Current()
	body, _ := bundle.MarshalCanonical()
	_ = repo.PutContractBundle(ctx, bundle.Hash(), "", body)
	run, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.EnsureStages(ctx, run.ID, []string{"TEST_STAGE_A"})
	prompt8SetTestStageHashes(t, repo, run.ID)
	payload := []byte(`{"crash":"after-artifact"}`)
	a, err := store.PutImmutable(ctx, artifacts.Location{SellerID: seller, RunID: run.ID, Stage: "TEST_STAGE_A", Kind: artifacts.TestStageOutput}, payload)
	if err != nil {
		t.Fatal(err)
	}
	id, err := repo.AddArtifact(ctx, run.ID, nil, string(artifacts.TestStageOutput), a.Key, a.Hash, a.Size)
	if err != nil {
		t.Fatal(err)
	}
	found, ok, err := repo.FindArtifactByHash(ctx, run.ID, a.Hash)
	if err != nil || !ok || found != id {
		t.Fatalf("artifact recovery lookup failed: %v ok=%v id=%d", err, ok, found)
	}
	b, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: run.ID, ResolvedContextHash: resolvedHash, Resolved: &resolved, EntitySets: sets,
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := universe.NewSealedContext(b.Record, b)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Mode() != factualapi.ModeSealedUniverse || sealed.UniverseID() != b.ID {
		t.Fatal("sealed context not pinned")
	}
}

func TestPrompt8SealedFutureLeakCutoff(t *testing.T) {
	pool := prompt8Pool(t)
	ctx := context.Background()
	seller := prompt8Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	sealer := universe.Sealer{Repo: repo, Store: store}
	bundle, _ := contracts.Current()
	body, _ := bundle.MarshalCanonical()
	_ = repo.PutContractBundle(ctx, bundle.Hash(), "", body)
	run, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.EnsureStages(ctx, run.ID, stages.ProductionImplementedStageNames())
	resolved := prompt8ResolvedManifest(seller, cut, "base")
	resolved.DQ = []resolve.DQSnapshot{{
		Domain: "CATALOG", FactFamily: "DEFAULT", DQPolicyVersion: "dq_policy_v3",
		DQResultHash: strings.Repeat("2", 64), HighestSeverity: "NONE", CanonicalIssueSetHash: strings.Repeat("3", 64),
	}}
	resolvedHash := factualapi.StableHash("resolved", "base")
	b1, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: run.ID, ResolvedContextHash: resolvedHash, Resolved: &resolved,
		EntitySets: []universe.EntitySet{{EntityKind: "SKU", SetHash: strings.Repeat("1", 64), MemberCount: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO sales_account_daily(
		seller_account_id,business_date,ordered_amount,ordered_units,currency_code,observed_at,revision_no,collection_domain_run_id,source_payload_id)
		SELECT $1,$2::date,999,1,'RUB',$3,1,dr.id,sp.id
		FROM collection_domain_runs dr
		JOIN source_payloads sp ON sp.collection_domain_run_id=dr.id
		WHERE dr.seller_account_id=$1 LIMIT 1`, seller, cut.AddDate(0, 0, -1), cut.Add(2*time.Hour)); err != nil {
		t.Logf("optional late sales insert skipped: %v", err)
	}
	b2, err := sealer.LoadValidated(ctx, b1.ID, seller)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Hash != b1.Hash {
		t.Fatal("sealed universe hash changed after late observation")
	}
	sealed, err := universe.NewSealedContext(b2.Record, b2)
	if err != nil {
		t.Fatal(err)
	}
	if !sealed.CutoffAt().Equal(cut.UTC()) || sealed.UniverseID() != b1.ID {
		t.Fatal("sealed context re-resolved cutoff/universe")
	}
	resolvedLate := prompt8ResolvedManifest(seller, cut, "late")
	b3, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: run.ID, ResolvedContextHash: factualapi.StableHash("resolved", "late"), Resolved: &resolvedLate,
		EntitySets: []universe.EntitySet{{EntityKind: "SKU", SetHash: strings.Repeat("9", 64), MemberCount: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b3.Hash == b1.Hash {
		t.Fatal("expected different universe after semantic entity-set change")
	}
}
