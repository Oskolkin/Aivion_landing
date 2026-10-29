package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func floatPtr(v float64) *float64 { return &v }

func testPackIndex() *PackReferenceIndex {
	return &PackReferenceIndex{
		SKUIds:      map[string]struct{}{"801000100001": {}},
		CampaignIds: map[string]struct{}{"555": {}},
	}
}

func TestValidateActionEvidenceMissing(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		Scope: contract.UnifiedActionScopeV1{ScopeType: "account"},
	}
	validateActionEvidence(report, "actions[0]", action, testPackIndex())
	if !hasCode(report.Errors, CodeActionEvidenceMissing) {
		t.Fatalf("expected %s, got %+v", CodeActionEvidenceMissing, report.Errors)
	}
}

func TestValidateActionEvidenceUnknownFact(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		Scope:           contract.UnifiedActionScopeV1{ScopeType: "sku", EntityIDs: []string{"801000100001"}},
		EvidenceFactIDs: []string{"sku:999999999999:orders_7d"},
	}
	validateActionEvidence(report, "actions[0]", action, testPackIndex())
	if !hasCode(report.Errors, CodeActionEvidenceFactUnknown) {
		t.Fatalf("expected %s, got %+v", CodeActionEvidenceFactUnknown, report.Errors)
	}
}

func TestValidateActionEvidenceResolvesKnownFact(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		Scope:           contract.UnifiedActionScopeV1{ScopeType: "sku", EntityIDs: []string{"801000100001"}},
		EvidenceFactIDs: []string{"sku:801000100001:orders_7d"},
	}
	validateActionEvidence(report, "actions[0]", action, testPackIndex())
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors for a resolvable fact, got %+v", report.Errors)
	}
}

func TestValidateEvidenceScopeInconsistentWarns(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		Scope:           contract.UnifiedActionScopeV1{ScopeType: "sku", EntityIDs: []string{"801000100001"}},
		EvidenceFactIDs: []string{"sku:801000100001:orders_7d", "sku:801000100002:orders_7d"},
	}
	// Make the out-of-scope sku resolvable so only the scope-consistency
	// warning (not a hard fact-unknown error) is exercised.
	idx := testPackIndex()
	idx.SKUIds["801000100002"] = struct{}{}
	validateActionEvidence(report, "actions[0]", action, idx)
	if !hasCode(report.Warnings, CodeActionEvidenceScopeInconsistent) {
		t.Fatalf("expected %s warning, got %+v", CodeActionEvidenceScopeInconsistent, report.Warnings)
	}
}

func TestValidateCompletionConditionBaselineMissing(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		CompletionCriteria: "orders per week reaches 20",
		CompletionConditions: []contract.UnifiedCompletionConditionV1{{
			MetricField: "orders_7d",
			Operator:    ">=",
			TargetValue: floatPtr(20),
			// baseline_fact_id / baseline_value intentionally absent.
		}},
	}
	validateCompletionConditions(report, "actions[0]", action, testPackIndex())
	if !hasCode(report.Errors, CodeActionCompletionBaselineMissing) {
		t.Fatalf("expected %s, got %+v", CodeActionCompletionBaselineMissing, report.Errors)
	}
}

func TestValidateCompletionConditionTargetAlreadyMet(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		CompletionCriteria: "orders per week stays at or above 10",
		CompletionConditions: []contract.UnifiedCompletionConditionV1{{
			MetricField:    "orders_7d",
			BaselineFactID: "sku:801000100001:orders_7d",
			BaselineValue:  floatPtr(25),
			Operator:       ">=",
			TargetValue:    floatPtr(20),
		}},
	}
	validateCompletionConditions(report, "actions[0]", action, testPackIndex())
	if !hasCode(report.Errors, CodeActionCompletionTargetAlreadyMet) {
		t.Fatalf("expected %s, got %+v", CodeActionCompletionTargetAlreadyMet, report.Errors)
	}
}

func TestValidateCompletionConditionTextMachineMismatch(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		CompletionCriteria: "", // machine spec present but no text criterion
		CompletionConditions: []contract.UnifiedCompletionConditionV1{{
			MetricField:    "orders_7d",
			BaselineFactID: "sku:801000100001:orders_7d",
			BaselineValue:  floatPtr(5),
			Operator:       ">=",
			TargetValue:    floatPtr(20),
		}},
	}
	validateCompletionConditions(report, "actions[0]", action, testPackIndex())
	if !hasCode(report.Errors, CodeActionCompletionTextMachineMismatch) {
		t.Fatalf("expected %s, got %+v", CodeActionCompletionTextMachineMismatch, report.Errors)
	}
}

func TestValidateCompletionConditionWellFormedPasses(t *testing.T) {
	report := &Report{}
	action := contract.UnifiedActionV1{
		CompletionCriteria: "orders per week grows from 5 to 20",
		CompletionConditions: []contract.UnifiedCompletionConditionV1{{
			MetricField:    "orders_7d",
			BaselineFactID: "sku:801000100001:orders_7d",
			BaselineValue:  floatPtr(5),
			Operator:       ">=",
			TargetValue:    floatPtr(20),
			PeriodDays:     func() *int { d := 7; return &d }(),
		}},
	}
	validateCompletionConditions(report, "actions[0]", action, testPackIndex())
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors for a well-formed condition, got %+v", report.Errors)
	}
}
