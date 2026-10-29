package publication

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

// Source identifies which pipeline is authoritative for a seller's current
// dashboard/actions data.
const (
	SourceLegacy  = "legacy"
	SourceUnified = "unified"
	// SourceNone means neither pipeline has anything publishable yet (brand
	// new seller, or unified-only seller with legacy fallback disabled and
	// no unified publication yet).
	SourceNone = "none"
)

// SourceResult reports the outcome of Service.ResolveCurrentAnalysisSource.
type SourceResult struct {
	// Source is one of SourceLegacy, SourceUnified, SourceNone.
	Source string
	// PublicationID is the unified_analysis_publications.id backing this
	// result. Zero unless Source == SourceUnified.
	PublicationID int64
	// PipelineRunID is the ai_pipeline_runs.id backing this result: the
	// unified run when Source == SourceUnified, the legacy pipeline run when
	// Source == SourceLegacy. Zero when Source == SourceNone.
	PipelineRunID int64
	// MetricDate is the resolved source's metric_date (YYYY-MM-DD), empty
	// when Source == SourceNone.
	MetricDate string
	// FallbackReason explains why legacy was used instead of unified, or why
	// no source is available at all. Empty when unified was used, or when
	// the seller is simply configured for plain legacy mode.
	FallbackReason string
}

// ResolveCurrentAnalysisSource decides whether a seller's current
// dashboard/actions data should be read from the legacy pipeline or the
// Stage 5 unified publication pointer (seller_current_unified_publications).
//
// Rules (spec section 58):
//  1. Sellers whose effective analysis mode is not "unified" (legacy or
//     shadow) always resolve to legacy: shadow-mode unified runs exist for
//     comparison/evaluation only and are never visibility=visible, so they
//     can never be the seller-facing source of truth.
//  2. Sellers whose effective mode is "unified" resolve to their current
//     unified publication when one exists and is published.
//  3. If no unified publication exists yet (first run still pending/not yet
//     published), fall back to legacy when the effective legacy-fallback
//     flag allows it; otherwise report SourceNone so callers can surface an
//     explicit "analysis not ready yet" state rather than serving stale
//     legacy data to a seller who opted into unified-only.
func (s *Service) ResolveCurrentAnalysisSource(ctx context.Context, sellerAccountID int64) (SourceResult, error) {
	if s == nil || s.queries == nil {
		return SourceResult{}, NewError(CodeSelectorInconsistentState, "publication service is not configured")
	}

	var settingsPtr *dbgen.SellerAnalysisSetting
	row, err := s.queries.GetSellerAnalysisSettings(ctx, sellerAccountID)
	if err == nil {
		settingsPtr = &row
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SourceResult{}, WrapError(CodeSelectorInconsistentState, "load seller analysis settings", err)
	}
	settings := EffectiveSettings(settingsPtr, s.global)

	// Kill switch: when global rollout is off, never surface Unified as the
	// seller-facing read source — even if analysis_mode=unified and a
	// publication pointer exists. Auto-run / auto-publish flags do NOT affect
	// read path once rollout is on.
	if !s.global.RolloutEnabled {
		return s.resolveLegacy(ctx, sellerAccountID, "unified_rollout_disabled")
	}

	if settings.Visibility != VisibilityVisible {
		reason := ""
		if settings.Mode == AnalysisModeShadow {
			reason = "seller_mode_is_shadow_not_unified"
		}
		return s.resolveLegacy(ctx, sellerAccountID, reason)
	}

	res, found, err := s.resolveUnified(ctx, sellerAccountID)
	if err != nil {
		return SourceResult{}, err
	}
	if found {
		return res, nil
	}
	// Invalid current pointer (wrong status/visibility) is an integrity signal.
	// Fall back to legacy only when fallback is allowed; otherwise surface SourceNone
	// with FallbackReason=unified_current_publication_invalid.
	if res.FallbackReason == "unified_current_publication_invalid" {
		if !s.global.LegacyFallbackEnabled || !settings.LegacyFallbackEnabled {
			return res, nil
		}
		return s.resolveLegacy(ctx, sellerAccountID, res.FallbackReason)
	}

	if !s.global.LegacyFallbackEnabled || !settings.LegacyFallbackEnabled {
		return SourceResult{Source: SourceNone, FallbackReason: "unified_publication_not_found_and_legacy_fallback_disabled"}, nil
	}
	return s.resolveLegacy(ctx, sellerAccountID, "unified_publication_not_found")
}

// resolveUnified loads the seller's current unified publication pointer, if
// any. found is false (with a nil error) both when no row exists yet and
// when the pointed-at publication is not (or no longer) published+visible.
// Invalid pointer status is reported via FallbackReason when falling back.
func (s *Service) resolveUnified(ctx context.Context, sellerAccountID int64) (SourceResult, bool, error) {
	row, err := s.queries.GetSellerCurrentUnifiedPublication(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SourceResult{}, false, nil
		}
		return SourceResult{}, false, WrapError(CodeSelectorInconsistentState, "load seller current unified publication", err)
	}
	if Status(row.Status) != StatusPublished {
		return SourceResult{
			Source:         SourceNone,
			PublicationID:  row.PublicationID,
			PipelineRunID:  row.RunID,
			MetricDate:     row.MetricDate.Time.UTC().Format("2006-01-02"),
			FallbackReason: "unified_current_publication_invalid",
		}, false, nil
	}
	if Visibility(row.Visibility) != VisibilityVisible {
		return SourceResult{
			Source:         SourceNone,
			PublicationID:  row.PublicationID,
			PipelineRunID:  row.RunID,
			MetricDate:     row.MetricDate.Time.UTC().Format("2006-01-02"),
			FallbackReason: "unified_current_publication_invalid",
		}, false, nil
	}
	return SourceResult{
		Source:        SourceUnified,
		PublicationID: row.PublicationID,
		PipelineRunID: row.RunID,
		MetricDate:    row.MetricDate.Time.UTC().Format("2006-01-02"),
	}, true, nil
}

// resolveLegacy loads the seller's current legacy pipeline pointer
// (seller_current_ai_snapshots, via GetSellerCurrentPipelineRunID).
func (s *Service) resolveLegacy(ctx context.Context, sellerAccountID int64, reason string) (SourceResult, error) {
	pipelineRunID, err := s.queries.GetSellerCurrentPipelineRunID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SourceResult{Source: SourceNone, FallbackReason: joinReason(reason, "no_legacy_pointer")}, nil
		}
		return SourceResult{}, WrapError(CodeSelectorInconsistentState, "load seller current legacy pipeline run id", err)
	}
	metricDate := ""
	if run, runErr := s.queries.GetAIPipelineRun(ctx, pipelineRunID); runErr == nil {
		metricDate = run.MetricDate.Time.UTC().Format("2006-01-02")
	}
	return SourceResult{
		Source:         SourceLegacy,
		PipelineRunID:  pipelineRunID,
		MetricDate:     metricDate,
		FallbackReason: reason,
	}, nil
}

func joinReason(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "; " + b
	}
}
