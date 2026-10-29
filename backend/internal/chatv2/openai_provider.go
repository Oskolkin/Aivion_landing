package chatv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

type OpenAIProvider struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

func NewOpenAIProvider(apiKey, model, baseURL string, timeout time.Duration) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &OpenAIProvider{APIKey: apiKey, Model: model, BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: timeout}}
}

func (p *OpenAIProvider) Turn(ctx context.Context, req ProviderRequest) (ProviderResponse, error) {
	ctx, span := observability.Tracer(observability.TracerGenAI).Start(ctx, "gen_ai.request")
	defer span.End()
	span.SetAttributes(
		attribute.String("gen_ai.provider.name", "openai"),
		attribute.String("gen_ai.operation.name", "chat.completions"),
		attribute.String("gen_ai.request.model", p.Model),
		attribute.String("gen_ai.request.model_policy_version", ModelPolicyV1),
	)
	start := time.Now()
	if strings.TrimSpace(p.APIKey) == "" {
		return ProviderResponse{}, ErrProviderFailed
	}
	tools := make([]map[string]any, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": t.Key, "description": t.Description,
				"parameters": t.Schema, "strict": true,
			},
		})
	}
	user, _ := json.Marshal(req.Input)
	body := map[string]any{
		"model": p.Model,
		"messages": []map[string]any{
			{"role": "system", "content": req.Instructions},
			{"role": "user", "content": string(user)},
		},
		"tools":       tools,
		"tool_choice": "auto",
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ProviderResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := p.HTTP.Do(httpReq)
	if err != nil {
		span.SetAttributes(attribute.String("error.type", "transport"))
		return ProviderResponse{}, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	latency := time.Since(start).Milliseconds()
	reqID := res.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = res.Header.Get("x-request-id")
	}
	if res.StatusCode >= 400 {
		span.SetAttributes(attribute.Int("http.status_code", res.StatusCode), attribute.String("error.type", "provider_http"))
		return ProviderResponse{ProviderRequestID: reqID, Latency: latency}, fmt.Errorf("%w: openai status %d", ErrProviderFailed, res.StatusCode)
	}
	var parsed struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			PromptTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   string           `json:"content"`
				ToolCalls []openaiToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ProviderResponse{}, err
	}
	if len(parsed.Choices) == 0 {
		return ProviderResponse{ID: parsed.ID, ProviderRequestID: reqID, ProviderResponseID: parsed.ID, Model: parsed.Model, Latency: latency}, ErrProviderFailed
	}
	out, err := decodeOpenAIMessage(parsed.ID, parsed.Choices[0].Message.ToolCalls)
	if err != nil {
		return ProviderResponse{}, err
	}
	out.InputTokens = parsed.Usage.PromptTokens
	out.OutputTokens = parsed.Usage.CompletionTokens
	out.CachedTokens = parsed.Usage.PromptTokensDetails.CachedTokens
	out.Model = parsed.Model
	if out.Model == "" {
		out.Model = p.Model
	}
	out.FinishReason = parsed.Choices[0].FinishReason
	out.ProviderRequestID = reqID
	out.ProviderResponseID = parsed.ID
	out.Latency = latency
	span.SetAttributes(
		attribute.String("gen_ai.response.model", observability.SanitizeTraceValue(out.Model)),
		attribute.Int("gen_ai.usage.input_tokens", out.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", out.OutputTokens),
		attribute.Int("gen_ai.usage.cached_tokens", out.CachedTokens),
		attribute.String("gen_ai.response.finish_reason", out.FinishReason),
		attribute.String("gen_ai.response.request_id", observability.SanitizeTraceValue(reqID)),
		attribute.Int64("gen_ai.response.latency_ms", latency),
	)
	return out, nil
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func decodeOpenAIMessage(id string, toolCalls []openaiToolCall) (ProviderResponse, error) {
	out := ProviderResponse{ID: id}
	var submit *TurnResult
	var submitID string
	for _, tc := range toolCalls {
		args, err := DecodeToolArgumentsJSON(tc.Function.Arguments)
		if err != nil {
			return ProviderResponse{}, err
		}
		if err := ValidateToolArguments(tc.Function.Name, args); err != nil {
			return ProviderResponse{}, err
		}
		if tc.Function.Name == ToolSubmitTurnResult {
			if submit != nil {
				return ProviderResponse{}, fmt.Errorf("%w: multiple submit_turn_result", ErrInvalidToolCall)
			}
			fin, err := turnResultFromArgs(args)
			if err != nil {
				return ProviderResponse{}, err
			}
			submit = &fin
			submitID = tc.ID
			continue
		}
		out.ToolCalls = append(out.ToolCalls, ProviderToolCall{CallID: tc.ID, Name: tc.Function.Name, Args: args})
	}
	if submit != nil && len(out.ToolCalls) > 0 {
		return ProviderResponse{}, fmt.Errorf("%w: mixed submit_turn_result", ErrInvalidToolCall)
	}
	if submit != nil {
		out.Final = submit
		out.FinalSubmitCallID = submitID
		out.ToolCalls = nil
		return out, nil
	}
	return out, nil
}
