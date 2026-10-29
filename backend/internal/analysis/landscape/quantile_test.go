package landscape

import (
	"math/big"
	"testing"
)

func TestNearestRankQuantileV1(t *testing.T) {
	vals := []*big.Rat{
		big.NewRat(1, 1),
		big.NewRat(2, 1),
		big.NewRat(3, 1),
		big.NewRat(4, 1),
	}
	p50 := nearestRankQuantileV1(vals, 1, 2)
	if p50 == nil || string(*p50) != "2" {
		t.Fatalf("p50=%v want 2", p50)
	}
	p25 := nearestRankQuantileV1(vals, 1, 4)
	if p25 == nil || string(*p25) != "1" {
		t.Fatalf("p25=%v want 1", p25)
	}
	p75 := nearestRankQuantileV1(vals, 3, 4)
	if p75 == nil || string(*p75) != "3" {
		t.Fatalf("p75=%v want 3", p75)
	}
}
