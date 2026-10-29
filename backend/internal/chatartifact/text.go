package chatartifact

import (
	"encoding/json"
	"strings"
)

func generateText(spec ArtifactSpec) ([]byte, error) {
	if spec.Type == "json" {
		payload := spec.JSON
		if payload == nil {
			built := map[string]any{
				"title": spec.Title,
			}
			if len(spec.Sections) > 0 {
				built["sections"] = spec.Sections
			}
			if len(spec.Tables) > 0 {
				built["tables"] = spec.Tables
			}
			if spec.Workbook != nil {
				built["workbook"] = spec.Workbook
			}
			if spec.Text != "" {
				built["text"] = spec.Text
			}
			payload = built
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	}
	if spec.Text != "" {
		return []byte(spec.Text), nil
	}
	var b strings.Builder
	if spec.Title != "" {
		if spec.Type == "md" {
			b.WriteString("# ")
		}
		b.WriteString(spec.Title + "\n\n")
	}
	for _, s := range spec.Sections {
		if s.Heading != "" {
			if spec.Type == "md" {
				b.WriteString("## ")
			}
			b.WriteString(s.Heading + "\n")
		}
		for _, p := range s.Paragraphs {
			b.WriteString(p + "\n\n")
		}
		for _, item := range s.Bullets {
			b.WriteString("- " + item + "\n")
		}
	}
	return []byte(b.String()), nil
}
