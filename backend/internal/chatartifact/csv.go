package chatartifact

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

func generateDelimited(spec ArtifactSpec, tab bool) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if tab {
		w.Comma = '\t'
	}
	for _, table := range spec.Tables {
		if err := w.Write(escapeFormulaRow(table.Columns)); err != nil {
			return nil, err
		}
		for _, row := range table.Rows {
			if err := w.Write(escapeFormulaRow(row)); err != nil {
				return nil, err
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("write delimited artifact: %w", err)
	}
	return buf.Bytes(), nil
}
