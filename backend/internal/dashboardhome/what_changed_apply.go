package dashboardhome

import (
	"log/slog"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/weeklychangepolicy"
)

func whatChangedWidgetFromNarrative(whatChanged *dashboardNarrativeWhatChanged) WhatChangedWidget {
	byKey := make(map[string]WhatChangedGroup, len(whatChanged.Groups))
	for _, group := range whatChanged.Groups {
		items := make([]WhatChangedItem, 0, len(group.Items))
		for _, item := range group.Items {
			names := compactStringSlice(item.EntityNames)
			items = append(items, WhatChangedItem{
				Label:             cleanupSellerFacingText(stripWeeklyChangeCampaignIDs(item.Label, names)),
				Text:              cleanupSellerFacingText(stripWeeklyChangeCampaignIDs(item.Text, names)),
				Highlights:        compactStringSlice(item.Highlights),
				Domain:            strings.TrimSpace(item.Domain),
				Severity:          strings.TrimSpace(item.Severity),
				CurrentValueText:  strings.TrimSpace(item.CurrentValueText),
				PreviousValueText: strings.TrimSpace(item.PreviousValueText),
				DeltaText:         strings.TrimSpace(item.DeltaText),
				ImpactText:        cleanupSellerFacingText(stripWeeklyChangeCampaignIDs(item.ImpactText, names)),
				EntityCount:       item.EntityCount,
				EntityNames:       names,
				LinkedActionID:    strings.TrimSpace(item.LinkedActionID),
				ActionLinkLabel:   strings.TrimSpace(item.ActionLinkLabel),
				DeepLink:          strings.TrimSpace(item.DeepLink),
				ChangeID:          strings.TrimSpace(item.ChangeID),
			})
		}
		emptyText := cleanupSellerFacingText(group.EmptyText)
		if len(items) == 0 {
			emptyText = fixWhatChangedEmptyText(group.Key, emptyText)
		}
		byKey[group.Key] = WhatChangedGroup{
			Key:       group.Key,
			Label:     group.Label,
			Tone:      group.Tone,
			Summary:   cleanupSellerFacingText(stripWeeklyChangeCampaignIDs(group.Summary, nil)),
			Items:     items,
			EmptyText: emptyText,
		}
	}
	ordered := make([]WhatChangedGroup, 0, len(whatChangedRequiredGroups))
	for _, spec := range whatChangedRequiredGroups {
		if g, ok := byKey[spec.Key]; ok {
			ordered = append(ordered, g)
		}
	}
	widget := WhatChangedWidget{
		WidgetBase:     WidgetBase{Status: WidgetAvailable},
		Summary:        strings.TrimSpace(whatChanged.Summary),
		MainConclusion: strings.TrimSpace(whatChanged.MainConclusion),
		Groups:         ordered,
	}
	if widget.MainConclusion == "" {
		widget.MainConclusion = deriveWhatChangedMainConclusion(whatChanged, "")
	}
	if widget.MainConclusion != "" && isGenericWhatChangedText(widget.Summary) {
		if brief := deriveWhatChangedBriefSummary(whatChanged); brief != "" {
			widget.Summary = brief
		}
	}
	dedup := &dashboardNarrativeWhatChanged{
		Summary:        widget.Summary,
		MainConclusion: widget.MainConclusion,
		Groups:         whatChanged.Groups,
	}
	ensureWhatChangedSummaryDistinctFromMainConclusion(dedup)
	widget.Summary = strings.TrimSpace(dedup.Summary)
	widget.MainConclusion = strings.TrimSpace(dedup.MainConclusion)
	if widget.Summary == "" && len(widget.Groups) == 0 {
		widget.Status = WidgetNotEnoughData
		widget.Message = "Нет сравнения с предыдущей сводкой."
	}
	return widget
}

func stripWeeklyChangeCampaignIDs(text string, allowedNames []string) string {
	return weeklychangepolicy.StripNumericCampaignIDs(text, allowedNames)
}

func fixWhatChangedEmptyText(groupKey, emptyText string) string {
	lower := strings.ToLower(strings.TrimSpace(emptyText))
	misleading := strings.Contains(lower, "нет долговремен") ||
		strings.Contains(lower, "нет долгоживущ") ||
		strings.Contains(lower, "за пределами текущего") ||
		strings.Contains(lower, "повторяющихся рисков из прошлой") ||
		(strings.Contains(lower, "публикац") && strings.Contains(lower, "впервые") && strings.Contains(lower, "нет новых"))
	historyMissing := strings.Contains(lower, "впервые") || strings.Contains(lower, "перв") || emptyText == ""
	switch groupKey {
	case whatChangedKeyUnchangedRisks:
		if misleading || historyMissing {
			return weeklychangepolicy.EmptyTextForGroup(weeklychangepolicy.GroupKeyUnchangedRisks, false)
		}
	case whatChangedKeyNewRisks:
		if misleading {
			return weeklychangepolicy.EmptyTextForGroup(weeklychangepolicy.GroupKeyNewRisks, false)
		}
	}
	if strings.TrimSpace(emptyText) == "" {
		return weeklychangepolicy.EmptyTextForGroup(groupKey, true)
	}
	return emptyText
}

func applyWhatChangedToResponse(resp *Response, payload *dashboardNarrativePayload, bundle whatChangedValidationBundle, narrativeApplied bool) {
	source := "missing"
	defer func() {
		if dashboardAIOnlyEnabled() {
			setBlockApplySource(resp, "what_changed", source)
		}
	}()

	if narrativeApplied && payload != nil && payload.WhatChanged != nil {
		cloned := cloneWhatChanged(payload.WhatChanged)
		if dashboardAIOnlyEnabled() {
			bundle = enrichWhatChangedBundleFromSellerPulse(bundle, resp)
			var originalImprovedLabel string
			for _, g := range cloned.Groups {
				if g.Key == whatChangedKeyImproved {
					originalImprovedLabel = strings.TrimSpace(g.Label)
					break
				}
			}
			repaired := repairWhatChangedAIOnly(cloned, bundle)
			repairWhatChangedMainConclusion(repaired, resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
			for i := range repaired.Groups {
				if repaired.Groups[i].Key != whatChangedKeyImproved {
					continue
				}
				if originalImprovedLabel != "" && originalImprovedLabel != repaired.Groups[i].Label {
					recordNarrativeApplyRepair(resp, "what_changed.groups["+itoa(i)+"].label:"+originalImprovedLabel+"->"+repaired.Groups[i].Label)
				}
				break
			}
			bundle = enrichWhatChangedBundleFromPayload(bundle, repaired)
			errs := validateWhatChangedPayload(repaired, bundle)
			if len(errs) == 0 {
				resp.AIDiagnostics.WhatChanged = whatChangedWidgetFromNarrative(repaired)
				source = "ai"
				return
			}
			slog.Warn("dashboard home: what_changed validation failed",
				"errors", errs,
				"period_comparison_facts", len(bundle.Context.PeriodComparison.Facts),
				"improved_facts", len(bundle.Context.ImprovedFacts),
			)
			resp.AIDiagnostics.WhatChanged = WhatChangedWidget{
				WidgetBase: WidgetBase{Status: WidgetSourceNotAvailable, Message: "AI what_changed не прошёл валидацию."},
				Groups:     []WhatChangedGroup{},
			}
			source = "missing"
			return
		}
		normalized := normalizeDashboardNarrativeWhatChanged(cloned)
		repairWhatChangedMainConclusion(normalized, resp.AIDiagnostics.AccountSummary.ExecutiveSummary)
		bundle = enrichWhatChangedBundleFromPayload(bundle, normalized)
		if errs := validateWhatChangedPayload(normalized, bundle); len(errs) == 0 {
			resp.AIDiagnostics.WhatChanged = whatChangedWidgetFromNarrative(normalized)
			source = "ai"
			return
		}
	}
	if dashboardAIOnlyEnabled() {
		resp.AIDiagnostics.WhatChanged = WhatChangedWidget{
			WidgetBase: WidgetBase{Status: WidgetSourceNotAvailable},
			Groups:     []WhatChangedGroup{},
		}
		source = "missing"
		return
	}
	resp.AIDiagnostics.WhatChanged = whatChangedWidgetFromNarrative(buildFallbackWhatChangedPayload(bundle.Context))
	source = "substituted_by_deterministic"
}

func cloneWhatChanged(in *dashboardNarrativeWhatChanged) *dashboardNarrativeWhatChanged {
	if in == nil {
		return nil
	}
	out := *in
	if in.Groups != nil {
		out.Groups = append([]dashboardNarrativeWhatChangedGroup(nil), in.Groups...)
	}
	return &out
}

func repairWhatChangedAIOnly(whatChanged *dashboardNarrativeWhatChanged, bundle whatChangedValidationBundle) *dashboardNarrativeWhatChanged {
	if whatChanged == nil {
		return nil
	}
	var dummy narrativeRepairResult
	working := ensureWhatChangedRequiredGroups(whatChanged, bundle, &dummy)
	out := normalizeDashboardNarrativeWhatChanged(working)
	if out == nil {
		return nil
	}
	fillWhatChangedGroupSummariesFromItems(out)
	if strings.TrimSpace(out.Summary) == "" && len(out.Groups) > 0 {
		for _, g := range out.Groups {
			if s := strings.TrimSpace(g.Summary); s != "" {
				out.Summary = s
				break
			}
		}
	}
	for i := range out.Groups {
		if out.Groups[i].Key != whatChangedKeyImproved {
			continue
		}
		if len(bundle.Context.ImprovedFacts) == 0 {
			// Home validation requires the canonical empty-improved phrases when
			// there are no improved facts. AI often omits group.summary or uses
			// a different empty_text; normalize here so apply does not drop the
			// whole widget to source_not_available.
			if len(out.Groups[i].Items) > 0 && !dashboardAIOnlyEnabled() {
				out.Groups[i].Items = []dashboardNarrativeWhatChangedItem{}
			}
			if !dashboardAIOnlyEnabled() || len(out.Groups[i].Items) == 0 {
				sumLower := strings.ToLower(strings.TrimSpace(out.Groups[i].Summary))
				if sumLower == "" || !strings.Contains(sumLower, "не выявлено") {
					out.Groups[i].Summary = "Заметных улучшений по сравнению с прошлой сводкой не выявлено."
				}
				emptyLower := strings.ToLower(strings.TrimSpace(out.Groups[i].EmptyText))
				if emptyLower == "" || !strings.Contains(emptyLower, "компенсируются") {
					out.Groups[i].EmptyText = "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе."
				}
			}
		}
	}
	if !dashboardAIOnlyEnabled() {
		enrichWhatChangedGroupsFromFacts(out, bundle)
		relabelWhatChangedImprovedGroup(out)
		repairWhatChangedSemantic(out)
	}
	for i := range out.Groups {
		if out.Groups[i].Key != whatChangedKeyImproved {
			continue
		}
		out.Groups[i].Label = resolveImprovedGroupLabel(out.Groups[i])
	}
	enriched := enrichWhatChangedBundleFromPayload(bundle, out)
	sanitizeWhatChangedHighlights(out, enriched)
	return out
}

// fillWhatChangedGroupSummariesFromItems synthesizes a group.summary when the
// model returned items without a summary. Home validation rejects
// missing_summary_when_items; this is structural repair only (no new facts).
func fillWhatChangedGroupSummariesFromItems(whatChanged *dashboardNarrativeWhatChanged) {
	if whatChanged == nil {
		return
	}
	for i := range whatChanged.Groups {
		group := &whatChanged.Groups[i]
		if strings.TrimSpace(group.Summary) != "" || len(group.Items) == 0 {
			continue
		}
		parts := make([]string, 0, len(group.Items))
		for _, item := range group.Items {
			label := strings.TrimSpace(item.Label)
			text := strings.TrimSpace(item.Text)
			switch {
			case label != "" && text != "":
				parts = append(parts, label+": "+text)
			case text != "":
				parts = append(parts, text)
			case label != "":
				parts = append(parts, label)
			}
		}
		if len(parts) == 0 {
			continue
		}
		summary := strings.Join(parts, " ")
		if len([]rune(summary)) > 280 {
			summary = string([]rune(summary)[:277]) + "..."
		}
		group.Summary = summary
	}
}

// sanitizeWhatChangedHighlights drops highlight tokens that would fail
// validateWhatChangedPayload (must appear in item text, allowed numbers, or
// known names). This is technical normalization only — it never invents
// seller-facing copy.
func sanitizeWhatChangedHighlights(whatChanged *dashboardNarrativeWhatChanged, bundle whatChangedValidationBundle) {
	if whatChanged == nil {
		return
	}
	for gi := range whatChanged.Groups {
		group := &whatChanged.Groups[gi]
		for ii := range group.Items {
			item := &group.Items[ii]
			if len(item.Highlights) == 0 {
				continue
			}
			kept := make([]string, 0, len(item.Highlights))
			for _, highlight := range item.Highlights {
				highlight = strings.TrimSpace(highlight)
				if highlight == "" {
					continue
				}
				if whatChangedHighlightIsValid(highlight, item.Text, bundle.AllowedNumbers, bundle.AllowedNames) {
					kept = append(kept, highlight)
				}
			}
			item.Highlights = kept
		}
	}
}

// enrichWhatChangedBundleFromSellerPulse fills empty PeriodComparison facts from
// projected Seller Pulse KPI deltas so unified home apply can validate AI
// what_changed without rebuilding the legacy Seller AI DataPack.
func enrichWhatChangedBundleFromSellerPulse(bundle whatChangedValidationBundle, resp *Response) whatChangedValidationBundle {
	if resp == nil || len(bundle.Context.PeriodComparison.Facts) > 0 {
		return bundle
	}
	fromPulse := periodComparisonFromSellerPulse(resp.MetricDate, resp.SellerPulse)
	if len(fromPulse.Facts) == 0 {
		return bundle
	}
	names := bundle.AllowedNames
	if names == nil {
		names = map[string]struct{}{}
	}
	enriched := buildWhatChangedValidationBundleFromPeriodComparison(fromPulse, names)
	enriched.Context.ImprovedFacts = bundle.Context.ImprovedFacts
	enriched.Context.WorsenedFacts = bundle.Context.WorsenedFacts
	enriched.Context.NewRiskFacts = bundle.Context.NewRiskFacts
	enriched.Context.UnchangedRiskFacts = bundle.Context.UnchangedRiskFacts
	enriched.Context.CampaignNames = bundle.Context.CampaignNames
	enriched.Context.ProductNames = bundle.Context.ProductNames
	return enriched
}

func reorderWhatChangedGroups(groups []dashboardNarrativeWhatChangedGroup) []dashboardNarrativeWhatChangedGroup {
	if dashboardAIOnlyEnabled() {
		for i := range groups {
			g := &groups[i]
			if strings.TrimSpace(g.Key) == "" && (len(g.Items) > 0 || strings.TrimSpace(g.Summary) != "" || strings.TrimSpace(g.EmptyText) != "") {
				fallbackKeys := []string{whatChangedKeyWorsened, whatChangedKeyNewRisks, whatChangedKeyUnchangedRisks, whatChangedKeyImproved}
				if i < len(fallbackKeys) {
					g.Key = fallbackKeys[i]
				}
				if g.Label == "" {
					g.Label = labelForWhatChangedKey(g.Key)
				}
				if g.Tone == "" {
					g.Tone = toneForWhatChangedKey(g.Key)
				}
			}
		}
		// Keep empty required groups (e.g. unchanged_risks with only empty_text).
		// Dropping them here caused home apply to fail with groups_count_mismatch.
	}
	byKey := make(map[string]dashboardNarrativeWhatChangedGroup, len(groups))
	for _, g := range groups {
		key := strings.TrimSpace(g.Key)
		if key == "" {
			continue
		}
		byKey[key] = g
	}
	out := make([]dashboardNarrativeWhatChangedGroup, 0, len(whatChangedRequiredGroups))
	for _, spec := range whatChangedRequiredGroups {
		if g, ok := byKey[spec.Key]; ok {
			out = append(out, g)
		}
	}
	return out
}

func normalizeDashboardNarrativeWhatChanged(whatChanged *dashboardNarrativeWhatChanged) *dashboardNarrativeWhatChanged {
	if whatChanged == nil {
		return nil
	}
	whatChanged.Summary = cleanupSellerFacingText(whatChanged.Summary)
	if whatChanged.Groups == nil {
		whatChanged.Groups = []dashboardNarrativeWhatChangedGroup{}
	}
	for i := range whatChanged.Groups {
		whatChanged.Groups[i].Key = strings.TrimSpace(whatChanged.Groups[i].Key)
		if label := labelForWhatChangedKey(whatChanged.Groups[i].Key); label != "" {
			whatChanged.Groups[i].Label = label
		} else {
			whatChanged.Groups[i].Label = cleanupSellerFacingText(whatChanged.Groups[i].Label)
		}
		if tone := toneForWhatChangedKey(whatChanged.Groups[i].Key); tone != "" {
			whatChanged.Groups[i].Tone = tone
		} else {
			whatChanged.Groups[i].Tone = strings.TrimSpace(whatChanged.Groups[i].Tone)
		}
		whatChanged.Groups[i].Summary = cleanupSellerFacingText(whatChanged.Groups[i].Summary)
		whatChanged.Groups[i].EmptyText = cleanupSellerFacingText(whatChanged.Groups[i].EmptyText)
		if whatChanged.Groups[i].Items == nil {
			whatChanged.Groups[i].Items = []dashboardNarrativeWhatChangedItem{}
		}
		for j := range whatChanged.Groups[i].Items {
			whatChanged.Groups[i].Items[j].Label = cleanupSellerFacingText(whatChanged.Groups[i].Items[j].Label)
			whatChanged.Groups[i].Items[j].Text = cleanupSellerFacingText(whatChanged.Groups[i].Items[j].Text)
			whatChanged.Groups[i].Items[j].Highlights = compactStringSlice(whatChanged.Groups[i].Items[j].Highlights)
		}
	}
	whatChanged.Groups = reorderWhatChangedGroups(whatChanged.Groups)
	return whatChanged
}
