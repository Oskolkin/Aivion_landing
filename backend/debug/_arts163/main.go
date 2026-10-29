package main

import (
	"context"
	"fmt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"os"
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
	rows, err := pg.Pool.Query(ctx, `
SELECT artifact_type::text, COALESCE(storage_key,''), COALESCE(content_hash,''), COALESCE(byte_size,0)
FROM ai_pipeline_artifacts WHERE run_id IN (162,163) ORDER BY run_id, id`)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var t, key, hash string
		var size int64
		_ = rows.Scan(&t, &key, &hash, &size)
		fmt.Printf("%s size=%d hash=%s key=%s\n", t, size, hash, key)
	}
}
