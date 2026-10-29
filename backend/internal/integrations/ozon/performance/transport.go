package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("performance provider error: status=%d body=%s", e.StatusCode, e.Body)
}

type ResponseMeta struct {
	StatusCode int
	RequestID  string
}

type RawResponse struct {
	Body []byte
	Meta ResponseMeta
}

type Transport struct {
	httpClient  *http.Client
	baseURL     string
	retryConfig RetryConfig
}

// APIBaseURL returns Ozon Performance API base URL (override via OZON_PERFORMANCE_API_BASE_URL for tests).
func APIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("OZON_PERFORMANCE_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://performance.ozon.ru"
}

func NewTransport() *Transport {
	return &Transport{
		httpClient:  &http.Client{Timeout: 20 * time.Second},
		baseURL:     APIBaseURL(),
		retryConfig: DefaultRetryConfig(),
	}
}

func (t *Transport) doRequestOnce(
	ctx context.Context,
	bearerToken string,
	method string,
	path string,
	queryParams map[string]string,
	body []byte,
) (*RawResponse, error) {
	requestURL := strings.TrimRight(t.baseURL, "/") + path
	if len(queryParams) > 0 {
		values := url.Values{}
		for key, value := range queryParams {
			if strings.TrimSpace(value) == "" {
				continue
			}
			values.Set(key, value)
		}
		encoded := values.Encode()
		if encoded != "" {
			requestURL += "?" + encoded
		}
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, fmt.Errorf("build performance request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
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
		return nil, fmt.Errorf("read performance response body: %w", err)
	}

	meta := ResponseMeta{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return &RawResponse{Body: respBody, Meta: meta}, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrInvalidCredentials
	default:
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}
}

func (t *Transport) doRequest(
	ctx context.Context,
	bearerToken string,
	method string,
	path string,
	queryParams map[string]string,
	body []byte,
) (*RawResponse, error) {
	return withRetry[*RawResponse](ctx, t.retryConfig, func() (*RawResponse, error) {
		return t.doRequestOnce(ctx, bearerToken, method, path, queryParams, body)
	})
}

func (t *Transport) doJSON(
	ctx context.Context,
	bearerToken string,
	method string,
	path string,
	queryParams map[string]string,
	request any,
	responseDest any,
) (*RawResponse, error) {
	var body []byte
	var err error
	if request != nil {
		body, err = json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("marshal performance request: %w", err)
		}
	}

	rawResp, err := t.doRequest(ctx, bearerToken, method, path, queryParams, body)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(rawResp.Body))
	decoder.UseNumber()
	if err := decoder.Decode(responseDest); err != nil {
		return nil, fmt.Errorf("unmarshal performance response: %w", err)
	}
	return rawResp, nil
}

func (t *Transport) doJSONWithFallbackPaths(
	ctx context.Context,
	bearerToken string,
	method string,
	paths []string,
	queryParams map[string]string,
	request any,
	responseDest any,
) (*RawResponse, error) {
	var lastErr error
	for _, path := range paths {
		rawResp, err := t.doJSON(ctx, bearerToken, method, path, queryParams, request, responseDest)
		if err == nil {
			return rawResp, nil
		}
		lastErr = err

		var providerErr *ProviderError
		if errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusNotFound {
			continue
		}
		return nil, err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no performance endpoint paths provided")
	}
	return nil, lastErr
}
