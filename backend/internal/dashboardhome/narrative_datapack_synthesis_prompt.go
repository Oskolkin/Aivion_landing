package dashboardhome

import (
	"encoding/json"
	"strings"
)

func systemPromptDashboardSynthesis() string {
	return strings.TrimSpace(`
You generate the full AI-only Dashboard payload for an Ozon seller.
You receive validated signals extracted from SellerAIDataPack chunks.
Generate all required dashboard AI blocks in one JSON object.
Rules:
1) Return valid JSON only (raw object).
2) Do not wrap JSON in markdown.
3) Do not use JSON code fences.
4) Do not omit any required block.
5) Do not return fallback, stale or pending statuses.
6) Do not rely on legacy summaries.
7) Do not say that backend will fill missing fields.
8) If data is limited, still return available block with a limitation note.
9) All seller-facing text is Russian.
10) Do not output fact_id/entity_id in seller-facing text unless seller-useful.
11) top_actions and recommended_actions must be from same context and not conflict.
12) Do not mix old advertising numbers with current ad_efficiency facts.
13) Empty objects are not allowed for available blocks.
14) If status=available, block must contain complete seller-facing content.
15) Use allowed drivers and categories exactly from driver_hints.
16) Use category_name from category_catalog; never output «Категория <id>» if name is available.
17) Distinguish daily_account_orders, funnel_ordered_units, completed_units, status_orders using order_semantics labels.
18) Distinguish account_drr, ad_efficiency_drr, campaign_drr; always name the denominator.
19) Distinguish absolute weakest funnel stage from largest deterioration stage (see funnel_semantics).
20) Do not classify stock/replenishment actions as sales.
21) risk_overview group and severity are required for every group.
22) recommended_actions urgency_group is required.
23) account_health.risk_drivers[] must include category_label (Russian) for category.
24) category_score 0-3 => severity high/critical; 4-6 => medium; 7-8 => low; 9-10 => low/stable/positive.
25) ad_efficiency_narrative.checklist[]: title is short action; text is explanation (title must differ from text).
26) what_changed: main_conclusion must differ from summary; item text must not repeat label prefix (avoid «Выручка: Выручка...»).
27) If improved group has no real improvements, label it «Точки опоры» or «Сильные стороны», not «Улучшилось».
28) Never use «AI видит» / «система видит»; prefer «В кабинете сочетаются...».
29) top_actions[] must include fact_ids[] grounded in validated signals.
30) When funnel_semantics.stage_day_over_day is present, use it and never claim prior-day comparison is unavailable.
31) Never expose internal metric keys in seller-facing text (ad_efficiency_drr, daily_account_orders_count, daily_account_metrics, period_comparison).
32) When global_semantic_context is present, align headline/executive_summary with overall_story and risk drivers with primary/secondary risks.
33) Data Pack facts remain source of truth; Global Context helps prioritize and phrase conclusions.
34) Do not add facts not in Data Pack. Do not change factual blocks.
35) Dashboard Top-3 must NOT be taken from final Actions yet.
`)
}

func userPromptDashboardSynthesis() string {
	example := synthesisSchemaExampleJSON()
	return strings.TrimSpace(`
Synthesize dashboard narrative JSON with fields:
account_health with health_score (0-100), severity, headline, executive_summary, improvement_summary, risk_drivers[] (id, category, category_label, title, reason, severity, impact_score).
health_reasons[] with fields driver_id, category, title, reason (not summary), fact_ids[].
top_actions[] with fields driver_id, source_candidate_id, priority, category, title, reason (not summary), next_step, expected_effect, fact_ids[].
recommended_actions[] with ActionItem schema including urgency_group.
funnel_ai_analysis with FunnelAIAnalysisWidget schema and status=available including weakest_stage, comparisons, comparisons_commentary, action_items.
category_summaries[] with category_id, category_name, category_score (0-10 required), headline, key_risk, recommended_action, severity.
risk_overview with RiskDistributionWidget schema and status=available.
data_quality_notes[] must be plain strings, not objects.

Use driver_id and source_candidate_id from synthesis input hints when present.
Use category_catalog for all category names.
Use order_semantics and funnel_semantics — do not mix metrics without labels.
Ground health_reasons and top_actions in validated signal fact_ids.
Keep top_actions as dashboard-level suggestions, not operational action_items.

ad_efficiency_narrative and stock_coverage_narrative must be JSON objects, never plain strings.
seller_pulse_narrative and what_changed are required top-level objects.
category_score must be an integer 0-10, never a decimal.
Every required block is mandatory at the top level. If any block is missing, generation is rejected.

STRICT_VALID_JSON_EXAMPLE:
` + example)
}

func synthesisSchemaExampleJSON() string {
	example := map[string]any{
		"headline":          "Кабинет требует внимания по нескольким направлениям",
		"executive_summary": "В кабинете сочетаются сигналы в продажах, остатках и рекламе.",
		"account_health": map[string]any{
			"health_score": 62, "severity": "medium",
			"headline":            "Кабинет требует внимания по нескольким направлениям",
			"executive_summary":   "В кабинете сочетаются сигналы в продажах, остатках и рекламе.",
			"improvement_summary": "Сфокусируйтесь на остатках, рекламе и верхней воронке.",
			"risk_drivers": []any{
				map[string]any{"id": driverStock, "category": "stock", "category_label": "Остатки", "title": "Риск по остаткам", "reason": "Есть SKU с коротким покрытием.", "severity": "high", "impact_score": 70},
			},
		},
		"health_reasons": []any{
			map[string]any{"driver_id": driverStock, "category": "stock", "title": "Риск по остаткам", "reason": "Есть SKU с коротким покрытием.", "fact_ids": []any{}},
		},
		"improvement_summary": "Сфокусируйтесь на остатках, рекламе и верхней воронке.",
		"top_actions": []any{
			map[string]any{
				"driver_id": driverStock, "source_candidate_id": candidateStockReview, "priority": "high", "category": "stock",
				"title": "Пополнить остатки у продающих SKU", "reason": "...", "next_step": "...", "expected_effect": "...",
			},
		},
		"recommended_actions": []any{
			map[string]any{"priority": "high", "urgency_group": "urgent", "action": "...", "reason": "...", "expected_effect": "..."},
			map[string]any{"priority": "high", "urgency_group": "urgent", "action": "...", "reason": "...", "expected_effect": "..."},
			map[string]any{"priority": "medium", "urgency_group": "today", "action": "...", "reason": "...", "expected_effect": "..."},
		},
		"what_changed": map[string]any{
			"summary":         "Ключевые изменения относительно предыдущей сводки.",
			"main_conclusion": "Выручка снизилась относительно прошлой сводки, а основные риски сосредоточены в остатках и рекламе.",
			"groups": []any{
				map[string]any{"key": "worsened", "label": "Ухудшилось", "tone": "negative", "summary": "...", "items": []any{map[string]any{"label": "Выручка", "text": "..."}}},
				map[string]any{"key": "new_risks", "label": "Новые риски", "tone": "warning", "summary": "...", "items": []any{}},
				map[string]any{"key": "unchanged_risks", "label": "Активные проблемы", "tone": "neutral", "summary": "...", "items": []any{}},
				map[string]any{"key": "improved", "label": "Точки опоры", "tone": "positive", "summary": "Заметных улучшений по сравнению с прошлой сводкой не выявлено.", "empty_text": "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе.", "items": []any{}},
			},
		},
		"seller_pulse_narrative": map[string]any{
			"status": "available", "headline": "Пульс продавца", "summary": "Сводка дня.",
		},
		"stock_coverage_narrative": map[string]any{
			"status":    "available",
			"headline":  "Есть риск дефицита у продающих SKU",
			"summary":   "4 SKU находятся в критичной зоне 0–3 дня, ещё 4 — в зоне короткого запаса 4–7 дней. Приоритет — пополнить товары с подтверждённым спросом, чтобы не потерять продажи на ходовых позициях.",
			"next_step": "Открыть SKU с риском и приоритизировать поставку по темпу продаж и текущему покрытию.",
			"tone":      "warning",
		},
		"ad_efficiency_narrative": map[string]any{
			"status": "available", "summary": "Сводка по рекламе.",
			"checklist": []any{
				map[string]any{"title": "Ограничить перегретые кампании", "text": "Снизьте ставки или остановите кампании с ДРР выше целевого порога, чтобы не сжигать маржу.", "priority": "high"},
			},
		},
		"funnel_ai_analysis": map[string]any{
			"status":        "available",
			"short_summary": "Самая низкая абсолютная конверсия на верхнем этапе; отдельно — ухудшение корзины к вчера.",
			"weakest_stage": map[string]any{
				"key": "impressions_to_product_views", "label": "Показы → Просмотры карточки",
				"metric": "Конверсия", "value": "13,22%", "explanation": "Самая низкая абсолютная конверсия.",
			},
			"main_conclusion": "Резерв роста — верх воронки и карточки.",
			"action_items":    []any{"Усилить карточки лидеров продаж"},
			"comparisons": map[string]any{
				"stage_metrics": []any{}, "conversion_metrics": []any{}, "business_metrics": []any{},
			},
			"comparisons_commentary": map[string]any{
				"previous_period":   "Сильнее всего просела конверсия в корзину к вчера.",
				"seven_day_average": "К среднему за 7 дней просадка меньше.",
			},
		},
		"category_summaries": []any{
			map[string]any{
				"category_id": "17036100", "category_name": "Организация пространства", "category_score": 5,
				"headline": "Лидер выручки", "key_risk": "...", "recommended_action": "...", "severity": "medium",
			},
		},
		"risk_overview": map[string]any{
			"status": "available",
			"groups": []any{
				map[string]any{"group": "high_risk", "label": "Высокий риск", "count": 3, "severity": "critical"},
				map[string]any{"group": "medium_risk", "label": "Средний риск", "count": 2, "severity": "medium"},
				map[string]any{"group": "growth_zone", "label": "Зона роста", "count": 2, "severity": "positive"},
			},
			"total": 7,
		},
		"data_quality_notes": []any{"note"},
	}
	raw, _ := json.MarshalIndent(example, "", "  ")
	return string(raw)
}
