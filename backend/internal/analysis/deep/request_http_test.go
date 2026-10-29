package deep

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestDeepResponsesHTTPContract(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/responses" {
			t.Errorf("path %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		_, _ = w.Write([]byte(`{"id":"resp_d","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"{}"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()
	c := openaix.NewResponsesClient(srv.Client(), "sk-test", srv.URL)
	req := Request{
		Model: "deep-model", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{"analysis_context":{}}`),
		Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV1, MaxOutputTokens: 32768,
		ReasoningEffort: "medium", Verbosity: "low",
	}
	_, err := c.Create(context.Background(), BuildDeepResponsesRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if got["model"] != "deep-model" || got["store"] != false || got["truncation"] != "disabled" {
		t.Fatalf("%#v", got)
	}
	if got["previous_response_id"] != nil {
		t.Fatal("previous_response_id")
	}
	if tools, _ := got["tools"].([]any); len(tools) != 0 {
		t.Fatal("tools")
	}
	if got["instructions"] != StableInstructionsV1 {
		t.Fatal("instructions")
	}
	if got["input"] != `{"analysis_context":{}}` {
		t.Fatal("payload")
	}
	text := got["text"].(map[string]any)
	format := text["format"].(map[string]any)
	if format["name"] != OutputSchemaNameV1 || format["strict"] != true || format["type"] != "json_schema" {
		t.Fatalf("%#v", format)
	}
	if text["verbosity"] != "low" {
		t.Fatal("verbosity")
	}
	if got["max_output_tokens"].(float64) != 32768 {
		t.Fatal("max out")
	}
	reason := got["reasoning"].(map[string]any)
	if reason["effort"] != "medium" {
		t.Fatal("effort")
	}
}
