package dashboardhome

import "testing"

func TestRepairRawDashboardNarrativeJSON_MarkdownFence(t *testing.T) {
	raw := "```json\n{\"headline\":\"h\",\"executive_summary\":\"e e e\",\"improvement_summary\":\"i\"}\n```"
	repaired, repairs, err := repairRawDashboardNarrativeJSON(raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if repaired == "" || len(repairs) == 0 {
		t.Fatalf("repairs=%v repaired=%q", repairs, repaired)
	}
}

func TestRepairRawDashboardNarrativeJSON_TrailingComma(t *testing.T) {
	raw := `{"headline":"h","executive_summary":"e e e","improvement_summary":"i","top_actions":[],}`
	repaired, repairs, err := repairRawDashboardNarrativeJSON(raw)
	if err != nil {
		t.Fatalf("err=%v repairs=%v", err, repairs)
	}
	if repaired == "" {
		t.Fatal("empty repaired json")
	}
}
