package dashboardhome

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reSevenDayRU     = regexp.MustCompile(`(?is)(к\s+)?средн(ему|его|яя|ий)\s+за\s+7\s*дн(ей|я)?`)
	reSevenAlt       = regexp.MustCompile(`(?is)за\s+последн(ие|их)\s+7\s*дн(ей|я)?`)
	reSevenParen     = regexp.MustCompile(`(?is)\(\s*7\s*дн(ей|я)?\s*\)`)
	reWeekRU         = regexp.MustCompile(`(?is)недел(и|ю|е|я)\s+к\s+средн`)
	reSevenBare      = regexp.MustCompile(`(?is)за\s+7\s*дн(ей|я)?`)
	reCausalPriveloK = regexp.MustCompile(`(?i)привел[ао]?\s+к`)
	reCausalVyzval   = regexp.MustCompile(`(?i)вызвал[ао]?\s+`)
	reCausalIzZa     = regexp.MustCompile(`(?i)из-?за`)
)

// limitedHistoryNote is shown as a separate InfoNote (context signal), never inline in the commentary.
const limitedHistoryNote = "Сравнение ограничено: доступно меньше 7 дней истории."

var (
	reLimitedHistoryNote = regexp.MustCompile(`(?is)Сравнение\s+ограничено:?\s*доступно\s+меньше\s*7\s*дней\s+истории\.?`)
	reSevenDayAvgPhrase  = regexp.MustCompile(`(?i)средн(ему|его|ей|яя|ий)\s+за\s+7\s*дн(?:ей|я)?`)
	reCommentaryDelta    = regexp.MustCompile(`(?i)[-+−]?\d+(?:[.,]\d+)?\s*(?:%|п\.п\.)`)

	// reCommentaryMovement matches "что изменилось" wording (direction of change).
	reCommentaryMovement = regexp.MustCompile(`(?i)просел|просад|ухудш|снизил|снижен|упал|пада|сократ|вырос|подрос|подня|стал[аои]?\s+(?:сильнее|слабее|выше|ниже)|выше|ниже|держ|улучш|растёт|растет`)
	// reCommentaryInterpretive matches "что это значит" wording (consequence /
	// causation / localization verdict). Mere region mentions ("верх воронки") do
	// NOT count — the text must explain a consequence, not just name a place.
	reCommentaryInterpretive = regexp.MustCompile(`(?i)реже|чаще|компенс|за\s+счёт|за\s+счет|поэтому|значит|означает|слабее\s+работает|база|базы|интерес|спрос|выбор|доходит|барьер|тер[яюя]|уход|выполн|локальн|цепочк|по\s+всей`)
)

// stripSevenDayMentionsIfNoData removes 7-day comparison language when we have no
// seven_day_average rows (AI-first: do not let text contradict empty structured data).
func stripSevenDayMentionsIfNoData(text string, hasSevenDay bool) string {
	if hasSevenDay {
		return strings.TrimSpace(text)
	}
	t := text
	for _, re := range []*regexp.Regexp{reSevenDayRU, reSevenAlt, reSevenParen, reWeekRU, reSevenBare} {
		t = re.ReplaceAllString(t, " ")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(t), " "))
}

// rephraseLimitedHistoryWording replaces "за/к среднему за 7 дней" wording with
// "доступное среднее за период" / "по доступной истории" when fewer than 7 days
// of history are available but seven-day rows still exist.
func rephraseLimitedHistoryWording(text string) string {
	t := reSevenDayAvgPhrase.ReplaceAllStringFunc(text, func(m string) string {
		switch lm := strings.ToLower(m); {
		case strings.HasPrefix(lm, "среднему"):
			return "доступному среднему за период"
		case strings.HasPrefix(lm, "среднего"):
			return "доступного среднего за период"
		case strings.HasPrefix(lm, "средней"):
			return "доступной средней за период"
		case strings.HasPrefix(lm, "средняя"):
			return "доступная средняя за период"
		case strings.HasPrefix(lm, "средний"):
			return "доступный средний за период"
		default:
			return "доступному среднему за период"
		}
	})
	t = reSevenAlt.ReplaceAllString(t, "по доступной истории")
	t = reSevenBare.ReplaceAllString(t, "по доступной истории")
	t = reSevenParen.ReplaceAllString(t, " ")
	return strings.Join(strings.Fields(t), " ")
}

// sanitizeComparisonCommentary keeps AI text as primary but applies guardrails:
// strips the limited-history note (shown separately), removes legacy terms,
// softens causal claims, drops 7-day language when no rows, and rephrases 7-day
// wording when history is limited.
func sanitizeComparisonCommentary(text string, hasSevenDay, limitedHistory bool) string {
	t := reLimitedHistoryNote.ReplaceAllString(text, " ")
	t = stripSevenDayMentionsIfNoData(t, hasSevenDay)
	if limitedHistory {
		t = rephraseLimitedHistoryWording(t)
	}
	t = replaceLegacyFunnelTerms(t)
	t = softenCausalClaims(t)
	return strings.TrimSpace(strings.Join(strings.Fields(t), " "))
}

// commentaryNeedsFallback is true when the AI commentary is empty, is mostly a
// dump of delta numbers, or is overly generic (states "what moved" without
// interpreting "what it means"). AI stays the primary source; this only kicks in
// for missing / number-dump / non-interpretive text.
func commentaryNeedsFallback(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if commentaryIsNumberDump(t) {
		return true
	}
	return commentaryIsOverlyGeneric(t)
}

func commentaryIsNumberDump(t string) bool {
	if len(reCommentaryDelta.FindAllString(t, -1)) < 3 {
		return false
	}
	stripped := strings.ToLower(reCommentaryDelta.ReplaceAllString(t, " "))
	analytical := []string{
		"просад", "слабе", "середин", "верх", "низ", "перех", "конверси", "компенс",
		"локальн", "цепочк", "значит", "поэтому", "хуже", "сильнее", "норм", "охват",
		"доставк", "выполнен", "база", "интерес", "выбор",
	}
	for _, w := range analytical {
		if strings.Contains(stripped, w) {
			return false
		}
	}
	return true
}

// commentaryIsOverlyGeneric is true when the text describes the direction of
// change (просел / стал сильнее / выше / ниже) but offers no interpretation,
// causation or localization verdict. Such "обзорный" text is replaced by the
// deterministic analytical fallback so the block answers "что это значит".
func commentaryIsOverlyGeneric(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if !reCommentaryMovement.MatchString(t) {
		return false
	}
	return !reCommentaryInterpretive.MatchString(t)
}

// ensureLimitedHistorySignal guarantees the limited-history note exists as a
// context signal so the UI can render it as a separate InfoNote.
func ensureLimitedHistorySignal(signals []FunnelComparisonItem) []FunnelComparisonItem {
	for i := range signals {
		if signals[i].Key == "history_status" {
			if strings.TrimSpace(signals[i].Value) == "" {
				signals[i].Value = limitedHistoryNote
			}
			signals[i].Tone = "warning"
			if strings.TrimSpace(signals[i].Delta) == "" {
				signals[i].Delta = "limited"
			}
			return signals
		}
	}
	return append(signals, FunnelComparisonItem{
		Key:   "history_status",
		Label: "История сравнения",
		Value: limitedHistoryNote,
		Delta: "limited",
		Tone:  "warning",
	})
}

func factorRequiresAdvertisingEvidence(it FunnelTextItem) bool {
	k := strings.ToLower(it.Key)
	blob := strings.ToLower(it.Label + " " + it.Text)
	if strings.Contains(k, "ad") || strings.Contains(k, "реклам") || strings.Contains(k, "campaign") {
		return true
	}
	if strings.Contains(blob, "реклам") || strings.Contains(blob, "дрр") || strings.Contains(blob, "кампани") {
		return true
	}
	return false
}

func factorRequiresStockEvidence(it FunnelTextItem) bool {
	k := strings.ToLower(it.Key)
	blob := strings.ToLower(it.Label + " " + it.Text)
	if strings.Contains(k, "stock") || strings.Contains(k, "остат") || strings.Contains(k, "склад") || strings.Contains(k, "покрыти") {
		return true
	}
	if strings.Contains(blob, "остат") || strings.Contains(blob, "склад") || strings.Contains(blob, "покрыти") {
		return true
	}
	return false
}

func factorRequiresCancellationEvidence(it FunnelTextItem) bool {
	k := strings.ToLower(it.Key)
	blob := strings.ToLower(it.Label + " " + it.Text)
	if strings.Contains(k, "cancel") || strings.Contains(k, "отмен") {
		return true
	}
	if strings.Contains(blob, "отмен") || strings.Contains(blob, "возврат") {
		return true
	}
	return false
}

func filterInfluenceFactorsByEvidence(items []FunnelTextItem, ev FunnelEvidenceSnapshot) []FunnelTextItem {
	if len(items) == 0 {
		return items
	}
	out := make([]FunnelTextItem, 0, len(items))
	for _, it := range items {
		if factorRequiresAdvertisingEvidence(it) && !ev.HasAdvertising {
			continue
		}
		if factorRequiresStockEvidence(it) && !ev.HasStock {
			continue
		}
		if factorRequiresCancellationEvidence(it) && !ev.HasCancellations {
			continue
		}
		out = append(out, it)
	}
	return out
}

var imperativePrefixes = []string{
	"срочно", "остановите", "остановить", "сократите", "сократить",
	"пополните", "пополнить", "уберите", "убрать", "увеличьте", "уменьшите",
	"снизьте", "повысьте", "отключите", "включите", "проверьте", "проверить",
	"сверьте", "сверить", "разберите", "разобрать",
	"оцените", "сравните", "изучите",
}

func sanitizeCheckDirections(items []FunnelTextItem) []FunnelTextItem {
	if len(items) == 0 {
		return items
	}
	out := make([]FunnelTextItem, 0, len(items))
	for _, it := range items {
		label := strings.TrimSpace(it.Label)
		text := strings.TrimSpace(it.Text)
		if strings.HasPrefix(strings.ToLower(label), "направление ") {
			label = compactDirectionLabel(text)
		}
		label = stripImperativeOpeners(label)
		text = stripImperativeOpeners(text)
		text = softenUnsupportedSKUFunnelClaims(text)
		if text == "" {
			continue
		}
		if label == "" {
			label = compactDirectionLabel(text)
		}
		out = append(out, FunnelTextItem{
			Key: it.Key, Label: label, Priority: it.Priority, DataNeeded: it.DataNeeded, Reason: it.Reason, Text: text, Tone: it.Tone,
		})
	}
	return out
}

func stripPrefixFold(s, prefix string) (rest string, ok bool) {
	s = strings.TrimSpace(s)
	pr := []rune(prefix)
	sr := []rune(s)
	if len(sr) < len(pr) {
		return s, false
	}
	for i := range pr {
		if unicode.ToLower(sr[i]) != pr[i] {
			return s, false
		}
	}
	return string(sr[len(pr):]), true
}

func stripImperativeOpeners(s string) string {
	s = strings.TrimSpace(s)
	for changed := true; changed && s != ""; {
		changed = false
		lower := strings.ToLower(s)
		for _, p := range imperativePrefixes {
			if strings.HasPrefix(lower, p+" ") || strings.HasPrefix(lower, p+",") || strings.HasPrefix(lower, p+":") {
				rest, ok := stripPrefixFold(s, p)
				if !ok {
					continue
				}
				s = strings.TrimSpace(rest)
				s = strings.TrimLeftFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == ':' || r == '.' })
				changed = true
				break
			}
		}
	}
	return strings.TrimSpace(s)
}

func compactDirectionLabel(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Направление анализа"
	}
	runes := []rune(text)
	const max = 52
	if len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}

// compactFactorChipLabel returns a short chip label; full sentence stays in Text.
func compactFactorChipLabel(it FunnelTextItem) string {
	if lab := strings.TrimSpace(it.Label); lab != "" && utf8.RuneCountInString(lab) <= 36 {
		return lab
	}
	src := strings.TrimSpace(it.Text)
	if src == "" {
		return "Фактор"
	}
	return heuristicFactorShortLabel(src)
}

func heuristicFactorShortLabel(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "реклам") || strings.Contains(lower, "дрр"):
		return "Реклама"
	case strings.Contains(lower, "остат") || strings.Contains(lower, "склад"):
		return "Остатки"
	case strings.Contains(lower, "отмен") || strings.Contains(lower, "возврат"):
		return "Отмены"
	case strings.Contains(lower, "чек") || strings.Contains(lower, "aov"):
		return "Средний чек"
	case strings.Contains(lower, "цен"):
		return "Цены"
	default:
		return wordsChipLabel(s, 5, 30)
	}
}

// wordsChipLabel builds a short chip title from the first words (no mid-word ellipsis).
func wordsChipLabel(s string, maxWords, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Фактор"
	}
	if idx := strings.IndexAny(s, "\n.;"); idx > 0 && idx < len(s) {
		clause := strings.TrimSpace(s[:idx])
		if clause != "" {
			s = clause
		}
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "Фактор"
	}
	if len(fields) > maxWords {
		fields = fields[:maxWords]
	}
	out := strings.Join(fields, " ")
	r := []rune(out)
	if len(r) > maxRunes {
		out = strings.TrimSpace(string(r[:maxRunes]))
	}
	return out
}

func applyCompactFactorLabels(items []FunnelTextItem) []FunnelTextItem {
	out := make([]FunnelTextItem, len(items))
	for i := range items {
		out[i] = items[i]
		out[i].Label = compactFactorChipLabel(items[i])
	}
	return out
}

func normalizeDetailsSectionsOrder(sections []FunnelDetailsSection) []FunnelDetailsSection {
	if len(sections) == 0 {
		return sections
	}
	type sectionKey string
	const (
		supporting sectionKey = "supporting"
		mitigating sectionKey = "mitigating"
		related    sectionKey = "related"
		limits     sectionKey = "limits"
		conclusion sectionKey = "conclusion"
		other      sectionKey = "other"
	)
	classify := func(title string) sectionKey {
		t := strings.ToLower(strings.TrimSpace(title))
		switch {
		case strings.Contains(t, "подтверж"), strings.Contains(t, "support"), strings.Contains(t, "evidence"), strings.Contains(t, "proof"), strings.Contains(t, "доказ"):
			return supporting
		case strings.Contains(t, "смягча"), strings.Contains(t, "mitigat"), strings.Contains(t, "counter"), strings.Contains(t, "uncertain"), strings.Contains(t, "неопредел"), strings.Contains(t, "противореч"):
			return mitigating
		case strings.Contains(t, "связан"), strings.Contains(t, "related"), strings.Contains(t, "signal"), strings.Contains(t, "context"), strings.Contains(t, "сигнал"), strings.Contains(t, "контекст"):
			return related
		case strings.Contains(t, "огранич"), strings.Contains(t, "limit"), strings.Contains(t, "assumption"), strings.Contains(t, "допущен"):
			return limits
		case strings.Contains(t, "итог"), strings.Contains(t, "вывод"), strings.Contains(t, "заключ"), strings.Contains(t, "conclusion"), strings.Contains(t, "summary"), strings.Contains(t, "result"), strings.Contains(t, "final"):
			return conclusion
		default:
			return other
		}
	}
	rename := func(k sectionKey) string {
		switch k {
		case supporting:
			return "Что подтверждает вывод"
		case mitigating:
			return "Что смягчает вывод"
		case related:
			return "Связанные сигналы"
		case limits:
			return "Ограничения анализа"
		case conclusion:
			return "Итог"
		default:
			return "Связанные сигналы"
		}
	}
	grouped := map[sectionKey]FunnelDetailsSection{}
	appendOrMerge := func(k sectionKey, sec FunnelDetailsSection) {
		cur, ok := grouped[k]
		if !ok {
			sec.Title = rename(k)
			sec.Items = dedupeSectionItems(sec.Items)
			grouped[k] = sec
			return
		}
		cur.Items = dedupeSectionItems(append(cur.Items, sec.Items...))
		grouped[k] = cur
	}
	for _, sec := range sections {
		appendOrMerge(classify(sec.Title), sec)
	}
	order := []sectionKey{supporting, mitigating, related, limits, conclusion}
	out := make([]FunnelDetailsSection, 0, len(order))
	for _, key := range order {
		if sec, ok := grouped[key]; ok && len(sec.Items) > 0 {
			out = append(out, sec)
		}
	}
	if len(out) == 0 {
		return sections
	}
	return out
}

func dedupeSectionItems(items []FunnelDetailsSectionItem) []FunnelDetailsSectionItem {
	out := make([]FunnelDetailsSectionItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Text))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func softenCausalClaims(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	replaced := reCausalPriveloK.ReplaceAllString(text, "может быть связано с ")
	replaced = reCausalVyzval.ReplaceAllString(replaced, "может быть связано с ")
	replaced = reCausalIzZa.ReplaceAllString(replaced, "на фоне")
	return strings.Join(strings.Fields(replaced), " ")
}

func softenUnsupportedSKUFunnelClaims(text string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return ""
	}
	hasSKURef := strings.Contains(t, "sku") ||
		strings.Contains(t, "товар") ||
		strings.Contains(t, "offer") ||
		strings.Contains(t, "карточ")
	hasView := strings.Contains(t, "просмотр") || strings.Contains(t, "карточ")
	hasCart := strings.Contains(t, "добавлен") || strings.Contains(t, "корзин")
	if hasSKURef && hasView && hasCart {
		return "Если SKU-детализация воронки доступна, проверьте этап просмотры → добавления. В MVP начните с товаров с падением заказов и значимой долей выручки."
	}
	return text
}

func classifyComparisonItems(items []FunnelComparisonItem) (stage []FunnelComparisonItem, conversion []FunnelComparisonItem, business []FunnelComparisonItem, context []FunnelComparisonItem) {
	stage = make([]FunnelComparisonItem, 0, len(items))
	conversion = make([]FunnelComparisonItem, 0, len(items))
	business = make([]FunnelComparisonItem, 0, len(items))
	context = make([]FunnelComparisonItem, 0, len(items))
	for _, item := range items {
		switch classifyComparisonKey(item.Key, item.Label) {
		case "stage":
			stage = append(stage, item)
		case "conversion":
			conversion = append(conversion, item)
		case "business":
			business = append(business, item)
		default:
			context = append(context, item)
		}
	}
	return stage, conversion, business, context
}

func reorderComparisonItems(items []FunnelComparisonItem) []FunnelComparisonItem {
	stage, conversion, business, context := classifyComparisonItems(items)
	out := make([]FunnelComparisonItem, 0, len(items))
	out = append(out, stage...)
	out = append(out, conversion...)
	out = append(out, business...)
	out = append(out, context...)
	return out
}

func classifyComparisonKey(key, label string) string {
	k := strings.ToLower(strings.TrimSpace(key + " " + label))
	switch {
	case strings.Contains(k, "impressions"),
		strings.Contains(k, "product_views"),
		strings.Contains(k, "add_to_cart"),
		strings.Contains(k, "orders"),
		strings.Contains(k, "delivered_units"),
		strings.Contains(k, "показы"),
		strings.Contains(k, "карточки"),
		strings.Contains(k, "корзину"),
		strings.Contains(k, "заказанные"),
		strings.Contains(k, "доставленные"):
		return "stage"
	case strings.Contains(k, "_to_"),
		strings.Contains(k, "→"),
		strings.Contains(k, "конверси"):
		return "conversion"
	case strings.Contains(k, "revenue"),
		strings.Contains(k, "выручк"),
		strings.Contains(k, "average_order_value"),
		strings.Contains(k, "aov"),
		strings.Contains(k, "средний чек"):
		return "business"
	default:
		return "context"
	}
}
