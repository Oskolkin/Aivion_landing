package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// GenerationService orchestrates the AI action generation pipeline.
type GenerationService struct {
	candidateBuilder *CandidateBuilder
	ai               ActionAIClient
	repo             GenerationRepository
	dataPackBuilder  AIDataPackBuilder
}

// NewGenerationService creates a generation service.
func NewGenerationService(builder *CandidateBuilder, ai ActionAIClient, repo GenerationRepository) *GenerationService {
	return &GenerationService{
		candidateBuilder: builder,
		ai:               ai,
		repo:             repo,
	}
}

// WithAIDataPackBuilder wires the Seller AI Data Pack builder used by the
// AI-first pipeline (ACTIONS_AI_FIRST=true). The candidate-first pipeline
// never uses it.
func (s *GenerationService) WithAIDataPackBuilder(builder AIDataPackBuilder) *GenerationService {
	s.dataPackBuilder = builder
	return s
}

// GenerateActions runs the generation pipeline for a seller and metric date.
// With ACTIONS_AI_FIRST=false (default) the legacy candidate-first pipeline
// runs unchanged; with ACTIONS_AI_FIRST=true the AI-first pipeline generates
// actions directly from the Seller AI Data Pack.
func (s *GenerationService) GenerateActions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	opts = normalizeGenerateOptions(opts)
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	if opts.AIFirst {
		return s.generateActionsAIFirst(ctx, sellerAccountID, metricDate, opts)
	}
	return s.generateActionsCandidateFirst(ctx, sellerAccountID, metricDate, opts)
}

// generateActionsCandidateFirst is the unchanged legacy candidate-first pipeline.
func (s *GenerationService) generateActionsCandidateFirst(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	var runID int64
	if !opts.DryRun {
		var createErr error
		runID, createErr = s.repo.CreateActionGenerationRun(ctx, sellerAccountID, metricDate, GenerationRunStatusRunning)
		if createErr != nil {
			return nil, fmt.Errorf("create generation run: %w", createErr)
		}
		s.candidateBuilder.BindActionsRunID(runID)
	}

	candidateCtx, err := s.candidateBuilder.BuildCandidateContext(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, fmt.Errorf("build candidate context: %w", err)
	}
	contextHash := hashCandidateContext(candidateCtx)

	domainResults := make([]DomainPipelineResult, 0, 4)

	if !opts.DryRun && runID > 0 {
		defer func() {
			if err != nil {
				s.finalizeRunOnError(ctx, runID, contextHash, candidateCtx, domainResults)
			}
		}()
	}

	for _, domain := range domainOrder() {
		domainCtx, _ := findDomainContext(candidateCtx, domain)
		dr := s.processDomain(ctx, candidateCtx, domainCtx, opts)
		domainResults = append(domainResults, dr)

		if !opts.DryRun && runID > 0 {
			domainModel := rawResponseModel(dr.RawResponse)
			if rm := rawResponseModel(dr.RepairResponse); rm != "" {
				domainModel = rm
			}
			_, catErr := s.repo.CreateActionGenerationCategoryRun(ctx, CreateCategoryRunParams{
				RunID:               runID,
				SellerAccountID:     sellerAccountID,
				Domain:              domain,
				Status:              dr.Status,
				Model:               domainModel,
				InputContextJSON:    dr.InputContext,
				RawResponseJSON:     rawResponseJSON(dr.RawResponse),
				ValidationJSON:      marshalJSON(dr.Validation),
				RepairResponseJSON:  rawResponseJSON(dr.RepairResponse),
				FinalValidationJSON: marshalJSON(dr.FinalValidation),
				AcceptedCount:       len(dr.AcceptedActions),
				RepairedCount:       boolToInt(dr.Repaired),
				DroppedCount:        dr.DroppedCount,
			})
			if catErr != nil {
				slog.Warn("actions: create category run failed", "domain", domain, "err", catErr)
			}
		}
	}

	allDrafts := scoreDomainResults(candidateCtx, domainResults)
	merged := MergeAndDedup(allDrafts)

	var (
		globalRaw            *AIRawResponse
		globalStatus         = CategoryRunStatusCompleted
		globalModel          string
		globalInputJSON      []byte
		globalValidationJSON []byte
		globalFinalJSON      []byte
	)
	ordered, top, aiFocus := OrderDraftsByScore(merged), []ScoredDraftAction{}, (*ActionsAIFocus)(nil)
	if len(merged) > 0 {
		if opts.EnableLegacyGlobalCheck && opts.AIEnabled && s.ai != nil {
			reviewInput := LegacyGlobalCheckInput{
				MetricDate:       candidateCtx.MetricDate,
				AccountSnapshot:  candidateCtx.AccountSnapshot,
				DraftActions:     buildDraftsForLegacyGlobalCheck(merged),
				MaxTopActions:    opts.MaxTopActions,
				MaxActionsOutput: opts.MaxActionsOutput,
				ForbiddenClaims:  candidateCtx.ForbiddenClaims,
			}
			globalInputJSON = marshalJSON(buildLegacyGlobalCheckInputContext(reviewInput, merged, candidateCtx.MetricDate))
			reviewOut, raw, reviewErr := s.ai.RunLegacyGlobalCheck(ctx, reviewInput)
			globalRaw = raw
			globalModel = rawResponseModel(raw)
			if reviewErr != nil {
				slog.Warn("actions: legacy global check failed", "err", reviewErr)
				globalStatus = CategoryRunStatusFallback
				gv := LegacyGlobalCheckValidationResult{}
				diag := buildLegacyGlobalCheckDiagnostic("global_review_call", globalStatus, gv)
				diag.Reason = "legacy_global_check_error"
				diag.ParseError = reviewErr.Error()
				diag.FallbackScope = "full"
				globalValidationJSON = marshalJSON(diag)
				globalFinalJSON = globalValidationJSON
				ordered, top, aiFocus = ApplyLegacyGlobalCheckOrdering(merged, gv, opts)
			} else {
				gv := ValidateLegacyGlobalCheckOutput(reviewOut, draftsByTempID(merged), opts, candidateCtx.ForbiddenClaims, candidateCtx.AccountSnapshot)
				globalStatus = legacyGlobalCheckCategoryStatus(gv)
				globalValidationJSON = marshalJSON(buildLegacyGlobalCheckDiagnostic("global_review_validate", globalStatus, gv))
				ordered, top, aiFocus = ApplyLegacyGlobalCheckOrdering(merged, gv, opts)
				finalDiag := buildLegacyGlobalCheckDiagnostic("global_review_apply", globalStatus, gv)
				// Strip unsupported numeric claims from an applied AI focus before persist
				// so the user never sees a number the validator flagged as unsupported.
				if aiFocus != nil && strings.EqualFold(aiFocus.Source, "ai") {
					allowed := aiFocusAllowedNumbers(draftsByTempID(merged), candidateCtx.AccountSnapshot)
					aiFocus = cloneAIFocus(aiFocus)
					if removed := SanitizeAIFocusNumbers(aiFocus, allowed); len(removed) > 0 {
						finalDiag.SanitizedNumbers = removed
						for i := range finalDiag.Issues {
							if finalDiag.Issues[i].Code == "unsupported_ai_focus_number" {
								finalDiag.Issues[i].Code = "sanitized_ai_focus_number"
								finalDiag.Issues[i].Severity = IssueSeverityWarning
							}
						}
					}
				}
				if aiFocus != nil {
					finalDiag.FocusSource = aiFocus.Source
					finalDiag.FocusApplied = strings.EqualFold(aiFocus.Source, "ai")
				}
				globalFinalJSON = marshalJSON(finalDiag)
			}
		} else {
			ordered, top, aiFocus = ApplyLegacyGlobalCheckOrdering(merged, LegacyGlobalCheckValidationResult{}, opts)
		}
	} else {
		aiFocus = BuildDeterministicAIFocus(nil)
	}

	finalItems := make([]ActionItem, 0, len(ordered))
	topKeys := make([]string, 0, len(top))
	for _, d := range ordered {
		item := MapDomainActionToItem(candidateCtx.MetricDate, runID, d, candidateCtx)
		if err := ValidatePersistableActionItem(item); err != nil {
			slog.Warn("actions: skip invalid mapped item", "type", item.Type, "err", err)
			continue
		}
		finalItems = append(finalItems, item)
	}
	for _, d := range top {
		key := BuildActionKey(candidateCtx.MetricDate, d.Action.Type, d.Action.SourceCandidateIDs, d.EntityGroupRef)
		topKeys = append(topKeys, key)
	}

	accepted, repaired, dropped, fallback := aggregateCounts(domainResults)
	finalStatus := computeFinalStatus(domainResults, len(finalItems), !opts.AIEnabled, repaired, dropped)

	result := &GenerateActionsResult{
		RunID:         runID,
		MetricDate:    candidateCtx.MetricDate,
		FinalStatus:   finalStatus,
		Actions:       finalItems,
		TopActionKeys: topKeys,
		AIFocus:       aiFocus,
		AcceptedCount: accepted,
		RepairedCount: repaired,
		DroppedCount:  dropped,
		FallbackCount: fallback,
		DryRun:        opts.DryRun,
	}

	if opts.DryRun {
		return result, nil
	}

	if finalStatus == GenerationFinalStatusFailed || finalStatus == GenerationFinalStatusEmpty {
		_ = s.repo.UpdateActionGenerationRun(ctx, UpdateGenerationRunParams{
			RunID:            runID,
			Status:           GenerationRunStatusCompleted,
			ContextHash:      contextHash,
			FinalStatus:      finalStatus,
			AcceptedCount:    accepted,
			RepairedCount:    repaired,
			DroppedCount:     dropped,
			FallbackCount:    fallback,
			AIFocusJSON:      marshalJSON(aiFocus),
			DataQualityNotes: marshalJSON(candidateCtx.DataQualityNotes),
		})
		return result, nil
	}

	upserts := make([]UpsertActionItemParams, 0, len(finalItems))
	keepKeys := make([]string, 0, len(finalItems))
	for _, item := range finalItems {
		params, err := actionItemToUpsertParams(sellerAccountID, runID, metricDate, item)
		if err != nil {
			return nil, err
		}
		upserts = append(upserts, params)
		keepKeys = append(keepKeys, item.ActionKey)
	}

	aiFocusJSON, _ := json.Marshal(aiFocus)
	topKeysJSON, _ := json.Marshal(topKeys)
	dqJSON, _ := json.Marshal(candidateCtx.DataQualityNotes)

	err = s.repo.PersistGenerationAtomic(ctx, PersistGenerationParams{
		RunUpdate: UpdateGenerationRunParams{
			RunID:             runID,
			Status:            GenerationRunStatusCompleted,
			ContextHash:       contextHash,
			FinalStatus:       finalStatus,
			AcceptedCount:     accepted,
			RepairedCount:     repaired,
			DroppedCount:      dropped,
			FallbackCount:     fallback,
			AIFocusJSON:       aiFocusJSON,
			TopActionKeysJSON: topKeysJSON,
			DataQualityNotes:  dqJSON,
		},
		ActionItems: upserts,
		StaleDelete: &StaleDeleteParams{
			RunID:           runID,
			SellerAccountID: sellerAccountID,
			MetricDate:      metricDate,
			KeepKeys:        keepKeys,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("persist generation: %w", err)
	}

	if runID > 0 {
		_, _ = s.repo.CreateActionGenerationCategoryRun(ctx, CreateCategoryRunParams{
			RunID:               runID,
			SellerAccountID:     sellerAccountID,
			Domain:              DomainLegacyGlobalCheck,
			Status:              globalStatus,
			Model:               globalModel,
			InputContextJSON:    globalInputJSON,
			RawResponseJSON:     rawResponseJSON(globalRaw),
			ValidationJSON:      globalValidationJSON,
			FinalValidationJSON: globalFinalJSON,
			AcceptedCount:       len(top),
		})
	}

	return result, nil
}

func normalizeGenerateOptions(opts GenerateActionsOptions) GenerateActionsOptions {
	def := DefaultGenerateActionsOptions()
	if opts.MaxActionsOutput == 0 {
		opts.MaxActionsOutput = def.MaxActionsOutput
	}
	if opts.MaxTopActions == 0 {
		opts.MaxTopActions = def.MaxTopActions
	}
	// When caller passes a zero-value struct, apply full defaults.
	if opts == (GenerateActionsOptions{}) {
		return def
	}
	return opts
}

func scoreDomainResults(candidateCtx *ActionCandidateContext, results []DomainPipelineResult) []ScoredDraftAction {
	out := make([]ScoredDraftAction, 0)
	for _, dr := range results {
		domainCtx, _ := findDomainContext(candidateCtx, dr.Domain)
		for i, action := range dr.AcceptedActions {
			candidates := resolveSourceCandidates(candidateCtx, action)
			score, priority := ScoreDraftAction(action, domainCtx.Status, candidates)
			priority = OverridePriority(action.Priority, priority, score)
			out = append(out, ScoredDraftAction{
				TempActionID:     fmt.Sprintf("%s_%d", dr.Domain, i),
				Action:           action,
				Score:            score,
				Priority:         priority,
				Domain:           dr.Domain,
				EntityGroupRef:   entityGroupRefFromCandidates(candidates),
				SourceCandidates: candidates,
			})
		}
	}
	return out
}

func aggregateCounts(results []DomainPipelineResult) (accepted, repaired, dropped, fallback int) {
	for _, r := range results {
		accepted += len(r.AcceptedActions)
		dropped += r.DroppedCount
		if r.Repaired {
			repaired++
		}
		if r.UsedFallback {
			fallback++
		}
	}
	return
}

func computeFinalStatus(results []DomainPipelineResult, finalCount int, aiDisabled bool, repairedCount int, droppedCount int) string {
	if finalCount == 0 {
		allFailed := true
		for _, r := range results {
			if r.Status != CategoryRunStatusFailed && r.Status != CategoryRunStatusSkippedEmpty {
				allFailed = false
				break
			}
		}
		if allFailed {
			return GenerationFinalStatusEmpty
		}
		return GenerationFinalStatusFailed
	}
	completed := 0
	failed := 0
	producing := 0
	fallbackProducing := 0
	partialDomains := 0
	repairedDomains := 0
	for _, r := range results {
		switch r.Status {
		case CategoryRunStatusSkippedEmpty:
			continue
		case CategoryRunStatusFailed:
			failed++
			continue
		}
		// Any non-failed, non-skipped domain produced actions.
		producing++
		completed++
		switch r.Status {
		case CategoryRunStatusCompletedPartial:
			partialDomains++
		case CategoryRunStatusRepair, CategoryRunStatusCompletedRepaired:
			repairedDomains++
		}
		if r.UsedFallback {
			fallbackProducing++
		}
	}
	// Full deterministic fallback only when AI is disabled or every producing domain fell back.
	if aiDisabled || (producing > 0 && fallbackProducing == producing) {
		if finalCount > 0 {
			return GenerationFinalStatusFallback
		}
	}
	if failed > 0 && completed > 0 {
		return GenerationFinalStatusCompletedPartial
	}
	if partialDomains > 0 || fallbackProducing > 0 || droppedCount > 0 {
		return GenerationFinalStatusCompletedPartial
	}
	if repairedDomains > 0 || repairedCount > 0 {
		return GenerationFinalStatusCompletedRepaired
	}
	return GenerationFinalStatusCompleted
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func actionItemToUpsertParams(sellerID, runID int64, metricDate time.Time, item ActionItem) (UpsertActionItemParams, error) {
	evidenceJSON, err := json.Marshal(item.Evidence)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	limitationsJSON, err := json.Marshal(item.Limitations)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	ctaJSON, err := json.Marshal(item.CTAs)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	affectedJSON, err := json.Marshal(item.AffectedEntities)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	sourceIDsJSON, err := json.Marshal(item.SourceCandidateIDs)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	factIDsJSON, err := json.Marshal(item.EvidenceFactIDs)
	if err != nil {
		return UpsertActionItemParams{}, err
	}
	return UpsertActionItemParams{
		SellerAccountID:      sellerID,
		RunID:                runID,
		ActionKey:            item.ActionKey,
		Type:                 item.Type,
		Category:             item.Category,
		Priority:             item.Priority,
		Title:                item.Title,
		Summary:              item.Summary,
		Reason:               item.Reason,
		NextStep:             item.NextStep,
		ExpectedEffect:       item.ExpectedEffect,
		EvidenceJSON:         evidenceJSON,
		LimitationsJSON:      limitationsJSON,
		CTAJSON:              ctaJSON,
		AffectedEntitiesJSON: affectedJSON,
		EntityGroupRef:       item.EntityGroupRef,
		SourceCandidateIDs:   sourceIDsJSON,
		EvidenceFactIDs:      factIDsJSON,
		PayloadJSON:          MarshalActionItemPayloadJSON(item),
		MetricDate:           metricDate,
	}, nil
}

// finalizeRunOnError marks a generation run as completed/failed when pipeline errors after run creation.
func (s *GenerationService) finalizeRunOnError(
	ctx context.Context,
	runID int64,
	contextHash string,
	candidateCtx *ActionCandidateContext,
	domainResults []DomainPipelineResult,
) {
	if runID <= 0 {
		return
	}
	accepted, repaired, dropped, fallback := aggregateCounts(domainResults)
	dqJSON := marshalJSON(candidateCtx.DataQualityNotes)
	_ = s.repo.UpdateActionGenerationRun(ctx, UpdateGenerationRunParams{
		RunID:            runID,
		Status:           GenerationRunStatusCompleted,
		ContextHash:      contextHash,
		FinalStatus:      GenerationFinalStatusFailed,
		AcceptedCount:    accepted,
		RepairedCount:    repaired,
		DroppedCount:     dropped,
		FallbackCount:    fallback,
		DataQualityNotes: dqJSON,
	})
}
