package publication

import "time"

// PublishRequest is the input to Service.PublishRun.
type PublishRequest struct {
	// RunID is the unified_analysis ai_pipeline_runs.id to publish. Required.
	RunID int64
	// Actor identifies who/what triggered publication (e.g. "system", "admin:<email>",
	// "devtool:run_unified_analysis"). Recorded on the audit trail.
	Actor string
	// AllowNonCurrentPrompt bypasses PublicationPolicy prompt/schema pinning.
	// Reserved for admin/devtools republish of historical runs; regular
	// auto-publish callers must leave this false.
	AllowNonCurrentPrompt bool
	// ForceVisibility overrides the visibility derived from seller settings
	// (shadow mode -> shadow, unified mode -> visible). Reserved for
	// devtools/admin flows that want to force-publish as shadow for QA.
	ForceVisibility *Visibility
	// PublishAsCurrent, when set, controls whether this publication should
	// become the seller's current Dashboard pointer:
	//   true  → ForceVisibility=visible (unless ForceVisibility already set)
	//           and allow replacing a newer metric_date pointer
	//   false → ForceVisibility=shadow (unless ForceVisibility already set)
	PublishAsCurrent *bool
	// AllowOlderMetricDate permits making an older metric_date the current
	// pointer when the seller already has a newer current publication.
	// Used for explicit fixed-mode republish of the true latest-complete date.
	AllowOlderMetricDate bool
}

// PublishOutcome summarizes what PublishRun actually did.
type PublishOutcome string

const (
	// OutcomePublished means a new (or first-time) publication row reached
	// status=published in this call.
	OutcomePublished PublishOutcome = "published"
	// OutcomeAlreadyPublished means the run's publication already existed
	// with status=published; PublishRun is idempotent and returned it as-is.
	OutcomeAlreadyPublished PublishOutcome = "already_published"
	// OutcomeSkippedAnalysisFailed means the validated output's
	// analysis_outcome was analysis_failed; the publication row is recorded
	// as publication_failed and nothing is projected into dashboard/actions.
	OutcomeSkippedAnalysisFailed PublishOutcome = "skipped_analysis_failed"
	// OutcomeStale means a newer/current unified publication already exists
	// for this seller and this run's output would regress it; the publish
	// attempt was rejected without mutating the current pointer.
	OutcomeStale PublishOutcome = "stale"
	// OutcomeRolledBack means Service.RollbackPublication switched the
	// seller's current visible unified publication pointer back onto a
	// previously published TargetPublicationID. The previously-current
	// publication is marked superseded (the StatusRolledBack status is not
	// actually used by this flow — nothing is ever set to "rolled_back";
	// only the pointer moves), and the target itself becomes/stays
	// status=published.
	OutcomeRolledBack PublishOutcome = "rolled_back"
	// OutcomePromoted means Service.PromoteShadowPublication flipped a
	// shadow-visibility publication to visible and made it the seller's
	// current publication.
	OutcomePromoted PublishOutcome = "promoted"
	// OutcomeBundleValidationFailed means bundlevalidator.ValidatePublicationBundle
	// rejected the merged facts+AI bundle (missing/inconsistent factual or
	// narrative content); the publication row is recorded as
	// publication_failed with the specific bundlevalidator error code, and
	// nothing is projected into dashboard/actions — the seller's current
	// pointer is left untouched.
	OutcomeBundleValidationFailed PublishOutcome = "bundle_validation_failed"
)

// PublishResult is the output of Service.PublishRun.
type PublishResult struct {
	Outcome         PublishOutcome
	PublicationID   int64
	RunID           int64
	SellerAccountID int64
	MetricDate      string

	Mode       PublicationMode
	Visibility Visibility
	Status     Status

	AnalysisOutcome string
	OutputHash      string
	BundleHash      string

	DashboardRunID          string
	ActionsRunID            int64
	DashboardDataSnapshotID int64

	DashboardRecordCount    int
	ActionsRecordCount      int
	TopActionsCount         int
	RecommendedActionsCount int

	ErrorCode    string
	ErrorMessage string

	PublishedAt time.Time
}

// Settings is the resolved effective per-seller publication configuration:
// the raw seller_analysis_settings row (or legacy defaults when missing)
// combined with the deployment-wide GlobalFlags.
type Settings struct {
	// Mode is the seller's configured rollout mode (legacy/shadow/unified).
	Mode AnalysisMode

	// Raw per-seller flags as stored on seller_analysis_settings. Meaningless
	// on their own — always combine with EffectiveAutoRun/EffectiveAutoPublish.
	AutoRunEnabled        bool
	AutoPublishEnabled    bool
	LegacyFallbackEnabled bool

	// EffectiveAutoRun reports whether the orchestrator may automatically
	// schedule a unified analysis run for this seller: requires the global
	// rollout+auto-run switches, the seller's own auto_run_enabled flag, and
	// mode in {shadow, unified} (legacy sellers are never auto-run).
	EffectiveAutoRun bool
	// EffectiveAutoPublish reports whether PublishRun may be invoked
	// automatically (as opposed to admin/devtools-triggered) for this
	// seller: requires the global rollout+auto-publish switches, the
	// seller's own auto_publish_enabled flag, and mode in {shadow, unified}.
	// It does NOT by itself make the publication visible — that is
	// entirely controlled by Visibility (shadow mode always publishes with
	// Visibility == VisibilityShadow, so auto-publishing there only
	// materializes the shadow projection for comparison/QA; it never
	// becomes the seller's current dashboard/actions source).
	EffectiveAutoPublish bool

	// Visibility is the visibility PublishRun should use by default for this
	// seller: shadow mode -> shadow, unified mode -> visible. Legacy sellers
	// have no meaningful visibility (publication should not be attempted).
	Visibility Visibility
}
