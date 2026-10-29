package aipipeline

import (
	"errors"
	"os"
	"strings"
)

const (
	PipelineModeUnified = "unified"

	PipelineStatusRunning           = "running"
	PipelineStatusCompleted         = "completed"
	PipelineStatusCompletedPartial  = "completed_partial"
	PipelineStatusFailed            = "failed"
	PipelineStatusContextFailed     = "context_failed"
	PipelineStatusDashboardFailed   = "dashboard_failed"
	PipelineStatusActionsFailed     = "actions_failed"
	PipelineStatusActionsAllDropped = "actions_all_dropped"
	PipelineStatusEmpty             = "empty"
	PipelineStatusSkipped           = "skipped"

	GlobalContextStatusCompleted = "completed"
	GlobalContextStatusFailed    = "failed"

	StepStatusCompleted = "completed"
	StepStatusFailed    = "failed"
	StepStatusSkipped   = "skipped"
)

var ErrPipelineDisabled = errors.New("seller_ai_pipeline_disabled")
var ErrPipelineRequiresAIFirst = errors.New("seller_ai_pipeline_requires_actions_ai_first")

// Artifact kinds stored when DebugArtifacts=true.
const (
	ArtifactKindDataPack                = "data_pack"
	ArtifactKindGlobalContextInput      = "global_context_input"
	ArtifactKindGlobalContextPrompt     = "global_context_prompt"
	ArtifactKindGlobalContextResponse   = "global_context_response"
	ArtifactKindGlobalContextValidation = "global_context_validation"
	ArtifactKindDashboardInput          = "dashboard_input"
	ArtifactKindDashboardOutput         = "dashboard_output"
	ArtifactKindDashboardValidation     = "dashboard_validation"
	ArtifactKindActionsInput            = "actions_input"
	ArtifactKindActionsOutput           = "actions_output"
	ArtifactKindActionsValidation       = "actions_validation"
	ArtifactKindPipelineSummary         = "pipeline_summary"
	ArtifactKindSharedChunkAnalysis     = "shared_chunk_analysis"
	ArtifactKindGlobalContextFinal      = "global_context"
)

const SharedChunkAnalysisArtifactSchemaVersion = "shared_chunk_analysis_v2"

const (
	ChunkSetArtifactSchemaVersion           = "chunk_set_v1"
	ChunkRawResponseArtifactSchemaVersion   = "chunk_raw_response_v1"
	ChunkValidationArtifactSchemaVersion    = "chunk_validation_v1"
	GlobalContextInputArtifactSchemaVersion = "global_context_input_v1"
	DashboardInputArtifactSchemaVersion     = "dashboard_input_v1"
	ActionsInputArtifactSchemaVersion       = "actions_input_v1"
	RawOutputArtifactSchemaVersion          = "raw_output_v1"
)

const (
	ArtifactKindChunkSet               = "chunk_set"
	ArtifactKindChunkRawResponse       = "chunk_raw_response"
	ArtifactKindChunkValidation        = "chunk_validation"
	ArtifactKindGlobalContextRawOutput = "global_context_raw_output"
	ArtifactKindDashboardRawOutput     = "dashboard_raw_output"
	ArtifactKindActionsRawOutput       = "actions_raw_output"

	ArtifactKindActionsComposeInput          = "actions_compose_input"
	ArtifactKindActionsComposeOutput         = "actions_compose_output"
	ArtifactKindActionsComposeRawOutput      = "actions_compose_raw_output"
	ArtifactKindActionsComposeValidation     = "actions_compose_validation"
	ArtifactKindActionsNormalization         = "actions_normalization"
	ArtifactKindActionsObjectDetailMerge     = "actions_object_detail_merge"
	ArtifactKindActionsFinalOutput           = "actions_final_output"
	ArtifactKindActionObjectDetailInput      = "action_object_detail_input"
	ArtifactKindActionObjectDetailOutput     = "action_object_detail_output"
	ArtifactKindActionObjectDetailValidation = "action_object_detail_validation"
	ArtifactKindAIPipelineLineage            = "ai_pipeline_lineage"

	ArtifactKindMetricSemanticsSnapshot       = "metric_semantics_snapshot"
	ArtifactKindSharedChunkRefMapping         = "shared_chunk_ref_mapping"
	ArtifactKindFinalOutputSemanticValidation = "final_output_semantic_validation"
	ArtifactKindPublicationGuardDecision      = "publication_guard_decision"
	ArtifactKindActionsExecutionDiagnostics   = "actions_execution_diagnostics"

	ArtifactKindActionsComposeCallDiagnostics     = "actions_compose_call_diagnostics"
	ArtifactKindActionsComposeRawResponse         = "actions_compose_raw_response"
	ArtifactKindActionsComposeParsedOutput        = "actions_compose_parsed_output"
	ArtifactKindActionsLifecycleDiagnostics       = "actions_lifecycle_diagnostics"
	ArtifactKindActionObjectDetailCallDiagnostics = "action_object_detail_call_diagnostics"
	ArtifactKindActionObjectDetailRawResponse     = "action_object_detail_raw_response"
	ArtifactKindGlobalContextCallDiagnostics      = "global_context_call_diagnostics"
	ArtifactKindGlobalContextParsedOutput         = "global_context_parsed_output"
	ArtifactKindGlobalContextRawResponse          = "global_context_raw_response"
	ArtifactKindDashboardCallDiagnostics          = "dashboard_call_diagnostics"
	ArtifactKindDashboardParsedOutput             = "dashboard_parsed_output"
	ArtifactKindDashboardRawResponse              = "dashboard_raw_response"
)

const (
	MetricSemanticsSnapshotSchemaVersion         = "metric_semantics_snapshot_v1"
	SharedChunkRefMappingSchemaVersion           = "shared_chunk_ref_mapping_v1"
	GlobalContextValidationArtifactSchemaVersion = "global_context_validation_v1"
	ActionsExecutionDiagnosticsSchemaVersion     = "actions_execution_diagnostics_v1"
	ActionsComposeParsedOutputSchemaVersion      = "actions_compose_parsed_output_v1"
	ActionsLifecycleDiagnosticsSchemaVersion     = "actions_lifecycle_diagnostics_v1"
	GlobalContextParsedOutputSchemaVersion       = "global_context_parsed_output_v1"
	DashboardParsedOutputSchemaVersion           = "dashboard_parsed_output_v1"
)

const (
	sharedChunkAnalysisPartialLimitation     = "Часть данных не была проанализирована из-за технической ошибки."
	sharedChunkAnalysisPartialLimitationCode = "shared_chunk_analysis_partial"
)

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// SellerAIPipelineEnabled reports whether the unified pipeline is enabled.
func SellerAIPipelineEnabled() bool {
	return envBool("SELLER_AI_PIPELINE_ENABLED", true)
}

// ActionsAIFirstEnabled reports whether AI-first actions generation is enabled.
func ActionsAIFirstEnabled() bool {
	return envBool("ACTIONS_AI_FIRST", true)
}

// LegacyAIPostSyncEnabled reports whether legacy AI daily summaries post-sync flow is enabled.
func LegacyAIPostSyncEnabled() bool {
	return envBool("LEGACY_AI_POST_SYNC_ENABLED", false)
}

// SharedChunkAnalysisEnabled reports whether shared chunk analysis (signals +
// action candidates from a single chunk pass) is enabled. Stage 1: flag only.
func SharedChunkAnalysisEnabled() bool {
	return envBool("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", false)
}

// DevAIPipelineAllowed reports whether devtools may run the pipeline.
func DevAIPipelineAllowed() bool {
	return strings.TrimSpace(os.Getenv("ALLOW_DEV_AI_PIPELINE")) == "1"
}

// ValidatePipelineConfig returns an error when unified pipeline cannot run.
func ValidatePipelineConfig() error {
	if !SellerAIPipelineEnabled() {
		return ErrPipelineDisabled
	}
	if !ActionsAIFirstEnabled() {
		return ErrPipelineRequiresAIFirst
	}
	return nil
}
