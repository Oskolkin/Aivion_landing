package actioncatalog

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func jsonCanon(v any) ([]byte, error) { return canonicaljson.Marshal(v) }

func BuildMenu(snap Snapshot, mains []MainEntity) Menu {
	items := make([]MenuItem, 0, 19)
	for _, def := range Catalog() {
		if !capabilitiesOK(def, snap) {
			continue
		}
		ids := eligibleTargets(def, snap, mains)
		if len(ids) == 0 {
			continue
		}
		sort.Strings(ids)
		related := relatedIDs(def, snap, ids)
		items = append(items, MenuItem{
			ActionType:               def.ActionTypeKey,
			TargetKind:               string(def.TargetKind),
			EligibleTargetIDs:        ids,
			ParameterContract:        def.ValidationPolicy + ":" + def.ActionTypeKey,
			AllowedRelatedEntities:   append([]string(nil), def.AllowedRelatedEntities...),
			EligibleRelatedEntityIDs: related,
			HardConstraints:          append(hardConstraints(def), stateConstraints(def, snap, ids)...),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ActionType < items[j].ActionType })
	return Menu{Version: MenuPolicyV1, Catalog: CatalogV1, Items: items}
}

// MenuItemForContinuity builds a menu row that satisfies ValidateProposal for a
// previous action being revalidated on the same target.
func MenuItemForContinuity(def Definition, snap Snapshot, targetKind, targetID string) MenuItem {
	ids := []string{targetID}
	return MenuItem{
		ActionType:        def.ActionTypeKey,
		TargetKind:        targetKind,
		EligibleTargetIDs: ids,
		ParameterContract: def.ValidationPolicy + ":" + def.ActionTypeKey,
		HardConstraints:   append(hardConstraints(def), stateConstraints(def, snap, ids)...),
	}
}

func capabilitiesOK(def Definition, snap Snapshot) bool {
	if len(def.RequiredCapabilities) == 0 {
		return true
	}
	for _, need := range def.RequiredCapabilities {
		if hasCapability(snap, need) {
			return true
		}
	}
	return false
}

func hasCapability(snap Snapshot, family string) bool {
	fam := stringsUpper(family)
	for k, ready := range snap.Capabilities {
		if ready != "READY" && ready != "PARTIAL" && ready != "USABLE" {
			continue
		}
		if stringsContainsFold(k, fam) {
			return true
		}
	}
	return false
}

func eligibleTargets(def Definition, snap Snapshot, mains []MainEntity) []string {
	var ids []string
	for _, m := range mains {
		if m.Ref.Kind != def.TargetKind {
			continue
		}
		if !isMain(snap, string(m.Ref.Kind), m.Ref.ID) {
			continue
		}
		if targetEligible(def, snap, m.Ref) {
			ids = append(ids, m.Ref.ID)
		}
	}
	return ids
}

func targetEligible(def Definition, snap Snapshot, ref EntityRef) bool {
	for _, ctx := range def.RequiredFactualContext {
		if !contextOK(def, snap, ref, ctx) {
			return false
		}
	}
	return true
}

func contextOK(def Definition, snap Snapshot, ref EntityRef, ctx string) bool {
	switch ctx {
	case "PRICE_CURRENT":
		st := snap.SKU[ref.ID]
		return st.HasPrice && st.CurrentPrice != nil
	case "STOCK_OR_DEMAND":
		st := snap.SKU[ref.ID]
		return st.HasStockContext || st.HasDemandContext
	case "REPLENISHMENT_INBOUND":
		return snap.SKU[ref.ID].HasReplenishmentContext
	case "SKU_CAMPAIGN_RELATION":
		return len(snap.SKU[ref.ID].CampaignRefs) > 0
	case "BID_LIMITS":
		st := snap.SKU[ref.ID]
		if len(st.CampaignRefs) == 0 {
			return false
		}
		for _, c := range st.CampaignRefs {
			if st.BidKnownForCampaign[c] {
				return true
			}
		}
		return false
	case "PROMOTION_CANDIDATE":
		return len(snap.SKU[ref.ID].PromotionCandidates) > 0
	case "PROMOTION_PARTICIPANT":
		return len(snap.SKU[ref.ID].PromotionParticipants) > 0
	case "CATALOG_STATE":
		return snap.SKU[ref.ID].CatalogState == "ACTIVE" || snap.SKU[ref.ID].CatalogState == "ARCHIVED"
	case "CAMPAIGN_BUDGET":
		return snap.Campaign[ref.ID].Budget != nil
	case "CAMPAIGN_STATE":
		st := snap.Campaign[ref.ID].State
		return st == "ACTIVE" || st == "PAUSED"
	case "CAMPAIGN_STRATEGY_ENUM":
		return len(snap.Campaign[ref.ID].AllowedStrategies) > 0
	case "CAMPAIGN_PRODUCT_SET":
		return true
	case "ACCOUNT_HEALTH_KEYS":
		return len(snap.Account.RatingKeys) > 0
	case "CATEGORY_MATERIAL_SKUS":
		return len(snap.Category[ref.ID].MaterialSKURefs) > 0
	default:
		return false
	}
}

func relatedIDs(def Definition, snap Snapshot, targets []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, tid := range targets {
		switch def.TargetKind {
		case discovery.KindSKU:
			st := snap.SKU[tid]
			for _, c := range st.CampaignRefs {
				add(c)
			}
			for _, p := range st.PromotionCandidates {
				add(p)
			}
			for _, p := range st.PromotionParticipants {
				add(p)
			}
		case discovery.KindCategory:
			for _, s := range snap.Category[tid].MaterialSKURefs {
				add(s)
			}
			for _, c := range snap.Category[tid].MaterialCampaignRefs {
				add(c)
			}
		case discovery.KindCampaign:
			for _, s := range snap.Campaign[tid].ProductSKURefs {
				add(s)
			}
		}
	}
	sort.Strings(out)
	return out
}

func hardConstraints(def Definition) []string {
	switch def.ActionTypeKey {
	case TypeSKUAdjustPrice:
		return []string{"recommended_price>0", "same_currency", "not_equal_current", "respect_floor"}
	case TypeSKUJoinPromotion:
		return []string{"candidate_only", "not_participant"}
	case TypeSKULeavePromotion:
		return []string{"participant_only"}
	case TypeCampaignChangeState:
		return []string{"ACTIVE|PAUSED", "no_noop"}
	case TypeCampaignAdjustProductSet:
		return []string{"at_least_one_list", "no_intersection", "child_sku_main"}
	default:
		return []string{"catalog_v1"}
	}
}

func stateConstraints(def Definition, snap Snapshot, ids []string) []string {
	var out []string
	if def.ActionTypeKey == TypeSKUAdjustPrice {
		for _, id := range ids {
			st := snap.SKU[id]
			if st.CurrentPrice != nil {
				out = append(out, "current_price:"+id+"="+formatAmt(st.CurrentPrice.Amount)+st.CurrentPrice.Currency)
			}
		}
	}
	return out
}

func formatAmt(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}

// PriceHintFromMenu extracts observed current price encoded in menu hard constraints.
func PriceHintFromMenu(m Menu, targetKind, targetID string) *Money {
	if targetID == "" {
		return nil
	}
	prefix := "current_price:" + targetID + "="
	for _, item := range m.Items {
		for _, c := range item.HardConstraints {
			if strings.HasPrefix(c, prefix) {
				return parseConstraintMoney(strings.TrimPrefix(c, prefix))
			}
		}
	}
	return nil
}

func parseConstraintMoney(s string) *Money {
	i := 0
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
		i++
	}
	if i == 0 {
		return nil
	}
	amt, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return nil
	}
	cur := strings.TrimSpace(s[i:])
	if cur == "" {
		cur = "RUB"
	}
	return &Money{Amount: amt, Currency: cur}
}

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(a, b)
}

func stringsUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}

func stringsContainsFold(hay, needle string) bool {
	h, n := stringsUpper(hay), stringsUpper(needle)
	return len(n) > 0 && (h == n || containsSub(h, n))
}

func containsSub(h, n string) bool {
	return len(h) >= len(n) && (h == n || indexOf(h, n) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

func HashMenu(m Menu) string {
	return canonicaljson.Hash(m)
}
