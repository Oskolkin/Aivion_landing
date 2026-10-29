package chatartifact

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

func generateDOCX(spec ArtifactSpec) ([]byte, error) {
	var body strings.Builder
	addDOCXParagraph(&body, spec.Title, true)
	for _, section := range spec.Sections {
		addDOCXParagraph(&body, section.Heading, true)
		for _, p := range section.Paragraphs {
			addDOCXParagraph(&body, p, false)
		}
		for _, bullet := range section.Bullets {
			addDOCXParagraph(&body, "• "+bullet, false)
		}
	}
	for _, table := range spec.Tables {
		body.WriteString("<w:tbl>")
		for _, row := range append([][]string{table.Columns}, table.Rows...) {
			body.WriteString("<w:tr>")
			for _, cell := range row {
				body.WriteString("<w:tc><w:p><w:r><w:t>")
				body.WriteString(xmlEscape(cell))
				body.WriteString("</w:t></w:r></w:p></w:tc>")
			}
			body.WriteString("</w:tr>")
		}
		body.WriteString("</w:tbl>")
	}
	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `<w:sectPr/></w:body></w:document>`
	files := map[string]string{
		"[Content_Types].xml":          `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":                  `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml":            document,
	}
	return zipStrings(files)
}

func addDOCXParagraph(b *strings.Builder, text string, bold bool) {
	if text == "" {
		return
	}
	b.WriteString("<w:p><w:r>")
	if bold {
		b.WriteString("<w:rPr><w:b/></w:rPr>")
	}
	b.WriteString("<w:t xml:space=\"preserve\">")
	b.WriteString(xmlEscape(text))
	b.WriteString("</w:t></w:r></w:p>")
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func zipStrings(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := files[name]
		f, err := w.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
