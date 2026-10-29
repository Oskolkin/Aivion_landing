package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func runCheckUnifiedAnalysisReadiness(args []string) int {
	fs := flag.NewFlagSet("check-unified-analysis-readiness", flag.ContinueOnError)
	var live bool
	fs.BoolVar(&live, "live", false, "also probe OpenAI schema acceptance")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{ForceEnabled: true, PromptProfile: "analysis"})
	offline := map[string]any{
		"prompt_version": prompt.VersionAnalystV1_5,
		"schema_name":    contract.SchemaName(),
		"schema_strict":  true,
	}
	if err != nil {
		offline["env_error"] = err.Error()
		raw, _ := json.MarshalIndent(offline, "", "  ")
		fmt.Println(string(raw))
		return 1
	}
	defer env.Close()

	rendered, err := prompt.Render(prompt.PromptProfileAnalysis)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prompt: %v\n", err)
		return 1
	}
	schemaHash, err := contract.SchemaHash()
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema: %v\n", err)
		return 1
	}
	offline["prompt_version"] = rendered.Version
	offline["prompt_hash"] = rendered.Hash
	offline["prompt_bytes"] = len(rendered.Instructions) + len(rendered.Input)
	offline["schema_hash"] = schemaHash
	offline["schema_version"] = contract.UnifiedAnalysisOutputSchemaVersionCurrent
	offline["model"] = env.UAConfig.Model
	offline["max_output_tokens"] = env.UAConfig.MaxOutputTokens
	offline["output_validation_enabled"] = env.UAConfig.OutputValidationEnabled
	offline["provider_enabled"] = env.UAConfig.Enabled
	offline["prompt_profile"] = env.UAConfig.PromptProfile
	offline["api_key_configured"] = strings.TrimSpace(env.Cfg.OpenAI.APIKey) != ""

	ok54, migMsg54 := checkUnifiedAnalysisMigration(ctx, env.Postgres.Pool)
	offline["migration_000054"] = ok54
	offline["migration_000054_note"] = migMsg54
	ok55, migMsg55 := checkUnifiedAnalysisMigration055(ctx, env.Postgres.Pool)
	offline["migration_000055"] = ok55
	offline["migration_000055_note"] = migMsg55

	offline["postgres_reachable"] = true
	offline["redis_reachable"] = checkRedisReachable(ctx, env.Cfg.Redis.Addr, env.Cfg.Redis.Password)
	offline["minio_reachable"] = env.S3 != nil

	exitCode := 0
	if !ok54 || !ok55 || !offline["api_key_configured"].(bool) || strings.TrimSpace(env.UAConfig.Model) == "" {
		exitCode = 1
	}
	if reachable, ok := offline["redis_reachable"].(bool); ok && !reachable {
		exitCode = 1
	}

	if live {
		code := runCheckUnifiedProviderReadiness([]string{"--live=true"})
		offline["live_provider_exit_code"] = code
		if code != 0 {
			exitCode = code
		}
	}

	raw, _ := json.MarshalIndent(offline, "", "  ")
	fmt.Println(string(raw))
	return exitCode
}

func checkUnifiedAnalysisMigration055(ctx context.Context, pool *pgxpool.Pool) (bool, string) {
	var indexDef string
	err := pool.QueryRow(ctx, `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname = 'uq_ai_pipeline_runs_one_active_unified_analysis'
`).Scan(&indexDef)
	if err != nil {
		return false, err.Error()
	}
	if !strings.Contains(indexDef, "publication_ready") || !strings.Contains(indexDef, "publication_blocked") {
		return false, "unique index missing publication_* exclusions"
	}
	return true, "unique index excludes publication_ready/publication_blocked"
}

func checkRedisReachable(ctx context.Context, addr, password string) bool {
	if strings.TrimSpace(addr) == "" {
		return false
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	defer client.Close()
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return client.Ping(cctx).Err() == nil
}
