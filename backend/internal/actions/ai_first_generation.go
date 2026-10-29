package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// generateActionsAIFirst runs the AI-first pipeline (Stage 2):
//
//	Seller AI Data Pack -> ActionsAIFirstInput -> AI-first prompt ->
//	AI finds problems and returns self-sufficient actions ->
//	backend validates facts/entities/numbers/claims ->
//	valid actions persist as regular action_items.
//
// CandidateBuilder and backend candidates are never used on this path.
func (s *GenerationService) generateActionsAIFirst(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	input, err := s.BuildActionsAIFirstInput(ctx, sellerAccountID, metricDate, opts)
	if err != nil {
		return nil, err
	}
	return s.GenerateActionsAIFirstFromInput(ctx, input, opts)
}

// GenerateActionsAIFirstFromInput runs AI-first generation from a prebuilt input.
func (s *GenerationService) GenerateActionsAIFirstFromInput(
	ctx context.Context,
	input ActionsAIFirstInput,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	if input.IsV2CandidateInput() {
		return s.generateActionsAIFirstV2FromInput(ctx, input, opts)
	}
	return s.generateActionsAIFirstV1FromInput(ctx, input, opts)
}

func (s *GenerationService) generateActionsAIFirstV1FromInput(
	ctx context.Context,
	input ActionsAIFirstInput,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	metricDateStr := input.MetricDate
	metricDate, err := time.Parse("2006-01-02", metricDateStr)
	if err != nil {
		return nil, fmt.Errorf("ai-first: invalid metric_date %q: %w", metricDateStr, err)
	}
	sellerAccountID := input.SellerAccountID

	var runID int64
	if !opts.DryRun {
		runID, err = s.beginAIFirstGenerationRun(ctx, sellerAccountID, metricDate, input, opts)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				s.finalizeAIFirstRunOnError(ctx, runID, input.DataPackHash)
			}
		}()
	}

	aiClient, availability := resolveAIFirstClientAvailability(s.ai, opts)
	if availability != "" {
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID:            runID,
			SellerID:         sellerAccountID,
			MetricDate:       metricDate,
			MetricDateStr:    metricDateStr,
			Input:            input,
			FinalStatus:      GenerationFinalStatusFailed,
			FailureSubreason: availability,
			ComposeCalls:     0,
			Notes:            []string{availability},
			ValidatorMode:    StrictLegacyValidatorMode,
			Opts:             opts,
			DryRun:           opts.DryRun,
		})
	}

	output, rawResponse, aiErr := aiClient.ComposeAIFirstActions(ctx, input)
	if aiErr != nil {
		slog.Warn("actions ai-first: compose failed", "err", aiErr)
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID:            runID,
			SellerID:         sellerAccountID,
			MetricDate:       metricDate,
			MetricDateStr:    metricDateStr,
			Input:            input,
			RawResponse:      rawResponse,
			FinalStatus:      GenerationFinalStatusFailed,
			FailureSubreason: ActionsErrorProviderFailed,
			ComposeCalls:     1,
			Notes:            []string{ActionsErrorProviderFailed + ": " + aiErr.Error()},
			ValidatorMode:    StrictLegacyValidatorMode,
			Opts:             opts,
			DryRun:           opts.DryRun,
		})
	}

	validation := ValidateActionsAIFirstOutput(input, *output)
	items, guardDropped := mapAIFirstDraftsPreservingOrder(metricDateStr, input, validation.AcceptedActions)
	droppedCount := len(validation.DroppedActions) + guardDropped
	focusValid := output.Focus != nil && len(validation.FocusIssues) == 0
	aiFocus := BuildAIFocusFromAIFirst(output.Focus, focusValid, items)
	finalStatus := computeAIFirstFinalStatus(len(items), droppedCount, false)

	return s.finishAIFirstRun(ctx, aiFirstRunState{
		RunID:                 runID,
		SellerID:              sellerAccountID,
		MetricDate:            metricDate,
		MetricDateStr:         metricDateStr,
		Input:                 input,
		Items:                 items,
		TopKeys:               firstTopActionKeys(items, 3),
		AIFocus:               aiFocus,
		FinalStatus:           finalStatus,
		DroppedCount:          droppedCount,
		ReturnedCount:         len(output.Actions),
		ValidBeforeLimitCount: len(validation.AcceptedActions),
		ComposeCalls:          1,
		ValidatorMode:         StrictLegacyValidatorMode,
		RawResponse:           rawResponse,
		Validation:            &validation,
		FinalValidation:       &validation,
		Opts:                  opts,
		DryRun:                opts.DryRun,
	})
}

func (s *GenerationService) generateActionsAIFirstV2FromInput(
	ctx context.Context,
	input ActionsAIFirstInput,
	opts GenerateActionsOptions,
) (*GenerateActionsResult, error) {
	if _, err := input.requireDataPack(); err != nil {
		return nil, &ActionsGenerationError{Code: ActionsErrorInputMissingDataPack, Err: err}
	}
	metricDateStr := input.MetricDate
	metricDate, err := time.Parse("2006-01-02", metricDateStr)
	if err != nil {
		return nil, fmt.Errorf("ai-first v2: invalid metric_date %q: %w", metricDateStr, err)
	}
	sellerAccountID := input.SellerAccountID

	var runID int64
	if !opts.DryRun {
		runID, err = s.beginAIFirstGenerationRun(ctx, sellerAccountID, metricDate, input, opts)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				s.finalizeAIFirstRunOnError(ctx, runID, input.DataPackHash)
			}
		}()
	}

	if len(input.ActionCandidates) == 0 {
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID:         runID,
			SellerID:      sellerAccountID,
			MetricDate:    metricDate,
			MetricDateStr: metricDateStr,
			Input:         input,
			Items:         []ActionItem{},
			TopKeys:       []string{},
			FinalStatus:   GenerationFinalStatusEmpty,
			ComposeCalls:  0,
			ValidatorMode: PilotValidatorMode,
			Notes:         []string{"actions v2: zero candidates, deterministic business-empty"},
			Opts:          opts,
			DryRun:        opts.DryRun,
		})
	}

	aiClient, availability := resolveAIFirstClientAvailability(s.ai, opts)
	if availability != "" {
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID:            runID,
			SellerID:         sellerAccountID,
			MetricDate:       metricDate,
			MetricDateStr:    metricDateStr,
			Input:            input,
			FinalStatus:      GenerationFinalStatusFailed,
			FailureSubreason: availability,
			ComposeCalls:     0,
			Notes:            []string{availability},
			ValidatorMode:    PilotValidatorMode,
			Opts:             opts,
			DryRun:           opts.DryRun,
		})
	}

	if _, _, tokenErr := guardActionsAIFirstInputTokens(input); tokenErr != nil {
		return nil, tokenErr
	}

	if IsTwoPhaseActionsPrompt(actionsAIFirstV2PromptVersion) {
		return s.generateActionsAIFirstV2TwoPhase(ctx, input, opts, aiClient, runID, sellerAccountID, metricDate, metricDateStr)
	}

	output, rawResponse, aiErr := aiClient.ComposeAIFirstActions(ctx, input)
	if aiErr != nil {
		slog.Warn("actions ai-first v2: compose failed", "err", aiErr)
		code := ActionsErrorProviderFailed
		if strings.Contains(aiErr.Error(), ActionsErrorInvalidJSON) {
			code = ActionsErrorInvalidJSON
		}
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID:            runID,
			SellerID:         sellerAccountID,
			MetricDate:       metricDate,
			MetricDateStr:    metricDateStr,
			Input:            input,
			RawResponse:      rawResponse,
			FinalStatus:      GenerationFinalStatusFailed,
			FailureSubreason: code,
			ComposeCalls:     1,
			Notes:            []string{code + ": " + aiErr.Error()},
			ValidatorMode:    PilotValidatorMode,
			Opts:             opts,
			DryRun:           opts.DryRun,
		})
	}

	normOutput, normResult := NormalizeActionsAIFirstV2Output(input, *output)
	rawReturnedActions := append([]AIFirstDraftAction(nil), output.Actions...)
	output = &normOutput

	pilot := ValidateActionsAIFirstOutputPilot(input, *output)
	if len(normResult.Issues) > 0 {
		pilot.Issues = append(pilot.Issues, normResult.Issues...)
	}
	allIssues := pilot.Issues
	warningsByIndex := warningsByActionIndex(allIssues)
	persistResult := mapAIFirstDraftsWithDiagnostics(metricDateStr, input, pilot.Accepted, warningsByIndex)

	persistedCandidateKeys := map[string]bool{}
	for _, item := range persistResult.Items {
		for _, id := range item.SourceCandidateIDs {
			persistedCandidateKeys[strings.TrimSpace(id)] = true
		}
	}
	candidateCoverage := BuildActionsCandidateCoverageDiagnostics(
		input, rawReturnedActions, pilot.Accepted, pilot.Dropped, persistedCandidateKeys,
	)

	pipelineDiag := buildAIFirstPipelineDiagnostics(
		normResult.ReturnedActionCount,
		normResult.DroppedActionCount,
		len(output.Actions),
		pilot,
		persistResult,
		allIssues,
		candidateCoverage,
	)
	pipelineDiag.CompletionConditionsReturnedCount = normResult.CompletionConditionsReturnedCount
	pipelineDiag.CompletionConditionsValidCount = normResult.CompletionConditionsValidCount
	pipelineDiag.CompletionConditionsDroppedCount = normResult.CompletionConditionsDroppedCount
	pipelineDiag.CompletionConditionsFactIDRemovedCount = normResult.CompletionConditionsFactIDRemovedCount
	pipelineDiag.CompletionConditionsEmptyActionsCount = normResult.CompletionConditionsEmptyActionsCount
	if rawResponse != nil {
		applyActionsOutputTokenDiagnostics(&pipelineDiag, rawResponse.OutputTokens)
	}
	if normResult.DroppedActionCount > 0 {
		if pipelineDiag.DropIssueCounts == nil {
			pipelineDiag.DropIssueCounts = map[string]int{}
		}
		pipelineDiag.DropIssueCounts["duplicate_scope_after_ai_order_normalization"] = normResult.DroppedActionCount
	}

	validation := pilotValidationToLegacyResult(pilot)
	items := persistResult.Items
	validationDropped := normResult.DroppedActionCount + pilot.DroppedCount + persistResult.DroppedCount
	totalDropped := validationDropped + pilot.TruncatedCount
	dropIssueCounts := pipelineDiag.DropIssueCounts
	warningIssueCounts := pipelineDiag.WarningIssueCounts

	finalStatus := GenerationFinalStatusCompleted
	failureSubreason := ""
	switch {
	case len(items) == 0 && pilot.ReturnedCount == 0:
		finalStatus = GenerationFinalStatusFailed
		failureSubreason = ActionsErrorNoValidActions
	case len(items) == 0:
		finalStatus = GenerationFinalStatusFailed
		failureSubreason = ActionsErrorAllInvalid
	case validationDropped > 0 || persistResult.DroppedCount > 0:
		finalStatus = GenerationFinalStatusCompletedPartial
	}

	focusValid := output.Focus != nil
	aiFocus := BuildAIFocusFromAIFirst(output.Focus, focusValid, items)

	notes := []string(nil)
	if failureSubreason != "" {
		notes = []string{failureSubreason}
	}

	return s.finishAIFirstRun(ctx, aiFirstRunState{
		RunID:                 runID,
		SellerID:              sellerAccountID,
		MetricDate:            metricDate,
		MetricDateStr:         metricDateStr,
		Input:                 input,
		Items:                 items,
		TopKeys:               firstTopActionKeys(items, 3),
		AIFocus:               aiFocus,
		FinalStatus:           finalStatus,
		DroppedCount:          totalDropped,
		ReturnedCount:         normResult.ReturnedActionCount,
		ValidBeforeLimitCount: pilot.ValidBeforeLimit,
		TruncatedCount:        pilot.TruncatedCount,
		ComposeCalls:          1,
		ValidatorMode:         PilotValidatorMode,
		FailureSubreason:      failureSubreason,
		DropIssueCounts:       dropIssueCounts,
		WarningIssueCounts:    warningIssueCounts,
		PipelineDiagnostics:   &pipelineDiag,
		RawResponse:           rawResponse,
		Validation:            &validation,
		FinalValidation:       &validation,
		Notes:                 notes,
		Opts:                  opts,
		DryRun:                opts.DryRun,
	})
}

func resolveAIFirstClientAvailability(ai ActionAIClient, opts GenerateActionsOptions) (ActionsAIFirstClient, string) {
	if !opts.AIEnabled {
		return nil, ActionsErrorAIDisabled
	}
	if ai == nil {
		return nil, ActionsErrorAIClientUnavailable
	}
	aiClient, ok := ai.(ActionsAIFirstClient)
	if !ok {
		return nil, ActionsErrorAIFirstUnsupported
	}
	return aiClient, ""
}

func mapAIFirstDraftsPreservingOrder(metricDateStr string, input ActionsAIFirstInput, drafts []AIFirstDraftAction) ([]ActionItem, int) {
	result := mapAIFirstDraftsWithDiagnostics(metricDateStr, input, drafts, nil)
	return result.Items, result.DroppedCount
}

func firstTopActionKeys(items []ActionItem, max int) []string {
	if max <= 0 || len(items) == 0 {
		return []string{}
	}
	if len(items) < max {
		max = len(items)
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		out = append(out, items[i].ActionKey)
	}
	return out
}

func computeAIFirstFinalStatus(acceptedCount, droppedCount int, retried bool) string {
	switch {
	case acceptedCount == 0 && droppedCount > 0:
		return GenerationFinalStatusFailed
	case acceptedCount == 0:
		return GenerationFinalStatusEmpty
	case droppedCount > 0:
		return GenerationFinalStatusCompletedPartial
	case retried:
		return GenerationFinalStatusCompletedAfterRetry
	default:
		return GenerationFinalStatusCompleted
	}
}

// aiFirstRunState bundles everything needed to finalize an AI-first run.
type aiFirstRunState struct {
	RunID         int64
	SellerID      int64
	MetricDate    time.Time
	MetricDateStr string
	Input         ActionsAIFirstInput

	Items   []ActionItem
	TopKeys []string
	AIFocus *ActionsAIFocus

	FinalStatus           string
	DroppedCount          int
	ReturnedCount         int
	ValidBeforeLimitCount int
	TruncatedCount        int
	ComposeCalls          int
	ObjectDetailCalls     int
	ValidatorMode         string
	FailureSubreason      string
	DropIssueCounts       map[string]int
	WarningIssueCounts    map[string]int
	PipelineDiagnostics   *AIFirstPipelineDiagnostics
	Retried               bool
	Notes                 []string

	RawResponse     *AIRawResponse
	RetryResponse   *AIRawResponse
	Validation      *AIFirstValidationResult
	FinalValidation *AIFirstValidationResult

	ComposeInputTokens       int
	ComposeOutputTokens      int
	ObjectDetailInputTokens  int
	ObjectDetailOutputTokens int
	InputTokens              int
	OutputTokens             int

	ComposeOutput       *ActionsAIFirstComposeOutput
	ComposeWireInput    json.RawMessage
	BaseActions         []AIFirstBaseAction
	DetailMerges        []ObjectDetailMergeResult
	NormalizationEvents []AIPipelineNormalizationEvent

	LifecycleSummary ActionsLifecycleSummary     `json:"-"`
	ActionLifecycles []ActionLifecycleDiagnostic `json:"-"`

	Opts   GenerateActionsOptions
	DryRun bool
}

func (s *GenerationService) finishAIFirstRun(ctx context.Context, state aiFirstRunState) (*GenerateActionsResult, error) {
	if state.AIFocus == nil {
		state.AIFocus = buildDeterministicAIFirstFocus(state.Items)
	}
	if state.TopKeys == nil {
		state.TopKeys = firstTopActionKeys(state.Items, 3)
	}
	result := &GenerateActionsResult{
		RunID:                 state.RunID,
		MetricDate:            state.MetricDateStr,
		FinalStatus:           state.FinalStatus,
		Actions:               state.Items,
		TopActionKeys:         state.TopKeys,
		AIFocus:               state.AIFocus,
		AcceptedCount:         len(state.Items),
		DroppedCount:          state.DroppedCount,
		DryRun:                state.DryRun,
		ReturnedCount:         state.ReturnedCount,
		ValidBeforeLimitCount: state.ValidBeforeLimitCount,
		TruncatedCount:        state.TruncatedCount,
		ComposeCalls:          state.ComposeCalls,
		ObjectDetailCalls:     state.ObjectDetailCalls,
		ValidatorMode:         state.ValidatorMode,
		FailureSubreason:      state.FailureSubreason,
		DropIssueCounts:       state.DropIssueCounts,
		AIFirstValidation:     state.FinalValidation,
		PipelineDiagnostics:   state.PipelineDiagnostics,
		ComposeOutput:         state.ComposeOutput,
		ComposeWireInput:      state.ComposeWireInput,
		ComposeRawResponse:    state.RawResponse,
		BaseActions:           state.BaseActions,
		DetailMerges:          state.DetailMerges,
		NormalizationEvents:   state.NormalizationEvents,
		LifecycleSummary:      state.LifecycleSummary,
		ActionLifecycles:      state.ActionLifecycles,
	}
	if state.InputTokens > 0 || state.OutputTokens > 0 {
		result.InputTokens = state.InputTokens
		result.OutputTokens = state.OutputTokens
	} else if state.RawResponse != nil {
		result.InputTokens = state.RawResponse.InputTokens
		result.OutputTokens = state.RawResponse.OutputTokens
		result.LatencyMs = state.RawResponse.LatencyMs
		result.RequestID = state.RawResponse.RequestID
	}
	if state.RawResponse != nil && result.LatencyMs == 0 {
		result.LatencyMs = state.RawResponse.LatencyMs
		result.RequestID = state.RawResponse.RequestID
	}
	if state.DryRun {
		return result, nil
	}

	aiFocusJSON := marshalJSON(state.AIFocus)
	topKeysJSON := marshalJSON(state.TopKeys)
	notesJSON := marshalJSON(state.Notes)

	runUpdate := UpdateGenerationRunParams{
		RunID:             state.RunID,
		Status:            GenerationRunStatusCompleted,
		ContextHash:       state.Input.DataPackHash,
		FinalStatus:       state.FinalStatus,
		AcceptedCount:     len(state.Items),
		RepairedCount:     boolToInt(state.Retried),
		DroppedCount:      state.DroppedCount,
		AIFocusJSON:       aiFocusJSON,
		TopActionKeysJSON: topKeysJSON,
		DataQualityNotes:  notesJSON,
	}

	if state.FinalStatus == GenerationFinalStatusFailed || state.FinalStatus == GenerationFinalStatusEmpty || len(state.Items) == 0 {
		if err := s.repo.UpdateActionGenerationRun(ctx, runUpdate); err != nil {
			return nil, fmt.Errorf("update ai-first run: %w", err)
		}
		s.recordAIFirstCategoryRun(ctx, state)
		return result, nil
	}

	upserts := make([]UpsertActionItemParams, 0, len(state.Items))
	keepKeys := make([]string, 0, len(state.Items))
	for _, item := range state.Items {
		params, err := actionItemToUpsertParams(state.SellerID, state.RunID, state.MetricDate, item)
		if err != nil {
			return nil, err
		}
		upserts = append(upserts, params)
		keepKeys = append(keepKeys, item.ActionKey)
	}

	if err := s.repo.PersistGenerationAtomic(ctx, PersistGenerationParams{
		RunUpdate:   runUpdate,
		ActionItems: upserts,
		StaleDelete: &StaleDeleteParams{
			RunID:           state.RunID,
			SellerAccountID: state.SellerID,
			MetricDate:      state.MetricDate,
			KeepKeys:        keepKeys,
		},
		PipelineRunID:     state.Opts.PipelineRunID,
		DataPackHash:      state.Input.DataPackHash,
		GlobalContextHash: state.Opts.GlobalContextHash,
	}); err != nil {
		return nil, fmt.Errorf("persist ai-first generation: %w", err)
	}

	s.recordAIFirstCategoryRun(ctx, state)
	return result, nil
}

func (s *GenerationService) recordAIFirstCategoryRun(ctx context.Context, state aiFirstRunState) {
	if state.RunID <= 0 {
		return
	}
	model := rawResponseModel(state.RawResponse)
	status := CategoryRunStatusCompleted
	switch state.FinalStatus {
	case GenerationFinalStatusFailed:
		status = CategoryRunStatusFailed
	case GenerationFinalStatusEmpty:
		status = CategoryRunStatusSkippedEmpty
	case GenerationFinalStatusCompletedPartial:
		status = CategoryRunStatusCompletedPartial
	case GenerationFinalStatusCompletedAfterRetry:
		status = CategoryRunStatusCompletedRepaired
	}
	if _, err := s.repo.CreateActionGenerationCategoryRun(ctx, CreateCategoryRunParams{
		RunID:               state.RunID,
		SellerAccountID:     state.SellerID,
		Domain:              DomainAIFirst,
		Status:              status,
		Model:               model,
		InputContextJSON:    marshalJSON(compactAIFirstInputContext(state.Input)),
		RawResponseJSON:     rawResponseJSON(state.RawResponse),
		ValidationJSON:      marshalValidationJSON(state.Validation),
		RepairResponseJSON:  rawResponseJSON(state.RetryResponse),
		FinalValidationJSON: marshalValidationJSON(state.FinalValidation),
		AcceptedCount:       len(state.Items),
		RepairedCount:       boolToInt(state.Retried),
		DroppedCount:        state.DroppedCount,
	}); err != nil {
		slog.Warn("actions ai-first: create category run failed", "err", err)
	}
}

func marshalValidationJSON(v *AIFirstValidationResult) []byte {
	if v == nil {
		return nil
	}
	return marshalJSON(v)
}

// compactAIFirstInputContext is a compact, storable summary of the AI-first
// input (the full Data Pack is too large for category run storage).
func compactAIFirstInputContext(input ActionsAIFirstInput) map[string]any {
	out := map[string]any{
		"schema_version":           input.SchemaVersion,
		"seller_account_id":        input.SellerAccountID,
		"metric_date":              input.MetricDate,
		"data_pack_schema_version": input.DataPackSchemaVersion,
		"data_pack_hash":           input.DataPackHash,
		"global_context_hash":      input.GlobalContextHash,
		"allowed_action_types":     input.AllowedActionTypes,
		"allowed_categories":       input.AllowedCategories,
		"allowed_priorities":       input.AllowedPriorities,
		"forbidden_claims":         input.ForbiddenClaims,
		"max_actions":              input.MaxActions,
		"prompt_version":           actionsAIFirstPromptVersion,
	}
	if input.IsV2CandidateInput() {
		out["prompt_version"] = actionsAIFirstV2PromptVersion
		out["candidate_count"] = len(input.ActionCandidates)
		out["fact_catalog_count"] = len(input.FactCatalog)
		out["entity_catalog_count"] = len(input.EntityCatalog)
		out["failed_chunk_count"] = len(input.FailedChunkIDs)
		out["limitations_count"] = len(input.DataLimitations)
		return out
	}
	pack, err := input.requireDataPack()
	if err == nil && pack != nil {
		out["fact_count"] = len(pack.FactRegistry)
		out["sku_entity_count"] = len(pack.EntityRegistry.SKUs)
		out["campaign_entity_count"] = len(pack.EntityRegistry.Campaigns)
	}
	return out
}

func (s *GenerationService) beginAIFirstGenerationRun(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	input ActionsAIFirstInput,
	opts GenerateActionsOptions,
) (int64, error) {
	if opts.ExistingActionsRunID != nil && *opts.ExistingActionsRunID > 0 && !opts.ForceActionsRegenerate {
		runID := *opts.ExistingActionsRunID
		if err := s.repo.UpdateActionGenerationRun(ctx, UpdateGenerationRunParams{
			RunID:  runID,
			Status: GenerationRunStatusRunning,
		}); err != nil {
			return 0, fmt.Errorf("reset actions generation run: %w", err)
		}
		if opts.PipelineRunID != nil && *opts.PipelineRunID > 0 {
			if linkErr := s.repo.LinkActionGenerationRunToPipeline(ctx, *opts.PipelineRunID, runID, input.DataPackHash, opts.GlobalContextHash); linkErr != nil {
				return 0, fmt.Errorf("link actions run to pipeline: %w", linkErr)
			}
		}
		return runID, nil
	}
	runID, err := s.repo.CreateActionGenerationRun(ctx, sellerAccountID, metricDate, GenerationRunStatusRunning)
	if err != nil {
		return 0, fmt.Errorf("create generation run: %w", err)
	}
	if opts.PipelineRunID != nil && *opts.PipelineRunID > 0 {
		if linkErr := s.repo.LinkActionGenerationRunToPipeline(ctx, *opts.PipelineRunID, runID, input.DataPackHash, opts.GlobalContextHash); linkErr != nil {
			return 0, fmt.Errorf("link actions run to pipeline: %w", linkErr)
		}
	}
	return runID, nil
}

func (s *GenerationService) finalizeAIFirstRunOnError(ctx context.Context, runID int64, contextHash string) {
	if runID <= 0 {
		return
	}
	_ = s.repo.UpdateActionGenerationRun(ctx, UpdateGenerationRunParams{
		RunID:       runID,
		Status:      GenerationRunStatusCompleted,
		ContextHash: contextHash,
		FinalStatus: GenerationFinalStatusFailed,
	})
}
