package aidatapack

import "strings"

// MetricSemanticDefinition describes canonical meaning of a metric code for prompts and validators.
type MetricSemanticDefinition struct {
	MetricCode          string   `json:"metric_code"`
	CanonicalLabelRU    string   `json:"canonical_label_ru"`
	CanonicalLabelEN    string   `json:"canonical_label_en,omitempty"`
	DefinitionRU        string   `json:"definition_ru"`
	Unit                string   `json:"unit,omitempty"`
	AllowedSynonymsRU   []string `json:"allowed_synonyms_ru,omitempty"`
	ForbiddenTermsRU    []string `json:"forbidden_terms_ru,omitempty"`
	InterpretationNotes []string `json:"interpretation_notes,omitempty"`
}

var metricSemanticsRegistry = map[string]MetricSemanticDefinition{
	"idc_days": {
		MetricCode:       "idc_days",
		CanonicalLabelRU: "Покрытие запасом",
		CanonicalLabelEN: "Stock coverage (days)",
		DefinitionRU:     "Оценочное количество дней, на которое хватит текущего доступного запаса при текущем темпе спроса.",
		Unit:             "days",
		AllowedSynonymsRU: []string{
			"покрытие запасом в днях",
			"запаса хватит на",
			"дней покрытия",
			"покрытие запасом",
		},
		ForbiddenTermsRU: []string{
			"срок хранения",
			"дни хранения",
			"количество дней хранения",
			"срок годности",
			"товар хранится",
			"возраст запаса",
			"длительный срок хранения",
			"короткий срок хранения",
			"низкое количество дней хранения",
		},
		InterpretationNotes: []string{
			"Большее значение означает более длительное покрытие запасом.",
			"Меньшее значение означает более высокий риск дефицита при сохранении спроса.",
		},
	},
	"orders": {
		MetricCode:       "orders",
		CanonicalLabelRU: "Заказы",
		DefinitionRU:     "Количество заказов за указанный период.",
		Unit:             "count",
	},
	"orders_delta_pct": {
		MetricCode:       "orders_delta_pct",
		CanonicalLabelRU: "Изменение заказов",
		DefinitionRU:     "Процентное изменение количества заказов между сравниваемыми периодами.",
		Unit:             "percent",
	},
}

// MetricSemanticDefinitionFor returns the registry entry for a metric code, if present.
func MetricSemanticDefinitionFor(metricCode string) (MetricSemanticDefinition, bool) {
	def, ok := metricSemanticsRegistry[strings.TrimSpace(metricCode)]
	return def, ok
}

// MetricSemanticsForCodes returns definitions for the given metric codes only.
func MetricSemanticsForCodes(codes ...string) map[string]MetricSemanticDefinition {
	out := make(map[string]MetricSemanticDefinition)
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if def, ok := metricSemanticsRegistry[code]; ok {
			out[code] = def
		}
	}
	return out
}

// MetricSemanticsSnapshot returns all registered definitions for artifact/debug export.
func MetricSemanticsSnapshot() map[string]MetricSemanticDefinition {
	out := make(map[string]MetricSemanticDefinition, len(metricSemanticsRegistry))
	for k, v := range metricSemanticsRegistry {
		out[k] = v
	}
	return out
}

// MetricsPresentInChunk returns metric codes referenced by chunk local fact catalog.
func MetricsPresentInChunk(chunk DataPackChunk) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, fact := range chunk.LocalFactCatalog {
		m := strings.TrimSpace(fact.Metric)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}
