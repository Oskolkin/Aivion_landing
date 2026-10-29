package chatartifact

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"image"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

func TestGenerateAllFormats(t *testing.T) {
	base := ArtifactSpec{
		Title:    "Отчёт sales",
		Text:     "plain report",
		Sections: []ArtifactSection{{Heading: "Summary", Paragraphs: []string{"A deterministic report."}, Bullets: []string{"First"}}},
		Tables:   []ArtifactTable{{Columns: []string{"Name", "Value"}, Rows: [][]string{{"Revenue", "42"}}}},
		Workbook: &ArtifactWorkbook{Sheets: []ArtifactSheet{
			{Name: "First", Columns: []string{"A"}, Rows: [][]string{{"1"}}},
			{Name: "Second", Columns: []string{"B"}, Rows: [][]string{{"2"}}},
		}},
		Slides: []ArtifactSlide{{Title: "Slide", Body: "Body", Bullets: []string{"Point"}}},
		JSON:   map[string]any{"ok": true},
		Chart:  &ArtifactChart{Title: "Chart", Labels: []string{"A", "B"}, Values: []float64{2, 4}},
	}
	for _, kind := range []string{"pdf", "docx", "xlsx", "csv", "tsv", "pptx", "txt", "md", "json", "png", "jpeg"} {
		t.Run(kind, func(t *testing.T) {
			spec := base
			spec.Type, spec.Filename = kind, "report"
			data, mime, filename, err := Generate(spec)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) == 0 || mime == "" || !strings.HasSuffix(filename, "."+kind) {
				t.Fatalf("invalid result: %d %q %q", len(data), mime, filename)
			}
			assertParseable(t, kind, data)
		})
	}
}

func TestGenerateZIPAndRejectTraversal(t *testing.T) {
	data, _, _, err := Generate(ArtifactSpec{Type: "zip", Title: "bundle", ZipFiles: []ArtifactSpec{
		{Type: "txt", Filename: "notes.txt", Text: "hello"},
		{Type: "json", Filename: "data.json", JSON: map[string]int{"n": 1}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(zr.File) != 3 {
		t.Fatalf("invalid zip: %v, entries=%d", err, len(zr.File))
	}
	if err := ValidateSpec(ArtifactSpec{Type: "zip", ZipFiles: []ArtifactSpec{{Type: "txt", Filename: "../bad.txt"}}}); err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestGeneratedSpreadsheetArtifactsEscapeFormulaInjection(t *testing.T) {
	spec := ArtifactSpec{
		Title:  "security",
		Tables: []ArtifactTable{{Columns: []string{"=header", "safe", "safe", "safe"}, Rows: [][]string{{"=SUM(1,1)", "+cmd", "-1", "@value"}}}},
	}
	spec.Type, spec.Filename = "csv", "security.csv"
	csvData, _, _, err := Generate(spec)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(csvData)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range append(records[0], records[1]...) {
		if value != "safe" && value[0] != '\'' {
			t.Fatalf("unescaped CSV value %q", value)
		}
	}

	spec.Type, spec.Filename = "xlsx", "security.xlsx"
	xlsxData, _, _, err := Generate(spec)
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
}

func TestGenerateIsDeterministic(t *testing.T) {
	for _, kind := range []string{"pdf", "docx", "xlsx", "pptx", "png", "zip"} {
		spec := ArtifactSpec{
			Type: kind, Title: "Stable", Text: "Content",
			Tables: []ArtifactTable{{Columns: []string{"A"}, Rows: [][]string{{"1"}}}},
			Chart:  &ArtifactChart{Labels: []string{"A"}, Values: []float64{1}},
			ZipFiles: []ArtifactSpec{
				{Type: "txt", Filename: "note.txt", Text: "content"},
			},
		}
		first, _, _, err := Generate(spec)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		second, _, _, err := Generate(spec)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("%s output is not deterministic", kind)
		}
	}
}

func TestGeneratePDFPreservesCyrillicUnicode(t *testing.T) {
	const russian = "Привет, рынок — 1 250 ₽ и 15%"
	data, _, _, err := Generate(ArtifactSpec{
		Type:     "pdf",
		Title:    russian,
		Sections: []ArtifactSection{{Paragraphs: []string{russian}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/FontFile2")) {
		t.Fatal("PDF does not embed a TrueType Unicode font")
	}
	for _, code := range utf16.Encode([]rune(russian)) {
		utf16BE := []byte{byte(code >> 8), byte(code)}
		if !bytes.Contains(data, utf16BE) {
			t.Fatalf("PDF ToUnicode map is missing UTF-16BE code point %04X", code)
		}
	}
}

func assertParseable(t *testing.T, kind string, data []byte) {
	t.Helper()
	switch kind {
	case "pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Fatal("missing PDF magic")
		}
	case "docx", "pptx":
		if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatal(err)
		}
	case "xlsx":
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if len(f.GetSheetList()) != 2 {
			t.Fatal("expected two sheets")
		}
	case "png", "jpeg":
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width < 400 || cfg.Height < 300 {
			t.Fatalf("invalid image: %v, %#v", err, cfg)
		}
	case "csv", "tsv":
		if !bytes.Contains(data, []byte("Name")) {
			t.Fatal("missing table")
		}
	case "json":
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
	}
}
