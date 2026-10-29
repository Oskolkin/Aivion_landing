package actioncatalog

import (
	"fmt"
	"sort"
	"strings"
)

func ValidateProposal(p Proposal, menu Menu, snap Snapshot) error {
	def, ok := ByKey()[p.ActionType]
	if !ok {
		return fmt.Errorf("UNKNOWN_ACTION_TYPE:%s", p.ActionType)
	}
	item, ok := menuItem(menu, p.ActionType)
	if !ok {
		return fmt.Errorf("ACTION_NOT_IN_MENU:%s", p.ActionType)
	}
	if string(p.Target.Kind) != item.TargetKind {
		return fmt.Errorf("ACTION_TARGET_KIND_MISMATCH")
	}
	if !contains(item.EligibleTargetIDs, p.Target.ID) {
		return fmt.Errorf("ACTION_TARGET_NOT_ELIGIBLE")
	}
	if !isMain(snap, string(p.Target.Kind), p.Target.ID) {
		return fmt.Errorf("ACTION_TARGET_NOT_MAIN")
	}
	if err := validateParams(def, p, snap); err != nil {
		return err
	}
	return nil
}

func menuItem(m Menu, key string) (MenuItem, bool) {
	for _, it := range m.Items {
		if it.ActionType == key {
			return it, true
		}
	}
	return MenuItem{}, false
}

func isMain(snap Snapshot, kind, id string) bool {
	return snap.MainIDs[kind+"|"+id]
}

func validateParams(def Definition, p Proposal, snap Snapshot) error {
	if extra := unexpectedFields(def.ActionTypeKey, p.Parameters); extra != "" {
		return fmt.Errorf("FORBIDDEN_PARAMETER:%s", extra)
	}
	switch def.ActionTypeKey {
	case TypeSKUAdjustPrice:
		return validateSKUAdjustPrice(p, snap)
	case TypeSKUIncreaseReplenishment:
		return optionalNonNegInt(p.Parameters.RecommendedUnits, "recommended_units")
	case TypeSKUReduceReplenishment:
		sk := snap.SKU[p.Target.ID]
		if !sk.HasReplenishmentContext {
			return fmt.Errorf("REPLENISHMENT_CONTEXT_REQUIRED")
		}
		return optionalNonNegInt(p.Parameters.RecommendedUnitsReduction, "recommended_units_reduction")
	case TypeSKUAdjustAdBid:
		return validateAdBid(p, snap)
	case TypeSKUChangeAdParticipation:
		return validateAdParticipation(p, snap)
	case TypeSKUJoinPromotion:
		return validateJoinPromotion(p, snap)
	case TypeSKULeavePromotion:
		return validateLeavePromotion(p, snap)
	case TypeSKUAdjustPromotionPrice:
		return validateAdjustPromotionPrice(p, snap)
	case TypeSKUChangeCatalogState:
		return validateCatalogState(p, snap)
	case TypeCampaignAdjustBudget:
		return validateCampaignBudget(p, snap)
	case TypeCampaignChangeState:
		return validateCampaignState(p, snap)
	case TypeCampaignAdjustStrategy:
		return validateCampaignStrategy(p, snap)
	case TypeCampaignAdjustProductSet:
		return validateProductSet(p, snap)
	case TypeAccountRatingRemediation:
		return validateAccountRating(p, snap)
	case TypeCategoryRepricingPlan:
		return validateCategoryDirection(p, []string{"LOWER", "RAISE", "MIXED"}, snap)
	case TypeCategoryReplenishmentPlan:
		return validateCategoryDirection(p, []string{"INCREASE", "DECREASE", "MIXED"}, snap)
	case TypeCategoryAdvertisingReallocation:
		return validateCategoryIntent(p, []string{"INCREASE", "DECREASE", "REALLOCATE"}, snap)
	case TypeCategoryPromotionPlan:
		return validateCategoryIntent(p, []string{"EXPAND_PARTICIPATION", "REDUCE_PARTICIPATION", "CHANGE_TERMS", "MIXED"}, snap)
	case TypeCategoryAssortmentRationalization:
		return validateCategoryIntent(p, []string{"REDUCE_ACTIVE_SET", "REACTIVATE_ARCHIVED", "MIXED"}, snap)
	default:
		return fmt.Errorf("UNKNOWN_ACTION_TYPE:%s", def.ActionTypeKey)
	}
}

func validateSKUAdjustPrice(p Proposal, snap Snapshot) error {
	m := p.Parameters.RecommendedPrice
	if m == nil || m.Amount <= 0 || strings.TrimSpace(m.Currency) == "" {
		return fmt.Errorf("RECOMMENDED_PRICE_INVALID")
	}
	st := snap.SKU[p.Target.ID]
	if st.CurrentPrice == nil {
		return fmt.Errorf("CURRENT_PRICE_UNKNOWN")
	}
	if m.Currency != st.CurrentPrice.Currency {
		return fmt.Errorf("CURRENCY_MISMATCH")
	}
	if m.Amount == st.CurrentPrice.Amount {
		return fmt.Errorf("INVALID_NOOP")
	}
	if st.MinAllowedPrice != nil && m.Amount < *st.MinAllowedPrice {
		return fmt.Errorf("BELOW_PROVIDER_FLOOR")
	}
	return nil
}

func validateAdBid(p Proposal, snap Snapshot) error {
	if p.Parameters.CampaignRef == nil || strings.TrimSpace(*p.Parameters.CampaignRef) == "" {
		return fmt.Errorf("CAMPAIGN_REF_REQUIRED")
	}
	if p.Parameters.RecommendedBid == nil || p.Parameters.RecommendedBid.Amount <= 0 {
		return fmt.Errorf("RECOMMENDED_BID_INVALID")
	}
	st := snap.SKU[p.Target.ID]
	if !contains(st.CampaignRefs, *p.Parameters.CampaignRef) {
		return fmt.Errorf("SKU_CAMPAIGN_RELATION_MISSING")
	}
	if st.BidKnownForCampaign != nil && !st.BidKnownForCampaign[*p.Parameters.CampaignRef] {
		return fmt.Errorf("BID_LIMITS_UNKNOWN")
	}
	if !isMain(snap, "CAMPAIGN", *p.Parameters.CampaignRef) {
		return fmt.Errorf("RELATED_CAMPAIGN_NOT_MAIN")
	}
	return nil
}

func validateAdParticipation(p Proposal, snap Snapshot) error {
	if p.Parameters.CampaignRef == nil {
		return fmt.Errorf("CAMPAIGN_REF_REQUIRED")
	}
	st := p.Parameters.DesiredState
	if st == nil || (*st != "ENABLED" && *st != "DISABLED") {
		return fmt.Errorf("DESIRED_STATE_INVALID")
	}
	sk := snap.SKU[p.Target.ID]
	if !contains(sk.CampaignRefs, *p.Parameters.CampaignRef) && *st == "DISABLED" {
		return fmt.Errorf("SKU_CAMPAIGN_RELATION_MISSING")
	}
	if *st == "ENABLED" && !isMain(snap, "CAMPAIGN", *p.Parameters.CampaignRef) {
		return fmt.Errorf("RELATED_CAMPAIGN_NOT_MAIN")
	}
	return nil
}

func validateJoinPromotion(p Proposal, snap Snapshot) error {
	if p.Parameters.PromotionRef == nil || *p.Parameters.PromotionRef == "" {
		return fmt.Errorf("PROMOTION_REF_REQUIRED")
	}
	sk := snap.SKU[p.Target.ID]
	if contains(sk.PromotionParticipants, *p.Parameters.PromotionRef) {
		return fmt.Errorf("ALREADY_PARTICIPANT")
	}
	if !contains(sk.PromotionCandidates, *p.Parameters.PromotionRef) {
		return fmt.Errorf("NOT_PROMOTION_CANDIDATE")
	}
	if p.Parameters.RecommendedActionPrice != nil && p.Parameters.RecommendedActionPrice.Amount <= 0 {
		return fmt.Errorf("RECOMMENDED_ACTION_PRICE_INVALID")
	}
	return nil
}

func validateLeavePromotion(p Proposal, snap Snapshot) error {
	if p.Parameters.PromotionRef == nil {
		return fmt.Errorf("PROMOTION_REF_REQUIRED")
	}
	sk := snap.SKU[p.Target.ID]
	if !contains(sk.PromotionParticipants, *p.Parameters.PromotionRef) {
		return fmt.Errorf("NOT_PARTICIPANT")
	}
	return nil
}

func validateAdjustPromotionPrice(p Proposal, snap Snapshot) error {
	if p.Parameters.PromotionRef == nil || p.Parameters.RecommendedActionPrice == nil {
		return fmt.Errorf("PROMOTION_PRICE_REQUIRED")
	}
	if p.Parameters.RecommendedActionPrice.Amount <= 0 {
		return fmt.Errorf("RECOMMENDED_ACTION_PRICE_INVALID")
	}
	sk := snap.SKU[p.Target.ID]
	if !contains(sk.PromotionParticipants, *p.Parameters.PromotionRef) {
		return fmt.Errorf("NOT_PARTICIPANT")
	}
	if sk.CurrentPrice != nil && p.Parameters.RecommendedActionPrice.Currency != "" &&
		p.Parameters.RecommendedActionPrice.Currency == sk.CurrentPrice.Currency &&
		p.Parameters.RecommendedActionPrice.Amount == sk.CurrentPrice.Amount {
		return fmt.Errorf("PROMOTION_PRICE_EQUALS_OBSERVED_PRICE")
	}
	return nil
}

func validateCatalogState(p Proposal, snap Snapshot) error {
	if p.Parameters.DesiredState == nil {
		return fmt.Errorf("DESIRED_STATE_REQUIRED")
	}
	ds := *p.Parameters.DesiredState
	if ds != "ACTIVE" && ds != "ARCHIVED" {
		return fmt.Errorf("DESIRED_STATE_INVALID")
	}
	cur := snap.SKU[p.Target.ID].CatalogState
	if cur != "" && cur == ds {
		return fmt.Errorf("INVALID_NOOP")
	}
	return nil
}

func validateCampaignBudget(p Proposal, snap Snapshot) error {
	if p.Parameters.RecommendedBudget == nil || p.Parameters.RecommendedBudget.Amount <= 0 {
		return fmt.Errorf("RECOMMENDED_BUDGET_INVALID")
	}
	if p.Parameters.BudgetPeriod == nil {
		return fmt.Errorf("BUDGET_PERIOD_REQUIRED")
	}
	bp := *p.Parameters.BudgetPeriod
	if bp != "WEEKLY" && bp != "PROVIDER_DEFINED" {
		return fmt.Errorf("BUDGET_PERIOD_INVALID")
	}
	cur := snap.Campaign[p.Target.ID].Budget
	if cur != nil && cur.Currency == p.Parameters.RecommendedBudget.Currency && cur.Amount == p.Parameters.RecommendedBudget.Amount {
		return fmt.Errorf("INVALID_NOOP")
	}
	return nil
}

func validateCampaignState(p Proposal, snap Snapshot) error {
	if p.Parameters.DesiredState == nil {
		return fmt.Errorf("DESIRED_STATE_REQUIRED")
	}
	ds := *p.Parameters.DesiredState
	if ds != "ACTIVE" && ds != "PAUSED" {
		return fmt.Errorf("DESIRED_STATE_INVALID")
	}
	cur := snap.Campaign[p.Target.ID].State
	if cur != "" && cur == ds {
		return fmt.Errorf("INVALID_NOOP")
	}
	return nil
}

func validateCampaignStrategy(p Proposal, snap Snapshot) error {
	if p.Parameters.DesiredStrategy == nil || *p.Parameters.DesiredStrategy == "" {
		return fmt.Errorf("DESIRED_STRATEGY_REQUIRED")
	}
	allowed := snap.Campaign[p.Target.ID].AllowedStrategies
	if len(allowed) == 0 {
		return fmt.Errorf("STRATEGY_ENUM_UNKNOWN")
	}
	if !contains(allowed, *p.Parameters.DesiredStrategy) {
		return fmt.Errorf("UNSUPPORTED_CAMPAIGN_STRATEGY")
	}
	if snap.Campaign[p.Target.ID].CurrentStrategy == *p.Parameters.DesiredStrategy {
		return fmt.Errorf("INVALID_NOOP")
	}
	return nil
}

func validateProductSet(p Proposal, snap Snapshot) error {
	add := uniq(p.Parameters.AddSKURefs)
	rem := uniq(p.Parameters.RemoveSKURefs)
	if len(add) == 0 && len(rem) == 0 {
		return fmt.Errorf("PRODUCT_SET_EMPTY")
	}
	seen := map[string]bool{}
	for _, id := range add {
		if seen[id] {
			return fmt.Errorf("ADD_REMOVE_INTERSECTION")
		}
		seen[id] = true
		if !isMain(snap, "SKU", id) {
			return fmt.Errorf("CHILD_SKU_NOT_MAIN")
		}
	}
	for _, id := range rem {
		if seen[id] {
			return fmt.Errorf("ADD_REMOVE_INTERSECTION")
		}
		camp := snap.Campaign[p.Target.ID]
		if len(camp.ProductSKURefs) > 0 && !contains(camp.ProductSKURefs, id) {
			return fmt.Errorf("REMOVE_SKU_NOT_IN_CAMPAIGN")
		}
		if !isMain(snap, "SKU", id) {
			return fmt.Errorf("CHILD_SKU_NOT_MAIN")
		}
	}
	return nil
}

func validateAccountRating(p Proposal, snap Snapshot) error {
	if p.Parameters.PrimaryRatingKey == nil || *p.Parameters.PrimaryRatingKey == "" {
		return fmt.Errorf("PRIMARY_RATING_KEY_REQUIRED")
	}
	if !contains(snap.Account.RatingKeys, *p.Parameters.PrimaryRatingKey) {
		return fmt.Errorf("RATING_KEY_UNKNOWN")
	}
	for _, k := range p.Parameters.RelatedRatingKeys {
		if !contains(snap.Account.RatingKeys, k) {
			return fmt.Errorf("RELATED_RATING_KEY_UNKNOWN")
		}
	}
	for _, k := range p.Parameters.ProviderErrorTypes {
		if len(snap.Account.ErrorTypes) > 0 && !contains(snap.Account.ErrorTypes, k) {
			return fmt.Errorf("PROVIDER_ERROR_TYPE_UNKNOWN")
		}
	}
	return nil
}

func validateCategoryDirection(p Proposal, allowed []string, snap Snapshot) error {
	if p.Parameters.Direction == nil || !contains(allowed, *p.Parameters.Direction) {
		return fmt.Errorf("DIRECTION_INVALID")
	}
	return requireMainSKUs(p.Parameters.AffectedMaterialSKURefs, snap)
}

func validateCategoryIntent(p Proposal, allowed []string, snap Snapshot) error {
	if p.Parameters.Intent == nil || !contains(allowed, *p.Parameters.Intent) {
		return fmt.Errorf("INTENT_INVALID")
	}
	if err := requireMainSKUs(p.Parameters.AffectedMaterialSKURefs, snap); err != nil {
		return err
	}
	for _, id := range p.Parameters.RelatedMaterialCampaignRefs {
		if !isMain(snap, "CAMPAIGN", id) {
			return fmt.Errorf("RELATED_CAMPAIGN_NOT_MAIN")
		}
	}
	return nil
}

func requireMainSKUs(ids []string, snap Snapshot) error {
	if len(ids) == 0 {
		return fmt.Errorf("AFFECTED_SKU_REFS_REQUIRED")
	}
	for _, id := range ids {
		if !isMain(snap, "SKU", id) {
			return fmt.Errorf("AFFECTED_SKU_NOT_MAIN")
		}
	}
	return nil
}

func optionalNonNegInt(v *int, field string) error {
	if v == nil {
		return nil
	}
	if *v < 0 {
		return fmt.Errorf("%s_NEGATIVE", strings.ToUpper(field))
	}
	return nil
}

func unexpectedFields(action string, p Parameters) string {
	used := usedFields(p)
	allowed := allowedFields(action)
	var extra []string
	for _, f := range used {
		if !allowed[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(extra)
	if len(extra) == 0 {
		return ""
	}
	return strings.Join(extra, ",")
}

func usedFields(p Parameters) []string {
	var f []string
	if p.RecommendedPrice != nil {
		f = append(f, "recommended_price")
	}
	if p.RecommendedUnits != nil {
		f = append(f, "recommended_units")
	}
	if p.RecommendedUnitsReduction != nil {
		f = append(f, "recommended_units_reduction")
	}
	if p.CampaignRef != nil {
		f = append(f, "campaign_ref")
	}
	if p.RecommendedBid != nil {
		f = append(f, "recommended_bid")
	}
	if p.DesiredState != nil {
		f = append(f, "desired_state")
	}
	if p.PromotionRef != nil {
		f = append(f, "promotion_ref")
	}
	if p.RecommendedActionPrice != nil {
		f = append(f, "recommended_action_price")
	}
	if p.PrimaryRatingKey != nil {
		f = append(f, "primary_rating_key")
	}
	if len(p.RelatedRatingKeys) > 0 {
		f = append(f, "related_rating_keys")
	}
	if len(p.ProviderErrorTypes) > 0 {
		f = append(f, "provider_error_types")
	}
	if p.Direction != nil {
		f = append(f, "direction")
	}
	if len(p.AffectedMaterialSKURefs) > 0 {
		f = append(f, "affected_material_sku_refs")
	}
	if p.Intent != nil {
		f = append(f, "intent")
	}
	if len(p.RelatedMaterialCampaignRefs) > 0 {
		f = append(f, "related_material_campaign_refs")
	}
	if p.RecommendedBudget != nil {
		f = append(f, "recommended_budget")
	}
	if p.BudgetPeriod != nil {
		f = append(f, "budget_period")
	}
	if p.DesiredStrategy != nil {
		f = append(f, "desired_strategy")
	}
	if len(p.AddSKURefs) > 0 {
		f = append(f, "add_sku_refs")
	}
	if len(p.RemoveSKURefs) > 0 {
		f = append(f, "remove_sku_refs")
	}
	return f
}

func allowedFields(action string) map[string]bool {
	switch action {
	case TypeSKUAdjustPrice:
		return set("recommended_price")
	case TypeSKUIncreaseReplenishment:
		return set("recommended_units")
	case TypeSKUReduceReplenishment:
		return set("recommended_units_reduction")
	case TypeSKUAdjustAdBid:
		return set("campaign_ref", "recommended_bid")
	case TypeSKUChangeAdParticipation:
		return set("campaign_ref", "desired_state")
	case TypeSKUJoinPromotion:
		return set("promotion_ref", "recommended_action_price")
	case TypeSKULeavePromotion:
		return set("promotion_ref")
	case TypeSKUAdjustPromotionPrice:
		return set("promotion_ref", "recommended_action_price")
	case TypeSKUChangeCatalogState:
		return set("desired_state")
	case TypeCampaignAdjustBudget:
		return set("recommended_budget", "budget_period")
	case TypeCampaignChangeState:
		return set("desired_state")
	case TypeCampaignAdjustStrategy:
		return set("desired_strategy")
	case TypeCampaignAdjustProductSet:
		return set("add_sku_refs", "remove_sku_refs")
	case TypeAccountRatingRemediation:
		return set("primary_rating_key", "related_rating_keys", "provider_error_types")
	case TypeCategoryRepricingPlan, TypeCategoryReplenishmentPlan:
		return set("direction", "affected_material_sku_refs")
	case TypeCategoryAdvertisingReallocation:
		return set("intent", "related_material_campaign_refs", "affected_material_sku_refs")
	case TypeCategoryPromotionPlan, TypeCategoryAssortmentRationalization:
		return set("intent", "affected_material_sku_refs")
	default:
		return map[string]bool{}
	}
}

func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

func Fingerprint(p Proposal) string {
	return strings.Join([]string{
		p.ActionType, string(p.Target.Kind), p.Target.ID, paramFingerprint(p.Parameters),
	}, "|")
}

func paramFingerprint(p Parameters) string {
	b, _ := jsonCanon(ParametersWireFrom(p))
	return string(b)
}
