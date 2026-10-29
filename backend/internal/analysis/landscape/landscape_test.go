package landscape

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestBuildMoneyComparison_FromZero(t *testing.T) {
	cur := &MoneyAmount{Amount: "100", CurrencyCode: "RUB"}
	prev := &MoneyAmount{Amount: "0", CurrencyCode: "RUB"}
	m := BuildMoneyComparison(cur, prev)
	if m.ChangeState != ChangeFromZero {
		t.Fatalf("change_state=%s", m.ChangeState)
	}
	if m.PctDelta != nil {
		t.Fatalf("FROM_ZERO must have nil pct_delta, got %v", *m.PctDelta)
	}
	if m.AbsoluteDelta == nil || m.AbsoluteDelta.Amount != "100" {
		t.Fatalf("absolute_delta=%v", m.AbsoluteDelta)
	}
}

func TestBuildMoneyComparison_ToZero(t *testing.T) {
	cur := &MoneyAmount{Amount: "0", CurrencyCode: "RUB"}
	prev := &MoneyAmount{Amount: "50", CurrencyCode: "RUB"}
	m := BuildMoneyComparison(cur, prev)
	if m.ChangeState != ChangeToZero {
		t.Fatalf("change_state=%s", m.ChangeState)
	}
	if m.PctDelta == nil || *m.PctDelta != "-100" {
		t.Fatalf("pct_delta=%v", m.PctDelta)
	}
}

func TestBuildMoneyComparison_MissingNotZero(t *testing.T) {
	cur := &MoneyAmount{Amount: "10", CurrencyCode: "RUB"}
	m := BuildMoneyComparison(cur, nil)
	if m.ChangeState != ChangeNoBase {
		t.Fatalf("missing previous must be NO_BASE, got %s", m.ChangeState)
	}
	m2 := BuildMoneyComparison(nil, cur)
	if m2.ChangeState != ChangeNoCurrent {
		t.Fatalf("missing current must be NO_CURRENT, got %s", m2.ChangeState)
	}
}

func TestBuildPctPointComparison_DeltaPP(t *testing.T) {
	cur := canonicaljson.Decimal("4.1")
	prev := canonicaljson.Decimal("3.2")
	p := BuildPctPointComparison(&cur, &prev)
	if p.DeltaPP == nil || string(*p.DeltaPP) != "0.9" {
		t.Fatalf("delta_pp=%v", p.DeltaPP)
	}
	if p.ChangeState != ChangeIncreased {
		t.Fatalf("change_state=%s", p.ChangeState)
	}
}

func TestTruncationMetadata(t *testing.T) {
	items := []CampaignContextItem{{CampaignID: 1}, {CampaignID: 2}, {CampaignID: 3}, {CampaignID: 4}}
	tl := ApplyTruncation(items, 17, 3, "PAID_SPEND_DESC", "TOP_N_BY_PAID_SPEND")
	if !tl.Truncated || tl.Returned != 3 || tl.TotalKnown != 17 || len(tl.Items) != 3 {
		t.Fatalf("%+v", tl)
	}
	if tl.SortBasis == "" || tl.SemanticScope == "" {
		t.Fatal("sort_basis/semantic_scope required")
	}
}

func TestForbiddenVocabularyRejectsSemanticField(t *testing.T) {
	type bad struct {
		StockProblem string `json:"stock_problem"`
	}
	if err := scanForbiddenStruct(reflect.ValueOf(bad{StockProblem: "x"})); err == nil {
		t.Fatal("expected forbidden vocabulary error")
	}
}
func TestForbiddenVocabularyAllowsProviderCritical(t *testing.T) {
	set := minimalSet(1)
	crit := int64(2)
	set.Account.Health = &AccountHealthBlock{
		State: BlockReady, ProviderCriticalCount: &crit, ProviderBadChangesCount: &crit, ProviderGoodChangesCount: &crit,
	}
	if err := ValidateLandscapeCore(set, 1); err != nil {
		t.Fatal(err)
	}
}

func TestDeterminismHashStable(t *testing.T) {
	ctx, sealed := sealedCtx(t, 42)
	feed := CoreFeed{
		GlobalCapabilities: []CapabilityFlag{
			{Key: "SALES.ACCOUNT_DEMAND", FinalState: "READY"},
			{Key: "SALES.SKU_DEMAND", FinalState: "READY"},
			{Key: "STOCK.SKU_CURRENT", FinalState: "READY"},
			{Key: "PRICE.CUSTOMER_PRICE", FinalState: "READY"},
		},
		Account: AccountLandscape{
			Profile:  ProfileAccountDiscoveryV3,
			Identity: AccountIdentity{SellerAccountID: 42},
			Sales: &AccountSalesBlock{
				State: BlockReady,
				OrderedAmount: BuildMoneyComparison(
					&MoneyAmount{Amount: "100", CurrencyCode: "RUB"},
					&MoneyAmount{Amount: "0", CurrencyCode: "RUB"},
				),
				OrderedUnits: BuildCountComparison(int64PtrLocal(10), int64PtrLocal(0)),
			},
			Assortment:    &AccountAssortmentBlock{State: BlockReadyEmpty},
			Concentration: &AccountConcentrationBlock{State: BlockReadyEmpty},
		},
		SKUs: []SKULandscape{
			{
				Profile: ProfileSKUCoreV3, Identity: SKUIdentity{CanonicalSKUID: 2},
				Sales: &SKUSalesBlock{State: BlockReadyEmpty},
				Stock: &SKUStockBlock{State: BlockReadyEmpty},
				Price: &SKUPriceBlock{State: BlockReadyEmpty},
			},
			{
				Profile: ProfileSKUCoreV3, Identity: SKUIdentity{CanonicalSKUID: 1},
				Sales: &SKUSalesBlock{State: BlockReadyEmpty},
				Stock: &SKUStockBlock{State: BlockReadyEmpty},
				Price: &SKUPriceBlock{State: BlockReadyEmpty},
			},
		},
	}
	b := CoreBuilder{Source: MemSource{Feed: feed}}
	s1, body1, h1, err := b.BuildCore(ctx, sealed, "u1", stringsRepeat("a", 64), stringsRepeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	s2, body2, h2, err := b.BuildCore(ctx, sealed, "u1", stringsRepeat("a", 64), stringsRepeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || string(body1) != string(body2) {
		t.Fatalf("non-deterministic hash/body")
	}
	if s1.SKUs[0].Identity.CanonicalSKUID != 1 || s2.SKUs[0].Identity.CanonicalSKUID != 1 {
		t.Fatalf("SKUs not sorted by semantic key")
	}
}

func TestDetailTruncationAndSelection(t *testing.T) {
	ctx, sealed := sealedCtx(t, 7)
	coreFeed := CoreFeed{
		GlobalCapabilities: []CapabilityFlag{
			{Key: "SALES.ACCOUNT_DEMAND", FinalState: "READY"},
			{Key: "SALES.SKU_DEMAND", FinalState: "READY"},
			{Key: "STOCK.SKU_CURRENT", FinalState: "READY"},
			{Key: "PRICE.CUSTOMER_PRICE", FinalState: "READY"},
			{Key: "ADVERTISING.SKU_TRAFFIC", FinalState: "READY"},
			{Key: "PROMOTIONS.MEMBERSHIP", FinalState: "READY"},
			{Key: "SEARCH.SELLER_SUMMARY", FinalState: "READY"},
			{Key: "SEARCH.QUERY_DETAILS", FinalState: "READY"},
		},
		Account: AccountLandscape{
			Profile:       ProfileAccountDiscoveryV3,
			Identity:      AccountIdentity{SellerAccountID: 7},
			Sales:         &AccountSalesBlock{State: BlockReadyEmpty},
			Assortment:    &AccountAssortmentBlock{State: BlockReadyEmpty},
			Concentration: &AccountConcentrationBlock{State: BlockReadyEmpty},
		},
		SKUs: []SKULandscape{
			{
				Profile: ProfileSKUCoreV3, Identity: SKUIdentity{CanonicalSKUID: 100},
				Sales:       &SKUSalesBlock{State: BlockReadyEmpty},
				Stock:       &SKUStockBlock{State: BlockReadyEmpty},
				Price:       &SKUPriceBlock{State: BlockReadyEmpty},
				Advertising: &SKUAdsBlock{State: BlockReadyEmpty, Traffic: &SKUAdsTrafficBlock{State: BlockReadyEmpty}},
				Promotions:  &SKUPromotionsBlock{State: BlockReadyEmpty},
				Search:      &SKUSearchBlock{State: BlockReadyEmpty},
			},
			{
				Profile: ProfileSKUCoreV3, Identity: SKUIdentity{CanonicalSKUID: 200},
				Sales:       &SKUSalesBlock{State: BlockReadyEmpty},
				Stock:       &SKUStockBlock{State: BlockReadyEmpty},
				Price:       &SKUPriceBlock{State: BlockReadyEmpty},
				Advertising: &SKUAdsBlock{State: BlockReadyEmpty, Traffic: &SKUAdsTrafficBlock{State: BlockReadyEmpty}},
				Promotions:  &SKUPromotionsBlock{State: BlockReadyEmpty},
				Search:      &SKUSearchBlock{State: BlockReadyEmpty},
			},
		},
	}
	coreB := CoreBuilder{Source: MemSource{Feed: coreFeed}}
	core, _, _, err := coreB.BuildCore(ctx, sealed, "u1", stringsRepeat("c", 64), stringsRepeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	src := MemSource{
		SKUExtra: map[int64]SKUDetailExtras{
			100: {
				CampaignContext: supportingFromTruncation(ApplyTruncation(
					[]CampaignContextItem{{CampaignID: 1}, {CampaignID: 2}, {CampaignID: 3}, {CampaignID: 4}, {CampaignID: 5}, {CampaignID: 6}},
					6, 6, "PAID_SPEND_DESC", "TOP_N_BY_PAID_SPEND",
				)),
				PromotionContext: emptySupportingContext[PromotionContextItem]("PROMOTION_ID_ASC", "TOP_N_ACTIVE_OR_CANDIDATE"),
				QueryContext:     emptySupportingContext[SearchQueryContextItem]("PROVIDER_SEARCH_RANK", "TOP_N_PROVIDER_RANKED"),
			},
		},
	}
	detail, _, _, err := (DetailBuilder{Source: src}).BuildDetail(ctx, sealed, DetailBuildInput{
		Core: core, Selected: []EntityRef{{Kind: EntitySKU, ID: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.SKUs) != 1 || detail.SKUs[0].Identity.CanonicalSKUID != 100 {
		t.Fatalf("expected only selected SKU, got %+v", detail.SKUs)
	}
	if len(detail.Campaigns) != 0 {
		t.Fatalf("unselected campaigns must be omitted")
	}
	if detail.SKUs[0].CampaignContext == nil || !detail.SKUs[0].CampaignContext.Truncated || detail.SKUs[0].CampaignContext.ReturnedCount != 5 {
		t.Fatalf("truncation=%+v", detail.SKUs[0].CampaignContext)
	}
}

func TestValidateFromZeroPctRejected(t *testing.T) {
	set := minimalSet(1)
	pct := canonicaljson.Decimal("500")
	set.Account.Sales = &AccountSalesBlock{
		State: BlockReady,
		OrderedAmount: MoneyComparison{
			Current:       &MoneyAmount{Amount: "100", CurrencyCode: "RUB"},
			Previous:      &MoneyAmount{Amount: "0", CurrencyCode: "RUB"},
			AbsoluteDelta: &MoneyAmount{Amount: "100", CurrencyCode: "RUB"},
			PctDelta:      &pct,
			ChangeState:   ChangeFromZero,
		},
		OrderedUnits: BuildCountComparison(int64PtrLocal(1), int64PtrLocal(0)),
	}
	if err := ValidateLandscapeCore(set, 1); err == nil {
		t.Fatal("expected FROM_ZERO pct_delta rejection")
	}
}

func minimalSet(seller int64) LandscapeSet {
	return LandscapeSet{
		Context: LandscapeContext{
			AnalysisDate:            canonicaljson.Date(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)),
			AnalysisCutoffAt:        time.Date(2026, 8, 20, 5, 40, 0, 0, time.UTC),
			SellerAccountID:         seller,
			FactualUniverseID:       "u",
			FactualUniverseHash:     stringsRepeat("e", 64),
			ContractBundleHash:      stringsRepeat("f", 64),
			LandscapeSchemaVersion:  LandscapeSchemaV1,
			LandscapeProfileVersion: ProfileAccountDiscoveryV1,
			BusinessTimezone:        "Europe/Moscow",
		},
		Account: AccountLandscape{Profile: ProfileAccountDiscoveryV1, Identity: AccountIdentity{SellerAccountID: seller}},
	}
}

func sealedCtx(t *testing.T, seller int64) (context.Context, factualapi.FactualReadContext) {
	t.Helper()
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: seller,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        time.Date(2026, 8, 20, 2, 40, 0, 0, time.UTC),
		Policies:        factualapi.CurrentPolicies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return context.Background(), sealed
}

type stubSeal struct{ id, hash string }

func (s stubSeal) SealedBindingHash() string { return s.hash }
func (s stubSeal) SealedUniverseID() string  { return s.id }

func int64PtrLocal(v int64) *int64 { return &v }

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, n*len(s))
	for len(b) < n {
		b = append(b, s...)
	}
	return string(b[:n])
}

func TestDomainWindowsSalesNotSearch(t *testing.T) {
	salesTo := canonicaljson.Date(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC))
	searchTo := canonicaljson.Date(time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC))
	feed := CoreFeed{
		DomainWindows: []DomainWindow{
			{
				Domain: "sales",
				Current: &DateRange{
					From: canonicaljson.Date(time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)),
					To:   salesTo,
				},
				AlignedToAnalysisDate: true,
			},
			{
				Domain: "search",
				Current: &DateRange{
					From: canonicaljson.Date(time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)),
					To:   searchTo,
				},
				AlignedToAnalysisDate: false,
				LagDays:               int64PtrLocal(2),
			},
		},
		Account: AccountLandscape{Profile: ProfileAccountDiscoveryV1, Identity: AccountIdentity{SellerAccountID: 1}},
	}
	sales := feed.DomainWindows[0]
	search := feed.DomainWindows[1]
	if sales.Domain == search.Domain {
		t.Fatal("domains must differ")
	}
	if sales.Current.To == search.Current.To {
		t.Fatal("Search must not reuse Sales window end date")
	}
	if search.LagDays == nil || *search.LagDays != 2 {
		t.Fatalf("search lag=%v", search.LagDays)
	}
}

func TestUnmappedCategoryIdentity(t *testing.T) {
	set := minimalSet(1)
	set.Categories = []CategoryLandscape{{
		Profile:  ProfileCategoryCoreV1,
		Identity: CategoryIdentity{CategoryID: UnmappedCategoryID, Name: "UNMAPPED", IsLeaf: true, Unmapped: true},
		Sales: &CategorySalesBlock{
			State: BlockReady,
			OrderedAmount: BuildMoneyComparison(
				&MoneyAmount{Amount: "10", CurrencyCode: "RUB"},
				&MoneyAmount{Amount: "5", CurrencyCode: "RUB"},
			),
			OrderedUnits: BuildCountComparison(int64PtrLocal(2), int64PtrLocal(1)),
		},
	}}
	if err := ValidateLandscapeCore(set, 1); err != nil {
		t.Fatal(err)
	}
	if !set.Categories[0].Identity.Unmapped {
		t.Fatal("UNMAPPED flag required")
	}
}
