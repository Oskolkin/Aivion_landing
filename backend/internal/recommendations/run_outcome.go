package recommendations

import (
	"encoding/json"
	"strings"
)

const (
	errStageValidation   = "validation"
	errStageAIEmpty      = "ai_empty"
	errStageOpenAI       = "openai"
	errStageContext      = "context"
	errStageContextParse = "validation_parse"
)

const (
	errMsgAIEmptyWithAlerts       = "AI returned zero recommendations despite open alerts"
	errMsgAllRejectedByValidator  = "AI recommendations rejected by validator"
	errMsgZeroSavedWithOpenAlerts = "AI returned zero valid recommendations despite open alerts"
)

type runOutcome struct {
	FailRun      bool
	ErrorStage   string
	ErrorMessage string
	Warnings     []string
}

func evaluateRunOutcome(ctx *AIRecommendationContext, validation *ValidationResult) runOutcome {
	openAlerts := countOpenAlertsInContext(ctx)
	validN := 0
	rejectedN := 0
	generatedN := 0
	if validation != nil {
		validN = len(validation.ValidRecommendations)
		rejectedN = len(validation.RejectedRecommendations)
		generatedN = validation.TotalRecommendations
	}

	if openAlerts > 0 && generatedN == 0 {
		return runOutcome{
			FailRun:      true,
			ErrorStage:   errStageAIEmpty,
			ErrorMessage: errMsgAIEmptyWithAlerts,
		}
	}

	if generatedN > 0 && validN == 0 {
		return runOutcome{
			FailRun:      true,
			ErrorStage:   errStageValidation,
			ErrorMessage: errMsgAllRejectedByValidator,
		}
	}

	out := runOutcome{}
	if validN > 0 && rejectedN > 0 {
		out.Warnings = []string{
			"partial_validation: some AI recommendations were rejected by validator",
		}
	}
	return out
}

func evaluateAfterSave(openAlerts, savedCount int, validation *ValidationResult, prior runOutcome) runOutcome {
	if prior.FailRun {
		return prior
	}
	if openAlerts > 0 && savedCount == 0 {
		out := runOutcome{
			FailRun:      true,
			ErrorStage:   errStageValidation,
			ErrorMessage: errMsgZeroSavedWithOpenAlerts,
		}
		if validation != nil && validation.TotalRecommendations > 0 && len(validation.RejectedRecommendations) > 0 {
			out.ErrorMessage = errMsgAllRejectedByValidator
		} else if validation != nil && validation.TotalRecommendations == 0 {
			out.ErrorStage = errStageAIEmpty
			out.ErrorMessage = errMsgAIEmptyWithAlerts
		}
		return out
	}
	return prior
}

func countOpenAlertsInContext(ctx *AIRecommendationContext) int {
	if ctx == nil {
		return 0
	}
	n := int(ctx.Alerts.OpenTotal)
	if len(ctx.Alerts.TopOpen) > n {
		n = len(ctx.Alerts.TopOpen)
	}
	return n
}

func countCriticalHighAlerts(alerts []AlertSignal) int {
	n := 0
	for _, a := range alerts {
		switch strings.ToLower(strings.TrimSpace(a.Severity)) {
		case "critical", "high":
			n++
		}
	}
	return n
}

func buildContextDiagnosticsSummary(ctx *AIRecommendationContext) map[string]any {
	if ctx == nil {
		return map[string]any{}
	}
	summary := map[string]any{
		"open_alerts_count":          countOpenAlertsInContext(ctx),
		"open_critical_high_count":   ctx.Alerts.OpenCriticalHighCount,
		"context_truncated":          ctx.ContextTruncated,
		"context_truncation_reason":  ctx.ContextTruncationReason,
		"context_approx_bytes":       ctx.ContextApproxUncompressedBytes,
		"top_open_alerts_in_context": len(ctx.Alerts.TopOpen),
	}
	summary["top_alerts_summary"] = summarizeAlertsForDiagnostics(ctx.Alerts.TopOpen, 12)
	return summary
}

func summarizeAlertsForDiagnostics(alerts []AlertSignal, limit int) []map[string]any {
	if limit <= 0 {
		limit = 10
	}
	out := make([]map[string]any, 0, limit)
	for i, a := range alerts {
		if i >= limit {
			break
		}
		item := map[string]any{
			"id":          a.ID,
			"alert_type":  a.AlertType,
			"alert_group": a.AlertGroup,
			"severity":    a.Severity,
			"urgency":     a.Urgency,
			"entity_type": a.EntityType,
			"title":       a.Title,
			"message":     a.Message,
		}
		if a.EntitySKU != nil {
			item["entity_sku"] = *a.EntitySKU
		}
		if a.EntityOfferID != nil {
			item["entity_offer_id"] = *a.EntityOfferID
		}
		if a.EntityID != nil {
			item["entity_id"] = *a.EntityID
		}
		if len(a.Evidence) > 0 {
			item["evidence_payload"] = a.Evidence
		}
		out = append(out, item)
	}
	return out
}

func buildValidationDiagnosticsPayload(ctx *AIRecommendationContext, validation *ValidationResult, outcome runOutcome, savedCount int) map[string]any {
	openAlerts := countOpenAlertsInContext(ctx)
	payload := map[string]any{
		"open_alerts_count":     openAlerts,
		"candidates_count":      0,
		"valid_count":           0,
		"rejected_count":        0,
		"saved_count":           savedCount,
		"total_recommendations": 0,
	}
	if validation != nil {
		payload["candidates_count"] = validation.TotalRecommendations
		payload["valid_count"] = len(validation.ValidRecommendations)
		payload["rejected_count"] = len(validation.RejectedRecommendations)
		payload["total_recommendations"] = validation.TotalRecommendations
		payload["normalized_types_count"] = validation.NormalizedTypesCount
		if len(validation.NormalizedTypes) > 0 {
			payload["normalized_types"] = validation.NormalizedTypes
		}
	}
	if len(outcome.Warnings) > 0 {
		payload["warnings"] = outcome.Warnings
	}
	if outcome.FailRun {
		payload["outcome"] = "failed"
		payload["error_stage"] = outcome.ErrorStage
		payload["error_message"] = outcome.ErrorMessage
		payload["reason"] = outcome.ErrorMessage
	} else {
		payload["outcome"] = "completed"
	}
	return payload
}

func rawAIResponsePreview(raw json.RawMessage, content string, max int) string {
	if max <= 0 {
		max = 2000
	}
	if len(raw) > 0 {
		s := string(raw)
		if len(s) > max {
			return s[:max] + "…"
		}
		return s
	}
	s := strings.TrimSpace(content)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func rejectedReasonsSummary(rejected []RejectedRecommendation, limit int) []map[string]any {
	if limit <= 0 {
		limit = 20
	}
	out := make([]map[string]any, 0, limit)
	for i, r := range rejected {
		if i >= limit {
			break
		}
		item := map[string]any{
			"index":  r.Index,
			"reason": r.Reason,
		}
		if r.RecommendationType != "" {
			item["recommendation_type"] = r.RecommendationType
		}
		out = append(out, item)
	}
	return out
}

func buildStoredRawAIResponse(aiOutput *GenerateRecommendationsOutput) json.RawMessage {
	if aiOutput == nil {
		return json.RawMessage(`{}`)
	}
	wrapper := map[string]any{
		"parsed_content": strings.TrimSpace(aiOutput.Content),
	}
	if len(aiOutput.RawResponse) > 0 {
		var body any
		if err := json.Unmarshal(aiOutput.RawResponse, &body); err == nil {
			wrapper["openai_response"] = body
		} else {
			wrapper["openai_response_raw"] = string(aiOutput.RawResponse)
		}
	}
	b, err := json.Marshal(wrapper)
	if err != nil {
		if len(aiOutput.RawResponse) > 0 {
			return sanitizeRawAIResponse(aiOutput.RawResponse)
		}
		return json.RawMessage(aiOutput.Content)
	}
	return sanitizeRawAIResponse(b)
}
