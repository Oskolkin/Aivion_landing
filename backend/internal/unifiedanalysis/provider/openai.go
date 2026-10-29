package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

// OpenAIClient implements Provider against the OpenAI REST API.
type OpenAIClient struct {
	httpClient *http.Client
	cfg        ClientConfig
}

// NewOpenAIClient builds a typed OpenAI provider client.
func NewOpenAIClient(cfg ClientConfig) (*OpenAIClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, newProviderError(CodeProviderAPIKeyMissing, "openai api key is missing", 0, "", nil)
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = openaix.DefaultRequestTimeoutSeconds
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	httpClient, err := openaix.BuildHTTPClient(timeout, cfg.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("build openai http client: %w", err)
	}
	return &OpenAIClient{
		httpClient: httpClient,
		cfg:        cfg,
	}, nil
}

type doJSONOptions struct {
	method    string
	path      string
	body      any
	retrySafe bool
	errorCode string
	operation string
}

func (c *OpenAIClient) baseURL() string {
	return strings.TrimRight(c.cfg.BaseURL, "/")
}

func (c *OpenAIClient) doJSON(ctx context.Context, opts doJSONOptions) ([]byte, string, error) {
	attempts := 1
	if opts.retrySafe {
		attempts = c.cfg.MaxRetries + 1
		if attempts <= 0 {
			attempts = 1
		}
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		body, requestID, err := c.doJSONOnce(ctx, opts)
		if err == nil {
			return body, requestID, nil
		}
		lastErr = err
		if !opts.retrySafe || attempt == attempts || !isRetryableProviderError(err) {
			return nil, requestID, err
		}
		delay := time.Duration(attempt) * 500 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, requestID, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, "", lastErr
}

func (c *OpenAIClient) doJSONOnce(ctx context.Context, opts doJSONOptions) ([]byte, string, error) {
	var reader io.Reader
	if opts.body != nil {
		payload, err := json.Marshal(opts.body)
		if err != nil {
			return nil, "", fmt.Errorf("marshal %s request: %w", opts.operation, err)
		}
		reader = bytes.NewReader(payload)
	}
	url := c.baseURL() + opts.path
	req, err := http.NewRequestWithContext(ctx, opts.method, url, reader)
	if err != nil {
		return nil, "", fmt.Errorf("build %s request: %w", opts.operation, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	if opts.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", openaix.WrapIfUnavailable(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("read %s response: %w", opts.operation, err)
	}
	requestID := strings.TrimSpace(resp.Header.Get("x-request-id"))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, requestID, nil
	}

	errBody := strings.TrimSpace(string(respBody))
	if errBody == "" {
		errBody = "<empty>"
	}
	if err := openaix.WrapHTTPOutage(resp.StatusCode, errBody); err != nil {
		return nil, requestID, wrapOutageError(opts.errorCode, opts.operation, resp.StatusCode, requestID, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, requestID, notFoundError(opts.errorCode, formatHTTPMessage(opts.operation, resp.StatusCode, errBody), requestID)
	}
	return nil, requestID, newProviderError(
		opts.errorCode,
		formatHTTPMessage(opts.operation, resp.StatusCode, errBody),
		resp.StatusCode,
		requestID,
		nil,
	)
}

func (c *OpenAIClient) doRaw(ctx context.Context, method, path string, contentType string, body io.Reader, errorCode, operation string, retrySafe bool) ([]byte, string, error) {
	opts := doJSONOptions{
		method:    method,
		path:      path,
		retrySafe: retrySafe,
		errorCode: errorCode,
		operation: operation,
	}
	if body == nil {
		return c.doJSON(ctx, opts)
	}

	attempts := 1
	if retrySafe {
		attempts = c.cfg.MaxRetries + 1
		if attempts <= 0 {
			attempts = 1
		}
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		reqBody, err := io.ReadAll(body)
		if err != nil {
			return nil, "", fmt.Errorf("read %s request body: %w", operation, err)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, bytes.NewReader(reqBody))
		if err != nil {
			return nil, "", fmt.Errorf("build %s request: %w", operation, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			err = openaix.WrapIfUnavailable(err)
			lastErr = err
			if !retrySafe || attempt == attempts || !isRetryableProviderError(err) {
				return nil, "", err
			}
			continue
		}
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("read %s response: %w", operation, readErr)
		}
		requestID := strings.TrimSpace(resp.Header.Get("x-request-id"))
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, requestID, nil
		}
		errBody := strings.TrimSpace(string(respBody))
		if errBody == "" {
			errBody = "<empty>"
		}
		if err := openaix.WrapHTTPOutage(resp.StatusCode, errBody); err != nil {
			lastErr = wrapOutageError(errorCode, operation, resp.StatusCode, requestID, err)
		} else if resp.StatusCode == http.StatusNotFound {
			lastErr = notFoundError(errorCode, formatHTTPMessage(operation, resp.StatusCode, errBody), requestID)
		} else {
			lastErr = newProviderError(errorCode, formatHTTPMessage(operation, resp.StatusCode, errBody), resp.StatusCode, requestID, nil)
		}
		if !retrySafe || attempt == attempts || !isRetryableProviderError(lastErr) {
			return nil, requestID, lastErr
		}
		body = bytes.NewReader(reqBody)
	}
	return nil, "", lastErr
}

func wrapOutageError(code, operation string, status int, requestID string, err error) *Error {
	return newProviderError(code, formatHTTPMessage(operation, status, err.Error()), status, requestID, err)
}

func isRetryableProviderError(err error) bool {
	if openaix.IsTemporarilyUnavailable(err) {
		return true
	}
	var httpErr *openaix.HTTPOutageError
	if errors.As(err, &httpErr) {
		return openaix.IsOutageStatus(httpErr.StatusCode)
	}
	var pe *Error
	if errors.As(err, &pe) {
		return openaix.IsOutageStatus(pe.HTTPStatus)
	}
	return false
}

func parseUnixTime(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 {
		return time.Time{}, false
	}
	var asInt int64
	if err := json.Unmarshal(raw, &asInt); err == nil && asInt > 0 {
		return time.Unix(asInt, 0).UTC(), true
	}
	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil && asFloat > 0 {
		sec := int64(asFloat)
		nsec := int64((asFloat - float64(sec)) * float64(time.Second))
		return time.Unix(sec, nsec).UTC(), true
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && strings.TrimSpace(asString) != "" {
		if t, err := time.Parse(time.RFC3339Nano, asString); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(time.RFC3339, asString); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func optionalUnixTime(raw json.RawMessage) *time.Time {
	if t, ok := parseUnixTime(raw); ok {
		return &t
	}
	return nil
}
