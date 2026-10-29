package pack

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Validator performs technical (non-semantic) validation of a built pack ZIP:
// entry presence/order, CSV headers, JSON schema versions, and size/row limits.
// It never inspects business-level correctness of the data itself.
type Validator struct {
	opts BuildOptions
}

// NewValidator creates a Validator bound to the given technical limits.
func NewValidator(opts BuildOptions) *Validator {
	return &Validator{opts: opts.withDefaults()}
}

// ValidateZip runs all technical checks against raw ZIP bytes and returns a report.
// A non-nil error is only returned for structurally unreadable input (e.g. not a zip);
// business/contract issues are reported via ValidationReport.Issues instead.
func (v *Validator) ValidateZip(zipBytes []byte) (ValidationReport, error) {
	report := ValidationReport{Valid: true, CompressedBytes: int64(len(zipBytes))}

	if len(zipBytes) == 0 {
		return ValidationReport{}, ErrEmptyZip
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return ValidationReport{}, fmt.Errorf("unifiedanalysis/pack: open zip: %w", err)
	}

	addIssue := func(code, file, msg string) {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Code: code, File: file, Message: msg})
	}

	wantNames := make([]string, len(contract.AnalysisPackFilesV1))
	for i, def := range contract.AnalysisPackFilesV1 {
		wantNames[i] = def.Name
	}
	gotNames := make([]string, len(zr.File))
	for i, f := range zr.File {
		gotNames[i] = f.Name
	}
	if len(gotNames) != len(wantNames) {
		addIssue("entry_count_mismatch", "", fmt.Sprintf("zip has %d entries, want %d", len(gotNames), len(wantNames)))
	}
	if !slices.Equal(gotNames, wantNames) {
		addIssue("entry_order_mismatch", "", fmt.Sprintf("zip entry order=%v want=%v", gotNames, wantNames))
	}

	byName := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		byName[f.Name] = f
	}

	var totalUncompressed int64
	var totalRows int

	for _, def := range contract.AnalysisPackFilesV1 {
		f, ok := byName[def.Name]
		if !ok {
			if def.Required {
				addIssue("missing_required_file", def.Name, "required file is missing from the zip")
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			addIssue("unreadable_entry", def.Name, err.Error())
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			addIssue("unreadable_entry", def.Name, err.Error())
			continue
		}
		totalUncompressed += int64(len(content))

		switch def.Kind {
		case "csv":
			rows, err := validateCSVEntry(def.Name, content)
			if err != nil {
				addIssue("bad_csv_header", def.Name, err.Error())
				continue
			}
			totalRows += rows
		case "json":
			if err := validateJSONEntry(def.Name, content); err != nil {
				addIssue("bad_json_document", def.Name, err.Error())
			}
		}
	}

	report.UncompressedBytes = totalUncompressed
	report.TotalRows = totalRows

	if v.opts.MaxCompressedBytes > 0 && report.CompressedBytes > v.opts.MaxCompressedBytes {
		addIssue("compressed_size_exceeded", "", fmt.Sprintf("compressed size %d exceeds limit %d", report.CompressedBytes, v.opts.MaxCompressedBytes))
	}
	if v.opts.MaxUncompressedBytes > 0 && report.UncompressedBytes > v.opts.MaxUncompressedBytes {
		addIssue("uncompressed_size_exceeded", "", fmt.Sprintf("uncompressed size %d exceeds limit %d", report.UncompressedBytes, v.opts.MaxUncompressedBytes))
	}
	if v.opts.MaxRows > 0 && report.TotalRows > v.opts.MaxRows {
		addIssue("row_count_exceeded", "", fmt.Sprintf("total rows %d exceeds limit %d", report.TotalRows, v.opts.MaxRows))
	}

	return report, nil
}

// validateCSVEntry checks that content parses as CSV and its header matches the frozen
// contract column order exactly. It returns the number of data rows (excluding header).
func validateCSVEntry(name string, content []byte) (int, error) {
	wantCols, ok := contract.AnalysisPackCSVColumnsV1[name]
	if !ok {
		return 0, fmt.Errorf("no contract column definition for %s", name)
	}
	r := csv.NewReader(bytes.NewReader(content))
	header, err := r.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	if !slices.Equal(header, wantCols) {
		return 0, fmt.Errorf("header mismatch: got %v want %v", header, wantCols)
	}
	rows := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rows, fmt.Errorf("row %d: %w", rows, err)
		}
		if len(record) != len(wantCols) {
			return rows, fmt.Errorf("row %d has %d fields, want %d", rows, len(record), len(wantCols))
		}
		rows++
	}
	return rows, nil
}

// validateJSONEntry checks that content is valid JSON and, when the document
// carries a schema_version field, that it matches the frozen v1 schema version.
func validateJSONEntry(name string, content []byte) error {
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	sv, ok := doc["schema_version"]
	if !ok {
		return nil
	}
	s, ok := sv.(string)
	if !ok || s != contract.AnalysisPackSchemaVersionV1 {
		return fmt.Errorf("schema_version=%v want %s", sv, contract.AnalysisPackSchemaVersionV1)
	}
	return nil
}
