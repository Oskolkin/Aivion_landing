package openaix_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestBuildHTTPClient_FailClosedWhenProxyUnreachable(t *testing.T) {
	// Closed local port: CONNECT must fail via proxy; must not silently reach OpenAI directly.
	client, err := openaix.BuildHTTPClient(2, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	resp, err := client.Get("https://api.openai.com/v1/models")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected fail-closed error when AI proxy is unreachable")
	}
	if time.Since(started) > 15*time.Second {
		t.Fatalf("fail-closed took too long: %s", time.Since(started))
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "password") || strings.Contains(msg, "sk-") {
		t.Fatalf("error must not leak secrets: %q", msg)
	}
}

func TestProxySummary_DoesNotEmbedCredentials(t *testing.T) {
	summary := openaix.ProxySummaryFromURL("http://user:s3cret@ai-egress:10809")
	if summary.EffectiveProxyHost != "ai-egress:10809" {
		t.Fatalf("host=%q", summary.EffectiveProxyHost)
	}
	if strings.Contains(summary.ProxyError, "s3cret") {
		t.Fatalf("proxy error leaked credentials: %q", summary.ProxyError)
	}
	redacted := openaix.RedactProxyURL("http://user:s3cret@ai-egress:10809")
	if strings.Contains(redacted, "s3cret") {
		t.Fatalf("redacted URL leaked credentials: %q", redacted)
	}
}

func TestBuildHTTPClient_UsesDedicatedTransportNotDefault(t *testing.T) {
	client, err := openaix.BuildHTTPClient(30, "http://ai-egress:10809")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport == http.DefaultTransport {
		t.Fatal("OpenAI client must not share DefaultTransport")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/responses", nil)
	proxyURL, err := transport.Proxy(req)
	if err != nil || proxyURL == nil || proxyURL.String() != "http://ai-egress:10809" {
		t.Fatalf("proxy=%v err=%v", proxyURL, err)
	}
}
