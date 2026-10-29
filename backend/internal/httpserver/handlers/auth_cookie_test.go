package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestSetAuthCookieSecureFlag(t *testing.T) {
	t.Run("local_default_insecure", func(t *testing.T) {
		rr := httptest.NewRecorder()
		setAuthCookie(rr, "session_token", "tok", false)
		cookies := rr.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies len = %d, want 1", len(cookies))
		}
		if cookies[0].Secure {
			t.Fatal("expected Secure=false for local development")
		}
	})

	t.Run("staging_production_secure", func(t *testing.T) {
		rr := httptest.NewRecorder()
		setAuthCookie(rr, "session_token", "tok", true)
		cookies := rr.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].Secure {
			t.Fatal("expected Secure=true when SESSION_COOKIE_SECURE is enabled")
		}
	})
}
