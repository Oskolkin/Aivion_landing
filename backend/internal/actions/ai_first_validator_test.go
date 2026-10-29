package actions

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func testAIFirstInput() ActionsAIFirstInput {
	pack := testAIFirstDataPack()
	return ActionsAIFirstInput{
		SchemaVersion:      ActionsAIFirstSchemaVersionV1,
		SellerAccountID:    1,
		MetricDate:         "2026-06-24",
		DataPack:           &pack,
		AllowedActionTypes: AIFirstAllowedActionTypes,
		AllowedCategories:  AIFirstAllowedCategories,
		AllowedPriorities:  []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch},
		AllowedCTAKeys:     []string{ActionCTAAskAI, ActionCTAOpenSKUList},
		ForbiddenClaims:    DefaultForbiddenClaims,
	}
}

func testAIFirstDataPack() aidatapack.SellerAIDataPack {
	return aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{
			SellerAccountID: 1,
			MetricDate:      "2026-06-24",
			Marketplace:     aidatapack.MarketplaceOzon,
			Timezone:        aidatapack.DefaultTimezone,
		},
		DataWindows: aidatapack.DataWindows{
			CurrentDay: "2026-06-24",
			Yesterday:  "2026-06-23",
			Last7d:     aidatapack.DateRange{From: "2026-06-18", To: "2026-06-24"},
			Previous7d: aidatapack.DateRange{From: "2026-06-11", To: "2026-06-17"},
			Last14d:    aidatapack.DateRange{From: "2026-06-11", To: "2026-06-24"},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs: map[string]aidatapack.SKUEntity{
				"sku:1": {EntityID: "sku:1", EntityType: "sku", SKU: "1"},
			},
			Campaigns: map[string]aidatapack.CampaignEntity{
				"campaign:10": {EntityID: "campaign:10", EntityType: "campaign", CampaignExternalID: "10"},
			},
			Categories: map[string]aidatapack.CategoryEntity{
				"category:20": {EntityID: "category:20", EntityType: "category", CategoryID: "20"},
			},
		},
		FactRegistry: map[string]aidatapack.Fact{
			"account.last_7d.orders": {FactID: "account.last_7d.orders", EntityType: "account", EntityID: "account:seller", Metric: "orders", Value: float64(12), Unit: "orders"},
			"sku.1.last_7d.orders":   {FactID: "sku.1.last_7d.orders", EntityType: "sku", EntityID: "sku:1", Metric: "orders", Value: float64(5), Unit: "orders"},
		},
	}
}

func validAIFirstAction() AIFirstDraftAction {
	longReason := "По фактам видно 5 заказов за 7 дней и замедление оборота по SKU из списка. " +
		"Товары занимают складской ресурс, но оборот ниже нормы для текущего остатка. " +
		"Нужно проверить карточки, цены, отзывы и стимулирование спроса по каждому SKU из evidence, " +
		"чтобы не накапливать замороженный остаток и вернуть SKU в активные продажи через 7 дней."
	return AIFirstDraftAction{
		Type:               ActionTypeSlowTurnoverReview,
		Category:           ActionCategorySlowTurnover,
		Priority:           ActionPriorityImportant,
		Title:              "Ускорить оборачиваемость 5 SKU",
		Summary:            "Продажи составили 5 заказов.",
		Reason:             longReason,
		NextStep:           "Открыть список SKU и начать с товаров с наименьшим числом заказов за 7 дней.",
		ExpectedEffect:     "Снизить риск замороженных остатков и ускорить оборот.",
		CompletionCriteria: "Решение принято по каждому SKU из списка.",
		EvidenceFactIDs:    []string{"sku.1.last_7d.orders"},
		CTAKeys:            []string{ActionCTAAskAI},
		AffectedEntities: []AIFirstAffectedEntityRef{
			{EntityType: "sku", EntityID: "sku:1", FactIDs: []string{"sku.1.last_7d.orders"}},
		},
		ActionPlan: []AIFirstActionStep{
			{Title: "Открыть SKU", Instruction: "Открыть список SKU с низкой оборачиваемостью и начать с sku:1.", ExpectedResult: "Определён порядок обработки."},
			{Title: "Проверить карточку", Instruction: "Проверить цену, фото, название и отзывы карточки sku:1.", ExpectedResult: "Выявлены проблемы контента."},
			{Title: "Стимулировать спрос", Instruction: "Запустить акцию или перераспределить рекламное внимание на кампании sku:1.", ExpectedResult: "Запущены меры стимулирования."},
		},
		ObjectActions: []AIFirstObjectAction{
			{
				EntityType:        "sku",
				EntityID:          "sku:1",
				Name:              "SKU 1",
				RecommendedAction: "Проверить карточку и остатки.",
				Reason:            "Замедление оборота и только 5 заказов за 7 дней.",
				EvidenceFactIDs:   []string{"sku.1.last_7d.orders"},
				Metrics: []AIFirstObjectMetric{
					{Label: "Заказы 7д", Value: "5", Unit: "orders"},
					{Label: "Остаток", Value: "120", Unit: "шт"},
				},
			},
		},
	}
}

func TestValidateActionsAIFirstOutput_ValidAcceptedWithoutSourceCandidates(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.SourceCandidateIDs = nil
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{
		SchemaVersion: input.SchemaVersion,
		Actions:       []AIFirstDraftAction{action},
	})
	if len(result.AcceptedActions) != 1 {
		t.Fatalf("accepted=%d issues=%+v", len(result.AcceptedActions), result.Issues)
	}
}

func TestValidateActionsAIFirstOutput_UnknownFactRejected(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.EvidenceFactIDs = []string{"unknown.fact"}
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected rejection")
	}
}

func TestValidateActionsAIFirstOutput_UnknownAffectedEntityRejected(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku:999"}}
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected rejection")
	}
}

func TestValidateActionsAIFirstOutput_InvalidTypeRejected(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.Type = ActionType("bogus_type")
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected rejection")
	}
}

func TestValidateActionsAIFirstOutput_InvalidCategoryRejected(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.Category = ActionCategory("bogus")
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected rejection")
	}
}

func TestValidateActionsAIFirstOutput_AdvertisingSKULevelRejected(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.Type = ActionTypeAdHighDRRReview
	action.Category = ActionCategoryAdvertising
	action.Title = "SKU сливает бюджет"
	action.AffectedEntities = []AIFirstAffectedEntityRef{{EntityType: "campaign", EntityID: "campaign:10"}}
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 0 {
		t.Fatal("expected fatal rejection")
	}
}

func TestValidateActionsAIFirstOutput_UnsupportedNumberRepairable(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.Title = "Ускорить оборачиваемость 999 SKU"
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if result.Status != ValidationStatusRepairable {
		t.Fatalf("status=%q issues=%+v", result.Status, result.Issues)
	}
}

func TestValidateActionsAIFirstOutput_SlowTurnoverAccepted(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	result := ValidateActionsAIFirstOutput(input, ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: []AIFirstDraftAction{action}})
	if len(result.AcceptedActions) != 1 {
		t.Fatalf("accepted=%d issues=%+v", len(result.AcceptedActions), result.Issues)
	}
}

func TestHasStockSupplyLimitation_AcceptsEnglishSnapshotLimitation(t *testing.T) {
	action := AIFirstDraftAction{
		Type: ActionTypeStockReplenishment,
		VisibleLimitations: []string{
			"daily_sku_metrics.stock_available is treated as a current snapshot, not historical stock movement.",
			"Stock analytics source is not available, so warehouse-level and historical stock movement conclusions are limited.",
		},
	}
	if !hasStockSupplyLimitation(action) {
		t.Fatal("expected english stock limitation markers to pass")
	}
}

func TestAllowedNumbersForAIFirstAction_AllowsOppositeSignForPctFact(t *testing.T) {
	input := testAIFirstInput()
	input.DataPack.FactRegistry["account.revenue.last_7d_vs_previous_7d_delta_pct"] = aidatapack.Fact{
		FactID: "account.revenue.last_7d_vs_previous_7d_delta_pct",
		Value:  -5.03,
		Unit:   "percent",
	}
	allowed := allowedNumbersForAIFirstAction(input, collectAIFirstAllowedNumbers(input), AIFirstDraftAction{
		EvidenceFactIDs: []string{"account.revenue.last_7d_vs_previous_7d_delta_pct"},
	})
	if !numberAllowed("5.03", allowed) {
		t.Fatalf("expected opposite-sign pct to be allowed, allowed=%v", allowed)
	}
}
