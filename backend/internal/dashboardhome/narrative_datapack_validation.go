package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
)

var dataPackForbiddenSKULevelAdPatterns = []string{
	"расход по sku",
	"sku сливает",
	"рекламный p&l sku",
	"sku-level",
	"sku level",
	"по sku",
}

var dataPackForbiddenFinancePatterns = []string{
	"маржа",
	"margin",
	"прибыль",
	"profit",
	"рентабельн",
}

var dataPackForbiddenPriceCutPatterns = []string{
	"снизьте цену",
	"снизить цену",
	"уменьшите цену",
	"снизь цену",
}

var dataPackNumericPattern = regexp.MustCompile(`[\d\s]+(?:[.,]\d+)?\s*(?:₽|%|дн\.)`)

func validateDataPackNarrativeExtras(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	if ctx.ContextSource != ContextSourceSellerAIDataPack &&
		ctx.ContextSource != ContextSourceSellerAIDataPackChunks &&
		ctx.ContextSource != ContextSourceSellerAIDataPackOneShot {
		return nil
	}
	var errs []string
	errs = append(errs, validateDataPackFactIDs(payload, ctx)...)
	errs = append(errs, validateDataPackForbiddenAssumptions(payload, ctx)...)
	errs = append(errs, validateDataPackNumericGuards(payload, ctx)...)
	return errs
}

func validateDataPackFactIDs(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	if payload == nil || len(ctx.DataPackFactIDs) == 0 {
		return nil
	}
	var errs []string
	check := func(prefix string, ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := ctx.DataPackFactIDs[id]; !ok {
				errs = append(errs, fmt.Sprintf("%s_unknown_fact_id_%s", prefix, id))
			}
		}
	}
	for i, reason := range payload.HealthReasons {
		check(fmt.Sprintf("health_reason_%d", i), reason.FactIDs)
	}
	for i, action := range payload.TopActions {
		check(fmt.Sprintf("top_action_%d", i), action.FactIDs)
	}
	return errs
}

func validateDataPackForbiddenAssumptions(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	if payload == nil {
		return nil
	}
	var errs []string
	texts := collectNarrativeTexts(payload)
	for i, text := range texts {
		lower := strings.ToLower(text)
		for _, pat := range dataPackForbiddenSKULevelAdPatterns {
			if strings.Contains(lower, pat) {
				errs = append(errs, fmt.Sprintf("narrative_text_%d_forbidden_sku_level_ad", i))
				break
			}
		}
		if !ctx.FinanceAvailable {
			for _, pat := range dataPackForbiddenFinancePatterns {
				if strings.Contains(lower, pat) {
					errs = append(errs, fmt.Sprintf("narrative_text_%d_finance_unavailable", i))
					break
				}
			}
		}
		if !ctx.PriceConstraintsAvailable {
			for _, pat := range dataPackForbiddenPriceCutPatterns {
				if strings.Contains(lower, pat) {
					errs = append(errs, fmt.Sprintf("narrative_text_%d_forbidden_price_cut", i))
					break
				}
			}
		}
	}
	return errs
}

func validateDataPackNumericGuards(payload *dashboardNarrativePayload, ctx dashboardNarrativeValidationContext) []string {
	if payload == nil || len(ctx.AllowedFormattedValues) == 0 {
		return nil
	}
	var errs []string
	for i, text := range collectNarrativeTexts(payload) {
		matches := dataPackNumericPattern.FindAllString(text, -1)
		for _, m := range matches {
			normalized := strings.TrimSpace(m)
			if normalized == "" {
				continue
			}
			if _, ok := ctx.AllowedFormattedValues[normalized]; ok {
				continue
			}
			// Allow small integers like "1 SKU" without hard error — only currency/percent patterns.
			if strings.Contains(normalized, "₽") || strings.Contains(normalized, "%") {
				errs = append(errs, fmt.Sprintf("narrative_text_%d_unbacked_number", i))
				break
			}
		}
	}
	return errs
}

func collectNarrativeTexts(payload *dashboardNarrativePayload) []string {
	var out []string
	out = append(out, payload.Headline, payload.ExecutiveSummary, payload.ImprovementSummary)
	out = append(out, payload.DataQualityNotes...)
	for _, r := range payload.HealthReasons {
		out = append(out, r.Title, r.Reason)
	}
	for _, a := range payload.TopActions {
		out = append(out, a.Title, a.Reason, a.NextStep, a.ExpectedEffect)
	}
	if payload.StockCoverageNarrative != nil {
		out = append(out, payload.StockCoverageNarrative.Summary, payload.StockCoverageNarrative.RiskComment, payload.StockCoverageNarrative.NoSalesInsight)
	}
	if payload.AdEfficiencyNarrative != nil {
		out = append(out, payload.AdEfficiencyNarrative.Summary)
	}
	if payload.SellerPulseNarrative != nil {
		out = append(out, payload.SellerPulseNarrative.Headline, payload.SellerPulseNarrative.Summary)
	}
	return out
}

func buildDataPackFormattedValueAllowlist(factIDs map[string]struct{}, catalog map[string]dashboardNarrativeMetric) map[string]struct{} {
	out := map[string]struct{}{}
	for id := range factIDs {
		if m, ok := catalog[id]; ok && m.Value != "" {
			out[m.Value] = struct{}{}
		}
	}
	return out
}

func buildDataPackFactIDSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}
