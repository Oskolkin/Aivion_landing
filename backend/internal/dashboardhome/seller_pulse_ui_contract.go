package dashboardhome

import "strings"

const sellerPulseUIContractSource = "dashboard_narrative_ai_only"

// finalizeSellerPulseUIContract fills seller-facing important_notes and quick_actions
// when AI narrative is available but those arrays are empty.
func finalizeSellerPulseUIContract(resp *Response) {
	if resp == nil || resp.SellerPulse.Narrative == nil {
		return
	}
	n := resp.SellerPulse.Narrative
	if n.Status != "available" && n.Status != "ready" {
		return
	}
	if len(n.ImportantNotes) == 0 {
		n.ImportantNotes = buildSellerPulseImportantNotesFromRiskDrivers(resp)
	}
	if len(n.QuickActions) == 0 {
		n.QuickActions = buildSellerPulseQuickActionsFromTopActions(resp)
	}
	repairSellerPulseQuickActionKinds(n, resp.AIDiagnostics.TopActions, resp)
	finalizeSellerPulseKPITrends(resp)
	normalizeSellerPulseNarrativePayload(n)
}

func finalizeSellerPulseKPITrends(resp *Response) {
	if resp == nil {
		return
	}
	for i := range resp.SellerPulse.KPIs {
		kpi := &resp.SellerPulse.KPIs[i]
		if countSparklineDataPoints(kpi.Sparkline7D) < 2 {
			continue
		}
		label := "Динамика за 7 дней"
		if kpi.Delta != nil {
			if deltaLabel := strings.TrimSpace(kpi.Delta.Label); deltaLabel != "" {
				label = deltaLabel
			}
		}
		kpi.Trend = &KPICardTrend{Status: "available", Label: label}
	}
}

func buildSellerPulseImportantNotesFromRiskDrivers(resp *Response) []SellerPulseImportantNote {
	drivers := resp.AIDiagnostics.AccountSummary.RiskDrivers
	if len(drivers) == 0 {
		if summary := strings.TrimSpace(resp.AIDiagnostics.AccountSummary.ExecutiveSummary); summary != "" {
			return []SellerPulseImportantNote{{
				Title: "Главное",
				Text:  summary,
				Tone:  toneForSeverity(resp.AIDiagnostics.AccountSummary.Severity),
			}}
		}
		return nil
	}
	limit := minInt(len(drivers), 4)
	if limit < 2 && len(drivers) > 0 {
		limit = len(drivers)
	}
	out := make([]SellerPulseImportantNote, 0, limit)
	for i := 0; i < limit && i < len(drivers); i++ {
		d := drivers[i]
		text := strings.TrimSpace(d.Reason)
		if text == "" {
			text = strings.TrimSpace(d.Title)
		}
		if text == "" {
			continue
		}
		out = append(out, SellerPulseImportantNote{
			Title: strings.TrimSpace(d.Title),
			Text:  text,
			Tone:  toneForSeverity(d.Severity),
		})
	}
	return out
}

func buildSellerPulseQuickActionsFromTopActions(resp *Response) []SellerPulseQuickAction {
	items := resp.AIDiagnostics.TopActions
	if len(items) == 0 {
		items = topActionsFromRecommended(resp.RecommendedActions)
	}
	if len(items) == 0 {
		return nil
	}
	limit := minInt(len(items), 3)
	out := make([]SellerPulseQuickAction, 0, limit)
	for i := 0; i < limit; i++ {
		item := items[i]
		label := strings.TrimSpace(item.Title)
		if label == "" {
			continue
		}
		href := actionDetailHref(item.ActionKey, item.ActionID)
		if href == "/app/actions" {
			if key := strings.TrimSpace(item.DriverID); key != "" {
				href = actionDetailHref(key, 0)
			} else if key := strings.TrimSpace(item.ID); key != "" && !strings.HasPrefix(key, "ai-only-") {
				href = actionDetailHref(key, 0)
			}
		}
		out = append(out, SellerPulseQuickAction{
			Label:       label,
			Description: firstNonEmptyTrim(strings.TrimSpace(item.NextStep), strings.TrimSpace(item.ExpectedEffect)),
			Priority:    normalizeTopActionPriority(item.Priority),
			Kind:        "action",
			Href:        href,
			ActionID:    item.ActionID,
			ActionKey:   firstNonEmptyTrim(item.ActionKey, item.DriverID, item.ID),
		})
	}
	return out
}

func firstNonEmptyTrim(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func topActionsFromRecommended(actions []ActionItem) []TopActionItem {
	out := make([]TopActionItem, 0, len(actions))
	for _, a := range actions {
		if strings.TrimSpace(a.Action) == "" {
			continue
		}
		out = append(out, TopActionItem{
			Title:          a.Action,
			Reason:         a.Reason,
			ExpectedEffect: a.ExpectedEffect,
			Priority:       a.Priority,
			Category:       a.UrgencyGroup,
		})
	}
	return out
}

func sellerPulseKindForCategory(category string) string {
	kind, _ := sellerPulseKindForTopAction(category)
	return kind
}

func toneForSeverity(severity string) string {
	switch normalizeTopActionPriority(severity) {
	case "critical":
		return "critical"
	case "high":
		return "warning"
	case "medium":
		return "warning"
	default:
		return "neutral"
	}
}
