package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	appRedis "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/redis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

type localEnvCheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}

type localEnvReport struct {
	Checks []localEnvCheck `json:"checks"`
	Ready  bool            `json:"ready"`
}

func runCheckLocalEnv(args []string) int {
	fs := flag.NewFlagSet("check-local-env", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON report")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_ = config.LoadEnvFiles()
	report := buildLocalEnvReport(context.Background())

	if *jsonOut {
		raw, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(raw))
	} else {
		printLocalEnvReport(report)
	}
	if report.Ready {
		return 0
	}
	return 1
}

func buildLocalEnvReport(ctx context.Context) localEnvReport {
	checks := []localEnvCheck{
		checkConfigLoad(),
		checkPostgresConfig(),
		checkRedisConfig(),
		checkObjectStorageConfig(),
		checkOpenAIAPIKey(),
		checkAIProxyConfig(),
		checkGlobalContextOutputLimitEnv(),
		checkActionsComposeOutputLimitEnv(),
		checkObjectDetailOutputLimitEnv(),
	}
	checks = append(checks, checkPostgresConnection(ctx))
	checks = append(checks, checkRedisConnection(ctx))
	checks = append(checks, checkObjectStorageConnection(ctx))

	ready := true
	for _, c := range checks {
		if !c.Pass {
			ready = false
			break
		}
	}
	return localEnvReport{Checks: checks, Ready: ready}
}

func checkConfigLoad() localEnvCheck {
	cfg, err := config.Load()
	if err != nil {
		return localEnvCheck{Name: "Configuration", Pass: false, Detail: err.Error()}
	}
	return localEnvCheck{Name: "Configuration", Pass: true, Detail: cfg.App.Env}
}

func checkPostgresConfig() localEnvCheck {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return localEnvCheck{Name: "PostgreSQL config", Pass: false, Detail: "DATABASE_URL unset"}
	}
	return localEnvCheck{Name: "PostgreSQL config", Pass: true}
}

func checkRedisConfig() localEnvCheck {
	if strings.TrimSpace(os.Getenv("REDIS_ADDR")) == "" {
		return localEnvCheck{Name: "Redis config", Pass: false, Detail: "REDIS_ADDR unset"}
	}
	return localEnvCheck{Name: "Redis config", Pass: true}
}

func checkObjectStorageConfig() localEnvCheck {
	required := []string{"S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET_RAW", "S3_BUCKET_EXPORTS", "S3_BUCKET_ARTIFACTS"}
	missing := make([]string, 0)
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return localEnvCheck{Name: "Object storage config", Pass: false, Detail: "missing: " + strings.Join(missing, ", ")}
	}
	return localEnvCheck{Name: "Object storage config", Pass: true}
}

func checkOpenAIAPIKey() localEnvCheck {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return localEnvCheck{Name: "OpenAI API key", Pass: false, Detail: "OPENAI_API_KEY unset"}
	}
	return localEnvCheck{Name: "OpenAI API key", Pass: true, Detail: "configured"}
}

func checkAIProxyConfig() localEnvCheck {
	proxy := strings.TrimSpace(os.Getenv("AI_PROXY_URL"))
	if proxy == "" {
		proxy = strings.TrimSpace(os.Getenv("AI_HTTPS_PROXY"))
	}
	if proxy == "" {
		return localEnvCheck{Name: "AI proxy", Pass: true, Detail: "not configured (direct connection)"}
	}
	return localEnvCheck{Name: "AI proxy", Pass: true, Detail: proxy}
}

func checkGlobalContextOutputLimitEnv() localEnvCheck {
	v := positiveEnvIntLocal("OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS")
	if v < 16384 {
		return localEnvCheck{Name: "Global Context output limit", Pass: false, Detail: strconv.Itoa(v)}
	}
	return localEnvCheck{Name: "Global Context output limit", Pass: true, Detail: strconv.Itoa(v)}
}

func checkActionsComposeOutputLimitEnv() localEnvCheck {
	v := positiveEnvIntLocal("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS")
	if v < 16384 {
		return localEnvCheck{Name: "Actions Compose output limit", Pass: false, Detail: strconv.Itoa(v)}
	}
	return localEnvCheck{Name: "Actions Compose output limit", Pass: true, Detail: strconv.Itoa(v)}
}

func checkObjectDetailOutputLimitEnv() localEnvCheck {
	v := positiveEnvIntLocal("OPENAI_ACTION_OBJECT_DETAIL_MAX_OUTPUT_TOKENS")
	if v < 8192 {
		return localEnvCheck{Name: "Object Detail output limit", Pass: false, Detail: strconv.Itoa(v)}
	}
	return localEnvCheck{Name: "Object Detail output limit", Pass: true, Detail: strconv.Itoa(v)}
}

func checkPostgresConnection(ctx context.Context) localEnvCheck {
	cfg, err := config.Load()
	if err != nil || strings.TrimSpace(cfg.DB.URL) == "" {
		return localEnvCheck{Name: "PostgreSQL connection", Pass: false, Detail: "not configured"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	postgres, err := db.New(pingCtx, cfg.DB.URL)
	if err != nil {
		return localEnvCheck{Name: "PostgreSQL connection", Pass: false, Detail: "connection failed"}
	}
	defer postgres.Close()
	return localEnvCheck{Name: "PostgreSQL connection", Pass: true}
}

func checkRedisConnection(ctx context.Context) localEnvCheck {
	cfg, err := config.Load()
	if err != nil || strings.TrimSpace(cfg.Redis.Addr) == "" {
		return localEnvCheck{Name: "Redis connection", Pass: false, Detail: "not configured"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client, err := appRedis.New(pingCtx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return localEnvCheck{Name: "Redis connection", Pass: false, Detail: "connection failed"}
	}
	defer client.Close()
	return localEnvCheck{Name: "Redis connection", Pass: true}
}

func checkObjectStorageConnection(ctx context.Context) localEnvCheck {
	cfg, err := config.Load()
	if err != nil {
		return localEnvCheck{Name: "Object storage connection", Pass: false, Detail: "config failed"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = storage.New(pingCtx, storage.S3Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKey:       cfg.S3.AccessKey,
		SecretKey:       cfg.S3.SecretKey,
		UseSSL:          cfg.S3.UseSSL,
		BucketRaw:       cfg.S3.BucketRaw,
		BucketExports:   cfg.S3.BucketExports,
		BucketArtifacts: cfg.S3.BucketArtifacts,
	})
	if err != nil {
		return localEnvCheck{Name: "Object storage connection", Pass: false, Detail: "connection failed"}
	}
	return localEnvCheck{Name: "Object storage connection", Pass: true}
}

func positiveEnvIntLocal(key string) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func printLocalEnvReport(report localEnvReport) {
	for _, c := range report.Checks {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
		}
		if c.Detail != "" {
			fmt.Printf("%s: %s (%s)\n", c.Name, status, c.Detail)
		} else {
			fmt.Printf("%s: %s\n", c.Name, status)
		}
	}
	fmt.Println()
	if report.Ready {
		fmt.Println("Ready to run AI pipeline: YES")
	} else {
		fmt.Println("Ready to run AI pipeline: NO")
	}
}
