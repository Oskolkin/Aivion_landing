package dashboardhome

import (
	"regexp"
	"strings"
)

var (
	whatChangedWeakMainConclusionPatterns = []string{
		"основное ухудшение видно",
		"есть устойчивые точки роста",
		"несмотря на риски",
		"ключевые изменения",
		"ai видит",
		"система видит",
	}
	whatChangedMalformedStockText = regexp.MustCompile(`(?i)коротких остатков`)
)

func repairWhatChangedSemantic(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	for i := range whatChanged.Groups {
		repairWhatChangedGroupSemantic(&whatChanged.Groups[i])
	}
}

func repairWhatChangedGroupSemantic(group *dashboardNarrativeWhatChangedGroup) {
	if group == nil {
		return
	}
	labels := map[string]int{}
	out := make([]dashboardNarrativeWhatChangedItem, 0, len(group.Items))
	for _, item := range group.Items {
		repaired := repairWhatChangedItemSemantic(item)
		if repaired.Label == "" && repaired.Text == "" {
			continue
		}
		labelKey := strings.ToLower(strings.TrimSpace(repaired.Label))
		if labelKey != "" {
			if count := labels[labelKey]; count > 0 {
				repaired.Label = refineDuplicateWhatChangedLabel(repaired.Label, count+1)
			}
			labels[strings.ToLower(strings.TrimSpace(repaired.Label))]++
		}
		if isGenericWhatChangedText(repaired.Text) || actionGenericSignalPattern.MatchString(repaired.Text) {
			continue
		}
		out = append(out, repaired)
	}
	group.Items = out
}

func repairWhatChangedItemSemantic(item dashboardNarrativeWhatChangedItem) dashboardNarrativeWhatChangedItem {
	label := strings.TrimSpace(item.Label)
	text := strings.TrimSpace(item.Text)
	text = polishWhatChangedItemText(label, text)
	label = polishWhatChangedItemLabel(label, text)
	if label == "Остатки" && strings.Contains(strings.ToLower(text), "критич") {
		label = "Риск дефицита"
	}
	if label == "Продажи" && actionGenericSignalPattern.MatchString(text) {
		return dashboardNarrativeWhatChangedItem{}
	}
	item.Label = label
	item.Text = text
	return item
}

func polishWhatChangedItemLabel(label, text string) string {
	label = strings.TrimSpace(label)
	lower := strings.ToLower(label + " " + text)
	switch {
	case label == "Остатки" || strings.Contains(lower, "остат"):
		if strings.Contains(lower, "критич") || strings.Contains(lower, "коротк") {
			return "Остатки с коротким покрытием"
		}
		return "Риск дефицита"
	case strings.Contains(lower, "дрр") || strings.Contains(lower, "реклам"):
		return "Рекламный перерасход"
	case strings.Contains(lower, "воронк") || strings.Contains(lower, "просмотр"):
		return "Слабый верх воронки"
	}
	return label
}

func polishWhatChangedItemText(label, text string) string {
	text = strings.TrimSpace(text)
	if actionMalformedWhatChangedStart.MatchString(text) {
		text = rewriteMalformedStockWhatChangedText(text)
	}
	text = strings.ReplaceAll(text, "коротких остатков", "короткого запаса")
	text = strings.ReplaceAll(text, "коротких остаток", "короткого запаса")
	if whatChangedMalformedStockText.MatchString(text) {
		text = rewriteMalformedStockWhatChangedText(text)
	}
	return cleanupSellerFacingText(text)
}

func rewriteMalformedStockWhatChangedText(text string) string {
	// "8, сохраняются 4 критичных и 4 коротких остатков" -> seller-facing
	re := regexp.MustCompile(`(?i)^\d+\s*,\s*сохраняются\s+(\d+)\s+критичн\w*\s+и\s+(\d+)`)
	if m := re.FindStringSubmatch(text); len(m) == 3 {
		return "Сохраняются " + m[1] + " SKU в критичной зоне 0–3 дня и " + m[2] + " — в зоне короткого запаса 4–7 дней."
	}
	re2 := regexp.MustCompile(`(?i)(\d+)\s+критичн\w*.*(\d+).*коротк`)
	if m := re2.FindStringSubmatch(text); len(m) == 3 {
		return "Сохраняются " + m[1] + " SKU в критичной зоне 0–3 дня и " + m[2] + " — в зоне короткого запаса 4–7 дней."
	}
	return text
}

func refineDuplicateWhatChangedLabel(label string, n int) string {
	base := strings.TrimSpace(label)
	if n <= 1 {
		return base
	}
	return base + " (" + itoa(n-1) + ")"
}

func isWeakWhatChangedMainConclusion(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, p := range whatChangedWeakMainConclusionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return isGenericWhatChangedText(text) || isAISeesWhatChangedText(text)
}

func strengthenWhatChangedMainConclusion(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	mc := strings.TrimSpace(whatChanged.MainConclusion)
	if mc != "" && !isWeakWhatChangedMainConclusion(mc) && !looksLikeSerializedWhatChangedLabels(mc) {
		return
	}
	derived := deriveGroundedWhatChangedMainConclusion(whatChanged)
	if derived != "" && !isWeakWhatChangedMainConclusion(derived) {
		whatChanged.MainConclusion = derived
		return
	}
	if rebuilt := buildSellerFacingWhatChangedConclusionFromGroups(whatChanged); rebuilt != "" {
		whatChanged.MainConclusion = rebuilt
	}
}

func looksLikeSerializedWhatChangedLabels(text string) bool {
	// "Fact A: sentence. Fact B: sentence." — technical dump, not synthesis.
	colonHits := 0
	for _, part := range strings.Split(text, ". ") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, ": "); idx > 0 && idx < 90 {
			colonHits++
		}
	}
	return colonHits >= 2
}

func buildSellerFacingWhatChangedConclusionFromGroups(whatChanged *dashboardNarrativeWhatChanged) string {
	if whatChanged == nil {
		return ""
	}
	return deriveGroundedWhatChangedMainConclusion(whatChanged)
}

func sanitizeWhatChangedMainConclusion(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Preserve intentional paragraph breaks from grounded synthesis.
	if strings.Contains(text, "\n\n") {
		paras := strings.Split(text, "\n\n")
		out := make([]string, 0, len(paras))
		for _, p := range paras {
			p = strings.TrimSpace(p)
			if p == "" || isWeakWhatChangedMainConclusion(p) || isGenericWhatChangedText(p) {
				continue
			}
			out = append(out, p)
		}
		if len(out) == 0 {
			return ""
		}
		return strings.Join(out, "\n\n")
	}
	parts := strings.Split(text, ". ")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || isWeakWhatChangedMainConclusion(part) || isGenericWhatChangedText(part) {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return ""
	}
	result := strings.Join(out, ". ")
	if !strings.HasSuffix(result, ".") && !strings.HasSuffix(result, "!") && !strings.HasSuffix(result, "?") {
		result += "."
	}
	return result
}

func repairSellerFacingWhatChangedConclusion(wc *WhatChangedWidget) {
	if wc == nil || wc.Status != WidgetAvailable {
		return
	}
	narrative := whatChangedNarrativeFromWidget(wc)
	repairWhatChangedSemantic(narrative)
	strengthenWhatChangedMainConclusion(narrative)
	repairWhatChangedMainConclusion(narrative, "")
	if conclusion := strings.TrimSpace(narrative.MainConclusion); conclusion != "" {
		wc.MainConclusion = conclusion
		if isGenericWhatChangedText(wc.Summary) || isWeakWhatChangedMainConclusion(wc.Summary) {
			wc.Summary = conclusion
		}
	}
}

func whatChangedNarrativeFromWidget(wc *WhatChangedWidget) *dashboardNarrativeWhatChanged {
	if wc == nil {
		return nil
	}
	out := &dashboardNarrativeWhatChanged{
		Summary:        wc.Summary,
		MainConclusion: wc.MainConclusion,
		Groups:         make([]dashboardNarrativeWhatChangedGroup, 0, len(wc.Groups)),
	}
	for _, g := range wc.Groups {
		group := dashboardNarrativeWhatChangedGroup{
			Key: g.Key, Label: g.Label, Tone: g.Tone, Summary: g.Summary, EmptyText: g.EmptyText,
			Items: make([]dashboardNarrativeWhatChangedItem, 0, len(g.Items)),
		}
		for _, item := range g.Items {
			group.Items = append(group.Items, dashboardNarrativeWhatChangedItem{
				Label: item.Label, Text: item.Text,
			})
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}
