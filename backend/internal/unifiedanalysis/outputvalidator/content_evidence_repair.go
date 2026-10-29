package outputvalidator

import (
	"regexp"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

var unsupportedContentStepRE = regexp.MustCompile(`(?i)(фотограф|фото\b|описан|атрибут|контент\s*карточ|полнот\w*\s*карточ|барьер\w*\s*к\s*покупк)`)

var diagnosticOnlyContentRE = regexp.MustCompile(`(?i)^(проверить|сопоставить|убедиться|сверить)\b`)

// RepairUnsupportedContentSteps strips causal photo/description/content
// recommendations when Pack does not expose content-score evidence.
// Diagnostic "проверить …" checklist steps are kept; causal "обновить фото"
// style steps are rewritten to a stock/price/availability checklist.
func RepairUnsupportedContentSteps(out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) int {
	if out == nil {
		return 0
	}
	if contentEvidenceAvailable(idx) {
		return 0
	}
	repaired := 0
	for i := range out.Actions {
		a := &out.Actions[i]
		newSteps := make([]contract.UnifiedActionStepV1, 0, len(a.Steps))
		changed := false
		for _, step := range a.Steps {
			blob := step.Title + " " + step.Description
			if !unsupportedContentStepRE.MatchString(blob) {
				newSteps = append(newSteps, step)
				continue
			}
			if diagnosticOnlyContentRE.MatchString(strings.TrimSpace(step.Title)) ||
				diagnosticOnlyContentRE.MatchString(strings.TrimSpace(step.Description)) {
				// Soft diagnostic wording is allowed without content evidence.
				newSteps = append(newSteps, step)
				continue
			}
			changed = true
			repaired++
			newSteps = append(newSteps, contract.UnifiedActionStepV1{
				Order:       step.Order,
				Title:       "Проверить остаток, цену и доступность",
				Description: "Сопоставить падение с остатком и статусом товара; проверить цену и участие в кампании. Не утверждать проблему контента без данных о карточке.",
				EntityIDs:   step.EntityIDs,
			})
		}
		if changed {
			// Re-number orders sequentially.
			for j := range newSteps {
				newSteps[j].Order = j + 1
			}
			a.Steps = newSteps
			a.Summary = scrubUnsupportedContentPhrases(a.Summary)
			a.Problem = scrubUnsupportedContentPhrases(a.Problem)
			a.WhyNow = scrubUnsupportedContentPhrases(a.WhyNow)
			a.ExpectedEffect = scrubUnsupportedContentPhrases(a.ExpectedEffect)
		}
	}
	return repaired
}

func contentEvidenceAvailable(idx *PackReferenceIndex) bool {
	if idx == nil {
		return false
	}
	// Pack currently never ships product content scores; keep the gate explicit
	// so a future enrichment column can unlock content recommendations.
	return idx.HasContentScoreEvidence()
}

func scrubUnsupportedContentPhrases(s string) string {
	if s == "" || !unsupportedContentStepRE.MatchString(s) {
		return s
	}
	// Soften causal content claims without inventing a new narrative.
	out := unsupportedContentStepRE.ReplaceAllString(s, "доступность и цену")
	return strings.TrimSpace(out)
}
