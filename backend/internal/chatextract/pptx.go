package chatextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

func extractPPTX(data []byte) (*ExtractedAttachment, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	slides := make(map[string][]byte)
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "ppt/slides/slide") || strings.Contains(file.Name, "_rels") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		slides[file.Name] = content
	}
	names := make([]string, 0, len(slides))
	for name := range slides {
		names = append(names, name)
	}
	sort.Strings(names)
	sections := make([]ExtractedSection, 0, len(names))
	for index, name := range names {
		text, err := slideText(slides[name])
		if err != nil {
			return nil, err
		}
		sections = append(sections, ExtractedSection{Title: fmt.Sprintf("Slide %d", index+1), Text: text})
	}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.Text)
	}
	return &ExtractedAttachment{PlainText: strings.Join(parts, "\n\n"), Sections: sections, Metadata: map[string]any{"slides": len(sections)}}, nil
}

func slideText(content []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var parts []string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return strings.Join(parts, "\n"), nil
		}
		if err != nil {
			return "", err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "t" {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return "", err
			}
			if value != "" {
				parts = append(parts, value)
			}
		}
	}
}
