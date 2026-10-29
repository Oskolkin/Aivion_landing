package materiality

import (
	"math/big"
	"sort"
)

// LaneRank is one entity's position inside a single Materiality Lane.
type LaneRank struct {
	SemanticKey string
	Metric      Exact
	Exposure    Exact
	Rank        int // 1-based
}

// WeightedRRF computes fusion utility = Σ weight/(k+rank) over lanes the entity qualified for.
// Ranks start at 1. Tie-break between equal utilities uses semanticKey ascending.
func WeightedRRF(k int, memberships map[string][]LaneMembership, weights map[string]*big.Rat) map[string]Exact {
	if k < 1 {
		k = 60
	}
	out := make(map[string]Exact, len(memberships))
	for key, lanes := range memberships {
		sum := new(big.Rat)
		for _, m := range lanes {
			w := weights[m.LaneKey]
			if w == nil || w.Sign() == 0 {
				w = ratInt(1)
			}
			term := quoRat(w, ratInt(int64(k+m.Rank)))
			sum = addRat(sum, term)
		}
		out[key] = formatRat(sum)
	}
	return out
}

// LaneMembership records that an entity appeared in a lane ranking.
type LaneMembership struct {
	LaneKey string `json:"lane_key"`
	Rank    int    `json:"rank"`
	Metric  Exact  `json:"metric"`
}

// OrderByFusion returns semantic keys sorted by fusion utility DESC, semantic key ASC.
func OrderByFusion(utilities map[string]Exact) []string {
	keys := make([]string, 0, len(utilities))
	for k := range utilities {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ci, err := compareExact(utilities[keys[i]], utilities[keys[j]])
		if err != nil {
			return false
		}
		if ci != 0 {
			return ci > 0
		}
		return keys[i] < keys[j]
	})
	return keys
}

// AssignRanks sorts candidates by metric (DESC or ASC), then exposure DESC, then semantic key ASC.
func AssignRanks(cands []LaneRank, dir Direction) []LaneRank {
	out := append([]LaneRank(nil), cands...)
	sort.SliceStable(out, func(i, j int) bool {
		mi, errI := parseRat(out[i].Metric)
		mj, errJ := parseRat(out[j].Metric)
		if errI == nil && errJ == nil {
			if c := mi.Cmp(mj); c != 0 {
				if dir == DirectionAsc {
					return c < 0
				}
				return c > 0
			}
		} else if errI != nil && errJ == nil {
			return false
		} else if errI == nil && errJ != nil {
			return true
		}
		ei, errEI := parseRat(out[i].Exposure)
		ej, errEJ := parseRat(out[j].Exposure)
		if errEI == nil && errEJ == nil {
			if c := ei.Cmp(ej); c != 0 {
				return c > 0
			}
		} else if errEI != nil && errEJ == nil {
			return false
		} else if errEI == nil && errEJ != nil {
			return true
		}
		return out[i].SemanticKey < out[j].SemanticKey
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// TruncateLane keeps top candidateLimit ranks (already sorted).
func TruncateLane(ranked []LaneRank, limit int) []LaneRank {
	if limit <= 0 || len(ranked) <= limit {
		return ranked
	}
	return ranked[:limit]
}
