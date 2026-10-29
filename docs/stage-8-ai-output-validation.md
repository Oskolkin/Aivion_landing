# Stage 8. AI Output Validation Guardrails

## Зачем нужен validator

Backend не должен доверять AI-ответу blindly.  
Даже при корректном prompt contract модель может вернуть:

- невалидный JSON;
- неизвестный `recommendation_type`;
- рекомендации с несуществующими entity/alert ссылками;
- действия, противоречащие pricing/stock/margin ограничениям.

Validator отсекает такие случаи до любого сохранения в БД.

## Canonical `recommendation_type` (MVP)

Единый список в `backend/internal/recommendations/recommendation_types.go`:

- `replenish_sku`, `review_ad_spend`, `pause_or_reduce_ads`, `avoid_ads_for_low_stock_sku`, `investigate_sales_drop`, `review_price_margin`, `review_price_floor`, `discount_overstock`, `monitor_sku`, `account_priority_review`

Prompt и validator используют только этот список. OpenAI-синонимы (например `stock_replenishment`) проходят через `NormalizeRecommendationType` и сохраняются в БД как canonical. Неизвестные типы отклоняются с причиной `recommendation_type is not allowed: <value>`.

## Что проверяет validator

`backend/internal/recommendations/validator.go` выполняет:

- parse и shape check (`{"recommendations":[...]}` или массив);
- required fields + enum checks;
- allowlist `recommendation_type` (canonical MVP enum in `recommendation_types.go`; AI aliases such as `stock_replenishment` normalize to `replenish_sku` before validation);
- required `supporting_metrics` (non-empty object);
- required `constraints_checked`/`constraints` (non-empty object, alias support);
- entity existence checks по входному `AIRecommendationContext`;
- `supporting_alert_ids` existence;
- `related_alert_types` consistency с context и related ids;
- evidence contradiction checks для ключевых recommendation типов;
- deterministic pricing guardrails (простые text triggers + numeric extraction);
- stock/ad guardrail (запрет на усиление рекламы при low stock signals);
- margin-risk guardrail (reject по expected margin thresholds и missing margin checks);
- sanitization (trim, caps, dedupe ids/types).

## Reject vs Warning

Validator разделяет результат на:

- `valid_recommendations`;
- `rejected_recommendations` с причиной.

Для части soft-несоответствий добавляются warnings, а confidence понижается:

- `high -> medium`
- `medium -> low`
- `low -> low`

Это позволяет не терять потенциально полезные рекомендации, но делать их менее приоритетными и более безопасными для последующих шагов.

## Источник данных для проверок

Validator работает только по:

- AI output;
- входному `AIRecommendationContext`.

На этапе validation он не ходит в БД и не вызывает внешние API.
