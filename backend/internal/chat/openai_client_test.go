package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestNewOpenAIClientDefaultTimeout(t *testing.T) {
	client, err := NewOpenAIClient(OpenAIClientConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient.Timeout != openaix.DefaultRequestTimeout {
		t.Fatalf("timeout=%s want %s", client.httpClient.Timeout, openaix.DefaultRequestTimeout)
	}
}

func mustNewOpenAIClient(t *testing.T, cfg OpenAIClientConfig) *OpenAIClient {
	t.Helper()
	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestOpenAIClientPlanToolsParsesStrictJSON(t *testing.T) {
	var reqPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("x-request-id", "req_plan_1")
		_, _ = w.Write([]byte(`{
			"id":"resp_1","model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"intent\":\"priorities\",\"confidence\":0.9,\"language\":\"ru\",\"tool_calls\":[{\"name\":\"get_open_recommendations\",\"args\":{\"limit\":5}}],\"assumptions\":[\"a1\"]}"}]}],
			"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}
		}`))
	}))
	defer srv.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:  "k",
		Model:   "gpt-test",
		BaseURL: srv.URL,
	})
	out, err := client.PlanTools(context.Background(), PlanToolsInput{SystemPrompt: "s", UserPrompt: "u"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Plan.Intent != ChatIntentPriorities {
		t.Fatalf("intent parse failed: %s", out.Plan.Intent)
	}
	if out.RequestID != "req_plan_1" || out.TotalTokens != 30 {
		t.Fatalf("metadata parse failed")
	}
	text, ok := reqPayload["text"].(map[string]any)
	if !ok {
		t.Fatalf("missing text config in request")
	}
	format, ok := text["format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("expected json_object format in request")
	}
}

func TestOpenAIClientGenerateAnswerParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_answer_1")
		_, _ = w.Write([]byte(`{
			"id":"resp_2","model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"answer\":\"ok\",\"summary\":\"sum\",\"intent\":\"sales\",\"confidence_level\":\"high\",\"related_alert_ids\":[1],\"related_recommendation_ids\":[2],\"supporting_facts\":[{\"source\":\"tool\",\"id\":1,\"fact\":\"f\"}],\"limitations\":[\"l\"]}"}]}],
			"usage":{"input_tokens":11,"output_tokens":22,"total_tokens":33}
		}`))
	}))
	defer srv.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:  "k",
		Model:   "gpt-test",
		BaseURL: srv.URL,
	})
	out, err := client.GenerateAnswer(context.Background(), GenerateAnswerInput{SystemPrompt: "s", UserPrompt: "u"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Answer.Answer != "ok" || out.Answer.ConfidenceLevel != ConfidenceLevelHigh {
		t.Fatalf("answer parse failed")
	}
	if out.RequestID != "req_answer_1" || out.TotalTokens != 33 {
		t.Fatalf("metadata parse failed")
	}
}

func TestResolveImageMIMETypeSniffsOctetStream(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	if got := resolveImageMIMEType("png", "application/octet-stream", png); got != "image/png" {
		t.Fatalf("got %s", got)
	}
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}
	if got := resolveImageMIMEType("", "application/octet-stream", jpeg); got != "image/jpeg" {
		t.Fatalf("got %s", got)
	}
}

func TestOpenAIClientStreamAnswerEmitsProviderDeltas(t *testing.T) {
	var reqPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_1")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"Первая \"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"часть\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_1\",\"model\":\"gpt-test\",\"usage\":{\"input_tokens\":3,\"output_tokens\":4,\"total_tokens\":7}}}\n\n"))
	}))
	defer srv.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{APIKey: "k", Model: "gpt-test", BaseURL: srv.URL})
	var deltas strings.Builder
	out, err := client.StreamAnswer(context.Background(), GenerateAnswerInput{
		UserPrompt: "u",
		FactContext: &FactContext{
			Intent: ChatIntentGeneralOverview,
			Facts:  FactContextFacts{Dashboard: map[string]any{"revenue": 100}},
		},
		InputFileIDs: []string{"", "file_1"},
	}, func(delta string) error {
		deltas.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatalf("stream answer: %v", err)
	}
	if deltas.String() != "Первая часть" || out.Answer.Answer != deltas.String() {
		t.Fatalf("unexpected streamed output: %+v / %q", out, deltas.String())
	}
	if out.RequestID != "req_stream_1" || out.TotalTokens != 7 {
		t.Fatalf("metadata parse failed: %+v", out)
	}
	if reqPayload["stream"] != true {
		t.Fatalf("expected stream=true request: %+v", reqPayload)
	}
	if _, hasText := reqPayload["text"]; hasText {
		t.Fatalf("stream request must not request json text format")
	}
	inputs := reqPayload["input"].([]any)
	user := inputs[1].(map[string]any)
	content := user["content"].([]any)
	if len(content) != 2 || content[1].(map[string]any)["file_id"] != "file_1" {
		t.Fatalf("empty file IDs must be filtered: %+v", content)
	}
}

func TestOpenAIClientMissingAPIKey(t *testing.T) {
	client := mustNewOpenAIClient(t, OpenAIClientConfig{APIKey: ""})
	_, err := client.PlanTools(context.Background(), PlanToolsInput{SystemPrompt: "s", UserPrompt: "u"})
	if !errors.Is(err, ErrOpenAIAPIKeyMissing) {
		t.Fatalf("expected api key missing error, got: %v", err)
	}
}

func TestOpenAIClientRetriesOnServerError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id":"resp_3","model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"intent\":\"unknown\",\"confidence\":0.5,\"language\":\"ru\",\"tool_calls\":[],\"assumptions\":[]}"}]}],
			"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
		}`))
	}))
	defer srv.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:     "k",
		Model:      "gpt-test",
		BaseURL:    srv.URL,
		MaxRetries: 1,
	})
	_, err := client.PlanTools(context.Background(), PlanToolsInput{SystemPrompt: "s", UserPrompt: "u"})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls with retry, got %d", callCount)
	}
}

func TestOpenAIClientInvalidJSONPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"resp_4","model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"not-json"}]}],
			"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
		}`))
	}))
	defer srv.Close()

	client := mustNewOpenAIClient(t, OpenAIClientConfig{
		APIKey:  "k",
		Model:   "gpt-test",
		BaseURL: srv.URL,
	})
	_, err := client.PlanTools(context.Background(), PlanToolsInput{SystemPrompt: "s", UserPrompt: "u"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
