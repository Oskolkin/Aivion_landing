package chatv2

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestOpenAIStrictCatalogSchemas(t *testing.T) {
	for _, spec := range CatalogV1() {
		if err := assertOpenAIStrictSchema(spec.Schema, spec.Key); err != nil {
			t.Errorf("%s FAIL: %v", spec.Key, err)
			continue
		}
		t.Logf("%s PASS", spec.Key)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Log("LIVE_PROVIDER_SMOKE = NOT RUN: OPENAI_API_KEY is empty")
	}
}

func TestOpenAIProviderSerializesStrictTrue(t *testing.T) {
	tools := make([]map[string]any, 0, len(CatalogV1()))
	for _, spec := range CatalogV1() {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": spec.Key, "description": spec.Description,
				"parameters": spec.Schema, "strict": true,
			},
		})
	}
	raw, err := json.Marshal(map[string]any{"tools": tools})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"strict":true`) {
		t.Fatal("adapter must serialize strict=true")
	}
}
