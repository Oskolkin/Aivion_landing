package outputvalidator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

var internalFieldNameRe = regexp.MustCompile(`(?i)\b(available_stock_units|stock_coverage_days_7d|safety\s*stock|surplus_120d|in_transit_units|target_coverage_days)\b|critical\s*/\s*urgent\s*/\s*planning|\btransit\b`)

func validateNoInternalFieldNames(report *Report, out *contract.UnifiedAnalysisOutputV1) {
	if out == nil {
		return
	}
	for ai, a := range out.Actions {
		path := fmt.Sprintf("$.actions[%d]", ai)
		check := func(field, text string) {
			if internalFieldNameRe.MatchString(text) {
				report.addError(CodeActionContainsInternalFieldName, path+"."+field,
					"user-facing text contains internal field name")
			}
		}
		check("title", a.Title)
		check("summary", a.Summary)
		check("problem", a.Problem)
		check("why_now", a.WhyNow)
		check("expected_effect", a.ExpectedEffect)
		check("completion_criteria", a.CompletionCriteria)
		check("evidence_summary", a.EvidenceSummary)
		for si, s := range a.Steps {
			check(fmt.Sprintf("steps[%d].title", si), s.Title)
			check(fmt.Sprintf("steps[%d].description", si), s.Description)
		}
	}
}

// ScrubInternalFieldNames replaces known internal tokens in user-facing action
// copy. Used as a safety net before publication when repair is preferred over block.
func ScrubInternalFieldNames(out *contract.UnifiedAnalysisOutputV1) int {
	if out == nil {
		return 0
	}
	n := 0
	replacer := strings.NewReplacer(
		"available_stock_units", "доступный остаток",
		"stock_coverage_days_7d", "покрытие спроса в днях",
		"safety stock", "страховой запас",
		"Safety stock", "Страховой запас",
		"surplus_120d", "избыточный запас с покрытием более 120 дней",
		"critical / urgent / planning", "критический / срочный / плановый приоритет",
		"`transit`", "товары в пути",
		"/transit", "/товары в пути",
	)
	scrub := func(s string) string {
		out := replacer.Replace(s)
		if out != s {
			n++
		}
		return out
	}
	for i := range out.Actions {
		a := &out.Actions[i]
		a.Title = scrub(a.Title)
		a.Summary = scrub(a.Summary)
		a.Problem = scrub(a.Problem)
		a.WhyNow = scrub(a.WhyNow)
		a.ExpectedEffect = scrub(a.ExpectedEffect)
		a.CompletionCriteria = scrub(a.CompletionCriteria)
		a.EvidenceSummary = scrub(a.EvidenceSummary)
		for j := range a.Steps {
			a.Steps[j].Title = scrub(a.Steps[j].Title)
			a.Steps[j].Description = scrub(a.Steps[j].Description)
		}
	}
	return n
}
