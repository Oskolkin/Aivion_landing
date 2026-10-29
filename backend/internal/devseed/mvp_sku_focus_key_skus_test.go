package devseed

import "testing"

func TestExpectedMVPKeySkuCount(t *testing.T) {
	if ExpectedMVPKeySkuCount() < 8 || ExpectedMVPKeySkuCount() > 12 {
		t.Fatalf("expected 8-12 curated demo key SKUs, got %d", ExpectedMVPKeySkuCount())
	}
}

func TestExpectedMVPKeySkuCohortCount(t *testing.T) {
	count := ExpectedMVPKeySkuCohortCount()
	if count < 8 || count > 25 {
		t.Fatalf("expected simulated key SKU cohort 8-25 after 70%% threshold, got %d", count)
	}
}

func TestExpectedMVPKeySkuMinCumulativeSharePct(t *testing.T) {
	pct := ExpectedMVPKeySkuMinCumulativeSharePct()
	if pct < 55 || pct > 75 {
		t.Fatalf("demo key SKUs cumulative share %.1f should be in 55–75 band", pct)
	}
}

func TestMvpKeySkuWeeklyRevenues_shareOfAccount(t *testing.T) {
	const otherWeekly = 1_000_000.0
	revenues := mvpKeySkuWeeklyRevenues(otherWeekly)
	if len(revenues) != len(mvpKeySkuDemoSKUs) {
		t.Fatalf("expected %d key revenues, got %d", len(mvpKeySkuDemoSKUs), len(revenues))
	}

	var keyTotal float64
	for i, rev := range revenues {
		if rev <= 0 {
			t.Fatalf("%q must have positive planned revenue, got %.2f", mvpKeySkuDemoSKUs[i].ProductName, rev)
		}
		keyTotal += rev
		if i > 0 && rev > revenues[i-1]+1 {
			t.Fatalf("key SKUs must be defined in descending revenue order (%d=%0.f > %d=%0.f)", i, rev, i-1, revenues[i-1])
		}
	}

	account := keyTotal + otherWeekly
	share := keyTotal / account
	if share < 0.55 || share > 0.75 {
		t.Fatalf("key SKU share of account revenue %.3f must be within [0.55, 0.75]", share)
	}
}

func TestKeySkuDemoProductNames_disjointFromOrderDrop(t *testing.T) {
	orderDrop := make(map[string]struct{}, len(mvpOrderDropDemoSKUs))
	for _, scenario := range mvpOrderDropDemoSKUs {
		orderDrop[scenario.ProductName] = struct{}{}
	}
	for _, name := range keySkuDemoProductNames() {
		if _, ok := orderDrop[name]; ok {
			t.Fatalf("key-sku demo overlaps order-drop product %q", name)
		}
	}
}
