package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type openAIResponseCreateBody struct {
	Model           string                    `json:"model"`
	Background      bool                      `json:"background"`
	Tools           []openAITool              `json:"tools"`
	ToolChoice      string                    `json:"tool_choice"`
	Instructions    string                    `json:"instructions"`
	Input           []openAIResponseInputItem `json:"input"`
	MaxOutputTokens int                       `json:"max_output_tokens,omitempty"`
	Metadata        map[string]string         `json:"metadata,omitempty"`
	Text            *openAITextFormatWrapper  `json:"text,omitempty"`
}

type openAITextFormatWrapper struct {
	Format openAITextFormat `json:"format"`
}

type openAITextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type openAITool struct {
	Type      string `json:"type"`
	Container string `json:"container,omitempty"`
}

type openAIResponseInputItem struct {
	Role    string                      `json:"role"`
	Content []openAIResponseContentItem `json:"content"`
}

type openAIResponseContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// StartBackgroundResponse creates a background Response with required Code Interpreter.
func (c *OpenAIClient) StartBackgroundResponse(ctx context.Context, req StartBackgroundResponseRequest) (*ProviderResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, newProviderError(CodeProviderResponseStartFailed, "start response: empty model", 0, "", nil)
	}
	containerID := strings.TrimSpace(req.ContainerID)
	if containerID == "" {
		return nil, newProviderError(CodeProviderResponseStartFailed, "start response: empty container id", 0, "", nil)
	}

	body := openAIResponseCreateBody{
		Model:      model,
		Background: true,
		Tools: []openAITool{
			{
				Type:      "code_interpreter",
				Container: containerID,
			},
		},
		ToolChoice:      firstNonEmpty(strings.TrimSpace(req.ToolChoice), "required"),
		Instructions:    req.Instructions,
		MaxOutputTokens: req.MaxOutputTokens,
		Metadata:        req.Metadata,
		Input: []openAIResponseInputItem{
			{
				Role: "user",
				Content: []openAIResponseContentItem{
					{
						Type: "input_text",
						Text: req.Input,
					},
				},
			},
		},
	}
	if req.TextFormat != nil {
		schema := req.TextFormat.Schema
		if len(schema) == 0 {
			return nil, newProviderError(CodeProviderResponseStartFailed, "start response: empty structured output schema", 0, "", nil)
		}
		name := strings.TrimSpace(req.TextFormat.Name)
		if name == "" {
			return nil, newProviderError(CodeProviderResponseStartFailed, "start response: empty structured output schema name", 0, "", nil)
		}
		formatType := strings.TrimSpace(req.TextFormat.Type)
		if formatType == "" {
			formatType = "json_schema"
		}
		body.Text = &openAITextFormatWrapper{
			Format: openAITextFormat{
				Type:   formatType,
				Name:   name,
				Strict: req.TextFormat.Strict,
				Schema: schema,
			},
		}
	}

	respBody, requestID, err := c.doJSON(ctx, doJSONOptions{
		method:    "POST",
		path:      "/responses",
		body:      body,
		errorCode: CodeProviderResponseStartFailed,
		operation: "start background response",
	})
	if err != nil {
		return nil, err
	}
	parsed, err := ParseResponseJSON(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse start response: %w", err)
	}
	parsed.RequestID = requestID
	return parsed, nil
}

// RetrieveResponse fetches a background Response by ID.
func (c *OpenAIClient) RetrieveResponse(ctx context.Context, responseID string) (*ProviderResponse, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil, newProviderError(CodeProviderResponseRetrieveFailed, "retrieve response: empty id", 0, "", nil)
	}
	respBody, requestID, err := c.doJSON(ctx, doJSONOptions{
		method:    "GET",
		path:      "/responses/" + responseID,
		retrySafe: true,
		errorCode: CodeProviderResponseRetrieveFailed,
		operation: "retrieve response",
	})
	if err != nil {
		return nil, err
	}
	parsed, err := ParseResponseJSON(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse retrieve response: %w", err)
	}
	parsed.RequestID = requestID
	return parsed, nil
}

// CancelResponse cancels a background Response.
func (c *OpenAIClient) CancelResponse(ctx context.Context, responseID string) (*ProviderResponse, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil, newProviderError(CodeProviderResponseCancelFailed, "cancel response: empty id", 0, "", nil)
	}
	respBody, requestID, err := c.doJSON(ctx, doJSONOptions{
		method:    "POST",
		path:      "/responses/" + responseID + "/cancel",
		retrySafe: true,
		errorCode: CodeProviderResponseCancelFailed,
		operation: "cancel response",
	})
	if err != nil {
		return nil, err
	}
	parsed, err := ParseResponseJSON(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse cancel response: %w", err)
	}
	parsed.RequestID = requestID
	return parsed, nil
}
