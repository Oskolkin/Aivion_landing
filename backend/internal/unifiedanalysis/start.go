package unifiedanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

// StartRun idempotently advances a run through pack build, container setup, upload, and response start.
func (s *Service) StartRun(ctx context.Context, runID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("unified analysis service is not configured")
	}
	if !s.cfg.Enabled {
		return ErrProviderDisabled
	}
	if s.provider == nil {
		return errorf(CodeProviderDisabled, "provider is not configured")
	}

	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Mode != ModeUnifiedAnalysis {
		return ErrInconsistentRunState
	}
	if IsTerminalRunStatus(run.Status) {
		return nil
	}
	if run.ProviderResponseID != "" {
		return nil
	}

	for {
		switch run.Status {
		case StatusCreated, StatusPackBuilding, StatusPackValidating:
			if err := s.buildAndSavePack(ctx, run); err != nil {
				return err
			}
		case StatusPackReady, StatusProviderContainerCreating:
			if run.ProviderContainerID == "" {
				if err := s.ensureStatus(ctx, run, StatusProviderContainerCreating); err != nil {
					return err
				}
				container, err := s.provider.CreateContainer(ctx, provider.CreateContainerRequest{
					Name:        fmt.Sprintf("aivion-unified-run-%d", run.ID),
					MemoryLimit: strings.ToLower(strings.TrimSpace(s.cfg.MemoryLimit)),
				})
				if err != nil {
					return s.failRun(ctx, run.ID, CodeProviderFailed, err)
				}
				run, err = s.repo.SaveContainer(ctx, run.ID, container.ID)
				if err != nil {
					return err
				}
				if err := s.transition(ctx, run.ID, run.Status, StatusProviderContainerReady); err != nil {
					return err
				}
			}
		case StatusProviderContainerReady, StatusProviderFileUploading:
			if run.ProviderContainerFileID == "" {
				if err := s.ensureStatus(ctx, run, StatusProviderFileUploading); err != nil {
					return err
				}
				zipBytes, err := s.loadPackZip(ctx, run)
				if err != nil {
					return s.failRun(ctx, run.ID, CodeInconsistentRunState, err)
				}
				genReport, genErr := pack.ValidateAnalysisPackForGeneration(zipBytes, run.PackID, run.PackHash, pack.BuildOptions{})
				if genErr != nil {
					return s.failRun(ctx, run.ID, CodePackValidationFailed, genErr)
				}
				if s.artifacts != nil {
					_ = s.artifacts.SavePackGenerationValidation(ctx, run.ID, genReport)
				}
				if !genReport.Valid {
					return s.failRun(ctx, run.ID, CodePackValidationFailed, fmt.Errorf("%w: %s", pack.ErrValidationFailed, formatGenerationValidationIssues(genReport)))
				}
				archiveSHA := sha256.Sum256(zipBytes)
				file, err := s.provider.UploadContainerFile(ctx, provider.UploadContainerFileRequest{
					ContainerID: run.ProviderContainerID,
					Filename:    PackFilename,
					ContentType: "application/zip",
					Content:     zipBytes,
					SHA256:      hex.EncodeToString(archiveSHA[:]),
				})
				if err != nil {
					return s.failRun(ctx, run.ID, CodeProviderFailed, err)
				}
				run, err = s.repo.SaveContainerFile(ctx, run.ID, file.ID, file.Path)
				if err != nil {
					return err
				}
				if err := s.transition(ctx, run.ID, run.Status, StatusProviderFileReady); err != nil {
					return err
				}
			}
		case StatusProviderFileReady, StatusProviderResponseStarting:
			if run.ProviderResponseID == "" {
				if err := s.ensureStatus(ctx, run, StatusProviderResponseStarting); err != nil {
					return err
				}
				started, err := s.buildAndStartProviderResponse(ctx, run)
				if err != nil {
					return s.failRun(ctx, run.ID, CodeProviderFailed, err)
				}
				resp := started.Response
				if s.artifacts != nil {
					summary := started.Summary
					summary.Attempt = int(run.ProviderAttemptCount) + 1
					_ = s.artifacts.SaveProviderRequestSummary(ctx, run.ID, summary)
				}
				now := s.now()
				run, err = s.repo.SaveResponseStarted(ctx, run.ID, dbgen.SaveUnifiedAnalysisResponseStartedParams{
					ProviderResponseID: nullableText(resp.ID),
					ProviderStatus:     nullableText(string(resp.Status)),
					ProviderStartedAt:  timestamptzNow(now),
					ProviderRequestID:  nullableText(resp.RequestID),
				})
				if err != nil {
					return err
				}
				if err := s.transition(ctx, run.ID, run.Status, StatusProviderRunning); err != nil {
					return err
				}
			}
			return nil
		case StatusProviderRunning:
			return nil
		default:
			return errorf(CodeInconsistentRunState, "cannot start run from status %q", run.Status)
		}

		run, err = s.repo.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		if run.ProviderResponseID != "" {
			return nil
		}
	}
}

func (s *Service) buildAndSavePack(ctx context.Context, run *Run) error {
	if s.packs == nil {
		return errorf(CodeInconsistentRunState, "pack builder is not configured")
	}
	if run.PackID != "" && run.PackStorageKey != "" {
		if run.Status != StatusPackReady {
			// Early-exit path: the pack was already built and persisted by a
			// prior invocation. We must still gate on source reconciliation
			// before allowing the run to advance toward OpenAI. The snapshot
			// is not available here, so require a passing reconciliation
			// artifact on record; a missing or failing report blocks.
			if err := s.enforceReconciliationArtifact(ctx, run.ID); err != nil {
				return err
			}
			return s.transition(ctx, run.ID, run.Status, StatusPackReady)
		}
		return nil
	}

	if err := s.transitionFrom(ctx, run, StatusPackBuilding); err != nil {
		return err
	}
	if err := s.transition(ctx, run.ID, StatusPackBuilding, StatusPackValidating); err != nil && !errors.Is(err, ErrCASConflict) {
		return err
	}

	result, storageKey, err := s.packs.BuildAndStore(ctx, run.SellerAccountID, run.MetricDate)
	if err != nil {
		if errors.Is(err, pack.ErrValidationFailed) || !resultValidationValid(result) {
			return s.failRun(ctx, run.ID, CodePackValidationFailed, err)
		}
		return s.failRun(ctx, run.ID, CodeInconsistentRunState, err)
	}
	if result == nil || !resultValidationValid(result) {
		return s.failRun(ctx, run.ID, CodePackValidationFailed, pack.ErrValidationFailed)
	}

	archiveSHA := sha256.Sum256(result.ZipBytes)
	compressed := int64(len(result.ZipBytes))
	uncompressed := result.Validation.UncompressedBytes
	if uncompressed == 0 {
		uncompressed = compressed
	}

	s.cachePackZip(run.ID, result.ZipBytes)

	_, err = s.repo.SavePack(ctx, run.ID, dbgen.SaveUnifiedAnalysisPackParams{
		PackID:                nullableText(result.PackID),
		PackHash:              nullableText(result.PackHash),
		PackStorageKey:        nullableText(storageKey),
		PackArchiveSha256:     nullableText(hex.EncodeToString(archiveSHA[:])),
		PackCompressedBytes:   pgtype.Int8{Int64: compressed, Valid: true},
		PackUncompressedBytes: pgtype.Int8{Int64: uncompressed, Valid: true},
		DataPackSchemaVersion: nullableText(result.SchemaVersion),
	})
	if err != nil {
		return err
	}

	if s.artifacts != nil {
		_ = s.artifacts.SavePackReference(ctx, run.ID, packReferenceArtifact{
			PackID:        result.PackID,
			PackHash:      result.PackHash,
			StorageKey:    storageKey,
			SchemaVersion: result.SchemaVersion,
		})
	}

	if err := s.saveDashboardFactsFromPack(ctx, run.ID, result); err != nil {
		return err
	}

	// Pre-OpenAI reconciliation gate: the raw snapshot backing this pack
	// must reconcile (account↔SKU totals, no revenue-without-orders) before
	// the run advances toward OpenAI upload/publication. The detailed report
	// is saved on success and failure; a failing report blocks the run.
	if err := s.gateSourceReconciliation(ctx, run.ID, result); err != nil {
		return err
	}

	if err := s.transition(ctx, run.ID, StatusPackValidating, StatusPackReady); err != nil && !errors.Is(err, ErrCASConflict) {
		return err
	}
	return nil
}

// gateSourceReconciliation runs source reconciliation on the pack's raw
// snapshot, persists the report artifact (even on failure), and fails the
// run with CodeSourceReconciliationFailed when the snapshot does not
// reconcile. A failed gate means no OpenAI upload and no publication.
func (s *Service) gateSourceReconciliation(ctx context.Context, runID int64, result *pack.BuildResult) error {
	if result == nil || result.Snapshot == nil {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, fmt.Errorf("pack snapshot is missing for reconciliation"))
	}
	report, err := sourcerecon.ReconcileSnapshot(result.Snapshot)
	if err != nil {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, err)
	}
	if s.artifacts != nil {
		_ = s.artifacts.SaveSourceReconciliation(ctx, runID, report)
	}
	if !report.Valid {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, fmt.Errorf("source reconciliation failed: %s", report.Error()))
	}
	return nil
}

// enforceReconciliationArtifact blocks the early-exit (pack already built)
// path unless a passing source reconciliation report exists for the run.
// A missing report cannot prove the snapshot reconciled, so it blocks.
func (s *Service) enforceReconciliationArtifact(ctx context.Context, runID int64) error {
	if s.artifacts == nil {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, fmt.Errorf("artifact store is not configured for reconciliation gate"))
	}
	report, ok := s.artifacts.LoadSourceReconciliation(ctx, runID)
	if !ok {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, fmt.Errorf("no source reconciliation report on record for run"))
	}
	if !report.Valid {
		return s.failRun(ctx, runID, CodeSourceReconciliationFailed, fmt.Errorf("source reconciliation failed: %s", report.Error()))
	}
	return nil
}

func resultValidationValid(result *pack.BuildResult) bool {
	return result != nil && result.Validation.Valid
}

func formatGenerationValidationIssues(report pack.GenerationValidationReport) string {
	if len(report.Issues) == 0 {
		return "pack generation validation failed"
	}
	parts := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		if issue.File != "" {
			parts = append(parts, fmt.Sprintf("%s:%s:%s", issue.Code, issue.File, issue.Message))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", issue.Code, issue.Message))
	}
	return strings.Join(parts, "; ")
}

func (s *Service) saveDashboardFactsFromPack(ctx context.Context, runID int64, result *pack.BuildResult) error {
	if s.artifacts == nil {
		return errorf(CodeInconsistentRunState, "artifact store is not configured")
	}
	if result == nil || result.Snapshot == nil {
		return errorf(CodeInconsistentRunState, "pack snapshot is missing")
	}
	if existing, ok := s.artifacts.LoadDashboardFacts(ctx, runID); ok {
		if existing.PackHash == result.PackHash && existing.FactsHash != "" {
			return nil
		}
	}

	facts, report, err := dashboardfacts.BuildDashboardFacts(result.Snapshot, dashboardfacts.PackIdentity{
		PackID:   result.PackID,
		PackHash: result.PackHash,
	})
	if err != nil {
		return s.failRun(ctx, runID, CodeFactsValidationFailed, err)
	}
	if !report.Valid {
		return s.failRun(ctx, runID, CodeFactsValidationFailed, fmt.Errorf("dashboard facts validation failed"))
	}
	if err := s.artifacts.SaveDashboardFacts(ctx, runID, facts); err != nil {
		return s.failRun(ctx, runID, CodeFactsValidationFailed, err)
	}
	return nil
}

func (s *Service) loadPackZip(ctx context.Context, run *Run) ([]byte, error) {
	if cached := s.getCachedPackZip(run.ID); len(cached) > 0 {
		return cached, nil
	}
	if s.packStore == nil {
		return nil, fmt.Errorf("pack object store is not configured")
	}
	if strings.TrimSpace(run.PackStorageKey) == "" {
		return nil, fmt.Errorf("pack storage key is empty")
	}
	return s.packStore.Get(ctx, run.PackStorageKey)
}

func (s *Service) cachePackZip(runID int64, zip []byte) {
	s.packZipMu.Lock()
	defer s.packZipMu.Unlock()
	s.packZip[runID] = append([]byte(nil), zip...)
}

func (s *Service) getCachedPackZip(runID int64) []byte {
	s.packZipMu.Lock()
	defer s.packZipMu.Unlock()
	raw := s.packZip[runID]
	if len(raw) == 0 {
		return nil
	}
	return append([]byte(nil), raw...)
}

func (s *Service) transitionFrom(ctx context.Context, run *Run, target string) error {
	if run.Status == target {
		return nil
	}
	if !CanTransition(ModeUnifiedAnalysis, run.Status, target) {
		return errorf(CodeInconsistentRunState, "invalid transition %q -> %q", run.Status, target)
	}
	_, err := s.repo.TransitionStatus(ctx, run.ID, run.Status, target)
	if errors.Is(err, ErrCASConflict) {
		current, getErr := s.repo.GetRun(ctx, run.ID)
		if getErr != nil {
			return getErr
		}
		if current.Status == target {
			return nil
		}
	}
	return err
}

func (s *Service) transition(ctx context.Context, runID int64, from, to string) error {
	if from == to {
		return nil
	}
	if !CanTransition(ModeUnifiedAnalysis, from, to) {
		return errorf(CodeInconsistentRunState, "invalid transition %q -> %q", from, to)
	}
	_, err := s.repo.TransitionStatus(ctx, runID, from, to)
	if errors.Is(err, ErrCASConflict) {
		current, getErr := s.repo.GetRun(ctx, runID)
		if getErr != nil {
			return getErr
		}
		if current.Status == to {
			return nil
		}
	}
	return err
}

func (s *Service) ensureStatus(ctx context.Context, run *Run, target string) error {
	if run.Status == target {
		return nil
	}
	return s.transition(ctx, run.ID, run.Status, target)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Service) promptProfileForRun(run *Run) prompt.PromptProfile {
	if run != nil {
		if profile := prompt.ProfileForVersion(run.PromptVersion); profile == prompt.PromptProfileAnalysis {
			return profile
		}
	}
	return s.promptProfile()
}

func (s *Service) promptProfile() prompt.PromptProfile {
	switch strings.ToLower(strings.TrimSpace(s.cfg.PromptProfile)) {
	case string(prompt.PromptProfileAnalysis), "production", "analyst":
		return prompt.PromptProfileAnalysis
	default:
		return prompt.PromptProfileSmoke
	}
}

func (s *Service) renderProviderPrompt(ctx context.Context, run *Run) (prompt.RenderedPrompt, *provider.StructuredOutputFormat, error) {
	profile := s.promptProfileForRun(run)
	var rendered prompt.RenderedPrompt
	var err error
	if profile == prompt.PromptProfileAnalysis {
		version := prompt.CurrentAnalystVersion
		if run != nil && strings.TrimSpace(run.PromptVersion) != "" {
			version = strings.TrimSpace(run.PromptVersion)
		} else if ver := strings.TrimSpace(s.cfg.PromptVersion); ver != "" {
			version = ver
		}
		rendered, err = prompt.RenderByVersion(version)
		if err != nil {
			return prompt.RenderedPrompt{}, nil, err
		}
		rendered.Profile = profile
	} else {
		rendered, err = prompt.Render(profile)
		if err != nil {
			return prompt.RenderedPrompt{}, nil, err
		}
	}
	if rendered.Profile == prompt.PromptProfileAnalysis && run != nil {
		rendered.Input = prompt.AppendPackIdentityInput(
			rendered.Input,
			run.PackID,
			run.PackHash,
			run.MetricDate.UTC().Format("2006-01-02"),
		)
		if zipBytes, zipErr := s.loadPackZip(ctx, run); zipErr == nil && len(zipBytes) > 0 {
			if idx, idxErr := outputvalidator.BuildPackReferenceIndex(zipBytes); idxErr == nil && idx != nil {
				rendered.Input = prompt.AppendEntityIDExamplesInput(
					rendered.Input,
					bareEntityIDs(idx.SKUIds, "sku:", 12),
					bareEntityIDs(idx.CampaignIds, "campaign:", 12),
				)
				rendered.Input = prompt.AppendAccountSummaryGroundingInput(
					rendered.Input,
					idx.SellerPulseGroundingLines(),
				)
			}
		}
	}
	if rendered.Profile != prompt.PromptProfileAnalysis {
		return rendered, nil, nil
	}
	schema, err := contract.SchemaJSON()
	if err != nil {
		return prompt.RenderedPrompt{}, nil, err
	}
	format := &provider.StructuredOutputFormat{
		Type:   "json_schema",
		Name:   contract.SchemaName(),
		Strict: true,
		Schema: schema,
	}
	return rendered, format, nil
}

func bareEntityIDs(m map[string]struct{}, prefix string, limit int) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	seen := map[string]struct{}{}
	for k := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if strings.HasPrefix(k, prefix) {
			k = strings.TrimPrefix(k, prefix)
		}
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Service) markFailed(ctx context.Context, runID int64, code string, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_, markErr := s.repo.MarkFailed(ctx, runID, dbgen.MarkUnifiedAnalysisFailedParams{
		ErrorCode:     nullableText(code),
		ErrorMessage:  nullableText(msg),
		CleanupStatus: nullableText(CleanupStatusPending),
	})
	return markErr
}

func (s *Service) failRun(ctx context.Context, runID int64, code string, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if markErr := s.markFailed(ctx, runID, code, err); markErr != nil {
		return markErr
	}
	return wrapError(code, msg, err)
}
