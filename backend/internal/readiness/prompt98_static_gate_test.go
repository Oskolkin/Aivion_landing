package readiness

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrompt98NoUnconditionalFourteenDayAnalysisBlock(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)
	resolver, err := os.ReadFile(filepath.Join(dir, "analysisdate.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(resolver)
	if strings.Contains(src, "salesDays < 14") || strings.Contains(src, "len(st.matureDates) < 14") {
		t.Fatal("analysisdate.go still contains an unconditional 14-day analysis gate")
	}
	if strings.Contains(src, "AddDate(0, 0, -13)") {
		t.Fatal("analysisdate.go still requires a 14-day candidate window")
	}

	evaluator, err := os.ReadFile(filepath.Join(dir, "evaluator.go"))
	if err != nil {
		t.Fatal(err)
	}
	eval := string(evaluator)
	forbidden := []string{
		`Code: "SALES_HISTORY_INSUFFICIENT", Severity: "WARNING", ReadinessEffect: "BLOCK_ANALYSIS_DATE"`,
		`Code: "SALES_MATURITY_INSUFFICIENT", Severity: "WARNING", ReadinessEffect: "BLOCK_ANALYSIS_DATE"`,
		`Code: "SALES_DAILY_COVERAGE_HOLE", Severity: "ERROR", ReadinessEffect: "BLOCK_ANALYSIS_DATE"`,
	}
	for _, tok := range forbidden {
		if strings.Contains(eval, tok) {
			t.Fatalf("evaluator.go still uses BLOCK_ANALYSIS_DATE for historical depth: %s", tok)
		}
	}
}
