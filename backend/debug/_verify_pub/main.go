package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	// Old pub 41 must remain published and not mutated to new hashes.
	var status41, pack41, out41, bundle41, superseded string
	var run41 int64
	err = conn.QueryRow(ctx, `
		SELECT status, run_id, COALESCE(pack_hash,''), COALESCE(output_hash,''), COALESCE(bundle_hash,''),
		       COALESCE(superseded_at::text,'')
		FROM unified_analysis_publications WHERE id = 41`).Scan(&status41, &run41, &pack41, &out41, &bundle41, &superseded)
	if err != nil {
		panic(err)
	}
	fmt.Printf("OLD pub41 status=%s run=%d superseded_at=%q pack=%s output=%s bundle=%s\n",
		status41, run41, superseded, trunc(pack41), trunc(out41), trunc(bundle41))

	var status47, pack47, out47, bundle47, prompt, schema string
	var run47 int64
	err = conn.QueryRow(ctx, `
		SELECT status, run_id, COALESCE(pack_hash,''), COALESCE(output_hash,''), COALESCE(bundle_hash,''),
		       COALESCE(prompt_version,''), COALESCE(schema_version,'')
		FROM unified_analysis_publications WHERE id = 47`).Scan(&status47, &run47, &pack47, &out47, &bundle47, &prompt, &schema)
	if err != nil {
		panic(err)
	}
	fmt.Printf("NEW pub47 status=%s run=%d pack=%s output=%s bundle=%s prompt=%s schema=%s\n",
		status47, run47, trunc(pack47), trunc(out47), trunc(bundle47), prompt, schema)

	// Extract ad efficiency trend from dashboard snapshot if available.
	var snapJSON []byte
	err = conn.QueryRow(ctx, `
		SELECT payload FROM dashboard_data_snapshots WHERE id = (
			SELECT dashboard_data_snapshot_id FROM unified_analysis_publications WHERE id = 47
		)`).Scan(&snapJSON)
	if err != nil {
		fmt.Println("snapshot err:", err)
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(snapJSON, &payload)
	ae, _ := payload["ai_diagnostics"].(map[string]any)
	if ae == nil {
		ae, _ = payload["AIDiagnostics"].(map[string]any)
	}
	ad, _ := ae["ad_efficiency"].(map[string]any)
	if ad == nil {
		// try nested
		b, _ := json.MarshalIndent(topKeys(payload), "", "  ")
		fmt.Println("payload top keys:", string(b))
		return
	}
	trend, _ := ad["trend_7d"].([]any)
	fmt.Println("trend_7d:")
	seen := map[string]struct{}{}
	for _, pt := range trend {
		m, _ := pt.(map[string]any)
		fmt.Printf("  %v spend=%v rev=%v drr=%v\n", m["date"], m["spend"], m["revenue"], m["drr_pct"])
		if drr, ok := m["drr_pct"]; ok && drr != nil {
			seen[fmt.Sprintf("%v", drr)] = struct{}{}
		}
	}
	fmt.Printf("distinct drr values: %d\n", len(seen))
	sum, _ := ad["summary"].(map[string]any)
	fmt.Printf("summary current=%v avg7d=%v\n", sum["current_drr_pct"], sum["average_drr_7d_pct"])

	// Pulse KPI labels from snapshot
	sp, _ := payload["seller_pulse"].(map[string]any)
	kpis, _ := sp["kpis"].([]any)
	for _, k := range kpis {
		m, _ := k.(map[string]any)
		fmt.Printf("pulse kpi key=%v label=%v value=%v\n", m["key"], m["label"], m["value"])
	}
}

func trunc(s string) string {
	if len(s) > 24 {
		return s[:24] + "..."
	}
	return s
}

func topKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
