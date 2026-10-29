package materiality

import (
	"math/big"
	"sort"
)

// DiversityCandidate is an entity considered during soft diversity reranking.
type DiversityCandidate struct {
	SemanticKey     string
	CategoryKey     string // SKU leaf category; empty for Category/Campaign kinds
	LaneFingerprint []string
	FusionUtility   Exact
	IsAnchor        bool
}

// SoftDiversityOrder produces a diversified order without hard-capping categories
// and without evicting protected anchors. Anchors are emitted first (stable by
// fusion utility), then remaining candidates are greedily selected by adjusted utility.
//
// V1_1 adjusted = fusion * (1 - min(MaxPenalty, catPen+lanePen)) * (1 + bonus).
func SoftDiversityOrder(cands []DiversityCandidate, params DiversityParams) []string {
	if len(cands) == 0 {
		return nil
	}
	anchors := make([]DiversityCandidate, 0)
	rest := make([]DiversityCandidate, 0, len(cands))
	for _, c := range cands {
		if c.IsAnchor {
			anchors = append(anchors, c)
		} else {
			rest = append(rest, c)
		}
	}
	sort.SliceStable(anchors, func(i, j int) bool {
		ci, err := compareExact(anchors[i].FusionUtility, anchors[j].FusionUtility)
		if err != nil {
			return false
		}
		if ci != 0 {
			return ci > 0
		}
		return anchors[i].SemanticKey < anchors[j].SemanticKey
	})

	selected := make([]DiversityCandidate, 0, len(cands))
	order := make([]string, 0, len(cands))
	catCount := map[string]int{}
	laneSeen := map[string]int{}

	emit := func(c DiversityCandidate) {
		selected = append(selected, c)
		order = append(order, c.SemanticKey)
		if c.CategoryKey != "" {
			catCount[c.CategoryKey]++
		}
		for _, lk := range c.LaneFingerprint {
			laneSeen[lk]++
		}
	}

	for _, a := range anchors {
		emit(a)
	}

	remaining := append([]DiversityCandidate(nil), rest...)
	for len(remaining) > 0 {
		bestIdx := 0
		bestScore := adjustedUtility(remaining[0], catCount, laneSeen, params)
		for i := 1; i < len(remaining); i++ {
			s := adjustedUtility(remaining[i], catCount, laneSeen, params)
			if cmpRat(s, bestScore) > 0 || (cmpRat(s, bestScore) == 0 && remaining[i].SemanticKey < remaining[bestIdx].SemanticKey) {
				bestScore = s
				bestIdx = i
			}
		}
		emit(remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return order
}

func adjustedUtility(c DiversityCandidate, catCount, laneSeen map[string]int, p DiversityParams) *big.Rat {
	fusion, err := parseRat(c.FusionUtility)
	if err != nil {
		fusion = new(big.Rat)
	}
	catPen := new(big.Rat)
	bonus := new(big.Rat)
	if c.CategoryKey != "" {
		if n := catCount[c.CategoryKey]; n > 0 {
			catPen = mulRat(ratFromFloat64(p.CategoryRepetitionFraction), ratInt(int64(n)))
		} else if len(catCount) > 0 {
			bonus = ratFromFloat64(p.NewCategoryBonusFraction)
		}
	}
	lanePen := new(big.Rat)
	overlap := 0
	for _, lk := range c.LaneFingerprint {
		if laneSeen[lk] > 0 {
			overlap++
		}
	}
	if overlap > 0 && len(c.LaneFingerprint) > 0 {
		frac := quoRat(ratInt(int64(overlap)), ratInt(int64(len(c.LaneFingerprint))))
		lanePen = mulRat(ratFromFloat64(p.LaneOverlapFraction), frac)
	}
	pen := addRat(catPen, lanePen)
	maxPen := ratFromFloat64(p.MaxPenaltyFraction)
	if cmpRat(pen, maxPen) > 0 {
		pen = maxPen
	}
	if pen.Sign() < 0 {
		pen = new(big.Rat)
	}
	// (1 - pen) * (1 + bonus)
	one := ratInt(1)
	factor := mulRat(subRat(one, pen), addRat(one, bonus))
	if factor.Sign() < 0 {
		factor = new(big.Rat)
	}
	return mulRat(fusion, factor)
}

// Fingerprint returns a stable sorted lane-key fingerprint.
func Fingerprint(memberships []LaneMembership) []string {
	out := make([]string, len(memberships))
	for i, m := range memberships {
		out[i] = m.LaneKey
	}
	sort.Strings(out)
	return out
}
