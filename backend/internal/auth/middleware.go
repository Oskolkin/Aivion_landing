package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// Authenticator is the auth-service surface used by Middleware.
type Authenticator interface {
	GetCurrentUser(ctx context.Context, sessionToken string) (*AuthResult, error)
	ResolveReviewInvite(ctx context.Context, rawToken string) (*AuthResult, error)
}

// Middleware authenticates requests via session cookie.
// Optional LiveReviewConfig enables header-based stateless live review auth
// that is attempted before cookie auth when live-review headers are present.
// Invalid live-review headers never fall back to cookie auth.
func Middleware(service Authenticator, cookieName string, liveReview ...*LiveReviewConfig) func(http.Handler) http.Handler {
	var liveCfg *LiveReviewConfig
	if len(liveReview) > 0 {
		liveCfg = liveReview[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if LiveReviewHeadersPresent(r) {
				if err := authenticateLiveReview(w, r, service, liveCfg); err != nil {
					writeLiveReviewAuthError(w, r, liveCfg, err)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			result, err := service.GetCurrentUser(r.Context(), cookie.Value)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			ctx := WithAuthContext(r.Context(), result.User, result.SellerAccount)
			ctx = WithSessionFlags(ctx, result.ReadOnly, result.IsReviewMode)
			ctx = WithLiveReviewMode(ctx, false)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticateLiveReview(w http.ResponseWriter, r *http.Request, service Authenticator, cfg *LiveReviewConfig) error {
	host := liveReviewHostFromRequest(r)
	method := r.Method
	path := r.URL.Path

	if cfg == nil || !cfg.Enabled {
		emitLiveReviewEvent(cfg, LiveReviewEvent{
			Name:     "live_review_auth_failed",
			Hostname: host,
			Method:   method,
			Path:     path,
			Status:   http.StatusUnauthorized,
			Reason:   "disabled",
		})
		return ErrLiveReviewDisabled
	}

	token := liveReviewTokenFromRequest(r)
	if token == "" || host == "" {
		emitLiveReviewEvent(cfg, LiveReviewEvent{
			Name:     "live_review_auth_failed",
			Hostname: host,
			Method:   method,
			Path:     path,
			Status:   http.StatusUnauthorized,
			Reason:   "missing_headers",
		})
		return ErrUnauthorized
	}

	if !hostMatchesLiveReviewSuffix(host, cfg.AllowedHostSuffix) {
		emitLiveReviewEvent(cfg, LiveReviewEvent{
			Name:     "live_review_auth_failed",
			Hostname: host,
			Method:   method,
			Path:     path,
			Status:   http.StatusUnauthorized,
			Reason:   "host_mismatch",
		})
		return ErrLiveReviewHost
	}

	result, err := service.ResolveReviewInvite(r.Context(), token)
	if err != nil {
		reason := "invalid"
		switch {
		case errors.Is(err, ErrReviewInviteExpired):
			reason = "expired"
			emitLiveReviewEvent(cfg, LiveReviewEvent{
				Name:     "live_review_expired",
				Hostname: host,
				Method:   method,
				Path:     path,
				Status:   http.StatusUnauthorized,
				Reason:   reason,
			})
		case errors.Is(err, ErrReviewInviteRevoked):
			reason = "revoked"
			emitLiveReviewEvent(cfg, LiveReviewEvent{
				Name:     "live_review_revoked",
				Hostname: host,
				Method:   method,
				Path:     path,
				Status:   http.StatusUnauthorized,
				Reason:   reason,
			})
		default:
			emitLiveReviewEvent(cfg, LiveReviewEvent{
				Name:     "live_review_auth_failed",
				Hostname: host,
				Method:   method,
				Path:     path,
				Status:   http.StatusUnauthorized,
				Reason:   reason,
			})
		}
		return err
	}

	sellerID := int64(0)
	if result.SellerAccount != nil {
		sellerID = result.SellerAccount.ID
	}
	if cfg.AllowedSellerAccountID <= 0 || sellerID != cfg.AllowedSellerAccountID {
		emitLiveReviewEvent(cfg, LiveReviewEvent{
			Name:            "live_review_auth_failed",
			InviteID:        result.ReviewInviteID,
			SellerAccountID: sellerID,
			Hostname:        host,
			Method:          method,
			Path:            path,
			Status:          http.StatusForbidden,
			Reason:          "seller_not_allowed",
		})
		return ErrLiveReviewForbidden
	}
	if cfg.IsAllowedSeller == nil || !cfg.IsAllowedSeller(sellerID) {
		emitLiveReviewEvent(cfg, LiveReviewEvent{
			Name:            "live_review_auth_failed",
			InviteID:        result.ReviewInviteID,
			SellerAccountID: sellerID,
			Hostname:        host,
			Method:          method,
			Path:            path,
			Status:          http.StatusForbidden,
			Reason:          "seller_not_in_demo_allowlist",
		})
		return ErrLiveReviewForbidden
	}

	result.IsLiveReviewMode = true
	result.ReadOnly = true
	result.IsReviewMode = true

	ctx := WithAuthContext(r.Context(), result.User, result.SellerAccount)
	ctx = WithSessionFlags(ctx, true, true)
	ctx = WithLiveReviewMode(ctx, true)
	ctx = WithReviewInviteID(ctx, result.ReviewInviteID)
	ctx = WithLiveReviewConfig(ctx, cfg)
	*r = *r.WithContext(ctx)

	setLiveReviewResponseHeaders(w)
	emitLiveReviewEvent(cfg, LiveReviewEvent{
		Name:            "live_review_auth_success",
		InviteID:        result.ReviewInviteID,
		SellerAccountID: sellerID,
		Hostname:        host,
		Method:          method,
		Path:            path,
		Status:          http.StatusOK,
	})
	return nil
}

func writeLiveReviewAuthError(w http.ResponseWriter, _ *http.Request, _ *LiveReviewConfig, err error) {
	setLiveReviewResponseHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusUnauthorized
	msg := "unauthorized"
	if errors.Is(err, ErrLiveReviewForbidden) {
		status = http.StatusForbidden
		msg = "forbidden"
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

func RequireSellerAccount() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := SellerAccountFromContext(r.Context()); !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"seller account required"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func IsAdminUser(user *dbgen.User, adminEmails []string) bool {
	if user == nil || user.Email == "" || len(adminEmails) == 0 {
		return false
	}

	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email == "" {
		return false
	}

	for _, adminEmail := range adminEmails {
		if strings.EqualFold(strings.TrimSpace(adminEmail), email) {
			return true
		}
	}

	return false
}

func AdminMiddleware(adminEmails []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ReadOnlyFromContext(r.Context()) || ReviewModeFromContext(r.Context()) || LiveReviewModeFromContext(r.Context()) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"admin access required"}`))
				return
			}
			user, ok := UserFromContext(r.Context())
			if !ok || !IsAdminUser(&user, adminEmails) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"admin access required"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ReadOnlyGuard blocks mutating HTTP methods for review/read-only sessions.
// Safe exceptions: logout and chat session lookup (opens existing context without writing AI turns).
func ReadOnlyGuard() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ReadOnlyFromContext(r.Context()) && !ReviewModeFromContext(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			method := strings.ToUpper(r.Method)
			path := r.URL.Path
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if method == http.MethodPost && (path == "/api/v1/auth/logout" || path == "/api/v1/chat/sessions/lookup-by-action") {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			if LiveReviewModeFromContext(r.Context()) {
				w.Header().Set(HeaderLiveReview, "true")
				emitLiveReviewEvent(liveReviewConfigFromContext(r.Context()), LiveReviewEvent{
					Name:            "live_review_write_blocked",
					InviteID:        ReviewInviteIDFromContext(r.Context()),
					SellerAccountID: sellerIDFromContext(r.Context()),
					Hostname:        liveReviewHostFromRequest(r),
					Method:          method,
					Path:            path,
					Status:          http.StatusForbidden,
					Reason:          "read_only",
				})
			}
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"review session is read-only"}`))
		})
	}
}

func sellerIDFromContext(ctx context.Context) int64 {
	if sa, ok := SellerAccountFromContext(ctx); ok {
		return sa.ID
	}
	return 0
}
