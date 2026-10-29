package dashboardhome

import (
	"strings"
	"unicode/utf8"
)

var genericWhatChangedSummaryPatterns = []string{
	"ключевые изменения и активные сигналы",
	"ключевые изменения относительно предыдущего периода",
	"ключевые изменения относительно предыдущей сводки",
	"есть изменения по продажам, остаткам и рекламе",
	"данные обновлены",
	"ai видит",
	"система видит",
	"три главные зоны",
	"есть несколько важных факторов",
}

func isGenericWhatChangedText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return true
	}
	for _, pattern := range genericWhatChangedSummaryPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func isAISeesWhatChangedText(text string) bool {
	return containsAISeesSellerFacingPhrase(text)
}

func isUsableWhatChangedConclusion(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && !isGenericWhatChangedText(text) && !isAISeesWhatChangedText(text) && !isWeakWhatChangedMainConclusion(text)
}

func deriveWhatChangedMainConclusion(whatChanged *dashboardNarrativeWhatChanged, executiveSummary string) string {
	if whatChanged != nil {
		if mc := strings.TrimSpace(whatChanged.MainConclusion); isUsableWhatChangedConclusion(mc) {
			return mc
		}
	}
	if grounded := deriveGroundedWhatChangedMainConclusion(whatChanged); grounded != "" {
		return grounded
	}
	if exec := strings.TrimSpace(executiveSummary); isUsableWhatChangedConclusion(exec) {
		return exec
	}
	return deriveGroundedWhatChangedMainConclusion(whatChanged)
}

func deriveGroundedWhatChangedMainConclusion(whatChanged *dashboardNarrativeWhatChanged) string {
	if whatChanged == nil {
		return ""
	}

	sales := preferItemText(whatChanged, whatChangedKeyImproved, "sales")
	if sales == "" {
		sales = preferItemText(whatChanged, whatChangedKeyImproved, "")
	}
	worsened := preferItemText(whatChanged, whatChangedKeyWorsened, "")
	newRisk := preferItemText(whatChanged, whatChangedKeyNewRisks, "")
	active := preferItemText(whatChanged, whatChangedKeyUnchangedRisks, "")

	paragraphs := make([]string, 0, 4)
	if sales != "" {
		paragraphs = append(paragraphs, sales)
	}
	if worsened != "" {
		paragraphs = append(paragraphs, worsened)
	}
	riskPara := joinDistinctSentences(newRisk, active)
	if riskPara != "" {
		paragraphs = append(paragraphs, riskPara)
	}
	if len(paragraphs) == 0 {
		for _, group := range whatChanged.Groups {
			if sentence := whatChangedGroupFactSentenceFromGroup(group); sentence != "" {
				paragraphs = append(paragraphs, sentence)
				if len(paragraphs) >= 2 {
					break
				}
			}
		}
	}
	if len(paragraphs) == 0 {
		return ""
	}
	priority := "Главный приоритет — устранить ключевые риски по остаткам и рекламе, чтобы сохранить рост продаж."
	if practical := whatChangedPracticalTail(whatChanged); practical != "" {
		priority = "Главный приоритет — " + lowercaseFirst(practical)
	}
	paragraphs = append(paragraphs, priority)

	conclusion := strings.Join(paragraphs, "\n\n")
	if utf8.RuneCountInString(conclusion) > whatChangedMainConclusionMaxRunes {
		conclusion = truncateAtSentenceBoundary(conclusion, whatChangedMainConclusionMaxRunes)
	}
	return conclusion
}

func preferItemText(whatChanged *dashboardNarrativeWhatChanged, key, domainHint string) string {
	for _, group := range whatChanged.Groups {
		if group.Key != key {
			continue
		}
		for _, item := range group.Items {
			if domainHint != "" && !strings.EqualFold(strings.TrimSpace(item.Domain), domainHint) {
				continue
			}
			if text := strings.TrimSpace(item.Text); text != "" {
				return stripLabelColonPrefix(text, item.Label)
			}
			if label := strings.TrimSpace(item.Label); label != "" {
				return label
			}
		}
		if domainHint == "" {
			if summary := strings.TrimSpace(group.Summary); isUsableWhatChangedConclusion(summary) {
				return summary
			}
		}
	}
	return ""
}

func stripLabelColonPrefix(text, label string) string {
	text = strings.TrimSpace(text)
	label = strings.TrimSpace(label)
	if label == "" {
		return text
	}
	prefix := label + ": "
	if strings.HasPrefix(text, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	return text
}

func joinDistinctSentences(parts ...string) string {
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

func lowercaseFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	lower := strings.ToLower(string(r))
	return lower + s[size:]
}

func whatChangedGroupFactSentence(whatChanged *dashboardNarrativeWhatChanged, key string) string {
	for _, group := range whatChanged.Groups {
		if group.Key == key {
			return whatChangedGroupFactSentenceFromGroup(group)
		}
	}
	return ""
}

func whatChangedGroupFactSentenceFromGroup(group dashboardNarrativeWhatChangedGroup) string {
	for _, item := range group.Items {
		text := strings.TrimSpace(item.Text)
		label := strings.TrimSpace(item.Label)
		if text != "" {
			return stripLabelColonPrefix(text, label)
		}
		if label != "" {
			return label
		}
	}
	if summary := strings.TrimSpace(group.Summary); isUsableWhatChangedConclusion(summary) {
		return summary
	}
	return ""
}

func whatChangedPracticalTail(whatChanged *dashboardNarrativeWhatChanged) string {
	for _, group := range whatChanged.Groups {
		if group.Key != whatChangedKeyImproved {
			continue
		}
		if summary := strings.TrimSpace(group.Summary); summary != "" && !isGenericWhatChangedText(summary) && !isWeakWhatChangedMainConclusion(summary) {
			return summary
		}
	}
	return "Приоритет — устранить главные риски, чтобы не терять спрос и не перерасходовать бюджет."
}

func deriveWhatChangedBriefSummary(whatChanged *dashboardNarrativeWhatChanged) string {
	if whatChanged == nil {
		return ""
	}
	for _, key := range []string{whatChangedKeyWorsened, whatChangedKeyNewRisks, whatChangedKeyUnchangedRisks} {
		for _, group := range whatChanged.Groups {
			if group.Key != key {
				continue
			}
			if summary := strings.TrimSpace(group.Summary); summary != "" && !isGenericWhatChangedText(summary) {
				return summary
			}
		}
	}
	return ""
}

func ensureWhatChangedSummaryDistinctFromMainConclusion(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	summary := strings.TrimSpace(whatChanged.Summary)
	main := strings.TrimSpace(whatChanged.MainConclusion)
	if summary == "" || main == "" || !strings.EqualFold(summary, main) {
		return
	}
	if brief := deriveWhatChangedBriefSummary(whatChanged); brief != "" && !strings.EqualFold(brief, main) {
		whatChanged.Summary = brief
		return
	}
	if tail := whatChangedPracticalTail(whatChanged); tail != "" && !strings.EqualFold(tail, main) {
		whatChanged.MainConclusion = strings.TrimSpace(main + " " + tail)
	}
}

func repairWhatChangedMainConclusion(whatChanged *dashboardNarrativeWhatChanged, executiveSummary string) {
	if whatChanged == nil {
		return
	}
	strengthenWhatChangedMainConclusion(whatChanged)
	conclusion := deriveWhatChangedMainConclusion(whatChanged, executiveSummary)
	if conclusion != "" {
		whatChanged.MainConclusion = conclusion
		if isGenericWhatChangedText(whatChanged.Summary) || isAISeesWhatChangedText(whatChanged.Summary) {
			if brief := deriveWhatChangedBriefSummary(whatChanged); brief != "" {
				whatChanged.Summary = brief
			}
		}
	}
	ensureWhatChangedSummaryDistinctFromMainConclusion(whatChanged)
	normalizeWhatChangedMainConclusion(whatChanged)
	if whatChanged != nil && strings.TrimSpace(whatChanged.MainConclusion) != "" {
		whatChanged.MainConclusion = sanitizeWhatChangedMainConclusion(whatChanged.MainConclusion)
	}
}
