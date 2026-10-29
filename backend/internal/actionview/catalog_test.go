package actionview

import "testing"

func TestCanonicalActionTypesAreNineteen(t *testing.T) {
	got := CanonicalActionTypes()
	if len(got) != 19 {
		t.Fatalf("len=%d", len(got))
	}
	seen := map[string]struct{}{}
	for _, k := range got {
		if _, ok := seen[k]; ok {
			t.Fatalf("duplicate %s", k)
		}
		seen[k] = struct{}{}
	}
}
