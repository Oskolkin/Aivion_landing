package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDockerCompose_doesNotHardcodeFakeProvider(t *testing.T) {
	repoRoot := findRepoRoot(t)
	composePath := filepath.Join(repoRoot, "docker-compose.yml")
	raw, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}

	hardcodedFake := regexp.MustCompile(`(?m)^\s+AI_PROVIDER_MODE:\s*fake\s*$`)
	if hardcodedFake.Find(raw) != nil {
		t.Fatalf("docker-compose.yml must not hardcode AI_PROVIDER_MODE: fake; use ${AI_PROVIDER_MODE:-real} and set AI_PROVIDER_MODE=fake only in local .env for offline dev")
	}

	text := string(raw)
	if !strings.Contains(text, "${AI_PROVIDER_MODE:-real}") {
		t.Fatalf("docker-compose.yml should default AI_PROVIDER_MODE to real via env substitution")
	}
	if !strings.Contains(text, "${OPENAI_MAX_RETRIES:-0}") {
		t.Fatalf("docker-compose.yml should default OPENAI_MAX_RETRIES to 0")
	}
	if !strings.Contains(text, "${AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED:-true}") {
		t.Fatalf("docker-compose.yml should default AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED to true for pilot")
	}
}

func TestEnvPilotExample_doesNotContainRealAPIKey(t *testing.T) {
	repoRoot := findRepoRoot(t)
	path := filepath.Join(repoRoot, ".env.pilot.example")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env.pilot.example: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "sk-") && !strings.Contains(text, "sk-test") {
		t.Fatal(".env.pilot.example must not contain real-looking OpenAI API keys")
	}
	if !strings.Contains(text, "AI_PROVIDER_MODE=real") {
		t.Fatal(".env.pilot.example must set AI_PROVIDER_MODE=real")
	}
	if strings.Contains(text, "AI_PROVIDER_MODE=fake") {
		t.Fatal(".env.pilot.example must not default to fake provider")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing docker-compose.yml")
		}
		dir = parent
	}
}
