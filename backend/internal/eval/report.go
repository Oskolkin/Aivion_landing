package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Report struct {
	RunID            int64             `json:"run_id"`
	Suite            string            `json:"suite"`
	Version          string            `json:"version"`
	Split            string            `json:"split"`
	Models           ResolvedModels    `json:"models"`
	HardVerdict      string            `json:"hard_verdict"`
	SemanticVerdict  string            `json:"semantic_verdict"`
	CriticalFailures int               `json:"critical_failures"`
	Tokens           map[string]int64  `json:"tokens"`
	DurationMS       int64             `json:"duration_ms"`
	ReportArtifact   string            `json:"report_artifact,omitempty"`
	Limitations      []KnownLimitation `json:"limitations,omitempty"`
	HumanReview      string            `json:"human_review"`
}

func BuildReport(suite Suite, run Run) Report {
	var dur int64
	if run.CompletedAt != nil {
		dur = run.CompletedAt.Sub(run.StartedAt).Milliseconds()
	}
	human := "PENDING"
	if run.Status == "PASSED" {
		human = "NOT_REQUIRED"
	}
	if run.Status == StatusHumanReviewPending {
		human = "PENDING"
	}
	return Report{
		RunID: run.ID, Suite: suite.SuiteKey, Version: suite.Version, Split: run.Split,
		Models: run.ResolvedModels, HardVerdict: run.Verdict, SemanticVerdict: run.Verdict,
		CriticalFailures: run.CriticalFailuresCount,
		Tokens:           map[string]int64{"input": run.Aggregate.InputTokens, "output": run.Aggregate.OutputTokens, "cached": run.Aggregate.CachedTokens},
		DurationMS:       dur, ReportArtifact: run.ReportArtifactKey, Limitations: run.Limitations, HumanReview: human,
	}
}

func HashReport(r Report) string {
	b, _ := json.Marshal(r)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
