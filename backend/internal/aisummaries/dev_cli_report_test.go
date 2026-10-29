package aisummaries

import (
	"strings"
	"testing"
)

func TestLoadDevRunReport_partialFields_documented(t *testing.T) {
	// DevRunReport is populated from DB; ensure aggregate format matches service output.
	d := newCategoryRunDiagnostics(1)
	d.recordErr(10, "Electronics", categoryFailOpenAI, errExample("rate limit"))
	msg := d.aggregatePartialRunError(7, 7)
	if !strings.Contains(msg, "7/7") || !strings.Contains(msg, "first_error=") {
		t.Fatalf("msg=%q", msg)
	}
}

type errExample string

func (e errExample) Error() string { return string(e) }
