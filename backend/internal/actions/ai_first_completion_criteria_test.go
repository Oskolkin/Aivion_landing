package actions

import (
	"strings"
	"testing"
)

func TestValidateCompletionConditions_NumericVerifiable(t *testing.T) {
	input := buildRun69LikeInput(t)
	target := 15.0
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.CompletionCriteria = "ДРР каждой кампании ≤ 15% за 7 дней"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "drr_pct", Operator: "lte", TargetValue: &target,
		Scope: "per_campaign", PeriodDays: 7, DisplayText: "ДРР ≤ 15%",
	}}
	issues := validateCompletionConditionsStructure(input, action, 0)
	for _, issue := range issues {
		if issue.Severity == IssueSeverityFatal {
			t.Fatalf("unexpected fatal: %+v", issue)
		}
	}
}

func TestValidateCompletionConditions_MissingTargetValueFatal(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.CompletionCriteria = "ДРР снижен"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "drr_pct", Operator: "lte", Scope: "per_campaign",
	}}
	issues := validateCompletionConditionsStructure(input, action, 0)
	if !hasIssueCode(issues, "invalid_completion_condition_dropped") {
		// structural invalidity is handled in normalizer; validator no longer fatals
		normalized, normIssues, _ := normalizeActionCompletionConditionsWithStats(input, action.CompletionConditions, 0)
		if len(normalized) != 0 {
			t.Fatalf("expected condition drop, normalized=%+v issues=%v", normalized, normIssues)
		}
	}
}

func TestValidateCompletionConditions_VaguePhraseWarning(t *testing.T) {
	input := buildRun69LikeInput(t)
	target := 10.0
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.CompletionCriteria = "ДРР снижен до приемлемого уровня"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "drr_pct", Operator: "lte", TargetValue: &target, Scope: "per_campaign", PeriodDays: 7,
	}}
	issues := validateCompletionConditionsStructure(input, action, 0)
	if !hasIssueCode(issues, "vague_completion_criteria_phrase") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestValidateCompletionConditions_VagueOnlyLegacyWarning(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := ValidMinimalPilotDraftFromCandidateForTests(testStockCandidate("repl:crit"))
	action.CompletionCriteria = "Кампания оптимизирована до оптимального значения"
	issues := validateCompletionConditionsStructure(input, action, 0)
	if !hasIssueCode(issues, "vague_completion_criteria_phrase") {
		t.Fatalf("issues=%v", issues)
	}
	if !hasIssueCode(issues, "legacy_completion_criteria_only") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestValidateCompletionConditions_DecisionRecordedWithoutNumericTarget(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.CompletionCriteria = "По каждой кампании зафиксировано решение о целевом пределе ДРР"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "drr_pct", Operator: "decision_recorded", Scope: "per_campaign",
		DisplayText: "Целевой предел ДРР зафиксирован пользователем",
	}}
	issues := validateCompletionConditionsStructure(input, action, 0)
	for _, issue := range issues {
		if issue.Severity == IssueSeverityFatal {
			t.Fatalf("unexpected fatal: %+v", issue)
		}
	}
}

func TestValidateCompletionConditions_UnknownFactRemovedWithWarning(t *testing.T) {
	input := buildRun69LikeInput(t)
	target := 15.0
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.OperationalGoal = "replenish_stock"
	action.CompletionCriteria = "DRR <= 15"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "drr_pct", Operator: "lte", TargetValue: &target, Scope: "per_campaign",
		FactID: "missing.fact.id",
	}}
	normalized, issues, stats := normalizeActionCompletionConditionsWithStats(input, action.CompletionConditions, 0)
	if len(normalized) != 1 || normalized[0].FactID != "" {
		t.Fatalf("normalized=%+v", normalized)
	}
	if !hasIssueCode(issues, "unknown_completion_fact_id_removed") {
		t.Fatalf("issues=%v", issues)
	}
	if stats.FactIDRemovedCount != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	pilotAction := action
	pilotAction.CompletionConditions = normalized
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{pilotAction}})
	pilot := ValidateActionsAIFirstOutputPilot(input, out)
	if pilot.AcceptedCount != 1 {
		t.Fatalf("accepted=%d issues=%v dropped=%v", pilot.AcceptedCount, pilot.Issues, pilot.Dropped)
	}
}

func TestValidateCompletionConditions_UnknownFactRemovedDecisionRecorded(t *testing.T) {
	input := buildRun69LikeInput(t)
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.OperationalGoal = "replenish_stock"
	action.CompletionCriteria = "Целевой предел зафиксирован"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "target_drr_configured", Operator: "decision_recorded", Scope: "per_campaign",
		FactID: "invented.fact.id",
	}}
	normalized, issues, _ := normalizeActionCompletionConditionsWithStats(input, action.CompletionConditions, 0)
	if len(normalized) != 1 || normalized[0].FactID != "" {
		t.Fatalf("normalized=%+v", normalized)
	}
	if !hasIssueCode(issues, "unknown_completion_fact_id_removed") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestPilotValidator_CompletionConditionsQualityWarningDoesNotDrop(t *testing.T) {
	input := buildRun69LikeInput(t)
	target := 15.0
	action := detailedNormalizedAction(input, "repl:crit", "sku_1")
	action.TitleTemplate = "Пополнить {object_count} SKU"
	action.OperationalGoal = "replenish_stock"
	action.CompletionCriteria = "Покрытие улучшено до приемлемого уровня"
	action.CompletionConditions = []AIFirstCompletionCondition{{
		MetricCode: "idc_days", Operator: "gte", TargetValue: &target, Scope: "per_sku", PeriodDays: 7,
	}}
	out, _ := NormalizeActionsAIFirstV2Output(input, ActionsAIFirstOutput{Actions: []AIFirstDraftAction{action}})
	pilot := ValidateActionsAIFirstOutputPilot(input, out)
	if pilot.AcceptedCount != 1 {
		t.Fatalf("accepted=%d issues=%v", pilot.AcceptedCount, pilot.Issues)
	}
	if !normalizeTestPilotHasIssue(pilot, "vague_completion_criteria_phrase") {
		t.Fatalf("issues=%v", pilot.Issues)
	}
}

func TestMapCompletionConditionsToAPI_PreservesAIValues(t *testing.T) {
	target := 12.0
	mapped := mapCompletionConditionsToAPI([]AIFirstCompletionCondition{{
		MetricCode: "stock_available", Operator: "gte", TargetValue: &target,
		Scope: "per_sku", PeriodDays: 7, DisplayText: "Остаток ≥ 12 шт.",
	}})
	if len(mapped) != 1 || mapped[0].MetricCode != "stock_available" || mapped[0].TargetValue == nil || *mapped[0].TargetValue != 12 {
		t.Fatalf("mapped=%+v", mapped)
	}
	summary := formatCompletionConditionSummary(mapped[0])
	if !strings.Contains(summary, "12") {
		t.Fatalf("summary=%q", summary)
	}
}
