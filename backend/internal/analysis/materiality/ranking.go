package materiality

import (
	"math/big"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// RankedEntity is one entity in a kind-specific diversified ranking (pre-budget).
type RankedEntity struct {
	SemanticKey     string           `json:"semantic_key"`
	CanonicalRefID  string           `json:"canonical_ref_id,omitempty"`
	EntityKind      EntityKind       `json:"entity_kind"`
	CategoryKey     string           `json:"category_key,omitempty"`
	LaneMemberships []LaneMembership `json:"lane_memberships"`
	LaneFingerprint []string         `json:"lane_fingerprint"`
	FusionUtility   Exact            `json:"fusion_utility"`
	FusionRank      int              `json:"fusion_rank"`
	DiversityRank   int              `json:"diversity_rank"`
	IsAnchor        bool             `json:"is_anchor"`
	AnchorOrder     int              `json:"anchor_order,omitempty"`
	ExposureMetric  Exact            `json:"exposure_metric"`
}

// Ranking is Phase A output: deterministic diversified orderings without budget truncation.
type Ranking struct {
	PolicyVersion       string         `json:"policy_version"`
	PolicyHash          string         `json:"policy_hash"`
	LaneRegistryVersion string         `json:"lane_registry_version"`
	CoreHash            string         `json:"input_core_hash"`
	Categories          []RankedEntity `json:"categories"`
	SKUs                []RankedEntity `json:"skus"`
	Campaigns           []RankedEntity `json:"campaigns"`
	RankingHash         string         `json:"ranking_hash"`
}

// BuildMaterialityRanking runs lanes → RRF → soft diversity for Category/SKU/Campaign.
// Account is never ranked. Does not apply final budget truncation.
// inputCoreHash should be the Landscape Core artifact hash when available; if empty,
// a canonical hash of CoreFacts is used so ranking_hash remains semantic.
func BuildMaterialityRanking(core CoreFacts, policy PolicyV1, inputCoreHash string) Ranking {
	reg := LaneRegistryV1Current()
	if policy.LaneRegistryVersion == LaneRegistryV2 {
		reg = LaneRegistryV2Current()
	}
	coreHash := inputCoreHash
	if coreHash == "" {
		coreHash = hashCoreFacts(core)
	}
	cat := rankKind(EntityCategory, core, policy, reg)
	sku := rankKind(EntitySKU, core, policy, reg)
	camp := rankKind(EntityCampaign, core, policy, reg)

	r := Ranking{
		PolicyVersion:       policy.Version,
		PolicyHash:          policy.Hash(),
		LaneRegistryVersion: policy.LaneRegistryVersion,
		CoreHash:            coreHash,
		Categories:          cat,
		SKUs:                sku,
		Campaigns:           camp,
	}
	r.RankingHash = hashRanking(r)
	return r
}

func hashRanking(r Ranking) string {
	art, err := NewRankingArtifact(r)
	if err != nil {
		panic("materiality: internal ranking artifact invariant violated: " + err.Error())
	}
	art.RankingHash = ""
	return canonicaljson.Hash(art)
}

func hashCoreFacts(core CoreFacts) string {
	return canonicaljson.Hash(core)
}

func rankKind(kind EntityKind, core CoreFacts, policy PolicyV1, reg LaneRegistry) []RankedEntity {
	anchors := buildAnchors(kind, core, policy)
	anchorSet := map[string]int{}
	for i, a := range anchors {
		anchorSet[a] = i + 1
	}

	laneResults := map[string][]LaneRank{} // laneKey → ranked
	for _, lv := range reg.LanesForKind(kind) {
		if !core.CapabilitiesAllowLane(lv) {
			continue
		}
		weight := policy.LaneWeightRat(kind, lv.LaneKey)
		if weight.Sign() <= 0 {
			continue
		}
		cands := runTypedLane(lv, core, policy)
		ranked := AssignRanks(cands, lv.Direction)
		ranked = TruncateLane(ranked, lv.CandidateLimit)
		if len(ranked) > 0 {
			laneResults[lv.LaneKey] = ranked
		}
	}

	memberships := map[string][]LaneMembership{}
	meta := entityMeta(kind, core)
	for laneKey, ranked := range laneResults {
		for _, lr := range ranked {
			memberships[lr.SemanticKey] = append(memberships[lr.SemanticKey], LaneMembership{
				LaneKey: laneKey, Rank: lr.Rank, Metric: lr.Metric,
			})
		}
	}
	// Ensure anchors enter the candidate pool even if no change lane hit them.
	for key := range anchorSet {
		if _, ok := memberships[key]; !ok {
			memberships[key] = nil
		}
	}

	weights := map[string]*big.Rat{}
	for _, lv := range reg.LanesForKind(kind) {
		weights[lv.LaneKey] = policy.LaneWeightRat(kind, lv.LaneKey)
	}
	utilities := WeightedRRF(policy.RRFK, memberships, weights)
	// Anchors with empty memberships still get zero utility so they sort stably;
	// primary ordering for anchors is exposure via diversity emit order + IsAnchor.
	for key := range anchorSet {
		if _, ok := utilities[key]; !ok {
			utilities[key] = Exact("0")
		}
	}

	fusionOrder := OrderByFusion(utilities)
	fusionRank := map[string]int{}
	for i, k := range fusionOrder {
		fusionRank[k] = i + 1
	}

	divCands := make([]DiversityCandidate, 0, len(fusionOrder))
	for _, key := range fusionOrder {
		m := memberships[key]
		divCands = append(divCands, DiversityCandidate{
			SemanticKey:     key,
			CategoryKey:     meta[key].categoryKey,
			LaneFingerprint: Fingerprint(m),
			FusionUtility:   utilities[key],
			IsAnchor:        anchorSet[key] > 0,
		})
	}
	divOrder := SoftDiversityOrder(divCands, policy.Diversity)

	out := make([]RankedEntity, 0, len(divOrder))
	for i, key := range divOrder {
		m := memberships[key]
		sort.Slice(m, func(a, b int) bool { return m[a].LaneKey < m[b].LaneKey })
		exposure := Exact("0")
		if ex, ok := featureExact(meta[key].exposure); ok {
			exposure = ex
		}
		e := RankedEntity{
			SemanticKey:     key,
			CanonicalRefID:  meta[key].refID,
			EntityKind:      kind,
			CategoryKey:     meta[key].categoryKey,
			LaneMemberships: m,
			LaneFingerprint: Fingerprint(m),
			FusionUtility:   utilities[key],
			FusionRank:      fusionRank[key],
			DiversityRank:   i + 1,
			IsAnchor:        anchorSet[key] > 0,
			AnchorOrder:     anchorSet[key],
			ExposureMetric:  exposure,
		}
		out = append(out, e)
	}
	return out
}

type entMeta struct {
	refID       string
	categoryKey string
	exposure    MoneyFeature
}

func entityMeta(kind EntityKind, core CoreFacts) map[string]entMeta {
	out := map[string]entMeta{}
	switch kind {
	case EntityCategory:
		for _, c := range core.Categories {
			out[c.SemanticKey] = entMeta{refID: c.CanonicalRefID, exposure: c.OrderedAmountCurrent7}
		}
	case EntitySKU:
		for _, s := range core.SKUs {
			out[s.SemanticKey] = entMeta{
				refID: s.CanonicalRefID, categoryKey: s.CategorySemanticKey,
				exposure: s.OrderedAmountCurrent7,
			}
		}
	case EntityCampaign:
		for _, c := range core.Campaigns {
			out[c.SemanticKey] = entMeta{
				refID:    c.CanonicalRefID,
				exposure: c.PaidSpendCurrent7,
			}
		}
	}
	return out
}

func buildAnchors(kind EntityKind, core CoreFacts, policy PolicyV1) []string {
	type item struct {
		key string
		val *big.Rat
	}
	var items []item
	total := new(big.Rat)
	var maxN int
	switch kind {
	case EntityCategory:
		maxN = policy.AnchorMax.Categories
		for _, c := range core.Categories {
			ex, ok := featureExact(c.OrderedAmountCurrent7)
			if !ok {
				continue
			}
			v, err := parseRat(ex)
			if err != nil {
				continue
			}
			items = append(items, item{c.SemanticKey, v})
			total = addRat(total, v)
		}
	case EntitySKU:
		maxN = policy.AnchorMax.SKUs
		for _, s := range core.SKUs {
			ex, ok := featureExact(s.OrderedAmountCurrent7)
			if !ok {
				continue
			}
			v, err := parseRat(ex)
			if err != nil {
				continue
			}
			items = append(items, item{s.SemanticKey, v})
			total = addRat(total, v)
		}
	case EntityCampaign:
		maxN = policy.AnchorMax.Campaigns
		for _, c := range core.Campaigns {
			ex, ok := featureExact(c.PaidSpendCurrent7)
			if !ok {
				continue
			}
			v, err := parseRat(ex)
			if err != nil {
				continue
			}
			items = append(items, item{c.SemanticKey, v})
			total = addRat(total, v)
		}
	default:
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if c := cmpRat(items[i].val, items[j].val); c != 0 {
			return c > 0
		}
		return items[i].key < items[j].key
	})
	cum := new(big.Rat)
	target := ratFromFloat64(policy.AnchorCoverageTarget)
	out := make([]string, 0)
	for _, it := range items {
		if maxN > 0 && len(out) >= maxN {
			break
		}
		// Current-exposure anchors only: zero current must not anchor (e.g. TO_ZERO).
		if it.val.Sign() <= 0 {
			break
		}
		out = append(out, it.key)
		if total.Sign() > 0 {
			cum = addRat(cum, quoRat(it.val, total))
			if cmpRat(cum, target) >= 0 && len(out) > 0 {
				break
			}
		}
	}
	return out
}

func runTypedLane(lv LaneVersion, core CoreFacts, policy PolicyV1) []LaneRank {
	switch lv.TypedImplKey {
	case ImplSKUSalesCurrentExposure:
		return skuSalesExposure(core, false)
	case ImplSKUSales30DExposure:
		return skuSalesExposure(core, true)
	case ImplSKUSalesAbsIncrease:
		return skuSalesAbsDelta(core, true)
	case ImplSKUSalesAbsDecrease:
		return skuSalesAbsDelta(core, false)
	case ImplSKUSalesRelIncrease:
		return skuSalesRelDelta(core, policy, true)
	case ImplSKUSalesRelDecrease:
		return skuSalesRelDelta(core, policy, false)
	case ImplSKUSalesFromZero:
		return skuSalesFromZero(core)
	case ImplSKUSalesToZero:
		return skuSalesToZero(core)
	case ImplSKUZeroSellableRecentDemand:
		return skuZeroSellable(core)
	case ImplSKUStockCoverageAsc:
		return skuStockCoverage(core, policy)
	case ImplSKUAdPaidSpendExposure:
		return skuAdExposure(core)
	case ImplSKUAdPaidSpendIncrease:
		return skuAdAbsDelta(core, true)
	case ImplSKUAdPaidSpendDecrease:
		return skuAdAbsDelta(core, false)
	case ImplSKUPriceAbsChange:
		return skuPriceChange(core, policy)
	case ImplSKUSearchPositionChange:
		return skuSearchChange(core, policy)
	case ImplSKUSupplyOverdueExposure:
		return skuSupplyOverdue(core)
	case ImplCatCurrentSalesExposure:
		return catSalesExposure(core)
	case ImplCatSalesAbsIncrease:
		return catSalesAbsDelta(core, true)
	case ImplCatSalesAbsDecrease:
		return catSalesAbsDelta(core, false)
	case ImplCatSalesRelChange:
		return catSalesRelChange(core, policy)
	case ImplCatZeroStockSalesExposure:
		return catZeroStockSales(core)
	case ImplCatAdPaidSpendExposure:
		return catAdExposure(core)
	case ImplCatAdPaidSpendChange:
		return catAdChange(core, policy)
	case ImplCatSupplyOverdueSalesExposure:
		return catSupplyOverdue(core)
	case ImplCampPaidSpendExposure:
		return campSpendExposure(core)
	case ImplCampPaidSpendIncrease:
		return campSpendAbsDelta(core, true)
	case ImplCampPaidSpendDecrease:
		return campSpendAbsDelta(core, false)
	case ImplCampAttrAmountExposure:
		return campAttrExposure(core)
	case ImplCampAttrAmountIncrease:
		return campAttrAbsDelta(core, true)
	case ImplCampAttrAmountDecrease:
		return campAttrAbsDelta(core, false)
	case ImplCampDRRAbsPPChange:
		return campPPChange(core, policy, true)
	case ImplCampCTRAbsPPChange:
		return campPPChange(core, policy, false)
	case ImplCampPaidSpendFromZero:
		return campSpendFromZero(core)
	case ImplCampPaidSpendToZero:
		return campSpendToZero(core)
	default:
		return nil
	}
}

func skuSalesExposure(core CoreFacts, use30d bool) []LaneRank {
	out := make([]LaneRank, 0, len(core.SKUs))
	for _, s := range core.SKUs {
		if !s.SalesAvailable {
			continue
		}
		m := s.OrderedAmountCurrent7
		if use30d {
			m = s.OrderedAmount30D
		}
		if !featureKnownPositive(m) {
			continue
		}
		ex, ok := featureExact(m)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func skuSalesAbsDelta(core CoreFacts, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.SalesAvailable || !s.OrderedAmountCurrent7.IsKnown() || !s.OrderedAmountPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(s.OrderedAmountCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(s.OrderedAmountPrevious7)
		if err != nil {
			continue
		}
		d := subRat(cur, prev)
		if increase && d.Sign() <= 0 {
			continue
		}
		if !increase && d.Sign() >= 0 {
			continue
		}
		metric := d
		if !increase {
			metric = absRat(d)
		}
		exp := maxRat(cur, prev)
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(exp)})
	}
	return out
}

func skuSalesRelDelta(core CoreFacts, policy PolicyV1, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.OrderedAmountCurrent7, core.Account.OrderedAmountPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinSales)
	for _, s := range core.SKUs {
		if !s.SalesAvailable || !featureKnownPositive(s.OrderedAmountPrevious7) {
			continue // FROM_ZERO handled separately; no +∞%
		}
		if !s.OrderedAmountCurrent7.IsKnown() {
			continue
		}
		cur, err := featureRat(s.OrderedAmountCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(s.OrderedAmountPrevious7)
		if err != nil {
			continue
		}
		expAmt := maxRat(cur, prev)
		share := shareOfRat(expAmt, owner)
		if !policy.MeetsRelativeExposure(share, expAmt, absMin) {
			continue
		}
		rel := quoRat(subRat(cur, prev), prev)
		if increase && rel.Sign() <= 0 {
			continue
		}
		if !increase && rel.Sign() >= 0 {
			continue
		}
		metric := rel
		if !increase {
			metric = absRat(rel)
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(expAmt)})
	}
	return out
}

func skuSalesFromZero(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.SalesAvailable {
			continue
		}
		if featureKnownZero(s.OrderedAmountPrevious7) && featureKnownPositive(s.OrderedAmountCurrent7) {
			ex, ok := featureExact(s.OrderedAmountCurrent7)
			if !ok {
				continue
			}
			out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: ex, Exposure: ex})
		}
	}
	return out
}

func skuSalesToZero(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.SalesAvailable {
			continue
		}
		if featureKnownZero(s.OrderedAmountCurrent7) && featureKnownPositive(s.OrderedAmountPrevious7) {
			ex, ok := featureExact(s.OrderedAmountPrevious7)
			if !ok {
				continue
			}
			out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: ex, Exposure: ex})
		}
	}
	return out
}

func skuZeroSellable(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.StockAvailable || !s.SellableStock.IsKnown() {
			continue
		}
		if stock, err := featureRat(s.SellableStock); err != nil || stock.Sign() != 0 {
			continue
		}
		if countKnownNonPositive(s.OrderedUnitsCurrent7) && countKnownNonPositive(s.OrderedUnitsPrevious7) {
			continue
		}
		exp := maxKnownRat(s.OrderedAmountCurrent7, s.OrderedAmountPrevious7)
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(exp), Exposure: formatRat(exp)})
	}
	return out
}

func skuStockCoverage(core CoreFacts, policy PolicyV1) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.OrderedAmountCurrent7, core.Account.OrderedAmountPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinSales)
	for _, s := range core.SKUs {
		if !s.StockAvailable || !s.CoverageDays.IsKnown() {
			continue
		}
		if countKnownNonPositive(s.OrderedUnitsCurrent7) && countKnownNonPositive(s.OrderedUnitsPrevious7) {
			continue
		}
		expAmt := maxKnownRat(s.OrderedAmountCurrent7, s.OrderedAmountPrevious7)
		share := shareOfRat(expAmt, owner)
		if !policy.MeetsRelativeExposure(share, expAmt, absMin) {
			continue
		}
		metric, ok := featureExact(s.CoverageDays)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: metric, Exposure: formatRat(expAmt)})
	}
	return out
}

func skuAdExposure(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.AdsAvailable || !featureKnownPositive(s.PaidSpendCurrent7) {
			continue
		}
		ex, ok := featureExact(s.PaidSpendCurrent7)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func skuAdAbsDelta(core CoreFacts, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.AdsAvailable || !s.PaidSpendCurrent7.IsKnown() || !s.PaidSpendPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(s.PaidSpendCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(s.PaidSpendPrevious7)
		if err != nil {
			continue
		}
		d := subRat(cur, prev)
		if increase && d.Sign() <= 0 {
			continue
		}
		if !increase && d.Sign() >= 0 {
			continue
		}
		metric := d
		if !increase {
			metric = absRat(d)
		}
		exp := maxRat(cur, prev)
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(exp)})
	}
	return out
}

func skuPriceChange(core CoreFacts, policy PolicyV1) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.OrderedAmountCurrent7, core.Account.OrderedAmountPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinSales)
	for _, s := range core.SKUs {
		if !s.PriceAvailable || !s.PriceCurrent.IsKnown() || !s.PricePrevious.IsKnown() || !featureKnownPositive(s.PricePrevious) {
			continue
		}
		expAmt := maxKnownRat(s.OrderedAmountCurrent7, s.OrderedAmountPrevious7)
		share := shareOfRat(expAmt, owner)
		if !policy.MeetsRelativeExposure(share, expAmt, absMin) {
			continue
		}
		cur, err := featureRat(s.PriceCurrent)
		if err != nil {
			continue
		}
		prev, err := featureRat(s.PricePrevious)
		if err != nil {
			continue
		}
		pct := absRat(quoRat(subRat(cur, prev), prev))
		if pct.Sign() == 0 {
			continue
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(pct), Exposure: formatRat(expAmt)})
	}
	return out
}

func skuSearchChange(core CoreFacts, policy PolicyV1) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.OrderedAmountCurrent7, core.Account.OrderedAmountPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinSales)
	for _, s := range core.SKUs {
		if !s.SearchAvailable || !s.SearchPositionCurrent.IsKnown() || !s.SearchPositionPrevious.IsKnown() {
			continue
		}
		expAmt := maxKnownRat(s.OrderedAmountCurrent7, s.OrderedAmountPrevious7)
		share := shareOfRat(expAmt, owner)
		if !policy.MeetsRelativeExposure(share, expAmt, absMin) {
			continue
		}
		cur, err := featureRat(s.SearchPositionCurrent)
		if err != nil {
			continue
		}
		prev, err := featureRat(s.SearchPositionPrevious)
		if err != nil {
			continue
		}
		delta := absRat(subRat(cur, prev))
		if delta.Sign() == 0 {
			continue
		}
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(delta), Exposure: formatRat(expAmt)})
	}
	return out
}

func skuSupplyOverdue(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, s := range core.SKUs {
		if !s.HasSupplyOverdue {
			continue
		}
		exp := maxKnownRat(s.OrderedAmountCurrent7, s.OrderedAmountPrevious7)
		out = append(out, LaneRank{SemanticKey: s.SemanticKey, Metric: formatRat(exp), Exposure: formatRat(exp)})
	}
	return out
}

func catSalesExposure(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Categories {
		if !featureKnownPositive(c.OrderedAmountCurrent7) {
			continue
		}
		ex, ok := featureExact(c.OrderedAmountCurrent7)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func catSalesAbsDelta(core CoreFacts, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Categories {
		if !c.OrderedAmountCurrent7.IsKnown() || !c.OrderedAmountPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(c.OrderedAmountCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(c.OrderedAmountPrevious7)
		if err != nil {
			continue
		}
		d := subRat(cur, prev)
		if increase && d.Sign() <= 0 {
			continue
		}
		if !increase && d.Sign() >= 0 {
			continue
		}
		metric := d
		if !increase {
			metric = absRat(d)
		}
		exp := maxRat(cur, prev)
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(exp)})
	}
	return out
}

func catSalesRelChange(core CoreFacts, policy PolicyV1) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.OrderedAmountCurrent7, core.Account.OrderedAmountPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinSales)
	for _, c := range core.Categories {
		if !featureKnownPositive(c.OrderedAmountPrevious7) || !c.OrderedAmountCurrent7.IsKnown() {
			continue
		}
		cur, err := featureRat(c.OrderedAmountCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(c.OrderedAmountPrevious7)
		if err != nil {
			continue
		}
		expAmt := maxRat(cur, prev)
		share := shareOfRat(expAmt, owner)
		if !policy.MeetsRelativeExposure(share, expAmt, absMin) {
			continue
		}
		rel := absRat(quoRat(subRat(cur, prev), prev))
		if rel.Sign() == 0 {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(rel), Exposure: formatRat(expAmt)})
	}
	return out
}

func catZeroStockSales(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Categories {
		if !featureKnownPositive(c.ZeroStockSalesAmount) {
			continue
		}
		ex, ok := featureExact(c.ZeroStockSalesAmount)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func catAdExposure(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Categories {
		if !featureKnownPositive(c.PaidSpendCurrent7) {
			continue
		}
		ex, ok := featureExact(c.PaidSpendCurrent7)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func catAdChange(core CoreFacts, policy PolicyV1) []LaneRank {
	out := make([]LaneRank, 0)
	owner := maxKnownRat(core.Account.PaidSpendCurrent7, core.Account.PaidSpendPrevious7)
	absMin := ratFromFloat64(policy.ExposureFloors.AbsoluteMinAdSpend)
	for _, c := range core.Categories {
		if !c.PaidSpendCurrent7.IsKnown() || !c.PaidSpendPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(c.PaidSpendCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(c.PaidSpendPrevious7)
		if err != nil {
			continue
		}
		d := absRat(subRat(cur, prev))
		if d.Sign() == 0 {
			continue
		}
		exp := maxRat(cur, prev)
		share := shareOfRat(exp, owner)
		if !policy.MeetsRelativeExposure(share, exp, absMin) {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(d), Exposure: formatRat(exp)})
	}
	return out
}

func catSupplyOverdue(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Categories {
		if !c.HasSupplyOverdue {
			continue
		}
		ex, ok := featureExact(c.SupplyOverdueSales)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func campSpendExposure(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.TrafficAvailable || !featureKnownPositive(c.PaidSpendCurrent7) {
			continue
		}
		ex, ok := featureExact(c.PaidSpendCurrent7)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func campSpendAbsDelta(core CoreFacts, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.TrafficAvailable || !c.PaidSpendCurrent7.IsKnown() || !c.PaidSpendPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(c.PaidSpendCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(c.PaidSpendPrevious7)
		if err != nil {
			continue
		}
		d := subRat(cur, prev)
		if increase && d.Sign() <= 0 {
			continue
		}
		if !increase && d.Sign() >= 0 {
			continue
		}
		metric := d
		if !increase {
			metric = absRat(d)
		}
		exp := maxRat(cur, prev)
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(exp)})
	}
	return out
}

func campAttrExposure(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.AttributionAvailable || !featureKnownPositive(c.AttributedAmountCurrent7) {
			continue
		}
		ex, ok := featureExact(c.AttributedAmountCurrent7)
		if !ok {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
	}
	return out
}

func campAttrAbsDelta(core CoreFacts, increase bool) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.AttributionAvailable || !c.AttributedAmountCurrent7.IsKnown() || !c.AttributedAmountPrevious7.IsKnown() {
			continue
		}
		cur, err := featureRat(c.AttributedAmountCurrent7)
		if err != nil {
			continue
		}
		prev, err := featureRat(c.AttributedAmountPrevious7)
		if err != nil {
			continue
		}
		d := subRat(cur, prev)
		if increase && d.Sign() <= 0 {
			continue
		}
		if !increase && d.Sign() >= 0 {
			continue
		}
		metric := d
		if !increase {
			metric = absRat(d)
		}
		exp := maxRat(cur, prev)
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(metric), Exposure: formatRat(exp)})
	}
	return out
}

func campPPChange(core CoreFacts, policy PolicyV1, drr bool) []LaneRank {
	out := make([]LaneRank, 0)
	minSpend := ratFromFloat64(policy.ExposureFloors.CampaignMetricMinSpend)
	for _, c := range core.Campaigns {
		spend := maxKnownRat(c.PaidSpendCurrent7, c.PaidSpendPrevious7)
		if cmpRat(spend, minSpend) < 0 {
			continue
		}
		var cur, prev MoneyFeature
		if drr {
			if !c.DRRAvailable {
				continue
			}
			cur, prev = c.DRRCurrentPP, c.DRRPreviousPP
		} else {
			if !c.TrafficAvailable {
				continue
			}
			cur, prev = c.CTRCurrentPP, c.CTRPreviousPP
		}
		if !cur.IsKnown() || !prev.IsKnown() {
			continue
		}
		curRat, err := featureRat(cur)
		if err != nil {
			continue
		}
		prevRat, err := featureRat(prev)
		if err != nil {
			continue
		}
		delta := absRat(subRat(curRat, prevRat))
		if delta.Sign() == 0 {
			continue
		}
		out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: formatRat(delta), Exposure: formatRat(spend)})
	}
	return out
}

func campSpendFromZero(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.TrafficAvailable {
			continue
		}
		if featureKnownZero(c.PaidSpendPrevious7) && featureKnownPositive(c.PaidSpendCurrent7) {
			ex, ok := featureExact(c.PaidSpendCurrent7)
			if !ok {
				continue
			}
			out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
		}
	}
	return out
}

func campSpendToZero(core CoreFacts) []LaneRank {
	out := make([]LaneRank, 0)
	for _, c := range core.Campaigns {
		if !c.TrafficAvailable {
			continue
		}
		if featureKnownZero(c.PaidSpendCurrent7) && featureKnownPositive(c.PaidSpendPrevious7) {
			ex, ok := featureExact(c.PaidSpendPrevious7)
			if !ok {
				continue
			}
			out = append(out, LaneRank{SemanticKey: c.SemanticKey, Metric: ex, Exposure: ex})
		}
	}
	return out
}
