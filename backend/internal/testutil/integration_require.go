//go:build integration

package testutil

import (
	"os"
	"testing"
)

// RequireIntegrationDatabase returns TEST_DATABASE_URL after fail-fast safety
// checks. It never falls back to DATABASE_URL (dev/prod). Unsafe targets
// fail immediately with unsafe_test_database_target before any destructive work.
func RequireIntegrationDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatal("TEST_DATABASE_URL is required for integration tests in CI")
		}
		t.Skip("set TEST_DATABASE_URL to run integration tests")
		return ""
	}
	RequireSafeTestDatabaseURL(t, dsn)
	return dsn
}
