package testutil

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrUnsafeTestDatabaseTarget is returned when a DSN points at a non-test database.
const ErrUnsafeTestDatabaseTarget = "unsafe_test_database_target"

// approvedTestDatabaseNames is an explicit allowlist for integration / destructive tests.
// Names ending in "_test" are also accepted without listing them here.
var approvedTestDatabaseNames = map[string]struct{}{
	"marketplace_ai_test": {},
}

// approvedTestHosts restricts destructive tests to local / CI postgres endpoints.
var approvedTestHosts = map[string]struct{}{
	"127.0.0.1": {},
	"localhost": {},
	"::1":       {},
	"postgres":  {}, // docker-compose service name in CI
}

// ValidateTestDatabaseURL fail-fast-validates that dsn is safe for destructive
// integration tests (schema reset, truncate, migrations down, etc.).
//
// Rules (all must hold):
//   - dsn is non-empty
//   - database name ends with "_test" OR is on the approved allowlist
//   - host is on the approved local/CI allowlist
//
// It never rewrites DATABASE_URL into a test URL. Password is never included
// in the returned error message.
func ValidateTestDatabaseURL(dsn string) error {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return fmt.Errorf("%s: TEST_DATABASE_URL is empty", ErrUnsafeTestDatabaseTarget)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("%s: cannot parse TEST_DATABASE_URL: %v", ErrUnsafeTestDatabaseTarget, err)
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if i := strings.IndexByte(dbName, '/'); i >= 0 {
		dbName = dbName[:i]
	}
	dbName = strings.TrimSpace(dbName)
	if dbName == "" {
		return fmt.Errorf("%s: database name is missing in TEST_DATABASE_URL", ErrUnsafeTestDatabaseTarget)
	}
	if !isApprovedTestDatabaseName(dbName) {
		return fmt.Errorf("%s: database %q is not an approved test database (must end with _test or be allowlisted)", ErrUnsafeTestDatabaseTarget, dbName)
	}
	host := u.Hostname()
	if host == "" {
		// postgres://user:pass@/dbname?host=/var/run/postgresql
		if h := u.Query().Get("host"); h != "" {
			host = h
		}
	}
	if host == "" {
		return fmt.Errorf("%s: host is missing for database %q", ErrUnsafeTestDatabaseTarget, dbName)
	}
	// Strip IPv6 brackets if present via Hostname(); handle port-only leftovers.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if !isApprovedTestHost(host) {
		return fmt.Errorf("%s: host %q is not approved for destructive tests (database %q)", ErrUnsafeTestDatabaseTarget, host, dbName)
	}
	return nil
}

func isApprovedTestDatabaseName(name string) bool {
	if _, ok := approvedTestDatabaseNames[name]; ok {
		return true
	}
	return strings.HasSuffix(name, "_test")
}

func isApprovedTestHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	_, ok := approvedTestHosts[host]
	return ok
}
