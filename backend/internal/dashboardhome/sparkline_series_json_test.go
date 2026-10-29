package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSparklinePointJSON_missingDayOmitsValue(t *testing.T) {
	to := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -2)
	v16 := 3.0
	series := []SparklinePoint{
		{Date: "2026-05-15"},
		{Date: "2026-05-16", Value: &v16},
		{Date: "2026-05-17"},
	}
	_ = buildCalendarSparklineSeries(from, to, map[string]float64{"2026-05-16": 3})
	payload, err := json.Marshal(struct {
		Sparkline7D []SparklinePoint `json:"sparkline_7d"`
	}{Sparkline7D: series})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(payload)
	if strings.Contains(raw, `"2026-05-15"`) && strings.Contains(raw, `"value":0`) {
		t.Fatalf("missing day must not serialize as zero: %s", raw)
	}
	if !strings.Contains(raw, `"2026-05-16"`) || !strings.Contains(raw, `"value":3`) {
		t.Fatalf("expected real value: %s", raw)
	}
	var decoded struct {
		Sparkline7D []struct {
			Date  string   `json:"date"`
			Value *float64 `json:"value"`
		} `json:"sparkline_7d"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Sparkline7D) != 3 || decoded.Sparkline7D[0].Value != nil {
		t.Fatalf("missing day value must be nil after unmarshal: %+v", decoded.Sparkline7D[0])
	}
	if decoded.Sparkline7D[1].Value == nil || *decoded.Sparkline7D[1].Value != 3 {
		t.Fatalf("present day: %+v", decoded.Sparkline7D[1])
	}
}
