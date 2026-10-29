package chatexport

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestWriteCSVBOMSemicolonAndCyrillic(t *testing.T) {
	table := ExportTable{
		Columns: []string{"sku", "name", "revenue"},
		Rows:    [][]string{{"100", "Товар А", "12.50"}, {"", "Пусто", ""}},
	}
	data, mime, err := WriteFile(FormatCSV, "t", table)
	if err != nil {
		t.Fatal(err)
	}
	if mime != MIMETypeCSV {
		t.Fatalf("mime=%s", mime)
	}
	if len(data) < 3 || data[0] != 0xEF || data[1] != 0xBB || data[2] != 0xBF {
		t.Fatal("expected UTF-8 BOM")
	}
	r := csv.NewReader(bytes.NewReader(data[3:]))
	r.Comma = ';'
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records=%d", len(records))
	}
	if records[1][1] != "Товар А" {
		t.Fatalf("cyrillic=%q", records[1][1])
	}
	if records[2][0] != "" || records[2][2] != "" {
		t.Fatalf("expected empty null cells, got %+v", records[2])
	}
}

func TestWriteXLSXReadableNumbersAndNulls(t *testing.T) {
	table := ExportTable{
		Columns: []string{"count", "label"},
		Rows:    [][]string{{"42", "ok"}, {"", "empty"}},
	}
	data, _, err := WriteFile(FormatXLSX, "risks", table)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 500 {
		t.Fatalf("xlsx too small: %d bytes", len(data))
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	v, err := f.GetCellValue(sheet, "A2")
	if err != nil {
		t.Fatal(err)
	}
	if v != "42" {
		t.Fatalf("A2=%q want 42", v)
	}
	empty, err := f.GetCellValue(sheet, "A3")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(empty) != "" {
		t.Fatalf("A3=%q want empty", empty)
	}
}

func TestCellTypedValue(t *testing.T) {
	if cellTypedValue("") != nil {
		t.Fatal("empty should be nil")
	}
	if f, ok := cellTypedValue("3.14").(float64); !ok || f != 3.14 {
		t.Fatal("float parse")
	}
	if i, ok := cellTypedValue("7").(int64); !ok || i != 7 {
		t.Fatal("int parse")
	}
}

func TestWriteFileEscapesFormulaInjection(t *testing.T) {
	table := ExportTable{
		Columns: []string{"=header", "safe", "safe", "safe", "safe"},
		Rows:    [][]string{{"=SUM(1,1)", "+cmd", "-1", "@value", "plain"}},
	}
	csvData, _, err := WriteFile(FormatCSV, "security", table)
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(csvData[3:]))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range append(records[0], records[1]...) {
		if value != "safe" && value != "plain" && value[0] != '\'' {
			t.Fatalf("unescaped formula-capable CSV value %q", value)
		}
	}

	xlsxData, _, err := WriteFile(FormatXLSX, "security", table)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(xlsxData))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	for _, cell := range []string{"A1", "A2", "B2", "C2", "D2"} {
		value, err := book.GetCellValue(sheet, cell)
		if err != nil {
			t.Fatal(err)
		}
		if value == "" || value[0] != '\'' {
			t.Fatalf("%s=%q is not escaped", cell, value)
		}
	}
	if formula, err := book.GetCellFormula(sheet, "A2"); err != nil || formula != "" {
		t.Fatalf("A2 formula=%q, err=%v", formula, err)
	}
}
