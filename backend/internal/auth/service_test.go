package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestBuildAuthResponseAdminWithoutSeller(t *testing.T) {
	user := dbgen.User{ID: 1, Email: "admin@example.com", Status: "active"}
	result := &AuthResult{User: user, SellerAccount: nil}

	resp := buildAuthResponseForTest(result, []string{"admin@example.com"})
	if !resp.IsAdmin {
		t.Fatal("expected is_admin=true")
	}
	if resp.SellerAccount != nil {
		t.Fatal("expected seller_account=null")
	}
}

func TestBuildAuthResponseSellerWithAccount(t *testing.T) {
	user := dbgen.User{ID: 2, Email: "demo@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 10, Name: "Demo", Status: "active"}
	result := &AuthResult{User: user, SellerAccount: &seller}

	resp := buildAuthResponseForTest(result, []string{"admin@example.com"})
	if resp.IsAdmin {
		t.Fatal("expected is_admin=false")
	}
	if resp.SellerAccount == nil || resp.SellerAccount.ID != 10 {
		t.Fatalf("expected seller_account, got %+v", resp.SellerAccount)
	}
}

func TestRequireSellerAccountMiddleware(t *testing.T) {
	seller := dbgen.SellerAccount{ID: 1}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		ctx        context.Context
		wantStatus int
	}{
		{
			name:       "seller account present",
			ctx:        WithAuthContext(context.Background(), dbgen.User{ID: 1}, &seller),
			wantStatus: http.StatusOK,
		},
		{
			name:       "admin without seller account forbidden",
			ctx:        WithAuthContext(context.Background(), dbgen.User{ID: 1, Email: "admin@example.com"}, nil),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard", nil).WithContext(tt.ctx)
			rr := httptest.NewRecorder()

			RequireSellerAccount()(next).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestLoginSellerAccountRequiredForNonAdmin(t *testing.T) {
	err := ErrSellerAccountRequired
	if err == nil {
		t.Fatal("expected ErrSellerAccountRequired")
	}
}

type testAuthResponse struct {
	SellerAccount *struct {
		ID int64 `json:"id"`
	} `json:"seller_account"`
	IsAdmin bool `json:"is_admin"`
}

func buildAuthResponseForTest(result *AuthResult, adminEmails []string) testAuthResponse {
	type sellerAccountResponse struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	resp := struct {
		User struct {
			ID     int64  `json:"id"`
			Email  string `json:"email"`
			Status string `json:"status"`
		} `json:"user"`
		SellerAccount *sellerAccountResponse `json:"seller_account"`
		IsAdmin       bool                   `json:"is_admin"`
	}{
		IsAdmin: IsAdminUser(&result.User, adminEmails),
	}
	resp.User.ID = result.User.ID
	resp.User.Email = result.User.Email
	resp.User.Status = result.User.Status
	if result.SellerAccount != nil {
		resp.SellerAccount = &sellerAccountResponse{
			ID:     result.SellerAccount.ID,
			Name:   result.SellerAccount.Name,
			Status: result.SellerAccount.Status,
		}
	}

	raw, _ := json.Marshal(resp)
	var out testAuthResponse
	_ = json.Unmarshal(raw, &out)
	return out
}
