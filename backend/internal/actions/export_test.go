package actions

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSanitizeExportCell_FormulaInjection(t *testing.T) {
	cases := []string{"=1+1", "+cmd", "-10", "@evil", "  =SUM(A1:A2)", "\t+X"}
	for _, c := range cases {
		got := sanitizeExportCell(c)
		if !strings.HasPrefix(got, "'") {
			t.Fatalf("expected formula escape for %q, got %q", c, got)
		}
	}
	if got := sanitizeExportCell("normal text"); got != "normal text" {
		t.Fatalf("unexpected sanitize for normal text: %q", got)
	}
}

func TestRenderActionExportXLSX_SanitizesFormulaCells(t *testing.T) {
	item := ActionItem{
		ActionKey:      "k1",
		Type:           ActionTypeStockReplenishment,
		Category:       ActionCategoryStock,
		Priority:       ActionPriorityCritical,
		Status:         ActionStatusOpen,
		Title:          "=HYPERLINK(\"http://evil\")",
		Reason:         "+SUM(A1:A2)",
		NextStep:       "@cmd",
		ExpectedEffect: "-1",
		MetricDate:     "2026-06-24",
		Limitations:    []string{"=warn"},
		EntityGroupRef: "group",
		AffectedEntities: ActionAffectedEntities{
			SKUs: []ActionSKURef{{SKU: "=sku"}},
		},
	}
	data, _, err := renderActionExportXLSX(item, nil)
	if err != nil {
		t.Fatalf("render xlsx: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer func() { _ = f.Close() }()
	v, err := f.GetCellValue("Действие", "B3")
	if err != nil {
		t.Fatalf("get cell: %v", err)
	}
	if !strings.HasPrefix(v, "'=") {
		t.Fatalf("expected escaped formula in title, got %q", v)
	}
}
