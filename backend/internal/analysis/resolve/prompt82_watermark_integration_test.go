//go:build integration

package resolve_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func prompt82Pool(t *testing.T) *pgxpool.Pool {
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

func prompt82Seller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var user, seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'x','active')RETURNING id`,
		fmt.Sprintf("p82_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'p82','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	return seller
}

func prompt82DomainRun(t *testing.T, pool *pgxpool.Pool, seller int64, at time.Time, contractKey, contractVersion, endpoint, mappingVer, revHash string) int64 {
	t.Helper()
	ctx := context.Background()
	var parent, run int64
	if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
		VALUES($1,'MANUAL','COMPLETED','prompt82',$2,$2)RETURNING id`, seller, at).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
		collection_run_id,seller_account_id,domain,status,started_at,finished_at,
		source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,source_revision_hash)
		VALUES($1,$2,'STOCK','COMPLETED',$3,$3,$4,$5,$6,$7,$8)RETURNING id`,
		parent, seller, at, contractKey, contractVersion, endpoint, mappingVer, revHash).Scan(&run); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPrompt82WatermarkABAAndCutoffRace(t *testing.T) {
	pool := prompt82Pool(t)
	ctx := context.Background()
	seller := prompt82Seller(t, pool)
	svc := readiness.NewService(pool, nil, readiness.StaticSemanticRegistry{})

	t0 := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	t2 := t0.Add(2 * time.Minute)

	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	revHash := strings.Repeat("c", 64)
	runA := prompt82DomainRun(t, pool, seller, t0, "OZON_STOCK_PRIMARY", "1.0.0", "STOCK_WAREHOUSE", "canonical-stock-v1", revHash)
	runB := prompt82DomainRun(t, pool, seller, t1, "OZON_STOCK_PRIMARY", "1.0.1", "STOCK_WAREHOUSE", "canonical-stock-v1", revHash)

	persist := func(at time.Time, hash string, runID int64) {
		t.Helper()
		obs := at
		w := readiness.Watermark{
			Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT",
			LatestObservedAt: &obs, LatestCompleteSnapshotAt: &obs,
			DomainRunID: &runID, SourceContractHash: hash, StateHash: hash, AssessedAt: at,
		}
		if err := svc.PersistWatermarkForTest(ctx, seller, &w); err != nil {
			t.Fatal(err)
		}
	}

	persist(t0, hashA, runA)
	persist(t0.Add(time.Second), hashA, runA) // A→A: no new revision
	persist(t1, hashB, runB)                  // A→B
	persist(t2, hashA, runA)                  // A→B→A

	var revCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domain_watermark_revisions
		WHERE seller_account_id=$1 AND domain='STOCK' AND fact_family='STOCK.SNAPSHOT'`, seller).Scan(&revCount); err != nil {
		t.Fatal(err)
	}
	if revCount != 3 {
		t.Fatalf("expected 3 revisions for A→A skip + A→B + B→A, got %d", revCount)
	}

	var latestProj string
	if err := pool.QueryRow(ctx, `SELECT state_hash FROM domain_watermarks
		WHERE seller_account_id=$1 AND domain='STOCK' AND fact_family='STOCK.SNAPSHOT'`, seller).Scan(&latestProj); err != nil {
		t.Fatal(err)
	}
	if latestProj != hashA {
		t.Fatalf("projection should be A, got %s", latestProj)
	}

	resolver := resolve.Resolver{Pool: pool}
	atCutoff, _, _, err := resolver.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: t0.AddDate(0, 0, -1), CutoffAt: t0,
		RunKind: "EVALUATION", IntentHash: factualapi.StableHash("intent"),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	src := findStockSource(t, atCutoff)
	if src.WatermarkHash != hashA {
		t.Fatalf("cutoff race: expected hash A at 10:00, got %s", src.WatermarkHash)
	}
	if src.SourceContractKey != "OZON_STOCK_PRIMARY" || src.SourceContractVersion != "1.0.0" {
		t.Fatalf("synthetic provenance still present: key=%s ver=%s", src.SourceContractKey, src.SourceContractVersion)
	}
	if src.SourceEndpointFamily != "STOCK_WAREHOUSE" || src.SemanticMappingVersion != "canonical-stock-v1" {
		t.Fatalf("endpoint/mapping missing: %+v", src)
	}
	if src.SourceRevisionHash != revHash {
		t.Fatalf("source_revision_hash=%s", src.SourceRevisionHash)
	}
	if src.SourceContractKey == "STOCK" || src.SourceContractVersion == "v1" {
		t.Fatal("forbidden synthetic Domain/v1 defaults")
	}

	afterB, _, _, err := resolver.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: t1.AddDate(0, 0, -1), CutoffAt: t1,
		RunKind: "EVALUATION", IntentHash: factualapi.StableHash("intent"),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if findStockSource(t, afterB).WatermarkHash != hashB {
		t.Fatalf("at t1 expected B")
	}

	latest, _, _, err := resolver.Resolve(ctx, resolve.Input{
		SellerID: seller, AnalysisDate: t2.AddDate(0, 0, -1), CutoffAt: t2,
		RunKind: "EVALUATION", IntentHash: factualapi.StableHash("intent"),
		Policies: factualapi.CurrentPolicies(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if findStockSource(t, latest).WatermarkHash != hashA {
		t.Fatalf("at t2 expected return to A")
	}
}

func findStockSource(t *testing.T, m resolve.Manifest) resolve.SourceBinding {
	t.Helper()
	for _, s := range m.Sources {
		if s.Domain == "STOCK" && s.FactFamily == "STOCK.SNAPSHOT" {
			return s
		}
	}
	t.Fatal("STOCK.SNAPSHOT binding missing")
	return resolve.SourceBinding{}
}
