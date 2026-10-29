package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

type LoginRateLimiter interface {
	CheckLoginEmail(ctx context.Context, email string) (blocked bool, retryAfter int)
}

type AuthHandler struct {
	authService         *auth.Service
	cookieName          string
	adminEmails         []string
	sessionCookieSecure bool
	publicRegistration  bool
	loginRateLimiter    LoginRateLimiter
	demoSellerChecker   DemoSellerChecker
	publicDemoMode      bool
}

// DemoSellerChecker reports whether a seller is on the staging/public demo allow-list.
type DemoSellerChecker interface {
	IsStagingDemoSeller(sellerID int64) bool
}

func NewAuthHandler(authService *auth.Service, cookieName string, adminEmails []string, sessionCookieSecure bool) *AuthHandler {
	return &AuthHandler{
		authService:         authService,
		cookieName:          cookieName,
		adminEmails:         adminEmails,
		sessionCookieSecure: sessionCookieSecure,
		publicRegistration:  true,
	}
}

func (h *AuthHandler) WithPublicRegistration(enabled bool) *AuthHandler {
	h.publicRegistration = enabled
	return h
}

func (h *AuthHandler) WithDemoSellerChecker(checker DemoSellerChecker) *AuthHandler {
	h.demoSellerChecker = checker
	return h
}

func (h *AuthHandler) WithPublicDemoMode(enabled bool) *AuthHandler {
	h.publicDemoMode = enabled
	return h
}

func (h *AuthHandler) PublicRegistrationEnabled() bool {
	return h.publicRegistration
}

func (h *AuthHandler) WithLoginRateLimiter(limiter LoginRateLimiter) *AuthHandler {
	h.loginRateLimiter = limiter
	return h
}

func (h *AuthHandler) RegisterDisabled(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusForbidden, "public registration is disabled")
}

func (h *AuthHandler) PublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_registration": h.publicRegistration,
		"public_demo_mode":    h.publicDemoMode,
	})
}

type registerRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sellerAccountResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type authResponse struct {
	User struct {
		ID     int64  `json:"id"`
		Email  string `json:"email"`
		Status string `json:"status"`
	} `json:"user"`
	SellerAccount *sellerAccountResponse `json:"seller_account"`
	IsAdmin       bool                   `json:"is_admin"`
	IsDemoSeller  bool                   `json:"is_demo_seller"`
	IsReviewMode  bool                   `json:"is_review_mode"`
	IsLiveReview  bool                   `json:"is_live_review"`
	ReadOnly      bool                   `json:"read_only"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.authService.Register(r.Context(), auth.RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		PasswordConfirm: req.PasswordConfirm,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			writeJSONError(w, http.StatusConflict, "email already exists")
		default:
			writeJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	setAuthCookie(w, h.cookieName, result.SessionToken, h.sessionCookieSecure)

	writeJSON(w, http.StatusCreated, h.buildAuthResponse(result))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if h.loginRateLimiter != nil {
		if blocked, retryAfter := h.loginRateLimiter.CheckLoginEmail(r.Context(), req.Email); blocked {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
			return
		}
	}

	result, err := h.authService.Login(r.Context(), auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, auth.ErrUnauthorized):
			writeJSONError(w, http.StatusForbidden, "user is not active")
		case errors.Is(err, auth.ErrSellerAccountRequired):
			writeJSONError(w, http.StatusForbidden, "seller account required")
		default:
			writeJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	setAuthCookie(w, h.cookieName, result.SessionToken, h.sessionCookieSecure)

	writeJSON(w, http.StatusOK, h.buildAuthResponse(result))
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result := &auth.AuthResult{
		User:             user,
		ReadOnly:         auth.ReadOnlyFromContext(r.Context()),
		IsReviewMode:     auth.ReviewModeFromContext(r.Context()),
		IsLiveReviewMode: auth.LiveReviewModeFromContext(r.Context()),
	}
	if sellerAccount, ok := auth.SellerAccountFromContext(r.Context()); ok {
		result.SellerAccount = &sellerAccount
	}

	if result.IsReviewMode || result.IsLiveReviewMode {
		setReviewSecurityHeaders(w)
	}
	if result.IsLiveReviewMode {
		w.Header().Set(auth.HeaderLiveReview, "true")
	}
	writeJSON(w, http.StatusOK, h.buildAuthResponse(result))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookieName)
	if err != nil || cookie.Value == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.authService.Logout(r.Context(), cookie.Value); err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	clearAuthCookie(w, h.cookieName, h.sessionCookieSecure)

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "logged_out",
	})
}

func (h *AuthHandler) buildAuthResponse(result *auth.AuthResult) authResponse {
	resp := authResponse{
		IsAdmin:      auth.IsAdminUser(&result.User, h.adminEmails) && !result.IsReviewMode && !result.ReadOnly && !result.IsLiveReviewMode,
		IsReviewMode: result.IsReviewMode,
		IsLiveReview: result.IsLiveReviewMode,
		ReadOnly:     result.ReadOnly || result.IsReviewMode || result.IsLiveReviewMode,
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
		if h.demoSellerChecker != nil {
			resp.IsDemoSeller = h.demoSellerChecker.IsStagingDemoSeller(result.SellerAccount.ID)
		}
	}

	return resp
}

func (h *AuthHandler) RedeemReviewInvite(w http.ResponseWriter, r *http.Request) {
	setReviewSecurityHeaders(w)
	rawToken := strings.TrimSpace(chi.URLParam(r, "token"))
	result, err := h.authService.RedeemReviewInvite(r.Context(), rawToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrReviewInviteInvalid),
			errors.Is(err, auth.ErrReviewInviteExpired),
			errors.Is(err, auth.ErrReviewInviteRevoked),
			errors.Is(err, auth.ErrUnauthorized):
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to redeem review invite")
		}
		return
	}

	maxAge := 0
	if !result.SessionExpires.IsZero() {
		maxAge = int(time.Until(result.SessionExpires).Seconds())
		if maxAge < 60 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	setAuthCookieWithMaxAge(w, h.cookieName, result.SessionToken, h.sessionCookieSecure, maxAge)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func setReviewSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func setAuthCookie(w http.ResponseWriter, cookieName, value string, secure bool) {
	setAuthCookieWithMaxAge(w, cookieName, value, secure, 0)
}

func setAuthCookieWithMaxAge(w http.ResponseWriter, cookieName, value string, secure bool, maxAge int) {
	c := &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	if maxAge > 0 {
		c.MaxAge = maxAge
	}
	http.SetCookie(w, c)
}

func clearAuthCookie(w http.ResponseWriter, cookieName string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

// setAnalysisSourceHeaders attaches safe read-path diagnostics for Dashboard/Actions.
func setAnalysisSourceHeaders(w http.ResponseWriter, source string, publicationID, runID int64, metricDate string, fallbackUsed bool, contentStatus string) {
	if source != "" {
		w.Header().Set("X-Aivion-Analysis-Source", source)
	}
	if publicationID > 0 {
		w.Header().Set("X-Aivion-Publication-Id", strconv.FormatInt(publicationID, 10))
	}
	if runID > 0 {
		w.Header().Set("X-Aivion-Run-Id", strconv.FormatInt(runID, 10))
	}
	if metricDate != "" {
		w.Header().Set("X-Aivion-Metric-Date", metricDate)
	}
	if fallbackUsed {
		w.Header().Set("X-Aivion-Fallback-Used", "true")
	} else {
		w.Header().Set("X-Aivion-Fallback-Used", "false")
	}
	if contentStatus != "" {
		w.Header().Set("X-Aivion-Dashboard-Content-Status", contentStatus)
	}
}
