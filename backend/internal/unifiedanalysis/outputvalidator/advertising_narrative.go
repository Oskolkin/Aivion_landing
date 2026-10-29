package outputvalidator

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

var (
	truncatedDecimalTailRe    = regexp.MustCompile(`(?i)(?:дrr|дрр)?\s*\d+\.\s*$`)
	adNarrativePlaceholderRe  = regexp.MustCompile(`(?i)(нет данных|данные отсутствуют|требуется дополнительн|нужно провести дополнительн|информация отсутствует|lorem ipsum|\bTODO\b|\bTBD\b)`)
	genericHeadlineRe         = regexp.MustCompile(`(?i)(есть кампании,?\s*требующие внимания|требуют внимания без конкретики|реклама работает нормально|нет данных)`)
	recommendationForbiddenRe = regexp.MustCompile(`(?i)(?:\bрекоменд|\bнужно\b|\bследует\b|\bпроверьте\b|\bостановите\b|\bпополните\b|\bперераспредел)`)

	adSharePercentRe = regexp.MustCompile(`(?i)(\d+[.,]\d+|\d+)\s*%`)
)

// validateAdvertisingNarrativeStructure requires separate headline + summary
// when advertising data exists. Recommendations are forbidden (Actions only).
// Free-text percentage grounding lives in validateAdvertisingClaimGrounding
// (structured claims only — no Russian semantic interpretation here).
func validateAdvertisingNarrativeStructure(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if !isMaterialOutcome(outcome) || idx == nil || !idx.HasAdvertisingData() {
		return
	}
	narrative := dash.AdEfficiency.Narrative
	if narrative == nil {
		report.addError(CodeAdvertisingHeadlineMissing, "$.dashboard.ad_efficiency.narrative.headline",
			"ad_efficiency.narrative is required when pack has advertising data")
		return
	}

	headline := strings.TrimSpace(narrative.Headline)
	summary := strings.TrimSpace(narrative.Summary)
	recommendation := strings.TrimSpace(narrative.Recommendation)

	if recommendation != "" {
		report.addError("advertising_recommendation_not_allowed", "$.dashboard.ad_efficiency.narrative.recommendation",
			"advertising narrative must not include recommendation; put actions in Actions")
	}

	if headline == "" {
		report.addError(CodeAdvertisingHeadlineMissing, "$.dashboard.ad_efficiency.narrative.headline",
			"headline is required and must be a short thesis, not derived by splitting summary")
	} else {
		if truncatedDecimalTailRe.MatchString(headline) {
			report.addError(CodeAdvertisingHeadlineTruncated, "$.dashboard.ad_efficiency.narrative.headline",
				"headline appears truncated on a decimal point (e.g. 'ДРР 19.')")
		}
		if utf8.RuneCountInString(headline) > 160 {
			report.addError(CodeAdvertisingHeadlineTruncated, "$.dashboard.ad_efficiency.narrative.headline",
				"headline exceeds max length for a short thesis")
		}
		if adNarrativePlaceholderRe.MatchString(headline) || genericHeadlineRe.MatchString(headline) {
			report.addError(CodeAdvertisingHeadlineTooGeneric, "$.dashboard.ad_efficiency.narrative.headline",
				"headline is too generic or contains placeholder filler")
		}
		if recommendationForbiddenRe.MatchString(headline) {
			report.addError("advertising_recommendation_not_allowed", "$.dashboard.ad_efficiency.narrative.headline",
				"headline must not contain recommendations or imperative steps")
		}
	}

	if summary == "" {
		report.addError(CodeAdvertisingExplanationMissing, "$.dashboard.ad_efficiency.narrative.summary",
			"summary/explanation is required when pack has advertising data")
	} else {
		if looksLikeFragmentedExplanationStart(summary) {
			report.addError(CodeAdvertisingExplanationFragmented, "$.dashboard.ad_efficiency.narrative.summary",
				"summary starts with a sentence fragment (likely split on decimal point)")
		}
		if adNarrativePlaceholderRe.MatchString(summary) {
			report.addError(CodeEmptyRequiredText, "$.dashboard.ad_efficiency.narrative.summary",
				"summary contains placeholder filler")
		}
		if sentenceCount(summary) < 2 {
			report.addError(CodeAdvertisingExplanationTooShallow, "$.dashboard.ad_efficiency.narrative.summary",
				"summary must contain at least two substantive sentences")
		}
		if recommendationForbiddenRe.MatchString(summary) {
			report.addError("advertising_recommendation_not_allowed", "$.dashboard.ad_efficiency.narrative.summary",
				"summary must not contain recommendations or imperative steps")
		}
	}

	if headline != "" && summary != "" {
		hn := normalizeNarrativeCompare(headline)
		sn := normalizeNarrativeCompare(summary)
		if hn == sn || (strings.HasPrefix(sn, hn) && utf8.RuneCountInString(hn) > 40) {
			report.addError(CodeAdvertisingHeadlineExplanationDuplicate,
				"$.dashboard.ad_efficiency.narrative",
				"headline and summary must not duplicate each other")
		}
	}
}

// percentHit is one free-text percentage with its byte offsets in the text.
type percentHit struct {
	Value float64
	Start int
	End   int
}

func parsePercentHits(text string) []percentHit {
	matches := adSharePercentRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]percentHit, 0, len(matches))
	for _, loc := range matches {
		if len(loc) < 4 {
			continue
		}
		raw := strings.ReplaceAll(text[loc[2]:loc[3]], ",", ".")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		out = append(out, percentHit{Value: v, Start: loc[0], End: loc[1]})
	}
	return out
}

func parsePercentsInText(text string) []float64 {
	hits := parsePercentHits(text)
	if len(hits) == 0 {
		return nil
	}
	out := make([]float64, len(hits))
	for i, h := range hits {
		out[i] = h.Value
	}
	return out
}

func splitAdvertisingSentences(text string) []string {
	parts := regexp.MustCompile(`[.!?…\n]+`).Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func looksLikeFragmentedExplanationStart(summary string) bool {
	s := strings.TrimSpace(summary)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "4%") || strings.HasPrefix(s, "4%)") || strings.HasPrefix(s, "),") {
		return true
	}
	r := []rune(s)
	if len(r) == 0 {
		return false
	}
	switch r[0] {
	case '%', ')', ',', '.', '!', '?':
		return true
	}
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i > 0 && i < len(r) && (r[i] == '%' || r[i] == ')') {
		return true
	}
	return false
}

func normalizeNarrativeCompare(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}
