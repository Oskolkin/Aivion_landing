package dashboardhome

import (
	"strings"
)

type narrativeFreshnessDecision struct {
	Fresh               bool
	MinorDrift          bool
	Reason              string
	StoredFactualHash   string
	CurrentFactualHash  string
	StoredRenderedHash  string
	CurrentRenderedHash string
}

type narrativeAcceptanceOutcome struct {
	Applied               bool
	Payload               *dashboardNarrativePayload
	TopActions            []TopActionItem
	ValidationStatus      string
	Freshness             string
	Repair                narrativeRepairResult
	FreshnessDecision     narrativeFreshnessDecision
	FallbackActionsFilled int
}

func evaluateNarrativeFreshness(narrative *storedDashboardNarrative, build dashboardNarrativeBuildResult) narrativeFreshnessDecision {
	decision := narrativeFreshnessDecision{
		CurrentFactualHash:  build.FactualContextHash,
		CurrentRenderedHash: build.ContextHash,
	}
	if narrative == nil || narrative.Payload == nil {
		decision.Reason = "missing_narrative_row"
		return decision
	}
	decision.StoredFactualHash = narrative.ContextHash
	decision.StoredRenderedHash = narrative.RenderedContextHash

	if narrative.Status != "completed" {
		decision.Reason = "row_status_not_completed"
		return decision
	}
	if narrative.PromptVersion != DashboardNarrativePromptVersion {
		decision.Reason = "prompt_version_mismatch"
		return decision
	}
	if narrative.ContextHash == "" {
		decision.Reason = "empty_stored_factual_hash"
		return decision
	}
	switch {
	case narrative.ContextHash == build.FactualContextHash:
		decision.Fresh = true
	case narrative.ContextHash == build.ContextHash && narrative.RiskDriversHash == build.RiskDriversHash:
		// Legacy rows stored rendered context_hash + risk_drivers_hash.
		decision.Fresh = true
		decision.Reason = "legacy_context_hash_matched"
	default:
		decision.Reason = "factual_context_hash_mismatch"
		return decision
	}

	if narrative.RenderedContextHash != "" && narrative.RenderedContextHash != build.ContextHash {
		decision.MinorDrift = true
		decision.Reason = "rendered_context_hash_mismatch_ignored"
	}
	return decision
}

func acceptStoredDashboardNarrative(
	narrative *storedDashboardNarrative,
	build dashboardNarrativeBuildResult,
	drivers []RiskDriver,
	actionPool []TopActionItem,
	aiSummaryActions []TopActionItem,
	limit int,
) narrativeAcceptanceOutcome {
	if dashboardAIOnlyEnabled() {
		return acceptStoredDashboardNarrativeAIOnly(narrative, drivers, limit)
	}
	outcome := narrativeAcceptanceOutcome{
		ValidationStatus: "fallback",
		Freshness:        "fallback",
	}
	outcome.FreshnessDecision = evaluateNarrativeFreshness(narrative, build)
	if !outcome.FreshnessDecision.Fresh {
		if narrative != nil {
			outcome.ValidationStatus = "fallback_stale"
		}
		recordNarrativeAcceptance(outcome)
		return outcome
	}

	validationCtx := buildDashboardNarrativeValidationContext(drivers, build)
	repair := repairDashboardNarrativePayload(cloneDashboardNarrativePayload(narrative.Payload), validationCtx)
	outcome.Repair = repair
	if len(repair.HardErrors) > 0 {
		outcome.ValidationStatus = "fallback_validation_failed"
		recordNarrativeAcceptance(outcome)
		return outcome
	}

	payload := repair.Payload
	enrichedDrivers := applyDashboardNarrativeHealthReasons(drivers, payload)
	topActions := buildTopActionsFromDashboardNarrative(payload, build, enrichedDrivers, actionPool, limit)
	if len(topActions) < limit {
		filled := buildFallbackTopActionsForAIDiagnostics(aiSummaryActions, enrichedDrivers, actionPool, limit-len(topActions))
		existing := map[string]struct{}{}
		for _, action := range topActions {
			existing[fallbackTopActionDedupKey(action)] = struct{}{}
		}
		for _, action := range filled {
			if len(topActions) >= limit {
				break
			}
			key := fallbackTopActionDedupKey(action)
			if _, exists := existing[key]; exists {
				continue
			}
			existing[key] = struct{}{}
			topActions = append(topActions, action)
			outcome.FallbackActionsFilled++
		}
	}

	if len(topActions) == 0 && !hasUsableNarrativeSummary(payload) {
		outcome.ValidationStatus = "fallback_validation_failed"
		recordNarrativeAcceptance(outcome)
		return outcome
	}

	outcome.Applied = true
	outcome.Payload = payload
	outcome.TopActions = topActions
	outcome.ValidationStatus = narrativeValidationStatusFromRepair(repair, outcome.FallbackActionsFilled)
	outcome.Freshness = narrativeFreshnessFromDecision(outcome.FreshnessDecision)
	recordNarrativeAcceptance(outcome)
	return outcome
}

func acceptStoredDashboardNarrativeAIOnly(
	narrative *storedDashboardNarrative,
	drivers []RiskDriver,
	limit int,
) narrativeAcceptanceOutcome {
	outcome := narrativeAcceptanceOutcome{
		ValidationStatus: "failed",
		Freshness:        "current",
	}
	if narrative == nil || narrative.Payload == nil {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	// Stage 5 Unified publications stamp prompt_version=unified_publication_adapter_v1.
	// They must be applied as-is; v9 AI-only acceptance would reject them and surface
	// the legacy "Run dashboard_narrative_v9_ai_only_chunks" placeholder.
	if IsUnifiedPublicationPromptVersion(narrative.PromptVersion) {
		return acceptStoredUnifiedPublicationNarrative(narrative, drivers, limit)
	}
	if narrative.Status != "completed" || narrative.PromptVersion != DashboardNarrativePromptVersion {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	repair := repairDashboardNarrativePayloadAIOnly(cloneDashboardNarrativePayload(narrative.Payload))
	outcome.Repair = repair
	if len(repair.HardErrors) > 0 {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	mode := CurrentDashboardValidationMode()
	if isPilotStructuralMode(mode) {
		blockErrs := validatePilotStructuralRequiredBlocks(repair.Payload)
		blockValidation := classifyBlockingCodes(blockErrs, mode, "required_blocks")
		if !blockValidation.Valid {
			outcome.ValidationStatus = "failed"
			recordNarrativeAcceptance(outcome)
			return outcome
		}
	} else if errs := validateAIOnlyRequiredBlocks(repair.Payload); len(errs) > 0 {
		outcome.ValidationStatus = "failed"
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	contentValidation := evaluateAIOnlyContentValidation(repair.Payload, mode)
	if !contentValidation.Valid {
		outcome.ValidationStatus = "failed"
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	if len(contentValidation.Warnings) > 0 {
		outcome.ValidationStatus = "completed_with_warnings"
	}
	topActions := buildTopActionsFromDashboardNarrativeAIOnly(repair.Payload, drivers, limit)
	outcome.Applied = true
	outcome.Payload = repair.Payload
	outcome.TopActions = topActions
	if outcome.ValidationStatus == "" {
		outcome.ValidationStatus = "completed"
	}
	outcome.Freshness = "current"
	recordNarrativeAcceptance(outcome)
	return outcome
}

// acceptStoredUnifiedPublicationNarrative applies a Stage 5 publication
// narrative without re-running the legacy v9 AI-only content gate (that full
// structural/content validation already ran once, at publish time, in
// outputvalidator.Validate + bundlevalidator — re-running it on every GET
// would be wasteful and is unnecessary).
//
// It does NOT, however, blindly trust that publish-time validation passed:
// a stored row could predate a validator change, be corrupted, or have been
// written by a since-changed adapter version. So this function still
// requires its own minimal evidence that the row is a genuinely valid
// publication bundle before applying it to the seller-facing dashboard:
//   - narrative.Status == "completed" (never apply pending/failed rows)
//   - a supported/known Unified publication prompt version
//   - for a "material" payload (has a headline, executive_summary, or any
//     top_actions — i.e. shaped like material_actions_found/no_material_actions):
//     headline AND executive_summary must both be non-empty
//   - for a payload without any of the above ("insufficient_data"-shaped):
//     limited/absent texts are allowed, but the seller_pulse narrative must
//     still carry an explicit, resolved status — an empty/zero-value row
//     must never be mistaken for a valid "insufficient data" analysis
func acceptStoredUnifiedPublicationNarrative(
	narrative *storedDashboardNarrative,
	drivers []RiskDriver,
	limit int,
) narrativeAcceptanceOutcome {
	outcome := narrativeAcceptanceOutcome{
		ValidationStatus: "failed",
		Freshness:        "current",
	}
	if narrative == nil || narrative.Payload == nil {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	if narrative.Status != "completed" {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	if !IsUnifiedPublicationPromptVersion(narrative.PromptVersion) {
		recordNarrativeAcceptance(outcome)
		return outcome
	}
	payload := cloneDashboardNarrativePayload(narrative.Payload)
	if payload == nil {
		recordNarrativeAcceptance(outcome)
		return outcome
	}

	headline := strings.TrimSpace(payload.Headline)
	executiveSummary := strings.TrimSpace(payload.ExecutiveSummary)
	looksMaterial := headline != "" || executiveSummary != "" || len(payload.TopActions) > 0

	if looksMaterial {
		if headline == "" || executiveSummary == "" {
			recordNarrativeAcceptance(outcome)
			return outcome
		}
	} else {
		if !hasResolvedSellerPulseNarrativeStatus(payload.SellerPulseNarrative) {
			recordNarrativeAcceptance(outcome)
			return outcome
		}
	}

	topActions := buildTopActionsFromDashboardNarrativeAIOnly(payload, drivers, limit)
	outcome.Applied = true
	outcome.Payload = payload
	outcome.TopActions = topActions
	outcome.ValidationStatus = "completed"
	outcome.Freshness = "current"
	recordNarrativeAcceptance(outcome)
	return outcome
}

// hasResolvedSellerPulseNarrativeStatus reports whether n carries an
// explicit, resolved status (not empty/pending/stale/fallback). Used to
// gate insufficient_data-shaped unified publication payloads, which are
// allowed minimal/absent headline/executive_summary/top_actions but must
// still show evidence of a real (even if limited) analysis run.
func hasResolvedSellerPulseNarrativeStatus(n *SellerPulseNarrative) bool {
	if n == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(n.Status)) {
	case "", "pending", "stale", "fallback":
		return false
	default:
		return true
	}
}

func narrativeValidationStatusFromRepair(repair narrativeRepairResult, fallbackFilled int) string {
	if dashboardAIOnlyEnabled() {
		if repair.Repaired {
			return "completed_repaired"
		}
		return "completed"
	}
	switch {
	case fallbackFilled > 0 || repair.Partial:
		return "completed_partial"
	case repair.Repaired:
		return "completed_repaired"
	default:
		return "completed"
	}
}

func narrativeFreshnessFromDecision(decision narrativeFreshnessDecision) string {
	if decision.MinorDrift && !dashboardAIOnlyEnabled() {
		return "current_with_minor_drift"
	}
	return "current"
}

func cloneDashboardNarrativePayload(payload *dashboardNarrativePayload) *dashboardNarrativePayload {
	if payload == nil {
		return nil
	}
	copyPayload := *payload
	if payload.HealthReasons != nil {
		copyPayload.HealthReasons = append([]dashboardNarrativeHealthReason(nil), payload.HealthReasons...)
	}
	if payload.TopActions != nil {
		copyPayload.TopActions = append([]dashboardNarrativeTopAction(nil), payload.TopActions...)
		for i := range copyPayload.TopActions {
			copyPayload.TopActions[i].SupportingMetricRefs = append([]string(nil), payload.TopActions[i].SupportingMetricRefs...)
		}
	}
	if payload.DataQualityNotes != nil {
		copyPayload.DataQualityNotes = append([]string(nil), payload.DataQualityNotes...)
	}
	return &copyPayload
}

func acceptGeneratedDashboardNarrative(
	payload *dashboardNarrativePayload,
	build dashboardNarrativeBuildResult,
) (accepted bool, repaired *dashboardNarrativePayload, repair narrativeRepairResult, hardErrors []string) {
	accepted, repaired, repair, validation := acceptGeneratedDashboardNarrativeWithValidation(payload, build)
	return accepted, repaired, repair, validation.BlockingCodes()
}

func acceptGeneratedDashboardNarrativeWithValidation(
	payload *dashboardNarrativePayload,
	build dashboardNarrativeBuildResult,
) (accepted bool, repaired *dashboardNarrativePayload, repair narrativeRepairResult, validation DashboardValidationResult) {
	mode := CurrentDashboardValidationMode()
	validation = DashboardValidationResult{Mode: mode, Valid: true}
	if dashboardAIOnlyEnabled() {
		repair = repairDashboardNarrativePayloadAIOnly(cloneDashboardNarrativePayload(payload))
	} else {
		validationCtx := buildDashboardNarrativeValidationContextFromBuild(build)
		repair = repairDashboardNarrativePayload(cloneDashboardNarrativePayload(payload), validationCtx)
	}
	if len(repair.HardErrors) > 0 {
		repairValidation := classifyBlockingCodes(repair.HardErrors, mode, "repair")
		validation = mergeDashboardValidationResults(validation, repairValidation)
		if !validation.Valid {
			return false, nil, repair, validation
		}
	}
	if repair.Payload == nil {
		rootValidation := classifyBlockingCodes([]string{"nil_payload"}, mode, "acceptance")
		validation = mergeDashboardValidationResults(validation, rootValidation)
		return false, nil, repair, validation
	}
	if !isPilotStructuralMode(mode) {
		if !hasUsableNarrativeSummary(repair.Payload) && len(repair.Payload.TopActions) == 0 {
			usableValidation := classifyBlockingCodes([]string{"no_usable_narrative_content"}, mode, "acceptance")
			validation = mergeDashboardValidationResults(validation, usableValidation)
			if !validation.Valid {
				return false, nil, repair, validation
			}
		}
	}
	if dashboardAIOnlyEnabled() && !isPilotStructuralMode(mode) {
		blockErrs := validateAIOnlyRequiredBlocks(repair.Payload)
		blockValidation := classifyBlockingCodes(blockErrs, mode, "required_blocks")
		validation = mergeDashboardValidationResults(validation, blockValidation)
		if !validation.Valid {
			return false, nil, repair, validation
		}
	} else if isPilotStructuralMode(mode) {
		blockErrs := validatePilotStructuralRequiredBlocks(repair.Payload)
		blockValidation := classifyBlockingCodes(blockErrs, mode, "required_blocks")
		validation = mergeDashboardValidationResults(validation, blockValidation)
		if !validation.Valid {
			return false, nil, repair, validation
		}
	}
	if dashboardAIOnlyEnabled() {
		contentValidation := evaluateAIOnlyContentValidation(repair.Payload, mode)
		validation = mergeDashboardValidationResults(validation, contentValidation)
	}
	return true, repair.Payload, repair, validation
}

// validatePilotStructuralRequiredBlocks checks only the minimal technical contract
// required to unmarshal, map and render the dashboard in pilot_structural mode.
func validatePilotStructuralRequiredBlocks(payload *dashboardNarrativePayload) []string {
	if payload == nil {
		return []string{"nil_payload"}
	}
	if payload.TopActions == nil {
		payload.TopActions = []dashboardNarrativeTopAction{}
	}
	if payload.CategorySummaries == nil {
		payload.CategorySummaries = []CategorySummaryCard{}
	}
	if payload.DataQualityNotes == nil {
		payload.DataQualityNotes = []string{}
	}
	errs := make([]string, 0, 2)
	if strings.TrimSpace(payload.Headline) == "" {
		if payload.AccountHealth != nil && strings.TrimSpace(payload.AccountHealth.Headline) != "" {
			payload.Headline = payload.AccountHealth.Headline
		} else {
			errs = append(errs, "missing_account_summary_headline")
		}
	}
	if dashboardAIOnlyEnabled() && payload.AccountHealth == nil {
		errs = append(errs, "missing_account_health")
	}
	return errs
}

func validateAIOnlyRequiredBlocks(payload *dashboardNarrativePayload) []string {
	errs := make([]string, 0, 8)
	if payload == nil {
		return []string{"nil_payload"}
	}
	if strings.TrimSpace(payload.Headline) == "" {
		if payload.AccountHealth != nil && strings.TrimSpace(payload.AccountHealth.Headline) != "" {
			payload.Headline = payload.AccountHealth.Headline
		} else {
			errs = append(errs, "missing_account_summary_headline")
		}
	}
	if strings.TrimSpace(payload.ExecutiveSummary) == "" {
		if payload.AccountHealth != nil && strings.TrimSpace(payload.AccountHealth.ExecutiveSummary) != "" {
			payload.ExecutiveSummary = payload.AccountHealth.ExecutiveSummary
		} else {
			errs = append(errs, "missing_account_summary_executive_summary")
		}
	}
	if payload.SellerPulseNarrative == nil {
		errs = append(errs, "missing_seller_pulse_narrative")
	} else {
		status := strings.ToLower(strings.TrimSpace(payload.SellerPulseNarrative.Status))
		if status == "" || status == "pending" || status == "stale" || status == "fallback" {
			errs = append(errs, "invalid_seller_pulse_status")
		}
		if strings.TrimSpace(payload.SellerPulseNarrative.Headline) == "" || strings.TrimSpace(payload.SellerPulseNarrative.Summary) == "" {
			errs = append(errs, "missing_seller_pulse_text")
		}
	}
	if payload.WhatChanged == nil || len(payload.WhatChanged.Groups) == 0 {
		errs = append(errs, "missing_what_changed")
	}
	if len(payload.TopActions) < 3 {
		errs = append(errs, "top_actions_lt_3")
	}
	if payload.StockCoverageNarrative == nil || strings.EqualFold(strings.TrimSpace(payload.StockCoverageNarrative.Status), "pending") || strings.EqualFold(strings.TrimSpace(payload.StockCoverageNarrative.Status), "stale") || strings.EqualFold(strings.TrimSpace(payload.StockCoverageNarrative.Status), "fallback") {
		errs = append(errs, "invalid_stock_coverage_narrative")
	}
	if payload.AdEfficiencyNarrative == nil || strings.EqualFold(strings.TrimSpace(payload.AdEfficiencyNarrative.Status), "pending") || strings.EqualFold(strings.TrimSpace(payload.AdEfficiencyNarrative.Status), "stale") || strings.EqualFold(strings.TrimSpace(payload.AdEfficiencyNarrative.Status), "fallback") {
		errs = append(errs, "invalid_ad_efficiency_narrative")
	}
	if payload.FunnelAIAnalysis == nil || strings.EqualFold(strings.TrimSpace(string(payload.FunnelAIAnalysis.Status)), "") || strings.EqualFold(strings.TrimSpace(string(payload.FunnelAIAnalysis.Status)), "pending") || strings.EqualFold(strings.TrimSpace(string(payload.FunnelAIAnalysis.Status)), "stale") || strings.EqualFold(strings.TrimSpace(string(payload.FunnelAIAnalysis.Status)), "fallback") {
		errs = append(errs, "invalid_funnel_ai_analysis")
	}
	if len(payload.CategorySummaries) == 0 {
		errs = append(errs, "missing_category_summaries")
	}
	if healthErrs := validateAIOnlyAccountHealth(payload); len(healthErrs) > 0 {
		errs = append(errs, healthErrs...)
	}
	if categoryErrs := validateAIOnlyCategoryScoresRequired(payload); len(categoryErrs) > 0 {
		errs = append(errs, categoryErrs...)
	}
	if payload.RiskOverview == nil || strings.EqualFold(strings.TrimSpace(string(payload.RiskOverview.Status)), "") || strings.EqualFold(strings.TrimSpace(string(payload.RiskOverview.Status)), "pending") || strings.EqualFold(strings.TrimSpace(string(payload.RiskOverview.Status)), "stale") || strings.EqualFold(strings.TrimSpace(string(payload.RiskOverview.Status)), "fallback") {
		errs = append(errs, "invalid_risk_overview")
	}
	if len(payload.DataQualityNotes) == 0 {
		errs = append(errs, "missing_data_quality_notes")
	}
	return errs
}

func buildNarrativeDebug(
	narrative *storedDashboardNarrative,
	build dashboardNarrativeBuildResult,
	outcome narrativeAcceptanceOutcome,
) *NarrativeDebug {
	fallbackReason := ""
	if !outcome.Applied {
		fallbackReason = firstNonEmpty(outcome.FreshnessDecision.Reason, outcome.ValidationStatus, "full_fallback")
	}
	debug := &NarrativeDebug{
		FreshnessDecision:          firstNonEmpty(outcome.Freshness, "fallback"),
		FreshnessReason:            firstNonEmpty(outcome.FreshnessDecision.Reason, fallbackReason),
		FallbackReason:             fallbackReason,
		HardValidationErrors:       append([]string(nil), outcome.Repair.HardErrors...),
		RepairableValidationErrors: append([]string(nil), outcome.Repair.RepairableErrors...),
		RepairActions:              append([]string(nil), outcome.Repair.RepairActions...),
		AIActionsReceived:          outcome.Repair.AIActionsReceived,
		AIActionsAccepted:          outcome.Repair.AIActionsAccepted,
		DroppedActions:             outcome.Repair.DroppedActions,
		PartialBlocks:              append([]string(nil), outcome.Repair.PartialBlocks...),
		FallbackBlocks:             append([]string(nil), outcome.Repair.FallbackBlocks...),
		FallbackActionsFilled:      outcome.FallbackActionsFilled,
		CurrentFactualContextHash:  build.FactualContextHash,
		CurrentRenderedContextHash: build.ContextHash,
	}
	if narrative != nil {
		debug.SelectedRunID = narrative.ID
		debug.RowStatus = narrative.Status
		debug.StoredFactualContextHash = narrative.ContextHash
		debug.StoredRenderedContextHash = narrative.RenderedContextHash
		if narrative.Status != "completed" && outcome.FreshnessDecision.Reason == "" {
			debug.FreshnessReason = "row_status_not_completed"
		}
	}
	if outcome.FreshnessDecision.MinorDrift {
		debug.FreshnessReason = strings.TrimSpace(debug.FreshnessReason + ";rendered_context_hash_mismatch_ignored")
	}
	debug.ContextSource = build.ContextSource
	debug.PromptVersion = DashboardNarrativePromptVersion
	debug.FactCount = len(build.DataPackFactIDs)
	debug.DataPackAuditValid = build.DataPackAuditValid
	return debug
}
