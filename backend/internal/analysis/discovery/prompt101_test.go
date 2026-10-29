package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestVisibleFactualObjectsHaveHandles(t *testing.T) {
	detail := testDetail()
	cache := seedCache(detail)
	built, err := BuildPack(context.Background(), BuildInput{
		SellerAccountID: 1, RunID: 1, UniverseID: "u1", UniverseHash: strings.Repeat("a", 64),
		LandscapeDetailHash: strings.Repeat("d", 64), MaterialitySelectionHash: strings.Repeat("s", 64),
		Detail: detail, Selection: testSelection(detail), Cache: &cache, Model: "gpt-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	visible := CollectVisibleFactHandles(built.Pack.Payload)
	if len(visible) == 0 {
		t.Fatal("expected visible factual objects")
	}
	t.Logf("visible_factual_objects=%d fact_handles=%d pack_bytes=%d estimated_tokens=%d",
		len(visible), len(built.HandleMap.Entries), len(built.PackBytes), built.Pack.EstimatedInputTokens)
	envelope, payload, err := UnmarshalPackWire(built.PackBytes)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.FactHandleMapHash != built.Pack.FactHandleMapHash || len(payload.EvidenceMenu) == 0 {
		t.Fatalf("wire envelope hash=%q pack=%q menu=%d", envelope.FactHandleMapHash, built.Pack.FactHandleMapHash, len(payload.EvidenceMenu))
	}
	if acc := built.Pack.Payload.Account.Sales; acc != nil && acc.OrderedAmount != nil {
		b, _ := json.Marshal(acc.OrderedAmount)
		t.Logf("payload_account_ordered_amount=%s", b)
	}
	if len(built.HandleMap.Entries) > 0 {
		t.Logf("handle_map_first=%+v", built.HandleMap.Entries[0])
	}
	mapped := built.HandleMap.ByHandle()
	if len(visible) != len(built.HandleMap.Entries) {
		t.Fatalf("visible=%d handles=%d", len(visible), len(built.HandleMap.Entries))
	}
	for _, v := range visible {
		if v.Handle == "" {
			t.Fatal("empty fact_handle")
		}
		if _, ok := mapped[v.Handle]; !ok {
			t.Fatalf("visible %s missing from map", v.Handle)
		}
	}
	raw, _ := json.Marshal(built.Pack.Payload)
	if strings.Contains(strings.ToLower(string(raw)), `"ctr"`) || strings.Contains(string(raw), `"impressions"`) && !strings.Contains(string(raw), "fact_handle") {
		// CTR must never appear; impressions only inside grounded funnel object
	}
	if strings.Contains(string(raw), `"ctr"`) || strings.Contains(string(raw), `"cpc"`) || strings.Contains(string(raw), `"cpm"`) {
		t.Fatal("ungrounded ads efficiency metrics must not be AI-visible")
	}
	acc := built.Pack.Payload.Account.Sales
	if acc == nil || acc.OrderedAmount == nil || acc.OrderedAmount.FactHandle == "" {
		t.Fatal("account ordered_amount must carry visible fact_handle")
	}
	if _, ok := mapped[acc.OrderedAmount.FactHandle]; !ok {
		t.Fatal("account handle missing from map")
	}
}

func TestComparisonUsabilityMatrix(t *testing.T) {
	ready := factualapi.FactMeta{Readiness: "READY", FactRef: "c"}
	readyP := factualapi.FactMeta{Readiness: "READY", FactRef: "p"}
	partial := factualapi.FactMeta{Readiness: "PARTIAL", FactRef: "c", Limitations: []factualapi.Limitation{{Code: "EXPECTED_PROVIDER_LAG"}}}
	stale := factualapi.FactMeta{Readiness: "STALE", FactRef: "p"}
	failed := factualapi.FactMeta{Readiness: "FAILED", FactRef: "p"}
	empty := factualapi.FactMeta{Readiness: "", FactRef: "p"}

	u, ok := comparisonUsability(ready, readyP)
	if !ok || u != UsabilityReady {
		t.Fatalf("READY+READY got %s ok=%v", u, ok)
	}
	u, ok = comparisonUsability(ready, partial)
	if !ok || u != UsabilityPartial {
		t.Fatalf("READY+PARTIAL got %s ok=%v", u, ok)
	}
	u, ok = comparisonUsability(ready, stale)
	if !ok || u != UsabilityStale {
		t.Fatalf("READY+STALE got %s ok=%v", u, ok)
	}
	if _, ok = comparisonUsability(ready, failed); ok {
		t.Fatal("READY+FAILED must be unusable")
	}
	if _, ok = comparisonUsability(ready, empty); ok {
		t.Fatal("READY+empty must be unusable")
	}
	if _, ok = requiredSideUsability(factualapi.FactMeta{Readiness: ""}); ok {
		t.Fatal("empty readiness is not READY")
	}

	detail := landscape.LandscapeSet{SKUs: []landscape.SKULandscape{{
		Identity: landscape.SKUIdentity{CanonicalSKUID: 1},
		Sales: &landscape.SKUSalesBlock{State: landscape.BlockReady, OrderedAmount: landscape.MoneyComparison{
			Current: &landscape.MoneyAmount{Amount: "1", CurrencyCode: "RUB"}, Previous: &landscape.MoneyAmount{Amount: "2", CurrencyCode: "RUB"},
		}},
	}}}
	cache := newGroundingCache()
	cache.SKUDemandCur[1] = factualapi.SKUDemand{Demand: factualapi.Demand{Meta: ready}}
	cache.SKUDemandPrev[1] = factualapi.SKUDemand{Demand: factualapi.Demand{Meta: failed}}
	m := BuildFactHandleMap(detail, cache)
	for _, e := range m.Entries {
		if e.SemanticPath == "sales.ordered_amount_comparison" {
			t.Fatal("READY+FAILED must not emit comparison handle")
		}
	}
}

func TestPriceChangeIsSingleFact(t *testing.T) {
	detail := landscape.LandscapeSet{SKUs: []landscape.SKULandscape{{
		Identity: landscape.SKUIdentity{CanonicalSKUID: 7},
		Price: &landscape.SKUPriceBlock{
			State: landscape.BlockReady,
			LastObservedChange: &landscape.MoneyComparison{
				Current: &landscape.MoneyAmount{Amount: "10", CurrencyCode: "RUB"},
			},
		},
	}}}
	cache := newGroundingCache()
	cache.SKUPriceChange[7] = factualapi.SKUPriceLastObservedChange{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "price-change"}}
	m := BuildFactHandleMap(detail, cache)
	found := false
	for _, e := range m.Entries {
		if e.SemanticPath == "price.last_observed_price_change" {
			found = true
			if e.Derivation != nil {
				t.Fatal("price last change must be a single fact, not a derived comparison")
			}
			if len(e.FactRefs) != 1 || e.FactRefs[0] != "price-change" {
				t.Fatalf("refs=%v", e.FactRefs)
			}
		}
	}
	if !found {
		t.Fatal("expected single-fact price change handle")
	}
}

func TestGroundingAPIFailureFailsPack(t *testing.T) {
	detail := testDetail()
	spy := &failingGroundingAPI{fail: "BatchGetSKUDemand"}
	_, err := BuildPack(context.Background(), BuildInput{
		SellerAccountID: 1, RunID: 1, UniverseID: "u1", UniverseHash: strings.Repeat("a", 64),
		LandscapeDetailHash: strings.Repeat("d", 64), MaterialitySelectionHash: strings.Repeat("s", 64),
		Detail: detail, Selection: testSelection(detail), API: nil, Cache: nil, Model: "gpt-test",
	})
	if err == nil {
		t.Fatal("nil API with factual objects must fail")
	}
	_, err = LoadGrounding(context.Background(), spy, factualapi.FactualReadContext{}, detail)
	if err == nil {
		t.Fatal("required grounding read must fail closed")
	}
	if !strings.Contains(err.Error(), "ground SKU demand") {
		t.Fatalf("err=%v", err)
	}
}

func TestNilAPINoFactsOK(t *testing.T) {
	detail := landscape.LandscapeSet{
		Context: landscape.LandscapeContext{AnalysisDate: canonicaljson.Date(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC))},
		Account: landscape.AccountLandscape{Sales: &landscape.AccountSalesBlock{State: landscape.BlockUnavailable}},
	}
	_, err := LoadGrounding(context.Background(), nil, factualapi.FactualReadContext{}, detail)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceMenuUsesSealedDomainsNotWindows(t *testing.T) {
	caps := map[string]string{"PRICE.CUSTOMER_PRICE": "READY", "SALES.SKU_DEMAND": "READY"}
	kinds := map[EntityKind]bool{KindSKU: true, KindAccount: true}
	windowsOnly := map[string]bool{"sales": true}
	menuWindows := BuildEvidenceMenu(caps, kinds, windowsOnly)
	for _, it := range menuWindows {
		if it.Key == "SKU_PRICE_HISTORY" {
			t.Fatal("price evidence must be absent without price domain")
		}
	}
	sealed := map[string]bool{"sales": true, "price": true}
	menuSealed := BuildEvidenceMenu(caps, kinds, sealed)
	found := false
	for _, it := range menuSealed {
		if it.Key == "SKU_PRICE_HISTORY" {
			found = true
		}
	}
	if !found {
		t.Fatal("price domain in sealed universe must allow SKU_PRICE_HISTORY even without DomainWindow")
	}
}

func TestRelatedEntityRejectedWhenNotSupported(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{
		"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true},
	}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", AnalyticalUsability: UsabilityReady, FactRefs: []string{"r"}}}}
	menu := []EvidenceMenuItem{{Key: "SKU_SALES_HISTORY", SubjectKinds: []EntityKind{KindSKU}, RelatedKinds: []EntityKind{}}}
	rel := PackEntity{Kind: KindSKU, ID: "1"}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{{
		PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, RelatedEntities: []PackEntity{},
		Observation: "o", InvestigationFocus: "f", BasisFactHandles: []string{"f_0001"},
		EvidenceRequests: []EvidenceRequest{{
			EvidenceType: "SKU_SALES_HISTORY", Subject: PackEntity{Kind: KindSKU, ID: "1"},
			RelatedEntity: &rel, InvestigationQuestion: "q",
		}},
	}}}
	rep := Validate(ValidateInput{Output: out, Index: idx, Handles: handles, Menu: menu, Budget: CurrentOutputBudget()})
	if rep.Valid {
		t.Fatal("related_entity must be INVALID when RelatedKinds is empty")
	}
}

func TestAttemptPersistenceOrderAndFailure(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true}}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", AnalyticalUsability: UsabilityReady, FactRefs: []string{"r"}}}}
	vin := ValidateInput{Index: idx, Handles: handles, Budget: CurrentOutputBudget()}
	invalid := `{"discovery_state":"THREADS_FOUND","threads":[{"primary_entity":{"kind":"SKU","id":"1"},"related_entities":[],"observation":"o","investigation_focus":"f","basis_fact_handles":["f_9999"],"evidence_requests":[]}]}`
	valid := `{"discovery_state":"THREADS_FOUND","threads":[{"primary_entity":{"kind":"SKU","id":"1"},"related_entities":[],"observation":"o","investigation_focus":"f","basis_fact_handles":["f_0001"],"evidence_requests":[]}]}`
	fp := &FakeProvider{Responses: []Response{{Status: "completed", OutputText: invalid}, {Status: "completed", OutputText: valid}}}
	req := Request{Model: "m", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{}`), Schema: OutputJSONSchema(), MaxOutputTokens: 8192}
	var persisted []int
	_, _, err := RunAttempts(context.Background(), fp, req, vin, 2, func(_ context.Context, at ModelAttempt) error {
		persisted = append(persisted, at.No)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 || persisted[0] != 1 || persisted[1] != 2 {
		t.Fatalf("persist order %v", persisted)
	}
	if fp.Calls != 2 {
		t.Fatalf("calls=%d", fp.Calls)
	}

	fp2 := &FakeProvider{Responses: []Response{{Status: "completed", OutputText: invalid}, {Status: "completed", OutputText: valid}}}
	_, _, err = RunAttempts(context.Background(), fp2, req, vin, 2, func(_ context.Context, at ModelAttempt) error {
		if at.No == 1 {
			return errors.New("persist boom")
		}
		return nil
	})
	if err == nil || fp2.Calls != 1 {
		t.Fatalf("persist failure must stop before second call: err=%v calls=%d", err, fp2.Calls)
	}
}

func TestPayloadWindowsSerializePrevious(t *testing.T) {
	p := ProjectPayload(testDetail(), nil, FactHandleMapV1{})
	if len(p.Windows) == 0 || p.Windows[0].Current == nil || p.Windows[0].Previous == nil {
		t.Fatalf("expected current+previous windows: %+v", p.Windows)
	}
	if p.Windows[0].Previous.From != "2026-08-06" || p.Windows[0].Previous.To != "2026-08-12" {
		t.Fatalf("previous=%+v", p.Windows[0].Previous)
	}
}

func TestRequestFingerprintIncludesWireContract(t *testing.T) {
	req := Request{Model: "gpt-test", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{"x":1}`), Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV1, MaxOutputTokens: 8192}
	a := HashRequest(req)
	b := HashRequest(req)
	if a == "" || a != b {
		t.Fatal("request hash unstable")
	}
	req.SchemaName = "other"
	if HashRequest(req) == a {
		t.Fatal("schema name must change request hash")
	}
	wire := BuildDiscoveryResponsesRequest(req)
	if !wire.Text.Format.Strict || wire.Store || wire.Truncation != "disabled" || wire.PreviousResponseID != nil || wire.Tools == nil {
		t.Fatalf("wire contract %+v", wire)
	}
	est, err := EstimateDiscoveryRequestTokens(req)
	if err != nil || est <= 0 {
		t.Fatalf("estimate %d %v", est, err)
	}
}

func TestGroundingQueryCountIsBatched(t *testing.T) {
	detail := testDetail()
	spy := &countingGroundingAPI{}
	_, err := LoadGrounding(context.Background(), spy, factualapi.FactualReadContext{}, detail)
	if err != nil {
		t.Fatal(err)
	}
	if spy.calls == 0 {
		t.Fatal("expected grounding calls")
	}
	if spy.calls > 20 {
		t.Fatalf("too many grounding calls %d (N+1?)", spy.calls)
	}
	t.Logf("stage7_grounding_api_calls=%d", spy.calls)
}

type failingGroundingAPI struct {
	countingGroundingAPI
	fail string
}

func (f *failingGroundingAPI) BatchGetSKUDemand(ctx context.Context, c factualapi.FactualReadContext, ids []int64, p factualapi.StandardPeriod, per *factualapi.Period) (factualapi.BatchSKUDemandResult, error) {
	f.calls++
	if f.fail == "BatchGetSKUDemand" {
		return factualapi.BatchSKUDemandResult{}, errors.New("forced")
	}
	return factualapi.BatchSKUDemandResult{}, nil
}

type countingGroundingAPI struct{ calls int }

func (c *countingGroundingAPI) inc() { c.calls++ }

func (c *countingGroundingAPI) GetAccountDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.Demand, error) {
	c.inc()
	return factualapi.Demand{}, nil
}
func (c *countingGroundingAPI) GetFinanceAccountSummary(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.FinanceAccountSummary, error) {
	c.inc()
	return factualapi.FinanceAccountSummary{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUDemand(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUDemandResult, error) {
	c.inc()
	return factualapi.BatchSKUDemandResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUStock(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUStockResult, error) {
	c.inc()
	return factualapi.BatchSKUStockResult{}, nil
}
func (c *countingGroundingAPI) BatchGetStockCoverage(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchStockCoverageResult, error) {
	c.inc()
	return factualapi.BatchStockCoverageResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUPrice(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUPriceResult, error) {
	c.inc()
	return factualapi.BatchSKUPriceResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUPriceLastObservedChange(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUPriceLastObservedChangeResult, error) {
	c.inc()
	return factualapi.BatchSKUPriceLastObservedChangeResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUInbound(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUInboundResult, error) {
	c.inc()
	return factualapi.BatchSKUInboundResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUAdvertisingTraffic(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingTrafficResult, error) {
	c.inc()
	return factualapi.BatchSKUAdvertisingTrafficResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUAdvertisingAttribution(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingAttributionResult, error) {
	c.inc()
	return factualapi.BatchSKUAdvertisingAttributionResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUFunnelPeriod(context.Context, factualapi.FactualReadContext, []int64, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUFunnelPeriodResult, error) {
	c.inc()
	return factualapi.BatchSKUFunnelPeriodResult{}, nil
}
func (c *countingGroundingAPI) BatchGetSKUSearchSummary(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUSearchSummaryResult, error) {
	c.inc()
	return factualapi.BatchSKUSearchSummaryResult{}, nil
}
func (c *countingGroundingAPI) BatchGetCampaignTrafficPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchCampaignTrafficResult, error) {
	c.inc()
	return factualapi.BatchCampaignTrafficResult{}, nil
}
func (c *countingGroundingAPI) BatchGetCampaignAttributionPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchCampaignAttributionResult, error) {
	c.inc()
	return factualapi.BatchCampaignAttributionResult{}, nil
}
func (c *countingGroundingAPI) ListCategoryDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error) {
	c.inc()
	return factualapi.Page[factualapi.CategoryDemand]{}, nil
}
