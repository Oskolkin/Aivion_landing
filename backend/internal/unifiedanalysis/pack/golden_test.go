package pack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGoldenUnifiedAnalysisPack(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := populatedSnapshot(t, metricDate)

	b := NewPackBuilder(DefaultBuildOptions())
	result, err := b.Build(snap)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(result.ZipBytes)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid {
		t.Fatalf("golden pack must validate: %+v", report.Issues)
	}

	dir := filepath.Join("testdata")
	zipPath := filepath.Join(dir, "analysis_pack_v1_golden.zip")
	manifestPath := filepath.Join(dir, "manifest_v1_golden.json")
	limitationsPath := filepath.Join(dir, "limitations_v1_golden.json")
	summaryPath := filepath.Join(dir, "summary_v1_golden.json")

	summary := map[string]any{
		"pack_id":            result.PackID,
		"pack_hash":          result.PackHash,
		"schema_version":     result.SchemaVersion,
		"seller_account_id":  result.SellerAccountID,
		"metric_date":        result.MetricDate,
		"compressed_bytes":   len(result.ZipBytes),
		"uncompressed_bytes": report.UncompressedBytes,
		"limitations_count":  len(result.Limitations),
		"row_counts":         map[string]int{},
	}
	rowCounts := summary["row_counts"].(map[string]int)
	for _, f := range result.Files {
		if f.Kind == "csv" {
			rowCounts[f.Name] = f.RowCount
		}
	}
	archiveSHA := sha256.Sum256(result.ZipBytes)
	summary["archive_sha256"] = hex.EncodeToString(archiveSHA[:])

	if os.Getenv("UPDATE_UNIFIED_PACK_GOLDEN") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(zipPath, result.ZipBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		manifestRaw, _ := json.MarshalIndent(result.Manifest, "", "  ")
		_ = os.WriteFile(manifestPath, append(manifestRaw, '\n'), 0o644)
		limRaw, _ := json.MarshalIndent(map[string]any{
			"schema_version": result.SchemaVersion,
			"limitations":    result.Limitations,
		}, "", "  ")
		_ = os.WriteFile(limitationsPath, append(limRaw, '\n'), 0o644)
		summaryRaw, _ := json.MarshalIndent(summary, "", "  ")
		_ = os.WriteFile(summaryPath, append(summaryRaw, '\n'), 0o644)
		t.Logf("updated golden pack_hash=%s archive_sha256=%s", result.PackHash, summary["archive_sha256"])
		return
	}

	wantZip, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("read golden zip (set UPDATE_UNIFIED_PACK_GOLDEN=1 to create): %v", err)
	}
	if string(wantZip) != string(result.ZipBytes) {
		wantSum := sha256.Sum256(wantZip)
		t.Fatalf("golden zip mismatch\ngot_pack_hash=%s\ngot_archive=%s\nwant_archive=%s\nset UPDATE_UNIFIED_PACK_GOLDEN=1 only after intentional pack changes",
			result.PackHash, hex.EncodeToString(archiveSHA[:]), hex.EncodeToString(wantSum[:]))
	}

	wantSummaryRaw, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read golden summary: %v", err)
	}
	var wantSummary map[string]any
	if err := json.Unmarshal(wantSummaryRaw, &wantSummary); err != nil {
		t.Fatalf("parse golden summary: %v", err)
	}
	if wantSummary["pack_id"] != result.PackID {
		t.Fatalf("pack_id mismatch: got %s want %v", result.PackID, wantSummary["pack_id"])
	}
	if wantSummary["pack_hash"] != result.PackHash {
		t.Fatalf("pack_hash mismatch: got %s want %v", result.PackHash, wantSummary["pack_hash"])
	}
}

func TestFakePackStoragePut(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := minimalSnapshot(1, metricDate)
	b := NewPackBuilder(DefaultBuildOptions())
	result, err := b.Build(snap)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	result.Validation = ValidationReport{Valid: true, UncompressedBytes: 100}

	fake := &fakePackStorage{}
	key, err := fake.Put(context.Background(), result)
	if err != nil {
		t.Fatal(err)
	}
	want := PackObjectKey(1, "2026-07-08", result.PackHash)
	if key != want {
		t.Fatalf("key=%s want=%s", key, want)
	}
	if len(fake.body) == 0 {
		t.Fatal("expected uploaded body")
	}
}

type fakePackStorage struct {
	key  string
	body []byte
}

func (f *fakePackStorage) Put(_ context.Context, result *BuildResult) (string, error) {
	f.key = PackObjectKey(result.SellerAccountID, result.MetricDate, result.PackHash)
	f.body = append([]byte(nil), result.ZipBytes...)
	return f.key, nil
}
