package dashboardhome

import "strings"

func canonicalWhatChangedGroupKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	if isAllowedWhatChangedKey(k) {
		return k
	}
	switch k {
	case "worse", "declined", "negative":
		return whatChangedKeyWorsened
	case "attention", "risk", "risks", "new", "warning", "requires_attention":
		return whatChangedKeyNewRisks
	case "stable", "unchanged", "active", "persists", "ongoing":
		return whatChangedKeyUnchangedRisks
	case "better", "positive", "growth":
		return whatChangedKeyImproved
	default:
		return ""
	}
}

func ensureWhatChangedRequiredGroups(
	whatChanged *dashboardNarrativeWhatChanged,
	bundle whatChangedValidationBundle,
	result *narrativeRepairResult,
) *dashboardNarrativeWhatChanged {
	if whatChanged == nil {
		return nil
	}
	buckets := make(map[string]dashboardNarrativeWhatChangedGroup, len(whatChangedRequiredGroups))
	for i, group := range whatChanged.Groups {
		key := canonicalWhatChangedGroupKey(group.Key)
		if key == "" {
			fallback := []string{whatChangedKeyWorsened, whatChangedKeyNewRisks, whatChangedKeyUnchangedRisks, whatChangedKeyImproved}
			if i < len(fallback) {
				key = fallback[i]
			}
		}
		if key == "" {
			continue
		}
		group.Key = key
		if key == whatChangedKeyImproved && dashboardAIOnlyEnabled() {
			if strings.TrimSpace(group.Label) == "" {
				group.Label = resolveImprovedGroupLabel(group)
			}
		} else {
			group.Label = labelForWhatChangedKey(key)
		}
		group.Tone = toneForWhatChangedKey(key)
		fixWhatChangedItemLabels(&group)
		if existing, ok := buckets[key]; ok {
			existing.Summary = mergeWhatChangedText(existing.Summary, group.Summary)
			existing.EmptyText = mergeWhatChangedText(existing.EmptyText, group.EmptyText)
			existing.Items = append(existing.Items, group.Items...)
			buckets[key] = existing
			if result != nil {
				result.RepairActions = append(result.RepairActions, "what_changed_merged_group_"+key)
				result.Repaired = true
			}
			continue
		}
		buckets[key] = group
		if canonicalWhatChangedGroupKey(whatChanged.Groups[i].Key) != key && result != nil {
			result.RepairActions = append(result.RepairActions, "what_changed_remapped_group_"+key)
			result.Repaired = true
		}
	}
	out := &dashboardNarrativeWhatChanged{Summary: strings.TrimSpace(whatChanged.Summary)}
	out.Groups = make([]dashboardNarrativeWhatChangedGroup, 0, len(whatChangedRequiredGroups))
	for _, spec := range whatChangedRequiredGroups {
		if group, ok := buckets[spec.Key]; ok {
			out.Groups = append(out.Groups, group)
			continue
		}
		out.Groups = append(out.Groups, emptyWhatChangedGroup(spec.Key, bundle))
		if result != nil {
			result.RepairActions = append(result.RepairActions, "what_changed_filled_missing_"+spec.Key)
			result.Repaired = true
		}
	}
	return out
}

func emptyWhatChangedGroup(key string, bundle whatChangedValidationBundle) dashboardNarrativeWhatChangedGroup {
	group := dashboardNarrativeWhatChangedGroup{
		Key:   key,
		Label: labelForWhatChangedKey(key),
		Tone:  toneForWhatChangedKey(key),
		Items: []dashboardNarrativeWhatChangedItem{},
	}
	if key == whatChangedKeyImproved && len(bundle.Context.ImprovedFacts) == 0 {
		group.Summary = "Заметных улучшений по сравнению с прошлой сводкой не выявлено."
		group.EmptyText = "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе."
		return group
	}
	group.EmptyText = "В этой группе значимых изменений не отмечено."
	return group
}

func fixWhatChangedItemLabels(group *dashboardNarrativeWhatChangedGroup) {
	if group == nil {
		return
	}
	filtered := make([]dashboardNarrativeWhatChangedItem, 0, len(group.Items))
	for _, item := range group.Items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = "Изменение"
		}
		filtered = append(filtered, dashboardNarrativeWhatChangedItem{
			Label:      label,
			Text:       text,
			Highlights: compactStringSlice(item.Highlights),
		})
	}
	group.Items = filtered
}

func mergeWhatChangedText(left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	switch {
	case left == "":
		return right
	case right == "":
		return left
	case left == right:
		return left
	default:
		return left + " " + right
	}
}

func repairWhatChangedPayload(
	whatChanged *dashboardNarrativeWhatChanged,
	bundle whatChangedValidationBundle,
	result *narrativeRepairResult,
) *dashboardNarrativeWhatChanged {
	if whatChanged == nil {
		fallback := buildFallbackWhatChangedPayload(bundle.Context)
		if result != nil {
			result.RepairableErrors = append(result.RepairableErrors, "missing_what_changed")
			result.PartialBlocks = append(result.PartialBlocks, "what_changed")
			result.RepairActions = append(result.RepairActions, "what_changed_filled_from_context")
			result.Repaired = true
			result.Partial = true
		}
		return fallback
	}
	working := whatChanged
	if dashboardAIOnlyEnabled() {
		working = ensureWhatChangedRequiredGroups(working, bundle, result)
	}
	normalized := normalizeDashboardNarrativeWhatChanged(working)
	if result != nil && working != whatChanged {
		result.RepairActions = append(result.RepairActions, "what_changed_normalized_order")
		result.Repaired = true
	}
	for i := range normalized.Groups {
		group := &normalized.Groups[i]
		if group.Key != whatChangedKeyImproved {
			continue
		}
		if len(bundle.Context.ImprovedFacts) > 0 {
			continue
		}
		if len(group.Items) > 0 {
			group.Items = []dashboardNarrativeWhatChangedItem{}
			if result != nil {
				result.RepairActions = append(result.RepairActions, "what_changed_improved_items_cleared")
				result.Repaired = true
			}
		}
		if strings.TrimSpace(group.Summary) == "" {
			group.Summary = "Заметных улучшений по сравнению с прошлой сводкой не выявлено."
			if result != nil {
				result.RepairActions = append(result.RepairActions, "what_changed_improved_summary_filled")
				result.Repaired = true
			}
		}
		if strings.TrimSpace(group.EmptyText) == "" {
			group.EmptyText = "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе."
			if result != nil {
				result.RepairActions = append(result.RepairActions, "what_changed_improved_empty_text_filled")
				result.Repaired = true
			}
		}
	}
	return normalized
}

func enrichWhatChangedBundleFromPayload(bundle whatChangedValidationBundle, whatChanged *dashboardNarrativeWhatChanged) whatChangedValidationBundle {
	if whatChanged == nil {
		return bundle
	}
	if bundle.AllowedNumbers == nil {
		bundle.AllowedNumbers = map[string]struct{}{}
	}
	if bundle.AllowedNames == nil {
		bundle.AllowedNames = map[string]struct{}{}
	}
	addQuotedNames := func(parts ...string) {
		for _, part := range parts {
			matches := quotedNarrativeNamePattern.FindAllStringSubmatch(part, -1)
			for _, match := range matches {
				name := strings.TrimSpace(match[1])
				if name == "" && len(match) > 2 {
					name = strings.TrimSpace(match[2])
				}
				if name != "" {
					bundle.AllowedNames[name] = struct{}{}
				}
			}
		}
	}
	addTokens := func(parts ...string) {
		for _, part := range parts {
			for _, token := range extractNumberTokens(part) {
				for _, expanded := range expandAllowedNumberTokens(token) {
					bundle.AllowedNumbers[normalizeNumberKey(expanded)] = struct{}{}
					bundle.AllowedNumbers[expanded] = struct{}{}
				}
			}
		}
	}
	addTokens(whatChanged.Summary)
	addQuotedNames(whatChanged.Summary)
	for _, group := range whatChanged.Groups {
		addTokens(group.Summary, group.EmptyText)
		addQuotedNames(group.Summary, group.EmptyText)
		for _, item := range group.Items {
			addTokens(item.Label, item.Text)
			addQuotedNames(item.Label, item.Text)
		}
	}
	return bundle
}
