package actionreconcile

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type SignatureInput struct {
	SellerID          int64
	ActionType        string
	Target            EntityRef
	NormalizedContext map[string]string
	IntentSignature   string
}

func HashSignature(in SignatureInput) string {
	if in.NormalizedContext == nil {
		in.NormalizedContext = map[string]string{}
	}
	return canonicaljson.Hash(map[string]any{
		"policy":             IdentityPolicyV1,
		"seller_account_id":  in.SellerID,
		"action_type":        in.ActionType,
		"target_kind":        string(in.Target.Kind),
		"target_id":          in.Target.ID,
		"normalized_context": in.NormalizedContext,
		"intent_signature":   in.IntentSignature,
	})
}

func ContextHash(ctx map[string]string) string {
	if ctx == nil {
		ctx = map[string]string{}
	}
	return canonicaljson.Hash(ctx)
}

func NormalizedContext(actionType string, params actioncatalog.Parameters) map[string]string {
	out := map[string]string{}
	switch actionType {
	case actioncatalog.TypeSKUAdjustAdBid, actioncatalog.TypeSKUChangeAdParticipation:
		if params.CampaignRef != nil {
			out["campaign_ref"] = *params.CampaignRef
		}
	case actioncatalog.TypeSKUJoinPromotion, actioncatalog.TypeSKULeavePromotion, actioncatalog.TypeSKUAdjustPromotionPrice:
		if params.PromotionRef != nil {
			out["promotion_ref"] = *params.PromotionRef
		}
	}
	return out
}

func IntentSignature(actionType string, params actioncatalog.Parameters, currentPrice *actioncatalog.Money) (string, error) {
	switch actionType {
	case actioncatalog.TypeSKUAdjustPrice:
		if params.RecommendedPrice == nil {
			return "", fmt.Errorf("PRICE_INTENT_MISSING_RECOMMENDED")
		}
		if currentPrice == nil {
			return "UNSPECIFIED", nil
		}
		if params.RecommendedPrice.Amount < currentPrice.Amount {
			return "DECREASE", nil
		}
		if params.RecommendedPrice.Amount > currentPrice.Amount {
			return "INCREASE", nil
		}
		return "HOLD", nil
	case actioncatalog.TypeCampaignAdjustBudget:
		if params.RecommendedBudget == nil {
			return "UNSPECIFIED", nil
		}
		if params.Direction != nil {
			return *params.Direction, nil
		}
		return "ADJUST", nil
	case actioncatalog.TypeSKUAdjustAdBid:
		if params.Direction != nil {
			return *params.Direction, nil
		}
		return "ADJUST", nil
	case actioncatalog.TypeCampaignChangeState, actioncatalog.TypeSKUChangeAdParticipation:
		if params.DesiredState != nil {
			return *params.DesiredState, nil
		}
		return "STATE", nil
	case actioncatalog.TypeSKUChangeCatalogState:
		if params.DesiredState != nil {
			return *params.DesiredState, nil
		}
		return "CATALOG_STATE", nil
	case actioncatalog.TypeSKUIncreaseReplenishment:
		return "INCREASE", nil
	case actioncatalog.TypeSKUReduceReplenishment:
		return "DECREASE", nil
	case actioncatalog.TypeSKUJoinPromotion:
		return "JOIN", nil
	case actioncatalog.TypeSKULeavePromotion:
		return "LEAVE", nil
	default:
		return "DEFAULT", nil
	}
}

func CurrentPriceHint(p deep.ActionProposal) *actioncatalog.Money {
	return nil
}
