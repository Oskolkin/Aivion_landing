package dashboardfacts

import (
	"math"
	"sort"
)

// BuildKeySKUCohortFacts selects SKUs whose cumulative 7-day revenue reaches
// KeySKUCohortThresholdPct of store total (revenue_7d > 0 only).
func BuildKeySKUCohortFacts(idx SKUFactsIndex) KeySKUCohortFacts {
	return buildKeySKUCohortFacts(idx)
}

// buildKeySKUCohortFacts selects SKUs whose cumulative 7-day revenue reaches
// KeySKUCohortThresholdPct of store total (revenue_7d > 0 only).
func buildKeySKUCohortFacts(idx SKUFactsIndex) KeySKUCohortFacts {
	out := KeySKUCohortFacts{
		Status:        "not_enough_data",
		ThresholdPct:  KeySKUCohortThresholdPct,
		PolicyVersion: KeySKUCohortPolicyVersion,
		Items:         []KeySKUCohortItem{},
	}

	candidates := make([]SKUFactRecord, 0, len(idx.ByOzonProductID))
	var storeTotal float64
	for _, rec := range idx.ByOzonProductID {
		if rec.Revenue7d <= 0 {
			continue
		}
		storeTotal += rec.Revenue7d
		candidates = append(candidates, rec)
	}
	out.StoreTotalRevenue7d = storeTotal
	if storeTotal <= 0 {
		return out
	}

	sortKeySKUCohortCandidates(candidates)

	var cumulative float64
	var keyTotal float64
	items := make([]KeySKUCohortItem, 0, len(candidates))
	for i, rec := range candidates {
		share := (rec.Revenue7d / storeTotal) * 100
		cumulative += share
		keyTotal += rec.Revenue7d
		items = append(items, KeySKUCohortItem{
			OzonProductID:             rec.OzonProductID,
			SKU:                       rec.SKU,
			OfferID:                   rec.OfferID,
			ProductName:               rec.ProductName,
			Revenue7d:                 rec.Revenue7d,
			Orders7d:                  rec.Orders7d,
			RevenueSharePct:           roundKeySKUCohortPct(share),
			CumulativeRevenueSharePct: roundKeySKUCohortPct(cumulative),
			Rank:                      i + 1,
		})
		if items[len(items)-1].CumulativeRevenueSharePct >= KeySKUCohortThresholdPct {
			break
		}
	}

	out.Status = "available"
	out.Items = items
	out.KeySKUsTotalRevenue7d = keyTotal
	if len(items) > 0 {
		out.CumulativeRevenueSharePct = items[len(items)-1].CumulativeRevenueSharePct
	}
	return out
}

func sortKeySKUCohortCandidates(candidates []SKUFactRecord) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Revenue7d != b.Revenue7d {
			return a.Revenue7d > b.Revenue7d
		}
		aSKU, aOK := skuPtrValue(a.SKU)
		bSKU, bOK := skuPtrValue(b.SKU)
		if aOK && bOK && aSKU != bSKU {
			return aSKU < bSKU
		}
		if aOK != bOK {
			return aOK // non-nil SKU before nil
		}
		return a.OzonProductID < b.OzonProductID
	})
}

func skuPtrValue(sku *int64) (int64, bool) {
	if sku == nil {
		return 0, false
	}
	return *sku, true
}

func roundKeySKUCohortPct(v float64) float64 {
	return math.Round(v*10) / 10
}
