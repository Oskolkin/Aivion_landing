package aipipeline

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildInspectorMonitoringViews(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	return map[string]any{
		"metric_semantics":          buildInspectorMetricSemanticsView(byKind),
		"chunk_ref_mappings":        buildInspectorChunkRefMappingsView(byKind),
		"global_context_validation": buildInspectorGlobalContextValidationView(byKind),
		"final_output_validation":   buildInspectorFinalOutputValidationView(byKind),
		"actions_execution":         buildInspectorActionsExecutionView(byKind),
		"publication_guard":         buildInspectorPublicationGuardView(byKind),
	}
}

func buildInspectorMetricSemanticsView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindMetricSemanticsSnapshot]
	if !ok {
		return artifactNotAvailableView(ArtifactKindMetricSemanticsSnapshot)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{
		"status":            "available",
		"registry_version":  payload["registry_version"],
		"registry_hash":     payload["registry_hash"],
		"used_metric_codes": payload["used_metric_codes"],
		"definitions":       payload["definitions"],
	}
}

func buildInspectorChunkRefMappingsView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindSharedChunkRefMapping]
	if !ok {
		return artifactNotAvailableView(ArtifactKindSharedChunkRefMapping)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{
		"status": "available",
		"chunks": payload["chunks"],
	}
}

func buildInspectorGlobalContextValidationView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindGlobalContextValidation]
	if !ok {
		return artifactNotAvailableView(ArtifactKindGlobalContextValidation)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{
		"status":                       "available",
		"status_before_validation":     payload["status_before_validation"],
		"status_after_validation":      payload["status_after_validation"],
		"confidence_before_validation": payload["confidence_before_validation"],
		"confidence_after_validation":  payload["confidence_after_validation"],
		"blocking_issue_count":         payload["blocking_issue_count"],
		"warning_count":                payload["warning_count"],
		"issues":                       payload["issues"],
	}
}

func buildInspectorFinalOutputValidationView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindFinalOutputSemanticValidation]
	if !ok {
		if guard, ok := byKind[ArtifactKindPublicationGuardDecision]; ok {
			if payload, ok := decodeArtifactPayload(guard.PayloadJson).(map[string]any); ok {
				if nested, ok := payload["final_semantic_validation"].(map[string]any); ok {
					return map[string]any{"status": "compact_from_publication_guard", "payload": nested}
				}
			}
		}
		return artifactNotAvailableView(ArtifactKindFinalOutputSemanticValidation)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{"status": "available", "payload": payload}
}

func buildInspectorActionsExecutionView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindActionsExecutionDiagnostics]
	if !ok {
		return artifactNotAvailableView(ArtifactKindActionsExecutionDiagnostics)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{"status": "available", "payload": payload}
}

func buildInspectorPublicationGuardView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	art, ok := byKind[ArtifactKindPublicationGuardDecision]
	if !ok {
		return artifactNotAvailableView(ArtifactKindPublicationGuardDecision)
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error"}
	}
	return map[string]any{"status": "available", "payload": payload}
}

func artifactNotAvailableView(kind string) map[string]any {
	return map[string]any{
		"status":  "not_available",
		"message": "Artifact not available for this pipeline version",
		"kind":    kind,
	}
}
