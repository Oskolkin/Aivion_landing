package chatv2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrompt16OpenAITelemetryParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing auth header in test server (value not logged)")
		}
		w.Header().Set("X-Request-Id", "req_live_1")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp_1",
			"model": "gpt-4.1-mini",
			"usage": map[string]any{"prompt_tokens": 11, "completion_tokens": 7, "prompt_tokens_details": map[string]any{"cached_tokens": 2}},
			"choices": []map[string]any{{
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"tool_calls": []map[string]any{{
						"id": "call_final",
						"function": map[string]any{
							"name":      ToolSubmitTurnResult,
							"arguments": `{"answer":"Заказано 5","answer_mode":"FACTUAL_QUERY","referenced_entities":[],"grounding_handles":[],"limitations":[],"export_dataset":null}`,
						},
					}},
				},
			}},
		})
	}))
	defer srv.Close()
	p := NewOpenAIProvider("test-key", "gpt-4.1-mini", srv.URL, time.Second)
	out, err := p.Turn(t.Context(), ProviderRequest{Instructions: "x", Input: []map[string]any{{"q": "y"}}, Tools: CatalogV1()})
	if err != nil {
		t.Fatal(err)
	}
	if out.InputTokens != 11 || out.OutputTokens != 7 || out.CachedTokens != 2 {
		t.Fatalf("tokens %+v", out)
	}
	if out.ProviderRequestID != "req_live_1" || out.Model != "gpt-4.1-mini" || out.FinishReason != "tool_calls" {
		t.Fatalf("meta %+v", out)
	}
	if out.FinalSubmitCallID != "call_final" || out.Final == nil {
		t.Fatalf("submit identity %+v", out)
	}
	if len(out.ToolCalls) != 0 {
		t.Fatal("submit must not be an executable tool call")
	}
}
