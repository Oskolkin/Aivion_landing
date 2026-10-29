package chatartifact

import (
	"bytes"
	_ "embed"
	"time"

	"github.com/phpdave11/gofpdf"
)

//go:embed fonts/DejaVuSans.ttf
var dejavuSansTTF []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var dejavuSansBoldTTF []byte

func generatePDF(spec ArtifactSpec) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(time.Unix(0, 0).UTC())
	pdf.SetModificationDate(time.Unix(0, 0).UTC())
	pdf.AddUTF8FontFromBytes("DejaVuSans", "", dejavuSansTTF)
	pdf.AddUTF8FontFromBytes("DejaVuSans", "B", dejavuSansBoldTTF)
	pdf.SetTitle(spec.Title, true)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	writePDFText(pdf, "B", 18, spec.Title)
	for _, section := range spec.Sections {
		if section.Heading != "" {
			writePDFText(pdf, "B", 13, section.Heading)
		}
		for _, paragraph := range section.Paragraphs {
			writePDFText(pdf, "", 11, paragraph)
		}
		for _, bullet := range section.Bullets {
			writePDFText(pdf, "", 11, "- "+bullet)
		}
	}
	for _, table := range spec.Tables {
		pdf.SetFont("DejaVuSans", "B", 9)
		width := 180.0 / float64(len(table.Columns))
		for _, cell := range table.Columns {
			pdf.CellFormat(width, 7, cell, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("DejaVuSans", "", 9)
		for _, row := range table.Rows {
			for _, cell := range row {
				pdf.CellFormat(width, 7, cell, "1", 0, "L", false, 0, "")
			}
			pdf.Ln(-1)
		}
		pdf.Ln(3)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writePDFText(pdf *gofpdf.Fpdf, style string, size float64, text string) {
	if text == "" {
		return
	}
	pdf.SetFont("DejaVuSans", style, size)
	pdf.MultiCell(180, size*0.48+2, text, "", "L", false)
	pdf.Ln(1)
}
