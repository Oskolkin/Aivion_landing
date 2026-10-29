package aipipeline

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

const FinalOutputSemanticValidationSchemaVersion = "final_output_semantic_validation_v1"

// FinalOutputSemanticValidation captures publication-time semantic checks.
type FinalOutputSemanticValidation struct {
	SchemaVersion      string                         `json:"schema_version"`
	Dashboard          FinalOutputComponentValidation `json:"dashboard"`
	Actions            FinalOutputComponentValidation `json:"actions"`
	PublicationAllowed bool                           `json:"publication_allowed"`
}

type FinalOutputComponentValidation struct {
	Status             string                          `json:"status"`
	BlockingIssueCount int                             `json:"blocking_issue_count"`
	WarningCount       int                             `json:"warning_count"`
	Issues             []aidatapack.FinalSemanticIssue `json:"issues,omitempty"`
}

// ValidateFinalDashboardOutput checks seller-facing dashboard narrative text.
func ValidateFinalDashboardOutput(pack *aidatapack.SellerAIDataPack, raw json.RawMessage) FinalOutputComponentValidation {
	result := FinalOutputComponentValidation{Status: "passed"}
	result.Issues = append(result.Issues, dashboardhome.ValidateFinalNarrativePlaceholders(raw, func(text string) []dashboardhome.PlaceholderFinding {
		var out []dashboardhome.PlaceholderFinding
		for _, ph := range DetectUnresolvedPlaceholders(text) {
			out = append(out, dashboardhome.PlaceholderFinding{
				MatchedFragment: ph.MatchedFragment,
				ContextSnippet:  ph.ContextSnippet,
			})
		}
		return out
	})...)
	result.Issues = append(result.Issues, dashboardhome.ValidateFinalNarrativeSemantics(pack, raw)...)
	finalizeComponentValidation(&result)
	return result
}

// ValidateFinalDashboardOutputFromJSON parses narrative JSON and validates seller-facing text.
func ValidateFinalDashboardOutputFromJSON(pack *aidatapack.SellerAIDataPack, raw json.RawMessage) FinalOutputComponentValidation {
	return ValidateFinalDashboardOutput(pack, raw)
}

// ValidateFinalActionsOutput checks persisted/API action payloads.
func ValidateFinalActionsOutput(pack *aidatapack.SellerAIDataPack, items []actions.ActionItem) FinalOutputComponentValidation {
	result := FinalOutputComponentValidation{Status: "passed"}
	for i, item := range items {
		prefix := "actions[" + itoa(i) + "]"
		actionFacts := append([]string(nil), item.EvidenceFactIDs...)
		for _, id := range item.SourceCandidateIDs {
			_ = id
		}
		actionMetrics := aidatapack.MetricCodesFromFactIDs(pack, actionFacts)
		for _, text := range []struct {
			path string
			val  string
		}{
			{prefix + ".title", item.Title},
			{prefix + ".summary", item.Summary},
			{prefix + ".reason", item.Reason},
			{prefix + ".next_step", item.NextStep},
			{prefix + ".expected_effect", item.ExpectedEffect},
			{prefix + ".completion_criteria", item.CompletionCriteria},
		} {
			appendFinalSemanticIssues(&result, pack, text.val, text.path, actionFacts)
			_ = actionMetrics
		}
		if item.OperationalDetail != nil {
			validateOperationalDetailTexts(&result, pack, prefix, item.ActionKey, item.OperationalDetail, actionFacts)
		}
		for _, ev := range item.Evidence {
			appendFinalSemanticIssues(&result, pack, ev.Label, prefix+".evidence.label", actionFacts)
			appendFinalSemanticIssues(&result, pack, ev.Description, prefix+".evidence.description", actionFacts)
		}
	}
	finalizeComponentValidation(&result)
	return result
}

func validateOperationalDetailTexts(
	result *FinalOutputComponentValidation,
	pack *aidatapack.SellerAIDataPack,
	prefix, actionKey string,
	detail *actions.OperationalDetail,
	actionFacts []string,
) {
	if detail == nil {
		return
	}
	for _, text := range []struct{ path, val string }{
		{prefix + ".operational_detail.why.headline", detail.Why.Headline},
		{prefix + ".operational_detail.why.text", detail.Why.Text},
		{prefix + ".operational_detail.action_plan.headline", detail.ActionPlan.Headline},
	} {
		appendFinalSemanticIssues(result, pack, text.val, text.path, actionFacts)
	}
	for si, step := range detail.ActionPlan.Steps {
		stepPath := prefix + ".operational_detail.action_plan.steps[" + itoa(si) + "]"
		stepFacts := append(append([]string{}, actionFacts...), step.EvidenceFactIDs...)
		stepMetrics := aidatapack.MetricCodesFromFactIDs(pack, stepFacts)
		for _, text := range []struct{ path, val string }{
			{stepPath + ".title", step.Title},
			{stepPath + ".instruction", step.Instruction},
			{stepPath + ".why_this_step", step.WhyThisStep},
			{stepPath + ".expected_result", step.ExpectedResult},
		} {
			appendFinalSemanticIssuesWithMetrics(result, pack, text.val, text.path, stepFacts, stepMetrics)
		}
	}
	for oi, obj := range detail.ObjectActions {
		objPath := prefix + ".operational_detail.object_actions[" + itoa(oi) + "]"
		objMetrics := metricCodesFromObjectMetrics(obj.Metrics)
		for _, text := range []struct{ path, val string }{
			{objPath + ".recommended_action", obj.RecommendedAction},
			{objPath + ".reason", obj.Reason},
		} {
			appendFinalSemanticIssuesWithMetricsAndEntity(result, pack, text.val, text.path, actionFacts, objMetrics, actionKey, obj.EntityID)
		}
		for mi, metric := range obj.Metrics {
			metricPath := objPath + ".metrics[" + itoa(mi) + "]"
			metricCodes := append(append([]string(nil), objMetrics...), strings.TrimSpace(metric.MetricCode))
			appendFinalSemanticIssuesWithMetricsAndEntity(result, pack, metric.Label, metricPath+".label", actionFacts, metricCodes, actionKey, obj.EntityID)
			appendFinalSemanticIssuesWithMetricsAndEntity(result, pack, metric.FormattedValue, metricPath+".formatted_value", actionFacts, metricCodes, actionKey, obj.EntityID)
			appendFinalSemanticIssuesWithMetricsAndEntity(result, pack, metric.DisplayLabel, metricPath+".display_label", actionFacts, metricCodes, actionKey, obj.EntityID)
		}
	}
}

func metricCodesFromObjectMetrics(metrics []actions.OperationalMetricRef) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range metrics {
		code := strings.TrimSpace(m.MetricCode)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func appendFinalSemanticIssuesWithMetricsAndEntity(
	result *FinalOutputComponentValidation,
	pack *aidatapack.SellerAIDataPack,
	text, fieldPath string,
	factIDs, metricCodes []string,
	actionKey, entityID string,
) {
	before := len(result.Issues)
	appendFinalSemanticIssuesWithMetrics(result, pack, text, fieldPath, factIDs, metricCodes)
	for i := before; i < len(result.Issues); i++ {
		if result.Issues[i].ActionKey == "" {
			result.Issues[i].ActionKey = actionKey
		}
		if result.Issues[i].EntityID == "" {
			result.Issues[i].EntityID = entityID
		}
	}
}

func appendFinalSemanticIssues(result *FinalOutputComponentValidation, pack *aidatapack.SellerAIDataPack, text, fieldPath string, factIDs []string) {
	metrics := aidatapack.MetricCodesFromFactIDs(pack, factIDs)
	appendFinalSemanticIssuesWithMetrics(result, pack, text, fieldPath, factIDs, metrics)
}

func appendFinalSemanticIssuesWithMetrics(
	result *FinalOutputComponentValidation,
	pack *aidatapack.SellerAIDataPack,
	text, fieldPath string,
	factIDs, metricCodes []string,
) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	for _, issue := range DetectUnresolvedPlaceholders(text) {
		result.Issues = append(result.Issues, aidatapack.FinalSemanticIssue{
			Code:           aidatapack.FinalUnresolvedNumericPlaceholderCode,
			Severity:       "blocking",
			FieldPath:      fieldPath,
			ContextSnippet: issue.ContextSnippet,
			SourceFactIDs:  append([]string(nil), factIDs...),
		})
	}
	for _, issue := range aidatapack.FinalMetricSemanticIssues(text, metricCodes, fieldPath, factIDs) {
		result.Issues = append(result.Issues, issue)
	}
	for _, issue := range aidatapack.DetectUnsupportedShelfLifeClaim(text, pack, fieldPath) {
		issue.SourceFactIDs = append([]string(nil), factIDs...)
		result.Issues = append(result.Issues, issue)
	}
}

func finalizeComponentValidation(result *FinalOutputComponentValidation) {
	blocking := 0
	for _, issue := range result.Issues {
		if issue.Severity == "blocking" {
			blocking++
		}
	}
	result.BlockingIssueCount = blocking
	if blocking > 0 {
		result.Status = "failed"
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
