package aisummaries

import "testing"

func baseCategoryJSON(extra string) string {
	raw := `{
	  "headline":"",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"Категория стабильна по выручке.",
	  "category_id": 12,
	  "category_name": {"text":"Books"},
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]`
	if extra != "" {
		raw += "," + extra
	}
	return raw + "}"
}

func TestAcceptSummaryJSON_categoryMissingHeadlineRepaired(t *testing.T) {
	out, err := AcceptSummaryJSON(baseCategoryJSON(""), SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected accepted, hard=%v repairable=%v", out.Repair.HardErrors, out.Repair.RepairableErrors)
	}
	if out.Payload.Headline == "" {
		t.Fatal("expected repaired headline")
	}
}

func TestAcceptSummaryJSON_categoryObjectInsteadOfString(t *testing.T) {
	out, err := AcceptSummaryJSON(baseCategoryJSON(`"unexpected_field":"x"`), SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected accepted: %+v", out.Repair)
	}
	if out.Payload.CategoryName == nil || *out.Payload.CategoryName != "Books" {
		t.Fatalf("category name: %+v", out.Payload.CategoryName)
	}
}

func TestAcceptSummaryJSON_funnelMissingDetailsSectionsRepaired(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "funnel_ai_analysis":{"status":"available","summary":"Воронка проседает на этапе корзины","drop_factors":[{"label":"Корзина","explanation":"Низкая конверсия"}]},
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[{"priority":"low","action":"a","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := AcceptSummaryJSON(raw, SummaryRepairContext{SummaryType: "account_daily_summary"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected accepted: %+v", out.Repair)
	}
	if out.Payload.FunnelAIAnalysis == nil || len(out.Payload.FunnelAIAnalysis.DropFactors) == 0 {
		t.Fatal("expected funnel drop factors")
	}
}

func TestAcceptSummaryJSON_markdownFencedJSON(t *testing.T) {
	raw := "```json\n" + baseCategoryJSON("") + "\n```"
	out, err := AcceptSummaryJSON(raw, SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected accepted: %+v", out.Repair)
	}
}

func TestAcceptSummaryJSON_recommendedActionsForbiddenDropped(t *testing.T) {
	raw := `{
	  "headline":"t",
	  "health_score":50,
	  "severity":"low",
	  "data_richness":"medium",
	  "summary_depth":"normal",
	  "executive_summary":"e",
	  "what_changed_since_previous_summary":{"improved":[],"worsened":[],"unchanged_risks":[],"resolved_risks":[],"new_risks":[]},
	  "key_findings":[{"title":"k","metric":"m","current_value":1,"comparison_value":2,"delta":"d","evidence":"ev","impact":"low","interpretation":"i"}],
	  "likely_causes":[],
	  "recommended_actions":[
	    {"priority":"high","action":"Снизить цену на все SKU","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null},
	    {"priority":"medium","action":"Проверить остатки","reason":"r","expected_effect":"x","related_entity_type":"account","related_entity_id":null}
	  ],
	  "watchlist":[],
	  "linked_alert_ids":[],
	  "linked_recommendation_ids":[],
	  "data_quality_notes":[]
	}`
	out, err := AcceptSummaryJSON(raw, SummaryRepairContext{SummaryType: "account_daily_summary"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatal("expected accepted partial")
	}
	if len(out.Payload.RecommendedActions) != 1 {
		t.Fatalf("expected 1 action kept, got %d", len(out.Payload.RecommendedActions))
	}
	if out.Repair.DroppedActions != 1 {
		t.Fatalf("expected 1 dropped, got %d", out.Repair.DroppedActions)
	}
}

func TestAcceptSummaryJSON_oneCategoryScopeMismatchRepaired(t *testing.T) {
	raw := baseCategoryJSON("")
	out, err := AcceptSummaryJSON(raw, SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "99",
		CategoryName: "Mapped Name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted {
		t.Fatalf("expected repaired scope: %+v", out.Repair)
	}
	if out.Payload.CategoryID == nil || *out.Payload.CategoryID != 99 {
		t.Fatalf("category id: %+v", out.Payload.CategoryID)
	}
}
