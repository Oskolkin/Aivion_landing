package aisummaries

import (
	"errors"
	"strings"
	"testing"
)

func TestCategoryRunDiagnostics_aggregatePartialRunError(t *testing.T) {
	d := newCategoryRunDiagnostics(1)
	d.record(42, "Shoes", categoryFailOpenAI, "model not found")

	msg := d.aggregatePartialRunError(7, 7)
	if !strings.Contains(msg, "category summaries failed: 7/7") {
		t.Fatalf("msg=%q", msg)
	}
	if !strings.Contains(msg, "first_error=") {
		t.Fatalf("msg=%q", msg)
	}
	if !strings.Contains(msg, "category_id=42") {
		t.Fatalf("msg=%q", msg)
	}
	if !strings.Contains(msg, "openai_error") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestCategoryRunDiagnostics_aggregatePartialRunError_noFailures(t *testing.T) {
	d := newCategoryRunDiagnostics(1)
	if d.aggregatePartialRunError(0, 7) != "" {
		t.Fatal("expected empty")
	}
}

func TestCategoryRunDiagnostics_recordErr(t *testing.T) {
	d := newCategoryRunDiagnostics(99)
	d.recordErr(1, "Cat", categoryFailSave, errors.New("insert failed"))
	if len(d.failures) != 1 {
		t.Fatalf("failures=%d", len(d.failures))
	}
	if d.failures[0].Kind != categoryFailSave {
		t.Fatalf("kind=%s", d.failures[0].Kind)
	}
}
