package dashboardhome

import (
	"strings"
	"unicode/utf8"
)

const (
	whatChangedGroupMaxItems          = 4
	whatChangedGroupMinItems          = 2
	whatChangedMainConclusionMaxRunes = 450
)

func normalizeWhatChangedMainConclusion(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	mc := strings.TrimSpace(whatChanged.MainConclusion)
	if mc == "" {
		mc = strings.TrimSpace(whatChanged.Summary)
	}
	if mc == "" {
		return
	}
	mc = strings.TrimSuffix(mc, "...")
	mc = strings.TrimSuffix(mc, "…")
	mc = strings.TrimSpace(mc)
	if utf8.RuneCountInString(mc) > whatChangedMainConclusionMaxRunes {
		mc = truncateAtSentenceBoundary(mc, whatChangedMainConclusionMaxRunes)
	}
	whatChanged.MainConclusion = mc
}

func truncateAtSentenceBoundary(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	cut := string(runes[:maxRunes])
	if idx := strings.LastIndexAny(cut, ".!?"); idx > maxRunes/2 {
		return strings.TrimSpace(cut[:idx+1])
	}
	return strings.TrimSpace(cut)
}

func enrichWhatChangedGroupsFromFacts(whatChanged *dashboardNarrativeWhatChanged, bundle whatChangedValidationBundle) {
	if whatChanged == nil {
		return
	}
	factsByKey := map[string][]whatChangedFact{
		whatChangedKeyWorsened:       bundle.Context.WorsenedFacts,
		whatChangedKeyNewRisks:       bundle.Context.NewRiskFacts,
		whatChangedKeyUnchangedRisks: bundle.Context.UnchangedRiskFacts,
		whatChangedKeyImproved:       bundle.Context.ImprovedFacts,
	}
	for i := range whatChanged.Groups {
		group := &whatChanged.Groups[i]
		facts := factsByKey[group.Key]
		if len(facts) < whatChangedGroupMinItems {
			continue
		}
		if len(group.Items) >= whatChangedGroupMinItems {
			if len(group.Items) > whatChangedGroupMaxItems {
				group.Items = group.Items[:whatChangedGroupMaxItems]
			}
			continue
		}
		existing := map[string]struct{}{}
		for _, item := range group.Items {
			existing[item.Label+"|"+item.Text] = struct{}{}
		}
		for _, fact := range facts {
			if len(group.Items) >= whatChangedGroupMaxItems {
				break
			}
			text, highlights := fallbackFactText(fact)
			item := dashboardNarrativeWhatChangedItem{
				Label:      fact.Label,
				Text:       text,
				Highlights: highlights,
			}
			key := item.Label + "|" + item.Text
			if _, ok := existing[key]; ok {
				continue
			}
			group.Items = append(group.Items, item)
			existing[key] = struct{}{}
			if len(group.Items) >= whatChangedGroupMinItems && len(group.Items) >= len(facts) {
				break
			}
		}
		if len(group.Items) > whatChangedGroupMaxItems {
			group.Items = group.Items[:whatChangedGroupMaxItems]
		}
	}
}

func relabelWhatChangedImprovedGroup(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	for i := range whatChanged.Groups {
		group := &whatChanged.Groups[i]
		if group.Key != whatChangedKeyImproved || len(group.Items) == 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(group.Label), "Улучшилось") {
			for _, item := range group.Items {
				lower := strings.ToLower(item.Label + " " + item.Text)
				if strings.Contains(lower, "точк") && strings.Contains(lower, "рост") {
					group.Label = "Точки роста"
					break
				}
			}
		}
	}
}

func countSparklineDataPoints(points []SparklinePoint) int {
	count := 0
	for _, p := range points {
		if p.HasData && p.Value != nil {
			count++
		}
	}
	return count
}
