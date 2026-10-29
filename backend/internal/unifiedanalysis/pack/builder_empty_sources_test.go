package pack

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestBuildAlwaysProducesAllTwelveFiles(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := minimalSnapshot(42, metricDate)

	b := NewPackBuilder(DefaultBuildOptions())
	result, err := b.Build(snap)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(result.Files) != len(contract.AnalysisPackFilesV1) {
		t.Fatalf("files=%d want=%d", len(result.Files), len(contract.AnalysisPackFilesV1))
	}
	for i, def := range contract.AnalysisPackFilesV1 {
		if result.Files[i].Name != def.Name {
			t.Fatalf("file[%d]=%s want=%s", i, result.Files[i].Name, def.Name)
		}
		if result.Files[i].SHA256 == "" {
			t.Fatalf("file %s missing sha256", def.Name)
		}
	}

	v := NewValidator(DefaultBuildOptions())
	report, err := v.ValidateZip(result.ZipBytes)
	if err != nil {
		t.Fatalf("ValidateZip: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected valid report, issues=%+v", report.Issues)
	}

	// account_summary.csv must still have exactly one row (header-only tables aside).
	var accountSummaryRows int
	var skuSummaryRows int
	for _, f := range result.Files {
		switch f.Name {
		case "account_summary.csv":
			accountSummaryRows = f.RowCount
		case "sku_summary.csv":
			skuSummaryRows = f.RowCount
		}
	}
	if accountSummaryRows != 1 {
		t.Fatalf("account_summary.csv rows=%d want=1", accountSummaryRows)
	}
	if skuSummaryRows != 0 {
		t.Fatalf("sku_summary.csv rows=%d want=0 for empty snapshot", skuSummaryRows)
	}

	var manifest contract.AnalysisPackManifestV1
	for _, f := range result.Files {
		if f.Name == "manifest.json" {
			if err := json.Unmarshal(f.Content, &manifest); err != nil {
				t.Fatalf("unmarshal manifest: %v", err)
			}
		}
	}
	if manifest.SchemaVersion != contract.AnalysisPackSchemaVersionV1 {
		t.Fatalf("manifest schema=%s", manifest.SchemaVersion)
	}
	if manifest.PackID == "" {
		t.Fatal("expected non-empty pack_id")
	}
	if len(manifest.Tables) != 8 {
		t.Fatalf("manifest tables=%d want=8", len(manifest.Tables))
	}

	if len(result.Limitations) == 0 {
		t.Fatal("expected limitations for an empty snapshot (no account/funnel/enrichment data)")
	}
	for i := 1; i < len(result.Limitations); i++ {
		if result.Limitations[i-1].Code > result.Limitations[i].Code {
			t.Fatalf("limitations not sorted ascending by code: %v", result.Limitations)
		}
	}
}

func TestBuildRejectsNilSnapshot(t *testing.T) {
	b := NewPackBuilder(DefaultBuildOptions())
	if _, err := b.Build(nil); err != ErrNilSnapshot {
		t.Fatalf("err=%v want=%v", err, ErrNilSnapshot)
	}
}

func TestBuildRejectsInvalidSellerAccountID(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := minimalSnapshot(0, metricDate)
	b := NewPackBuilder(DefaultBuildOptions())
	if _, err := b.Build(snap); err != ErrInvalidSellerAccountID {
		t.Fatalf("err=%v want=%v", err, ErrInvalidSellerAccountID)
	}
}
