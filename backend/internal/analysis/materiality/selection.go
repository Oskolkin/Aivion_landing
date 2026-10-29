package materiality

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// SelectedEntity is one entity in the final Materiality Selection.
type SelectedEntity struct {
	SemanticKey      string     `json:"semantic_key"`
	CanonicalRefID   string     `json:"canonical_ref_id,omitempty"`
	EntityKind       EntityKind `json:"entity_kind"`
	SelectionOrder   int        `json:"selection_order"`
	SelectedAsAnchor bool       `json:"selected_as_anchor"`
}

// Selection is Phase B output after applying a BudgetProfile.
type Selection struct {
	Account       SelectedEntity   `json:"account"`
	Categories    []SelectedEntity `json:"categories"`
	SKUs          []SelectedEntity `json:"skus"`
	Campaigns     []SelectedEntity `json:"campaigns"`
	RankingHash   string           `json:"ranking_hash"`
	BudgetVersion string           `json:"budget_version"`
	BudgetHash    string           `json:"budget_hash"`
	SelectionHash string           `json:"selection_hash"`
	PolicyVersion string           `json:"policy_version"`
	PolicyHash    string           `json:"policy_hash"`
}

// SelectMaterialEntities applies budget truncation to a Ranking.
// RankingHash is unchanged when only the budget changes; SelectionHash includes budget.
func SelectMaterialEntities(ranking Ranking, budget BudgetProfile, account AccountFeatures) Selection {
	sel := Selection{
		Account: SelectedEntity{
			SemanticKey:      account.SemanticKey,
			CanonicalRefID:   account.CanonicalRefID,
			EntityKind:       EntityAccount,
			SelectionOrder:   1,
			SelectedAsAnchor: true,
		},
		RankingHash:   ranking.RankingHash,
		BudgetVersion: budget.Version,
		BudgetHash:    budget.Hash(),
		PolicyVersion: ranking.PolicyVersion,
		PolicyHash:    ranking.PolicyHash,
	}
	if sel.Account.SemanticKey == "" {
		sel.Account.SemanticKey = "account"
	}
	sel.Categories = selectKind(ranking.Categories, budget.MaxCategories, budget.AnchorMaxCategories)
	sel.SKUs = selectKind(ranking.SKUs, budget.MaxSKUs, budget.AnchorMaxSKUs)
	sel.Campaigns = selectKind(ranking.Campaigns, budget.MaxCampaigns, budget.AnchorMaxCampaigns)
	sel.SelectionHash = hashSelection(sel)
	return sel
}

func selectKind(ranked []RankedEntity, maxN, anchorMax int) []SelectedEntity {
	if maxN <= 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]SelectedEntity, 0, maxN)

	// Anchors first (capped), preserving diversity/anchor order from ranking.
	anchorCount := 0
	for _, e := range ranked {
		if !e.IsAnchor {
			continue
		}
		if anchorMax > 0 && anchorCount >= anchorMax {
			continue
		}
		if seen[e.SemanticKey] {
			continue
		}
		anchorCount++
		seen[e.SemanticKey] = true
		out = append(out, SelectedEntity{
			SemanticKey: e.SemanticKey, CanonicalRefID: e.CanonicalRefID,
			EntityKind: e.EntityKind, SelectionOrder: len(out) + 1, SelectedAsAnchor: true,
		})
		if len(out) >= maxN {
			return out
		}
	}
	for _, e := range ranked {
		if seen[e.SemanticKey] {
			continue
		}
		seen[e.SemanticKey] = true
		out = append(out, SelectedEntity{
			SemanticKey: e.SemanticKey, CanonicalRefID: e.CanonicalRefID,
			EntityKind: e.EntityKind, SelectionOrder: len(out) + 1, SelectedAsAnchor: e.IsAnchor,
		})
		if len(out) >= maxN {
			break
		}
	}
	return out
}

func hashSelection(s Selection) string {
	type slim struct {
		RankingHash string   `json:"ranking_hash"`
		BudgetHash  string   `json:"budget_hash"`
		Account     string   `json:"account"`
		Categories  []string `json:"categories"`
		SKUs        []string `json:"skus"`
		Campaigns   []string `json:"campaigns"`
	}
	x := slim{
		RankingHash: s.RankingHash,
		BudgetHash:  s.BudgetHash,
		Account:     s.Account.SemanticKey,
	}
	for _, e := range s.Categories {
		x.Categories = append(x.Categories, e.SemanticKey)
	}
	for _, e := range s.SKUs {
		x.SKUs = append(x.SKUs, e.SemanticKey)
	}
	for _, e := range s.Campaigns {
		x.Campaigns = append(x.Campaigns, e.SemanticKey)
	}
	return canonicaljson.Hash(x)
}
