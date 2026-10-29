// Package analysissource resolves which pipeline (unified or legacy) is
// authoritative for a seller's current seller-facing dashboard/actions data,
// and exposes the identifiers (publication/run/metric date) both read paths
// must share so Dashboard and Actions always render the same bundle.
//
// This package is intentionally neutral: it must not import dashboardhome,
// actions, unifiedanalysis/publication/adapter, or any HTTP package, so both
// dashboardhome and actions can depend on it without creating an import
// cycle (unifiedanalysis/publication itself imports dashboardhome/actions).
package analysissource

import "context"

// Source identifies which pipeline is authoritative for a seller's current
// dashboard/actions data.
const (
	SourceUnified = "unified"
	SourceLegacy  = "legacy"
	// SourceNone means neither pipeline has anything publishable yet (brand
	// new seller, or unified-only seller with legacy fallback disabled and
	// no unified publication yet).
	SourceNone = "none"
)

// CurrentAnalysisSource is the authoritative read-path selection result for
// a seller: which pipeline to read from, and the identifiers Dashboard and
// Actions must both stamp on their responses so they can be shown to be
// reading the exact same bundle.
type CurrentAnalysisSource struct {
	// Source is one of SourceUnified, SourceLegacy, SourceNone.
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
	// FallbackUsed is true when the seller's configured/intended source
	// could not be used as-is and a different source (or no source) was
	// resolved instead — e.g. unified rollout disabled, seller in shadow
	// mode, or the unified publication pointer missing/invalid.
	FallbackUsed bool
	// FallbackReason explains why legacy was used instead of unified, or why
	// no source is available at all. Empty when the seller's primary
	// configured source was used without any override.
	FallbackReason string
	// DashboardContentStatus is an optional diagnostic the dashboard read
	// path may fill in after resolving (e.g. "ready", "partial") — callers
	// that only need Source/PublicationID/PipelineRunID can leave it empty.
	DashboardContentStatus string
}

// Resolver decides, for a given seller, which pipeline is authoritative for
// their current seller-facing dashboard/actions bundle.
type Resolver interface {
	ResolveCurrent(ctx context.Context, sellerAccountID int64) (CurrentAnalysisSource, error)
}
