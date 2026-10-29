package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestIsAdminUser(t *testing.T) {
	tests := []struct {
		name        string
		user        *dbgen.User
		adminEmails []string
		want        bool
	}{
		{
			name:        "nil user",
			user:        nil,
			adminEmails: []string{"admin@example.com"},
			want:        false,
		},
		{
			name:        "empty allowlist",
			user:        &dbgen.User{Email: "admin@example.com"},
			adminEmails: []string{},
			want:        false,
		},
		{
			name:        "email in allowlist",
			user:        &dbgen.User{Email: "admin@example.com"},
			adminEmails: []string{"admin@example.com", "support@example.com"},
			want:        true,
		},
		{
			name:        "case insensitive match",
			user:        &dbgen.User{Email: "Admin@Example.com"},
			adminEmails: []string{"admin@example.com"},
			want:        true,
		},
		{
			name:        "email not in allowlist",
			user:        &dbgen.User{Email: "seller@example.com"},
			adminEmails: []string{"admin@example.com"},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAdminUser(tt.user, tt.adminEmails)
			if got != tt.want {
				t.Fatalf("IsAdminUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdminMiddleware(t *testing.T) {
	adminUser := dbgen.User{Email: "admin@example.com"}
	nonAdminUser := dbgen.User{Email: "seller@example.com"}
	seller := dbgen.SellerAccount{}
	adminEmails := []string{"admin@example.com"}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	})

	tests := []struct {
		name       string
		ctx        context.Context
		wantStatus int
	}{
		{
			name:       "admin user allowed",
			ctx:        WithAuthContext(context.Background(), adminUser, &seller),
			wantStatus: http.StatusOK,
		},
		{
			name:       "admin user without seller account allowed",
			ctx:        WithAuthContext(context.Background(), adminUser, nil),
			wantStatus: http.StatusOK,
		},
		{
			name:       "non admin user forbidden",
			ctx:        WithAuthContext(context.Background(), nonAdminUser, &seller),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no user in context forbidden",
			ctx:        context.Background(),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil).WithContext(tt.ctx)
			rr := httptest.NewRecorder()

			AdminMiddleware(adminEmails)(next).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
