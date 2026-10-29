package openaix

import "encoding/json"

type ResponsesRequest struct {
	Model              string           `json:"model"`
	Instructions       string           `json:"instructions"`
	Input              string           `json:"input"`
	Tools              []any            `json:"tools"`
	Text               TextFormat       `json:"text"`
	MaxOutputTokens    int              `json:"max_output_tokens"`
	Truncation         string           `json:"truncation"`
	Store              bool             `json:"store"`
	PreviousResponseID *string          `json:"previous_response_id,omitempty"`
	Reasoning          *ReasoningConfig `json:"reasoning,omitempty"`
}

type ReasoningConfig struct {
	Effort string `json:"effort,omitempty"`
}

type TextFormat struct {
	Format    JSONSchemaFormat `json:"format"`
	Verbosity string           `json:"verbosity,omitempty"`
}

type JSONSchemaFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type ResponsesResult struct {
	ID               string
	Status           string
	OutputText       string
	Refusal          string
	IncompleteReason string
	RequestID        string
	Usage            Usage
	Raw              []byte
}

type Usage struct {
	InputTokens       int64
	OutputTokens      int64
	ReasoningTokens   int64
	CachedInputTokens int64
	TotalTokens       int64
}

type providerErrorBody struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *struct {
		InputTokens        int64 `json:"input_tokens"`
		OutputTokens       int64 `json:"output_tokens"`
		TotalTokens        int64 `json:"total_tokens"`
		InputTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		OutputTokensDetails *struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
	Output json.RawMessage `json:"output"`
}

type outputItem struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Role    string `json:"role"`
	Content []struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		Refusal    string `json:"refusal"`
		OutputText string `json:"output_text"`
	} `json:"content"`
}
