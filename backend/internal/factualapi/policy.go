package factualapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	TemporalPolicyV1         = "temporal_policy_v1"
	DQPolicyV1               = "dq_policy_v1"
	CapabilityPolicyV1       = "capability_policy_v1"
	SalesMaturityV1          = "sales_maturity_policy_v1"
	APIVersionV1             = "factual_api_v1"
	DQPolicyV2               = "dq_policy_v2"
	CapabilityPolicyV2       = "capability_policy_v2"
	SalesMaturityV2          = "sales_maturity_policy_v2"
	APIVersionV2             = "factual_api_v2"
	DQPolicyV3               = "dq_policy_v3"
	CapabilityPolicyV3       = "capability_policy_v3"
	APIVersionV3             = "factual_api_v3"
	APIVersionV4             = "factual_api_v4"
	DQPolicyV4               = "dq_policy_v4"
	CapabilityPolicyV4       = "capability_policy_v4"
	APIVersionV5             = "factual_api_v5"
	APIVersionV6             = "factual_api_v6"
	StockFreshnessV1         = "stock_freshness_policy_v1"
	PriceFreshnessV1         = "price_freshness_policy_v1"
	AdvertisingMaturityV1    = "advertising_attribution_maturity_policy_v1"
	AttributionAggregationV1 = "advertising_attribution_aggregation_policy_v1"
)

func StableHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

type PolicyVersions struct {
	Temporal, DQ, Capability, SalesMaturity, API                                string
	StockFreshness, PriceFreshness, AdvertisingMaturity, AttributionAggregation string
}

func V1Policies() PolicyVersions {
	return PolicyVersions{Temporal: TemporalPolicyV1, DQ: DQPolicyV1, Capability: CapabilityPolicyV1, SalesMaturity: SalesMaturityV1, API: APIVersionV1, StockFreshness: StockFreshnessV1, PriceFreshness: PriceFreshnessV1, AdvertisingMaturity: AdvertisingMaturityV1, AttributionAggregation: AttributionAggregationV1}
}
func CurrentPolicies() PolicyVersions {
	return PolicyVersions{Temporal: TemporalPolicyV1, DQ: DQPolicyV4, Capability: CapabilityPolicyV4, SalesMaturity: SalesMaturityV2, API: APIVersionV6, StockFreshness: StockFreshnessV1, PriceFreshness: PriceFreshnessV1, AdvertisingMaturity: AdvertisingMaturityV1, AttributionAggregation: AttributionAggregationV1}
}
