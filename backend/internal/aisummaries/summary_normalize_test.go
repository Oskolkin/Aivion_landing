package aisummaries

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAndNormalizeSummaryJSON_evidenceObject(t *testing.T) {
	raw := `{
	  "headline":"Категория",
	  "health_score":55,
	  "severity":"warning",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"Кратко",
	  "category_id": 12,
	  "category_name": "Кухня",
	  "key_risk":"Риск дефицита",
	  "recommended_action":"Проверить остатки",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"orders","current_value":4,"comparison_value":12,"delta":"-8","evidence":{"orders_before":12,"orders_after":4},"impact":"negative","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"medium","action":"Проверить остатки","reason":"r","expected_effect":"x","related_entity_type":"category","related_entity_id":"12"}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("expected clean after normalize, got %+v", out.ValidationErrors)
	}
	if out.Payload.KeyFindings[0].Impact != "high" {
		t.Fatalf("impact=%q", out.Payload.KeyFindings[0].Impact)
	}
	if out.Payload.Severity != "medium" {
		t.Fatalf("severity=%q", out.Payload.Severity)
	}
	if !strings.Contains(out.Payload.KeyFindings[0].Evidence, "orders_before") {
		t.Fatalf("evidence=%q", out.Payload.KeyFindings[0].Evidence)
	}
}

func TestParseAndNormalizeSummaryJSON_rejectsMissingHeadline(t *testing.T) {
	raw := baseValidSummaryJSON()
	raw = raw[:len(raw)-1] + `,"category_id":12,"category_name":"Books"}`
	raw = strings.Replace(raw, `"headline":"t"`, `"headline":""`, 1)
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if IsValidationClean(out) {
		t.Fatal("expected missing_headline")
	}
}

func TestParseAndNormalizeSummaryJSON_categoryWhatChangedObjectPasses(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "category_id": 12,
	  "category_name": "Books",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":{"risk":"Риск дефицита","reason":"остаток ниже недельного покрытия"}},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"medium","action":"Проверить остатки","reason":"r","expected_effect":"x","related_entity_type":"category","related_entity_id":"12"}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("expected clean, got %+v", out.ValidationErrors)
	}
	if got := out.Payload.WhatChangedSincePreviousSummary.NewRisks; len(got) != 1 || got[0] != "Риск дефицита - остаток ниже недельного покрытия" {
		t.Fatalf("new_risks=%v", got)
	}
}

func categoryWatchlistDef() watchlistNormalizeDefaults {
	return watchlistNormalizeDefaults{
		summaryType:  "category_daily_summary",
		scopeID:      "17026957",
		categoryName: "Освещение и декор",
	}
}

func TestNormalizeWatchlist_string(t *testing.T) {
	wl := normalizeWatchlist(json.RawMessage(`"Проверить остатки по товарам категории"`), categoryWatchlistDef())
	if len(wl) != 1 {
		t.Fatalf("len=%d", len(wl))
	}
	if wl[0].EntityType != "category" || wl[0].EntityID != "17026957" {
		t.Fatalf("%+v", wl[0])
	}
	if wl[0].Name != "Освещение и декор" || wl[0].Reason == "" {
		t.Fatalf("%+v", wl[0])
	}
}

func TestNormalizeWatchlist_arrayOfStrings(t *testing.T) {
	raw := json.RawMessage(`["Следить за остатками","Проверить рекламу"]`)
	wl := normalizeWatchlist(raw, categoryWatchlistDef())
	if len(wl) != 2 {
		t.Fatalf("len=%d", len(wl))
	}
	if wl[0].Reason == "" || wl[1].Reason == "" {
		t.Fatalf("%+v", wl)
	}
}

func TestNormalizeWatchlist_singleObject(t *testing.T) {
	raw := json.RawMessage(`{"entity_type":"category","reason":"Наблюдать SKU"}`)
	wl := normalizeWatchlist(raw, categoryWatchlistDef())
	if len(wl) != 1 {
		t.Fatalf("len=%d", len(wl))
	}
	if wl[0].EntityID != "17026957" || wl[0].Name != "Освещение и декор" {
		t.Fatalf("%+v", wl[0])
	}
}

func TestNormalizeWatchlist_mixedStringsAndObjects(t *testing.T) {
	raw := json.RawMessage(`["Строка наблюдения",{"entity_type":"sku","entity_id":"99","name":"SKU","reason":"r","metric_snapshot":"m"}]`)
	wl := normalizeWatchlist(raw, categoryWatchlistDef())
	if len(wl) != 2 {
		t.Fatalf("len=%d", len(wl))
	}
	if wl[0].EntityType != "category" {
		t.Fatalf("first=%+v", wl[0])
	}
	if wl[1].EntityType != "sku" {
		t.Fatalf("second=%+v", wl[1])
	}
}

func TestParseAndNormalizeSummaryJSON_watchlistStringPassesValidator(t *testing.T) {
	raw := `{
	  "headline":"Категория",
	  "health_score":55,
	  "severity":"medium",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"Кратко",
	  "category_id": 17026957,
	  "category_name": "Освещение и декор",
	  "key_risk":"Риск дефицита",
	  "recommended_action":"Проверить остатки",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"orders","current_value":4,"comparison_value":12,"delta":"-8","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"medium","action":"Действие","reason":"r","expected_effect":"x","related_entity_type":"category","related_entity_id":"17026957"}],
	  "watchlist": "Проверить остатки по товарам категории",
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "17026957", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	if len(out.Payload.Watchlist) != 1 || out.Payload.Watchlist[0].EntityType != "category" {
		t.Fatalf("watchlist=%+v", out.Payload.Watchlist)
	}
}

func TestParseAndValidateSummaryJSON_preservesNonEmptyImproved(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "what_changed_since_previous_summary":{"improved":["Выросла выручка на 12%"],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	if got := out.Payload.WhatChangedSincePreviousSummary.Improved; len(got) != 1 || got[0] != "Выросла выручка на 12%" {
		t.Fatalf("improved=%v", got)
	}
}

func TestParseAndValidateSummaryJSON_normalizesMissingWhatChangedArraysToEmpty(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "what_changed_since_previous_summary":{},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	wc := out.Payload.WhatChangedSincePreviousSummary
	if wc.Improved == nil || wc.Worsened == nil || wc.UnchangedRisks == nil || wc.ResolvedRisks == nil || wc.NewRisks == nil {
		t.Fatalf("what_changed arrays must be normalized: %+v", wc)
	}
	b, err := json.Marshal(out.Payload)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"improved":[]`,
		`"worsened":[]`,
		`"unchanged_risks":[]`,
		`"resolved_risks":[]`,
		`"new_risks":[]`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload json missing %s in %s", want, s)
		}
	}
}

func TestParseAndValidateSummaryJSON_acceptsFunnelAIAnalysis(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "funnel_ai_analysis":{
	    "status":"available",
	    "short_summary":"Краткий вывод.",
	    "drop_factors":[{"label":"Реклама","evidence_level":"likely","metric":"ДРР 120%","delta":"+12%","explanation":"Сигнал"}],
	    "trend_vs_yesterday":"Ко вчера стало хуже.",
	    "trend_vs_7d":"Ниже среднего за 7 дней.",
	    "related_signals":["Есть слабые кампании."],
	    "recommended_focus":["Проверить кампании без заказов."],
	    "full_analysis":"Полный разбор."
	  },
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	if out.Payload.FunnelAIAnalysis == nil || out.Payload.FunnelAIAnalysis.ShortSummary != "Краткий вывод." {
		t.Fatalf("funnel_ai_analysis=%+v", out.Payload.FunnelAIAnalysis)
	}
	if len(out.Payload.FunnelAIAnalysis.DropFactors) != 1 || out.Payload.FunnelAIAnalysis.DropFactors[0].EvidenceLevel != "likely" {
		t.Fatalf("drop_factors=%+v", out.Payload.FunnelAIAnalysis.DropFactors)
	}
}

func TestParseAndValidateSummaryJSON_missingFunnelAIAnalysisRemainsValid(t *testing.T) {
	out, err := ParseAndValidateSummaryJSON(baseValidSummaryJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	if out.Payload.FunnelAIAnalysis != nil {
		t.Fatalf("expected nil funnel_ai_analysis, got %+v", out.Payload.FunnelAIAnalysis)
	}
}

func TestParseAndValidateSummaryJSON_malformedFunnelAIAnalysisNormalizesSafely(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "funnel_ai_analysis":"unexpected",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("errs=%+v", out.ValidationErrors)
	}
	if out.Payload.FunnelAIAnalysis != nil {
		t.Fatalf("expected malformed funnel_ai_analysis to normalize to nil, got %+v", out.Payload.FunnelAIAnalysis)
	}
}

func TestNormalizeStringListField_acceptsFlexibleShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want []string
	}{
		{name: "strings", raw: []any{"text 1", "text 2"}, want: []string{"text 1", "text 2"}},
		{name: "single string", raw: "text", want: []string{"text"}},
		{name: "null", raw: nil, want: []string{}},
		{name: "object risk reason", raw: map[string]any{"risk": "Риск дефицита", "reason": "остаток ниже недельного покрытия"}, want: []string{"Риск дефицита - остаток ниже недельного покрытия"}},
		{name: "object title description", raw: map[string]any{"title": "Просадка продаж", "description": "заказы ниже среднего"}, want: []string{"Просадка продаж - заказы ниже среднего"}},
		{name: "objects array", raw: []any{map[string]any{"action": "Остановить кампанию", "reason": "расход без заказов"}}, want: []string{"Остановить кампанию - расход без заказов"}},
		{name: "mixed", raw: []any{"text", map[string]any{"risk": "Риск дефицита", "reason": "остаток ниже недельного покрытия"}}, want: []string{"text", "Риск дефицита - остаток ниже недельного покрытия"}},
		{name: "numbers and bool do not panic", raw: []any{15.0, true, false}, want: []string{"15"}},
		{name: "trim empty dedupe", raw: []any{"  text  ", "", "text"}, want: []string{"text"}},
		{name: "control chars", raw: []any{"101\u0016021 ₽", "abc\u0016def"}, want: []string{"101 021 ₽", "abc def"}},
	}

	for _, tc := range cases {
		got := normalizeStringListField(tc.raw)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got=%v want=%v", tc.name, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: got=%v want=%v", tc.name, got, tc.want)
			}
		}
	}
}

func TestNormalizeFlexibleText_acceptsFlexibleShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want string
	}{
		{name: "string", raw: "text", want: "text"},
		{name: "object text", raw: map[string]any{"text": "text"}, want: "text"},
		{name: "object title description", raw: map[string]any{"title": "Просадка продаж", "description": "заказы ниже среднего"}, want: "Просадка продаж - заказы ниже среднего"},
		{name: "object risk reason", raw: map[string]any{"risk": "Риск дефицита", "reason": "остаток ниже недельного покрытия"}, want: "Риск дефицита - остаток ниже недельного покрытия"},
		{name: "null", raw: nil, want: ""},
		{name: "control chars", raw: "20\u001845 ₽", want: "20 845 ₽"},
	}

	for _, tc := range cases {
		if got := normalizeFlexibleText(tc.raw); got != tc.want {
			t.Fatalf("%s: got=%q want=%q", tc.name, got, tc.want)
		}
	}
}

func TestParseAndValidateSummaryJSON_categoryWhatChangedObjectsArrayPasses(t *testing.T) {
	raw := `{
	  "headline":"Категория",
	  "health_score":55,
	  "severity":"medium",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"Кратко",
	  "category_id": 12,
	  "category_name": "Кухня",
	  "what_changed_since_previous_summary":{
	    "improved":["Рост заказов"],
	    "worsened":[{"title":"Просадка продаж","description":"заказы ниже среднего"}],
	    "unchanged_risks":null,
	    "resolved_risks":[],
	    "new_risks":[{"risk":"Риск дефицита","reason":"остаток ниже недельного покрытия"}]
	  },
	  "key_findings":[{"title":"k","metric":"orders","current_value":4,"comparison_value":12,"delta":"-8","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"medium","action":"Проверить остатки","reason":"r","expected_effect":"x","related_entity_type":"category","related_entity_id":"12"}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("expected clean, got %+v", out.ValidationErrors)
	}
	if got := out.Payload.WhatChangedSincePreviousSummary.NewRisks; len(got) != 1 || got[0] != "Риск дефицита - остаток ниже недельного покрытия" {
		t.Fatalf("new_risks=%v", got)
	}
	if got := out.Payload.WhatChangedSincePreviousSummary.Worsened; len(got) != 1 || got[0] != "Просадка продаж - заказы ниже среднего" {
		t.Fatalf("worsened=%v", got)
	}
	if out.Payload.KeyRisk != "Риск дефицита - остаток ниже недельного покрытия" {
		t.Fatalf("key_risk=%q", out.Payload.KeyRisk)
	}
	if out.Payload.RecommendedAction != "Проверить остатки" {
		t.Fatalf("recommended_action=%q", out.Payload.RecommendedAction)
	}
	b, err := json.Marshal(out.Payload)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `"new_risks":[{`) || strings.Contains(s, `"worsened":[{`) {
		t.Fatalf("normalized what_changed still contains raw objects: %s", s)
	}
}

func TestParseAndValidateSummaryJSON_optionalMalformedFieldsDoNotFail(t *testing.T) {
	raw := `{
	  "headline":{"text":"Выручка под давлением"},
	  "health_score":"50",
	  "severity":"warning",
	  "executive_summary":{"summary":"Нужно проверить рекламу."},
	  "key_findings":"single finding",
	  "likely_causes":{"cause":"Причина","evidence":{"x":1}},
	  "recommended_actions":{"action":"Проверить кампании","reason":"Расход без заказов","related_entity_type":"campaign"},
	  "watchlist":{"reason":"Наблюдать"},
	  "data_quality_notes":{"text":"note"}
	}`
	out, err := ParseAndValidateSummaryJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("expected clean, got %+v", out.ValidationErrors)
	}
	if out.Payload.Headline != "Выручка под давлением" {
		t.Fatalf("headline=%q", out.Payload.Headline)
	}
	if out.Payload.ExecutiveSummary != "Нужно проверить рекламу." {
		t.Fatalf("executive_summary=%q", out.Payload.ExecutiveSummary)
	}
}

func TestSanitizeControlText_removesControlChars(t *testing.T) {
	cases := map[string]string{
		"101\u0016021 ₽": "101 021 ₽",
		"20\u001845 ₽":   "20 845 ₽",
		"abc\u0016def":   "abc def",
	}
	for in, want := range cases {
		got := sanitizeControlText(in)
		if got != want {
			t.Fatalf("sanitize %q => %q want %q", in, got, want)
		}
		if strings.ContainsRune(got, '\u0016') {
			t.Fatalf("control char remained in %q", got)
		}
	}
}
