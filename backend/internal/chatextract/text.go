package chatextract

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

func extractText(format string, data []byte) (*ExtractedAttachment, error) {
	text := string(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	result := &ExtractedAttachment{PlainText: text}
	switch format {
	case "csv", "tsv":
		reader := csv.NewReader(strings.NewReader(text))
		if format == "tsv" {
			reader.Comma = '\t'
		}
		rows, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		result.Tables = []ExtractedTable{{Rows: rows}}
	case "json":
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("format json: %w", err)
		}
		result.PlainText = string(pretty)
	}
	return result, nil
}
