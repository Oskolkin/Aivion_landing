package actions

import (
	"testing"
)

func sampleStockDomainContext() ActionDomainContext {
	cand := ActionCandidate{
		CandidateID:  "stock_replenishment_2026-06-24_key_low_cover",
		Domain:       ActionDomainStock,
		Type:         ActionTypeStockReplenishment,
		Category:     ActionCategoryStock,
		PriorityHint: ActionPriorityImportant,
		Summary:      "12 SKU с покрытием меньше 7 дней",
		Aggregates: map[string]any{
			"sku_count":         12,
			"coverage_max_days": 5.0,
		},
		Facts: []ActionCandidateFact{
			{FactID: "stock_replenishment_2026-06-24_key_low_cover.sku_count", Kind: "count", Label: "SKU", Value: "12"},
			{FactID: "stock_replenishment_2026-06-24_key_low_cover.coverage_max", Kind: "metric", Label: "Покрытие", Value: "5", Unit: "дней"},
		},
		AllowedCTAs: []string{ActionCTAOpenSKUList, ActionCTAExportStockReplenishment, ActionCTAMarkChecked},
		Limitations: []string{stockReplenishmentLimitation},
		EntityGroupRef: &ActionEntityGroupRef{
			GroupKey:   "stock_replenishment_2026-06-24_key_low_cover",
			EntityType: "sku",
			Count:      12,
		},
	}
	return ActionDomainContext{
		Domain:     ActionDomainStock,
		Status:     DomainStatusAvailable,
		Candidates: []ActionCandidate{cand},
	}
}

func validStockAIAction() DomainAIAction {
	return DomainAIAction{
		SourceCandidateIDs: []string{"stock_replenishment_2026-06-24_key_low_cover"},
		Type:               ActionTypeStockReplenishment,
		Category:           ActionCategoryStock,
		Priority:           ActionPriorityImportant,
		Title:              "Пополнить 12 SKU с покрытием меньше 5 дней",
		Summary:            "12 SKU с покрытием меньше 7 дней",
		Reason:             "Низкое покрытие остатков по 12 SKU.",
		NextStep:           "Откройте список SKU и проверьте остатки перед поставкой.",
		EvidenceFactIDs:    []string{"stock_replenishment_2026-06-24_key_low_cover.sku_count", "stock_replenishment_2026-06-24_key_low_cover.coverage_max"},
		CTAKeys:            []string{ActionCTAExportStockReplenishment, ActionCTAMarkChecked},
		Limitations:        []string{stockReplenishmentLimitation},
	}
}

func TestValidateDomainComposerOutput_ValidAccepted(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{validStockAIAction()}}
	rules := validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims)
	result := ValidateDomainComposerOutput(domainCtx, output, rules)
	if result.Status != ValidationStatusValid {
		t.Fatalf("status=%q issues=%+v", result.Status, result.Issues)
	}
	if len(result.AcceptedActions) != 1 {
		t.Fatalf("accepted=%d want 1", len(result.AcceptedActions))
	}
}

func TestValidateDomainComposerOutput_UnknownCandidateRejected(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.SourceCandidateIDs = []string{"unknown_candidate"}
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatalf("expected rejection")
	}
	if result.Status != ValidationStatusInvalid {
		t.Fatalf("status=%q want invalid", result.Status)
	}
}

func TestValidateDomainComposerOutput_UnknownFactRejected(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.EvidenceFactIDs = append(action.EvidenceFactIDs, "unknown.fact")
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatalf("expected rejection for unknown fact")
	}
}

func TestValidateDomainComposerOutput_UnknownCTARepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.CTAKeys = []string{"unknown_cta"}
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_UnknownActionTypeRejected(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Type = ActionType("bogus_type")
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatalf("expected rejection")
	}
}

func TestValidateDomainComposerOutput_GenericTitleRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Title = "Проверьте остатки"
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_UnsupportedNumberRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Title = "Пополнить 999 SKU с покрытием меньше 5 дней"
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable, issues=%+v", result.Status, result.Issues)
	}
}

func TestValidateDomainComposerOutput_ForbiddenClaimRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Reason = "Нужно снизить цену на SKU"
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_AdvertisingSKULevelRejected(t *testing.T) {
	domainCtx := ActionDomainContext{
		Domain: ActionDomainAdvertising,
		Status: DomainStatusAvailable,
		Candidates: []ActionCandidate{{
			CandidateID: "ad_high_drr_2026-06-24_top",
			Domain:      ActionDomainAdvertising,
			Type:        ActionTypeAdHighDRR,
			Category:    ActionCategoryAdvertising,
			Summary:     "2 кампании с высоким ДРР",
			Aggregates:  map[string]any{"campaign_count": 2, "drr_threshold_pct": 25.0},
			Facts:       []ActionCandidateFact{{FactID: "ad_high_drr_2026-06-24_top.count", Kind: "count", Label: "Кампании", Value: "2"}},
			AllowedCTAs: []string{ActionCTAOpenCampaignList},
			Limitations: []string{"Рекомендации только на уровне кампаний."},
		}},
	}
	action := DomainAIAction{
		SourceCandidateIDs: []string{"ad_high_drr_2026-06-24_top"},
		Type:               ActionTypeAdHighDRR,
		Category:           ActionCategoryAdvertising,
		Priority:           ActionPriorityImportant,
		Title:              "SKU сливает бюджет в кампании",
		Reason:             "Проверьте кампании",
		NextStep:           "Откройте Ozon Performance",
		EvidenceFactIDs:    []string{"ad_high_drr_2026-06-24_top.count"},
		CTAKeys:            []string{ActionCTAOpenCampaignList},
		Limitations:        []string{"Рекомендации только на уровне кампаний."},
	}
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainAdvertising, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainAdvertising, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatalf("expected fatal rejection for SKU-level ad claim")
	}
}

func TestValidateDomainComposerOutput_MissingLimitationRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.Limitations = nil
	output := DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}
	result := ValidateDomainComposerOutput(domainCtx, output, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_MissingEvidenceFactIDsRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.EvidenceFactIDs = nil
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_MissingCTAKeysRepairable(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.CTAKeys = nil
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q want repairable", result.Status)
	}
}

func TestValidateDomainComposerOutput_MissingSourceCandidatesFatal(t *testing.T) {
	domainCtx := sampleStockDomainContext()
	action := validStockAIAction()
	action.SourceCandidateIDs = nil
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected fatal rejection")
	}
	if result.Status != ValidationStatusInvalid {
		t.Fatalf("status=%q want invalid", result.Status)
	}
}

func TestValidateDomainComposerOutput_StockWithoutEntityGroupFatal(t *testing.T) {
	domainCtx := ActionDomainContext{
		Domain: ActionDomainStock,
		Status: DomainStatusAvailable,
		Candidates: []ActionCandidate{{
			CandidateID:  "stock_no_group",
			Domain:       ActionDomainStock,
			Type:         ActionTypeStockReplenishment,
			Category:     ActionCategoryStock,
			PriorityHint: ActionPriorityCritical,
			Facts:        []ActionCandidateFact{{FactID: "stock_no_group.f1", Kind: "metric", Label: "SKU", Value: "2"}},
			AllowedCTAs:  []string{ActionCTAOpenSKUList, ActionCTAMarkChecked},
		}},
	}
	action := DomainAIAction{
		SourceCandidateIDs: []string{"stock_no_group"},
		Type:               ActionTypeStockReplenishment,
		Category:           ActionCategoryStock,
		Priority:           ActionPriorityCritical,
		Title:              "Пополнить 2 SKU",
		Reason:             "Низкое покрытие",
		NextStep:           "Проверьте остатки",
		EvidenceFactIDs:    []string{"stock_no_group.f1"},
		CTAKeys:            []string{ActionCTAOpenSKUList, ActionCTAMarkChecked},
	}
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainStock, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainStock, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected fatal rejection for missing entity group")
	}
}

func TestValidateDomainComposerOutput_AdWithoutCampaignGroupFatal(t *testing.T) {
	domainCtx := ActionDomainContext{
		Domain: ActionDomainAdvertising,
		Status: DomainStatusAvailable,
		Candidates: []ActionCandidate{{
			CandidateID:  "ad_no_group",
			Domain:       ActionDomainAdvertising,
			Type:         ActionTypeAdHighDRR,
			Category:     ActionCategoryAdvertising,
			PriorityHint: ActionPriorityWatch,
			Facts:        []ActionCandidateFact{{FactID: "ad_no_group.f1", Kind: "count", Label: "Кампании", Value: "1"}},
			AllowedCTAs:  []string{ActionCTAOpenCampaignList, ActionCTAMarkChecked},
		}},
	}
	action := DomainAIAction{
		SourceCandidateIDs: []string{"ad_no_group"},
		Type:               ActionTypeAdHighDRR,
		Category:           ActionCategoryAdvertising,
		Priority:           ActionPriorityWatch,
		Title:              "Снизить DRR",
		Reason:             "Высокий DRR",
		NextStep:           "Проверьте кампании",
		EvidenceFactIDs:    []string{"ad_no_group.f1"},
		CTAKeys:            []string{ActionCTAOpenCampaignList, ActionCTAMarkChecked},
	}
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainAdvertising, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainAdvertising, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected fatal rejection for missing campaign entity group")
	}
}

func TestValidateDomainComposerOutput_FunnelTransitionAccepted(t *testing.T) {
	domainCtx := ActionDomainContext{
		Domain: ActionDomainFunnel,
		Status: DomainStatusAvailable,
		Candidates: []ActionCandidate{{
			CandidateID:  "funnel_ok",
			Domain:       ActionDomainFunnel,
			Type:         ActionTypeFunnelWeakTransition,
			Category:     ActionCategoryFunnel,
			PriorityHint: ActionPriorityWatch,
			Aggregates: map[string]any{
				"transition_key": "pdp_to_cart",
				"from_stage":     "Просмотры",
				"to_stage":       "Корзина",
			},
			Facts:       []ActionCandidateFact{{FactID: "funnel_ok.f1", Kind: "metric", Label: "Конверсия", Value: "1.8", Unit: "%"}},
			AllowedCTAs: []string{ActionCTAAskAI, ActionCTAMarkChecked},
		}},
	}
	action := DomainAIAction{
		SourceCandidateIDs: []string{"funnel_ok"},
		Type:               ActionTypeFunnelWeakTransition,
		Category:           ActionCategoryFunnel,
		Priority:           ActionPriorityWatch,
		Title:              "Усилить переход",
		Reason:             "Слабый переход",
		NextStep:           "Проверьте карточку",
		EvidenceFactIDs:    []string{"funnel_ok.f1"},
		CTAKeys:            []string{ActionCTAAskAI, ActionCTAMarkChecked},
	}
	result := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{Domain: ActionDomainFunnel, Actions: []DomainAIAction{action}}, validationRulesForDomain(ActionDomainFunnel, DefaultForbiddenClaims))
	if len(result.AcceptedActions) != 1 {
		t.Fatalf("expected accepted funnel action, status=%q issues=%+v", result.Status, result.Issues)
	}
}
