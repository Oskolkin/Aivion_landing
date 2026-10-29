package actioncatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

func TestCatalogExactly19(t *testing.T) {
	c := MustHave19()
	if len(c) != 19 {
		t.Fatalf("want 19 got %d", len(c))
	}
	seen := map[string]bool{}
	for _, d := range c {
		if d.ActionTypeKey == "" || d.Version != CatalogV1 || d.TargetKind == "" || d.ActionFamily == "" || d.ValidationPolicy == "" {
			t.Fatalf("incomplete %#v", d)
		}
		if seen[d.ActionTypeKey] {
			t.Fatalf("duplicate %s", d.ActionTypeKey)
		}
		seen[d.ActionTypeKey] = true
		if d.ActionFamily == "SEARCH" || d.ActionFamily == "FINANCE" || d.ActionFamily == "FUNNEL" {
			t.Fatal("forbidden family")
		}
	}
}

func TestMenuDeterministicAndGates(t *testing.T) {
	sku := discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}
	snap := Snapshot{
		Capabilities: map[string]string{"PRICE.SKU": "READY", "SALES.SKU_DEMAND": "READY"},
		SKU: map[string]SKUState{"417": {
			HasPrice: true, CurrentPrice: &Money{Amount: 2300, Currency: "RUB"}, HasDemandContext: true,
		}},
		MainIDs: map[string]bool{"SKU|417": true, "ACCOUNT|acc": true},
	}
	mains := []MainEntity{{Ref: sku}, {Ref: discovery.PackEntity{Kind: discovery.KindAccount, ID: "acc"}}}
	m1 := BuildMenu(snap, mains)
	m2 := BuildMenu(snap, mains)
	if HashMenu(m1) != HashMenu(m2) {
		t.Fatal("menu hash not deterministic")
	}
	if _, ok := menuItem(m1, TypeSKUAdjustPrice); !ok {
		t.Fatal("eligible price action omitted")
	}
	if _, ok := menuItem(m1, TypeSKUJoinPromotion); ok {
		t.Fatal("impossible join promotion must be omitted")
	}
	if _, ok := menuItem(m1, TypeAccountRatingRemediation); ok {
		t.Fatal("account health without keys must be omitted")
	}
	noCap := snap
	noCap.Capabilities = map[string]string{"SEARCH.SKU": "READY"}
	m3 := BuildMenu(noCap, mains)
	if _, ok := menuItem(m3, TypeSKUAdjustPrice); ok {
		t.Fatal("unsupported capability must omit price action")
	}
	supportOnly := snap
	supportOnly.MainIDs = map[string]bool{"ACCOUNT|acc": true}
	m4 := BuildMenu(supportOnly, mains)
	if _, ok := menuItem(m4, TypeSKUAdjustPrice); ok {
		t.Fatal("non-main target must be omitted")
	}
}

func TestProposalValidators(t *testing.T) {
	sku := discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}
	snap := Snapshot{
		Capabilities: map[string]string{"PRICE.SKU": "READY"},
		SKU:          map[string]SKUState{"417": {HasPrice: true, CurrentPrice: &Money{Amount: 2300, Currency: "RUB"}}},
		MainIDs:      map[string]bool{"SKU|417": true},
	}
	menu := BuildMenu(snap, []MainEntity{{Ref: sku}})
	price := 2190.0
	ok := Proposal{ActionType: TypeSKUAdjustPrice, Target: sku, Parameters: Parameters{RecommendedPrice: &Money{Amount: price, Currency: "RUB"}}}
	if err := ValidateProposal(ok, menu, snap); err != nil {
		t.Fatal(err)
	}
	noop := ok
	noop.Parameters.RecommendedPrice = &Money{Amount: 2300, Currency: "RUB"}
	if err := ValidateProposal(noop, menu, snap); err == nil {
		t.Fatal("noop price")
	}
	unknown := ok
	unknown.ActionType = "SKU_FIX_FUNNEL"
	if err := ValidateProposal(unknown, menu, snap); err == nil {
		t.Fatal("unknown type")
	}
	notMenu := ok
	notMenu.ActionType = TypeSKUJoinPromotion
	if err := ValidateProposal(notMenu, menu, snap); err == nil {
		t.Fatal("not in menu")
	}
}

func TestNoForbiddenImports(t *testing.T) {
	root := "."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := string(b)
		for _, bad := range []string{"integrations/ozon", "pgx", "dbgen", "unifiedanalysis", "aipipeline", "openai", "jackc/pgx"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s imports or mentions %s", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
