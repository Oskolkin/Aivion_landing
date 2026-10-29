package resolve

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFactFamilyCapabilityRegistryStockMultiCap(t *testing.T) {
	keys := RelatedCapabilityKeys("STOCK", "STOCK.SNAPSHOT")
	want := map[string]bool{
		"STOCK.SKU_CURRENT": true, "STOCK.LOCATION_BREAKDOWN": true, "STOCK.PROVIDER_TURNOVER": true,
		"STOCK.COVERAGE_7D": true, "STOCK.COVERAGE_30D": true,
	}
	if len(keys) != len(want) {
		t.Fatalf("stock keys=%v", keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected key %s", k)
		}
	}
	caps := []CapabilitySnapshot{
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY", SourceState: "SUCCESS"},
		{Key: "STOCK.LOCATION_BREAKDOWN", Scope: "HISTORICAL_QUERY", FinalState: "PARTIAL", SourceState: "PARTIAL"},
		{Key: "STOCK.PROVIDER_TURNOVER", Scope: "HISTORICAL_QUERY", FinalState: "UNAVAILABLE", SourceState: "NOT_ATTEMPTED"},
		{Key: "STOCK.COVERAGE_7D", Scope: "CURRENT_COLLECTION", FinalState: "READY", SourceState: "SUCCESS"},
	}
	matched := matchCapabilities(caps, "STOCK", "STOCK.SNAPSHOT")
	if len(matched) != 4 {
		t.Fatalf("expected 4 matched caps, got %d", len(matched))
	}
	byKey := map[string]string{}
	for _, c := range matched {
		byKey[c.Key] = c.FinalState
	}
	if byKey["STOCK.SKU_CURRENT"] != "READY" || byKey["STOCK.LOCATION_BREAKDOWN"] != "PARTIAL" || byKey["STOCK.PROVIDER_TURNOVER"] != "UNAVAILABLE" || byKey["STOCK.COVERAGE_7D"] != "READY" {
		t.Fatalf("capability states lost: %+v", byKey)
	}
}

func TestNoPrefixGuessingForAdvertisingTraffic(t *testing.T) {
	keys := RelatedCapabilityKeys("ADVERTISING", "ADVERTISING.TRAFFIC")
	for _, k := range keys {
		if strings.HasPrefix(k, "ADVERTISING.CAMPAIGN_ATTRIBUTION") || k == "ADVERTISING.SKU_ATTRIBUTION" {
			t.Fatalf("attribution capability leaked into TRAFFIC registry: %s", k)
		}
	}
	caps := []CapabilitySnapshot{
		{Key: "ADVERTISING.CAMPAIGN_ATTRIBUTION", Scope: "HISTORICAL_QUERY", FinalState: "FAILED", SourceState: "FAILED"},
		{Key: "ADVERTISING.CAMPAIGN_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY", SourceState: "SUCCESS"},
	}
	matched := matchCapabilities(caps, "ADVERTISING", "ADVERTISING.TRAFFIC")
	if len(matched) != 1 || matched[0].Key != "ADVERTISING.CAMPAIGN_TRAFFIC" {
		t.Fatalf("prefix guessing still active: %+v", matched)
	}
}

func TestSourceStateIndependentOfCapability(t *testing.T) {
	obs := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	run := int64(9)
	// Capability would prefer PARTIAL/FAILED; SourceState must follow run evidence.
	if got := resolveSourceState(&obs, &run, "COMPLETED"); got != "SUCCESS" {
		t.Fatalf("completed run source_state=%s", got)
	}
	if got := resolveSourceState(&obs, &run, "PARTIAL"); got != "PARTIAL" {
		t.Fatalf("partial run source_state=%s", got)
	}
	if got := resolveSourceState(nil, nil, ""); got != "NOT_ATTEMPTED" {
		t.Fatalf("empty evidence source_state=%s", got)
	}
}

func TestResolveContractHashNoSyntheticDefaults(t *testing.T) {
	real := strings.Repeat("a", 64)
	derived := factualapi.DeriveSourceContractHash("OZON_X", "2.0.0", "FAMILY")
	if got := resolveContractHash(real, "OZON_X", "2.0.0", "FAMILY"); got != derived {
		t.Fatalf("must derive from run fields when available, got %s want %s", got, derived)
	}
	derivedSearch := resolveContractHash("", "OZON_ANALYTICS_PRODUCT_QUERIES", "1.2.0", "SEARCH_QUERIES")
	if derivedSearch == "" || derivedSearch == "SEARCH" || len(derivedSearch) != 64 {
		t.Fatalf("expected derived 64-hex contract hash, got %q", derivedSearch)
	}
	if got := resolveContractHash(real, "", "", ""); got != real {
		t.Fatalf("stored hash used when run fields absent: %s", got)
	}
	if got := resolveContractHash("", "", "", ""); got != "" {
		t.Fatalf("empty provenance must not invent hash: %q", got)
	}
}

func TestNotEvaluatedDQHonest(t *testing.T) {
	dq := notEvaluatedDQ()
	if len(dq) != 1 {
		t.Fatalf("len=%d", len(dq))
	}
	if dq[0].HighestSeverity != "NONE" || dq[0].DQPolicyVersion != "NOT_EVALUATED" || dq[0].FactFamily != "NOT_EVALUATED" {
		t.Fatalf("not honest empty: %+v", dq[0])
	}
	if dq[0].DQRunID != nil {
		t.Fatal("NO_DQ_RUN must not invent run id")
	}
}

// TestDQQueryErrorFailsClosed documents that only pgx.ErrNoRows yields honest
// NOT_EVALUATED DQ (see TestNotEvaluatedDQHonest). Any other pool/query failure
// must abort Resolve — never fail-open to empty or clean DQ.
func TestDQQueryErrorFailsClosed(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://p82_fail:closed@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _, err = Resolver{Pool: pool}.Resolve(ctx, Input{
		SellerID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		RunKind:  "EVALUATION", IntentHash: factualapi.StableHash("p82-dq-fail"),
		Policies: factualapi.CurrentPolicies(),
	})
	if err == nil {
		t.Fatal("expected Resolve to fail closed on DB/query error")
	}
}

func TestRegistryVersionConstant(t *testing.T) {
	if FACT_FAMILY_CAPABILITY_REGISTRY_V1 == "" {
		t.Fatal("missing registry version")
	}
}
