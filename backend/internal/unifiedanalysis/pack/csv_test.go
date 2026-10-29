package pack

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestRenderCSVHeaderMatchesContract(t *testing.T) {
	table := csvTable{
		Name:    "account_summary.csv",
		Columns: contract.AccountSummaryColumnsV1,
		Rows:    nil,
	}
	out, err := renderCSV(table)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	lines := strings.Split(string(out), "\n")
	if lines[0] != strings.Join(contract.AccountSummaryColumnsV1, ",") {
		t.Fatalf("header=%q", lines[0])
	}
	if strings.Contains(string(out), "\r") {
		t.Fatal("csv output must use LF, not CRLF")
	}
}

func TestRenderCSVQuotesFieldsWithSpecialChars(t *testing.T) {
	table := csvTable{
		Name:    "sku_summary.csv",
		Columns: contract.SKUSummaryColumnsV1,
		Rows: [][]string{
			padRow(t, contract.SKUSummaryColumnsV1, map[int]string{
				4: `Title, with comma`,
			}),
			padRow(t, contract.SKUSummaryColumnsV1, map[int]string{
				4: "Title with \"quotes\"",
			}),
		},
	}
	out, err := renderCSV(table)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"Title, with comma"`) {
		t.Fatalf("expected quoted comma field, got: %s", s)
	}
	if !strings.Contains(s, `"Title with ""quotes"""`) {
		t.Fatalf("expected escaped quotes field, got: %s", s)
	}
}

func TestRenderCSVRejectsRowLengthMismatch(t *testing.T) {
	table := csvTable{
		Name:    "account_summary.csv",
		Columns: contract.AccountSummaryColumnsV1,
		Rows:    [][]string{{"only", "two"}},
	}
	if _, err := renderCSV(table); err == nil {
		t.Fatal("expected error for row/column length mismatch")
	}
}

func TestRenderCSVEmptyCellForNil(t *testing.T) {
	table := csvTable{
		Name:    "sku_summary.csv",
		Columns: contract.SKUSummaryColumnsV1,
		Rows: [][]string{
			padRow(t, contract.SKUSummaryColumnsV1, nil),
		},
	}
	out, err := renderCSV(table)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header+1 row, got %d lines", len(lines))
	}
	fields := strings.Split(lines[1], ",")
	if len(fields) != len(contract.SKUSummaryColumnsV1) {
		t.Fatalf("fields=%d want=%d", len(fields), len(contract.SKUSummaryColumnsV1))
	}
}

// padRow builds a full-width row of empty strings for cols, applying overrides at given indexes.
func padRow(t *testing.T, cols []string, overrides map[int]string) []string {
	t.Helper()
	row := make([]string, len(cols))
	for i, v := range overrides {
		row[i] = v
	}
	return row
}
