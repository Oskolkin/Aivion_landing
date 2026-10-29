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
	"strings"
	"time"
)

// TokenResponse is returned by POST /api/client/token on the Performance API auth host.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type AuthClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewAuthClient(baseURL string) *AuthClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api-performance.ozon.ru"
	}
	return &AuthClient{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (c *AuthClient) ObtainToken(ctx context.Context, clientID, clientSecret string) (TokenResponse, error) {
	payload := map[string]string{
		"client_id":     strings.TrimSpace(clientID),
		"client_secret": strings.TrimSpace(clientSecret),
		"grant_type":    "client_credentials",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("marshal performance token request: %w", err)
	}

	url := c.baseURL + "/api/client/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("build performance token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return TokenResponse{}, ErrNetworkError
		}
		return TokenResponse{}, ErrNetworkError
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("read performance token response: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var parsed TokenResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return TokenResponse{}, fmt.Errorf("unmarshal performance token response: %w", err)
		}
		if strings.TrimSpace(parsed.AccessToken) == "" {
			return TokenResponse{}, fmt.Errorf("performance token response missing access_token")
		}
		return parsed, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return TokenResponse{}, ErrInvalidCredentials
	case resp.StatusCode == http.StatusTooManyRequests:
		return TokenResponse{}, &ProviderError{StatusCode: resp.StatusCode, Body: string(respBody)}
	default:
		if resp.StatusCode >= 500 {
			return TokenResponse{}, &ProviderError{StatusCode: resp.StatusCode, Body: string(respBody)}
		}
		return TokenResponse{}, &ProviderError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
}
