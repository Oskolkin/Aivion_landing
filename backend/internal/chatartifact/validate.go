package chatartifact

import (
	"fmt"
	"path/filepath"
	"strings"
)

var supportedTypes = map[string]bool{
	"pdf": true, "docx": true, "xlsx": true, "csv": true, "tsv": true,
	"pptx": true, "txt": true, "md": true, "json": true, "png": true,
	"jpeg": true, "zip": true,
}

func ValidateSpec(spec ArtifactSpec) error {
	spec.Type = strings.ToLower(strings.TrimSpace(spec.Type))
	if !supportedTypes[spec.Type] {
		return fmt.Errorf("unsupported artifact type %q", spec.Type)
	}
	if err := validateFilename(spec.Filename); err != nil {
		return err
	}
	for i, table := range spec.Tables {
		if len(table.Columns) == 0 {
			return fmt.Errorf("table %d has no columns", i)
		}
		for j, row := range table.Rows {
			if len(row) != len(table.Columns) {
				return fmt.Errorf("table %d row %d has %d cells, want %d", i, j, len(row), len(table.Columns))
			}
		}
	}
	if spec.Workbook != nil {
		for i, sheet := range spec.Workbook.Sheets {
			if len(sheet.Columns) == 0 {
				return fmt.Errorf("sheet %d has no columns", i)
			}
			for j, row := range sheet.Rows {
				if len(row) != len(sheet.Columns) {
					return fmt.Errorf("sheet %d row %d has %d cells, want %d", i, j, len(row), len(sheet.Columns))
				}
			}
		}
	}
	if spec.Chart != nil && len(spec.Chart.Labels) != len(spec.Chart.Values) {
		return fmt.Errorf("chart has %d labels and %d values", len(spec.Chart.Labels), len(spec.Chart.Values))
	}
	if spec.Type == "zip" {
		for i, child := range spec.ZipFiles {
			if strings.EqualFold(child.Type, "zip") {
				return fmt.Errorf("zip child %d cannot be a zip", i)
			}
			if err := ValidateSpec(child); err != nil {
				return fmt.Errorf("zip child %d: %w", i, err)
			}
		}
	}
	return nil
}

func validateFilename(name string) error {
	if name == "" {
		return nil
	}
	if filepath.Base(name) != name || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid filename %q", name)
	}
	return nil
}
