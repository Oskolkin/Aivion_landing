//go:build integration

package collection_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/accounthealth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/finance"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/funnel"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/promotions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/search"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/supply"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPrompt7CanonicalDomainsAsKnownAt(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var user, seller, sku int64
	err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`,
		fmt.Sprintf("prompt7_%d@test.local", time.Now().UnixNano())).Scan(&user)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'prompt7','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,7001,NOW()-INTERVAL '1 day',NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku); err != nil {
		t.Fatal(err)
	}

	type runFixture struct{ run, payload int64 }
	newRun := func(domain, label string) runFixture {
		t.Helper()
		var parent int64
		var f runFixture
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING',$2)RETURNING id`, seller, label).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,$3,'RUNNING',NOW())RETURNING id`, parent, seller, domain).Scan(&f.run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON',$3,$4,1,'raw',$5,$6,200,NOW(),'{}','TEST_REDACTION_V1')RETURNING id`,
			seller, f.run, "prompt7."+strings.ToLower(domain), label, "prompt7/"+label, fmt.Sprintf("%064x", f.run)).Scan(&f.payload); e != nil {
			t.Fatal(e)
		}
		return f
	}

	day := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	initialAt := time.Now().UTC().Add(-time.Minute)
	sourceSKU, units := int64(7001), int64(12)
	initialRuns := map[string]int64{}
	hexHash := func(seed string) string {
		sum := 0
		for i := 0; i < len(seed); i++ {
			sum = sum*31 + int(seed[i])
		}
		return fmt.Sprintf("%064x", sum)
	}
	contractHash := hexHash("funnel-contract")
	financeHashV1 := hexHash("finance-v1")
	financeHashV2 := hexHash("finance-v2")

	fr := newRun("FUNNEL", "prompt7.funnel.1")
	initialRuns["FUNNEL"] = fr.run
	funnelResult := funnel.Result{RequestedFrom: day, RequestedTo: day, RecordsReceived: 2, Pages: 1, Observations: []funnel.Observation{
		{ScopeKind: "ACCOUNT", MetricKey: "SEARCH_IMPRESSIONS", PeriodStart: day, PeriodEnd: day, ValueNumeric: "100", Unit: "COUNT", TemporalSemantics: "ADDITIVE_FLOW", EntityAggregationSemantics: "ADDITIVE", Surface: "SEARCH_CATALOG", ProviderMetricName: "hits_view_search", RegistryVersion: "test", SourceContractHash: contractHash, SourcePayloadID: fr.payload, SKUResolutionStatus: "NOT_APPLICABLE"},
		{ScopeKind: "SKU", SourceSKU: &sourceSKU, CanonicalSKUID: &sku, SKUResolutionStatus: "RESOLVED", MetricKey: "SEARCH_IMPRESSIONS", PeriodStart: day, PeriodEnd: day, ValueNumeric: "60", Unit: "COUNT", TemporalSemantics: "ADDITIVE_FLOW", EntityAggregationSemantics: "ADDITIVE", Surface: "SEARCH_CATALOG", ProviderMetricName: "hits_view_search", RegistryVersion: "test", SourceContractHash: contractHash, SourcePayloadID: fr.payload},
	}}
	if err = funnel.NewRepositoryWithClock(pool, func() time.Time { return initialAt }).Finalize(ctx, seller, fr.run, &funnelResult); err != nil {
		t.Fatal(err)
	}

	fin := newRun("FINANCE", "prompt7.finance.1")
	initialRuns["FINANCE"] = fin.run
	financeResult := finance.Result{CompletenessStatus: "COMPLETE", PagesReceived: 1, Days: []finance.DayRevision{{
		BusinessDate: day, CompletenessStatus: "COMPLETE", StateHash: financeHashV1, SourcePayloadID: fin.payload,
		Records: []finance.Record{{RowOrdinal: 1, ProviderTypeID: "operation", TotalAmountSigned: "100", CurrencyCode: "RUB",
			Lines: []finance.Line{{Ordinal: 1, SourceComponentPath: "services[0]", SignedAmount: "20", MappingStatus: "MAPPED"}, {Ordinal: 2, SourceComponentPath: "items[0]", SignedAmount: "10", MappingStatus: "MAPPED", CanonicalSKUID: &sku}}},
		}}}}
	if err = finance.NewRepository(pool).Finalize(ctx, seller, fin.run, &financeResult); err != nil {
		t.Fatal(err)
	}
	var parentNet, childExplanation string
	if err = pool.QueryRow(ctx, `SELECT r.total_amount_signed::text,(SELECT COALESCE(sum(l.signed_amount),0)::text FROM finance_accrual_lines l WHERE l.finance_accrual_record_id=r.id) FROM finance_accrual_records r WHERE r.seller_account_id=$1 ORDER BY r.id DESC LIMIT 1`, seller).Scan(&parentNet, &childExplanation); err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(strings.TrimRight(parentNet, "0"), ".") != "100" || strings.TrimRight(strings.TrimRight(childExplanation, "0"), ".") != "30" {
		t.Fatalf("finance parent/children = %s/%s; children must be explanation only", parentNet, childExplanation)
	}

	sr := newRun("SEARCH", "prompt7.search.1")
	initialRuns["SEARCH"] = sr.run
	query := search.QueryIdentity{Provider: "OZON", Market: "RU", QueryText: "blue mug", NormalizedQueryKey: "blue mug"}
	searchResult := search.Result{StartedAt: initialAt, CompletedAt: initialAt, PagesReceived: 2, SellerSummaryStatus: "COMPLETE", QueryDetailsStatus: "COMPLETE", MarketSearchStatus: "UNAVAILABLE", CompletenessStatus: "COMPLETE",
		SKUSummaries: []search.SKUSummary{{CanonicalSKUID: &sku, SourceSKU: sourceSKU, SKUResolutionStatus: "RESOLVED", PeriodStart: day, PeriodEnd: day, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "500", Orders: "5", UniqueSearchUsers: "40", UniqueUsersAggregation: "NON_ADDITIVE", SourcePayloadID: sr.payload, ObservedAt: initialAt}},
		Details:      []search.Detail{{Identity: query, CanonicalSKUID: &sku, SourceSKU: sourceSKU, SKUResolutionStatus: "RESOLVED", ProviderRank: 1, ProviderSearchGMV: "300", Orders: "3", RetrievalScope: "TOP_N_PROVIDER_RANKED", RetrievalReason: "TOP_N_PROVIDER_RANKED", TopNLimit: 15, PeriodStart: day, PeriodEnd: day, SourcePayloadID: sr.payload, ObservedAt: initialAt}},
	}
	if err = search.NewRepository(pool).Finalize(ctx, seller, sr.run, &searchResult); err != nil {
		t.Fatal(err)
	}

	pr := newRun("PROMOTIONS", "prompt7.promotions.1")
	initialRuns["PROMOTIONS"] = pr.run
	promotionResult := promotions.Result{StartedAt: initialAt, CompletedAt: initialAt, CompletenessStatus: "COMPLETE",
		Definitions:             []promotions.DefinitionObservation{{Origin: "OZON", ProviderActionID: 77, Title: "Prompt7", ProviderState: "ACTIVE", SourcePayloadID: pr.payload}},
		Participants:            []promotions.SKUObservation{{Origin: "OZON", ProviderActionID: 77, CanonicalSKUID: &sku, SourceSKU: &sourceSKU, SKUResolutionStatus: "RESOLVED", SourcePayloadID: pr.payload}},
		Eligibility:             []promotions.EligibilitySnapshot{{Origin: "OZON", ProviderActionID: 77, CompletenessStatus: "COMPLETE", SourcePayloadID: &pr.payload, Rows: []promotions.SKUObservation{{Origin: "OZON", ProviderActionID: 77, CanonicalSKUID: &sku, SourceSKU: &sourceSKU, SKUResolutionStatus: "RESOLVED", SourcePayloadID: pr.payload}}}},
		CompleteParticipantKeys: []promotions.PromotionKey{{Origin: "OZON", ProviderActionID: 77}},
	}
	if err = promotions.NewRepository(pool).Finalize(ctx, seller, pr.run, &promotionResult); err != nil {
		t.Fatal(err)
	}

	hr := newRun("ACCOUNT_HEALTH", "prompt7.health.1")
	initialRuns["ACCOUNT_HEALTH"] = hr.run
	healthResult := accounthealth.Result{StartedAt: initialAt, CompletedAt: initialAt, CompletenessStatus: "COMPLETE", SummaryPayloadID: hr.payload, SummaryRequests: 1,
		Metrics: []accounthealth.MetricObservation{{ProviderRatingKey: "future_unknown_rating", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", Value: "7", ProviderStatus: "WARNING", SourcePayloadID: hr.payload}},
	}
	if err = accounthealth.NewRepository(pool).Finalize(ctx, seller, hr.run, &healthResult); err != nil {
		t.Fatal(err)
	}

	sur := newRun("SUPPLY", "prompt7.supply.1")
	initialRuns["SUPPLY"] = sur.run
	supplyResult := supply.Result{StartedAt: initialAt, CompletedAt: initialAt, CompletenessStatus: "COMPLETE", ListPages: 1, DetailRequests: 1, BundlePages: 1,
		Orders:   []supply.OrderObservation{{ProviderOrderID: "ORDER-7", ProviderState: "READY_TO_SUPPLY", CanonicalState: "READY", StateMappingStatus: "MAPPED", SourcePayloadID: sur.payload}},
		Supplies: []supply.SupplyObservation{{ProviderOrderID: "ORDER-7", ProviderSupplyID: "SUPPLY-7", ProviderState: "READY_TO_SUPPLY", CanonicalState: "READY", StateMappingStatus: "MAPPED", SourcePayloadID: sur.payload}},
		Bundles:  []supply.BundleObservation{{ProviderSupplyID: "SUPPLY-7", ProviderBundleID: "BUNDLE-7", CompletenessStatus: "COMPLETE", PageCount: 1, SourcePayloadID: &sur.payload, Items: []supply.PlannedItem{{CanonicalSKUID: &sku, SourceSKU: &sourceSKU, SKUResolutionStatus: "RESOLVED", DeclaredUnits: &units, SourcePayloadID: sur.payload}}}},
	}
	if err = supply.NewRepository(pool).Finalize(ctx, seller, sur.run, &supplyResult); err != nil {
		t.Fatal(err)
	}
	var stockRows, stateObs int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM stock_snapshots WHERE seller_account_id=$1`, seller).Scan(&stockRows); err != nil || stockRows != 0 {
		t.Fatalf("supply must not create stock snapshots: rows=%d err=%v", stockRows, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM supply_state_observations WHERE seller_account_id=$1`, seller).Scan(&stateObs); err != nil || stateObs != 2 {
		t.Fatalf("expected 2 initial state observations (order+supply), got %d err=%v", stateObs, err)
	}

	cutoff := time.Now().UTC()
	api := prompt7FactualService(t, pool)
	c1 := prompt7Context(t, seller, day, cutoff, initialRuns)
	before := prompt7ReadFacts(t, ctx, api, c1, day, sku)

	laterAt := cutoff.Add(time.Minute)
	laterRuns := map[string]int64{}
	revise := func(domain string) runFixture {
		f := newRun(domain, "prompt7."+strings.ToLower(domain)+".2")
		laterRuns[domain] = f.run
		return f
	}
	f2 := revise("FUNNEL")
	funnelResult.Observations[0].ValueNumeric, funnelResult.Observations[0].SourcePayloadID = "200", f2.payload
	funnelResult.Observations[1].SourcePayloadID = f2.payload
	if err = funnel.NewRepositoryWithClock(pool, func() time.Time { return laterAt }).Finalize(ctx, seller, f2.run, &funnelResult); err != nil {
		t.Fatal(err)
	}
	f2n := revise("FINANCE")
	financeResult.Days[0].StateHash, financeResult.Days[0].SourcePayloadID = financeHashV2, f2n.payload
	financeResult.Days[0].Records[0].TotalAmountSigned = "120"
	if err = finance.NewRepository(pool).Finalize(ctx, seller, f2n.run, &financeResult); err != nil {
		t.Fatal(err)
	}
	s2 := revise("SEARCH")
	searchResult.SKUSummaries[0].ProviderSearchGMV, searchResult.SKUSummaries[0].SourcePayloadID, searchResult.SKUSummaries[0].ObservedAt = "900", s2.payload, laterAt
	searchResult.Details[0].SourcePayloadID, searchResult.Details[0].ObservedAt = s2.payload, laterAt
	if err = search.NewRepository(pool).Finalize(ctx, seller, s2.run, &searchResult); err != nil {
		t.Fatal(err)
	}
	p2 := revise("PROMOTIONS")
	promotionResult.StartedAt, promotionResult.CompletedAt = laterAt, laterAt
	promotionResult.Definitions[0].ProviderState, promotionResult.Definitions[0].SourcePayloadID = "PAUSED", p2.payload
	promotionResult.Participants[0].SourcePayloadID = p2.payload
	promotionResult.Eligibility[0].SourcePayloadID = &p2.payload
	if err = promotions.NewRepository(pool).Finalize(ctx, seller, p2.run, &promotionResult); err != nil {
		t.Fatal(err)
	}
	h2 := revise("ACCOUNT_HEALTH")
	healthResult.StartedAt, healthResult.CompletedAt, healthResult.SummaryPayloadID = laterAt, laterAt, h2.payload
	healthResult.Metrics[0].Value, healthResult.Metrics[0].SourcePayloadID = "9", h2.payload
	if err = accounthealth.NewRepository(pool).Finalize(ctx, seller, h2.run, &healthResult); err != nil {
		t.Fatal(err)
	}
	su2 := revise("SUPPLY")
	supplyResult.StartedAt, supplyResult.CompletedAt = laterAt, laterAt
	supplyResult.Orders[0].ProviderState, supplyResult.Orders[0].CanonicalState, supplyResult.Orders[0].SourcePayloadID = "IN_TRANSIT", "IN_TRANSIT", su2.payload
	supplyResult.Supplies[0].ProviderState, supplyResult.Supplies[0].CanonicalState, supplyResult.Supplies[0].SourcePayloadID = "IN_TRANSIT", "IN_TRANSIT", su2.payload
	supplyResult.Bundles[0].SourcePayloadID = &su2.payload
	if err = supply.NewRepository(pool).Finalize(ctx, seller, su2.run, &supplyResult); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM supply_state_observations WHERE seller_account_id=$1`, seller).Scan(&stateObs); err != nil || stateObs != 4 {
		t.Fatalf("state change must append observations (READY→IN_TRANSIT): got %d err=%v", stateObs, err)
	}
	su3 := revise("SUPPLY")
	supplyResult.StartedAt, supplyResult.CompletedAt = laterAt.Add(time.Minute), laterAt.Add(time.Minute)
	supplyResult.Orders[0].SourcePayloadID = su3.payload
	supplyResult.Supplies[0].SourcePayloadID = su3.payload
	supplyResult.Bundles[0].SourcePayloadID = &su3.payload
	// same IN_TRANSIT state again — change-compressed, no new observations
	if err = supply.NewRepository(pool).Finalize(ctx, seller, su3.run, &supplyResult); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM supply_state_observations WHERE seller_account_id=$1`, seller).Scan(&stateObs); err != nil || stateObs != 4 {
		t.Fatalf("unchanged state must not insert observations: got %d err=%v", stateObs, err)
	}

	afterSameContext := prompt7ReadFacts(t, ctx, api, c1, day, sku)
	if before != afterSameContext {
		t.Fatalf("AS-KNOWN-AT context changed after later revisions\nbefore=%s\nafter=%s", before, afterSameContext)
	}
	c2 := prompt7Context(t, seller, day, laterAt.Add(time.Minute), laterRuns)
	afterNewContext := prompt7ReadFacts(t, ctx, api, c2, day, sku)
	if before == afterNewContext {
		t.Fatal("new context did not observe later Prompt7 revisions")
	}
	funnelV2, err := api.GetAccountFunnelPeriod(ctx, c2, "SEARCH_IMPRESSIONS", "", &factualapi.Period{From: day, To: day})
	if err != nil || funnelV2.Metric.Value.Value == nil || funnelV2.Metric.Value.Value.String() != "200" {
		t.Fatalf("new context funnel revision not visible: value=%+v err=%v", funnelV2.Metric.Value, err)
	}
	financeV2, err := api.GetFinanceAccountSummary(ctx, c2, "", &factualapi.Period{From: day, To: day})
	if err != nil || financeV2.AccountNet.Value == nil || strings.TrimRight(strings.TrimRight(financeV2.AccountNet.Value.Amount, "0"), ".") != "120" {
		t.Fatalf("new context finance parent revision not visible: value=%+v err=%v", financeV2.AccountNet, err)
	}
	searchV2, err := api.GetSKUSearchSummary(ctx, c2, sku, "", &factualapi.Period{From: day, To: day})
	if err != nil || searchV2.ProviderSearchGMV.Value == nil || searchV2.ProviderSearchGMV.Value.Amount.String() != "900" {
		t.Fatalf("new context search revision not visible: value=%+v err=%v", searchV2.ProviderSearchGMV, err)
	}
	promotionsV2, err := api.ListPromotions(ctx, c2, factualapi.PromotionFilter{}, factualapi.PageRequest{PageSize: 10})
	if err != nil || len(promotionsV2.Items) == 0 || promotionsV2.Items[0].ProviderState != "PAUSED" {
		t.Fatalf("new context promotion revision not visible: items=%+v err=%v", promotionsV2.Items, err)
	}
	healthV2, err := api.GetCurrentRatings(ctx, c2)
	if err != nil || len(healthV2) == 0 || healthV2[0].CurrentValue.Value == nil || healthV2[0].CurrentValue.Value.String() != "9" {
		t.Fatalf("new context health revision not visible: ratings=%+v err=%v", healthV2, err)
	}
	suppliesV2, err := api.ListSupplies(ctx, c2, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 10})
	if err != nil || len(suppliesV2.Items) == 0 || suppliesV2.Items[0].CanonicalState != "IN_TRANSIT" {
		t.Fatalf("new context supply revision not visible: supplies=%+v err=%v", suppliesV2.Items, err)
	}
}

func prompt7FactualService(t *testing.T, pool *pgxpool.Pool) *factual.Service {
	t.Helper()
	signer, err := factualapi.NewTokenSigner([]byte(strings.Repeat("p", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return factual.NewService(pool, signer, 50, 500)
}

func prompt7Context(t *testing.T, seller int64, day, cutoff time.Time, runs map[string]int64) factualapi.FactualReadContext {
	t.Helper()
	domainFamily := map[string]string{
		"FUNNEL": "FUNNEL.DAILY_ADDITIVE", "FINANCE": "FINANCE.DAILY_LEDGER", "SEARCH": "SEARCH.SELLER_SUMMARY",
		"PROMOTIONS": "PROMOTIONS.DEFINITIONS", "ACCOUNT_HEALTH": "CURRENT_SNAPSHOT", "SUPPLY": "SUPPLY.STATE",
	}
	var domains []factualapi.DomainBinding
	for domain, run := range runs {
		family := domainFamily[domain]
		if family == "" {
			t.Fatalf("prompt7: unknown domain %s", domain)
		}
		domains = append(domains, factualapi.DomainBinding{Domain: domain, FactFamily: family, DomainRunID: run, FinalizedAt: cutoff, ContractHash: "prompt7"})
	}
	var capabilities []factualapi.CapabilityBinding
	for i, key := range readiness.CapabilityKeys() {
		capabilities = append(capabilities, factualapi.CapabilityBinding{CapabilityKey: key, Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: int64(i + 1), AssessedAt: cutoff})
	}
	c, err := factualapi.NewReadContext(factualapi.ContextSpec{SellerAccountID: seller, AnalysisDate: day, CutoffAt: cutoff, Mode: factualapi.ModeLatestStable, Policies: factualapi.V1Policies(), Domains: domains, Capabilities: capabilities})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func prompt7ReadFacts(t *testing.T, ctx context.Context, api *factual.Service, c factualapi.FactualReadContext, day time.Time, sku int64) string {
	t.Helper()
	period := &factualapi.Period{From: day, To: day}
	funnelFact, err := api.GetAccountFunnelPeriod(ctx, c, "SEARCH_IMPRESSIONS", "", period)
	if err != nil {
		t.Fatal(err)
	}
	financeFact, err := api.GetFinanceAccountSummary(ctx, c, "", period)
	if err != nil {
		t.Fatal(err)
	}
	searchFacts, err := api.ListSKUSearchSummary(ctx, c, "", &factualapi.Period{From: day, To: day}, factualapi.PageRequest{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	promotionFacts, err := api.ListPromotions(ctx, c, factualapi.PromotionFilter{}, factualapi.PageRequest{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	healthFact, err := api.GetAccountHealthSummary(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	supplyFacts, err := api.ListSupplies(ctx, c, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	inboundFact, err := api.GetSKUInbound(ctx, c, sku)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]any{funnelFact, financeFact, searchFacts, promotionFacts.Items, healthFact, supplyFacts.Items, inboundFact})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
