package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSecretFromFileOrEnv_fromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("sk-from-file"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "sk-from-env")

	got, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sk-from-file" {
		t.Fatalf("got %q, want file value", got)
	}
}

func TestLoadSecretFromFileOrEnv_fileTakesPriorityOverEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("priority-file-key"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "should-not-win")

	got, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if got != "priority-file-key" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadSecretFromFileOrEnv_missingFile(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("TEST_SECRET", "fallback")

	_, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "fallback") || strings.Contains(err.Error(), "sk-") {
		t.Fatalf("error must not leak secret-like values: %v", err)
	}
}

func TestLoadSecretFromFileOrEnv_emptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SECRET_FILE", path)
	_, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSecretFromFileOrEnv_trimTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("sk-trimmed\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_SECRET_FILE", path)
	got, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-trimmed" {
		t.Fatalf("got %q, want trimmed value", got)
	}
}

func TestLoadSecretFromFileOrEnv_envFallbackWhenFileUnset(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", "")
	t.Setenv("TEST_SECRET", "env-only-key")

	got, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if got != "env-only-key" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadSecretFromFileOrEnv_errorOmitsSecretValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	secretLike := "sk-proj-super-secret-should-never-appear"
	// Empty after trim: write only newlines so value is empty, then separately assert
	// a missing-file path that includes no secret content.
	if err := os.WriteFile(path, []byte("\n\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", secretLike)

	_, err := LoadSecretFromFileOrEnv("TEST_SECRET_FILE", "TEST_SECRET")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, secretLike) || strings.Contains(msg, "sk-proj") {
		t.Fatalf("error leaked secret: %q", msg)
	}
}

func TestOpenAIAPIKeyConfigured(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY_FILE", "")
	if OpenAIAPIKeyConfigured() {
		t.Fatal("expected false")
	}

	t.Setenv("OPENAI_API_KEY_FILE", "/run/secrets/openai_api_key")
	if !OpenAIAPIKeyConfigured() {
		t.Fatal("expected true when file path set")
	}
}
