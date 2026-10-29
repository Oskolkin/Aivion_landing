package openaix

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateResponsesContract(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/responses" {
			t.Errorf("path %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("X-Request-Id", "req_123")
		_, _ = w.Write([]byte(`{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"{\"ok\":true}"}]}],"usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14,"input_tokens_details":{"cached_tokens":2},"output_tokens_details":{"reasoning_tokens":1}}}`))
	}))
	defer srv.Close()
	c := NewResponsesClient(srv.Client(), "sk-secret-key", srv.URL)
	out, err := c.Create(context.Background(), ResponsesRequest{
		Model: "gpt-test", Instructions: "stable", Input: `{"a":1}`, Tools: []any{},
		MaxOutputTokens: 8192, Truncation: "disabled", Store: false,
		Text: TextFormat{Format: JSONSchemaFormat{Type: "json_schema", Name: "aivion_discovery_output_v1", Strict: true, Schema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sk-secret-key" {
		t.Fatal("authorization")
	}
	if gotBody["model"] != "gpt-test" || gotBody["store"] != false || gotBody["truncation"] != "disabled" {
		t.Fatalf("body %#v", gotBody)
	}
	if gotBody["previous_response_id"] != nil {
		t.Fatal("previous_response_id must be omitted or null")
	}
	tools, _ := gotBody["tools"].([]any)
	if len(tools) != 0 {
		t.Fatal("tools")
	}
	text := gotBody["text"].(map[string]any)
	format := text["format"].(map[string]any)
	if format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("format %#v", format)
	}
	if out.Usage.CachedInputTokens != 2 || out.Usage.ReasoningTokens != 1 || out.RequestID != "req_123" {
		t.Fatalf("usage %#v id=%s", out.Usage, out.RequestID)
	}
	if strings.Contains(fmtLog(gotBody), "sk-secret") {
		t.Fatal("body log should not be used with secrets")
	}
}

func fmtLog(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestParseRefusalIncompleteMalformed(t *testing.T) {
	ref, err := ParseResponsesJSON([]byte(`{"status":"completed","output":[{"content":[{"type":"refusal","refusal":"no"}]}]}`))
	if err != nil || ref.Refusal != "no" {
		t.Fatalf("refusal %#v %v", ref, err)
	}
	inc, err := ParseResponsesJSON([]byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}`))
	if err != nil || inc.Status != "incomplete" || inc.IncompleteReason != "max_output_tokens" {
		t.Fatal(inc)
	}
	if _, err = ParseResponsesJSON([]byte(`{`)); err == nil {
		t.Fatal("malformed")
	}
}

func TestHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate"}`))
	}))
	defer srv.Close()
	c := NewResponsesClient(srv.Client(), "k", srv.URL)
	_, err := c.Create(context.Background(), ResponsesRequest{Model: "m", Tools: []any{}})
	if !IsTemporarilyUnavailable(err) {
		t.Fatalf("429: %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()
	cl := srv.Client()
	cl.Timeout = time.Millisecond
	c := NewResponsesClient(cl, "k", srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := c.Create(ctx, ResponsesRequest{Model: "m", Tools: []any{}})
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestRedactSecrets(t *testing.T) {
	s := RedactSecrets("Authorization Bearer sk-abc123token and more")
	if strings.Contains(s, "sk-abc123token") {
		t.Fatal(s)
	}
}
