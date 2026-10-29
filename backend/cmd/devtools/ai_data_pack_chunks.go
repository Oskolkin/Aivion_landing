package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func runDumpAIDataPackChunks(args []string) int {
	fs := flag.NewFlagSet("dump-ai-data-pack-chunks", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		outputDir     string
		maxRows       int
		maxTokens     int
		pretty        bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.StringVar(&outputDir, "output-dir", "chunks", "output directory")
	fs.IntVar(&maxRows, "max-rows-per-chunk", 50, "max rows per chunk")
	fs.IntVar(&maxTokens, "max-approx-tokens-per-chunk", 12000, "approx token budget per chunk")
	fs.BoolVar(&pretty, "pretty", true, "pretty-print JSON files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}
	metricDate, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	svc := aidatapack.NewService(postgres.Pool)
	pack, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build data pack: %v\n", err)
		return 1
	}
	opts := aidatapack.ChunkOptions{
		MaxRowsPerChunk:           maxRows,
		MaxApproxTokensPerChunk:   maxTokens,
		IncludeLocalFactCatalog:   true,
		IncludeLocalEntityCatalog: true,
	}
	chunkSet, err := aidatapack.BuildChunks(pack, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build chunks: %v\n", err)
		return 1
	}
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		fmt.Fprintf(os.Stderr, "validate chunks: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		return 1
	}
	indent := ""
	if pretty {
		indent = "  "
	}
	setPath := filepath.Join(outputDir, "chunk-set.json")
	if err := writeJSONFile(setPath, chunkSet, indent); err != nil {
		fmt.Fprintf(os.Stderr, "write chunk-set: %v\n", err)
		return 1
	}
	audit := map[string]any{
		"valid":                  true,
		"coverage":               chunkSet.Coverage,
		"chunk_count":            chunkSet.Manifest.ChunkCount,
		"total_approx_tokens":    chunkSet.Manifest.TotalApproxTokens,
		"selection_policy":       chunkSet.ChunkingPolicy.SelectionPolicy,
		"campaign_product_links": campaignProductLinksChunkAudit(pack),
	}
	auditPath := filepath.Join(outputDir, "chunks-audit.json")
	if err := writeJSONFile(auditPath, audit, indent); err != nil {
		fmt.Fprintf(os.Stderr, "write audit: %v\n", err)
		return 1
	}
	for i, ch := range chunkSet.Chunks {
		filename := fmt.Sprintf("%03d_%s.json", i+1, ch.ChunkID)
		if err := writeJSONFile(filepath.Join(outputDir, filename), ch, indent); err != nil {
			fmt.Fprintf(os.Stderr, "write chunk %s: %v\n", ch.ChunkID, err)
			return 1
		}
	}
	fmt.Printf("wrote %d chunks to %s\n", len(chunkSet.Chunks), outputDir)
	return 0
}

func runAnalyzeDashboardChunks(args []string) int {
	fs := flag.NewFlagSet("analyze-dashboard-chunks", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		output        string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.StringVar(&output, "output", "chunk-analyses.json", "output file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}
	metricDate, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 1
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	narrativeSvc, err := dashboardhome.BuildNarrativeService(cfg, postgres.Pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "narrative service: %v\n", err)
		return 1
	}
	packSvc := aidatapack.NewService(postgres.Pool)
	pack, err := packSvc.Build(ctx, sellerID, metricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build pack: %v\n", err)
		return 1
	}
	chunkSet, err := dashboardhome.BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "build chunks: %v\n", err)
		return 1
	}
	orch := &dashboardhome.DataPackChunksOrchestrator{AI: narrativeSvc.OpenAIClient(), Model: cfg.OpenAI.Model}
	run, err := orch.AnalyzeChunksOnly(ctx, pack, chunkSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze chunks: %v\n", err)
		return 1
	}
	result := map[string]any{
		"seller_account_id":      sellerID,
		"metric_date":            metricDate.Format("2006-01-02"),
		"chunk_count":            run.ChunkCount,
		"completed":              run.Completed,
		"failed":                 run.Failed,
		"validated_signal_count": run.ValidatedSignalCount,
		"rejected_signal_count":  run.RejectedSignalCount,
		"analyses":               run.Analyses,
	}
	if err := writeJSONFile(output, result, "  "); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		return 1
	}
	fmt.Printf("wrote chunk analyses to %s\n", output)
	return 0
}

func writeJSONFile(path string, v any, indent string) error {
	var raw []byte
	var err error
	if indent != "" {
		raw, err = json.MarshalIndent(v, "", indent)
	} else {
		raw, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func campaignProductLinksChunkAudit(pack *aidatapack.SellerAIDataPack) map[string]int {
	report := aidatapack.AuditDataPack(pack).CampaignProductLinks
	return map[string]int{
		"source_count":             report.SourceCount,
		"distinct_by_campaign_sku": report.DistinctByCampaignSKU,
		"duplicate_count":          report.DuplicateCount,
	}
}
