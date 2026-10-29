package discovery

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"

const (
	PackSchemaV2                = "discovery_pack_schema_v2"
	FactHandleRegistryV2        = "discovery_fact_handle_registry_v2"
	FactHandleMapSchemaV2       = "discovery_fact_handle_map_v2"
	EvidenceCatalogV1           = "evidence_catalog_v1"
	EvidenceMenuPolicyV2        = "discovery_evidence_menu_policy_v2"
	EvidenceMenuPolicyV3        = "discovery_evidence_menu_policy_v3"
	PackBudgetV2                = "discovery_pack_budget_v2"
	PromptPolicyV1              = "discovery_prompt_v1"
	OutputSchemaV1              = "discovery_output_schema_v1"
	OutputBudgetV1              = "discovery_output_budget_v1"
	ValidationV2                = "discovery_validation_v2"
	ValidationV3                = "discovery_validation_v3"
	ModelPolicyV1               = "discovery_model_policy_v1"
	AIAttemptPolicyV1           = "discovery_ai_attempt_policy_v1"
	RequestContractV2           = "discovery_request_contract_v2"
	PackArtifactSchemaV2        = "discovery_pack_artifact_v2"
	ValidatedOutputSchemaV1     = "discovery_validated_output_v1"
	ValidationReportSchemaV1    = "discovery_validation_report_v1"
	GroundingPolicyV2           = "discovery_grounding_policy_v2"
	FormulaMoneyComparisonV1    = "landscape.money_comparison_v1"
	FormulaCountComparisonV1    = "landscape.count_comparison_v1"
	FormulaPctPointComparisonV1 = "landscape.pct_point_comparison_v1"
	OutputSchemaNameV1          = "aivion_discovery_output_v1"
)

func packSeal() string {
	return canonicaljson.Hash(map[string]any{
		"pack_schema":          PackSchemaV2,
		"fact_handle_registry": FactHandleRegistryV2,
		"evidence_catalog":     EvidenceCatalogV1,
		"evidence_menu_policy": EvidenceMenuPolicyV3,
		"pack_budget":          PackBudgetV2,
		"grounding_policy":     GroundingPolicyV2,
	})
}

func promptSeal() string {
	return canonicaljson.Hash(map[string]any{
		"prompt":       PromptPolicyV1,
		"model_policy": ModelPolicyV1,
		"instructions": StableInstructionsV1,
	})
}

func outputSeal() string {
	return canonicaljson.Hash(map[string]any{
		"output_schema": OutputSchemaV1,
		"output_budget": OutputBudgetV1,
	})
}

func validationSeal() string {
	return canonicaljson.Hash(map[string]any{
		"validation":     ValidationV3,
		"attempt_policy": AIAttemptPolicyV1,
	})
}

func PackContractSeal() string       { return "DISCOVERY_PACK#" + packSeal() }
func PromptContractSeal() string     { return "DISCOVERY_PROMPT#" + promptSeal() }
func OutputContractSeal() string     { return "DISCOVERY_OUTPUT#" + outputSeal() }
func ValidationContractSeal() string { return "DISCOVERY_VALIDATION#" + validationSeal() }
