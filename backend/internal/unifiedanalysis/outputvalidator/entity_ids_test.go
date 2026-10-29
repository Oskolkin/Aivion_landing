package outputvalidator_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func testIndex() *outputvalidator.PackReferenceIndex {
	return &outputvalidator.PackReferenceIndex{
		SKUIds: map[string]struct{}{
			"801000100002": {},
			"801000100005": {},
			"801000100003": {},
			"801000100004": {},
		},
		CampaignIds: map[string]struct{}{
			"882000001625": {},
		},
		LimitationCodes: map[string]struct{}{
			"sku_funnel_metrics_unavailable":    {},
			"campaign_sku_spend_not_attributed": {},
		},
	}
}

func hasCode(report outputvalidator.Report, code string) bool {
	for _, e := range report.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestValidateSKUScopeRequiresEntityIDs(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Actions = out.Actions[:1]
	out.Actions[0].Scope.ScopeType = "sku"
	out.Actions[0].Scope.EntityIDs = nil
	out.Dashboard.TopActionIDs = []string{out.Actions[0].AnalysisActionID}
	out.Dashboard.RecommendedActionIDs = []string{out.Actions[0].AnalysisActionID}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for empty sku entity_ids")
	}
	if !hasCode(report, outputvalidator.CodeOutputMissingScopeEntities) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputMissingScopeEntities, report.Errors)
	}
}

func TestValidateCampaignScopeRequiresEntityIDs(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Actions = out.Actions[:1]
	out.Actions[0].Scope.ScopeType = "campaign"
	out.Actions[0].Scope.EntityIDs = []string{}
	out.Dashboard.TopActionIDs = []string{out.Actions[0].AnalysisActionID}
	out.Dashboard.RecommendedActionIDs = []string{out.Actions[0].AnalysisActionID}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for empty campaign entity_ids")
	}
	if !hasCode(report, outputvalidator.CodeOutputMissingScopeEntities) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputMissingScopeEntities, report.Errors)
	}
}

func TestValidateAccountScopeAllowsEmptyEntityIDs(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	// Keep material action floor; only the account-scoped action must allow empty entity_ids.
	out.Actions[2].Scope.ScopeType = "account"
	out.Actions[2].Scope.EntityIDs = []string{}
	out.Actions[2].Limitations = nil
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("account scope empty entity_ids must be valid, errors=%+v", report.Errors)
	}
}

func TestValidateUnknownSKUID(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Actions = out.Actions[:1]
	out.Actions[0].Scope.ScopeType = "sku"
	out.Actions[0].Scope.EntityIDs = []string{"999999999999"}
	out.Dashboard.TopActionIDs = []string{out.Actions[0].AnalysisActionID}
	out.Dashboard.RecommendedActionIDs = []string{out.Actions[0].AnalysisActionID}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for unknown sku")
	}
	if !hasCode(report, outputvalidator.CodeOutputUnknownSKU) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputUnknownSKU, report.Errors)
	}
}

func TestValidateUnknownCampaignID(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Actions = out.Actions[:1]
	out.Actions[0].Scope.ScopeType = "campaign"
	out.Actions[0].Scope.EntityIDs = []string{"999999999999"}
	out.Dashboard.TopActionIDs = []string{out.Actions[0].AnalysisActionID}
	out.Dashboard.RecommendedActionIDs = []string{out.Actions[0].AnalysisActionID}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for unknown campaign")
	}
	if !hasCode(report, outputvalidator.CodeOutputUnknownCampaign) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputUnknownCampaign, report.Errors)
	}
}

func TestValidateDuplicateSKUAcrossActions(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	if len(out.Actions) < 2 {
		t.Fatal("need 2 actions")
	}
	out.Actions[0].Scope.ScopeType = "sku"
	out.Actions[0].Scope.EntityIDs = []string{"801000100002"}
	out.Actions[1].Scope.ScopeType = "sku"
	out.Actions[1].Scope.EntityIDs = []string{"801000100002"}
	out.Dashboard.TopActionIDs = []string{out.Actions[0].AnalysisActionID}
	out.Dashboard.RecommendedActionIDs = []string{out.Actions[0].AnalysisActionID, out.Actions[1].AnalysisActionID}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for duplicate sku across actions")
	}
	if !hasCode(report, outputvalidator.CodeOutputDuplicateSKUAcrossActions) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputDuplicateSKUAcrossActions, report.Errors)
	}
}

func TestValidateFreeTextLimitationInvalid(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	out.Limitations = []string{"Нет данных по SKU"}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid for free-text limitation")
	}
	if !hasCode(report, outputvalidator.CodeOutputUnknownLimitation) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputUnknownLimitation, report.Errors)
	}
}

func TestValidateAllowedLimitationCode(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	out.Limitations = []string{"sku_funnel_metrics_unavailable"}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if !report.Valid {
		t.Fatalf("expected valid for allowlisted pack limitation, errors=%+v", report.Errors)
	}
}

func TestValidateHealthyOutcomeRejectsActions(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.no_actions.json")
	out.Actions = loadFixture(t, "unified_output_v1.valid.json").Actions[:1]
	out.Actions[0].Scope.ScopeType = "account"
	out.Actions[0].Scope.EntityIDs = []string{}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid when no_material_actions has actions")
	}
	if !hasCode(report, outputvalidator.CodeOutcomeActionsMismatch) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutcomeActionsMismatch, report.Errors)
	}
}

func TestValidateInsufficientOutcomeRejectsActions(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	action := loadFixture(t, "unified_output_v1.valid.json").Actions[0]
	action.Scope.ScopeType = "account"
	action.Scope.EntityIDs = []string{}
	out.Actions = []contract.UnifiedActionV1{action}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid when insufficient_data has actions")
	}
	if !hasCode(report, outputvalidator.CodeOutcomeActionsMismatch) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutcomeActionsMismatch, report.Errors)
	}
}

func TestValidateMaterialActionsFoundRequiresHeadline(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.AccountSummary.Headline = ""
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when material_actions_found has empty account_summary.headline")
	}
	if !hasCode(report, outputvalidator.CodeMissingAccountSummaryHeadline) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingAccountSummaryHeadline, report.Errors)
	}
}

func TestValidateMaterialActionsFoundRequiresExecutiveSummary(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.AccountSummary.ExecutiveSummary = ""
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when material_actions_found has empty account_summary.executive_summary")
	}
	if !hasCode(report, outputvalidator.CodeMissingExecutiveSummary) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingExecutiveSummary, report.Errors)
	}
}

func TestValidateNoMaterialActionsRequiresHeadlineAndExecutiveSummary(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.no_actions.json")
	out.Dashboard.AccountSummary.Headline = ""
	out.Dashboard.AccountSummary.ExecutiveSummary = ""
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when no_material_actions has empty headline/executive_summary")
	}
	if !hasCode(report, outputvalidator.CodeMissingAccountSummaryHeadline) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingAccountSummaryHeadline, report.Errors)
	}
	if !hasCode(report, outputvalidator.CodeMissingExecutiveSummary) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingExecutiveSummary, report.Errors)
	}
}

func TestValidateMaterialOutcomeRequiresSellerPulseNarrative(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SellerPulse.Narrative = nil
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when material outcome has nil seller_pulse narrative")
	}
	if !hasCode(report, outputvalidator.CodeMissingSellerPulseNarrative) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingSellerPulseNarrative, report.Errors)
	}
}

func TestValidateMaterialOutcomeRejectsPendingSellerPulseNarrativeStatus(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SellerPulse.Narrative.Status = "pending"
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when material outcome has pending seller_pulse narrative status")
	}
	if !hasCode(report, outputvalidator.CodeInvalidSellerPulseNarrativeStatus) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeInvalidSellerPulseNarrativeStatus, report.Errors)
	}
}

func TestValidateMaterialOutcomeRequiresSellerPulseNarrativeText(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SellerPulse.Narrative.Headline = ""
	out.Dashboard.SellerPulse.Narrative.Summary = ""
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when material outcome has empty seller_pulse narrative text")
	}
	if !hasCode(report, outputvalidator.CodeMissingSellerPulseNarrativeText) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingSellerPulseNarrativeText, report.Errors)
	}
}

func TestValidateInsufficientDataAllowsMinimalSellerPulseNarrative(t *testing.T) {
	// insufficient_data / analysis_failed are not "material" outcomes: the
	// seller_pulse narrative gate must not apply to them even though the
	// fixture's narrative status is not_enough_data.
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("expected valid for insufficient_data fixture, errors=%+v", report.Errors)
	}
}

func TestValidateMaterialOutcomeIgnoresKPINumberMismatch(t *testing.T) {
	// The output validator never fails purely on AI-reported KPI numbers
	// disagreeing with anything — those arrays are backend/facts-owned, not
	// validated here at all.
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SellerPulse.KPIs[0].Value = "999999999"
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("KPI number content must never fail validation, errors=%+v", report.Errors)
	}
}

func TestValidateInsufficientOutcomeRequiresLimitations(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	out.Limitations = []string{}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate,
	}, testIndex())
	if report.Valid {
		t.Fatal("expected invalid when insufficient_data has empty limitations")
	}
	if !hasCode(report, outputvalidator.CodeInsufficientLimitations) {
		t.Fatalf("want %s, errors=%+v", outputvalidator.CodeInsufficientLimitations, report.Errors)
	}
}
