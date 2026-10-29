package pack

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
	"time"
)

func buildValidZip(t *testing.T) []byte {
	t.Helper()
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := populatedSnapshot(t, metricDate)
	b := NewPackBuilder(DefaultBuildOptions())
	result, err := b.Build(snap)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return result.ZipBytes
}

func TestValidatorAcceptsValidPack(t *testing.T) {
	zipBytes := buildValidZip(t)
	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(zipBytes)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected valid, issues=%+v", report.Issues)
	}
	if report.CompressedBytes != int64(len(zipBytes)) {
		t.Fatalf("compressed_bytes=%d want=%d", report.CompressedBytes, len(zipBytes))
	}
	if report.UncompressedBytes <= 0 {
		t.Fatal("expected non-zero uncompressed bytes")
	}
}

func TestValidatorRejectsEmptyZip(t *testing.T) {
	v := NewValidator(DefaultBuildOptions())
	if _, err := v.ValidateZip(nil); err != ErrEmptyZip {
		t.Fatalf("err=%v want=%v", err, ErrEmptyZip)
	}
}

func TestValidatorRejectsMissingRequiredFile(t *testing.T) {
	zipBytes := buildValidZip(t)
	mutated := removeZipEntry(t, zipBytes, "limitations.json")

	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(mutated)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if report.Valid {
		t.Fatal("expected invalid report after removing a required file")
	}
	if !hasIssueCode(report, "missing_required_file") {
		t.Fatalf("issues=%+v want missing_required_file", report.Issues)
	}
	if !hasIssueCode(report, "entry_count_mismatch") {
		t.Fatalf("issues=%+v want entry_count_mismatch", report.Issues)
	}
}

func TestValidatorRejectsBadCSVHeader(t *testing.T) {
	zipBytes := buildValidZip(t)
	mutated := replaceZipEntry(t, zipBytes, "account_summary.csv", []byte("not,the,right,header\n"))

	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(mutated)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if report.Valid {
		t.Fatal("expected invalid report after corrupting a csv header")
	}
	if !hasIssueCode(report, "bad_csv_header") {
		t.Fatalf("issues=%+v want bad_csv_header", report.Issues)
	}
}

func TestValidatorRejectsBadJSONSchemaVersion(t *testing.T) {
	zipBytes := buildValidZip(t)
	mutated := replaceZipEntry(t, zipBytes, "analysis_policy.json", []byte(`{"schema_version":"wrong_v1"}`))

	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(mutated)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if report.Valid {
		t.Fatal("expected invalid report after corrupting schema_version")
	}
	if !hasIssueCode(report, "bad_json_document") {
		t.Fatalf("issues=%+v want bad_json_document", report.Issues)
	}
}

func TestValidatorEnforcesCompressedSizeLimit(t *testing.T) {
	zipBytes := buildValidZip(t)
	opts := DefaultBuildOptions()
	opts.MaxCompressedBytes = 10 // absurdly small to force a failure
	v := NewValidator(opts)
	report, err := v.ValidateZip(zipBytes)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if report.Valid {
		t.Fatal("expected invalid report when compressed size limit is exceeded")
	}
	if !hasIssueCode(report, "compressed_size_exceeded") {
		t.Fatalf("issues=%+v want compressed_size_exceeded", report.Issues)
	}
}

func hasIssueCode(report ValidationReport, code string) bool {
	for _, iss := range report.Issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

func removeZipEntry(t *testing.T, zipBytes []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == name {
			continue
		}
		copyZipEntry(t, w, f)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func replaceZipEntry(t *testing.T, zipBytes []byte, name string, content []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == name {
			entry, err := w.Create(name)
			if err != nil {
				t.Fatalf("create entry: %v", err)
			}
			if _, err := entry.Write(content); err != nil {
				t.Fatalf("write entry: %v", err)
			}
			continue
		}
		copyZipEntry(t, w, f)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func copyZipEntry(t *testing.T, w *zip.Writer, f *zip.File) {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatalf("open entry %s: %v", f.Name, err)
	}
	defer rc.Close()
	entry, err := w.Create(f.Name)
	if err != nil {
		t.Fatalf("create entry %s: %v", f.Name, err)
	}
	if _, err := io.Copy(entry, rc); err != nil {
		t.Fatalf("copy entry %s: %v", f.Name, err)
	}
}
