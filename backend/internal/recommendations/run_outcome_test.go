package recommendations

import "testing"

func TestEvaluateRunOutcome_EmptyAIWithOpenAlerts(t *testing.T) {
	ctx := &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 35}}
	out := evaluateRunOutcome(ctx, &ValidationResult{TotalRecommendations: 0})
	if !out.FailRun || out.ErrorMessage != errMsgAIEmptyWithAlerts {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestEvaluateRunOutcome_EmptyAIUsesTopOpenCount(t *testing.T) {
	ctx := &AIRecommendationContext{
		Alerts: AlertsContext{OpenTotal: 0, TopOpen: make([]AlertSignal, 35)},
	}
	out := evaluateRunOutcome(ctx, &ValidationResult{TotalRecommendations: 0})
	if !out.FailRun {
		t.Fatalf("expected fail when top_open has alerts but open_total is zero: %+v", out)
	}
}

func TestEvaluateRunOutcome_AllRejected(t *testing.T) {
	ctx := &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 5}}
	out := evaluateRunOutcome(ctx, &ValidationResult{
		TotalRecommendations:    3,
		RejectedRecommendations: []RejectedRecommendation{{Index: 0, Reason: "x"}},
	})
	if !out.FailRun || out.ErrorMessage != errMsgAllRejectedByValidator {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestEvaluateRunOutcome_PartialSuccess(t *testing.T) {
	ctx := &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 2}}
	out := evaluateRunOutcome(ctx, &ValidationResult{
		TotalRecommendations:    2,
		ValidRecommendations:    []ValidatedRecommendation{{}},
		RejectedRecommendations: []RejectedRecommendation{{Index: 1, Reason: "x"}},
	})
	if out.FailRun || len(out.Warnings) == 0 {
		t.Fatalf("expected warning without fail, got %+v", out)
	}
}
