package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestAuthHandlerMeAdminWithoutSeller(t *testing.T) {
	h := NewAuthHandler(nil, "session_token", []string{"admin@example.com"}, false)

	user := dbgen.User{ID: 1, Email: "admin@example.com", Status: "active"}
	ctx := auth.WithAuthContext(context.Background(), user, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		IsAdmin       bool `json:"is_admin"`
		SellerAccount any  `json:"seller_account"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.IsAdmin {
		t.Fatal("expected is_admin=true")
	}
	if body.SellerAccount != nil {
		t.Fatalf("expected seller_account=null, got %v", body.SellerAccount)
	}
}

func TestAuthHandlerMeSellerWithAccount(t *testing.T) {
	h := NewAuthHandler(nil, "session_token", []string{"admin@example.com"}, false)

	user := dbgen.User{ID: 2, Email: "demo@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 10, Name: "Demo", Status: "active"}
	ctx := auth.WithAuthContext(context.Background(), user, &seller)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		IsAdmin       bool `json:"is_admin"`
		SellerAccount struct {
			ID int64 `json:"id"`
		} `json:"seller_account"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.IsAdmin {
		t.Fatal("expected is_admin=false")
	}
	if body.SellerAccount.ID != 10 {
		t.Fatalf("seller_account.id = %d, want 10", body.SellerAccount.ID)
	}
}

func TestAuthHandlerMeDemoSellerFlag(t *testing.T) {
	h := NewAuthHandler(nil, "session_token", []string{"admin@example.com"}, false).
		WithDemoSellerChecker(demoSellerMap{10: true})

	user := dbgen.User{ID: 2, Email: "demo@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 10, Name: "Demo", Status: "active"}
	ctx := auth.WithAuthContext(context.Background(), user, &seller)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body struct {
		IsDemoSeller bool `json:"is_demo_seller"`
		IsAdmin      bool `json:"is_admin"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.IsDemoSeller || body.IsAdmin {
		t.Fatalf("expected demo seller non-admin, got %+v", body)
	}
}

func TestAuthHandlerPublicConfig(t *testing.T) {
	h := NewAuthHandler(nil, "session_token", nil, false).
		WithPublicRegistration(false).
		WithPublicDemoMode(true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/public-config", nil)
	rr := httptest.NewRecorder()
	h.PublicConfig(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var body struct {
		PublicRegistration bool `json:"public_registration"`
		PublicDemoMode     bool `json:"public_demo_mode"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.PublicRegistration || !body.PublicDemoMode {
		t.Fatalf("unexpected public config: %+v", body)
	}
}

type demoSellerMap map[int64]bool

func (d demoSellerMap) IsStagingDemoSeller(id int64) bool { return d[id] }
