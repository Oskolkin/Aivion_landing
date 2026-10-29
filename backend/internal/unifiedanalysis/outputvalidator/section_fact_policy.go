package outputvalidator

import (
	"strings"
)

// SectionFactPolicyVersion identifies the closed section↔fact-scope rules
// shared by prompt grounding hints, finalization, and strict validation.
const SectionFactPolicyVersion = "section_fact_policy_v1"

// FactScope is the entity aggregation level encoded in a Pack fact_id prefix.
type FactScope string

const (
	FactScopeAccount  FactScope = "account"
	FactScopeCampaign FactScope = "campaign"
	FactScopeSKU      FactScope = "sku"
	FactScopeCategory FactScope = "category"
	FactScopeUnknown  FactScope = "unknown"
)

// DashboardSection identifies a Unified Dashboard narrative area for grounding.
type DashboardSection string

const (
	SectionSellerPulse DashboardSection = "seller_pulse"
	SectionAdvertising DashboardSection = "advertising"
	SectionStock       DashboardSection = "stock"
	SectionFunnel      DashboardSection = "funnel"
	SectionSKUFocus    DashboardSection = "sku_focus"
	SectionCategory    DashboardSection = "category"
	SectionHealth      DashboardSection = "health"
	SectionWhatChanged DashboardSection = "what_changed"
	SectionActions     DashboardSection = "actions"
	SectionUnknown     DashboardSection = "unknown"
)

// ParseFactIDScope extracts the leading scope token from a Pack-style fact_id
// (account:…, campaign:…, sku:…, category:…).
func ParseFactIDScope(factID string) FactScope {
	factID = strings.TrimSpace(factID)
	if factID == "" {
		return FactScopeUnknown
	}
	prefix, _, ok := strings.Cut(factID, ":")
	if !ok {
		return FactScopeUnknown
	}
	switch strings.ToLower(strings.TrimSpace(prefix)) {
	case "account", "seller", "store":
		return FactScopeAccount
	case "campaign":
		return FactScopeCampaign
	case "sku":
		return FactScopeSKU
	case "category":
		return FactScopeCategory
	default:
		return FactScopeUnknown
	}
}

// AllowedFactScopesForSection returns the closed allowlist of fact scopes for
// a Dashboard section. Unknown sections intentionally return nil (deny-all
// for section-specific references) — never "allow all".
func AllowedFactScopesForSection(section DashboardSection) []FactScope {
	switch section {
	case SectionSellerPulse, SectionFunnel, SectionWhatChanged:
		return []FactScope{FactScopeAccount}
	case SectionAdvertising:
		return []FactScope{FactScopeAccount, FactScopeCampaign}
	case SectionStock:
		return []FactScope{FactScopeAccount, FactScopeSKU}
	case SectionSKUFocus:
		return []FactScope{FactScopeSKU, FactScopeAccount}
	case SectionCategory:
		return []FactScope{FactScopeCategory, FactScopeAccount}
	case SectionHealth, SectionActions:
		return []FactScope{FactScopeAccount, FactScopeCampaign, FactScopeSKU, FactScopeCategory}
	default:
		return nil
	}
}

// FactScopeAllowedInSection reports whether scope may appear in section.
func FactScopeAllowedInSection(section DashboardSection, scope FactScope) bool {
	if scope == FactScopeUnknown {
		return false
	}
	for _, allowed := range AllowedFactScopesForSection(section) {
		if allowed == scope {
			return true
		}
	}
	return false
}

// SellerPulseForbiddenMetricCodes are campaign/SKU/category metrics that must
// never ground Seller Pulse claims even if mis-prefixed as account:*.
// Built from the live advertising / stock registries used by Pack grounding.
var SellerPulseForbiddenMetricCodes = map[string]struct{}{
	"campaign_drr_pct":            {},
	"campaign_spend":              {},
	"campaign_attributed_revenue": {},
	"drr_pct":                     {}, // campaign_summary.csv column when used as campaign metric
	"sku_stock_days":              {},
	"stock_coverage_days_7d":      {},
	"sku_revenue":                 {},
	"sku_orders":                  {},
	"category_revenue":            {},
}

// IsSellerPulseForbiddenMetric reports metrics that are entity-level by nature.
func IsSellerPulseForbiddenMetric(metricCode string) bool {
	mc := strings.ToLower(strings.TrimSpace(metricCode))
	if mc == "" {
		return false
	}
	_, ok := SellerPulseForbiddenMetricCodes[mc]
	return ok
}

// FormatAllowedScopes joins scopes for error messages.
func FormatAllowedScopes(section DashboardSection) string {
	scopes := AllowedFactScopesForSection(section)
	if len(scopes) == 0 {
		return "(none — unknown section denies all)"
	}
	parts := make([]string, 0, len(scopes))
	for _, s := range scopes {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}
