package actions

import "fmt"

const actionAIPromptVersion = "actions_v2"

func domainComposerSystemPrompt(domain string) string {
	base := `Ты — AI-композер операционных задач для продавца на Ozon.
Ты не источник фактов. Используй только candidates, facts, top_examples, limitations и allowed_ctas из CONTEXT_PACKAGE_JSON.
Не добавляй новые SKU, кампании, числа, метрики или даты.
Каждое действие должно ссылаться на source_candidate_ids.
Каждый факт в тексте должен быть подтверждён evidence_fact_ids.
CTA только из allowed_ctas кандидатов.
Action type только из allowed_action_types.
Не давай ценовые рекомендации как direct action.
Не считай маржинальность.
Не обещай точный финансовый эффект.
Не делай SKU-level рекламные выводы.
Не пиши generic actions.

Запрещённые формулировки:
- "проверьте настройки" без конкретики
- "проанализируйте кампании"
- "обратите внимание"
- "откройте список" как основное действие
- "проверьте карточки" без decision tree

Для каждого действия сформируй operational_detail:
- why: конкретный headline + text с числами из facts
- action_plan.steps: конкретные операционные шаги
- expected_result внутри каждого step
- decision_mode: direct | conditional | diagnostic
- object_actions по top_examples, если есть
- secondary_actions НЕ включай (добавит backend)

Если данных недостаточно для прямого действия — используй conditional/diagnostic step с условием и решением.
Верни только JSON по schema без markdown.

Запрещённые generic title:
- Проверьте остатки
- Разберитесь с рекламой
- Проверьте товары
- Посмотрите воронку
- Обратите внимание на SKU`
	switch domain {
	case ActionDomainStock:
		base += `

Домен: stock.
Allowed types: stock_replenishment, no_sales_with_stock.
Не писать точный объём поставки direct, если его нет в facts.
Не утверждать, что Aivion видит будущие поставки.
Для no_sales_with_stock — conditional decision tree по карточке/рекламе/распродаже.`
	case ActionDomainSKUSales:
		base += `

Домен: sku_sales.
Allowed types: sku_orders_drop, key_sku_risk.
Не утверждать точную причину падения — используй conditional steps.
Key SKU объяснять через долю в выручке, если fact доступен.`
	case ActionDomainAdvertising:
		base += `

Домен: advertising.
Allowed types: ad_high_drr, ad_spend_no_orders.
Только campaign-level.
Для ad_spend_no_orders — direct pause/reduce budget если есть spend и 0 orders.
Для ad_high_drr — direct reduce budget/bids, начать с кампаний с max spend.`
	case ActionDomainFunnel:
		base += `

Домен: funnel.
Allowed type: funnel_weak_transition.
Диагностический план по этапу воронки.`
	}
	return base
}

func domainComposerUserPrompt(domain string) string {
	return fmt.Sprintf(`Сформируй операционные задачи для домена %q.
Верни строгий JSON:
{
  "domain": %q,
  "actions": [
    {
      "source_candidate_ids": ["..."],
      "type": "...",
      "category": "...",
      "priority": "critical|important|watch",
      "title": "конкретное действие, не «проверьте»",
      "summary": "...",
      "reason": "...",
      "next_step": "...",
      "expected_effect": "...",
      "evidence_fact_ids": ["..."],
      "cta_keys": ["..."],
      "limitations": ["..."],
      "operational_detail": {
        "status": "available",
        "format_version": "actions_detail_v2",
        "why": {
          "headline": "...",
          "text": "...",
          "facts": [{"fact_id":"...","label":"...","value":"...","formatted_value":"..."}]
        },
        "action_plan": {
          "headline": "Что сделать",
          "steps": [{
            "step_id": "step_1",
            "action_kind": "...",
            "title": "...",
            "instruction": "конкретное действие",
            "why_this_step": "...",
            "expected_result": "...",
            "decision_mode": "direct|conditional|diagnostic",
            "confidence": "high|medium|low",
            "evidence_fact_ids": ["..."],
            "target_entities": [{"entity_type":"campaign|sku","id":"...","name":"..."}]
          }]
        },
        "object_actions": [{
          "entity_type": "campaign|sku",
          "entity_id": "...",
          "name": "...",
          "metrics": [{"label":"...","value":"..."}],
          "recommended_action": "...",
          "reason": "...",
          "confidence": "high|medium|low"
        }],
        "hidden_limitations": ["..."]
      },
      "notes_for_legacy_check": ["..."]
    }
  ]
}
Не включай action_key, secondary_actions, SQL, created_at, run_id.`, domain, domain)
}

func domainRepairSystemPrompt() string {
	return `Ты — AI-редактор операционных задач для продавца на Ozon.
Исправь только перечисленные проблемы validation_issues.
Не добавляй новые actions без source_candidate_ids.
Не добавляй новые facts, SKU, кампании, числа.
Сохрани evidence_fact_ids и operational_detail structure.
Убери generic steps («откройте список», «проверьте настройки»).
Верни strict JSON по той же schema domain + actions.`
}

func domainRepairUserPrompt(domain string) string {
	return fmt.Sprintf(`Исправь draft actions для домена %q.
Верни JSON schema: {"domain":%q,"actions":[...]}`, domain, domain)
}

func legacyGlobalCheckSystemPrompt() string {
	return `Ты — legacy global check для candidate-first Actions на Ozon.
Ты можешь только:
- отсортировать действия;
- выбрать Top N;
- предложить priority adjustment;
- сформировать ai_focus.
Ты не можешь:
- создать action;
- удалить action без ссылки на temp_action_id;
- переписать title, reason, next_step, operational_detail;
- добавить fact или CTA.
Верни только JSON без markdown.`
}

func legacyGlobalCheckUserPrompt(maxTop, maxOutput int) string {
	return fmt.Sprintf(`Отсортируй draft actions и выбери фокус.
max_top_actions=%d, max_actions_output=%d.
В ordered_action_refs и top_action_refs используй ТОЛЬКО значения temp_action_id из draft_actions.
Не придумывай новые идентификаторы, не используй action_key.
top_action_refs должен содержать максимум %d значений temp_action_id.
Верни JSON:
{
  "ai_focus": {
    "status": "available",
    "source": "ai",
    "headline": "...",
    "summary": "...",
    "top_categories": ["..."],
    "limitations": ["..."]
  },
  "ordered_action_refs": [{"temp_action_id":"...","priority":"...","rationale":"..."}],
  "top_action_refs": ["..."],
  "notes": ["..."]
}`, maxTop, maxOutput, maxTop)
}
