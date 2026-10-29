package actionreconcile

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

func TestSignatureCanonicalNotSprintf(t *testing.T) {
	a := HashSignature(SignatureInput{SellerID: 1, ActionType: "SKU_ADJUST_PRICE", Target: EntityRef{Kind: discovery.KindSKU, ID: "417"}, IntentSignature: "DECREASE"})
	b := HashSignature(SignatureInput{SellerID: 1, ActionType: "SKU_ADJUST_PRICE", Target: EntityRef{Kind: discovery.KindSKU, ID: "417"}, IntentSignature: "DECREASE", NormalizedContext: map[string]string{}})
	if a != b || len(a) != 64 {
		t.Fatalf("%s vs %s", a, b)
	}
	inc := HashSignature(SignatureInput{SellerID: 1, ActionType: "SKU_ADJUST_PRICE", Target: EntityRef{Kind: discovery.KindSKU, ID: "417"}, IntentSignature: "INCREASE"})
	if inc == a {
		t.Fatal("intent must change identity")
	}
}

func TestPriceIntent(t *testing.T) {
	cur := money(2300)
	rec := money(2190)
	got, err := IntentSignature("SKU_ADJUST_PRICE", paramsPrice(rec), &cur)
	if err != nil || got != "DECREASE" {
		t.Fatalf("%s %v", got, err)
	}
	got, _ = IntentSignature("SKU_ADJUST_PRICE", paramsPrice(money(2400)), &cur)
	if got != "INCREASE" {
		t.Fatal(got)
	}
}

func TestPriceIntentFromMenuHint(t *testing.T) {
	menu := actioncatalog.Menu{Items: []actioncatalog.MenuItem{{
		HardConstraints: []string{"current_price:417=2300RUB"},
	}}}
	hint := actioncatalog.PriceHintFromMenu(menu, "SKU", "417")
	if hint == nil || hint.Amount != 2300 {
		t.Fatalf("hint=%v", hint)
	}
	inc, err := IntentSignature("SKU_ADJUST_PRICE", paramsPrice(money(2310)), hint)
	if err != nil || inc != "INCREASE" {
		t.Fatalf("inc=%s err=%v", inc, err)
	}
	dec, err := IntentSignature("SKU_ADJUST_PRICE", paramsPrice(money(2250)), hint)
	if err != nil || dec != "DECREASE" {
		t.Fatalf("dec=%s err=%v", dec, err)
	}
	incSig := HashSignature(SignatureInput{SellerID: 1, ActionType: "SKU_ADJUST_PRICE", Target: EntityRef{Kind: discovery.KindSKU, ID: "417"}, IntentSignature: inc})
	decSig := HashSignature(SignatureInput{SellerID: 1, ActionType: "SKU_ADJUST_PRICE", Target: EntityRef{Kind: discovery.KindSKU, ID: "417"}, IntentSignature: dec})
	if incSig == decSig {
		t.Fatal("opposite intents must differ")
	}
}
