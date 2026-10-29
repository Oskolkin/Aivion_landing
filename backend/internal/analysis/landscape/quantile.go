package landscape

import (
	"math/big"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// nearestRankQuantileV1 implements landscape_nearest_rank_quantile_v1:
// sort ascending, rank(p)=ceil(p*N), index=max(rank-1,0).
func nearestRankQuantileV1(values []*big.Rat, num, den int64) *canonicaljson.Decimal {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]*big.Rat(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Cmp(sorted[j]) < 0
	})
	n := int64(len(sorted))
	rank := (num*n + den - 1) / den
	idx := rank - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	d := canonicaljson.Decimal(formatRatLocal(sorted[idx]))
	return &d
}
