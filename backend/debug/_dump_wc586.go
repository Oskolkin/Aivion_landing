package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	var raw []byte
	err = pool.QueryRow(ctx, `SELECT payload_json->'dashboard'->'what_changed' FROM ai_pipeline_artifacts WHERE pipeline_run_id=586 AND kind='unified_analysis_output_validated'`).Scan(&raw)
	if err != nil {
		panic(err)
	}
	os.WriteFile("debug/_wc_586_utf8.json", raw, 0644)
	var wc map[string]any
	json.Unmarshal(raw, &wc)
	b, _ := json.MarshalIndent(wc, "", "  ")
	fmt.Println(string(b))
}
