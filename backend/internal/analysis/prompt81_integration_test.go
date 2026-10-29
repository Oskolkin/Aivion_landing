//go:build integration

package analysis_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
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

type prompt81Fixture struct {
	pool      *pgxpool.Pool
	repo      *analysispg.Repository
	store     *artifacts.MemoryStore
	sealer    universe.Sealer
	seller    int64
	cut       time.Time
	bundle    contracts.ContractBundle
	domainRun int64
	dqRun     int64
}

func prompt81Pool(t *testing.T) *pgxpool.Pool {
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

func prompt81Seller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var user, seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'x','active')RETURNING id`,
		fmt.Sprintf("p81_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'p81','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	return seller
}

func prompt81Hex(seed string) string { return factualapi.StableHash(seed) }

func prompt81Seed(t *testing.T) prompt81Fixture {
	t.Helper()
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	pool := prompt81Pool(t)
	ctx := context.Background()
	seller := prompt81Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	bundle, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := bundle.MarshalCanonical()
	if err = repo.PutContractBundle(ctx, bundle.Hash(), "", body); err != nil {
		t.Fatal(err)
	}

	var parent, domainRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
		VALUES($1,'MANUAL','COMPLETED','prompt81',$2,$2)RETURNING id`, seller, cut).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
		collection_run_id,seller_account_id,domain,status,started_at,finished_at,
		source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
		VALUES($1,$2,'CATALOG','COMPLETED',$3,$3,'OZON_CATALOG_PRODUCT_INFO','2.1.0','CATALOG_PRODUCT','canonical-catalog-v1',$4)RETURNING id`,
		parent, seller, cut, prompt81Hex("catalog-rev")).Scan(&domainRun); err != nil {
		t.Fatal(err)
	}
	var salesParent, salesRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
		VALUES($1,'MANUAL','COMPLETED','prompt81.sales',$2,$2)RETURNING id`, seller, cut).Scan(&salesParent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
		collection_run_id,seller_account_id,domain,status,started_at,finished_at,
		source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
		VALUES($1,$2,'SALES_DEMAND','COMPLETED',$3,$3,'OZON_ANALYTICS_DATA','3.0.0','SALES_ANALYTICS','canonical-sales-v1',$4)RETURNING id`,
		salesParent, seller, cut, prompt81Hex("sales-rev")).Scan(&salesRun); err != nil {
		t.Fatal(err)
	}

	families := []struct {
		domain, family, state string
		runID                 int64
	}{
		{"CATALOG", "CATALOG.SKU_UNIVERSE", "catalog-wm", domainRun},
		{"SALES_DEMAND", "SALES_DEMAND.ACCOUNT_DAILY", "sales-wm", salesRun},
	}
	for _, f := range families {
		wh := prompt81Hex(f.state)
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermarks(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$5::date,$5::date,$6,$7,$7,$4)
			ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET
			  latest_observed_at=EXCLUDED.latest_observed_at,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at,
			  last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id,source_contract_hash=EXCLUDED.source_contract_hash`,
			seller, f.domain, f.family, cut, cut.AddDate(0, 0, -1), f.runID, wh)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$5::date,$5::date,$6,$7,$7,$4,$4)`,
			seller, f.domain, f.family, cut, cut.AddDate(0, 0, -1), f.runID, wh)
		if err != nil {
			t.Fatal(err)
		}
	}

	var dqRun int64
	inHash := prompt81Hex("dq-input")
	resHash := prompt81Hex("dq-result")
	if err = pool.QueryRow(ctx, `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`, seller, cut, inHash, resHash).Scan(&dqRun); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO data_quality_issues(
		data_quality_run_id,seller_account_id,domain,dimension,code,severity,readiness_effect,retry_class,details_json,detected_at)
		VALUES($1,$2,'CATALOG','COMPLETENESS','MISSING_SKU','WARNING','LIMITATION_ONLY','DATA_STATE','{}',$3)`,
		dqRun, seller, cut)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES
		($1,$2,'CATALOG.SKU_UNIVERSE','HISTORICAL_QUERY','CATALOG','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','PARTIAL','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4),
		($1,$2,'SALES_DEMAND.ACCOUNT_DAILY','HISTORICAL_QUERY','SALES_DEMAND','PARTIAL',
		 'ACTIVE','ACCESSIBLE','PARTIAL','VALIDATED','PARTIAL','LAGGED','PARTIAL','NON_EMPTY',
		 'LAG','{}','capability_policy_v3',$3,$5)`,
		seller, dqRun, cut.Add(-time.Minute), domainRun, salesRun)
	if err != nil {
		t.Fatal(err)
	}

	for i, sku := range []int64{1001, 1002} {
		_, err = pool.Exec(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
			VALUES($1,$2,$3,$3,'ACTIVE')ON CONFLICT DO NOTHING`, seller, sku, cut.Add(-time.Duration(i+1)*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
	}

	return prompt81Fixture{
		pool: pool, repo: repo, store: store, sealer: universe.Sealer{Repo: repo, Store: store},
		seller: seller, cut: cut, bundle: bundle, domainRun: domainRun, dqRun: dqRun,
	}
}

func (f prompt81Fixture) resolve(t *testing.T) (resolve.Manifest, string) {
	t.Helper()
	m, _, hash, err := resolve.Resolver{Pool: f.pool}.Resolve(context.Background(), resolve.Input{
		SellerID: f.seller, AnalysisDate: f.cut.AddDate(0, 0, -1), CutoffAt: f.cut,
		RunKind: "EVALUATION", IntentHash: runtime.StandardIntentHash(), ContractBundleHash: f.bundle.Hash(),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, hash
}

func (f prompt81Fixture) evalRun(t *testing.T) analysispg.Run {
	t.Helper()
	run, err := f.repo.CreateEvaluationRun(context.Background(), analysispg.Run{
		SellerID: f.seller, AnalysisDate: f.cut.AddDate(0, 0, -1), CutoffAt: f.cut,
		ContractHash: f.bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.repo.EnsureStages(context.Background(), run.ID, stages.ProductionImplementedStageNames()); err != nil {
		t.Fatal(err)
	}
	return run
}

func (f prompt81Fixture) seal(t *testing.T, runID int64, resolved resolve.Manifest, hash string, sets []universe.EntitySet) universe.Binding {
	t.Helper()
	if sets == nil {
		sets = []universe.EntitySet{{
			EntityKind: "SKU", SetHash: canonicaljson.Hash([]string{"1001", "1002"}), MemberCount: 2, Members: []string{"1001", "1002"},
		}}
	}
	b, err := f.sealer.Seal(context.Background(), universe.Input{
		SellerID: f.seller, RunID: runID, ResolvedContextHash: hash, Resolved: &resolved, EntitySets: sets,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPrompt81ResolveCompleteFactFamilies(t *testing.T) {
	f := prompt81Seed(t)
	m, _ := f.resolve(t)
	if len(m.Sources) < 2 {
		t.Fatalf("expected >=2 fact families, got %d", len(m.Sources))
	}
	seen := map[string]bool{}
	for _, s := range m.Sources {
		seen[s.Domain+"/"+s.FactFamily] = true
		if s.SourceState == "" || s.SourceHash == "" || s.BindingHash == "" {
			t.Fatalf("incomplete source: %+v", s)
		}
	}
	if !seen["CATALOG/CATALOG.SKU_UNIVERSE"] || !seen["SALES_DEMAND/SALES_DEMAND.ACCOUNT_DAILY"] {
		t.Fatalf("missing expected families: %#v", seen)
	}
	if len(m.Capabilities) < 2 {
		t.Fatalf("expected capabilities, got %d", len(m.Capabilities))
	}
	if len(m.DQ) == 0 {
		t.Fatal("expected DQ snapshots")
	}
}

func TestPrompt81SealReloadEquality(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b1 := f.seal(t, run.ID, resolved, hash, nil)
	b2, err := f.sealer.LoadValidated(context.Background(), b1.ID, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if b1.Hash != b2.Hash || b1.ID != b2.ID {
		t.Fatalf("reload mismatch %s/%s vs %s/%s", b1.ID, b1.Hash, b2.ID, b2.Hash)
	}
	h1 := canonicaljson.Hash(universe.SemanticHashInput(b1.Manifest))
	h2 := canonicaljson.Hash(universe.SemanticHashInput(b2.Manifest))
	if h1 != h2 || h1 != b1.Hash {
		t.Fatal("semantic hash reload divergence")
	}
}

func TestPrompt81NoFakeProvenance(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved := resolve.Manifest{
		Version: resolve.ManifestVersion, SellerAccountID: f.seller,
		AnalysisDate: canonicaljson.Date(f.cut.AddDate(0, 0, -1)), CutoffAt: f.cut,
		Policies: factualapi.CurrentPolicies(),
		Sources: []resolve.SourceBinding{{
			Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", SourceBindingKey: "CATALOG.SKU_UNIVERSE",
			SourceState: "NOT_ATTEMPTED", SourceContractKey: "CATALOG", SourceContractVersion: "v1",
			SourceContractHash: prompt81Hex("no-run"), SourceHash: prompt81Hex("src-nil"), BindingHash: prompt81Hex("bind-nil"),
		}},
		Capabilities: []resolve.CapabilitySnapshot{{
			Key: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "UNAVAILABLE",
			ContractState: "ACTIVE", AccessState: "ACCESSIBLE", SourceState: "NOT_ATTEMPTED", SemanticState: "UNKNOWN_PROVIDER_SEMANTICS",
			CoverageState: "UNKNOWN", TemporalState: "UNKNOWN", HistoryState: "NONE", PopulationState: "UNKNOWN",
			PolicyVersion: "capability_policy_v3", AssessedAt: f.cut.Add(-time.Minute), AssessmentHash: prompt81Hex("cap-nil"),
		}},
	}
	b := f.seal(t, run.ID, resolved, prompt81Hex("resolved-nil"), []universe.EntitySet{
		{EntityKind: "SKU", SetHash: prompt81Hex("empty-sku"), MemberCount: 0},
	})
	sealed, err := universe.NewSealedContext(b.Record, b)
	if err != nil {
		t.Fatal(err)
	}
	dom, ok := sealed.DomainBinding("CATALOG")
	if !ok || dom.DomainRunID != 0 {
		t.Fatalf("fabricated domain run id: ok=%v id=%d", ok, dom.DomainRunID)
	}
	cap, ok := sealed.CapabilityBinding("CATALOG.SKU_UNIVERSE", "HISTORICAL_QUERY")
	if !ok || cap.AssessmentID != 0 {
		t.Fatalf("fabricated assessment id: ok=%v id=%d", ok, cap.AssessmentID)
	}
}

func TestPrompt81CapabilityDimensionsFreeze(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b := f.seal(t, run.ID, resolved, hash, nil)
	loaded, err := f.sealer.LoadValidated(context.Background(), b.ID, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range loaded.Manifest.Capabilities {
		if c.Key != "CATALOG.SKU_UNIVERSE" {
			continue
		}
		found = true
		if c.ContractState != "ACTIVE" || c.AccessState != "ACCESSIBLE" || c.HistoryState != "PARTIAL" {
			t.Fatalf("capability dimensions not frozen: %+v", c)
		}
		if c.SemanticState == "" || c.TemporalState == "" || c.CoverageState == "" || c.PopulationState == "" {
			t.Fatalf("missing capability dimensions: %+v", c)
		}
	}
	if !found {
		t.Fatal("catalog capability missing from sealed manifest")
	}
}

func TestPrompt81DQFreeze(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	if len(resolved.DQ) == 0 {
		t.Fatal("resolve produced no DQ")
	}
	b := f.seal(t, run.ID, resolved, hash, nil)
	loaded, err := f.sealer.LoadValidated(context.Background(), b.ID, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Manifest.DQ) == 0 {
		t.Fatal("DQ not frozen into universe")
	}
	dq := loaded.Manifest.DQ[0]
	if dq.DQPolicyVersion != "dq_policy_v3" || dq.CanonicalIssueSetHash == "" || dq.WarningCount < 1 {
		t.Fatalf("unexpected DQ freeze: %+v", dq)
	}
}

func TestPrompt81EntitySetCounts(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	sets := []universe.EntitySet{{
		EntityKind: "SKU", SetHash: canonicaljson.Hash([]string{"1001", "1002"}), MemberCount: 2, Members: []string{"1001", "1002"},
	}}
	b := f.seal(t, run.ID, resolved, hash, sets)
	if len(b.Record.EntitySets) != 1 || b.Record.EntitySets[0].MemberCount != 2 {
		t.Fatalf("entity set counts wrong: %+v", b.Record.EntitySets)
	}
	loaded, err := f.sealer.LoadValidated(context.Background(), b.ID, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.EntitySets[0].MemberCount != 2 || loaded.Manifest.EntitySets[0].SetHash != sets[0].SetHash {
		t.Fatalf("entity set reload mismatch: %+v", loaded.Manifest.EntitySets)
	}
}

func TestPrompt81ChildImmutability(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b := f.seal(t, run.ID, resolved, hash, nil)
	ctx := context.Background()
	_, err := f.pool.Exec(ctx, `INSERT INTO factual_universe_entity_sets(universe_id,entity_kind,set_hash,member_count)
		VALUES($1::uuid,'PRODUCT',$2,0)`, b.ID, prompt81Hex("mutate"))
	if err == nil {
		t.Fatal("expected child insert after seal to fail")
	}
	_, err = f.pool.Exec(ctx, `UPDATE factual_universe_entity_sets SET member_count=99 WHERE universe_id=$1::uuid`, b.ID)
	if err == nil {
		t.Fatal("expected child update after seal to fail")
	}
	_, err = f.pool.Exec(ctx, `DELETE FROM factual_universe_entity_sets WHERE universe_id=$1::uuid`, b.ID)
	if err == nil {
		t.Fatal("expected child delete after seal to fail")
	}
}

func TestPrompt81TamperIntegrity(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b := f.seal(t, run.ID, resolved, hash, nil)
	f.store.Corrupt(b.Record.ObjectKey, []byte(`{"tampered":true}`))
	if _, err := f.sealer.LoadValidated(context.Background(), b.ID, f.seller); err == nil {
		t.Fatal("tampered MinIO accepted")
	}
}

func TestPrompt81ManifestArtifactFK(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b := f.seal(t, run.ID, resolved, hash, nil)
	if b.Record.ManifestArtifactID == nil || *b.Record.ManifestArtifactID <= 0 {
		t.Fatal("manifest_artifact_id not set")
	}
	_, err := f.pool.Exec(context.Background(), `DELETE FROM analysis_artifacts WHERE id=$1`, *b.Record.ManifestArtifactID)
	if err == nil {
		t.Fatal("expected ON DELETE RESTRICT for manifest artifact FK")
	}
}

func TestPrompt81StageInputHashRequired(t *testing.T) {
	f := prompt81Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	_, err := f.repo.ClaimByRunStage(ctx, run.ID, string(stages.ResolveContext), "worker", time.Minute, f.cut)
	if !errors.Is(err, analysispg.ErrStageInputHashInvalid) {
		t.Fatalf("expected STAGE_INPUT_HASH_INVALID, got %v", err)
	}
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	zero := strings.Repeat("0", 64)
	for _, s := range list {
		setErr := f.repo.SetStageInputHash(ctx, s.ID, zero)
		if setErr == nil {
			_, claimErr := f.repo.ClaimByRunStage(ctx, run.ID, s.Key, "worker", time.Minute, f.cut)
			if !errors.Is(claimErr, analysispg.ErrStageInputHashInvalid) {
				t.Fatalf("zero input hash must be rejected at set or claim; set=%v claim=%v", setErr, claimErr)
			}
		}
	}
}

func TestPrompt81CancellationConsistent(t *testing.T) {
	f := prompt81Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	prompt8SetTestStageHashes(t, f.repo, run.ID)
	if err := f.repo.RequestCancel(ctx, run.ID, "PROMPT81"); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.ApplyCancelAtBoundary(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "CANCELLED" {
		t.Fatalf("run status=%s", got.Status)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		st, err := f.repo.GetStage(ctx, run.ID, key)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != "CANCELLED" {
			t.Fatalf("stage %s status=%s", key, st.Status)
		}
	}
}

func TestPrompt81ProductionExecutorResolveSeal(t *testing.T) {
	f := prompt81Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	exec, err := runtime.NewProductionExecutor(f.pool, f.store, "prompt81-exec", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = exec.ExecuteStage(ctx, run.ID, string(stages.ResolveContext)); err != nil {
		t.Fatal(err)
	}
	resolveStage, err := f.repo.GetStage(ctx, run.ID, string(stages.ResolveContext))
	if err != nil || resolveStage.Status != "SUCCEEDED" {
		t.Fatalf("resolve status=%v err=%v", resolveStage.Status, err)
	}
	if err = f.repo.MarkNextStageReady(ctx, run.ID, string(stages.ResolveContext), stages.ProductionImplementedStageNames()); err != nil {
		t.Fatal(err)
	}
	if err = exec.ExecuteStage(ctx, run.ID, string(stages.SealUniverse)); err != nil {
		t.Fatal(err)
	}
	sealStage, err := f.repo.GetStage(ctx, run.ID, string(stages.SealUniverse))
	if err != nil || sealStage.Status != "SUCCEEDED" {
		t.Fatalf("seal status=%v err=%v", sealStage.Status, err)
	}
	got, err := f.repo.GetRun(ctx, run.ID)
	if err != nil || got.UniverseID == "" {
		t.Fatalf("run missing universe: %+v err=%v", got, err)
	}
}

func TestPrompt81FutureLeakSealed(t *testing.T) {
	f := prompt81Seed(t)
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	b1 := f.seal(t, run.ID, resolved, hash, nil)
	late := f.cut.Add(3 * time.Hour)
	_, err := f.pool.Exec(context.Background(), `INSERT INTO domain_watermarks(
		seller_account_id,domain,fact_family,latest_observed_at,state_hash,assessed_at,source_contract_hash)
		VALUES($1,'FUNNEL','FUNNEL.ACCOUNT',$2,$3,$2,$3)
		ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET assessed_at=EXCLUDED.assessed_at,state_hash=EXCLUDED.state_hash`,
		f.seller, late, prompt81Hex("future-funnel"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(context.Background(), `INSERT INTO domain_watermark_revisions(
		seller_account_id,domain,fact_family,latest_observed_at,state_hash,assessed_at,knowledge_time,source_contract_hash)
		VALUES($1,'FUNNEL','FUNNEL.ACCOUNT',$2,$3,$2,$2,$3)`,
		f.seller, late, prompt81Hex("future-funnel"))
	if err != nil {
		t.Fatal(err)
	}
	b2, err := f.sealer.LoadValidated(context.Background(), b1.ID, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Hash != b1.Hash {
		t.Fatal("sealed universe changed after future watermark")
	}
	for _, s := range b2.Manifest.Sources {
		if s.Domain == "FUNNEL" {
			t.Fatal("future fact family leaked into sealed universe")
		}
	}
}

func TestPrompt81ContractBundleNoOpSensitivity(t *testing.T) {
	a, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	b, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash() != b.Hash() || a.Hash() == "" {
		t.Fatal("contract bundle hash unstable")
	}
	v := a.Versions()
	if v.DiscoveryPack == contracts.NotImplemented || v.Publication == contracts.NotImplemented ||
		v.ActionReconciliation == contracts.NotImplemented || v.Publish == contracts.NotImplemented {
		t.Fatal("Prompt 13 pipeline contracts must be implemented in Current()")
	}
	if v.LandscapeCore == contracts.NotImplemented || v.Materiality == contracts.NotImplemented || v.LandscapeDetail == contracts.NotImplemented {
		t.Fatal("Prompt9 Landscape/Materiality seals must be present")
	}
	emptyFuture, err := contracts.New(contracts.Versions{
		OrchestrationPolicy: v.OrchestrationPolicy, FactualAPI: v.FactualAPI,
		TemporalPolicy: v.TemporalPolicy, DQPolicy: v.DQPolicy, CapabilityPolicy: v.CapabilityPolicy,
		SalesMaturityPolicy: v.SalesMaturityPolicy, StockFreshnessPolicy: v.StockFreshnessPolicy,
		PriceFreshnessPolicy: v.PriceFreshnessPolicy, AdvertisingMaturityPolicy: v.AdvertisingMaturityPolicy,
		AttributionPolicy: v.AttributionPolicy, UniverseManifest: v.UniverseManifest, ContextManifest: v.ContextManifest,
		FunnelMetricRegistry: v.FunnelMetricRegistry, FinanceTypeRegistry: v.FinanceTypeRegistry,
		SearchCollectionPolicy: v.SearchCollectionPolicy, PromotionsSemanticRegistry: v.PromotionsSemanticRegistry,
		AccountHealthMetricRegistry: v.AccountHealthMetricRegistry, SupplyStateRegistry: v.SupplyStateRegistry,
		AdvertisingAttributionSemantics: v.AdvertisingAttributionSemantics,
		ContinuityContract:              v.ContinuityContract, LandscapeCore: v.LandscapeCore, Materiality: v.Materiality, LandscapeDetail: v.LandscapeDetail,
	})
	if err != nil {
		t.Fatal(err)
	}
	explicitFuture, err := contracts.New(contracts.Versions{
		OrchestrationPolicy: v.OrchestrationPolicy, FactualAPI: v.FactualAPI,
		TemporalPolicy: v.TemporalPolicy, DQPolicy: v.DQPolicy, CapabilityPolicy: v.CapabilityPolicy,
		SalesMaturityPolicy: v.SalesMaturityPolicy, StockFreshnessPolicy: v.StockFreshnessPolicy,
		PriceFreshnessPolicy: v.PriceFreshnessPolicy, AdvertisingMaturityPolicy: v.AdvertisingMaturityPolicy,
		AttributionPolicy: v.AttributionPolicy, UniverseManifest: v.UniverseManifest, ContextManifest: v.ContextManifest,
		FunnelMetricRegistry: v.FunnelMetricRegistry, FinanceTypeRegistry: v.FinanceTypeRegistry,
		SearchCollectionPolicy: v.SearchCollectionPolicy, PromotionsSemanticRegistry: v.PromotionsSemanticRegistry,
		AccountHealthMetricRegistry: v.AccountHealthMetricRegistry, SupplyStateRegistry: v.SupplyStateRegistry,
		AdvertisingAttributionSemantics: v.AdvertisingAttributionSemantics,
		ContinuityContract:              v.ContinuityContract, LandscapeCore: v.LandscapeCore, Materiality: v.Materiality, LandscapeDetail: v.LandscapeDetail,
		DiscoveryPack: contracts.NotImplemented, Publication: contracts.NotImplemented,
	})
	if err != nil {
		t.Fatal(err)
	}
	if emptyFuture.Hash() != explicitFuture.Hash() {
		t.Fatal("empty vs explicit NOT_IMPLEMENTED must be hash-noop")
	}
	changedRegistry, err := contracts.New(contracts.Versions{
		OrchestrationPolicy: v.OrchestrationPolicy, FactualAPI: v.FactualAPI,
		TemporalPolicy: v.TemporalPolicy, DQPolicy: v.DQPolicy, CapabilityPolicy: v.CapabilityPolicy,
		SalesMaturityPolicy: v.SalesMaturityPolicy, StockFreshnessPolicy: v.StockFreshnessPolicy,
		PriceFreshnessPolicy: v.PriceFreshnessPolicy, AdvertisingMaturityPolicy: v.AdvertisingMaturityPolicy,
		AttributionPolicy: v.AttributionPolicy, UniverseManifest: v.UniverseManifest, ContextManifest: v.ContextManifest,
		FunnelMetricRegistry: "funnel_registry_v1_CHANGED", FinanceTypeRegistry: v.FinanceTypeRegistry,
		SearchCollectionPolicy: v.SearchCollectionPolicy, PromotionsSemanticRegistry: v.PromotionsSemanticRegistry,
		AccountHealthMetricRegistry: v.AccountHealthMetricRegistry, SupplyStateRegistry: v.SupplyStateRegistry,
		AdvertisingAttributionSemantics: v.AdvertisingAttributionSemantics,
		ContinuityContract:              v.ContinuityContract, LandscapeCore: v.LandscapeCore, Materiality: v.Materiality, LandscapeDetail: v.LandscapeDetail,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changedRegistry.Hash() == a.Hash() {
		t.Fatal("registry version change must alter contract_bundle_hash (NO_OP sensitivity)")
	}
}
