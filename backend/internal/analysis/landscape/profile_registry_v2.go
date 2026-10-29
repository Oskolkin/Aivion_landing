package landscape

import (
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// BlockSpec is one executable factual block in a Landscape profile (Prompt №9.3 Part D).
type BlockSpec struct {
	BlockID       string
	Required      bool
	CapabilityKey string
	FactFamily    string
	FactualMethod string
	Periods       []string
}

// ExecutableProfile is a versioned, hashable Landscape profile registry entry.
type ExecutableProfile struct {
	Name          string
	SchemaVersion string
	Blocks        []BlockSpec
}

// CallPlanEntry is one deterministic traceability row for profile-driven Factual reads.
type CallPlanEntry struct {
	Order         int      `json:"order"`
	BlockID       string   `json:"block_id"`
	FactualMethod string   `json:"factual_method"`
	Periods       []string `json:"periods,omitempty"`
	CapabilityKey string   `json:"capability_key,omitempty"`
	FactFamily    string   `json:"fact_family,omitempty"`
	Required      bool     `json:"required"`
}

// CoreProfileBundle holds the four Core entity profiles used by BUILD_LANDSCAPE_CORE.
type CoreProfileBundle struct {
	Account  ExecutableProfile
	Category ExecutableProfile
	SKU      ExecutableProfile
	Campaign ExecutableProfile
}

func DefaultCoreProfilesV2() CoreProfileBundle {
	return CoreProfileBundle{
		Account:  AccountDiscoveryV2(),
		Category: CategoryCoreV2(),
		SKU:      SKUCoreV2(),
		Campaign: CampaignCoreV2(),
	}
}

func (b CoreProfileBundle) IsZero() bool {
	return b.Account.Name == "" && b.Category.Name == "" && b.SKU.Name == "" && b.Campaign.Name == ""
}

// ProfileHash returns the canonical SHA-256 hex digest of the executable profile spec.
func (p ExecutableProfile) ProfileHash() string {
	return canonicaljson.Hash(profileHashPayload(p))
}

// CallPlan returns a deterministic traceability plan for profile-driven Factual reads.
func (p ExecutableProfile) CallPlan() []CallPlanEntry {
	out := make([]CallPlanEntry, len(p.Blocks))
	for i, b := range p.Blocks {
		periods := append([]string(nil), b.Periods...)
		sort.Strings(periods)
		out[i] = CallPlanEntry{
			Order:         i + 1,
			BlockID:       b.BlockID,
			FactualMethod: b.FactualMethod,
			Periods:       periods,
			CapabilityKey: b.CapabilityKey,
			FactFamily:    b.FactFamily,
			Required:      b.Required,
		}
	}
	return out
}

func (p ExecutableProfile) HasBlock(blockID string) bool {
	_, ok := p.blockByID(blockID)
	return ok
}

func (p ExecutableProfile) blockByID(blockID string) (BlockSpec, bool) {
	for _, b := range p.Blocks {
		if b.BlockID == blockID {
			return b, true
		}
	}
	return BlockSpec{}, false
}

func (p ExecutableProfile) blockSet() map[string]struct{} {
	out := make(map[string]struct{}, len(p.Blocks))
	for _, b := range p.Blocks {
		out[b.BlockID] = struct{}{}
	}
	return out
}

func profileHashPayload(p ExecutableProfile) map[string]any {
	blocks := make([]map[string]any, len(p.Blocks))
	for i, b := range p.Blocks {
		periods := append([]string(nil), b.Periods...)
		sort.Strings(periods)
		blocks[i] = map[string]any{
			"block_id":       b.BlockID,
			"required":       b.Required,
			"capability_key": b.CapabilityKey,
			"fact_family":    b.FactFamily,
			"factual_method": b.FactualMethod,
			"periods":        periods,
		}
	}
	return map[string]any{
		"name":           p.Name,
		"schema_version": p.SchemaVersion,
		"blocks":         blocks,
	}
}

func sealExecutableProfile(p ExecutableProfile) string {
	return p.Name + "#" + p.ProfileHash()
}

// CoreProfileVersionSealV2 seals Core profiles as NAME#hash tuples (Prompt №9.3 Part P).
func CoreProfileVersionSealV2() string {
	b := DefaultCoreProfilesV2()
	return sealExecutableProfile(b.Account) + "|" +
		sealExecutableProfile(b.Category) + "|" +
		sealExecutableProfile(b.SKU) + "|" +
		sealExecutableProfile(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

// DetailProfileVersionSealV2 seals Detail profiles as NAME#hash tuples.
func DetailProfileVersionSealV2() string {
	return sealExecutableProfile(AccountDiscoveryV2()) + "|" +
		sealExecutableProfile(CategoryDetailV2()) + "|" +
		sealExecutableProfile(SKUDetailV2()) + "|" +
		sealExecutableProfile(CampaignDetailV2()) + "|" +
		LandscapeEligibilityV1Seal
}

func block(
	blockID, factFamily, factualMethod, capabilityKey string,
	required bool,
	periods ...string,
) BlockSpec {
	return BlockSpec{
		BlockID:       blockID,
		Required:      required,
		CapabilityKey: capabilityKey,
		FactFamily:    factFamily,
		FactualMethod: factualMethod,
		Periods:       periods,
	}
}

func derivedBlock(blockID string, required bool) BlockSpec {
	return BlockSpec{
		BlockID:       blockID,
		Required:      required,
		FactualMethod: "DERIVED",
	}
}

// AccountDiscoveryV2 is the normative Account Discovery Core profile.
func AccountDiscoveryV2() ExecutableProfile {
	return ExecutableProfile{
		Name:          ProfileAccountDiscoveryV2,
		SchemaVersion: LandscapeSchemaV2,
		Blocks: []BlockSpec{
			block("account.sales", "SALES.DEMAND", "GetAccountDemand", "SALES.ACCOUNT_DEMAND", true, "CURRENT_7D", "PREVIOUS_7D", "TREND_30D"),
			derivedBlock("account.assortment", true),
			derivedBlock("account.stock", true),
			derivedBlock("account.price", true),
			block("account.advertising", "ADVERTISING.TRAFFIC", "GetAdvertisingAccountSummary", "ADVERTISING.ACCOUNT_SPEND", true, "CURRENT_7D", "PREVIOUS_7D"),
			block("account.funnel", "FUNNEL.DAILY_ADDITIVE", "GetAccountFunnelPeriod", "FUNNEL.ACCOUNT", false, "CURRENT_7D", "PREVIOUS_7D"),
			block("account.search", "SEARCH.SELLER_SUMMARY", "BatchGetSKUSearchSummary", "SEARCH.SELLER_SUMMARY", false, "CURRENT_7D", "PREVIOUS_7D"),
			block("account.finance", "FINANCE.DAILY_LEDGER", "GetFinanceAccountSummary", "FINANCE.ACCOUNT_NET", false, "CURRENT_7D", "PREVIOUS_7D"),
			block("account.promotions", "PROMOTIONS.DEFINITIONS", "ListPromotions", "PROMOTIONS.DEFINITIONS", false),
			block("account.health", "ACCOUNT_HEALTH.CURRENT_SNAPSHOT", "GetAccountHealthSummary", "HEALTH.CURRENT_RATINGS", false),
			block("account.supply", "SUPPLY.STATE", "ListSupplies", "SUPPLY.STATE", false),
			derivedBlock("account.concentration", false),
		},
	}
}

// CategoryCoreV2 is the normative Category Core profile.
func CategoryCoreV2() ExecutableProfile {
	return ExecutableProfile{
		Name:          ProfileCategoryCoreV2,
		SchemaVersion: LandscapeSchemaV2,
		Blocks: []BlockSpec{
			block("category.sales", "SALES.DEMAND", "ListCategoryDemand", "SALES.CATEGORY_DEMAND", true, "CURRENT_7D", "PREVIOUS_7D"),
			derivedBlock("category.stock", true),
			derivedBlock("category.price", false),
			derivedBlock("category.advertising", true),
			block("category.funnel", "FUNNEL.DAILY_ADDITIVE", "GetCategoryFunnelPeriod", "FUNNEL.CATEGORY_ADDITIVE", false, "CURRENT_7D"),
			block("category.search", "SEARCH.SELLER_SUMMARY", "GetCategorySearchSummary", "SEARCH.CATEGORY_SUMMARY", false, "CURRENT_7D"),
			block("category.finance", "FINANCE.DAILY_LEDGER", "GetFinanceAccountSummary", "FINANCE.CATEGORY_DIRECT", false, "CURRENT_7D"),
			block("category.promotions", "PROMOTIONS.MEMBERSHIP", "ListPromotions", "PROMOTIONS.MEMBERSHIP", false),
			derivedBlock("category.supply", true),
		},
	}
}

// CategoryDetailV2 extends Category Core with Detail-only context limits.
func CategoryDetailV2() ExecutableProfile {
	p := CategoryCoreV2()
	p.Name = ProfileCategoryDetailV2
	return p
}

// SKUCoreV2 is the normative SKU Core profile (7d comparisons + 30d context).
func SKUCoreV2() ExecutableProfile {
	return ExecutableProfile{
		Name:          ProfileSKUCoreV2,
		SchemaVersion: LandscapeSchemaV2,
		Blocks: []BlockSpec{
			block("sku.sales", "SALES.DEMAND", "BatchGetSKUDemand", "SALES.SKU_DEMAND", true, "CURRENT_7D", "PREVIOUS_7D", "TREND_30D"),
			block("sku.stock", "STOCK.SNAPSHOT", "BatchGetSKUStock", "STOCK.SKU_CURRENT", true),
			block("sku.stock.coverage", "STOCK.SNAPSHOT", "BatchGetStockCoverage", "STOCK.COVERAGE_7D", false),
			block("sku.price", "PRICE.SNAPSHOT", "BatchGetSKUPrice", "PRICE.CUSTOMER_PRICE", true),
			block("sku.price.history", "PRICE.SNAPSHOT", "BatchGetSKUPriceLastObservedChange", "PRICE.HISTORY", false),
			block("sku.advertising", "ADVERTISING.TRAFFIC", "BatchGetSKUAdvertising", "ADVERTISING.SKU_TRAFFIC", false, "CURRENT_7D", "PREVIOUS_7D"),
			block("sku.search", "SEARCH.SELLER_SUMMARY", "BatchGetSKUSearchSummary", "SEARCH.SELLER_SUMMARY", false, "CURRENT_7D", "PREVIOUS_7D"),
			block("sku.funnel", "FUNNEL.DAILY_ADDITIVE", "BatchGetSKUFunnelPeriod", "FUNNEL.SKU", false, "CURRENT_7D"),
			block("sku.finance", "FINANCE.DAILY_LEDGER", "GetFinanceAccountSummary", "FINANCE.SKU_DIRECT", false, "CURRENT_7D"),
			block("sku.promotions", "PROMOTIONS.MEMBERSHIP", "ListPromotions", "PROMOTIONS.MEMBERSHIP", false),
			block("sku.supply", "SUPPLY.STATE", "BatchGetSKUInbound", "SUPPLY.SKU_INBOUND", false),
		},
	}
}

// SKUDetailV2 extends SKU Core with bounded supporting context.
func SKUDetailV2() ExecutableProfile {
	p := SKUCoreV2()
	p.Name = ProfileSKUDetailV2
	p.Blocks = append(append([]BlockSpec(nil), p.Blocks...),
		block("sku.detail.campaign_context", "ADVERTISING.TRAFFIC", "ListSKUCampaignPerformance", "ADVERTISING.SKU_TRAFFIC", false, "CURRENT_7D"),
		block("sku.detail.promotion_context", "PROMOTIONS.MEMBERSHIP", "ListPromotions", "PROMOTIONS.MEMBERSHIP", false),
		block("sku.detail.query_context", "SEARCH.QUERY_DETAILS", "ListSKUSearchQueries", "SEARCH.QUERY_DETAILS", false, "CURRENT_7D"),
	)
	return p
}

// CampaignCoreV2 is the normative Campaign Core profile.
func CampaignCoreV2() ExecutableProfile {
	return ExecutableProfile{
		Name:          ProfileCampaignCoreV2,
		SchemaVersion: LandscapeSchemaV2,
		Blocks: []BlockSpec{
			block("campaign.performance", "ADVERTISING.TRAFFIC", "BatchGetCampaignPeriodPerformance", "ADVERTISING.CAMPAIGN_TRAFFIC", true, "CURRENT_7D", "PREVIOUS_7D"),
			derivedBlock("campaign.stock", false),
		},
	}
}

// CampaignDetailV2 extends Campaign Core with SKU ads supporting context.
func CampaignDetailV2() ExecutableProfile {
	p := CampaignCoreV2()
	p.Name = ProfileCampaignDetailV2
	p.Blocks = append(append([]BlockSpec(nil), p.Blocks...),
		block("campaign.detail.sku_context", "ADVERTISING.TRAFFIC", "BatchGetSKUAdvertising", "ADVERTISING.SKU_TRAFFIC", false, "CURRENT_7D"),
	)
	return p
}

// ExecutableProfileByName resolves a V2 profile by seal name (legacy V1 uses ProfileByName).
func ExecutableProfileByName(name string) (ExecutableProfile, bool) {
	switch name {
	case ProfileAccountDiscoveryV2:
		return AccountDiscoveryV2(), true
	case ProfileCategoryCoreV2:
		return CategoryCoreV2(), true
	case ProfileCategoryDetailV2:
		return CategoryDetailV2(), true
	case ProfileSKUCoreV2:
		return SKUCoreV2(), true
	case ProfileSKUDetailV2:
		return SKUDetailV2(), true
	case ProfileCampaignCoreV2:
		return CampaignCoreV2(), true
	case ProfileCampaignDetailV2:
		return CampaignDetailV2(), true
	default:
		return ExecutableProfile{}, false
	}
}
