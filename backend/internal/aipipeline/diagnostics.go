package aipipeline

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// PipelineDiagnostics captures runtime diagnostics for devtool verification.
type PipelineDiagnostics struct {
	GlobalContext       GlobalContextDiagnostics       `json:"global_context"`
	SharedChunkAnalysis SharedChunkAnalysisDiagnostics `json:"shared_chunk_analysis,omitempty"`
	Dashboard           StepDiagnostics                `json:"dashboard"`
	Actions             ActionsDiagnostics             `json:"actions"`
	Resume              *ResumeDiagnostics             `json:"resume,omitempty"`
	Restart             *StageRestartDiagnostics       `json:"restart,omitempty"`

	TotalInputTokens  int `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int `json:"total_output_tokens,omitempty"`
	RetryCount        int `json:"retry_count"`
	AttemptCount      int `json:"attempt_count,omitempty"`

	ReusedSharedChunkInputTokens    int `json:"reused_shared_chunk_input_tokens,omitempty"`
	ReusedSharedChunkOutputTokens   int `json:"reused_shared_chunk_output_tokens,omitempty"`
	ReusedGlobalContextInputTokens  int `json:"reused_global_context_input_tokens,omitempty"`
	ReusedGlobalContextOutputTokens int `json:"reused_global_context_output_tokens,omitempty"`
	ReusedDashboardInputTokens      int `json:"reused_dashboard_input_tokens,omitempty"`
	ReusedDashboardOutputTokens     int `json:"reused_dashboard_output_tokens,omitempty"`

	PublicationGuard PublicationGuardDecision `json:"publication_guard,omitempty"`
}

type SharedChunkAnalysisDiagnostics struct {
	Enabled                 bool                           `json:"shared_chunk_analysis_enabled,omitempty"`
	Source                  string                         `json:"shared_chunk_analysis_source,omitempty"`
	Status                  string                         `json:"shared_chunk_analysis_status,omitempty"`
	ChunkCount              int                            `json:"shared_chunk_count,omitempty"`
	Completed               int                            `json:"shared_chunk_completed,omitempty"`
	Failed                  int                            `json:"shared_chunk_failed,omitempty"`
	ProviderFailedCount     int                            `json:"shared_chunk_provider_failed,omitempty"`
	ValidationFailedCount   int                            `json:"shared_chunk_validation_failed,omitempty"`
	InsufficientDataCount   int                            `json:"shared_chunk_insufficient_data,omitempty"`
	UsablePercent           int                            `json:"shared_chunk_usable_percent,omitempty"`
	CoverageGatePassed      bool                           `json:"shared_chunk_coverage_gate_passed,omitempty"`
	CoverageFailureReasons  []string                       `json:"shared_chunk_coverage_failure_reasons,omitempty"`
	FamilyCoverage          map[string]FamilyCoverage      `json:"shared_chunk_family_coverage,omitempty"`
	ValidatedSignalCount    int                            `json:"validated_signal_count,omitempty"`
	RejectedSignalCount     int                            `json:"rejected_signal_count,omitempty"`
	ValidatedCandidateCount int                            `json:"validated_candidate_count,omitempty"`
	RejectedCandidateCount  int                            `json:"rejected_candidate_count,omitempty"`
	FailedChunkIDs          []string                       `json:"failed_chunk_ids,omitempty"`
	InputTokens             int                            `json:"shared_chunk_input_tokens,omitempty"`
	OutputTokens            int                            `json:"shared_chunk_output_tokens,omitempty"`
	UsageSource             string                         `json:"shared_chunk_usage_source,omitempty"`
	ChunkCalls              []dashboardhome.ChunkCallUsage `json:"shared_chunk_calls,omitempty"`
	ArtifactHash            string                         `json:"shared_chunk_artifact_hash,omitempty"`
	ArtifactMissReason      string                         `json:"shared_chunk_artifact_miss_reason,omitempty"`
	PartialLimitation       string                         `json:"shared_chunk_partial_limitation,omitempty"`
	PartialLimitationCode   string                         `json:"shared_chunk_partial_limitation_code,omitempty"`
}

type GlobalContextDiagnostics struct {
	Provider             string `json:"provider"`
	Status               string `json:"status"`
	InputSchema          string `json:"global_context_input_schema,omitempty"`
	Source               string `json:"global_context_source,omitempty"`
	InputHash            string `json:"global_context_input_hash,omitempty"`
	SignalCount          int    `json:"global_context_signal_count,omitempty"`
	FactCount            int    `json:"global_context_fact_count,omitempty"`
	EntityCount          int    `json:"global_context_entity_count,omitempty"`
	FailedChunkCount     int    `json:"global_context_failed_chunk_count,omitempty"`
	LimitationsCount     int    `json:"global_context_limitations_count,omitempty"`
	AICalls              int    `json:"global_context_ai_calls,omitempty"`
	RetryCalls           int    `json:"global_context_retry_calls,omitempty"`
	InputTokens          int    `json:"global_context_input_tokens,omitempty"`
	OutputTokens         int    `json:"global_context_output_tokens,omitempty"`
	UsageSource          string `json:"global_context_usage_source,omitempty"`
	ArtifactSource       string `json:"global_context_artifact_source,omitempty"`
	ArtifactHash         string `json:"global_context_artifact_hash,omitempty"`
	ArtifactMissReason   string `json:"global_context_artifact_miss_reason,omitempty"`
	FailureSubreason     string `json:"global_context_failure_subreason,omitempty"`
	RetryCount           int    `json:"retry_count"`
	IssuesCount          int    `json:"issues_count"`
	EstimatedInputTokens int    `json:"estimated_input_tokens,omitempty"`
	MaxInputTokens       int    `json:"max_input_tokens,omitempty"`
	Model                string `json:"model,omitempty"`

	InputJSONBytes               int    `json:"global_context_input_json_bytes,omitempty"`
	InputEstimatedTokens         int    `json:"global_context_input_estimated_tokens,omitempty"`
	SystemPromptBytes            int    `json:"global_context_system_prompt_bytes,omitempty"`
	SystemPromptEstimatedTokens  int    `json:"global_context_system_prompt_estimated_tokens,omitempty"`
	TaskPromptBytes              int    `json:"global_context_task_prompt_bytes,omitempty"`
	TaskPromptEstimatedTokens    int    `json:"global_context_task_prompt_estimated_tokens,omitempty"`
	SchemaBytes                  int    `json:"global_context_schema_bytes,omitempty"`
	SchemaEstimatedTokens        int    `json:"global_context_schema_estimated_tokens,omitempty"`
	TotalRequestBytes            int    `json:"global_context_total_request_bytes,omitempty"`
	TotalEstimatedInputTokens    int    `json:"global_context_total_estimated_input_tokens,omitempty"`
	TimeoutSeconds               int    `json:"global_context_timeout_seconds,omitempty"`
	MaxOutputTokens              int    `json:"global_context_max_output_tokens,omitempty"`
	InputHeadroomTokens          int    `json:"global_context_input_headroom_tokens,omitempty"`
	ValidationStatus             string `json:"global_context_validation_status,omitempty"`
	Confidence                   string `json:"global_context_confidence,omitempty"`
	ValidationStatusBefore       string `json:"global_context_validation_status_before,omitempty"`
	ConfidenceBefore             string `json:"global_context_confidence_before,omitempty"`
	ValidationBlockingIssueCount int    `json:"global_context_validation_blocking_issue_count,omitempty"`
	ValidationWarningCount       int    `json:"global_context_validation_warning_count,omitempty"`
	LatencyMs                    int64  `json:"global_context_latency_ms,omitempty"`

	LastValidation GlobalContextValidationResult `json:"-"`
}

type ResumeDiagnostics struct {
	Requested                 bool   `json:"resume_requested,omitempty"`
	SourcePipelineRunID       int64  `json:"resume_source_pipeline_run_id,omitempty"`
	Mode                      string `json:"resume_mode,omitempty"`
	ResumeFromStage           string `json:"resume_from_stage,omitempty"`
	DataPackSource            string `json:"data_pack_source,omitempty"`
	SharedChunkAnalysisSource string `json:"shared_chunk_analysis_source,omitempty"`
	SharedChunkAnalysisReused bool   `json:"shared_chunk_analysis_reused,omitempty"`
	SharedChunkAICalls        int    `json:"shared_chunk_ai_calls,omitempty"`
	GlobalContextSource       string `json:"global_context_source,omitempty"`
	GlobalContextReused       bool   `json:"global_context_reused,omitempty"`
	GlobalContextAICalls      int    `json:"global_context_ai_calls,omitempty"`
	DashboardSource           string `json:"dashboard_source,omitempty"`
	DashboardReused           bool   `json:"dashboard_reused,omitempty"`
	DashboardAICalls          int    `json:"dashboard_ai_calls,omitempty"`
	ActionsAICalls            int    `json:"actions_ai_calls,omitempty"`

	ReuseDataPack            bool     `json:"resume_plan_reuse_data_pack,omitempty"`
	ReuseChunkSet            bool     `json:"resume_plan_reuse_chunk_set,omitempty"`
	ReuseSharedChunkAnalysis bool     `json:"resume_plan_reuse_shared_chunk_analysis,omitempty"`
	ReuseGlobalContext       bool     `json:"resume_plan_reuse_global_context,omitempty"`
	ReuseDashboard           bool     `json:"resume_plan_reuse_dashboard,omitempty"`
	ReuseActions             bool     `json:"resume_plan_reuse_actions,omitempty"`
	NeedsActions             bool     `json:"resume_plan_needs_actions,omitempty"`
	NeedsPublication         bool     `json:"resume_plan_needs_publication,omitempty"`
	PlanReasons              []string `json:"resume_plan_reasons,omitempty"`
}

// StageRestartDiagnostics captures derived-run stage restart metadata.
type StageRestartDiagnostics struct {
	Requested            bool   `json:"requested,omitempty"`
	Mode                 string `json:"mode,omitempty"`
	SourcePipelineRunID  int64  `json:"source_pipeline_run_id,omitempty"`
	TargetPipelineRunID  int64  `json:"target_pipeline_run_id,omitempty"`
	RestartFromStage     string `json:"restart_from_stage,omitempty"`
	DataPackSource       string `json:"data_pack_source,omitempty"`
	DataPackReused       bool   `json:"data_pack_reused,omitempty"`
	SharedChunkSource    string `json:"shared_chunk_analysis_source,omitempty"`
	SharedChunkReused    bool   `json:"shared_chunk_analysis_reused,omitempty"`
	SharedChunkAICalls   int    `json:"shared_chunk_ai_calls,omitempty"`
	GlobalContextSource  string `json:"global_context_source,omitempty"`
	GlobalContextReused  bool   `json:"global_context_reused,omitempty"`
	GlobalContextAICalls int    `json:"global_context_ai_calls,omitempty"`
	DashboardSource      string `json:"dashboard_source,omitempty"`
	SourceDashboardRunID string `json:"source_dashboard_run_id,omitempty"`
	DashboardRunID       string `json:"dashboard_run_id,omitempty"`
	DashboardReused      bool   `json:"dashboard_reused,omitempty"`
	DashboardAICalls     int    `json:"dashboard_ai_calls,omitempty"`
	ActionsSource        string `json:"actions_source,omitempty"`
	ActionsReused        bool   `json:"actions_reused,omitempty"`
	ActionsAICalls       int    `json:"actions_ai_calls,omitempty"`
}

type StepDiagnostics struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Model    string `json:"model,omitempty"`

	ChunkSource                string `json:"dashboard_chunk_source,omitempty"`
	ChunkSetSource             string `json:"dashboard_chunk_set_source,omitempty"`
	PrecomputedSignalCount     int    `json:"dashboard_precomputed_signal_count,omitempty"`
	PrecomputedCandidateCount  int    `json:"dashboard_precomputed_candidate_count,omitempty"`
	SynthesisSignalCount       int    `json:"dashboard_synthesis_signal_count,omitempty"`
	SynthesisInputEstTokens    int    `json:"dashboard_synthesis_input_estimated_tokens,omitempty"`
	SynthesisInputTokenLimit   int    `json:"dashboard_synthesis_input_token_limit,omitempty"`
	SynthesisAICalls           int    `json:"dashboard_synthesis_ai_calls,omitempty"`
	InternalChunkAnalysisCalls int    `json:"dashboard_internal_chunk_analysis_calls,omitempty"`
	ChunkUsageIncluded         bool   `json:"dashboard_chunk_usage_included,omitempty"`
	SynthesisInputTokens       int    `json:"dashboard_synthesis_input_tokens,omitempty"`
	SynthesisOutputTokens      int    `json:"dashboard_synthesis_output_tokens,omitempty"`
	SynthesisRequestID         string `json:"dashboard_synthesis_request_id,omitempty"`
	SynthesisLatencyMs         int64  `json:"dashboard_synthesis_latency_ms,omitempty"`
	UsageSource                string `json:"dashboard_usage_source,omitempty"`
	ArtifactSource             string `json:"dashboard_artifact_source,omitempty"`
	Reused                     bool   `json:"dashboard_reused,omitempty"`
	ErrorCode                  string `json:"dashboard_error_code,omitempty"`

	ValidationMode               string   `json:"dashboard_validation_mode,omitempty"`
	ValidationStatus             string   `json:"dashboard_validation_status,omitempty"`
	ValidationWarningCount       int      `json:"dashboard_validation_warning_count,omitempty"`
	ValidationErrorCount         int      `json:"dashboard_validation_error_count,omitempty"`
	ValidationWarningCodes       []string `json:"dashboard_validation_warning_codes,omitempty"`
	ValidationErrorCodes         []string `json:"dashboard_validation_error_codes,omitempty"`
	ValidationBlockingErrorCount int      `json:"dashboard_validation_blocking_error_count,omitempty"`
	ValidationBlockingErrorCodes []string `json:"dashboard_validation_blocking_error_codes,omitempty"`
	Stage4WarningCount           int      `json:"dashboard_stage4_warning_count,omitempty"`

	PreOverrideTopActionsCount               int    `json:"dashboard_pre_override_top_actions_count,omitempty"`
	PreOverrideTopActionsMissingFactIDsCount int    `json:"dashboard_pre_override_top_actions_missing_fact_ids_count,omitempty"`
	FinalTopActionsSource                    string `json:"dashboard_final_top_actions_source,omitempty"`
	FinalTopActionsCount                     int    `json:"dashboard_final_top_actions_count,omitempty"`
	FinalTopActionsMissingFactIDsCount       int    `json:"dashboard_final_top_actions_missing_fact_ids_count,omitempty"`
}

type ActionsDiagnostics struct {
	Provider      string `json:"provider"`
	Status        string `json:"status"`
	AcceptedCount int    `json:"accepted_count"`
	DroppedCount  int    `json:"dropped_count"`
	RetryCount    int    `json:"retry_count"`
	Model         string `json:"model,omitempty"`

	InputSchema             string `json:"actions_input_schema,omitempty"`
	InputSource             string `json:"actions_input_source,omitempty"`
	CandidateCount          int    `json:"actions_candidate_count,omitempty"`
	FactCatalogCount        int    `json:"actions_fact_catalog_count,omitempty"`
	EntityCatalogCount      int    `json:"actions_entity_catalog_count,omitempty"`
	FailedChunkCount        int    `json:"actions_failed_chunk_count,omitempty"`
	LimitationsCount        int    `json:"actions_limitations_count,omitempty"`
	InputEstimatedTokens    int    `json:"actions_input_estimated_tokens,omitempty"`
	InputTokenLimit         int    `json:"actions_input_token_limit,omitempty"`
	InputTokens             int    `json:"actions_input_tokens,omitempty"`
	OutputTokens            int    `json:"actions_output_tokens,omitempty"`
	UsageSource             string `json:"actions_usage_source,omitempty"`
	ArtifactSource          string `json:"actions_artifact_source,omitempty"`
	ComposeCalls            int    `json:"actions_compose_calls,omitempty"`
	ObjectDetailCalls       int    `json:"actions_object_detail_calls,omitempty"`
	TotalAICalls            int    `json:"actions_total_ai_calls,omitempty"`
	RetryCalls              int    `json:"actions_retry_calls,omitempty"`
	FinalCount              int    `json:"actions_final_count,omitempty"`
	FinalStatus             string `json:"actions_final_status,omitempty"`
	InputHash               string `json:"actions_input_hash,omitempty"`
	InputDataPackSerialized bool   `json:"actions_input_data_pack_serialized,omitempty"`

	ValidatorMode         string         `json:"actions_validator_mode,omitempty"`
	ReturnedCount         int            `json:"actions_returned_count,omitempty"`
	ValidBeforeLimitCount int            `json:"actions_valid_before_limit_count,omitempty"`
	TruncatedCount        int            `json:"actions_truncated_count,omitempty"`
	AIOrderPreserved      bool           `json:"actions_ai_order_preserved,omitempty"`
	TopActionKeysCount    int            `json:"actions_top_action_keys_count,omitempty"`
	FailureSubreason      string         `json:"actions_failure_subreason,omitempty"`
	DropIssueCounts       map[string]int `json:"actions_drop_issue_counts,omitempty"`
	WarningIssueCounts    map[string]int `json:"actions_warning_issue_counts,omitempty"`

	NormalizationActionDroppedCount int `json:"actions_normalization_action_dropped_count,omitempty"`
	AfterNormalizationCount         int `json:"actions_after_normalization_count,omitempty"`
	ValidationActionDroppedCount    int `json:"actions_validation_action_dropped_count,omitempty"`
	AfterValidationCount            int `json:"actions_after_validation_count,omitempty"`
	LimitDroppedCount               int `json:"actions_limit_dropped_count,omitempty"`
	AfterLimitCount                 int `json:"actions_after_limit_count,omitempty"`
	PersistGuardActionDroppedCount  int `json:"actions_persist_guard_action_dropped_count,omitempty"`
	AfterPersistGuardCount          int `json:"actions_after_persist_guard_count,omitempty"`

	CompletionConditionsReturnedCount      int `json:"actions_completion_conditions_returned_count,omitempty"`
	CompletionConditionsValidCount         int `json:"actions_completion_conditions_valid_count,omitempty"`
	CompletionConditionsDroppedCount       int `json:"actions_completion_conditions_dropped_count,omitempty"`
	CompletionConditionsFactIDRemovedCount int `json:"actions_completion_conditions_fact_id_removed_count,omitempty"`
	CompletionConditionsEmptyActionsCount  int `json:"actions_completion_conditions_empty_actions_count,omitempty"`

	ActionsWithCompletionConditionsCount    int     `json:"actions_with_completion_conditions_count,omitempty"`
	ActionsWithoutCompletionConditionsCount int     `json:"actions_without_completion_conditions_count,omitempty"`
	ActionsCompletionConditionsCoveragePct  float64 `json:"actions_completion_conditions_coverage_pct,omitempty"`
	ActionsCompletionDisplayMismatchCount   int     `json:"actions_completion_display_mismatch_count,omitempty"`
	ActionsVagueCompletionCriteriaCount     int     `json:"actions_vague_completion_criteria_count,omitempty"`

	ActionsEntityScopedCount  int `json:"actions_entity_scoped_count,omitempty"`
	ActionsAccountScopedCount int `json:"actions_account_scoped_count,omitempty"`
	ActionsMixedScopedCount   int `json:"actions_mixed_scoped_count,omitempty"`

	ActionsObjectActionsFullCount          int     `json:"actions_object_actions_full_count,omitempty"`
	ActionsObjectActionsPartialCount       int     `json:"actions_object_actions_partial_count,omitempty"`
	ActionsObjectActionsMissingCount       int     `json:"actions_object_actions_missing_count,omitempty"`
	ActionsAffectedEntitiesTotalCount      int     `json:"actions_affected_entities_total_count,omitempty"`
	ActionsObjectActionsReturnedTotalCount int     `json:"actions_object_actions_returned_total_count,omitempty"`
	ActionsObjectActionsCoveragePct        float64 `json:"actions_object_actions_coverage_pct,omitempty"`

	CandidateCoverage map[string]any `json:"actions_candidate_coverage,omitempty"`

	InputActualTokens            int     `json:"actions_input_actual_tokens,omitempty"`
	InputHeadroomEstimatedTokens int     `json:"actions_input_headroom_estimated_tokens,omitempty"`
	InputHeadroomActualTokens    int     `json:"actions_input_headroom_actual_tokens,omitempty"`
	InputEstimationErrorTokens   int     `json:"actions_input_estimation_error_tokens,omitempty"`
	InputEstimationErrorPct      float64 `json:"actions_input_estimation_error_pct,omitempty"`

	TimeoutSeconds                     int    `json:"actions_timeout_seconds,omitempty"`
	ComposeTimeoutSeconds              int    `json:"actions_compose_timeout_seconds,omitempty"`
	ObjectDetailTimeoutSeconds         int    `json:"actions_object_detail_timeout_seconds,omitempty"`
	TotalRequestBytes                  int    `json:"actions_total_request_bytes,omitempty"`
	TotalEstimatedInputTokens          int    `json:"actions_total_estimated_input_tokens,omitempty"`
	MaxInputTokens                     int    `json:"actions_max_input_tokens,omitempty"`
	MaxOutputTokens                    int    `json:"actions_max_output_tokens,omitempty"`
	RequestedMaxOutputTokens           int    `json:"actions_requested_max_output_tokens,omitempty"`
	ProviderOutputTokens               int    `json:"actions_provider_output_tokens,omitempty"`
	OutputTokensExceededRequestedLimit bool   `json:"actions_output_tokens_exceeded_requested_limit,omitempty"`
	InputHeadroomTokens                int    `json:"actions_input_headroom_tokens,omitempty"`
	LatencyMs                          int64  `json:"actions_latency_ms,omitempty"`
	RequestID                          string `json:"actions_request_id,omitempty"`

	DataPackSource       string `json:"actions_data_pack_source,omitempty"`
	SharedAnalysisSource string `json:"actions_shared_analysis_source,omitempty"`
	GlobalContextSource  string `json:"actions_global_context_source,omitempty"`
	DashboardSource      string `json:"actions_dashboard_source,omitempty"`
}

func newPipelineDiagnostics(providers PipelineProviders) PipelineDiagnostics {
	return PipelineDiagnostics{
		GlobalContext: GlobalContextDiagnostics{Provider: providers.GlobalContext},
		Dashboard:     StepDiagnostics{Provider: providers.Dashboard},
		Actions:       ActionsDiagnostics{Provider: providers.Actions},
	}
}

func populateDashboardDiagnostics(diag *PipelineDiagnostics, outcome *DashboardRunOutcome) {
	if diag == nil || outcome == nil || outcome.Result == nil {
		return
	}
	r := outcome.Result
	diag.Dashboard.ChunkSource = r.DashboardChunkSource
	diag.Dashboard.ChunkSetSource = r.DashboardChunkSetSource
	diag.Dashboard.PrecomputedSignalCount = r.DashboardPrecomputedSignalCount
	diag.Dashboard.PrecomputedCandidateCount = r.DashboardPrecomputedCandidateCount
	diag.Dashboard.SynthesisSignalCount = r.DashboardSynthesisSignalCount
	diag.Dashboard.SynthesisInputEstTokens = r.DashboardSynthesisInputEstTokens
	diag.Dashboard.SynthesisInputTokenLimit = r.DashboardSynthesisInputTokenLimit
	diag.Dashboard.SynthesisAICalls = r.DashboardSynthesisAICalls
	diag.Dashboard.InternalChunkAnalysisCalls = r.DashboardInternalChunkAnalysisCalls
	diag.Dashboard.ChunkUsageIncluded = r.DashboardChunkUsageIncluded
	diag.Dashboard.SynthesisInputTokens = r.DashboardSynthesisInputTokens
	diag.Dashboard.SynthesisOutputTokens = r.DashboardSynthesisOutputTokens
	diag.Dashboard.SynthesisRequestID = r.DashboardSynthesisRequestID
	diag.Dashboard.SynthesisLatencyMs = r.DashboardSynthesisLatencyMs
	if diag.Dashboard.SynthesisInputTokens == 0 && !r.DashboardChunkUsageIncluded {
		diag.Dashboard.SynthesisInputTokens = r.InputTokens
		diag.Dashboard.SynthesisOutputTokens = r.OutputTokens
	}
	if diag.Dashboard.SynthesisAICalls == 0 && diag.Dashboard.ArtifactSource == "persisted" {
		diag.Dashboard.UsageSource = UsageSourceArtifact
	}
	diag.Dashboard.ErrorCode = r.ErrorCode
	diag.Dashboard.ValidationMode = r.DashboardValidationMode
	if diag.Dashboard.ValidationMode == "" {
		diag.Dashboard.ValidationMode = dashboardhome.CurrentDashboardValidationMode()
	}
	diag.Dashboard.ValidationStatus = r.DashboardValidationStatus
	diag.Dashboard.ValidationWarningCount = r.DashboardValidationWarningCount
	diag.Dashboard.ValidationErrorCount = r.DashboardValidationErrorCount
	diag.Dashboard.ValidationBlockingErrorCount = r.DashboardValidationErrorCount
	diag.Dashboard.ValidationWarningCodes = append([]string(nil), r.DashboardValidationWarningCodes...)
	diag.Dashboard.ValidationErrorCodes = append([]string(nil), r.DashboardValidationErrorCodes...)
	diag.Dashboard.ValidationBlockingErrorCodes = append([]string(nil), r.DashboardValidationErrorCodes...)
	diag.Dashboard.Stage4WarningCount = r.DashboardStage4WarningCount
	diag.Dashboard.PreOverrideTopActionsCount = r.DashboardPreOverrideTopActionsCount
	diag.Dashboard.PreOverrideTopActionsMissingFactIDsCount = r.DashboardPreOverrideTopActionsMissingFactIDsCount
	diag.Dashboard.FinalTopActionsSource = r.DashboardFinalTopActionsSource
	diag.Dashboard.FinalTopActionsCount = r.DashboardFinalTopActionsCount
	diag.Dashboard.FinalTopActionsMissingFactIDsCount = r.DashboardFinalTopActionsMissingFactIDsCount
	if r.IsSuccess() {
		if r.Status == "completed_with_warnings" || r.DashboardValidationStatus == "completed_with_warnings" {
			diag.Dashboard.Status = "completed"
			if diag.Dashboard.ValidationStatus == "" {
				diag.Dashboard.ValidationStatus = "completed_with_warnings"
			}
		} else if diag.Dashboard.Status == "" {
			diag.Dashboard.Status = "completed"
		}
	} else if !r.Skipped && diag.Dashboard.Status == "" {
		diag.Dashboard.Status = "failed"
	}
}
