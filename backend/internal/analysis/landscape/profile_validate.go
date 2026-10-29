package landscape

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func stockCoverageBlockPresent(st *SKUStockBlock) bool {
	for _, lim := range st.Limitations {
		if strings.HasPrefix(lim.Code, "STOCK_COVERAGE") {
			return true
		}
	}
	return false
}

// ValidateCoreProfileCapabilities fail-closes when a required profile block capability
// is absent or unusable in the sealed context (Prompt №9.3 Part M).
func ValidateCoreProfileCapabilities(sealed factualapi.FactualReadContext, profiles CoreProfileBundle) error {
	for _, p := range []ExecutableProfile{profiles.Account, profiles.Category, profiles.SKU, profiles.Campaign} {
		if p.Name == "" {
			continue
		}
		if err := ValidateProfileCapabilities(sealed, p); err != nil {
			return err
		}
	}
	return nil
}

// ValidateProfileCapabilities checks required block capabilities against sealed bindings.
func ValidateProfileCapabilities(sealed factualapi.FactualReadContext, profile ExecutableProfile) error {
	for _, spec := range profile.Blocks {
		if !spec.Required || spec.CapabilityKey == "" {
			continue
		}
		if err := requireSealedCapability(sealed, spec.CapabilityKey); err != nil {
			return fmt.Errorf("%s: block %q capability %q: %w", ErrProfileCapabilityContractViolation, spec.BlockID, spec.CapabilityKey, err)
		}
	}
	return nil
}

func requireSealedCapability(sealed factualapi.FactualReadContext, key string) error {
	b, ok := sealed.CapabilityBinding(key, "HISTORICAL_QUERY")
	if !ok {
		return fmt.Errorf("not bound in sealed context")
	}
	switch b.FinalState {
	case "READY", "PARTIAL":
		return nil
	default:
		return fmt.Errorf("final_state=%s reason=%s", b.FinalState, b.ReasonCode)
	}
}

// ValidateProfileConformance checks that a built landscape entity matches its executable profile.
func ValidateProfileConformance(present map[string]bool, profile ExecutableProfile, capabilities []CapabilityFlag) error {
	allowed := profile.blockSet()
	for blockID := range present {
		if !present[blockID] {
			continue
		}
		if _, ok := allowed[blockID]; !ok {
			return fmt.Errorf("profile conformance: block %q not allowed by profile %s", blockID, profile.Name)
		}
	}
	for _, spec := range profile.Blocks {
		if spec.Required && spec.CapabilityKey != "" {
			if err := requireCapabilityFlags(capabilities, spec.CapabilityKey); err != nil {
				return fmt.Errorf("%s: block %q capability %q: %w", ErrProfileCapabilityContractViolation, spec.BlockID, spec.CapabilityKey, err)
			}
		}
		if spec.Required && !present[spec.BlockID] {
			return fmt.Errorf("profile conformance: required block %q missing from landscape for profile %s", spec.BlockID, profile.Name)
		}
	}
	return nil
}

func requireCapabilityFlags(capabilities []CapabilityFlag, key string) error {
	b, ok := capabilityFlagBinding(capabilities, key)
	if !ok {
		return fmt.Errorf("not bound")
	}
	switch b.FinalState {
	case "READY", "PARTIAL":
		return nil
	default:
		return fmt.Errorf("final_state=%s reason=%s", b.FinalState, b.ReasonCode)
	}
}

func capabilityFlagBinding(capabilities []CapabilityFlag, key string) (CapabilityFlag, bool) {
	for _, c := range capabilities {
		if c.Key == key && (c.Scope == "" || c.Scope == "HISTORICAL_QUERY") {
			return c, true
		}
	}
	return CapabilityFlag{}, false
}

// AccountPresentBlocks maps an AccountLandscape to profile block IDs.
func AccountPresentBlocks(a AccountLandscape) map[string]bool {
	out := map[string]bool{}
	if a.Sales != nil {
		out["account.sales"] = true
	}
	if a.Assortment != nil {
		out["account.assortment"] = true
	}
	if a.Stock != nil {
		out["account.stock"] = true
	}
	if a.Price != nil {
		out["account.price"] = true
	}
	if a.Advertising != nil {
		if a.Advertising.Traffic != nil {
			out["account.advertising.traffic"] = true
		}
		if a.Advertising.Attribution != nil {
			out["account.advertising.attribution"] = true
		}
		if a.Advertising.Traffic == nil && a.Advertising.Attribution == nil {
			out["account.advertising"] = true
		}
	}
	if a.Funnel != nil {
		out["account.funnel"] = true
	}
	if a.Search != nil {
		out["account.search"] = true
	}
	if a.Finance != nil {
		out["account.finance"] = true
	}
	if a.Promotions != nil {
		out["account.promotions"] = true
	}
	if a.Health != nil {
		out["account.health"] = true
	}
	if a.Supply != nil {
		out["account.supply"] = true
	}
	if a.Concentration != nil {
		out["account.concentration"] = true
	}
	return out
}

// SKUPresentBlocks maps a SKULandscape to profile block IDs.
func SKUPresentBlocks(s SKULandscape) map[string]bool {
	out := map[string]bool{}
	if s.Sales != nil {
		out["sku.sales"] = true
	}
	if s.Stock != nil {
		out["sku.stock"] = true
		if s.Stock.CoverageDays7D != nil || s.Stock.CoverageDays30D != nil || stockCoverageBlockPresent(s.Stock) {
			out["sku.stock.coverage"] = true
		}
	}
	if s.Price != nil {
		out["sku.price"] = true
		if s.Price.LastObservedChange != nil || len(s.Price.Limitations) > 0 {
			out["sku.price.history"] = true
		}
	}
	if s.Advertising != nil {
		if s.Advertising.Traffic != nil {
			out["sku.advertising.traffic"] = true
		}
		if s.Advertising.Attribution != nil {
			out["sku.advertising.attribution"] = true
		}
		if s.Advertising.Traffic == nil && s.Advertising.Attribution == nil {
			out["sku.advertising"] = true
		}
	}
	if s.Search != nil {
		out["sku.search"] = true
	}
	if s.Funnel != nil {
		out["sku.funnel"] = true
	}
	if s.Finance != nil {
		out["sku.finance"] = true
	}
	if s.Promotions != nil {
		out["sku.promotions"] = true
	}
	if s.Supply != nil {
		out["sku.supply"] = true
	}
	if s.CampaignContext != nil {
		out["sku.detail.campaign_context"] = true
	}
	if s.PromotionContext != nil {
		out["sku.detail.promotion_context"] = true
	}
	if s.QueryContext != nil {
		out["sku.detail.query_context"] = true
	}
	return out
}

// CategoryPresentBlocks maps a CategoryLandscape to profile block IDs.
func CategoryPresentBlocks(c CategoryLandscape) map[string]bool {
	out := map[string]bool{}
	if c.Sales != nil {
		out["category.sales"] = true
	}
	if c.Stock != nil {
		out["category.stock"] = true
	}
	if c.Price != nil {
		out["category.price"] = true
	}
	if c.Advertising != nil {
		if c.Advertising.Traffic != nil {
			out["category.advertising.traffic"] = true
		}
		if c.Advertising.Attribution != nil {
			out["category.advertising.attribution"] = true
		}
		if c.Advertising.Traffic == nil && c.Advertising.Attribution == nil {
			out["category.advertising"] = true
		}
	}
	if c.Funnel != nil {
		out["category.funnel"] = true
	}
	if c.Search != nil {
		out["category.search"] = true
	}
	if c.Finance != nil {
		out["category.finance"] = true
	}
	if c.Promotions != nil {
		out["category.promotions"] = true
	}
	if c.Supply != nil {
		out["category.supply"] = true
	}
	return out
}

// CampaignPresentBlocks maps a CampaignLandscape to profile block IDs.
func CampaignPresentBlocks(c CampaignLandscape) map[string]bool {
	out := map[string]bool{}
	if c.Identity.CampaignID != 0 {
		out["campaign.identity"] = true
	}
	if c.Performance != nil {
		if c.Performance.Traffic != nil {
			out["campaign.traffic"] = true
		}
		if c.Performance.Attribution != nil {
			out["campaign.attribution"] = true
		}
	}
	if c.Exposure != nil {
		out["campaign.exposure"] = true
	}
	if c.Targeting != nil {
		out["campaign.targeting"] = true
	}
	if c.Stock != nil {
		out["campaign.stock"] = true
	}
	if c.SKUContext != nil {
		out["campaign.detail.sku_context"] = true
	}
	return out
}
