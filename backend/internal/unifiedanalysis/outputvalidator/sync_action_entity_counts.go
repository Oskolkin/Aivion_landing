package outputvalidator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// SyncActionEntityCountResult summarizes deterministic count repairs applied
// after SanitizeGrounding so free-text titles/summaries cannot disagree with
// the validated scope.entity_ids list.
type SyncActionEntityCountResult struct {
	ActionsRewritten int
	TitlesSynced     int
	SummariesSynced  int
	ProblemsSynced   int
	GenericEffects   int
	PlansExpanded    int
	EmptyEntityScope int
}

var (
	// Matches a digit run before SKU / кампания / товар mentions, allowing a
	// short adjective phrase between the number and the noun
	// (e.g. «4 ключевых SKU», «2 проблемные кампании»).
	entityCountMentionRE = regexp.MustCompile(`(?i)(\d+)(?:\s+[\p{L}-]+){0,3}\s*(SKU|sku|кампани[ияйю]|товар[аовыу]?|позици[яийю])`)
	genericExpectedRE    = regexp.MustCompile(`(?i)^(рост эффективности|улучшение показателей|увеличение продаж|снижение рисков|нормализация ситуации|оптимизация показателей)\.?$`)
)

// SyncActionEntityCounts rewrites free-text entity counts to match
// len(scope.entity_ids) after pruning/dedup, expands generic expected_effect,
// and ensures multi-entity stock/ads actions keep a usable step plan.
//
// Authoritative count is always len(validated entity_ids). AI-invented numbers
// in title/summary/problem are never trusted.
func SyncActionEntityCounts(out *contract.UnifiedAnalysisOutputV1) SyncActionEntityCountResult {
	var res SyncActionEntityCountResult
	if out == nil {
		return res
	}
	for i := range out.Actions {
		a := &out.Actions[i]
		scopeType := strings.ToLower(strings.TrimSpace(a.Scope.ScopeType))
		n := len(a.Scope.EntityIDs)

		if (scopeType == "sku" || scopeType == "campaign") && n == 0 {
			res.EmptyEntityScope++
			continue
		}

		changed := false
		if n > 0 {
			if synced, did := syncCountMentions(a.Title, n); did {
				a.Title = synced
				res.TitlesSynced++
				changed = true
			}
			if synced, did := syncCountMentions(a.Summary, n); did {
				a.Summary = synced
				res.SummariesSynced++
				changed = true
			}
			if synced, did := syncCountMentions(a.Problem, n); did {
				a.Problem = synced
				res.ProblemsSynced++
				changed = true
			}
			if synced, did := syncCountMentions(a.WhyNow, n); did {
				a.WhyNow = synced
				changed = true
			}
			if synced, did := syncCountMentions(a.ExpectedEffect, n); did {
				a.ExpectedEffect = synced
				changed = true
			}
		}

		if expandGenericExpectedEffect(a) {
			res.GenericEffects++
			changed = true
		}
		if ensureActionablePlan(a, n) {
			res.PlansExpanded++
			changed = true
		}
		if expandThinSituation(a, n) {
			changed = true
		}
		if changed {
			res.ActionsRewritten++
		}
	}
	return res
}

func syncCountMentions(text string, n int) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || n <= 0 {
		return text, false
	}
	changed := false
	out := entityCountMentionRE.ReplaceAllStringFunc(text, func(match string) string {
		sub := entityCountMentionRE.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		mentioned, err := strconv.Atoi(sub[1])
		if err != nil || mentioned == n {
			return match
		}
		changed = true
		// Keep any middle adjectives, only rewrite the leading digit.
		rest := strings.TrimPrefix(match, sub[1])
		return strconv.Itoa(n) + rest
	})
	return out, changed
}

func expandGenericExpectedEffect(a *contract.UnifiedActionV1) bool {
	if !IsGenericExpectedEffect(a.ExpectedEffect) {
		return false
	}
	n := len(a.Scope.EntityIDs)
	cat := strings.ToLower(strings.TrimSpace(a.Category))
	switch {
	case cat == "stock" || cat == "slow_turnover":
		if n > 0 {
			a.ExpectedEffect = fmt.Sprintf(
				"После пополнения %s снова станут доступны для заказа, а риск обнуления остатков снизится. Это поможет сохранить продажи по позициям с подтверждённым спросом; фактический объём зависит от срока приёмки и сохранения текущего спроса.",
				pluralSKUPhrase(n),
			)
		} else {
			a.ExpectedEffect = "После пополнения затронутые позиции снова будут доступны для заказа, а риск обнуления остатка снизится. Это поможет сохранить продажи по SKU с подтверждённым спросом; фактический объём зависит от срока приёмки и сохранения текущего спроса."
		}
	case cat == "advertising":
		a.ExpectedEffect = "После корректировки кампаний снизится неэффективный расход бюджета при сохранении контроля за заказами. Итоговый эффект зависит от фактической реакции ставок и спроса."
	default:
		a.ExpectedEffect = "После выполнения шагов снизится зафиксированный риск по затронутым позициям, а кабинет получит более управляемый операционный результат. Фактический эффект зависит от своевременности реакции."
	}
	return true
}

func ensureActionablePlan(a *contract.UnifiedActionV1, n int) bool {
	minSteps := 2
	if n >= 2 || strings.EqualFold(a.Category, "stock") || strings.EqualFold(a.Category, "advertising") {
		minSteps = 3
	}
	if len(a.Steps) >= minSteps && !hasVagueOnlySteps(a.Steps) {
		return false
	}
	cat := strings.ToLower(strings.TrimSpace(a.Category))
	entityIDs := append([]string{}, a.Scope.EntityIDs...)
	var steps []contract.UnifiedActionStepV1
	switch cat {
	case "stock", "slow_turnover":
		steps = []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить остатки и входящие поставки", Description: "Сверить доступный остаток, товары в пути, уже созданные поставки и ожидаемые даты приёмки по каждому SKU.", EntityIDs: entityIDs},
			{Order: 2, Title: "Рассчитать потребность", Description: "Определить объём поставки с учётом продаж за 7 и 30 дней, срока доставки и страхового запаса.", EntityIDs: entityIDs},
			{Order: 3, Title: "Расставить приоритеты", Description: "Сначала включить в поставку SKU с минимальным покрытием, подтверждённым спросом и высокой долей выручки."},
			{Order: 4, Title: "Создать или ускорить поставку", Description: "Добавить товары в ближайшую поставку либо ускорить уже созданную заявку."},
			{Order: 5, Title: "Проверить результат", Description: "После приёмки убедиться, что остаток обновился и товары снова доступны для заказа."},
		}
	case "advertising":
		steps = []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить проблемные кампании", Description: "Сверить расход, ДРР, заказы и связанные SKU по каждой затронутой кампании.", EntityIDs: entityIDs},
			{Order: 2, Title: "Скорректировать ставки и бюджет", Description: "Снизить неэффективный расход, сохранив контроль за кампаниями с заказами."},
			{Order: 3, Title: "Проверить результат за 7 дней", Description: "Убедиться, что ДРР и расход вернулись в допустимый диапазон без потери ключевых заказов."},
		}
	default:
		steps = []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить исходные показатели", Description: "Сверить факты, подтверждающие проблему, по каждой затронутой позиции.", EntityIDs: entityIDs},
			{Order: 2, Title: "Выполнить корректировку", Description: "Внести изменения, устраняющие причину отклонения."},
			{Order: 3, Title: "Проверить результат", Description: "Убедиться, что показатели вернулись в допустимый диапазон."},
		}
	}
	a.Steps = steps
	return true
}

func hasVagueOnlySteps(steps []contract.UnifiedActionStepV1) bool {
	if len(steps) == 0 {
		return true
	}
	vagueTitles := map[string]struct{}{
		"следующий шаг": {},
		"шаг":           {},
		"проверить":     {},
		"проанализировать ситуацию": {},
		"обратить внимание":         {},
	}
	for _, s := range steps {
		title := strings.ToLower(strings.TrimSpace(s.Title))
		desc := strings.TrimSpace(s.Description)
		if _, ok := vagueTitles[title]; ok || title == "" || desc == "" {
			return true
		}
	}
	return false
}

func expandThinSituation(a *contract.UnifiedActionV1, n int) bool {
	why := strings.TrimSpace(a.WhyNow)
	summary := strings.TrimSpace(a.Summary)
	problem := strings.TrimSpace(a.Problem)
	// Prefer WhyNow as situation body; if thin, compose from summary+problem.
	if utf8RuneCount(why) >= 80 && !nearlySameText(why, summary) && !nearlySameText(why, a.Title) {
		return false
	}
	parts := make([]string, 0, 3)
	if summary != "" {
		parts = append(parts, summary)
	}
	if problem != "" && !nearlySameText(problem, summary) {
		parts = append(parts, problem)
	}
	if why != "" && !nearlySameText(why, summary) && !nearlySameText(why, problem) {
		parts = append(parts, why)
	}
	if len(parts) == 0 {
		if n > 0 {
			a.WhyNow = fmt.Sprintf(
				"По %s зафиксировано отклонение, требующее операционной реакции. Ниже — конкретные шаги и критерии завершения.",
				pluralSKUPhrase(n),
			)
			return true
		}
		return false
	}
	joined := strings.Join(parts, " ")
	if utf8RuneCount(joined) < 40 {
		return false
	}
	a.WhyNow = joined
	return true
}

func nearlySameText(a, b string) bool {
	na := normalizeCompare(a)
	nb := normalizeCompare(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	return false
}

func normalizeCompare(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}

func pluralSKUPhrase(n int) string {
	return fmt.Sprintf("%d SKU", n)
}

// ExtractMentionedEntityCount returns the first entity-count mention in text, or -1.
func ExtractMentionedEntityCount(text string) int {
	sub := entityCountMentionRE.FindStringSubmatch(text)
	if len(sub) < 2 {
		return -1
	}
	n, err := strconv.Atoi(sub[1])
	if err != nil {
		return -1
	}
	return n
}

// IsGenericExpectedEffect reports one-word/vague expected-effect copy.
func IsGenericExpectedEffect(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if genericExpectedRE.MatchString(trimmed) {
		return true
	}
	return utf8RuneCount(trimmed) < 40
}
