package aisummaries

import (
	"strings"
	"testing"
)

func TestSystemPromptDailySummary_mentionsImprovedRules(t *testing.T) {
	prompt := systemPromptDailySummary()
	for _, want := range []string{
		"Если измеримых улучшений нет — верни improved=[]",
		"Не выдумывай позитивные изменения",
		"Для what_changed_since_previous_summary используй только изменения, подтверждённые данными",
		"должны содержать только plain strings",
		"Не возвращай объекты вида {risk, reason}",
		"Все календарные даты",
		"ДД.ММ.ГГГГ",
		"в ₽",
		"week-to-week, dashboard, account_metrics_last_7d",
		"вместо week-to-week пиши «по сравнению с прошлой неделей»",
		"«заказы по кабинету»",
		"суммы округляй до рублей",
		"«предыдущих 7 дней по сравнению с прошлой неделей»",
		"«ниже уровня прошлой недели»",
		"Для funnel_ai_analysis не ограничивайся крупнейшим текущим провалом воронки",
		"изменения этапов ко вчера",
		"средним уровнем за последние 7 дней",
		"stage_metrics, conversion_metrics, business_metrics, context_signals",
		"связанные сигналы из рекламы, остатков, SKU, заказов, отмен, возвратов и рисков",
		"Не подгоняй факты под слабое место",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}

func TestUserPromptAccountSummary_mentionsImprovedRules(t *testing.T) {
	prompt := userPromptAccountSummary()
	for _, want := range []string{
		`"main_conclusion": "string"`,
		`"action_items": [`,
		"main_conclusion обязателен при status=available",
		"Что сделать:",
		"5–7 предложений",
		"Не возвращай drop_factors, check_directions, details_sections",
		"Primary seller-facing контент: main_conclusion",
		"comparisons_commentary.previous_period: короткий комментарий",
		"comparisons_commentary.seven_day_average: короткий комментарий",
		"доступное среднее за период",
		"не пиши «за последние 7 дней»",
		"служебную пометку об ограничении истории",
		`"explanation":"string"`,
		"improved: перечисляй только конкретные измеримые улучшения",
		"Если видимых улучшений нет — верни []",
		"Не выдумывай позитивные изменения ради заполнения блока",
		"должны содержать только plain strings",
		"Не возвращай объекты внутри этих массивов",
		"Не используй internal field names",
		"Даты показывай в формате ДД.ММ.ГГГГ",
		"Деньги и метрики всегда сопровождай единицами измерения",
		"Не пиши «заказы по кабинету»",
		"суммы до рублей",
		"по сравнению с прошлой неделей",
		"ниже уровня прошлой недели",
		"structured mini-report",
		"В MVP воронка доступна на уровне кабинета",
		"stage_metrics",
		"conversion_metrics",
		"business_metrics",
		"context_signals",
		"Показы → Просмотры карточки: карточки/категории с просадкой просмотров",
		"Запрещены generic-действия",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("account prompt missing %q", want)
		}
	}
}

func TestUserPromptCategorySummary_mentionsSellerFacingStyle(t *testing.T) {
	prompt := strings.ToLower(userPromptCategorySummary())
	for _, want := range []string{
		"без англицизмов и внутренних названий полей/таблиц",
		"вместо week-to-week пиши «по сравнению с прошлой неделей»",
		"вместо dashboard — «кабинет»",
		"все даты показывай в формате дд.мм.гггг",
		"деньги и метрики всегда сопровождай единицами измерения",
		"не пиши «заказы по кабинету»",
		"округляй денежные суммы до рублей",
		"ниже уровня прошлой недели",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("category prompt missing %q", want)
		}
	}
}

func TestUserPromptCategorySummary_contract(t *testing.T) {
	p := userPromptCategorySummary()
	if !strings.Contains(p, "low|medium|high|critical") {
		t.Fatal("missing severity enum")
	}
	if !strings.Contains(p, `impact":"low|medium|high"`) {
		t.Fatal("missing impact enum")
	}
	if !strings.Contains(p, "evidence") || !strings.Contains(p, "СТРОКА") {
		t.Fatal("missing evidence string rule")
	}
	if !strings.Contains(p, "category_id") || !strings.Contains(p, "category_name") {
		t.Fatal("missing category fields")
	}
	if !strings.Contains(p, "key_risk") || !strings.Contains(p, "recommended_action") {
		t.Fatal("missing category strict summary fields")
	}
	if !strings.Contains(p, "НЕ строка") || !strings.Contains(p, "массив ОБЪЕКТОВ") {
		t.Fatal("missing watchlist object array rule")
	}
	if !strings.Contains(p, "plain strings") || !strings.Contains(p, "Не возвращай объекты внутри этих массивов") {
		t.Fatal("missing plain string arrays rule")
	}
}

func TestCategorySummaryUserPrompt_runConstraints(t *testing.T) {
	p := categorySummaryUserPrompt(42, "Текстиль для дома", "high")
	if !strings.Contains(p, "category_id MUST be 42") {
		t.Fatal(p)
	}
	if !strings.Contains(p, "Текстиль для дома") {
		t.Fatal(p)
	}
	if !strings.Contains(p, "recommended_actions MUST have at least 1") {
		t.Fatal(p)
	}
	low := categorySummaryUserPrompt(1, "Уборка", "low")
	if !strings.Contains(low, "data_richness=\"low\"") {
		t.Fatal(low)
	}
}
