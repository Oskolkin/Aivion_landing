package aipipeline_test

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestGlobalContextPromptRules(t *testing.T) {
	prompt := aipipeline.GlobalContextSystemPromptForTest()
	for _, needle := range []string{
		"stock", "advertising", "Генерировать actions",
		"SKU-level", "Data Pack", "source of truth",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}

func TestValidateGlobalContextUnknownFactFails(t *testing.T) {
	input := aipipeline.DefaultGlobalContextGenerationInput(samplePack(), "sha256:test")
	ctx := validGlobalContext(input)
	ctx.SourceFactIDs = []string{"unknown.fact"}
	result := aipipeline.ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusInvalid {
		t.Fatalf("expected invalid, got %s", result.Status)
	}
}

func TestValidateGlobalContextForbiddenCategory(t *testing.T) {
	input := aipipeline.DefaultGlobalContextGenerationInput(samplePack(), "sha256:test")
	ctx := validGlobalContext(input)
	ctx.PrimaryRisks = []aicontext.SemanticRisk{{
		RiskID: "r1", Category: "data_quality", Severity: "high", Title: "bad",
		EvidenceFactIDs: []string{"account.current_day.revenue"},
	}}
	result := aipipeline.ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusInvalid {
		t.Fatalf("expected invalid for data_quality, got %s", result.Status)
	}
}

func TestDeterministicFakeGlobalContextValid(t *testing.T) {
	input := aipipeline.DefaultGlobalContextGenerationInput(samplePack(), "sha256:test")
	client := aipipeline.NewDeterministicGlobalContextFakeClient()
	ctx, _, err := client.GenerateGlobalContext(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	result := aipipeline.ValidateGlobalSemanticContext(input, *ctx)
	if result.Status != aicontext.ValidationStatusValid && result.Status != aicontext.ValidationStatusValidWithWarnings {
		t.Fatalf("expected valid fake context, got %s issues=%v", result.Status, result.Issues)
	}
}

func samplePack() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-24"},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			Campaigns: map[string]aidatapack.CampaignEntity{
				"campaign:1": {EntityID: "campaign:1", Name: "Test"},
			},
		},
		Tables: aidatapack.Tables{
			CampaignRollups: []aidatapack.CampaignRollupRow{{EntityID: "campaign:1", Name: "Test"}},
		},
	}
}

func validGlobalContext(input aipipeline.GlobalContextGenerationInput) aicontext.GlobalSemanticContext {
	return aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		OverallStory:  "Кабинет требует внимания.",
		Confidence:    "medium",
		SourceFactIDs: []string{"account.current_day.revenue"},
		PriorityOrder: []string{"sales"},
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID: "risk1", Category: "sales", Severity: "medium", Title: "Продажи",
			EvidenceFactIDs: []string{"account.current_day.revenue"},
		}},
		DataPackHash: input.DataPackHash,
	}
}
