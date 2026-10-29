package aipipeline

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildInspectorAICallViews(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) map[string]any {
	pipelineSummary := decodeArtifactPayloadMap(byKind, ArtifactKindPipelineSummary)
	summary := buildAICallsTopologySummary(byKind, pipelineSummary, legacy)
	return map[string]any{
		"summary": summary,
		"global_context_call": inspectorStageCallDetailForTopology(byKind, pipelineSummary, "global_context", legacy,
			ArtifactKindGlobalContextCallDiagnostics, ArtifactKindGlobalContextRawResponse, ArtifactKindGlobalContextParsedOutput),
		"dashboard_call": inspectorStageCallDetailForTopology(byKind, pipelineSummary, "dashboard", legacy,
			ArtifactKindDashboardCallDiagnostics, ArtifactKindDashboardRawResponse, ArtifactKindDashboardParsedOutput),
		"actions_calls": inspectorActionsCallDetail(byKind, legacy),
	}
}

func buildAICallsTopologySummary(byKind map[string]dbgen.AiPipelineArtifact, summary map[string]any, legacy LegacyActionsInspectorAdapter) []map[string]any {
	sourceRun := intValue(summary["source_pipeline_run_id"])
	aiCalls := mapValue(summary, "ai_calls")
	rows := make([]map[string]any, 0, 4)

	if stageRestartReused(summary, "global_context") {
		rows = append(rows, topologyRow("global_context", "global_context", "reused", "not_executed", sourceRun, DiagnosticStatusNotApplicable, 0, ""))
	} else if row := envelopeTopologyRow(byKind, ArtifactKindGlobalContextCallDiagnostics, "global_context", "global_context", "generated", "executed"); row != nil {
		rows = append(rows, row)
	} else {
		rows = append(rows, topologyRow("global_context", "global_context", "generated", "not_executed", 0, DiagnosticStatusNotCaptured, aiCallCount(aiCalls, "global_context"), ""))
	}

	if stageRestartCloned(summary, "dashboard") {
		rows = append(rows, topologyRow("dashboard", "dashboard_synthesis", "cloned", "not_executed", sourceRun, DiagnosticStatusNotApplicable, 0, ""))
	} else if row := envelopeTopologyRow(byKind, ArtifactKindDashboardCallDiagnostics, "dashboard", "dashboard_synthesis", "generated", "executed"); row != nil {
		rows = append(rows, row)
	} else {
		rows = append(rows, topologyRow("dashboard", "dashboard_synthesis", "generated", "not_executed", 0, DiagnosticStatusNotCaptured, aiCallCount(aiCalls, "dashboard"), ""))
	}

	if stageRestartReused(summary, "shared_chunk_analysis") || stageRestartReused(summary, "chunk_set") {
		rows = append(rows, topologyRow("shared_chunk_analysis", "chunk_analysis", "reused", "not_executed", sourceRun, DiagnosticStatusNotApplicable, 0, ""))
	} else if _, ok := byKind[ArtifactKindSharedChunkAnalysis]; ok {
		rows = append(rows, topologyRow("shared_chunk_analysis", "chunk_analysis", "generated", "executed", 0, DiagnosticStatusPartial, aiCallCount(aiCalls, "shared_chunk_analysis"), ""))
	}

	rows = append(rows, buildActionsTopologyRow(byKind, summary, legacy))
	return rows
}

func buildActionsTopologyRow(byKind map[string]dbgen.AiPipelineArtifact, summary map[string]any, legacy LegacyActionsInspectorAdapter) map[string]any {
	aiCalls := mapValue(summary, "ai_calls")
	callCount := aiCallCount(aiCalls, "actions")
	if callCount == 0 && legacy.Available() {
		callCount = 1
	}
	outcome := actionsStageOutcome(byKind, legacy)
	row := topologyRow("actions_compose", "actions_synthesis", "generated", "executed", 0, DiagnosticStatusPartial, callCount, outcome)

	if art, ok := byKind[ArtifactKindActionsComposeCallDiagnostics]; ok {
		env := decodeCallEnvelope(art.PayloadJson)
		mergeEnvelopeIntoTopologyRow(row, env, art.ID)
		row["diagnostics"] = DiagnosticStatusAvailable
		if rawArt, ok := byKind[ArtifactKindActionsComposeRawResponse]; ok {
			payload := decodeArtifactPayloadMapSingle(rawArt.PayloadJson)
			row["raw_body_status"] = ClassifyArtifactContentCompleteness(ArtifactKindActionsComposeRawResponse, payload)
		}
		return row
	}

	if art, ok := byKind[ArtifactKindActionsRawOutput]; ok {
		payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
		row["diagnostics"] = DiagnosticStatusPartial
		row["raw_body_status"] = ClassifyArtifactContentCompleteness(ArtifactKindActionsRawOutput, payload)
		row["model"] = payload["model"]
		row["request_id"] = payload["request_id"]
		row["input_tokens"] = payload["input_tokens"]
		row["output_tokens"] = payload["output_tokens"]
		row["provider_status"] = payload["status"]
		row["parse_status"] = ParseStatusNotCaptured
		row["artifact_id"] = art.ID
		return row
	}

	row["diagnostics"] = DiagnosticStatusNotCaptured
	return row
}

func topologyRow(stage, component, execution, aiCallInRun string, sourceRun int, diagnostics string, aiCalls int, outcome string) map[string]any {
	row := map[string]any{
		"stage":                  stage,
		"component":              component,
		"execution":              execution,
		"ai_call_in_current_run": aiCallInRun,
		"diagnostics":            diagnostics,
		"ai_calls":               aiCalls,
		"stage_outcome":          outcome,
	}
	if sourceRun > 0 {
		row["source_pipeline_run_id"] = sourceRun
	}
	return row
}

func envelopeTopologyRow(byKind map[string]dbgen.AiPipelineArtifact, kind, stage, component, execution, aiCallInRun string) map[string]any {
	art, ok := byKind[kind]
	if !ok {
		return nil
	}
	env := decodeCallEnvelope(art.PayloadJson)
	row := topologyRow(stage, component, execution, aiCallInRun, 0, DiagnosticStatusAvailable, 1, "")
	mergeEnvelopeIntoTopologyRow(row, env, art.ID)
	return row
}

func mergeEnvelopeIntoTopologyRow(row map[string]any, env AICallDiagnosticEnvelope, artID int64) {
	row["model"] = env.Model
	row["request_id"] = env.RequestID
	row["input_tokens"] = env.InputTokens
	row["output_tokens"] = env.OutputTokens
	row["requested_max_output_tokens"] = env.RequestedMaxOutputTokens
	row["truncated"] = env.OutputTruncated
	row["truncation_class"] = env.OutputTruncationClass
	row["parse_status"] = env.ParseStatus
	row["provider_status"] = env.ProviderStatus
	row["artifact_id"] = artID
}

func inspectorStageCallDetailForTopology(
	byKind map[string]dbgen.AiPipelineArtifact,
	summary map[string]any,
	stage string,
	legacy LegacyActionsInspectorAdapter,
	diagKind, rawKind, parsedKind string,
) map[string]any {
	if stage == "global_context" && stageRestartReused(summary, "global_context") {
		return map[string]any{
			"execution":              "reused",
			"ai_call_in_current_run": "not_executed",
			"source_pipeline_run_id": intValue(summary["source_pipeline_run_id"]),
			"call_diagnostics":       map[string]any{"status": DiagnosticStatusNotApplicable, "reason": "no AI call in current run"},
			"raw_response":           map[string]any{"status": DiagnosticStatusNotApplicable},
			"parsed_output":          map[string]any{"status": DiagnosticStatusNotApplicable},
		}
	}
	if stage == "dashboard" && stageRestartCloned(summary, "dashboard") {
		return map[string]any{
			"execution":              "cloned",
			"ai_call_in_current_run": "not_executed",
			"source_pipeline_run_id": intValue(summary["source_pipeline_run_id"]),
			"call_diagnostics":       map[string]any{"status": DiagnosticStatusNotApplicable, "reason": "dashboard cloned from source run"},
			"raw_response":           map[string]any{"status": DiagnosticStatusNotApplicable},
			"parsed_output":          map[string]any{"status": DiagnosticStatusNotApplicable},
		}
	}
	return inspectorStageCallDetail(byKind, diagKind, rawKind, parsedKind)
}

func inspectorActionsCallDetail(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) map[string]any {
	if _, ok := byKind[ArtifactKindActionsComposeCallDiagnostics]; ok {
		return inspectorStageCallDetail(byKind, ArtifactKindActionsComposeCallDiagnostics, ArtifactKindActionsComposeRawResponse, ArtifactKindActionsComposeParsedOutput)
	}
	detail := map[string]any{
		"execution":              "generated",
		"ai_call_in_current_run": "executed",
		"diagnostics":            DiagnosticStatusPartial,
	}
	if art, ok := byKind[ArtifactKindActionsRawOutput]; ok {
		payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
		detail["call_diagnostics"] = map[string]any{"status": DiagnosticStatusPartial, "artifact_id": art.ID}
		detail["raw_response"] = map[string]any{
			"status":      ClassifyArtifactContentCompleteness(ArtifactKindActionsRawOutput, payload),
			"artifact_id": art.ID,
		}
		detail["parsed_output"] = map[string]any{"status": DiagnosticStatusNotCaptured, "reason": "legacy run without parsed-output artifact"}
	} else if legacy.Available() {
		detail["call_diagnostics"] = map[string]any{"status": DiagnosticStatusPartial}
		detail["raw_response"] = map[string]any{"status": DiagnosticStatusNotCaptured}
		detail["parsed_output"] = map[string]any{"status": DiagnosticStatusNotCaptured}
	}
	return detail
}

func stageRestartReused(summary map[string]any, stage string) bool {
	for _, s := range stringSlice(summary["reused_stages"]) {
		if s == stage {
			return true
		}
	}
	restart := mapValue(summary, "restart")
	if restart == nil {
		return false
	}
	switch stage {
	case "global_context":
		return boolValue(restart["global_context_reused"])
	case "shared_chunk_analysis", "chunk_set":
		return boolValue(restart["shared_chunk_analysis_reused"]) || boolValue(restart["data_pack_reused"])
	case "data_pack":
		return boolValue(restart["data_pack_reused"])
	}
	return false
}

func stageRestartCloned(summary map[string]any, stage string) bool {
	for _, s := range stringSlice(summary["cloned_stages"]) {
		if s == stage {
			return true
		}
	}
	restart := mapValue(summary, "restart")
	return restart != nil && stage == "dashboard" && boolValue(restart["dashboard_reused"])
}

func aiCallCount(aiCalls map[string]any, key string) int {
	if aiCalls == nil {
		return 0
	}
	return intValue(aiCalls[key])
}
