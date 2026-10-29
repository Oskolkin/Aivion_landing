package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingOAuthCompleter struct {
	sellerID int64
	code     string
}

func (f *recordingOAuthCompleter) CompletePrivateOAuth(_ context.Context, sellerID int64, code, _ string) error {
	f.sellerID, f.code = sellerID, code
	return nil
}

func configureOAuthStateSecret(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jwt-secret")
	if err := os.WriteFile(path, []byte("test-state-signing-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET_FILE", path)
}

func TestOAuthCallbackValidatesStatePersistsSellerBoundGrantAndReturnsNoTokens(t *testing.T) {
	configureOAuthStateSecret(t)
	state, err := newOAuthState(77)
	if err != nil {
		t.Fatal(err)
	}
	signature, ok := stateSignature(state)
	if !ok {
		t.Fatal("state signature unavailable")
	}
	fake := &recordingOAuthCompleter{}
	handler := NewOzonOAuthCallback(fake)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?state="+state+"&code=provider-code", nil)
	req.AddCookie(&http.Cookie{Name: "aivion_ozon_state", Value: state + "." + signature})
	response := httptest.NewRecorder()
	handler.Get(response, req)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/app/integrations/ozon?oauth=success" {
		t.Fatalf("status=%d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if fake.sellerID != 77 || fake.code != "provider-code" {
		t.Fatalf("persisted seller=%d code=%q", fake.sellerID, fake.code)
	}
	if body := response.Body.String(); strings.Contains(body, "provider-code") || strings.Contains(body, "access_token") || strings.Contains(body, "refresh_token") {
		t.Fatalf("callback exposed security material: %q", body)
	}
}

func TestOAuthCallbackRejectsStateMismatchBeforePersistence(t *testing.T) {
	configureOAuthStateSecret(t)
	state, _ := newOAuthState(77)
	signature, _ := stateSignature(state)
	fake := &recordingOAuthCompleter{}
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?state=tampered&code=provider-code", nil)
	req.AddCookie(&http.Cookie{Name: "aivion_ozon_state", Value: state + "." + signature})
	response := httptest.NewRecorder()
	NewOzonOAuthCallback(fake).Get(response, req)
	if response.Code != http.StatusBadRequest || fake.sellerID != 0 {
		t.Fatalf("status=%d persisted seller=%d", response.Code, fake.sellerID)
	}
}
