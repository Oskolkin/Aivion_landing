package discovery

import (
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type PackArtifactV1 struct {
	SchemaVersion string `json:"schema_version"`

	SellerAccountID   int64  `json:"-"`
	RunID             int64  `json:"-"`
	FactualUniverseID string `json:"-"`

	FactualUniverseHash      string `json:"factual_universe_hash"`
	MaterialitySelectionHash string `json:"materiality_selection_hash"`
	SelectionArtifactSHA256  string `json:"selection_artifact_sha256,omitempty"`
	LandscapeDetailHash      string `json:"landscape_detail_hash"`
	LandscapeProfileVersion  string `json:"landscape_profile_version"`
	ContractBundleHash       string `json:"contract_bundle_hash,omitempty"`

	DiscoveryPackSchemaVersion   string `json:"discovery_pack_schema_version"`
	FactHandleRegistryVersion    string `json:"fact_handle_registry_version"`
	DiscoveryPromptPolicyVersion string `json:"discovery_prompt_policy_version"`
	DiscoveryOutputSchemaVersion string `json:"discovery_output_schema_version"`
	DiscoveryValidationVersion   string `json:"discovery_validation_version"`
	EvidenceCatalogVersion       string `json:"evidence_catalog_version"`
	EvidenceMenuPolicyVersion    string `json:"evidence_menu_policy_version"`
	PackBudgetProfileVersion     string `json:"pack_budget_profile_version"`
	OutputBudgetProfileVersion   string `json:"output_budget_profile_version"`
	ModelPolicyVersion           string `json:"model_policy_version"`
	AIAttemptPolicyVersion       string `json:"ai_attempt_policy_version"`
	RequestContractVersion       string `json:"request_contract_version,omitempty"`

	PayloadHash       string `json:"payload_hash"`
	FactHandleMapHash string `json:"fact_handle_map_hash"`
	EvidenceMenuHash  string `json:"evidence_menu_hash"`
	RequestHash       string `json:"request_hash,omitempty"`

	EstimatedInputTokens int `json:"estimated_input_tokens"`

	Payload PayloadV1 `json:"payload"`
}

type AuditEnvelope struct {
	SchemaVersion                string `json:"schema_version"`
	SellerAccountID              int64  `json:"seller_account_id"`
	RunID                        int64  `json:"run_id"`
	FactualUniverseID            string `json:"factual_universe_id"`
	FactualUniverseHash          string `json:"factual_universe_hash"`
	MaterialitySelectionHash     string `json:"materiality_selection_hash"`
	SelectionArtifactSHA256      string `json:"selection_artifact_sha256,omitempty"`
	LandscapeDetailHash          string `json:"landscape_detail_hash"`
	LandscapeProfileVersion      string `json:"landscape_profile_version"`
	ContractBundleHash           string `json:"contract_bundle_hash,omitempty"`
	DiscoveryPackSchemaVersion   string `json:"discovery_pack_schema_version"`
	FactHandleRegistryVersion    string `json:"fact_handle_registry_version"`
	DiscoveryPromptPolicyVersion string `json:"discovery_prompt_policy_version"`
	DiscoveryOutputSchemaVersion string `json:"discovery_output_schema_version"`
	DiscoveryValidationVersion   string `json:"discovery_validation_version"`
	EvidenceCatalogVersion       string `json:"evidence_catalog_version"`
	EvidenceMenuPolicyVersion    string `json:"evidence_menu_policy_version"`
	PackBudgetProfileVersion     string `json:"pack_budget_profile_version"`
	OutputBudgetProfileVersion   string `json:"output_budget_profile_version"`
	ModelPolicyVersion           string `json:"model_policy_version"`
	AIAttemptPolicyVersion       string `json:"ai_attempt_policy_version"`
	RequestContractVersion       string `json:"request_contract_version,omitempty"`
	PayloadHash                  string `json:"payload_hash"`
	FactHandleMapHash            string `json:"fact_handle_map_hash"`
	EvidenceMenuHash             string `json:"evidence_menu_hash"`
	RequestHash                  string `json:"request_hash,omitempty"`
	EstimatedInputTokens         int    `json:"estimated_input_tokens"`
}

func (p PackArtifactV1) Envelope() AuditEnvelope {
	return AuditEnvelope{
		SchemaVersion:   PackArtifactSchemaV2,
		SellerAccountID: p.SellerAccountID, RunID: p.RunID, FactualUniverseID: p.FactualUniverseID,
		FactualUniverseHash: p.FactualUniverseHash, MaterialitySelectionHash: p.MaterialitySelectionHash,
		SelectionArtifactSHA256: p.SelectionArtifactSHA256, LandscapeDetailHash: p.LandscapeDetailHash,
		LandscapeProfileVersion: p.LandscapeProfileVersion, ContractBundleHash: p.ContractBundleHash,
		DiscoveryPackSchemaVersion: p.DiscoveryPackSchemaVersion, FactHandleRegistryVersion: p.FactHandleRegistryVersion,
		DiscoveryPromptPolicyVersion: p.DiscoveryPromptPolicyVersion, DiscoveryOutputSchemaVersion: p.DiscoveryOutputSchemaVersion,
		DiscoveryValidationVersion: p.DiscoveryValidationVersion, EvidenceCatalogVersion: p.EvidenceCatalogVersion,
		EvidenceMenuPolicyVersion: p.EvidenceMenuPolicyVersion, PackBudgetProfileVersion: p.PackBudgetProfileVersion,
		OutputBudgetProfileVersion: p.OutputBudgetProfileVersion, ModelPolicyVersion: p.ModelPolicyVersion,
		AIAttemptPolicyVersion: p.AIAttemptPolicyVersion, RequestContractVersion: p.RequestContractVersion,
		PayloadHash: p.PayloadHash, FactHandleMapHash: p.FactHandleMapHash, EvidenceMenuHash: p.EvidenceMenuHash,
		RequestHash: p.RequestHash, EstimatedInputTokens: p.EstimatedInputTokens,
	}
}

func HashPayload(p PayloadV1) string { return canonicaljson.Hash(p) }

type RequestManifestV1 struct {
	RequestContractVersion  string `json:"request_contract_version"`
	Model                   string `json:"model"`
	ModelPolicyVersion      string `json:"model_policy_version"`
	PromptPolicyVersion     string `json:"prompt_policy_version"`
	PromptHash              string `json:"prompt_hash"`
	OutputSchemaVersion     string `json:"output_schema_version"`
	OutputSchemaHash        string `json:"output_schema_hash"`
	SchemaName              string `json:"schema_name"`
	Strict                  bool   `json:"strict"`
	PackHash                string `json:"pack_hash"`
	PayloadHash             string `json:"payload_hash"`
	FactHandleMapHash       string `json:"fact_handle_map_hash"`
	EvidenceMenuHash        string `json:"evidence_menu_hash"`
	RequestHash             string `json:"request_hash"`
	MaxOutputTokens         int    `json:"max_output_tokens"`
	TruncationMode          string `json:"truncation_mode"`
	Store                   bool   `json:"store"`
	ToolsEnabled            bool   `json:"tools_enabled"`
	PreviousResponseEnabled bool   `json:"previous_response_enabled"`
	EstimatedInputTokens    int    `json:"estimated_input_tokens"`
}

func (m RequestManifestV1) Hash() string { return canonicaljson.Hash(m) }

// PackWireV1 matches canonicaljson.Marshal of the discovery pack artifact body.
type PackWireV1 struct {
	Envelope AuditEnvelope `json:"AuditEnvelope"`
	Payload  PayloadV1     `json:"payload"`
}

func UnmarshalPackWire(body []byte) (AuditEnvelope, PayloadV1, error) {
	var wire PackWireV1
	if err := json.Unmarshal(body, &wire); err != nil {
		return AuditEnvelope{}, PayloadV1{}, err
	}
	return wire.Envelope, wire.Payload, nil
}
