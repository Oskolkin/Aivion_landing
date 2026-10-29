package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// OpenAIGlobalContextClient uses a stage-scoped OpenAI client for global context generation.
type OpenAIGlobalContextClient struct {
	AI             *recommendations.OpenAIClient
	TimeoutSeconds int
}

func (c *OpenAIGlobalContextClient) GenerateGlobalContext(
	ctx context.Context,
	input GlobalContextGenerationInput,
) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error) {
	wire, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		return nil, nil, err
	}
	var wirePayload any
	if err := json.Unmarshal(wire, &wirePayload); err != nil {
		return nil, nil, err
	}
	systemPrompt, userPrompt := globalContextPromptsForInput(input)
	return c.call(ctx, systemPrompt, userPrompt, map[string]any{
		"prompt_version":            globalContextPromptVersionForInput(input),
		"global_context_input_json": wirePayload,
	}, input.SchemaVersion)
}

func (c *OpenAIGlobalContextClient) RetryGlobalContext(
	ctx context.Context,
	input GlobalContextRetryInput,
) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error) {
	return c.call(ctx, globalContextRetrySystemPrompt(), globalContextRetryUserPrompt(), map[string]any{
		"prompt_version":            globalContextPromptVersionForInput(input.Input),
		"global_context_input_json": input.Input,
		"draft_context":             input.DraftContext,
		"validation_issues":         input.ValidationIssues,
	}, input.Input.SchemaVersion)
}

func (c *OpenAIGlobalContextClient) call(
	ctx context.Context,
	systemPrompt, userPrompt string,
	payload any,
	inputSchemaVersion string,
) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error) {
	if c == nil || c.AI == nil {
		return nil, nil, fmt.Errorf("openai client is not configured")
	}
	maxOut := GlobalContextRequestedMaxOutputTokens()
	start := time.Now()
	out, err := c.AI.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt:    systemPrompt,
		UserPrompt:      userPrompt,
		Context:         payload,
		MaxOutputTokens: maxOut,
	})
	finishedAt := time.Now()
	latency := finishedAt.Sub(start).Milliseconds()
	if err != nil {
		return nil, &GlobalContextRawResponse{
			LatencyMs:                latency,
			StartedAt:                start,
			FinishedAt:               finishedAt,
			ProviderError:            err.Error(),
			RequestedMaxOutputTokens: maxOut,
			ParseStatus:              ParseStatusNotCaptured,
		}, err
	}
	raw := buildGlobalContextRawFromProvider(out, maxOut, start, finishedAt, latency, inputSchemaVersion)
	parsed, parseErr := ParseGlobalSemanticContext(out.Content)
	if parseErr != nil {
		applyParseErrorDetails(raw, parseErr)
		return nil, raw, parseErr
	}
	raw.ParseStatus = ParseStatusParsed
	return parsed, raw, nil
}

func buildGlobalContextRawFromProvider(
	out *recommendations.GenerateRecommendationsOutput,
	maxOut int,
	start, finishedAt time.Time,
	latency int64,
	inputSchemaVersion string,
) *GlobalContextRawResponse {
	truncated := out.ResponseStatus == "incomplete" || out.IncompleteReason == "max_output_tokens"
	if maxOut > 0 && out.OutputTokens >= maxOut && out.IncompleteReason == "" {
		truncated = truncated || out.OutputTokens >= maxOut
	}
	return &GlobalContextRawResponse{
		Content:                  out.Content,
		Model:                    out.Model,
		RequestID:                out.RequestID,
		InputTokens:              out.InputTokens,
		OutputTokens:             out.OutputTokens,
		LatencyMs:                latency,
		ProviderStatus:           out.ResponseStatus,
		FinishReason:             out.FinishReason,
		IncompleteReason:         out.IncompleteReason,
		OutputTruncated:          truncated,
		RequestedMaxOutputTokens: maxOut,
		InputSchemaVersion:       inputSchemaVersion,
		StartedAt:                start,
		FinishedAt:               finishedAt,
	}
}

func applyParseErrorDetails(raw *GlobalContextRawResponse, parseErr error) {
	if raw == nil || parseErr == nil {
		return
	}
	raw.ParseStatus = ParseStatusInvalidJSON
	raw.ParseErrorType, raw.ParseError, raw.ParseErrorOffset = extractJSONParseErrorDetails(parseErr)
}
