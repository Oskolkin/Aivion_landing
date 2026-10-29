package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AuthRateLimiter struct {
	client          redis.Cmdable
	loginIPLimit    int
	loginEmailLimit int
	registerIPLimit int
	window          time.Duration
	failOpen        bool
	log             *zap.Logger
}

func NewAuthRateLimiter(client redis.Cmdable, loginIP, loginEmail, registerIP int, window time.Duration, failOpen bool, log *zap.Logger) *AuthRateLimiter {
	return &AuthRateLimiter{
		client:          client,
		loginIPLimit:    loginIP,
		loginEmailLimit: loginEmail,
		registerIPLimit: registerIP,
		window:          window,
		failOpen:        failOpen,
		log:             log,
	}
}

func (l *AuthRateLimiter) WrapLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if l != nil && l.loginIPLimit > 0 {
			if blocked, retryAfter := l.check(r.Context(), "auth:login:ip:"+clientIP(r), l.loginIPLimit); blocked {
				writeRateLimited(w, retryAfter)
				return
			}
		}
		next(w, r)
	}
}

func (l *AuthRateLimiter) WrapRegister(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if l != nil && l.registerIPLimit > 0 {
			if blocked, retryAfter := l.check(r.Context(), "auth:register:ip:"+clientIP(r), l.registerIPLimit); blocked {
				writeRateLimited(w, retryAfter)
				return
			}
		}
		next(w, r)
	}
}

func (l *AuthRateLimiter) check(ctx context.Context, key string, limit int) (bool, int) {
	if l.client == nil {
		return false, 0
	}
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		if l.log != nil {
			l.log.Warn("auth rate limit redis error", zap.String("key", key), zap.Error(err))
		}
		return !l.failOpen, int(l.window.Seconds())
	}
	if count == 1 {
		_ = l.client.Expire(ctx, key, l.window).Err()
	}
	if count > int64(limit) {
		ttl, _ := l.client.TTL(ctx, key).Result()
		retry := int(ttl.Seconds())
		if retry <= 0 {
			retry = int(l.window.Seconds())
		}
		return true, retry
	}
	return false, 0
}

func writeRateLimited(w http.ResponseWriter, retryAfter int) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// CheckLoginEmail enforces per-email login rate limits after credentials are parsed.
func (l *AuthRateLimiter) CheckLoginEmail(ctx context.Context, email string) (bool, int) {
	if l == nil || l.loginEmailLimit <= 0 {
		return false, 0
	}
	key := loginEmailRedisKey(email)
	if key == "" {
		return false, 0
	}
	return l.check(ctx, key, l.loginEmailLimit)
}
