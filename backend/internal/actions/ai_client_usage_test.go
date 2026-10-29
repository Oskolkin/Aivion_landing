package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func TestOpenAIActionClient_callRaw_UsesProviderUsageNotEstimate(t *testing.T) {
	estimateInput := map[string]any{
		"large_payload": string(make([]byte, 5000)),
	}
	estimateBytes, _ := json.Marshal(estimateInput)
	estimatedTokens := len(estimateBytes) / 4
	if estimatedTokens == 123 {
		estimatedTokens++
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_abc")
		_, _ = w.Write([]byte(`{
			"id": "resp_123",
			"model": "gpt-test",
			"output": [{
				"type": "message",
				"content": [{"type":"output_text","text":"{\"actions\":[]}"}]
			}],
			"usage": {"input_tokens": 123, "output_tokens": 45, "total_tokens": 168}
		}`))
	}))
	defer server.Close()

	openAI, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:  "test-key",
		Model:   "gpt-test",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAIActionClient(openAI, "gpt-test")
	_, resp, err := client.callRaw(context.Background(), "sys", "user", estimateInput)
	if err != nil {
		t.Fatalf("callRaw: %v", err)
	}
	if resp.RequestID != "req_abc" {
		t.Fatalf("request_id=%q", resp.RequestID)
	}
	if resp.InputTokens != 123 {
		t.Fatalf("input_tokens=%d want 123 (estimate would be %d)", resp.InputTokens, estimatedTokens)
	}
	if resp.OutputTokens != 45 {
		t.Fatalf("output_tokens=%d want 45", resp.OutputTokens)
	}
	if resp.LatencyMs < 0 {
		t.Fatalf("latency_ms=%d", resp.LatencyMs)
	}
}
