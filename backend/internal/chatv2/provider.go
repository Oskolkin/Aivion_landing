package chatv2

import "context"

type ProviderRequest struct {
	Instructions string
	Input        []map[string]any
	Tools        []ToolSpec
	PreviousID   string
}

type ProviderToolCall struct {
	CallID string
	Name   string
	Args   map[string]any
}

type ProviderResponse struct {
	ID                 string
	ToolCalls          []ProviderToolCall
	Final              *TurnResult
	Text               string
	InputTokens        int
	OutputTokens       int
	CachedTokens       int
	Model              string
	FinishReason       string
	ProviderRequestID  string
	ProviderResponseID string
	Latency            int64
	FinalSubmitCallID  string
}

type ChatAgentProvider interface {
	Turn(ctx context.Context, req ProviderRequest) (ProviderResponse, error)
}
