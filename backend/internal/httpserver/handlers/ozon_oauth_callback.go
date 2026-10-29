package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
)

const (
	ozonOAuthRedirectURI  = "https://aivion.pro/oauth/callback"
	ozonOAuthAuthorizeURL = "https://seller.ozon.ru/app/appstore/oauth/authorize"
)

// OzonOAuthCallback completes the production seller-bound private application
// authorization-code flow. Tokens are handed directly to encrypted persistence
// and never enter response DTOs or diagnostic artifacts.
type OzonOAuthCallback struct {
	client  *http.Client
	service privateOAuthCompleter
}

type privateOAuthCompleter interface {
	CompletePrivateOAuth(context.Context, int64, string, string) error
}

func NewOzonOAuthCallback(service privateOAuthCompleter) *OzonOAuthCallback {
	return &OzonOAuthCallback{client: &http.Client{Timeout: 30 * time.Second}, service: service}
}

func (h *OzonOAuthCallback) Start(w http.ResponseWriter, r *http.Request) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		oauthPage(w, http.StatusUnauthorized, "Авторизация Ozon не выполнена.")
		return
	}
	clientID := strings.TrimSpace(os.Getenv("OZON_OAUTH_CLIENT_ID"))
	if clientID == "" || strings.TrimSpace(os.Getenv("OZON_OAUTH_CLIENT_SECRET")) == "" {
		oauthPage(w, http.StatusServiceUnavailable, "Авторизация Ozon не выполнена.")
		return
	}
	state, err := newOAuthState(seller.ID)
	if err != nil {
		oauthPage(w, http.StatusInternalServerError, "Авторизация Ozon не выполнена.")
		return
	}
	if !setStateCookie(w, state) {
		oauthPage(w, http.StatusServiceUnavailable, "Авторизация Ozon не выполнена.")
		return
	}
	// Coverage consent requests documented read/data roles only.  In particular,
	// neither Seller nor Performance admin/write access can enter this flow.
	q := url.Values{"response_type": {"code"}, "access_type": {"offline"}, "client_id": {clientID}, "redirect_uri": {ozonOAuthRedirectURI}, "scope": {strings.Join(requestedCoverageScopes, " ")}, "state": {state}, "prompt": {"select_company"}}
	http.Redirect(w, r, ozonOAuthAuthorizeURL+"?"+strings.ReplaceAll(q.Encode(), "+", "%20"), http.StatusFound)
}

func (h *OzonOAuthCallback) Get(w http.ResponseWriter, r *http.Request) {
	receivedState := r.URL.Query().Get("state")
	if valid, classification := h.consumeState(w, r, receivedState); !valid {
		_ = classification
		oauthPage(w, http.StatusBadRequest, "Авторизация Ozon не выполнена.")
		return
	}
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		_ = safeOAuthLabel(oauthError)
		oauthPage(w, http.StatusBadRequest, "Авторизация Ozon не выполнена.")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		oauthPage(w, http.StatusBadRequest, "Авторизация Ozon не выполнена.")
		return
	}
	sellerID, err := sellerIDFromOAuthState(receivedState)
	if err != nil || h.service == nil {
		oauthPage(w, http.StatusBadRequest, "Авторизация Ozon не выполнена.")
		return
	}
	if err := h.service.CompletePrivateOAuth(r.Context(), sellerID, code, ozonOAuthRedirectURI); err != nil {
		oauthPage(w, http.StatusBadGateway, "Авторизация Ozon не выполнена.")
		return
	}
	http.Redirect(w, r, "/app/integrations/ozon?oauth=success", http.StatusSeeOther)
}

func (h *OzonOAuthCallback) consumeState(w http.ResponseWriter, r *http.Request, received string) (bool, string) {
	if received == "" {
		return false, "MISSING_STATE"
	}
	cookie, err := r.Cookie("aivion_ozon_state")
	if err != nil {
		return false, "MISSING_STATE_COOKIE"
	}
	if !validStateCookie(cookie.Value, received) {
		return false, "STATE_MISMATCH"
	}
	http.SetCookie(w, &http.Cookie{Name: "aivion_ozon_state", Value: "", Path: "/oauth/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	return true, ""
}

func newOAuthState(sellerID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	payload := strconv.FormatInt(sellerID, 10) + ":" + base64.RawURLEncoding.EncodeToString(b)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)), nil
}

func sellerIDFromOAuthState(state string) (int64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return 0, err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, fmt.Errorf("invalid OAuth state")
	}
	sellerID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || sellerID <= 0 {
		return 0, fmt.Errorf("invalid OAuth seller")
	}
	return sellerID, nil
}

func setStateCookie(w http.ResponseWriter, state string) bool {
	signature, ok := stateSignature(state)
	if !ok {
		return false
	}
	http.SetCookie(w, &http.Cookie{Name: "aivion_ozon_state", Value: state + "." + signature, Path: "/oauth/", MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	return true
}

func validStateCookie(value, received string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(received)) != 1 {
		return false
	}
	expected, ok := stateSignature(received)
	return ok && subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) == 1
}

func stateSignature(state string) (string, bool) {
	path := strings.TrimSpace(os.Getenv("JWT_SECRET_FILE"))
	var key []byte
	var err error
	if path != "" {
		key, err = os.ReadFile(path)
	} else {
		key = []byte(strings.TrimSpace(os.Getenv("JWT_SECRET")))
	}
	if err != nil || len(key) == 0 {
		return "", false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(state))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), true
}

func safeOAuthLabel(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return -1
	}, value)
}

func oauthPage(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, text)
}
