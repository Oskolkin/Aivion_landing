package collection

import "testing"

func TestSourceContractIdentityIsDeterministicAndSemantic(t *testing.T) {
	first := NewSourceContract("CATALOG", "OZON:CATALOG", "v2", "/v3/product", "catalog-v2", "identity=sku")
	second := NewSourceContract("CATALOG", "OZON:CATALOG", "v2", "/v3/product", "catalog-v2", "identity=sku")
	changed := NewSourceContract("CATALOG", "OZON:CATALOG", "v2", "/v3/product", "catalog-v2", "identity=product_id")
	if first != second || first.SemanticMappingHash == "" {
		t.Fatalf("same semantics produced different identity: first=%+v second=%+v", first, second)
	}
	if first.SemanticMappingHash == changed.SemanticMappingHash {
		t.Fatal("semantic change did not change source contract hash")
	}
}
