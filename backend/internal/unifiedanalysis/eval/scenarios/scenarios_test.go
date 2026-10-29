package scenarios_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval/scenarios"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
)

func TestScenarioPacksBuildAndValidate(t *testing.T) {
	for _, def := range scenarios.All() {
		def := def
		t.Run(def.Name, func(t *testing.T) {
			snap := def.Snapshot(t)
			if snap.SellerAccountID == 1 {
				t.Fatal("scenario must not depend on seller 1 seed")
			}

			b := pack.NewPackBuilder(pack.DefaultBuildOptions())
			result, err := b.Build(snap)
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			if result.SchemaVersion != contract.AnalysisPackSchemaVersionV1 {
				t.Fatalf("schema=%s want=%s", result.SchemaVersion, contract.AnalysisPackSchemaVersionV1)
			}
			if len(result.Files) != len(contract.AnalysisPackFilesV1) {
				t.Fatalf("files=%d want=%d", len(result.Files), len(contract.AnalysisPackFilesV1))
			}

			v := pack.NewValidator(pack.DefaultBuildOptions())
			report, err := v.ValidateZip(result.ZipBytes)
			if err != nil {
				t.Fatalf("validate zip: %v", err)
			}
			if !report.Valid {
				t.Fatalf("pack must validate: %+v", report.Issues)
			}

			checksPath := filepath.Join(scenarioDir(t), def.Name, "expected_checks.json")
			raw, err := os.ReadFile(checksPath)
			if err != nil {
				t.Fatalf("read expected_checks.json: %v", err)
			}
			var fileChecks eval.ExpectedChecks
			if err := json.Unmarshal(raw, &fileChecks); err != nil {
				t.Fatalf("unmarshal expected_checks: %v", err)
			}
			if fileChecks.Outcome != def.Checks.Outcome {
				t.Fatalf("expected_checks outcome drift: file=%q registry=%q", fileChecks.Outcome, def.Checks.Outcome)
			}

			t.Logf("pack_id=%s pack_hash=%s bytes=%d", result.PackID, result.PackHash, len(result.ZipBytes))
		})
	}
}

func scenarioDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Dir(file)
}
