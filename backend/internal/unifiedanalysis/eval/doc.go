// Package eval performs semantic checks on Unified Analysis AI output against
// scenario-specific ExpectedChecks. It is used by Stage 4 fixture tests and
// offline eval harnesses — not by the production outputvalidator.
//
// # analysis_failed semantics
//
// The AI may return analysis_outcome=analysis_failed when the Analysis Pack is
// technically present (all 12 files, valid manifest) but analysis cannot be
// completed for documented data-integrity reasons (contradictory sources,
// unrecoverable gaps, etc.). In that case:
//
//   - actions must be empty
//   - dashboard.top_action_ids and recommended_action_ids must be empty
//   - limitations must be non-empty (at least one pack or allowlisted code)
//
// publication_ready is allowed when the output is technically valid; the
// outputvalidator enforces outcome consistency for analysis_failed but does
// not repair or reinterpret business content.
package eval
