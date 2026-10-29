package landscape

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// LandscapeEligibilityV1 is the versioned Landscape Core eligibility policy seal.
const LandscapeEligibilityV1 = "LANDSCAPE_ELIGIBILITY_V1"

// SKUEligibilityInput is the fact surface used by LANDSCAPE_ELIGIBILITY_V1.
type SKUEligibilityInput struct {
	CatalogState         string
	SalesUnitsCur        *int64
	SalesUnitsPrev       *int64
	SalesAmountCurKnown  bool
	SalesAmountPrevKnown bool
	SellableUnits        *int64
	OpenInboundUnits     *int64
	OpenSupplyCount      *int64
	AdsSpendCurKnown     bool
	AdsSpendPrevKnown    bool
	AdsAttrCurKnown      bool
	AdsAttrPrevKnown     bool
	HasPromoMembership   bool
}

// CampaignEligibilityInput is the fact surface used by LANDSCAPE_ELIGIBILITY_V1.
type CampaignEligibilityInput struct {
	ProviderState     string
	SpendCurKnown     bool
	SpendPrevKnown    bool
	SpendCurPositive  bool
	SpendPrevPositive bool
	AttrCurKnown      bool
	AttrPrevKnown     bool
	AttrCurPositive   bool
	AttrPrevPositive  bool
}

// SKUEligible reports whether a SKU belongs in Landscape Core under LANDSCAPE_ELIGIBILITY_V1.
// Eligible if ANY: catalog active OR sales cur/prev OR sellable>0 OR open inbound OR ads activity OR promo.
func SKUEligible(in SKUEligibilityInput) bool {
	if catalogStateActive(in.CatalogState) {
		return true
	}
	if positiveInt(in.SalesUnitsCur) || positiveInt(in.SalesUnitsPrev) || in.SalesAmountCurKnown || in.SalesAmountPrevKnown {
		return true
	}
	if in.SellableUnits != nil && *in.SellableUnits > 0 {
		return true
	}
	if in.OpenInboundUnits != nil && *in.OpenInboundUnits > 0 {
		return true
	}
	if in.OpenSupplyCount != nil && *in.OpenSupplyCount > 0 {
		return true
	}
	if in.AdsSpendCurKnown || in.AdsSpendPrevKnown || in.AdsAttrCurKnown || in.AdsAttrPrevKnown {
		return true
	}
	if in.HasPromoMembership {
		return true
	}
	return false
}

// CampaignEligible reports whether a campaign belongs in Landscape Core under LANDSCAPE_ELIGIBILITY_V1.
// Eligible if ANY: active/planned status OR spend cur/prev > 0 OR attribution cur/prev > 0.
func CampaignEligible(in CampaignEligibilityInput) bool {
	if campaignStateActiveOrPlanned(in.ProviderState) {
		return true
	}
	if (in.SpendCurKnown && in.SpendCurPositive) || (in.SpendPrevKnown && in.SpendPrevPositive) {
		return true
	}
	if (in.AttrCurKnown && in.AttrCurPositive) || (in.AttrPrevKnown && in.AttrPrevPositive) {
		return true
	}
	return false
}

func catalogStateActive(state string) bool {
	s := strings.ToUpper(strings.TrimSpace(state))
	switch s {
	case "ACTIVE", "IN_SALE", "FOR_SALE", "READY", "VISIBLE", "IN_STOCK":
		return true
	default:
		return false
	}
}

func campaignStateActiveOrPlanned(state string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case "active", "running", "campaign_state_running", "campaign_state_active",
		"planned", "ready", "moderation", "campaign_state_moderation", "campaign_state_planned":
		return true
	default:
		return strings.Contains(s, "active") || strings.Contains(s, "planned") || strings.Contains(s, "running")
	}
}

func positiveInt(v *int64) bool {
	return v != nil && *v > 0
}

func moneyKnownPositive(v factualapi.Value[factualapi.Money]) (known, positive bool) {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return false, false
	}
	return true, !v.Value.Amount.IsZero()
}

func intKnownPositive(v factualapi.Value[int64]) (known, positive bool) {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return false, false
	}
	return true, *v.Value > 0
}
