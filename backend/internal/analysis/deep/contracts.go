package deep

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func PackContractSeal() string {
	return "DEEP_PACK#" + canonicaljson.Hash(map[string]any{
		"pack_schema": PackSchemaV2, "fact_registry": FactRegistryProjectionV1,
		"entity_index": EntityIndexV1, "relationship_policy": RelationshipClaimPolicyV1,
		"action_catalog": actioncatalog.CatalogV1, "action_menu_policy": actioncatalog.MenuPolicyV1,
		"pack_budget": PackBudgetV1,
	})
}

func PromptContractSeal() string {
	return "DEEP_PROMPT#" + canonicaljson.Hash(map[string]any{
		"prompt": PromptPolicyV4, "model_policy": ModelPolicyV1, "instructions": StableInstructionsV2,
	})
}

func OutputContractSeal() string {
	return "DEEP_OUTPUT#" + canonicaljson.Hash(map[string]any{
		"output_schema": OutputSchemaV4, "output_budget": OutputBudgetV1, "request_contract": RequestContractV1,
	})
}

func ValidationContractSeal() string {
	return "DEEP_VALIDATION#" + canonicaljson.Hash(map[string]any{
		"validation": ValidationV3, "attempt_policy": AIAttemptPolicyV1,
	})
}
