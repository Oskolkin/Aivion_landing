package aicost

import (
	"strings"
)

// Estimate holds a USD estimate for a single OpenAI call (MVP heuristic).
type Estimate struct {
	CostUSD float64
	// Known is false when the model is not in the built-in price table.
	Known bool
	// Reason is set when Known is false (e.g. "unknown_price").
	Reason string
}

// Per-million-token USD rates (input, output) for MVP — adjust as pricing changes.
var modelRatesUSDPerMillion = map[string]struct {
	In  float64
	Out float64
}{
	"gpt-4.1-mini":  {In: 0.40, Out: 1.60},
	"gpt-4.1":       {In: 2.00, Out: 8.00},
	"gpt-4o-mini":   {In: 0.15, Out: 0.60},
	"gpt-4o":        {In: 2.50, Out: 10.00},
	"gpt-3.5-turbo": {In: 0.50, Out: 1.50},
}

func normalizeModel(model string) string {
	return strings.TrimSpace(strings.ToLower(model))
}

// EstimateUSD returns approximate USD cost from token usage.
func EstimateUSD(model string, inputTokens, outputTokens int) Estimate {
	m := normalizeModel(model)
	rates, ok := modelRatesUSDPerMillion[m]
	if !ok {
		// Prefix / snapshot models: try longest prefix match
		for k, v := range modelRatesUSDPerMillion {
			if strings.HasPrefix(m, k) {
				rates, ok = v, true
				break
			}
		}
	}
	if !ok {
		return Estimate{CostUSD: 0, Known: false, Reason: "unknown_price"}
	}
	in := float64(inputTokens) / 1e6 * rates.In
	out := float64(outputTokens) / 1e6 * rates.Out
	return Estimate{CostUSD: in + out, Known: true}
}
