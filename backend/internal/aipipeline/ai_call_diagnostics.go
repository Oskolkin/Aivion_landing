package aipipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

const (
	AICallRawResponseSchemaVersion = "ai_call_raw_response_v2"
	AICallDiagnosticsSchemaVersion = "ai_call_diagnostics_v1"

	ArtifactContentMetadataOnly = "metadata_only"
	ArtifactContentFullBody     = "full_body"

	ParseStatusParsed      = "parsed"
	ParseStatusInvalidJSON = "invalid_json"
	ParseStatusPartialJSON = "partial_json"
	ParseStatusEmpty       = "empty"
	ParseStatusNotCaptured = "not_captured"
	ParseStatusUnknown     = "unknown"

	DiagnosticStatusAvailable          = "available"
	DiagnosticStatusPartial            = "partial"
	DiagnosticStatusNotCaptured        = "not_captured"
	DiagnosticStatusNotApplicable      = "not_applicable"
	DiagnosticStatusNotRequested       = "not_requested"
	DiagnosticStatusMalformed          = "malformed"
	DiagnosticStatusNotReconstructable = "not_reconstructable"

	TruncationConfirmed  = "output_truncated_confirmed"
	TruncationSuspected  = "output_truncation_suspected"
	TruncationNotConfirm = "truncation_cause_not_confirmable"
)

// AICallDiagnosticEnvelope is the unified per-call diagnostics contract.
type AICallDiagnosticEnvelope struct {
	SchemaVersion            string    `json:"schema_version"`
	PipelineRunID            int64     `json:"pipeline_run_id,omitempty"`
	Stage                    string    `json:"stage"`
	Component                string    `json:"component"`
	ChunkID                  string    `json:"chunk_id,omitempty"`
	ActionKey                string    `json:"action_key,omitempty"`
	BatchIndex               *int      `json:"batch_index,omitempty"`
	Provider                 string    `json:"provider"`
	Model                    string    `json:"model"`
	PromptVersion            string    `json:"prompt_version,omitempty"`
	InputSchemaVersion       string    `json:"input_schema_version,omitempty"`
	OutputSchemaVersion      string    `json:"output_schema_version,omitempty"`
	RequestedMaxOutputTokens int       `json:"requested_max_output_tokens"`
	TimeoutSeconds           int       `json:"timeout_seconds,omitempty"`
	RequestID                string    `json:"request_id,omitempty"`
	StartedAt                time.Time `json:"started_at,omitempty"`
	FinishedAt               time.Time `json:"finished_at,omitempty"`
	LatencyMS                int64     `json:"latency_ms"`
	InputTokens              int       `json:"input_tokens"`
	OutputTokens             int       `json:"output_tokens"`
	ProviderStatus           string    `json:"provider_status,omitempty"`
	FinishReason             string    `json:"finish_reason,omitempty"`
	IncompleteReason         string    `json:"incomplete_reason,omitempty"`
	OutputTruncated          bool      `json:"output_truncated"`
	OutputTruncationClass    string    `json:"output_truncation_class,omitempty"`
	ParentCancelReason       string    `json:"parent_cancel_reason,omitempty"`
	HTTPStatusCode           int       `json:"http_status_code,omitempty"`
	ProviderError            string    `json:"provider_error,omitempty"`
	RawResponseCaptured      bool      `json:"raw_response_captured"`
	RawResponseHash          string    `json:"raw_response_hash,omitempty"`
	RawBodyBytes             int       `json:"raw_body_bytes,omitempty"`
	ParseStatus              string    `json:"parse_status"`
	ParseError               string    `json:"parse_error,omitempty"`
	ParseErrorType           string    `json:"parse_error_type,omitempty"`
	ParseErrorOffset         *int      `json:"parse_error_offset,omitempty"`
	RawBodyHead              string    `json:"raw_body_head,omitempty"`
	RawBodyTail              string    `json:"raw_body_tail,omitempty"`
	ValidationStatus         string    `json:"validation_status,omitempty"`
}

// AICallRawResponseArtifact stores provider body before parsing.
type AICallRawResponseArtifact struct {
	SchemaVersion                  string                   `json:"schema_version"`
	Call                           AICallDiagnosticEnvelope `json:"call"`
	Body                           string                   `json:"body,omitempty"`
	BodyHash                       string                   `json:"body_hash,omitempty"`
	BodyBytes                      int                      `json:"body_bytes"`
	BodyTruncatedByArtifactStorage bool                     `json:"body_truncated_by_artifact_storage"`
	ContentType                    string                   `json:"content_type,omitempty"`
}

// DiagnosticCompletenessEntry describes availability of one diagnostic dimension.
type DiagnosticCompletenessEntry struct {
	Dimension  string `json:"dimension"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	ArtifactID int64  `json:"artifact_id,omitempty"`
}

// BuildActionsLifecycleArtifact packages lifecycle diagnostics for persistence.
func BuildActionsLifecycleArtifact(
	summary actions.ActionsLifecycleSummary,
	lifecycles []actions.ActionLifecycleDiagnostic,
) map[string]any {
	return map[string]any{
		"schema_version": ActionsLifecycleDiagnosticsSchemaVersion,
		"summary":        summary,
		"actions":        lifecycles,
	}
}

func hashBody(body string) string {
	if body == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func bodyPreviewHeadTail(body string, maxLen int) (head, tail string) {
	if maxLen <= 0 {
		maxLen = 512
	}
	if len(body) <= maxLen*2 {
		return body, body
	}
	return body[:maxLen], body[len(body)-maxLen:]
}

// ClassifyOutputTruncation determines truncation classification from call metadata.
func ClassifyOutputTruncation(env AICallDiagnosticEnvelope) string {
	if !env.RawResponseCaptured {
		return TruncationNotConfirm
	}
	if env.ProviderStatus == "incomplete" && env.IncompleteReason == "max_output_tokens" {
		return TruncationConfirmed
	}
	if env.OutputTokens > 0 && env.RequestedMaxOutputTokens > 0 &&
		env.OutputTokens >= env.RequestedMaxOutputTokens && env.IncompleteReason == "" {
		return TruncationSuspected
	}
	return ""
}

// ClassifyLegacyRawOutputArtifact inspects legacy actions_raw_output payloads.
func ClassifyLegacyRawOutputArtifact(doc map[string]any) (contentType string, rawBodyAvailable bool) {
	if doc == nil {
		return ArtifactContentMetadataOnly, false
	}
	for _, key := range []string{"body", "raw_text", "content", "response_json"} {
		if v, ok := doc[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return ArtifactContentFullBody, true
				}
			case map[string]any:
				if len(t) > 0 {
					return ArtifactContentFullBody, true
				}
			case []any:
				if len(t) > 0 {
					return ArtifactContentFullBody, true
				}
			}
		}
	}
	if actions, ok := doc["actions"].([]any); ok && len(actions) > 0 {
		return ArtifactContentFullBody, true
	}
	if parsed, ok := doc["parsed_output"].(map[string]any); ok && len(parsed) > 0 {
		return ArtifactContentFullBody, true
	}
	return ArtifactContentMetadataOnly, false
}

// BuildAICallDiagnosticEnvelopeFromRaw maps actions.AIRawResponse to envelope fields.
func BuildAICallDiagnosticEnvelopeFromRaw(
	stage, component, provider string,
	raw *actions.AIRawResponse,
	requestedMaxTokens, timeoutSeconds int,
	startedAt, finishedAt time.Time,
	parseStatus, parseError string,
) AICallDiagnosticEnvelope {
	env := AICallDiagnosticEnvelope{
		SchemaVersion:            AICallDiagnosticsSchemaVersion,
		Stage:                    stage,
		Component:                component,
		Provider:                 provider,
		RequestedMaxOutputTokens: requestedMaxTokens,
		TimeoutSeconds:           timeoutSeconds,
		StartedAt:                startedAt,
		FinishedAt:               finishedAt,
		ParseStatus:              parseStatus,
		ParseError:               parseError,
	}
	if raw == nil {
		env.ParseStatus = ParseStatusNotCaptured
		return env
	}
	env.Model = raw.Model
	env.RequestID = raw.RequestID
	env.LatencyMS = raw.LatencyMs
	env.InputTokens = raw.InputTokens
	env.OutputTokens = raw.OutputTokens
	env.ProviderStatus = raw.ProviderStatus
	env.FinishReason = raw.FinishReason
	env.IncompleteReason = raw.IncompleteReason
	env.HTTPStatusCode = raw.HTTPStatusCode
	env.ProviderError = raw.ProviderError
	env.PromptVersion = raw.PromptVersion
	env.InputSchemaVersion = raw.InputSchemaVersion
	env.OutputSchemaVersion = raw.OutputSchemaVersion
	body := raw.RawText
	if body == "" && len(raw.ResponseJSON) > 0 {
		body = string(raw.ResponseJSON)
	}
	if body != "" {
		env.RawResponseCaptured = true
		env.RawResponseHash = hashBody(body)
		env.RawBodyBytes = len(body)
		head, tail := bodyPreviewHeadTail(body, 256)
		env.RawBodyHead = head
		env.RawBodyTail = tail
	}
	env.OutputTruncated = raw.OutputTruncated
	if env.OutputTruncated || env.IncompleteReason == "max_output_tokens" {
		env.OutputTruncationClass = ClassifyOutputTruncation(env)
	}
	if parseStatus == "" && body != "" {
		env.ParseStatus = ParseStatusUnknown
	}
	return env
}

// BuildAICallRawResponseArtifact creates the raw response artifact payload.
func BuildAICallRawResponseArtifact(env AICallDiagnosticEnvelope, body string) map[string]any {
	truncated := false
	if env.RawBodyBytes > 0 && len(body) < env.RawBodyBytes {
		truncated = true
	}
	contentType := ArtifactContentFullBody
	if strings.TrimSpace(body) == "" {
		contentType = ArtifactContentMetadataOnly
	}
	return map[string]any{
		"schema_version":                     AICallRawResponseSchemaVersion,
		"call":                               env,
		"body":                               body,
		"body_hash":                          hashBody(body),
		"body_bytes":                         len(body),
		"body_truncated_by_artifact_storage": truncated,
		"content_type":                       contentType,
	}
}

// BuildActionsComposeParsedOutputArtifact stores immutable parsed draft before normalization.
func BuildActionsComposeParsedOutputArtifact(compose *actions.ActionsAIFirstComposeOutput) map[string]any {
	if compose == nil {
		return nil
	}
	raw, _ := json.Marshal(compose)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	return map[string]any{
		"schema_version": "actions_compose_parsed_output_v1",
		"parsed_output":  doc,
		"action_count":   len(compose.Actions),
	}
}

// BuildActionsLifecycleArtifactFromResult packages lifecycle from GenerateActionsResult.
func BuildActionsLifecycleArtifactFromResult(res *actions.GenerateActionsResult) map[string]any {
	if res == nil || len(res.ActionLifecycles) == 0 {
		return nil
	}
	return BuildActionsLifecycleArtifact(res.LifecycleSummary, res.ActionLifecycles)
}

func extractJSONParseErrorDetails(err error) (parseErrorType, parseErrorMessage string, offset *int) {
	if err == nil {
		return "", "", nil
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		off := int(syntaxErr.Offset)
		return "json_syntax_error", syntaxErr.Error(), &off
	}
	return "parse_error", err.Error(), nil
}

// BuildAICallDiagnosticEnvelopeFromGlobalContextRaw maps GlobalContextRawResponse to envelope fields.
func BuildAICallDiagnosticEnvelopeFromGlobalContextRaw(
	provider string,
	raw *GlobalContextRawResponse,
	timeoutSeconds int,
) AICallDiagnosticEnvelope {
	if raw == nil {
		return AICallDiagnosticEnvelope{
			SchemaVersion: AICallDiagnosticsSchemaVersion,
			Stage:         "global_context",
			Component:     AIUsageComponentGlobalContext,
			Provider:      provider,
			ParseStatus:   ParseStatusNotCaptured,
		}
	}
	parseStatus := raw.ParseStatus
	if parseStatus == "" && raw.Content != "" {
		parseStatus = ParseStatusUnknown
	}
	env := AICallDiagnosticEnvelope{
		SchemaVersion:            AICallDiagnosticsSchemaVersion,
		Stage:                    "global_context",
		Component:                AIUsageComponentGlobalContext,
		Provider:                 provider,
		Model:                    raw.Model,
		PromptVersion:            raw.PromptVersion,
		InputSchemaVersion:       raw.InputSchemaVersion,
		OutputSchemaVersion:      aicontext.GlobalSemanticContextSchemaVersion,
		RequestedMaxOutputTokens: raw.RequestedMaxOutputTokens,
		TimeoutSeconds:           timeoutSeconds,
		RequestID:                raw.RequestID,
		StartedAt:                raw.StartedAt,
		FinishedAt:               raw.FinishedAt,
		LatencyMS:                raw.LatencyMs,
		InputTokens:              raw.InputTokens,
		OutputTokens:             raw.OutputTokens,
		ProviderStatus:           raw.ProviderStatus,
		FinishReason:             raw.FinishReason,
		IncompleteReason:         raw.IncompleteReason,
		OutputTruncated:          raw.OutputTruncated,
		HTTPStatusCode:           raw.HTTPStatusCode,
		ProviderError:            raw.ProviderError,
		ParseStatus:              parseStatus,
		ParseError:               raw.ParseError,
		ParseErrorType:           raw.ParseErrorType,
		ParseErrorOffset:         raw.ParseErrorOffset,
	}
	body := raw.Content
	if body != "" {
		env.RawResponseCaptured = true
		env.RawResponseHash = hashBody(body)
		env.RawBodyBytes = len(body)
		head, tail := bodyPreviewHeadTail(body, 256)
		env.RawBodyHead = head
		env.RawBodyTail = tail
	}
	env.OutputTruncationClass = ClassifyOutputTruncation(env)
	return env
}

// BuildGlobalContextParsedOutputArtifact stores parsed global context before validation.
func BuildGlobalContextParsedOutputArtifact(ctx *aicontext.GlobalSemanticContext) map[string]any {
	if ctx == nil {
		return nil
	}
	return map[string]any{
		"schema_version": GlobalContextParsedOutputSchemaVersion,
		"context":        ctx,
	}
}

// BuildDashboardParsedOutputArtifact stores parsed dashboard payload.
func BuildDashboardParsedOutputArtifact(payload any) map[string]any {
	if payload == nil {
		return nil
	}
	return map[string]any{
		"schema_version": DashboardParsedOutputSchemaVersion,
		"payload":        payload,
	}
}
