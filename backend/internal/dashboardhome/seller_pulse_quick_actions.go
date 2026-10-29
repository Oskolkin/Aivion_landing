package dashboardhome

import "strings"

func repairSellerPulseQuickActionKinds(n *SellerPulseNarrative, topActions []TopActionItem, resp *Response) {
	if n == nil {
		return
	}
	for i := range n.QuickActions {
		action := &n.QuickActions[i]
		before := action.Kind
		action.Kind = inferSellerPulseQuickActionKind(action.Label, action.Kind, topActions)
		if action.Kind != before && resp != nil {
			recordNarrativeApplyRepair(resp, "seller_pulse.quick_actions["+itoa(i)+"].kind:"+before+"->"+action.Kind)
		}
		if href, ok := sellerPulseAllowedKinds[action.Kind]; ok && action.Href == "" && href != "" {
			action.Href = href
		}
	}
}

func inferSellerPulseQuickActionKind(label, kind string, topActions []TopActionItem) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	labelLower := strings.ToLower(strings.TrimSpace(label))
	if matchesSlowTurnoverActionText(labelLower) {
		return "slow_turnover"
	}
	for _, top := range topActions {
		if !strings.EqualFold(strings.TrimSpace(top.Title), strings.TrimSpace(label)) {
			continue
		}
		inferred, _ := sellerPulseKindForTopAction(top.Category)
		if top.Category == "slow_turnover" {
			return "slow_turnover"
		}
		if kind == "" || kind == "monitoring" {
			return inferred
		}
	}
	return kind
}

func matchesSlowTurnoverActionText(text string) bool {
	for _, token := range []string{
		"без заказов", "зависш", "оборачива", "медленн", "излиш",
		"крупным остатком", "крупный остаток", "no sales", "slow turnover",
	} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}
