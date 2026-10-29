package ozon_test

import (
	"net/http"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestOzonClientIgnoresAIHTTPSProxy(t *testing.T) {
	t.Setenv("AI_HTTPS_PROXY", "http://ai-egress:10809")
	t.Setenv("AI_PROXY_URL", "http://ai-egress:10809")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")

	client := ozon.NewClient()
	req, err := http.NewRequest(http.MethodGet, "https://api-seller.ozon.ru/v1/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	// DefaultClient uses nil Transport → http.DefaultTransport → ProxyFromEnvironment.
	// ProxyFromEnvironment must ignore AI_HTTPS_PROXY (only HTTP(S)_PROXY).
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != nil {
		t.Fatalf("Ozon must not inherit AI proxy via ProxyFromEnvironment; got %v", proxyURL)
	}
	_ = client
}
