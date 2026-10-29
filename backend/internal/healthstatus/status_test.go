package healthstatus

import "testing"

func TestFromScoreBands(t *testing.T) {
	cases := []struct {
		score int32
		code  string
		sev   string
		label string
	}{
		{100, "stable", "low", "Устойчивое состояние"},
		{80, "stable", "low", "Устойчивое состояние"},
		{79, "good", "medium", "Хорошее состояние, есть локальные риски"},
		{65, "good", "medium", "Хорошее состояние, есть локальные риски"},
		{64, "attention", "high", "Требует внимания"},
		{50, "attention", "high", "Требует внимания"},
		{49, "high_risk", "high", "Высокий риск"},
		{30, "high_risk", "high", "Высокий риск"},
		{29, "critical", "critical", "Критическое состояние"},
		{0, "critical", "critical", "Критическое состояние"},
	}
	for _, tc := range cases {
		s := tc.score
		got := FromScore(&s)
		if got.Code != tc.code || got.Severity != tc.sev || got.Label != tc.label {
			t.Fatalf("score=%d got=%+v want code=%s sev=%s label=%s", tc.score, got, tc.code, tc.sev, tc.label)
		}
	}
	if FromScore(nil).Code != "" {
		t.Fatal("nil score must yield empty status")
	}
}

func TestNormalizeSeverityWarning(t *testing.T) {
	if NormalizeSeverity("warning") != "high" {
		t.Fatalf("warning should map to high")
	}
}
