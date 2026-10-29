package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	appmw "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/middleware"
)

type demoChecker map[int64]bool

func (d demoChecker) IsStagingDemoSeller(id int64) bool { return d[id] }

func TestRequireOzonConnection_AllowsStagingDemoSeller(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := appmw.RequireOzonConnection(nil, demoChecker{1: true})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	seller := dbgen.SellerAccount{ID: 1, UserID: 1, Name: "demo", Status: "active"}
	user := dbgen.User{ID: 1, Email: "a@b.c", Status: "active"}
	ctx := auth.WithAuthContext(req.Context(), user, &seller)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK || !called {
		t.Fatalf("status=%d called=%v body=%s", rr.Code, called, rr.Body.String())
	}
}

func TestRequireOzonConnection_NonDemoStillNeedsSellerContext(t *testing.T) {
	mw := appmw.RequireOzonConnection(nil, demoChecker{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	rr := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run")
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
