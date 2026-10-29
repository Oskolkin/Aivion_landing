package publication_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
)

func TestPublishRequest_PublishAsCurrentSemantics(t *testing.T) {
	trueVal := true
	falseVal := false

	reqTrue := publication.PublishRequest{RunID: 1, PublishAsCurrent: &trueVal}
	if reqTrue.PublishAsCurrent == nil || !*reqTrue.PublishAsCurrent {
		t.Fatal("expected publish_as_current=true")
	}
	reqFalse := publication.PublishRequest{RunID: 1, PublishAsCurrent: &falseVal}
	if reqFalse.PublishAsCurrent == nil || *reqFalse.PublishAsCurrent {
		t.Fatal("expected publish_as_current=false")
	}
}

func TestApplyDateFreshness_StaleNote(t *testing.T) {
	proj := adapter.DashboardProjection{}
	proj.Frozen.MetricDate = "2026-07-08"
	adapter.ApplyDateFreshness(&proj, analysisdata.MetricDateResolution{
		RequestedDate:          "2026-07-17",
		EffectiveMetricDate:    "2026-07-08",
		LatestCompleteDataDate: "2026-07-08",
		FreshnessLagDays:       9,
		IsStale:                true,
	})
	if proj.Frozen.DataFreshnessLagDays != 9 || !proj.Frozen.DataIsStale {
		t.Fatalf("freshness not applied: %+v", proj.Frozen)
	}
	if len(proj.Narrative.DataQualityNotes) == 0 {
		t.Fatal("expected stale note")
	}
}

// TestApplyDateFreshness_HistoricalAnalysisNoStaleNoteFromAgeAlone verifies
// that a deliberate fixed-date historical analysis, where the requested
// analysis date is far in the past but the underlying SOURCE data for that
// date is fully fresh/complete (not stale), never gets a "Данные не
// обновлялись N дн." note purely because the analysis itself is old. It
// should instead get the neutral historical-analysis note.
func TestApplyDateFreshness_HistoricalAnalysisNoStaleNoteFromAgeAlone(t *testing.T) {
	proj := adapter.DashboardProjection{}
	proj.Frozen.MetricDate = "2026-01-01"
	adapter.ApplyDateFreshness(&proj, analysisdata.MetricDateResolution{
		DateMode:               analysisdata.DateModeFixed,
		RequestedDate:          "2026-01-01",
		EffectiveMetricDate:    "2026-01-01",
		LatestCompleteDataDate: "2026-07-17", // sources are current; analysis date is intentionally old
		HistoricalAnalysis:     true,
		AnalysisAgeDays:        197, // large gap vs "today" (2026-07-17) — expected for a fixed historical run.
		SourceFreshnessLagDays: 0,
		FreshnessLagDays:       0,
		IsStale:                false,
	})
	if proj.Frozen.DataIsStale {
		t.Fatalf("expected DataIsStale=false for a historical analysis whose sources are fresh, got %+v", proj.Frozen)
	}
	for _, note := range proj.Narrative.DataQualityNotes {
		if note == "Данные не обновлялись 197 дн." {
			t.Fatalf("historical analysis must not surface a stale note derived from analysis age alone: %+v", proj.Narrative.DataQualityNotes)
		}
	}
	found := false
	for _, note := range proj.Narrative.DataQualityNotes {
		if note == "Исторический анализ по данным на 01.01.2026" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected historical analysis note, got %+v", proj.Narrative.DataQualityNotes)
	}
}

// TestApplyDateFreshness_HistoricalAnalysisWithGenuinelyStaleSources verifies
// that even for a fixed historical analysis, a genuine SOURCE staleness
// signal (SourceFreshnessLagDays/IsStale) still surfaces the staleness note
// — historical framing must not mask real source staleness.
func TestApplyDateFreshness_HistoricalAnalysisWithGenuinelyStaleSources(t *testing.T) {
	proj := adapter.DashboardProjection{}
	proj.Frozen.MetricDate = "2026-01-01"
	adapter.ApplyDateFreshness(&proj, analysisdata.MetricDateResolution{
		DateMode:               analysisdata.DateModeFixed,
		RequestedDate:          "2026-01-01",
		EffectiveMetricDate:    "2026-01-01",
		LatestCompleteDataDate: "2025-12-20",
		HistoricalAnalysis:     true,
		AnalysisAgeDays:        197,
		SourceFreshnessLagDays: 12,
		FreshnessLagDays:       12,
		IsStale:                true,
	})
	found := false
	for _, note := range proj.Narrative.DataQualityNotes {
		if note == "Данные не обновлялись 12 дн." {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected genuine source staleness note even for a historical analysis, got %+v", proj.Narrative.DataQualityNotes)
	}
}
