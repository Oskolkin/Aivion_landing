package weeklychangepolicy

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// ActionRef is a minimal action handle for linking.
type ActionRef struct {
	AnalysisActionID string
	Category         string
	ScopeType        string
	EntityIDs        []string
	SourceSignalIDs  []string
	Title            string
}

// AttachActionLinks sets RelatedActionID / DeepLink when a matching action exists.
func AttachActionLinks(candidates []ChangeCandidate, actions []ActionRef) []ChangeCandidate {
	if len(actions) == 0 {
		return candidates
	}
	out := make([]ChangeCandidate, len(candidates))
	copy(out, candidates)
	for i := range out {
		c := &out[i]
		if c.ChangeType == ChangeImproved {
			continue
		}
		best := matchAction(*c, actions)
		if best == nil {
			continue
		}
		c.RelatedActionID = best.AnalysisActionID
		c.DeepLink = "/app/actions?action=" + best.AnalysisActionID
		if c.ActionLinkLabel == "" {
			c.ActionLinkLabel = "Открыть действие"
		}
	}
	return out
}

func matchAction(c ChangeCandidate, actions []ActionRef) *ActionRef {
	// 1) Exact source signal / change_id match.
	for i := range actions {
		a := &actions[i]
		for _, sig := range a.SourceSignalIDs {
			if sig == c.ChangeID || sig == c.MetricCode || sig == c.RelatedDriverID {
				return a
			}
		}
	}
	// 2) Entity overlap + compatible scope.
	bestScore := 0
	var best *ActionRef
	for i := range actions {
		a := &actions[i]
		score := 0
		if c.EntityType != "" && strings.EqualFold(a.ScopeType, c.EntityType) {
			score += 2
		}
		if domainMatchesCategory(c.Domain, a.Category) {
			score += 2
		}
		overlap := entityOverlap(c.EntityIDs, a.EntityIDs)
		score += overlap * 3
		if score > bestScore && (overlap > 0 || score >= 4) {
			bestScore = score
			best = a
		}
	}
	return best
}

func domainMatchesCategory(domain, category string) bool {
	d := strings.ToLower(strings.TrimSpace(domain))
	c := strings.ToLower(strings.TrimSpace(category))
	if d == "" || c == "" {
		return false
	}
	if d == c {
		return true
	}
	switch d {
	case "advertising":
		return c == "ads" || c == "advertising" || c == "реклама"
	case "stock", "no_sales", "surplus":
		return c == "stock" || c == "sku" || c == "остатки"
	case "funnel":
		return c == "funnel" || c == "воронка"
	case "sales":
		return c == "sales" || c == "продажи"
	default:
		return false
	}
}

func entityOverlap(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, id := range a {
		set[strings.TrimSpace(id)] = struct{}{}
	}
	n := 0
	for _, id := range b {
		if _, ok := set[strings.TrimSpace(id)]; ok {
			n++
		}
	}
	return n
}

// ProjectedItem is the seller-facing weekly change card payload.
type ProjectedItem struct {
	ID                string   `json:"id"`
	Domain            string   `json:"domain,omitempty"`
	Severity          string   `json:"severity,omitempty"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	CurrentValueText  string   `json:"current_value_text,omitempty"`
	PreviousValueText string   `json:"previous_value_text,omitempty"`
	DeltaText         string   `json:"delta_text,omitempty"`
	ImpactText        string   `json:"impact_text,omitempty"`
	EntityCount       int      `json:"entity_count,omitempty"`
	EntityNames       []string `json:"entity_names,omitempty"`
	EvidenceFactIDs   []string `json:"evidence_fact_ids,omitempty"`
	LinkedActionID    string   `json:"linked_action_id,omitempty"`
	ActionLinkLabel   string   `json:"action_link_label,omitempty"`
	DeepLink          string   `json:"deep_link,omitempty"`
	Highlights        []string `json:"highlights,omitempty"`
	ChangeID          string   `json:"change_id,omitempty"`
	MaterialityScore  int      `json:"materiality_score,omitempty"`
}

// ToProjectedItem converts a candidate into a seller-facing item.
// Optional AI overlay may replace title/description when grounded.
func ToProjectedItem(c ChangeCandidate, aiTitle, aiText string) ProjectedItem {
	title := strings.TrimSpace(c.TitleCandidate)
	desc := strings.TrimSpace(c.DescriptionCandidate)
	if t := strings.TrimSpace(aiTitle); t != "" && !ContainsNumericCampaignID(t) {
		title = StripNumericCampaignIDs(t, c.EntityNames)
	}
	if t := strings.TrimSpace(aiText); t != "" && !ContainsNumericCampaignID(t) {
		desc = StripNumericCampaignIDs(t, c.EntityNames)
	} else if t := strings.TrimSpace(aiText); ContainsNumericCampaignID(t) {
		desc = StripNumericCampaignIDs(t, c.EntityNames)
		if strings.TrimSpace(desc) == "" || ContainsNumericCampaignID(desc) {
			desc = c.DescriptionCandidate
		}
	}
	title = StripNumericCampaignIDs(title, c.EntityNames)
	desc = StripNumericCampaignIDs(desc, c.EntityNames)

	highlights := make([]string, 0, 4)
	if c.CurrentValueText != "" {
		highlights = append(highlights, c.CurrentValueText)
	}
	if c.DeltaText != "" {
		highlights = append(highlights, c.DeltaText)
	}
	for _, n := range c.EntityNames {
		if len(highlights) >= 4 {
			break
		}
		highlights = append(highlights, n)
	}

	return ProjectedItem{
		ID:                c.ChangeID,
		Domain:            c.Domain,
		Severity:          c.Severity,
		Title:             title,
		Description:       desc,
		CurrentValueText:  c.CurrentValueText,
		PreviousValueText: c.PreviousValueText,
		DeltaText:         c.DeltaText,
		ImpactText:        c.ImpactText,
		EntityCount:       c.EntityCount,
		EntityNames:       append([]string{}, c.EntityNames...),
		EvidenceFactIDs:   append([]string{}, c.EvidenceFactIDs...),
		LinkedActionID:    c.RelatedActionID,
		ActionLinkLabel:   c.ActionLinkLabel,
		DeepLink:          c.DeepLink,
		Highlights:        highlights,
		ChangeID:          c.ChangeID,
		MaterialityScore:  c.MaterialityScore,
	}
}

// BuildGroupsFromRegistry builds the four home groups from registry candidates.
// AI items (by signal_id / change_id) overlay copy when present.
func BuildGroupsFromRegistry(
	reg RegistryResult,
	ai *contract.UnifiedWhatChangedV1,
	maxPerGroup int,
) (summary string, mainConclusion string, groups []ProjectedGroup) {
	if maxPerGroup <= 0 {
		maxPerGroup = 3
	}
	aiByChangeID := map[string]contract.UnifiedWhatChangedItemV1{}
	aiByGroup := map[string][]contract.UnifiedWhatChangedItemV1{}
	if ai != nil {
		summary = strings.TrimSpace(ai.Summary)
		mainConclusion = strings.TrimSpace(ai.MainConclusion)
		for _, g := range ai.Groups {
			key := strings.TrimSpace(g.Key)
			for _, it := range g.Items {
				aiByGroup[key] = append(aiByGroup[key], it)
				if id := strings.TrimSpace(it.SignalID); id != "" {
					aiByChangeID[id] = it
				}
			}
		}
	}

	order := []string{GroupKeyWorsened, GroupKeyNewRisks, GroupKeyUnchangedRisks, GroupKeyImproved}
	for _, key := range order {
		cands := reg.CandidatesByGroup(key)
		items := make([]ProjectedItem, 0, maxPerGroup)
		used := map[string]struct{}{}
		// Prefer AI-selected change_ids that exist in registry.
		for _, it := range aiByGroup[key] {
			id := strings.TrimSpace(it.SignalID)
			var match *ChangeCandidate
			for i := range cands {
				if cands[i].ChangeID == id || cands[i].MetricCode == id {
					match = &cands[i]
					break
				}
			}
			if match == nil {
				continue
			}
			if _, ok := used[match.ChangeID]; ok {
				continue
			}
			items = append(items, ToProjectedItem(*match, it.Label, it.Text))
			used[match.ChangeID] = struct{}{}
			if len(items) >= maxPerGroup {
				break
			}
		}
		// Fill remaining slots from registry materiality order.
		for _, c := range cands {
			if len(items) >= maxPerGroup {
				break
			}
			if _, ok := used[c.ChangeID]; ok {
				continue
			}
			var aiTitle, aiText string
			if it, ok := aiByChangeID[c.ChangeID]; ok {
				aiTitle, aiText = it.Label, it.Text
			}
			items = append(items, ToProjectedItem(c, aiTitle, aiText))
			used[c.ChangeID] = struct{}{}
		}

		empty := reg.EmptyTextByGroup[key]
		groupSummary := ""
		if len(items) == 0 {
			groupSummary = empty
		}
		groups = append(groups, ProjectedGroup{
			Key:            key,
			Label:          DefaultGroupLabel(key),
			Tone:           DefaultGroupTone(key),
			Summary:        groupSummary,
			EmptyText:      empty,
			Items:          items,
			TotalCount:     len(cands),
			RemainingCount: maxInt(0, len(cands)-len(items)),
		})
	}
	return summary, mainConclusion, groups
}

// ProjectedGroup is one of the four weekly-change columns.
type ProjectedGroup struct {
	Key            string          `json:"key"`
	Label          string          `json:"label"`
	Tone           string          `json:"tone"`
	Summary        string          `json:"summary,omitempty"`
	EmptyText      string          `json:"empty_text,omitempty"`
	Items          []ProjectedItem `json:"items"`
	TotalCount     int             `json:"total_count"`
	RemainingCount int             `json:"remaining_count"`
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ValidateRegistrySemantics runs structural checks on classified candidates.
func ValidateRegistrySemantics(reg RegistryResult) []string {
	var errs []string
	seen := map[string]ChangeType{}
	for _, c := range reg.Candidates {
		if prev, ok := seen[c.ChangeID]; ok {
			errs = append(errs, "duplicate change_id across types: "+c.ChangeID+" ("+string(prev)+" vs "+string(c.ChangeType)+")")
		}
		seen[c.ChangeID] = c.ChangeType

		switch c.ChangeType {
		case ChangeWorsened:
			if c.CurrentValue == nil || c.PreviousValue == nil {
				errs = append(errs, "worsened missing current/previous: "+c.ChangeID)
			} else if !isNegativeDelta(c) {
				errs = append(errs, "worsened without negative delta: "+c.ChangeID)
			}
		case ChangeImproved:
			if c.CurrentValue == nil || c.PreviousValue == nil {
				errs = append(errs, "improved missing current/previous: "+c.ChangeID)
			} else if !isPositiveDelta(c) {
				errs = append(errs, "improved without positive delta: "+c.ChangeID)
			}
		case ChangeNewRisk:
			if !reg.HistoryAvailable {
				errs = append(errs, "new_risk without history context: "+c.ChangeID)
			}
		case ChangeActiveProblem:
			if !reg.HistoryAvailable && (c.PreviousValue == nil || c.CurrentValue == nil || *c.PreviousValue != *c.CurrentValue) {
				// Inherently persistent problems (same previous=current) are allowed
				// without publication history; otherwise require history.
				errs = append(errs, "active_problem without history context: "+c.ChangeID)
			}
		}
		if ContainsNumericCampaignID(c.TitleCandidate) || ContainsNumericCampaignID(c.DescriptionCandidate) {
			errs = append(errs, "candidate copy contains numeric campaign id: "+c.ChangeID)
		}
	}
	return errs
}

func isNegativeDelta(c ChangeCandidate) bool {
	if c.AbsoluteDelta == nil {
		return false
	}
	// For higher-is-worse metrics (stockout count), positive abs delta is worsening.
	switch c.MetricCode {
	case MetricStockoutSKUCount, MetricCriticalSKUCount, MetricShortageSKUCount,
		MetricNoSalesWithStock, MetricSurplusSKUCount, MetricHighDRRCampaigns,
		MetricSpendWithoutOrders, MetricAdShareOfRevenue:
		return *c.AbsoluteDelta > 0
	default:
		return *c.AbsoluteDelta < 0
	}
}

func isPositiveDelta(c ChangeCandidate) bool {
	if c.AbsoluteDelta == nil {
		return false
	}
	switch c.MetricCode {
	case MetricStockoutSKUCount, MetricCriticalSKUCount, MetricShortageSKUCount,
		MetricNoSalesWithStock, MetricSurplusSKUCount, MetricHighDRRCampaigns,
		MetricSpendWithoutOrders, MetricAdShareOfRevenue:
		return *c.AbsoluteDelta < 0
	default:
		return *c.AbsoluteDelta > 0
	}
}
