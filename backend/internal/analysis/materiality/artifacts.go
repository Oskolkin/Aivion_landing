package materiality

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"

// RankingArtifact is the immutable MinIO JSON for full diversified rankings (audit).
// Schema: materiality_ranking_v1. Never embed in Landscape Detail.
// Float metrics are stored as Decimal strings for canonicaljson compliance.
type RankingArtifact struct {
	SchemaVersion       string                 `json:"schema_version"`
	PolicyVersion       string                 `json:"policy_version"`
	PolicyHash          string                 `json:"policy_hash"`
	LaneRegistryVersion string                 `json:"lane_registry_version"`
	InputCoreHash       string                 `json:"input_core_hash"`
	RankingHash         string                 `json:"ranking_hash"`
	Categories          []RankedEntityArtifact `json:"categories"`
	SKUs                []RankedEntityArtifact `json:"skus"`
	Campaigns           []RankedEntityArtifact `json:"campaigns"`
}

// RankedEntityArtifact is the float-free serialization of RankedEntity.
type RankedEntityArtifact struct {
	SemanticKey     string                   `json:"semantic_key"`
	CanonicalRefID  string                   `json:"canonical_ref_id,omitempty"`
	EntityKind      EntityKind               `json:"entity_kind"`
	CategoryKey     string                   `json:"category_key,omitempty"`
	LaneMemberships []LaneMembershipArtifact `json:"lane_memberships"`
	LaneFingerprint []string                 `json:"lane_fingerprint"`
	FusionUtility   Exact                    `json:"fusion_utility"`
	FusionRank      int                      `json:"fusion_rank"`
	DiversityRank   int                      `json:"diversity_rank"`
	IsAnchor        bool                     `json:"is_anchor"`
	AnchorOrder     int                      `json:"anchor_order,omitempty"`
	ExposureMetric  Exact                    `json:"exposure_metric"`
}

// LaneMembershipArtifact is the float-free form of LaneMembership.
type LaneMembershipArtifact struct {
	LaneKey string `json:"lane_key"`
	Rank    int    `json:"rank"`
	Metric  Exact  `json:"metric"`
}

// SelectionArtifact is the immutable MinIO JSON for final selected EntityRefs.
// Schema: materiality_selection_v1. Audit reasons stay in DB / ranking artifact, not Detail.
type SelectionArtifact struct {
	SchemaVersion string           `json:"schema_version"`
	PolicyVersion string           `json:"policy_version"`
	PolicyHash    string           `json:"policy_hash"`
	BudgetVersion string           `json:"budget_version"`
	BudgetHash    string           `json:"budget_hash"`
	RankingHash   string           `json:"ranking_hash"`
	SelectionHash string           `json:"selection_hash"`
	Account       SelectedEntity   `json:"account"`
	Categories    []SelectedEntity `json:"categories"`
	SKUs          []SelectedEntity `json:"skus"`
	Campaigns     []SelectedEntity `json:"campaigns"`
}

// NewRankingArtifact wraps a Ranking for artifact persistence.
func NewRankingArtifact(r Ranking) (RankingArtifact, error) {
	cats, err := toRankedArtifacts(r.Categories)
	if err != nil {
		return RankingArtifact{}, err
	}
	skus, err := toRankedArtifacts(r.SKUs)
	if err != nil {
		return RankingArtifact{}, err
	}
	camps, err := toRankedArtifacts(r.Campaigns)
	if err != nil {
		return RankingArtifact{}, err
	}
	return RankingArtifact{
		SchemaVersion:       ArtifactSchemaRankingV1,
		PolicyVersion:       r.PolicyVersion,
		PolicyHash:          r.PolicyHash,
		LaneRegistryVersion: r.LaneRegistryVersion,
		InputCoreHash:       r.CoreHash,
		RankingHash:         r.RankingHash,
		Categories:          cats,
		SKUs:                skus,
		Campaigns:           camps,
	}, nil
}

func toRankedArtifacts(in []RankedEntity) ([]RankedEntityArtifact, error) {
	out := make([]RankedEntityArtifact, 0, len(in))
	for _, e := range in {
		lanes := make([]LaneMembershipArtifact, 0, len(e.LaneMemberships))
		for _, m := range e.LaneMemberships {
			metric, err := formatExact(m.Metric)
			if err != nil {
				return nil, err
			}
			lanes = append(lanes, LaneMembershipArtifact{
				LaneKey: m.LaneKey, Rank: m.Rank, Metric: metric,
			})
		}
		fusion, err := formatExact(e.FusionUtility)
		if err != nil {
			return nil, err
		}
		exposure, err := formatExact(e.ExposureMetric)
		if err != nil {
			return nil, err
		}
		out = append(out, RankedEntityArtifact{
			SemanticKey: e.SemanticKey, CanonicalRefID: e.CanonicalRefID, EntityKind: e.EntityKind,
			CategoryKey: e.CategoryKey, LaneMemberships: lanes, LaneFingerprint: e.LaneFingerprint,
			FusionUtility: fusion, FusionRank: e.FusionRank, DiversityRank: e.DiversityRank,
			IsAnchor: e.IsAnchor, AnchorOrder: e.AnchorOrder, ExposureMetric: exposure,
		})
	}
	return out, nil
}

// NewSelectionArtifact wraps a Selection for artifact persistence.
func NewSelectionArtifact(s Selection) SelectionArtifact {
	return SelectionArtifact{
		SchemaVersion: ArtifactSchemaSelectionV1,
		PolicyVersion: s.PolicyVersion,
		PolicyHash:    s.PolicyHash,
		BudgetVersion: s.BudgetVersion,
		BudgetHash:    s.BudgetHash,
		RankingHash:   s.RankingHash,
		SelectionHash: s.SelectionHash,
		Account:       s.Account,
		Categories:    s.Categories,
		SKUs:          s.SKUs,
		Campaigns:     s.Campaigns,
	}
}

// FormatFusionUtility returns a stable decimal string for audit tooling.
func FormatFusionUtility(u Exact) (string, error) {
	ex, err := formatExact(u)
	return string(ex), err
}

// RecomputeRankingHash returns the canonical hash of a ranking artifact body (RankingHash cleared).
func RecomputeRankingHash(art RankingArtifact) string {
	art.RankingHash = ""
	return canonicaljson.Hash(art)
}

// RecomputeSelectionHash returns the canonical selection hash from a selection artifact.
func RecomputeSelectionHash(art SelectionArtifact) string {
	type slim struct {
		RankingHash string   `json:"ranking_hash"`
		BudgetHash  string   `json:"budget_hash"`
		Account     string   `json:"account"`
		Categories  []string `json:"categories"`
		SKUs        []string `json:"skus"`
		Campaigns   []string `json:"campaigns"`
	}
	x := slim{
		RankingHash: art.RankingHash,
		BudgetHash:  art.BudgetHash,
		Account:     art.Account.SemanticKey,
	}
	for _, e := range art.Categories {
		x.Categories = append(x.Categories, e.SemanticKey)
	}
	for _, e := range art.SKUs {
		x.SKUs = append(x.SKUs, e.SemanticKey)
	}
	for _, e := range art.Campaigns {
		x.Campaigns = append(x.Campaigns, e.SemanticKey)
	}
	return canonicaljson.Hash(x)
}
