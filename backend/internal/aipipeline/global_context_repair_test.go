package aipipeline

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestRepairGlobalSemanticContextCategoryAliases(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Кабинет требует внимания.",
		SecondaryRisks: []aicontext.SemanticRisk{{
			RiskID:          "r1",
			Category:        "product_data",
			Severity:        "medium",
			Title:           "Качество карточек",
			EvidenceFactIDs: []string{"account.current_day.revenue"},
		}},
		PriorityOrder: []string{"stock", "product_data"},
	}
	RepairGlobalSemanticContext(&ctx, input)
	if ctx.SecondaryRisks[0].Category != "sku" || ctx.PriorityOrder[1] != "sku" {
		t.Fatalf("categories=%v priority=%v", ctx.SecondaryRisks[0].Category, ctx.PriorityOrder)
	}
}

func TestRepairGlobalSemanticContextInfersSourceFactIDs(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Кабинет требует внимания.",
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID:          "r1",
			Category:        "sales",
			Severity:        "medium",
			Title:           "Продажи",
			EvidenceFactIDs: []string{"account.current_day.revenue"},
		}},
	}
	RepairGlobalSemanticContext(&ctx, input)
	if len(ctx.SourceFactIDs) != 1 || ctx.SourceFactIDs[0] != "account.current_day.revenue" {
		t.Fatalf("source_fact_ids=%v", ctx.SourceFactIDs)
	}
	if ctx.Confidence != "medium" {
		t.Fatalf("confidence=%q", ctx.Confidence)
	}
	result := ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusValid && result.Status != aicontext.ValidationStatusValidWithWarnings {
		t.Fatalf("status=%s issues=%+v", result.Status, result.Issues)
	}
}

func TestRepairGlobalSemanticContextDropsUnknownFacts(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Кабинет требует внимания.",
		Confidence:    "medium",
		SourceFactIDs: []string{"account.current_day.revenue", "unknown.fact"},
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID:          "r1",
			Category:        "sales",
			Severity:        "medium",
			Title:           "Продажи",
			EvidenceFactIDs: []string{"account.current_day.revenue", "unknown.fact"},
		}},
	}
	RepairGlobalSemanticContext(&ctx, input)
	result := ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusValid && result.Status != aicontext.ValidationStatusValidWithWarnings {
		t.Fatalf("status=%s issues=%+v", result.Status, result.Issues)
	}
}

func TestRepairGlobalSemanticContextStripsUnsupportedNumbers(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Выручка снизилась на 7.8% за 7 дней.",
		Confidence:    "medium",
		SourceFactIDs: []string{"account.current_day.revenue"},
	}
	RepairGlobalSemanticContext(&ctx, input)
	result := ValidateGlobalSemanticContext(input, ctx)
	if !globalContextValidationAccepted(result.Status) {
		t.Fatalf("status=%s issues=%+v", result.Status, result.Issues)
	}
	if strings.Contains(ctx.OverallStory, "7.8") {
		t.Fatalf("overall_story=%q", ctx.OverallStory)
	}
}

func TestRepairGlobalSemanticContextDropsUnknownCategory(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Кабинет требует внимания.",
		Confidence:    "medium",
		SourceFactIDs: []string{"account.current_day.revenue"},
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID: "r1", Category: "data_quality", Severity: "high", Title: "Плохие данные",
			EvidenceFactIDs: []string{"account.current_day.revenue"},
		}},
	}
	RepairGlobalSemanticContext(&ctx, input)
	if len(ctx.PrimaryRisks) != 0 {
		t.Fatalf("primary_risks=%+v", ctx.PrimaryRisks)
	}
}

func TestRepairGlobalSemanticContextNumericConfidence(t *testing.T) {
	input := DefaultGlobalContextGenerationInput(repairTestPack(), "sha256:test")
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		DataPackHash:  input.DataPackHash,
		OverallStory:  "Кабинет требует внимания.",
		Confidence:    "7.8",
		SourceFactIDs: []string{"account.current_day.revenue"},
	}
	RepairGlobalSemanticContext(&ctx, input)
	if ctx.Confidence != "medium" {
		t.Fatalf("confidence=%q", ctx.Confidence)
	}
}

func TestGlobalContextIsEffectivelyEmpty(t *testing.T) {
	if !GlobalContextIsEffectivelyEmpty(&aicontext.GlobalSemanticContext{}) {
		t.Fatal("expected empty")
	}
	if GlobalContextIsEffectivelyEmpty(&aicontext.GlobalSemanticContext{OverallStory: "story"}) {
		t.Fatal("expected non-empty")
	}
}

func repairTestPack() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-24"},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
		},
	}
}
