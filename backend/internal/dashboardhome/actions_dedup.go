package dashboardhome

import (
	"sort"
	"strings"
)

// Action feed source priority (lower = wins dedup): recommendation > ai_summary > alert > deterministic.
var actionSourcePriority = map[string]int{
	"recommendation": 0,
	"ai_summary":     1,
	"alert":          2,
	"deterministic":  3,
}

func actionDedupKey(item ActionItem) string {
	entityType := strings.ToLower(strings.TrimSpace(item.RelatedEntityType))
	entityID := ""
	if item.RelatedEntityID != nil {
		entityID = strings.ToLower(strings.TrimSpace(*item.RelatedEntityID))
	}
	riskKey := normalizeActionKey(item.Action)
	if riskKey == "" {
		riskKey = normalizeActionKey(item.Reason)
	}
	return entityType + "|" + entityID + "|" + riskKey
}

func dedupeActionFeed(candidates []ActionItem, limit int) []ActionItem {
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		pi := actionSourcePriority[strings.ToLower(candidates[i].Source)]
		pj := actionSourcePriority[strings.ToLower(candidates[j].Source)]
		if pi != pj {
			return pi < pj
		}
		return priorityRank(candidates[i].Priority) < priorityRank(candidates[j].Priority)
	})

	seen := make(map[string]struct{}, len(candidates))
	out := make([]ActionItem, 0, limit)
	for _, item := range candidates {
		key := actionDedupKey(item)
		if key == "||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return priorityRank(out[i].Priority) < priorityRank(out[j].Priority)
	})
	return out
}

func collectActionCandidates(
	aiTop []ActionItem,
	aiAccount []ActionItem,
	recs []ActionItem,
	alertItems []ActionItem,
) []ActionItem {
	total := len(aiTop) + len(aiAccount) + len(recs) + len(alertItems)
	out := make([]ActionItem, 0, total)
	out = append(out, recs...)
	out = append(out, aiTop...)
	out = append(out, aiAccount...)
	out = append(out, alertItems...)
	return out
}
