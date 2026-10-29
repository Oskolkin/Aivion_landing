package dashboardhome

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NarrativeGenerationResult is the structured outcome of a dashboard narrative run.
type NarrativeGenerationResult struct {
	RunID                    string   `json:"RunID,omitempty"`
	Status                   string   `json:"Status"`
	MetricDate               string   `json:"MetricDate,omitempty"`
	PromptVersion            string   `json:"PromptVersion,omitempty"`
	ContextHash              string   `json:"ContextHash,omitempty"`
	RenderedContextHash      string   `json:"RenderedContextHash,omitempty"`
	ValidationErrors         []string `json:"ValidationErrors,omitempty"`
	ErrorMessage             string   `json:"ErrorMessage,omitempty"`
	InputTokens              int      `json:"InputTokens,omitempty"`
	OutputTokens             int      `json:"OutputTokens,omitempty"`
	Skipped                  bool     `json:"Skipped,omitempty"`
	ContextSource            string   `json:"context_source,omitempty"`
	AIOnly                   bool     `json:"ai_only,omitempty"`
	AllRequiredBlocksPresent bool     `json:"all_required_blocks_present,omitempty"`
	MissingRequiredBlocks    []string `json:"missing_required_blocks,omitempty"`
	FallbackBlocks           []string `json:"fallback_blocks,omitempty"`
	LegacyBlocks             []string `json:"legacy_blocks,omitempty"`
	PendingBlocks            []string `json:"pending_blocks,omitempty"`
	StaleBlocks              []string `json:"stale_blocks,omitempty"`

	UsedLegacyFallback    bool   `json:"used_legacy_fallback"`
	FinalPayloadSource    string `json:"final_payload_source,omitempty"`
	ChunkPipelineStatus   string `json:"chunk_pipeline_status,omitempty"`
	ChunkSynthesisStatus  string `json:"chunk_synthesis_status,omitempty"`
	ChunkAnalysisStatus   string `json:"chunk_analysis_status,omitempty"`
	ChunkParseErrorsCount int    `json:"chunk_parse_errors_count,omitempty"`
	SynthesisParseError   string `json:"synthesis_parse_error,omitempty"`
	FallbackReason        string `json:"fallback_reason,omitempty"`

	ChunkCount             int `json:"chunk_count,omitempty"`
	ChunkAnalysisCompleted int `json:"chunk_analysis_completed,omitempty"`
	ChunkAnalysisFailed    int `json:"chunk_analysis_failed,omitempty"`
	ValidatedSignalCount   int `json:"validated_signal_count,omitempty"`
	RejectedSignalCount    int `json:"rejected_signal_count,omitempty"`

	ErrorCode string `json:"error_code,omitempty"`

	DashboardChunkSource                       string `json:"dashboard_chunk_source,omitempty"`
	DashboardChunkSetSource                    string `json:"dashboard_chunk_set_source,omitempty"`
	DashboardPrecomputedSignalCount            int    `json:"dashboard_precomputed_signal_count,omitempty"`
	DashboardPrecomputedCandidateCount         int    `json:"dashboard_precomputed_candidate_count,omitempty"`
	DashboardSynthesisSignalCount              int    `json:"dashboard_synthesis_signal_count,omitempty"`
	DashboardSynthesisInputEstTokens           int    `json:"dashboard_synthesis_input_estimated_tokens,omitempty"`
	DashboardSynthesisInputTokenLimit          int    `json:"dashboard_synthesis_input_token_limit,omitempty"`
	DashboardSynthesisAICalls                  int    `json:"dashboard_synthesis_ai_calls,omitempty"`
	DashboardInternalChunkAnalysisCalls        int    `json:"dashboard_internal_chunk_analysis_calls,omitempty"`
	DashboardChunkUsageIncluded                bool   `json:"dashboard_chunk_usage_included,omitempty"`
	DashboardSynthesisInputTokens              int    `json:"dashboard_synthesis_input_tokens,omitempty"`
	DashboardSynthesisOutputTokens             int    `json:"dashboard_synthesis_output_tokens,omitempty"`
	DashboardSynthesisRequestID                string `json:"dashboard_synthesis_request_id,omitempty"`
	DashboardSynthesisLatencyMs                int64  `json:"dashboard_synthesis_latency_ms,omitempty"`
	DashboardSynthesisModel                    string `json:"dashboard_synthesis_model,omitempty"`
	DashboardSynthesisRawResponse              string `json:"dashboard_synthesis_raw_response,omitempty"`
	DashboardSynthesisProviderStatus           string `json:"dashboard_synthesis_provider_status,omitempty"`
	DashboardSynthesisFinishReason             string `json:"dashboard_synthesis_finish_reason,omitempty"`
	DashboardSynthesisIncompleteReason         string `json:"dashboard_synthesis_incomplete_reason,omitempty"`
	DashboardSynthesisOutputTruncated          bool   `json:"dashboard_synthesis_output_truncated,omitempty"`
	DashboardSynthesisRequestedMaxOutputTokens int    `json:"dashboard_synthesis_requested_max_output_tokens,omitempty"`

	FullDataPackFactCount      int `json:"full_data_pack_fact_count,omitempty"`
	ChunkLocalFactCount        int `json:"chunk_local_fact_count,omitempty"`
	ValidatedSignalFactCount   int `json:"validated_signal_fact_count,omitempty"`
	FullEntityCount            int `json:"full_entity_count,omitempty"`
	ChunkLocalEntityCount      int `json:"chunk_local_entity_count,omitempty"`
	ValidatedSignalEntityCount int `json:"validated_signal_entity_count,omitempty"`
	InventedFactIDsRejected    int `json:"invented_fact_ids_rejected,omitempty"`

	// DataPackFactCount is deprecated; use full_data_pack_fact_count.
	DataPackFactCount   int `json:"data_pack_fact_count,omitempty"`
	DataPackEntityCount int `json:"data_pack_entity_count,omitempty"`

	RepairSummary *NarrativeRepairSummary `json:"repair_summary,omitempty"`

	DashboardValidationMode         string   `json:"dashboard_validation_mode,omitempty"`
	DashboardValidationStatus       string   `json:"dashboard_validation_status,omitempty"`
	DashboardValidationWarningCount int      `json:"dashboard_validation_warning_count,omitempty"`
	DashboardValidationErrorCount   int      `json:"dashboard_validation_error_count,omitempty"`
	DashboardValidationWarningCodes []string `json:"dashboard_validation_warning_codes,omitempty"`
	DashboardValidationErrorCodes   []string `json:"dashboard_validation_error_codes,omitempty"`
	DashboardStage4WarningCount     int      `json:"dashboard_stage4_warning_count,omitempty"`

	ValidatedNarrativePayload json.RawMessage `json:"-"`

	DashboardPreOverrideTopActionsCount               int    `json:"dashboard_pre_override_top_actions_count,omitempty"`
	DashboardPreOverrideTopActionsMissingFactIDsCount int    `json:"dashboard_pre_override_top_actions_missing_fact_ids_count,omitempty"`
	DashboardFinalTopActionsSource                    string `json:"dashboard_final_top_actions_source,omitempty"`
	DashboardFinalTopActionsCount                     int    `json:"dashboard_final_top_actions_count,omitempty"`
	DashboardFinalTopActionsMissingFactIDsCount       int    `json:"dashboard_final_top_actions_missing_fact_ids_count,omitempty"`
}

type NarrativeRepairSummary struct {
	StatusBeforeRepair string                 `json:"status_before_repair,omitempty"`
	RepairsTotal       int                    `json:"repairs_total"`
	Repairs            []NarrativeRepairEntry `json:"repairs,omitempty"`
	SemanticRepairs    int                    `json:"semantic_repairs"`
	TechnicalRepairs   int                    `json:"technical_repairs"`
}

type NarrativeRepairEntry struct {
	Path           string `json:"path"`
	Type           string `json:"type"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	SemanticChange bool   `json:"semantic_change"`
}

func buildNarrativeRepairSummary(repair narrativeRepairResult, finalStatus string) *NarrativeRepairSummary {
	if finalStatus != "completed_repaired" && len(repair.RepairableErrors) == 0 && len(repair.RepairActions) == 0 {
		return nil
	}
	entries := make([]NarrativeRepairEntry, 0, len(repair.RepairActions)+len(repair.RepairableErrors))
	for _, action := range repair.RepairActions {
		entries = append(entries, NarrativeRepairEntry{
			Path: action, Type: "repair_action", SemanticChange: false,
		})
	}
	for _, err := range repair.RepairableErrors {
		entries = append(entries, NarrativeRepairEntry{
			Path: err, Type: "repairable_validation", SemanticChange: false,
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return &NarrativeRepairSummary{
		StatusBeforeRepair: "completed_with_repairable_issues",
		RepairsTotal:       len(entries),
		Repairs:            entries,
		TechnicalRepairs:   len(entries),
	}
}

func (r *NarrativeGenerationResult) IsSuccess() bool {
	if r == nil || r.Skipped {
		return true
	}
	switch r.Status {
	case "completed", "completed_repaired", "completed_with_warnings":
		return !r.UsedLegacyFallback
	case "completed_partial":
		return false
	default:
		return false
	}
}

func (r *NarrativeGenerationResult) DevExitError() error {
	if r == nil || r.IsSuccess() {
		return nil
	}
	msg := strings.TrimSpace(r.ErrorMessage)
	if msg == "" {
		msg = r.Status
	}
	if r.UsedLegacyFallback {
		msg = fmt.Sprintf("legacy fallback used (final_payload_source=%s): %s", r.FinalPayloadSource, msg)
	}
	if len(r.ValidationErrors) > 0 {
		msg = fmt.Sprintf("%s: %s", msg, strings.Join(r.ValidationErrors, ", "))
	}
	return fmt.Errorf("dashboard narrative %s (run_id=%s): %s", r.Status, r.RunID, msg)
}

func narrativeGenerationFailureMessage(parseErr error, out *narrativeOpenAIOutput, hardErrors []string) string {
	if out != nil && out.IncompleteReason == "max_output_tokens" {
		return "openai_output_incomplete_length"
	}
	if parseErr != nil {
		return parseErr.Error()
	}
	if len(hardErrors) > 0 {
		return "validation_failed"
	}
	return "validation_failed"
}

// narrativeOpenAIOutput carries OpenAI response metadata used during generation.
type narrativeOpenAIOutput struct {
	Content          string
	Model            string
	RequestID        string
	InputTokens      int
	OutputTokens     int
	LatencyMs        int64
	ResponseStatus   string
	IncompleteReason string
}
