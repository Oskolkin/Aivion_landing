package actions

import (
	"fmt"
)

// ScoreDraftAction computes backend score and priority for a draft action.
func ScoreDraftAction(action DomainAIAction, domainStatus string, candidates []ActionCandidate) (int, ActionPriority) {
	base := 40
	for _, c := range candidates {
		switch c.PriorityHint {
		case ActionPriorityCritical:
			if base < 80 {
				base = 80
			}
		case ActionPriorityImportant:
			if base < 60 {
				base = 60
			}
		case ActionPriorityWatch:
			if base < 40 {
				base = 40
			}
		}
	}

	base += impactBonus(candidates)
	base += confidenceBonus(candidates)
	if domainStatus == DomainStatusPartial {
		base -= 10
	}
	if base < 0 {
		base = 0
	}
	if base > 100 {
		base = 100
	}
	return base, MapScoreToPriority(base)
}

func impactBonus(candidates []ActionCandidate) int {
	bonus := 0
	for _, c := range candidates {
		if v, ok := c.Aggregates["revenue_share_pct"]; ok {
			if f, ok := toFloat(v); ok && f >= 10 {
				bonus += 5
			}
		}
		if v, ok := c.Aggregates["spend_7d"]; ok {
			if f, ok := toFloat(v); ok && f >= 1000 {
				bonus += 5
			}
		}
		if v, ok := c.Aggregates["orders_delta"]; ok {
			if f, ok := toFloat(v); ok && f <= -3 {
				bonus += 5
			}
		}
	}
	if bonus > 15 {
		bonus = 15
	}
	return bonus
}

func confidenceBonus(candidates []ActionCandidate) int {
	bonus := 0
	for _, c := range candidates {
		if len(c.Facts) >= 3 {
			bonus += 2
		}
		if len(c.TopExamples) > 0 {
			bonus += 1
		}
	}
	if bonus > 10 {
		bonus = 10
	}
	return bonus
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// MapScoreToPriority maps backend score to priority.
func MapScoreToPriority(score int) ActionPriority {
	switch {
	case score >= 80:
		return ActionPriorityCritical
	case score >= 50:
		return ActionPriorityImportant
	default:
		return ActionPriorityWatch
	}
}

// OverridePriority applies backend priority override over AI priority when score differs materially.
func OverridePriority(aiPriority, backendPriority ActionPriority, score int) ActionPriority {
	_ = aiPriority
	return MapScoreToPriority(score)
}

// OrderDraftsByScore sorts drafts by score descending.
func OrderDraftsByScore(drafts []ScoredDraftAction) []ScoredDraftAction {
	out := append([]ScoredDraftAction(nil), drafts...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[i].Score {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// BuildDeterministicAIFocus creates fallback AI focus from top actions.
func BuildDeterministicAIFocus(top []ScoredDraftAction) *ActionsAIFocus {
	if len(top) == 0 {
		return &ActionsAIFocus{
			Status:   "empty",
			Source:   "deterministic",
			Headline: "Критичных действий не найдено",
			Summary:  "На выбранную дату нет действий, требующих немедленного внимания.",
		}
	}
	cats := uniqueCategories(top)
	headline := fmt.Sprintf("Фокус: %s", top[0].Action.Title)
	if len(top) > 1 {
		headline = fmt.Sprintf("Фокус: %d приоритетных действия", len(top))
	}
	return &ActionsAIFocus{
		Status:        "available",
		Source:        "deterministic",
		Headline:      headline,
		Summary:       top[0].Action.Reason,
		TopCategories: cats,
	}
}

func uniqueCategories(drafts []ScoredDraftAction) []string {
	seen := make(map[string]bool)
	out := []string{}
	for _, d := range drafts {
		c := string(d.Action.Category)
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
