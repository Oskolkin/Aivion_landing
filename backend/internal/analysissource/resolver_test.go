package analysissource

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeQueries struct {
	settings    *dbgen.SellerAnalysisSetting
	settingsErr error

	unifiedRow dbgen.GetSellerCurrentUnifiedPublicationRow
	unifiedErr error

	legacyRunID int64
	legacyErr   error
	pipelineRun dbgen.AiPipelineRun
	pipelineErr error
}

func (f *fakeQueries) GetSellerAnalysisSettings(ctx context.Context, sellerAccountID int64) (dbgen.SellerAnalysisSetting, error) {
	if f.settingsErr != nil {
		return dbgen.SellerAnalysisSetting{}, f.settingsErr
	}
	if f.settings == nil {
		return dbgen.SellerAnalysisSetting{}, pgx.ErrNoRows
	}
	return *f.settings, nil
}

func (f *fakeQueries) GetSellerCurrentUnifiedPublication(ctx context.Context, sellerAccountID int64) (dbgen.GetSellerCurrentUnifiedPublicationRow, error) {
	if f.unifiedErr != nil {
		return dbgen.GetSellerCurrentUnifiedPublicationRow{}, f.unifiedErr
	}
	return f.unifiedRow, nil
}

func (f *fakeQueries) GetSellerCurrentPipelineRunID(ctx context.Context, sellerAccountID int64) (int64, error) {
	if f.legacyErr != nil {
		return 0, f.legacyErr
	}
	return f.legacyRunID, nil
}

func (f *fakeQueries) GetAIPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	if f.pipelineErr != nil {
		return dbgen.AiPipelineRun{}, f.pipelineErr
	}
	return f.pipelineRun, nil
}

func metricDate(s string) pgtype.Date {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return pgtype.Date{Time: t, Valid: true}
}

func TestResolveCurrent_PlainLegacyMode(t *testing.T) {
	q := &fakeQueries{
		settings:    &dbgen.SellerAnalysisSetting{AnalysisMode: "legacy", LegacyFallbackEnabled: true},
		legacyRunID: 42,
		pipelineRun: dbgen.AiPipelineRun{ID: 42, MetricDate: metricDate("2026-07-10")},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy || got.PipelineRunID != 42 || got.FallbackUsed {
		t.Fatalf("got %+v", got)
	}
	if got.MetricDate != "2026-07-10" {
		t.Fatalf("metric_date=%q", got.MetricDate)
	}
}

func TestResolveCurrent_ShadowModeAlwaysLegacy(t *testing.T) {
	q := &fakeQueries{
		settings:    &dbgen.SellerAnalysisSetting{AnalysisMode: "shadow", LegacyFallbackEnabled: true},
		legacyRunID: 7,
		pipelineRun: dbgen.AiPipelineRun{ID: 7, MetricDate: metricDate("2026-07-11")},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy {
		t.Fatalf("shadow mode must never resolve to unified, got %+v", got)
	}
	if !got.FallbackUsed || got.FallbackReason != "seller_mode_is_shadow_not_unified" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_UnifiedModeRolloutOnPublishedVisible(t *testing.T) {
	q := &fakeQueries{
		settings: &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: true},
		unifiedRow: dbgen.GetSellerCurrentUnifiedPublicationRow{
			PublicationID: 100,
			RunID:         200,
			Status:        "published",
			Visibility:    "visible",
			MetricDate:    metricDate("2026-07-12"),
		},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceUnified || got.PublicationID != 100 || got.PipelineRunID != 200 || got.FallbackUsed {
		t.Fatalf("got %+v", got)
	}
	if got.MetricDate != "2026-07-12" {
		t.Fatalf("metric_date=%q", got.MetricDate)
	}
}

func TestResolveCurrent_RolloutOffForcesLegacyEvenForUnifiedSeller(t *testing.T) {
	q := &fakeQueries{
		settings: &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: true},
		unifiedRow: dbgen.GetSellerCurrentUnifiedPublicationRow{
			PublicationID: 100, RunID: 200, Status: "published", Visibility: "visible",
		},
		legacyRunID: 9,
		pipelineRun: dbgen.AiPipelineRun{ID: 9},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: false, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy || got.PipelineRunID != 9 {
		t.Fatalf("got %+v", got)
	}
	if !got.FallbackUsed || got.FallbackReason != "unified_rollout_disabled" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_UnifiedPublicationNotFoundFallsBackToLegacy(t *testing.T) {
	q := &fakeQueries{
		settings:    &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: true},
		unifiedErr:  pgx.ErrNoRows,
		legacyRunID: 55,
		pipelineRun: dbgen.AiPipelineRun{ID: 55},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy || got.PipelineRunID != 55 {
		t.Fatalf("got %+v", got)
	}
	if got.FallbackReason != "unified_publication_not_found" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_UnifiedPublicationNotFoundNoFallbackAllowedYieldsNone(t *testing.T) {
	q := &fakeQueries{
		settings:   &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: false},
		unifiedErr: pgx.ErrNoRows,
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceNone {
		t.Fatalf("got %+v", got)
	}
	if got.FallbackReason != "unified_publication_not_found_and_legacy_fallback_disabled" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_InvalidUnifiedPointerFallsBackToLegacy(t *testing.T) {
	q := &fakeQueries{
		settings: &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: true},
		unifiedRow: dbgen.GetSellerCurrentUnifiedPublicationRow{
			PublicationID: 100, RunID: 200, Status: "publishing", Visibility: "visible",
		},
		legacyRunID: 3,
		pipelineRun: dbgen.AiPipelineRun{ID: 3},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy || got.PipelineRunID != 3 {
		t.Fatalf("got %+v", got)
	}
	if got.FallbackReason != "unified_current_publication_invalid" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_InvalidUnifiedPointerNoFallbackAllowedYieldsNone(t *testing.T) {
	q := &fakeQueries{
		settings: &dbgen.SellerAnalysisSetting{AnalysisMode: "unified", LegacyFallbackEnabled: true},
		unifiedRow: dbgen.GetSellerCurrentUnifiedPublicationRow{
			PublicationID: 100, RunID: 200, Status: "published", Visibility: "shadow",
		},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: false})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceNone || got.PublicationID != 100 {
		t.Fatalf("got %+v", got)
	}
	if got.FallbackReason != "unified_current_publication_invalid" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_NoSettingsRowDefaultsToLegacy(t *testing.T) {
	q := &fakeQueries{
		legacyRunID: 11,
		pipelineRun: dbgen.AiPipelineRun{ID: 11},
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceLegacy || got.FallbackUsed {
		t.Fatalf("unconfigured seller must be plain legacy (no fallback flag), got %+v", got)
	}
}

func TestResolveCurrent_NoLegacyPointerYieldsNone(t *testing.T) {
	q := &fakeQueries{
		settings:  &dbgen.SellerAnalysisSetting{AnalysisMode: "legacy", LegacyFallbackEnabled: true},
		legacyErr: pgx.ErrNoRows,
	}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	got, err := r.ResolveCurrent(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != SourceNone {
		t.Fatalf("got %+v", got)
	}
	if got.FallbackReason != "no_legacy_pointer" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveCurrent_PropagatesUnexpectedSettingsError(t *testing.T) {
	q := &fakeQueries{settingsErr: errors.New("boom")}
	r := NewResolver(q, GlobalFlags{RolloutEnabled: true, LegacyFallbackEnabled: true})
	if _, err := r.ResolveCurrent(context.Background(), 1); err == nil {
		t.Fatal("expected error to propagate")
	}
}
