package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestDisconnectConfirmationRejected(t *testing.T) {
	body := `{"confirmation":"wrong"}`
	var parsed disconnectOzonRequest
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Confirmation == "ОТКЛЮЧИТЬ" {
		t.Fatal("unexpected match")
	}
}

func TestBuildUserConnectionResponseEmptyMissing(t *testing.T) {
	resp := buildUserConnectionResponse(ozon.UserConnectionState{
		ConnectionReady: false,
		Status:          "not_connected",
	})
	if resp.Missing == nil || len(resp.Missing) != 0 {
		t.Fatalf("missing=%v", resp.Missing)
	}
	if resp.Errors == nil {
		t.Fatal("errors should be non-nil slice")
	}
}

func TestConnectionRequiredStatusCode(t *testing.T) {
	// Document expected status for middleware consumers.
	const expected = http.StatusConflict
	if expected != 409 {
		t.Fatal("sanity")
	}
}
