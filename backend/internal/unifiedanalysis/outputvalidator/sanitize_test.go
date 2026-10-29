package outputvalidator_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func TestSanitizeGroundingFixesMechanicalFailures(t *testing.T) {
	out := loadFixture(t, "unified_output_v1.valid.json")
	out.AnalysisMeta.PackHash = "wrong-hash"
	out.AnalysisMeta.PackID = "wrong-pack"
	out.AnalysisMeta.MetricDate = "2000-01-01"

	// Duplicate SKU across two actions + one invented SKU.
	out.Actions = out.Actions[:2]
	out.Actions[0].Scope.ScopeType = "sku"
	out.Actions[0].Scope.EntityIDs = []string{"801000100002", "999999999999"}
	out.Actions[1].Scope.ScopeType = "sku"
	out.Actions[1].Scope.EntityIDs = []string{"801000100002", "801000100005"}

	base := 5.0
	target := 3.0
	period := 7
	out.Actions[0].CompletionConditions = []contract.UnifiedCompletionConditionV1{{
		MetricField:    "sku_count",
		Operator:       ">=",
		TargetValue:    &target,
		BaselineValue:  &base,
		BaselineFactID: "account:orders_count:current_7d",
		PeriodDays:     &period,
	}}

	// Duplicate signal_id across what_changed groups.
	if len(out.Dashboard.WhatChanged.Groups) >= 2 &&
		len(out.Dashboard.WhatChanged.Groups[0].Items) > 0 &&
		len(out.Dashboard.WhatChanged.Groups[1].Items) > 0 {
		out.Dashboard.WhatChanged.Groups[0].Items[0].SignalID = "shared_signal"
		out.Dashboard.WhatChanged.Groups[1].Items[0].SignalID = "shared_signal"
	}

	meta := outputvalidator.ExpectedRunMeta{
		PackID:     "pack_example_v1",
		PackHash:   "sha256:example",
		MetricDate: "2026-07-08",
	}
	res := outputvalidator.SanitizeGrounding(out, meta, testIndex())
	if !res.PackIdentityForced {
		t.Fatal("expected pack identity force")
	}
	if out.AnalysisMeta.PackID != meta.PackID || out.AnalysisMeta.PackHash != meta.PackHash || out.AnalysisMeta.MetricDate != meta.MetricDate {
		t.Fatalf("analysis_meta not forced: %+v", out.AnalysisMeta)
	}
	if res.UnknownSKUsRemoved < 1 {
		t.Fatalf("expected unknown sku removed, got %+v", res)
	}
	if res.DuplicateSKUsRemoved < 1 {
		t.Fatalf("expected duplicate sku removed, got %+v", res)
	}
	if res.CompletionTargetsRaised < 1 {
		t.Fatalf("expected completion target raised, got %+v", res)
	}
	if out.Actions[0].CompletionConditions[0].TargetValue == nil || *out.Actions[0].CompletionConditions[0].TargetValue <= base {
		t.Fatalf("target not raised beyond baseline: %v", out.Actions[0].CompletionConditions[0].TargetValue)
	}
	if res.SignalIDConflictsCleared < 1 {
		t.Fatalf("expected signal conflict cleared, got %+v", res)
	}

	report := outputvalidator.Validate(out, meta, testIndex())
	// Fixture may still fail other unrelated checks after truncation; assert the
	// mechanical codes we sanitized are gone.
	for _, code := range []string{
		outputvalidator.CodeWrongPackHash,
		outputvalidator.CodeOutputPackIDMismatch,
		outputvalidator.CodeOutputMetricDateMismatch,
		outputvalidator.CodeOutputDuplicateSKUAcrossActions,
		outputvalidator.CodeOutputUnknownSKU,
		outputvalidator.CodeActionCompletionTargetAlreadyMet,
	} {
		if hasCode(report, code) {
			t.Fatalf("sanitized output still has %s: %+v", code, report.Errors)
		}
	}
}
