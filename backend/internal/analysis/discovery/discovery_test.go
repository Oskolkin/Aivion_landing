package discovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func testDetail() landscape.LandscapeSet {
	catID := int64(11)
	return landscape.LandscapeSet{
		Context: landscape.LandscapeContext{
			AnalysisDate:            canonicaljson.Date(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)),
			AnalysisCutoffAt:        time.Date(2026, 8, 20, 2, 40, 0, 0, time.UTC),
			BusinessTimezone:        "Europe/Moscow",
			LandscapeProfileVersion: "detail_v5",
		},
		DomainWindows: []landscape.DomainWindow{
			{
				Domain: "sales",
				Current: &landscape.DateRange{
					From: canonicaljson.Date(time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)),
					To:   canonicaljson.Date(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)),
				},
				Previous: &landscape.DateRange{
					From: canonicaljson.Date(time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)),
					To:   canonicaljson.Date(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)),
				},
			},
			{Domain: "search", LagDays: int64ptr(2)},
		},
		GlobalCapabilities: []landscape.CapabilityFlag{
			{Key: "SALES.SKU_DEMAND", FinalState: "READY"},
			{Key: "PRICE.CUSTOMER_PRICE", FinalState: "READY"},
			{Key: "STOCK.SKU_CURRENT", FinalState: "READY"},
			{Key: "SEARCH.SELLER_SUMMARY", FinalState: "PARTIAL", ReasonCode: "EXPECTED_PROVIDER_LAG"},
		},
		Account: landscape.AccountLandscape{
			Identity: landscape.AccountIdentity{},
			Sales: &landscape.AccountSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current:     &landscape.MoneyAmount{Amount: "5000", CurrencyCode: "RUB"},
					Previous:    &landscape.MoneyAmount{Amount: "4000", CurrencyCode: "RUB"},
					ChangeState: landscape.ChangeIncreased,
				},
			},
		},
		Categories: []landscape.CategoryLandscape{{
			Identity: landscape.CategoryIdentity{CategoryID: 11, IsLeaf: true, Name: "Leaf"},
			Sales:    &landscape.CategorySalesBlock{State: landscape.BlockReady, OrderedAmount: landscape.MoneyComparison{Current: &landscape.MoneyAmount{Amount: "3000", CurrencyCode: "RUB"}, ChangeState: landscape.ChangeNoBase}},
		}},
		SKUs: []landscape.SKULandscape{{
			Identity: landscape.SKUIdentity{CanonicalSKUID: 417, CategoryID: &catID, Name: "SKU 417", OfferID: "offer-417"},
			Sales: &landscape.SKUSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current:     &landscape.MoneyAmount{Amount: "2000", CurrencyCode: "RUB"},
					Previous:    &landscape.MoneyAmount{Amount: "1800", CurrencyCode: "RUB"},
					ChangeState: landscape.ChangeIncreased,
				},
			},
			Price: &landscape.SKUPriceBlock{State: landscape.BlockReady, CurrentPrice: &landscape.MoneyAmount{Amount: "100", CurrencyCode: "RUB"}},
		}},
		Campaigns: []landscape.CampaignLandscape{{
			Identity:    landscape.CampaignIdentity{CampaignID: 9, Title: "Camp"},
			Performance: &landscape.CampaignPerfBlock{State: landscape.BlockReady, Traffic: &landscape.CampaignTrafficBlock{State: landscape.BlockReady, PaidSpend: landscape.MoneyComparison{Current: &landscape.MoneyAmount{Amount: "10", CurrencyCode: "RUB"}, ChangeState: landscape.ChangeNoBase}}},
		}},
	}
}

func int64ptr(v int64) *int64 { return &v }

func testSelection(detail landscape.LandscapeSet) materiality.SelectionArtifact {
	return materiality.SelectionArtifact{
		SchemaVersion: "materiality_selection_v1",
		SelectionHash: strings.Repeat("s", 64),
		Account:       materiality.SelectedEntity{EntityKind: materiality.EntityAccount, SemanticKey: "account", CanonicalRefID: "1"},
		Categories:    []materiality.SelectedEntity{{EntityKind: materiality.EntityCategory, CanonicalRefID: "11"}},
		SKUs:          []materiality.SelectedEntity{{EntityKind: materiality.EntitySKU, CanonicalRefID: "417"}},
		Campaigns:     []materiality.SelectedEntity{{EntityKind: materiality.EntityCampaign, CanonicalRefID: "9"}},
	}
}

func seedCache(detail landscape.LandscapeSet) GroundingCache {
	c := newGroundingCache()
	c.AccountDemandCur.Meta = factualapi.FactMeta{Readiness: "READY", FactRef: "ref-acct-cur"}
	c.AccountDemandPrev.Meta = factualapi.FactMeta{Readiness: "READY", FactRef: "ref-acct-prev"}
	c.CatDemandCur[11] = factualapi.CategoryDemand{Demand: factualapi.Demand{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-cat-cur"}}}
	c.CatDemandPrev[11] = factualapi.CategoryDemand{Demand: factualapi.Demand{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-cat-prev"}}}
	c.SKUDemandCur[417] = factualapi.SKUDemand{Demand: factualapi.Demand{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-sku-cur"}}}
	c.SKUDemandPrev[417] = factualapi.SKUDemand{Demand: factualapi.Demand{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-sku-prev"}}}
	c.SKUPrice[417] = factualapi.SKUPrice{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-price"}}
	c.CampTrafficCur[9] = factualapi.CampaignTrafficPerformance{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-camp-cur"}}
	c.CampTrafficPrev[9] = factualapi.CampaignTrafficPerformance{Meta: factualapi.FactMeta{Readiness: "READY", FactRef: "ref-camp-prev"}}
	return c
}

func TestPackDeterminism(t *testing.T) {
	detail := testDetail()
	sel := testSelection(detail)
	cache := seedCache(detail)
	in := BuildInput{SellerAccountID: 1, RunID: 1, UniverseID: "u1", UniverseHash: strings.Repeat("a", 64),
		LandscapeDetailHash: strings.Repeat("d", 64), MaterialitySelectionHash: strings.Repeat("s", 64),
		Detail: detail, Selection: sel, Cache: &cache, Model: "gpt-test"}
	a, err := BuildPack(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildPack(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Pack.PayloadHash != b.Pack.PayloadHash || a.HandleMap.Hash() != b.HandleMap.Hash() {
		t.Fatal("same inputs must produce same hashes")
	}
	if string(a.PayloadBytes) != string(b.PayloadBytes) {
		t.Fatal("payload bytes unstable")
	}
	in.UniverseHash = strings.Repeat("b", 64)
	c, err := BuildPack(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Pack.FactualUniverseHash == a.Pack.FactualUniverseHash && c.Pack.Envelope().FactualUniverseHash == a.Pack.Envelope().FactualUniverseHash {
		// universe hash is in envelope; payload may be equal
	}
	if a.Pack.FactualUniverseHash == c.Pack.FactualUniverseHash {
		t.Fatal("universe hash must change pack envelope")
	}
}

func TestFactHandlesUniqueOrderedGrounded(t *testing.T) {
	detail := testDetail()
	m := BuildFactHandleMap(detail, seedCache(detail))
	if len(m.Entries) == 0 {
		t.Fatal("expected handles")
	}
	seen := map[string]bool{}
	for i, e := range m.Entries {
		if e.Handle != formatHandle(i+1) {
			t.Fatalf("order %s want %s", e.Handle, formatHandle(i+1))
		}
		if seen[e.Handle] {
			t.Fatal("duplicate")
		}
		seen[e.Handle] = true
		if len(e.FactRefs) == 0 {
			t.Fatal("ungrounded " + e.Handle)
		}
	}
	m2 := BuildFactHandleMap(detail, seedCache(detail))
	if m.Hash() != m2.Hash() {
		t.Fatal("handle map hash unstable")
	}
}

func TestNoUsableHandleForUnavailable(t *testing.T) {
	cache := newGroundingCache()
	cache.SKUDemandCur[1] = factualapi.SKUDemand{Demand: factualapi.Demand{Meta: factualapi.FactMeta{Readiness: "UNAVAILABLE", FactRef: "x"}}}
	detail := landscape.LandscapeSet{SKUs: []landscape.SKULandscape{{
		Identity: landscape.SKUIdentity{CanonicalSKUID: 1},
		Sales:    &landscape.SKUSalesBlock{State: landscape.BlockUnavailable},
	}}}
	m := BuildFactHandleMap(detail, cache)
	for _, e := range m.Entries {
		if e.Entity.Kind == KindSKU && e.SemanticPath == "sales.ordered_amount_comparison" {
			t.Fatal("UNAVAILABLE must not get usable sales handle")
		}
	}
}

func TestSTALECannotBeSoleGrounding(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{
		"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true},
	}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{
		Handle: "f_0001", Entity: PackEntity{Kind: KindSKU, ID: "1"}, SemanticPath: "sales.ordered_amount_comparison",
		AnalyticalUsability: UsabilityStale, FactRefs: []string{"r1"},
	}}}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{{
		PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, RelatedEntities: []PackEntity{},
		Observation: "x", InvestigationFocus: "y", BasisFactHandles: []string{"f_0001"},
	}}}
	rep := Validate(ValidateInput{Output: out, Index: idx, Handles: handles, Menu: nil, Budget: CurrentOutputBudget()})
	if rep.Valid {
		t.Fatal("STALE-only must be invalid")
	}
}

func TestAllowedPartialGrounding(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{
		"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true},
	}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{
		Handle: "f_0001", Entity: PackEntity{Kind: KindSKU, ID: "1"}, SemanticPath: "search.provider_average_position",
		AnalyticalUsability: UsabilityPartial, FactRefs: []string{"r1"},
	}}}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{{
		PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, RelatedEntities: []PackEntity{},
		Observation: "search lag", InvestigationFocus: "clarify lag", BasisFactHandles: []string{"f_0001"},
	}}}
	rep := Validate(ValidateInput{Output: out, Index: idx, Handles: handles, Menu: nil, Budget: CurrentOutputBudget()})
	if !rep.Valid {
		t.Fatalf("allowed PARTIAL should work: %v", rep.Errors)
	}
}

func TestPayloadNoMaterialityLeakage(t *testing.T) {
	detail := testDetail()
	sel := testSelection(detail)
	cache := seedCache(detail)
	built, err := BuildPack(context.Background(), BuildInput{
		SellerAccountID: 99, RunID: 7, UniverseID: "univ-1", UniverseHash: strings.Repeat("a", 64),
		LandscapeDetailHash: strings.Repeat("d", 64), MaterialitySelectionHash: strings.Repeat("s", 64),
		Detail: detail, Selection: sel, Cache: &cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.ToLower(string(built.PayloadBytes))
	for _, banned := range []string{"materiality_score", "fusion_utility", "lane_rank", "reason_codes", "selected_as_anchor", "selection_reason", "seller_account_id", "universe_id", "run_id", "authorization", "api_key"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("payload leaked %q", banned)
		}
	}
}

func TestEntityValidation(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{
		"ACCOUNT:account": {Kind: KindAccount, AIID: "account", Present: true, MayBePrimary: true},
		"CATEGORY:11":     {Kind: KindCategory, AIID: "11", Present: true, MayBePrimary: true, Leaf: true},
		"CATEGORY:0":      {Kind: KindCategory, AIID: "0", Present: true, MayBePrimary: false, Unmapped: true, Leaf: true},
		"CATEGORY:2":      {Kind: KindCategory, AIID: "2", Present: true, MayBePrimary: false, Leaf: false},
		"SKU:417":         {Kind: KindSKU, AIID: "417", Present: true, MayBePrimary: true},
		"SKU:support":     {Kind: KindSKU, AIID: "support", Present: true, MayBePrimary: false, MayBeSupporting: true},
	}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", FactRefs: []string{"r"}, AnalyticalUsability: UsabilityReady}}}
	hmap := handles.ByHandle()
	_ = hmap
	base := ThreadV1{RelatedEntities: []PackEntity{}, Observation: "o", InvestigationFocus: "f", BasisFactHandles: []string{"f_0001"}}
	ok := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{base}}
	ok.Threads[0].PrimaryEntity = PackEntity{Kind: KindAccount, ID: "account"}
	if !Validate(ValidateInput{Output: ok, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("account primary")
	}
	ok.Threads[0].PrimaryEntity = PackEntity{Kind: KindCategory, ID: "0"}
	if Validate(ValidateInput{Output: ok, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("UNMAPPED category")
	}
	ok.Threads[0].PrimaryEntity = PackEntity{Kind: KindCategory, ID: "2"}
	if Validate(ValidateInput{Output: ok, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("non-leaf")
	}
	ok.Threads[0].PrimaryEntity = PackEntity{Kind: KindSKU, ID: "999"}
	if Validate(ValidateInput{Output: ok, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("unknown sku")
	}
	ok.Threads[0].PrimaryEntity = PackEntity{Kind: KindSKU, ID: "support"}
	if Validate(ValidateInput{Output: ok, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("supporting-only primary")
	}
}

func TestEvidenceCatalogClosedAndMenu(t *testing.T) {
	keys := ExpectedCatalogKeysV1()
	if len(keys) != 50 {
		t.Fatalf("catalog want 50 got %d", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			t.Fatal("dup " + k)
		}
		seen[k] = true
	}
	caps := map[string]string{"SALES.SKU_DEMAND": "READY"}
	kinds := map[EntityKind]bool{KindSKU: true, KindAccount: true}
	menu := BuildEvidenceMenu(caps, kinds, map[string]bool{"sales": true})
	for _, it := range menu {
		if it.Key == "SKU_PRICE_HISTORY" {
			t.Fatal("price evidence must be absent without price capability/domain")
		}
	}
	m1 := BuildEvidenceMenu(caps, kinds, map[string]bool{"sales": true})
	m2 := BuildEvidenceMenu(caps, kinds, map[string]bool{"sales": true})
	if MenuHash(m1) != MenuHash(m2) {
		t.Fatal("menu hash unstable")
	}
	for _, tpe := range CatalogV1() {
		if !tpe.EnabledForDiscoveryV1 {
			for _, it := range m1 {
				if it.Key == tpe.Key {
					t.Fatalf("disabled %s leaked", tpe.Key)
				}
			}
		}
	}
}

func TestOutputSchemaContract(t *testing.T) {
	raw, _ := json.Marshal(OutputJSONSchema())
	s := string(raw)
	if !strings.Contains(s, `"additionalProperties":false`) {
		t.Fatal("strict additionalProperties")
	}
	if strings.Contains(s, "recommendation") || strings.Contains(s, "root_cause") || strings.Contains(s, "action_type") {
		t.Fatal("forbidden fields in schema")
	}
	if !strings.Contains(s, "THREADS_FOUND") || !strings.Contains(s, "NO_MEANINGFUL_THREADS") {
		t.Fatal("states")
	}
	if strings.Contains(s, `"from"`) && strings.Contains(s, "evidence") {
		// evidence request must not have query fields
	}
	schema := OutputJSONSchema()
	props := schema["properties"].(map[string]any)
	threads := props["threads"].(map[string]any)
	items := threads["items"].(map[string]any)
	tprops := items["properties"].(map[string]any)
	er := tprops["evidence_requests"].(map[string]any)
	erItems := er["items"].(map[string]any)
	erProps := erItems["properties"].(map[string]any)
	for _, banned := range []string{"from", "to", "limit", "sql", "group_by"} {
		if _, ok := erProps[banned]; ok {
			t.Fatalf("evidence request has %s", banned)
		}
	}
}

func TestNoMeaningfulThreads(t *testing.T) {
	out := OutputV1{DiscoveryState: StateNoMeaningfulThreads, Threads: []ThreadV1{}}
	rep := Validate(ValidateInput{Output: out, Index: PackEntityIndex{ByKey: map[string]PackEntityRecord{}}, Handles: FactHandleMapV1{}, Budget: CurrentOutputBudget()})
	if !rep.Valid {
		t.Fatal(rep.Errors)
	}
	v := AssignThreadIDs(out)
	if len(v.Threads) != 0 {
		t.Fatal("empty validated")
	}
}

func TestThreadIDsDeterministic(t *testing.T) {
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{
		{PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, Observation: "a", InvestigationFocus: "b", BasisFactHandles: []string{"f_0001"}},
		{PrimaryEntity: PackEntity{Kind: KindSKU, ID: "2"}, Observation: "c", InvestigationFocus: "d", BasisFactHandles: []string{"f_0001"}},
	}}
	v1 := AssignThreadIDs(out)
	v2 := AssignThreadIDs(out)
	if v1.Threads[0].ThreadID != "T001" || v1.Threads[1].ThreadID != "T002" {
		t.Fatalf("%s %s", v1.Threads[0].ThreadID, v1.Threads[1].ThreadID)
	}
	b1, _ := canonicaljson.Marshal(v1)
	b2, _ := canonicaljson.Marshal(v2)
	if string(b1) != string(b2) {
		t.Fatal("validated hash unstable")
	}
	if v1.Threads[0].Observation != "a" {
		t.Fatal("semantic rewrite")
	}
}

func TestStage8Stage9SameValidator(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true}}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", AnalyticalUsability: UsabilityReady, FactRefs: []string{"r"}}}}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{{
		PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, RelatedEntities: []PackEntity{},
		Observation: "o", InvestigationFocus: "f", BasisFactHandles: []string{"f_0001"},
	}}}
	vin := ValidateInput{Output: out, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}
	pre := Validate(vin)
	_, auth := ValidateAuthoritative(vin)
	if !pre.Valid || !auth.Valid {
		t.Fatalf("pre=%v auth=%v", pre.Errors, auth.Errors)
	}
}

func TestAttemptPolicySameRequest(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true}}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", AnalyticalUsability: UsabilityReady, FactRefs: []string{"r"}}}}
	vin := ValidateInput{Index: idx, Handles: handles, Budget: CurrentOutputBudget()}
	valid := `{"discovery_state":"THREADS_FOUND","threads":[{"primary_entity":{"kind":"SKU","id":"1"},"related_entities":[],"observation":"o","investigation_focus":"f","basis_fact_handles":["f_0001"],"evidence_requests":[]}]}`
	invalid := `{"discovery_state":"THREADS_FOUND","threads":[{"primary_entity":{"kind":"SKU","id":"1"},"related_entities":[],"observation":"o","investigation_focus":"f","basis_fact_handles":["f_9999"],"evidence_requests":[]}]}`
	fp := &FakeProvider{Responses: []Response{
		{Status: "completed", OutputText: invalid},
		{Status: "completed", OutputText: valid},
	}}
	req := Request{Model: "m", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{}`), Schema: OutputJSONSchema(), MaxOutputTokens: 8192}
	attempts, _, err := RunAttempts(context.Background(), fp, req, vin, 2)
	if err != nil || fp.Calls != 2 {
		t.Fatalf("err=%v calls=%d", err, fp.Calls)
	}
	if attempts[0].RequestHash != attempts[1].RequestHash {
		t.Fatal("request hash must match on rerun")
	}
	fp2 := &FakeProvider{Responses: []Response{{Status: "completed", OutputText: invalid}, {Status: "completed", OutputText: invalid}}}
	_, _, err = RunAttempts(context.Background(), fp2, req, vin, 2)
	if err == nil || fp2.Calls != 2 {
		t.Fatalf("two invalid should fail after 2: err=%v calls=%d", err, fp2.Calls)
	}
}

func TestRefusalAndIncomplete(t *testing.T) {
	vin := ValidateInput{Budget: CurrentOutputBudget()}
	out, class, _ := ClassifyResponse(Response{Status: "completed", Refusal: "no"}, nil, vin)
	if class != AttemptRefused || out.DiscoveryState != "" {
		t.Fatalf("refusal class=%s", class)
	}
	_, class, _ = ClassifyResponse(Response{Status: "incomplete", IncompleteReason: "max_output_tokens", OutputText: `{"discovery_state":`}, nil, vin)
	if class != AttemptIncomplete {
		t.Fatalf("incomplete class=%s", class)
	}
}

func TestImportBoundary(t *testing.T) {
	root := "."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := string(b)
		for _, banned := range []string{"internal/unifiedanalysis", "internal/aipipeline", "internal/recommendations", "integrations/ozon", "jackc/pgx", "internal/dbgen"} {
			if strings.Contains(s, banned) {
				t.Errorf("%s imports %s", path, banned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPromptFixturesExist(t *testing.T) {
	dir := filepath.Join("testdata", "eval")
	files, err := os.ReadDir(dir)
	if err != nil || len(files) < 8 {
		t.Fatalf("eval fixtures: %v count=%d", err, len(files))
	}
}

func TestPackBudgetExceededBeforeProvider(t *testing.T) {
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", "8")
	detail := testDetail()
	sel := testSelection(detail)
	cache := seedCache(detail)
	_, err := BuildPack(context.Background(), BuildInput{
		SellerAccountID: 1, RunID: 1, UniverseID: "u1", UniverseHash: strings.Repeat("a", 64),
		LandscapeDetailHash: strings.Repeat("d", 64), MaterialitySelectionHash: strings.Repeat("s", 64),
		Detail: detail, Selection: sel, Cache: &cache, Model: "gpt-test",
	})
	if err == nil {
		t.Fatal("expected DISCOVERY_PACK_BUDGET_EXCEEDED")
	}
	if !strings.Contains(err.Error(), CodePackBudgetExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestOutputBudgetMaxThreads(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{"SKU:1": {Kind: KindSKU, AIID: "1", Present: true, MayBePrimary: true}}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{Handle: "f_0001", AnalyticalUsability: UsabilityReady, FactRefs: []string{"r"}}}}
	threads := make([]ThreadV1, 11)
	for i := range threads {
		threads[i] = ThreadV1{PrimaryEntity: PackEntity{Kind: KindSKU, ID: "1"}, RelatedEntities: []PackEntity{}, Observation: "o", InvestigationFocus: "f", BasisFactHandles: []string{"f_0001"}}
	}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: threads}
	rep := Validate(ValidateInput{Output: out, Index: idx, Handles: handles, Budget: CurrentOutputBudget()})
	if rep.Valid {
		t.Fatal("max_threads must reject")
	}
}

func TestFinancePartialCannotBeSoleGrounding(t *testing.T) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{"ACCOUNT:account": {Kind: KindAccount, AIID: "account", Present: true, MayBePrimary: true}}}
	handles := FactHandleMapV1{Entries: []HandleEntry{{
		Handle: "f_0001", Entity: PackEntity{Kind: KindAccount, ID: "account"}, SemanticPath: "finance.net_accrual_comparison",
		AnalyticalUsability: UsabilityPartial, FactRefs: []string{"r1"},
	}}}
	out := OutputV1{DiscoveryState: StateThreadsFound, Threads: []ThreadV1{{
		PrimaryEntity: PackEntity{Kind: KindAccount, ID: "account"}, RelatedEntities: []PackEntity{},
		Observation: "fees", InvestigationFocus: "clarify", BasisFactHandles: []string{"f_0001"},
	}}}
	if Validate(ValidateInput{Output: out, Index: idx, Handles: handles, Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("unsafe PARTIAL finance must not be sole grounding")
	}
}
