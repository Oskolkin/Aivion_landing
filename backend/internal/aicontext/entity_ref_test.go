package aicontext_test

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

func TestSemanticEntityRefUnmarshalString(t *testing.T) {
	var risk struct {
		Entities []aicontext.SemanticEntityRef `json:"entities"`
	}
	raw := `{"entities":["campaign:1","sku:42"]}`
	if err := json.Unmarshal([]byte(raw), &risk); err != nil {
		t.Fatal(err)
	}
	if len(risk.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(risk.Entities))
	}
	if risk.Entities[0].EntityID != "campaign:1" || risk.Entities[0].EntityType != "campaign" {
		t.Fatalf("unexpected entity %+v", risk.Entities[0])
	}
}
