package outputvalidator

import (
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// FinalizeTopActionsResult summarizes deterministic Top Action repairs applied
// before the hard publication contract. Repairs never invent Actions, never
// promote recommended → required, and never pad Top 3 with recommended IDs.
type FinalizeTopActionsResult struct {
	DroppedNonRequired  int
	DroppedUnknown      int
	DroppedDuplicates   int
	Backfilled          int
	RecommendedPromoted int // demoted-from-top recommended IDs appended to recommended_action_ids
	RerankedByMaterial  int
}

type actionIndexEntry struct {
	action contract.UnifiedActionV1
	order  int
}

// FinalizeTopActionIDs rewrites dashboard.top_action_ids so every retained ID
// references an existing execution_tier=required Action. Algorithm:
//  1. Index Actions by ID (first occurrence wins).
//  2. Collect unique existing required IDs from AI top_action_ids (order kept
//     only as a weak signal).
//  3. Backfill other required Actions when fewer than MaxTopActionsV1.
//  4. Re-rank the required pool by materiality (type + priority + confidence +
//     entity count) so high-impact stock shortage outranks low-priority
//     no-sales when both are required.
//  5. Cap at MaxTopActionsV1; never invent Actions or promote tiers.
//  6. Demoted recommended IDs stay in Actions and are appended to
//     recommended_action_ids without duplicates.
func FinalizeTopActionIDs(out *contract.UnifiedAnalysisOutputV1) FinalizeTopActionsResult {
	var res FinalizeTopActionsResult
	if out == nil {
		return res
	}

	byID := map[string]actionIndexEntry{}
	for i, action := range out.Actions {
		id := strings.TrimSpace(action.AnalysisActionID)
		if id == "" {
			continue
		}
		if _, exists := byID[id]; exists {
			continue
		}
		byID[id] = actionIndexEntry{action: action, order: i}
	}

	seen := map[string]struct{}{}
	pool := make([]string, 0, contract.MaxTopActionsV1*2)
	demotedRecommended := make([]string, 0)

	for _, raw := range out.Dashboard.TopActionIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			res.DroppedDuplicates++
			continue
		}
		entry, ok := byID[id]
		if !ok {
			res.DroppedUnknown++
			continue
		}
		seen[id] = struct{}{}
		tier := strings.TrimSpace(entry.action.ExecutionTier)
		if tier != contract.ExecutionTierRequired {
			res.DroppedNonRequired++
			if tier == contract.ExecutionTierRecommended {
				demotedRecommended = append(demotedRecommended, id)
			}
			continue
		}
		pool = append(pool, id)
	}

	aiOrder := append([]string(nil), pool...)
	backfill := requiredBackfillCandidates(byID, seen)
	for _, id := range backfill {
		pool = append(pool, id)
		seen[id] = struct{}{}
		res.Backfilled++
	}

	ranked := rankRequiredByMateriality(pool, byID)
	if !sameIDOrder(aiOrder, ranked) && len(ranked) > 0 {
		res.RerankedByMaterial++
	}
	if len(ranked) > contract.MaxTopActionsV1 {
		ranked = ranked[:contract.MaxTopActionsV1]
	}

	out.Dashboard.TopActionIDs = ranked
	res.RecommendedPromoted = mergeRecommendedActionIDs(out, demotedRecommended, ranked)
	return res
}

func requiredBackfillCandidates(byID map[string]actionIndexEntry, already map[string]struct{}) []string {
	type cand struct {
		id          string
		materiality float64
		priority    int
		order       int
	}
	cands := make([]cand, 0)
	for id, entry := range byID {
		if _, ok := already[id]; ok {
			continue
		}
		if strings.TrimSpace(entry.action.ExecutionTier) != contract.ExecutionTierRequired {
			continue
		}
		cands = append(cands, cand{
			id:          id,
			materiality: actionMaterialityScore(entry.action),
			priority:    priorityRank(entry.action.Priority),
			order:       entry.order,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].materiality != cands[j].materiality {
			return cands[i].materiality > cands[j].materiality
		}
		if cands[i].priority != cands[j].priority {
			return cands[i].priority < cands[j].priority
		}
		if cands[i].order != cands[j].order {
			return cands[i].order < cands[j].order
		}
		return cands[i].id < cands[j].id
	})
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.id)
	}
	return out
}

func rankRequiredByMateriality(ids []string, byID map[string]actionIndexEntry) []string {
	type cand struct {
		id          string
		materiality float64
		priority    int
		order       int
	}
	cands := make([]cand, 0, len(ids))
	for _, id := range ids {
		entry, ok := byID[id]
		if !ok {
			continue
		}
		cands = append(cands, cand{
			id:          id,
			materiality: actionMaterialityScore(entry.action),
			priority:    priorityRank(entry.action.Priority),
			order:       entry.order,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].materiality != cands[j].materiality {
			return cands[i].materiality > cands[j].materiality
		}
		if cands[i].priority != cands[j].priority {
			return cands[i].priority < cands[j].priority
		}
		if cands[i].order != cands[j].order {
			return cands[i].order < cands[j].order
		}
		return cands[i].id < cands[j].id
	})
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.id)
	}
	return out
}

// actionMaterialityScore ranks required Actions for Top 3 by business impact.
// Stock replenishment outranks low-priority frozen inventory when both are required.
func actionMaterialityScore(a contract.UnifiedActionV1) float64 {
	score := 0.0
	switch strings.TrimSpace(a.Type) {
	case "stock_replenishment", "key_sku_protection":
		score += 100
	case "ad_high_drr_review", "ad_spend_no_orders", "ad_budget_reallocation":
		score += 85
	case "sku_orders_drop", "sales_drop_review":
		score += 75
	case "funnel_weak_transition", "card_conversion_review", "category_underperform":
		score += 55
	case "no_sales_with_stock", "slow_turnover_review", "stock_redistribution":
		score += 45
	default:
		score += 40
	}
	switch strings.TrimSpace(strings.ToLower(a.Priority)) {
	case "critical":
		score += 30
	case "important":
		score += 15
	case "watch":
		score -= 10
	}
	switch strings.TrimSpace(strings.ToLower(a.Confidence)) {
	case "high":
		score += 10
	case "low":
		score -= 15
	}
	n := len(a.Scope.EntityIDs)
	if n > 8 {
		n = 8
	}
	score += float64(n) * 2
	// Explicit low priority must not outrank material stock/ads even if AI put it first.
	if strings.EqualFold(strings.TrimSpace(a.Priority), "watch") &&
		(a.Type == "no_sales_with_stock" || a.Type == "slow_turnover_review") {
		score -= 25
	}
	return score
}

func sameIDOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func priorityRank(priority string) int {
	switch strings.TrimSpace(strings.ToLower(priority)) {
	case "critical":
		return 0
	case "important":
		return 1
	case "watch":
		return 2
	default:
		return 3
	}
}

// mergeRecommendedActionIDs appends demoted recommended IDs into
// recommended_action_ids, drops any ID now present in top_action_ids, and
// preserves first-seen order without duplicates.
func mergeRecommendedActionIDs(out *contract.UnifiedAnalysisOutputV1, demoted, topIDs []string) int {
	topSet := map[string]struct{}{}
	for _, id := range topIDs {
		topSet[id] = struct{}{}
	}
	seen := map[string]struct{}{}
	merged := make([]string, 0, len(out.Dashboard.RecommendedActionIDs)+len(demoted))
	appendUnique := func(id string) bool {
		id = strings.TrimSpace(id)
		if id == "" {
			return false
		}
		if _, inTop := topSet[id]; inTop {
			return false
		}
		if _, ok := seen[id]; ok {
			return false
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
		return true
	}
	for _, id := range out.Dashboard.RecommendedActionIDs {
		_ = appendUnique(id)
	}
	added := 0
	for _, id := range demoted {
		if appendUnique(id) {
			added++
		}
	}
	out.Dashboard.RecommendedActionIDs = merged
	return added
}
