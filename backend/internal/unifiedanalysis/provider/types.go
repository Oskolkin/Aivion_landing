package provider

import (
	"encoding/json"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// ProviderResponseStatus is the lifecycle status of a background Response.
type ProviderResponseStatus string

const (
	ProviderResponseStatusQueued     ProviderResponseStatus = "queued"
	ProviderResponseStatusInProgress ProviderResponseStatus = "in_progress"
	ProviderResponseStatusCompleted  ProviderResponseStatus = "completed"
	ProviderResponseStatusFailed     ProviderResponseStatus = "failed"
	ProviderResponseStatusIncomplete ProviderResponseStatus = "incomplete"
	ProviderResponseStatusCancelled  ProviderResponseStatus = "cancelled"
)

// ClientConfig configures the typed OpenAI REST client.
type ClientConfig struct {
	APIKey         string
	BaseURL        string // default https://api.openai.com/v1
	ProxyURL       string
	TimeoutSeconds int
	MaxRetries     int // for safe retrieve/delete only
}

// CreateContainerRequest creates an explicit Code Interpreter container.
type CreateContainerRequest struct {
	Name        string
	MemoryLimit string
}

// Container is an OpenAI Code Interpreter container.
type Container struct {
	ID           string
	Name         string
	Status       string
	MemoryLimit  string
	CreatedAt    time.Time
	LastActiveAt *time.Time
	ExpiresAt    *time.Time
}

// UploadContainerFileRequest uploads a raw file into a container.
type UploadContainerFileRequest struct {
	ContainerID string
	Filename    string
	ContentType string
	Content     []byte
	SHA256      string
}

// ContainerFile is a file stored inside a container.
type ContainerFile struct {
	ID          string
	ContainerID string
	Path        string
	Bytes       int64
	Source      string
	CreatedAt   time.Time
}

// StartBackgroundResponseRequest starts a background Response with Code Interpreter.
type StartBackgroundResponseRequest struct {
	Model       string
	ContainerID string

	Instructions string
	Input        string

	// TextFormat enables OpenAI Structured Outputs (json_schema). Nil for smoke profile.
	TextFormat *StructuredOutputFormat

	// ToolChoice overrides the Responses tool_choice. Empty defaults to "required".
	// Analysis profile with Structured Outputs should use "auto" so the model can
	// emit the final json_schema message after Code Interpreter work.
	ToolChoice string

	MaxOutputTokens int
	Metadata        map[string]string
}

// StructuredOutputFormat is the typed text.format payload for Responses API.
type StructuredOutputFormat struct {
	Type        string          // "json_schema"
	Name        string          // schema name, e.g. aivion_unified_analysis_v1
	Description string          // optional
	Strict      bool            // true for production
	Schema      json.RawMessage // exact JSON Schema document
}

// ProviderResponse is the normalized Aivion view of an OpenAI Response.
type ProviderResponse struct {
	ID        string
	Status    ProviderResponseStatus
	Model     string
	CreatedAt time.Time

	// RequestID is the provider transport request id (x-request-id header) of
	// the call that produced this response. It is populated by the typed client
	// for start/retrieve/cancel and is useful for provider-side diagnostics.
	RequestID string

	CompletedAt *time.Time

	OutputText string
	RawJSON    []byte

	Refusal               string
	AssistantMessageCount int

	// OutputItemTypes lists every output item "type" value in payload order
	// (e.g. ["code_interpreter_call","message"]). It lets callers detect
	// tool-only completed responses (CI present, no assistant message).
	OutputItemTypes []string

	Usage ProviderUsage

	CodeInterpreterCalls []CodeInterpreterCall

	Error             *ProviderResponseError
	IncompleteDetails *IncompleteDetails
}

// ProviderUsage holds token usage from the provider.
type ProviderUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// CodeInterpreterCall records a Code Interpreter tool invocation.
type CodeInterpreterCall struct {
	ID          string
	ContainerID string
	Status      string
	Code        string
}

// ProviderResponseError is a provider-side response error payload.
type ProviderResponseError struct {
	Code    string
	Message string
}

// IncompleteDetails explains why a response ended incomplete.
type IncompleteDetails struct {
	Reason string
}
