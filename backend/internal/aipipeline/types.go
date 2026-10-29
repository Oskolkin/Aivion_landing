package aipipeline

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type RunSellerAIPipelineRequest struct {
	SellerAccountID int64
	MetricDate      time.Time

	GenerateDashboard bool
	GenerateActions   bool

	ForceRegenerate bool
	DebugArtifacts  bool

	AIEnabled bool

	ProviderOverrides PipelineProviderOverrides

	// ResolvedProviders is populated by devtools before Run (optional).
	ResolvedProviders PipelineProviders

	Trigger string // admin, devtool, scheduled, sync, test

	// Dispatch-bound execution (post-sync worker path).
	ExistingPipelineRunID int64
	ResumeRequested       bool
	DispatchID            int64
	LeaseGuard            ExecutionLeaseGuard

	// Derived-run stage restart (immutable source run, new pipeline run).
	SourcePipelineRunID   int64
	RestartFromStage      string
	StageRestartRequested bool
}

type RunSellerAIPipelineResult struct {
	PipelineRunID int64 `json:"pipeline_run_id"`

	Status string `json:"status"`

	SellerAccountID int64  `json:"seller_account_id"`
	MetricDate      string `json:"metric_date"`

	DataPackHash string `json:"data_pack_hash"`

	GlobalContextHash   string `json:"global_context_hash,omitempty"`
	GlobalContextStatus string `json:"global_context_status,omitempty"`

	DashboardRunID  string `json:"dashboard_run_id,omitempty"`
	DashboardStatus string `json:"dashboard_status,omitempty"`

	ActionsRunID  int64  `json:"actions_run_id,omitempty"`
	ActionsStatus string `json:"actions_status,omitempty"`

	Providers   *PipelineProviders   `json:"providers,omitempty"`
	Diagnostics *PipelineDiagnostics `json:"diagnostics,omitempty"`
	Warnings    []string             `json:"warnings,omitempty"`

	AIProxy *openaix.ProxySummary `json:"ai_proxy,omitempty"`

	ArtifactRefs map[string]string `json:"artifact_refs,omitempty"`

	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	Reused    bool `json:"reused,omitempty"`
	Published bool `json:"published,omitempty"`

	SourcePipelineRunID int64  `json:"source_pipeline_run_id,omitempty"`
	RestartFromStage    string `json:"restart_from_stage,omitempty"`

	Resume  *ResumeDiagnostics       `json:"resume,omitempty"`
	Restart *StageRestartDiagnostics `json:"restart,omitempty"`
}

type GlobalContextRetryInput struct {
	Input            GlobalContextGenerationInput    `json:"input"`
	DraftContext     aicontext.GlobalSemanticContext `json:"draft_context"`
	ValidationIssues []GlobalContextValidationIssue  `json:"validation_issues"`
}

type GlobalContextRawResponse struct {
	Content                  string    `json:"content,omitempty"`
	Model                    string    `json:"model,omitempty"`
	RequestID                string    `json:"request_id,omitempty"`
	InputTokens              int       `json:"input_tokens,omitempty"`
	OutputTokens             int       `json:"output_tokens,omitempty"`
	LatencyMs                int64     `json:"latency_ms,omitempty"`
	ProviderStatus           string    `json:"provider_status,omitempty"`
	FinishReason             string    `json:"finish_reason,omitempty"`
	IncompleteReason         string    `json:"incomplete_reason,omitempty"`
	OutputTruncated          bool      `json:"output_truncated,omitempty"`
	RequestedMaxOutputTokens int       `json:"requested_max_output_tokens,omitempty"`
	HTTPStatusCode           int       `json:"http_status_code,omitempty"`
	ProviderError            string    `json:"provider_error,omitempty"`
	PromptVersion            string    `json:"prompt_version,omitempty"`
	InputSchemaVersion       string    `json:"input_schema_version,omitempty"`
	ParseStatus              string    `json:"parse_status,omitempty"`
	ParseError               string    `json:"parse_error,omitempty"`
	ParseErrorType           string    `json:"parse_error_type,omitempty"`
	ParseErrorOffset         *int      `json:"parse_error_offset,omitempty"`
	StartedAt                time.Time `json:"started_at,omitempty"`
	FinishedAt               time.Time `json:"finished_at,omitempty"`
}

type GlobalContextValidationResult struct {
	Status string                         `json:"status"`
	Issues []GlobalContextValidationIssue `json:"issues,omitempty"`
}

type GlobalContextValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
}
