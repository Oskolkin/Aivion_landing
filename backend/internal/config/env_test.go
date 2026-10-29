package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		line   string
		key    string
		value  string
		wantOK bool
	}{
		{"", "", "", false},
		{"# comment", "", "", false},
		{"APP_ENV=local", "APP_ENV", "local", true},
		{`DATABASE_URL="postgres://u:p@h:55432/db?sslmode=disable"`, "DATABASE_URL", "postgres://u:p@h:55432/db?sslmode=disable", true},
		{"OPENAI_API_KEY='sk-test'", "OPENAI_API_KEY", "sk-test", true},
		{"NAME=value=with=equals", "NAME", "value=with=equals", true},
		{"AI_PROXY_URL=http://127.0.0.1:10809", "AI_PROXY_URL", "http://127.0.0.1:10809", true},
	}

	for _, tt := range tests {
		key, value, ok := ParseEnvLine(tt.line)
		if ok != tt.wantOK || key != tt.key || value != tt.value {
			t.Fatalf("ParseEnvLine(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.line, key, value, ok, tt.key, tt.value, tt.wantOK)
		}
	}
}

func TestLoadEnvFileDoesNotOverrideProcessEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	if err := os.WriteFile(path, []byte("APP_ENV=from_file\nDATABASE_URL=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "process")
	// CI/workflows often inject DATABASE_URL; clear it so the file value can load.
	origDB, hadDB := os.LookupEnv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() {
		if hadDB {
			_ = os.Setenv("DATABASE_URL", origDB)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	})
	if err := loadEnvFile(path, false); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("APP_ENV"); got != "process" {
		t.Fatalf("APP_ENV = %q, want process", got)
	}
	if got := os.Getenv("DATABASE_URL"); got != "from_file" {
		t.Fatalf("DATABASE_URL = %q, want from_file", got)
	}
}

func TestLoadEnvFileOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	if err := os.WriteFile(path, []byte("APP_ENV=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "process")
	if err := loadEnvFile(path, true); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("APP_ENV"); got != "from_file" {
		t.Fatalf("APP_ENV = %q, want from_file", got)
	}
}

func TestShouldSkipEnvFileLoad(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if !shouldSkipEnvFileLoad() {
		t.Fatal("expected production to skip env file load")
	}
	t.Setenv("APP_ENV", "local")
	if shouldSkipEnvFileLoad() {
		t.Fatal("expected local to allow env file load")
	}
}
