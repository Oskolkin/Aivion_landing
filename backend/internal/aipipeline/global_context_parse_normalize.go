package aipipeline

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

func normalizeGlobalContextJSON(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []byte("{}"), nil
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wire); err != nil {
		return []byte(trimmed), err
	}
	for _, key := range []string{"primary_risks", "secondary_risks", "opportunities", "source_fact_ids", "data_limitations", "priority_order"} {
		if field, ok := wire[key]; ok {
			wire[key] = normalizeGlobalContextJSONArrayField(field)
		}
	}
	return json.Marshal(wire)
}

func normalizeGlobalContextJSONArrayField(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return json.RawMessage("[]")
	}
	switch v := probe.(type) {
	case string:
		inner := strings.TrimSpace(v)
		if inner == "" {
			return json.RawMessage("[]")
		}
		if strings.HasPrefix(inner, "[") {
			return json.RawMessage(inner)
		}
		encoded, err := json.Marshal([]string{inner})
		if err != nil {
			return json.RawMessage("[]")
		}
		return json.RawMessage(encoded)
	case []any:
		return raw
	default:
		return json.RawMessage("[]")
	}
}

func parseFlexibleStringList(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		return compactStrings(items)
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}
	return nil
}

func parseFlexibleStringSliceField(raw json.RawMessage) aicontext.FlexibleStringSlice {
	return aicontext.FlexibleStringSlice(parseFlexibleStringList(raw))
}

func parseFlexibleSemanticRisks(raw json.RawMessage) []aicontext.SemanticRisk {
	return parseFlexibleSemanticItems(raw, func(item json.RawMessage) (aicontext.SemanticRisk, bool) {
		var risk aicontext.SemanticRisk
		if err := json.Unmarshal(item, &risk); err != nil {
			return aicontext.SemanticRisk{}, false
		}
		return risk, true
	})
}

func parseFlexibleSemanticSignals(raw json.RawMessage) []aicontext.SemanticSignal {
	return parseFlexibleSemanticItems(raw, func(item json.RawMessage) (aicontext.SemanticSignal, bool) {
		var signal aicontext.SemanticSignal
		if err := json.Unmarshal(item, &signal); err != nil {
			return aicontext.SemanticSignal{}, false
		}
		return signal, true
	})
}

func parseFlexibleSemanticItems[T any](raw json.RawMessage, parse func(json.RawMessage) (T, bool)) []T {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if item, ok := parse(raw); ok {
			return []T{item}
		}
		return nil
	}
	out := make([]T, 0, len(items))
	for _, item := range items {
		if len(item) == 0 || string(item) == "null" {
			continue
		}
		if item[0] == '"' {
			continue
		}
		if parsed, ok := parse(item); ok {
			out = append(out, parsed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
