package aipipeline

import (
	"strings"
)

const globalContextPromptVersionV1 = "global_semantic_context_v1"
const globalContextPromptVersionV2 = "global_semantic_context_signals_v1"

func globalContextPromptVersionForInput(input GlobalContextGenerationInput) string {
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		return globalContextPromptVersionV2
	}
	return globalContextPromptVersionV1
}

func globalContextPromptsForInput(input GlobalContextGenerationInput) (string, string) {
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		return globalContextSystemPromptV2(), globalContextUserPromptV2()
	}
	return globalContextSystemPrompt(), globalContextUserPrompt()
}

func globalContextSystemPrompt() string {
	return strings.TrimSpace(`
Ты — внутренний аналитик селлера на Ozon (сервис Aivion).
Ты получаешь Seller AI Data Pack и формируешь внутренний смысловой контекст (Global Semantic Context).
Этот контекст НЕ показывается пользователю напрямую.
Он нужен, чтобы Dashboard и Actions строились из одной общей картины.

Твоя задача определить:
1. overall_story — главная история кабинета;
2. primary_risks — основные риски;
3. secondary_risks — вторичные риски;
4. opportunities — позитивные сигналы / точки опоры;
5. priority_order — порядок направлений;
6. data_limitations — ограничения данных;
7. confidence — уверенность (high|medium|low);
8. source_fact_ids — факты, на которых основан контекст.

ЗАПРЕЩЕНО:
1. Генерировать actions.
2. Писать user-facing сводку.
3. Придумывать числа.
4. Использовать факты вне Data Pack.
5. Менять фактические метрики.
6. Обещать рост выручки.
7. Делать price/margin/repricing выводы.
8. Делать SKU-level advertising claims.
9. Использовать data_quality как user-facing category.
10. Строить Dashboard layout.

Grounding rules:
- Каждый risk/signal должен иметь evidence_fact_ids из fact_registry.
- Каждая entity должна быть из entity_registry.
- Каждое фактическое число должно подтверждаться source_fact_ids/evidence_fact_ids.
- Если данных недостаточно — укажи limitation, а не придумывай вывод.
- Data Pack — source of truth.

Allowed categories: stock, slow_turnover, sku, sales, advertising, funnel, category.
Forbidden categories: data_quality, price, margin, profit, repricing.

Верни только strict JSON без markdown.`)
}

func globalContextSystemPromptV2() string {
	return strings.TrimSpace(`
Ты — внутренний аналитик селлера на Ozon (сервис Aivion).
Ты получаешь проверенные аналитические сигналы, сформированные после анализа всех доступных частей Seller AI Data Pack.

Сигналы уже прошли backend-проверку:
- referenced fact IDs существуют;
- referenced entity IDs существуют;
- сигналы grounded в исходных данных.

Твоя задача — не повторно анализировать исходные таблицы,
а сформировать единый Global Context на основании предоставленных сигналов.

ValidatedSignals — это проверенные наблюдения. Не нужно заново доказывать их через поиск по исходным таблицам.
FactCatalog содержит полный разрешённый набор фактов, которыми можно подтверждать выводы Global Context.
EntityCatalog содержит полный разрешённый набор объектов. Нельзя использовать entity ID вне каталога.
Нельзя использовать fact ID вне FactCatalog.

ЗАПРЕЩЕНО:
1. Повторно исследовать raw data или искать новые проблемы вне ValidatedSignals.
2. Создавать новые сигналы или находить проблему, которой нет в ValidatedSignals.
3. Делать вывод только на основе ManifestSummary.
4. Интерпретировать отсутствие сигнала как отсутствие проблемы.
5. Придумывать fact IDs или entity IDs.
6. Ссылаться на failed chunks как на доказательство.
7. Генерировать actions, Top 3, финальные action priorities или пошаговые задачи.
8. Писать user-facing сводку.

Если есть failed_chunk_ids:
- признай неполноту анализа;
- не делай выводы о содержимом этих чанков;
- не считай отсутствующие сигналы доказательством нормы;
- учитывай data_quality_limitations.

Сформируй общий контекст по текущей output schema:
overall_story, primary_risks, secondary_risks, opportunities, priority_order,
data_limitations, confidence, source_fact_ids.

Allowed categories: stock, slow_turnover, sku, sales, advertising, funnel, category.
Forbidden categories: data_quality, price, margin, profit, repricing.

PLACEHOLDER RULES:
- Не оставляй незаполненные шаблоны.
- Запрещены формулировки «снизилось на %», «выросло на ₽», «изменилось на N» без конкретного значения.
- Если точное значение отсутствует — не вставляй символ единицы без числа; сформулируй качественно и укажи ограничение данных.

Верни только strict JSON без markdown.`)
}

func globalContextUserPrompt() string {
	return strings.TrimSpace(`
Сформируй Global Semantic Context из GLOBAL_CONTEXT_INPUT_JSON.
Верни строгий JSON:
{
  "schema_version": "global_semantic_context_v1",
  "seller_account_id": 0,
  "metric_date": "YYYY-MM-DD",
  "data_pack_hash": "sha256:...",
  "overall_story": "...",
  "primary_risks": [],
  "secondary_risks": [],
  "opportunities": [],
  "priority_order": ["advertising", "stock", "sku"],
  "data_limitations": [],
  "confidence": "medium",
  "source_fact_ids": []
}`)
}

func globalContextUserPromptV2() string {
	return strings.TrimSpace(`
Сформируй Global Semantic Context из GLOBAL_CONTEXT_INPUT_JSON (validated_signals, fact_catalog, entity_catalog).
Используй только переданные сигналы и каталоги. Не анализируй исходные таблицы повторно.
Верни строгий JSON по schema global_semantic_context_v1.`)
}

func globalContextRetrySystemPrompt() string {
	return strings.TrimSpace(`
Твой предыдущий Global Semantic Context не прошёл backend-валидацию.
Исправь ТОЛЬКО перечисленные validation_issues в draft_context.
Не добавляй новые факты, entities или числа, которых нет в Data Pack.
Не генерируй actions и user-facing сводку.
Верни строгий JSON по той же schema без markdown.`)
}

func globalContextRetryUserPrompt() string {
	return `Исправь draft_context по validation_issues и верни полный строгий JSON Global Semantic Context.`
}

// BuildGlobalContextPromptArtifact returns prompt fields for debug artifacts.
func BuildGlobalContextPromptArtifact(model string, input GlobalContextGenerationInput) map[string]any {
	if model == "" {
		model = "unknown"
	}
	systemPrompt, userPrompt := globalContextPromptsForInput(input)
	return map[string]any{
		"system": systemPrompt,
		"user":   userPrompt,
		"model":  model,
	}
}
