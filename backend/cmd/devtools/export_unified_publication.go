package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/jackc/pgx/v5"
)

const importedDemoSnapshotSourceMode = "imported_demo_snapshot"
const exportUnifiedPublicationToolVersion = "export-unified-publication/v1"

type unifiedPublicationExportManifest struct {
	SchemaVersion             string            `json:"schema_version"`
	ToolVersion               string            `json:"tool_version"`
	SourceMarker              string            `json:"source_marker"`
	ExportedAt                string            `json:"exported_at"`
	SourceGitSHA              string            `json:"source_git_sha,omitempty"`
	SourcePublicationID       int64             `json:"source_local_publication_id"`
	SourceRunID               int64             `json:"source_local_run_id"`
	SourceSellerAccountID     int64             `json:"source_local_seller_account_id"`
	SourceMetricDate          string            `json:"source_metric_date"`
	SourcePackID              string            `json:"source_pack_id"`
	SourcePackHash            string            `json:"source_pack_hash"`
	SourcePromptVersion       string            `json:"source_prompt_version"`
	SourceSchemaVersion       string            `json:"source_schema_version"`
	SourceAnalysisOutcome     string            `json:"source_analysis_outcome"`
	SourceStatus              string            `json:"source_status"`
	SourceVisibility          string            `json:"source_visibility"`
	SourceOutputHash          string            `json:"source_output_hash"`
	SourceBundleHash          string            `json:"source_bundle_hash"`
	SourceActionsRecordCount  int               `json:"source_actions_record_count"`
	SourceTopActionsCount     int               `json:"source_top_actions_count"`
	SourceRecommendedCount    int               `json:"source_recommended_actions_count"`
	DashboardBlocksPresent    []string          `json:"dashboard_blocks_present"`
	SellerPulseKeyPointsCount int               `json:"seller_pulse_key_points_count"`
	ReferencedOzonProductIDs  []string          `json:"referenced_ozon_product_ids"`
	ArtifactFiles             map[string]string `json:"artifact_files"`
	ChecksumSHA256            string            `json:"checksum_sha256"`
	Notes                     []string          `json:"notes,omitempty"`
}

func runExportUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("export-unified-publication", flag.ContinueOnError)
	var (
		publicationID int64
		runID         int64
		outputDir     string
		gitSHA        string
	)
	fs.Int64Var(&publicationID, "publication-id", 0, "unified_analysis_publications.id (preferred)")
	fs.Int64Var(&runID, "run-id", 0, "ai_pipeline_runs.id (optional if publication-id set)")
	fs.StringVar(&outputDir, "output", "", "output directory for portable export (required)")
	fs.StringVar(&gitSHA, "git-sha", "", "optional git SHA recorded in manifest")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(outputDir) == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		return 2
	}
	if publicationID <= 0 && runID <= 0 {
		fmt.Fprintln(os.Stderr, "publication-id or run-id is required")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	type pubRow struct {
		ID                 int64
		SellerAccountID    int64
		MetricDate         time.Time
		RunID              int64
		PackID             string
		PackHash           string
		PromptVersion      string
		SchemaVersion      string
		OutputHash         string
		BundleHash         string
		Status             string
		Visibility         string
		AnalysisOutcome    string
		ActionsRecordCount int32
		TopActionsCount    int32
		RecommendedCount   int32
	}
	var pub pubRow
	q := `
SELECT id, seller_account_id, metric_date, run_id, pack_id, pack_hash, prompt_version, schema_version,
       output_hash, bundle_hash, status, visibility, analysis_outcome,
       actions_record_count, top_actions_count, recommended_actions_count
FROM unified_analysis_publications`
	var row pgx.Row
	if publicationID > 0 {
		row = env.Postgres.Pool.QueryRow(ctx, q+` WHERE id=$1`, publicationID)
	} else {
		row = env.Postgres.Pool.QueryRow(ctx, q+` WHERE run_id=$1`, runID)
	}
	if err := row.Scan(
		&pub.ID, &pub.SellerAccountID, &pub.MetricDate, &pub.RunID, &pub.PackID, &pub.PackHash,
		&pub.PromptVersion, &pub.SchemaVersion, &pub.OutputHash, &pub.BundleHash,
		&pub.Status, &pub.Visibility, &pub.AnalysisOutcome,
		&pub.ActionsRecordCount, &pub.TopActionsCount, &pub.RecommendedCount,
	); err != nil {
		fmt.Fprintf(os.Stderr, "load publication: %v\n", err)
		return 1
	}

	validated, validatedHash, ok := loadLatestArtifactJSON(ctx, env, pub.RunID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated)
	if !ok {
		fmt.Fprintln(os.Stderr, "validated output artifact missing")
		return 1
	}
	factsJSON, factsHash, ok := loadLatestArtifactJSON(ctx, env, pub.RunID, unifiedanalysis.ArtifactUnifiedDashboardFacts)
	if !ok {
		fmt.Fprintln(os.Stderr, "dashboard facts artifact missing")
		return 1
	}
	validationJSON, _, _ := loadLatestArtifactJSON(ctx, env, pub.RunID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation)
	summaryJSON, _, _ := loadLatestArtifactJSON(ctx, env, pub.RunID, unifiedanalysis.ArtifactUnifiedAnalysisOutputSummary)

	var parsed contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(validated, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "parse validated output: %v\n", err)
		return 1
	}
	canonical, err := outputvalidator.CanonicalJSON(&parsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonicalize validated output: %v\n", err)
		return 1
	}
	canonicalHash := outputvalidator.OutputHash(canonical)

	blocks := make([]string, 0, 12)
	dashRaw, _ := json.Marshal(parsed.Dashboard)
	var dashMap map[string]json.RawMessage
	_ = json.Unmarshal(dashRaw, &dashMap)
	for k := range dashMap {
		blocks = append(blocks, k)
	}
	keyPoints := 0
	if parsed.Dashboard.SellerPulse.Narrative != nil {
		keyPoints = len(parsed.Dashboard.SellerPulse.Narrative.KeyPoints)
	}
	productIDs := extractReferencedOzonProductIDs(factsJSON, &parsed)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir output: %v\n", err)
		return 1
	}
	files := map[string]string{
		"validated_output": "validated_output.json",
		"dashboard_facts":  "dashboard_facts.json",
		"validation":       "output_validation.json",
		"summary":          "output_summary.json",
	}
	write := func(name string, payload []byte) error {
		if len(payload) == 0 {
			return nil
		}
		return os.WriteFile(filepath.Join(outputDir, name), payload, 0o644)
	}
	if err := write(files["validated_output"], canonical); err != nil {
		fmt.Fprintf(os.Stderr, "write validated: %v\n", err)
		return 1
	}
	if err := write(files["dashboard_facts"], factsJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write facts: %v\n", err)
		return 1
	}
	if len(validationJSON) == 0 {
		validationJSON, _ = json.Marshal(map[string]any{
			"valid":          true,
			"prompt_version": pub.PromptVersion,
			"schema_version": pub.SchemaVersion,
			"output_hash":    canonicalHash,
			"imported":       true,
		})
	}
	if err := write(files["validation"], validationJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write validation: %v\n", err)
		return 1
	}
	if len(summaryJSON) == 0 {
		summaryJSON, _ = json.Marshal(map[string]any{
			"outcome":      parsed.AnalysisOutcome,
			"action_count": len(parsed.Actions),
			"output_hash":  canonicalHash,
		})
	}
	if err := write(files["summary"], summaryJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write summary: %v\n", err)
		return 1
	}

	manifest := unifiedPublicationExportManifest{
		SchemaVersion:             "aivion_unified_publication_export_v1",
		ToolVersion:               exportUnifiedPublicationToolVersion,
		SourceMarker:              importedDemoSnapshotSourceMode,
		ExportedAt:                time.Now().UTC().Format(time.RFC3339),
		SourceGitSHA:              strings.TrimSpace(gitSHA),
		SourcePublicationID:       pub.ID,
		SourceRunID:               pub.RunID,
		SourceSellerAccountID:     pub.SellerAccountID,
		SourceMetricDate:          pub.MetricDate.UTC().Format("2006-01-02"),
		SourcePackID:              pub.PackID,
		SourcePackHash:            pub.PackHash,
		SourcePromptVersion:       pub.PromptVersion,
		SourceSchemaVersion:       pub.SchemaVersion,
		SourceAnalysisOutcome:     pub.AnalysisOutcome,
		SourceStatus:              pub.Status,
		SourceVisibility:          pub.Visibility,
		SourceOutputHash:          firstNonEmptyStr(canonicalHash, validatedHash, pub.OutputHash),
		SourceBundleHash:          pub.BundleHash,
		SourceActionsRecordCount:  int(pub.ActionsRecordCount),
		SourceTopActionsCount:     int(pub.TopActionsCount),
		SourceRecommendedCount:    int(pub.RecommendedCount),
		DashboardBlocksPresent:    blocks,
		SellerPulseKeyPointsCount: keyPoints,
		ReferencedOzonProductIDs:  productIDs,
		ArtifactFiles:             files,
		Notes: []string{
			"Portable export for staging demo browse; not a Unified Analysis generation fix.",
			fmt.Sprintf("facts_hash=%s", factsHash),
		},
	}
	manifest.ChecksumSHA256, err = checksumExportDir(outputDir, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "checksum: %v\n", err)
		return 1
	}
	manifestRaw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDir, "manifest.json"), manifestRaw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write manifest: %v\n", err)
		return 1
	}
	printJSON(map[string]any{
		"ok":              true,
		"output":          outputDir,
		"publication_id":  pub.ID,
		"run_id":          pub.RunID,
		"metric_date":     manifest.SourceMetricDate,
		"pack_hash":       manifest.SourcePackHash,
		"actions":         len(parsed.Actions),
		"key_points":      keyPoints,
		"checksum_sha256": manifest.ChecksumSHA256,
	})
	return 0
}

func loadLatestArtifactJSON(ctx context.Context, env *unifiedDevtoolEnv, runID int64, kind string) ([]byte, string, bool) {
	var payload []byte
	var hash string
	err := env.Postgres.Pool.QueryRow(ctx, `
SELECT payload_json, coalesce(hash,'')
FROM ai_pipeline_artifacts
WHERE pipeline_run_id=$1 AND kind=$2
ORDER BY id DESC
LIMIT 1`, runID, kind).Scan(&payload, &hash)
	if err != nil {
		return nil, "", false
	}
	return payload, hash, true
}

func extractReferencedOzonProductIDs(factsJSON []byte, out *contract.UnifiedAnalysisOutputV1) []string {
	seen := map[string]struct{}{}
	var facts map[string]any
	_ = json.Unmarshal(factsJSON, &facts)
	if idx, ok := facts["sku_facts_index"].(map[string]any); ok {
		if byID, ok := idx["by_ozon_product_id"].(map[string]any); ok {
			for id := range byID {
				seen[id] = struct{}{}
			}
		}
	}
	raw, _ := json.Marshal(out)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, child := range t {
				lk := strings.ToLower(k)
				if (lk == "ozon_product_id" || lk == "product_id") && child != nil {
					seen[fmt.Sprint(child)] = struct{}{}
				}
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	var asAny any
	_ = json.Unmarshal(raw, &asAny)
	walk(asAny)
	outIDs := make([]string, 0, len(seen))
	for id := range seen {
		if strings.TrimSpace(id) == "" || id == "<nil>" {
			continue
		}
		outIDs = append(outIDs, id)
	}
	return outIDs
}

func checksumExportDir(dir string, files map[string]string) (string, error) {
	h := sha256.New()
	order := []string{"validated_output", "dashboard_facts", "validation", "summary"}
	for _, key := range order {
		name := files[key]
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(name))
		_, _ = h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
