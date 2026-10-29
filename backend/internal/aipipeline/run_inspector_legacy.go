package aipipeline

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// LegacyActionsInspectorAdapter maps legacy single-phase Actions artifacts to inspector views.
type LegacyActionsInspectorAdapter struct {
	ActionsInput      json.RawMessage `json:"actions_input,omitempty"`
	ActionsRawOutput  json.RawMessage `json:"actions_raw_output,omitempty"`
	ActionsValidation json.RawMessage `json:"actions_validation,omitempty"`
	ActionsOutput     json.RawMessage `json:"actions_output,omitempty"`
	PipelineSummary   json.RawMessage `json:"pipeline_summary,omitempty"`
}

func loadLegacyActionsAdapter(byKind map[string]dbgen.AiPipelineArtifact) LegacyActionsInspectorAdapter {
	var adapter LegacyActionsInspectorAdapter
	if art, ok := byKind[ArtifactKindActionsInput]; ok {
		adapter.ActionsInput = extractArtifactPayloadRaw(art.PayloadJson)
	}
	if art, ok := byKind[ArtifactKindActionsRawOutput]; ok {
		adapter.ActionsRawOutput = extractArtifactPayloadRaw(art.PayloadJson)
	}
	if art, ok := byKind[ArtifactKindActionsValidation]; ok {
		adapter.ActionsValidation = extractArtifactPayloadRaw(art.PayloadJson)
	}
	if art, ok := byKind[ArtifactKindActionsOutput]; ok {
		adapter.ActionsOutput = extractArtifactPayloadRaw(art.PayloadJson)
	}
	if art, ok := byKind[ArtifactKindPipelineSummary]; ok {
		adapter.PipelineSummary = extractArtifactPayloadRaw(art.PayloadJson)
	}
	return adapter
}

func extractArtifactPayloadRaw(raw []byte) json.RawMessage {
	payload := decodeArtifactPayload(raw)
	if payload == nil {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return b
}

func (a LegacyActionsInspectorAdapter) Available() bool {
	return len(a.ActionsOutput) > 0 || len(a.ActionsValidation) > 0 || len(a.ActionsRawOutput) > 0
}

func (a LegacyActionsInspectorAdapter) BuildView() map[string]any {
	if !a.Available() {
		return nil
	}
	view := map[string]any{
		"adapter":       "legacy_actions",
		"compose":       a.buildComposeView(),
		"validation":    a.buildValidationView(),
		"final_actions": a.buildFinalActionsView(),
		"diagnostics":   a.buildDiagnosticsView(),
		"normalization": a.buildNormalizationView(),
		"lineage":       a.buildLegacyLineageView(),
	}
	return view
}

func (a LegacyActionsInspectorAdapter) buildComposeView() map[string]any {
	rawActions := a.extractRawActions()
	rawAvailable := a.rawBodyAvailable()
	view := map[string]any{
		"source":             "legacy",
		"raw_body_available": rawAvailable,
		"content_type":       a.rawContentType(),
	}
	if rawAvailable {
		view["raw_actions"] = rawActions
		view["returned_count"] = len(rawActions)
		view["returned_count_status"] = "known"
	} else {
		view["raw_actions"] = nil
		view["returned_count"] = nil
		view["returned_count_status"] = "unknown"
		view["notice"] = "Raw provider body was not captured by the source run."
	}
	return view
}

func (a LegacyActionsInspectorAdapter) rawBodyAvailable() bool {
	if len(a.ActionsRawOutput) == 0 {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsRawOutput, &doc); err != nil {
		return false
	}
	_, available := ClassifyLegacyRawOutputArtifact(doc)
	return available
}

func (a LegacyActionsInspectorAdapter) rawContentType() string {
	if len(a.ActionsRawOutput) == 0 {
		return ArtifactContentMetadataOnly
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsRawOutput, &doc); err != nil {
		return ArtifactContentMetadataOnly
	}
	contentType, _ := ClassifyLegacyRawOutputArtifact(doc)
	return contentType
}

func (a LegacyActionsInspectorAdapter) buildFinalActionsView() map[string]any {
	out := map[string]any{"source": "legacy", "actions": []any{}}
	if len(a.ActionsOutput) == 0 {
		return out
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsOutput, &doc); err != nil {
		return out
	}
	out["final_status"] = doc["final_status"]
	out["accepted_count"] = doc["accepted_count"]
	out["dropped_count"] = doc["dropped_count"]
	out["actions"] = doc["actions"]
	return out
}

func (a LegacyActionsInspectorAdapter) buildValidationView() map[string]any {
	view := map[string]any{
		"source":                      "legacy",
		"artifact_parse_status":       "ok",
		"validation_execution_status": "completed",
		"blocking_errors":             []any{},
		"warnings":                    []any{},
		"normalization":               []any{},
		"persist_guard":               []any{},
		"drop_reasons":                []any{},
	}
	if len(a.ActionsValidation) == 0 {
		return view
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsValidation, &doc); err != nil {
		return view
	}
	view["status"] = doc["status"]
	view["artifact_validation_status"] = doc["status"]
	view["validation_outcome"] = deriveValidationOutcome(doc)
	if issues, ok := doc["issues"].([]any); ok {
		for _, item := range issues {
			issue, ok := item.(map[string]any)
			if !ok {
				continue
			}
			severity := strings.ToLower(stringValue(issue["severity"]))
			switch severity {
			case "fatal", "error":
				view["blocking_errors"] = append(view["blocking_errors"].([]any), issue)
			default:
				code := stringValue(issue["code"])
				if strings.Contains(code, "persist") {
					view["persist_guard"] = append(view["persist_guard"].([]any), issue)
				} else if strings.Contains(code, "removed") || strings.Contains(code, "duplicate") || strings.Contains(code, "outside") {
					view["normalization"] = append(view["normalization"].([]any), issue)
				} else {
					view["warnings"] = append(view["warnings"].([]any), issue)
				}
			}
		}
	}
	if dropped, ok := doc["dropped_actions"].([]any); ok && len(dropped) > 0 {
		view["drop_reasons"] = dropped
	} else {
		view["drop_reasons"] = projectLegacyDropReasons(doc)
	}
	return view
}

func projectLegacyDropReasons(doc map[string]any) []map[string]any {
	droppedCount := intValue(doc["dropped_count"])
	if droppedCount <= 0 {
		return nil
	}
	for _, item := range issuesAsMaps(doc["issues"]) {
		code := stringValue(item["code"])
		if code != "duplicate_scope_after_ai_order_normalization" {
			continue
		}
		actionIndex := intValue(item["action_index"])
		return []map[string]any{{
			"action_index":                 actionIndex,
			"drop_stage":                   "normalization",
			"drop_code":                    code,
			"drop_reason":                  "all entities were removed during scope deduplication",
			"comparison_details_available": false,
		}}
	}
	return nil
}

func issuesAsMaps(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (a LegacyActionsInspectorAdapter) buildDiagnosticsView() map[string]any {
	diag := map[string]any{"source": "legacy"}
	if len(a.PipelineSummary) > 0 {
		var summary map[string]any
		if err := json.Unmarshal(a.PipelineSummary, &summary); err == nil {
			for k, v := range summary {
				diag[k] = v
			}
		}
	}
	if len(a.ActionsOutput) > 0 {
		var out map[string]any
		if err := json.Unmarshal(a.ActionsOutput, &out); err == nil {
			diag["actions_final_status"] = out["final_status"]
			diag["actions_accepted_count"] = out["accepted_count"]
			diag["actions_dropped_count"] = out["dropped_count"]
		}
	}
	coverage := a.extractCandidateCoverage()
	if len(coverage) > 0 {
		diag["candidate_coverage"] = coverage
	}
	return diag
}

func (a LegacyActionsInspectorAdapter) buildNormalizationView() map[string]any {
	events := a.buildNormalizationEvents()
	return map[string]any{
		"source": "legacy_projection",
		"events": events,
	}
}

func (a LegacyActionsInspectorAdapter) buildLegacyLineageView() map[string]any {
	rawActions := a.extractRawActions()
	candidates := []string{}
	if a.rawBodyAvailable() {
		for _, action := range rawActions {
			if m, ok := action.(map[string]any); ok {
				for _, id := range stringSlice(m["source_candidate_ids"]) {
					candidates = append(candidates, id)
				}
			}
		}
	}
	coverage := a.extractCandidateCoverage()
	coverageStatus := DiagnosticStatusNotReconstructable
	coverageReason := "raw_candidate_references_not_captured"
	if len(coverage) > 0 {
		coverageStatus = DiagnosticStatusAvailable
		coverageReason = "pipeline_summary"
	} else if !a.rawBodyAvailable() {
		coverageReason = "raw Action body was not captured by the source run"
	}
	return map[string]any{
		"source":                    "legacy_projection",
		"raw_referenced_candidates": ternaryCandidates(candidates, a.rawBodyAvailable()),
		"candidate_coverage":        coverage,
		"coverage_status":           coverageStatus,
		"coverage_reason":           coverageReason,
	}
}

func ternaryCandidates(candidates []string, available bool) any {
	if available {
		return candidates
	}
	return nil
}

func deriveValidationOutcome(doc map[string]any) string {
	accepted := intValue(doc["accepted_count"])
	dropped := intValue(doc["dropped_count"])
	switch {
	case accepted > 0 && dropped == 0:
		return "accepted"
	case accepted > 0:
		return "accepted_with_warnings"
	case dropped > 0:
		return "all_actions_dropped"
	default:
		return "failed"
	}
}

func (a LegacyActionsInspectorAdapter) buildNormalizationEvents() []map[string]any {
	var events []map[string]any
	if len(a.ActionsValidation) == 0 {
		return events
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsValidation, &doc); err != nil {
		return events
	}
	issues, _ := doc["issues"].([]any)
	for _, item := range issues {
		issue, ok := item.(map[string]any)
		if !ok {
			continue
		}
		code := stringValue(issue["code"])
		if code == "" {
			continue
		}
		ev := map[string]any{
			"stage":      "legacy_validation",
			"event_code": code,
			"severity":   issue["severity"],
			"details":    issue,
		}
		if idx, ok := issue["action_index"]; ok {
			ev["action_index"] = idx
		}
		events = append(events, ev)
	}
	if dropped, ok := doc["dropped_actions"].([]any); ok {
		for _, d := range dropped {
			dm, ok := d.(map[string]any)
			if !ok {
				continue
			}
			events = append(events, map[string]any{
				"stage": "persist_guard", "event_code": "action_dropped",
				"severity": "warning", "details": dm,
			})
		}
	}
	return events
}

func (a LegacyActionsInspectorAdapter) extractRawActions() []any {
	if len(a.ActionsRawOutput) == 0 {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(a.ActionsRawOutput, &doc); err != nil {
		return nil
	}
	if actions, ok := doc["actions"].([]any); ok {
		return actions
	}
	if parsed, ok := doc["parsed_output"].(map[string]any); ok {
		if actions, ok := parsed["actions"].([]any); ok {
			return actions
		}
	}
	if responseJSON, ok := doc["response_json"].(map[string]any); ok {
		if actions, ok := responseJSON["actions"].([]any); ok {
			return actions
		}
	}
	return nil
}

func (a LegacyActionsInspectorAdapter) extractCandidateCoverage() map[string]any {
	if len(a.PipelineSummary) == 0 {
		return nil
	}
	var summary map[string]any
	if err := json.Unmarshal(a.PipelineSummary, &summary); err != nil {
		return nil
	}
	if cov, ok := summary["actions_candidate_coverage"].(map[string]any); ok {
		return cov
	}
	if diag, ok := summary["diagnostics"].(map[string]any); ok {
		if actions, ok := diag["actions"].(map[string]any); ok {
			if cov, ok := actions["candidate_coverage"].(map[string]any); ok {
				return cov
			}
		}
	}
	return nil
}

func (a LegacyActionsInspectorAdapter) rawReferencedCandidateIDs() []string {
	if !a.rawBodyAvailable() {
		return nil
	}
	var ids []string
	for _, action := range a.extractRawActions() {
		if m, ok := action.(map[string]any); ok {
			ids = append(ids, stringSlice(m["source_candidate_ids"])...)
		}
	}
	return ids
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func resolveActionsSection(byKind map[string]dbgen.AiPipelineArtifact, kind string, legacy LegacyActionsInspectorAdapter) any {
	if art, ok := byKind[kind]; ok {
		return decodeArtifactPayload(art.PayloadJson)
	}
	if legacyView := legacy.BuildView(); legacyView != nil {
		switch kind {
		case ArtifactKindActionsComposeOutput:
			return legacyView["compose"]
		case ArtifactKindActionsNormalization:
			return legacyView["normalization"]
		case ArtifactKindActionsFinalOutput:
			return legacyView["final_actions"]
		case ArtifactKindAIPipelineLineage:
			return legacyView["lineage"]
		}
	}
	return map[string]any{"notice": "Artifact not available for this pipeline version"}
}
