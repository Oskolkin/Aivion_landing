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
SELECT id, status::text, COALESCE(error_code,''), COALESCE(left(error_message,200),''),
       COALESCE(pack_hash,''), COALESCE(provider_response_id,''),
       unified_publication_id, created_at, completed_at
FROM ai_pipeline_runs
WHERE seller_account_id=1
ORDER BY id DESC LIMIT 5`)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var status, errCode, errMsg, packHash, respID string
		var pubID *int64
		var created, completed interface{}
		if err := rows.Scan(&id, &status, &errCode, &errMsg, &packHash, &respID, &pubID, &created, &completed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		pub := "null"
		if pubID != nil {
			pub = fmt.Sprintf("%d", *pubID)
		}
		fmt.Printf("run=%d status=%s err=%s | %s\n  pack=%s\n  resp=%s pub=%s created=%v completed=%v\n",
			id, status, errCode, errMsg, packHash, respID, pub, created, completed)
	}
}
