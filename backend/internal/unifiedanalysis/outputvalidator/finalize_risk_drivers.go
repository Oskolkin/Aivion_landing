package outputvalidator

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// FinalizeRiskDriversResult summarizes deterministic risk_drivers repairs.
type FinalizeRiskDriversResult struct {
	RewrittenComposite int
	MergedDuplicates   int
	Unresolved         []string
}

// Canonical health_policy_v1 driver codes. Kept local to avoid an import
// cycle with dashboardhome/healthpolicy (which pulls unifiedanalysis).
// Must stay identical to healthpolicy.KnownDriverIDs — see
// TestRiskDriverRegistry_MatchesHealthPolicy.
const (
	driverSalesDecline               = "sales_decline"
	driverRevenueDecline             = "revenue_decline"
	driverKeySKUStockRisk            = "key_sku_stock_risk"
	driverStockout                   = "stockout"
	driverNoSalesWithStock           = "no_sales_with_stock"
	driverSurplus                    = "surplus"
	driverViewToCartDecline          = "view_to_cart_decline"
	driverCartToOrderDecline         = "cart_to_order_decline"
	driverHighDRRCampaign            = "high_drr_campaign"
	driverCampaignSpendWithoutOrders = "campaign_spend_without_orders"
	driverAdShareGrowth              = "ad_share_growth"
	driverDataLimitation             = "data_limitation"
)

// CanonicalRiskDriverIDs mirrors healthpolicy.KnownDriverIDs.
var CanonicalRiskDriverIDs = []string{
	driverSalesDecline,
	driverRevenueDecline,
	driverKeySKUStockRisk,
	driverStockout,
	driverNoSalesWithStock,
	driverSurplus,
	driverViewToCartDecline,
	driverCartToOrderDecline,
	driverHighDRRCampaign,
	driverCampaignSpendWithoutOrders,
	driverAdShareGrowth,
	driverDataLimitation,
}

var canonicalRiskDriverIDSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalRiskDriverIDs))
	for _, id := range CanonicalRiskDriverIDs {
		set[id] = struct{}{}
	}
	return set
}()

// IsCanonicalRiskDriverID reports whether id is a closed health_policy_v1 code.
func IsCanonicalRiskDriverID(id string) bool {
	_, ok := canonicalRiskDriverIDSet[strings.TrimSpace(id)]
	return ok
}

// Common AI aliases for canonical registry codes. Keys are exact prefix
// strings the model sometimes emits instead of the registry code (never
// entity-suffixed forms).
var riskDriverIDAliases = map[string]string{
	"high_drr":                       driverHighDRRCampaign,
	"high_drr_campaign":              driverHighDRRCampaign,
	"risk_ads_high_drr":              driverHighDRRCampaign,
	"low_stock":                      driverKeySKUStockRisk,
	"low_stock_coverage":             driverKeySKUStockRisk,
	"stock_risk":                     driverKeySKUStockRisk,
	"stock_deficit":                  driverKeySKUStockRisk,
	"risk_stock_deficit":             driverKeySKUStockRisk,
	"key_sku_stock_risk":             driverKeySKUStockRisk,
	"out_of_stock":                   driverStockout,
	"stockout":                       driverStockout,
	"no_sales_with_stock":            driverNoSalesWithStock,
	"risk_stock_no_sales_with_stock": driverNoSalesWithStock,
	"ad_spend_no_orders":             driverCampaignSpendWithoutOrders,
	"campaign_no_orders":             driverCampaignSpendWithoutOrders,
	"ad_share_growth":                driverAdShareGrowth,
	"sales_decline":                  driverSalesDecline,
	"revenue_decline":                driverRevenueDecline,
	"view_to_cart_decline":           driverViewToCartDecline,
	"cart_to_order_decline":          driverCartToOrderDecline,
	"surplus":                        driverSurplus,
	"data_limitation":                driverDataLimitation,
}

// FinalizeDashboardRiskDrivers rewrites AI account_summary.risk_drivers so
// every retained id is a canonical health_policy_v1 registry code. Composite
// forms like "high_drr_<campaign_id>" are split only when the suffix exactly
// matches an entity already present on the driver (entity_ids or fact_ids)
// and the prefix resolves to a KnownDriverID (directly or via alias).
//
// Schema v10 already separates id (type code) from entity_ids — this never
// invents new registry codes or promotes unknown prefixes.
func FinalizeDashboardRiskDrivers(out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) FinalizeRiskDriversResult {
	var res FinalizeRiskDriversResult
	if out == nil {
		return res
	}
	drivers := out.Dashboard.AccountSummary.RiskDrivers
	if len(drivers) == 0 {
		return res
	}

	normalized := make([]contract.UnifiedRiskDriverV1, 0, len(drivers))
	// Instance key = canonical code + sorted entity_ids. Distinct entities
	// keep separate instances of the same registry code (PASS: two high_drr
	// campaigns). Exact duplicates are merged (FAIL: duplicate instance).
	byInstance := map[string]int{}

	for i := range drivers {
		d := drivers[i]
		origID := strings.TrimSpace(d.ID)
		entities := collectRiskDriverEntities(d)
		code, rewritten, unresolved := resolveRiskDriverID(origID, entities, d, idx)
		if unresolved {
			res.Unresolved = append(res.Unresolved, origID)
			// Keep original so strict validation / bundle gate can fail clearly.
			normalized = append(normalized, d)
			continue
		}
		if rewritten {
			res.RewrittenComposite++
		}
		d.ID = code
		d.EntityIDs = mergeUniqueStrings(d.EntityIDs, entities)
		instanceKey := riskDriverInstanceKey(code, d.EntityIDs)
		if prev, ok := byInstance[instanceKey]; ok {
			res.MergedDuplicates++
			normalized[prev].FactIDs = mergeUniqueStrings(normalized[prev].FactIDs, d.FactIDs)
			continue
		}
		byInstance[instanceKey] = len(normalized)
		normalized = append(normalized, d)
	}
	out.Dashboard.AccountSummary.RiskDrivers = normalized
	return res
}

func riskDriverInstanceKey(code string, entityIDs []string) string {
	ents := append([]string{}, entityIDs...)
	sort.Strings(ents)
	return code + "|" + strings.Join(ents, ",")
}

func resolveRiskDriverID(
	id string,
	entities []string,
	driver contract.UnifiedRiskDriverV1,
	idx *PackReferenceIndex,
) (code string, rewritten bool, unresolved bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", false, true
	}
	if IsCanonicalRiskDriverID(id) {
		return id, false, false
	}
	if mapped := mapDriverAlias(id); mapped != "" {
		return mapped, true, false
	}

	// Longest entity-suffix match (deterministic: sort entities descending by length).
	ents := append([]string{}, entities...)
	sort.SliceStable(ents, func(i, j int) bool {
		if len(ents[i]) != len(ents[j]) {
			return len(ents[i]) > len(ents[j])
		}
		return ents[i] < ents[j]
	})

	type candidate struct {
		code   string
		entity string
	}
	var matches []candidate
	for _, ent := range ents {
		ent = strings.TrimSpace(ent)
		if ent == "" {
			continue
		}
		suffix := "_" + ent
		if !strings.HasSuffix(id, suffix) {
			continue
		}
		prefix := strings.TrimSpace(strings.TrimSuffix(id, suffix))
		if prefix == "" || strings.Contains(prefix, " ") {
			continue
		}
		resolved := mapDriverAlias(prefix)
		if resolved == "" && IsCanonicalRiskDriverID(prefix) {
			resolved = prefix
		}
		if resolved == "" {
			continue
		}
		if !entityCompatibleWithDriver(resolved, ent, driver, idx) {
			continue
		}
		matches = append(matches, candidate{code: resolved, entity: ent})
	}
	if len(matches) == 0 {
		return id, false, true
	}
	// Ambiguous if two different codes match.
	codeSet := map[string]struct{}{}
	for _, m := range matches {
		codeSet[m.code] = struct{}{}
	}
	if len(codeSet) != 1 {
		return id, false, true
	}
	return matches[0].code, true, false
}

func mapDriverAlias(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if IsCanonicalRiskDriverID(raw) {
		return raw
	}
	if mapped, ok := riskDriverIDAliases[raw]; ok && IsCanonicalRiskDriverID(mapped) {
		return mapped
	}
	return ""
}

func collectRiskDriverEntities(d contract.UnifiedRiskDriverV1) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range d.EntityIDs {
		add(id)
	}
	for _, fact := range d.FactIDs {
		if ent := entityFromFactID(fact); ent != "" {
			add(ent)
		}
	}
	return out
}

func entityFromFactID(factID string) string {
	factID = strings.TrimSpace(factID)
	parts := strings.Split(factID, ":")
	if len(parts) < 2 {
		return ""
	}
	switch parts[0] {
	case "sku", "campaign":
		return strings.TrimSpace(parts[1])
	default:
		return ""
	}
}

func entityCompatibleWithDriver(code, entity string, driver contract.UnifiedRiskDriverV1, idx *PackReferenceIndex) bool {
	entity = strings.TrimSpace(entity)
	if entity == "" {
		return false
	}
	switch code {
	case driverHighDRRCampaign, driverCampaignSpendWithoutOrders:
		if idx != nil {
			return idx.HasCampaign(entity) && factScopeIs(driver.FactIDs, "campaign", entity)
		}
		return factScopeIs(driver.FactIDs, "campaign", entity)
	case driverKeySKUStockRisk, driverStockout, driverNoSalesWithStock, driverSurplus:
		if idx != nil {
			return idx.HasSKU(entity) && factScopeIs(driver.FactIDs, "sku", entity)
		}
		return factScopeIs(driver.FactIDs, "sku", entity)
	default:
		// Account-level drivers should not carry entity-suffixed composite IDs.
		return false
	}
}

func factScopeIs(factIDs []string, scope, entity string) bool {
	prefix := scope + ":" + entity + ":"
	alt := scope + ":" + entity
	for _, f := range factIDs {
		f = strings.TrimSpace(f)
		if f == alt || strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

func mergeUniqueStrings(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, xs := range [][]string{a, b} {
		for _, x := range xs {
			x = strings.TrimSpace(x)
			if x == "" {
				continue
			}
			if _, ok := seen[x]; ok {
				continue
			}
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}

// validateRiskDriverRegistryIDs requires every risk_drivers[].id to be a
// canonical healthpolicy registry code after finalization.
func validateRiskDriverRegistryIDs(report *Report, out *contract.UnifiedAnalysisOutputV1) {
	if out == nil || report == nil {
		return
	}
	seen := map[string]struct{}{}
	for i, d := range out.Dashboard.AccountSummary.RiskDrivers {
		path := "$.dashboard.account_summary.risk_drivers[" + strconv.Itoa(i) + "].id"
		id := strings.TrimSpace(d.ID)
		if id == "" {
			report.addError(CodeEmptyRequiredText, path, "risk driver id is empty")
			continue
		}
		if !IsCanonicalRiskDriverID(id) {
			report.addError(CodeRiskDriverUnknownID, path,
				"risk_drivers id must be a canonical healthpolicy registry code; got "+id)
			continue
		}
		key := riskDriverInstanceKey(id, d.EntityIDs)
		if _, dup := seen[key]; dup {
			report.addError(CodeRiskDriverDuplicateInstance, path,
				"duplicate risk driver instance for code+entities")
			continue
		}
		seen[key] = struct{}{}
	}
}
