// Package contract defines immutable v1 schemas for the future Unified Analysis
// Pack (ZIP) and Unified Analysis Output. Stage 1 ships types, constants,
// fixtures, and documentation only — no pack builder or OpenAI runtime.
package contract

// Analysis pack and unified output schema version identifiers.
const (
	AnalysisPackSchemaVersionV1          = "aivion_analysis_pack_v1"
	UnifiedAnalysisOutputSchemaVersionV1 = "aivion_unified_analysis_v1"
	// UnifiedAnalysisOutputSchemaVersionV2 extends v1 with diagnostic signals,
	// improvement levers, and evidence metadata for seller pulse / diagnostics.
	UnifiedAnalysisOutputSchemaVersionV2 = "aivion_unified_analysis_v2"
	// UnifiedAnalysisOutputSchemaVersionV3 requires separate ad_efficiency.narrative.headline
	// and summary (explanation) fields — never derived by splitting on '.'.
	UnifiedAnalysisOutputSchemaVersionV3 = "aivion_unified_analysis_v3"
	// UnifiedAnalysisOutputSchemaVersionV4 adds ad_efficiency.narrative.recommendation
	// as a separate AI-authored actionable line.
	UnifiedAnalysisOutputSchemaVersionV4 = "aivion_unified_analysis_v4"
	// UnifiedAnalysisOutputSchemaVersionV5 drops ad_efficiency.narrative.recommendation
	// and requires dual-metric advertising narrative (store vs attributed share).
	UnifiedAnalysisOutputSchemaVersionV5 = "aivion_unified_analysis_v5"
	// UnifiedAnalysisOutputSchemaVersionV6 requires execution_tier on every Action
	// and raises the material_actions_found floor to 5–15 Actions (≥3 required).
	UnifiedAnalysisOutputSchemaVersionV6 = "aivion_unified_analysis_v6"
	// UnifiedAnalysisOutputSchemaVersionV7 separates seller_pulse key_points from
	// important_facts and requires structured metric claims with period_type.
	UnifiedAnalysisOutputSchemaVersionV7 = "aivion_unified_analysis_v7"
	// UnifiedAnalysisOutputSchemaVersionV8 requires per-action evidence
	// (evidence_fact_ids ≥1, source_signal_ids) and structured completion
	// conditions with an explicit baseline_fact_id/baseline_value.
	UnifiedAnalysisOutputSchemaVersionV8 = "aivion_unified_analysis_v8"
	// UnifiedAnalysisOutputSchemaVersionV9 adds seller_pulse_key_point to the
	// primary_section enum (Wave 2) and requires ad metric_claims.
	UnifiedAnalysisOutputSchemaVersionV9 = "aivion_unified_analysis_v9"
	// UnifiedAnalysisOutputSchemaVersionV10 adds ad_efficiency.narrative.threshold_claims
	// for policy-backed comparative thresholds (e.g. campaign ДРР > 80%).
	UnifiedAnalysisOutputSchemaVersionV10 = "aivion_unified_analysis_v10"
	// UnifiedAnalysisOutputSchemaVersionV11 is the findings-first contract:
	// every dashboard block and Action references shared Finding records by
	// finding_id, and every factual/comparative claim is grounded via
	// top-level metric_claims/threshold_claims rather than embedded ad hoc
	// in prose. See output_v11.go for the Go struct tree. Not yet the
	// accepted runtime schema — UnifiedAnalysisOutputSchemaVersionCurrent
	// stays on v10 until the v11 pipeline is wired up.
	UnifiedAnalysisOutputSchemaVersionV11 = "aivion_unified_analysis_v11"
	// UnifiedAnalysisOutputSchemaVersionCurrent is the schema accepted for new runs.
	UnifiedAnalysisOutputSchemaVersionCurrent = UnifiedAnalysisOutputSchemaVersionV10
)

// Policy defaults for Analysis Pack / Unified Analyst v1.
// MinActionsMaterialV1 / MinRequiredActionsMaterialV1 are floors of 1 so
// material_actions_found never forces the model to invent filler Actions
// to hit a quota of 5 / 3 (acceptance fix after run 191). MaxActionsV1 and
// quality validators (evidence, grounding, dedup) remain hard gates.
const (
	MaxActionsV1                 = 15
	MaxTopActionsV1              = 3
	MinActionsMaterialV1         = 1
	MinRequiredActionsMaterialV1 = 1
)
