package aidatapack

import (
	"strings"
	"unicode"
)

const MetricSemanticMismatchCode = "metric_semantic_mismatch"

// MetricSemanticIssue records forbidden terminology for a metric in AI text.
type MetricSemanticIssue struct {
	MetricCode      string `json:"metric_code"`
	ForbiddenPhrase string `json:"forbidden_phrase"`
	ExpectedLabel   string `json:"expected_label"`
	Field           string `json:"field,omitempty"`
	ContextSnippet  string `json:"context_snippet,omitempty"`
}

// DetectMetricSemanticMismatches scans text for forbidden terminology when metric codes are in scope.
func DetectMetricSemanticMismatches(text string, metricCodes []string, field string) []MetricSemanticIssue {
	textLower := strings.ToLower(strings.TrimSpace(text))
	if textLower == "" || len(metricCodes) == 0 {
		return nil
	}
	scope := map[string]struct{}{}
	for _, code := range metricCodes {
		if code = strings.TrimSpace(code); code != "" {
			scope[code] = struct{}{}
		}
	}
	var issues []MetricSemanticIssue
	for code := range scope {
		def, ok := metricSemanticsRegistry[code]
		if !ok {
			continue
		}
		for _, forbidden := range def.ForbiddenTermsRU {
			if forbidden = strings.TrimSpace(forbidden); forbidden == "" {
				continue
			}
			if strings.Contains(textLower, strings.ToLower(forbidden)) {
				issues = append(issues, MetricSemanticIssue{
					MetricCode:      code,
					ForbiddenPhrase: forbidden,
					ExpectedLabel:   def.CanonicalLabelRU,
					Field:           field,
					ContextSnippet:  snippetAround(text, forbidden, 60),
				})
			}
		}
	}
	return issues
}

// DetectMetricSemanticMismatchesForMetrics scans text against all registry metrics present in codes slice.
func DetectMetricSemanticMismatchesForMetrics(text, field string, metrics ...string) []MetricSemanticIssue {
	return DetectMetricSemanticMismatches(text, metrics, field)
}

func snippetAround(text, phrase string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(phrase))
	if idx < 0 {
		if len(text) <= maxLen {
			return text
		}
		return text[:maxLen] + "…"
	}
	start := idx - maxLen/3
	if start < 0 {
		start = 0
	}
	end := idx + len(phrase) + maxLen/2
	if end > len(text) {
		end = len(text)
	}
	out := strings.TrimSpace(text[start:end])
	out = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, out)
	if start > 0 {
		out = "…" + out
	}
	if end < len(text) {
		out += "…"
	}
	return out
}
