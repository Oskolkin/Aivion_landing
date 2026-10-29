package aipipeline

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// GlobalContextRequestMetrics captures serialized Global Context OpenAI request size.
type GlobalContextRequestMetrics struct {
	InputJSONBytes              int `json:"global_context_input_json_bytes"`
	InputEstimatedTokens        int `json:"global_context_input_estimated_tokens"`
	SystemPromptBytes           int `json:"global_context_system_prompt_bytes"`
	SystemPromptEstimatedTokens int `json:"global_context_system_prompt_estimated_tokens"`
	TaskPromptBytes             int `json:"global_context_task_prompt_bytes"`
	TaskPromptEstimatedTokens   int `json:"global_context_task_prompt_estimated_tokens"`
	SchemaBytes                 int `json:"global_context_schema_bytes"`
	SchemaEstimatedTokens       int `json:"global_context_schema_estimated_tokens"`
	TotalRequestBytes           int `json:"global_context_total_request_bytes"`
	TotalEstimatedInputTokens   int `json:"global_context_total_estimated_input_tokens"`
	MaxInputTokens              int `json:"global_context_max_input_tokens"`
	MaxOutputTokens             int `json:"global_context_max_output_tokens"`
	TimeoutSeconds              int `json:"global_context_timeout_seconds"`
	InputHeadroomTokens         int `json:"global_context_input_headroom_tokens,omitempty"`
}

func estimateGlobalContextRequestMetrics(
	input GlobalContextGenerationInput,
	promptModel string,
	timeoutSeconds int,
	maxInputTokens int,
	maxOutputTokens int,
) (GlobalContextRequestMetrics, error) {
	wire, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		return GlobalContextRequestMetrics{}, err
	}
	systemPrompt, userPrompt := globalContextPromptsForInput(input)
	var wirePayload any
	if err := json.Unmarshal(wire, &wirePayload); err != nil {
		return GlobalContextRequestMetrics{}, err
	}
	contextPayload := map[string]any{
		"prompt_version":            globalContextPromptVersionForInput(input),
		"global_context_input_json": wirePayload,
	}
	estimate, err := recommendations.EstimateStructuredJSONRequest(recommendations.OpenAIClientConfig{
		Model:           promptModel,
		MaxOutputTokens: maxOutputTokens,
	}, recommendations.GenerateStructuredJSONInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Context:      contextPayload,
	})
	if err != nil {
		return GlobalContextRequestMetrics{}, err
	}
	schemaSnippet := `{"type":"json_object"}`
	metrics := GlobalContextRequestMetrics{
		InputJSONBytes:              len(wire),
		InputEstimatedTokens:        aitokens.EstimateBytesTokens(wire),
		SystemPromptBytes:           len(systemPrompt),
		SystemPromptEstimatedTokens: aitokens.EstimateBytesTokens([]byte(systemPrompt)),
		TaskPromptBytes:             len(strings.TrimSpace(userPrompt)),
		TaskPromptEstimatedTokens:   aitokens.EstimateBytesTokens([]byte(strings.TrimSpace(userPrompt))),
		SchemaBytes:                 len(schemaSnippet),
		SchemaEstimatedTokens:       aitokens.EstimateBytesTokens([]byte(schemaSnippet)),
		TotalRequestBytes:           estimate.TotalRequestBytes,
		TotalEstimatedInputTokens:   estimate.TotalEstimatedInputTokens,
		MaxInputTokens:              maxInputTokens,
		MaxOutputTokens:             maxOutputTokens,
		TimeoutSeconds:              timeoutSeconds,
	}
	if maxInputTokens > 0 {
		metrics.InputHeadroomTokens = maxInputTokens - metrics.TotalEstimatedInputTokens
	}
	return metrics, nil
}

func populateGlobalContextRequestDiagnostics(diag *PipelineDiagnostics, metrics GlobalContextRequestMetrics) {
	if diag == nil {
		return
	}
	diag.GlobalContext.InputJSONBytes = metrics.InputJSONBytes
	diag.GlobalContext.InputEstimatedTokens = metrics.InputEstimatedTokens
	diag.GlobalContext.SystemPromptBytes = metrics.SystemPromptBytes
	diag.GlobalContext.SystemPromptEstimatedTokens = metrics.SystemPromptEstimatedTokens
	diag.GlobalContext.TaskPromptBytes = metrics.TaskPromptBytes
	diag.GlobalContext.TaskPromptEstimatedTokens = metrics.TaskPromptEstimatedTokens
	diag.GlobalContext.SchemaBytes = metrics.SchemaBytes
	diag.GlobalContext.SchemaEstimatedTokens = metrics.SchemaEstimatedTokens
	diag.GlobalContext.TotalRequestBytes = metrics.TotalRequestBytes
	diag.GlobalContext.TotalEstimatedInputTokens = metrics.TotalEstimatedInputTokens
	diag.GlobalContext.TimeoutSeconds = metrics.TimeoutSeconds
	diag.GlobalContext.EstimatedInputTokens = metrics.TotalEstimatedInputTokens
	diag.GlobalContext.MaxInputTokens = metrics.MaxInputTokens
	diag.GlobalContext.MaxOutputTokens = metrics.MaxOutputTokens
	diag.GlobalContext.InputHeadroomTokens = metrics.InputHeadroomTokens
}

func globalContextRequestMetricsArtifactMeta(metrics GlobalContextRequestMetrics, inputHash string) map[string]any {
	return map[string]any{
		"global_context_input_hash":                   inputHash,
		"global_context_input_json_bytes":             metrics.InputJSONBytes,
		"global_context_input_estimated_tokens":       metrics.InputEstimatedTokens,
		"global_context_system_prompt_bytes":          metrics.SystemPromptBytes,
		"global_context_task_prompt_bytes":            metrics.TaskPromptBytes,
		"global_context_total_request_bytes":          metrics.TotalRequestBytes,
		"global_context_total_estimated_input_tokens": metrics.TotalEstimatedInputTokens,
		"global_context_timeout_seconds":              metrics.TimeoutSeconds,
		"global_context_max_input_tokens":             metrics.MaxInputTokens,
		"global_context_max_output_tokens":            metrics.MaxOutputTokens,
	}
}
