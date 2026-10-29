package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

var (
	ErrOpenAIAPIKeyMissing   = errors.New("openai api key is missing")
	ErrOpenAIProvider        = errors.New("openai provider error")
	ErrOpenAIRequestTooLarge = errors.New("openai request exceeds approx input token budget")
)

type AIClient interface {
	PlanTools(ctx context.Context, input PlanToolsInput) (*PlanToolsOutput, error)
	GenerateAnswer(ctx context.Context, input GenerateAnswerInput) (*GenerateAnswerOutput, error)
	StreamAnswer(ctx context.Context, input GenerateAnswerInput, onDelta func(delta string) error) (*GenerateAnswerOutput, error)
}

type OpenAIClientConfig struct {
	APIKey               string
	Model                string
	TimeoutSeconds       int
	MaxRetries           int
	BaseURL              string
	ProxyURL             string // optional HTTP(S) proxy for OpenAI only
	MaxInputTokensApprox int
	MaxOutputTokens      int
}

type OpenAIClient struct {
	httpClient *http.Client
	cfg        OpenAIClientConfig
}

type PlanToolsInput struct {
	SystemPrompt string
	UserPrompt   string
}

type PlanToolsOutput struct {
	Plan          ToolPlan
	Content       string
	RawResponse   json.RawMessage
	Model         string
	RequestID     string
	InputTokens   int32
	OutputTokens  int32
	TotalTokens   int32
	FinishedAtUTC time.Time
}

type GenerateAnswerInput struct {
	SystemPrompt string
	UserPrompt   string
	FactContext  *FactContext
	InputFileIDs []string
	InputImages  []AnswerImageInput
}

type AnswerImageInput struct {
	MimeType string
	Data     []byte
}

type GenerateAnswerOutput struct {
	Answer             ChatAnswer
	Content            string
	RawResponse        json.RawMessage
	Model              string
	RequestID          string
	ProviderResponseID string
	InputTokens        int32
	OutputTokens       int32
	TotalTokens        int32
	FinishedAtUTC      time.Time
}

type openAIResponsesRequest struct {
	Model           string            `json:"model"`
	Input           []openAIInputItem `json:"input"`
	Temperature     float64           `json:"temperature"`
	Text            *openAITextConfig `json:"text,omitempty"`
	MaxOutputTokens int               `json:"max_output_tokens,omitempty"`
	Stream          bool              `json:"stream,omitempty"`
}

type openAIInputItem struct {
	Role    string              `json:"role"`
	Content []openAIContentItem `json:"content"`
}

type openAIContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type openAITextConfig struct {
	Format openAITextFormat `json:"format"`
}

type openAITextFormat struct {
	Type string `json:"type"`
}

type openAIResponsesResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func NewOpenAIClient(cfg OpenAIClientConfig) (*OpenAIClient, error) {
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = openaix.DefaultRequestTimeoutSeconds
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4.1-mini"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	httpClient, err := openaix.BuildHTTPClient(timeout, cfg.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &OpenAIClient{
		httpClient: httpClient,
		cfg:        cfg,
	}, nil
}

func approxInputTokensFromRequest(req openAIResponsesRequest) int {
	raw, err := json.Marshal(req)
	if err != nil {
		return 0
	}
	return (len(raw) + 3) / 4
}

func (c *OpenAIClient) PlanTools(ctx context.Context, input PlanToolsInput) (*PlanToolsOutput, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return nil, ErrOpenAIAPIKeyMissing
	}
	reqBody := openAIResponsesRequest{
		Model: c.cfg.Model,
		Input: []openAIInputItem{
			{Role: "system", Content: []openAIContentItem{{Type: "input_text", Text: strings.TrimSpace(input.SystemPrompt)}}},
			{Role: "user", Content: []openAIContentItem{{Type: "input_text", Text: strings.TrimSpace(input.UserPrompt)}}},
		},
		Temperature: 0.1,
		Text:        &openAITextConfig{Format: openAITextFormat{Type: "json_object"}},
	}
	if c.cfg.MaxOutputTokens > 0 {
		reqBody.MaxOutputTokens = c.cfg.MaxOutputTokens
	}
	if c.cfg.MaxInputTokensApprox > 0 {
		if approx := approxInputTokensFromRequest(reqBody); approx > c.cfg.MaxInputTokensApprox {
			return nil, fmt.Errorf("%w (approx_input_tokens=%d limit=%d)", ErrOpenAIRequestTooLarge, approx, c.cfg.MaxInputTokensApprox)
		}
	}
	respBody, requestID, err := c.doWithRetry(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	var decoded openAIResponsesResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	content := extractOutputText(decoded)
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("planner response is empty")
	}
	var plan ToolPlan
	if err := parseJSONPayload(content, &plan); err != nil {
		return nil, fmt.Errorf("parse planner json: %w", err)
	}
	return &PlanToolsOutput{
		Plan:          plan,
		Content:       content,
		RawResponse:   json.RawMessage(respBody),
		Model:         decoded.Model,
		RequestID:     requestID,
		InputTokens:   int32(decoded.Usage.InputTokens),
		OutputTokens:  int32(decoded.Usage.OutputTokens),
		TotalTokens:   int32(decoded.Usage.TotalTokens),
		FinishedAtUTC: time.Now().UTC(),
	}, nil
}

func (c *OpenAIClient) GenerateAnswer(ctx context.Context, input GenerateAnswerInput) (*GenerateAnswerOutput, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return nil, ErrOpenAIAPIKeyMissing
	}
	userContent := []openAIContentItem{{Type: "input_text", Text: strings.TrimSpace(input.UserPrompt)}}
	for _, fileID := range input.InputFileIDs {
		if strings.TrimSpace(fileID) == "" {
			continue
		}
		userContent = append(userContent, openAIContentItem{Type: "input_file", FileID: strings.TrimSpace(fileID)})
	}
	userContent = append(userContent, imageContentItems(input.InputImages)...)
	reqBody := openAIResponsesRequest{
		Model: c.cfg.Model,
		Input: []openAIInputItem{
			{Role: "system", Content: []openAIContentItem{{Type: "input_text", Text: strings.TrimSpace(input.SystemPrompt)}}},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.2,
		Text:        &openAITextConfig{Format: openAITextFormat{Type: "json_object"}},
	}
	if c.cfg.MaxOutputTokens > 0 {
		reqBody.MaxOutputTokens = c.cfg.MaxOutputTokens
	}
	if c.cfg.MaxInputTokensApprox > 0 {
		if approx := approxInputTokensFromRequest(reqBody); approx > c.cfg.MaxInputTokensApprox {
			return nil, fmt.Errorf("%w (approx_input_tokens=%d limit=%d)", ErrOpenAIRequestTooLarge, approx, c.cfg.MaxInputTokensApprox)
		}
	}
	respBody, requestID, err := c.doWithRetry(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	var decoded openAIResponsesResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	content := extractOutputText(decoded)
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("answerer response is empty")
	}
	var answer ChatAnswer
	if err := parseJSONPayload(content, &answer); err != nil {
		return nil, fmt.Errorf("parse answer json: %w", err)
	}
	return &GenerateAnswerOutput{
		Answer:        answer,
		Content:       content,
		RawResponse:   json.RawMessage(respBody),
		Model:         decoded.Model,
		RequestID:     requestID,
		InputTokens:   int32(decoded.Usage.InputTokens),
		OutputTokens:  int32(decoded.Usage.OutputTokens),
		TotalTokens:   int32(decoded.Usage.TotalTokens),
		FinishedAtUTC: time.Now().UTC(),
	}, nil
}

// StreamAnswer streams plain visible answer text from the Responses API. Metadata
// required by the persisted ChatAnswer is synthesized from the trusted fact context.
func (c *OpenAIClient) StreamAnswer(ctx context.Context, input GenerateAnswerInput, onDelta func(delta string) error) (*GenerateAnswerOutput, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return nil, ErrOpenAIAPIKeyMissing
	}
	if onDelta == nil {
		return nil, errors.New("stream delta callback is required")
	}
	userContent := []openAIContentItem{{Type: "input_text", Text: strings.TrimSpace(input.UserPrompt)}}
	for _, fileID := range input.InputFileIDs {
		if fileID = strings.TrimSpace(fileID); fileID != "" {
			userContent = append(userContent, openAIContentItem{Type: "input_file", FileID: fileID})
		}
	}
	userContent = append(userContent, imageContentItems(input.InputImages)...)
	reqBody := openAIResponsesRequest{
		Model: c.cfg.Model,
		Input: []openAIInputItem{
			{Role: "system", Content: []openAIContentItem{{Type: "input_text", Text: BuildStreamingAnswerSystemPrompt()}}},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.2,
		Stream:      true,
	}
	if c.cfg.MaxOutputTokens > 0 {
		reqBody.MaxOutputTokens = c.cfg.MaxOutputTokens
	}
	if c.cfg.MaxInputTokensApprox > 0 {
		if approx := approxInputTokensFromRequest(reqBody); approx > c.cfg.MaxInputTokensApprox {
			return nil, fmt.Errorf("%w (approx_input_tokens=%d limit=%d)", ErrOpenAIRequestTooLarge, approx, c.cfg.MaxInputTokensApprox)
		}
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal openai stream request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/responses", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build openai stream request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, openaix.WrapIfUnavailable(err)
	}
	defer resp.Body.Close()
	requestID := resp.Header.Get("x-request-id")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		if readErr != nil {
			return nil, fmt.Errorf("read openai stream error: %w", readErr)
		}
		if outage := openaix.WrapHTTPOutage(resp.StatusCode, strings.TrimSpace(string(body))); outage != nil {
			return nil, outage
		}
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrOpenAIProvider, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var text strings.Builder
	var completed openAIResponsesResponse
	var rawCompleted json.RawMessage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Error    json.RawMessage `json:"error"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return nil, fmt.Errorf("decode openai stream event: %w", err)
		}
		switch event.Type {
		case "response.output_text.delta":
			if event.Delta == "" {
				continue
			}
			if err := onDelta(event.Delta); err != nil {
				return nil, err
			}
			text.WriteString(event.Delta)
		case "response.completed":
			rawCompleted = append(rawCompleted[:0], event.Response...)
			if len(event.Response) > 0 {
				if err := json.Unmarshal(event.Response, &completed); err != nil {
					return nil, fmt.Errorf("decode completed response: %w", err)
				}
			}
		case "response.failed", "error":
			if len(event.Error) > 0 {
				return nil, fmt.Errorf("%w: %s", ErrOpenAIProvider, strings.TrimSpace(string(event.Error)))
			}
			return nil, fmt.Errorf("%w: stream event %s", ErrOpenAIProvider, event.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("read openai stream: %w", err)
	}
	content := strings.TrimSpace(text.String())
	if content == "" {
		return nil, errors.New("answerer stream is empty")
	}
	return &GenerateAnswerOutput{
		Answer:             synthesizeStreamAnswer(content, input.FactContext),
		Content:            content,
		RawResponse:        rawCompleted,
		Model:              firstNonEmpty(completed.Model, c.cfg.Model),
		RequestID:          requestID,
		ProviderResponseID: completed.ID,
		InputTokens:        int32(completed.Usage.InputTokens),
		OutputTokens:       int32(completed.Usage.OutputTokens),
		TotalTokens:        int32(completed.Usage.TotalTokens),
		FinishedAtUTC:      time.Now().UTC(),
	}, nil
}

func (c *OpenAIClient) doWithRetry(ctx context.Context, reqBody openAIResponsesRequest) ([]byte, string, error) {
	attempts := c.cfg.MaxRetries + 1
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		body, requestID, err := c.doOnce(ctx, reqBody)
		if err == nil {
			return body, requestID, nil
		}
		lastErr = err
		if !isRetryableOpenAIError(err) || attempt == attempts {
			return nil, "", err
		}
		delay := time.Duration(attempt) * 500 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, "", ctx.Err()
		case <-timer.C:
		}
	}
	return nil, "", lastErr
}

func (c *OpenAIClient) doOnce(ctx context.Context, reqBody openAIResponsesRequest) ([]byte, string, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("marshal openai request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/responses", bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", openaix.WrapIfUnavailable(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("read openai response: %w", err)
	}
	requestID := resp.Header.Get("x-request-id")
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, requestID, nil
	}
	errBody := strings.TrimSpace(string(respBody))
	if errBody == "" {
		errBody = "<empty>"
	}
	if err := openaix.WrapHTTPOutage(resp.StatusCode, errBody); err != nil {
		return nil, requestID, err
	}
	return nil, requestID, fmt.Errorf("%w: status=%d body=%s", ErrOpenAIProvider, resp.StatusCode, errBody)
}

func isRetryableOpenAIError(err error) bool {
	var httpErr *openaix.HTTPOutageError
	if errors.As(err, &httpErr) {
		return openaix.IsOutageStatus(httpErr.StatusCode)
	}
	return openaix.IsTemporarilyUnavailable(err)
}

func extractOutputText(resp openAIResponsesResponse) string {
	var b strings.Builder
	for _, item := range resp.Output {
		for _, c := range item.Content {
			if c.Type == "output_text" || c.Type == "text" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(c.Text)
			}
		}
	}
	return b.String()
}

func imageContentItems(images []AnswerImageInput) []openAIContentItem {
	if len(images) == 0 {
		return nil
	}
	out := make([]openAIContentItem, 0, len(images))
	for _, img := range images {
		if len(img.Data) == 0 {
			continue
		}
		mime := resolveImageMIMEType("", img.MimeType, img.Data)
		out = append(out, openAIContentItem{
			Type:     "input_image",
			ImageURL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img.Data),
		})
	}
	return out
}

// resolveImageMIMEType returns an OpenAI-compatible image MIME type.
// Multipart uploads often declare application/octet-stream; never forward that.
func resolveImageMIMEType(format, declared string, data []byte) string {
	declared = strings.ToLower(strings.TrimSpace(declared))
	switch declared {
	case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif":
		if declared == "image/jpg" {
			return "image/jpeg"
		}
		return declared
	}
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "jpg", "jpeg":
		return "image/jpeg"
	}
	if len(data) >= 12 {
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		if data[0] == 0xFF && data[1] == 0xD8 {
			return "image/jpeg"
		}
		if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return "image/webp"
		}
		if string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a" {
			return "image/gif"
		}
	}
	return "image/png"
}

func parseJSONPayload(content string, dst any) error {
	// Defensive tolerant parsing: prompts and response_format require strict JSON,
	// but provider output may still include wrappers in edge cases.
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	if err := json.Unmarshal([]byte(trimmed), dst); err == nil {
		return nil
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(trimmed[start:end+1]), dst)
	}
	return errors.New("json object not found")
}
