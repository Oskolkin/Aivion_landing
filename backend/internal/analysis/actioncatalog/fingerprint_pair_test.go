package actioncatalog

import "testing"

func TestPriceFingerprintOppositeAmountsDistinct(t *testing.T) {
	p1 := Proposal{
		ActionType: TypeSKUAdjustPrice,
		Parameters: Parameters{RecommendedPrice: &Money{Amount: 990, Currency: "RUB"}},
	}
	p2 := Proposal{
		ActionType: TypeSKUAdjustPrice,
		Parameters: Parameters{RecommendedPrice: &Money{Amount: 1010, Currency: "RUB"}},
	}
	if Fingerprint(p1) == Fingerprint(p2) {
		t.Fatal("fingerprints must differ for opposite amounts")
	}
}
