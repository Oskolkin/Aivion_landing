package materiality

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func TestCoreFactsFromLandscapeSkipsUnmapped(t *testing.T) {
	amt := func(v string) *landscape.MoneyAmount {
		return &landscape.MoneyAmount{Amount: canonicaljson.Decimal(v), CurrencyCode: "RUB"}
	}
	set := landscape.LandscapeSet{
		Account: landscape.AccountLandscape{
			Identity: landscape.AccountIdentity{SellerAccountID: 42},
			Sales: &landscape.AccountSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current: amt("1000.50"), Previous: amt("800"), ChangeState: landscape.ChangeIncreased,
				},
			},
		},
		GlobalCapabilities: []landscape.CapabilityFlag{
			{Key: "SALES.SKU_DEMAND", FinalState: ReadinessReady},
		},
		Categories: []landscape.CategoryLandscape{
			{Identity: landscape.CategoryIdentity{CategoryID: 1, IsLeaf: true}, Sales: &landscape.CategorySalesBlock{
				State:         landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{Current: amt("500"), ChangeState: landscape.ChangeNoBase},
			}},
			{Identity: landscape.CategoryIdentity{CategoryID: 0, Unmapped: true}},
		},
		SKUs: []landscape.SKULandscape{
			{Identity: landscape.SKUIdentity{CanonicalSKUID: 7, CategoryID: int64Ptr(1)}, Sales: &landscape.SKUSalesBlock{
				State:         landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{Current: amt("200.25"), ChangeState: landscape.ChangeNoBase},
			}},
		},
	}
	facts, err := CoreFactsFromLandscape(set)
	if err != nil {
		t.Fatal(err)
	}
	if !facts.Account.OrderedAmountCurrent7.IsKnown() || *facts.Account.OrderedAmountCurrent7.Value != Exact("1000.50") {
		t.Fatalf("account amount: %v", facts.Account.OrderedAmountCurrent7)
	}
	if !facts.Account.OrderedAmountPrevious7.IsKnown() {
		t.Fatalf("account previous must be KNOWN when present")
	}
	if len(facts.Categories) != 1 || facts.Categories[0].SemanticKey != "category:1" {
		t.Fatalf("unmapped must be skipped: %#v", facts.Categories)
	}
	if facts.Capabilities["SALES.SKU_DEMAND"] != ReadinessReady {
		t.Fatalf("capabilities: %#v", facts.Capabilities)
	}
	if len(facts.SKUs) != 1 || !facts.SKUs[0].OrderedAmountCurrent7.IsKnown() || *facts.SKUs[0].OrderedAmountCurrent7.Value != Exact("200.25") {
		t.Fatalf("sku features: %#v", facts.SKUs)
	}
	if facts.SKUs[0].OrderedAmountPrevious7.State != FeatureUnknown {
		t.Fatalf("missing previous must be UNKNOWN, got %#v", facts.SKUs[0].OrderedAmountPrevious7)
	}
}

func TestParseExactDecimalInvalidFails(t *testing.T) {
	if _, err := parseExactDecimal(canonicaljson.Decimal("not-a-number")); err == nil {
		t.Fatal("invalid decimal must fail parseExactDecimal")
	}
	if _, err := parseExactDecimal(canonicaljson.Decimal("")); err == nil {
		t.Fatal("empty decimal must fail parseExactDecimal")
	}
	if ex, err := parseExactDecimal(canonicaljson.Decimal("12.5")); err != nil || ex != Exact("12.5") {
		t.Fatalf("valid decimal: ex=%v err=%v", ex, err)
	}
}

func TestCoreFactsFromLandscapeInvalidOperationalDecimalFails(t *testing.T) {
	bad := canonicaljson.Decimal("bad")
	good := canonicaljson.Decimal("3.5")
	prev := canonicaljson.Decimal("8")
	units := int64(10)
	set := landscape.LandscapeSet{
		Account: landscape.AccountLandscape{Identity: landscape.AccountIdentity{SellerAccountID: 1}},
		SKUs: []landscape.SKULandscape{{
			Identity: landscape.SKUIdentity{CanonicalSKUID: 99},
			Stock: &landscape.SKUStockBlock{
				State: landscape.BlockReady, SellableUnits: &units,
				CoverageDays7D: &bad,
			},
			Search: &landscape.SKUSearchBlock{
				State:                           landscape.BlockReady,
				ProviderAveragePosition:         &good,
				PreviousProviderAveragePosition: &prev,
			},
		}},
	}
	if _, err := CoreFactsFromLandscape(set); err == nil {
		t.Fatal("invalid coverage decimal must fail CoreFactsFromLandscape")
	}
}

func TestCoreFactsFromLandscapeMissingMoneyNotZero(t *testing.T) {
	amt := func(v string) *landscape.MoneyAmount {
		return &landscape.MoneyAmount{Amount: canonicaljson.Decimal(v), CurrencyCode: "RUB"}
	}
	set := landscape.LandscapeSet{
		Account: landscape.AccountLandscape{Identity: landscape.AccountIdentity{SellerAccountID: 1}},
		SKUs: []landscape.SKULandscape{{
			Identity: landscape.SKUIdentity{CanonicalSKUID: 1},
			Sales: &landscape.SKUSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current: amt("100"), ChangeState: landscape.ChangeNoBase,
				},
			},
		}},
	}
	facts, err := CoreFactsFromLandscape(set)
	if err != nil {
		t.Fatal(err)
	}
	s := facts.SKUs[0]
	if s.OrderedAmountPrevious7.State != FeatureUnknown {
		t.Fatalf("missing previous must be UNKNOWN, got %#v", s.OrderedAmountPrevious7)
	}
	if s.OrderedAmountPrevious7.IsKnown() {
		t.Fatal("missing previous must not be KNOWN")
	}
}

func int64Ptr(v int64) *int64 { return &v }
