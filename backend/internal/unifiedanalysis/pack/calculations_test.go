package pack

import "testing"

func floatPtr(v float64) *float64 { return &v }

func TestDeltaAbs(t *testing.T) {
	if got := DeltaAbs(150, 100); got != 50 {
		t.Fatalf("got=%v want=50", got)
	}
	if got := DeltaAbs(100, 150); got != -50 {
		t.Fatalf("got=%v want=-50", got)
	}
	if got := DeltaAbs(100.555, 100); got != 0.56 {
		t.Fatalf("got=%v want=0.56 (round2)", got)
	}
}

func TestDeltaPct(t *testing.T) {
	if got := DeltaPct(0, 0); got == nil || *got != 0 {
		t.Fatalf("prev=0,cur=0 got=%v want=0", got)
	}
	if got := DeltaPct(10, 0); got != nil {
		t.Fatalf("prev=0,cur>0 got=%v want=nil", *got)
	}
	if got := DeltaPct(150, 100); got == nil || *got != 50 {
		t.Fatalf("got=%v want=50", got)
	}
	if got := DeltaPct(50, 100); got == nil || *got != -50 {
		t.Fatalf("got=%v want=-50", got)
	}
}

func TestConversionPct(t *testing.T) {
	if got := ConversionPct(0, 10); got != nil {
		t.Fatalf("from=0 got=%v want=nil", *got)
	}
	if got := ConversionPct(200, 50); got == nil || *got != 25 {
		t.Fatalf("got=%v want=25", got)
	}
}

func TestDRRPct(t *testing.T) {
	if got := DRRPct(100, 0); got != nil {
		t.Fatalf("revenue=0 got=%v want=nil", *got)
	}
	if got := DRRPct(50, 200); got == nil || *got != 25 {
		t.Fatalf("got=%v want=25", got)
	}
}

func TestCTRPct(t *testing.T) {
	if got := CTRPct(10, 0); got != nil {
		t.Fatalf("impressions=0 got=%v want=nil", *got)
	}
	if got := CTRPct(50, 1000); got == nil || *got != 5 {
		t.Fatalf("got=%v want=5", got)
	}
}

func TestCPC(t *testing.T) {
	if got := CPC(100, 0); got != nil {
		t.Fatalf("clicks=0 got=%v want=nil", *got)
	}
	if got := CPC(100, 50); got == nil || *got != 2 {
		t.Fatalf("got=%v want=2", got)
	}
}

func TestAOV(t *testing.T) {
	if got := AOV(100, 0); got != nil {
		t.Fatalf("orders=0 got=%v want=nil", *got)
	}
	if got := AOV(1000, 10); got == nil || *got != 100 {
		t.Fatalf("got=%v want=100", got)
	}
}

func TestRevenueSharePct(t *testing.T) {
	if got := RevenueSharePct(100, 0); got != nil {
		t.Fatalf("total=0 got=%v want=nil", *got)
	}
	if got := RevenueSharePct(25, 100); got == nil || *got != 25 {
		t.Fatalf("got=%v want=25", got)
	}
}

func TestCoverageDays(t *testing.T) {
	if got := CoverageDays(100, nil); got != nil {
		t.Fatalf("ads=nil got=%v want=nil", *got)
	}
	zero := 0.0
	if got := CoverageDays(100, &zero); got != nil {
		t.Fatalf("ads=0 got=%v want=nil", *got)
	}
	if got := CoverageDays(100, floatPtr(10)); got == nil || *got != 10 {
		t.Fatalf("got=%v want=10", got)
	}
}

func TestFormatMoneyCSV(t *testing.T) {
	if got := FormatMoneyCSV(1234.5); got != "1234.50" {
		t.Fatalf("got=%q want=1234.50", got)
	}
	if got := FormatMoneyCSVPtr(nil); got != "" {
		t.Fatalf("got=%q want empty", got)
	}
	if got := FormatMoneyCSVPtr(floatPtr(1.005)); got != "1.01" && got != "1.00" {
		// rounding edge case both acceptable depending on float representation
		t.Fatalf("got=%q", got)
	}
}

func TestFormatPctCSV(t *testing.T) {
	if got := FormatPctCSV(nil); got != "" {
		t.Fatalf("got=%q want empty", got)
	}
	if got := FormatPctCSV(floatPtr(12.5)); got != "12.5" {
		t.Fatalf("got=%q want=12.5", got)
	}
	if got := FormatPctCSV(floatPtr(0)); got != "0" {
		t.Fatalf("got=%q want=0", got)
	}
	if got := FormatPctCSV(floatPtr(12.3456)); got != "12.3456" {
		t.Fatalf("got=%q want=12.3456", got)
	}
}

func TestFormatFloatCSV(t *testing.T) {
	if got := FormatFloatCSV(nil); got != "" {
		t.Fatalf("got=%q want empty", got)
	}
	if got := FormatFloatCSV(floatPtr(3.0)); got != "3" {
		t.Fatalf("got=%q want=3", got)
	}
}

func TestFormatIntAndBoolCSV(t *testing.T) {
	if got := FormatIntCSV(42); got != "42" {
		t.Fatalf("got=%q want=42", got)
	}
	if got := FormatIntCSVPtr(nil); got != "" {
		t.Fatalf("got=%q want empty", got)
	}
	v := int64(7)
	if got := FormatIntCSVPtr(&v); got != "7" {
		t.Fatalf("got=%q want=7", got)
	}
	if got := FormatBoolCSVPtr(nil); got != "" {
		t.Fatalf("got=%q want empty", got)
	}
	tru := true
	if got := FormatBoolCSVPtr(&tru); got != "true" {
		t.Fatalf("got=%q want=true", got)
	}
	fls := false
	if got := FormatBoolCSVPtr(&fls); got != "false" {
		t.Fatalf("got=%q want=false", got)
	}
}
