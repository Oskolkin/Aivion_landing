package recommendations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func mustNewOpenAIClient(t *testing.T, cfg OpenAIClientConfig) *OpenAIClient {
	t.Helper()
	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestNewOpenAIClientDefaultTimeout(t *testing.T) {
	client, err := NewOpenAIClient(OpenAIClientConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient.Timeout != openaix.DefaultRequestTimeout {
		t.Fatalf("timeout=%s want %s", client.httpClient.Timeout, openaix.DefaultRequestTimeout)
	}
}

func TestOpenAIClientGenerateRecommendations_setsMaxOutputTokens(t *testing.T) {
	var captured openAIResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"recommendations\":[]}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:          "test-key",
		Model:           "gpt-test",
		BaseURL:         server.URL,
		MaxOutputTokens: 1500,
	})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{SellerAccountID: 1},
	})
	if err != nil {
		t.Fatalf("GenerateRecommendations: %v", err)
	}
	if captured.MaxOutputTokens != 1500 {
		t.Fatalf("max_output_tokens=%d", captured.MaxOutputTokens)
	}
}

func TestOpenAIClientGenerateRecommendations_UsesJSONObjectFormat(t *testing.T) {
	var captured openAIResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"recommendations\":[]}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{APIKey: "test-key", Model: "gpt-test", BaseURL: server.URL})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{SellerAccountID: 1},
	})
	if err != nil {
		t.Fatalf("GenerateRecommendations: %v", err)
	}
	if captured.Text.Format.Type != "json_object" {
		t.Fatalf("expected json_object format, got %+v", captured.Text)
	}
}

func TestOpenAIClientGenerateRecommendationsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("x-request-id", "req_123")
		_, _ = w.Write([]byte(`{
			"id": "resp_1",
			"model": "gpt-4.1-mini",
			"output": [{
				"type": "message",
				"content": [{"type":"output_text","text":"{\"recommendations\":[]}"}]
			}],
			"usage": {"input_tokens": 10, "output_tokens": 20, "total_tokens": 30}
		}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-4.1-mini",
		TimeoutSeconds: 5,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	out, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "You are an AI analyst.",
		UserPrompt:   "Return JSON only.",
		Context: &AIRecommendationContext{
			ContextVersion:  "v1",
			SellerAccountID: 1,
			AsOfDate:        "2026-04-30",
			GeneratedAt:     time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("GenerateRecommendations returned error: %v", err)
	}
	if out.RequestID != "req_123" {
		t.Fatalf("unexpected request id: %s", out.RequestID)
	}
	if out.TotalTokens != 30 {
		t.Fatalf("unexpected token usage: %d", out.TotalTokens)
	}
	if out.Content == "" {
		t.Fatalf("expected non-empty content")
	}
}

func TestOpenAIClientGenerateRecommendationsMissingAPIKey(t *testing.T) {
	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "",
		Model:          "gpt-4.1-mini",
		TimeoutSeconds: 5,
		MaxRetries:     0,
	})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{ContextVersion: "v1"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrOpenAIAPIKeyMissing {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAIClientGenerateRecommendationsNoRetryOn500WhenMaxRetriesZero(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error"}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-4.1-mini",
		TimeoutSeconds: 5,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{ContextVersion: "v1"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 HTTP call with MaxRetries=0, got %d", got)
	}
}

func TestOpenAIClientGenerateRecommendationsRetriesOn429(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&calls, 1)
		if current == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "resp_1",
			"model": "gpt-4.1-mini",
			"output": [{
				"type": "message",
				"content": [{"type":"output_text","text":"{\"recommendations\":[]}"}]
			}],
			"usage": {"input_tokens": 1, "output_tokens": 2, "total_tokens": 3}
		}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-4.1-mini",
		TimeoutSeconds: 5,
		MaxRetries:     1,
		BaseURL:        server.URL,
	})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{ContextVersion: "v1"},
	})
	if err != nil {
		t.Fatalf("GenerateRecommendations returned error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestOpenAIClientCloneWithTimeoutUsesIndependentHTTPDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"recommendations\":[]}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	base := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-test",
		TimeoutSeconds: 1,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	long, err := base.CloneWithTimeout(3)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := long.GenerateRecommendations(ctx, GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{SellerAccountID: 1},
	}); err != nil {
		t.Fatalf("long client should succeed: %v", err)
	}

	ctxShort, cancelShort := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShort()
	if _, err := base.GenerateRecommendations(ctxShort, GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{SellerAccountID: 1},
	}); err == nil {
		t.Fatal("base client expected timeout")
	}
}

func TestOpenAIClientCloneWithTimeoutFailsWhenServerExceedsStageDeadline(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"recommendations\":[]}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	client, err := NewOpenAIClient(OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-test",
		TimeoutSeconds: 1,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.GenerateRecommendations(ctx, GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{SellerAccountID: 1},
	}); err == nil {
		t.Fatal("expected timeout error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}
}

func TestOpenAIClientGenerateRecommendationsNoOutputText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"x","model":"gpt-4.1-mini","output":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}`))
	}))
	defer server.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-4.1-mini",
		TimeoutSeconds: 5,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	_, err := client.GenerateRecommendations(context.Background(), GenerateRecommendationsInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      &AIRecommendationContext{ContextVersion: "v1"},
	})
	if err == nil {
		t.Fatalf("expected error for empty output text")
	}
	if want := "does not contain text output"; err != nil && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %v", want, err)
	}
}
