package chatv2

import (
	"strings"
	"unicode/utf8"
)

type MemorySelection struct {
	Messages           []HistoryMessage
	ReferencedEntities []EntityRef
	Truncated          bool
	Reason             string
	Bytes              int
	NeedsClarification bool
	Clarification      string
}

func SelectConversation(history []HistoryMessage, currentQuestion string, prior []EntityRef, policy MemoryPolicy) MemorySelection {
	if policy.MaxRecentTurns <= 0 {
		policy = DefaultMemoryPolicy()
	}
	var selected []HistoryMessage
	bytes := 0
	truncated := false
	reason := ""
	maxMsgs := policy.MaxRecentTurns * 2
	for i := len(history) - 1; i >= 0 && len(selected) < maxMsgs; i-- {
		m := history[i]
		size := utf8.RuneCountInString(m.Content)
		if bytes+size > policy.MaxHistoryBytes && len(selected) > 0 {
			truncated = true
			reason = "history_bytes"
			break
		}
		selected = append(selected, m)
		bytes += size
	}
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	if len(history) > len(selected) && !truncated {
		truncated = true
		reason = "max_recent_turns"
	}
	entities, clarify, note := resolveCanonicalMemory(prior, currentQuestion, policy.MaxReferencedEntities)
	return MemorySelection{
		Messages: selected, ReferencedEntities: entities, Truncated: truncated, Reason: reason, Bytes: bytes,
		NeedsClarification: clarify, Clarification: note,
	}
}

func resolveCanonicalMemory(prior []EntityRef, question string, max int) ([]EntityRef, bool, string) {
	uniq := uniqueEntityRefs(prior, max)
	q := strings.ToLower(strings.TrimSpace(question))
	if !isCoreferenceFollowUp(q) {
		return uniq, false, ""
	}
	kindHint := coreferenceKindHint(q)
	var candidates []EntityRef
	for _, e := range uniq {
		if kindHint == "" || strings.EqualFold(e.Kind, kindHint) {
			candidates = append(candidates, e)
		}
	}
	if kindHint != "" && len(candidates) == 0 {
		candidates = uniq
	}
	if len(candidates) == 0 {
		return uniq, true, "Уточните, о каком товаре, кампании или категории речь."
	}
	if len(candidates) > 1 {
		return uniq, true, "В предыдущем контексте несколько сущностей. Уточните, о какой именно речь."
	}
	return []EntityRef{candidates[0]}, false, ""
}

func isCoreferenceFollowUp(q string) bool {
	needles := []string{"по нему", "по ней", "в ней", "по этому", "по этой", "а что с", "а сколько"}
	for _, n := range needles {
		if strings.Contains(q, n) {
			return true
		}
	}
	return false
}

func coreferenceKindHint(q string) string {
	if strings.Contains(q, "кампан") || strings.Contains(q, "реклам") {
		return "CAMPAIGN"
	}
	if strings.Contains(q, "категор") || strings.Contains(q, "в ней") {
		if strings.Contains(q, "категор") {
			return "CATEGORY"
		}
	}
	if strings.Contains(q, "по нему") || strings.Contains(q, "товар") || strings.Contains(q, "sku") || strings.Contains(q, "продаж") || strings.Contains(q, "остат") || strings.Contains(q, "заказать") {
		return "SKU"
	}
	return ""
}

func uniqueEntityRefs(in []EntityRef, max int) []EntityRef {
	seen := map[string]struct{}{}
	var out []EntityRef
	for _, e := range in {
		key := strings.ToUpper(strings.TrimSpace(e.Kind)) + ":" + strings.TrimSpace(e.ID)
		if e.ID == "" || strings.TrimSpace(e.Kind) == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		e.Kind = strings.ToUpper(strings.TrimSpace(e.Kind))
		out = append(out, e)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func mergeEntityRefs(sets ...[]EntityRef) []EntityRef {
	var all []EntityRef
	for _, s := range sets {
		all = append(all, s...)
	}
	return uniqueEntityRefs(all, 64)
}

// tokenizeEntities is an optional hint only. It must not be the owner of multi-turn memory.
func tokenizeEntities(s string) []string {
	fields := strings.Fields(s)
	var out []string
	for _, f := range fields {
		f = strings.Trim(f, ".,;:!?()[]\"'«»")
		upper := strings.ToUpper(f)
		if strings.HasPrefix(upper, "SKU") && len(upper) > 3 {
			out = append(out, upper)
		}
	}
	return out
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
