package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
)

func TestReadOnlyGuard_blocksMutations(t *testing.T) {
	t.Parallel()
	guard := auth.ReadOnlyGuard()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := guard(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/foo/check", nil)
	ctx := auth.WithSessionFlags(req.Context(), true, true)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestReadOnlyGuard_allowsLogoutAndLookup(t *testing.T) {
	t.Parallel()
	guard := auth.ReadOnlyGuard()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := guard(next)

	for _, path := range []string{"/api/v1/auth/logout", "/api/v1/chat/sessions/lookup-by-action"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req = req.WithContext(auth.WithSessionFlags(req.Context(), true, true))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("%s: expected 204, got %d", path, rr.Code)
		}
	}
}

func TestReadOnlyGuard_allowsGET(t *testing.T) {
	t.Parallel()
	guard := auth.ReadOnlyGuard()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := guard(next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	req = req.WithContext(auth.WithSessionFlags(req.Context(), true, true))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNormalizeReviewToken(t *testing.T) {
	t.Parallel()
	raw, _, err := auth.GenerateReviewInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	got, err := auth.NormalizeReviewToken(raw)
	if err != nil || got != raw {
		t.Fatalf("normalize failed: %v %q", err, got)
	}
	if _, err := auth.NormalizeReviewToken("short"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
