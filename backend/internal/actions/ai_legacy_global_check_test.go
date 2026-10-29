package actions

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// sampleScoredDrafts builds a small set of scored drafts with source candidates
// carrying facts/aggregates so number validation has context to match against.
func sampleScoredDrafts() []ScoredDraftAction {
	return []ScoredDraftAction{
		{
			TempActionID: "stock_0",
			Domain:       ActionDomainStock,
			Score:        90,
			Priority:     ActionPriorityCritical,
			Action: DomainAIAction{
				Type:     ActionTypeStockReplenishment,
				Category: ActionCategoryStock,
				Title:    "Пополнить 16 SKU",
				Reason:   "Низкое покрытие.",
			},
			SourceCandidates: []ActionCandidate{{
				CandidateID: "stock_cand_0",
				Summary:     "16 SKU дают 94.7% выручки",
				Aggregates:  map[string]any{"revenue_share_pct": 94.7, "sku_count": 16},
			}},
		},
		{
			TempActionID: "sku_sales_1",
			Domain:       ActionDomainSKUSales,
			Score:        80,
			Priority:     ActionPriorityCritical,
			Action: DomainAIAction{
				Type:     ActionTypeKeySKURisk,
				Category: ActionCategorySKU,
				Title:    "Проверить 4 ключевых SKU",
				Reason:   "Сигналы риска.",
			},
			SourceCandidates: []ActionCandidate{{
				CandidateID: "key_sku_0",
				Summary:     "4 ключевых SKU дают 39.8% выручки",
				Aggregates:  map[string]any{"revenue_share_pct": 39.8},
			}},
		},
		{
			TempActionID: "advertising_0",
			Domain:       ActionDomainAdvertising,
			Score:        70,
			Priority:     ActionPriorityImportant,
			Action: DomainAIAction{
				Type:     ActionTypeAdHighDRR,
				Category: ActionCategoryAdvertising,
				Title:    "Ограничить 5 кампаний",
				Reason:   "Высокий ДРР.",
			},
			SourceCandidates: []ActionCandidate{{
				CandidateID: "ad_cand_0",
				Summary:     "5 кампаний со средним ДРР 364.7%",
				Aggregates:  map[string]any{"avg_drr_pct": 364.7, "campaigns": 5},
			}},
		},
	}
}

// Scenario A: valid ai_focus + temp_action_id top refs -> focus applied, top resolved.
func TestGlobalReview_ValidFocusAndTopRefs(t *testing.T) {
	drafts := sampleScoredDrafts()
	out := &LegacyGlobalCheckOutput{
		AIFocus: &ActionsAIFocus{
			Status: "available", Source: "ai",
			Headline: "Защитите выручку",
			Summary:  "16 SKU дают 94.7% выручки, 5 кампаний с ДРР 364.7%.",
		},
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "sku_sales_1"}, {TempActionID: "stock_0"}, {TempActionID: "advertising_0"}},
		TopActionRefs:     []string{"sku_sales_1", "stock_0", "advertising_0"},
	}
	opts := DefaultGenerateActionsOptions()
	gv := ValidateLegacyGlobalCheckOutput(out, draftsByTempID(drafts), opts, DefaultForbiddenClaims, AccountSnapshot{})
	if !gv.FocusValid || !gv.OrderingValid || !gv.TopValid {
		t.Fatalf("expected all parts valid, got ordering=%v top=%v focus=%v issues=%+v", gv.OrderingValid, gv.TopValid, gv.FocusValid, gv.Issues)
	}
	if status := legacyGlobalCheckCategoryStatus(gv); status != CategoryRunStatusCompleted {
		t.Fatalf("status=%q want completed", status)
	}
	_, top, focus := ApplyLegacyGlobalCheckOrdering(drafts, gv, opts)
	if focus.Source != "ai" {
		t.Fatalf("focus source=%q want ai", focus.Source)
	}
	if len(top) != 3 {
		t.Fatalf("top count=%d want 3", len(top))
	}
	if top[0].TempActionID != "sku_sales_1" {
		t.Fatalf("top[0]=%q want sku_sales_1", top[0].TempActionID)
	}
}

// Scenario B: valid ai_focus but unknown top refs -> focus kept, top deterministic, diagnostics record unknown refs.
func TestGlobalReview_ValidFocusUnknownTopRefs(t *testing.T) {
	drafts := sampleScoredDrafts()
	out := &LegacyGlobalCheckOutput{
		AIFocus: &ActionsAIFocus{
			Status: "available", Source: "ai",
			Headline: "Защитите выручку",
			Summary:  "Сводка без неподтверждённых чисел.",
		},
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "stock_0"}, {TempActionID: "sku_sales_1"}, {TempActionID: "advertising_0"}},
		TopActionRefs:     []string{"ghost_99", "phantom_1"},
	}
	opts := DefaultGenerateActionsOptions()
	gv := ValidateLegacyGlobalCheckOutput(out, draftsByTempID(drafts), opts, DefaultForbiddenClaims, AccountSnapshot{})
	if !gv.FocusValid {
		t.Fatalf("focus should be valid")
	}
	if gv.TopValid {
		t.Fatalf("top should be invalid")
	}
	if status := legacyGlobalCheckCategoryStatus(gv); status != CategoryRunStatusCompletedPartial {
		t.Fatalf("status=%q want completed_partial", status)
	}
	diag := buildLegacyGlobalCheckDiagnostic("global_review_validate", legacyGlobalCheckCategoryStatus(gv), gv)
	if len(diag.UnknownRefs) == 0 {
		t.Fatalf("expected unknown_refs in diagnostic")
	}
	raw, _ := json.Marshal(diag)
	if !strings.Contains(string(raw), "ghost_99") {
		t.Fatalf("diagnostic missing unknown ref: %s", raw)
	}
	_, top, focus := ApplyLegacyGlobalCheckOrdering(drafts, gv, opts)
	if focus.Source != "ai" {
		t.Fatalf("focus source=%q want ai", focus.Source)
	}
	if len(top) != 3 {
		t.Fatalf("deterministic top fill expected 3, got %d", len(top))
	}
}

// Scenario C: extra ai_focus.limitations field must not break parsing and is preserved.
func TestGlobalReview_ExtraLimitationsField(t *testing.T) {
	raw := `{
		"ai_focus": {
			"status": "available",
			"source": "ai",
			"headline": "Фокус",
			"summary": "Сводка",
			"top_categories": ["stock", "sku"],
			"limitations": ["Нельзя оценивать маржинальность.", "Причины не подтверждены."]
		},
		"ordered_action_refs": [{"temp_action_id": "stock_0"}],
		"top_action_refs": ["stock_0"],
		"unexpected_field": {"x": 1}
	}`
	var out LegacyGlobalCheckOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.AIFocus == nil {
		t.Fatalf("ai_focus not parsed")
	}
	if len(out.AIFocus.Limitations) != 2 {
		t.Fatalf("limitations=%v want 2", out.AIFocus.Limitations)
	}
	drafts := sampleScoredDrafts()
	gv := ValidateLegacyGlobalCheckOutput(&out, draftsByTempID(drafts), DefaultGenerateActionsOptions(), DefaultForbiddenClaims, AccountSnapshot{})
	if !gv.FocusValid {
		t.Fatalf("focus should remain valid with limitations field")
	}
	_, _, focus := ApplyLegacyGlobalCheckOrdering(drafts, gv, DefaultGenerateActionsOptions())
	if focus.Source != "ai" || len(focus.Limitations) != 2 {
		t.Fatalf("focus not applied with limitations: %+v", focus)
	}
}

// Scenario D: unsupported number in summary -> focus still applied, warning recorded, not full fallback.
func TestGlobalReview_UnsupportedNumberDoesNotVoidFocus(t *testing.T) {
	drafts := sampleScoredDrafts()
	out := &LegacyGlobalCheckOutput{
		AIFocus: &ActionsAIFocus{
			Status: "available", Source: "ai",
			Headline: "Фокус",
			Summary:  "Выручка снизилась на 0.11%, заказы на 9.91%, неизвестное число 123456789.",
		},
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "stock_0"}, {TempActionID: "sku_sales_1"}, {TempActionID: "advertising_0"}},
		TopActionRefs:     []string{"stock_0", "sku_sales_1", "advertising_0"},
	}
	snapshot := AccountSnapshot{RevenueDeltaPct: ptrFloat(-0.11), OrdersDeltaPct: ptrFloat(-9.91)}
	opts := DefaultGenerateActionsOptions()
	gv := ValidateLegacyGlobalCheckOutput(out, draftsByTempID(drafts), opts, DefaultForbiddenClaims, snapshot)
	if !gv.FocusValid {
		t.Fatalf("focus should stay valid despite unsupported number")
	}
	hasWarning := false
	for _, iss := range gv.FocusIssues {
		if iss.Code == "unsupported_ai_focus_number" && strings.Contains(iss.Message, "123456789") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected unsupported number warning, issues=%+v", gv.FocusIssues)
	}
	// account-level deltas must NOT be flagged because snapshot supports them.
	for _, iss := range gv.FocusIssues {
		if strings.Contains(iss.Message, "0.11") || strings.Contains(iss.Message, "9.91") {
			t.Fatalf("snapshot-supported number wrongly flagged: %+v", iss)
		}
	}
	_, _, focus := ApplyLegacyGlobalCheckOrdering(drafts, gv, opts)
	if focus.Source != "ai" {
		t.Fatalf("focus source=%q want ai", focus.Source)
	}
}

// Scenario D2: forbidden claim in ai_focus must void the focus (safety guardrail kept).
func TestGlobalReview_ForbiddenClaimVoidsFocus(t *testing.T) {
	drafts := sampleScoredDrafts()
	out := &LegacyGlobalCheckOutput{
		AIFocus: &ActionsAIFocus{
			Status: "available", Source: "ai",
			Headline: "Снизить цену срочно",
			Summary:  "Сводка",
		},
		OrderedActionRefs: []LegacyGlobalCheckActionRef{{TempActionID: "stock_0"}},
		TopActionRefs:     []string{"stock_0"},
	}
	opts := DefaultGenerateActionsOptions()
	gv := ValidateLegacyGlobalCheckOutput(out, draftsByTempID(drafts), opts, DefaultForbiddenClaims, AccountSnapshot{})
	if gv.FocusValid {
		t.Fatalf("focus with forbidden claim must be invalid")
	}
	// ordering/top still valid -> partial, not full fallback
	if status := legacyGlobalCheckCategoryStatus(gv); status != CategoryRunStatusCompletedPartial {
		t.Fatalf("status=%q want completed_partial", status)
	}
	_, _, focus := ApplyLegacyGlobalCheckOrdering(drafts, gv, opts)
	if focus.Source != "deterministic" {
		t.Fatalf("focus source=%q want deterministic", focus.Source)
	}
}

// Scenario E + acceptance: end-to-end, valid AI focus persists with source=ai and model filled.
func TestGenerateActions_GlobalReviewAppliesAIFocusAndModel(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn: func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
			refs := make([]LegacyGlobalCheckActionRef, 0, len(input.DraftActions))
			top := make([]string, 0)
			for i, d := range input.DraftActions {
				refs = append(refs, LegacyGlobalCheckActionRef{TempActionID: d.TempActionID})
				if i < input.MaxTopActions {
					top = append(top, d.TempActionID)
				}
			}
			return &LegacyGlobalCheckOutput{
				AIFocus:           &ActionsAIFocus{Status: "available", Source: "ai", Headline: "AI фокус дня", Summary: "Краткая сводка."},
				OrderedActionRefs: refs,
				TopActionRefs:     top,
			}, &AIRawResponse{Model: "gpt-5.4-2026-03-05"}, nil
		},
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	result, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	if result.AIFocus == nil || result.AIFocus.Source != "ai" {
		t.Fatalf("expected ai focus, got %+v", result.AIFocus)
	}
	run := repo.runs[1]
	if run == nil {
		t.Fatalf("missing run")
	}
	focus := parseAIFocusJSON(run.AIFocusJSON)
	if focus == nil || focus.Source != "ai" {
		t.Fatalf("persisted ai_focus source != ai: %+v", focus)
	}
	cr := repo.categoryRunByDomain(DomainLegacyGlobalCheck)
	if cr == nil {
		t.Fatalf("missing global_review category run")
	}
	if cr.Model != "gpt-5.4-2026-03-05" {
		t.Fatalf("global_review model=%q want gpt-5.4-2026-03-05", cr.Model)
	}
	if cr.Status != CategoryRunStatusCompleted && cr.Status != CategoryRunStatusCompletedRepaired && cr.Status != CategoryRunStatusCompletedPartial {
		t.Fatalf("global_review status=%q", cr.Status)
	}
	if len(cr.InputContextJSON) == 0 {
		t.Fatalf("expected input_context_json for global_review")
	}
	if len(cr.ValidationJSON) == 0 || len(cr.FinalValidationJSON) == 0 {
		t.Fatalf("expected validation/final_validation json for global_review")
	}
	// domain runs should also carry a model now.
	stockRun := repo.categoryRunByDomain(ActionDomainStock)
	if stockRun != nil && stockRun.Status == CategoryRunStatusCompleted && stockRun.Model == "" {
		t.Fatalf("expected domain model to be filled")
	}
}

// Scenario F: legacy global check error -> fallback status with non-empty diagnostics.
func TestGenerateActions_GlobalReviewErrorStoresDiagnostics(t *testing.T) {
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{
		ComposeFn: defaultValidComposeFn(),
		ReviewFn: func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
			return nil, &AIRawResponse{Model: "gpt-5.4-2026-03-05"}, context.DeadlineExceeded
		},
	}
	svc := NewGenerationService(builder, fakeAI, repo)
	if _, err := svc.GenerateActions(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions()); err != nil {
		t.Fatalf("GenerateActions: %v", err)
	}
	cr := repo.categoryRunByDomain(DomainLegacyGlobalCheck)
	if cr == nil {
		t.Fatalf("missing global_review category run")
	}
	if cr.Status != CategoryRunStatusFallback {
		t.Fatalf("status=%q want fallback", cr.Status)
	}
	if len(cr.ValidationJSON) == 0 || len(cr.FinalValidationJSON) == 0 {
		t.Fatalf("fallback must store validation/final_validation json")
	}
	if !strings.Contains(string(cr.ValidationJSON), "legacy_global_check_error") {
		t.Fatalf("validation_json missing reason: %s", cr.ValidationJSON)
	}
}
