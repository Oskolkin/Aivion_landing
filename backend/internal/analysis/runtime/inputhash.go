package runtime

import (
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

func IsForbiddenInputHash(h string) bool {
	return h == "" || h == zeroHash || len(h) != 64
}

type ResolveInput struct {
	SellerID           int64
	AnalysisDate       time.Time
	CutoffAt           time.Time
	RunKind            string
	IntentHash         string
	ContractBundleHash string
	StageVersion       string
}

func HashResolveContext(in ResolveInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                string(stages.ResolveContext),
		"stage_version":        in.StageVersion,
		"seller_account_id":    in.SellerID,
		"analysis_date":        in.AnalysisDate.UTC().Format("2006-01-02"),
		"analysis_cutoff_at":   in.CutoffAt.UTC().Format(time.RFC3339Nano),
		"run_kind":             in.RunKind,
		"intent_hash":          in.IntentHash,
		"contract_bundle_hash": in.ContractBundleHash,
	})
}

type SealInput struct {
	ResolvedContextHash string
	AnalysisDate        time.Time
	CutoffAt            time.Time
	ContractBundleHash  string
	StageVersion        string
}

func HashSealUniverse(in SealInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.ResolvedContextHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                   string(stages.SealUniverse),
		"stage_version":           in.StageVersion,
		"resolved_context_hash":   in.ResolvedContextHash,
		"analysis_date":           in.AnalysisDate.UTC().Format("2006-01-02"),
		"analysis_cutoff_at":      in.CutoffAt.UTC().Format(time.RFC3339Nano),
		"contract_bundle_hash":    in.ContractBundleHash,
		"universe_policy_version": "factual_universe_policy_v1",
	})
}

// ContinuityInput hashes PREPARE_CONTINUITY (§196).
type ContinuityInput struct {
	SellerID           int64
	AnalysisDate       time.Time
	CutoffAt           time.Time
	RunKind            string
	IntentHash         string
	ContractBundleHash string
	BasePublicationID  *int64
	StageVersion       string
}

func HashPrepareContinuity(in ContinuityInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	var base any
	if in.BasePublicationID != nil && *in.BasePublicationID > 0 {
		base = *in.BasePublicationID
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                       string(stages.PrepareContinuity),
		"stage_version":               in.StageVersion,
		"seller_account_id":           in.SellerID,
		"analysis_date":               in.AnalysisDate.UTC().Format("2006-01-02"),
		"analysis_cutoff_at":          in.CutoffAt.UTC().Format(time.RFC3339Nano),
		"run_kind":                    in.RunKind,
		"intent_hash":                 in.IntentHash,
		"contract_bundle_hash":        in.ContractBundleHash,
		"base_publication_id":         base,
		"continuity_contract_version": continuity.ContractInitialNoBaseV1,
	})
}

// LandscapeCoreInput hashes BUILD_LANDSCAPE_CORE (§197).
type LandscapeCoreInput struct {
	UniverseManifestHash string
	ContractBundleHash   string
	StageVersion         string
}

func HashLandscapeCore(in LandscapeCoreInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.UniverseManifestHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                    string(stages.BuildLandscapeCore),
		"stage_version":            in.StageVersion,
		"universe_manifest_hash":   in.UniverseManifestHash,
		"landscape_schema_profile": landscape.LandscapeSchemaV5 + "|" + landscape.CoreProfileVersionSealV5(),
		"contract_bundle_hash":     in.ContractBundleHash,
	})
}

// MaterialityInput hashes RUN_MATERIALITY (§198).
type MaterialityInput struct {
	LandscapeCoreHash     string
	MaterialityPolicyHash string
	LaneRegistryHash      string
	BudgetProfileHash     string
	ContractBundleHash    string
	StageVersion          string
}

func HashMateriality(in MaterialityInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.LandscapeCoreHash) || IsForbiddenInputHash(in.MaterialityPolicyHash) || IsForbiddenInputHash(in.LaneRegistryHash) || IsForbiddenInputHash(in.BudgetProfileHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                   string(stages.RunMateriality),
		"stage_version":           in.StageVersion,
		"landscape_core_hash":     in.LandscapeCoreHash,
		"materiality_policy_hash": in.MaterialityPolicyHash,
		"lane_registry_hash":      in.LaneRegistryHash,
		"budget_profile_hash":     in.BudgetProfileHash,
		"contract_bundle_hash":    in.ContractBundleHash,
	})
}

// LandscapeDetailInput hashes BUILD_LANDSCAPE_DETAIL (§199).
type LandscapeDetailInput struct {
	UniverseManifestHash     string
	MaterialitySelectionHash string
	ContractBundleHash       string
	StageVersion             string
}

func HashLandscapeDetail(in LandscapeDetailInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.UniverseManifestHash) || IsForbiddenInputHash(in.MaterialitySelectionHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                      string(stages.BuildLandscapeDetail),
		"stage_version":              in.StageVersion,
		"universe_manifest_hash":     in.UniverseManifestHash,
		"materiality_selection_hash": in.MaterialitySelectionHash,
		"landscape_schema_profile":   landscape.LandscapeSchemaV5 + "|" + landscape.DetailProfileVersionSealV5(),
		"contract_bundle_hash":       in.ContractBundleHash,
	})
}

type BuildDiscoveryPackInput struct {
	UniverseManifestHash     string
	MaterialitySelectionHash string
	LandscapeDetailHash      string
	DiscoveryPackSeal        string
	FactHandleRegistry       string
	EvidenceCatalogSeal      string
	PackBudgetVersion        string
	ContractBundleHash       string
	StageVersion             string
}

func HashBuildDiscoveryPack(in BuildDiscoveryPackInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.UniverseManifestHash) || IsForbiddenInputHash(in.MaterialitySelectionHash) || IsForbiddenInputHash(in.LandscapeDetailHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                      string(stages.BuildDiscoveryPack),
		"stage_version":              in.StageVersion,
		"universe_manifest_hash":     in.UniverseManifestHash,
		"materiality_selection_hash": in.MaterialitySelectionHash,
		"landscape_detail_hash":      in.LandscapeDetailHash,
		"discovery_pack_seal":        in.DiscoveryPackSeal,
		"fact_handle_registry":       in.FactHandleRegistry,
		"evidence_catalog_menu":      in.EvidenceCatalogSeal,
		"pack_budget":                in.PackBudgetVersion,
		"contract_bundle_hash":       in.ContractBundleHash,
	})
}

type DiscoveryAIInput struct {
	DiscoveryPackHash   string
	PayloadHash         string
	FactHandleMapHash   string
	RequestManifestHash string
	PromptPolicyVersion string
	OutputSchemaVersion string
	ModelPolicyVersion  string
	OutputBudgetVersion string
	ContractBundleHash  string
	StageVersion        string
}

func HashDiscoveryAI(in DiscoveryAIInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.DiscoveryPackHash) || IsForbiddenInputHash(in.PayloadHash) || IsForbiddenInputHash(in.FactHandleMapHash) || IsForbiddenInputHash(in.RequestManifestHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                 string(stages.DiscoveryAI),
		"stage_version":         in.StageVersion,
		"discovery_pack_hash":   in.DiscoveryPackHash,
		"payload_hash":          in.PayloadHash,
		"fact_handle_map_hash":  in.FactHandleMapHash,
		"request_manifest_hash": in.RequestManifestHash,
		"prompt_policy_version": in.PromptPolicyVersion,
		"output_schema_version": in.OutputSchemaVersion,
		"model_policy_version":  in.ModelPolicyVersion,
		"output_budget_version": in.OutputBudgetVersion,
		"contract_bundle_hash":  in.ContractBundleHash,
	})
}

type ValidateDiscoveryInput struct {
	AcceptedDiscoveryOutputHash string
	DiscoveryPackHash           string
	FactHandleMapHash           string
	EvidenceMenuHash            string
	ValidationPolicyVersion     string
	OutputBudgetVersion         string
	ContractBundleHash          string
	StageVersion                string
}

func HashValidateDiscovery(in ValidateDiscoveryInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.AcceptedDiscoveryOutputHash) || IsForbiddenInputHash(in.DiscoveryPackHash) || IsForbiddenInputHash(in.FactHandleMapHash) || IsForbiddenInputHash(in.EvidenceMenuHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                          string(stages.ValidateDiscovery),
		"stage_version":                  in.StageVersion,
		"accepted_discovery_output_hash": in.AcceptedDiscoveryOutputHash,
		"discovery_pack_hash":            in.DiscoveryPackHash,
		"fact_handle_map_hash":           in.FactHandleMapHash,
		"evidence_menu_hash":             in.EvidenceMenuHash,
		"validation_policy_version":      in.ValidationPolicyVersion,
		"output_budget_version":          in.OutputBudgetVersion,
		"contract_bundle_hash":           in.ContractBundleHash,
	})
}

type ExpandEvidenceInput struct {
	AcceptedDiscoveryOutputHash string
	DiscoveryPackHash           string
	EvidenceMenuHash            string
	FactHandleMapHash           string
	FactualUniverseHash         string
	ContinuityContextHash       string
	EvidenceCatalogVersion      string
	ResolverRegistryVersion     string
	RequestNormalizationVersion string
	EvidenceEnvelopeVersion     string
	EvidenceBudgetVersion       string
	FactHandleRegistryVersion   string
	EvidenceValidationVersion   string
	ContractBundleHash          string
	StageVersion                string
}

func HashExpandEvidence(in ExpandEvidenceInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.AcceptedDiscoveryOutputHash) || IsForbiddenInputHash(in.DiscoveryPackHash) || IsForbiddenInputHash(in.FactHandleMapHash) || IsForbiddenInputHash(in.EvidenceMenuHash) || IsForbiddenInputHash(in.FactualUniverseHash) || IsForbiddenInputHash(in.ContinuityContextHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                          string(stages.ExpandEvidence),
		"stage_version":                  in.StageVersion,
		"accepted_discovery_output_hash": in.AcceptedDiscoveryOutputHash,
		"discovery_pack_hash":            in.DiscoveryPackHash,
		"evidence_menu_hash":             in.EvidenceMenuHash,
		"fact_handle_map_hash":           in.FactHandleMapHash,
		"factual_universe_hash":          in.FactualUniverseHash,
		"continuity_context_hash":        in.ContinuityContextHash,
		"evidence_catalog_version":       in.EvidenceCatalogVersion,
		"resolver_registry_version":      in.ResolverRegistryVersion,
		"request_normalization_version":  in.RequestNormalizationVersion,
		"evidence_envelope_version":      in.EvidenceEnvelopeVersion,
		"evidence_budget_version":        in.EvidenceBudgetVersion,
		"fact_handle_registry_version":   in.FactHandleRegistryVersion,
		"evidence_validation_version":    in.EvidenceValidationVersion,
		"contract_bundle_hash":           in.ContractBundleHash,
	})
}

type BuildDeepPackInput struct {
	ValidatedDiscoveryHash         string
	DiscoveryPackHash              string
	EvidenceBundleHash             string
	AnalysisFactHandleRegistryHash string
	ContinuityContextHash          string
	ActionCatalogVersion           string
	ActionMenuPolicyVersion        string
	RelationshipClaimPolicyVersion string
	DeepPackSchemaVersion          string
	DeepPackBudgetVersion          string
	ContractBundleHash             string
	StageVersion                   string
}

func HashBuildDeepPack(in BuildDeepPackInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.ValidatedDiscoveryHash) || IsForbiddenInputHash(in.DiscoveryPackHash) || IsForbiddenInputHash(in.EvidenceBundleHash) || IsForbiddenInputHash(in.AnalysisFactHandleRegistryHash) || IsForbiddenInputHash(in.ContinuityContextHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                              string(stages.BuildDeepPack),
		"stage_version":                      in.StageVersion,
		"validated_discovery_hash":           in.ValidatedDiscoveryHash,
		"discovery_pack_hash":                in.DiscoveryPackHash,
		"evidence_bundle_hash":               in.EvidenceBundleHash,
		"analysis_fact_handle_registry_hash": in.AnalysisFactHandleRegistryHash,
		"continuity_context_hash":            in.ContinuityContextHash,
		"action_catalog_version":             in.ActionCatalogVersion,
		"action_menu_policy_version":         in.ActionMenuPolicyVersion,
		"relationship_claim_policy_version":  in.RelationshipClaimPolicyVersion,
		"deep_pack_schema_version":           in.DeepPackSchemaVersion,
		"deep_pack_budget_version":           in.DeepPackBudgetVersion,
		"contract_bundle_hash":               in.ContractBundleHash,
	})
}

type DeepAIInput struct {
	DeepPackArtifactSHA    string
	PayloadHash            string
	ActionMenuHash         string
	RelationshipPolicyHash string
	PromptHash             string
	OutputSchemaHash       string
	ModelPolicyVersion     string
	AttemptPolicyVersion   string
	OutputBudgetVersion    string
	ContractBundleHash     string
	StageVersion           string
}

func HashDeepAI(in DeepAIInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.DeepPackArtifactSHA) || IsForbiddenInputHash(in.PayloadHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                    string(stages.DeepAI),
		"stage_version":            in.StageVersion,
		"deep_pack_artifact_sha":   in.DeepPackArtifactSHA,
		"payload_hash":             in.PayloadHash,
		"action_menu_hash":         in.ActionMenuHash,
		"relationship_policy_hash": in.RelationshipPolicyHash,
		"prompt_hash":              in.PromptHash,
		"output_schema_hash":       in.OutputSchemaHash,
		"model_policy_version":     in.ModelPolicyVersion,
		"attempt_policy_version":   in.AttemptPolicyVersion,
		"output_budget_version":    in.OutputBudgetVersion,
		"contract_bundle_hash":     in.ContractBundleHash,
	})
}

type ValidateDeepInput struct {
	AcceptedDeepOutputHash  string
	DeepPackHash            string
	ActionMenuHash          string
	RelationshipPolicyHash  string
	ValidationPolicyVersion string
	OutputBudgetVersion     string
	ContractBundleHash      string
	StageVersion            string
}

func HashValidateDeep(in ValidateDeepInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.AcceptedDeepOutputHash) || IsForbiddenInputHash(in.DeepPackHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage":                     string(stages.ValidateDeep),
		"stage_version":             in.StageVersion,
		"accepted_deep_output_hash": in.AcceptedDeepOutputHash,
		"deep_pack_hash":            in.DeepPackHash,
		"action_menu_hash":          in.ActionMenuHash,
		"relationship_policy_hash":  in.RelationshipPolicyHash,
		"validation_policy_version": in.ValidationPolicyVersion,
		"output_budget_version":     in.OutputBudgetVersion,
		"contract_bundle_hash":      in.ContractBundleHash,
	})
}

type PlanActionInput struct {
	ValidatedDeepHash, ContinuityContextHash, IdentityPolicy, VisibilityPolicy, UserSuppressionPolicy, RecurrencePolicy, GroupingPolicy, ContractBundleHash, StageVersion string
	BasePublicationID                                                                                                                                                     *int64
}

func HashPlanActionReconciliation(in PlanActionInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.ValidatedDeepHash) || IsForbiddenInputHash(in.ContinuityContextHash) {
		return ""
	}
	var base any
	if in.BasePublicationID != nil {
		base = *in.BasePublicationID
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.PlanActionReconciliation), "stage_version": in.StageVersion,
		"validated_deep_hash": in.ValidatedDeepHash, "continuity_context_hash": in.ContinuityContextHash,
		"base_publication_id": base, "action_identity_policy": in.IdentityPolicy,
		"visibility_policy": in.VisibilityPolicy, "user_suppression_policy": in.UserSuppressionPolicy,
		"recurrence_policy": in.RecurrencePolicy, "grouping_policy": in.GroupingPolicy,
		"contract_bundle_hash": in.ContractBundleHash,
	})
}

type BuildPublicationInput struct {
	ValidatedDeepHash, ActionPlanHash, FactualUniverseHash, ContentHashPolicy, ContractBundleHash, StageVersion string
	BasePublicationID                                                                                           *int64
}

func HashBuildPublication(in BuildPublicationInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.ValidatedDeepHash) || IsForbiddenInputHash(in.ActionPlanHash) || IsForbiddenInputHash(in.FactualUniverseHash) {
		return ""
	}
	var base any
	if in.BasePublicationID != nil {
		base = *in.BasePublicationID
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.BuildPublication), "stage_version": in.StageVersion,
		"validated_deep_hash": in.ValidatedDeepHash, "action_reconciliation_plan_hash": in.ActionPlanHash,
		"factual_universe_hash": in.FactualUniverseHash, "base_publication_id": base,
		"content_hash_policy": in.ContentHashPolicy, "contract_bundle_hash": in.ContractBundleHash,
	})
}

type BuildProjectionsInput struct {
	PublicationCandidateHash, ValidatedDeepHash, ActionPlanHash, FactualUniverseHash, ProjectionPolicy, ContractBundleHash, StageVersion string
}

func HashBuildProjections(in BuildProjectionsInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.PublicationCandidateHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.BuildProjections), "stage_version": in.StageVersion,
		"publication_candidate_hash": in.PublicationCandidateHash, "validated_deep_hash": in.ValidatedDeepHash,
		"action_plan_hash": in.ActionPlanHash, "factual_universe_hash": in.FactualUniverseHash,
		"projection_policy": in.ProjectionPolicy, "contract_bundle_hash": in.ContractBundleHash,
	})
}

type ValidatePublicationInput struct {
	PublicationCandidateHash, ProjectionManifestHash, ActionPlanHash, ValidationPolicy, ContractBundleHash, StageVersion string
}

func HashValidatePublication(in ValidatePublicationInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.PublicationCandidateHash) || IsForbiddenInputHash(in.ProjectionManifestHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.ValidatePublication), "stage_version": in.StageVersion,
		"publication_candidate_hash": in.PublicationCandidateHash, "projection_manifest_hash": in.ProjectionManifestHash,
		"action_plan_hash": in.ActionPlanHash, "validation_policy": in.ValidationPolicy,
		"contract_bundle_hash": in.ContractBundleHash,
	})
}

type PublishInputHash struct {
	ValidatedCandidateHash, ContractBundleHash, StageVersion string
	SellerID, RunID                                          int64
}

func HashPublish(in PublishInputHash) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.ValidatedCandidateHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.Publish), "stage_version": in.StageVersion,
		"validated_candidate_hash": in.ValidatedCandidateHash, "seller_account_id": in.SellerID,
		"analysis_run_id": in.RunID, "contract_bundle_hash": in.ContractBundleHash,
	})
}

type PostPublishInput struct {
	PublishOutputHash, OutboxPolicy, ContractBundleHash, StageVersion string
}

func HashPostPublish(in PostPublishInput) string {
	if in.StageVersion == "" {
		in.StageVersion = "v1"
	}
	if IsForbiddenInputHash(in.PublishOutputHash) {
		return ""
	}
	return canonicaljson.Hash(map[string]any{
		"stage": string(stages.PostPublish), "stage_version": in.StageVersion,
		"publish_output_hash": in.PublishOutputHash, "outbox_policy": in.OutboxPolicy,
		"contract_bundle_hash": in.ContractBundleHash,
	})
}

func RequireInputHash(h string) error {
	if IsForbiddenInputHash(h) {
		return fmt.Errorf("runtime: STAGE_INPUT_HASH_INVALID")
	}
	return nil
}

func materialityPolicyHash() string       { return materiality.CurrentPolicy().Hash() }
func materialityBudgetHash() string       { return materiality.DiscoveryStandardBudget().Hash() }
func materialityLaneRegistryHash() string { return materiality.LaneRegistryV2Current().Hash() }
