package projection

import (
	"encoding/json"
	"testing"
)

func TestOverviewExactlyThreeKPIsNoAOV(t *testing.T) {
	ov := OverviewV1DTO{SchemaVersion: OverviewV1, KPIs: []KPI{
		{Key: "ORDERED_AMOUNT", Status: "MISSING"},
		{Key: "ORDERED_UNITS", Status: "MISSING"},
		{Key: "AD_SHARE", Status: "MISSING"},
	}}
	if len(ov.KPIs) != 3 {
		t.Fatal(len(ov.KPIs))
	}
	raw, _ := json.Marshal(ov)
	if containsAOV(string(raw)) {
		t.Fatal("AOV must be absent")
	}
	for _, k := range ov.KPIs {
		if k.Status == "MISSING" && k.Amount != nil {
			t.Fatal("Missing != Zero")
		}
	}
}

func containsAOV(s string) bool {
	return len(s) > 0 && (containsWord(s, "AOV") || containsWord(s, "aov"))
}

func containsWord(s, n string) bool {
	return len(s) >= len(n) && (s == n || jsonContains(s, n))
}

func jsonContains(s, n string) bool {
	for i := 0; i+len(n) <= len(s); i++ {
		if s[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
