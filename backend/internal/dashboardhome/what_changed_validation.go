package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var whatChangedNumberTokenPattern = regexp.MustCompile(`\d[\d\s]*(?:[.,]\d+)?`)

func validateWhatChangedPayload(whatChanged *dashboardNarrativeWhatChanged, bundle whatChangedValidationBundle) []string {
	if whatChanged == nil {
		return []string{"missing_what_changed"}
	}
	var errs []string
	if strings.TrimSpace(whatChanged.Summary) == "" {
		errs = append(errs, "missing_what_changed_summary")
	}
	if containsWhatChangedActionVerbs(whatChanged.Summary) {
		errs = append(errs, "what_changed_summary_action_verb")
	}
	if len(whatChanged.Groups) != len(whatChangedRequiredGroups) {
		errs = append(errs, "what_changed_groups_count_mismatch")
	}
	for i, spec := range whatChangedRequiredGroups {
		if i >= len(whatChanged.Groups) {
			break
		}
		if whatChanged.Groups[i].Key != spec.Key {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_wrong_order_or_key", i))
		}
	}

	seenKeys := map[string]struct{}{}
	for i, group := range whatChanged.Groups {
		if !isAllowedWhatChangedKey(group.Key) {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_invalid_key", i))
			continue
		}
		if _, exists := seenKeys[group.Key]; exists {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_duplicate_key", i))
		}
		seenKeys[group.Key] = struct{}{}
		if group.Label != labelForWhatChangedKey(group.Key) {
			if group.Key == whatChangedKeyImproved && dashboardAIOnlyEnabled() && allowedImprovedGroupLabel(group.Label, len(group.Items) > 0) {
				// AI-only improved group may use strengths label when no real improvement.
			} else {
				errs = append(errs, fmt.Sprintf("what_changed_group_%d_label_mismatch", i))
			}
		}
		if labelContainsCount(group.Label) {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_label_contains_count", i))
		}
		if group.Tone != toneForWhatChangedKey(group.Key) {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_tone_mismatch", i))
		}
		if strings.TrimSpace(group.Summary) == "" && strings.TrimSpace(group.EmptyText) == "" {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_missing_summary", i))
		}
		if len(group.Items) > 0 && strings.TrimSpace(group.Summary) == "" {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_missing_summary_when_items", i))
		}
		if group.Key == whatChangedKeyImproved && len(bundle.Context.ImprovedFacts) == 0 {
			if dashboardAIOnlyEnabled() && len(bundle.Context.PeriodComparison.Facts) > 0 {
				// AI-only uses neutral period_comparison; improved group content is AI-owned.
			} else if len(group.Items) != 0 {
				errs = append(errs, "what_changed_improved_items_must_be_empty_when_no_improved_facts")
			}
			if !dashboardAIOnlyEnabled() || len(bundle.Context.PeriodComparison.Facts) == 0 {
				sumLower := strings.ToLower(strings.TrimSpace(group.Summary))
				if !strings.Contains(sumLower, "не выявлено") {
					errs = append(errs, "what_changed_improved_summary_phrase_required")
				}
				emptyLower := strings.ToLower(strings.TrimSpace(group.EmptyText))
				if !strings.Contains(emptyLower, "компенсируются") {
					errs = append(errs, "what_changed_improved_empty_text_phrase_required")
				}
			}
		}
		groupText := strings.Join([]string{group.Summary, group.EmptyText}, " ")
		for _, item := range group.Items {
			groupText += " " + item.Label + " " + item.Text
		}
		if containsWhatChangedActionVerbs(groupText) {
			errs = append(errs, fmt.Sprintf("what_changed_group_%d_action_verb", i))
		}
		errs = append(errs, validateNarrativeQuotedNames(
			fmt.Sprintf("what_changed_group_%d_unknown_name", i),
			bundle.AllowedNames,
			groupText,
		)...)
		for j, item := range group.Items {
			if strings.TrimSpace(item.Label) == "" || strings.TrimSpace(item.Text) == "" {
				errs = append(errs, fmt.Sprintf("what_changed_group_%d_item_%d_empty", i, j))
			}
			if containsWhatChangedActionVerbs(item.Text) {
				errs = append(errs, fmt.Sprintf("what_changed_group_%d_item_%d_action_verb", i, j))
			}
			if containsInventedWhatChangedNumber(item.Text, bundle.AllowedNumbers) {
				errs = append(errs, fmt.Sprintf("what_changed_group_%d_item_%d_invented_number", i, j))
			}
			for k, highlight := range item.Highlights {
				highlight = strings.TrimSpace(highlight)
				if highlight == "" {
					continue
				}
				if !whatChangedHighlightIsValid(highlight, item.Text, bundle.AllowedNumbers, bundle.AllowedNames) {
					errs = append(errs, fmt.Sprintf("what_changed_group_%d_item_%d_highlight_%d_invalid", i, j, k))
				}
			}
		}
	}
	for _, required := range whatChangedRequiredGroups {
		if _, ok := seenKeys[required.Key]; !ok {
			errs = append(errs, "what_changed_missing_group_"+required.Key)
		}
	}
	return compactStringSlice(errs)
}

// whatChangedHighlightIsValid: highlight must appear in item text, match allowed_numbers, or be a known campaign/product name.
func whatChangedHighlightIsValid(highlight, itemText string, allowedNumbers, allowedNames map[string]struct{}) bool {
	highlight = strings.TrimSpace(highlight)
	if highlight == "" {
		return true
	}
	if strings.Contains(itemText, highlight) {
		return true
	}
	if allowedNumberToken(allowedNumbers, highlight) {
		return true
	}
	if _, ok := allowedNames[highlight]; ok {
		return true
	}
	if strings.Contains(itemText, "«"+highlight+"»") {
		return true
	}
	return false
}

func containsWhatChangedActionVerbs(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	for _, verb := range whatChangedActionVerbPatterns {
		if strings.Contains(normalized, verb) {
			return true
		}
	}
	return false
}

func containsInventedWhatChangedNumber(text string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, token := range extractNumberTokens(text) {
		if token == "" {
			continue
		}
		if allowedNumberToken(allowed, token) {
			continue
		}
		if isSmallOrdinalNumber(token) {
			continue
		}
		return true
	}
	return false
}

func extractNumberTokens(text string) []string {
	matches := whatChangedNumberTokenPattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, strings.TrimSpace(match))
	}
	return out
}

func expandAllowedNumberTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	tokens := []string{value, normalizeNumberKey(value)}
	tokens = append(tokens, extractNumberTokens(value)...)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		key := normalizeNumberKey(token)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, token)
	}
	return out
}

func normalizeNumberKey(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if r == ',' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func allowedNumberToken(allowed map[string]struct{}, token string) bool {
	for _, candidate := range expandAllowedNumberTokens(token) {
		if _, ok := allowed[normalizeNumberKey(candidate)]; ok {
			return true
		}
		if _, ok := allowed[candidate]; ok {
			return true
		}
	}
	key := normalizeNumberKey(token)
	if _, ok := allowed[key]; ok {
		return true
	}
	return false
}

func isSmallOrdinalNumber(token string) bool {
	digits := normalizeNumberKey(token)
	if digits == "" {
		return true
	}
	if !strings.ContainsAny(digits, ",.") && len(digits) <= 1 {
		return true
	}
	return false
}
