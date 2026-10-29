package dashboardhome

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aijsonrepair"
)

type synthesisParseResult struct {
	Payload       *dashboardNarrativePayload
	JSONRepairs   []string
	ParseError    error
	RepairedJSON  string
	ExtractedJSON string
}

func coerceStringNarrativeObject(root map[string]json.RawMessage, key, summaryField string) {
	raw, ok := root[key]
	if !ok {
		return
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || strings.TrimSpace(s) == "" {
		return
	}
	obj := map[string]any{summaryField: strings.TrimSpace(s)}
	if key == "ad_efficiency_narrative" {
		obj["status"] = "available"
		obj["checklist"] = []any{}
	}
	if key == "stock_coverage_narrative" {
		obj["status"] = "available"
		obj["actions"] = []any{}
	}
	encoded, err := json.Marshal(obj)
	if err == nil {
		root[key] = encoded
	}
}

func normalizeDashboardPayloadRoot(root map[string]json.RawMessage) {
	coerceStringNarrativeObject(root, "ad_efficiency_narrative", "summary")
	coerceStringNarrativeObject(root, "stock_coverage_narrative", "summary")
	if raw, ok := root["data_quality_notes"]; ok {
		root["data_quality_notes"] = normalizePayloadDataQualityNotesJSON(raw)
	}
	if raw, ok := root["health_reasons"]; ok {
		root["health_reasons"] = normalizeHealthReasonsJSONArray(raw)
	}
	if raw, ok := root["top_actions"]; ok {
		root["top_actions"] = normalizeTopActionsJSONArray(raw)
	}
	if raw, ok := root["ad_efficiency_narrative"]; ok {
		root["ad_efficiency_narrative"] = normalizeAdEfficiencyNarrativeJSON(raw)
	}
	if raw, ok := root["stock_coverage_narrative"]; ok {
		root["stock_coverage_narrative"] = normalizeStockCoverageNarrativeJSON(raw)
	}
	if raw, ok := root["what_changed"]; ok {
		root["what_changed"] = normalizeWhatChangedJSON(raw)
	}
	if raw, ok := root["recommended_actions"]; ok {
		root["recommended_actions"] = normalizeRecommendedActionsJSONArray(raw)
	}
	if raw, ok := root["category_summaries"]; ok {
		root["category_summaries"] = normalizeCategorySummariesJSONArray(raw)
	}
	if raw, ok := root["account_health"]; ok {
		root["account_health"] = normalizeAccountHealthJSON(raw)
	}
}

func normalizeWhatChangedJSON(raw json.RawMessage) json.RawMessage {
	var wc map[string]any
	if err := json.Unmarshal(raw, &wc); err != nil {
		return raw
	}
	groupsAny, ok := wc["groups"]
	if !ok {
		if items, hasItems := wc["items"]; hasItems {
			wc["groups"] = []any{map[string]any{
				"key": "worsened", "label": "Ухудшилось", "tone": "negative", "summary": wc["summary"], "items": items,
			}}
			delete(wc, "items")
		}
		groupsAny = wc["groups"]
	}
	groups, ok := groupsAny.([]any)
	if !ok {
		encoded, err := json.Marshal(wc)
		if err != nil {
			return raw
		}
		return encoded
	}
	for i, groupAny := range groups {
		group, ok := groupAny.(map[string]any)
		if !ok {
			continue
		}
		if jsonStringField(group, "key") == "" {
			category := strings.ToLower(jsonStringField(group, "category"))
			switch category {
			case "sales":
				group["key"] = whatChangedKeyWorsened
			case "stock", "assortment", "advertising":
				group["key"] = whatChangedKeyNewRisks
			case "operations", "data_quality":
				group["key"] = whatChangedKeyUnchangedRisks
			default:
				fallbackKeys := []string{whatChangedKeyWorsened, whatChangedKeyNewRisks, whatChangedKeyUnchangedRisks, whatChangedKeyImproved}
				if i < len(fallbackKeys) {
					group["key"] = fallbackKeys[i]
				}
			}
		} else if mapped := canonicalWhatChangedGroupKey(jsonStringField(group, "key")); mapped != "" && mapped != jsonStringField(group, "key") {
			group["key"] = mapped
		}
		if jsonStringField(group, "label") == "" {
			if label := labelForWhatChangedKey(jsonStringField(group, "key")); label != "" {
				group["label"] = label
			}
		}
		if jsonStringField(group, "tone") == "" {
			if tone := toneForWhatChangedKey(jsonStringField(group, "key")); tone != "" {
				group["tone"] = tone
			}
		}
		itemsAny, ok := group["items"]
		if !ok {
			continue
		}
		items, ok := itemsAny.([]any)
		if !ok {
			continue
		}
		normalizedItems := make([]any, 0, len(items))
		for _, itemAny := range items {
			switch item := itemAny.(type) {
			case string:
				text := strings.TrimSpace(item)
				if text != "" {
					normalizedItems = append(normalizedItems, map[string]any{"text": text})
				}
			case map[string]any:
				if jsonStringField(item, "label") == "" {
					if text := jsonStringField(item, "text"); text != "" {
						item["label"] = "Изменение"
					}
				}
				if jsonStringField(item, "text") == "" {
					if summary := jsonStringField(item, "summary"); summary != "" {
						item["text"] = summary
					}
				}
				normalizedItems = append(normalizedItems, item)
			default:
				normalizedItems = append(normalizedItems, itemAny)
			}
		}
		group["items"] = normalizedItems
		groups[i] = group
	}
	wc["groups"] = groups
	encoded, err := json.Marshal(wc)
	if err != nil {
		return raw
	}
	return encoded
}

func jsonStringField(item map[string]any, key string) string {
	if v, ok := item[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func categoryFromDriverID(driverID string) string {
	switch strings.TrimSpace(driverID) {
	case driverStock:
		return "stock"
	case driverAdvertising:
		return "advertising"
	case driverAccountSales:
		return "sales"
	case driverDataQuality:
		return "data_quality"
	default:
		if strings.Contains(driverID, "stock") {
			return "stock"
		}
		if strings.Contains(driverID, "ad") {
			return "advertising"
		}
		return "sales"
	}
}

func normalizeHealthReasonsJSONArray(raw json.RawMessage) json.RawMessage {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		reason := jsonStringField(item, "reason")
		if reason == "" {
			reason = jsonStringField(item, "summary")
		}
		category := strings.ToLower(jsonStringField(item, "category"))
		if category == "" {
			category = categoryFromDriverID(jsonStringField(item, "driver_id"))
		}
		out = append(out, map[string]any{
			"driver_id": item["driver_id"],
			"category":  category,
			"title":     item["title"],
			"reason":    reason,
			"fact_ids":  item["fact_ids"],
		})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return encoded
}

func normalizeTopActionsJSONArray(raw json.RawMessage) json.RawMessage {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		reason := jsonStringField(item, "reason")
		if reason == "" {
			reason = jsonStringField(item, "summary")
		}
		nextStep := jsonStringField(item, "next_step")
		if nextStep == "" {
			nextStep = "Проверить детали в кабинете и подтвердить следующий шаг."
		}
		expected := jsonStringField(item, "expected_effect")
		if expected == "" {
			expected = "Снижение операционного риска по направлению."
		}
		priority := strings.ToLower(jsonStringField(item, "priority"))
		if priority == "" {
			priority = "medium"
		}
		category := strings.ToLower(jsonStringField(item, "category"))
		if category == "" {
			category = categoryFromDriverID(jsonStringField(item, "driver_id"))
		}
		out = append(out, map[string]any{
			"driver_id":              item["driver_id"],
			"source_candidate_id":    item["source_candidate_id"],
			"priority":               priority,
			"category":               category,
			"title":                  item["title"],
			"reason":                 reason,
			"next_step":              nextStep,
			"expected_effect":        expected,
			"supporting_metric_refs": item["supporting_metric_refs"],
			"fact_ids":               item["fact_ids"],
		})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return encoded
}

func normalizeCategorySummariesJSONArray(raw json.RawMessage) json.RawMessage {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if score, ok := coerceScore10FromAny(item["category_score"]); ok {
			item["category_score"] = score
		}
		if score, ok := coerceScore100FromAny(item["health_score"]); ok {
			item["health_score"] = score
		}
		out = append(out, item)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return encoded
}

func normalizeAccountHealthJSON(raw json.RawMessage) json.RawMessage {
	var block map[string]any
	if err := json.Unmarshal(raw, &block); err != nil {
		return raw
	}
	if score, ok := coerceScore100FromAny(block["health_score"]); ok {
		block["health_score"] = score
	}
	if drivers, ok := block["risk_drivers"].([]any); ok {
		for i, d := range drivers {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			if impact, ok := coerceIntFromAny(m["impact_score"]); ok {
				m["impact_score"] = impact
			}
			drivers[i] = m
		}
		block["risk_drivers"] = drivers
	}
	encoded, err := json.Marshal(block)
	if err != nil {
		return raw
	}
	return encoded
}

func coerceScore10FromAny(v any) (int32, bool) {
	switch n := v.(type) {
	case float64:
		return int32(n + 0.5), true
	case int:
		return int32(n), true
	case int32:
		return n, true
	case int64:
		return int32(n), true
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return int32(f + 0.5), true
		}
	}
	return 0, false
}

func coerceScore100FromAny(v any) (int32, bool) {
	return coerceScore10FromAny(v)
}

func coerceIntFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n + 0.5), true
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func normalizeRecommendedActionsJSONArray(raw json.RawMessage) json.RawMessage {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		action := jsonStringField(item, "action")
		if action == "" {
			action = firstNonEmpty(jsonStringField(item, "title"), jsonStringField(item, "name"))
		}
		reason := jsonStringField(item, "reason")
		if reason == "" {
			reason = firstNonEmpty(jsonStringField(item, "description"), jsonStringField(item, "summary"))
		}
		out = append(out, map[string]any{
			"priority":            item["priority"],
			"urgency_group":       item["urgency_group"],
			"action":              action,
			"reason":              reason,
			"expected_effect":     item["expected_effect"],
			"source":              firstNonEmpty(jsonStringField(item, "source"), "dashboard_narrative_ai_only"),
			"related_entity_type": item["related_entity_type"],
			"related_entity_id":   item["related_entity_id"],
		})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return encoded
}

func normalizePayloadDataQualityNotesJSON(raw json.RawMessage) json.RawMessage {
	var notes []string
	if err := json.Unmarshal(raw, &notes); err == nil {
		out, _ := json.Marshal(notes)
		return out
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return raw
	}
	for _, item := range arr {
		title := jsonStringField(item, "title")
		summary := jsonStringField(item, "summary")
		switch {
		case title != "" && summary != "":
			notes = append(notes, title+": "+summary)
		case title != "":
			notes = append(notes, title)
		case summary != "":
			notes = append(notes, summary)
		}
	}
	out, err := json.Marshal(notes)
	if err != nil {
		return raw
	}
	return out
}

func normalizeAdEfficiencyNarrativeJSON(raw json.RawMessage) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}
	if checklist, ok := obj["checklist"].([]any); ok {
		normalized := make([]map[string]any, 0, len(checklist))
		for _, item := range checklist {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				normalized = append(normalized, map[string]any{"title": strings.TrimSpace(s), "text": strings.TrimSpace(s)})
				continue
			}
			if m, ok := item.(map[string]any); ok {
				normalized = append(normalized, m)
			}
		}
		obj["checklist"] = normalized
	}
	if strings.TrimSpace(jsonStringField(obj, "status")) == "" {
		obj["status"] = "available"
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

func normalizeStockCoverageNarrativeJSON(raw json.RawMessage) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}
	if actions, ok := obj["actions"].([]any); ok {
		normalized := make([]map[string]any, 0, len(actions))
		for _, item := range actions {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				normalized = append(normalized, map[string]any{"title": strings.TrimSpace(s)})
				continue
			}
			if m, ok := item.(map[string]any); ok {
				normalized = append(normalized, m)
			}
		}
		obj["actions"] = normalized
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

func repairRawDashboardNarrativeJSON(raw string) (string, []string, error) {
	repaired, repairs, err := aijsonrepair.RepairRawJSON(raw)
	if err != nil {
		if err.Error() == "empty json" {
			return "", repairs, fmt.Errorf("empty dashboard narrative json")
		}
		return repaired, repairs, err
	}
	return repaired, repairs, nil
}

func parseDashboardNarrativePayloadWithRepair(raw string) (*dashboardNarrativePayload, []string, error) {
	result := parseDashboardSynthesisPayloadDetailed(raw)
	return result.Payload, result.JSONRepairs, result.ParseError
}

func parseDashboardSynthesisPayloadDetailed(raw string) synthesisParseResult {
	out := synthesisParseResult{ExtractedJSON: strings.TrimSpace(raw)}
	repaired, repairs, err := repairRawDashboardNarrativeJSON(raw)
	out.JSONRepairs = repairs
	out.RepairedJSON = repaired
	if err != nil {
		out.ParseError = err
		return out
	}
	payload, err := parseDashboardNarrativePayload(repaired)
	if err != nil {
		out.ParseError = err
		return out
	}
	out.Payload = payload
	return out
}
