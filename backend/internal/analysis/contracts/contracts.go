package contracts

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

const (
	BundleVersionV1 = "contract_bundle_v1"
	NotImplemented  = "NOT_IMPLEMENTED"
)

// Versions is the complete v1 version seal. A field must never be silently
// omitted: unavailable future contracts are explicitly NOT_IMPLEMENTED.
type Versions struct {
	OrchestrationPolicy             string `json:"orchestration_policy"`
	FactualAPI                      string `json:"factual_api"`
	TemporalPolicy                  string `json:"temporal_policy"`
	DQPolicy                        string `json:"dq_policy"`
	CapabilityPolicy                string `json:"capability_policy"`
	SalesMaturityPolicy             string `json:"sales_maturity_policy"`
	StockFreshnessPolicy            string `json:"stock_freshness_policy"`
	PriceFreshnessPolicy            string `json:"price_freshness_policy"`
	AdvertisingMaturityPolicy       string `json:"advertising_maturity_policy"`
	AttributionPolicy               string `json:"attribution_aggregation_policy"`
	UniverseManifest                string `json:"universe_manifest"`
	ContextManifest                 string `json:"context_manifest"`
	FactFamilyBinding               string `json:"fact_family_binding"`
	WatermarkPolicy                 string `json:"watermark_policy"`
	CapabilityRegistry              string `json:"capability_registry"`
	FunnelMetricRegistry            string `json:"funnel_metric_registry"`
	FinanceTypeRegistry             string `json:"finance_type_registry"`
	SearchCollectionPolicy          string `json:"search_collection_policy"`
	PromotionsSemanticRegistry      string `json:"promotions_semantic_registry"`
	AccountHealthMetricRegistry     string `json:"account_health_metric_registry"`
	SupplyStateRegistry             string `json:"supply_state_registry"`
	AdvertisingAttributionSemantics string `json:"advertising_attribution_semantics"`
	ContinuityContract              string `json:"continuity_contract"`
	LandscapeCore                   string `json:"landscape_core"`
	Materiality                     string `json:"materiality"`
	LandscapeDetail                 string `json:"landscape_detail"`
	DiscoveryPack                   string `json:"discovery_pack"`
	DiscoveryPrompt                 string `json:"discovery_prompt"`
	DiscoveryOutput                 string `json:"discovery_output"`
	DiscoveryValidation             string `json:"discovery_validation"`
	EvidenceExpansion               string `json:"evidence_expansion"`
	DeepPack                        string `json:"deep_pack"`
	DeepPrompt                      string `json:"deep_prompt"`
	DeepOutput                      string `json:"deep_output"`
	DeepValidation                  string `json:"deep_validation"`
	ActionReconciliation            string `json:"action_reconciliation"`
	Publication                     string `json:"publication"`
	Projection                      string `json:"projection"`
	PublicationValidation           string `json:"publication_validation"`
	Publish                         string `json:"publish"`
	PostPublish                     string `json:"post_publish"`
}

type value struct {
	BundleVersion string   `json:"bundle_version"`
	Versions      Versions `json:"versions"`
}

// ContractBundle owns an immutable copy of the version seal.
type ContractBundle struct{ v value }

func New(v Versions) (ContractBundle, error) {
	if v.OrchestrationPolicy == "" || v.FactualAPI == "" || v.UniverseManifest == "" || v.ContextManifest == "" {
		return ContractBundle{}, fmt.Errorf("contracts: required v1 versions are empty")
	}
	fillFuture(&v)
	return ContractBundle{v: value{BundleVersion: BundleVersionV1, Versions: v}}, nil
}

func (b ContractBundle) BundleVersion() string { return b.v.BundleVersion }
func (b ContractBundle) Versions() Versions    { return b.v.Versions }
func (b ContractBundle) Hash() string          { return canonicaljson.Hash(b.v) }
func (b ContractBundle) MarshalCanonical() ([]byte, error) {
	if b.v.BundleVersion != BundleVersionV1 {
		return nil, fmt.Errorf("contracts: invalid bundle")
	}
	return canonicaljson.Marshal(b.v)
}

func fillFuture(v *Versions) {
	// Empty fields become NOT_IMPLEMENTED. Non-empty Landscape/Materiality/Detail
	// (and ContinuityContract) seals from Current() are preserved.
	fields := []*string{
		&v.FactFamilyBinding, &v.WatermarkPolicy, &v.CapabilityRegistry,
		&v.ContinuityContract,
		&v.LandscapeCore, &v.Materiality, &v.LandscapeDetail, &v.DiscoveryPack,
		&v.DiscoveryPrompt, &v.DiscoveryOutput, &v.DiscoveryValidation,
		&v.EvidenceExpansion, &v.DeepPack, &v.DeepPrompt, &v.DeepOutput,
		&v.DeepValidation, &v.ActionReconciliation, &v.Publication,
		&v.Projection, &v.PublicationValidation, &v.Publish, &v.PostPublish,
	}
	for _, field := range fields {
		if *field == "" {
			*field = NotImplemented
		}
	}
}

// nameHashSeal builds a Prompt №9.1-style "NAME#hash" contract seal.
func nameHashSeal(name string, payload any) string {
	return name + "#" + canonicaljson.Hash(payload)
}

// Current returns the Prompt №11 production contract bundle for new runs.
// Version strings mirror factualapi.CurrentPolicies / analysis_orchestration_policy_v5
// and Prompt №9 landscape/materiality/continuity seals. Discovery four fields plus
// EvidenceExpansion are composite seals.
func Current() (ContractBundle, error) {
	return New(Versions{
		OrchestrationPolicy:       "analysis_orchestration_policy_v7",
		FactualAPI:                "factual_api_v6",
		TemporalPolicy:            "temporal_policy_v1",
		DQPolicy:                  "dq_policy_v4",
		CapabilityPolicy:          "capability_policy_v4",
		SalesMaturityPolicy:       "sales_maturity_policy_v2",
		StockFreshnessPolicy:      "stock_freshness_policy_v1",
		PriceFreshnessPolicy:      "price_freshness_policy_v1",
		AdvertisingMaturityPolicy: "advertising_attribution_maturity_policy_v1",
		AttributionPolicy:         "advertising_attribution_aggregation_policy_v1",
		UniverseManifest:          "factual_universe_manifest_v1",
		ContextManifest:           "resolved_context_manifest_v1",
		FactFamilyBinding: nameHashSeal("FACT_FAMILY_BINDING_V1", map[string]any{
			"version": "FACT_FAMILY_BINDING_V1",
			"key":     "domain+fact_family",
			"access":  "Binding(domain,fact_family)",
		}),
		WatermarkPolicy: nameHashSeal("WATERMARK_REVISION_POLICY_V1", map[string]any{
			"version":    "WATERMARK_REVISION_POLICY_V1",
			"projection": "domain_watermarks",
			"history":    "domain_watermark_revisions",
			"read":       "AS_KNOWN_AT_cutoff",
		}),
		CapabilityRegistry: nameHashSeal("FACT_FAMILY_CAPABILITY_REGISTRY_V1", map[string]any{
			"version": "FACT_FAMILY_CAPABILITY_REGISTRY_V1",
			"match":   "explicit_registry",
			"forbid":  "HasPrefix_heuristic",
		}),
		FunnelMetricRegistry:            "funnel_registry_v1",
		FinanceTypeRegistry:             "finance_type_registry_v1",
		SearchCollectionPolicy:          "search_detail_collection_policy_v1",
		PromotionsSemanticRegistry:      "promotions_semantic_registry_v1",
		AccountHealthMetricRegistry:     "account_health_metric_registry_v1",
		SupplyStateRegistry:             "supply_state_mapping_v1",
		AdvertisingAttributionSemantics: "advertising_attribution_kind_v1",
		ContinuityContract:              "CONTINUITY_INITIAL_NO_BASE_V1",
		LandscapeCore:                   productionLandscapeCoreSeal(),
		Materiality:                     productionMaterialitySeal(),
		LandscapeDetail:                 productionLandscapeDetailSeal(),
		DiscoveryPack:                   productionDiscoveryPackSeal(),
		DiscoveryPrompt:                 productionDiscoveryPromptSeal(),
		DiscoveryOutput:                 productionDiscoveryOutputSeal(),
		DiscoveryValidation:             productionDiscoveryValidationSeal(),
		EvidenceExpansion:               productionEvidenceExpansionSeal(),
		DeepPack:                        productionDeepPackSeal(),
		DeepPrompt:                      productionDeepPromptSeal(),
		DeepOutput:                      productionDeepOutputSeal(),
		DeepValidation:                  productionDeepValidationSeal(),
		ActionReconciliation:            productionActionReconciliationSeal(),
		Publication:                     productionPublicationSeal(),
		Projection:                      productionProjectionSeal(),
		PublicationValidation:           productionPublicationValidationSeal(),
		Publish:                         productionPublishSeal(),
		PostPublish:                     productionPostPublishSeal(),
	})
}
