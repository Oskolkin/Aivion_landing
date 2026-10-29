package dashboardhome

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// FinalSellerFacingTextField is one seller-facing text node with grounding context.
type FinalSellerFacingTextField struct {
	FieldPath     string
	Text          string
	SourceFactIDs []string
}

// CollectFinalSellerFacingTextFields walks accepted narrative payload text nodes.
func CollectFinalSellerFacingTextFields(payload *dashboardNarrativePayload) []FinalSellerFacingTextField {
	if payload == nil {
		return nil
	}
	var out []FinalSellerFacingTextField
	add := func(path, text string, factIDs []string) {
		if stringsTrim(text) == "" {
			return
		}
		out = append(out, FinalSellerFacingTextField{
			FieldPath: path, Text: text, SourceFactIDs: append([]string(nil), factIDs...),
		})
	}
	add("headline", payload.Headline, nil)
	add("executive_summary", payload.ExecutiveSummary, nil)
	add("improvement_summary", payload.ImprovementSummary, nil)
	for i, note := range payload.DataQualityNotes {
		add("data_quality_notes["+strconv.Itoa(i)+"]", note, nil)
	}
	for i, r := range payload.HealthReasons {
		add("health_reasons["+strconv.Itoa(i)+"].title", r.Title, r.FactIDs)
		add("health_reasons["+strconv.Itoa(i)+"].reason", r.Reason, r.FactIDs)
	}
	for i, a := range payload.TopActions {
		facts := append([]string(nil), a.FactIDs...)
		add("top_actions["+strconv.Itoa(i)+"].title", a.Title, facts)
		add("top_actions["+strconv.Itoa(i)+"].reason", a.Reason, facts)
		add("top_actions["+strconv.Itoa(i)+"].next_step", a.NextStep, facts)
		add("top_actions["+strconv.Itoa(i)+"].expected_effect", a.ExpectedEffect, facts)
	}
	if payload.StockCoverageNarrative != nil {
		n := payload.StockCoverageNarrative
		add("stock_coverage.summary", n.Summary, nil)
		add("stock_coverage.risk_comment", n.RiskComment, nil)
		add("stock_coverage.no_sales_insight", n.NoSalesInsight, nil)
	}
	if payload.AdEfficiencyNarrative != nil {
		add("ad_efficiency.summary", payload.AdEfficiencyNarrative.Summary, nil)
	}
	if payload.SellerPulseNarrative != nil {
		n := payload.SellerPulseNarrative
		add("seller_pulse.headline", n.Headline, nil)
		add("seller_pulse.summary", n.Summary, nil)
	}
	if payload.WhatChanged != nil {
		wc := payload.WhatChanged
		add("what_changed.summary", wc.Summary, nil)
		for gi, group := range wc.Groups {
			gp := "what_changed.groups[" + strconv.Itoa(gi) + "]"
			add(gp+".summary", group.Summary, nil)
			add(gp+".empty_text", group.EmptyText, nil)
			for ii, item := range group.Items {
				ip := gp + ".items[" + strconv.Itoa(ii) + "]"
				add(ip+".label", item.Label, nil)
				add(ip+".text", item.Text, nil)
			}
		}
	}
	return out
}

// ValidateFinalNarrativeSemantics validates seller-facing narrative text before publication.
func ValidateFinalNarrativeSemantics(pack *aidatapack.SellerAIDataPack, raw json.RawMessage) []aidatapack.FinalSemanticIssue {
	if len(raw) == 0 {
		return nil
	}
	payload, err := parseDashboardNarrativePayload(string(raw))
	if err != nil {
		return []aidatapack.FinalSemanticIssue{{
			Code: "final_dashboard_payload_invalid", Severity: "blocking", FieldPath: "validated_payload",
		}}
	}
	return validateFinalSellerFacingFields(pack, CollectFinalSellerFacingTextFields(payload))
}

// PlaceholderFinding is a unresolved placeholder match in seller-facing text.
type PlaceholderFinding struct {
	MatchedFragment string
	ContextSnippet  string
}

// ValidateFinalNarrativePlaceholders scans narrative payload for unresolved placeholders.
func ValidateFinalNarrativePlaceholders(
	raw json.RawMessage,
	detect func(text string) []PlaceholderFinding,
) []aidatapack.FinalSemanticIssue {
	if len(raw) == 0 || detect == nil {
		return nil
	}
	payload, err := parseDashboardNarrativePayload(string(raw))
	if err != nil {
		return nil
	}
	var issues []aidatapack.FinalSemanticIssue
	for _, field := range CollectFinalSellerFacingTextFields(payload) {
		for _, ph := range detect(field.Text) {
			issues = append(issues, aidatapack.FinalSemanticIssue{
				Code: aidatapack.FinalUnresolvedNumericPlaceholderCode, Severity: "blocking",
				FieldPath: field.FieldPath, ContextSnippet: ph.ContextSnippet,
				SourceFactIDs: append([]string(nil), field.SourceFactIDs...),
			})
		}
	}
	return issues
}

func validateFinalSellerFacingFields(pack *aidatapack.SellerAIDataPack, fields []FinalSellerFacingTextField) []aidatapack.FinalSemanticIssue {
	var issues []aidatapack.FinalSemanticIssue
	for _, field := range fields {
		metrics := aidatapack.MetricCodesFromFactIDs(pack, field.SourceFactIDs)
		issues = append(issues, aidatapack.FinalMetricSemanticIssues(field.Text, metrics, field.FieldPath, field.SourceFactIDs)...)
		issues = append(issues, aidatapack.DetectUnsupportedShelfLifeClaim(field.Text, pack, field.FieldPath)...)
	}
	return issues
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}
