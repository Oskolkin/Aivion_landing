package aidatapack_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestFinalMetricSemanticIssues_IDCDaysForbiddenTerm(t *testing.T) {
	issues := aidatapack.FinalMetricSemanticIssues(
		"Срок хранения составляет 142 дня",
		[]string{"idc_days"},
		"reason",
		[]string{"sku.801000100002.stock.idc_days"},
	)
	if len(issues) == 0 || issues[0].Code != aidatapack.FinalMetricSemanticMismatchCode {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestFinalMetricSemanticIssues_IDCDaysCorrectTerm(t *testing.T) {
	issues := aidatapack.FinalMetricSemanticIssues(
		"Покрытие запасом составляет 142 дня",
		[]string{"idc_days"},
		"reason",
		nil,
	)
	if len(issues) > 0 {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestDetectUnsupportedShelfLifeClaim(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{FactRegistry: map[string]aidatapack.Fact{}}
	issues := aidatapack.DetectUnsupportedShelfLifeClaim("Срок хранения составляет 90 дней", pack, "summary")
	if len(issues) == 0 || issues[0].Code != aidatapack.FinalUnsupportedMetricClaimCode {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestDetectUnsupportedShelfLifeClaim_AllowedWithMetric(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"sku.1.shelf_life_days": {FactID: "sku.1.shelf_life_days", Metric: "shelf_life_days"},
		},
	}
	text := "Срок хранения составляет 90 дней"
	issues := aidatapack.DetectUnsupportedShelfLifeClaim(text, pack, "summary")
	if len(issues) > 0 {
		t.Fatalf("issues=%+v", issues)
	}
	issues = aidatapack.FinalMetricSemanticIssues(text, []string{"shelf_life_days"}, "summary", []string{"sku.1.shelf_life_days"})
	if len(issues) > 0 {
		t.Fatalf("grounded shelf life should pass: %+v", issues)
	}
}
