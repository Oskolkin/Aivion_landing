package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/handlers"
)

func TestMe_LiveReviewResponse(t *testing.T) {
	t.Parallel()
	h := handlers.NewAuthHandler(nil, "session_token", nil, true)

	user := dbgen.User{ID: 7, Email: "demo@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 1, Name: "Demo", Status: "active"}
	ctx := auth.WithAuthContext(context.Background(), user, &seller)
	ctx = auth.WithSessionFlags(ctx, true, true)
	ctx = auth.WithLiveReviewMode(ctx, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Set-Cookie") != "" {
		t.Fatal("live review Me must not Set-Cookie")
	}
	if rr.Header().Get(auth.HeaderLiveReview) != "true" {
		t.Fatal("expected live review response header")
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["is_live_review"] != true {
		t.Fatalf("is_live_review=%v", body["is_live_review"])
	}
	if body["is_admin"] != false {
		t.Fatalf("is_admin=%v", body["is_admin"])
	}
	if body["read_only"] != true || body["is_review_mode"] != true {
		t.Fatalf("flags=%v", body)
	}
	sa, _ := body["seller_account"].(map[string]any)
	if sa == nil || sa["id"].(float64) != 1 {
		t.Fatalf("seller=%v", body["seller_account"])
	}
	raw := rr.Body.String()
	for _, needle := range []string{"session_token", "raw_token", "invite_token", "X-Aivion-Live-Review-Token"} {
		if strings.Contains(raw, needle) {
			t.Fatalf("response must not contain %q", needle)
		}
	}
}
