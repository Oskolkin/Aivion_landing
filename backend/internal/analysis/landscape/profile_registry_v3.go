package landscape

import (
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// BlockMode is the executable contract mode for one V3 profile block (Prompt №9.4 §10).
type BlockMode string

const (
	BlockRequired                BlockMode = "REQUIRED"
	BlockOptionalCapability      BlockMode = "OPTIONAL_CAPABILITY"
	BlockDerived                 BlockMode = "DERIVED"
	BlockNotSupportedBySemantics BlockMode = "NOT_SUPPORTED_BY_SEMANTICS"
)

// CapabilityMatch governs multi-capability block readiness (Prompt №9.4 §10).
type CapabilityMatch string

const (
	CapabilityAll CapabilityMatch = "ALL"
	CapabilityAny CapabilityMatch = "ANY"
)

// SerializationPolicy controls Landscape serialization for one block (Prompt №9.4 §10).
type SerializationPolicy string

const (
	SerializeBlockState                 SerializationPolicy = "SERIALIZE_BLOCK_STATE"
	OmitWhenGlobalCapabilityUnavailable SerializationPolicy = "OMIT_WHEN_GLOBAL_CAPABILITY_UNAVAILABLE"
	NeverSerialize                      SerializationPolicy = "NEVER_SERIALIZE"
)

// PeriodAnchor selects the temporal anchor for a profile period (Prompt №9.4 §10).
type PeriodAnchor string

const (
	PeriodAnchorAnalysisDate         PeriodAnchor = "ANALYSIS_DATE"
	PeriodAnchorFactFamilyCoverageTo PeriodAnchor = "FACT_FAMILY_COVERAGE_TO"
	PeriodAnchorSnapshot             PeriodAnchor = "SNAPSHOT_AS_OF"
)

// PeriodSpec is one source-native or analysis-anchored period in a V3 block contract.
type PeriodSpec struct {
	Name       string       `json:"name"`
	Anchor     PeriodAnchor `json:"anchor"`
	FactFamily string       `json:"fact_family,omitempty"`
}

// SupportingSpec seals bounded Detail supporting-context policy (Prompt №9.4 §10/§16).
type SupportingSpec struct {
	Limit         int        `json:"limit"`
	SortBasis     string     `json:"sort_basis"`
	SemanticScope string     `json:"semantic_scope"`
	Period        PeriodSpec `json:"period"`
}

// BlockSpecV3 is one executable factual block in a Landscape V3 profile.
type BlockSpecV3 struct {
	BlockID string `json:"block_id"`

	Mode BlockMode `json:"mode"`

	CapabilityKeys  []string        `json:"capability_keys,omitempty"`
	CapabilityMatch CapabilityMatch `json:"capability_match,omitempty"`

	FactFamilies []string `json:"fact_families,omitempty"`

	FactualMethods []string `json:"factual_methods,omitempty"`

	Periods []PeriodSpec `json:"periods,omitempty"`

	FormulaPolicy string `json:"formula_policy,omitempty"`

	Serialization SerializationPolicy `json:"serialization"`

	Supporting *SupportingSpec `json:"supporting,omitempty"`
}

// ExecutableProfileV3 is a versioned, hashable Landscape profile registry entry (V3).
type ExecutableProfileV3 struct {
	Name          string        `json:"name"`
	SchemaVersion string        `json:"schema_version"`
	Blocks        []BlockSpecV3 `json:"blocks"`
}

// CoreProfileBundleV3 holds the four Core entity profiles used by BUILD_LANDSCAPE_CORE (V3).
type CoreProfileBundleV3 struct {
	Account  ExecutableProfileV3
	Category ExecutableProfileV3
	SKU      ExecutableProfileV3
	Campaign ExecutableProfileV3
}

// DetailProfileBundleV3 holds the four Detail entity profiles used by BUILD_LANDSCAPE_DETAIL (V3).
type DetailProfileBundleV3 struct {
	Account  ExecutableProfileV3
	Category ExecutableProfileV3
	SKU      ExecutableProfileV3
	Campaign ExecutableProfileV3
}

func DefaultCoreProfilesV3() CoreProfileBundleV3 {
	return CoreProfileBundleV3{
		Account:  AccountDiscoveryV3(),
		Category: CategoryCoreV3(),
		SKU:      SKUCoreV3(),
		Campaign: CampaignCoreV3(),
	}
}

func DefaultDetailProfilesV3() DetailProfileBundleV3 {
	return DetailProfileBundleV3{
		Account:  AccountDiscoveryV3(),
		Category: CategoryDetailV3(),
		SKU:      SKUDetailV3(),
		Campaign: CampaignDetailV3(),
	}
}

func (b CoreProfileBundleV3) IsZero() bool {
	return b.Account.Name == "" && b.Category.Name == "" && b.SKU.Name == "" && b.Campaign.Name == ""
}

// ProfileHash returns the canonical SHA-256 hex digest of the full V3 semantic contract.
func (p ExecutableProfileV3) ProfileHash() string {
	return canonicaljson.Hash(p.canonicalPayload())
}

// Supporting returns the supporting-context spec for a Detail block, if declared.
func (p ExecutableProfileV3) Supporting(blockID string) (SupportingSpec, bool) {
	spec, ok := p.blockByID(blockID)
	if !ok || spec.Supporting == nil {
		return SupportingSpec{}, false
	}
	return *spec.Supporting, true
}

func (p ExecutableProfileV3) HasBlock(blockID string) bool {
	_, ok := p.blockByID(blockID)
	return ok
}

func (p ExecutableProfileV3) blockByID(blockID string) (BlockSpecV3, bool) {
	for _, b := range p.Blocks {
		if b.BlockID == blockID {
			return b, true
		}
	}
	return BlockSpecV3{}, false
}

func (p ExecutableProfileV3) canonicalPayload() map[string]any {
	blocks := make([]map[string]any, len(p.Blocks))
	for i, b := range p.Blocks {
		blocks[i] = blockSpecV3HashPayload(b)
	}
	return map[string]any{
		"name":           p.Name,
		"schema_version": p.SchemaVersion,
		"blocks":         blocks,
	}
}

func blockSpecV3HashPayload(b BlockSpecV3) map[string]any {
	capKeys := append([]string(nil), b.CapabilityKeys...)
	sort.Strings(capKeys)
	families := append([]string(nil), b.FactFamilies...)
	sort.Strings(families)
	methods := append([]string(nil), b.FactualMethods...)
	sort.Strings(methods)
	periods := append([]PeriodSpec(nil), b.Periods...)
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].Name != periods[j].Name {
			return periods[i].Name < periods[j].Name
		}
		if periods[i].Anchor != periods[j].Anchor {
			return periods[i].Anchor < periods[j].Anchor
		}
		return periods[i].FactFamily < periods[j].FactFamily
	})
	periodMaps := make([]map[string]any, len(periods))
	for i, ps := range periods {
		periodMaps[i] = periodSpecHashPayload(ps)
	}
	payload := map[string]any{
		"block_id":         b.BlockID,
		"mode":             b.Mode,
		"capability_keys":  capKeys,
		"capability_match": b.CapabilityMatch,
		"fact_families":    families,
		"factual_methods":  methods,
		"periods":          periodMaps,
		"formula_policy":   b.FormulaPolicy,
		"serialization":    b.Serialization,
	}
	if b.Supporting != nil {
		payload["supporting"] = supportingSpecHashPayload(*b.Supporting)
	}
	return payload
}

func periodSpecHashPayload(p PeriodSpec) map[string]any {
	return map[string]any{
		"name":        p.Name,
		"anchor":      p.Anchor,
		"fact_family": p.FactFamily,
	}
}

func supportingSpecHashPayload(s SupportingSpec) map[string]any {
	return map[string]any{
		"limit":          s.Limit,
		"sort_basis":     s.SortBasis,
		"semantic_scope": s.SemanticScope,
		"period":         periodSpecHashPayload(s.Period),
	}
}

func sealExecutableProfileV3(p ExecutableProfileV3) string {
	return p.Name + "#" + p.ProfileHash()
}

// CoreProfileVersionSealV3 seals Core profiles as NAME#hash tuples (Prompt №9.4).
func CoreProfileVersionSealV3() string {
	b := DefaultCoreProfilesV3()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

// DetailProfileVersionSealV3 seals Detail profiles as NAME#hash tuples (Prompt №9.4).
func DetailProfileVersionSealV3() string {
	b := DefaultDetailProfilesV3()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

func periodSpec(name string, anchor PeriodAnchor, factFamily ...string) PeriodSpec {
	ps := PeriodSpec{Name: name, Anchor: anchor}
	if len(factFamily) > 0 {
		ps.FactFamily = factFamily[0]
	}
	return ps
}

func sourceNativePeriod(name, factFamily string) PeriodSpec {
	return periodSpec(name, PeriodAnchorFactFamilyCoverageTo, factFamily)
}

func analysisPeriod(name string) PeriodSpec {
	return periodSpec(name, PeriodAnchorAnalysisDate)
}

func snapshotPeriod() PeriodSpec {
	return periodSpec("SNAPSHOT", PeriodAnchorSnapshot)
}

func requiredBlock(
	blockID string,
	factFamilies, factualMethods, capabilityKeys []string,
	periods ...PeriodSpec,
) BlockSpecV3 {
	return BlockSpecV3{
		BlockID:         blockID,
		Mode:            BlockRequired,
		FactFamilies:    factFamilies,
		FactualMethods:  factualMethods,
		CapabilityKeys:  capabilityKeys,
		CapabilityMatch: CapabilityAll,
		Periods:         periods,
		Serialization:   SerializeBlockState,
	}
}

func optionalBlock(
	blockID string,
	factFamilies, factualMethods, capabilityKeys []string,
	capMatch CapabilityMatch,
	periods ...PeriodSpec,
) BlockSpecV3 {
	if capMatch == "" {
		capMatch = CapabilityAll
	}
	return BlockSpecV3{
		BlockID:         blockID,
		Mode:            BlockOptionalCapability,
		FactFamilies:    factFamilies,
		FactualMethods:  factualMethods,
		CapabilityKeys:  capabilityKeys,
		CapabilityMatch: capMatch,
		Periods:         periods,
		Serialization:   OmitWhenGlobalCapabilityUnavailable,
	}
}

func derivedBlockV3(blockID string, factFamilies ...string) BlockSpecV3 {
	return BlockSpecV3{
		BlockID:       blockID,
		Mode:          BlockDerived,
		FactFamilies:  factFamilies,
		Serialization: SerializeBlockState,
	}
}

func notSupportedBlockV3(blockID string) BlockSpecV3 {
	return BlockSpecV3{
		BlockID:       blockID,
		Mode:          BlockNotSupportedBySemantics,
		Serialization: NeverSerialize,
	}
}

func supportingBlock(
	blockID string,
	factFamilies, factualMethods, capabilityKeys []string,
	support SupportingSpec,
	periods ...PeriodSpec,
) BlockSpecV3 {
	return BlockSpecV3{
		BlockID:         blockID,
		Mode:            BlockOptionalCapability,
		FactFamilies:    factFamilies,
		FactualMethods:  factualMethods,
		CapabilityKeys:  capabilityKeys,
		CapabilityMatch: CapabilityAll,
		Periods:         periods,
		Serialization:   OmitWhenGlobalCapabilityUnavailable,
		Supporting:      &support,
	}
}

// AccountDiscoveryV3 is the normative Account Discovery profile (Prompt №9.4 §51).
func AccountDiscoveryV3() ExecutableProfileV3 {
	salesPeriods := []PeriodSpec{
		sourceNativePeriod("CURRENT_7D", "SALES.DEMAND"),
		sourceNativePeriod("PREVIOUS_7D", "SALES.DEMAND"),
		sourceNativePeriod("TREND_30D", "SALES.DEMAND"),
		analysisPeriod("ANALYSIS_DAY"),
		analysisPeriod("PREVIOUS_DAY"),
	}
	return ExecutableProfileV3{
		Name:          ProfileAccountDiscoveryV3,
		SchemaVersion: LandscapeSchemaV3,
		Blocks: []BlockSpecV3{
			requiredBlock(
				"account.sales",
				[]string{"SALES.DEMAND"},
				[]string{"GetAccountDemand"},
				[]string{"SALES.ACCOUNT_DEMAND"},
				salesPeriods...,
			),
			derivedBlockV3("account.assortment"),
			optionalBlock(
				"account.stock",
				[]string{"STOCK.SNAPSHOT"},
				nil,
				[]string{"STOCK.SKU_CURRENT"},
				CapabilityAll,
			),
			optionalBlock(
				"account.price",
				[]string{"PRICE.SNAPSHOT"},
				nil,
				[]string{"PRICE.CUSTOMER_PRICE", "PRICE.HISTORY"},
				CapabilityAny,
			),
			optionalBlock(
				"account.advertising",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"GetAdvertisingAccountSummary"},
				[]string{"ADVERTISING.ACCOUNT_SPEND"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
			),
			optionalBlock(
				"account.funnel",
				[]string{"FUNNEL.DAILY_ADDITIVE"},
				[]string{"GetAccountFunnelPeriod"},
				[]string{"FUNNEL.ACCOUNT"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "FUNNEL.DAILY_ADDITIVE"),
				sourceNativePeriod("PREVIOUS_7D", "FUNNEL.DAILY_ADDITIVE"),
			),
			optionalBlock(
				"account.search",
				[]string{"SEARCH.SELLER_SUMMARY"},
				[]string{"BatchGetSKUSearchSummary"},
				[]string{"SEARCH.SELLER_SUMMARY"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "SEARCH.SELLER_SUMMARY"),
				sourceNativePeriod("PREVIOUS_7D", "SEARCH.SELLER_SUMMARY"),
			),
			optionalBlock(
				"account.finance",
				[]string{"FINANCE.DAILY_LEDGER"},
				[]string{"GetFinanceAccountSummary", "GetFinanceAccountBreakdown"},
				[]string{"FINANCE.ACCOUNT_NET"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "FINANCE.DAILY_LEDGER"),
				sourceNativePeriod("PREVIOUS_7D", "FINANCE.DAILY_LEDGER"),
			),
			optionalBlock(
				"account.promotions",
				[]string{"PROMOTIONS.DEFINITIONS"},
				[]string{"GetPromotionAccountSummary"},
				[]string{"PROMOTIONS.DEFINITIONS"},
				CapabilityAll,
			),
			optionalBlock(
				"account.health",
				[]string{"ACCOUNT_HEALTH.CURRENT_SNAPSHOT"},
				[]string{"GetAccountHealthSummary"},
				[]string{"HEALTH.CURRENT_RATINGS"},
				CapabilityAll,
				snapshotPeriod(),
			),
			optionalBlock(
				"account.supply",
				[]string{"SUPPLY.STATE"},
				[]string{"GetSupplyAccountSummary"},
				[]string{"SUPPLY.STATE"},
				CapabilityAll,
				snapshotPeriod(),
			),
			derivedBlockV3("account.concentration"),
		},
	}
}

// CategoryCoreV3 is the normative Category Core profile (Prompt №9.4 §68).
func CategoryCoreV3() ExecutableProfileV3 {
	return ExecutableProfileV3{
		Name:          ProfileCategoryCoreV3,
		SchemaVersion: LandscapeSchemaV3,
		Blocks: []BlockSpecV3{
			requiredBlock(
				"category.sales",
				[]string{"SALES.DEMAND"},
				[]string{"ListCategoryDemand"},
				[]string{"SALES.CATEGORY_DEMAND"},
				sourceNativePeriod("CURRENT_7D", "SALES.DEMAND"),
				sourceNativePeriod("PREVIOUS_7D", "SALES.DEMAND"),
			),
			optionalBlock(
				"category.stock",
				[]string{"STOCK.SNAPSHOT"},
				nil,
				[]string{"STOCK.SKU_CURRENT"},
				CapabilityAll,
				snapshotPeriod(),
			),
			optionalBlock(
				"category.price",
				[]string{"PRICE.SNAPSHOT"},
				nil,
				[]string{"PRICE.CUSTOMER_PRICE"},
				CapabilityAll,
				snapshotPeriod(),
			),
			optionalBlock(
				"category.advertising",
				[]string{"ADVERTISING.TRAFFIC"},
				nil,
				[]string{"ADVERTISING.SKU_TRAFFIC"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
			),
			optionalBlock(
				"category.supply",
				[]string{"SUPPLY.STATE"},
				nil,
				[]string{"SUPPLY.STATE"},
				CapabilityAll,
				snapshotPeriod(),
			),
		},
	}
}

// CategoryDetailV3 extends Category Core with Detail-only blocks (Prompt №9.4 §72).
func CategoryDetailV3() ExecutableProfileV3 {
	p := CategoryCoreV3()
	p.Name = ProfileCategoryDetailV3
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		optionalBlock(
			"category.funnel",
			[]string{"FUNNEL.DAILY_ADDITIVE"},
			[]string{"GetCategoryFunnelPeriod"},
			[]string{"FUNNEL.CATEGORY_ADDITIVE"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "FUNNEL.DAILY_ADDITIVE"),
		),
		optionalBlock(
			"category.search",
			[]string{"SEARCH.SELLER_SUMMARY"},
			[]string{"GetCategorySearchSummary"},
			[]string{"SEARCH.CATEGORY_SUMMARY"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "SEARCH.SELLER_SUMMARY"),
		),
		optionalBlock(
			"category.promotions",
			[]string{"PROMOTIONS.MEMBERSHIP"},
			[]string{"ListPromotions"},
			[]string{"PROMOTIONS.MEMBERSHIP"},
			CapabilityAll,
		),
		notSupportedBlockV3("category.finance"),
	)
	return p
}

// SKUCoreV3 is the normative SKU Core profile (Prompt №9.4 §75).
func SKUCoreV3() ExecutableProfileV3 {
	salesPeriods := []PeriodSpec{
		sourceNativePeriod("CURRENT_7D", "SALES.DEMAND"),
		sourceNativePeriod("PREVIOUS_7D", "SALES.DEMAND"),
		analysisPeriod("ANALYSIS_DAY"),
		analysisPeriod("PREVIOUS_DAY"),
		sourceNativePeriod("TREND_30D", "SALES.DEMAND"),
	}
	return ExecutableProfileV3{
		Name:          ProfileSKUCoreV3,
		SchemaVersion: LandscapeSchemaV3,
		Blocks: []BlockSpecV3{
			requiredBlock(
				"sku.sales",
				[]string{"SALES.DEMAND"},
				[]string{"BatchGetSKUDemand", "BatchGetSKUTrendContext"},
				[]string{"SALES.SKU_DEMAND"},
				salesPeriods...,
			),
			requiredBlock(
				"sku.stock",
				[]string{"STOCK.SNAPSHOT"},
				[]string{"BatchGetSKUStock"},
				[]string{"STOCK.SKU_CURRENT"},
				snapshotPeriod(),
			),
			optionalBlock(
				"sku.stock.coverage",
				[]string{"STOCK.SNAPSHOT"},
				[]string{"BatchGetStockCoverage"},
				[]string{"STOCK.COVERAGE_7D"},
				CapabilityAll,
				snapshotPeriod(),
			),
			requiredBlock(
				"sku.price",
				[]string{"PRICE.SNAPSHOT"},
				[]string{"BatchGetSKUPrice"},
				[]string{"PRICE.CUSTOMER_PRICE"},
				snapshotPeriod(),
			),
			optionalBlock(
				"sku.price.history",
				[]string{"PRICE.SNAPSHOT"},
				[]string{"BatchGetSKUPriceLastObservedChange"},
				[]string{"PRICE.HISTORY"},
				CapabilityAll,
				snapshotPeriod(),
			),
			optionalBlock(
				"sku.advertising",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchGetSKUAdvertising"},
				[]string{"ADVERTISING.SKU_TRAFFIC"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
			),
			optionalBlock(
				"sku.search",
				[]string{"SEARCH.SELLER_SUMMARY"},
				[]string{"BatchGetSKUSearchSummary"},
				[]string{"SEARCH.SELLER_SUMMARY"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "SEARCH.SELLER_SUMMARY"),
				sourceNativePeriod("PREVIOUS_7D", "SEARCH.SELLER_SUMMARY"),
			),
			optionalBlock(
				"sku.promotions",
				[]string{"PROMOTIONS.MEMBERSHIP"},
				[]string{"BatchGetSKUPromotionSummary"},
				[]string{"PROMOTIONS.MEMBERSHIP"},
				CapabilityAll,
				snapshotPeriod(),
			),
			optionalBlock(
				"sku.supply",
				[]string{"SUPPLY.STATE"},
				[]string{"BatchGetSKUInbound"},
				[]string{"SUPPLY.SKU_INBOUND"},
				CapabilityAll,
				snapshotPeriod(),
			),
		},
	}
}

// SKUDetailV3 extends SKU Core with bounded supporting context (Prompt №9.4 §16/§81).
func SKUDetailV3() ExecutableProfileV3 {
	p := SKUCoreV3()
	p.Name = ProfileSKUDetailV3
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		optionalBlock(
			"sku.funnel",
			[]string{"FUNNEL.DAILY_ADDITIVE"},
			[]string{"BatchGetSKUFunnelPeriod"},
			[]string{"FUNNEL.SKU"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "FUNNEL.DAILY_ADDITIVE"),
		),
		notSupportedBlockV3("sku.finance"),
		supportingBlock(
			"sku.detail.campaign_context",
			[]string{"ADVERTISING.TRAFFIC"},
			[]string{"BatchListSKUCampaignPerformance"},
			[]string{"ADVERTISING.SKU_TRAFFIC"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PAID_SPEND_DESC",
				SemanticScope: "TOP_N_BY_PAID_SPEND",
				Period:        sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
			},
			sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
		),
		supportingBlock(
			"sku.detail.promotion_context",
			[]string{"PROMOTIONS.MEMBERSHIP"},
			[]string{"BatchGetSKUPromotions"},
			[]string{"PROMOTIONS.MEMBERSHIP"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PROMOTION_ID_ASC",
				SemanticScope: "TOP_N_ACTIVE_OR_CANDIDATE",
				Period:        snapshotPeriod(),
			},
		),
		supportingBlock(
			"sku.detail.query_context",
			[]string{"SEARCH.QUERY_DETAILS"},
			[]string{"BatchGetSKUQueryDetails"},
			[]string{"SEARCH.QUERY_DETAILS"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PROVIDER_SEARCH_RANK",
				SemanticScope: "TOP_N_PROVIDER_RANKED",
				Period:        sourceNativePeriod("CURRENT_7D", "SEARCH.QUERY_DETAILS"),
			},
			sourceNativePeriod("CURRENT_7D", "SEARCH.QUERY_DETAILS"),
		),
	)
	return p
}

// CampaignCoreV3 is the normative Campaign Core profile (Prompt №9.4 §83).
func CampaignCoreV3() ExecutableProfileV3 {
	return ExecutableProfileV3{
		Name:          ProfileCampaignCoreV3,
		SchemaVersion: LandscapeSchemaV3,
		Blocks: []BlockSpecV3{
			requiredBlock(
				"campaign.identity",
				nil,
				nil,
				nil,
			),
			requiredBlock(
				"campaign.traffic",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchGetCampaignPeriodPerformance"},
				[]string{"ADVERTISING.CAMPAIGN_TRAFFIC"},
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
			),
			requiredBlock(
				"campaign.attribution",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchGetCampaignPeriodPerformance"},
				[]string{"ADVERTISING.CAMPAIGN_ATTRIBUTION"},
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
			),
			derivedBlockV3("campaign.exposure", "ADVERTISING.TRAFFIC"),
			requiredBlock(
				"campaign.targeting",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchListCampaignTargetSKUs"},
				[]string{"ADVERTISING.CAMPAIGN_TRAFFIC"},
				snapshotPeriod(),
			),
		},
	}
}

// CampaignDetailV3 extends Campaign Core with Detail-only supporting context (Prompt №9.4 §16/§86).
func CampaignDetailV3() ExecutableProfileV3 {
	p := CampaignCoreV3()
	p.Name = ProfileCampaignDetailV3
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		supportingBlock(
			"campaign.detail.sku_context",
			[]string{"ADVERTISING.TRAFFIC"},
			[]string{"BatchGetSKUAdvertising"},
			[]string{"ADVERTISING.SKU_TRAFFIC"},
			SupportingSpec{
				Limit:         10,
				SortBasis:     "PAID_SPEND_DESC",
				SemanticScope: "TOP_N_OBSERVED_PERFORMANCE",
				Period:        sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
			},
			sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
		),
		derivedBlockV3("campaign.stock"),
	)
	return p
}
