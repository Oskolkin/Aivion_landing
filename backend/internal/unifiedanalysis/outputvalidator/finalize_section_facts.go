package outputvalidator

import (
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// FinalizeSectionFactsResult summarizes section-aware fact reference repairs.
type FinalizeSectionFactsResult struct {
	DroppedPulseClaims  int
	DroppedPulseFactIDs int
	DroppedPulseItems   int
	DroppedMetricClaims int
	PolicyVersion       string
}

// FinalizeDashboardSectionFactReferences applies the closed SectionFactPolicy
// before strict validation. It never invents replacement facts or rewrites
// campaign metrics into account metrics.
//
// Seller Pulse: drop campaign/SKU/category fact_ids and claims; drop the whole
// pulse item when no account-level evidence remains or when free-text still
// asserts a percentage without a remaining claim.
func FinalizeDashboardSectionFactReferences(out *contract.UnifiedAnalysisOutputV1) FinalizeSectionFactsResult {
	res := FinalizeSectionFactsResult{PolicyVersion: SectionFactPolicyVersion}
	if out == nil {
		return res
	}
	n := out.Dashboard.SellerPulse.Narrative
	if n == nil {
		return res
	}

	filteredClaims := make([]contract.UnifiedMetricClaimV1, 0, len(n.MetricClaims))
	for _, c := range n.MetricClaims {
		if sellerPulseClaimAllowed(c) {
			filteredClaims = append(filteredClaims, c)
			continue
		}
		res.DroppedMetricClaims++
	}
	n.MetricClaims = filteredClaims

	var droppedItems int
	n.KeyPoints, droppedItems = finalizeSellerPulsePoints(n.KeyPoints, &res)
	res.DroppedPulseItems += droppedItems
	n.ImportantFacts, droppedItems = finalizeSellerPulsePoints(n.ImportantFacts, &res)
	res.DroppedPulseItems += droppedItems

	out.Dashboard.SellerPulse.Narrative = n
	return res
}

func finalizeSellerPulsePoints(items []contract.UnifiedSellerPulsePointV1, res *FinalizeSectionFactsResult) ([]contract.UnifiedSellerPulsePointV1, int) {
	if len(items) == 0 {
		return items, 0
	}
	out := make([]contract.UnifiedSellerPulsePointV1, 0, len(items))
	dropped := 0
	for _, item := range items {
		kept, dropFactIDs, dropClaims := filterSellerPulsePoint(item)
		res.DroppedPulseFactIDs += dropFactIDs
		res.DroppedPulseClaims += dropClaims
		if !sellerPulsePointRetainable(kept) {
			dropped++
			continue
		}
		out = append(out, kept)
	}
	return out, dropped
}

func filterSellerPulsePoint(item contract.UnifiedSellerPulsePointV1) (contract.UnifiedSellerPulsePointV1, int, int) {
	dropFactIDs := 0
	dropClaims := 0

	filteredFacts := make([]string, 0, len(item.FactIDs))
	for _, id := range item.FactIDs {
		if FactScopeAllowedInSection(SectionSellerPulse, ParseFactIDScope(id)) &&
			!IsSellerPulseForbiddenMetric(metricFromFactID(id)) {
			filteredFacts = append(filteredFacts, id)
			continue
		}
		dropFactIDs++
	}
	item.FactIDs = filteredFacts

	filteredClaims := make([]contract.UnifiedMetricClaimV1, 0, len(item.Claims))
	for _, c := range item.Claims {
		if sellerPulseClaimAllowed(c) {
			filteredClaims = append(filteredClaims, c)
			continue
		}
		dropClaims++
	}
	item.Claims = filteredClaims

	if IsSellerPulseForbiddenMetric(item.MetricCode) ||
		(item.Metric != "" && IsSellerPulseForbiddenMetric(item.Metric)) {
		item.MetricCode = ""
		item.Metric = ""
		item.Value = nil
	}

	// Drop campaign/SKU entity_ids when no remaining fact still references them.
	item.EntityIDs = filterEntityIDsAgainstFacts(item.EntityIDs, item.FactIDs)
	return item, dropFactIDs, dropClaims
}

func sellerPulseClaimAllowed(c contract.UnifiedMetricClaimV1) bool {
	if IsSellerPulseForbiddenMetric(c.MetricCode) {
		return false
	}
	scope := ParseFactIDScope(c.FactID)
	if c.FactID == "" {
		// Bare metric without fact_id: only allow when metric is not entity-forbidden.
		return !IsSellerPulseForbiddenMetric(c.MetricCode)
	}
	return FactScopeAllowedInSection(SectionSellerPulse, scope)
}

func sellerPulsePointRetainable(item contract.UnifiedSellerPulsePointV1) bool {
	hasAccountEvidence := false
	for _, id := range item.FactIDs {
		if ParseFactIDScope(id) == FactScopeAccount {
			hasAccountEvidence = true
			break
		}
	}
	if !hasAccountEvidence {
		for _, c := range item.Claims {
			if ParseFactIDScope(c.FactID) == FactScopeAccount {
				hasAccountEvidence = true
				break
			}
		}
	}
	if !hasAccountEvidence {
		return false
	}
	text := item.Title + " " + item.Explanation
	if textHasPercentAssertion(text) && len(item.Claims) == 0 && item.Value == nil {
		// Percentage left ungrounded after stripping entity facts.
		return false
	}
	return strings.TrimSpace(item.Title) != "" || strings.TrimSpace(item.Explanation) != ""
}

func textHasPercentAssertion(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.Contains(text, "%") || strings.Contains(text, "％") {
		return true
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "процент") || strings.Contains(lower, "pct") {
		return true
	}
	// Digits followed by optional space and % already covered; also "80 ДРР" style
	// without % is common — require explicit % / процент for this safety net.
	_ = unicode.IsDigit
	return false
}

func metricFromFactID(factID string) string {
	parts := strings.Split(strings.TrimSpace(factID), ":")
	if len(parts) < 3 {
		if len(parts) == 2 && strings.EqualFold(parts[0], "account") {
			return parts[1]
		}
		return ""
	}
	// account:metric:period OR campaign:id:metric
	switch strings.ToLower(parts[0]) {
	case "account", "seller", "store":
		return parts[1]
	case "campaign", "sku", "category":
		return parts[len(parts)-1]
	default:
		return parts[len(parts)-1]
	}
}

func filterEntityIDsAgainstFacts(entityIDs, factIDs []string) []string {
	if len(entityIDs) == 0 {
		return entityIDs
	}
	needed := map[string]struct{}{}
	for _, id := range factIDs {
		scope := ParseFactIDScope(id)
		if scope != FactScopeCampaign && scope != FactScopeSKU && scope != FactScopeCategory {
			continue
		}
		parts := strings.Split(id, ":")
		if len(parts) >= 2 {
			needed[strings.TrimSpace(parts[1])] = struct{}{}
		}
	}
	if len(needed) == 0 {
		// Account-only facts: drop leftover campaign/SKU entity refs.
		return nil
	}
	out := make([]string, 0, len(entityIDs))
	for _, e := range entityIDs {
		e = strings.TrimSpace(e)
		if _, ok := needed[e]; ok {
			out = append(out, e)
		}
	}
	return out
}
