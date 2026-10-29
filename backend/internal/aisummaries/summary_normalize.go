package aisummaries

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type watchlistNormalizeDefaults struct {
	summaryType  string
	scopeID      string
	categoryName string
}

type watchlistEntry struct {
	EntityType     string `json:"entity_type"`
	EntityID       string `json:"entity_id"`
	Name           string `json:"name"`
	Reason         string `json:"reason"`
	MetricSnapshot string `json:"metric_snapshot"`
}

func parseAndNormalizeSummaryJSON(raw string) (*SummaryPayload, error) {
	return parseAndNormalizeSummaryJSONWithScope(raw, "", "", "")
}

func parseAndNormalizeSummaryJSONWithScope(raw, summaryType, scopeID, categoryName string) (*SummaryPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errEmptySummaryJSON
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}

	if categoryName == "" {
		categoryName = normalizeFlexibleText(root["category_name"])
	}
	def := watchlistNormalizeDefaults{
		summaryType:  summaryType,
		scopeID:      scopeID,
		categoryName: categoryName,
	}
	normalized := normalizeSummaryRoot(root, def)
	normalizedBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}

	var payload SummaryPayload
	if err := json.Unmarshal(normalizedBytes, &payload); err != nil {
		return nil, err
	}
	payload.WhatChangedSincePreviousSummary.Improved = normalizeStringSlice(payload.WhatChangedSincePreviousSummary.Improved)
	payload.WhatChangedSincePreviousSummary.Worsened = normalizeStringSlice(payload.WhatChangedSincePreviousSummary.Worsened)
	payload.WhatChangedSincePreviousSummary.UnchangedRisks = normalizeStringSlice(payload.WhatChangedSincePreviousSummary.UnchangedRisks)
	payload.WhatChangedSincePreviousSummary.ResolvedRisks = normalizeStringSlice(payload.WhatChangedSincePreviousSummary.ResolvedRisks)
	payload.WhatChangedSincePreviousSummary.NewRisks = normalizeStringSlice(payload.WhatChangedSincePreviousSummary.NewRisks)
	payload.KeyFindings = normalizeStructSlice(payload.KeyFindings)
	payload.LikelyCauses = normalizeStructSlice(payload.LikelyCauses)
	payload.RecommendedActions = normalizeStructSlice(payload.RecommendedActions)
	payload.Watchlist = normalizeStructSlice(payload.Watchlist)
	if payload.FunnelAIAnalysis != nil {
		payload.FunnelAIAnalysis.RelatedSignals = normalizeStringSlice(payload.FunnelAIAnalysis.RelatedSignals)
		payload.FunnelAIAnalysis.RecommendedFocus = normalizeStringSlice(payload.FunnelAIAnalysis.RecommendedFocus)
	}
	return &payload, nil
}

var errEmptySummaryJSON = &parseError{msg: "empty json"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func normalizeSummaryRoot(root map[string]any, wlDef watchlistNormalizeDefaults) map[string]any {
	if root == nil {
		root = map[string]any{}
	}

	whatChanged := asMap(root["what_changed_since_previous_summary"])
	normalized := map[string]any{
		"headline":           normalizeFlexibleText(root["headline"]),
		"health_score":       normalizeIntField(root["health_score"]),
		"severity":           normalizeSeverity(normalizeFlexibleText(root["severity"])),
		"data_richness":      normalizeDataRichness(normalizeFlexibleText(root["data_richness"])),
		"summary_depth":      normalizeSummaryDepth(normalizeFlexibleText(root["summary_depth"])),
		"executive_summary":  normalizeFlexibleText(root["executive_summary"]),
		"category_id":        normalizeOptionalInt64Field(root["category_id"]),
		"category_name":      normalizeOptionalTextField(root["category_name"]),
		"key_risk":           normalizeFlexibleText(root["key_risk"]),
		"recommended_action": normalizeFlexibleText(root["recommended_action"]),
		"what_changed_since_previous_summary": map[string]any{
			"improved":        normalizeStringListField(whatChanged["improved"]),
			"worsened":        normalizeStringListField(whatChanged["worsened"]),
			"unchanged_risks": normalizeStringListField(whatChanged["unchanged_risks"]),
			"resolved_risks":  normalizeStringListField(whatChanged["resolved_risks"]),
			"new_risks":       normalizeStringListField(whatChanged["new_risks"]),
		},
		"key_findings":              normalizeKeyFindings(root["key_findings"]),
		"likely_causes":             normalizeLikelyCauses(root["likely_causes"]),
		"recommended_actions":       normalizeRecommendedActions(root["recommended_actions"]),
		"watchlist":                 normalizeWatchlist(root["watchlist"], wlDef),
		"linked_alert_ids":          normalizeInt64List(root["linked_alert_ids"]),
		"linked_recommendation_ids": normalizeInt64List(root["linked_recommendation_ids"]),
		"data_quality_notes":        normalizeStringListField(root["data_quality_notes"]),
	}

	if funnel := normalizeFunnelAIAnalysis(root["funnel_ai_analysis"]); funnel != nil {
		normalized["funnel_ai_analysis"] = funnel
	}

	if normalized["key_risk"] == "" {
		keyRisk := firstNonEmptyString(
			normalizeFlexibleText(root["key_risk"]),
			firstString(normalizeStringListField(whatChanged["new_risks"])),
			firstLikelyCause(normalized["likely_causes"]),
		)
		if keyRisk != "" {
			normalized["key_risk"] = keyRisk
		}
	}

	if normalized["recommended_action"] == "" {
		action := firstNonEmptyString(
			normalizeFlexibleText(root["recommended_action"]),
			firstRecommendedActionText(normalized["recommended_actions"]),
		)
		if action != "" {
			normalized["recommended_action"] = action
		}
	}

	return normalized
}

func normalizeStringListField(raw any) []string {
	items := flattenFlexibleStrings(raw)
	if len(items) == 0 {
		return []string{}
	}

	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = sanitizeControlText(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func flattenFlexibleStrings(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return []string{}
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := sanitizeControlText(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, flattenFlexibleStrings(item)...)
		}
		return out
	case string:
		if s := sanitizeControlText(v); s != "" {
			return []string{s}
		}
		return []string{}
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			return []string{}
		}
		return flattenFlexibleStrings(decoded)
	case map[string]any:
		if s := objectToText(v); s != "" {
			return []string{s}
		}
		return []string{}
	case bool:
		return []string{}
	case float64:
		return []string{sanitizeControlText(formatNumericValue(v))}
	case int, int8, int16, int32, int64:
		return []string{sanitizeControlText(fmt.Sprintf("%v", v))}
	case uint, uint8, uint16, uint32, uint64:
		return []string{sanitizeControlText(fmt.Sprintf("%v", v))}
	default:
		if s := normalizeFlexibleText(v); s != "" {
			return []string{s}
		}
		return []string{}
	}
}

func normalizeFlexibleText(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return sanitizeControlText(v)
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			return ""
		}
		return normalizeFlexibleText(decoded)
	case []any:
		return sanitizeControlText(strings.Join(normalizeStringListField(v), " "))
	case []string:
		return sanitizeControlText(strings.Join(normalizeStringListField(v), " "))
	case map[string]any:
		return objectToText(v)
	case float64:
		return sanitizeControlText(formatNumericValue(v))
	case int, int8, int16, int32, int64:
		return sanitizeControlText(fmt.Sprintf("%v", v))
	case uint, uint8, uint16, uint32, uint64:
		return sanitizeControlText(fmt.Sprintf("%v", v))
	case bool:
		return ""
	default:
		return ""
	}
}

func objectToText(raw map[string]any) string {
	if raw == nil {
		return ""
	}

	primary := firstKnownObjectText(raw, []string{
		"text", "summary", "headline", "title", "risk", "issue", "problem", "action", "recommendation", "name", "label",
	})
	secondary := firstKnownObjectText(raw, []string{
		"reason", "description", "details", "detail", "explanation", "metric", "value", "context",
	})

	if primary != "" && secondary != "" && primary != secondary {
		return sanitizeControlText(primary + " - " + secondary)
	}
	if primary != "" {
		return sanitizeControlText(primary)
	}
	if secondary != "" {
		return sanitizeControlText(secondary)
	}

	values := collectObjectTextValues(raw)
	switch len(values) {
	case 0:
		return ""
	case 1:
		return sanitizeControlText(values[0])
	default:
		return sanitizeControlText(values[0] + " - " + values[1])
	}
}

func firstKnownObjectText(raw map[string]any, keys []string) string {
	for _, key := range keys {
		if text := normalizeFlexibleText(raw[key]); text != "" {
			return text
		}
	}
	return ""
}

func collectObjectTextValues(raw map[string]any) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, key := range keys {
		text := normalizeFlexibleText(raw[key])
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
		if len(out) >= 2 {
			break
		}
	}
	return out
}

func normalizeIntField(raw any) int {
	if v, ok := parseInt(raw); ok {
		return v
	}
	return -1
}

func normalizeOptionalInt64Field(raw any) *int64 {
	if v, ok := parseInt64(raw); ok {
		return &v
	}
	return nil
}

func normalizeOptionalTextField(raw any) *string {
	if text := normalizeFlexibleText(raw); text != "" {
		return &text
	}
	return nil
}

func parseInt(raw any) (int, bool) {
	switch v := raw.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i), true
		}
	case string:
		digits := strings.TrimSpace(v)
		if digits == "" {
			return 0, false
		}
		i, err := strconv.Atoi(digits)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func parseInt64(raw any) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i, true
		}
	case string:
		digits := strings.TrimSpace(v)
		if digits == "" {
			return 0, false
		}
		i, err := strconv.ParseInt(digits, 10, 64)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func normalizeInt64List(raw any) []int64 {
	switch v := raw.(type) {
	case nil:
		return []int64{}
	case []any:
		out := make([]int64, 0, len(v))
		for _, item := range v {
			if parsed, ok := parseInt64(item); ok {
				out = append(out, parsed)
			}
		}
		return out
	case []int64:
		return normalizeInt64Slice(v)
	default:
		if parsed, ok := parseInt64(raw); ok {
			return []int64{parsed}
		}
		return []int64{}
	}
}

func normalizeKeyFindings(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			text := normalizeFlexibleText(item)
			if text == "" {
				continue
			}
			out = append(out, map[string]any{
				"title":            text,
				"metric":           "",
				"current_value":    nil,
				"comparison_value": nil,
				"delta":            "",
				"evidence":         text,
				"impact":           "medium",
				"interpretation":   "",
			})
			continue
		}
		entry := map[string]any{
			"title":            normalizeFlexibleText(obj["title"]),
			"metric":           normalizeFlexibleText(obj["metric"]),
			"current_value":    normalizeComparableValue(obj["current_value"]),
			"comparison_value": normalizeComparableValue(obj["comparison_value"]),
			"delta":            normalizeFlexibleText(obj["delta"]),
			"evidence":         normalizeEvidenceText(obj["evidence"]),
			"impact":           normalizeImpact(normalizeFlexibleText(obj["impact"])),
			"interpretation":   normalizeFlexibleText(obj["interpretation"]),
		}
		if entry["title"] == "" && entry["evidence"] == "" && entry["interpretation"] == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeLikelyCauses(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			text := normalizeFlexibleText(item)
			if text == "" {
				continue
			}
			out = append(out, map[string]any{
				"cause":         text,
				"confidence":    "low",
				"evidence":      text,
				"is_hypothesis": true,
			})
			continue
		}
		entry := map[string]any{
			"cause":         normalizeFlexibleText(obj["cause"]),
			"confidence":    normalizeConfidence(normalizeFlexibleText(obj["confidence"])),
			"evidence":      normalizeEvidenceText(obj["evidence"]),
			"is_hypothesis": normalizeBool(obj["is_hypothesis"]),
		}
		if entry["cause"] == "" && entry["evidence"] == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeRecommendedActions(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			text := normalizeFlexibleText(item)
			if text == "" {
				continue
			}
			out = append(out, map[string]any{
				"priority":            "medium",
				"action":              text,
				"reason":              "",
				"expected_effect":     "",
				"related_entity_type": "account",
				"related_entity_id":   nil,
			})
			continue
		}
		entry := map[string]any{
			"priority":            normalizePriority(normalizeFlexibleText(obj["priority"])),
			"action":              normalizeFlexibleText(obj["action"]),
			"reason":              normalizeFlexibleText(obj["reason"]),
			"expected_effect":     normalizeFlexibleText(obj["expected_effect"]),
			"related_entity_type": normalizeRelatedEntityType(normalizeFlexibleText(obj["related_entity_type"])),
			"related_entity_id":   normalizeOptionalTextField(obj["related_entity_id"]),
		}
		if entry["action"] == "" {
			entry["action"] = firstNonEmptyString(
				normalizeFlexibleText(obj["recommendation"]),
				normalizeFlexibleText(obj["title"]),
			)
		}
		if entry["reason"] == "" {
			entry["reason"] = normalizeFlexibleText(obj["description"])
		}
		if entry["action"] == "" {
			continue
		}
		if entry["priority"] == "" {
			entry["priority"] = "medium"
		}
		if entry["related_entity_type"] == "" {
			entry["related_entity_type"] = "account"
		}
		out = append(out, entry)
	}
	return out
}

func normalizeFunnelAIAnalysis(raw any) map[string]any {
	obj := asMap(raw)
	if obj == nil {
		return nil
	}

	entry := map[string]any{
		"status":                 normalizeFunnelAIAnalysisStatus(normalizeFlexibleText(obj["status"])),
		"short_summary":          normalizeFlexibleText(obj["short_summary"]),
		"trend_vs_yesterday":     normalizeFlexibleText(obj["trend_vs_yesterday"]),
		"trend_vs_7d":            normalizeFlexibleText(obj["trend_vs_7d"]),
		"related_signals":        normalizeStringListField(obj["related_signals"]),
		"recommended_focus":      normalizeStringListField(obj["recommended_focus"]),
		"full_analysis":          normalizeFlexibleText(obj["full_analysis"]),
		"summary":                normalizeFlexibleText(obj["summary"]),
		"weakest_stage":          normalizeFunnelWeakestStage(obj["weakest_stage"]),
		"main_conclusion":        normalizeFlexibleText(obj["main_conclusion"]),
		"action_items":           normalizeStringListField(obj["action_items"]),
		"comparisons":            normalizeFunnelComparisons(obj["comparisons"]),
		"comparisons_commentary": normalizeFunnelComparisonsCommentary(obj["comparisons_commentary"]),
		"drop_factors":           normalizeFunnelDropFactors(obj["drop_factors"], obj["influence_factors"]),
		"check_directions":       normalizeFunnelTextItems(obj["check_directions"]),
		"details_sections":       normalizeFunnelDetailsSections(obj["details_sections"]),
		"influence_factors":      normalizeFunnelTextItems(obj["influence_factors"]),
		"details":                normalizeFlexibleText(obj["details"]),
	}

	if entry["short_summary"] == "" {
		entry["short_summary"] = firstNonEmptyString(
			normalizeFlexibleText(obj["summary"]),
			normalizeFlexibleText(obj["text"]),
		)
	}

	allEmpty := true
	for _, key := range []string{"status", "short_summary", "trend_vs_yesterday", "trend_vs_7d", "full_analysis", "summary", "details"} {
		if s, ok := entry[key].(string); ok && s != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty &&
		len(entry["related_signals"].([]string)) == 0 &&
		len(entry["recommended_focus"].([]string)) == 0 &&
		len(entry["influence_factors"].([]map[string]any)) == 0 &&
		len(entry["drop_factors"].([]map[string]any)) == 0 &&
		len(entry["check_directions"].([]map[string]any)) == 0 &&
		len(entry["details_sections"].([]map[string]any)) == 0 {
		return nil
	}
	return entry
}

func normalizeFunnelComparisonsCommentary(raw any) map[string]any {
	obj := asMap(raw)
	if obj == nil {
		return nil
	}
	entry := map[string]any{
		"previous_period":   normalizeFlexibleText(obj["previous_period"]),
		"seven_day_average": normalizeFlexibleText(obj["seven_day_average"]),
	}
	if entry["previous_period"] == "" && entry["seven_day_average"] == "" {
		return nil
	}
	return entry
}

func normalizeFunnelDropFactors(primary any, legacyInfluence any) []map[string]any {
	out := normalizeFunnelDropFactorItems(primary)
	if len(out) > 0 {
		return out
	}
	legacy := normalizeFunnelTextItems(legacyInfluence)
	for _, it := range legacy {
		out = append(out, map[string]any{
			"key":         it["key"],
			"label":       it["label"],
			"metric":      "",
			"explanation": it["text"],
			"tone":        it["tone"],
		})
	}
	return out
}

func normalizeFunnelDropFactorItems(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			continue
		}
		entry := map[string]any{
			"key":            normalizeFlexibleText(obj["key"]),
			"label":          normalizeFlexibleText(obj["label"]),
			"group":          normalizeFlexibleText(obj["group"]),
			"evidence_level": normalizeFunnelEvidenceLevel(normalizeFlexibleText(obj["evidence_level"])),
			"metric":         normalizeFlexibleText(obj["metric"]),
			"delta":          normalizeFlexibleText(obj["delta"]),
			"explanation":    normalizeFlexibleText(firstNonEmptyString(normalizeFlexibleText(obj["explanation"]), normalizeFlexibleText(obj["text"]))),
			"limitation":     normalizeFlexibleText(obj["limitation"]),
			"tone":           normalizeFunnelTone(normalizeFlexibleText(obj["tone"])),
			"evidence_key":   normalizeFlexibleText(obj["evidence_key"]),
		}
		if entry["label"] == "" && entry["explanation"] == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeFunnelDetailsSections(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			continue
		}
		title := normalizeFlexibleText(obj["title"])
		sectionItems := normalizeFunnelDetailsSectionItems(obj["items"])
		if title == "" && len(sectionItems) == 0 {
			continue
		}
		out = append(out, map[string]any{"title": title, "items": sectionItems})
	}
	return out
}

func normalizeFunnelDetailsSectionItems(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			continue
		}
		text := normalizeFlexibleText(obj["text"])
		metrics := normalizeStringListField(obj["metrics"])
		if text == "" && len(metrics) == 0 {
			continue
		}
		out = append(out, map[string]any{"text": text, "metrics": metrics})
	}
	return out
}

func normalizeFunnelWeakestStage(raw any) map[string]any {
	obj := asMap(raw)
	if obj == nil {
		return nil
	}
	reason := firstNonEmptyString(
		normalizeFlexibleText(obj["explanation"]),
		normalizeFlexibleText(obj["reason"]),
	)
	entry := map[string]any{
		"status":         normalizeFunnelWeakestStageStatus(normalizeFlexibleText(obj["status"])),
		"key":            normalizeFlexibleText(obj["key"]),
		"label":          normalizeFlexibleText(obj["label"]),
		"metric":         normalizeFlexibleText(obj["metric"]),
		"value":          normalizeFlexibleText(obj["value"]),
		"explanation":    reason,
		"reason":         reason,
		"delta_previous": normalizeFlexibleText(obj["delta_previous"]),
		"delta_7d":       normalizeFlexibleText(obj["delta_7d"]),
		"tone":           normalizeFunnelTone(normalizeFlexibleText(obj["tone"])),
	}
	if entry["status"] == "" && entry["label"] == "" && entry["reason"] == "" {
		return nil
	}
	return entry
}

func normalizeFunnelComparisons(raw any) map[string]any {
	obj := asMap(raw)
	if obj == nil {
		return map[string]any{"previous_period": []map[string]any{}, "seven_day_average": []map[string]any{}}
	}
	return map[string]any{
		"previous_period":    normalizeFunnelComparisonItems(obj["previous_period"]),
		"seven_day_average":  normalizeFunnelComparisonItems(obj["seven_day_average"]),
		"stage_metrics":      normalizeFunnelComparisonItems(obj["stage_metrics"]),
		"conversion_metrics": normalizeFunnelComparisonItems(obj["conversion_metrics"]),
		"business_metrics":   normalizeFunnelComparisonItems(obj["business_metrics"]),
		"context_signals":    normalizeFunnelComparisonItems(obj["context_signals"]),
	}
}

func normalizeFunnelComparisonItems(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			continue
		}
		entry := map[string]any{
			"key":   normalizeFlexibleText(obj["key"]),
			"label": normalizeFlexibleText(obj["label"]),
			"value": normalizeFlexibleText(obj["value"]),
			"delta": normalizeFlexibleText(obj["delta"]),
			"tone":  normalizeFunnelTone(normalizeFlexibleText(obj["tone"])),
		}
		if entry["key"] == "" && entry["label"] == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeFunnelTextItems(raw any) []map[string]any {
	items := asSliceOrSingle(raw)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj := asMap(item)
		if obj == nil {
			text := normalizeFlexibleText(item)
			if text == "" {
				continue
			}
			out = append(out, map[string]any{"text": text, "tone": "neutral"})
			continue
		}
		entry := map[string]any{
			"key":         normalizeFlexibleText(obj["key"]),
			"label":       normalizeFlexibleText(obj["label"]),
			"priority":    normalizeFlexibleText(obj["priority"]),
			"data_needed": normalizeFlexibleText(obj["data_needed"]),
			"reason":      normalizeFlexibleText(obj["reason"]),
			"text":        normalizeFlexibleText(obj["text"]),
			"tone":        normalizeFunnelTone(normalizeFlexibleText(obj["tone"])),
		}
		if entry["text"] == "" && entry["label"] == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func normalizeWatchlist(raw any, def watchlistNormalizeDefaults) []watchlistEntry {
	items := asSlice(raw)
	if len(items) == 0 {
		if obj := asMap(raw); obj != nil {
			return []watchlistEntry{finalizeWatchlistEntry(watchlistEntry{
				EntityType:     normalizeFlexibleText(obj["entity_type"]),
				EntityID:       normalizeFlexibleText(obj["entity_id"]),
				Name:           normalizeFlexibleText(obj["name"]),
				Reason:         normalizeFlexibleText(obj["reason"]),
				MetricSnapshot: normalizeFlexibleText(obj["metric_snapshot"]),
			}, def)}
		}
		if text := normalizeFlexibleText(raw); text != "" {
			return []watchlistEntry{watchlistFromString(text, def)}
		}
		return []watchlistEntry{}
	}

	out := make([]watchlistEntry, 0, len(items))
	for _, item := range items {
		if text := normalizeFlexibleText(item); text != "" && asMap(item) == nil {
			out = append(out, watchlistFromString(text, def))
			continue
		}
		obj := asMap(item)
		if obj == nil {
			continue
		}
		out = append(out, finalizeWatchlistEntry(watchlistEntry{
			EntityType:     normalizeFlexibleText(obj["entity_type"]),
			EntityID:       normalizeFlexibleText(obj["entity_id"]),
			Name:           normalizeFlexibleText(obj["name"]),
			Reason:         firstNonEmptyString(normalizeFlexibleText(obj["reason"]), objectToText(obj)),
			MetricSnapshot: normalizeFlexibleText(obj["metric_snapshot"]),
		}, def))
	}
	return out
}

func watchlistFromString(reason string, def watchlistNormalizeDefaults) watchlistEntry {
	return finalizeWatchlistEntry(watchlistEntry{Reason: reason}, def)
}

func finalizeWatchlistEntry(e watchlistEntry, def watchlistNormalizeDefaults) watchlistEntry {
	entityType := strings.TrimSpace(e.EntityType)
	if entityType == "" {
		entityType = defaultWatchlistEntityType(def)
	}
	entityType = normalizeWatchlistEntityType(entityType)

	entityID := sanitizeControlText(e.EntityID)
	name := sanitizeControlText(e.Name)
	reason := sanitizeControlText(e.Reason)
	metricSnapshot := sanitizeControlText(e.MetricSnapshot)

	if def.summaryType == "category_daily_summary" {
		if entityID == "" && def.scopeID != "" {
			entityID = def.scopeID
		}
		if name == "" && def.categoryName != "" {
			name = def.categoryName
		}
		if entityType == "" || entityType == "metric" {
			entityType = "category"
		}
	}

	if reason == "" {
		reason = "Требует наблюдения"
	}
	return watchlistEntry{
		EntityType:     entityType,
		EntityID:       entityID,
		Name:           name,
		Reason:         reason,
		MetricSnapshot: metricSnapshot,
	}
}

func normalizeComparableValue(raw any) any {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if s := sanitizeControlText(v); s != "" {
			return s
		}
		return nil
	case float64:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		return v
	case []any:
		values := normalizeStringListField(v)
		if len(values) == 0 {
			return nil
		}
		return strings.Join(values, "; ")
	case map[string]any:
		if text := objectToText(v); text != "" {
			return text
		}
		return nil
	default:
		return nil
	}
}

func normalizeEvidenceText(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return sanitizeControlText(v)
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			return ""
		}
		return normalizeEvidenceText(decoded)
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return normalizeFlexibleText(v)
		}
		return sanitizeControlText(string(b))
	default:
		return normalizeFlexibleText(v)
	}
}

func normalizeBool(raw any) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func asMap(raw any) map[string]any {
	switch v := raw.(type) {
	case map[string]any:
		return v
	case json.RawMessage:
		var out map[string]any
		if err := json.Unmarshal(v, &out); err == nil {
			return out
		}
	}
	return nil
}

func asSlice(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case json.RawMessage:
		var out []any
		if err := json.Unmarshal(v, &out); err == nil {
			return out
		}
	}
	return nil
}

func asSliceOrSingle(raw any) []any {
	switch raw.(type) {
	case nil:
		return []any{}
	case []any, []string, json.RawMessage:
		if items := asSlice(raw); items != nil {
			return items
		}
	}
	if items := asSlice(raw); len(items) > 0 {
		return items
	}
	return []any{raw}
}

func firstLikelyCause(raw any) string {
	items, ok := raw.([]map[string]any)
	if !ok || len(items) == 0 {
		return ""
	}
	return normalizeFlexibleText(items[0]["cause"])
}

func firstRecommendedActionText(raw any) string {
	items, ok := raw.([]map[string]any)
	if !ok || len(items) == 0 {
		return ""
	}
	action := normalizeFlexibleText(items[0]["action"])
	reason := normalizeFlexibleText(items[0]["reason"])
	return firstNonEmptyString(action, reason)
}

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = sanitizeControlText(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatNumericValue(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func sanitizeControlText(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i, r := range runes {
		if r >= 0 && r < 32 {
			prevDigit := i > 0 && unicode.IsDigit(runes[i-1])
			nextDigits := 0
			for j := i + 1; j < len(runes) && unicode.IsDigit(runes[j]); j++ {
				nextDigits++
			}
			b.WriteRune(' ')
			if prevDigit && nextDigits == 2 {
				lowNibble := int(r) & 0xF
				if lowNibble > 0 && lowNibble <= 9 {
					b.WriteRune(rune('0' + lowNibble))
				}
			}
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func normalizeStringSlice[T ~string](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func normalizeInt64Slice(in []int64) []int64 {
	if in == nil {
		return []int64{}
	}
	return in
}

func normalizeStructSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func defaultWatchlistEntityType(def watchlistNormalizeDefaults) string {
	if def.summaryType == "category_daily_summary" {
		return "category"
	}
	return "metric"
}

func normalizeWatchlistEntityType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "product", "sku_item", "товар":
		return "sku"
	case "cat", "категория":
		return "category"
	case "ad", "campaign_ad", "реклама":
		return "campaign"
	case "показатель", "metrics":
		return "metric"
	default:
		return s
	}
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "normal", "ok", "info", "none":
		return "low"
	case "warning", "warn", "moderate":
		return "medium"
	case "danger", "severe", "urgent":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizeDataRichness(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "minimal", "sparse":
		return "low"
	case "moderate", "average", "":
		return "medium"
	case "rich", "full":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizeSummaryDepth(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "brief", "shallow":
		return "short"
	case "standard", "regular", "":
		return "normal"
	case "detailed", "full":
		return "deep"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizeImpact(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "severe", "negative", "bad", "high_negative":
		return "high"
	case "positive", "good", "low_positive":
		return "low"
	case "neutral", "mixed", "moderate_impact", "":
		return "medium"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizeConfidence(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "very_low", "":
		return "low"
	case "very_high":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizePriority(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "urgent", "severe":
		return "critical"
	case "":
		return "medium"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func normalizeRelatedEntityType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "product", "sku_item":
		return "sku"
	case "ad", "campaign_ad":
		return "campaign"
	case "":
		return ""
	default:
		return s
	}
}

func normalizeFunnelAIAnalysisStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "available", "not_enough_data", "source_not_available", "source_pending", "error":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func normalizeFunnelWeakestStageStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "available", "stable", "unavailable":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func normalizeFunnelTone(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "negative", "warning", "neutral", "positive":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return "neutral"
	}
}

func normalizeFunnelEvidenceLevel(s string) string {
	level := strings.ToLower(strings.TrimSpace(s))
	switch level {
	case "confirmed", "likely", "hypothesis", "limited":
		return level
	default:
		return level
	}
}
