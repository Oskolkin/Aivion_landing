package dashboardhome

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// applyFunnelAIGuards enriches AI funnel analysis with deterministic metrics and guardrails.
// Seller-facing narrative (weakest explanation, drop_factors, directions, details_sections) must come from AI.
// Backend only fills comparisons, structural weakest fields, evidence filtering, and sanitization.
func applyFunnelAIGuards(
	ai FunnelAIAnalysisWidget,
	stages []FunnelStage,
	ev FunnelEvidenceSnapshot,
	funnelHistory7d []dbgen.ProductAnalyticsDaily,
	currentAnalytics *dbgen.ProductAnalyticsDaily,
	previousAnalytics *dbgen.ProductAnalyticsDaily,
	topActions []TopActionItem,
) FunnelAIAnalysisWidget {
	sampleDays7d := len(filterAccountProductAnalyticsForFunnel(funnelHistory7d))
	ai = normalizeFunnelAIAnalysisTerminology(ai)
	// Deterministic comparison metrics (not narrative): merge into AI rows so gaps are filled.
	deterministicPrev := buildPreviousComparisons(stages)
	deterministicBiz := buildPreviousBusinessComparisons(currentAnalytics, previousAnalytics)
	if len(deterministicPrev) > 0 || len(deterministicBiz) > 0 {
		ai.Comparisons.PreviousPeriod = mergeComparisonItems(ai.Comparisons.PreviousPeriod, deterministicPrev, deterministicBiz)
	}
	if filled := buildSevenDayAverageComparisons(stages, funnelHistory7d, currentAnalytics); len(filled) > 0 {
		ai.Comparisons.SevenDayAverage = filled
	}
	ai.Comparisons.PreviousPeriod = reorderComparisonItems(ai.Comparisons.PreviousPeriod)
	ai.Comparisons.SevenDayAverage = reorderComparisonItems(ai.Comparisons.SevenDayAverage)
	ai.Comparisons.PreviousPeriod = normalizeComparisonLabels(ai.Comparisons.PreviousPeriod)
	ai.Comparisons.SevenDayAverage = normalizeComparisonLabels(ai.Comparisons.SevenDayAverage)
	stageItems, conversionItems, businessItems, contextItems := classifyComparisonItems(ai.Comparisons.PreviousPeriod)
	deterministicConv := buildConversionMetricsFallback(stages, funnelHistory7d)
	ai.Comparisons.StageMetrics = mergeComparisonItems(ai.Comparisons.StageMetrics, stageItems, deterministicPrev)
	ai.Comparisons.ConversionMetrics = mergeComparisonItems(ai.Comparisons.ConversionMetrics, conversionItems, deterministicConv)
	ai.Comparisons.BusinessMetrics = mergeComparisonItems(ai.Comparisons.BusinessMetrics, businessItems, deterministicBiz)
	if len(ai.Comparisons.ContextSignals) == 0 {
		ai.Comparisons.ContextSignals = contextItems
	}
	ai.Comparisons.StageMetrics = normalizeComparisonLabels(ai.Comparisons.StageMetrics)
	if len(ai.Comparisons.ConversionMetrics) == 0 {
		ai.Comparisons.ConversionMetrics = deterministicConv
	}
	ai.Comparisons.ConversionMetrics = normalizeComparisonLabels(ai.Comparisons.ConversionMetrics)
	ai.Comparisons.BusinessMetrics = normalizeComparisonLabels(ai.Comparisons.BusinessMetrics)
	if len(ai.Comparisons.ContextSignals) == 0 {
		ai.Comparisons.ContextSignals = buildContextSignalsFallback(sampleDays7d)
	}
	hasSevenDay := len(ai.Comparisons.SevenDayAverage) > 0
	limitedHistory := sampleDays7d > 0 && sampleDays7d < 7

	// AI commentary is the primary source; sanitize only (strip inline limited-history
	// note, legacy terms, soften causal claims, fix 7-day wording when history is limited).
	ai.ComparisonsCommentary.PreviousPeriod = sanitizeComparisonCommentary(ai.ComparisonsCommentary.PreviousPeriod, true, limitedHistory)
	ai.ComparisonsCommentary.SevenDayAverage = sanitizeComparisonCommentary(ai.ComparisonsCommentary.SevenDayAverage, hasSevenDay, limitedHistory)

	// Deterministic analytical fallback only when AI text is missing or just a number dump.
	hasFunnelPrev := len(ai.Comparisons.StageMetrics) > 0 || len(ai.Comparisons.ConversionMetrics) > 0
	if hasFunnelPrev && commentaryNeedsFallback(ai.ComparisonsCommentary.PreviousPeriod) {
		ai.ComparisonsCommentary.PreviousPeriod = buildPreviousCommentaryFallback(ai.Comparisons.StageMetrics, ai.Comparisons.ConversionMetrics)
	}
	if hasSevenDay && commentaryNeedsFallback(ai.ComparisonsCommentary.SevenDayAverage) {
		ai.ComparisonsCommentary.SevenDayAverage = buildSevenDayCommentaryFallback(ai.Comparisons.SevenDayAverage, limitedHistory)
	}

	// Limited-history note lives only as a separate context signal (rendered as a
	// dedicated InfoNote), never inline in the commentary.
	if limitedHistory {
		ai.Comparisons.ContextSignals = ensureLimitedHistorySignal(ai.Comparisons.ContextSignals)
	}

	// Legacy summary fields: strip 7d contradiction only; do not synthesize summary text.
	ai.Summary = stripSevenDayMentionsIfNoData(ai.Summary, hasSevenDay)
	ai.ShortSummary = stripSevenDayMentionsIfNoData(ai.ShortSummary, hasSevenDay)
	ai.TrendVs7D = stripSevenDayMentionsIfNoData(ai.TrendVs7D, hasSevenDay)
	ai.Summary = softenCausalClaims(ai.Summary)
	ai.ShortSummary = softenCausalClaims(ai.ShortSummary)
	ai.TrendVsYesterday = softenCausalClaims(ai.TrendVsYesterday)
	ai.TrendVs7D = softenCausalClaims(ai.TrendVs7D)
	ai.FullAnalysis = softenCausalClaims(ai.FullAnalysis)
	ai.Details = softenCausalClaims(ai.Details)

	// Drop factors: AI primary; legacy influence_factors only if drop_factors empty.
	if len(ai.DropFactors) == 0 && len(ai.InfluenceFactors) > 0 {
		ai.DropFactors = legacyInfluenceToDropFactors(ai.InfluenceFactors)
	}
	ai.DropFactors = filterDropFactorsByEvidence(ai.DropFactors, ev)
	for i := range ai.DropFactors {
		ai.DropFactors[i].Label = canonicalFunnelLabelByKey(ai.DropFactors[i].Key, ai.DropFactors[i].Label)
		ai.DropFactors[i].Metric = replaceLegacyFunnelTerms(ai.DropFactors[i].Metric)
		ai.DropFactors[i].Explanation = softenCausalClaims(ai.DropFactors[i].Explanation)
		ai.DropFactors[i].Limitation = softenCausalClaims(ai.DropFactors[i].Limitation)
		ai.DropFactors[i] = enrichDropFactorFallback(ai.DropFactors[i], stages)
	}
	ai.DropFactors = sortDropFactorsByGroup(ai.DropFactors)
	if len(ai.DropFactors) > 5 {
		ai.DropFactors = ai.DropFactors[:5]
	}

	// Directions: sanitize only; never synthesize from recommended_focus.
	ai.CheckDirections = sanitizeCheckDirections(ai.CheckDirections)
	ai.CheckDirections = dedupeCheckDirectionsFromTopActions(ai.CheckDirections, topActions)
	for i := range ai.CheckDirections {
		ai.CheckDirections[i].Label = canonicalFunnelLabelByKey(ai.CheckDirections[i].Key, ai.CheckDirections[i].Label)
		ai.CheckDirections[i].Text = softenCausalClaims(ai.CheckDirections[i].Text)
		ai.CheckDirections[i].Text = softenUnsupportedSKUFunnelClaims(ai.CheckDirections[i].Text)
		ai.CheckDirections[i] = enrichCheckDirectionFallback(ai.CheckDirections[i], ai.WeakestStage.Label)
	}
	if len(ai.CheckDirections) > 4 {
		ai.CheckDirections = ai.CheckDirections[:4]
	}

	// Weakest stage: keep AI narrative; enrich metrics only.
	if funnelWeakestStageFromAI(ai.WeakestStage) {
		enrichWeakestStageMetrics(&ai.WeakestStage, stages, ai.Comparisons)
	} else if len(stages) > 0 {
		ai.WeakestStage = deriveWeakestStageStructural(stages)
		enrichWeakestStageMetrics(&ai.WeakestStage, stages, ai.Comparisons)
	} else if strings.TrimSpace(ai.WeakestStage.Status) == "" {
		ai.WeakestStage = FunnelWeakestStage{
			Status: "unavailable", Label: "Недостаточно данных", Tone: "neutral",
		}
	}

	// Details: prefer structured sections; legacy plain text only as fallback for expand UI.
	if len(ai.DetailsSections) == 0 && strings.TrimSpace(ai.Details) == "" {
		ai.Details = strings.TrimSpace(ai.FullAnalysis)
	}
	for i := range ai.DetailsSections {
		ai.DetailsSections[i].Title = softenCausalClaims(ai.DetailsSections[i].Title)
		ai.DetailsSections[i].Title = replaceLegacyFunnelTerms(ai.DetailsSections[i].Title)
		for j := range ai.DetailsSections[i].Items {
			ai.DetailsSections[i].Items[j].Text = softenCausalClaims(ai.DetailsSections[i].Items[j].Text)
			ai.DetailsSections[i].Items[j].Text = softenUnsupportedSKUFunnelClaims(ai.DetailsSections[i].Items[j].Text)
			ai.DetailsSections[i].Items[j].Text = replaceLegacyFunnelTerms(ai.DetailsSections[i].Items[j].Text)
			for k := range ai.DetailsSections[i].Items[j].Metrics {
				ai.DetailsSections[i].Items[j].Metrics[k] = replaceLegacyFunnelTerms(ai.DetailsSections[i].Items[j].Metrics[k])
			}
		}
	}
	ai.DetailsSections = normalizeDetailsSectionsOrder(ai.DetailsSections)
	if len(ai.DetailsSections) == 0 {
		ai.DetailsSections = nil
	}

	// Mirror drop_factors to legacy influence_factors for backward-compatible clients.
	ai.InfluenceFactors = dropFactorsToLegacyInfluence(ai.DropFactors)

	ai = enrichFunnelMainConclusionWidget(ai)

	return ai
}

func normalizeFunnelAIAnalysisTerminology(ai FunnelAIAnalysisWidget) FunnelAIAnalysisWidget {
	ai.Summary = replaceLegacyFunnelTerms(ai.Summary)
	ai.ShortSummary = replaceLegacyFunnelTerms(ai.ShortSummary)
	ai.TrendVsYesterday = replaceLegacyFunnelTerms(ai.TrendVsYesterday)
	ai.TrendVs7D = replaceLegacyFunnelTerms(ai.TrendVs7D)
	ai.FullAnalysis = replaceLegacyFunnelTerms(ai.FullAnalysis)
	ai.Details = replaceLegacyFunnelTerms(ai.Details)
	return ai
}

func canonicalFunnelLabelByKey(key, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "orders":
		return "Заказанные единицы"
	case "delivered_units":
		return "Доставленные единицы"
	default:
		return replaceLegacyFunnelTerms(fallback)
	}
}

func replaceLegacyFunnelTerms(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	replacements := map[string]string{
		"Оформленные заказы": "Заказанные единицы",
		"оформленные заказы": "заказанные единицы",
		"Выкупили товар":     "Доставленные единицы",
		"выкупили товар":     "доставленные единицы",
		"Выкупы":             "Доставленные единицы",
		"выкупы":             "доставленные единицы",
		"Выкупленные":        "Доставленные",
		"выкупленные":        "доставленные",
		"Выкупленных":        "Доставленных",
		"выкупленных":        "доставленных",
		"Выкупленным":        "Доставленным",
		"выкупленным":        "доставленным",
		"Выкупленными":       "Доставленными",
		"выкупленными":       "доставленными",
	}
	for old, newVal := range replacements {
		t = strings.ReplaceAll(t, old, newVal)
	}
	// Bare "выкуп" family: RE2 \b is ASCII-only and never matches around Cyrillic,
	// so use explicit non-letter boundaries.
	t = reBuyoutWord.ReplaceAllStringFunc(t, func(m string) string {
		sub := reBuyoutWord.FindStringSubmatch(m)
		lead, word, trail := sub[1], sub[2], sub[3]
		repl := "доставка"
		if strings.HasPrefix(strings.ToLower(word), "выкуплен") {
			repl = "доставленные"
		}
		if r := []rune(word); len(r) > 0 && unicode.IsUpper(r[0]) {
			rr := []rune(repl)
			rr[0] = unicode.ToUpper(rr[0])
			repl = string(rr)
		}
		return lead + repl + trail
	})
	return t
}

var reBuyoutWord = regexp.MustCompile(`(?i)(^|[^\p{L}])(выкуплен\p{Cyrillic}*|выкуп\p{Cyrillic}*)([^\p{L}]|$)`)

var reFirstSignedNumber = regexp.MustCompile(`-?\d+(?:\.\d+)?`)

func parseSignedDeltaValue(delta string) (float64, bool) {
	s := strings.TrimSpace(delta)
	if s == "" {
		return 0, false
	}
	s = strings.ReplaceAll(s, "−", "-")
	s = strings.ReplaceAll(s, ",", ".")
	m := reFirstSignedNumber.FindString(s)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// funnelRegionFromKey maps a stage/conversion key to a funnel region so the
// fallback commentary stays universal (not hardcoded to card→cart).
func funnelRegionFromKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "impressions", "product_views", "impressions_to_product_views", "impressions_to_views":
		return "upper"
	case "add_to_cart", "product_views_to_add_to_cart", "views_to_cart":
		return "middle"
	case "orders", "add_to_cart_to_orders", "cart_to_order", "add_to_cart_to_order":
		return "lower"
	case "delivered_units", "orders_to_delivered_units", "orders_to_delivered", "order_to_delivered":
		return "delivery"
	default:
		return ""
	}
}

// weakestFunnelRegion returns the region with the strongest negative delta across
// the provided comparison sets, or "" if nothing funnel-related is negative.
func weakestFunnelRegion(itemSets ...[]FunnelComparisonItem) string {
	region := ""
	worst := 0.0
	for _, items := range itemSets {
		for _, it := range items {
			r := funnelRegionFromKey(it.Key)
			if r == "" {
				continue
			}
			v, ok := parseSignedDeltaValue(it.Delta)
			if !ok || v >= 0 {
				continue
			}
			if region == "" || v < worst {
				region = r
				worst = v
			}
		}
	}
	return region
}

// lowerTransitionImproved reports whether the cart→order transition got stronger,
// i.e. it partially compensates an upstream drop.
func lowerTransitionImproved(conversionItems []FunnelComparisonItem) bool {
	for _, it := range conversionItems {
		if funnelRegionFromKey(it.Key) != "lower" {
			continue
		}
		if v, ok := parseSignedDeltaValue(it.Delta); ok && v > 0 {
			return true
		}
	}
	return false
}

// buildPreviousCommentaryFallback produces an analytical (non number-dump) summary
// for the previous-summary column when AI commentary is missing or unusable.
func buildPreviousCommentaryFallback(stageItems, conversionItems []FunnelComparisonItem) string {
	switch weakestFunnelRegion(conversionItems, stageItems) {
	case "upper":
		return "По сравнению с прошлой сводкой основная просадка в верхней части воронки: товар реже доходит от показов до просмотра карточек, поэтому слабее наполняется вся остальная цепочка."
	case "middle":
		base := "По сравнению с прошлой сводкой основная просадка в середине воронки: после просмотра карточек покупатели реже добавляют товары в корзину."
		if lowerTransitionImproved(conversionItems) {
			base += " Нижний переход «корзина → заказ» стал сильнее, но полностью не компенсирует падение базы добавлений."
		}
		return base
	case "lower":
		return "По сравнению с прошлой сводкой слабее проходит нижний переход: спрос есть, но из корзины реже доходит до заказа."
	case "delivery":
		return "По сравнению с прошлой сводкой слабее проходит выполнение заказов: заказы хуже доходят до доставки, хотя интерес к товару держится."
	default:
		return "По сравнению с прошлой сводкой явного слабого места в воронке не видно: ключевые этапы и переходы не ухудшились."
	}
}

// buildSevenDayCommentaryFallback mirrors the previous-summary fallback but uses
// period-safe wording when history is limited (sample_days < 7).
func buildSevenDayCommentaryFallback(sevenDayItems []FunnelComparisonItem, limitedHistory bool) string {
	prefix := "Относительно среднего за последние 7 дней"
	if limitedHistory {
		prefix = "Относительно доступного среднего за период"
	}
	switch weakestFunnelRegion(sevenDayItems) {
	case "upper":
		return prefix + " слабее работает верх воронки: показы и просмотры карточек ниже обычного уровня."
	case "middle":
		return prefix + " слабее работает середина воронки: добавления в корзину ниже обычного уровня, тогда как нижние этапы держатся лучше."
	case "lower":
		return prefix + " слабее проходит переход из корзины в заказ, тогда как верх воронки ближе к норме."
	case "delivery":
		return prefix + " слабее проходит доставка заказов относительно обычного уровня."
	default:
		return prefix + " воронка держится около обычного уровня без явного слабого места."
	}
}

func normalizeComparisonLabels(items []FunnelComparisonItem) []FunnelComparisonItem {
	out := make([]FunnelComparisonItem, len(items))
	for i := range items {
		out[i] = items[i]
		out[i].Label = canonicalFunnelLabelByKey(out[i].Key, out[i].Label)
		out[i].Value = replaceLegacyFunnelTerms(out[i].Value)
		out[i].Delta = replaceLegacyFunnelTerms(out[i].Delta)
	}
	return out
}

func buildConversionMetricsFallback(stages []FunnelStage, history []dbgen.ProductAnalyticsDaily) []FunnelComparisonItem {
	if len(stages) < 2 {
		return nil
	}
	out := make([]FunnelComparisonItem, 0, 4)
	for i := 1; i < len(stages); i++ {
		prev := stages[i-1]
		cur := stages[i]
		if prev.Value == nil || cur.Value == nil || *prev.Value <= 0 || cur.ConversionFromPrevious == nil {
			continue
		}
		key := fmt.Sprintf("%s_to_%s", prev.Key, cur.Key)
		item := FunnelComparisonItem{
			Key:   key,
			Label: fmt.Sprintf("%s → %s", transitionStageLabel(prev.Key, prev.Label), transitionStageLabel(cur.Key, cur.Label)),
			Value: fmt.Sprintf("%.1f%%", *cur.ConversionFromPrevious),
			Tone:  "neutral",
		}
		if prev.PreviousDayValue != nil && cur.PreviousDayValue != nil && *prev.PreviousDayValue > 0 {
			prevConv := float64(*cur.PreviousDayValue) / float64(*prev.PreviousDayValue) * 100
			dpp := math.Round((*cur.ConversionFromPrevious-prevConv)*100) / 100
			item.Delta = fmt.Sprintf("%s к прошлому дню", formatSignedPercentPP(dpp))
			item.Tone = toneByPPDelta(dpp)
		}
		if item.Delta == "" {
			if avg := averageTransitionConversion(history, prev.Key, cur.Key); avg != nil {
				dpp := math.Round((*cur.ConversionFromPrevious-*avg)*100) / 100
				item.Delta = fmt.Sprintf("%s к среднему за 7 дней", formatSignedPercentPP(dpp))
				item.Tone = toneByPPDelta(dpp)
			}
		}
		out = append(out, item)
	}
	return out
}

func transitionStageLabel(key, fallback string) string {
	switch strings.TrimSpace(key) {
	case "impressions":
		return "Показы"
	case "product_views":
		return "Карточка"
	case "add_to_cart":
		return "Корзина"
	case "orders":
		return "Заказанные единицы"
	case "delivered_units":
		return "Доставленные единицы"
	default:
		return canonicalFunnelLabelByKey(key, fallback)
	}
}

func buildContextSignalsFallback(sampleDays7d int) []FunnelComparisonItem {
	if sampleDays7d < 7 {
		return []FunnelComparisonItem{
			{
				Key:   "history_status",
				Label: "История сравнения",
				Value: "Сравнение ограничено: доступно меньше 7 дней истории.",
				Delta: "limited",
				Tone:  "warning",
			},
		}
	}
	return []FunnelComparisonItem{
		{Key: "history_status", Label: "История сравнения", Value: "История сравнения доступна.", Tone: "neutral"},
	}
}

func enrichCheckDirectionFallback(d FunnelTextItem, weakestLabel string) FunnelTextItem {
	key := strings.ToLower(strings.TrimSpace(d.Key))
	if strings.TrimSpace(d.Priority) == "" {
		switch key {
		case "card_quality":
			d.Priority = "1"
		case "traffic_relevance", "paid_traffic_quality":
			d.Priority = "2"
		case "stock_mapping", "stock_linkage":
			d.Priority = "3"
		case "price_or_aov":
			d.Priority = "4"
		default:
			d.Priority = "3"
		}
	}
	if strings.TrimSpace(d.DataNeeded) == "" && strings.Contains(strings.ToLower(d.Text), "sku-детализация") {
		d.DataNeeded = "SKU-level воронка в MVP недоступна; используйте косвенные признаки: падение заказов, доля выручки, остатки и рекламные сигналы."
	}
	if strings.TrimSpace(d.DataNeeded) == "" {
		d.DataNeeded = "Сопоставьте сигнал воронки с динамикой заказов, выручки, остатков и рекламных метрик за тот же период."
	}
	if strings.TrimSpace(d.Reason) == "" {
		if strings.TrimSpace(weakestLabel) != "" {
			d.Reason = "Главное ухудшение видно на переходе " + weakestLabel + "."
		} else {
			d.Reason = "Нужно уточнить вклад факторов в текущее ухудшение воронки."
		}
	}
	d.Label = replaceLegacyFunnelTerms(d.Label)
	d.Text = replaceLegacyFunnelTerms(d.Text)
	return d
}

func funnelWeakestStageFromAI(ws FunnelWeakestStage) bool {
	st := strings.TrimSpace(ws.Status)
	switch st {
	case "stable", "unavailable":
		return st != ""
	case "available":
		return strings.TrimSpace(ws.Label) != "" || strings.TrimSpace(ws.Metric) != ""
	default:
		return strings.TrimSpace(ws.Label) != "" || strings.TrimSpace(ws.Metric) != "" || strings.TrimSpace(ws.Explanation) != ""
	}
}

func enrichWeakestStageMetrics(ws *FunnelWeakestStage, stages []FunnelStage, cmp FunnelComparisons) {
	if ws == nil {
		return
	}
	if strings.TrimSpace(ws.Value) == "" {
		ws.Value = weakestStageValueFromStages(*ws, stages)
	}
	prevDelta := findComparisonDelta(cmp.PreviousPeriod, ws.Key, ws.Metric, ws.Label)
	if prevDelta != "" && strings.TrimSpace(ws.DeltaPrevious) == "" {
		ws.DeltaPrevious = prevDelta
	}
	sevenDelta := findComparisonDelta(cmp.SevenDayAverage, ws.Key, ws.Metric, ws.Label)
	if sevenDelta != "" && strings.TrimSpace(ws.Delta7D) == "" {
		ws.Delta7D = sevenDelta
	}
	if strings.TrimSpace(ws.Explanation) == "" {
		ws.Explanation = strings.TrimSpace(ws.Reason)
	}
	if strings.TrimSpace(ws.Reason) == "" {
		ws.Reason = strings.TrimSpace(ws.Explanation)
	}
}

func weakestStageValueFromStages(ws FunnelWeakestStage, stages []FunnelStage) string {
	key := strings.TrimSpace(ws.Key)
	for _, s := range stages {
		if strings.TrimSpace(s.Key) == key && s.Value != nil {
			return formatInt(*s.Value)
		}
	}
	return ""
}

// deriveWeakestStageStructural picks weakest stage by metrics only (no seller-facing reason text).
func deriveWeakestStageStructural(stages []FunnelStage) FunnelWeakestStage {
	ws := deriveWeakestStage(stages)
	ws.Reason = ""
	ws.Explanation = ""
	return ws
}

func legacyInfluenceToDropFactors(items []FunnelTextItem) []FunnelDropFactor {
	out := make([]FunnelDropFactor, 0, len(items))
	for _, it := range items {
		out = append(out, FunnelDropFactor{
			Key: it.Key, Label: it.Label, EvidenceLevel: "likely", Explanation: it.Text, Tone: it.Tone,
		})
	}
	return out
}

func dropFactorsToLegacyInfluence(factors []FunnelDropFactor) []FunnelTextItem {
	if len(factors) == 0 {
		return nil
	}
	out := make([]FunnelTextItem, 0, len(factors))
	for _, f := range factors {
		text := strings.TrimSpace(f.Explanation)
		if text == "" {
			text = strings.TrimSpace(f.Metric)
		}
		out = append(out, FunnelTextItem{
			Key: f.Key, Label: f.Label, Text: text, Tone: f.Tone,
		})
	}
	return out
}

func dedupeCheckDirectionsFromTopActions(directions []FunnelTextItem, topActions []TopActionItem) []FunnelTextItem {
	if len(directions) == 0 || len(topActions) == 0 {
		return directions
	}
	actionTexts := make([]string, 0, len(topActions)*2)
	for _, a := range topActions {
		actionTexts = append(actionTexts, normalizeDedupeKey(a.Title), normalizeDedupeKey(a.NextStep), normalizeDedupeKey(a.Reason))
	}
	out := make([]FunnelTextItem, 0, len(directions))
	for _, d := range directions {
		blob := normalizeDedupeKey(d.Label + " " + d.Text)
		if blob == "" {
			continue
		}
		dup := false
		for _, at := range actionTexts {
			if at == "" {
				continue
			}
			if strings.Contains(blob, at) || strings.Contains(at, blob) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, d)
		}
	}
	return out
}

func normalizeDedupeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func enrichFunnelMainConclusionWidget(ai FunnelAIAnalysisWidget) FunnelAIAnalysisWidget {
	payload := funnelAIWidgetToAisummaries(ai)
	resolved := aisummaries.ResolveFunnelMainConclusion(&payload)
	if strings.TrimSpace(resolved.Text) != "" {
		ai.MainConclusion = resolved.Text
	}
	if strings.TrimSpace(ai.WeakestStage.Explanation) == "" && strings.TrimSpace(ai.MainConclusion) != "" {
		ai.WeakestStage.Explanation = truncateSentences(ai.MainConclusion, 2)
	}
	return ai
}

func funnelAIWidgetToAisummaries(ai FunnelAIAnalysisWidget) aisummaries.FunnelAIAnalysis {
	out := aisummaries.FunnelAIAnalysis{
		Status:         string(ai.Status),
		MainConclusion: strings.TrimSpace(ai.MainConclusion),
		ActionItems:    append([]string(nil), ai.ActionItems...),
	}
	if strings.TrimSpace(ai.WeakestStage.Status) != "" || strings.TrimSpace(ai.WeakestStage.Label) != "" {
		out.WeakestStage = &aisummaries.FunnelWeakestStage{
			Status:        ai.WeakestStage.Status,
			Key:           ai.WeakestStage.Key,
			Label:         ai.WeakestStage.Label,
			Metric:        ai.WeakestStage.Metric,
			Value:         ai.WeakestStage.Value,
			Explanation:   ai.WeakestStage.Explanation,
			Reason:        ai.WeakestStage.Reason,
			DeltaPrevious: ai.WeakestStage.DeltaPrevious,
			Delta7D:       ai.WeakestStage.Delta7D,
			Tone:          ai.WeakestStage.Tone,
		}
	}
	out.Comparisons = &aisummaries.FunnelComparisons{
		PreviousPeriod:    mapWidgetComparisonItems(ai.Comparisons.PreviousPeriod),
		SevenDayAverage:   mapWidgetComparisonItems(ai.Comparisons.SevenDayAverage),
		StageMetrics:      mapWidgetComparisonItems(ai.Comparisons.StageMetrics),
		ConversionMetrics: mapWidgetComparisonItems(ai.Comparisons.ConversionMetrics),
		BusinessMetrics:   mapWidgetComparisonItems(ai.Comparisons.BusinessMetrics),
		ContextSignals:    mapWidgetComparisonItems(ai.Comparisons.ContextSignals),
	}
	if strings.TrimSpace(ai.ComparisonsCommentary.PreviousPeriod) != "" || strings.TrimSpace(ai.ComparisonsCommentary.SevenDayAverage) != "" {
		out.ComparisonsCommentary = &aisummaries.FunnelComparisonsCommentary{
			PreviousPeriod:  ai.ComparisonsCommentary.PreviousPeriod,
			SevenDayAverage: ai.ComparisonsCommentary.SevenDayAverage,
		}
	}
	return out
}

func mapWidgetComparisonItems(items []FunnelComparisonItem) []aisummaries.FunnelComparisonItem {
	out := make([]aisummaries.FunnelComparisonItem, 0, len(items))
	for _, it := range items {
		out = append(out, aisummaries.FunnelComparisonItem{
			Key: it.Key, Label: it.Label, Value: it.Value, Delta: it.Delta, Tone: it.Tone,
		})
	}
	return out
}

func truncateSentences(text string, max int) string {
	t := strings.TrimSpace(text)
	if t == "" || max <= 0 {
		return ""
	}
	out := make([]string, 0, max)
	start := 0
	for i, r := range t {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		seg := strings.TrimSpace(t[start : i+1])
		if seg != "" {
			out = append(out, seg)
			if len(out) >= max {
				return strings.Join(out, " ")
			}
		}
		start = i + 1
	}
	if tail := strings.TrimSpace(t[start:]); tail != "" && len(out) < max {
		out = append(out, tail)
	}
	return strings.Join(out, " ")
}
