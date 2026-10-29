package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNetworkError       = errors.New("network error")
	ErrProviderError      = errors.New("provider error")
)

type ProviderError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider error: status=%d body=%s", e.StatusCode, e.Body)
}

type Client struct {
	httpClient  *http.Client
	baseURL     string
	retryConfig RetryConfig
}

// SellerAPIBaseURL returns Ozon seller API base URL (override via OZON_SELLER_API_BASE_URL for tests).
func SellerAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("OZON_SELLER_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api-seller.ozon.ru"
}

func NewClient() *Client {
	return NewClientWithConfig(nil, SellerAPIBaseURL(), DefaultRetryConfig())
}

func NewClientWithConfig(httpClient *http.Client, baseURL string, retryConfig RetryConfig) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://api-seller.ozon.ru"
	}

	return &Client{
		httpClient:  httpClient,
		baseURL:     baseURL,
		retryConfig: retryConfig,
	}
}

func (c *Client) doRequestOnce(
	ctx context.Context,
	clientID string,
	apiKey string,
	method string,
	path string,
	body []byte,
) (*RawResponse, error) {
	return c.doRequestOnceWithAuth(ctx, method, path, body, func(header http.Header) {
		header.Set("Client-Id", clientID)
		header.Set("Api-Key", apiKey)
	})
}

func (c *Client) doRequestOnceWithAuth(ctx context.Context, method, path string, body []byte, authorize func(http.Header)) (*RawResponse, error) {
	url := c.baseURL + path

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build ozon request: %w", err)
	}

	authorize(req.Header)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return nil, ErrNetworkError
		}
		return nil, ErrNetworkError
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read ozon response body: %w", err)
	}

	meta := ResponseMeta{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return &RawResponse{
			Body: respBody,
			Meta: meta,
		}, nil

	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrInvalidCredentials

	default:
		bodyText := strings.TrimSpace(string(respBody))
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Body:       bodyText,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		}
	}
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if at, err := http.ParseTime(raw); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func (c *Client) doBearerRequest(ctx context.Context, accessToken, method, path string, body []byte) (*RawResponse, error) {
	return withRetry[*RawResponse](ctx, c.retryConfig, func() (*RawResponse, error) {
		return c.doRequestOnceWithAuth(ctx, method, path, body, func(header http.Header) {
			header.Set("Authorization", "Bearer "+accessToken)
		})
	})
}

func (c *Client) doRequest(
	ctx context.Context,
	clientID string,
	apiKey string,
	method string,
	path string,
	body []byte,
) (*RawResponse, error) {
	return withRetry[*RawResponse](ctx, c.retryConfig, func() (*RawResponse, error) {
		return c.doRequestOnce(ctx, clientID, apiKey, method, path, body)
	})
}

func (c *Client) doJSON(
	ctx context.Context,
	clientID string,
	apiKey string,
	method string,
	path string,
	request any,
	responseDest any,
) (*RawResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal ozon request: %w", err)
	}

	rawResp, err := c.doRequest(ctx, clientID, apiKey, method, path, body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(rawResp.Body, responseDest); err != nil {
		return nil, fmt.Errorf("unmarshal ozon response: %w", err)
	}

	return rawResp, nil
}

func (c *Client) doJSONBearer(ctx context.Context, accessToken, method, path string, request, responseDest any) (*RawResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal ozon request: %w", err)
	}
	rawResp, err := c.doBearerRequest(ctx, accessToken, method, path, body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawResp.Body, responseDest); err != nil {
		return nil, fmt.Errorf("unmarshal ozon response: %w", err)
	}
	return rawResp, nil
}

func (c *Client) CheckConnection(ctx context.Context, clientID, apiKey string) error {
	// Primary business check.
	if _, err := c.GetSellerInfo(ctx, clientID, apiKey); err != nil {
		return err
	}

	// Secondary technical validation. Do not override successful primary check
	// for non-auth transient/provider issues.
	if _, err := c.GetRoles(ctx, clientID, apiKey); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return err
		}
	}
	return nil
}
