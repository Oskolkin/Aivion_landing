package openaix

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	// ErrInvalidAIHTTPSProxyURL indicates an AI proxy URL could not be parsed.
	ErrInvalidAIHTTPSProxyURL = errors.New("invalid AI proxy URL")
	// ErrUnsupportedAIHTTPSProxyScheme indicates a non-HTTP(S) proxy scheme (e.g. socks5).
	ErrUnsupportedAIHTTPSProxyScheme = errors.New("unsupported AI proxy scheme")

	// Deprecated: use ErrInvalidAIHTTPSProxyURL.
	ErrInvalidOpenAIProxyURL = ErrInvalidAIHTTPSProxyURL
	// Deprecated: use ErrUnsupportedAIHTTPSProxyScheme.
	ErrUnsupportedOpenAIProxyScheme = ErrUnsupportedAIHTTPSProxyScheme
)

const (
	// ProxySourceAIHTTPS is the canonical OpenAI-only proxy env var.
	ProxySourceAIHTTPS = "AI_HTTPS_PROXY"
	// ProxySourceAIProxyURL is a deprecated local alias for AI_HTTPS_PROXY.
	ProxySourceAIProxyURL = "AI_PROXY_URL"
	// Deprecated source labels retained for diagnostics compatibility; never selected by ResolveAIProxyURL.
	ProxySourceHTTPSProxy = "HTTPS_PROXY"
	ProxySourceHTTPProxy  = "HTTP_PROXY"
	ProxySourceNone       = "none"
)

// ProxySummary describes effective AI proxy configuration without credentials.
type ProxySummary struct {
	AIProxyConfigured    bool   `json:"ai_proxy_configured"`
	AIProxyHost          string `json:"ai_proxy_host,omitempty"`
	EffectiveProxyHost   string `json:"effective_proxy_host,omitempty"`
	EffectiveProxyScheme string `json:"effective_proxy_scheme,omitempty"`
	EffectiveProxySource string `json:"effective_proxy_source"` // AI_HTTPS_PROXY | AI_PROXY_URL | none
	ProxyError           string `json:"proxy_error,omitempty"`
}

// ResolveAIProxyURL returns the effective OpenAI proxy URL and its source env var.
//
// Canonical contract (see docs/ai-proxy-configuration.md):
//  1. AI_HTTPS_PROXY
//  2. AI_PROXY_URL (deprecated alias for local tooling)
//
// HTTP_PROXY and HTTPS_PROXY are intentionally ignored so Ozon and other
// integrations never inherit a process-wide proxy via the Go default transport
// path used by OpenAI clients.
func ResolveAIProxyURL() (proxyURL string, source string) {
	if v := strings.TrimSpace(os.Getenv("AI_HTTPS_PROXY")); v != "" {
		return v, ProxySourceAIHTTPS
	}
	if v := strings.TrimSpace(os.Getenv("AI_PROXY_URL")); v != "" {
		return v, ProxySourceAIProxyURL
	}
	return "", ProxySourceNone
}

// ParseProxyURL validates a proxy URL. Empty input returns (nil, nil).
func ParseProxyURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAIHTTPSProxyURL, err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		if parsed.Scheme != "" {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedAIHTTPSProxyScheme, parsed.Scheme)
		}
		return nil, fmt.Errorf("%w: missing scheme", ErrInvalidAIHTTPSProxyURL)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidAIHTTPSProxyURL)
	}
	return parsed, nil
}

// RedactProxyURL masks userinfo in proxy URLs for logs and errors.
func RedactProxyURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "<invalid AI proxy URL>"
	}
	if parsed.User != nil {
		parsed.User = url.UserPassword("***", "***")
	}
	return parsed.String()
}

// ProxySummaryFromResolved builds diagnostics from a resolved proxy URL and source label.
func ProxySummaryFromResolved(raw, source string) ProxySummary {
	if strings.TrimSpace(source) == "" {
		source = ProxySourceNone
	}
	if strings.TrimSpace(raw) == "" || source == ProxySourceNone {
		return ProxySummary{EffectiveProxySource: ProxySourceNone}
	}
	parsed, err := ParseProxyURL(raw)
	if err != nil {
		return ProxySummary{
			AIProxyConfigured:    true,
			EffectiveProxySource: source,
			ProxyError:           err.Error(),
		}
	}
	host := parsed.Host
	scheme := parsed.Scheme
	return ProxySummary{
		AIProxyConfigured:    true,
		AIProxyHost:          host,
		EffectiveProxyHost:   host,
		EffectiveProxyScheme: scheme,
		EffectiveProxySource: source,
	}
}

// ProxySummaryFromURL builds diagnostics from a raw proxy URL (legacy helper for tests).
func ProxySummaryFromURL(raw string) ProxySummary {
	if strings.TrimSpace(raw) == "" {
		return ProxySummary{EffectiveProxySource: ProxySourceNone}
	}
	return ProxySummaryFromResolved(raw, ProxySourceAIHTTPS)
}

// ProxySummaryFromEnv reads effective AI proxy settings from the environment.
func ProxySummaryFromEnv() ProxySummary {
	raw, source := ResolveAIProxyURL()
	return ProxySummaryFromResolved(raw, source)
}

// BuildHTTPClient returns an OpenAI-only http.Client. Empty proxyURL disables proxy
// and ignores HTTP_PROXY/HTTPS_PROXY (Proxy is explicitly nil).
// When proxyURL is set, requests fail closed if the proxy is unreachable (no direct fallback).
func BuildHTTPClient(timeoutSeconds int, proxyURL string) (*http.Client, error) {
	timeout := DefaultRequestTimeout
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	parsed, err := ParseProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if parsed != nil {
		transport.Proxy = http.ProxyURL(parsed)
	} else {
		transport.Proxy = nil
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}
