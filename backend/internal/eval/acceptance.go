package eval

import "fmt"

type Acceptance struct {
	Status      string
	Verdict     string
	Limitations []KnownLimitation
}

func Decide(run Run, cases []CaseRun, kind string) Acceptance {
	if kind == KindRealSellerReplay {
		return Acceptance{Status: StatusDatasetPending, Verdict: VerdictPassKnownLimitations, Limitations: run.Limitations}
	}
	if run.CriticalFailuresCount > 0 {
		return Acceptance{Status: "FAILED", Verdict: VerdictFail}
	}
	for _, c := range cases {
		if c.Verdict == VerdictFail || c.Status == "FAILED" {
			return Acceptance{Status: "FAILED", Verdict: VerdictFail}
		}
	}
	humanPending := kind == KindAIGolden || kind == KindChatGolden || kind == KindGraderCalibration
	if humanPending {
		return Acceptance{Status: StatusHumanReviewPending, Verdict: VerdictPass}
	}
	return Acceptance{Status: "PASSED", Verdict: VerdictPass}
}

func CanPromote(run Run, humanReviews []HumanReview) error {
	if run.Verdict != VerdictPass && run.Verdict != VerdictPassKnownLimitations {
		return fmt.Errorf("promote requires PASS, got %s", run.Verdict)
	}
	if run.CriticalFailuresCount != 0 {
		return fmt.Errorf("promote requires critical_failures=0")
	}
	if run.Status == StatusHumanReviewPending {
		return fmt.Errorf("promote blocked: HUMAN_REVIEW_PENDING")
	}
	if run.Split == SplitHoldout {
		pass := 0
		for _, h := range humanReviews {
			if h.Status == "PASS" {
				pass++
			}
			if h.Status == "FAIL" {
				return fmt.Errorf("human review FAIL")
			}
		}
		if pass == 0 {
			return fmt.Errorf("promote blocked: human review required")
		}
	}
	return nil
}
