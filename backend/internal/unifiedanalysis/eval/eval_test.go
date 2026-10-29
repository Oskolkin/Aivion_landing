package eval_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval"
)

func loadOutputFixture(t *testing.T, name string) *contract.UnifiedAnalysisOutputV1 {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &out
}

func TestEvaluatePassStockScenario(t *testing.T) {
	out := loadOutputFixture(t, "output_pass_stock.json")
	report := eval.Evaluate(out, eval.ExpectedChecks{
		Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
		RequiredCategories: []string{"stock"},
		RequiredSKUIDs:     []string{"901042000001"},
		RequireTopAction:   true,
		NoDuplicateSKU:     true,
	})
	if !report.Passed {
		t.Fatalf("expected pass, failures=%+v", report.Failures)
	}
}

func TestEvaluateFailWrongOutcome(t *testing.T) {
	out := loadOutputFixture(t, "output_fail_wrong_outcome.json")
	report := eval.Evaluate(out, eval.ExpectedChecks{
		Outcome: contract.AnalysisOutcomeMaterialActionsFound,
	})
	if report.Passed {
		t.Fatal("expected fail for wrong outcome")
	}
}

func TestEvaluateFailDuplicateSKU(t *testing.T) {
	out := loadOutputFixture(t, "output_fail_duplicate_sku.json")
	report := eval.Evaluate(out, eval.ExpectedChecks{
		NoDuplicateSKU: true,
	})
	if report.Passed {
		t.Fatal("expected fail for duplicate sku across actions")
	}
}

func TestEvaluatePassAnalysisFailedOutcome(t *testing.T) {
	out := loadOutputFixture(t, "output_pass_analysis_failed.json")
	maxZero := 0
	report := eval.Evaluate(out, eval.ExpectedChecks{
		Outcome:    contract.AnalysisOutcomeAnalysisFailed,
		MaxActions: &maxZero,
	})
	if !report.Passed {
		t.Fatalf("expected pass for analysis_failed fixture, failures=%+v", report.Failures)
	}
}

func TestEvaluateForbiddenCategory(t *testing.T) {
	out := loadOutputFixture(t, "output_pass_stock.json")
	report := eval.Evaluate(out, eval.ExpectedChecks{
		ForbiddenCategories: []string{"advertising"},
	})
	if !report.Passed {
		t.Fatal("expected pass when forbidden category absent")
	}

	report = eval.Evaluate(out, eval.ExpectedChecks{
		ForbiddenCategories: []string{"stock"},
	})
	if report.Passed {
		t.Fatal("expected fail when forbidden category present")
	}
}

func TestEvaluateMaxActions(t *testing.T) {
	out := loadOutputFixture(t, "output_pass_stock.json")
	one := 1
	report := eval.Evaluate(out, eval.ExpectedChecks{MaxActions: &one})
	if !report.Passed {
		t.Fatalf("expected pass with max_actions=1, failures=%+v", report.Failures)
	}

	zero := 0
	report = eval.Evaluate(out, eval.ExpectedChecks{MaxActions: &zero})
	if report.Passed {
		t.Fatal("expected fail with max_actions=0")
	}
}

func TestEvaluateNilOutput(t *testing.T) {
	report := eval.Evaluate(nil, eval.ExpectedChecks{Outcome: contract.AnalysisOutcomeNoMaterialActions})
	if report.Passed {
		t.Fatal("expected fail for nil output")
	}
}
