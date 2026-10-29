package actions

import (
	"context"
	"strings"
	"testing"
)

func buildTestAdvertisingDomain(t *testing.T) ActionDomainContext {
	t.Helper()
	repo := newFakeCandidateRepo()
	repo.adCampaignCount = 6
	drr := 364.7
	repo.adCampaigns = []AdCampaignMetricRow{
		{CampaignExternalID: "c1", CampaignName: "High DRR", Spend7d: 1074747, Revenue7d: 418094, Orders7d: 12, DRRPct: &drr, HasMetricData: true},
		{CampaignExternalID: "c2", CampaignName: "Zero orders", Spend7d: 21568, Revenue7d: 0, Orders7d: 0, HasMetricData: true},
	}
	cfg := DefaultCandidateBuilderConfig()
	domain, _, _ := buildAdvertisingDomain(context.Background(), repo, 1, testMetricDate(), cfg)
	if domain.Status != DomainStatusAvailable {
		t.Fatalf("advertising domain status=%q want available", domain.Status)
	}
	if len(domain.Candidates) != 2 {
		t.Fatalf("advertising candidates=%d want 2", len(domain.Candidates))
	}
	return domain
}

// G1: fallback for advertising with both candidates produces two valid actions,
// ad_high_drr is not dropped, dropped_count == 0.
func TestAdvertisingFallback_BothActionsSurvive(t *testing.T) {
	domain := buildTestAdvertisingDomain(t)
	fallback := finalizeAcceptedActions(domain, BuildFallbackActions(domain))
	if len(fallback) != 2 {
		t.Fatalf("fallback actions=%d want 2", len(fallback))
	}
	rules := validationRulesForDomain(ActionDomainAdvertising, DefaultForbiddenClaims)
	result := ValidateDomainComposerOutput(domain, DomainComposerOutput{Domain: ActionDomainAdvertising, Actions: fallback}, rules)
	if len(result.AcceptedActions) != 2 {
		t.Fatalf("accepted=%d dropped=%+v issues=%+v", len(result.AcceptedActions), result.DroppedActions, result.Issues)
	}
	if len(result.DroppedActions) != 0 {
		t.Fatalf("dropped=%d want 0: %+v", len(result.DroppedActions), result.DroppedActions)
	}
	hasHighDRR := false
	for _, a := range result.AcceptedActions {
		if a.Type == ActionTypeAdHighDRR {
			hasHighDRR = true
			if strings.Contains(a.Title, "0%") || strings.Contains(a.Title, " 0 ") {
				t.Fatalf("ad_high_drr title still generic/zero: %q", a.Title)
			}
		}
	}
	if !hasHighDRR {
		t.Fatalf("ad_high_drr missing from accepted fallback actions")
	}
}

// G2: when AI produces invalid advertising actions (0 accepted), processDomain
// recovers via fallback to both candidates and is not a generic fallback dead-end.
func TestProcessDomain_AdvertisingFallbackRecoversBoth(t *testing.T) {
	domain := buildTestAdvertisingDomain(t)
	candidateCtx := &ActionCandidateContext{
		MetricDate:      "2026-06-25",
		AccountSnapshot: AccountSnapshot{},
		Domains:         []ActionDomainContext{domain},
		ForbiddenClaims: DefaultForbiddenClaims,
	}
	fakeAI := &FakeActionAIClient{
		ComposeFn: func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
			// Return an action with no source candidates -> fatal, 0 accepted.
			return &DomainComposerOutput{Domain: input.Domain, Actions: []DomainAIAction{{
				Type: ActionTypeAdHighDRR, Category: ActionCategoryAdvertising, Title: "bad",
			}}}, &AIRawResponse{Model: "gpt-test"}, nil
		},
		RepairFn: func(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error) {
			return &DomainComposerOutput{Domain: input.DomainContext.Domain, Actions: nil}, &AIRawResponse{Model: "gpt-test"}, nil
		},
	}
	svc := NewGenerationService(buildTestCandidateBuilder(t), fakeAI, newFakeGenerationRepo())
	dr := svc.processDomain(context.Background(), candidateCtx, domain, DefaultGenerateActionsOptions())
	if dr.Status == CategoryRunStatusFallback {
		t.Fatalf("advertising domain should not be generic fallback, got %q", dr.Status)
	}
	if dr.Status != CategoryRunStatusCompletedPartial {
		t.Fatalf("status=%q want completed_partial", dr.Status)
	}
	if len(dr.AcceptedActions) != 2 {
		t.Fatalf("accepted=%d want 2 (both advertising actions recovered)", len(dr.AcceptedActions))
	}
	if dr.DroppedCount != 0 {
		t.Fatalf("dropped=%d want 0", dr.DroppedCount)
	}
}

// E + F: pluralization and CTA consistency on the ad_spend_no_orders fallback action.
func TestAdvertisingFallback_PluralizationAndCTAs(t *testing.T) {
	repo := newFakeCandidateRepo()
	repo.adCampaignCount = 6
	repo.adCampaigns = []AdCampaignMetricRow{
		{CampaignExternalID: "c2", CampaignName: "Zero orders", Spend7d: 21568, Revenue7d: 0, Orders7d: 0, HasMetricData: true},
	}
	domain, _, _ := buildAdvertisingDomain(context.Background(), repo, 1, testMetricDate(), DefaultCandidateBuilderConfig())
	fallback := finalizeAcceptedActions(domain, BuildFallbackActions(domain))
	var noOrders *DomainAIAction
	for i := range fallback {
		if fallback[i].Type == ActionTypeAdSpendNoOrders {
			noOrders = &fallback[i]
		}
	}
	if noOrders == nil {
		t.Fatalf("ad_spend_no_orders fallback not built")
	}
	if strings.Contains(noOrders.Title, "1 кампаний") {
		t.Fatalf("grammar regression in title: %q", noOrders.Title)
	}
	if !strings.Contains(noOrders.Title, "1 кампанию") {
		t.Fatalf("expected accusative '1 кампанию', got %q", noOrders.Title)
	}
	if strings.Contains(noOrders.Reason, "1 кампаний") {
		t.Fatalf("grammar regression in reason: %q", noOrders.Reason)
	}

	keys := canonicalCTAKeys(noOrders.CTAKeys, domain.Candidates)
	if !containsKey(keys, ActionCTAOpenCampaignList) {
		t.Fatalf("missing navigate CTA open_campaign_list: %v", keys)
	}
	if !containsKey(keys, ActionCTAAskAI) {
		t.Fatalf("missing ask_ai CTA: %v", keys)
	}
	if !containsKey(keys, ActionCTAExportNoOrderCampaigns) {
		t.Fatalf("missing export CTA: %v", keys)
	}
	// navigate must come before export, export before ask_ai, ask_ai before mark_checked.
	if idx(keys, ActionCTAOpenCampaignList) > idx(keys, ActionCTAExportNoOrderCampaigns) {
		t.Fatalf("navigate must precede export: %v", keys)
	}
	if idx(keys, ActionCTAAskAI) > idx(keys, ActionCTAMarkChecked) {
		t.Fatalf("ask_ai must precede mark_checked: %v", keys)
	}
}

func TestCanonicalCTAKeys_AskAIAlwaysPresentAndOrdered(t *testing.T) {
	cands := []ActionCandidate{{
		AllowedCTAs: []string{ActionCTAMarkChecked, ActionCTAExportSKUOrdersDrop, ActionCTAOpenSKUList, ActionCTAAskAI},
	}}
	// Action only selected an export CTA; canonical set should still include the rest.
	keys := canonicalCTAKeys([]string{ActionCTAExportSKUOrdersDrop}, cands)
	want := []string{ActionCTAOpenSKUList, ActionCTAExportSKUOrdersDrop, ActionCTAAskAI, ActionCTAMarkChecked}
	if len(keys) != len(want) {
		t.Fatalf("keys=%v want=%v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("order mismatch at %d: keys=%v want=%v", i, keys, want)
		}
	}
}

func TestRuPlural_Campaigns(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "кампанию"}, {2, "кампании"}, {4, "кампании"}, {5, "кампаний"},
		{11, "кампаний"}, {12, "кампаний"}, {21, "кампанию"}, {22, "кампании"}, {25, "кампаний"},
	}
	for _, c := range cases {
		if got := campaignsAccusative(c.n); got != c.want {
			t.Fatalf("campaignsAccusative(%d)=%q want %q", c.n, got, c.want)
		}
	}
}

func TestSanitizeAIFocusNumbers_RemovesUnsupportedKeepsSupported(t *testing.T) {
	allowed := map[string]bool{"9.91": true, "0.11": true}
	focus := &ActionsAIFocus{
		Source:   "ai",
		Headline: "Защитите выручку",
		Summary:  "На фоне снижения выручки на 11% и заказов на 9.91% проверьте ключевые SKU.",
	}
	removed := SanitizeAIFocusNumbers(focus, allowed)
	if !containsKey(removed, "11") {
		t.Fatalf("expected 11 to be removed, got %v", removed)
	}
	if strings.Contains(focus.Summary, "11%") {
		t.Fatalf("unsupported 11%% still present: %q", focus.Summary)
	}
	if !strings.Contains(focus.Summary, "9.91%") {
		t.Fatalf("supported 9.91%% wrongly removed: %q", focus.Summary)
	}
}

func TestGenerateActions_GlobalFocusSanitizedBeforePersist(t *testing.T) {
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
				AIFocus: &ActionsAIFocus{
					Status: "available", Source: "ai",
					Headline: "Фокус дня",
					Summary:  "На фоне снижения выручки на 11% приоритетно проверьте ключевые SKU.",
				},
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
	focus := parseAIFocusJSON(run.AIFocusJSON)
	if focus == nil {
		t.Fatalf("no persisted focus")
	}
	if focus.Source != "ai" {
		t.Fatalf("persisted focus source=%q want ai", focus.Source)
	}
	if strings.Contains(focus.Summary, "11%") {
		t.Fatalf("persisted focus still contains unsupported 11%%: %q", focus.Summary)
	}
	cr := repo.categoryRunByDomain(DomainLegacyGlobalCheck)
	if cr == nil || len(cr.FinalValidationJSON) == 0 {
		t.Fatalf("missing global_review final validation json")
	}
	if !strings.Contains(string(cr.FinalValidationJSON), "sanitized") {
		t.Fatalf("final_validation_json should indicate sanitization: %s", cr.FinalValidationJSON)
	}
}

func containsKey(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

func idx(keys []string, k string) int {
	for i, x := range keys {
		if x == k {
			return i
		}
	}
	return -1
}
