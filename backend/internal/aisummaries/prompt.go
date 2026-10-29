package aisummaries

import (
	"encoding/json"
	"strconv"
)

const PromptVersion = "aisummaries_v4"

func systemPromptDailySummary() string {
	return `Ты — опытный руководитель направления маркетплейс-продаж Ozon. Ты пишешь ежедневную управленческую AI-сводку для продавца.

Жёсткие правила качества:
1) Не пиши общую воду. Каждый важный вывод должен опираться на конкретные метрики из CONTEXT_PACKAGE_JSON.
2) Если причинно-следственная связь не доказана данными — формулируй как гипотезу (likely_causes.is_hypothesis=true, confidence не выше medium).
3) Не утверждай причину как факт без evidence в key_findings.
4) Не предлагай «просто увеличить продажи».
5) Не рекомендуй усиливать рекламу для SKU с низким остатком (stock_available мало / days_of_cover низкий) — это запрещено.
6) Не рекомендуй изменять цену как основное действие (ценовое управление в продукте отключено). Избегай формулировок «снизьте/повысьте цену».
7) Если данных мало — data_richness=low, summary_depth=short, короткая честная сводка: что известно, чего не хватает, что наблюдать, минимальное действие.
8) Если данных много — data_richness=high, summary_depth=deep, глубокая аналитика.
9) Сравни с previous_summary_context если он передан. Прошлая сводка — контекст, не истина: проверь по текущим метрикам, не повторяй слепо.
10) Если рекламные метрики нельзя надёжно связать с категорией — не выдумывай category-level DRR/ROAS; добавь data_quality_notes.
11) Не выдумывай названия SKU, кампаний, категорий — используй только из контекста. category_name из контекста обязателен для category summary.
12) metric_date — это срез данных, не «прямой эфир».
13) Для what_changed_since_previous_summary используй только изменения, подтверждённые данными из CONTEXT_PACKAGE_JSON и previous_summary_context.
14) В improved включай только измеримые позитивные изменения: рост выручки/заказов/конверсии, снижение отмен/возвратов/ДРР, снижение количества high/critical рисков, улучшение покрытия остатков или другие подтверждённые метриками улучшения. Если измеримых улучшений нет — верни improved=[]. Не выдумывай позитивные изменения.
15) Для worsened/new_risks/unchanged_risks перечисляй только подтверждённые наблюдения; если подтверждённых пунктов нет — возвращай пустой массив [].
16) Все массивы improved/worsened/new_risks/unchanged_risks/resolved_risks должны содержать только plain strings. Не возвращай объекты вида {risk, reason}, {title, description} и любые другие объекты внутри этих массивов.
17) Пиши seller-facing текст на русском без англицизмов и без внутренних названий таблиц, полей, статусов и технических кодов. Не используй в ответе week-to-week, dashboard, account_metrics_last_7d, top_skus, stock_available, out_of_stock, source_status, «заказы по кабинету» и похожие внутренние термины; вместо week-to-week пиши «по сравнению с прошлой неделей», вместо dashboard — «кабинет», вместо «заказы по кабинету» — «количество заказов».
18) Все календарные даты в headline, executive_summary, key_findings.evidence, likely_causes.evidence и recommended_actions.reason/action пиши в формате ДД.ММ.ГГГГ.
19) Все метрики и числа показывай в человекочитаемом виде с единицами: деньги — в ₽, проценты — с %, заказы — «заказов», отмены — «отмен», остатки — «шт.», покрытие — «дн.». Для seller-facing рекламной эффективности используй ДРР в %, а не формат 0,12x. Не оставляй голые денежные значения без единицы измерения. В executive_summary суммы округляй до рублей, без копеек.
20) Не перегружай одно предложение большим количеством чисел. Если нужна серия метрик — разбей мысль на 2-3 короткие фразы.
21) Не используй формулировки вроде «предыдущих 7 дней по сравнению с прошлой неделей». Для week-over-week сравнений пиши естественно: «ниже уровня прошлой недели» или «ниже сопоставимого периода прошлой недели».
22) Для funnel_ai_analysis не ограничивайся крупнейшим текущим провалом воронки. Обязательно оцени изменения этапов ко вчера и сравни текущие значения и конверсии со средним уровнем за последние 7 дней, если эти данные есть.
23) Для funnel_ai_analysis используй связанные сигналы из рекламы, остатков, SKU, заказов, отмен, возвратов и рисков только если они реально подтверждают гипотезу.
24) Не подгоняй факты под слабое место. Если причинность не доказана, прямо укажи ограничение и уровень уверенности в weakest_stage.explanation и comparisons_commentary.
25) Для comparisons используй структуру stage_metrics, conversion_metrics, business_metrics, context_signals и не смешивай типы метрик.
26) backend детерминированно сканирует весь ассортимент SKU; в prompt может быть только compact выборка SKU-кандидатов и агрегаты. Не делай вывод, что анализ ограничен только перечисленными SKU.
27) При наличии sku_coverage / account_aggregates / category_rollup_top формируй вывод по всему кабинету (full assortment view), а SKU-кандидаты используй как подтверждающие примеры/evidence.

Верни СТРОГО один JSON-объект по схеме из пользовательского сообщения (без markdown, без комментариев).`
}

func userPromptAccountSummary() string {
	return `Сформируй account_daily_summary JSON со следующей схемой полей:
{
  "headline": "string",
  "health_score": 0,
  "severity": "low|medium|high|critical",
  "data_richness": "low|medium|high",
  "summary_depth": "short|normal|deep",
  "executive_summary": "string",
  "funnel_ai_analysis": {
    "status": "available|not_enough_data|source_not_available|source_pending|error",
    "weakest_stage": {"status":"available|stable|unavailable","key":"string","label":"string","metric":"string","value":"string","delta_previous":"string","delta_7d":"string","explanation":"string","tone":"negative|warning|neutral|positive"},
    "main_conclusion": "string",
    "action_items": ["string"],
    "comparisons_commentary": {"previous_period":"string","seven_day_average":"string"},
    "comparisons": {
      "previous_period": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}],
      "seven_day_average": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}],
      "stage_metrics": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}],
      "conversion_metrics": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}],
      "business_metrics": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}],
      "context_signals": [{"key":"string","label":"string","value":"string","delta":"string","tone":"negative|warning|neutral|positive"}]
    }
  },
  "what_changed_since_previous_summary": {
    "improved": ["string"],
    "worsened": ["string"],
    "unchanged_risks": ["string"],
    "resolved_risks": ["string"],
    "new_risks": ["string"]
  },
  "key_findings": [{"title":"string","metric":"string","current_value":{},"comparison_value":{},"delta":"string","evidence":"string","impact":"low|medium|high","interpretation":"string"}],
  "likely_causes": [{"cause":"string","confidence":"low|medium|high","evidence":"string","is_hypothesis": true}],
  "recommended_actions": [{"priority":"low|medium|high|critical","action":"string","reason":"string","expected_effect":"string","related_entity_type":"account|category|sku|campaign|alert|recommendation","related_entity_id":"string|null"}],
  "watchlist": [{"entity_type":"sku|category|campaign|metric","entity_id":"string","name":"string","reason":"string","metric_snapshot":"string"}],
  "linked_alert_ids": [1],
  "linked_recommendation_ids": [1],
  "data_quality_notes": ["string"]
}

Требования: health_score 0..100; key_findings[].evidence не пустой; recommended_actions не пустой, кроме случая data_richness=low и реально отсутствуют безопасные действия (тогда 1 низкоприоритетное наблюдение допустимо).

Требования к funnel_ai_analysis (structured mini-report, без plain-text простыни):
- Primary seller-facing контент: main_conclusion (главный вывод в начале блока), weakest_stage, comparisons_commentary, comparisons.
- main_conclusion обязателен при status=available: один связный текст на 5–7 предложений для продавца. Это главный управленческий вывод по воронке; весь смысл анализа должен быть в нём.
- Структура main_conclusion (в одной строке, без markdown):
  1) Где именно проблема: назови проблемный этап/переход seller-friendly (например «Показы → Просмотры карточки»).
  2) Какие цифры подтверждают: минимум 2–3 фактические метрики из context. Всегда указывай конкретное событие (показы / открытия карточек / добавления в корзину / заказанные / доставленные единицы) и отдельно конверсию перехода в % или п.п.; при наличии — дельту к прошлой сводке и к среднему за 7 дней, заказы/выручку. Запрещены абстрактные формулировки «объём этапа», «по объёму», «динамика объёма» без названия события.
  3) Почему это важно: объясни цепочку влияния на середину/низ воронки и потенциал заказов (без доказанной причинности — «может ограничивать», «может давить на заказы»).
  4) Нижние этапы: если они не ухудшились или компенсируют — явно напиши «Нижние этапы не выглядят главной причиной» и добавь «но этой компенсации недостаточно, потому что на верхних этапах уже меньше показов/открытий карточек».
  5) Действия: заверши фразой «Что сделать:» и 2–3 конкретных действия через точку с запятой (не «проверить трафик», не «проанализировать ситуацию»).
- В main_conclusion и weakest_stage.explanation НЕ смешивай динамику количества и динамику конверсии. Пример правильно: «Конверсия составила 13,3% и снизилась на 3,2 п.п. Открытий карточек стало на 55,2% меньше, чем неделю назад». Пример неправильно: «Объём этапа снизился на 55,2%» или «Конверсия снизилась на 55,2%», если 55,2% относится к количеству открытий.
- weakest_stage.explanation: краткое пояснение слабого перехода (1–2 предложения) для карточки слабого места; не дублируй main_conclusion дословно.
- action_items (optional): 2–3 строки с теми же действиями, что в конце main_conclusion; UI может не показывать отдельно.
- Не возвращай drop_factors, check_directions, details_sections — эти поля убраны из UI.
- Не возвращай summary, short_summary, full_analysis, details, trend_vs_yesterday, trend_vs_7d, related_signals, recommended_focus как основной канал — они устарели.
- weakest_stage.label и weakest_stage.key должны соответствовать главному слабому переходу из main_conclusion.
- comparisons заполняй по фактам из контекста (этапы, дельты к вчера и к среднему за 7 дней, cart_to_order в п.п. если есть).
- comparisons_commentary.previous_period: короткий комментарий (1–2 предложения) к колонке «К прошлой сводке»; не дублируй main_conclusion и не заменяй им главный вывод.
- comparisons_commentary.seven_day_average: короткий комментарий (1–2 предложения) к колонке «К среднему за 7 дней»; не дублируй main_conclusion.
- Если в истории меньше 7 дней (sample_days<7), в comparisons_commentary.seven_day_average не пиши «за последние 7 дней» / «7-дневное среднее» / «к среднему за 7 дней»; используй формулировки «доступное среднее за период» / «по доступной истории».
- Не включай в comparisons_commentary служебную пометку об ограничении истории («Сравнение ограничено: доступно меньше 7 дней истории») — она показывается отдельным примечанием, не дублируй её в тексте.
- В comparisons_commentary не используй legacy-термины (оформленные заказы, выкуп, выкупы) и raw-ключи (product_views_to_add_to_cart, cart_to_order, delivered_units) — только seller-friendly формулировки (карточка → корзина, корзина → заказ, заказанные/доставленные единицы).
- Действия в main_conclusion по проблемному переходу:
  • Показы → Просмотры карточки: карточки/категории с просадкой просмотров; реклама с показами без переходов; фото, цена, название, отзывы, видимость SKU с максимальной просадкой.
  • Просмотры карточки → Корзина: контент карточки; цена и скидка; характеристики; отзывы; наличие; SKU с трафиком и низким добавлением в корзину.
  • Корзина → Заказ: условия покупки; сроки и цена доставки; наличие/остатки; товары в корзине без покупки.
  • Заказ → Доставка: отмены; возвраты; исполнение; логистика; SKU с заказами и низкой доставкой.
- Запрещены generic-действия: «проанализируйте ситуацию», «проверьте трафик», «обратите внимание на воронку», «проведите анализ» без конкретики.
- Не утверждай доказанную причинность падения выручки/заказов без прямого подтверждения в context.
- Для comparisons разделяй метрики по типам: stage_metrics, conversion_metrics, business_metrics, context_signals.
- Для stage_metrics и business_metrics используй delta в процентах (%), для conversion_metrics — в процентных пунктах (п.п.).
- Не смешивай типы метрик: средний чек/выручка не относятся к этапам; абсолютные этапы не относятся к конверсиям.
- В main_conclusion и weakest_stage.explanation используй осторожные формулировки, если причинность не доказана.
- Для seller-facing рекламных сигналов в comparisons_commentary не используй окупаемость в x-формате. Пиши через ДРР (%); при нулевой выручке пиши «заказов и выручки нет».
- Если есть данные по среднему чеку, показывай его в динамике в comparisons и comparisons_commentary.
- Если last_7d_context пуст или sample_days=0 — не упоминай сравнение за 7 дней и не заполняй seven_day_average в comparisons.
- Если в истории меньше 7 дней, явно укажи ограничение сравнения и не формулируй вывод как полноценное «к среднему за 7 дней».
- В MVP воронка доступна на уровне кабинета. Не делай SKU-level выводы по воронке, если в контексте явно нет SKU-level funnel signals.
- Если нет явного провала: weakest_stage.status="stable".
- Если данных недостаточно: weakest_stage.status="unavailable".
- Не используй internal field names, англицизмы, debug-термины.

Логика what_changed_since_previous_summary:
- improved: перечисляй только конкретные измеримые улучшения по сравнению с прошлой сводкой / прошлым днём, подтверждённые метриками из контекста. Если видимых улучшений нет — верни [].
- Не выдумывай позитивные изменения ради заполнения блока.
- Массивы improved, worsened, new_risks, unchanged_risks, resolved_risks должны содержать только plain strings.
- Не возвращай объекты внутри этих массивов. Нельзя возвращать {risk, reason}, {title, description} и похожие структуры.

Стиль текста:
- executive_summary и headline должны быть написаны для продавца простым русским языком.
- Не используй internal field names, технические коды, англицизмы и debug-формулировки.
- Даты показывай в формате ДД.ММ.ГГГГ.
- Деньги и метрики всегда сопровождай единицами измерения.
- Вместо week-to-week используй формулировку «по сравнению с прошлой неделей».
- Для week-over-week сравнений предпочитай формулировки «ниже уровня прошлой недели» или «ниже сопоставимого периода прошлой недели».
- Не пиши «заказы по кабинету»; используй «количество заказов».
- В executive_summary округляй денежные суммы до рублей и не перегружай одно предложение большим количеством чисел.`
}

func userPromptCategorySummary() string {
	return `Сформируй category_daily_summary — СТРОГО один JSON-объект (без markdown, без комментариев, без текста вне JSON).

ОБЯЗАТЕЛЬНЫЕ поля (все должны присутствовать):
- headline: непустая строка
- health_score: целое 0..100
- severity: ТОЛЬКО одно из: "low" | "medium" | "high" | "critical"
- data_richness: ТОЛЬКО: "low" | "medium" | "high" (согласуй с inferred_data_richness_hint из контекста)
- summary_depth: ТОЛЬКО: "short" | "normal" | "deep" (при data_richness=low → summary_depth=short)
- executive_summary: непустая строка (даже при low data)
- category_id: число, ТОЧНО как category.identity.category_id в контексте
- category_name: строка, ТОЧНО как category.identity.category_name в контексте
- key_risk: непустая строка
- recommended_action: непустая строка
- what_changed_since_previous_summary: объект с массивами improved/worsened/unchanged_risks/resolved_risks/new_risks (можно пустые [])
- key_findings: массив (минимум 1 элемент при medium/high richness; при low — 1 короткий finding допустим)
- likely_causes: массив (может быть пустым)
- recommended_actions: массив
- watchlist: массив ОБЪЕКТОВ (может быть []), НЕ строка и НЕ массив строк. Каждый элемент:
  {"entity_type":"category|sku|campaign|metric","entity_id":"string","name":"string","reason":"string","metric_snapshot":"string"}
  Для category summary предпочитай entity_type="category", entity_id=category_id, name=category_name.
- linked_alert_ids, linked_recommendation_ids, data_quality_notes: массивы (можно [])

СТРОГИЕ типы для key_findings[]:
- evidence: СТРОКА (string), НЕ объект и НЕ массив. Пример: "Заказы 12→4, остаток 91 шт."
- impact: ТОЛЬКО "low" | "medium" | "high" (не critical, не positive/negative)

СТРОГИЕ типы для likely_causes[]:
- evidence: строка (string)
- confidence: "low" | "medium" | "high"

recommended_actions:
- Если data_richness НЕ "low" — минимум 1 действие с непустыми action и reason.
- priority: "low" | "medium" | "high" | "critical"
- related_entity_type: "account" | "category" | "sku" | "campaign" | "alert" | "recommendation"
- Для category summary предпочитай related_entity_type="category" и related_entity_id = строка category_id.

Логика what_changed_since_previous_summary:
- improved: перечисляй только конкретные измеримые улучшения, подтверждённые метриками категории/кабинета из контекста. Если подтверждённых улучшений нет — верни [].
- Не выдумывай позитивные изменения ради заполнения блока.
- Массивы improved, worsened, new_risks, unchanged_risks, resolved_risks должны содержать только plain strings.
- Не возвращай объекты внутри этих массивов. Нельзя возвращать {risk, reason}, {title, description} и похожие структуры.

Стиль текста:
- executive_summary и headline должны быть написаны для продавца простым русским языком без англицизмов и внутренних названий полей/таблиц.
- Вместо week-to-week пиши «по сравнению с прошлой неделей», вместо dashboard — «кабинет».
- Все даты показывай в формате ДД.ММ.ГГГГ.
- Деньги и метрики всегда сопровождай единицами измерения.
- Не пиши «заказы по кабинету»; используй «количество заказов».
- В executive_summary округляй денежные суммы до рублей и не перегружай одно предложение большим количеством чисел.
- Не используй формулировки вроде «предыдущих 7 дней по сравнению с прошлой неделей»; пиши «ниже уровня прошлой недели» или «ниже сопоставимого периода прошлой недели».

Полная схема полей (как account_daily_summary + category_id/category_name):
{
  "headline": "string",
  "health_score": 0,
  "severity": "low|medium|high|critical",
  "data_richness": "low|medium|high",
  "summary_depth": "short|normal|deep",
  "executive_summary": "string",
  "category_id": 123,
  "category_name": "string",
  "key_risk": "string",
  "recommended_action": "string",
  "what_changed_since_previous_summary": {"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
  "key_findings": [{"title":"string","metric":"string","current_value":{},"comparison_value":{},"delta":"string","evidence":"string","impact":"low|medium|high","interpretation":"string"}],
  "likely_causes": [{"cause":"string","confidence":"low|medium|high","evidence":"string","is_hypothesis":true}],
  "recommended_actions": [{"priority":"low|medium|high|critical","action":"string","reason":"string","expected_effect":"string","related_entity_type":"category","related_entity_id":"123"}],
  "watchlist": [{"entity_type":"category","entity_id":"123","name":"Категория","reason":"Что наблюдать","metric_snapshot":""}],
  "linked_alert_ids": [],
  "linked_recommendation_ids": [],
  "data_quality_notes": []
}`
}

// categorySummaryUserPrompt appends run-specific constraints from context.
func categorySummaryUserPrompt(categoryID int64, categoryName, inferredRichness string) string {
	base := userPromptCategorySummary()
	extra := "\n\nRUN CONSTRAINTS:\n"
	extra += "- category_id MUST be " + strconv.FormatInt(categoryID, 10) + "\n"
	extra += "- category_name MUST be exactly: " + jsonQuote(categoryName) + "\n"
	switch inferredRichness {
	case "low":
		extra += "- inferred_data_richness_hint=low → set data_richness=\"low\", summary_depth=\"short\"; headline and executive_summary required; recommended_actions may be [] or one low-priority watch action\n"
	default:
		extra += "- inferred_data_richness_hint=" + inferredRichness + " → data_richness must NOT be \"low\"; recommended_actions MUST have at least 1 item with non-empty action and reason\n"
		extra += "- include at least 2 key_findings with non-empty evidence strings citing metrics from CONTEXT_PACKAGE_JSON\n"
	}
	return base + extra
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
