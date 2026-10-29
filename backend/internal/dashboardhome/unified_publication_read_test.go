package dashboardhome

import "testing"

func TestAcceptStoredUnifiedPublicationNarrative(t *testing.T) {
	payload := &dashboardNarrativePayload{
		Headline:         "Unified headline",
		ExecutiveSummary: "Summary from unified publication",
		AccountHealth: &dashboardAccountHealthBlock{
			HealthScore: ptrInt32(42),
			Severity:    "critical",
			RiskDrivers: []dashboardNarrativeRiskDriverView{{
				ID: "d1", Category: "sales", CategoryLabel: "Продажи",
				Severity: "critical", ImpactScore: 80, Title: "Drop", Reason: "Orders fell",
			}},
		},
		TopActions: []dashboardNarrativeTopAction{{
			Priority: "critical", Category: "sales", Title: "Fix drop",
			Reason: "Orders fell", NextStep: "Review SKUs", ExpectedEffect: "Stabilize",
		}},
		SellerPulseNarrative: &SellerPulseNarrative{Status: "available", Headline: "Pulse", Summary: "ok"},
	}
	stored := &storedDashboardNarrative{
		Status:        "completed",
		PromptVersion: UnifiedPublicationAdapterPromptVersion,
		Payload:       payload,
	}
	outcome := acceptStoredDashboardNarrativeAIOnly(stored, nil, 3)
	if !outcome.Applied {
		t.Fatalf("expected unified publication narrative to be applied, status=%s", outcome.ValidationStatus)
	}
	if outcome.Payload == nil || outcome.Payload.Headline != "Unified headline" {
		t.Fatalf("payload not applied: %#v", outcome.Payload)
	}
}

func TestEnforceAIOnlySkipsUnifiedPublicationPlaceholder(t *testing.T) {
	score := int32(42)
	resp := &Response{
		Status: StatusPartial,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				NarrativeSource:  "ai",
				PromptVersion:    UnifiedPublicationAdapterPromptVersion,
				ValidationStatus: "completed",
				HealthScore:      &score,
				Headline:         "Unified headline",
			},
		},
	}
	enforceAIOnlyGenerationRequired(resp)
	if resp.Status == StatusAIGenerationRequired {
		t.Fatal("unified publication must not be replaced with AI-generation-required placeholder")
	}
	for _, note := range resp.DataQualityNotes {
		if note == "AI dashboard not generated for current metric date. Run dashboard_narrative_v9_ai_only_chunks generation." {
			t.Fatal("legacy v9 placeholder must not appear for unified publication")
		}
	}
	if resp.Status != StatusReady && resp.Status != StatusPartial {
		t.Fatalf("status=%q", resp.Status)
	}
}

func TestIsUnifiedPublicationPromptVersion(t *testing.T) {
	if !IsUnifiedPublicationPromptVersion(UnifiedPublicationAdapterPromptVersion) {
		t.Fatal("expected adapter version to match")
	}
	if IsUnifiedPublicationPromptVersion(DashboardNarrativePromptVersion) {
		t.Fatal("v9 prompt must not be treated as unified adapter")
	}
}
