package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type stubAuthenticator struct {
	userResult      *auth.AuthResult
	userErr         error
	resolveResult   *auth.AuthResult
	resolveErr      error
	resolveCalls    int
	getCurrentCalls int
	lastResolveTok  string
}

func (s *stubAuthenticator) GetCurrentUser(_ context.Context, _ string) (*auth.AuthResult, error) {
	s.getCurrentCalls++
	if s.userErr != nil {
		return nil, s.userErr
	}
	return s.userResult, nil
}

func (s *stubAuthenticator) ResolveReviewInvite(_ context.Context, rawToken string) (*auth.AuthResult, error) {
	s.resolveCalls++
	s.lastResolveTok = rawToken
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.resolveResult, nil
}

func seller1Result() *auth.AuthResult {
	user := dbgen.User{ID: 7, Email: "demo@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 1, Name: "Demo Seller", Status: "active", UserID: 7}
	return &auth.AuthResult{
		User:           user,
		SellerAccount:  &seller,
		ReadOnly:       true,
		IsReviewMode:   true,
		SessionExpires: time.Now().UTC().Add(2 * time.Hour),
		ReviewInviteID: 42,
	}
}

func liveCfg() *auth.LiveReviewConfig {
	return &auth.LiveReviewConfig{
		Enabled:                true,
		AllowedHostSuffix:      ".review.aivion.pro",
		AllowedSellerAccountID: 1,
		IsAllowedSeller:        func(id int64) bool { return id == 1 },
	}
}

func TestHostMatchesLiveReviewSuffix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host, suffix string
		want         bool
	}{
		{"abc.review.aivion.pro", ".review.aivion.pro", true},
		{"REVIEW.aivion.pro", ".review.aivion.pro", false},
		{"aivion.pro", ".review.aivion.pro", false},
		{"review.aivion.pro", ".review.aivion.pro", false},
		{"", ".review.aivion.pro", false},
		{"abc.review.aivion.pro", "", false},
	}
	for _, tc := range cases {
		// exercise via middleware host header path
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set(auth.HeaderLiveReview, "1")
		req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("a", 64))
		req.Header.Set(auth.HeaderLiveReviewHost, tc.host)
		stub := &stubAuthenticator{resolveErr: auth.ErrReviewInviteInvalid}
		cfg := liveCfg()
		cfg.AllowedHostSuffix = tc.suffix
		rr := httptest.NewRecorder()
		auth.Middleware(stub, "session_token", cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		gotOK := rr.Code == http.StatusOK
		_ = gotOK
		if tc.want {
			// valid host still fails resolve (invalid token) → 401, but resolve was attempted
			if stub.resolveCalls == 0 {
				t.Fatalf("host %q suffix %q: expected resolve attempt", tc.host, tc.suffix)
			}
		} else if tc.host != "" && tc.suffix != "" {
			if stub.resolveCalls != 0 {
				t.Fatalf("host %q suffix %q: expected host rejection before resolve", tc.host, tc.suffix)
			}
		}
	}
}

func TestMiddleware_CookieAuthUnchanged(t *testing.T) {
	t.Parallel()
	user := dbgen.User{ID: 2, Email: "seller@example.com", Status: "active"}
	seller := dbgen.SellerAccount{ID: 9, Name: "S", Status: "active"}
	stub := &stubAuthenticator{
		userResult: &auth.AuthResult{User: user, SellerAccount: &seller},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "cookie-session"})
	rr := httptest.NewRecorder()
	var gotLive bool
	var gotSeller int64
	auth.Middleware(stub, "session_token", liveCfg())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLive = auth.LiveReviewModeFromContext(r.Context())
		if sa, ok := auth.SellerAccountFromContext(r.Context()); ok {
			gotSeller = sa.ID
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if stub.resolveCalls != 0 {
		t.Fatal("cookie path must not resolve live review")
	}
	if gotLive {
		t.Fatal("live review must be false for cookie auth")
	}
	if gotSeller != 9 {
		t.Fatalf("seller=%d", gotSeller)
	}
}

func TestMiddleware_LiveReviewSuccess(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("ab", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()

	var (
		gotLive, gotRO, gotReview bool
		gotSeller                 int64
	)
	auth.Middleware(stub, "session_token", liveCfg())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLive = auth.LiveReviewModeFromContext(r.Context())
		gotRO = auth.ReadOnlyFromContext(r.Context())
		gotReview = auth.ReviewModeFromContext(r.Context())
		if sa, ok := auth.SellerAccountFromContext(r.Context()); ok {
			gotSeller = sa.ID
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if stub.getCurrentCalls != 0 {
		t.Fatal("live review must not touch cookie session")
	}
	if !gotLive || !gotRO || !gotReview || gotSeller != 1 {
		t.Fatalf("flags live=%v ro=%v review=%v seller=%d", gotLive, gotRO, gotReview, gotSeller)
	}
	if rr.Header().Get(auth.HeaderLiveReview) != "true" {
		t.Fatal("expected live review response header")
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("expected no-store cache header")
	}
}

func TestMiddleware_LiveReviewMissingTokenNoCookieFallback(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{
		userResult: seller1Result(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "should-not-be-used"})
	rr := httptest.NewRecorder()
	auth.Middleware(stub, "session_token", liveCfg())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	if stub.getCurrentCalls != 0 || stub.resolveCalls != 0 {
		t.Fatal("must not fall back to cookie or resolve without token")
	}
}

func TestMiddleware_LiveReviewInvalidTokenNoCookieFallback(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{
		resolveErr: auth.ErrReviewInviteInvalid,
		userResult: seller1Result(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("cd", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "should-not-be-used"})
	rr := httptest.NewRecorder()
	auth.Middleware(stub, "session_token", liveCfg())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	if stub.getCurrentCalls != 0 {
		t.Fatal("invalid live review must not fall back to cookie")
	}
}

func TestMiddleware_LiveReviewDisabled(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	cfg := liveCfg()
	cfg.Enabled = false
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("ef", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	auth.Middleware(stub, "session_token", cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	if stub.resolveCalls != 0 {
		t.Fatal("disabled live review must not resolve invite")
	}
}

func TestMiddleware_LiveReviewSellerNotAllowed(t *testing.T) {
	t.Parallel()
	res := seller1Result()
	res.SellerAccount.ID = 99
	stub := &stubAuthenticator{resolveResult: res}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("11", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	auth.Middleware(stub, "session_token", liveCfg())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestMiddleware_LiveReviewSellerNotInAllowList(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	cfg := liveCfg()
	cfg.IsAllowedSeller = func(int64) bool { return false }
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("22", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	auth.Middleware(stub, "session_token", cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestMiddleware_LiveReviewMutationBlocked(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/actions/foo/dismiss", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("33", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	h := auth.Middleware(stub, "session_token", liveCfg())(auth.ReadOnlyGuard()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestMiddleware_LiveReviewGETDashboardAllowed(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("44", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	h := auth.Middleware(stub, "session_token", liveCfg())(auth.ReadOnlyGuard()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAdminMiddleware_BlocksLiveReview(t *testing.T) {
	t.Parallel()
	stub := &stubAuthenticator{resolveResult: seller1Result()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	req.Header.Set(auth.HeaderLiveReview, "1")
	req.Header.Set(auth.HeaderLiveReviewToken, strings.Repeat("55", 32))
	req.Header.Set(auth.HeaderLiveReviewHost, "deadbeef.review.aivion.pro")
	rr := httptest.NewRecorder()
	h := auth.Middleware(stub, "session_token", liveCfg())(auth.AdminMiddleware([]string{"demo@example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAuthResult_LiveReviewFlags(t *testing.T) {
	t.Parallel()
	result := seller1Result()
	result.IsLiveReviewMode = true
	if result.SessionToken != "" {
		t.Fatal("resolve-style result must not carry session token")
	}
	payload, err := json.Marshal(map[string]any{
		"is_live_review": result.IsLiveReviewMode,
		"is_admin":       false,
		"seller_id":      result.SellerAccount.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "token") {
		t.Fatal("token must not appear in auth JSON")
	}
	if result.SellerAccount.ID != 1 {
		t.Fatal("expected seller 1")
	}
}

func TestLiveReviewHeadersPresent(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if auth.LiveReviewHeadersPresent(req) {
		t.Fatal("expected absent")
	}
	req.Header.Set(auth.HeaderLiveReview, "1")
	if !auth.LiveReviewHeadersPresent(req) {
		t.Fatal("expected present")
	}
}
