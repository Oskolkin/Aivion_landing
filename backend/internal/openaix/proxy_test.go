package openaix_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestAIHTTPSProxyEmpty(t *testing.T) {
	u, err := openaix.ParseProxyURL("")
	if err != nil || u != nil {
		t.Fatalf("u=%v err=%v", u, err)
	}
	summary := openaix.ProxySummaryFromURL("")
	if summary.AIProxyConfigured || summary.EffectiveProxySource != openaix.ProxySourceNone {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestAIHTTPSProxySet(t *testing.T) {
	summary := openaix.ProxySummaryFromURL("http://127.0.0.1:10809")
	if !summary.AIProxyConfigured || summary.EffectiveProxyHost != "127.0.0.1:10809" {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.EffectiveProxyScheme != "http" {
		t.Fatalf("scheme=%q", summary.EffectiveProxyScheme)
	}
	if summary.EffectiveProxySource != openaix.ProxySourceAIHTTPS {
		t.Fatalf("source=%q", summary.EffectiveProxySource)
	}
}

func TestAIHTTPSProxyInvalid(t *testing.T) {
	_, err := openaix.ParseProxyURL("://bad")
	if err == nil || !errors.Is(err, openaix.ErrInvalidAIHTTPSProxyURL) {
		t.Fatalf("err=%v", err)
	}
	summary := openaix.ProxySummaryFromResolved("://bad", openaix.ProxySourceAIProxyURL)
	if !summary.AIProxyConfigured || summary.ProxyError == "" {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestResolveAIProxyURL_AIHTTPSProxyWins(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "http://127.0.0.1:10809")
	t.Setenv("AI_PROXY_URL", "http://should-not-win:7777")
	t.Setenv("HTTPS_PROXY", "http://should-not-win:9999")
	t.Setenv("HTTP_PROXY", "http://should-not-win:8888")

	raw, source := openaix.ResolveAIProxyURL()
	if raw != "http://127.0.0.1:10809" || source != openaix.ProxySourceAIHTTPS {
		t.Fatalf("raw=%q source=%q", raw, source)
	}
}

func TestResolveAIProxyURL_IgnoresProcessWideHTTPProxies(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "")
	t.Setenv("AI_PROXY_URL", "")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:10809")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:10808")

	raw, source := openaix.ResolveAIProxyURL()
	if raw != "" || source != openaix.ProxySourceNone {
		t.Fatalf("expected no AI proxy from HTTP(S)_PROXY; raw=%q source=%q", raw, source)
	}
}

func TestResolveAIProxyURL_DeprecatedAIProxyURLAlias(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "")
	t.Setenv("AI_PROXY_URL", "http://127.0.0.1:10809")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")

	raw, source := openaix.ResolveAIProxyURL()
	if raw != "http://127.0.0.1:10809" || source != openaix.ProxySourceAIProxyURL {
		t.Fatalf("raw=%q source=%q", raw, source)
	}
}

func TestResolveAIProxyURL_None(t *testing.T) {
	t.Setenv("AI_PROXY_URL", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("AI_HTTPS_PROXY", "")

	raw, source := openaix.ResolveAIProxyURL()
	if raw != "" || source != openaix.ProxySourceNone {
		t.Fatalf("raw=%q source=%q", raw, source)
	}
}

func TestResolveAIProxyURL_InvalidURL(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "://bad")
	raw, _ := openaix.ResolveAIProxyURL()
	_, err := openaix.ParseProxyURL(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestBuildHTTPClientDirectWhenAIHTTPSProxyEmpty(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://should-not-be-used:9999")
	t.Setenv("HTTPS_PROXY", "http://should-not-be-used:9999")

	client, err := openaix.BuildHTTPClient(30, "")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("expected Proxy func to be nil (no HTTP_PROXY / HTTPS_PROXY)")
	}
}

func TestBuildHTTPClientUsesAIHTTPSProxy(t *testing.T) {
	client, err := openaix.BuildHTTPClient(30, "http://127.0.0.1:10809")
	if err != nil {
		t.Fatal(err)
	}
	transport := client.Transport.(*http.Transport)
	req, _ := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/responses", nil)
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:10809" {
		t.Fatalf("proxy=%v", proxyURL)
	}
}

func TestBuildHTTPClient_httpsProxy(t *testing.T) {
	_, err := openaix.BuildHTTPClient(30, "https://127.0.0.1:10809")
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseProxyURL_socks5Unsupported(t *testing.T) {
	_, err := openaix.ParseProxyURL("socks5://127.0.0.1:10808")
	if err == nil || !errors.Is(err, openaix.ErrUnsupportedAIHTTPSProxyScheme) {
		t.Fatalf("err=%v", err)
	}
}

func TestProxySummaryMasksCredentials(t *testing.T) {
	got := openaix.RedactProxyURL("http://user:password@127.0.0.1:10809")
	if got == "" || strings.Contains(got, "password") || strings.Contains(got, "user@127") {
		t.Fatalf("redacted=%q", got)
	}
	if !strings.Contains(got, "127.0.0.1:10809") {
		t.Fatalf("host should remain: %q", got)
	}
	summary := openaix.ProxySummaryFromURL("http://user:password@127.0.0.1:10809")
	if summary.EffectiveProxyHost != "127.0.0.1:10809" {
		t.Fatalf("summary host=%q", summary.EffectiveProxyHost)
	}
}

func TestProxySummaryFromEnv(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "http://127.0.0.1:10809")
	t.Setenv("AI_PROXY_URL", "")
	summary := openaix.ProxySummaryFromEnv()
	if !summary.AIProxyConfigured {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.EffectiveProxySource != openaix.ProxySourceAIHTTPS {
		t.Fatalf("source=%q", summary.EffectiveProxySource)
	}
	if summary.EffectiveProxyScheme != "http" || summary.EffectiveProxyHost != "127.0.0.1:10809" {
		t.Fatalf("summary=%+v", summary)
	}
}
