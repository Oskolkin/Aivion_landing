package chatextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

func extractDOCX(data []byte) (*ExtractedAttachment, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		return parseWordDocument(stream)
	}
	return nil, errors.New("word/document.xml missing")
}

func parseWordDocument(reader io.Reader) (*ExtractedAttachment, error) {
	decoder := xml.NewDecoder(reader)
	var paragraphs, row []string
	var tables []ExtractedTable
	var text strings.Builder
	inTable, inRow := false, false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "tbl":
				inTable = true
				tables = append(tables, ExtractedTable{Name: "Table " + strconv.Itoa(len(tables)+1)})
			case "tr":
				if inTable {
					inRow, row = true, nil
				}
			case "t":
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return nil, err
				}
				text.WriteString(value)
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "p":
				value := strings.TrimSpace(text.String())
				if value != "" {
					if inRow {
						row = append(row, value)
					} else {
						paragraphs = append(paragraphs, value)
					}
				}
				text.Reset()
			case "tr":
				if inRow {
					tables[len(tables)-1].Rows = append(tables[len(tables)-1].Rows, row)
					inRow = false
				}
			case "tbl":
				inTable = false
			}
		}
	}
	return &ExtractedAttachment{PlainText: strings.Join(paragraphs, "\n"), Sections: []ExtractedSection{{Title: "Document", Text: strings.Join(paragraphs, "\n")}}, Tables: tables}, nil
}
