package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func runExportAIRunInspector(args []string) int {
	fs := flag.NewFlagSet("export-ai-run-inspector", flag.ContinueOnError)
	runID := fs.Int64("run-id", 0, "pipeline run id (required)")
	output := fs.String("output", "", "output directory (required)")
	includeRaw := fs.Bool("include-raw", true, "include raw payloads")
	includePreviews := fs.Bool("include-previews", true, "include diagnostic previews")
	validateLinks := fs.Bool("validate-links", true, "validate inspector anchor links")
	failOnIncomplete := fs.Bool("fail-on-incomplete-diagnostics", false, "fail when diagnostics incomplete")
	diagnosticSummary := fs.Bool("diagnostic-summary", false, "print diagnostic summary to stdout")
	prettyJSON := fs.Bool("pretty-json", true, "pretty-print JSON files")
	openBrowser := fs.Bool("open", false, "open index.html in system browser")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *runID <= 0 {
		fmt.Fprintln(os.Stderr, "run not found: --run-id is required")
		return 1
	}
	if strings.TrimSpace(*output) == "" {
		fmt.Fprintln(os.Stderr, "output directory unwritable: --output is required")
		return 1
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer postgres.Close()

	store := aipipeline.NewStore(dbgen.New(postgres.Pool))
	if err := aipipeline.ExportRunInspector(ctx, store, aipipeline.RunInspectorExportOptions{
		RunID: *runID, OutputDir: *output, IncludeRaw: *includeRaw,
		IncludePreviews: *includePreviews, ValidateLinks: *validateLinks,
		FailOnIncomplete: *failOnIncomplete, DiagnosticSummary: *diagnosticSummary,
		PrettyJSON: *prettyJSON,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Inspector exported to %s\n", *output)
	if *openBrowser {
		openLocalFile(filepathJoin(*output, "index.html"))
	}
	return 0
}

func filepathJoin(a, b string) string {
	if runtime.GOOS == "windows" {
		return strings.TrimRight(a, `/\`) + `\` + b
	}
	return strings.TrimRight(a, `/`) + `/` + b
}

func openLocalFile(path string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", path}
	case "darwin":
		cmd = "open"
		args = []string{path}
	default:
		cmd = "xdg-open"
		args = []string{path}
	}
	p, _ := os.StartProcess(cmd, append([]string{cmd}, args...), &os.ProcAttr{})
	if p != nil {
		_ = p.Release()
	}
}
