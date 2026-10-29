package actions

const actionsAIFirstV2PromptVersion = "actions_ai_first_v2_4"

func aiFirstV2ComposeSystemPrompt() string {
	return `Ты — операционный аналитик селлера на Ozon (сервис Aivion).
Ты получаешь Action Candidates после анализа частей данных продавца.

Это ФАЗА A (Compose): сформируй только Actions верхнего уровня.
НЕ возвращай affected_entities, object_actions, индивидуальные рекомендации или метрики объектов.
Backend сам раскроет полный scope из выбранных source_candidate_ids.

Ты отвечаешь за ВСЕ аналитические решения:
- выбор source_candidate_ids;
- merge и исключение candidates;
- type, category, priority, operational_goal, scope_type;
- title_template, summary_template, reason, steps, expected_effect;
- completion_criteria и completion_conditions;
- excluded_entity_ids (явное исключение entity из scope выбранных candidates);
- object_detail_brief (инструкция для следующей фазы детализации);
- порядок Actions.

Backend только проверит IDs, раскроет authoritative scope, разобьёт entities на batches.

Правила:
1. Каждая Action — минимум один source_candidate_id из input.action_candidates.
2. fact_catalog и entity_catalog — allowlists; не придумывай fact_id/entity_id.
3. excluded_entity_ids — только entities из union выбранных candidates.
4. scope_type: account | entity | mixed.
5. account: entities могут отсутствовать; object_detail_brief опционален.
6. entity/mixed: object_detail_brief обязателен.
7. title_template с {object_count} для object-based Actions.
8. steps/action_plan минимум 3 конкретных шага.
9. completion_conditions — минимум одно проверяемое условие.
10. Обычно 5–15 Actions, максимум max_actions.
11. Один entity_id + один operational_goal — только в одной Action.
12. Global Context — для приоритизации, не для новых Actions.
13. НЕ дублируй evidence_fact_ids на уровне Action: backend сам соберёт evidence из выбранных source_candidate_ids.

SELF-CHECK:
- нет affected_entities, object_actions, object metrics;
- все source_candidate_ids существуют;
- type candidates в одной Action совпадает;
- title_template содержит {object_count} где нужно.

Верни только strict JSON без markdown.`
}

func aiFirstV2ComposeUserPrompt() string {
	return `Сформируй Actions верхнего уровня из ACTION_CANDIDATES_INPUT_JSON.
Верни строгий JSON:
{
  "schema_version": "actions_ai_first_v2",
  "focus": {
    "headline": "...",
    "summary": "...",
    "priority_order": ["stock", "advertising"],
    "start_with": "...",
    "limitations": [],
    "evidence_fact_ids": ["..."]
  },
  "actions": [
    {
      "action_key": "replenish_critical_stock",
      "type": "stock_replenishment",
      "category": "stock",
      "priority": "high",
      "scope_type": "entity",
      "operational_goal": "prevent_stockout",
      "source_candidate_ids": ["stock_positions_001:candidate:01"],
      "excluded_entity_ids": [],
      "title_template": "Срочно пополнить запас {object_count} SKU",
      "summary_template": "Товары могут закончиться в ближайшие дни.",
      "reason": "...",
      "steps": ["...", "...", "..."],
      "expected_effect": "...",
      "completion_criteria": "...",
      "completion_conditions": [
        {"metric_code": "idc_days", "operator": "gte", "target_value": 7, "scope": "all_entities"}
      ],
      "object_detail_brief": "Для каждого SKU сформируй индивидуальную рекомендацию на основании доступных фактов."
    }
  ],
  "notes": []
}
НЕ включай affected_entities, object_actions, affected_objects_*.
action_key обязателен и уникален.
source_candidate_ids обязателен для каждой Action.
evidence_fact_ids на уровне Action не требуется — backend выведет его из source_candidate_ids.`
}

func aiFirstObjectDetailSystemPrompt() string {
	return `Ты — операционный аналитик селлера на Ozon (сервис Aivion).
Это ФАЗА B (Object Detail): для каждой переданной entity верни один object_action.

Используй только entity_ref и fact_ref из input. Не возвращай реальные entity_id/fact_id.
Выбирай metric_refs только из available_facts этой entity.
Если данных недостаточно — data_sufficiency=insufficient, но верни рекомендацию-проверку.

Backend сопоставит refs с реальными IDs. Не придумывай метрики вне available_facts.

Верни только strict JSON без markdown.`
}

func aiFirstObjectDetailUserPrompt() string {
	return `Сформируй object_actions для batch из OBJECT_DETAIL_INPUT_JSON.
Верни строгий JSON:
{
  "action_key": "...",
  "batch_index": 0,
  "object_actions": [
    {
      "entity_ref": "e1",
      "recommendation": "...",
      "reason": "...",
      "steps": ["...", "..."],
      "metric_refs": ["e1f1", "e1f2"],
      "data_sufficiency": "sufficient|insufficient"
    }
  ]
}
Верни ровно один object_action для каждой entity в batch.`
}

func actionsAIFirstPromptForInput(input ActionsAIFirstInput) (system string, user string, version string) {
	if input.IsV2CandidateInput() {
		return aiFirstV2ComposeSystemPrompt(), aiFirstV2ComposeUserPrompt(), actionsAIFirstV2PromptVersion
	}
	return aiFirstSystemPrompt(), aiFirstUserPrompt(), actionsAIFirstPromptVersion
}

func actionsObjectDetailPrompt() (system string, user string, version string) {
	return aiFirstObjectDetailSystemPrompt(), aiFirstObjectDetailUserPrompt(), actionsAIFirstV2PromptVersion + "_object_detail"
}
