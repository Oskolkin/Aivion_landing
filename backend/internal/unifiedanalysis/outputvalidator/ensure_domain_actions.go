package outputvalidator

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Exclusion reason codes for material candidates that were deliberately not
// turned into a backfilled Action. These are never surfaced to the seller;
// they exist purely for audit/debugging (logs, ExclusionReason slice).
const (
	ExclusionDuplicateScope    = "duplicate_scope"
	ExclusionLowMateriality    = "low_materiality"
	ExclusionCoveredByExisting = "covered_by_existing_action"
	ExclusionInsufficientData  = "insufficient_data"
	ExclusionNonActionable     = "non_actionable"
	ExclusionActionCapReached  = "action_cap_reached"
)

// ExclusionReason records why a material domain/candidate set was not
// turned into a backfilled Action. Purely informational — never persisted
// into the schema-frozen UnifiedAnalysisOutputV1.
type ExclusionReason struct {
	Domain     string   `json:"domain"`
	ActionType string   `json:"action_type"`
	ScopeType  string   `json:"scope_type"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
	Code       string   `json:"code"`
	Detail     string   `json:"detail"`
}

// EnsureDomainActionsResult summarizes deterministic material-domain backfill.
type EnsureDomainActionsResult struct {
	AddedActionIDs []string
	Exclusions     []ExclusionReason
}

// Deterministic thresholds for material-domain candidate detection. These
// mirror the canonical stock/advertising policy already used elsewhere in
// the pipeline (stockpolicy, contract threshold_claims) so the backfill
// never invents its own business rules.
const (
	// highDRRThresholdPct matches the canonical high_drr_campaign_threshold
	// policy (campaign_drr_pct > 80) referenced by UnifiedThresholdClaimV1.
	highDRRThresholdPct = 80.0
	// noSalesWithStockMinDays is lower than stockpolicy.PersistentNoSalesMinDays
	// (14d, which drives the canonical "no_sales" stock_status) — this domain
	// exists to catch material no-sales-with-stock problems earlier.
	noSalesWithStockMinDays = 5
	// salesDeclineMinPrevOrders/salesDeclineMinDropRatio require a real prior
	// baseline and a material (not noisy) week-over-week order drop.
	salesDeclineMinPrevOrders = 3
	salesDeclineMinDropRatio  = 0.3
	// surplusMinCandidates/surplusMinUnits gate the surplus domain so a
	// single small overstocked SKU never becomes an Action on its own.
	surplusMinCandidates = 2
	surplusMinUnits      = 30
	// maxEntitiesPerDomainAction caps how many SKUs/campaigns one backfilled
	// Action cites, keeping titles/evidence readable.
	maxEntitiesPerDomainAction = 8
	// domainBackfillSoftCap is the "prefer to stop here" target from the
	// spec (8–12 Actions). Only non-hard-required (lower priority) domains
	// respect it; material stock/ads/sales-decline domains are always
	// represented up to the hard contract.MaxActionsV1 cap.
	domainBackfillSoftCap = 12
)

type domainCandidate struct {
	entityID  string // bare id (no "sku:"/"campaign:" prefix)
	scopeType string
	// sortKey orders candidates by severity/materiality (higher = more material).
	sortKey float64
}

// materialDomain is one deterministic Action-shaped bucket of Pack-grounded
// evidence. detectMaterialDomains never invents ids — every candidate is a
// SKU/campaign that already exists in the Pack reference index.
type materialDomain struct {
	key           string
	actionType    string
	category      string
	priority      string
	executionTier string
	hardRequired  bool // enforced by validateDomainCoverage when uncovered
	candidates    []domainCandidate
	// context carries domain-specific numbers used for title/problem text.
	context materialDomainContext
}

type materialDomainContext struct {
	oosCount      int
	criticalCount int
	shortCount    int
	lostRevenue   float64
	maxDropRatio  float64
	maxDRR        float64
	totalSpend    float64
	maxDaysNoSale int64
	surplusUnits  int64
}

// detectMaterialDomains derives every material-domain candidate bucket from
// the Pack reference index. It is pure (never mutates idx/out) so it can be
// reused both by EnsureMaterialDomainActions (backfill) and by
// validateDomainCoverage (completeness safety net).
func detectMaterialDomains(idx *PackReferenceIndex) []materialDomain {
	if idx == nil {
		return nil
	}
	domains := make([]materialDomain, 0, 10)
	// Stock urgency bands are independent operational workflows — keep them
	// as separate domains so one AI stock_replenishment Action does not hide
	// remaining uncovered OOS / critical / short SKUs.
	domains = append(domains, detectStockBandDomains(idx)...)
	if d, ok := detectSalesDeclineDomain(idx); ok {
		domains = append(domains, d)
	}
	if d, ok := detectHighDRRDomain(idx); ok {
		domains = append(domains, d)
	}
	if d, ok := detectSpendNoOrdersDomain(idx); ok {
		domains = append(domains, d)
	}
	domains = append(domains, detectNoSalesBandDomains(idx)...)
	if d, ok := detectSurplusDomain(idx); ok {
		domains = append(domains, d)
	}
	// key_sku_protection is an umbrella over stock-constrained SKUs and must
	// not publish alongside mutually exclusive stock band Actions.
	hasStockBand := false
	for _, existing := range domains {
		if strings.HasPrefix(existing.key, "stock_") {
			hasStockBand = true
			break
		}
	}
	if !hasStockBand {
		if d, ok := detectKeySKUProtectionDomain(idx); ok {
			domains = append(domains, d)
		}
	}
	return domains
}

func hasRecentDemand(f SKUFactRow) bool {
	if f.Orders7D > 0 || f.OrdersPrev7D > 0 {
		return true
	}
	if f.AvgDailySales7D != nil && *f.AvgDailySales7D > 0 {
		return true
	}
	if f.AvgDailySales30D != nil && *f.AvgDailySales30D > 0 {
		return true
	}
	return false
}

// detectStockBandDomains returns independent stock operational scopes:
// out-of-stock, critical (0–3d), and short (4–7d). Each band may become its
// own Action when uncovered entities remain.
func detectStockBandDomains(idx *PackReferenceIndex) []materialDomain {
	var oos, critical, short []domainCandidate
	for _, id := range idx.SortedSKUIDs() {
		f := idx.SKUFacts[id]
		if !f.HasStockData {
			continue
		}
		switch f.StockStatus {
		case stockpolicy.StatusOutOfStock:
			if !hasRecentDemand(f) {
				continue
			}
			oos = append(oos, domainCandidate{entityID: id, scopeType: "sku", sortKey: 3 + float64(f.Revenue7D)/1e6})
		case stockpolicy.StatusCriticalShortage:
			cov := 999.0
			if f.CoverageDays7D != nil {
				cov = *f.CoverageDays7D
			}
			critical = append(critical, domainCandidate{entityID: id, scopeType: "sku", sortKey: 2 - cov/100 + float64(f.Revenue7D)/1e6})
		case stockpolicy.StatusShortage7D:
			cov := 999.0
			if f.CoverageDays7D != nil {
				cov = *f.CoverageDays7D
			}
			short = append(short, domainCandidate{entityID: id, scopeType: "sku", sortKey: -cov + float64(f.Revenue7D)/1e6})
		}
	}
	sortCandidatesDesc(oos)
	sortCandidatesDesc(critical)
	sortCandidatesDesc(short)

	out := make([]materialDomain, 0, 3)
	if len(oos) > 0 {
		out = append(out, materialDomain{
			key: "stock_oos", actionType: "stock_replenishment", category: "stock",
			priority: "critical", executionTier: contract.ExecutionTierRequired, hardRequired: true,
			candidates: oos, context: materialDomainContext{oosCount: len(oos)},
		})
	}
	if len(critical) > 0 {
		out = append(out, materialDomain{
			key: "stock_critical", actionType: "stock_replenishment", category: "stock",
			priority: "critical", executionTier: contract.ExecutionTierRequired, hardRequired: true,
			candidates: critical, context: materialDomainContext{criticalCount: len(critical)},
		})
	}
	if len(short) > 0 {
		out = append(out, materialDomain{
			key: "stock_short", actionType: "stock_replenishment", category: "stock",
			priority: "important", executionTier: contract.ExecutionTierRecommended, hardRequired: false,
			candidates: short, context: materialDomainContext{shortCount: len(short)},
		})
	}
	return out
}

// detectSalesDeclineDomain finds SKUs with a material week-over-week order
// drop and a real (non-zero) lost-revenue delta. SKUs whose drop is already
// explained by a stock constraint (out-of-stock/critical/shortage) are
// excluded — that is the stock domain's problem, not an independent demand
// decline, and double-flagging it would create a redundant Action for the
// same root cause.
func detectSalesDeclineDomain(idx *PackReferenceIndex) (materialDomain, bool) {
	var candidates []domainCandidate
	var lostRevenue float64
	var maxRatio float64
	for _, id := range idx.SortedSKUIDs() {
		f := idx.SKUFacts[id]
		if f.HasStockData && isStockConstrainedStatus(f.StockStatus) {
			continue
		}
		if f.OrdersPrev7D < salesDeclineMinPrevOrders || f.Orders7D >= f.OrdersPrev7D {
			continue
		}
		ratio := float64(f.OrdersPrev7D-f.Orders7D) / float64(f.OrdersPrev7D)
		if ratio < salesDeclineMinDropRatio {
			continue
		}
		delta := f.RevenuePrev7D - f.Revenue7D
		if delta <= 0 {
			continue
		}
		lostRevenue += delta
		if ratio > maxRatio {
			maxRatio = ratio
		}
		candidates = append(candidates, domainCandidate{entityID: id, scopeType: "sku", sortKey: delta})
	}
	if len(candidates) == 0 {
		return materialDomain{}, false
	}
	sortCandidatesDesc(candidates)
	priority := "important"
	if maxRatio >= 0.6 {
		priority = "critical"
	}
	return materialDomain{
		key:           "sales_decline",
		actionType:    "sku_orders_drop",
		category:      "sku",
		priority:      priority,
		executionTier: contract.ExecutionTierRequired,
		hardRequired:  true,
		candidates:    candidates,
		context:       materialDomainContext{lostRevenue: lostRevenue, maxDropRatio: maxRatio},
	}, true
}

// detectHighDRRDomain finds campaigns spending against real orders but with
// DRR above the canonical high_drr_campaign_threshold policy (>80%).
func detectHighDRRDomain(idx *PackReferenceIndex) (materialDomain, bool) {
	var candidates []domainCandidate
	var maxDRR, totalSpend float64
	for _, id := range idx.SortedCampaignIDs() {
		f := idx.CampaignFacts[id]
		if f.DRRPct == nil || *f.DRRPct <= highDRRThresholdPct || f.Orders7D <= 0 {
			continue
		}
		candidates = append(candidates, domainCandidate{entityID: id, scopeType: "campaign", sortKey: *f.DRRPct})
		if *f.DRRPct > maxDRR {
			maxDRR = *f.DRRPct
		}
		totalSpend += f.Spend7D
	}
	if len(candidates) == 0 {
		return materialDomain{}, false
	}
	sortCandidatesDesc(candidates)
	priority := "important"
	if maxDRR > 120 {
		priority = "critical"
	}
	return materialDomain{
		key:           "ads_high_drr",
		actionType:    "ad_high_drr_review",
		category:      "advertising",
		priority:      priority,
		executionTier: contract.ExecutionTierRequired,
		hardRequired:  true,
		candidates:    candidates,
		context:       materialDomainContext{maxDRR: maxDRR, totalSpend: totalSpend},
	}, true
}

// detectSpendNoOrdersDomain finds campaigns burning budget with zero orders.
func detectSpendNoOrdersDomain(idx *PackReferenceIndex) (materialDomain, bool) {
	var candidates []domainCandidate
	var totalSpend float64
	for _, id := range idx.SortedCampaignIDs() {
		f := idx.CampaignFacts[id]
		if f.Spend7D <= 0 || f.Orders7D != 0 {
			continue
		}
		candidates = append(candidates, domainCandidate{entityID: id, scopeType: "campaign", sortKey: f.Spend7D})
		totalSpend += f.Spend7D
	}
	if len(candidates) == 0 {
		return materialDomain{}, false
	}
	sortCandidatesDesc(candidates)
	return materialDomain{
		key:           "ads_spend_no_orders",
		actionType:    "ad_spend_no_orders",
		category:      "advertising",
		priority:      "important",
		executionTier: contract.ExecutionTierRequired,
		hardRequired:  true,
		candidates:    candidates,
		context:       materialDomainContext{totalSpend: totalSpend},
	}, true
}

// detectNoSalesBandDomains splits slow inventory into independent workflows:
// persistent 14+ days vs early attention 5–13 days.
func detectNoSalesBandDomains(idx *PackReferenceIndex) []materialDomain {
	var persistent, early []domainCandidate
	var maxPersistent, maxEarly int64
	for _, id := range idx.SortedSKUIDs() {
		f := idx.SKUFacts[id]
		if !f.HasStockData || f.AvailableStock <= 0 || !f.HasDaysWithoutSales {
			continue
		}
		if f.DaysWithoutSales < noSalesWithStockMinDays {
			continue
		}
		c := domainCandidate{
			entityID: id, scopeType: "sku",
			sortKey: float64(f.DaysWithoutSales)*1000 + float64(f.AvailableStock),
		}
		if f.DaysWithoutSales >= 14 {
			persistent = append(persistent, c)
			if f.DaysWithoutSales > maxPersistent {
				maxPersistent = f.DaysWithoutSales
			}
			continue
		}
		early = append(early, c)
		if f.DaysWithoutSales > maxEarly {
			maxEarly = f.DaysWithoutSales
		}
	}
	sortCandidatesDesc(persistent)
	sortCandidatesDesc(early)
	out := make([]materialDomain, 0, 2)
	if len(persistent) > 0 {
		out = append(out, materialDomain{
			key: "no_sales_persistent", actionType: "no_sales_with_stock", category: "slow_turnover",
			priority: "important", executionTier: contract.ExecutionTierRecommended, hardRequired: false,
			candidates: persistent, context: materialDomainContext{maxDaysNoSale: maxPersistent},
		})
	}
	if len(early) > 0 {
		out = append(out, materialDomain{
			key: "no_sales_early", actionType: "no_sales_with_stock", category: "slow_turnover",
			priority: "watch", executionTier: contract.ExecutionTierRecommended, hardRequired: false,
			candidates: early, context: materialDomainContext{maxDaysNoSale: maxEarly},
		})
	}
	return out
}

// detectKeySKUProtectionDomain protects revenue-concentrated SKUs that also
// carry stock risk (coverage ≤7d or out of stock).
func detectKeySKUProtectionDomain(idx *PackReferenceIndex) (materialDomain, bool) {
	type ranked struct {
		id    string
		share float64
		rev   float64
	}
	var rankedSKUs []ranked
	var totalRev float64
	for _, id := range idx.SortedSKUIDs() {
		f := idx.SKUFacts[id]
		if f.Revenue7D <= 0 {
			continue
		}
		totalRev += f.Revenue7D
		rankedSKUs = append(rankedSKUs, ranked{id: id, rev: f.Revenue7D})
	}
	if totalRev <= 0 || len(rankedSKUs) == 0 {
		return materialDomain{}, false
	}
	sort.SliceStable(rankedSKUs, func(i, j int) bool {
		if rankedSKUs[i].rev != rankedSKUs[j].rev {
			return rankedSKUs[i].rev > rankedSKUs[j].rev
		}
		return rankedSKUs[i].id < rankedSKUs[j].id
	})
	var candidates []domainCandidate
	var cumShare float64
	for _, r := range rankedSKUs {
		share := r.rev / totalRev
		cumShare += share
		f := idx.SKUFacts[r.id]
		if !f.HasStockData || !isStockConstrainedStatus(f.StockStatus) {
			if cumShare >= 0.8 {
				break
			}
			continue
		}
		candidates = append(candidates, domainCandidate{
			entityID: r.id, scopeType: "sku", sortKey: r.rev,
		})
		if cumShare >= 0.8 && len(candidates) >= 2 {
			break
		}
		if len(candidates) >= maxEntitiesPerDomainAction {
			break
		}
	}
	if len(candidates) == 0 {
		return materialDomain{}, false
	}
	sortCandidatesDesc(candidates)
	return materialDomain{
		key: "key_sku_protection", actionType: "key_sku_protection", category: "sku",
		priority: "important", executionTier: contract.ExecutionTierRecommended, hardRequired: false,
		candidates: candidates,
	}, true
}

// detectSurplusDomain finds SKUs canonically flagged surplus_120d, gated so
// a single lightly-overstocked SKU is never treated as material alone.
func detectSurplusDomain(idx *PackReferenceIndex) (materialDomain, bool) {
	var candidates []domainCandidate
	var totalUnits int64
	for _, id := range idx.SortedSKUIDs() {
		f := idx.SKUFacts[id]
		if !f.HasStockData || !stockpolicy.IsSurplusStatus(f.StockStatus) {
			continue
		}
		candidates = append(candidates, domainCandidate{entityID: id, scopeType: "sku", sortKey: float64(f.AvailableStock)})
		totalUnits += f.AvailableStock
	}
	if len(candidates) == 0 {
		return materialDomain{}, false
	}
	if len(candidates) < surplusMinCandidates && totalUnits < surplusMinUnits {
		return materialDomain{}, false
	}
	sortCandidatesDesc(candidates)
	return materialDomain{
		key:           "surplus",
		actionType:    "stock_redistribution",
		category:      "stock",
		priority:      "watch",
		executionTier: contract.ExecutionTierRecommended,
		hardRequired:  false,
		candidates:    candidates,
		context:       materialDomainContext{surplusUnits: totalUnits},
	}, true
}

func isStockConstrainedStatus(status string) bool {
	switch status {
	case stockpolicy.StatusOutOfStock, stockpolicy.StatusCriticalShortage, stockpolicy.StatusShortage7D:
		return true
	default:
		return false
	}
}

func sortCandidatesDesc(cands []domainCandidate) {
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].sortKey != cands[j].sortKey {
			return cands[i].sortKey > cands[j].sortKey
		}
		return cands[i].entityID < cands[j].entityID
	})
}

// EnsureMaterialDomainActions deterministically backfills Actions for
// material Pack-grounded problems the AI left uncovered, so demo/production
// cabinets with multiple independent problems surface all of them (target
// 8–12 Actions, hard cap contract.MaxActionsV1=15) instead of just the 1-2
// the model happened to write about.
//
// It NEVER invents SKU/campaign ids (every entity comes from idx), NEVER
// reuses a SKU already owned by an existing Action (the hard "one Action per
// sku_id" rule), and NEVER pads low-materiality domains once a soft target
// is reached (domainBackfillSoftCap) — see ExclusionReason for the audit
// trail of every material candidate that was deliberately skipped.
//
// Must run after SanitizeGrounding/RepairUnsupportedContentSteps and before
// FinalizeTopActionIDs, since top_action_ids backfill selects among
// execution_tier=required Actions and must see the final Actions list.
func EnsureMaterialDomainActions(out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) EnsureDomainActionsResult {
	var res EnsureDomainActionsResult
	if out == nil || idx == nil {
		return res
	}
	// Only material_actions_found carries Actions at all; no_material_actions/
	// insufficient_data/analysis_failed must keep an empty actions array.
	if out.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return res
	}
	domains := detectMaterialDomains(idx)
	if len(domains) == 0 {
		return res
	}

	owned := ownedEntitySet(out.Actions)
	nextID := newActionIDSequence(out.Actions)

	for _, d := range domains {
		if len(d.candidates) == 0 {
			continue
		}
		fullIDs := prefixedEntityIDs(d.candidates)

		// Coverage is entity-based (not action-type). One AI stock Action that
		// only lists OOS SKUs must not suppress a separate critical/short Action
		// for the remaining uncovered entities — those are different workflows.
		fresh := make([]domainCandidate, 0, len(d.candidates))
		for _, c := range d.candidates {
			key := c.scopeType + ":" + c.entityID
			if _, dup := owned[key]; dup {
				continue
			}
			fresh = append(fresh, c)
		}
		if len(fresh) == 0 {
			res.Exclusions = append(res.Exclusions, ExclusionReason{
				Domain: d.key, ActionType: d.actionType, ScopeType: d.candidates[0].scopeType,
				EntityIDs: fullIDs, Code: ExclusionCoveredByExisting,
				Detail: "every candidate entity is already owned by another action",
			})
			continue
		}

		if !d.hardRequired && len(out.Actions) >= domainBackfillSoftCap {
			res.Exclusions = append(res.Exclusions, ExclusionReason{
				Domain: d.key, ActionType: d.actionType, ScopeType: d.candidates[0].scopeType,
				EntityIDs: prefixedEntityIDs(fresh), Code: ExclusionLowMateriality,
				Detail: fmt.Sprintf("soft cap of %d actions reached; domain is not hard-required", domainBackfillSoftCap),
			})
			continue
		}
		if len(out.Actions) >= contract.MaxActionsV1 {
			res.Exclusions = append(res.Exclusions, ExclusionReason{
				Domain: d.key, ActionType: d.actionType, ScopeType: d.candidates[0].scopeType,
				EntityIDs: prefixedEntityIDs(fresh), Code: ExclusionActionCapReached,
				Detail: fmt.Sprintf("contract.MaxActionsV1=%d reached", contract.MaxActionsV1),
			})
			continue
		}

		capped := fresh
		if len(capped) > maxEntitiesPerDomainAction {
			capped = capped[:maxEntitiesPerDomainAction]
		}
		action := buildDomainAction(d, capped, idx, nextID())
		out.Actions = append(out.Actions, action)
		res.AddedActionIDs = append(res.AddedActionIDs, action.AnalysisActionID)
		for _, c := range capped {
			owned[c.scopeType+":"+c.entityID] = struct{}{}
		}

		if len(capped) < len(fresh) {
			res.Exclusions = append(res.Exclusions, ExclusionReason{
				Domain: d.key, ActionType: d.actionType, ScopeType: d.candidates[0].scopeType,
				EntityIDs: prefixedEntityIDs(fresh[len(capped):]), Code: ExclusionLowMateriality,
				Detail: fmt.Sprintf("kept the %d most material entities per action", maxEntitiesPerDomainAction),
			})
		}
	}

	return res
}

// validateDomainCoverage is the completeness safety net for
// EnsureMaterialDomainActions: it re-detects material domains from the same
// Pack reference index and rejects publication only when a hard-required
// domain (stock OOS/critical, material sales decline, high-DRR advertising,
// spend-without-orders) has candidates that are entirely uncovered — no
// Action of the matching type exists, and none of the candidate entities is
// owned by any other Action. Lower-priority domains (stock_short,
// no_sales_with_stock, surplus) are advisory only and never block
// publication, matching EnsureMaterialDomainActions' soft-cap behavior.
//
// No-ops when idx is nil (unknown Pack facts) or the outcome is not
// material_actions_found, so it never affects fixtures/tests that validate
// without a reference index.
func validateDomainCoverage(report *Report, out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) {
	if idx == nil || out.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return
	}
	domains := detectMaterialDomains(idx)
	if len(domains) == 0 {
		return
	}
	owned := ownedEntitySet(out.Actions)

	for _, d := range domains {
		if !d.hardRequired || len(d.candidates) == 0 {
			continue
		}
		uncovered := 0
		for _, c := range d.candidates {
			if _, ok := owned[c.scopeType+":"+c.entityID]; !ok {
				uncovered++
			}
		}
		if uncovered == 0 {
			continue
		}
		report.addError(CodeActionDomainCoverageIncomplete, "$.actions",
			fmt.Sprintf("material domain %q has %d uncovered candidate(s) grounded in the Pack",
				d.key, uncovered))
	}
}

func prefixedEntityIDs(cands []domainCandidate) []string {
	ids := make([]string, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.scopeType+":"+c.entityID)
	}
	return ids
}

func actionTypesPresent(actions []contract.UnifiedActionV1) map[string]bool {
	present := make(map[string]bool, len(actions))
	for _, a := range actions {
		if t := strings.TrimSpace(a.Type); t != "" {
			present[t] = true
		}
	}
	return present
}

// ownedEntitySet returns the set of "scope_type:bare_id" keys already
// referenced by any existing Action, mirroring the hard "one Action per
// sku_id" rule enforced by Validate's skuOwner tracking.
func ownedEntitySet(actions []contract.UnifiedActionV1) map[string]struct{} {
	owned := make(map[string]struct{})
	for _, a := range actions {
		scopeType := strings.TrimSpace(a.Scope.ScopeType)
		for _, raw := range a.Scope.EntityIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			bare := id
			if idx := strings.IndexByte(id, ':'); idx >= 0 {
				bare = id[idx+1:]
			}
			if scopeType == "sku" || scopeType == "campaign" {
				owned[scopeType+":"+bare] = struct{}{}
			} else {
				// Unknown/mixed scope: register under both to stay conservative.
				owned["sku:"+bare] = struct{}{}
				owned["campaign:"+bare] = struct{}{}
			}
		}
	}
	return owned
}

var actionIDSeqRE = regexp.MustCompile(`^action_(\d{3,})$`)

// newActionIDSequence returns a generator of fresh "action_NNN" ids that
// never collides with existing ids (including non-numeric/legacy ones).
func newActionIDSequence(actions []contract.UnifiedActionV1) func() string {
	max := 0
	existing := make(map[string]struct{}, len(actions))
	for _, a := range actions {
		id := strings.TrimSpace(a.AnalysisActionID)
		existing[id] = struct{}{}
		if m := actionIDSeqRE.FindStringSubmatch(id); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	counter := max
	return func() string {
		for {
			counter++
			id := fmt.Sprintf("action_%03d", counter)
			if _, taken := existing[id]; !taken {
				existing[id] = struct{}{}
				return id
			}
		}
	}
}
