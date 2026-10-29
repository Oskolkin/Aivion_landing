package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	sellerID := int64(1)
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &sellerID)
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		panic(err)
	}
	defer postgres.Close()
	rows, err := postgres.Pool.Query(ctx, `
		SELECT id::text, seller_account_id, metric_date::text, status, prompt_version, model,
		       COALESCE(error_message, ''), validation_errors, input_tokens, output_tokens,
		       context_hash, risk_drivers_hash, started_at, finished_at
		FROM ai_dashboard_narratives
		WHERE seller_account_id = $1
		ORDER BY started_at DESC
		LIMIT 10`, sellerID)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var seller int64
		var metricDate, status, promptVersion, model, errMsg, contextHash, renderedHash string
		var validationErrors []byte
		var inputTokens, outputTokens *int32
		var startedAt, finishedAt interface{}
		if err := rows.Scan(&id, &seller, &metricDate, &status, &promptVersion, &model, &errMsg, &validationErrors, &inputTokens, &outputTokens, &contextHash, &renderedHash, &startedAt, &finishedAt); err != nil {
			panic(err)
		}
		out := map[string]any{
			"id": id, "seller_account_id": seller, "metric_date": metricDate, "status": status,
			"prompt_version": promptVersion, "model": model, "error_message": errMsg,
			"context_hash": contextHash, "rendered_context_hash": renderedHash,
			"started_at": startedAt, "finished_at": finishedAt,
		}
		if inputTokens != nil {
			out["input_tokens"] = *inputTokens
		}
		if outputTokens != nil {
			out["output_tokens"] = *outputTokens
		}
		var errs []string
		_ = json.Unmarshal(validationErrors, &errs)
		out["validation_errors"] = errs
		encoded, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(encoded))
	}
}
