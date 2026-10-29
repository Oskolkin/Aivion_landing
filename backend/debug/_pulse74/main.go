package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pg.Close()
	var payload []byte
	if err := pg.Pool.QueryRow(ctx, `
SELECT s.payload_json FROM ai_dashboard_data_snapshots s
JOIN unified_analysis_publications p ON p.dashboard_data_snapshot_id = s.id
WHERE p.id=74`).Scan(&payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile("debug/seller-1-full-reseed/frozen-dash-74.json", payload, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", len(payload), "bytes")
}
