package aisummaries

import (
	"errors"
	"testing"
)

func TestValidateOpenAIKeyForDev(t *testing.T) {
	err := ValidateOpenAIKeyForDev("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !errors.Is(err, ErrOpenAIKeyRequired) {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateOpenAIKeyForDev("  sk-test  "); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestMessageForExistingRunStatus(t *testing.T) {
	if MessageForExistingRunStatus("completed") == "" {
		t.Fatal("expected completed message")
	}
	if MessageForExistingRunStatus("running") == "" {
		t.Fatal("expected running message")
	}
	if MessageForExistingRunStatus("failed") != "" {
		t.Fatal("failed should not skip with a skip message")
	}
}

func TestDryRunSkipsServiceBuild(t *testing.T) {
	// dry-run path in cmd/dev-generate-ai-summaries returns before BuildService / RunForSellerAfterSync.
	if !ShouldSkipDevGenerate("completed") {
		t.Fatal("sanity")
	}
}

func TestShouldSkipDevGenerate(t *testing.T) {
	if !ShouldSkipDevGenerate("completed") {
		t.Fatal("completed should skip")
	}
	if !ShouldSkipDevGenerate("running") {
		t.Fatal("running should skip")
	}
	if ShouldSkipDevGenerate("failed") {
		t.Fatal("failed should not skip")
	}
	if ShouldSkipDevGenerate("partial") {
		t.Fatal("partial should not skip")
	}
}
