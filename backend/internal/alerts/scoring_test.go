package alerts

import "testing"

func TestDeltaPercent(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		previous float64
		want     float64
		ok       bool
	}{
		{name: "drop", current: 80, previous: 100, want: -20, ok: true},
		{name: "growth", current: 120, previous: 100, want: 20, ok: true},
		{name: "zero previous", current: 10, previous: 0, want: 0, ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DeltaPercent(tc.current, tc.previous)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got=%v want=%v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("delta mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestSeverityAndUrgencyHelpers(t *testing.T) {
	if got := SeverityFromDropPercent(-55); got != SeverityCritical {
		t.Fatalf("expected critical, got %s", got)
	}
	if got := SeverityFromDropPercent(-20); got != SeverityMedium {
		t.Fatalf("expected medium, got %s", got)
	}
	if got := UrgencyFromDaysOfCover(0.5); got != UrgencyImmediate {
		t.Fatalf("expected immediate, got %s", got)
	}
	if got := UrgencyFromDaysOfCover(10); got != UrgencyLow {
		t.Fatalf("expected low, got %s", got)
	}
}
