package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publicationaudit"
	"github.com/jackc/pgx/v5/pgtype"
)

// runAuditUnifiedPublication implements `audit-unified-publication`.
//
// It reconciles one published unified-analysis run across the full pipeline:
//
//	Source DB → AnalysisDataSnapshot → Analysis Pack → Dashboard facts →
//	validated AI output → frozen snapshot → Dashboard API model → Actions API model
//
// and writes four reconciliation CSVs plus an aggregate audit-report.json under
// --output. Every CSV row carries a concrete ok/warning/error verdict — the
// audit never emits a NOT_CHECKED row. Exit code is non-zero when any layer
// carries an error finding, so it can gate manual/CI acceptance.
func runAuditUnifiedPublication(args []string) int {
	fs := flag.NewFlagSet("audit-unified-publication", flag.ContinueOnError)
	var (
		sellerID      int64
		publicationID int64
		output        string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required, must match the publication)")
	fs.Int64Var(&publicationID, "publication-id", 0, "unified_analysis_publications.id (required)")
	fs.StringVar(&output, "output", "./audit-unified-publication", "output directory for the reconciliation CSVs + audit-report.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if publicationID <= 0 {
		fmt.Fprintln(os.Stderr, "publication-id is required and must be > 0")
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer env.Close()

	pub, err := env.Queries.GetUnifiedAnalysisPublicationByID(ctx, publicationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get publication: %v\n", err)
		return 1
	}
	if pub.SellerAccountID != sellerID {
		fmt.Fprintf(os.Stderr, "seller-id %d does not match publication %d (seller_account_id=%d)\n", sellerID, publicationID, pub.SellerAccountID)
		return 2
	}
	runID := pub.RunID
	run, err := env.Service.GetRun(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get run %d: %v\n", runID, err)
		return 1
	}
	metricDate := ""
	if !run.MetricDate.IsZero() {
		metricDate = run.MetricDate.Format("2006-01-02")
	} else if pub.MetricDate.Valid {
		metricDate = pub.MetricDate.Time.UTC().Format("2006-01-02")
	}

	repo := unifiedanalysis.NewRepository(env.Queries)
	artStore := unifiedanalysis.NewArtifactStore(repo)

	// -----------------------------------------------------------------
	// Load every layer artifact. Loading failures become error findings in
	// the comparisons below, never skipped rows.
	// -----------------------------------------------------------------

	// Source DB → snapshot, reconciled live + compared to the gated artifact.
	srcInputs := publicationaudit.SourceInputs{}
	if snap, snapErr := analysisdata.NewSQLLoader(env.Postgres.Pool).Load(ctx, sellerID, run.MetricDate); snapErr == nil && snap != nil {
		if rep, recErr := sourcerecon.ReconcileSnapshot(snap); recErr == nil {
			srcInputs.Live = rep
			srcInputs.LiveLoaded = true
		} else {
			fmt.Fprintf(os.Stderr, "warn: reconcile snapshot: %v\n", recErr)
		}
	} else if snapErr != nil {
		fmt.Fprintf(os.Stderr, "warn: load source snapshot: %v\n", snapErr)
	}
	if stored, ok := artStore.LoadSourceReconciliation(ctx, runID); ok {
		srcInputs.Stored = stored
		srcInputs.StoredLoaded = true
	}

	// Analysis Pack → reference index (shared by AI-claims + actions layers).
	var packIndex *outputvalidator.PackReferenceIndex
	packIndexLoaded := false
	if run.PackStorageKey != "" {
		if zipBytes, dlErr := storage.DownloadBytes(ctx, env.S3, env.Cfg.S3.BucketArtifacts, run.PackStorageKey); dlErr == nil {
			if idx, idxErr := outputvalidator.BuildPackReferenceIndex(zipBytes); idxErr == nil {
				packIndex = idx
				packIndexLoaded = true
			} else {
				fmt.Fprintf(os.Stderr, "warn: build pack index: %v\n", idxErr)
			}
		} else {
			fmt.Fprintf(os.Stderr, "warn: download pack zip: %v\n", dlErr)
		}
	}

	// Validated AI output (reuses the shared eval loader).
	aiOutput, aiOutputErr := loadValidatedOutput(ctx, repo, runID)
	if aiOutputErr != nil {
		fmt.Fprintf(os.Stderr, "warn: load validated output: %v\n", aiOutputErr)
	}

	// Dashboard facts + frozen snapshot (Dashboard API model source).
	dashInputs := publicationaudit.DashboardInputs{PackHash: run.PackHash, MetricDate: metricDate}
	if facts, ok := artStore.LoadDashboardFacts(ctx, runID); ok {
		dashInputs.Facts = facts
		dashInputs.FactsLoaded = true
	}
	if snap, snapErr := env.Queries.GetAIDashboardDataSnapshotByPipelineRunID(ctx, runID); snapErr == nil {
		var frozen dashboardhome.FrozenDashboardData
		if json.Unmarshal(snap.PayloadJson, &frozen) == nil {
			dashInputs.Frozen = frozen
			dashInputs.FrozenLoaded = true
		}
	}

	// Persisted actions (Actions API model). Unified publications store
	// action_items under publication.actions_run_id (compat actions run),
	// which can differ from the unified analysis pipeline run id.
	actionsRunID := runID
	if pub.ActionsRunID.Valid && pub.ActionsRunID.Int64 > 0 {
		actionsRunID = pub.ActionsRunID.Int64
	}
	persisted, actErr := loadPersistedActions(ctx, env.Queries, actionsRunID)
	if actErr != nil {
		fmt.Fprintf(os.Stderr, "warn: load action items: %v\n", actErr)
	}

	// -----------------------------------------------------------------
	// Run the pure comparisons.
	// -----------------------------------------------------------------
	meta := outputvalidator.ExpectedRunMeta{PackID: run.PackID, PackHash: run.PackHash, MetricDate: metricDate}
	sourceFindings := publicationaudit.SourceReconciliation(srcInputs)
	aiFindings := publicationaudit.AIClaimsReconciliation(publicationaudit.AIClaimInputs{
		Output:      aiOutput,
		PackIndex:   packIndex,
		IndexLoaded: packIndexLoaded,
		Meta:        meta,
	})
	dashFindings := publicationaudit.DashboardReconciliation(dashInputs)
	actionFindings := publicationaudit.ActionsReconciliation(publicationaudit.ActionsInputs{
		Output:      aiOutput,
		Persisted:   persisted,
		PackIndex:   packIndex,
		IndexLoaded: packIndexLoaded,
	})

	report := publicationaudit.BuildReport(sourceFindings, aiFindings, dashFindings, actionFindings)
	report.SellerAccountID = sellerID
	report.PublicationID = publicationID
	report.RunID = runID
	report.MetricDate = metricDate
	report.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	// -----------------------------------------------------------------
	// Write outputs.
	// -----------------------------------------------------------------
	if err := os.MkdirAll(output, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", output, err)
		return 1
	}
	csvs := []struct {
		name     string
		findings []publicationaudit.Finding
	}{
		{publicationaudit.LayerSource, sourceFindings},
		{publicationaudit.LayerAIClaims, aiFindings},
		{publicationaudit.LayerDashboard, dashFindings},
		{publicationaudit.LayerActions, actionFindings},
	}
	for _, c := range csvs {
		if err := writeAuditFindingsCSV(filepath.Join(output, c.name+".csv"), c.findings); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", c.name, err)
			return 1
		}
	}
	if err := writeAuditReportJSON(filepath.Join(output, "audit-report.json"), report); err != nil {
		fmt.Fprintf(os.Stderr, "write audit-report.json: %v\n", err)
		return 1
	}

	fmt.Printf("audit-unified-publication: seller=%d publication=%d run=%d valid=%t\n", sellerID, publicationID, runID, report.Valid)
	for _, key := range []string{publicationaudit.LayerSource, publicationaudit.LayerAIClaims, publicationaudit.LayerDashboard, publicationaudit.LayerActions} {
		s := report.Layers[key]
		fmt.Printf("  %-26s ok=%d warn=%d err=%d valid=%t\n", key, s.OK, s.Warnings, s.Errors, s.Valid)
	}
	fmt.Printf("report written to %s\n", output)
	if !report.Valid {
		return 1
	}
	return 0
}

// loadPersistedActions reads the persisted action_items rows for a run and
// decodes their evidence_fact_ids into the DB-free audit view.
func loadPersistedActions(ctx context.Context, queries *dbgen.Queries, runID int64) ([]publicationaudit.PersistedAction, error) {
	items, err := queries.ListActionItemsByRunID(ctx, pgtype.Int8{Int64: runID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]publicationaudit.PersistedAction, 0, len(items))
	for _, it := range items {
		out = append(out, publicationaudit.PersistedAction{
			ActionKey:       it.ActionKey,
			EvidenceFactIDs: decodeStringList(it.EvidenceFactIdsJson),
		})
	}
	return out, nil
}

func decodeStringList(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func writeAuditFindingsCSV(path string, findings []publicationaudit.Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"check", "status", "entity_id", "expected", "actual", "detail"}); err != nil {
		return err
	}
	for _, fnd := range findings {
		if err := w.Write([]string{
			fnd.Check,
			string(fnd.Status),
			fnd.EntityID,
			fnd.Expected,
			fnd.Actual,
			fnd.Detail,
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeAuditReportJSON(path string, report publicationaudit.Report) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
