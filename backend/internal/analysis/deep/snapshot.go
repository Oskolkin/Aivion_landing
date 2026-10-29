package deep

import (
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

func RebuildSnapshot(p PayloadV1) actioncatalog.Snapshot {
	snap := actioncatalog.Snapshot{
		Capabilities: map[string]string{},
		SKU:          map[string]actioncatalog.SKUState{},
		Category:     map[string]actioncatalog.CategoryState{},
		Campaign:     map[string]actioncatalog.CampaignState{},
		MainIDs:      map[string]bool{},
	}
	for _, c := range p.CapabilitiesAndLimitations {
		snap.Capabilities[c.Key] = c.Readiness
	}
	for _, e := range p.ThreadEntityContext {
		k := string(e.Entity.Kind) + "|" + e.Entity.ID
		if e.Role == RoleMain {
			snap.MainIDs[k] = true
		}
	}
	acc := p.AccountContext.Entity
	if acc.ID != "" {
		snap.MainIDs[string(acc.Kind)+"|"+acc.ID] = true
	}
	for _, it := range p.EvidenceRegistry {
		if it.Subject.Kind == discovery.KindSKU {
			st := snap.SKU[it.Subject.ID]
			if strings.Contains(it.EvidenceType, "PRICE") {
				st.HasPrice = true
			}
			if strings.Contains(it.EvidenceType, "STOCK") {
				st.HasStockContext = true
			}
			if strings.Contains(it.EvidenceType, "SALES") {
				st.HasDemandContext = true
			}
			if strings.Contains(it.EvidenceType, "SUPPLY") {
				st.HasReplenishmentContext = true
			}
			snap.SKU[it.Subject.ID] = st
		}
	}
	for _, item := range p.ActionMenu.Items {
		for _, c := range item.HardConstraints {
			if strings.HasPrefix(c, "current_price:") {
				rest := strings.TrimPrefix(c, "current_price:")
				parts := strings.SplitN(rest, "=", 2)
				if len(parts) != 2 {
					continue
				}
				id := parts[0]
				amtCur := parts[1]
				st := snap.SKU[id]
				st.HasPrice = true
				st.CurrentPrice = parseAmtCur(amtCur)
				snap.SKU[id] = st
			}
		}
		if item.TargetKind == "SKU" {
			for _, id := range item.EligibleTargetIDs {
				st := snap.SKU[id]
				switch item.ActionType {
				case actioncatalog.TypeSKUIncreaseReplenishment:
					st.HasDemandContext = true
				case actioncatalog.TypeSKUReduceReplenishment:
					st.HasReplenishmentContext = true
				}
				snap.SKU[id] = st
			}
		}
	}
	return snap
}

func parseAmtCur(s string) *actioncatalog.Money {
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
	return &actioncatalog.Money{Amount: amt, Currency: cur}
}
