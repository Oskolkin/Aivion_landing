package materiality

// AuditExplanation is deterministic audit metadata for a selected (or ranked) entity.
// MUST NEVER be serialized into Landscape Detail or Discovery Pack.
type AuditExplanation struct {
	SemanticKey    string           `json:"semantic_key"`
	EntityKind     EntityKind       `json:"entity_kind"`
	Eligible       bool             `json:"eligible"`
	IsAnchor       bool             `json:"is_anchor"`
	Lanes          []LaneMembership `json:"lanes"`
	FusionUtility  Exact            `json:"fusion_utility"`
	FusionRank     int              `json:"fusion_rank"`
	DiversityRank  int              `json:"diversity_rank"`
	FinalSelected  bool             `json:"final_selected"`
	FinalOrder     int              `json:"final_order,omitempty"`
	ReasonCodes    []string         `json:"reason_codes"`
	DiversityNotes string           `json:"diversity_notes,omitempty"`
}

// ExplainSelection builds structured audit reasons for an entity from ranking + selection.
func ExplainSelection(ranking Ranking, selection Selection, kind EntityKind, semanticKey string) AuditExplanation {
	ex := AuditExplanation{
		SemanticKey: semanticKey,
		EntityKind:  kind,
	}
	var list []RankedEntity
	switch kind {
	case EntityAccount:
		ex.Eligible = true
		ex.IsAnchor = true
		ex.FinalSelected = true
		ex.FinalOrder = selection.Account.SelectionOrder
		ex.ReasonCodes = []string{"ACCOUNT_ALWAYS_INCLUDED"}
		return ex
	case EntityCategory:
		list = ranking.Categories
	case EntitySKU:
		list = ranking.SKUs
	case EntityCampaign:
		list = ranking.Campaigns
	default:
		ex.ReasonCodes = []string{"ENTITY_KIND_NOT_MATERIAL"}
		return ex
	}

	var found *RankedEntity
	for i := range list {
		if list[i].SemanticKey == semanticKey {
			found = &list[i]
			break
		}
	}
	if found == nil {
		ex.ReasonCodes = []string{"NOT_IN_RANKING"}
		return ex
	}
	ex.Eligible = true
	ex.IsAnchor = found.IsAnchor
	ex.Lanes = found.LaneMemberships
	ex.FusionUtility = found.FusionUtility
	ex.FusionRank = found.FusionRank
	ex.DiversityRank = found.DiversityRank
	if found.DiversityRank != found.FusionRank {
		ex.DiversityNotes = "soft diversity adjusted order"
	}
	codes := make([]string, 0, len(found.LaneMemberships)+2)
	if found.IsAnchor {
		codes = append(codes, "SELECTED_AS_ANCHOR")
	}
	for _, m := range found.LaneMemberships {
		codes = append(codes, "LANE_"+m.LaneKey)
	}
	codes = append(codes, "FUSION_RANKED")
	ex.ReasonCodes = codes

	var selList []SelectedEntity
	switch kind {
	case EntityCategory:
		selList = selection.Categories
	case EntitySKU:
		selList = selection.SKUs
	case EntityCampaign:
		selList = selection.Campaigns
	}
	for _, s := range selList {
		if s.SemanticKey == semanticKey {
			ex.FinalSelected = true
			ex.FinalOrder = s.SelectionOrder
			ex.ReasonCodes = append(ex.ReasonCodes, "BUDGET_SELECTED")
			break
		}
	}
	if !ex.FinalSelected {
		ex.ReasonCodes = append(ex.ReasonCodes, "BUDGET_TRUNCATED")
	}
	return ex
}

// AuditSummaryJSON is the compact JSON stored on materiality_selections (not Detail).
func (e AuditExplanation) AuditSummaryJSON() (map[string]any, error) {
	lanes := make([]map[string]any, 0, len(e.Lanes))
	for _, m := range e.Lanes {
		metric, err := formatExact(m.Metric)
		if err != nil {
			return nil, err
		}
		lanes = append(lanes, map[string]any{
			"lane_key": m.LaneKey,
			"rank":     m.Rank,
			"metric":   string(metric),
		})
	}
	fusion, err := formatExact(e.FusionUtility)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"eligible":       e.Eligible,
		"is_anchor":      e.IsAnchor,
		"lanes":          lanes,
		"fusion_utility": string(fusion),
		"fusion_rank":    e.FusionRank,
		"diversity_rank": e.DiversityRank,
		"final_selected": e.FinalSelected,
		"final_order":    e.FinalOrder,
		"reason_codes":   e.ReasonCodes,
	}, nil
}
