//go:build integration

package analysis_test

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// integrationRedisURL returns TEST_REDIS_URL or the local docker-compose Redis default.
func integrationRedisURL(t *testing.T) string {
	t.Helper()
	if u := strings.TrimSpace(os.Getenv("TEST_REDIS_URL")); u != "" {
		return u
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("TEST_REDIS_URL unset and local Redis unavailable at 127.0.0.1:6379: %v", err)
	}
	_ = conn.Close()
	return "redis://127.0.0.1:6379/15"
}
