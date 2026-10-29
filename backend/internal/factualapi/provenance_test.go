package factualapi

import "testing"

func TestDeriveSourceContractHash(t *testing.T) {
	if got := DeriveSourceContractHash("", "", ""); got != "" {
		t.Fatalf("empty inputs: %q", got)
	}
	a := DeriveSourceContractHash("K", "V", "E")
	b := DeriveSourceContractHash("K", "V", "E")
	if a == "" || len(a) != 64 || a != b {
		t.Fatalf("hash=%q", a)
	}
	if DeriveSourceContractHash("K", "V", "E") == StableHash("semantic_mapping", "search-v1") {
		t.Fatal("contract hash must not equal semantic mapping hash for typical inputs")
	}
}
