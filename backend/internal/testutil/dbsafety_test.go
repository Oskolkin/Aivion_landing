package testutil

import "testing"

func TestValidateTestDatabaseURL_RejectsDevMarketplaceAI(t *testing.T) {
	err := ValidateTestDatabaseURL("postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai?sslmode=disable")
	if err == nil {
		t.Fatal("expected rejection for marketplace_ai")
	}
	got := err.Error()
	if !contains(got, ErrUnsafeTestDatabaseTarget) || !contains(got, "marketplace_ai") {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(got, "postgres:postgres") || contains(got, "password") {
		t.Fatalf("error must not leak credentials: %v", err)
	}
}

func TestValidateTestDatabaseURL_AcceptsMarketplaceAITest(t *testing.T) {
	if err := ValidateTestDatabaseURL("postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai_test?sslmode=disable"); err != nil {
		t.Fatalf("expected accept: %v", err)
	}
}

func TestValidateTestDatabaseURL_RejectsEmpty(t *testing.T) {
	err := ValidateTestDatabaseURL("")
	if err == nil || !contains(err.Error(), ErrUnsafeTestDatabaseTarget) {
		t.Fatalf("expected empty rejection, got %v", err)
	}
}

func TestValidateTestDatabaseURL_RejectsProductionLike(t *testing.T) {
	cases := []string{
		"postgres://u:p@db.prod.example.com:5432/marketplace_ai_test?sslmode=require",
		"postgres://u:p@127.0.0.1:55432/marketplace_ai_prod?sslmode=disable",
		"postgres://u:p@127.0.0.1:55432/production?sslmode=disable",
	}
	for _, dsn := range cases {
		if err := ValidateTestDatabaseURL(dsn); err == nil {
			t.Fatalf("expected rejection for %s", redactDSN(dsn))
		}
	}
}

func TestRequireSafeTestDatabaseURL_FailsBeforeDestructiveWork(t *testing.T) {
	destructiveRan := false
	runDestructive := func(dsn string) error {
		if err := ValidateTestDatabaseURL(dsn); err != nil {
			return err
		}
		destructiveRan = true
		return nil
	}
	err := runDestructive("postgres://postgres:secret@127.0.0.1:55432/marketplace_ai?sslmode=disable")
	if err == nil {
		t.Fatal("expected unsafe_test_database_target")
	}
	if !contains(err.Error(), ErrUnsafeTestDatabaseTarget) {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(err.Error(), "secret") {
		t.Fatalf("error must not leak password: %v", err)
	}
	if destructiveRan {
		t.Fatal("destructive helper must not run after unsafe DSN guard failure")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

func redactDSN(dsn string) string {
	// Keep test output free of credentials even if a case string embeds them.
	if i := indexOf(dsn, "@"); i >= 0 {
		return "***@" + dsn[i+1:]
	}
	return dsn
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
