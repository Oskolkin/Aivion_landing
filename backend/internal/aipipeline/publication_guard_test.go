package aipipeline_test

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestRunPublicationGuard_BlocksIDCDaysShelfLifeClaim(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"sku.801000100002.stock.idc_days": {FactID: "sku.801000100002.stock.idc_days", Metric: "idc_days"},
		},
	}
	items := []actions.ActionItem{{
		ActionKey:       "reduce_surplus_stock",
		Reason:          "Срок хранения товара составляет 142 дня",
		EvidenceFactIDs: []string{"sku.801000100002.stock.idc_days"},
	}}
	decision := aipipeline.RunPublicationGuard(aipipeline.PublicationGuardInput{
		RequestedActions: true,
		Pack:             pack,
		Actions:          items,
	})
	if decision.PublicationAllowed {
		t.Fatal("expected publication blocked")
	}
	if decision.DecisionReason != aipipeline.PublicationBlockedByFinalSemanticValidation {
		t.Fatalf("reason=%q", decision.DecisionReason)
	}
}

func TestRunPublicationGuard_AllowsCorrectIDCDaysPhrase(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"sku.801000100002.stock.idc_days": {FactID: "sku.801000100002.stock.idc_days", Metric: "idc_days"},
		},
	}
	items := []actions.ActionItem{{
		ActionKey:       "reduce_surplus_stock",
		Reason:          "Покрытие запасом составляет 142 дня",
		EvidenceFactIDs: []string{"sku.801000100002.stock.idc_days"},
	}}
	decision := aipipeline.RunPublicationGuard(aipipeline.PublicationGuardInput{
		RequestedActions: true,
		Pack:             pack,
		Actions:          items,
	})
	if !decision.PublicationAllowed {
		t.Fatalf("expected allowed, issues=%+v", decision.FinalSemanticValidation.Actions.Issues)
	}
}

func TestValidateFinalDashboardOutput_BlocksPlaceholder(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"headline":          "Сводка",
		"executive_summary": "Выручка снизилась на %",
	})
	result := aipipeline.ValidateFinalDashboardOutput(nil, raw)
	if result.Status != "failed" {
		t.Fatalf("status=%q issues=%+v", result.Status, result.Issues)
	}
}

func TestValidateFinalDashboardOutput_AllowsFilledPercent(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"headline":          "Сводка",
		"executive_summary": "Продажи снизились на 6,01%",
	})
	result := aipipeline.ValidateFinalDashboardOutput(nil, raw)
	if result.BlockingIssueCount > 0 {
		t.Fatalf("issues=%+v", result.Issues)
	}
}
