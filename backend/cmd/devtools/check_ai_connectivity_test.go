package main

import (
	"encoding/json"
	"testing"
)

func TestCheckAIConnectivityOutputShape(t *testing.T) {
	raw := `{
		"status": "ok",
		"provider": "openai",
		"proxy": {
			"ai_proxy_configured": true,
			"ai_proxy_host": "127.0.0.1:10809",
			"effective_proxy_source": "AI_PROXY_URL"
		},
		"latency_ms": 123
	}`
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "ok" || out["provider"] != "openai" {
		t.Fatalf("unexpected output: %+v", out)
	}
	proxy, ok := out["proxy"].(map[string]any)
	if !ok || proxy["effective_proxy_source"] != "AI_PROXY_URL" {
		t.Fatalf("proxy=%+v", out["proxy"])
	}
}
