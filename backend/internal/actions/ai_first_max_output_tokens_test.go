package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func TestActionsAIFirstRequestedMaxOutputTokens_UsesEnv(t *testing.T) {
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")
	if got := ActionsAIFirstRequestedMaxOutputTokens(); got != 8192 {
		t.Fatalf("got %d want 8192", got)
	}
}

func TestOpenAIActionClient_ComposeAIFirstActions_SendsMaxOutputTokens(t *testing.T) {
	t.Setenv("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS", "8192")
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{
			"model":"gpt-test",
			"output":[{"type":"message","content":[{"type":"output_text","text":"{\"schema_version\":\"actions_ai_first_v2\",\"actions\":[],\"notes\":[]}"}]}],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{testStockCandidate("stock:candidate:01")}
	input, err := BuildActionsAIFirstInputFromCandidates(
		pack, "hash", testGlobalContext(), "gc", candidates, nil, nil, DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatal(err)
	}

	openAI, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:  "test-key",
		Model:   "gpt-test",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAIActionClient(openAI, "gpt-test")
	_, _, err = client.ComposeAIFirstActions(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeAIFirstActions: %v", err)
	}
	maxOut, _ := captured["max_output_tokens"].(float64)
	if int(maxOut) != 8192 {
		t.Fatalf("max_output_tokens=%v want 8192", captured["max_output_tokens"])
	}
	if captured["model"] == nil {
		t.Fatal("expected model in request")
	}
}

func TestApplyActionsOutputTokenDiagnostics_ExceedLimitWarning(t *testing.T) {
	diag := &AIFirstPipelineDiagnostics{WarningIssueCounts: map[string]int{}}
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")
	applyActionsOutputTokenDiagnostics(diag, 9000)
	if !diag.ActionsOutputTokensExceededRequestedLimit {
		t.Fatal("expected exceeded flag")
	}
	if diag.WarningIssueCounts["provider_output_tokens_exceed_requested_limit"] != 1 {
		t.Fatalf("warning counts=%v", diag.WarningIssueCounts)
	}
}
