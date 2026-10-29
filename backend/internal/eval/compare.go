package eval

func CompareRuns(baseline, candidate Run) CompareResult {
	out := CompareResult{}
	if baseline.CriticalFailuresCount == 0 && candidate.CriticalFailuresCount > 0 {
		out.CriticalRegression = true
		out.HardDeterministic = append(out.HardDeterministic, DiffItem{Name: "critical_failures", Baseline: baseline.CriticalFailuresCount, Candidate: candidate.CriticalFailuresCount, Severity: "critical"})
	}
	out.Semantic = append(out.Semantic, DiffItem{Name: "verdict", Baseline: baseline.Verdict, Candidate: candidate.Verdict, Severity: "info"})
	out.Tokens = append(out.Tokens, DiffItem{Name: "input_tokens", Baseline: baseline.Aggregate.InputTokens, Candidate: candidate.Aggregate.InputTokens, Severity: "info"})
	out.Tokens = append(out.Tokens, DiffItem{Name: "output_tokens", Baseline: baseline.Aggregate.OutputTokens, Candidate: candidate.Aggregate.OutputTokens, Severity: "info"})
	out.Cost = append(out.Cost, DiffItem{Name: "estimated_cost_usd", Baseline: baseline.Aggregate.EstimatedCostUSD, Candidate: candidate.Aggregate.EstimatedCostUSD, Severity: "info"})
	if baseline.CompletedAt != nil && candidate.CompletedAt != nil {
		out.Latency = append(out.Latency, DiffItem{
			Name: "duration_ns", Baseline: baseline.CompletedAt.Sub(baseline.StartedAt).Nanoseconds(),
			Candidate: candidate.CompletedAt.Sub(candidate.StartedAt).Nanoseconds(), Severity: "info",
		})
	}
	return out
}
