package aipipeline

// Artifact content completeness values for Inspector artifact rows.
const (
	ArtifactCompletenessFullStructured = "full_structured_document"
	ArtifactCompletenessRawProvider    = "raw_provider_body"
	ArtifactCompletenessPartial        = "partial"
	ArtifactCompletenessMalformed      = "malformed"
	ArtifactCompletenessNotApplicable  = "not_applicable"
)

// ClassifyArtifactContentCompleteness classifies persisted artifact payload shape by kind.
// JSON parse success is reported separately via parsed_status; this describes diagnostic payload depth.
func ClassifyArtifactContentCompleteness(kind string, payload map[string]any) string {
	if payload == nil {
		return DiagnosticStatusNotCaptured
	}
	switch kind {
	case ArtifactKindDataPack,
		ArtifactKindSharedChunkAnalysis,
		ArtifactKindGlobalContextFinal,
		ArtifactKindActionsInput,
		ArtifactKindActionsComposeInput,
		ArtifactKindActionsOutput,
		ArtifactKindActionsFinalOutput,
		ArtifactKindActionsValidation,
		ArtifactKindPipelineSummary,
		ArtifactKindChunkSet,
		ArtifactKindGlobalContextValidation,
		ArtifactKindDashboardValidation,
		ArtifactKindActionsComposeParsedOutput,
		ArtifactKindGlobalContextParsedOutput,
		ArtifactKindDashboardParsedOutput,
		ArtifactKindActionsNormalization,
		ArtifactKindAIPipelineLineage,
		ArtifactKindSharedChunkRefMapping,
		ArtifactKindMetricSemanticsSnapshot,
		ArtifactKindPublicationGuardDecision,
		ArtifactKindActionsExecutionDiagnostics,
		ArtifactKindFinalOutputSemanticValidation:
		return ArtifactCompletenessFullStructured
	case ArtifactKindGlobalContextCallDiagnostics,
		ArtifactKindDashboardCallDiagnostics,
		ArtifactKindActionsComposeCallDiagnostics,
		ArtifactKindActionObjectDetailCallDiagnostics,
		ArtifactKindActionsLifecycleDiagnostics:
		return ArtifactCompletenessFullStructured
	case ArtifactKindGlobalContextRawResponse,
		ArtifactKindDashboardRawResponse,
		ArtifactKindActionsComposeRawResponse,
		ArtifactKindActionObjectDetailRawResponse,
		ArtifactKindChunkRawResponse:
		if ct, ok := payload["content_type"].(string); ok && ct == ArtifactContentFullBody {
			return ArtifactCompletenessRawProvider
		}
		if body, ok := payload["body"].(string); ok && body != "" {
			return ArtifactCompletenessRawProvider
		}
		if calls, ok := payload["calls"].([]any); ok {
			for _, item := range calls {
				call, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if body, ok := call["body"].(string); ok && body != "" {
					return ArtifactCompletenessRawProvider
				}
			}
		}
		_, rawAvailable := ClassifyLegacyRawOutputArtifact(payload)
		if rawAvailable {
			return ArtifactCompletenessRawProvider
		}
		if ct, ok := payload["content_type"].(string); ok && ct == ArtifactContentMetadataOnly {
			return ArtifactContentMetadataOnly
		}
		return ArtifactCompletenessPartial
	case ArtifactKindActionsRawOutput, ArtifactKindDashboardRawOutput, ArtifactKindGlobalContextRawOutput:
		contentType, rawAvailable := ClassifyLegacyRawOutputArtifact(payload)
		if rawAvailable {
			return ArtifactCompletenessRawProvider
		}
		if contentType == ArtifactContentMetadataOnly {
			return ArtifactContentMetadataOnly
		}
		return ArtifactCompletenessPartial
	default:
		if len(payload) == 0 {
			return DiagnosticStatusNotCaptured
		}
		_, rawAvailable := ClassifyLegacyRawOutputArtifact(payload)
		if rawAvailable {
			return ArtifactCompletenessRawProvider
		}
		return ArtifactCompletenessFullStructured
	}
}

func artifactRawBodyAvailable(kind string, payload map[string]any) bool {
	switch ClassifyArtifactContentCompleteness(kind, payload) {
	case ArtifactCompletenessRawProvider:
		return true
	default:
		return false
	}
}
