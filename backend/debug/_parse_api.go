package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	raw, _ := os.ReadFile("debug/_api_dash_442.json")
	if len(raw) == 0 {
		raw, _ = os.ReadFile("debug/_api_dash_442.json")
	}
	// try both paths
	paths := []string{"debug/_api_dash_442.json", "debug\\_api_dash_442.json", "debug/api_dash_442.json"}
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			raw = b
			break
		}
	}
	// list dir
	entries, _ := os.ReadDir("debug")
	for _, e := range entries {
		if len(e.Name()) > 10 && e.Name()[0] == '_' && (contains(e.Name(), "dash") || contains(e.Name(), "act")) {
			fmt.Println("file", e.Name(), e.IsDir())
		}
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	fmt.Println("keys", len(m), "bytes", len(raw))
	fmt.Println("content_status", m["content_status"])
	fmt.Println("effective_metric_date", m["effective_metric_date"])
	fmt.Println("historical_analysis", m["historical_analysis"])
	ai, _ := m["ai_diagnostics"].(map[string]any)
	if ai == nil {
		fmt.Println("no ai_diagnostics")
		return
	}
	cats, _ := ai["category_summaries"].([]any)
	fmt.Println("categories", len(cats))
	sku, _ := ai["sku_focus"].(map[string]any)
	od, _ := sku["order_drop"].([]any)
	fmt.Println("order_drop", len(od))
	if len(od) > 0 {
		c0, _ := od[0].(map[string]any)
		sp, _ := c0["sparkline_7d"].([]any)
		has := 0
		for _, p := range sp {
			pt, _ := p.(map[string]any)
			if pt["has_data"] == true {
				has++
			}
		}
		fmt.Println("spark_len", len(sp), "has", has)
	}
	ad, _ := ai["ad_efficiency"].(map[string]any)
	nar, _ := ad["narrative"].(map[string]any)
	cl, _ := nar["checklist"].([]any)
	fmt.Println("checklist", len(cl))
	fa, _ := ai["funnel_analysis"].(map[string]any)
	faai, _ := fa["ai_analysis"].(map[string]any)
	fmt.Println("funnel_short", faai["short_summary"])
	rd, _ := ai["risk_distribution"].(map[string]any)
	fmt.Println("risk_groups", len(rd["groups"].([]any)))
	notes, _ := m["data_quality_notes"].([]any)
	fmt.Println("notes", notes)
}
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
