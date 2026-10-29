package outputvalidator

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

const (
	CodeSellerPulseConclusionTooShallow      = "seller_pulse_conclusion_too_shallow"
	CodeSellerPulseHeadlineMissing           = "seller_pulse_headline_missing"
	CodeSellerPulseSummaryMissing            = "seller_pulse_summary_missing"
	CodeSellerPulseKeyPointsMissing          = "seller_pulse_key_points_missing"
	CodeSellerPulseKeyPointDuplicate         = "seller_pulse_key_point_duplicate"
	CodeSellerPulseTextTooDense              = "seller_pulse_text_too_dense"
	CodeSellerPulseFactReferenceInvalid      = "seller_pulse_fact_reference_invalid"
	CodeSellerPulseAIFactMismatch            = "seller_pulse_ai_fact_mismatch"
	CodeSellerPulseAIPeriodMismatch          = "seller_pulse_ai_period_mismatch"
	CodeSellerPulseAIMetricMismatch          = "seller_pulse_ai_metric_mismatch"
	CodeSellerPulseCrossPeriodClaimInvalid   = "seller_pulse_cross_period_claim_invalid"
	CodeSellerPulseSummaryNotGrounded        = "seller_pulse_summary_not_grounded"
	CodeSellerPulseImportantFactsMissing     = "seller_pulse_important_facts_missing"
	CodeSellerPulseImportantFactIncomplete   = "seller_pulse_important_fact_incomplete"
	CodeSellerPulseImportantFactDuplicate    = "seller_pulse_important_fact_duplicate"
	CodeSellerPulseSummaryImportantDuplicate = "seller_pulse_summary_important_fact_duplicate"
	CodeSellerPulseImportantFactNotGrounded  = "seller_pulse_important_fact_not_grounded"
	CodeDiagnosticsConclusionTooShallow      = "diagnostics_conclusion_too_shallow"
	CodeDashboardSignalDuplicate             = "dashboard_signal_duplicate"
	CodeDashboardSignalSectionConflict       = "dashboard_signal_section_conflict"
	CodeDashboardNewRiskWithoutPreviousState = "dashboard_new_risk_without_previous_state"
	CodeDashboardActiveProblemWithoutPrev    = "dashboard_active_problem_without_previous_state"
	CodeHealthStatusMissing                  = "health_status_missing"
	CodeHealthImprovementLeversMissing       = "health_improvement_levers_missing"
	CodeHealthImprovementIntroMissing        = "health_improvement_intro_missing"
	CodeHealthImprovementItemIncomplete      = "health_improvement_item_incomplete"
	CodeHealthImprovementItemGeneric         = "health_improvement_item_generic"
	CodeSupportPointsMissing                 = "support_points_missing"
	CodeNonRussianPresentationTerm           = "non_russian_presentation_term"
	CodeDashboardPeriodSemanticsInvalid      = "dashboard_period_semantics_invalid"
	CodeDashboardAIMarkupInvalid             = "dashboard_ai_markup_invalid"
)

var forbiddenPresentationTerms = []string{
	" DRR", "DRR ", "DRR.", "(DRR",
	"пулсь селлера", "пульс селлера", "seller pulse",
	"ИИ-", "ИИ ", " ИИ",
}

var genericImprovementTitles = []string{
	"улучшить показатели",
	"работать над продажами",
	"оптимизировать кабинет",
	"улучшить кабинет",
	"повысить эффективность",
}

// validateDashboardDiagnosticsContract enforces seller-pulse / diagnostics
// depth, signal ownership, RU presentation, levers and support points.
func validateDashboardDiagnosticsContract(report *Report, dash *contract.UnifiedDashboardV1, outcome string) {
	if dash == nil || !isMaterialOutcome(outcome) {
		return
	}
	validateSellerPulseDepth(report, dash.SellerPulse.Narrative, outcome)
	validateDiagnosticsConclusionDepth(report, dash.WhatChanged, outcome)
	validateImprovementLevers(report, dash.AccountSummary, outcome)
	validateSupportPoints(report, dash.WhatChanged, dash.AccountSummary, outcome)
	validateHealthSeverityPresent(report, dash.AccountSummary, outcome)
	validateSignalOwnership(report, dash)
	validateNewActivePreviousState(report, dash.WhatChanged)
	validateWhatChangedSellerCopy(report, dash.WhatChanged)
	validateRussianPresentation(report, dash)
	validatePeriodSemantics(report, dash)
}

func validateSellerPulseDepth(report *Report, narrative *contract.UnifiedSellerPulseNarrativeV1, outcome string) {
	if narrative == nil {
		return
	}
	headline := strings.TrimSpace(narrative.Headline)
	if headline == "" {
		report.addError(CodeSellerPulseHeadlineMissing, "$.dashboard.seller_pulse.narrative.headline",
			"seller_pulse headline required for "+outcome)
	} else {
		lower := strings.ToLower(strings.TrimRight(headline, ".…"))
		if lower == "пульс продавца" || lower == "ai-резюме" || lower == "ai-резюме по кабинету" {
			report.addError(CodeSellerPulseHeadlineMissing, "$.dashboard.seller_pulse.narrative.headline",
				"seller_pulse headline must be a concrete thesis, not a section label")
		}
		if len([]rune(headline)) > 180 {
			report.addError(CodeSellerPulseTextTooDense, "$.dashboard.seller_pulse.narrative.headline",
				"seller_pulse headline is too long for a thesis")
		}
	}
	summary := strings.TrimSpace(narrative.Summary)
	if summary == "" {
		report.addError(CodeSellerPulseSummaryMissing, "$.dashboard.seller_pulse.narrative.summary",
			"seller_pulse summary required for "+outcome)
	} else {
		sc := sentenceCount(summary)
		if (sc < 2 || sc > 4 || len([]rune(summary)) < 80) && !(sc >= 1 && len([]rune(summary)) >= 160) {
			report.addError(CodeSellerPulseConclusionTooShallow, "$.dashboard.seller_pulse.narrative.summary",
				"seller_pulse summary must be 2–3 short analytical sentences for "+outcome)
		}
		if len([]rune(summary)) > 700 {
			report.addError(CodeSellerPulseTextTooDense, "$.dashboard.seller_pulse.narrative.summary",
				"seller_pulse summary is too dense; keep 2–3 short sentences")
		}
	}
	if containsUnsafeMarkup(headline) || containsUnsafeMarkup(summary) {
		report.addError(CodeDashboardAIMarkupInvalid, "$.dashboard.seller_pulse.narrative",
			"seller_pulse narrative must not contain HTML or unsafe markup")
	}

	keyPoints := narrative.KeyPoints
	// Compatibility: older fixtures may still only fill important_facts as key points.
	if len(keyPoints) == 0 && len(narrative.ImportantFacts) > 0 && len(narrative.MetricClaims) == 0 {
		keyPoints = narrative.ImportantFacts
	}
	nKey := len(keyPoints)
	if nKey < 3 || nKey > 5 {
		report.addError(CodeSellerPulseKeyPointsMissing, "$.dashboard.seller_pulse.narrative.key_points",
			"key_points must contain 3–5 items for "+outcome)
	}
	seenKeyFP := map[string]struct{}{}
	for i, fact := range keyPoints {
		path := fmt.Sprintf("$.dashboard.seller_pulse.narrative.key_points[%d]", i)
		validatePulseSignalItem(report, path, fact, headline, seenKeyFP, CodeSellerPulseKeyPointsMissing, CodeSellerPulseKeyPointDuplicate, true)
	}

	nFacts := len(narrative.ImportantFacts)
	if nFacts < 3 || nFacts > 5 {
		report.addError(CodeSellerPulseImportantFactsMissing, "$.dashboard.seller_pulse.narrative.important_facts",
			"important_facts must contain 3–5 items for "+outcome)
	}
	seenImpFP := map[string]struct{}{}
	for i, fact := range narrative.ImportantFacts {
		path := fmt.Sprintf("$.dashboard.seller_pulse.narrative.important_facts[%d]", i)
		validatePulseSignalItem(report, path, fact, headline, seenImpFP, CodeSellerPulseImportantFactIncomplete, CodeSellerPulseImportantFactDuplicate, false)
	}

	// Cross-array duplicate fingerprints between key_points and important_facts.
	for i, fact := range narrative.ImportantFacts {
		for _, kp := range keyPoints {
			if nearDuplicatePulseText(fact.Title, fact.Explanation, kp.Title, kp.Explanation) {
				report.addError(CodeSellerPulseSummaryImportantDuplicate,
					fmt.Sprintf("$.dashboard.seller_pulse.narrative.important_facts[%d]", i),
					"important_facts text nearly identical to a key_point")
				break
			}
		}
	}

	if len(narrative.MetricClaims) == 0 && looksNumericHeavy(summary) {
		report.addError(CodeSellerPulseSummaryNotGrounded, "$.dashboard.seller_pulse.narrative.metric_claims",
			"numeric seller_pulse summary requires metric_claims grounding")
	}
	for i, claim := range narrative.MetricClaims {
		path := fmt.Sprintf("$.dashboard.seller_pulse.narrative.metric_claims[%d]", i)
		validateMetricClaimStructure(report, path, claim)
	}
}

func validatePulseSignalItem(
	report *Report,
	path string,
	fact contract.UnifiedSellerPulsePointV1,
	headline string,
	seenFP map[string]struct{},
	incompleteCode, duplicateCode string,
	requireClaimsForNumbers bool,
) {
	title := strings.TrimSpace(fact.Title)
	text := strings.TrimSpace(fact.Explanation)
	if title == "" || text == "" {
		report.addError(incompleteCode, path, "each item needs title and explanation")
	}
	if containsUnsafeMarkup(title) || containsUnsafeMarkup(text) {
		report.addError(CodeDashboardAIMarkupInvalid, path, "item must not contain HTML or unsafe markup")
	}
	if headline != "" && strings.EqualFold(strings.TrimRight(title, ".…"), strings.TrimRight(headline, ".…")) {
		report.addError(duplicateCode, path, "item title must not duplicate headline")
	}
	fp := keyPointFingerprint(title, text)
	if _, ok := seenFP[fp]; ok {
		report.addError(duplicateCode, path, "duplicate item fingerprint")
	}
	seenFP[fp] = struct{}{}
	if len(fact.FactIDs) == 0 && len(fact.Claims) == 0 {
		report.addError(CodeSellerPulseFactReferenceInvalid, path+".fact_ids",
			"each item should reference at least one fact_id or claim")
	}
	if periodType := strings.TrimSpace(fact.PeriodType); periodType != "" {
		if !validPulsePeriodType(periodType) {
			report.addError(CodeSellerPulseAIPeriodMismatch, path+".period_type",
				"period_type must be current_day|previous_day|current_7d|previous_7d")
		}
	} else if looksNumericHeavy(title + " " + text) {
		report.addError(CodeSellerPulseAIPeriodMismatch, path+".period_type",
			"numeric assertions require period_type")
	}
	for j, claim := range fact.Claims {
		validateMetricClaimStructure(report, fmt.Sprintf("%s.claims[%d]", path, j), claim)
	}
	if requireClaimsForNumbers && looksNumericHeavy(title+" "+text) && len(fact.Claims) == 0 && fact.Value == nil {
		report.addError(CodeSellerPulseSummaryNotGrounded, path+".claims",
			"numeric key point requires claims or value grounding")
	}
	if !requireClaimsForNumbers && looksNumericHeavy(title+" "+text) && len(fact.Claims) == 0 && len(fact.FactIDs) == 0 {
		report.addError(CodeSellerPulseImportantFactNotGrounded, path,
			"important fact with numbers must be grounded")
	}
}

func validateMetricClaimStructure(report *Report, path string, claim contract.UnifiedMetricClaimV1) {
	if strings.TrimSpace(claim.FactID) == "" {
		report.addError(CodeSellerPulseAIFactMismatch, path+".fact_id", "fact_id is required")
	}
	if strings.TrimSpace(claim.MetricCode) == "" {
		report.addError(CodeSellerPulseAIMetricMismatch, path+".metric_code", "metric_code is required")
	}
	if !validPulsePeriodType(claim.PeriodType) {
		report.addError(CodeSellerPulseAIPeriodMismatch, path+".period_type",
			"period_type must be current_day|previous_day|current_7d|previous_7d")
	}
	if strings.TrimSpace(claim.PeriodStart) == "" || strings.TrimSpace(claim.PeriodEnd) == "" {
		report.addError(CodeSellerPulseAIPeriodMismatch, path+".period_start",
			"period_start and period_end are required")
	}
	if claim.Value == nil {
		report.addError(CodeSellerPulseAIFactMismatch, path+".value", "claim value is required")
	}
	// Reject weekly fact labeled as day via fact_id / metric_code heuristics.
	fid := strings.ToLower(claim.FactID)
	mc := strings.ToLower(claim.MetricCode)
	if (strings.Contains(mc, "_7d") || strings.Contains(fid, "current_7d") || strings.Contains(fid, "previous_7d")) &&
		(claim.PeriodType == "current_day" || claim.PeriodType == "previous_day") {
		report.addError(CodeSellerPulseCrossPeriodClaimInvalid, path,
			"weekly metric/fact cannot use day period_type")
	}
	if (strings.HasSuffix(mc, "_today") || strings.Contains(fid, "current_day") || strings.Contains(fid, "previous_day")) &&
		(claim.PeriodType == "current_7d" || claim.PeriodType == "previous_7d") &&
		!strings.Contains(mc, "_7d") {
		report.addError(CodeSellerPulseCrossPeriodClaimInvalid, path,
			"day metric/fact cannot use week period_type")
	}
}

func validPulsePeriodType(v string) bool {
	switch strings.TrimSpace(v) {
	case "current_day", "previous_day", "current_7d", "previous_7d":
		return true
	default:
		return false
	}
}

func looksNumericHeavy(text string) bool {
	digits := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits >= 3
}

func signalContentFingerprint(fact contract.UnifiedSellerPulsePointV1) string {
	return signalFingerprint(fact.SignalType, firstNonEmpty(fact.MetricCode, fact.Metric), fact.Direction, firstNonEmpty(fact.PeriodType, fact.Period), fact.EntityIDs)
}

func nearDuplicatePulseText(titleA, explA, titleB, explB string) bool {
	a := normalizePulseText(titleA + " " + explA)
	b := normalizePulseText(titleB + " " + explB)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// Near-identical if one contains the other and length ratio is high.
	if strings.Contains(a, b) || strings.Contains(b, a) {
		la, lb := len([]rune(a)), len([]rune(b))
		minL, maxL := la, lb
		if minL > maxL {
			minL, maxL = maxL, minL
		}
		return maxL > 0 && float64(minL)/float64(maxL) >= 0.85
	}
	return false
}

func normalizePulseText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func validateDiagnosticsConclusionDepth(report *Report, wc contract.UnifiedWhatChangedV1, outcome string) {
	main := strings.TrimSpace(wc.MainConclusion)
	if sentenceCount(main) < 4 || len([]rune(main)) < 200 {
		report.addError(CodeDiagnosticsConclusionTooShallow, "$.dashboard.what_changed.main_conclusion",
			"diagnostics main_conclusion must be a 4–6 sentence synthesis for "+outcome)
	}
}

func validateImprovementLevers(report *Report, summary contract.UnifiedAccountSummaryV1, outcome string) {
	if len(summary.RiskDrivers) == 0 {
		return
	}
	intro := strings.TrimSpace(summary.ImprovementSummary)
	if intro == "" {
		report.addError(CodeHealthImprovementIntroMissing, "$.dashboard.account_summary.improvement_summary",
			"improvement_summary (intro) required when risk_drivers are present for "+outcome)
	} else if containsUnsafeMarkup(intro) {
		report.addError(CodeDashboardAIMarkupInvalid, "$.dashboard.account_summary.improvement_summary",
			"improvement intro must not contain HTML or unsafe markup")
	} else if strings.Contains(strings.ToLower(intro), "что улучшит индекс") {
		report.addError(CodeHealthImprovementIntroMissing, "$.dashboard.account_summary.improvement_summary",
			"improvement intro must not repeat the section title")
	}
	n := len(summary.ImprovementLevers)
	if n == 0 || n > 4 {
		report.addError(CodeHealthImprovementLeversMissing, "$.dashboard.account_summary.improvement_levers",
			"improvement_levers must contain 1–4 items when risk_drivers are present for "+outcome)
		return
	}
	for i, lever := range summary.ImprovementLevers {
		path := fmt.Sprintf("$.dashboard.account_summary.improvement_levers[%d]", i)
		if strings.TrimSpace(lever.Title) == "" || strings.TrimSpace(lever.Explanation) == "" ||
			strings.TrimSpace(lever.FactorID) == "" || strings.TrimSpace(lever.ExpectedDirection) == "" {
			report.addError(CodeHealthImprovementItemIncomplete, path,
				"each improvement item needs title, explanation, factor_id, expected_direction")
		}
		if containsUnsafeMarkup(lever.Title) || containsUnsafeMarkup(lever.Explanation) {
			report.addError(CodeDashboardAIMarkupInvalid, path, "improvement item must not contain HTML or unsafe markup")
		}
		titleLower := strings.ToLower(strings.TrimSpace(lever.Title))
		for _, generic := range genericImprovementTitles {
			if titleLower == generic || strings.HasPrefix(titleLower, generic) {
				report.addError(CodeHealthImprovementItemGeneric, path+".title",
					"improvement item title is too generic")
				break
			}
		}
	}
}

func validateSupportPoints(report *Report, wc contract.UnifiedWhatChangedV1, summary contract.UnifiedAccountSummaryV1, outcome string) {
	var improved *contract.UnifiedWhatChangedGroupV1
	for i := range wc.Groups {
		if wc.Groups[i].Key == "improved" {
			improved = &wc.Groups[i]
			break
		}
	}
	if improved == nil {
		return
	}
	if strings.TrimSpace(improved.Label) != "" && !strings.EqualFold(strings.TrimSpace(improved.Label), "Точки опоры") &&
		!strings.Contains(strings.ToLower(improved.Label), "точк") {
		// Soft: prefer Точки опоры but allow legacy during transition only if items exist.
	}
	hasPositiveSignal := len(summary.RiskDrivers) > 0 || summary.HealthScore != nil
	if hasPositiveSignal && len(improved.Items) == 0 {
		report.addError(CodeSupportPointsMissing, "$.dashboard.what_changed.groups[improved].items",
			"support points (improved/Точки опоры) must be filled when positive/stable signals are available for "+outcome)
	}
}

func validateHealthSeverityPresent(report *Report, summary contract.UnifiedAccountSummaryV1, outcome string) {
	if summary.HealthScore == nil {
		return
	}
	sev := strings.ToLower(strings.TrimSpace(summary.Severity))
	switch sev {
	case "critical", "high", "medium", "low", "attention":
		return
	case "warning":
		report.addError(CodeHealthStatusMissing, "$.dashboard.account_summary.severity",
			"severity=warning is not a canonical health status; use critical|high|medium|low")
	default:
		report.addError(CodeHealthStatusMissing, "$.dashboard.account_summary.severity",
			"canonical severity required when health_score is set for "+outcome)
	}
}

func validateSignalOwnership(report *Report, dash *contract.UnifiedDashboardV1) {
	type hit struct {
		path    string
		section string
	}
	byID := map[string]hit{}
	byFP := map[string]hit{}

	add := func(path, id, section, sigType, metric, direction, period string, entities []string) {
		id = strings.TrimSpace(id)
		if id != "" {
			if prev, ok := byID[id]; ok && prev.section != section && section != "" && prev.section != "" {
				// Pulse bullets may intentionally restate a what_changed signal at a
				// different depth; keep uniqueness as a warning rather than blocking publish.
				if isPulseSection(prev.section) || isPulseSection(section) {
					report.addWarning(CodeDashboardSignalSectionConflict, path+".signal_id",
						fmt.Sprintf("signal_id %q used in %s and %s", id, prev.section, section))
				} else {
					report.addError(CodeDashboardSignalSectionConflict, path+".signal_id",
						fmt.Sprintf("signal_id %q used in %s and %s", id, prev.section, section))
				}
			} else if id != "" {
				byID[id] = hit{path: path, section: section}
			}
		}
		fp := signalFingerprint(sigType, metric, direction, period, entities)
		if fp == "" {
			return
		}
		if prev, ok := byFP[fp]; ok && prev.section != section && isExclusiveSection(prev.section) && isExclusiveSection(section) {
			// seller_pulse_important may briefly restate a worsened metric with a
			// different angle; only hard-fail when both sides are what_changed_*
			// buckets or index_drivers (true card duplication).
			if bothDetailedCardSections(prev.section, section) {
				report.addError(CodeDashboardSignalDuplicate, path,
					fmt.Sprintf("duplicate signal fingerprint across %s and %s", prev.section, section))
			} else {
				report.addWarning(CodeDashboardSignalDuplicate, path,
					fmt.Sprintf("related signal fingerprint across %s and %s", prev.section, section))
			}
		} else {
			byFP[fp] = hit{path: path, section: section}
		}
	}

	if n := dash.SellerPulse.Narrative; n != nil {
		for i, fact := range n.KeyPoints {
			path := fmt.Sprintf("$.dashboard.seller_pulse.narrative.key_points[%d]", i)
			section := fact.PrimarySection
			if section == "" {
				section = "seller_pulse_key_point"
			}
			add(path, fact.SignalID, section, fact.SignalType, firstNonEmpty(fact.MetricCode, fact.Metric), fact.Direction, firstNonEmpty(fact.PeriodType, fact.Period), fact.EntityIDs)
		}
		for i, fact := range n.ImportantFacts {
			path := fmt.Sprintf("$.dashboard.seller_pulse.narrative.important_facts[%d]", i)
			section := fact.PrimarySection
			if section == "" {
				section = "seller_pulse_important"
			}
			add(path, fact.SignalID, section, fact.SignalType, firstNonEmpty(fact.MetricCode, fact.Metric), fact.Direction, firstNonEmpty(fact.PeriodType, fact.Period), fact.EntityIDs)
		}
	}
	for i, g := range dash.WhatChanged.Groups {
		for j, item := range g.Items {
			path := fmt.Sprintf("$.dashboard.what_changed.groups[%d].items[%d]", i, j)
			section := item.PrimarySection
			if section == "" {
				section = "what_changed_" + g.Key
			}
			add(path, item.SignalID, section, item.SignalType, item.Metric, item.Direction, item.Period, item.EntityIDs)
		}
	}
}

func validateNewActivePreviousState(report *Report, wc contract.UnifiedWhatChangedV1) {
	for i, g := range wc.Groups {
		for j, item := range g.Items {
			path := fmt.Sprintf("$.dashboard.what_changed.groups[%d].items[%d]", i, j)
			switch g.Key {
			case "new_risks":
				if strings.TrimSpace(item.PreviousState) == "" && strings.TrimSpace(item.FirstSeenPublicationID) == "" {
					report.addWarning(CodeDashboardNewRiskWithoutPreviousState, path,
						"new_risks item requires previous_state or first_seen_publication_id proof")
				}
			case "unchanged_risks":
				if strings.TrimSpace(item.PreviousState) == "" {
					// Soft: empty previous_state is common on first-run / when AI
					// cannot cite a prior publication; keep as warning so pulse
					// grounding fixes are not blocked by this side channel.
					report.addWarning(CodeDashboardActiveProblemWithoutPrev, path,
						"unchanged_risks/active problem requires previous_state proof")
				}
			}
		}
	}
}

func validateWhatChangedSellerCopy(report *Report, wc contract.UnifiedWhatChangedV1) {
	seenChangeIDs := map[string]string{}
	checkText := func(path, text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if containsLongNumericID(text) {
			report.addError(CodeNonRussianPresentationTerm, path,
				"what_changed seller copy must not contain numeric campaign/product IDs")
		}
	}
	checkText("$.dashboard.what_changed.summary", wc.Summary)
	checkText("$.dashboard.what_changed.main_conclusion", wc.MainConclusion)

	for i, g := range wc.Groups {
		gPath := fmt.Sprintf("$.dashboard.what_changed.groups[%d]", i)
		empty := strings.ToLower(strings.TrimSpace(g.EmptyText))
		if g.Key == "unchanged_risks" && len(g.Items) == 0 {
			if strings.Contains(empty, "нет долговремен") || strings.Contains(empty, "нет долгоживущ") ||
				strings.Contains(empty, "за пределами текущего") {
				report.addError(CodeDiagnosticsConclusionTooShallow, gPath+".empty_text",
					"empty previous context must not be interpreted as absence of active problems")
			}
		}
		for j, item := range g.Items {
			path := fmt.Sprintf("%s.items[%d]", gPath, j)
			checkText(path+".label", item.Label)
			checkText(path+".text", item.Text)
			checkText(path+".current_value_text", item.CurrentValueText)
			checkText(path+".previous_value_text", item.PreviousValueText)
			checkText(path+".impact_text", item.ImpactText)
			id := strings.TrimSpace(firstNonEmpty(item.ChangeID, item.SignalID))
			if id != "" {
				if prev, ok := seenChangeIDs[id]; ok && prev != g.Key {
					report.addError(CodeInvalidWhatChangedKeys, path+".change_id",
						"change_id/signal_id must not appear in multiple what_changed groups")
				}
				seenChangeIDs[id] = g.Key
			}
			body := item.Label + " " + item.Text
			if !containsDigit(body) && strings.TrimSpace(item.CurrentValueText) == "" {
				report.addWarning(CodeDiagnosticsConclusionTooShallow, path,
					"what_changed item should include at least one concrete numeric fact")
			}
		}
	}
}

func containsLongNumericID(text string) bool {
	digitRun := 0
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digitRun++
			if digitRun >= 9 {
				return true
			}
			continue
		}
		digitRun = 0
	}
	return false
}

func containsDigit(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func validateRussianPresentation(report *Report, dash *contract.UnifiedDashboardV1) {
	check := func(path, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if strings.Contains(text, "DRR") {
			report.addError(CodeNonRussianPresentationTerm, path,
				"use ДРР instead of DRR")
		}
		if strings.Contains(text, "ИИ-") || strings.Contains(text, "ИИ ") || strings.HasPrefix(text, "ИИ") {
			report.addError(CodeNonRussianPresentationTerm, path, "use AI instead of ИИ in user-facing product terms")
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "пульс селлера") || strings.Contains(lower, "seller pulse") {
			report.addError(CodeNonRussianPresentationTerm, path, "use Пульс продавца")
		}
	}
	if n := dash.SellerPulse.Narrative; n != nil {
		check("$.dashboard.seller_pulse.narrative.summary", n.Summary)
		check("$.dashboard.seller_pulse.narrative.headline", n.Headline)
		for i, f := range n.KeyPoints {
			check(fmt.Sprintf("$.dashboard.seller_pulse.narrative.key_points[%d]", i), f.Title+" "+f.Explanation)
		}
		for i, f := range n.ImportantFacts {
			check(fmt.Sprintf("$.dashboard.seller_pulse.narrative.important_facts[%d]", i), f.Title+" "+f.Explanation)
		}
	}
	check("$.dashboard.account_summary.executive_summary", dash.AccountSummary.ExecutiveSummary)
	check("$.dashboard.account_summary.improvement_summary", dash.AccountSummary.ImprovementSummary)
	check("$.dashboard.what_changed.main_conclusion", dash.WhatChanged.MainConclusion)
	for i, d := range dash.AccountSummary.RiskDrivers {
		check(fmt.Sprintf("$.dashboard.account_summary.risk_drivers[%d]", i), d.Title+" "+d.Reason)
	}
	for i, g := range dash.WhatChanged.Groups {
		for j, item := range g.Items {
			check(fmt.Sprintf("$.dashboard.what_changed.groups[%d].items[%d]", i, j), item.Label+" "+item.Text)
		}
	}
}

func validatePeriodSemantics(report *Report, dash *contract.UnifiedDashboardV1) {
	check := func(path, text string) {
		lower := strings.ToLower(text)
		// Crude day/week mix detector for common AI mistakes.
		if strings.Contains(lower, "за день") && strings.Contains(lower, "за 7 дней") && strings.Contains(lower, "дневн") {
			report.addError(CodeDashboardPeriodSemanticsInvalid, path, "mixed day/week period wording without clear scopes")
		}
	}
	if n := dash.SellerPulse.Narrative; n != nil {
		check("$.dashboard.seller_pulse.narrative.summary", n.Summary)
	}
	check("$.dashboard.what_changed.main_conclusion", dash.WhatChanged.MainConclusion)
}

func sentenceCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := []rune(text)
	n := 0
	for i, r := range runes {
		if r != '.' && r != '!' && r != '?' && r != '…' {
			continue
		}
		// Ignore decimal points such as 19.4%.
		if r == '.' && i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9' {
			continue
		}
		n++
	}
	if n == 0 && len(runes) > 40 {
		return 1
	}
	return n
}

func signalFingerprint(sigType, metric, direction, period string, entities []string) string {
	sigType = strings.ToLower(strings.TrimSpace(sigType))
	metric = strings.ToLower(strings.TrimSpace(metric))
	direction = strings.ToLower(strings.TrimSpace(direction))
	period = strings.ToLower(strings.TrimSpace(period))
	if sigType == "" && metric == "" {
		return ""
	}
	ents := append([]string{}, entities...)
	for i := range ents {
		ents[i] = strings.TrimSpace(ents[i])
	}
	sortStrings(ents)
	raw := strings.Join([]string{sigType, metric, direction, period, strings.Join(ents, ",")}, "|")
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func isPulseSection(section string) bool {
	s := strings.ToLower(strings.TrimSpace(section))
	return strings.Contains(s, "seller_pulse")
}

func isExclusiveSection(section string) bool {
	switch section {
	case "seller_pulse_important", "seller_pulse_key_point", "index_drivers",
		"what_changed_worsened", "what_changed_new_risks",
		"what_changed_active_problems", "what_changed_support_points":
		return true
	default:
		return false
	}
}

func bothDetailedCardSections(a, b string) bool {
	detailed := map[string]bool{
		"index_drivers":                true,
		"what_changed_worsened":        true,
		"what_changed_new_risks":       true,
		"what_changed_active_problems": true,
		"what_changed_support_points":  true,
	}
	return detailed[a] && detailed[b]
}

func sortStrings(in []string) {
	for i := 0; i < len(in); i++ {
		for j := i + 1; j < len(in); j++ {
			if in[j] < in[i] {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func keyPointFingerprint(title, text string) string {
	raw := strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(text))
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

func containsUnsafeMarkup(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "<script") || strings.Contains(lower, "<style") ||
		strings.Contains(lower, "<iframe") || strings.Contains(lower, "javascript:") {
		return true
	}
	// Disallow raw HTML tags while allowing plain text with < or > comparisons rarely used.
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<div") ||
		strings.Contains(lower, "<span") || strings.Contains(lower, "<a ") ||
		strings.Contains(lower, "<img") || strings.Contains(lower, "<table") {
		return true
	}
	return false
}
