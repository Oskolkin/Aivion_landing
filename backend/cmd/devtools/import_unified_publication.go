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

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5/pgtype"
)

const importUnifiedPublicationToolVersion = "import-unified-publication/v1"

func runImportUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("import-unified-publication", flag.ContinueOnError)
	var (
		inputDir   string
		sellerID   int64
		gitSHA     string
		dryRun     bool
		markSource bool
	)
	fs.StringVar(&inputDir, "input", "", "export directory produced by export-unified-publication (required)")
	fs.Int64Var(&sellerID, "seller-id", 1, "target seller_account_id")
	fs.StringVar(&gitSHA, "git-sha", "", "import git sha recorded in audit provenance")
	fs.BoolVar(&dryRun, "dry-run", false, "validate export only")
	fs.BoolVar(&markSource, "mark-source-mode", true, "set seller current source_mode=imported_demo_snapshot")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(inputDir) == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "--seller-id must be > 0")
		return 2
	}

	manifestPath := filepath.Join(inputDir, "manifest.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read manifest: %v\n", err)
		return 1
	}
	var manifest unifiedPublicationExportManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "parse manifest: %v\n", err)
		return 1
	}
	if got, err := checksumExportDir(inputDir, manifest.ArtifactFiles); err != nil {
		fmt.Fprintf(os.Stderr, "checksum artifacts: %v\n", err)
		return 1
	} else if strings.TrimSpace(manifest.ChecksumSHA256) != "" && got != manifest.ChecksumSHA256 {
		fmt.Fprintf(os.Stderr, "checksum mismatch: manifest=%s got=%s\n", manifest.ChecksumSHA256, got)
		return 1
	}

	validatedRaw, err := os.ReadFile(filepath.Join(inputDir, manifest.ArtifactFiles["validated_output"]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read validated_output: %v\n", err)
		return 1
	}
	factsRaw, err := os.ReadFile(filepath.Join(inputDir, manifest.ArtifactFiles["dashboard_facts"]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dashboard_facts: %v\n", err)
		return 1
	}
	validationRaw, err := os.ReadFile(filepath.Join(inputDir, manifest.ArtifactFiles["validation"]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read validation: %v\n", err)
		return 1
	}
	summaryRaw, err := os.ReadFile(filepath.Join(inputDir, manifest.ArtifactFiles["summary"]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read summary: %v\n", err)
		return 1
	}

	var parsed contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(validatedRaw, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "parse validated output: %v\n", err)
		return 1
	}
	var facts dashboardfacts.DashboardFactsV1
	if err := json.Unmarshal(factsRaw, &facts); err != nil {
		fmt.Fprintf(os.Stderr, "parse facts: %v\n", err)
		return 1
	}
	metricDate, err := time.Parse("2006-01-02", manifest.SourceMetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric_date: %v\n", err)
		return 1
	}

	// Re-bind identity to the target seller while preserving pack hash / narrative content.
	packID := strings.TrimSpace(manifest.SourcePackID)
	packHash := strings.TrimSpace(manifest.SourcePackHash)
	if packID == "" {
		packID = fmt.Sprintf("pack:%d:%s:imported", sellerID, manifest.SourceMetricDate)
	}
	if packHash == "" {
		packHash = strings.TrimSpace(parsed.AnalysisMeta.PackHash)
	}
	parsed.AnalysisMeta.PackID = packID
	parsed.AnalysisMeta.PackHash = packHash
	parsed.AnalysisMeta.MetricDate = manifest.SourceMetricDate
	facts.SellerAccountID = sellerID
	facts.MetricDate = manifest.SourceMetricDate
	facts.PackID = packID
	facts.PackHash = packHash

	canonical, err := outputvalidator.CanonicalJSON(&parsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonicalize: %v\n", err)
		return 1
	}
	outputHash := outputvalidator.OutputHash(canonical)

	if dryRun {
		printJSON(map[string]any{
			"ok":                          true,
			"dry_run":                     true,
			"seller_id":                   sellerID,
			"source_local_publication_id": manifest.SourcePublicationID,
			"source_local_run_id":         manifest.SourceRunID,
			"metric_date":                 manifest.SourceMetricDate,
			"pack_hash":                   packHash,
			"actions":                     len(parsed.Actions),
			"output_hash":                 outputHash,
			"checksum_sha256":             manifest.ChecksumSHA256,
		})
		return 0
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	if err := ensureImportedDemoSourceModeAllowed(ctx, env); err != nil {
		fmt.Fprintf(os.Stderr, "source_mode constraint: %v\n", err)
		return 1
	}

	// Ensure seller is in unified mode so PublishRun may set a visible current pointer.
	if _, err := env.Queries.UpsertSellerAnalysisSettings(ctx, dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          "unified",
		AutoRunEnabled:        false,
		AutoPublishEnabled:    false,
		LegacyFallbackEnabled: true,
		UpdatedBy:             "devtool:import-unified-publication",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "set seller mode: %v\n", err)
		return 1
	}

	promptVersion := firstNonEmptyStr(manifest.SourcePromptVersion, publication.DefaultPublicationPolicy().AcceptedPromptVersion)
	q := env.Queries
	run, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               importedDemoSnapshotSourceMode,
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: pgtype.Text{String: contract.AnalysisPackSchemaVersionV1, Valid: true},
		DataPackHash:          packHash,
		PipelineVersion:       pgtype.Text{String: importUnifiedPublicationToolVersion, Valid: true},
		PromptVersion:         pgtype.Text{String: promptVersion, Valid: true},
		Provider:              pgtype.Text{String: "imported_demo_snapshot", Valid: true},
		ProviderModel:         pgtype.Text{String: "none", Valid: true},
		ProviderAttemptCount:  0,
		StartedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create run: %v\n", err)
		return 1
	}
	if _, err := env.Postgres.Pool.Exec(ctx, `
UPDATE ai_pipeline_runs SET
  pack_id=$2, pack_hash=$3, code_interpreter_calls=0,
  provider_status='skipped_imported', cleanup_status='completed'
WHERE id=$1`, run.ID, packID, packHash); err != nil {
		fmt.Fprintf(os.Stderr, "stamp run pack: %v\n", err)
		return 1
	}

	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   canonical,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "save validated artifact: %v\n", err)
		return 1
	}

	// Rebuild validation identity around the imported canonical hash while
	// preserving source prompt/schema versions for auditability.
	var validationObj map[string]any
	_ = json.Unmarshal(validationRaw, &validationObj)
	if validationObj == nil {
		validationObj = map[string]any{}
	}
	validationObj["valid"] = true
	validationObj["prompt_version"] = promptVersion
	validationObj["schema_version"] = firstNonEmptyStr(manifest.SourceSchemaVersion, parsed.SchemaVersion)
	validationObj["output_hash"] = outputHash
	validationObj["imported"] = true
	validationObj["source_mode"] = importedDemoSnapshotSourceMode
	validationPayload, _ := json.Marshal(validationObj)
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation,
		Hash:          pgtype.Text{String: "validation", Valid: true},
		Storage:       "db",
		PayloadJson:   validationPayload,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "save validation artifact: %v\n", err)
		return 1
	}

	var summaryObj map[string]any
	_ = json.Unmarshal(summaryRaw, &summaryObj)
	if summaryObj == nil {
		summaryObj = map[string]any{}
	}
	summaryObj["outcome"] = parsed.AnalysisOutcome
	summaryObj["action_count"] = len(parsed.Actions)
	summaryObj["output_hash"] = outputHash
	summaryObj["imported"] = true
	summaryPayload, _ := json.Marshal(summaryObj)
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: run.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputSummary,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   summaryPayload,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "save summary artifact: %v\n", err)
		return 1
	}

	artifacts := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(q))
	if err := artifacts.SaveDashboardFacts(ctx, run.ID, facts); err != nil {
		fmt.Fprintf(os.Stderr, "save facts artifact: %v\n", err)
		return 1
	}

	vis := publication.VisibilityVisible
	asCurrent := true
	_ = os.Setenv("ALLOW_DEV_SEED", "1")
	pubSvc := buildPublicationService(env)
	result, pubErr := pubSvc.PublishRun(ctx, publication.PublishRequest{
		RunID:                 run.ID,
		Actor:                 "devtool:import-unified-publication",
		AllowNonCurrentPrompt: true,
		ForceVisibility:       &vis,
		PublishAsCurrent:      &asCurrent,
		AllowOlderMetricDate:  true,
	})
	if pubErr != nil {
		fmt.Fprintf(os.Stderr, "publish imported run: %v\n", pubErr)
		printJSON(publishResultSummary(result))
		return 1
	}
	if !isSuccessfulPublishOutcome(result.Outcome) {
		fmt.Fprintf(os.Stderr, "publish did not succeed: outcome=%s error_code=%s error_message=%s\n",
			result.Outcome, result.ErrorCode, result.ErrorMessage)
		printJSON(publishResultSummary(result))
		return 1
	}

	provenance := map[string]any{
		"source_mode":                 importedDemoSnapshotSourceMode,
		"source_local_publication_id": manifest.SourcePublicationID,
		"source_local_run_id":         manifest.SourceRunID,
		"source_metric_date":          manifest.SourceMetricDate,
		"source_pack_hash":            packHash,
		"import_timestamp":            time.Now().UTC().Format(time.RFC3339),
		"import_tool_version":         importUnifiedPublicationToolVersion,
		"import_git_sha":              strings.TrimSpace(gitSHA),
		"export_checksum_sha256":      manifest.ChecksumSHA256,
		"export_tool_version":         manifest.ToolVersion,
		"staging_run_id":              run.ID,
		"staging_publication_id":      result.PublicationID,
	}
	provRaw, _ := json.Marshal(provenance)
	if _, err := env.Postgres.Pool.Exec(ctx, `
INSERT INTO unified_analysis_publication_audit
  (seller_account_id, operation, actor, run_id, publication_id, previous_publication_id, new_publication_id, reason, payload_json)
VALUES ($1,'import_demo_snapshot','devtool:import-unified-publication',$2,$3,NULL,$3,$4,$5::jsonb)`,
		sellerID, run.ID, result.PublicationID,
		"imported historical local demo publication without OpenAI regeneration",
		string(provRaw),
	); err != nil {
		fmt.Fprintf(os.Stderr, "audit provenance: %v\n", err)
		return 1
	}

	if markSource {
		if _, err := env.Postgres.Pool.Exec(ctx, `
UPDATE seller_current_unified_publications
SET source_mode=$2, updated_at=now()
WHERE seller_account_id=$1`, sellerID, importedDemoSnapshotSourceMode); err != nil {
			fmt.Fprintf(os.Stderr, "mark source_mode: %v\n", err)
			return 1
		}
	}

	var currentSource string
	var currentPub int64
	_ = env.Postgres.Pool.QueryRow(ctx, `
SELECT publication_id, source_mode FROM seller_current_unified_publications WHERE seller_account_id=$1`,
		sellerID).Scan(&currentPub, &currentSource)

	out := publishResultSummary(result)
	out["source_mode"] = currentSource
	out["current_publication_id"] = currentPub
	out["provenance"] = provenance
	out["ok"] = true
	printJSON(out)
	return 0
}

func ensureImportedDemoSourceModeAllowed(ctx context.Context, env *unifiedDevtoolEnv) error {
	_, err := env.Postgres.Pool.Exec(ctx, `
ALTER TABLE seller_current_unified_publications
  DROP CONSTRAINT IF EXISTS chk_seller_current_unified_publications_source_mode`)
	if err != nil {
		return err
	}
	_, err = env.Postgres.Pool.Exec(ctx, `
ALTER TABLE seller_current_unified_publications
  ADD CONSTRAINT chk_seller_current_unified_publications_source_mode
  CHECK (source_mode IN ('unified', 'imported_demo_snapshot'))`)
	return err
}
