package testutil

import "testing"

// RequireSafeTestDatabaseURL fails the test immediately when dsn is not an
// approved destructive-test target. Call this before DROP SCHEMA, migrations
// down, truncate, reset, seed, or any other destructive helper.
func RequireSafeTestDatabaseURL(t *testing.T, dsn string) {
	t.Helper()
	if err := ValidateTestDatabaseURL(dsn); err != nil {
		t.Fatalf("%v", err)
	}
}
