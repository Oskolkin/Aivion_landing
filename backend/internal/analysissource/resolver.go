package analysissource

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

// Queries is the minimal set of generated queries SQLCResolver needs. It is
// satisfied by *dbgen.Queries; defining it locally (rather than depending on
// the concrete type everywhere) keeps ResolveCurrent's DB access seam
// mockable in unit tests without a live database.
type Queries interface {
	GetSellerAnalysisSettings(ctx context.Context, sellerAccountID int64) (dbgen.SellerAnalysisSetting, error)
	GetSellerCurrentUnifiedPublication(ctx context.Context, sellerAccountID int64) (dbgen.GetSellerCurrentUnifiedPublicationRow, error)
	GetSellerCurrentPipelineRunID(ctx context.Context, sellerAccountID int64) (int64, error)
	GetAIPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error)
}

// SQLCResolver implements Resolver against generated sqlc queries.
type SQLCResolver struct {
	q      Queries
	global GlobalFlags
}

// NewResolver builds a Resolver backed by q (typically dbgen.New(pool)) and
// the given global rollout flags (typically GlobalFlagsFromEnv()).
func NewResolver(q Queries, global GlobalFlags) *SQLCResolver {
	return &SQLCResolver{q: q, global: global}
}

// ResolveCurrent decides whether a seller's current dashboard/actions data
// should be read from the unified publication pointer
// (seller_current_unified_publications) or the legacy pipeline pointer
// (seller_current_ai_snapshots).
//
// Rules:
//  1. Sellers whose effective analysis mode is not "unified" (legacy or
//     shadow) always resolve to legacy: shadow-mode unified runs exist for
//     comparison/evaluation only and are never visibility=visible, so they
//     can never be the seller-facing source of truth.
//  2. Sellers whose effective mode is "unified" resolve to their current
//     unified publication when one exists, is published, and is visible.
//  3. If no unified publication exists yet (or the pointer is invalid), fall
//     back to legacy when the effective legacy-fallback flag allows it;
//     otherwise report SourceNone so callers can surface an explicit
//     "analysis not ready yet" state rather than serving stale legacy data
//     to a seller who opted into unified-only.
//  4. Global rollout kill switch and auto_run/auto_publish flags never
//     affect this read path: they only gate whether new runs are scheduled
//     or published, not whether already-published data may be read. Only
//     RolloutEnabled=false forces legacy, matching the "never surface
//     unified when rollout is off" kill-switch contract.
func (r *SQLCResolver) ResolveCurrent(ctx context.Context, sellerAccountID int64) (CurrentAnalysisSource, error) {
	if r == nil || r.q == nil {
		return CurrentAnalysisSource{}, errors.New("analysissource: resolver is not configured")
	}

	var settingsRow *dbgen.SellerAnalysisSetting
	row, err := r.q.GetSellerAnalysisSettings(ctx, sellerAccountID)
	if err == nil {
		settingsRow = &row
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CurrentAnalysisSource{}, fmt.Errorf("analysissource: load seller analysis settings: %w", err)
	}
	settings := resolveEffectiveSettings(settingsRow)

	if !r.global.RolloutEnabled {
		return r.resolveLegacy(ctx, sellerAccountID, "unified_rollout_disabled")
	}

	if settings.Visibility != visibilityVisible {
		reason := ""
		if settings.Mode == analysisModeShadow {
			reason = "seller_mode_is_shadow_not_unified"
		}
		return r.resolveLegacy(ctx, sellerAccountID, reason)
	}

	res, found, err := r.resolveUnified(ctx, sellerAccountID)
	if err != nil {
		return CurrentAnalysisSource{}, err
	}
	if found {
		return res, nil
	}
	// Invalid current pointer (wrong status/visibility) is an integrity signal.
	// Fall back to legacy only when fallback is allowed; otherwise surface
	// SourceNone with FallbackReason=unified_current_publication_invalid.
	if res.FallbackReason == "unified_current_publication_invalid" {
		if !r.global.LegacyFallbackEnabled || !settings.LegacyFallbackEnabled {
			return res, nil
		}
		return r.resolveLegacy(ctx, sellerAccountID, res.FallbackReason)
	}

	if !r.global.LegacyFallbackEnabled || !settings.LegacyFallbackEnabled {
		return CurrentAnalysisSource{Source: SourceNone, FallbackReason: "unified_publication_not_found_and_legacy_fallback_disabled"}, nil
	}
	return r.resolveLegacy(ctx, sellerAccountID, "unified_publication_not_found")
}

// resolveUnified loads the seller's current unified publication pointer, if
// any. found is false (with a nil error) both when no row exists yet and
// when the pointed-at publication is not (or no longer) published+visible.
func (r *SQLCResolver) resolveUnified(ctx context.Context, sellerAccountID int64) (CurrentAnalysisSource, bool, error) {
	row, err := r.q.GetSellerCurrentUnifiedPublication(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CurrentAnalysisSource{}, false, nil
		}
		return CurrentAnalysisSource{}, false, fmt.Errorf("analysissource: load seller current unified publication: %w", err)
	}
	metricDate := ""
	if row.MetricDate.Valid {
		metricDate = row.MetricDate.Time.UTC().Format("2006-01-02")
	}
	if row.Status != publicationStatusPublished || row.Visibility != visibilityVisible {
		return CurrentAnalysisSource{
			Source:         SourceNone,
			PublicationID:  row.PublicationID,
			PipelineRunID:  row.RunID,
			MetricDate:     metricDate,
			FallbackReason: "unified_current_publication_invalid",
		}, false, nil
	}
	return CurrentAnalysisSource{
		Source:        SourceUnified,
		PublicationID: row.PublicationID,
		PipelineRunID: row.RunID,
		MetricDate:    metricDate,
	}, true, nil
}

// resolveLegacy loads the seller's current legacy pipeline pointer
// (seller_current_ai_snapshots, via GetSellerCurrentPipelineRunID). reason
// is non-empty whenever legacy is being used as a substitute for the
// seller's intended/configured source (i.e. a real fallback); it is empty
// for sellers who are simply configured for plain legacy mode.
func (r *SQLCResolver) resolveLegacy(ctx context.Context, sellerAccountID int64, reason string) (CurrentAnalysisSource, error) {
	pipelineRunID, err := r.q.GetSellerCurrentPipelineRunID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CurrentAnalysisSource{
				Source:         SourceNone,
				FallbackUsed:   reason != "",
				FallbackReason: joinReason(reason, "no_legacy_pointer"),
			}, nil
		}
		return CurrentAnalysisSource{}, fmt.Errorf("analysissource: load seller current legacy pipeline run id: %w", err)
	}
	metricDate := ""
	if run, runErr := r.q.GetAIPipelineRun(ctx, pipelineRunID); runErr == nil && run.MetricDate.Valid {
		metricDate = run.MetricDate.Time.UTC().Format("2006-01-02")
	}
	return CurrentAnalysisSource{
		Source:         SourceLegacy,
		PipelineRunID:  pipelineRunID,
		MetricDate:     metricDate,
		FallbackUsed:   reason != "",
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
