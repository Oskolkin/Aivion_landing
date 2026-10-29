package materiality

import (
	"math/big"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// PolicyV1 is the versioned Materiality ranking/eligibility/diversity policy.
// Tuning parameters are pilot defaults — not architecture invariants.
// Runtime money/weight comparisons use big.Rat; Hash() uses Decimal-only payload.
type PolicyV1 struct {
	Version              string             `json:"version"`
	LaneRegistryVersion  string             `json:"lane_registry_version"`
	RRFK                 int                `json:"rrf_k"`
	LaneWeights          map[string]float64 `json:"lane_weights"` // keyed by entity_kind|lane_key
	ExposureFloors       ExposureFloors     `json:"exposure_floors"`
	AnchorCoverageTarget float64            `json:"anchor_coverage_target"`
	AnchorMax            AnchorMax          `json:"anchor_max"`
	Diversity            DiversityParams    `json:"diversity"`
	ContinuityProfile    string             `json:"continuity_profile"`
}

// ExposureFloors gates relative-change and some operational lanes.
type ExposureFloors struct {
	// RelativeShareFloor is min share of owner total (e.g. 0.005 = 0.5%).
	RelativeShareFloor float64 `json:"relative_share_floor"`
	// AbsoluteMinSales is an absolute floor in ordered_amount units.
	AbsoluteMinSales float64 `json:"absolute_min_sales"`
	// AbsoluteMinAdSpend floors advertising relative/metric lanes.
	AbsoluteMinAdSpend float64 `json:"absolute_min_ad_spend"`
	// AbsoluteMinAttributed floors attribution-relative lanes.
	AbsoluteMinAttributed float64 `json:"absolute_min_attributed"`
	// CampaignMetricMinSpend is min current/previous spend for DRR/CTR pp lanes.
	CampaignMetricMinSpend float64 `json:"campaign_metric_min_spend"`
}

// AnchorMax caps cumulative-coverage anchors per kind.
type AnchorMax struct {
	Categories int `json:"categories"`
	SKUs       int `json:"skus"`
	Campaigns  int `json:"campaigns"`
}

// DiversityParams are relative soft penalties (never hard-cap a dominant category).
// V1 used absolute subtractive penalties; V1_1 uses fractions of fusion utility.
type DiversityParams struct {
	MaxPenaltyFraction         float64 `json:"max_penalty_fraction"`
	CategoryRepetitionFraction float64 `json:"category_repetition_fraction"`
	LaneOverlapFraction        float64 `json:"lane_overlap_fraction"`
	NewCategoryBonusFraction   float64 `json:"new_category_bonus_fraction"`
}

// PolicyV1_2 returns the frozen Prompt №9.3 policy seal (immutable).
func PolicyV1_2() PolicyV1 {
	reg := LaneRegistryV1Current()
	weights := make(map[string]float64, len(reg.Versions))
	for _, lv := range reg.Versions {
		weights[laneWeightKey(lv.EntityKind, lv.LaneKey)] = lv.Weight
	}
	return PolicyV1{
		Version:             MaterialityPolicyV1_2,
		LaneRegistryVersion: LaneRegistryV1,
		RRFK:                60,
		LaneWeights:         weights,
		ExposureFloors: ExposureFloors{
			RelativeShareFloor:     0.005,
			AbsoluteMinSales:       1000,
			AbsoluteMinAdSpend:     500,
			AbsoluteMinAttributed:  500,
			CampaignMetricMinSpend: 5000,
		},
		AnchorCoverageTarget: 0.70,
		AnchorMax: AnchorMax{
			Categories: 8,
			SKUs:       20,
			Campaigns:  8,
		},
		Diversity: DiversityParams{
			MaxPenaltyFraction:         0.25,
			CategoryRepetitionFraction: 0.10,
			LaneOverlapFraction:        0.08,
			NewCategoryBonusFraction:   0.04,
		},
		ContinuityProfile: ContinuityInitialNoBaseV1,
	}
}

// CurrentPolicy returns pilot MATERIALITY_POLICY_V1_3 (strict Exact parsing, integer counts).
func CurrentPolicy() PolicyV1 {
	reg := LaneRegistryV2Current()
	weights := make(map[string]float64, len(reg.Versions))
	for _, lv := range reg.Versions {
		weights[laneWeightKey(lv.EntityKind, lv.LaneKey)] = lv.Weight
	}
	return PolicyV1{
		Version:             MaterialityPolicyV1_4,
		LaneRegistryVersion: LaneRegistryV2,
		RRFK:                60,
		LaneWeights:         weights,
		ExposureFloors: ExposureFloors{
			RelativeShareFloor:     0.005, // 0.5% of owner total
			AbsoluteMinSales:       1000,
			AbsoluteMinAdSpend:     500,
			AbsoluteMinAttributed:  500,
			CampaignMetricMinSpend: 5000,
		},
		AnchorCoverageTarget: 0.70,
		AnchorMax: AnchorMax{
			Categories: 8,
			SKUs:       20,
			Campaigns:  8,
		},
		Diversity: DiversityParams{
			MaxPenaltyFraction:         0.25,
			CategoryRepetitionFraction: 0.10,
			LaneOverlapFraction:        0.08,
			NewCategoryBonusFraction:   0.04,
		},
		ContinuityProfile: ContinuityInitialNoBaseV1,
	}
}

func laneWeightKey(kind EntityKind, laneKey string) string {
	return string(kind) + "|" + laneKey
}

// policyHashPayload is the float-free canonical form of PolicyV1.
type policyHashPayload struct {
	Version              string                           `json:"version"`
	LaneRegistryVersion  string                           `json:"lane_registry_version"`
	RRFK                 int                              `json:"rrf_k"`
	LaneWeights          map[string]canonicaljson.Decimal `json:"lane_weights"`
	ExposureFloors       exposureFloorsHash               `json:"exposure_floors"`
	AnchorCoverageTarget canonicaljson.Decimal            `json:"anchor_coverage_target"`
	AnchorMax            AnchorMax                        `json:"anchor_max"`
	Diversity            diversityHash                    `json:"diversity"`
	ContinuityProfile    string                           `json:"continuity_profile"`
}

type exposureFloorsHash struct {
	RelativeShareFloor     canonicaljson.Decimal `json:"relative_share_floor"`
	AbsoluteMinSales       canonicaljson.Decimal `json:"absolute_min_sales"`
	AbsoluteMinAdSpend     canonicaljson.Decimal `json:"absolute_min_ad_spend"`
	AbsoluteMinAttributed  canonicaljson.Decimal `json:"absolute_min_attributed"`
	CampaignMetricMinSpend canonicaljson.Decimal `json:"campaign_metric_min_spend"`
}

type diversityHash struct {
	MaxPenaltyFraction         canonicaljson.Decimal `json:"max_penalty_fraction"`
	CategoryRepetitionFraction canonicaljson.Decimal `json:"category_repetition_fraction"`
	LaneOverlapFraction        canonicaljson.Decimal `json:"lane_overlap_fraction"`
	NewCategoryBonusFraction   canonicaljson.Decimal `json:"new_category_bonus_fraction"`
}

func decF(f float64) Exact {
	return Exact(strconv.FormatFloat(f, 'f', -1, 64))
}

// Hash returns the canonical policy hash (independent of budget).
func (p PolicyV1) Hash() string {
	weights := make(map[string]canonicaljson.Decimal, len(p.LaneWeights))
	for k, w := range p.LaneWeights {
		weights[k] = decF(w)
	}
	payload := policyHashPayload{
		Version:             p.Version,
		LaneRegistryVersion: p.LaneRegistryVersion,
		RRFK:                p.RRFK,
		LaneWeights:         weights,
		ExposureFloors: exposureFloorsHash{
			RelativeShareFloor:     decF(p.ExposureFloors.RelativeShareFloor),
			AbsoluteMinSales:       decF(p.ExposureFloors.AbsoluteMinSales),
			AbsoluteMinAdSpend:     decF(p.ExposureFloors.AbsoluteMinAdSpend),
			AbsoluteMinAttributed:  decF(p.ExposureFloors.AbsoluteMinAttributed),
			CampaignMetricMinSpend: decF(p.ExposureFloors.CampaignMetricMinSpend),
		},
		AnchorCoverageTarget: decF(p.AnchorCoverageTarget),
		AnchorMax:            p.AnchorMax,
		Diversity: diversityHash{
			MaxPenaltyFraction:         decF(p.Diversity.MaxPenaltyFraction),
			CategoryRepetitionFraction: decF(p.Diversity.CategoryRepetitionFraction),
			LaneOverlapFraction:        decF(p.Diversity.LaneOverlapFraction),
			NewCategoryBonusFraction:   decF(p.Diversity.NewCategoryBonusFraction),
		},
		ContinuityProfile: p.ContinuityProfile,
	}
	return canonicaljson.Hash(payload)
}

// LaneWeight returns the configured weight for a lane, defaulting to 1.
func (p PolicyV1) LaneWeight(kind EntityKind, laneKey string) float64 {
	if w, ok := p.LaneWeights[laneWeightKey(kind, laneKey)]; ok {
		return w
	}
	return 1.0
}

// LaneWeightRat returns the lane weight as an exact rational.
func (p PolicyV1) LaneWeightRat(kind EntityKind, laneKey string) *big.Rat {
	return ratFromFloat64(p.LaneWeight(kind, laneKey))
}

// MeetsRelativeExposure is true when share >= floor OR absolute >= absoluteMin.
func (p PolicyV1) MeetsRelativeExposure(share, absolute, absoluteMin *big.Rat) bool {
	min := nonzero(absoluteMin)
	if min.Sign() > 0 && cmpRat(absolute, min) >= 0 {
		return true
	}
	return cmpRat(share, ratFromFloat64(p.ExposureFloors.RelativeShareFloor)) >= 0
}
