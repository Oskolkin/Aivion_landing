//go:build integration

package collection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryLifecycleTransitionsUseDatabaseCAS(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run collection integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}

	var userID, sellerID, runID, domainRunID int64
	email := fmt.Sprintf("collection_cas_%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'collection-cas','active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','collection.cas.integration') RETURNING id`, sellerID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,'CATALOG','RUNNING',NOW()) RETURNING id`, runID, sellerID).Scan(&domainRunID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(dbgen.New(pool))
	if _, err := repo.TransitionDomainRun(ctx, sellerID, domainRunID, DomainRunStatusRunning, DomainRunStatusCompleted); err != nil {
		t.Fatalf("domain CAS completion: %v", err)
	}
	_, err = repo.TransitionDomainRun(ctx, sellerID, domainRunID, DomainRunStatusRunning, DomainRunStatusFailed)
	var stale *StaleTransitionError
	if !errors.As(err, &stale) {
		t.Fatalf("domain stale error=%v, want StaleTransitionError", err)
	}
	domain, err := repo.GetDomainRun(ctx, sellerID, domainRunID)
	if err != nil || domain.Status != DomainRunStatusCompleted {
		t.Fatalf("stale domain writer changed final status: status=%s err=%v", domain.Status, err)
	}

	if _, err := repo.TransitionRun(ctx, sellerID, runID, CollectionRunStatusRunning, CollectionRunStatusCompleted); err != nil {
		t.Fatalf("run CAS completion: %v", err)
	}
	_, err = repo.TransitionRun(ctx, sellerID, runID, CollectionRunStatusRunning, CollectionRunStatusFailed)
	stale = nil
	if !errors.As(err, &stale) {
		t.Fatalf("run stale error=%v, want StaleTransitionError", err)
	}
	run, err := repo.GetRun(ctx, sellerID, runID)
	if err != nil || run.Status != CollectionRunStatusCompleted {
		t.Fatalf("stale run writer changed final status: status=%s err=%v", run.Status, err)
	}

	if _, err := repo.TransitionRun(ctx, sellerID+1, runID, CollectionRunStatusCompleted, CollectionRunStatusFailed); !errors.As(err, &stale) {
		t.Fatalf("cross-seller CAS error=%v, want StaleTransitionError", err)
	}

	contract := NewSourceContract("CATALOG_CONTENT_PARTIAL", "OZON:CATALOG", "v2", "/v3/product", "catalog-v2", "stable semantic definition")
	contractRun, err := repo.CreateDomainRunWithContract(ctx, runID, sellerID, DomainCatalog, &contract, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := repo.GetDomainRun(ctx, sellerID, contractRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.SourceContractKey != contract.ContractKey || persisted.SourceContractVersion != contract.ContractVersion || persisted.SemanticMappingVersion != contract.SemanticMappingVersion {
		t.Fatalf("persisted source contract=%+v", persisted)
	}
	if persisted.SemanticMappingHash != contract.SemanticMappingHash {
		t.Fatalf("semantic_mapping_hash=%q want %q", persisted.SemanticMappingHash, contract.SemanticMappingHash)
	}
	if persisted.SourceRevisionHash != "" {
		t.Fatalf("source revision must be empty when provider exposes no revision, got %q", persisted.SourceRevisionHash)
	}
}
