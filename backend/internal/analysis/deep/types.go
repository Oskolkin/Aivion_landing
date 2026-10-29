package deep

import (
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
)

type EntityRef = discovery.PackEntity

type PayloadV1 struct {
	AnalysisContext                PayloadContext                  `json:"analysis_context"`
	DomainWindows                  []discovery.PayloadWindow       `json:"domain_windows"`
	CapabilitiesAndLimitations     []discovery.PayloadCapability   `json:"capabilities_and_limitations"`
	AccountContext                 discovery.PayloadAccount        `json:"account_context"`
	DiscoveryResult                DiscoveryResultView             `json:"discovery_result"`
	ThreadEntityContext            []EntityContext                 `json:"thread_entity_context"`
	EvidenceRegistry               []evidence.EvidenceItem         `json:"evidence_registry"`
	ThreadEvidenceLinks            []ThreadEvidenceLink            `json:"thread_evidence_links"`
	ContinuitySupportEvidenceLinks []ContinuitySupportEvidenceLink `json:"continuity_support_evidence_links"`
	FactRegistry                   []FactProjection                `json:"fact_registry"`
	RelationshipClaimPolicy        RelationshipPolicyV1            `json:"relationship_claim_policy"`
	ActionMenu                     actioncatalog.Menu              `json:"action_menu"`
	Continuity                     ContinuityView                  `json:"continuity"`
}

type PayloadContext struct {
	AnalysisDate     string `json:"analysis_date"`
	AnalysisCutoffAt string `json:"analysis_cutoff_at"`
	BusinessTimezone string `json:"business_timezone"`
}

type DiscoveryResultView struct {
	DiscoveryState string       `json:"discovery_state"`
	Threads        []ThreadView `json:"threads"`
}

type ThreadView struct {
	ThreadRef          string      `json:"thread_ref"`
	PrimaryEntity      EntityRef   `json:"primary_entity"`
	RelatedEntities    []EntityRef `json:"related_entities"`
	Observation        string      `json:"observation"`
	InvestigationFocus string      `json:"investigation_focus"`
	BasisFactHandles   []string    `json:"basis_fact_handles"`
}

type EntityContext struct {
	Entity EntityRef `json:"entity"`
	Role   string    `json:"role"`
}

type ThreadEvidenceLink struct {
	ThreadRef    string   `json:"thread_ref"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type ContinuitySupportEvidenceLink struct {
	PreviousCaseRef     string `json:"previous_case_ref"`
	PreviousProposalRef string `json:"previous_proposal_ref"`
	SupportKey          string `json:"support_key"`
	EvidenceID          string `json:"evidence_id"`
}

type FactProjection struct {
	Handle                   string    `json:"handle"`
	Entity                   EntityRef `json:"entity"`
	SemanticPath             string    `json:"semantic_path"`
	SourceKind               string    `json:"source_kind"`
	SourceRef                string    `json:"source_ref"`
	AnalyticalUsability      string    `json:"analytical_usability"`
	AllowedRelationshipBases []string  `json:"allowed_relationship_bases"`
}

type ContinuityView struct {
	Mode            string           `json:"mode"`
	PreviousActions []PreviousAction `json:"previous_actions"`
}

type PreviousAction struct {
	PreviousActionRef string                   `json:"previous_action_ref"`
	CaseRef           string                   `json:"case_ref,omitempty"`
	ProposalRef       string                   `json:"proposal_ref,omitempty"`
	SignatureHash     string                   `json:"signature_hash,omitempty"`
	ActionType        string                   `json:"action_type,omitempty"`
	Target            EntityRef                `json:"target,omitempty"`
	NormalizedContext map[string]string        `json:"normalized_context,omitempty"`
	IntentSignature   string                   `json:"intent_signature,omitempty"`
	Parameters        actioncatalog.Parameters `json:"parameters,omitempty"`
	RequiredSupports  []PackedSupport          `json:"required_supports"`
	ContextSupports   []PackedSupport          `json:"context_supports,omitempty"`
	CounterSupports   []PackedSupport          `json:"counterevidence_supports,omitempty"`
	SystemState       string                   `json:"system_state,omitempty"`
	UserState         string                   `json:"user_state,omitempty"`
	SuppressedUntil   string                   `json:"suppressed_until,omitempty"`
}

type SupportRevalidationSpec struct {
	EvidenceType  string     `json:"evidence_type"`
	Subject       EntityRef  `json:"subject"`
	RelatedEntity *EntityRef `json:"related_entity,omitempty"`
}

type PackedSupport struct {
	SupportKey    string                  `json:"support_key"`
	Role          string                  `json:"role,omitempty"`
	Claim         string                  `json:"claim"`
	FindingRefs   []string                `json:"finding_refs,omitempty"`
	FactHandles   []string                `json:"fact_handles"`
	EvidenceTypes []string                `json:"evidence_types,omitempty"`
	Revalidation  SupportRevalidationSpec `json:"revalidation,omitempty"`
}

type AuditEnvelope struct {
	SchemaVersion                   string `json:"schema_version"`
	RunID                           int64  `json:"run_id"`
	SellerAccountID                 int64  `json:"seller_account_id"`
	FactualUniverseHash             string `json:"factual_universe_hash"`
	DiscoveryPackArtifactSHA        string `json:"discovery_pack_artifact_sha"`
	ValidatedDiscoveryArtifactSHA   string `json:"validated_discovery_artifact_sha"`
	EvidenceBundleArtifactSHA       string `json:"evidence_bundle_artifact_sha"`
	AnalysisFactRegistryArtifactSHA string `json:"analysis_fact_registry_artifact_sha"`
	ContinuityArtifactSHA           string `json:"continuity_artifact_sha"`
	ActionCatalogVersion            string `json:"action_catalog_version"`
	ActionMenuHash                  string `json:"action_menu_hash"`
	RelationshipClaimPolicyVersion  string `json:"relationship_claim_policy_version"`
	DeepPackBudgetVersion           string `json:"deep_pack_budget_version"`
	PayloadHash                     string `json:"payload_hash"`
	EstimatedInputTokens            int    `json:"estimated_input_tokens"`
	ContractBundleHash              string `json:"contract_bundle_hash"`
}

type PackWireV1 struct {
	AuditEnvelope AuditEnvelope `json:"AuditEnvelope"`
	Payload       PayloadV1     `json:"payload"`
}

type OutputV1 struct {
	ExecutiveSummary          ExecutiveSummary           `json:"executive_summary"`
	ThreadResolutions         []ThreadResolution         `json:"thread_resolutions"`
	Findings                  []Finding                  `json:"findings"`
	Recommendations           []Recommendation           `json:"recommendations"`
	ActionProposals           []ActionProposal           `json:"action_proposals"`
	PreviousActionAssessments []PreviousActionAssessment `json:"previous_action_assessments"`
}

type ExecutiveSummary struct {
	Headline                  string     `json:"headline"`
	Summary                   string     `json:"summary"`
	KeyTakeaways              []Takeaway `json:"key_takeaways"`
	BasisFactHandles          []string   `json:"basis_fact_handles"`
	ReferencedFindingOrdinals []int      `json:"referenced_finding_ordinals"`
	Limitations               []string   `json:"limitations"`
}

type Takeaway struct {
	Statement             string   `json:"statement"`
	Direction             string   `json:"direction"`
	PrimaryDomain         string   `json:"primary_domain"`
	SupportingFactHandles []string `json:"supporting_fact_handles"`
	FindingOrdinals       []int    `json:"finding_ordinals"`
}

type ThreadResolution struct {
	DiscoveryThreadRef       string   `json:"discovery_thread_ref"`
	Resolution               string   `json:"resolution"`
	ResolutionSummary        string   `json:"resolution_summary"`
	SupportingFactHandles    []string `json:"supporting_fact_handles"`
	CounterFactHandles       []string `json:"counter_fact_handles"`
	Limitations              []string `json:"limitations"`
	ResultingFindingOrdinals []int    `json:"resulting_finding_ordinals"`
}

type Finding struct {
	Ordinal                 int                      `json:"ordinal"`
	OriginType              string                   `json:"origin_type"`
	PrimaryEntity           EntityRef                `json:"primary_entity"`
	RelatedEntities         []EntityRef              `json:"related_entities"`
	Title                   string                   `json:"title"`
	FindingStatement        string                   `json:"finding_statement"`
	BusinessDirection       string                   `json:"business_direction"`
	PrimaryDomain           string                   `json:"primary_domain"`
	OverviewBucket          string                   `json:"overview_bucket"`
	RelationshipAssessments []RelationshipAssessment `json:"relationship_assessments"`
	SupportingFactHandles   []string                 `json:"supporting_fact_handles"`
	CounterFactHandles      []string                 `json:"counter_fact_handles"`
	Limitations             []string                 `json:"limitations"`
	SourceThreadRefs        []string                 `json:"source_thread_refs"`
}

type RelationshipAssessment struct {
	RelationshipStatement   string   `json:"relationship_statement"`
	RelationshipBasis       string   `json:"relationship_basis"`
	DriverOrFactor          string   `json:"driver_or_factor"`
	Outcome                 string   `json:"outcome"`
	SupportingFactHandles   []string `json:"supporting_fact_handles"`
	CounterFactHandles      []string `json:"counter_fact_handles"`
	Limitations             []string `json:"limitations"`
	AlternativeExplanations []string `json:"alternative_explanations"`
}

type Recommendation struct {
	Ordinal                 int       `json:"ordinal"`
	TargetEntity            EntityRef `json:"target_entity"`
	RecommendationStatement string    `json:"recommendation_statement"`
	Rationale               string    `json:"rationale"`
	SupportFindingOrdinals  []int     `json:"support_finding_ordinals"`
	SupportFactHandles      []string  `json:"support_fact_handles"`
	Limitations             []string  `json:"limitations"`
	Actionability           string    `json:"actionability"`
}

type ActionProposal struct {
	Ordinal               int                      `json:"ordinal"`
	ActionType            string                   `json:"action_type"`
	Target                EntityRef                `json:"target"`
	Parameters            actioncatalog.Parameters `json:"parameters"`
	RecommendationOrdinal int                      `json:"recommendation_ordinal"`
	FindingOrdinals       []int                    `json:"finding_ordinals"`
	FactHandles           []string                 `json:"fact_handles"`
	Supports              []ActionSupportBasis     `json:"supports"`
}

type ActionSupportBasis struct {
	SupportKey      string                  `json:"support_key"`
	Ordinal         int                     `json:"ordinal"`
	Role            string                  `json:"role"`
	Claim           string                  `json:"claim"`
	FindingOrdinals []int                   `json:"finding_ordinals"`
	FactHandles     []string                `json:"fact_handles"`
	Revalidation    SupportRevalidationSpec `json:"revalidation"`
}

type PreviousActionAssessment struct {
	PreviousActionRef               string                      `json:"previous_action_ref"`
	Assessment                      string                      `json:"assessment"`
	AssessmentSummary               string                      `json:"assessment_summary"`
	SupportingFactHandles           []string                    `json:"supporting_fact_handles"`
	CounterFactHandles              []string                    `json:"counter_fact_handles"`
	Limitations                     []string                    `json:"limitations"`
	ResultingActionProposalOrdinals []int                       `json:"resulting_action_proposal_ordinals"`
	SupportAssessments              []PreviousSupportAssessment `json:"support_assessments"`
}

type PreviousSupportAssessment struct {
	SupportKey         string   `json:"support_key"`
	Status             string   `json:"status"`
	CurrentFactHandles []string `json:"current_fact_handles"`
	AssessmentSummary  string   `json:"assessment_summary"`
}

type ValidatedOutputV1 struct {
	SchemaVersion      string   `json:"schema_version"`
	Output             OutputV1 `json:"output"`
	FindingRefs        []string `json:"finding_refs"`
	RecommendationRefs []string `json:"recommendation_refs"`
	ActionProposalRefs []string `json:"action_proposal_refs"`
	SupportRefs        []string `json:"support_refs,omitempty"`
}

type ValidationReportV1 struct {
	SchemaVersion string   `json:"schema_version"`
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors"`
}

func (r ValidationReportV1) MarshalJSON() ([]byte, error) {
	type alias ValidationReportV1
	if r.Errors == nil {
		r.Errors = []string{}
	}
	return json.Marshal(alias(r))
}

type RequestManifestV1 struct {
	RequestContractVersion  string `json:"request_contract_version"`
	Model                   string `json:"model"`
	ReasoningEffort         string `json:"reasoning_effort,omitempty"`
	Verbosity               string `json:"verbosity,omitempty"`
	ModelPolicyVersion      string `json:"model_policy_version"`
	PromptPolicyVersion     string `json:"prompt_policy_version"`
	PromptHash              string `json:"prompt_hash"`
	OutputSchemaVersion     string `json:"output_schema_version"`
	OutputSchemaHash        string `json:"output_schema_hash"`
	SchemaName              string `json:"schema_name"`
	Strict                  bool   `json:"strict"`
	PackHash                string `json:"pack_hash"`
	PayloadHash             string `json:"payload_hash"`
	ActionMenuHash          string `json:"action_menu_hash"`
	RelationshipPolicyHash  string `json:"relationship_policy_hash"`
	RequestHash             string `json:"request_hash"`
	MaxOutputTokens         int    `json:"max_output_tokens"`
	TruncationMode          string `json:"truncation_mode"`
	Store                   bool   `json:"store"`
	ToolsEnabled            bool   `json:"tools_enabled"`
	PreviousResponseEnabled bool   `json:"previous_response_enabled"`
	EstimatedInputTokens    int    `json:"estimated_input_tokens"`
}
