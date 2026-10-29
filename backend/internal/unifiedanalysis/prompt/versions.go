package prompt

// Prompt profile selects smoke vs production analyst instructions.
type PromptProfile string

const (
	PromptProfileSmoke    PromptProfile = "smoke"
	PromptProfileAnalysis PromptProfile = "analysis"
)

const (
	VersionAnalystV1    = "unified_analyst_v1"
	VersionAnalystV1_1  = "unified_analyst_v1_1"
	VersionAnalystV1_2  = "unified_analyst_v1_2"
	VersionAnalystV1_3  = "unified_analyst_v1_3"
	VersionAnalystV1_4  = "unified_analyst_v1_4"
	VersionAnalystV1_5  = "unified_analyst_v1_5"
	VersionAnalystV1_6  = "unified_analyst_v1_6"
	VersionAnalystV1_7  = "unified_analyst_v1_7"
	VersionAnalystV1_8  = "unified_analyst_v1_8"
	VersionAnalystV1_9  = "unified_analyst_v1_9"
	VersionAnalystV1_10 = "unified_analyst_v1_10"
	VersionAnalystV1_11 = "unified_analyst_v1_11"
	VersionAnalystV1_12 = "unified_analyst_v1_12"
	VersionAnalystV1_13 = "unified_analyst_v1_13"
	VersionAnalystV1_14 = "unified_analyst_v1_14"
	VersionAnalystV1_15 = "unified_analyst_v1_15"
	VersionAnalystV1_16 = "unified_analyst_v1_16"
	VersionAnalystV1_17 = "unified_analyst_v1_17"
	VersionAnalystV1_18 = "unified_analyst_v1_18"
	VersionAnalystV1_19 = "unified_analyst_v1_19"
	VersionAnalystV1_20 = "unified_analyst_v1_20"
	// VersionAnalystV1_21 requires per-action evidence_fact_ids and structured
	// completion conditions with baseline_fact_id/baseline_value (schema v8).
	VersionAnalystV1_21 = "unified_analyst_v1_21"
	// VersionAnalystV1_22 tightens completion targets (must be unmet at analysis
	// time) and forbids SKU reuse across Actions (schema v8).
	VersionAnalystV1_22 = "unified_analyst_v1_22"
	// VersionAnalystV1_23 targets schema v9: seller_pulse_key_point enum,
	// store-share account_drr_pct grounding, backend Key SKU / health policy.
	VersionAnalystV1_23 = "unified_analyst_v1_23"
	// VersionAnalystV1_24 requires ad_efficiency.narrative.metric_claims
	// (schema v9): the adapter no longer rewrites advertising percentages,
	// so every free-text share in headline/summary must be backed by a
	// canonical-metric_code claim that outputvalidator grounds against
	// Pack facts (Wave 2 Dashboard audit).
	VersionAnalystV1_24 = "unified_analyst_v1_24"
	// VersionAnalystV1_25 tightens store-share vs attributed DRR labeling,
	// Pack fact_id shape (no invented sku:<metric>:… IDs), and drops the
	// mandatory 5-Action floor for material_actions_found (acceptance fix
	// after runs 190/191 on SHA cdafd49).
	VersionAnalystV1_25 = "unified_analyst_v1_25"
	// VersionAnalystV1_26 targets schema v10: ad_efficiency.narrative.threshold_claims
	// for policy-backed comparative thresholds (e.g. ДРР > 80%) so they are
	// not forced through factual metric_claims (acceptance fix after run 194).
	VersionAnalystV1_26 = "unified_analyst_v1_26"
	// VersionAnalystV1_27 keeps schema v10 and tightens Top Actions: every
	// dashboard.top_action_ids entry MUST be execution_tier=required; never
	// pad Top 3 with recommended (acceptance fix after run 201).
	VersionAnalystV1_27 = "unified_analyst_v1_27"
	// VersionAnalystV1_28 keeps schema v10 and requires canonical
	// health_policy_v1 risk_drivers[].id codes (never entity-suffixed
	// composites) after acceptance run 202.
	VersionAnalystV1_28 = "unified_analyst_v1_28"
	// VersionAnalystV1_29 keeps schema v10 and enforces section-aware
	// Seller Pulse fact grounding (account-level only; campaign DRR in ads).
	VersionAnalystV1_29 = "unified_analyst_v1_29"
	// VersionAnalystV1_30 keeps schema v10 and requires materiality-ranked
	// Top Actions (stock shortage before low-priority no-sales) plus a hard
	// ban on unsupported photo/description/content causal recommendations.
	VersionAnalystV1_30 = "unified_analyst_v1_30"
	// VersionAnalystV1_31 keeps schema v10 and requires demo-ready action
	// depth (8-12 Actions with stock-by-urgency-band / ads high-DRR-vs-
	// spend-without-orders splitting when the Pack has multiple independent
	// material problems), a structured 3-paragraph Seller Pulse narrative
	// (sales / funnel / advertising) with priority-action deep links,
	// mandatory funnel wording, and an expanded banned-phrase list.
	VersionAnalystV1_31 = "unified_analyst_v1_31"
	// VersionAnalystV1_32 targets schema v11 (findings-first contract, see
	// contract/output_v11.go) paired with analysis_policy_v2 (see
	// backend/internal/unifiedanalysis/policy/v2). Registered for
	// pipeline preparation, tests, and evals only — CurrentAnalystVersion
	// stays on v1_31 until the v11 + policy v2 pipeline is fully wired up.
	VersionAnalystV1_32 = "unified_analyst_v1_32"
)

// ProfileForVersion maps a stored prompt_version to a runtime profile.
func ProfileForVersion(version string) PromptProfile {
	switch version {
	case VersionAnalystV1, VersionAnalystV1_1, VersionAnalystV1_2, VersionAnalystV1_3, VersionAnalystV1_4, VersionAnalystV1_5, VersionAnalystV1_6, VersionAnalystV1_7, VersionAnalystV1_8, VersionAnalystV1_9, VersionAnalystV1_10, VersionAnalystV1_11, VersionAnalystV1_12, VersionAnalystV1_13, VersionAnalystV1_14, VersionAnalystV1_15, VersionAnalystV1_16, VersionAnalystV1_17, VersionAnalystV1_18, VersionAnalystV1_19, VersionAnalystV1_20, VersionAnalystV1_21, VersionAnalystV1_22, VersionAnalystV1_23, VersionAnalystV1_24, VersionAnalystV1_25, VersionAnalystV1_26, VersionAnalystV1_27, VersionAnalystV1_28, VersionAnalystV1_29, VersionAnalystV1_30, VersionAnalystV1_31, VersionAnalystV1_32:
		return PromptProfileAnalysis
	default:
		return PromptProfileSmoke
	}
}
