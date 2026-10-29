package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

type capturedOpenAIHTTP struct {
	requestBody []byte
}

func TestOpenAIActionClient_V2OutboundRequestCapture(t *testing.T) {
	var captured capturedOpenAIHTTP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured.requestBody = body
		_, _ = w.Write([]byte(`{
			"model":"gpt-test",
			"output":[{"type":"message","content":[{"type":"output_text","text":"{\"schema_version\":\"actions_ai_first_v2\",\"actions\":[],\"notes\":[]}"}]}],
			"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}
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

	pack := testV2CatalogPack()
	candidates := []AIFirstActionCandidate{
		testStockCandidate("stock_positions_001:candidate:01"),
		testStockCandidate("stock_positions_002:candidate:02"),
	}
	candidates[1].Problem = candidates[0].Problem
	candidates[1].RecommendedAction = candidates[0].RecommendedAction
	candidates[1].AffectedEntities = []AIFirstAffectedEntityRef{
		{EntityType: "sku", EntityID: "sku_2", FactIDs: []string{"fact_2"}},
	}
	candidates[1].EvidenceFactIDs = []string{"fact_2"}

	input, err := BuildActionsAIFirstInputFromCandidates(
		pack,
		"pack_hash_test",
		testGlobalContext(),
		"gc_hash_test",
		candidates,
		[]string{"chunk_failed_001"},
		[]string{"Часть данных не была проанализирована из-за технической ошибки."},
		DefaultGenerateActionsOptions(),
	)
	if err != nil {
		t.Fatalf("build input: %v", err)
	}

	_, _, err = client.ComposeAIFirstActions(context.Background(), input)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}

	var httpReq recommendationsOpenAIRequestCapture
	if err := json.Unmarshal(captured.requestBody, &httpReq); err != nil {
		t.Fatalf("decode http request: %v body=%s", err, string(captured.requestBody))
	}
	systemPrompt := ""
	userPrompt := ""
	for _, item := range httpReq.Input {
		if len(item.Content) == 0 {
			continue
		}
		switch item.Role {
		case "system":
			systemPrompt = item.Content[0].Text
		case "user":
			userPrompt = item.Content[0].Text
		}
	}
	if strings.Contains(systemPrompt, "Backend не передаёт кандидатов") {
		t.Fatal("legacy V1 system prompt must not be used for V2 input")
	}
	if !strings.Contains(systemPrompt, "ФАЗА A (Compose)") {
		preview := systemPrompt
		if len(preview) > 200 {
			preview = preview[:200]
		}
		t.Fatalf("expected V2 system prompt, got: %s", preview)
	}

	ctxPkg, err := extractContextPackageJSON(userPrompt)
	if err != nil {
		t.Fatalf("extract context package: %v", err)
	}
	if ctxPkg["prompt_version"] != actionsAIFirstV2PromptVersion {
		t.Fatalf("prompt_version=%v want %s", ctxPkg["prompt_version"], actionsAIFirstV2PromptVersion)
	}
	wireRaw, ok := ctxPkg["action_candidates_input_json"]
	if !ok {
		t.Fatalf("missing action_candidates_input_json in context package: %v", ctxPkg)
	}
	wire, err := json.Marshal(wireRaw)
	if err != nil {
		t.Fatal(err)
	}
	wireStr := string(wire)
	for _, forbidden := range []string{
		`"data_pack"`,
		`"fact_registry"`,
		`"entity_registry"`,
		`"sku_rollups"`,
		`"campaign_rollups"`,
		`"raw_chunk"`,
		`"chunk_analyses"`,
	} {
		if strings.Contains(wireStr, forbidden) {
			t.Fatalf("forbidden field %s in outbound wire: %s", forbidden, wireStr)
		}
	}

	var wireObj map[string]any
	if err := json.Unmarshal(wire, &wireObj); err != nil {
		t.Fatal(err)
	}
	if wireObj["schema_version"] != ActionsAIFirstSchemaVersionV2 {
		t.Fatalf("schema_version=%v", wireObj["schema_version"])
	}
	if wireObj["global_context"] == nil {
		t.Fatal("global_context missing")
	}
	if wireObj["fact_catalog"] == nil {
		t.Fatal("fact_catalog missing")
	}
	if wireObj["entity_catalog"] == nil {
		t.Fatal("entity_catalog missing")
	}
	if wireObj["data_limitations"] == nil {
		t.Fatal("data_limitations missing")
	}
	if wireObj["failed_chunk_ids"] == nil {
		t.Fatal("failed_chunk_ids missing")
	}

	cands, ok := wireObj["action_candidates"].([]any)
	if !ok || len(cands) != len(candidates) {
		t.Fatalf("action_candidates=%v want %d", wireObj["action_candidates"], len(candidates))
	}
	gotIDs := make([]string, 0, len(cands))
	for _, raw := range cands {
		m, _ := raw.(map[string]any)
		gotIDs = append(gotIDs, m["candidate_id"].(string))
	}
	wantIDs := []string{candidates[0].CandidateID, candidates[1].CandidateID}
	if gotIDs[0] != wantIDs[0] || gotIDs[1] != wantIDs[1] {
		t.Fatalf("candidate ids=%v want %v", gotIDs, wantIDs)
	}
}

// recommendationsOpenAIRequestCapture mirrors the outbound OpenAI responses request shape.
type recommendationsOpenAIRequestCapture struct {
	Input []struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"input"`
}

func extractContextPackageJSON(userPrompt string) (map[string]any, error) {
	const marker = "CONTEXT_PACKAGE_JSON:\n"
	idx := strings.Index(userPrompt, marker)
	if idx < 0 {
		return nil, fmt.Errorf("CONTEXT_PACKAGE_JSON marker not found")
	}
	raw := strings.TrimSpace(userPrompt[idx+len(marker):])
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
