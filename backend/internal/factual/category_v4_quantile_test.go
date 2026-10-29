package factual

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestNearestRankExactCategorySearchQuantiles(t *testing.T) {
	raw := []string{"10", "20", "30", "40"}
	positions := make([]exactPosition, 0, len(raw))
	for _, s := range raw {
		d, err := factualapi.ParseDecimal(s)
		if err != nil {
			t.Fatal(err)
		}
		pos, err := parseExactPosition(d)
		if err != nil {
			t.Fatal(err)
		}
		positions = append(positions, pos)
	}
	p25 := nearestRankExact(positions, 1, 4)
	p50 := nearestRankExact(positions, 1, 2)
	p75 := nearestRankExact(positions, 3, 4)
	if p25 == nil || p25.String() != "10" {
		t.Fatalf("P25=%v want 10", p25)
	}
	if p50 == nil || p50.String() != "20" {
		t.Fatalf("P50=%v want 20", p50)
	}
	if p75 == nil || p75.String() != "30" {
		t.Fatalf("P75=%v want 30", p75)
	}
}
