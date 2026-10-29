package aipipeline

import (
	"regexp"
	"strings"
)

const GlobalContextUnresolvedNumericPlaceholderCode = "global_context_unresolved_numeric_placeholder"

// PlaceholderIssue records an unresolved numeric placeholder in AI text.
type PlaceholderIssue struct {
	Field           string `json:"field,omitempty"`
	MatchedFragment string `json:"matched_fragment"`
	ContextSnippet  string `json:"context_snippet,omitempty"`
}

var (
	placeholderOnPercentOnly   = regexp.MustCompile(`(?i)(?:на|снизил(?:ась|ся|ось|лись)?|вырос(?:ла|ло|ли)?|уменьшил(?:ась|ся|ось|лись)?|увеличил(?:ась|ся|ось|лись)?|изменил(?:ась|ся|ось|лись)?|упал(?:а|и|о)?|вырос(?:ла|ло|ли)?)\s+(?:на\s+)?%`)
	placeholderOnCurrencyOnly  = regexp.MustCompile(`(?i)(?:на|снизил(?:ась|ся|ось|лись)?|вырос(?:ла|ло|ли)?|уменьшил(?:ась|ся|ось|лись)?|изменил(?:ась|ся|ось|лись)?)\s+(?:на\s+)?(?:₽|руб\.?)`)
	placeholderOnDaysOnly      = regexp.MustCompile(`(?i)(?:на|составляет|равн(?:а|о|ы)?)\s+(?:N\s+)?дней\b`)
	placeholderOnPiecesOnly    = regexp.MustCompile(`(?i)(?:на|составляет|равн(?:а|о|ы)?)\s+(?:N\s+)?шт\.`)
	placeholderBarePercentWord = regexp.MustCompile(`(?i)\b(?:XX|N)\s*%`)
	placeholderTemplateBraces  = regexp.MustCompile(`\{[^}]{0,40}\}|\{\{[^}]{0,40}\}\}`)
	placeholderBracketValue    = regexp.MustCompile(`\[VALUE\]|<placeholder>`)
	placeholderPercentCompare  = regexp.MustCompile(`(?i)%\s+по\s+сравнению`)
)

// DetectUnresolvedPlaceholders finds numeric placeholders left unfilled in AI text.
func DetectUnresolvedPlaceholders(text string) []PlaceholderIssue {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	checks := []struct {
		re *regexp.Regexp
	}{
		{placeholderOnPercentOnly},
		{placeholderOnCurrencyOnly},
		{placeholderOnDaysOnly},
		{placeholderOnPiecesOnly},
		{placeholderBarePercentWord},
		{placeholderTemplateBraces},
		{placeholderBracketValue},
		{placeholderPercentCompare},
	}
	var issues []PlaceholderIssue
	seen := map[string]struct{}{}
	for _, check := range checks {
		for _, match := range check.re.FindAllString(text, -1) {
			match = strings.TrimSpace(match)
			if match == "" {
				continue
			}
			if isValidFilledNumericPhrase(text, match) {
				continue
			}
			key := strings.ToLower(match)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			issues = append(issues, PlaceholderIssue{
				MatchedFragment: match,
				ContextSnippet:  snippetPlaceholderContext(text, match),
			})
		}
	}
	return issues
}

func isValidFilledNumericPhrase(fullText, match string) bool {
	// "на 7,79%" is valid — digit before %
	if regexp.MustCompile(`\d+[.,]?\d*\s*%`).MatchString(match) {
		return true
	}
	// full text may contain filled value near the matched fragment
	idx := strings.Index(strings.ToLower(fullText), strings.ToLower(match))
	if idx < 0 {
		return false
	}
	window := fullText
	if idx > 20 {
		window = fullText[idx-20:]
	}
	if regexp.MustCompile(`\d+[.,]?\d*\s*%`).MatchString(window) {
		return true
	}
	if regexp.MustCompile(`\d[\d\s.,]*\s*(?:₽|руб)`).MatchString(window) {
		return true
	}
	return false
}

func snippetPlaceholderContext(text, match string) string {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(match))
	if idx < 0 {
		if len(text) > 120 {
			return text[:120] + "…"
		}
		return text
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(match) + 40
	if end > len(text) {
		end = len(text)
	}
	out := strings.TrimSpace(text[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(text) {
		out += "…"
	}
	return out
}

func DetectUnresolvedPlaceholdersInFields(fields map[string]string) []PlaceholderIssue {
	var out []PlaceholderIssue
	for field, text := range fields {
		for _, issue := range DetectUnresolvedPlaceholders(text) {
			issue.Field = field
			out = append(out, issue)
		}
	}
	return out
}
