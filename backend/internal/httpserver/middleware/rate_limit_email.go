package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
)

func loginEmailRedisKey(email string) string {
	normalized := auth.NormalizeEmail(email)
	if normalized == "" {
		return ""
	}
	secret := strings.TrimSpace(os.Getenv("AUTH_RATE_LIMIT_HMAC_KEY"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if secret == "" {
		secret = "local-dev-rate-limit-hmac"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(normalized))
	return "auth:login:email:" + hex.EncodeToString(mac.Sum(nil))
}
