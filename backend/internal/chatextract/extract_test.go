package chatextract

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExtractTextJSONAndCSV(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		got, err := Extract("note.txt", "text/plain", []byte("hello"))
		if err != nil || got.PlainText != "hello" {
			t.Fatalf("Extract() = %#v, %v", got, err)
		}
	})
	t.Run("json", func(t *testing.T) {
		got, err := Extract("data.json", "", []byte(`{"answer":42}`))
		if err != nil || !strings.Contains(got.PlainText, `"answer": 42`) {
			t.Fatalf("Extract() = %#v, %v", got, err)
		}
	})
	t.Run("csv", func(t *testing.T) {
		got, err := Extract("data.csv", "", []byte("name,count\napples,2\n"))
		if err != nil || len(got.Tables) != 1 || got.Tables[0].Rows[1][1] != "2" {
			t.Fatalf("Extract() = %#v, %v", got, err)
		}
	})
}

func TestExtractXLSX(t *testing.T) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if err := book.SetCellValue("Sheet1", "B7", 12); err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellFormula("Sheet1", "C7", "=B7*2"); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := book.Write(&data); err != nil {
		t.Fatal(err)
	}
	got, err := Extract("report.xlsx", "", data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sheets) != 1 || !containsCell(got.Sheets[0].Cells, "B7", "12") || !containsCell(got.Sheets[0].Cells, "C7", "") {
		t.Fatalf("unexpected sheets: %#v", got.Sheets)
	}
}

func TestExtractDOCX(t *testing.T) {
	data := zipFixture(t, map[string]string{
		"word/document.xml": `<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>Hello DOCX</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>`,
	})
	got, err := Extract("note.docx", "", data)
	if err != nil || !strings.Contains(got.PlainText, "Hello DOCX") || len(got.Tables) != 1 {
		t.Fatalf("Extract() = %#v, %v", got, err)
	}
}

func TestExtractPPTX(t *testing.T) {
	data := zipFixture(t, map[string]string{
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><a:t>First slide</a:t></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/slide2.xml": `<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><a:t>Second slide</a:t></p:spTree></p:cSld></p:sld>`,
	})
	got, err := Extract("deck.pptx", "", data)
	if err != nil || len(got.Sections) != 2 || got.Metadata["slides"] != 2 {
		t.Fatalf("Extract() = %#v, %v", got, err)
	}
}

func TestExtractZIPRejectsSlip(t *testing.T) {
	data := zipFixture(t, map[string]string{"../escape.txt": "no"})
	if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "unsafe zip path") {
		t.Fatalf("expected zip slip rejection, got %v", err)
	}
}

func TestExtractZIPSecurityControls(t *testing.T) {
	t.Run("absolute paths", func(t *testing.T) {
		for _, name := range []string{"/etc/passwd", `C:\Windows\system.ini`} {
			data := zipFixture(t, map[string]string{name: "no"})
			if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "unsafe zip path") {
				t.Fatalf("%q: expected unsafe path rejection, got %v", name, err)
			}
		}
	})
	t.Run("too many files", func(t *testing.T) {
		t.Setenv("CHATEXTRACT_ZIP_MAX_FILES", "1")
		data := zipFixture(t, map[string]string{"one.txt": "1", "two.txt": "2"})
		if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "more than 1 files") {
			t.Fatalf("expected file count rejection, got %v", err)
		}
	})
	t.Run("uncompressed size limit", func(t *testing.T) {
		t.Setenv("CHATEXTRACT_ZIP_MAX_UNCOMPRESSED_BYTES", "32")
		data := zipFixture(t, map[string]string{"large.txt": strings.Repeat("a", 33)})
		if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "uncompressed size limit") {
			t.Fatalf("expected size rejection, got %v", err)
		}
	})
	t.Run("compression ratio limit", func(t *testing.T) {
		t.Setenv("CHATEXTRACT_ZIP_MAX_COMPRESSION_RATIO", "2")
		data := zipFixture(t, map[string]string{"bomb.txt": strings.Repeat("a", 2048)})
		if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "compression ratio limit") {
			t.Fatalf("expected compression ratio rejection, got %v", err)
		}
	})
	t.Run("nested archive", func(t *testing.T) {
		data := zipFixture(t, map[string]string{"nested.zip": "not opened"})
		if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "nested zip") {
			t.Fatalf("expected nested zip rejection, got %v", err)
		}
	})
	t.Run("encrypted entry flag", func(t *testing.T) {
		data := markZipEncrypted(t, zipFixture(t, map[string]string{"note.txt": "hello"}))
		if _, err := Extract("archive.zip", "", data); err == nil || !strings.Contains(err.Error(), "encrypted zip entry") {
			t.Fatalf("expected encrypted entry rejection, got %v", err)
		}
	})
}

func TestExtractImageDimensions(t *testing.T) {
	var data bytes.Buffer
	imageData := image.NewRGBA(image.Rect(0, 0, 7, 5))
	imageData.Set(0, 0, color.White)
	if err := png.Encode(&data, imageData); err != nil {
		t.Fatal(err)
	}
	got, err := Extract("image.png", "", data.Bytes())
	if err != nil || got.Image.Width != 7 || got.Image.Height != 5 || got.Metadata["vision_required"] != true {
		t.Fatalf("Extract() = %#v, %v", got, err)
	}
}

func TestExtractPDFUnicodeRevenue(t *testing.T) {
	// UTF-16BE hex string (BOM + text) inside a Flate-encoded content stream.
	content := "BT <FEFF004100200031003200380034003500300030> Tj ET"
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\n1 0 obj\n<< /Length " + itoa(compressed.Len()) + " /Filter /FlateDecode >>\nstream\n")
	pdf = append(pdf, compressed.Bytes()...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n%%EOF\n")...)
	got, err := Extract("rev.pdf", "application/pdf", pdf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.PlainText, "1284500") && !strings.Contains(got.PlainText, "1 284 500") {
		t.Fatalf("expected revenue digits in PlainText, got %q", got.PlainText)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func containsCell(cells []ExtractedCell, address, value string) bool {
	for _, cell := range cells {
		if cell.Address == address && cell.Value == value {
			return true
		}
	}
	return false
}

func zipFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func markZipEncrypted(t *testing.T, data []byte) []byte {
	t.Helper()
	out := append([]byte(nil), data...)
	for i := 0; i+10 <= len(out); i++ {
		if (string(out[i:i+4]) == "PK\x03\x04" && i+8 <= len(out)) || (string(out[i:i+4]) == "PK\x01\x02" && i+10 <= len(out)) {
			flagOffset := i + 6
			if string(out[i:i+4]) == "PK\x01\x02" {
				flagOffset = i + 8
			}
			out[flagOffset] |= 0x1
		}
	}
	return out
}
