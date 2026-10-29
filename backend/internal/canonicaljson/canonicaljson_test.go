package canonicaljson

import (
	"strings"
	"testing"
	"time"
)

func TestDeterministicAndNoFloat(t *testing.T) {
	a := map[string]any{"z": nil, "a": Decimal("001.2300"), "when": time.Date(2026, 8, 25, 12, 3, 4, 5, time.FixedZone("x", 3600))}
	b := map[string]any{"when": time.Date(2026, 8, 25, 11, 3, 4, 5, time.UTC), "a": Decimal("1.23"), "z": nil}
	ab, err := Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ab) != string(bb) || Hash(a) != Hash(b) || strings.Contains(string(ab), "\r") {
		t.Fatalf("not canonical:\n%s\n%s", ab, bb)
	}
	if _, err = Marshal(map[string]any{"money": 1.2}); err == nil {
		t.Fatal("float accepted")
	}
}
