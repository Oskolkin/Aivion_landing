package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	appRedis "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/redis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	uaprovider "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

type uaReadinessCheck struct {
	Name   string
	Pass   bool
	Detail string
}

func runCheckUnifiedProviderReadiness(args []string) int {
	fs := flag.NewFlagSet("check-unified-provider-readiness", flag.ContinueOnError)
	live := fs.Bool("live", false, "run live OpenAI container smoke (requires API key)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}

	ctx := context.Background()
	checks := []uaReadinessCheck{
		checkUnifiedProviderConfig(cfg),
		checkUnifiedProviderAPIKey(cfg),
		checkUnifiedProviderModel(cfg.UnifiedAnalysis),
		checkUnifiedProviderMemory(cfg.UnifiedAnalysis),
		checkUnifiedProviderPostgresConfig(),
		checkUnifiedProviderRedisConfig(),
		checkUnifiedProviderObjectStorageConfig(),
	}

	var pool *pgxpool.Pool
	if strings.TrimSpace(cfg.DB.URL) != "" {
		postgres, connErr := db.New(ctx, cfg.DB.URL)
		if connErr != nil {
			checks = append(checks, uaReadinessCheck{Name: "PostgreSQL connection", Pass: false, Detail: connErr.Error()})
		} else {
			pool = postgres.Pool
			checks = append(checks, uaReadinessCheck{Name: "PostgreSQL connection", Pass: true})
			defer postgres.Close()
		}
	} else {
		checks = append(checks, uaReadinessCheck{Name: "PostgreSQL connection", Pass: false, Detail: "DATABASE_URL unset"})
	}

	if strings.TrimSpace(cfg.Redis.Addr) != "" {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		client, connErr := appRedis.New(pingCtx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		cancel()
		if connErr != nil {
			checks = append(checks, uaReadinessCheck{Name: "Redis connection", Pass: false, Detail: connErr.Error()})
		} else {
			client.Close()
			checks = append(checks, uaReadinessCheck{Name: "Redis connection", Pass: true})
		}
	} else {
		checks = append(checks, uaReadinessCheck{Name: "Redis connection", Pass: false, Detail: "REDIS_ADDR unset"})
	}

	if hasObjectStorageConfig(cfg) {
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, connErr := storage.New(pingCtx, storage.S3Config{
			Endpoint:        cfg.S3.Endpoint,
			AccessKey:       cfg.S3.AccessKey,
			SecretKey:       cfg.S3.SecretKey,
			UseSSL:          cfg.S3.UseSSL,
			BucketRaw:       cfg.S3.BucketRaw,
			BucketExports:   cfg.S3.BucketExports,
			BucketArtifacts: cfg.S3.BucketArtifacts,
		})
		cancel()
		if connErr != nil {
			checks = append(checks, uaReadinessCheck{Name: "Object storage connection", Pass: false, Detail: connErr.Error()})
		} else {
			checks = append(checks, uaReadinessCheck{Name: "Object storage connection", Pass: true})
		}
	} else {
		checks = append(checks, uaReadinessCheck{Name: "Object storage connection", Pass: false, Detail: "S3 config incomplete"})
	}

	if pool != nil {
		ok, detail := checkUnifiedAnalysisMigration(ctx, pool)
		checks = append(checks, uaReadinessCheck{Name: "Unified analysis migration", Pass: ok, Detail: detail})
	} else {
		checks = append(checks, uaReadinessCheck{Name: "Unified analysis migration", Pass: false, Detail: "postgres unavailable"})
	}

	if *live {
		checks = append(checks, runUnifiedProviderLiveProbe(ctx, cfg))
	}

	fmt.Println("UNIFIED ANALYSIS PROVIDER READINESS")
	fmt.Println()
	allPass := true
	for _, c := range checks {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
			allPass = false
		}
		if c.Detail != "" {
			fmt.Printf("%s: %s (%s)\n", c.Name, status, c.Detail)
		} else {
			fmt.Printf("%s: %s\n", c.Name, status)
		}
	}
	fmt.Println()
	if allPass {
		fmt.Println("Ready for unified provider smoke: YES")
		return 0
	}
	fmt.Println("Ready for unified provider smoke: NO")
	return 1
}

func checkUnifiedProviderConfig(cfg *config.Config) uaReadinessCheck {
	if cfg == nil {
		return uaReadinessCheck{Name: "Configuration", Pass: false, Detail: "nil config"}
	}
	ua := cfg.UnifiedAnalysis
	if !ua.Enabled {
		return uaReadinessCheck{Name: "Configuration", Pass: true, Detail: "provider disabled (devtool can force enable when API key present)"}
	}
	if err := ua.Validate(cfg.OpenAI.APIKey); err != nil {
		return uaReadinessCheck{Name: "Configuration", Pass: false, Detail: err.Error()}
	}
	return uaReadinessCheck{Name: "Configuration", Pass: true, Detail: fmt.Sprintf("provider=%s model=%s", ua.Provider, ua.Model)}
}

func checkUnifiedProviderAPIKey(cfg *config.Config) uaReadinessCheck {
	if cfg == nil {
		return uaReadinessCheck{Name: "OpenAI API key", Pass: false}
	}
	if !cfg.UnifiedAnalysis.Enabled {
		if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
			return uaReadinessCheck{Name: "OpenAI API key", Pass: true, Detail: "not required while provider disabled"}
		}
		return uaReadinessCheck{Name: "OpenAI API key", Pass: true, Detail: "present (provider disabled)"}
	}
	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		return uaReadinessCheck{Name: "OpenAI API key", Pass: false, Detail: "OPENAI_API_KEY unset"}
	}
	if aipipeline.UseFakeAIProviders() {
		return uaReadinessCheck{Name: "OpenAI API key", Pass: true, Detail: "AI_PROVIDER_MODE=fake"}
	}
	return uaReadinessCheck{Name: "OpenAI API key", Pass: true, Detail: "configured"}
}

func checkUnifiedProviderModel(ua unifiedanalysis.Config) uaReadinessCheck {
	if !ua.Enabled {
		return uaReadinessCheck{Name: "Provider model", Pass: true, Detail: "not required while disabled"}
	}
	if strings.TrimSpace(ua.Model) == "" {
		return uaReadinessCheck{Name: "Provider model", Pass: false, Detail: "UNIFIED_ANALYSIS_MODEL unset"}
	}
	return uaReadinessCheck{Name: "Provider model", Pass: true, Detail: ua.Model}
}

func checkUnifiedProviderMemory(ua unifiedanalysis.Config) uaReadinessCheck {
	mem := strings.ToLower(strings.TrimSpace(ua.MemoryLimit))
	switch mem {
	case "1g", "4g", "16g", "64g":
		return uaReadinessCheck{Name: "Code interpreter memory", Pass: true, Detail: mem}
	default:
		return uaReadinessCheck{Name: "Code interpreter memory", Pass: false, Detail: ua.MemoryLimit}
	}
}

func checkUnifiedProviderPostgresConfig() uaReadinessCheck {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return uaReadinessCheck{Name: "PostgreSQL config", Pass: false, Detail: "DATABASE_URL unset"}
	}
	return uaReadinessCheck{Name: "PostgreSQL config", Pass: true}
}

func checkUnifiedProviderRedisConfig() uaReadinessCheck {
	if strings.TrimSpace(os.Getenv("REDIS_ADDR")) == "" {
		return uaReadinessCheck{Name: "Redis config", Pass: false, Detail: "REDIS_ADDR unset"}
	}
	return uaReadinessCheck{Name: "Redis config", Pass: true}
}

func checkUnifiedProviderObjectStorageConfig() uaReadinessCheck {
	if hasObjectStorageConfigFromEnv() {
		return uaReadinessCheck{Name: "Object storage config", Pass: true}
	}
	return uaReadinessCheck{Name: "Object storage config", Pass: false, Detail: "missing S3_* env vars"}
}

func hasObjectStorageConfig(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.S3.Endpoint) != "" &&
		strings.TrimSpace(cfg.S3.AccessKey) != "" &&
		strings.TrimSpace(cfg.S3.SecretKey) != "" &&
		strings.TrimSpace(cfg.S3.BucketArtifacts) != ""
}

func hasObjectStorageConfigFromEnv() bool {
	required := []string{"S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET_ARTIFACTS"}
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return false
		}
	}
	return true
}

func runUnifiedProviderLiveProbe(ctx context.Context, cfg *config.Config) uaReadinessCheck {
	if aipipeline.UseFakeAIProviders() {
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: true, Detail: "skipped (AI_PROVIDER_MODE=fake)"}
	}
	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: true, Detail: "skipped (no OPENAI_API_KEY)"}
	}
	if strings.TrimSpace(cfg.UnifiedAnalysis.Model) == "" {
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "UNIFIED_ANALYSIS_MODEL unset"}
	}

	client, err := uaprovider.NewOpenAIClient(uaprovider.ClientConfig{
		APIKey:         cfg.OpenAI.APIKey,
		ProxyURL:       cfg.OpenAI.ProxyURL,
		TimeoutSeconds: cfg.UnifiedAnalysis.HTTPTimeoutSeconds,
		MaxRetries:     cfg.UnifiedAnalysis.TransportRetries,
	})
	if err != nil {
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: err.Error()}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	container, err := client.CreateContainer(probeCtx, uaprovider.CreateContainerRequest{
		Name:        fmt.Sprintf("aivion-readiness-%d", time.Now().Unix()),
		MemoryLimit: "1g",
	})
	if err != nil {
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "create container: " + err.Error()}
	}

	zipBytes, err := tinyAnalysisPackZip()
	if err != nil {
		_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, "")
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: err.Error()}
	}
	archiveSHA := sha256.Sum256(zipBytes)
	file, err := client.UploadContainerFile(probeCtx, uaprovider.UploadContainerFileRequest{
		ContainerID: container.ID,
		Filename:    unifiedanalysis.PackFilename,
		ContentType: "application/zip",
		Content:     zipBytes,
		SHA256:      hex.EncodeToString(archiveSHA[:]),
	})
	if err != nil {
		_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, "")
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "upload file: " + err.Error()}
	}

	resp, err := client.StartBackgroundResponse(probeCtx, uaprovider.StartBackgroundResponseRequest{
		Model:           cfg.UnifiedAnalysis.Model,
		ContainerID:     container.ID,
		Instructions:    uaprovider.SmokeInstructions(),
		Input:           uaprovider.SmokeInput(),
		MaxOutputTokens: minInt(cfg.UnifiedAnalysis.MaxOutputTokens, 512),
	})
	if err != nil {
		_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID)
		return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "start response: " + err.Error()}
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		current, pollErr := client.RetrieveResponse(probeCtx, resp.ID)
		if pollErr != nil {
			_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID)
			return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: pollErr.Error()}
		}
		switch current.Status {
		case uaprovider.ProviderResponseStatusCompleted:
			if !uaprovider.HasCodeInterpreterCall(current) {
				_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID)
				return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "completed without code interpreter call"}
			}
			if cleanupErr := uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID); cleanupErr != nil {
				return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "cleanup: " + cleanupErr.Error()}
			}
			return uaReadinessCheck{Name: "Live OpenAI probe", Pass: true, Detail: "container+zip+response+CI verified"}
		case uaprovider.ProviderResponseStatusFailed, uaprovider.ProviderResponseStatusIncomplete, uaprovider.ProviderResponseStatusCancelled:
			_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID)
			return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "terminal status " + string(current.Status)}
		}
		if time.Now().After(deadline) {
			_, _ = client.CancelResponse(probeCtx, resp.ID)
			_ = uaprovider.CleanupContainer(probeCtx, client, container.ID, file.ID)
			return uaReadinessCheck{Name: "Live OpenAI probe", Pass: false, Detail: "timeout after 5m"}
		}
		time.Sleep(5 * time.Second)
	}
}

func tinyAnalysisPackZip() ([]byte, error) {
	manifest := map[string]any{
		"schema_version":      "aivion_analysis_pack_v1",
		"pack_id":             "pack:0:2026-01-01:readiness00000001",
		"seller_account_id":   0,
		"metric_date":         "2026-01-01",
		"archive_entry_count": 1,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func minInt(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
