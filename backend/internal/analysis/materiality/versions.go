package materiality

// Version seals for Prompt №9 Materiality Engine (pilot defaults).
const (
	// MaterialityPolicyV1 is the legacy absolute soft-diversity policy seal (do not mutate).
	MaterialityPolicyV1 = "MATERIALITY_POLICY_V1"

	// MaterialityPolicyV1_1 is the prior ranking policy (FeatureValue semantics absent).
	MaterialityPolicyV1_1 = "MATERIALITY_POLICY_V1_1"

	// MaterialityPolicyV1_2 is the frozen Prompt №9.3 policy (do not mutate).
	MaterialityPolicyV1_2 = "MATERIALITY_POLICY_V1_2"

	// MaterialityPolicyV1_3 is the active policy with strict Exact parsing and integer counts (Prompt №9.4).
	MaterialityPolicyV1_3 = "MATERIALITY_POLICY_V1_3"

	// MaterialityPolicyV1_4 is the active Prompt №9.6 policy (advertising family split).
	MaterialityPolicyV1_4 = "MATERIALITY_POLICY_V1_4"

	// DiscoveryStandardBudgetV1 is the pilot Discovery selection budget profile.
	DiscoveryStandardBudgetV1 = "DISCOVERY_STANDARD_V1"

	// ContinuityInitialNoBaseV1 is the no-base continuity stub (no prior publication).
	// Continuity targets do not compete with fresh materiality budget.
	ContinuityInitialNoBaseV1 = "CONTINUITY_INITIAL_NO_BASE_V1"

	// LaneRegistryV1 seals the typed lane definition set used by MaterialityPolicyV1_1.
	LaneRegistryV1 = "MATERIALITY_LANE_REGISTRY_V1"

	// LaneRegistryV2 seals Prompt №9.6 advertising-family-aligned lanes.
	LaneRegistryV2 = "MATERIALITY_LANE_REGISTRY_V2"

	// ArtifactSchemaRankingV1 / ArtifactSchemaSelectionV1 are JSON artifact schema versions.
	ArtifactSchemaRankingV1   = "materiality_ranking_v1"
	ArtifactSchemaSelectionV1 = "materiality_selection_v1"
)

// EntityKind is the closed set of Material Entity kinds.
type EntityKind string

const (
	EntityAccount  EntityKind = "ACCOUNT"
	EntityCategory EntityKind = "CATEGORY"
	EntitySKU      EntityKind = "SKU"
	EntityCampaign EntityKind = "CAMPAIGN"
)

// Domain groups lanes for observability (not a score).
type Domain string

const (
	DomainSales       Domain = "SALES"
	DomainStock       Domain = "STOCK"
	DomainAdvertising Domain = "ADVERTISING"
	DomainPrice       Domain = "PRICE"
	DomainSearch      Domain = "SEARCH"
	DomainSupply      Domain = "SUPPLY"
)

// Direction is the physical ranking direction of a lane (not good/bad).
type Direction string

const (
	DirectionDesc Direction = "DESC"
	DirectionAsc  Direction = "ASC"
)

// Readiness states allowed by lane eligibility (STALE never allowed for current operational lanes).
const (
	ReadinessReady   = "READY"
	ReadinessPartial = "PARTIAL"
)
