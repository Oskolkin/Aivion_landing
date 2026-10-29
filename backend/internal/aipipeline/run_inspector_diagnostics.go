package aipipeline

import (
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildInspectorDiagnostics(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) map[string]any {
	pipelineSummary := decodeArtifactPayloadMap(byKind, ArtifactKindPipelineSummary)
	return map[string]any{
		"completeness":  buildDiagnosticCompletenessMatrix(byKind, legacy),
		"stages_detail": buildRestartAwareStages(byKind, legacy, pipelineSummary),
		"failure_focus": buildFailureFocus(byKind, legacy, pipelineSummary),
		"statuses":      buildSeparatedStatuses(byKind, legacy, pipelineSummary),
		"timeline":      buildStageTimeline(byKind, legacy, pipelineSummary),
		"ai_calls":      buildInspectorAICallViews(byKind, legacy),
	}
}

func buildDiagnosticCompletenessMatrix(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) []DiagnosticCompletenessEntry {
	var entries []DiagnosticCompletenessEntry
	add := func(dimension, status, reason string, artID int64) {
		entries = append(entries, DiagnosticCompletenessEntry{
			Dimension: dimension, Status: status, Reason: reason, ArtifactID: artID,
		})
	}

	if art, ok := byKind[ArtifactKindActionsInput]; ok {
		add("Actions input", DiagnosticStatusAvailable, "artifact present", art.ID)
	} else if art, ok := byKind[ArtifactKindActionsComposeInput]; ok {
		add("Actions input", DiagnosticStatusAvailable, "compose input artifact", art.ID)
	} else {
		add("Actions input", DiagnosticStatusNotCaptured, "no actions input artifact", 0)
	}

	rawKind := ArtifactKindActionsComposeRawResponse
	if _, ok := byKind[rawKind]; !ok {
		rawKind = ArtifactKindActionsRawOutput
	}
	if art, ok := byKind[rawKind]; ok {
		payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
		contentType, rawAvailable := ClassifyLegacyRawOutputArtifact(payload)
		if rawKind == ArtifactKindActionsComposeRawResponse {
			if ct, ok := payload["content_type"].(string); ok && ct == ArtifactContentFullBody {
				rawAvailable = true
				contentType = ArtifactContentFullBody
			}
		}
		if rawAvailable {
			add("Raw provider body", DiagnosticStatusAvailable, "full body captured", art.ID)
		} else {
			add("Provider metadata", DiagnosticStatusAvailable, "legacy metadata artifact", art.ID)
			add("Raw provider body", DiagnosticStatusNotCaptured,
				"legacy artifact content_type="+contentType, art.ID)
		}
	} else {
		add("Provider metadata", DiagnosticStatusNotCaptured, "no raw output artifact", 0)
		add("Raw provider body", DiagnosticStatusNotCaptured, "no raw output artifact", 0)
	}

	if art, ok := byKind[ArtifactKindActionsComposeParsedOutput]; ok {
		add("Parsed AI draft", DiagnosticStatusAvailable, "parsed output artifact", art.ID)
	} else {
		add("Parsed AI draft", DiagnosticStatusNotCaptured, "source run predates parsed-output artifact", 0)
	}

	if art, ok := byKind[ArtifactKindActionsNormalization]; ok {
		add("Normalization events", DiagnosticStatusAvailable, "structured events artifact", art.ID)
	} else if legacy.Available() {
		add("Normalization events", DiagnosticStatusPartial, "reconstructed from validation issues", 0)
	} else {
		add("Normalization events", DiagnosticStatusNotCaptured, "no normalization artifact", 0)
	}

	coverageStatus, coverageReason := candidateCoverageAvailability(byKind, legacy)
	add("Candidate references", coverageStatus, coverageReason, 0)
	add("Candidate coverage", coverageStatus, coverageReason, 0)

	if art, ok := byKind[ArtifactKindActionsOutput]; ok {
		add("Final Actions", DiagnosticStatusAvailable, "artifact present", art.ID)
	} else if art, ok := byKind[ArtifactKindActionsFinalOutput]; ok {
		add("Final Actions", DiagnosticStatusAvailable, "final output artifact", art.ID)
	} else {
		add("Final Actions", DiagnosticStatusNotCaptured, "no actions output", 0)
	}

	if art, ok := byKind[ArtifactKindPublicationGuardDecision]; ok {
		add("Publication decision", DiagnosticStatusAvailable, "publication guard artifact", art.ID)
	} else {
		add("Publication decision", DiagnosticStatusPartial, "inferred from run state", 0)
	}

	if legacy.Available() && !legacy.rawBodyAvailable() {
		add("Dedup comparison details", DiagnosticStatusNotCaptured, "legacy warning has no compared index", 0)
	} else if _, ok := byKind[ArtifactKindActionsNormalization]; ok {
		add("Dedup comparison details", DiagnosticStatusAvailable, "structured dedup events", 0)
	}

	return entries
}

func candidateCoverageAvailability(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) (status, reason string) {
	if art, ok := byKind[ArtifactKindAIPipelineLineage]; ok {
		payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
		if cov, ok := payload["candidate_coverage"].(map[string]any); ok && len(cov) > 0 {
			return DiagnosticStatusAvailable, "lineage artifact"
		}
	}
	if cov := legacy.extractCandidateCoverage(); len(cov) > 0 {
		return DiagnosticStatusAvailable, "pipeline_summary candidate coverage"
	}
	if legacy.rawBodyAvailable() {
		return DiagnosticStatusPartial, "raw body present but coverage not persisted"
	}
	return DiagnosticStatusNotReconstructable, "raw_candidate_references_not_captured"
}

func buildRestartAwareStages(
	byKind map[string]dbgen.AiPipelineArtifact,
	legacy LegacyActionsInspectorAdapter,
	summary map[string]any,
) []map[string]any {
	restart := mapValue(summary, "restart")
	sourceRun := intValue(summary["source_pipeline_run_id"])
	aiCalls := mapValue(summary, "ai_calls")
	getAICalls := func(key string) int {
		if aiCalls == nil {
			return 0
		}
		return intValue(aiCalls[key])
	}

	stageRow := func(name, exec, source string, sourceRunID, aiCallCount int, outcome string) map[string]any {
		row := map[string]any{
			"stage": name, "execution": exec, "source": source,
			"ai_calls": aiCallCount, "outcome": outcome,
		}
		if sourceRunID > 0 {
			row["source_run"] = sourceRunID
		}
		return row
	}

	if restart == nil && sourceRun == 0 {
		return []map[string]any{
			stageRow("Data Pack", "generated", "current run", 0, 0, stageOutcome(byKind, ArtifactKindDataPack)),
			stageRow("Shared Chunk Analysis", "generated", "current run", 0, getAICalls("shared_chunk_analysis"), stageOutcome(byKind, ArtifactKindSharedChunkAnalysis)),
			stageRow("Global Context", "generated", "current run", 0, getAICalls("global_context"), stageOutcome(byKind, ArtifactKindGlobalContextFinal)),
			stageRow("Dashboard", "generated", "current run", 0, getAICalls("dashboard"), dashboardStageOutcome(byKind, summary)),
			stageRow("Actions", "generated", "current run", 0, getAICalls("actions"), actionsStageOutcome(byKind, legacy)),
			stageRow("Publication", "executed", "current run", 0, 0, publicationOutcome(summary)),
		}
	}

	reused := func(stage string) bool {
		for _, s := range stringSlice(summary["reused_stages"]) {
			if s == stage {
				return true
			}
		}
		return false
	}
	cloned := func(stage string) bool {
		for _, s := range stringSlice(summary["cloned_stages"]) {
			if s == stage {
				return true
			}
		}
		return false
	}

	targetRun := intValue(summary["target_pipeline_run_id"])
	if targetRun == 0 {
		targetRun = intValue(restart["target_pipeline_run_id"])
	}

	dataPackRun, chunkRun, gcRun, dashRun := 0, 0, 0, 0
	dataPackExec, dataPackSource := "generated", "current run"
	if reused("data_pack") {
		dataPackExec, dataPackSource, dataPackRun = "reused", "stage restart source", sourceRun
	}
	chunkExec, chunkSource := "generated", "current run"
	if reused("chunk_set") || reused("shared_chunk_analysis") {
		chunkExec, chunkSource, chunkRun = "reused", "stage restart source", sourceRun
	}
	gcExec, gcSource := "generated", "current run"
	if reused("global_context") {
		gcExec, gcSource, gcRun = "reused", "stage restart source", sourceRun
	}
	dashExec, dashSource := "generated", "current run"
	if cloned("dashboard") || boolValue(restart["dashboard_reused"]) {
		dashExec, dashSource, dashRun = "cloned", "source dashboard", sourceRun
	}

	return []map[string]any{
		stageRow("Data Pack", dataPackExec, dataPackSource, dataPackRun, 0, stageOutcome(byKind, ArtifactKindDataPack)),
		stageRow("Chunk Set", chunkExec, chunkSource, chunkRun, 0, stageOutcome(byKind, ArtifactKindChunkSet)),
		stageRow("Shared Chunk Analysis", chunkExec, chunkSource, chunkRun, 0, stageOutcome(byKind, ArtifactKindSharedChunkAnalysis)),
		stageRow("Global Context", gcExec, gcSource, gcRun, 0, stageOutcome(byKind, ArtifactKindGlobalContextFinal)),
		stageRow("Dashboard", dashExec, dashSource, dashRun, 0, dashboardStageOutcome(byKind, summary)),
		stageRow("Actions", "generated", "current run", targetRun, getAICalls("actions"), actionsStageOutcome(byKind, legacy)),
		stageRow("Publication", "executed", "current run", targetRun, 0, publicationOutcome(summary)),
	}
}

func stageOutcome(byKind map[string]dbgen.AiPipelineArtifact, kind string) string {
	if _, ok := byKind[kind]; ok {
		if kind == ArtifactKindSharedChunkAnalysis {
			return "completed"
		}
		return "available"
	}
	return "unavailable"
}

func dashboardStageOutcome(byKind map[string]dbgen.AiPipelineArtifact, summary map[string]any) string {
	restart := mapValue(summary, "restart")
	if restart != nil && (boolValue(restart["dashboard_reused"]) || stringValue(restart["dashboard_source"]) != "") {
		return "completed"
	}
	if _, ok := byKind[ArtifactKindDashboardOutput]; ok {
		return "completed"
	}
	return "unavailable"
}

func actionsStageOutcome(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) string {
	for _, kind := range []string{ArtifactKindActionsOutput, ArtifactKindActionsFinalOutput} {
		if art, ok := byKind[kind]; ok {
			payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
			if stringValue(payload["final_status"]) == "failed" {
				return "failed"
			}
		}
	}
	if final := legacy.buildFinalActionsView(); stringValue(final["final_status"]) == "failed" {
		return "failed"
	}
	if _, ok := byKind[ArtifactKindActionsComposeOutput]; ok {
		return "completed"
	}
	if legacy.Available() {
		return "failed"
	}
	return "unavailable"
}

func publicationOutcome(summary map[string]any) string {
	if stringValue(summary["status"]) == PipelineStatusCompleted {
		return "published"
	}
	return "not published"
}

func buildSeparatedStatuses(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter, summary map[string]any) map[string]any {
	rawKind := ArtifactKindActionsRawOutput
	if _, ok := byKind[ArtifactKindActionsComposeRawResponse]; ok {
		rawKind = ArtifactKindActionsComposeRawResponse
	}
	rawPayload := decodeArtifactPayloadMap(byKind, rawKind)
	contentType := ArtifactContentMetadataOnly
	rawAvailable := false
	if rawPayload != nil {
		contentType = ClassifyArtifactContentCompleteness(rawKind, rawPayload)
		_, rawAvailable = ClassifyLegacyRawOutputArtifact(rawPayload)
	}

	validationOutcome := "not_run"
	if art, ok := byKind[ArtifactKindActionsValidation]; ok {
		val := decodeArtifactPayloadMapSingle(art.PayloadJson)
		accepted := intValue(val["accepted_count"])
		dropped := intValue(val["dropped_count"])
		switch {
		case accepted > 0 && dropped == 0:
			validationOutcome = "accepted"
		case accepted > 0:
			validationOutcome = "accepted_with_warnings"
		case dropped > 0:
			validationOutcome = "all_actions_dropped"
		default:
			validationOutcome = "failed"
		}
	} else if legacy.Available() {
		final := legacy.buildFinalActionsView()
		if intValue(final["accepted_count"]) == 0 && intValue(final["dropped_count"]) > 0 {
			validationOutcome = "all_actions_dropped"
		}
	}

	truncationNote := ""
	if rawPayload != nil {
		outTokens := intValue(rawPayload["output_tokens"])
		reqTokens := intValue(rawPayload["requested_max_output_tokens"])
		if reqTokens == 0 {
			reqTokens = 8192
		}
		if outTokens == reqTokens && !rawAvailable {
			truncationNote = "Output tokens equal 8192. Raw body and provider incomplete details were not captured. Truncation is suspected but cannot be confirmed from persisted artifacts."
		}
	}

	hasValidation := legacy.Available()
	if art, ok := byKind[ArtifactKindActionsValidation]; ok && art.ID > 0 {
		hasValidation = true
	}

	return map[string]any{
		"artifact_json_parse_status":    "ok",
		"artifact_content_completeness": contentType,
		"raw_body_available":            rawAvailable,
		"provider_call_status":          stringValue(rawPayload["provider_status"]),
		"ai_output_parse_status":        legacyParseStatus(rawAvailable),
		"validation_execution_status":   ternaryBool(hasValidation, "completed", "not_run"),
		"validation_outcome":            validationOutcome,
		"actions_stage_status":          actionsStageOutcome(byKind, legacy),
		"pipeline_status":               stringValue(summary["status"]),
		"truncation_note":               truncationNote,
	}
}

func legacyParseStatus(rawAvailable bool) string {
	if rawAvailable {
		return ParseStatusUnknown
	}
	return ParseStatusNotCaptured
}

func buildFailureFocus(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter, summary map[string]any) map[string]any {
	status := stringValue(summary["status"])
	if status != PipelineStatusCompletedPartial && status != PipelineStatusFailed && status != PipelineStatusActionsFailed {
		return map[string]any{"applicable": false}
	}
	rawAvailable := legacy.rawBodyAvailable()
	if _, ok := byKind[ArtifactKindActionsComposeRawResponse]; ok {
		rawAvailable = true
	}
	final := legacy.buildFinalActionsView()
	validation := legacy.buildValidationView()
	dropped := intValue(final["dropped_count"])
	accepted := intValue(final["accepted_count"])
	if dropped == 0 {
		if art, ok := byKind[ArtifactKindActionsValidation]; ok {
			val := decodeArtifactPayloadMapSingle(art.PayloadJson)
			dropped = intValue(val["dropped_count"])
			accepted = intValue(val["accepted_count"])
		}
	}
	primary := "Legacy validation processed and dropped one Action."
	if dropped != 1 {
		primary = "Actions validation dropped all returned Actions."
	}
	if !rawAvailable {
		primary += " Raw provider Action count cannot be confirmed because the provider body was not captured."
	}
	focus := map[string]any{
		"applicable":                 true,
		"primary_failure":            primary,
		"provider_returned_count":    legacyReturnedCount(legacy),
		"provider_returned_status":   ternaryReturnedCountStatus(legacy),
		"parsed_count":               nil,
		"validation_processed_count": validationProcessedCount(validation, dropped, accepted),
		"dropped_count":              dropped,
		"persisted_count":            accepted,
		"cannot_confirm":             buildCannotConfirmList(rawAvailable),
		"recommended_artifact":       "actions_validation",
	}
	if art, ok := byKind[ArtifactKindActionsValidation]; ok {
		focus["recommended_artifact_id"] = art.ID
	}
	return focus
}

func validationProcessedCount(validation map[string]any, dropped, accepted int) int {
	if dropped+accepted > 0 {
		return dropped + accepted
	}
	if validation != nil && len(validation) > 0 {
		return 1
	}
	return 0
}

func ternaryReturnedCountStatus(legacy LegacyActionsInspectorAdapter) string {
	compose := legacy.buildComposeView()
	if status, _ := compose["returned_count_status"].(string); status != "" {
		return status
	}
	return "unknown"
}

func buildCannotConfirmList(rawAvailable bool) []string {
	if rawAvailable {
		return []string{"exact dedup comparison pair"}
	}
	return []string{"raw AI body", "candidate references", "provider truncation reason", "exact dedup comparison pair", "provider returned count"}
}

func buildStageTimeline(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter, summary map[string]any) []map[string]any {
	events := []map[string]any{{"event": "created", "stage": "pipeline"}}
	restart := mapValue(summary, "restart")
	if restart != nil {
		if boolValue(restart["data_pack_reused"]) {
			events = append(events, map[string]any{"event": "Data Pack reused", "stage": "data_pack"})
		}
		if boolValue(restart["shared_chunk_analysis_reused"]) {
			events = append(events, map[string]any{"event": "Chunks reused", "stage": "shared_chunk_analysis"})
		}
		if boolValue(restart["global_context_reused"]) {
			events = append(events, map[string]any{"event": "GC reused", "stage": "global_context"})
		}
		if boolValue(restart["dashboard_reused"]) {
			events = append(events, map[string]any{"event": "Dashboard cloned", "stage": "dashboard"})
		}
	}
	events = append(events,
		map[string]any{"event": "Actions call started", "stage": "actions"},
		map[string]any{"event": "Actions call finished", "stage": "actions"},
	)
	if legacy.Available() || byKind[ArtifactKindActionsValidation].ID > 0 {
		events = append(events, map[string]any{"event": "normalization/validation executed", "stage": "actions"})
	}
	if actionsStageOutcome(byKind, legacy) == "failed" {
		events = append(events, map[string]any{"event": "Action dropped", "stage": "actions"})
	}
	if publicationOutcome(summary) == "not published" {
		events = append(events, map[string]any{"event": "publication skipped", "stage": "publication"})
	}
	events = append(events, map[string]any{"event": "run finished", "stage": "pipeline"})
	return events
}

func decodeArtifactPayloadMap(byKind map[string]dbgen.AiPipelineArtifact, kind string) map[string]any {
	if art, ok := byKind[kind]; ok {
		return decodeArtifactPayloadMapSingle(art.PayloadJson)
	}
	return nil
}

func decodeArtifactPayloadMapSingle(raw []byte) map[string]any {
	payload := decodeArtifactPayload(raw)
	if m, ok := payload.(map[string]any); ok {
		return m
	}
	return nil
}

func mapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

func boolValue(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func ternaryBool(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func inspectorStageCallDetail(byKind map[string]dbgen.AiPipelineArtifact, diagKind, rawKind, parsedKind string) map[string]any {
	detail := map[string]any{
		"call_diagnostics": artifactAvailability(byKind, diagKind),
		"raw_response":     artifactAvailability(byKind, rawKind),
		"parsed_output":    artifactAvailability(byKind, parsedKind),
	}
	if art, ok := byKind[diagKind]; ok {
		env := decodeCallEnvelope(art.PayloadJson)
		detail["failure_focus"] = map[string]any{
			"provider_status":             env.ProviderStatus,
			"requested_max_output_tokens": env.RequestedMaxOutputTokens,
			"output_tokens":               env.OutputTokens,
			"truncation_class":            env.OutputTruncationClass,
			"parse_status":                env.ParseStatus,
			"parse_error":                 env.ParseError,
			"parse_error_offset":          env.ParseErrorOffset,
			"raw_body_bytes":              env.RawBodyBytes,
			"raw_body_head":               env.RawBodyHead,
			"raw_body_tail":               env.RawBodyTail,
		}
	}
	return detail
}

func artifactAvailability(byKind map[string]dbgen.AiPipelineArtifact, kind string) map[string]any {
	if art, ok := byKind[kind]; ok {
		return map[string]any{"status": DiagnosticStatusAvailable, "artifact_id": art.ID}
	}
	return map[string]any{"status": DiagnosticStatusNotCaptured}
}

func decodeCallEnvelope(raw []byte) AICallDiagnosticEnvelope {
	payload := decodeArtifactPayload(raw)
	if doc, ok := payload.(map[string]any); ok {
		if envRaw, ok := doc["call"].(map[string]any); ok {
			b, _ := json.Marshal(envRaw)
			var env AICallDiagnosticEnvelope
			_ = json.Unmarshal(b, &env)
			return env
		}
		b, _ := json.Marshal(doc)
		var env AICallDiagnosticEnvelope
		_ = json.Unmarshal(b, &env)
		return env
	}
	return AICallDiagnosticEnvelope{}
}
