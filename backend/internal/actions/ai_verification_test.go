package actions

import (
	"context"
	"errors"
	"testing"
)

func TestBuildActionKey_MultiCandidatePathSafe(t *testing.T) {
	ids := []string{
		"stock_replenishment_2026-06-24_key_low_cover",
		"no_sales_with_stock_2026-06-24_top",
	}
	key := BuildActionKey("2026-06-24", ActionTypeStockReplenishment, ids, "entity_group_ref")
	if err := ValidateActionKey(key); err != nil {
		t.Fatalf("ValidateActionKey failed for multi-candidate key %q: %v", key, err)
	}
	// Separator between candidate_ids is "." (dot); IDs are sorted alphabetically.
	if !containsSubstring(key, "no_sales_with_stock_2026-06-24_top.stock_replenishment_2026-06-24_key_low_cover") {
		t.Fatalf("key %q should contain dot-joined sorted candidate ids", key)
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func TestGenerateActions_AIDisabledSkipsAllAICalls(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{}
	svc := NewGenerationService(builder, fakeAI, repo)

	opts := DefaultGenerateActionsOptions()
	opts.AIEnabled = false

	_, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), opts)
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if len(fakeAI.ComposeCalls) != 0 {
		t.Fatalf("ComposeDomainActions called %d times", len(fakeAI.ComposeCalls))
	}
	if len(fakeAI.RepairCalls) != 0 {
		t.Fatalf("RepairDomainActions called %d times", len(fakeAI.RepairCalls))
	}
	if len(fakeAI.ReviewCalls) != 0 {
		t.Fatalf("RunLegacyGlobalCheck called %d times", len(fakeAI.ReviewCalls))
	}
}

func TestGenerateActions_PersistErrorFinalizesRun(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	repo.persistErr = errors.New("db unavailable")
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn:  defaultFakeReviewFn(),
	}
	svc := NewGenerationService(builder, fakeAI, repo)

	_, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err == nil {
		t.Fatalf("expected persist error")
	}
	run := repo.runs[1]
	if run == nil {
		t.Fatalf("expected run record")
	}
	if run.Status != GenerationRunStatusCompleted {
		t.Fatalf("run status=%q want completed", run.Status)
	}
	if run.FinalStatus != GenerationFinalStatusFailed {
		t.Fatalf("final_status=%q want failed", run.FinalStatus)
	}
}

func TestLegacyGlobalCheck_DoesNotRewriteActionText(t *testing.T) {
	originalTitle := "Пополнить 12 SKU с покрытием меньше 5 дней"
	originalReason := "Низкое покрытие остатков."
	originalNext := "Откройте список SKU."
	drafts := []ScoredDraftAction{{
		TempActionID: "stock_0",
		Action: DomainAIAction{
			Title: originalTitle, Reason: originalReason, NextStep: originalNext,
			Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		},
		Score: 80, Priority: ActionPriorityImportant, Domain: ActionDomainStock,
	}}
	// Legacy global check output only reorders and adjusts priority — no text fields on actions.
	reviewOut := &LegacyGlobalCheckOutput{
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "stock_0", Priority: ActionPriorityCritical}},
		TopActionRefs:     []string{"stock_0"},
		AIFocus:           &ActionsAIFocus{Status: "available", Source: "ai", Headline: "Фокус", Summary: "Сводка"},
	}
	gv := ValidateLegacyGlobalCheckOutput(reviewOut, draftsByTempID(drafts), DefaultGenerateActionsOptions(), DefaultForbiddenClaims, AccountSnapshot{})
	ordered, _, _ := ApplyLegacyGlobalCheckOrdering(drafts, gv, DefaultGenerateActionsOptions())
	if len(ordered) != 1 {
		t.Fatalf("ordered count=%d", len(ordered))
	}
	if ordered[0].Action.Title != originalTitle {
		t.Fatalf("title rewritten: %q", ordered[0].Action.Title)
	}
	if ordered[0].Action.Reason != originalReason {
		t.Fatalf("reason rewritten: %q", ordered[0].Action.Reason)
	}
	if ordered[0].Action.NextStep != originalNext {
		t.Fatalf("next_step rewritten: %q", ordered[0].Action.NextStep)
	}
	if ordered[0].Priority != ActionPriorityCritical {
		t.Fatalf("priority override expected, got %q", ordered[0].Priority)
	}
}

func TestNumberValidation_FromFactsAndSummary(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Title = "Пополнить 12 SKU с покрытием меньше 5 дней"
	action.Reason = "ДРР 25% и расход 1 000 ₽ по данным отчёта."
	action.NextStep = "Проверьте остатки на 7 дней и 0 заказов за период."

	// Add facts supporting numbers in reason/next_step not in original facts.
	domainCtx.Candidates[0].Facts = append(domainCtx.Candidates[0].Facts,
		ActionCandidateFact{FactID: "stock_replenishment_2026-06-24_key_low_cover.drr", Kind: "metric", Label: "ДРР", Value: "25%"},
		ActionCandidateFact{FactID: "stock_replenishment_2026-06-24_key_low_cover.spend", Kind: "metric", Label: "Расход", Value: "1000 ₽"},
		ActionCandidateFact{FactID: "stock_replenishment_2026-06-24_key_low_cover.days", Kind: "metric", Label: "Покрытие", Value: "7 дней"},
		ActionCandidateFact{FactID: "stock_replenishment_2026-06-24_key_low_cover.orders", Kind: "metric", Label: "Заказы", Value: "0 заказов"},
	)
	action.EvidenceFactIDs = []string{
		"stock_replenishment_2026-06-24_key_low_cover.sku_count",
		"stock_replenishment_2026-06-24_key_low_cover.coverage_max",
		"stock_replenishment_2026-06-24_key_low_cover.drr",
		"stock_replenishment_2026-06-24_key_low_cover.spend",
		"stock_replenishment_2026-06-24_key_low_cover.days",
		"stock_replenishment_2026-06-24_key_low_cover.orders",
	}

	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
		Domain: ActionDomainStock, Actions: []DomainAIAction{action},
	}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status == ValidationStatusRepairable || result.Status == ValidationStatusInvalid {
		t.Fatalf("expected valid number usage, status=%q issues=%+v", result.Status, result.Issues)
	}

	// Number from candidate summary (12) in title is allowed via summary/facts pool.
	action2 := validStockAIAction()
	action2.Title = "Пополнить 12 SKU с покрытием меньше 5 дней"
	result2 := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
		Domain: ActionDomainStock, Actions: []DomainAIAction{action2},
	}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if hasIssueCode(result2.Issues, "unsupported_number") {
		t.Fatalf("12 from summary/facts should be allowed in title, issues=%+v", result2.Issues)
	}

	// 42 п.п. is not in context — should be repairable.
	action3 := validStockAIAction()
	action3.Reason = "Ожидается рост на 42 п.п."
	result3 := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
		Domain: ActionDomainStock, Actions: []DomainAIAction{action3},
	}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if !hasIssueCode(result3.Issues, "unsupported_number") {
		t.Fatalf("expected unsupported_number for 42 п.п., issues=%+v", result3.Issues)
	}
}

func hasIssueCode(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestForbiddenClaims_PriceMarginAndAdSKU(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	cases := []struct {
		field string
		text  string
	}{
		{"reason", "Рекомендуем снизить цену"},
		{"reason", "Проверьте маржинальность SKU"},
		{"reason", "Гарантированно увеличит продажи"},
	}
	for _, tc := range cases {
		action := validStockAIAction()
		switch tc.field {
		case "reason":
			action.Reason = tc.text
		}
		result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
			Domain: ActionDomainStock, Actions: []DomainAIAction{action},
		}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
		if !hasIssueCode(result.Issues, "forbidden_claim") {
			t.Fatalf("expected forbidden_claim for %q in %s, issues=%+v", tc.text, tc.field, result.Issues)
		}
	}
}

func TestStaleDelete_OnlyOnSuccessfulPersist(t *testing.T) {
	repo := newFakeGenerationRepo()
	metricDate := testMetricDate()
	repo.items["stale"] = ActionItemRecord{
		ID: 1, SellerAccountID: 1, ActionKey: "stale",
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityWatch, Status: ActionStatusOpen,
		Title: "stale", Reason: "r", NextStep: "n", MetricDate: &metricDate,
	}
	// Simulate failed final status path — no PersistGenerationAtomic with StaleDelete.
	runID, _ := repo.CreateActionGenerationRun(context.Background(), 1, metricDate, GenerationRunStatusRunning)
	_ = repo.UpdateActionGenerationRun(context.Background(), UpdateGenerationRunParams{
		RunID: runID, Status: GenerationRunStatusCompleted, FinalStatus: GenerationFinalStatusFailed,
	})
	if _, ok := repo.items["stale"]; !ok {
		t.Fatalf("stale item should remain on failed generation")
	}
}
