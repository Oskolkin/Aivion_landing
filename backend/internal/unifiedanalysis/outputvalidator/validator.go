package outputvalidator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/healthstatus"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

var (
	// Local action IDs: frozen fixtures use a1/a2; production prompt prefers action_001.
	actionIDPattern = regexp.MustCompile(`^(action_\d{3}|[a-z][a-z0-9_]{0,31})$`)
	// placeholderRe flags English filler markers plus common Russian
	// non-committal placeholder phrases the prompt explicitly forbids
	// (see analyst_v1.txt): "нет данных", "данные отсутствуют", "нужно/
	// требуется провести/дополнительный анализ", "информация отсутствует".
	placeholderRe = regexp.MustCompile(`(?i)(\bTODO\b|\bTBD\b|\bN/?A\b|lorem ipsum|\bplaceholder\b|` +
		`нет данных|данные отсутствуют|информация отсутствует|` +
		`(нужно|требуется)\s+(провести\s+)?дополнительн\S*\s+анализ)`)
)

// ExpectedRunMeta is the run/pack identity the output must match.
type ExpectedRunMeta struct {
	PackID     string
	PackHash   string
	MetricDate string // YYYY-MM-DD
}

// Validate performs technical contract validation. Action content repairs
// (entity-count sync, generic expected_effect expansion, short plan fill)
// run first so Validate always inspects the same post-repair state as the
// publication pipeline (which also calls SyncActionEntityCounts beforehand).
func Validate(out *contract.UnifiedAnalysisOutputV1, meta ExpectedRunMeta, idx *PackReferenceIndex) Report {
	var report Report
	if out == nil {
		report.addError(CodeRootMissing, "$", "output is nil")
		return finalize(report, out)
	}
	_ = SyncActionEntityCounts(out)

	if out.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionCurrent {
		report.addError(CodeWrongSchemaVersion, "$.schema_version", fmt.Sprintf("got %q want %q", out.SchemaVersion, contract.UnifiedAnalysisOutputSchemaVersionCurrent))
	}
	if meta.PackID != "" && out.AnalysisMeta.PackID != meta.PackID {
		report.addError(CodeOutputPackIDMismatch, "$.analysis_meta.pack_id", "pack_id mismatch")
	}
	if meta.PackHash != "" && out.AnalysisMeta.PackHash != "" && out.AnalysisMeta.PackHash != meta.PackHash {
		report.addError(CodeWrongPackHash, "$.analysis_meta.pack_hash", "pack_hash mismatch")
	}
	if meta.MetricDate != "" && out.AnalysisMeta.MetricDate != meta.MetricDate {
		report.addError(CodeOutputMetricDateMismatch, "$.analysis_meta.metric_date", "metric_date mismatch")
	}
	if strings.TrimSpace(out.AnalysisMeta.PackID) == "" {
		report.addError(CodeEmptyRequiredText, "$.analysis_meta.pack_id", "pack_id is empty")
	}
	if strings.TrimSpace(out.AnalysisMeta.MetricDate) == "" {
		report.addError(CodeEmptyRequiredText, "$.analysis_meta.metric_date", "metric_date is empty")
	}

	if !contract.ValidAnalysisOutcome(out.AnalysisOutcome) {
		report.addError(CodeOutputInvalidOutcome, "$.analysis_outcome", "unknown analysis_outcome")
	}

	if out.Actions == nil {
		report.addError(CodeRootMissing, "$.actions", "actions must be an array")
	}
	if out.Limitations == nil {
		report.addError(CodeRootMissing, "$.limitations", "limitations must be an array")
	}
	if len(out.Actions) > contract.MaxActionsV1 {
		report.addError(CodeOutputActionLimitExceeded, "$.actions", fmt.Sprintf("actions=%d max=%d", len(out.Actions), contract.MaxActionsV1))
	}
	if len(out.Dashboard.TopActionIDs) > contract.MaxTopActionsV1 {
		report.addError(CodeTooManyTopActions, "$.dashboard.top_action_ids", fmt.Sprintf("top_action_ids=%d max=%d", len(out.Dashboard.TopActionIDs), contract.MaxTopActionsV1))
	}

	actionIDs := map[string]int{}
	actionByID := map[string]contract.UnifiedActionV1{}
	skuOwner := map[string]string{}
	fingerprints := map[string]string{}
	for i, action := range out.Actions {
		path := fmt.Sprintf("$.actions[%d]", i)
		validateAction(&report, path, action, idx, actionIDs, skuOwner)
		if id := strings.TrimSpace(action.AnalysisActionID); id != "" {
			actionByID[id] = action
		}
		fp := actionFingerprint(action)
		if prev, ok := fingerprints[fp]; ok {
			report.addError(CodeActionDuplicate, path, fmt.Sprintf("duplicate action fingerprint matches %s", prev))
		} else if fp != "" {
			fingerprints[fp] = path
		}
	}

	validateOutcomeConsistency(&report, out)
	validateActionIDRefs(&report, "$.dashboard.top_action_ids", out.Dashboard.TopActionIDs, actionIDs)
	validateActionIDRefs(&report, "$.dashboard.recommended_action_ids", out.Dashboard.RecommendedActionIDs, actionIDs)
	validateTopActionRequiredRefs(&report, out, actionByID)
	validateDomainCoverage(&report, out, idx)
	validateStockCohortExclusivity(&report, out, idx)
	validateNoInternalFieldNames(&report, out)
	validateRiskDriverRegistryIDs(&report, out)
	validateLimitations(&report, out.Limitations, idx)
	validateDashboardTexts(&report, &out.Dashboard, out.AnalysisOutcome, idx)

	return finalize(report, out)
}

// isMaterialOutcome reports whether outcome represents a completed analysis
// with a real (positive or negative) account state — as opposed to
// insufficient_data/analysis_failed, where the AI narrative layer is allowed
// to stay minimal/absent. Both material_actions_found and no_material_actions
// require a full AI analytical layer: headline, executive_summary, and a
// resolved seller_pulse narrative.
func isMaterialOutcome(outcome string) bool {
	switch outcome {
	case contract.AnalysisOutcomeMaterialActionsFound, contract.AnalysisOutcomeNoMaterialActions:
		return true
	default:
		return false
	}
}

func validateOutcomeConsistency(report *Report, out *contract.UnifiedAnalysisOutputV1) {
	switch out.AnalysisOutcome {
	case contract.AnalysisOutcomeMaterialActionsFound:
		n := len(out.Actions)
		if n < contract.MinActionsMaterialV1 {
			report.addError(CodeActionsCountBelowMinimum, "$.actions",
				fmt.Sprintf("material_actions_found requires at least %d actions, got %d", contract.MinActionsMaterialV1, n))
		}
		if n > contract.MaxActionsV1 {
			report.addError(CodeOutputActionLimitExceeded, "$.actions",
				fmt.Sprintf("material_actions_found allows at most %d actions, got %d", contract.MaxActionsV1, n))
		}
		requiredCount := 0
		for i, a := range out.Actions {
			tier := strings.TrimSpace(a.ExecutionTier)
			if tier == contract.ExecutionTierRequired {
				requiredCount++
				validateRequiredActionCompleteness(report, fmt.Sprintf("$.actions[%d]", i), a)
			}
		}
		if requiredCount < contract.MinRequiredActionsMaterialV1 {
			report.addError(CodeRequiredActionsCountBelowMinimum, "$.actions",
				fmt.Sprintf("material_actions_found requires at least %d execution_tier=required actions, got %d",
					contract.MinRequiredActionsMaterialV1, requiredCount))
		}
	case contract.AnalysisOutcomeNoMaterialActions:
		if len(out.Actions) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.actions", "no_material_actions requires empty actions")
		}
		if len(out.Dashboard.TopActionIDs) != 0 || len(out.Dashboard.RecommendedActionIDs) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.dashboard", "no_material_actions requires empty action id refs")
		}
	case contract.AnalysisOutcomeInsufficientData:
		if len(out.Actions) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.actions", "insufficient_data requires empty actions")
		}
		if len(out.Dashboard.TopActionIDs) != 0 || len(out.Dashboard.RecommendedActionIDs) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.dashboard", "insufficient_data requires empty action id refs")
		}
		if len(out.Limitations) == 0 {
			report.addError(CodeInsufficientLimitations, "$.limitations", "insufficient_data requires at least one limitation")
		}
	case contract.AnalysisOutcomeAnalysisFailed:
		if len(out.Actions) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.actions", "analysis_failed requires empty actions")
		}
		if len(out.Dashboard.TopActionIDs) != 0 || len(out.Dashboard.RecommendedActionIDs) != 0 {
			report.addError(CodeOutcomeActionsMismatch, "$.dashboard", "analysis_failed requires empty action id refs")
		}
		if len(out.Limitations) == 0 {
			report.addError(CodeInsufficientLimitations, "$.limitations", "analysis_failed requires at least one limitation")
		}
	}
}

func validateRequiredActionCompleteness(report *Report, path string, action contract.UnifiedActionV1) {
	incomplete := strings.TrimSpace(action.OperationalGoal) == "" ||
		strings.TrimSpace(action.CompletionCriteria) == "" ||
		strings.TrimSpace(action.EvidenceSummary) == "" ||
		len(action.Steps) == 0
	if incomplete {
		report.addError(CodeRequiredActionIncomplete, path,
			"required execution_tier action must have evidence_summary, steps, completion_criteria, and operational_goal")
	}
	if strings.TrimSpace(action.EvidenceSummary) == "" {
		report.addError(CodeActionNotGrounded, path+".evidence_summary", "required action must be grounded with evidence_summary")
	}
}

func actionFingerprint(action contract.UnifiedActionV1) string {
	typ := strings.TrimSpace(action.Type)
	goal := strings.TrimSpace(action.OperationalGoal)
	if typ == "" && goal == "" {
		return ""
	}
	entities := append([]string{}, action.Scope.EntityIDs...)
	for i := range entities {
		entities[i] = strings.TrimSpace(entities[i])
	}
	sort.Strings(entities)
	return typ + "\n" + goal + "\n" + strings.Join(entities, ",")
}

func validateTopActionRequiredRefs(report *Report, out *contract.UnifiedAnalysisOutputV1, actionByID map[string]contract.UnifiedActionV1) {
	if out.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return
	}
	for i, id := range out.Dashboard.TopActionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		action, ok := actionByID[id]
		if !ok {
			continue
		}
		if strings.TrimSpace(action.ExecutionTier) != contract.ExecutionTierRequired {
			report.addError(CodeQuickActionRequiredReferenceMissing,
				fmt.Sprintf("$.dashboard.top_action_ids[%d]", i),
				"top_action_ids must reference execution_tier=required actions")
		}
	}
}

func validateAction(report *Report, path string, action contract.UnifiedActionV1, idx *PackReferenceIndex, actionIDs map[string]int, skuOwner map[string]string) {
	id := strings.TrimSpace(action.AnalysisActionID)
	if id == "" {
		report.addError(CodeEmptyRequiredText, path+".analysis_action_id", "empty analysis_action_id")
	} else if !actionIDPattern.MatchString(id) {
		report.addError(CodeInvalidActionIDFormat, path+".analysis_action_id", "analysis_action_id has invalid local format")
	} else if _, exists := actionIDs[id]; exists {
		report.addError(CodeOutputDuplicateActionID, path+".analysis_action_id", "duplicate analysis_action_id")
	} else {
		actionIDs[id] = 1
	}

	if !contract.ValidActionType(action.Type) {
		report.addError(CodeUnknownEnum, path+".type", "unknown action type")
	}
	if !contract.ValidActionCategory(action.Category) {
		report.addError(CodeUnknownEnum, path+".category", "unknown action category")
	}
	if !contract.ValidActionPriority(action.Priority) {
		report.addError(CodeUnknownEnum, path+".priority", "unknown action priority")
	}
	if tier := strings.TrimSpace(action.ExecutionTier); tier == "" || !contract.ValidExecutionTier(tier) {
		report.addError(CodeUnknownEnum, path+".execution_tier", "unknown or missing execution_tier")
	}
	if !contract.ValidConfidence(action.Confidence) {
		report.addError(CodeUnknownEnum, path+".confidence", "unknown confidence")
	}
	if !contract.ValidScopeType(action.Scope.ScopeType) {
		report.addError(CodeUnknownEnum, path+".scope.scope_type", "unknown scope_type")
	}
	switch strings.TrimSpace(action.Scope.ScopeType) {
	case "sku", "campaign":
		if len(action.Scope.EntityIDs) == 0 {
			report.addError(CodeOutputMissingScopeEntities, path+".scope.entity_ids", "sku/campaign scope requires non-empty entity_ids")
		}
	}

	requireText(report, path+".title", action.Title)
	requireText(report, path+".summary", action.Summary)
	requireText(report, path+".problem", action.Problem)
	requireText(report, path+".why_now", action.WhyNow)
	requireText(report, path+".operational_goal", action.OperationalGoal)
	requireText(report, path+".expected_effect", action.ExpectedEffect)
	requireText(report, path+".completion_criteria", action.CompletionCriteria)
	requireText(report, path+".evidence_summary", action.EvidenceSummary)

	entityCount := len(action.Scope.EntityIDs)
	if mentioned := ExtractMentionedEntityCount(action.Title); mentioned >= 0 && entityCount > 0 && mentioned != entityCount {
		report.addError(CodeActionTitleEntityCountMismatch, path+".title",
			fmt.Sprintf("title mentions %d entities but scope has %d", mentioned, entityCount))
	}
	if mentioned := ExtractMentionedEntityCount(action.Summary); mentioned >= 0 && entityCount > 0 && mentioned != entityCount {
		report.addError(CodeActionSummaryEntityCountMismatch, path+".summary",
			fmt.Sprintf("summary mentions %d entities but scope has %d", mentioned, entityCount))
	}
	if IsGenericExpectedEffect(action.ExpectedEffect) {
		report.addError(CodeActionExpectedResultTooGeneric, path+".expected_effect",
			"expected_effect is too generic or too short")
	}
	minSteps := 2
	if entityCount >= 2 || strings.EqualFold(strings.TrimSpace(action.Category), "stock") || strings.EqualFold(strings.TrimSpace(action.Category), "advertising") {
		minSteps = 3
	}
	if len(action.Steps) < minSteps {
		report.addError(CodeActionPlanTooShort, path+".steps",
			fmt.Sprintf("steps must contain at least %d items for this action", minSteps))
	}
	if utf8RuneCount(action.WhyNow) < 40 {
		report.addWarning(CodeActionSituationTooShort, path+".why_now", "why_now is thinner than the preferred situation depth")
	}
	if utf8RuneCount(action.Problem) < 20 {
		report.addWarning(CodeActionImportanceTooShort, path+".problem", "problem is thinner than the preferred importance depth")
	}

	if len(action.Steps) == 0 {
		report.addError(CodeEmptySteps, path+".steps", "steps must be non-empty")
	}
	seenSteps := map[string]struct{}{}
	for si, step := range action.Steps {
		sp := fmt.Sprintf("%s.steps[%d]", path, si)
		requireText(report, sp+".title", step.Title)
		requireText(report, sp+".description", step.Description)
		key := strings.TrimSpace(step.Title) + "\n" + strings.TrimSpace(step.Description)
		if _, ok := seenSteps[key]; ok {
			report.addError(CodeDuplicateStep, sp, "duplicate step text")
		}
		seenSteps[key] = struct{}{}
	}

	seenEntity := map[string]struct{}{}
	for ei, entityID := range action.Scope.EntityIDs {
		ep := fmt.Sprintf("%s.scope.entity_ids[%d]", path, ei)
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			report.addError(CodeEmptyRequiredText, ep, "empty entity id")
			continue
		}
		if _, ok := seenEntity[entityID]; ok {
			report.addError(CodeDuplicateEntityInAction, ep, "duplicate entity id within action")
			continue
		}
		seenEntity[entityID] = struct{}{}
		if idx != nil {
			scopeType := strings.TrimSpace(action.Scope.ScopeType)
			isSKU := idx.HasSKU(entityID)
			isCampaign := idx.HasCampaign(entityID)
			switch scopeType {
			case "sku":
				if !isSKU {
					report.addError(CodeOutputUnknownSKU, ep, "sku id not found in pack")
				} else if owner, ok := skuOwner[entityID]; ok && owner != id {
					report.addError(CodeOutputDuplicateSKUAcrossActions, ep, "sku already used in another action")
				} else if id != "" {
					skuOwner[entityID] = id
				}
			case "campaign":
				if !isCampaign {
					report.addError(CodeOutputUnknownCampaign, ep, "campaign id not found in pack")
				}
			default:
				if !isSKU && !isCampaign {
					report.addError(CodeOutputUnknownSKU, ep, "entity id not found in pack")
				}
				if isSKU {
					if owner, ok := skuOwner[entityID]; ok && owner != id {
						report.addError(CodeOutputDuplicateSKUAcrossActions, ep, "sku already used in another action")
					} else if id != "" {
						skuOwner[entityID] = id
					}
				}
			}
		}
	}

	for li, lim := range action.Limitations {
		validateLimitationCode(report, fmt.Sprintf("%s.limitations[%d]", path, li), lim, idx)
	}

	validateActionEvidence(report, path, action, idx)
	validateCompletionConditions(report, path, action, idx)
}

func validateActionIDRefs(report *Report, path string, refs []string, actionIDs map[string]int) {
	seen := map[string]struct{}{}
	for i, id := range refs {
		p := fmt.Sprintf("%s[%d]", path, i)
		id = strings.TrimSpace(id)
		if id == "" {
			report.addError(CodeEmptyRequiredText, p, "empty action id reference")
			continue
		}
		if _, ok := seen[id]; ok {
			report.addError(CodeDuplicateActionRef, p, "duplicate action id reference")
			continue
		}
		seen[id] = struct{}{}
		if _, ok := actionIDs[id]; !ok {
			report.addError(CodeOutputUnknownActionReference, p, "unknown action id reference")
		}
	}
}

func validateLimitations(report *Report, limitations []string, idx *PackReferenceIndex) {
	seen := map[string]struct{}{}
	for i, lim := range limitations {
		p := fmt.Sprintf("$.limitations[%d]", i)
		validateLimitationCode(report, p, lim, idx)
		key := strings.TrimSpace(lim)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			report.addError(CodeDuplicateLimitation, p, "duplicate limitation")
		}
		seen[key] = struct{}{}
	}
}

func validateLimitationCode(report *Report, path, lim string, idx *PackReferenceIndex) {
	lim = strings.TrimSpace(lim)
	if lim == "" {
		report.addError(CodeEmptyRequiredText, path, "empty limitation")
		return
	}
	if idx != nil && !idx.HasLimitation(lim) && !isAllowlistedOutputLimitation(lim) {
		report.addError(CodeOutputUnknownLimitation, path, "unknown limitation code")
	}
}

func isAllowlistedOutputLimitation(code string) bool {
	switch code {
	case "insufficient_data", "partial_data", "source_unavailable", "analysis_limited":
		return true
	default:
		return false
	}
}

func validateDashboardTexts(report *Report, dash *contract.UnifiedDashboardV1, outcome string, idx *PackReferenceIndex) {
	if dash == nil {
		return
	}
	if dash.AccountSummary.Headline != "" {
		requireText(report, "$.dashboard.account_summary.headline", dash.AccountSummary.Headline)
	}
	if dash.AccountSummary.ExecutiveSummary != "" {
		requireText(report, "$.dashboard.account_summary.executive_summary", dash.AccountSummary.ExecutiveSummary)
	}

	if !isMaterialOutcome(outcome) {
		return
	}

	// material_actions_found / no_material_actions both describe a completed
	// analysis of a real account state: the seller-facing AI analytical layer
	// (headline + executive_summary + seller_pulse narrative) must be present
	// and non-placeholder. This is purely an AI-analytical-layer contract
	// check — it never validates or requires factual KPI/array content,
	// which is backend-owned (facts pipeline), not the AI's responsibility.
	if strings.TrimSpace(dash.AccountSummary.Headline) == "" {
		report.addError(CodeMissingAccountSummaryHeadline, "$.dashboard.account_summary.headline", "headline is required for "+outcome)
	}
	if strings.TrimSpace(dash.AccountSummary.ExecutiveSummary) == "" {
		report.addError(CodeMissingExecutiveSummary, "$.dashboard.account_summary.executive_summary", "executive_summary is required for "+outcome)
	}

	validateSellerPulseNarrative(report, dash.SellerPulse.Narrative, outcome)
	validateMaterialHealthScore(report, dash.AccountSummary, outcome)
	validateMaterialWhatChanged(report, dash.WhatChanged, outcome)
	validateAdvertisingChecklist(report, dash, idx, outcome)
	validateAdvertisingNarrativeStructure(report, dash, idx, outcome)
	validateAdvertisingClaimGrounding(report, dash, idx, outcome)
	validateCategorySummaries(report, dash, idx, outcome)
	validateFunnelAnalysis(report, dash, outcome)
	validateRiskDistribution(report, dash, outcome)
	validateSKUFocus(report, dash, idx, outcome)
	validateDashboardDiagnosticsContract(report, dash, outcome)
	validateSellerPulseClaimGrounding(report, dash, idx, outcome)
}

// minAdvertisingChecklistItems is the minimum number of AI-authored
// checklist items required whenever the Pack carries advertising/campaign
// data. The backend never invents these items — the AI must emit them.
const minAdvertisingChecklistItems = 2

// validateAdvertisingChecklist rejects material outcomes when the Pack has
// advertising/campaign data but the AI's ad_efficiency.narrative.checklist
// is empty or has fewer than minAdvertisingChecklistItems concrete items.
func validateAdvertisingChecklist(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if idx == nil || !idx.HasAdvertisingData() {
		return
	}
	narrative := dash.AdEfficiency.Narrative
	if narrative == nil || len(narrative.Checklist) < minAdvertisingChecklistItems {
		report.addError(CodeMissingAdvertisingChecklist, "$.dashboard.ad_efficiency.narrative.checklist",
			fmt.Sprintf("pack has advertising data but checklist has fewer than %d items for %s", minAdvertisingChecklistItems, outcome))
		return
	}
	for i, item := range narrative.Checklist {
		path := fmt.Sprintf("$.dashboard.ad_efficiency.narrative.checklist[%d]", i)
		requireText(report, path+".title", item.Title)
		requireText(report, path+".text", item.Text)
	}
}

// validateCategorySummaries rejects material outcomes when the Pack has
// category data (sku_summary.csv category_id column) but
// dashboard.category_summaries is empty, or any entry is missing the
// required category_score/headline fields.
func validateCategorySummaries(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if idx == nil || !idx.HasCategories() {
		return
	}
	if len(dash.CategorySummaries) == 0 {
		report.addError(CodeMissingCategorySummaries, "$.dashboard.category_summaries",
			"pack has categories but category_summaries is empty for "+outcome)
		return
	}
	for i, c := range dash.CategorySummaries {
		path := fmt.Sprintf("$.dashboard.category_summaries[%d]", i)
		if c.CategoryScore == nil {
			report.addError(CodeIncompleteCategorySummary, path+".category_score", "category_score is required when categories are present")
		}
		if strings.TrimSpace(c.Headline) == "" {
			report.addError(CodeIncompleteCategorySummary, path+".headline", "headline is required when categories are present")
		}
		if strings.TrimSpace(c.KeyRisk) == "" {
			report.addError(CodeIncompleteCategorySummary, path+".key_risk", "key_risk is required when categories are present")
		}
		if strings.TrimSpace(c.RecommendedAction) == "" {
			report.addError(CodeIncompleteCategorySummary, path+".recommended_action", "recommended_action is required when categories are present")
		}
	}
}

// validateFunnelAnalysis requires AI funnel interpretation for material
// outcomes. Backend facts supply stages; the AI must still emit short_summary
// or main_conclusion.
func validateFunnelAnalysis(report *Report, dash *contract.UnifiedDashboardV1, outcome string) {
	if !isMaterialOutcome(outcome) {
		return
	}
	ai := dash.FunnelAnalysis.AIAnalysis
	if strings.TrimSpace(ai.ShortSummary) == "" && strings.TrimSpace(ai.MainConclusion) == "" {
		report.addError(CodeMissingFunnelAnalysis, "$.dashboard.funnel_analysis.ai_analysis",
			"short_summary or main_conclusion is required for "+outcome)
	}
}

// validateRiskDistribution requires at least one risk group when the AI
// claims material_actions_found.
func validateRiskDistribution(report *Report, dash *contract.UnifiedDashboardV1, outcome string) {
	if outcome != contract.AnalysisOutcomeMaterialActionsFound {
		return
	}
	if len(dash.RiskDistribution.Groups) == 0 {
		report.addError(CodeMissingRiskDistribution, "$.dashboard.risk_distribution.groups",
			"material_actions_found requires at least one risk_distribution group")
		return
	}
	for i, g := range dash.RiskDistribution.Groups {
		path := fmt.Sprintf("$.dashboard.risk_distribution.groups[%d]", i)
		requireText(report, path+".group", g.Group)
		requireText(report, path+".label", g.Label)
		requireText(report, path+".severity", g.Severity)
	}
}

// validateSKUFocus requires at least one sku_focus card when the Pack has SKUs.
func validateSKUFocus(report *Report, dash *contract.UnifiedDashboardV1, idx *PackReferenceIndex, outcome string) {
	if !isMaterialOutcome(outcome) || idx == nil || !idx.HasSKUData() {
		return
	}
	focus := dash.SKUFocus
	if len(focus.OrderDrop)+len(focus.StockRisk)+len(focus.NoSalesWithStock)+len(focus.SlowTurnover)+len(focus.KeySKUs) == 0 {
		report.addError(CodeMissingSKUFocus, "$.dashboard.sku_focus",
			"pack has SKUs but sku_focus has no entries for "+outcome)
	}
}

var requiredWhatChangedKeys = []string{"worsened", "new_risks", "unchanged_risks", "improved"}

func validateMaterialHealthScore(report *Report, summary contract.UnifiedAccountSummaryV1, outcome string) {
	if summary.HealthScore == nil {
		report.addError(CodeMissingHealthScore, "$.dashboard.account_summary.health_score", "health_score is required for "+outcome)
		return
	}
	score := *summary.HealthScore
	if score < 0 || score > 100 {
		report.addError(CodeInvalidHealthScore, "$.dashboard.account_summary.health_score",
			fmt.Sprintf("health_score %d is out of range 0..100", score))
		return
	}
	if len(summary.RiskDrivers) == 0 && outcome == contract.AnalysisOutcomeMaterialActionsFound {
		report.addError(CodeHealthScoreFactorsMissing, "$.dashboard.account_summary.risk_drivers",
			"health_score requires non-empty risk_drivers/factors for "+outcome)
	}
	if score == 0 && !hasCriticalOrHighRiskDriver(summary.RiskDrivers) {
		report.addError(CodeHealthScoreDefaultedToZero, "$.dashboard.account_summary.health_score",
			"health_score 0 is only allowed when critical/high risk_drivers confirm a critical cabinet")
	}
	if sev := strings.TrimSpace(summary.Severity); sev != "" {
		want := healthstatus.FromScore(summary.HealthScore).Severity
		got := healthstatus.NormalizeSeverity(sev)
		if want != "" && got != want {
			report.addError(CodeHealthScoreStatusMismatch, "$.dashboard.account_summary.severity",
				fmt.Sprintf("severity %q does not match health_score %d band (want %q)", sev, score, want))
		}
	}
}

func hasCriticalOrHighRiskDriver(drivers []contract.UnifiedRiskDriverV1) bool {
	for _, d := range drivers {
		switch strings.ToLower(strings.TrimSpace(d.Severity)) {
		case "critical", "high":
			return true
		}
	}
	return false
}

func validateMaterialWhatChanged(report *Report, wc contract.UnifiedWhatChangedV1, outcome string) {
	hasSummary := strings.TrimSpace(wc.Summary) != "" || strings.TrimSpace(wc.MainConclusion) != ""
	if !hasSummary {
		report.addError(CodeMissingWhatChanged, "$.dashboard.what_changed", "what_changed summary/main_conclusion is required for "+outcome)
	}
	if len(wc.Groups) != len(requiredWhatChangedKeys) {
		report.addError(CodeInvalidWhatChangedKeys, "$.dashboard.what_changed.groups",
			fmt.Sprintf("what_changed must have exactly %d groups (%v), got %d", len(requiredWhatChangedKeys), requiredWhatChangedKeys, len(wc.Groups)))
		return
	}
	hasItem := false
	for i, want := range requiredWhatChangedKeys {
		got := strings.TrimSpace(wc.Groups[i].Key)
		if got != want {
			report.addError(CodeInvalidWhatChangedKeys, fmt.Sprintf("$.dashboard.what_changed.groups[%d].key", i),
				fmt.Sprintf("got %q want %q", got, want))
		}
		if len(wc.Groups[i].Items) > 0 {
			hasItem = true
		}
	}
	if !hasItem {
		report.addError(CodeMissingWhatChanged, "$.dashboard.what_changed.groups",
			"what_changed requires at least one group with an item for "+outcome)
	}
}

// validateSellerPulseNarrative requires a resolved (non-pending/stale/fallback)
// seller_pulse narrative with non-empty headline/summary whenever the
// analysis outcome is material. It intentionally does not touch
// dash.SellerPulse.KPIs/Trend7D — those are backend-owned factual arrays.
func validateSellerPulseNarrative(report *Report, narrative *contract.UnifiedSellerPulseNarrativeV1, outcome string) {
	if narrative == nil {
		report.addError(CodeMissingSellerPulseNarrative, "$.dashboard.seller_pulse.narrative", "seller_pulse narrative is required for "+outcome)
		return
	}
	status := strings.ToLower(strings.TrimSpace(narrative.Status))
	switch status {
	case "", "pending", "stale", "fallback", "not_enough_data", "source_not_available":
		report.addError(CodeInvalidSellerPulseNarrativeStatus, "$.dashboard.seller_pulse.narrative.status",
			fmt.Sprintf("seller_pulse narrative status %q is not resolved for %s", narrative.Status, outcome))
	}
	if strings.TrimSpace(narrative.Headline) == "" {
		report.addError(CodeMissingSellerPulseNarrativeText, "$.dashboard.seller_pulse.narrative.headline", "seller_pulse narrative headline is required for "+outcome)
	}
	if strings.TrimSpace(narrative.Summary) == "" {
		report.addError(CodeMissingSellerPulseNarrativeText, "$.dashboard.seller_pulse.narrative.summary", "seller_pulse narrative summary is required for "+outcome)
	}
}

func requireText(report *Report, path, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		report.addError(CodeEmptyRequiredText, path, "required text is empty")
		return
	}
	if onlyPunctuation(trimmed) {
		report.addError(CodeEmptyRequiredText, path, "required text is punctuation-only")
		return
	}
	if placeholderRe.MatchString(trimmed) {
		report.addError(CodePlaceholderText, path, "required text contains placeholder")
	}
	if len([]rune(trimmed)) > 4000 {
		report.addError(CodeTextTooLong, path, "required text exceeds length limit")
	}
}

func onlyPunctuation(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func finalize(report Report, out *contract.UnifiedAnalysisOutputV1) Report {
	sort.SliceStable(report.Errors, func(i, j int) bool {
		return lessIssue(report.Errors[i], report.Errors[j])
	})
	sort.SliceStable(report.Warnings, func(i, j int) bool {
		return lessIssue(report.Warnings[i], report.Warnings[j])
	})
	report.Valid = len(report.Errors) == 0
	if out != nil {
		report.ActionCount = len(out.Actions)
		report.TopActionCount = len(out.Dashboard.TopActionIDs)
	}
	return report
}

func lessIssue(a, b Issue) bool {
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Message < b.Message
}
