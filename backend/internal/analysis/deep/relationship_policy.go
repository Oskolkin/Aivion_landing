package deep

const (
	BasisDescriptive          = "DESCRIPTIVE"
	BasisCoMovement           = "CO_MOVEMENT"
	BasisTemporalAssociation  = "TEMPORAL_ASSOCIATION"
	BasisPlausibleContributor = "PLAUSIBLE_CONTRIBUTOR"
	BasisProviderAttributed   = "PROVIDER_ATTRIBUTED"
	BasisCausalEstimate       = "CAUSAL_ESTIMATE"
)

type RelationshipPolicyV1 struct {
	Version                   string              `json:"version"`
	ForbiddenBases            []string            `json:"forbidden_bases"`
	DefaultDiscoveryBases     []string            `json:"default_discovery_bases"`
	DefaultUnknownEvidence    []string            `json:"default_unknown_evidence_bases"`
	EvidenceTypeBases         map[string][]string `json:"evidence_type_bases"`
	PlausibleContributorPairs []ContributorPair   `json:"plausible_contributor_pairs"`
	ProviderAttributedTypes   []string            `json:"provider_attributed_types"`
}

type ContributorPair struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

func CurrentRelationshipPolicy() RelationshipPolicyV1 {
	timeBearing := []string{BasisDescriptive, BasisCoMovement, BasisTemporalAssociation}
	desc := []string{BasisDescriptive}
	return RelationshipPolicyV1{
		Version:                RelationshipClaimPolicyV1,
		ForbiddenBases:         []string{BasisCausalEstimate},
		DefaultDiscoveryBases:  desc,
		DefaultUnknownEvidence: desc,
		EvidenceTypeBases: map[string][]string{
			"SKU_SALES_HISTORY":            timeBearing,
			"CATEGORY_SALES_HISTORY":       timeBearing,
			"ACCOUNT_SALES_HISTORY":        timeBearing,
			"SKU_PRICE_HISTORY":            timeBearing,
			"SKU_STOCK_HISTORY":            timeBearing,
			"SKU_STOCK_CONTEXT":            desc,
			"SKU_ADVERTISING_HISTORY":      timeBearing,
			"CAMPAIGN_ADVERTISING_HISTORY": timeBearing,
			"SKU_SEARCH_QUERIES":           timeBearing,
			"SKU_SEARCH_HISTORY":           timeBearing,
			"SKU_PROMOTION_HISTORY":        timeBearing,
			"SKU_PROMOTION_CONTEXT":        desc,
			"AD_ATTRIBUTION_DETAIL":        {BasisDescriptive, BasisProviderAttributed},
			"ACCOUNT_FBS_ERROR_POSTINGS":   {BasisDescriptive, BasisProviderAttributed},
		},
		PlausibleContributorPairs: []ContributorPair{
			{Left: "SKU_SALES_HISTORY", Right: "SKU_PRICE_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_STOCK_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_STOCK_CONTEXT"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_ADVERTISING_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "CAMPAIGN_ADVERTISING_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_SEARCH_QUERIES"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_SEARCH_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_PROMOTION_HISTORY"},
			{Left: "SKU_SALES_HISTORY", Right: "SKU_PROMOTION_CONTEXT"},
		},
		ProviderAttributedTypes: []string{"AD_ATTRIBUTION_DETAIL", "ACCOUNT_FBS_ERROR_POSTINGS"},
	}
}

func basesSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func HandleAllowedBases(h FactProjection, items []evidenceItemView, pol RelationshipPolicyV1) map[string]bool {
	if h.SourceKind == "DISCOVERY" {
		return basesSet(pol.DefaultDiscoveryBases)
	}
	typ := ""
	for _, it := range items {
		if it.EvidenceID == h.SourceRef {
			typ = it.EvidenceType
			break
		}
	}
	if typ == "" {
		return basesSet(pol.DefaultUnknownEvidence)
	}
	if xs, ok := pol.EvidenceTypeBases[typ]; ok {
		out := basesSet(xs)
		return out
	}
	return basesSet(pol.DefaultUnknownEvidence)
}

type evidenceItemView struct {
	EvidenceID   string
	EvidenceType string
	TimeBearing  bool
}

func AllowRelationship(basis string, support []FactProjection, items []evidenceItemView, pol RelationshipPolicyV1) error {
	if basis == BasisCausalEstimate {
		return packErr(CodeRelationshipClaimExceeded, CodeCausalEstimate)
	}
	allowed := map[string]bool{BasisDescriptive: true}
	types := map[string]bool{}
	timeOK := false
	for _, h := range support {
		for b := range HandleAllowedBases(h, items, pol) {
			allowed[b] = true
		}
		for _, it := range items {
			if it.EvidenceID == h.SourceRef {
				types[it.EvidenceType] = true
				if it.TimeBearing {
					timeOK = true
				}
			}
		}
	}
	if basis == BasisCoMovement || basis == BasisTemporalAssociation {
		if !timeOK && !hasTimeType(types, pol) {
			return packErr(CodeRelationshipClaimExceeded, basis+":not_time_bearing")
		}
	}
	if basis == BasisPlausibleContributor {
		if !plausibleOK(types, pol) {
			return packErr(CodeRelationshipClaimExceeded, "PLAUSIBLE_CONTRIBUTOR unapproved pair")
		}
	}
	if basis == BasisProviderAttributed {
		ok := false
		for _, t := range pol.ProviderAttributedTypes {
			if types[t] {
				ok = true
			}
		}
		if !ok {
			return packErr(CodeRelationshipClaimExceeded, "PROVIDER_ATTRIBUTED requires attribution evidence")
		}
	}
	if !allowed[basis] && basis != BasisPlausibleContributor && basis != BasisProviderAttributed {
		return packErr(CodeRelationshipClaimExceeded, basis+" not allowed for sources")
	}
	if basis == BasisPlausibleContributor || basis == BasisProviderAttributed {
		return nil
	}
	if !allowed[basis] {
		return packErr(CodeRelationshipClaimExceeded, basis)
	}
	return nil
}

func hasTimeType(types map[string]bool, pol RelationshipPolicyV1) bool {
	for t := range types {
		for _, b := range pol.EvidenceTypeBases[t] {
			if b == BasisTemporalAssociation || b == BasisCoMovement {
				return true
			}
		}
	}
	return false
}

func plausibleOK(types map[string]bool, pol RelationshipPolicyV1) bool {
	for _, p := range pol.PlausibleContributorPairs {
		if types[p.Left] && types[p.Right] {
			return true
		}
		if types[p.Right] && types[p.Left] {
			return true
		}
	}
	return false
}
