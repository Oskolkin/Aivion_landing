//go:build integration

package analysis_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type prompt82FamilySpec struct {
	domain, family         string
	contractKey, contractV string
	endpoint, mapping      string
	revSeed, wmSeed        string
	capKey                 string
}

func TestPrompt82MultiFamilyResolveSealLoadValidated(t *testing.T) {
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	pool := prompt81Pool(t)
	ctx := context.Background()
	seller := prompt81Seller(t, pool)
	cut := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
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

	families := []prompt82FamilySpec{
		{
			domain: "ADVERTISING", family: "ADVERTISING.TRAFFIC",
			contractKey: "OZON_ADV_DAILY", contractV: "current_json_v2",
			endpoint: "ADVERTISING_DAILY", mapping: "advertising_daily_mapping_v2",
			revSeed: "adv-traffic-rev", wmSeed: "adv-traffic-wm",
			capKey: "ADVERTISING.CAMPAIGN_TRAFFIC",
		},
		{
			domain: "ADVERTISING", family: "ADVERTISING.ATTRIBUTION",
			contractKey: "OZON_ADV_CPO", contractV: "async_generate_v1",
			endpoint: "ADVERTISING_CPO", mapping: "advertising_cpo_mapping_v1",
			revSeed: "adv-attr-rev", wmSeed: "adv-attr-wm",
			capKey: "ADVERTISING.CAMPAIGN_ATTRIBUTION",
		},
		{
			domain: "SEARCH", family: "SEARCH.SELLER_SUMMARY",
			contractKey: "OZON_ANALYTICS_PRODUCT_QUERIES", contractV: "v1",
			endpoint: "SEARCH_QUERIES", mapping: "canonical_search_sku_summary_v1",
			revSeed: "search-summary-rev", wmSeed: "search-summary-wm",
			capKey: "SEARCH.SELLER_SUMMARY",
		},
	}

	type seeded struct {
		spec  prompt82FamilySpec
		runID int64
		rev   string
		wm    string
	}
	var seededFamilies []seeded
	for _, f := range families {
		rev := prompt81Hex(f.revSeed)
		wm := prompt81Hex(f.wmSeed)
		var parent, run int64
		if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
			VALUES($1,'MANUAL','COMPLETED','prompt82.acceptance',$2,$2)RETURNING id`, seller, cut).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
			collection_run_id,seller_account_id,domain,status,started_at,finished_at,
			source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
			VALUES($1,$2,$3,'COMPLETED',$4,$4,$5,$6,$7,$8,$9)RETURNING id`,
			parent, seller, f.domain, cut, f.contractKey, f.contractV, f.endpoint, f.mapping, rev).Scan(&run); err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermarks(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$5::date,$5::date,$6,$7,$7,$4)
			ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET
			  latest_observed_at=EXCLUDED.latest_observed_at,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at,
			  last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id,source_contract_hash=EXCLUDED.source_contract_hash`,
			seller, f.domain, f.family, cut, cut.AddDate(0, 0, -1), run, wm)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$5::date,$5::date,$6,$7,$7,$4,$4)`,
			seller, f.domain, f.family, cut, cut.AddDate(0, 0, -1), run, wm)
		if err != nil {
			t.Fatal(err)
		}
		seededFamilies = append(seededFamilies, seeded{spec: f, runID: run, rev: rev, wm: wm})
	}

	var dqRun int64
	if err = pool.QueryRow(ctx, `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, cut, prompt81Hex("p82-dq-in"), prompt81Hex("p82-dq-res")).Scan(&dqRun); err != nil {
		t.Fatal(err)
	}
	for _, s := range seededFamilies {
		_, err = pool.Exec(ctx, `INSERT INTO business_capability_assessments(
			seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
			contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
			reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
			VALUES($1,$2,$3,'HISTORICAL_QUERY',$4,'READY',
			 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','PARTIAL','NON_EMPTY',
			 'OK','{}','capability_policy_v3',$5,$6)`,
			seller, dqRun, s.spec.capKey, s.spec.domain, cut.Add(-time.Minute), s.runID)
		if err != nil {
			t.Fatal(err)
		}
	}

	resolved, _, hash, err := resolve.Resolver{Pool: pool}.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: "EVALUATION", IntentHash: runtime.StandardIntentHash(), ContractBundleHash: bundle.Hash(),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]resolve.SourceBinding{}
	for _, s := range resolved.Sources {
		byKey[s.Domain+"/"+s.FactFamily] = s
	}
	for _, k := range []string{
		"ADVERTISING/ADVERTISING.TRAFFIC",
		"ADVERTISING/ADVERTISING.ATTRIBUTION",
		"SEARCH/SEARCH.SELLER_SUMMARY",
	} {
		if _, ok := byKey[k]; !ok {
			t.Fatalf("resolved Sources missing %s; got %#v", k, byKey)
		}
	}
	for _, s := range seededFamilies {
		got := byKey[s.spec.domain+"/"+s.spec.family]
		if got.SourceContractKey != s.spec.contractKey || got.SourceContractVersion != s.spec.contractV {
			t.Fatalf("%s contract: key=%s ver=%s", s.spec.family, got.SourceContractKey, got.SourceContractVersion)
		}
		if got.SourceEndpointFamily != s.spec.endpoint || got.SemanticMappingVersion != s.spec.mapping {
			t.Fatalf("%s endpoint/mapping: %+v", s.spec.family, got)
		}
		if got.SourceRevisionHash != s.rev || got.WatermarkHash != s.wm {
			t.Fatalf("%s rev/wm mismatch: rev=%s wm=%s", s.spec.family, got.SourceRevisionHash, got.WatermarkHash)
		}
		// Forbidden synthetic defaults: Domain as contract key + literal "v1".
		if got.SourceContractKey == s.spec.domain && got.SourceContractVersion == "v1" {
			t.Fatalf("forbidden synthetic Domain/v1 for %s", s.spec.family)
		}
	}

	run, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: run.ID, ResolvedContextHash: hash, Resolved: &resolved,
		EntitySets: []universe.EntitySet{{
			EntityKind: "SKU", SetHash: canonicaljson.Hash([]string{"1"}), MemberCount: 1, Members: []string{"1"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := sealer.LoadValidated(ctx, sealed.ID, seller)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Manifest.Sources) != 3 {
		t.Fatalf("LoadValidated sources=%d want 3", len(loaded.Manifest.Sources))
	}
	loadedBy := map[string]universe.Source{}
	for _, s := range loaded.Manifest.Sources {
		loadedBy[s.Domain+"/"+s.FactFamily] = s
	}
	for _, s := range seededFamilies {
		got := loadedBy[s.spec.domain+"/"+s.spec.family]
		if got.ContractKey != s.spec.contractKey || got.ContractVersion != s.spec.contractV {
			t.Fatalf("sealed contract lost for %s: %+v", s.spec.family, got)
		}
		if got.SourceEndpointFamily != s.spec.endpoint || got.SemanticMappingVersion != s.spec.mapping {
			t.Fatalf("sealed endpoint/mapping lost for %s: %+v", s.spec.family, got)
		}
		if got.SourceRevisionHash != s.rev || got.WatermarkHash != s.wm {
			t.Fatalf("sealed rev/wm lost for %s: %+v", s.spec.family, got)
		}
		if got.ContractKey == s.spec.domain && got.ContractVersion == "v1" {
			t.Fatalf("sealed synthetic Domain/v1 for %s", s.spec.family)
		}
	}

	rc, err := universe.NewSealedContext(loaded.Record, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rc.DomainBinding("ADVERTISING"); ok {
		t.Fatal("DomainBinding(ADVERTISING) must be ambiguous (ok=false) with TRAFFIC+ATTRIBUTION")
	}
	traffic, ok := rc.Binding("ADVERTISING", "ADVERTISING.TRAFFIC")
	if !ok || traffic.FactFamily != "ADVERTISING.TRAFFIC" || traffic.ContractKey != "OZON_ADV_DAILY" {
		t.Fatalf("Binding(ADVERTISING,TRAFFIC) failed: ok=%v %+v", ok, traffic)
	}
}

func TestPrompt82CutoffCoherenceASKnownAt(t *testing.T) {
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	pool := prompt81Pool(t)
	ctx := context.Background()
	seller := prompt81Seller(t, pool)
	t0 := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)
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

	hashA := prompt81Hex("cutoff-state-A")
	hashB := prompt81Hex("cutoff-state-B")
	revA := prompt81Hex("cutoff-rev-A")
	revB := prompt81Hex("cutoff-rev-B")

	insertRun := func(at time.Time, key, ver, endpoint, mapping, rev string) int64 {
		t.Helper()
		var parent, run int64
		if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
			VALUES($1,'MANUAL','COMPLETED','prompt82.cutoff',$2,$2)RETURNING id`, seller, at).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
			collection_run_id,seller_account_id,domain,status,started_at,finished_at,
			source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
			VALUES($1,$2,'STOCK','COMPLETED',$3,$3,$4,$5,$6,$7,$8)RETURNING id`,
			parent, seller, at, key, ver, endpoint, mapping, rev).Scan(&run); err != nil {
			t.Fatal(err)
		}
		return run
	}
	runA := insertRun(t0, "OZON_STOCK_PRIMARY", "1.0.0", "STOCK_WAREHOUSE", "canonical-stock-v1", revA)
	runB := insertRun(t1, "OZON_STOCK_PRIMARY", "1.0.1", "STOCK_WAREHOUSE", "canonical-stock-v1", revB)

	persistWM := func(at time.Time, hash string, runID int64) {
		t.Helper()
		_, err := pool.Exec(ctx, `INSERT INTO domain_watermarks(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_snapshot_at,
			last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at)
			VALUES($1,'STOCK','STOCK.SNAPSHOT',$2,$2,$3,$4,$4,$2)
			ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET
			  latest_observed_at=EXCLUDED.latest_observed_at,latest_complete_snapshot_at=EXCLUDED.latest_complete_snapshot_at,
			  last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id,
			  source_contract_hash=EXCLUDED.source_contract_hash,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at`,
			seller, at, runID, hash)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_snapshot_at,
			last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,'STOCK','STOCK.SNAPSHOT',$2,$2,$3,$4,$4,$2,$2)`,
			seller, at, runID, hash)
		if err != nil {
			t.Fatal(err)
		}
	}
	persistWM(t0, hashA, runA)

	var dqA int64
	if err = pool.QueryRow(ctx, `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, t0, prompt81Hex("p82-cut-dq-a"), prompt81Hex("p82-cut-dq-a-res")).Scan(&dqA); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES($1,$2,'STOCK.SKU_CURRENT','HISTORICAL_QUERY','STOCK','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','PARTIAL','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4)`,
		seller, dqA, t0.Add(-time.Second), runA)
	if err != nil {
		t.Fatal(err)
	}

	// After cutoff: watermark B + newer capability must not leak into cutoff=t0 resolve/seal.
	persistWM(t1, hashB, runB)
	var dqB int64
	if err = pool.QueryRow(ctx, `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, t1, prompt81Hex("p82-cut-dq-b"), prompt81Hex("p82-cut-dq-b-res")).Scan(&dqB); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES($1,$2,'STOCK.LOCATION_BREAKDOWN','HISTORICAL_QUERY','STOCK','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','PARTIAL','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4)`,
		seller, dqB, t1, runB)
	if err != nil {
		t.Fatal(err)
	}

	resolved, _, hash, err := resolve.Resolver{Pool: pool}.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: t0.AddDate(0, 0, -1), CutoffAt: t0,
		RunKind: "EVALUATION", IntentHash: runtime.StandardIntentHash(), ContractBundleHash: bundle.Hash(),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var stock *resolve.SourceBinding
	for i := range resolved.Sources {
		if resolved.Sources[i].Domain == "STOCK" && resolved.Sources[i].FactFamily == "STOCK.SNAPSHOT" {
			stock = &resolved.Sources[i]
			break
		}
	}
	if stock == nil {
		t.Fatal("STOCK.SNAPSHOT missing after cutoff resolve")
	}
	if stock.WatermarkHash != hashA {
		t.Fatalf("cutoff coherence: want watermark A, got %s", stock.WatermarkHash)
	}
	if stock.SourceContractVersion != "1.0.0" || stock.SourceRevisionHash != revA {
		t.Fatalf("cutoff must keep run A provenance: %+v", stock)
	}
	for _, c := range resolved.Capabilities {
		if c.Key == "STOCK.LOCATION_BREAKDOWN" {
			t.Fatal("post-cutoff capability B leaked into cutoff=t0 resolve")
		}
	}

	eval, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: t0.AddDate(0, 0, -1), CutoffAt: t0,
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal(ctx, universe.Input{
		SellerID: seller, RunID: eval.ID, ResolvedContextHash: hash, Resolved: &resolved,
		EntitySets: []universe.EntitySet{{
			EntityKind: "SKU", SetHash: prompt81Hex("empty"), MemberCount: 0,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := sealer.LoadValidated(ctx, sealed.ID, seller)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range loaded.Manifest.Sources {
		if s.Domain == "STOCK" && s.FactFamily == "STOCK.SNAPSHOT" {
			found = true
			if s.WatermarkHash != hashA {
				t.Fatalf("sealed cutoff coherence: want A, got %s", s.WatermarkHash)
			}
			if s.ContractVersion != "1.0.0" {
				t.Fatalf("sealed contract version leaked B: %s", s.ContractVersion)
			}
		}
	}
	if !found {
		t.Fatal("sealed STOCK.SNAPSHOT missing")
	}
}
