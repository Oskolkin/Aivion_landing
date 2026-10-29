package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func runValidateUnifiedAnalysisOutput(args []string) int {
	fs := flag.NewFlagSet("validate-unified-analysis-output", flag.ContinueOnError)
	var (
		runID      int64
		packPath   string
		outputPath string
	)
	fs.Int64Var(&runID, "run-id", 0, "validate stored run output")
	fs.StringVar(&packPath, "pack", "", "path to analysis_pack.zip")
	fs.StringVar(&outputPath, "output", "", "path to unified_analysis_output.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if runID > 0 {
		return validateUnifiedRunOutput(runID)
	}
	if strings.TrimSpace(packPath) == "" || strings.TrimSpace(outputPath) == "" {
		fmt.Fprintln(os.Stderr, "provide --run-id=N or both --pack and --output")
		return 2
	}

	packBytes, err := os.ReadFile(packPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read pack: %v\n", err)
		return 1
	}
	outBytes, err := os.ReadFile(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read output: %v\n", err)
		return 1
	}
	out, err := outputextract.ParseOutput(outBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse output: %v\n", err)
		return 1
	}
	idx, err := outputvalidator.BuildPackReferenceIndex(packBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack index: %v\n", err)
		return 1
	}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		PackHash:   out.AnalysisMeta.PackHash,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, idx)
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
	if !report.Valid {
		return 1
	}
	return 0
}

func validateUnifiedRunOutput(runID int64) int {
	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{ForceEnabled: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	result, err := env.Service.ValidateOutput(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		return 1
	}
	out := map[string]any{
		"run_id":  runID,
		"valid":   result != nil && result.Valid,
		"blocked": result != nil && result.Blocked,
		"failed":  result != nil && result.Failed,
	}
	if result != nil {
		if result.Run != nil {
			out["status"] = result.Run.Status
		}
		out["output_hash"] = result.OutputHash
		out["outcome"] = result.Outcome
		out["action_count"] = result.ActionCount
		out["top_action_count"] = result.TopActionCount
		out["report"] = result.Report
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(raw))
	if result == nil || !result.Valid {
		return 1
	}
	_ = unifiedanalysis.StatusPublicationReady
	return 0
}
