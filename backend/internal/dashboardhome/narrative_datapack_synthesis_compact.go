package dashboardhome

import (
	"sort"
	"strings"
	"unicode/utf8"
)

var synthesisSignalPriority = map[string]int{
	ChunkSignalRiskCandidate:        0,
	ChunkSignalOpportunityCandidate: 1,
	ChunkSignalChange:               2,
	ChunkSignalRelationship:         3,
	ChunkSignalObservation:          4,
	ChunkSignalDataQuality:          5,
}

func compactSignalsForSynthesis(signals []DashboardChunkSignal) []DashboardChunkSignal {
	if len(signals) == 0 {
		return nil
	}
	sorted := append([]DashboardChunkSignal(nil), signals...)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi := synthesisSignalPriority[sorted[i].SignalType]
		pj := synthesisSignalPriority[sorted[j].SignalType]
		if pi != pj {
			return pi < pj
		}
		return len(sorted[i].FactIDs) > len(sorted[j].FactIDs)
	})
	if len(sorted) > synthesisMaxSignals {
		sorted = sorted[:synthesisMaxSignals]
	}
	out := make([]DashboardChunkSignal, 0, len(sorted))
	for _, sig := range sorted {
		sig.FactIDs = limitStringIDs(sig.FactIDs, 5)
		sig.EntityIDs = limitStringIDs(sig.EntityIDs, 4)
		sig.Observation = truncateRunes(sig.Observation, 180)
		sig.WhyItMayMatter = truncateRunes(sig.WhyItMayMatter, 120)
		sig.RelatedMetrics = limitStringIDs(sig.RelatedMetrics, 4)
		out = append(out, sig)
	}
	return out
}

func limitStringIDs(ids []string, max int) []string {
	if max <= 0 || len(ids) == 0 {
		return nil
	}
	if len(ids) <= max {
		return append([]string(nil), ids...)
	}
	return append([]string(nil), ids[:max]...)
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:max])) + "…"
}

func trimManifestSummaryForSynthesis(summary map[string]any) map[string]any {
	if summary == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for _, key := range []string{"metric_date", "seller_account_id", "chunk_count", "dataset_counts", "limitations"} {
		if v, ok := summary[key]; ok {
			out[key] = v
		}
	}
	return out
}
