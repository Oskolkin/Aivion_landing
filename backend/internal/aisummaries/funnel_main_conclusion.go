package aisummaries

import (
	"fmt"
	"regexp"
	"strings"
)

const funnelMainConclusionMinWeakRunes = 120

var (
	funnelConclusionDigitsRE      = regexp.MustCompile(`\d`)
	funnelConclusionPercentRE     = regexp.MustCompile(`%|п\.п\.|₽`)
	funnelConclusionStageRE       = regexp.MustCompile(`(?i)(показ|карточк|корзин|заказ|достав|воронк|переход|этап|→)`)
	funnelConclusionImpactRE      = regexp.MustCompile(`(?i)(меньше|ограничива|поток|доходит|компенсир|главн[а-я]*\s+причин|просадк|снизил|снизил|давит|выручк)`)
	funnelConclusionActionRE      = regexp.MustCompile(`(?i)(что сделать|нужно|разобрать|обновить|выгрузить|сравнить|проверить\s+карточ|кампани|sku|фото|цен|остатк|доставк)`)
	funnelConclusionGenericOnlyRE = regexp.MustCompile(`(?i)^(воронка просела|просел[а-я]* верх|главное ухудшение).*(проанализируйте|проведите анализ|обратите внимание|проверьте показател|улучшите карточки)[^.?!]*[.?!]?$`)
	funnelConclusionGenericHitRE  = regexp.MustCompile(`(?i)(проанализируйте ситуацию|проведите анализ|обратите внимание на воронку|проверьте показатели|улучшите карточки)`)
	funnelDeltaNegativePPRE       = regexp.MustCompile(`(?i)(изменил[а-я]*|составил[а-я]*)\s+на\s*[-−]\s*(\d+[.,]\d+)\s*п\.п\.`)
)

// FunnelMainConclusionResolve captures how main_conclusion was chosen for audit/debug.
type FunnelMainConclusionResolve struct {
	Text           string
	Source         string // ai | ai_repaired | promoted_weakest | promoted_commentary | fallback | empty
	Quality        string // ok | weak | repaired
	RepairReason   string
	FallbackReason string
}

// FunnelMainConclusionSignalScore counts how many of the four quality signals are present (0–4).
func FunnelMainConclusionSignalScore(text string) int {
	t := strings.TrimSpace(text)
	if t == "" {
		return 0
	}
	score := 0
	if funnelConclusionStageRE.MatchString(t) {
		score++
	}
	if funnelConclusionDigitsRE.MatchString(t) && (funnelConclusionPercentRE.MatchString(t) || funnelMetricWordRE.MatchString(t)) {
		score++
	}
	if funnelConclusionImpactRE.MatchString(t) {
		score++
	}
	if funnelConclusionActionRE.MatchString(t) {
		score++
	}
	return score
}

var funnelMetricWordRE = regexp.MustCompile(`(?i)(просмотр|показ|заказ|выручк|конверси)`)

// FunnelMainConclusionQualityWeak reports whether text should be replaced by deterministic fallback.
// AI-first: returns false when at least 3 of 4 content signals are present.
func FunnelMainConclusionQualityWeak(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if funnelMainConclusionIsGenericOnly(t) {
		return true
	}
	score := FunnelMainConclusionSignalScore(t)
	if score >= 3 {
		return false
	}
	runes := len([]rune(t))
	if runes < 80 {
		return true
	}
	if runes < funnelMainConclusionMinWeakRunes && score < 2 {
		return true
	}
	if !funnelConclusionDigitsRE.MatchString(t) && !funnelConclusionPercentRE.MatchString(t) {
		return true
	}
	if score < 2 {
		return true
	}
	if !funnelConclusionActionRE.MatchString(t) {
		return true
	}
	return false
}

func funnelMainConclusionIsGenericOnly(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if funnelConclusionGenericOnlyRE.MatchString(t) {
		return true
	}
	if funnelConclusionGenericHitRE.MatchString(t) && FunnelMainConclusionSignalScore(t) < 2 {
		return true
	}
	lower := strings.ToLower(t)
	if strings.Contains(lower, "что сделать:") {
		if strings.Contains(lower, "проверьте трафик") || strings.Contains(lower, "проверьте показател") {
			return FunnelMainConclusionSignalScore(t) < 3
		}
	}
	return false
}

// NormalizeFunnelMainConclusionText applies soft repairs without replacing AI content.
func NormalizeFunnelMainConclusionText(text string) (normalized string, repairReason string) {
	out := strings.TrimSpace(text)
	if out == "" {
		return "", ""
	}
	if funnelDeltaNegativePPRE.MatchString(out) {
		out = funnelDeltaNegativePPRE.ReplaceAllStringFunc(out, func(match string) string {
			sub := funnelDeltaNegativePPRE.FindStringSubmatch(match)
			if len(sub) < 3 {
				return match
			}
			return fmt.Sprintf("снизилась на %s п.п.", strings.ReplaceAll(sub[2], ".", ","))
		})
		if repairReason == "" {
			repairReason = "delta_pp_normalized"
		}
	}
	out = strings.ReplaceAll(out, "  ", " ")
	return strings.TrimSpace(out), repairReason
}

// ResolveFunnelMainConclusion picks the seller-facing main conclusion with AI-first priority.
func ResolveFunnelMainConclusion(f *FunnelAIAnalysis) FunnelMainConclusionResolve {
	res := FunnelMainConclusionResolve{Source: "empty", Quality: "weak"}
	if f == nil {
		res.FallbackReason = "nil_funnel"
		return res
	}

	raw := strings.TrimSpace(f.MainConclusion)
	source := "ai"
	if raw == "" && f.WeakestStage != nil {
		raw = strings.TrimSpace(f.WeakestStage.Explanation)
		if raw != "" {
			source = "promoted_weakest"
		}
	}
	if raw == "" && f.ComparisonsCommentary != nil {
		raw = strings.TrimSpace(f.ComparisonsCommentary.PreviousPeriod)
		if raw != "" {
			source = "promoted_commentary"
		}
	}
	if raw == "" {
		if built := BuildFunnelMainConclusionFallback(f); built != "" {
			res.Text = built
			res.Source = "fallback"
			res.Quality = "ok"
			res.FallbackReason = "empty_ai_text"
			return res
		}
		res.FallbackReason = "empty_no_fallback_data"
		return res
	}

	normalized, repairReason := NormalizeFunnelMainConclusionText(raw)
	if normalized != "" {
		raw = normalized
	}
	if repairReason != "" {
		source = "ai_repaired"
		res.RepairReason = repairReason
		res.Quality = "repaired"
	}

	if !FunnelMainConclusionQualityWeak(raw) {
		res.Text = raw
		res.Source = source
		if res.Quality == "" {
			res.Quality = "ok"
		}
		return res
	}

	if built := BuildFunnelMainConclusionFallback(f); built != "" {
		res.Text = built
		res.Source = "fallback"
		res.Quality = "ok"
		res.FallbackReason = "weak_ai_text"
		return res
	}

	res.Text = raw
	res.Source = source
	res.Quality = "weak"
	res.FallbackReason = "weak_no_fallback_data"
	return res
}

func repairFunnelMainConclusion(funnel *FunnelAIAnalysis) SummaryRepairResult {
	result := SummaryRepairResult{}
	if funnel == nil || funnel.Status != "available" {
		return result
	}

	resolved := ResolveFunnelMainConclusion(funnel)
	if resolved.Text != "" {
		changed := strings.TrimSpace(funnel.MainConclusion) != resolved.Text
		funnel.MainConclusion = resolved.Text
		if changed {
			result.Repaired = true
		}
	}

	switch resolved.Source {
	case "ai_repaired":
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_main_conclusion_ai_repaired:"+resolved.RepairReason)
	case "fallback":
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_main_conclusion_fallback:"+resolved.FallbackReason)
	case "promoted_weakest":
		if strings.TrimSpace(funnel.MainConclusion) != "" {
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "funnel_main_conclusion_promoted_weakest")
		}
	case "promoted_commentary":
		if strings.TrimSpace(funnel.MainConclusion) != "" {
			result.Repaired = true
			result.RepairActions = append(result.RepairActions, "funnel_main_conclusion_promoted_commentary")
		}
	}

	if resolved.Quality == "weak" && resolved.Source != "fallback" {
		result.Partial = true
		result.RepairableErrors = append(result.RepairableErrors, "funnel_main_conclusion_weak_kept_ai")
	}

	result.RepairActions = append(result.RepairActions,
		"funnel_main_conclusion_source:"+resolved.Source,
		"funnel_main_conclusion_quality:"+resolved.Quality,
	)

	if funnel.WeakestStage != nil && strings.TrimSpace(funnel.WeakestStage.Explanation) == "" && strings.TrimSpace(funnel.MainConclusion) != "" {
		funnel.WeakestStage.Explanation = truncateFunnelSentences(funnel.MainConclusion, 2)
		result.Repaired = true
		result.RepairActions = append(result.RepairActions, "funnel_weakest_explanation_from_main_conclusion")
	}

	return result
}

// BuildFunnelMainConclusionFallback assembles a deterministic seller-facing conclusion
// from structured funnel metrics when AI text is missing or too generic.
func BuildFunnelMainConclusionFallback(f *FunnelAIAnalysis) string {
	if f == nil {
		return ""
	}
	ws := f.WeakestStage
	label := ""
	key := ""
	if ws != nil {
		label = strings.TrimSpace(ws.Label)
		key = strings.TrimSpace(ws.Key)
	}
	if label == "" {
		label = funnelWeakestLabelFromComparisons(f)
	}
	if label == "" {
		return ""
	}

	stageLines := collectFunnelStageDeltaLines(f)
	convLine := collectFunnelConversionLine(f, key, label, ws)
	lowerNote := collectLowerStagesNote(f)
	impact := funnelImpactSentence(key, label)
	actions := funnelActionsForWeakestKey(key, label)

	parts := make([]string, 0, 6)
	if len(stageLines) > 0 {
		parts = append(parts, fmt.Sprintf("Основная просадка находится в зоне «%s»: %s.", label, strings.Join(stageLines, ", ")))
	} else {
		parts = append(parts, fmt.Sprintf("Основная просадка находится на этапе «%s».", label))
	}
	if convLine != "" {
		parts = append(parts, convLine)
	}
	if lowerNote != "" {
		parts = append(parts, lowerNote)
	}
	if impact != "" {
		parts = append(parts, impact)
	}
	if actions != "" {
		parts = append(parts, actions)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func funnelWeakestLabelFromComparisons(f *FunnelAIAnalysis) string {
	if f == nil || f.Comparisons == nil {
		return ""
	}
	worstKey := ""
	worstVal := 0.0
	found := false
	for _, bucket := range [][]FunnelComparisonItem{
		f.Comparisons.ConversionMetrics,
		f.Comparisons.StageMetrics,
		f.Comparisons.PreviousPeriod,
	} {
		for _, it := range bucket {
			v, ok := parseSignedDeltaNumber(it.Delta)
			if !ok || v >= 0 {
				continue
			}
			if !found || v < worstVal {
				worstVal = v
				worstKey = strings.TrimSpace(it.Key)
				if l := strings.TrimSpace(it.Label); l != "" {
					return l
				}
				found = true
			}
		}
	}
	_ = worstKey
	return ""
}

func collectFunnelStageDeltaLines(f *FunnelAIAnalysis) []string {
	if f == nil || f.Comparisons == nil {
		return nil
	}
	lines := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, it := range append(append([]FunnelComparisonItem{}, f.Comparisons.StageMetrics...), f.Comparisons.PreviousPeriod...) {
		delta := strings.TrimSpace(it.Delta)
		label := strings.TrimSpace(it.Label)
		if delta == "" || label == "" {
			continue
		}
		if _, ok := parseSignedDeltaNumber(delta); !ok {
			continue
		}
		line := fmt.Sprintf("%s — %s", label, cleanFunnelDeltaForText(delta))
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
		if len(lines) >= 2 {
			break
		}
	}
	return lines
}

func collectFunnelConversionLine(f *FunnelAIAnalysis, key, label string, ws *FunnelWeakestStage) string {
	value := ""
	delta := ""
	if ws != nil {
		value = strings.TrimSpace(ws.Value)
		delta = cleanFunnelDeltaForText(strings.TrimSpace(ws.DeltaPrevious))
	}
	if f != nil && f.Comparisons != nil {
		for _, it := range f.Comparisons.ConversionMetrics {
			if key != "" && !strings.EqualFold(strings.TrimSpace(it.Key), key) {
				continue
			}
			if delta == "" {
				delta = cleanFunnelDeltaForText(strings.TrimSpace(it.Delta))
			}
			if value == "" {
				value = strings.TrimSpace(it.Value)
			}
			break
		}
	}
	if label == "" {
		return ""
	}
	if value != "" && delta != "" {
		return fmt.Sprintf("Ключевое слабое место — переход «%s»: конверсия составила %s и изменилась на %s к прошлой сводке.", label, value, delta)
	}
	if delta != "" {
		return fmt.Sprintf("Ключевое слабое место — переход «%s»: изменение конверсии %s к прошлой сводке.", label, delta)
	}
	return fmt.Sprintf("Ключевое слабое место — переход «%s».", label)
}

func collectLowerStagesNote(f *FunnelAIAnalysis) string {
	if f == nil || f.Comparisons == nil {
		return "Нижние этапы могут частично компенсировать спад, но на верхних этапах уже меньше показов и открытий карточек."
	}
	lowerWorse := false
	lowerBetter := false
	for _, it := range f.Comparisons.ConversionMetrics {
		region := funnelRegionFromComparisonKey(it.Key)
		if region != "lower" && region != "delivery" {
			continue
		}
		v, ok := parseSignedDeltaNumber(it.Delta)
		if !ok {
			continue
		}
		if v < 0 {
			lowerWorse = true
		}
		if v > 0 {
			lowerBetter = true
		}
	}
	if lowerWorse {
		return ""
	}
	if lowerBetter {
		return "Нижние этапы не выглядят главной причиной проблемы: переходы ниже по воронке не ухудшились и частично компенсируют спад. Но этой компенсации недостаточно, потому что на входе в середину и низ воронки приходит меньше покупателей."
	}
	return "Нижние этапы не выглядят главной причиной проблемы, но на верхних этапах уже меньше показов и открытий карточек, поэтому на выходе может быть меньше заказов."
}

func funnelImpactSentence(key, label string) string {
	region := funnelRegionFromComparisonKey(key)
	switch region {
	case "upper":
		return "Из-за этого до карточек и следующих этапов доходит меньше покупателей, что может ограничивать поток в корзину и заказы."
	case "middle":
		return "Из-за этого меньше покупателей переходит к выбору и оформлению, что может давить на количество заказов на выходе."
	case "lower":
		return "Из-за этого часть спроса не доходит до заказа, даже если верх воронки держится лучше."
	case "delivery":
		return "Из-за этого заказы хуже доходят до доставки, что может снижать итоговый результат продаж."
	default:
		_ = label
		return "Это может ограничивать поток покупателей в нижние этапы и снижать потенциал заказов на выходе."
	}
}

func funnelActionsForWeakestKey(key, label string) string {
	region := funnelRegionFromComparisonKey(key)
	switch region {
	case "upper":
		return "Что сделать: найти карточки и категории, где просмотры упали сильнее показов; разобрать рекламные кампании, которые дают показы без переходов в карточку; обновить главное фото, цену, название и ключевые преимущества у SKU с наибольшей просадкой просмотров."
	case "middle":
		return "Что сделать: проверить контент карточек с высоким трафиком и низким добавлением в корзину; сравнить цену, скидку и условия покупки с конкурентами; проверить наличие и остатки у SKU, которые часто смотрят, но редко кладут в корзину."
	case "lower":
		return "Что сделать: разобрать товары, которые часто добавляют в корзину, но не покупают; проверить сроки и стоимость доставки; сверить остатки и доступность по проблемным SKU."
	case "delivery":
		return "Что сделать: проверить отмены и возвраты по SKU с высоким числом заказов; разобрать задержки и проблемы исполнения; сверить доступность товара и логистику по проблемным позициям."
	default:
		_ = label
		return "Что сделать: сопоставить просадку с карточками, рекламой и остатками по SKU с наибольшим отклонением; обновить слабые элементы карточки; проверить условия покупки на проблемном этапе."
	}
}

func funnelRegionFromComparisonKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.Contains(k, "impressions_to"), strings.Contains(k, "impressions"), strings.Contains(k, "product_views") && strings.Contains(k, "impressions"):
		return "upper"
	case strings.Contains(k, "product_views_to"), strings.Contains(k, "views_to_cart"), strings.Contains(k, "add_to_cart") && strings.Contains(k, "product_views"):
		return "middle"
	case strings.Contains(k, "add_to_cart_to"), strings.Contains(k, "cart_to_order"), strings.Contains(k, "orders") && strings.Contains(k, "add_to_cart"):
		return "lower"
	case strings.Contains(k, "delivered"), strings.Contains(k, "orders_to_delivered"):
		return "delivery"
	default:
		if strings.Contains(k, "impressions") {
			return "upper"
		}
		if strings.Contains(k, "cart") || strings.Contains(k, "add_to_cart") {
			return "middle"
		}
		if strings.Contains(k, "order") {
			return "lower"
		}
		return ""
	}
}

func parseSignedDeltaNumber(delta string) (float64, bool) {
	s := strings.TrimSpace(delta)
	if s == "" {
		return 0, false
	}
	s = strings.ReplaceAll(s, "−", "-")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, "п.п.", "")
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"к прошлой сводке", "к прошлому дню", "к среднему за 7 дней", "к вчера"} {
		if idx := strings.Index(strings.ToLower(s), prefix); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	var n float64
	if _, err := fmt.Sscanf(s, "%f", &n); err != nil {
		return 0, false
	}
	return n, true
}

func cleanFunnelDeltaForText(delta string) string {
	s := strings.TrimSpace(delta)
	repl := []string{
		" к прошлой сводке", " к прошлому дню", " к среднему за 7 дней", " к вчера",
	}
	lower := strings.ToLower(s)
	for _, r := range repl {
		if idx := strings.Index(lower, r); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
			lower = strings.ToLower(s)
		}
	}
	return s
}

func truncateFunnelSentences(text string, max int) string {
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
