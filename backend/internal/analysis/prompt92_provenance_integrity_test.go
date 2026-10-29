//go:build integration

package analysis_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func prompt92Pool(t *testing.T) *pgxpool.Pool {
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

func prompt92Seller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var user, seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'x','active')RETURNING id`,
		fmt.Sprintf("p92_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'p92','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	return seller
}

func prompt92Hex(seed string) string { return factualapi.StableHash(seed) }

func TestPrompt92ProvenanceIntegrity(t *testing.T) {
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	pool := prompt92Pool(t)
	ctx := context.Background()
	seller := prompt92Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	finishedAt := cut.Add(-3 * time.Hour)
	observedAt := cut.Add(-90 * time.Minute)
	analysisDay := cut.AddDate(0, 0, -1)

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

	contractKey := "OZON_ANALYTICS_PRODUCT_QUERIES"
	contractVersion := "1.2.0"
	endpoint := "SEARCH_QUERIES"
	mappingVersion := "search-semantic-v2"
	revisionHash := prompt92Hex("search-revision-R")
	contractHash := factualapi.DeriveSourceContractHash(contractKey, contractVersion, endpoint)
	mappingHash := factualapi.StableHash("semantic_mapping", mappingVersion)
	wmHash := prompt92Hex("search-wm-state")

	var parent, domainRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
		VALUES($1,'MANUAL','COMPLETED','prompt92',$2,$2)RETURNING id`, seller, finishedAt).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
		collection_run_id,seller_account_id,domain,status,started_at,finished_at,
		source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
		VALUES($1,$2,'SEARCH','COMPLETED',$3,$3,$4,$5,$6,$7,$8)RETURNING id`,
		parent, seller, finishedAt, contractKey, contractVersion, endpoint, mappingVersion, revisionHash).Scan(&domainRun); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO domain_watermarks(
		seller_account_id,domain,fact_family,latest_observed_at,last_successful_domain_run_id,
		source_contract_hash,semantic_mapping_hash,source_revision_hash,state_hash,assessed_at)
		VALUES($1,'SEARCH','SEARCH.SELLER_SUMMARY',$2,$3,$4,$5,$6,$7,$2)`,
		seller, observedAt, domainRun, contractHash, mappingHash, revisionHash, wmHash)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
		seller_account_id,domain,fact_family,latest_observed_at,last_successful_domain_run_id,
		source_contract_hash,semantic_mapping_hash,source_revision_hash,state_hash,assessed_at,knowledge_time)
		VALUES($1,'SEARCH','SEARCH.SELLER_SUMMARY',$2,$3,$4,$5,$6,$7,$2,$2)`,
		seller, observedAt, domainRun, contractHash, mappingHash, revisionHash, wmHash)
	if err != nil {
		t.Fatal(err)
	}

	var dqRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, cut, prompt92Hex("dq-in"), prompt92Hex("dq-res")).Scan(&dqRun); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES($1,$2,'SEARCH.SELLER_SUMMARY','HISTORICAL_QUERY','SEARCH','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','PARTIAL','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4)`,
		seller, dqRun, cut, domainRun)
	if err != nil {
		t.Fatal(err)
	}

	resolved, resolvedBody, resolvedHash, err := resolve.Resolver{Pool: pool}.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: analysisDay, CutoffAt: cut,
		RunKind: "EVALUATION", IntentHash: prompt92Hex("intent"), ContractBundleHash: bundle.Hash(),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Sources) != 1 {
		t.Fatalf("sources=%d", len(resolved.Sources))
	}
	src := resolved.Sources[0]
	if src.SourceContractHash != contractHash {
		t.Fatalf("contract hash=%q want %q", src.SourceContractHash, contractHash)
	}
	if src.SemanticMappingHash != mappingHash || src.SourceRevisionHash != revisionHash {
		t.Fatalf("mapping/revision: mapping=%q revision=%q", src.SemanticMappingHash, src.SourceRevisionHash)
	}
	if src.SourceContractHash == src.SemanticMappingHash {
		t.Fatal("contract hash must differ from semantic mapping hash")
	}
	if src.SourceFinalizedAt == nil || !src.SourceFinalizedAt.Equal(finishedAt) {
		t.Fatalf("source_finalized_at=%v want %v", src.SourceFinalizedAt, finishedAt)
	}
	if src.LatestObservedAt == nil || src.LatestObservedAt.Equal(finishedAt) {
		t.Fatalf("observed_at must differ from finished_at: observed=%v finished=%v", src.LatestObservedAt, finishedAt)
	}
	if src.WatermarkHash != wmHash {
		t.Fatalf("watermark=%q", src.WatermarkHash)
	}
	_ = resolvedBody

	var evalRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO analysis_runs(
		seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,
		contract_bundle_hash,run_intent_hash,trigger)
		VALUES($1,'EVALUATION','EVALUATION','PREPARING',$2,$3,$4,$5,'prompt92')RETURNING id`,
		seller, analysisDay, cut, bundle.Hash(), prompt92Hex("intent")).Scan(&evalRun); err != nil {
		t.Fatal(err)
	}

	sealer := universe.Sealer{Repo: repo, Store: store}
	binding, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: evalRun, ResolvedContextHash: resolvedHash, Resolved: &resolved,
		EntitySets: []universe.EntitySet{{EntityKind: "SKU", SetHash: prompt92Hex(fmt.Sprintf("sku-set-%d", seller)), MemberCount: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := sealer.LoadValidated(ctx, binding.ID, seller)
	if err != nil {
		t.Fatal(err)
	}
	dbSrc := loaded.Record.Sources[0]
	if dbSrc.SourceContractKey != contractKey || dbSrc.SourceContractVersion != contractVersion || dbSrc.SourceContractHash != contractHash {
		t.Fatalf("DB contract provenance wrong: %+v", dbSrc)
	}
	if dbSrc.SourceEndpointFamily != endpoint || dbSrc.SemanticMappingVersion != mappingVersion || dbSrc.SourceRevisionHash != revisionHash {
		t.Fatalf("DB endpoint/mapping/revision wrong: %+v", dbSrc)
	}
	if dbSrc.WatermarkHash != wmHash {
		t.Fatalf("DB watermark=%q", dbSrc.WatermarkHash)
	}
	if dbSrc.SourceFinalizedAt == nil || !dbSrc.SourceFinalizedAt.Equal(finishedAt) {
		t.Fatalf("DB source_finalized_at=%v", dbSrc.SourceFinalizedAt)
	}
	if dbSrc.LatestObservedAt == nil || !dbSrc.LatestObservedAt.Equal(observedAt) {
		t.Fatalf("DB latest_observed_at=%v", dbSrc.LatestObservedAt)
	}

	sealed, err := universe.NewSealedContext(loaded.Record, loaded)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := sealed.Binding("SEARCH", "SEARCH.SELLER_SUMMARY")
	if !ok {
		t.Fatal("missing sealed binding")
	}
	if !b.FinalizedAt.Equal(finishedAt) {
		t.Fatalf("sealed FinalizedAt=%v want finished_at %v", b.FinalizedAt, finishedAt)
	}
	if b.WatermarkHash != wmHash {
		t.Fatalf("sealed watermark=%q", b.WatermarkHash)
	}

	_, err = pool.Exec(ctx, `UPDATE factual_universe_sources SET source_contract_key='TAMPERED' WHERE universe_id=$1::uuid`, binding.ID)
	if err == nil {
		if _, err = sealer.LoadValidated(ctx, binding.ID, seller); err == nil {
			t.Fatal("LoadValidated must fail when DB contract key tampered")
		}
	} else if !strings.Contains(err.Error(), "cannot be updated") && !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("unexpected tamper error: %v", err)
	}

	store.Corrupt(binding.Record.ObjectKey, []byte(`{"tampered":true}`))
	if _, err = sealer.LoadValidated(ctx, binding.ID, seller); err == nil {
		t.Fatal("LoadValidated must fail when MinIO manifest tampered")
	}
}
