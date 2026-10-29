package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

type checkAIConnectivityResult struct {
	Status       string               `json:"status"`
	Provider     string               `json:"provider"`
	Proxy        openaix.ProxySummary `json:"proxy,omitempty"`
	LatencyMS    int64                `json:"latency_ms,omitempty"`
	ErrorCode    string               `json:"error_code,omitempty"`
	ErrorMessage string               `json:"error_message,omitempty"`
}

func runCheckAIConnectivity(args []string) int {
	fs := flag.NewFlagSet("check-ai-connectivity", flag.ContinueOnError)
	var (
		provider         string
		timeoutRaw       string
		proxyDiagnostics bool
	)
	fs.StringVar(&provider, "provider", "openai", "AI provider to check (openai)")
	fs.StringVar(&timeoutRaw, "timeout", "30s", "request timeout")
	fs.BoolVar(&proxyDiagnostics, "proxy-diagnostics", true, "include proxy diagnostics in output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	timeout, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
	if err != nil || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeout: expected positive duration, e.g. 30s")
		return 2
	}
	if strings.ToLower(strings.TrimSpace(provider)) != "openai" {
		fmt.Fprintln(os.Stderr, "only --provider=openai is supported")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		out := checkAIConnectivityResult{
			Status:       "failed",
			Provider:     "openai",
			ErrorCode:    "config_failed",
			ErrorMessage: err.Error(),
		}
		if proxyDiagnostics {
			out.Proxy = openaix.ProxySummaryFromEnv()
		}
		printCheckAIConnectivity(out)
		return 1
	}

	result := checkAIConnectivityResult{
		Provider: "openai",
	}
	if proxyDiagnostics {
		result.Proxy = openaix.ProxySummaryFromEnv()
	}

	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		result.Status = "failed"
		result.ErrorCode = "missing_api_key"
		result.ErrorMessage = "OPENAI_API_KEY is not configured"
		printCheckAIConnectivity(result)
		return 1
	}

	client, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:               cfg.OpenAI.APIKey,
		Model:                cfg.OpenAI.Model,
		TimeoutSeconds:       int(timeout.Seconds()),
		MaxRetries:           0,
		ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxOutputTokens:      16,
	})
	if err != nil {
		result.Status = "failed"
		result.ErrorCode = "client_init_failed"
		result.ErrorMessage = err.Error()
		printCheckAIConnectivity(result)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, callErr := client.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt:    "You are a connectivity probe. Reply with strict JSON only.",
		UserPrompt:      `Return {"ok":true}`,
		MaxOutputTokens: 16,
	})
	result.LatencyMS = time.Since(start).Milliseconds()

	if callErr != nil {
		result.Status = "failed"
		result.ErrorCode = "connection_failed"
		result.ErrorMessage = callErr.Error()
		printCheckAIConnectivity(result)
		return 1
	}

	result.Status = "ok"
	printCheckAIConnectivity(result)
	return 0
}

func printCheckAIConnectivity(result checkAIConnectivityResult) {
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
}
