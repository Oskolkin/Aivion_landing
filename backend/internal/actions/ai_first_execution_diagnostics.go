package actions

// ActionsDetailBatchExecution captures one Object Detail provider call timeline.
type ActionsDetailBatchExecution struct {
	ActionKey      string `json:"action_key,omitempty"`
	BatchIndex     int    `json:"batch_index,omitempty"`
	EntityCount    int    `json:"entity_count,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	ElapsedMs      int64  `json:"elapsed_ms,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Status         string `json:"status,omitempty"`
	ProviderError  string `json:"provider_error,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
	InputTokens    int    `json:"input_tokens,omitempty"`
	OutputTokens   int    `json:"output_tokens,omitempty"`
}

// ActionsTwoPhaseExecutionDiagnostics records compose/detail/stage execution topology.
type ActionsTwoPhaseExecutionDiagnostics struct {
	ComposeTimeoutSeconds    int                           `json:"compose_timeout_seconds,omitempty"`
	ComposeStartedAt         string                        `json:"compose_started_at,omitempty"`
	ComposeFinishedAt        string                        `json:"compose_finished_at,omitempty"`
	ComposeElapsedMs         int64                         `json:"compose_elapsed_ms,omitempty"`
	ComposeProviderStatus    string                        `json:"compose_provider_status,omitempty"`
	DetailTimeoutSeconds     int                           `json:"detail_timeout_seconds,omitempty"`
	PlannedBatches           int                           `json:"planned_batches,omitempty"`
	StartedBatches           int                           `json:"started_batches,omitempty"`
	CompletedBatches         int                           `json:"completed_batches,omitempty"`
	PartialBatches           int                           `json:"partial_batches,omitempty"`
	FailedBatches            int                           `json:"failed_batches,omitempty"`
	TruncatedBatches         int                           `json:"truncated_batches,omitempty"`
	DetailBatches            []ActionsDetailBatchExecution `json:"detail_batches,omitempty"`
	StageStartedAt           string                        `json:"stage_started_at,omitempty"`
	StageFinishedAt          string                        `json:"stage_finished_at,omitempty"`
	StageElapsedMs           int64                         `json:"stage_elapsed_ms,omitempty"`
	ParentContextHasDeadline bool                          `json:"parent_context_has_deadline,omitempty"`
	ParentDeadlineAt         string                        `json:"parent_deadline_at,omitempty"`
	CancelReason             string                        `json:"cancel_reason,omitempty"`
}
