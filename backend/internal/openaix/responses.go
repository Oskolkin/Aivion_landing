package openaix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultResponsesBaseURL = "https://api.openai.com/v1"

type ResponsesClient struct {
	HTTP    *http.Client
	APIKey  string
	BaseURL string
}

func NewResponsesClient(httpClient *http.Client, apiKey, baseURL string) *ResponsesClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultResponsesBaseURL
	}
	return &ResponsesClient{HTTP: httpClient, APIKey: apiKey, BaseURL: strings.TrimRight(baseURL, "/")}
}

func (c *ResponsesClient) Create(ctx context.Context, req ResponsesRequest) (ResponsesResult, error) {
	if c == nil || c.HTTP == nil {
		return ResponsesResult{}, fmt.Errorf("openaix: client not configured")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return ResponsesResult{}, fmt.Errorf("openaix: api key missing")
	}
	if req.Tools == nil {
		req.Tools = []any{}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return ResponsesResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return ResponsesResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return ResponsesResult{}, WrapIfUnavailable(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ResponsesResult{}, WrapIfUnavailable(err)
	}
	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = resp.Header.Get("x-request-id")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		redacted := RedactSecrets(string(raw))
		return ResponsesResult{}, WrapHTTPOutage(resp.StatusCode, redacted)
	}
	parsed, err := ParseResponsesJSON(raw)
	if err != nil {
		return ResponsesResult{}, err
	}
	parsed.RequestID = reqID
	parsed.Raw = raw
	return parsed, nil
}

func RedactSecrets(s string) string {
	s = strings.ReplaceAll(s, "Bearer ", "Bearer [REDACTED]")
	if i := strings.Index(s, "sk-"); i >= 0 {
		end := i + 3
		for end < len(s) && ((s[end] >= 'a' && s[end] <= 'z') || (s[end] >= 'A' && s[end] <= 'Z') || (s[end] >= '0' && s[end] <= '9') || s[end] == '-' || s[end] == '_') {
			end++
		}
		s = s[:i] + "sk-[REDACTED]" + s[end:]
	}
	return s
}
