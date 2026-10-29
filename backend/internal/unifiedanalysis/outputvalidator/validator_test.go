package outputvalidator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func loadFixture(t *testing.T, name string) *contract.UnifiedAnalysisOutputV1 {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "contract", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &out
}

func TestValidateValidFixture(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		PackHash:   out.AnalysisMeta.PackHash,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidateNoActionsFixture(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.no_actions.json")
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidateInsufficientDataFixture(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.insufficient_data.json")
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidateAnalysisFailedFixture(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.analysis_failed.json")
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidateAnalysisFailedRejectsActions(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.analysis_failed.json")
	out.Actions = loadFixture(t, "unified_output_v1.valid.json").Actions[:1]
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if report.Valid {
		t.Fatal("expected invalid when analysis_failed has actions")
	}
}

func TestValidateWrongSchemaVersion(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.SchemaVersion = "wrong"
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, nil)
	if report.Valid {
		t.Fatal("expected invalid")
	}
}

func TestValidateDuplicateActionID(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	if len(out.Actions) < 2 {
		t.Skip("need 2 actions")
	}
	out.Actions[1].AnalysisActionID = out.Actions[0].AnalysisActionID
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, nil)
	if report.Valid {
		t.Fatal("expected invalid for duplicate action id")
	}
}

func TestValidateUnknownTopActionID(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.TopActionIDs = []string{"action_999"}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, nil)
	if report.Valid {
		t.Fatal("expected invalid")
	}
}

// noActionsMaterialFixture loads the valid fixture but strips it down to a
// material_actions_found-free (no_material_actions) shape with no actions
// and no limitations, so tests can plug in a minimal PackReferenceIndex
// without tripping unrelated SKU/campaign/limitation reference checks.
func noActionsMaterialFixture(t *testing.T) *contract.UnifiedAnalysisOutputV1 {
	t.Helper()
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.AnalysisOutcome = contract.AnalysisOutcomeNoMaterialActions
	out.Actions = []contract.UnifiedActionV1{}
	out.Dashboard.TopActionIDs = nil
	out.Dashboard.RecommendedActionIDs = nil
	out.Limitations = []string{}
	return out
}

func TestValidateAdvertisingChecklistRequiredWhenPackHasCampaigns(t *testing.T) {
	out := noActionsMaterialFixture(t)
	out.Dashboard.AdEfficiency.Narrative.Checklist = nil
	idx := &outputvalidator.PackReferenceIndex{HasCampaignsAny: true}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if report.Valid {
		t.Fatal("expected invalid when pack has campaigns but checklist is empty")
	}
	if !hasCode(report, outputvalidator.CodeMissingAdvertisingChecklist) {
		t.Fatalf("expected %s among errors, got %+v", outputvalidator.CodeMissingAdvertisingChecklist, report.Errors)
	}
}

func TestValidateAdvertisingChecklistOKWithTwoItems(t *testing.T) {
	out := noActionsMaterialFixture(t)
	idx := &outputvalidator.PackReferenceIndex{HasCampaignsAny: true}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidateAdvertisingChecklistNotRequiredWithoutCampaigns(t *testing.T) {
	out := noActionsMaterialFixture(t)
	out.Dashboard.AdEfficiency.Narrative.Checklist = nil
	idx := &outputvalidator.PackReferenceIndex{HasCampaignsAny: false}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if !report.Valid {
		t.Fatalf("expected valid when pack has no campaigns, errors=%+v", report.Errors)
	}
}

func TestValidateCategorySummariesRequiredWhenPackHasCategories(t *testing.T) {
	out := noActionsMaterialFixture(t)
	out.Dashboard.CategorySummaries = nil
	idx := &outputvalidator.PackReferenceIndex{CategoryIds: map[string]struct{}{"10": {}}}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if report.Valid {
		t.Fatal("expected invalid when pack has categories but category_summaries is empty")
	}
	if !hasCode(report, outputvalidator.CodeMissingCategorySummaries) {
		t.Fatalf("expected %s among errors, got %+v", outputvalidator.CodeMissingCategorySummaries, report.Errors)
	}
}

func TestValidateCategorySummariesIncompleteEntryRejected(t *testing.T) {
	out := noActionsMaterialFixture(t)
	out.Dashboard.CategorySummaries[0].CategoryScore = nil
	idx := &outputvalidator.PackReferenceIndex{CategoryIds: map[string]struct{}{"10": {}}}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if report.Valid {
		t.Fatal("expected invalid when a category_summaries entry is missing category_score")
	}
	if !hasCode(report, outputvalidator.CodeIncompleteCategorySummary) {
		t.Fatalf("expected %s among errors, got %+v", outputvalidator.CodeIncompleteCategorySummary, report.Errors)
	}
}

func TestValidateCategorySummariesOKWhenComplete(t *testing.T) {
	out := noActionsMaterialFixture(t)
	idx := &outputvalidator.PackReferenceIndex{CategoryIds: map[string]struct{}{"10": {}, "20": {}}}
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, idx)
	if !report.Valid {
		t.Fatalf("expected valid, errors=%+v", report.Errors)
	}
}

func TestValidatePlaceholderTextRussianPhraseRejected(t *testing.T) {
	out := noActionsMaterialFixture(t)
	out.Dashboard.AccountSummary.ExecutiveSummary = "Нужно провести дополнительный анализ по этому SKU"
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{PackID: out.AnalysisMeta.PackID, MetricDate: out.AnalysisMeta.MetricDate}, nil)
	if report.Valid {
		t.Fatal("expected invalid when executive_summary contains a forbidden Russian placeholder phrase")
	}
	if !hasCode(report, outputvalidator.CodePlaceholderText) {
		t.Fatalf("expected %s among errors, got %+v", outputvalidator.CodePlaceholderText, report.Errors)
	}
}

func TestCanonicalHashStable(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	a, err := outputvalidator.CanonicalJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	b, err := outputvalidator.CanonicalJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if outputvalidator.OutputHash(a) != outputvalidator.OutputHash(b) {
		t.Fatal("hash not stable")
	}
}

func cloneValidMaterial(t *testing.T) *contract.UnifiedAnalysisOutputV1 {
	t.Helper()
	return loadFixture(t, "unified_output_v1.valid.json")
}

func TestValidateMaterialActionsCountBounds(t *testing.T) {
	meta := func(out *contract.UnifiedAnalysisOutputV1) outputvalidator.ExpectedRunMeta {
		return outputvalidator.ExpectedRunMeta{
			PackID: out.AnalysisMeta.PackID, PackHash: out.AnalysisMeta.PackHash, MetricDate: out.AnalysisMeta.MetricDate,
		}
	}

	t.Run("zero_actions_blocked", func(t *testing.T) {
		out := cloneValidMaterial(t)
		out.Actions = nil
		out.Dashboard.TopActionIDs = nil
		out.Dashboard.RecommendedActionIDs = nil
		report := outputvalidator.Validate(out, meta(out), nil)
		if report.Valid || !hasCode(report, outputvalidator.CodeActionsCountBelowMinimum) {
			t.Fatalf("want %s, errors=%+v", outputvalidator.CodeActionsCountBelowMinimum, report.Errors)
		}
	})

	t.Run("four_actions_with_required_pass", func(t *testing.T) {
		// Acceptance run 191 emitted 4 real Actions; after the min-5 floor
		// was removed this must no longer block publication on count alone.
		out := cloneValidMaterial(t)
		out.Actions = out.Actions[:4]
		out.Dashboard.TopActionIDs = []string{"a1", "a2", "a3"}
		report := outputvalidator.Validate(out, meta(out), nil)
		if hasCode(report, outputvalidator.CodeActionsCountBelowMinimum) {
			t.Fatalf("4 actions must not trigger %s: %+v", outputvalidator.CodeActionsCountBelowMinimum, report.Errors)
		}
		if hasCode(report, outputvalidator.CodeRequiredActionsCountBelowMinimum) {
			t.Fatalf("3 required among 4 must not trigger required floor: %+v", report.Errors)
		}
	})

	t.Run("two_actions_one_required_pass", func(t *testing.T) {
		out := cloneValidMaterial(t)
		out.Actions = out.Actions[:2]
		out.Actions[0].ExecutionTier = contract.ExecutionTierRequired
		out.Actions[1].ExecutionTier = contract.ExecutionTierRecommended
		out.Dashboard.TopActionIDs = []string{"a1"}
		out.Dashboard.RecommendedActionIDs = []string{"a2"}
		report := outputvalidator.Validate(out, meta(out), nil)
		if hasCode(report, outputvalidator.CodeActionsCountBelowMinimum) {
			t.Fatalf("2 actions must not trigger min floor: %+v", report.Errors)
		}
		if hasCode(report, outputvalidator.CodeRequiredActionsCountBelowMinimum) {
			t.Fatalf("1 required must satisfy MinRequiredActionsMaterialV1=1: %+v", report.Errors)
		}
	})

	t.Run("five_with_three_required_pass", func(t *testing.T) {
		out := cloneValidMaterial(t)
		if len(out.Actions) != 5 {
			t.Fatalf("fixture actions=%d want 5", len(out.Actions))
		}
		report := outputvalidator.Validate(out, meta(out), nil)
		if !report.Valid {
			t.Fatalf("expected valid, errors=%+v", report.Errors)
		}
	})

	t.Run("fifteen_pass", func(t *testing.T) {
		out := cloneValidMaterial(t)
		base := append([]contract.UnifiedActionV1(nil), out.Actions...)
		out.Actions = out.Actions[:5]
		for i := 5; i < 15; i++ {
			a := base[i%len(base)]
			a.AnalysisActionID = "extra_" + string(rune('a'+i-5))
			a.OperationalGoal = a.OperationalGoal + " variant " + a.AnalysisActionID
			a.Scope.ScopeType = "account"
			a.Scope.EntityIDs = nil
			a.ExecutionTier = contract.ExecutionTierRecommended
			out.Actions = append(out.Actions, a)
		}
		report := outputvalidator.Validate(out, meta(out), nil)
		if !report.Valid {
			t.Fatalf("expected 15 actions valid, errors=%+v", report.Errors)
		}
	})

	t.Run("sixteen_blocked", func(t *testing.T) {
		out := cloneValidMaterial(t)
		base := out.Actions[0]
		for i := 0; i < 11; i++ {
			a := base
			a.AnalysisActionID = "pad_" + string(rune('a'+i))
			a.OperationalGoal = a.OperationalGoal + " " + a.AnalysisActionID
			a.Scope.ScopeType = "account"
			a.Scope.EntityIDs = nil
			a.ExecutionTier = contract.ExecutionTierRecommended
			out.Actions = append(out.Actions, a)
		}
		if len(out.Actions) != 16 {
			t.Fatalf("actions=%d want 16", len(out.Actions))
		}
		report := outputvalidator.Validate(out, meta(out), nil)
		if report.Valid || !hasCode(report, outputvalidator.CodeOutputActionLimitExceeded) {
			t.Fatalf("want %s, errors=%+v", outputvalidator.CodeOutputActionLimitExceeded, report.Errors)
		}
	})

	t.Run("zero_required_blocked", func(t *testing.T) {
		out := cloneValidMaterial(t)
		for i := range out.Actions {
			out.Actions[i].ExecutionTier = contract.ExecutionTierRecommended
		}
		out.Dashboard.TopActionIDs = nil
		report := outputvalidator.Validate(out, meta(out), nil)
		if report.Valid || !hasCode(report, outputvalidator.CodeRequiredActionsCountBelowMinimum) {
			t.Fatalf("want %s, errors=%+v", outputvalidator.CodeRequiredActionsCountBelowMinimum, report.Errors)
		}
	})

	t.Run("duplicate_fingerprint_blocked", func(t *testing.T) {
		out := cloneValidMaterial(t)
		out.Actions[4].Type = out.Actions[3].Type
		out.Actions[4].OperationalGoal = out.Actions[3].OperationalGoal
		out.Actions[4].Scope.EntityIDs = append([]string{}, out.Actions[3].Scope.EntityIDs...)
		report := outputvalidator.Validate(out, meta(out), nil)
		if report.Valid || !hasCode(report, outputvalidator.CodeActionDuplicate) {
			t.Fatalf("want %s, errors=%+v", outputvalidator.CodeActionDuplicate, report.Errors)
		}
	})

	t.Run("missing_health_blocked", func(t *testing.T) {
		out := cloneValidMaterial(t)
		out.Dashboard.AccountSummary.HealthScore = nil
		report := outputvalidator.Validate(out, meta(out), nil)
		if report.Valid || !hasCode(report, outputvalidator.CodeMissingHealthScore) {
			t.Fatalf("want %s, errors=%+v", outputvalidator.CodeMissingHealthScore, report.Errors)
		}
	})

	t.Run("health_score_83_preserved", func(t *testing.T) {
		out := cloneValidMaterial(t)
		score := int32(83)
		out.Dashboard.AccountSummary.HealthScore = &score
		out.Dashboard.AccountSummary.Severity = "low" // 80–100 band
		report := outputvalidator.Validate(out, meta(out), nil)
		if !report.Valid {
			t.Fatalf("expected valid with health_score 83, errors=%+v", report.Errors)
		}
		if out.Dashboard.AccountSummary.HealthScore == nil || *out.Dashboard.AccountSummary.HealthScore != 83 {
			t.Fatalf("health_score mutated: %v", out.Dashboard.AccountSummary.HealthScore)
		}
	})
}
