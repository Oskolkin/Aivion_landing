package pack

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

// renderCSV serializes a csvTable to UTF-8 bytes with no BOM, LF line endings,
// and the exact frozen header from the contract package.
func renderCSV(t csvTable) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = false

	if err := w.Write(t.Columns); err != nil {
		return nil, fmt.Errorf("write header %s: %w", t.Name, err)
	}
	for i, row := range t.Rows {
		if len(row) != len(t.Columns) {
			return nil, fmt.Errorf("%s row %d has %d fields, want %d", t.Name, i, len(row), len(t.Columns))
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write row %d of %s: %w", i, t.Name, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush %s: %w", t.Name, err)
	}
	return buf.Bytes(), nil
}
