package aisummaries

import "testing"

func TestParseAndValidateSummaryJSON_minimalValid(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"low",
	  "summary_depth":"short",
	  "executive_summary":"e",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[{"cause":"c","confidence":"low","evidence":"e","is_hypothesis":true}],
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
		t.Fatalf("expected clean, got %+v", out.ValidationErrors)
	}
}

func TestParseAndValidateSummaryJSON_rejectsBadHealthScore(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":101,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
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
	if IsValidationClean(out) {
		t.Fatal("expected invalid")
	}
}

func TestParseAndValidateSummaryJSON_rejectsBadFunnelEvidenceLevel(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"low",
	  "summary_depth":"short",
	  "executive_summary":"e",
	  "funnel_ai_analysis":{"status":"available","drop_factors":[{"label":"x","evidence_level":"certain"}]},
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[{"cause":"c","confidence":"low","evidence":"e","is_hypothesis":true}],
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
	if IsValidationClean(out) {
		t.Fatalf("expected invalid evidence level, got %+v", out)
	}
}

func baseValidSummaryJSON() string {
	return `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "key_risk":"Риск",
	  "recommended_action":"Действие",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
}

func TestParseAndValidateSummaryJSONWithScope_accountRejectsCategoryFields(t *testing.T) {
	t.Parallel()
	raw := baseValidSummaryJSON()
	raw = raw[:len(raw)-1] + `,"category_id": 12, "category_name": "n"}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "account_daily_summary", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if IsValidationClean(out) {
		t.Fatal("expected scope errors")
	}
}

func TestParseAndValidateSummaryJSONWithScope_categoryRequiresAlignedIDs(t *testing.T) {
	t.Parallel()
	raw := baseValidSummaryJSON()
	raw = raw[:len(raw)-1] + `,"category_id": 12, "category_name": "Books"}`
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !IsValidationClean(out) {
		t.Fatalf("expected clean, got %+v", out.ValidationErrors)
	}

	out2, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "99", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if IsValidationClean(out2) {
		t.Fatal("expected mismatch error")
	}
}

func TestParseAndValidateSummaryJSONWithScope_lowDataRejectsNonShort(t *testing.T) {
	t.Parallel()
	raw := baseValidSummaryJSON()
	raw = raw[:len(raw)-1] + `,"category_id": 12, "category_name": "Books"}`
	// richness medium + depth normal while inferred low-data category
	out, err := ParseAndValidateSummaryJSONWithScope(raw, "category_daily_summary", "12", "low")
	if err != nil {
		t.Fatal(err)
	}
	if IsValidationClean(out) {
		t.Fatal("expected low-data depth errors")
	}
}

func TestAcceptSummaryJSON_categoryMissingKeyRiskRepaired(t *testing.T) {
	t.Parallel()
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "category_id": 12,
	  "category_name": "Books",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := AcceptSummaryJSON(raw, SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected repaired acceptance, got %+v", out.Repair)
	}
	if out.Payload.KeyRisk == "" || out.Payload.RecommendedAction == "" {
		t.Fatal("expected key_risk and recommended_action filled")
	}
}
