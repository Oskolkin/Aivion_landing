package aipipeline

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// ReadinessDiagnosticArtifactKinds lists required artifact kinds for a full diagnostic run.
func ReadinessDiagnosticArtifactKinds() []string {
	return []string{
		ArtifactKindMetricSemanticsSnapshot,
		ArtifactKindGlobalContextValidation,
		ArtifactKindPublicationGuardDecision,
		ArtifactKindSharedChunkRefMapping,
		ArtifactKindFinalOutputSemanticValidation,
		ArtifactKindActionsExecutionDiagnostics,
	}
}

// ReadinessVerifyPublicationGuardWiring checks publication guard blocks a known semantic defect.
func ReadinessVerifyPublicationGuardWiring() bool {
	pack := &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"sku.1.stock.idc_days": {FactID: "sku.1.stock.idc_days", Metric: "idc_days"},
		},
	}
	decision := RunPublicationGuard(PublicationGuardInput{
		RequestedActions: true,
		Pack:             pack,
		Actions: []actions.ActionItem{{
			Reason:          "Срок хранения составляет 142 дня",
			EvidenceFactIDs: []string{"sku.1.stock.idc_days"},
		}},
	})
	return !decision.PublicationAllowed
}

// ReadinessVerifyInspectorMonitoringSections checks monitoring view builders handle missing artifacts.
func ReadinessVerifyInspectorMonitoringSections() bool {
	views := buildInspectorMonitoringViews(map[string]dbgen.AiPipelineArtifact{})
	if views == nil {
		return false
	}
	for _, key := range []string{"metric_semantics", "chunk_ref_mappings", "global_context_validation", "final_output_validation", "actions_execution", "publication_guard"} {
		section, ok := views[key].(map[string]any)
		if !ok || section["status"] == "" {
			return false
		}
	}
	return true
}

// ReadinessVerifyFinalSemanticValidators checks dashboard and actions final validators.
func ReadinessVerifyFinalSemanticValidators() bool {
	raw, _ := json.Marshal(map[string]any{"executive_summary": "Выручка снизилась на %"})
	if ValidateFinalDashboardOutput(nil, raw).BlockingIssueCount == 0 {
		return false
	}
	pack := &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"sku.1.stock.idc_days": {FactID: "sku.1.stock.idc_days", Metric: "idc_days"},
		},
	}
	if ValidateFinalActionsOutput(pack, []actions.ActionItem{{
		Reason: "Срок хранения составляет 142 дня", EvidenceFactIDs: []string{"sku.1.stock.idc_days"},
	}}).BlockingIssueCount == 0 {
		return false
	}
	return true
}

// ReadinessVerifyActionsTimeoutTopology confirms compose/detail timeouts are independent per-call budgets.
func ReadinessVerifyActionsTimeoutTopology() bool {
	compose := actions.ActionsComposeTimeoutSeconds()
	detail := actions.ActionsObjectDetailTimeoutSeconds()
	if compose <= 0 || detail <= 0 {
		return false
	}
	return actions.ActionsComposeTimeoutSeconds() == compose && actions.ActionsObjectDetailTimeoutSeconds() == detail
}

// ReadinessVerifyDebugArtifactBuilders ensures diagnostic artifact builders produce schema versions.
func ReadinessVerifyDebugArtifactBuilders() bool {
	snapshot := buildMetricSemanticsSnapshotArtifact([]string{"idc_days"})
	if snapshot["schema_version"] == "" {
		return false
	}
	mapping := buildSharedChunkRefMappingArtifact(nil, nil)
	if mapping["schema_version"] == "" {
		return false
	}
	gc := buildGlobalContextValidationArtifact("valid", "valid_with_warnings", "high", "medium", GlobalContextValidationResult{})
	if gc["schema_version"] == "" {
		return false
	}
	return true
}

// ReadinessVerifyAICallDiagnosticBuilders ensures AI call diagnostic artifact builders work.
func ReadinessVerifyAICallDiagnosticBuilders() bool {
	env := BuildAICallDiagnosticEnvelopeFromRaw(
		"actions_compose", "actions_synthesis", "fake",
		&actions.AIRawResponse{Model: "fake", RawText: `{"actions":[]}`, OutputTokens: 10},
		16384, 600, time.Time{}, time.Time{}, ParseStatusParsed, "",
	)
	raw := BuildAICallRawResponseArtifact(env, `{"actions":[]}`)
	if raw["schema_version"] != AICallRawResponseSchemaVersion {
		return false
	}
	parsed := BuildActionsComposeParsedOutputArtifact(&actions.ActionsAIFirstComposeOutput{
		Actions: []actions.AIFirstComposeAction{{ActionKey: "a"}},
	})
	if parsed == nil || parsed["action_count"] != 1 {
		return false
	}
	lifecycle := BuildActionsLifecycleArtifact(actions.ActionsLifecycleSummary{ParsedCount: 1}, nil)
	if lifecycle["schema_version"] != ActionsLifecycleDiagnosticsSchemaVersion {
		return false
	}
	return ClassifyOutputTruncation(AICallDiagnosticEnvelope{
		ProviderStatus: "incomplete", IncompleteReason: "max_output_tokens",
		RawResponseCaptured: true,
	}) == TruncationConfirmed
}

// ReadinessVerifyInspectorEntityFlattening checks nested entity registry flattening.
func ReadinessVerifyInspectorEntityFlattening() bool {
	registry := map[string]any{
		"skus": map[string]any{
			"sku:801000100001": map[string]any{"entity_id": "sku:801000100001", "entity_type": "sku"},
		},
	}
	entities := flattenEntityRegistry(registry)
	return len(entities) == 1 && entities[0].EntityID == "sku:801000100001"
}

// ReadinessVerifyInspectorSearchWiring checks inspector HTML template includes search handlers.
func ReadinessVerifyInspectorSearchWiring() bool {
	return strings.Contains(inspectorHTMLTemplate, "addEventListener('input'") &&
		strings.Contains(inspectorHTMLTemplate, "Clear filters") &&
		strings.Contains(inspectorHTMLTemplate, `id="actions_input_candidates"`) &&
		strings.Contains(inspectorHTMLTemplate, "hashchange") &&
		strings.Contains(inspectorHTMLTemplate, "id:'ai_calls'")
}

// ReadinessVerifyGlobalContextInvalidJSONWiring verifies raw capture before parse on invalid JSON.
func ReadinessVerifyGlobalContextInvalidJSONWiring() bool {
	raw := &GlobalContextRawResponse{
		Content:     `{"overall_story":"x"`,
		ParseStatus: ParseStatusInvalidJSON,
		ParseError:  "invalid_json: eof",
	}
	env := BuildAICallDiagnosticEnvelopeFromGlobalContextRaw("fake", raw, 600)
	artifact := BuildAICallRawResponseArtifact(env, raw.Content)
	body, _ := artifact["body"].(string)
	return env.ParseStatus == ParseStatusInvalidJSON && env.RawResponseCaptured && body != "" && env.RawBodyHead != ""
}

// ReadinessVerifyGlobalContextTruncationWiring verifies confirmed truncation classification.
func ReadinessVerifyGlobalContextTruncationWiring() bool {
	env := BuildAICallDiagnosticEnvelopeFromGlobalContextRaw("fake", &GlobalContextRawResponse{
		Content:                  `{}`,
		ProviderStatus:           "incomplete",
		IncompleteReason:         "max_output_tokens",
		OutputTruncated:          true,
		RequestedMaxOutputTokens: 8192,
		OutputTokens:             8192,
	}, 600)
	return env.OutputTruncationClass == TruncationConfirmed
}

// ReadinessVerifyDashboardInvalidJSONWiring verifies dashboard call diagnostics on parse failure.
func ReadinessVerifyDashboardInvalidJSONWiring() bool {
	raw := &actions.AIRawResponse{
		RawText:      `{"executive_summary":`,
		ParseError:   "invalid json",
		OutputTokens: 10,
	}
	env := BuildAICallDiagnosticEnvelopeFromRaw(
		"dashboard", AIUsageComponentDashboardSynthesis, "fake",
		raw, 8192, 600, time.Time{}, time.Time{}, ParseStatusInvalidJSON, raw.ParseError,
	)
	artifact := BuildAICallRawResponseArtifact(env, raw.RawText)
	body, _ := artifact["body"].(string)
	return env.ParseStatus == ParseStatusInvalidJSON && body != "" && env.RawResponseCaptured
}

// ReadinessVerifyChunkInvalidJSONWiring verifies chunk raw artifact includes body on parse failure.
func ReadinessVerifyChunkInvalidJSONWiring() bool {
	chunkRun := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCalls: []dashboardhome.ChunkCallUsage{{
			ChunkID:     "chunk-1",
			Status:      dashboardhome.ChunkCallStatusFailed,
			RawBody:     `{"signals":[`,
			ParseStatus: ParseStatusInvalidJSON,
			ParseError:  "unexpected end",
		}},
	}
	artifact := buildChunkRawResponseArtifact(chunkRun)
	calls, ok := artifact["calls"].([]map[string]any)
	if !ok || len(calls) != 1 {
		return false
	}
	body, _ := calls[0]["body"].(string)
	return body == `{"signals":[`
}

// ReadinessVerifyFailedRunArtifactPersistence verifies compact diagnostics without full raw artifact.
func ReadinessVerifyFailedRunArtifactPersistence() bool {
	raw := &GlobalContextRawResponse{
		Content:     `not-json`,
		ParseStatus: ParseStatusInvalidJSON,
		ParseError:  "invalid_json",
	}
	env := BuildAICallDiagnosticEnvelopeFromGlobalContextRaw("fake", raw, 600)
	return env.RawResponseCaptured && env.RawResponseHash != "" && env.RawBodyHead != "" && env.ParseStatus == ParseStatusInvalidJSON
}

// ReadinessVerifyInspectorAICallViews verifies inspector builds AI call summary sections.
func ReadinessVerifyInspectorAICallViews() bool {
	views := buildInspectorAICallViews(map[string]dbgen.AiPipelineArtifact{}, LegacyActionsInspectorAdapter{})
	return views != nil && views["summary"] != nil
}
