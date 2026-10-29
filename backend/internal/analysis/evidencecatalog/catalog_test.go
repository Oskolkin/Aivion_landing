package evidencecatalog

import "testing"

func TestCatalogFiftyClosedKeys(t *testing.T) {
	keys := ExpectedKeys()
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
	enabled := Enabled()
	if len(enabled) == 0 {
		t.Fatal("expected enabled types")
	}
	for _, tpe := range Catalog() {
		if !tpe.EnabledForDiscoveryV1 && tpe.DisabledReason == "" {
			t.Fatalf("disabled %s missing reason", tpe.Key)
		}
	}
}
