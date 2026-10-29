package factualapi

import "testing"

func TestNormalizeEntitySearchQueryNaturalLanguage(t *testing.T) {
	tok := NormalizeEntitySearchQuery("Какой остаток у Polaris 417?")
	if len(tok.Texts) != 1 || tok.Texts[0] != "polaris" {
		t.Fatalf("texts=%v", tok.Texts)
	}
	if len(tok.Numerics) != 1 || tok.Numerics[0] != 417 {
		t.Fatalf("nums=%v", tok.Numerics)
	}
	tok2 := NormalizeEntitySearchQuery("SKU417")
	if len(tok2.Numerics) != 1 || tok2.Numerics[0] != 417 {
		t.Fatalf("sku417 nums=%v", tok2.Numerics)
	}
	tok3 := NormalizeEntitySearchQuery("POLARIS")
	if len(tok3.Texts) != 1 || tok3.Texts[0] != "polaris" {
		t.Fatalf("case=%v", tok3.Texts)
	}
}
