package evidence

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type RequestSetV1 struct {
	SchemaVersion           string                              `json:"schema_version"`
	FactualUniverseHash     string                              `json:"factual_universe_hash"`
	ValidatedDiscoveryHash  string                              `json:"validated_discovery_hash"`
	CatalogVersion          string                              `json:"catalog_version"`
	ResolverRegistryVersion string                              `json:"resolver_registry_version"`
	NormalizationVersion    string                              `json:"normalization_version"`
	RawRequestCount         int                                 `json:"raw_request_count"`
	UniqueRequestCount      int                                 `json:"unique_request_count"`
	Requests                []NormalizedEvidenceRequest         `json:"requests"`
	ThreadRelations         []ThreadRequestRelation             `json:"thread_request_relations"`
	ConsumerRelations       []EvidenceConsumerRelation          `json:"consumer_relations"`
	ContinuityRelations     []ContinuitySupportEvidenceRelation `json:"continuity_support_evidence_relations"`
}

type ThreadRequestRelation struct {
	ThreadID   string `json:"thread_id"`
	RequestKey string `json:"request_key"`
	EvidenceID string `json:"evidence_id"`
}

type ContinuitySupportEvidenceRelation struct {
	PreviousCaseRef     string `json:"previous_case_ref"`
	PreviousProposalRef string `json:"previous_proposal_ref"`
	SupportKey          string `json:"support_key"`
	EvidenceID          string `json:"evidence_id"`
}

type EvidenceConsumerRelation struct {
	Source              string  `json:"source"`
	RequestKey          string  `json:"request_key"`
	EvidenceID          string  `json:"evidence_id"`
	ThreadRef           *string `json:"thread_ref,omitempty"`
	PreviousCaseRef     *string `json:"previous_case_ref,omitempty"`
	PreviousProposalRef *string `json:"previous_proposal_ref,omitempty"`
	SupportKey          *string `json:"support_key,omitempty"`
}

type BundleSummary struct {
	Requested  int `json:"requested"`
	Unique     int `json:"unique"`
	Complete   int `json:"complete"`
	EmptyValid int `json:"empty_valid"`
	Partial    int `json:"partial"`
	Failed     int `json:"failed"`
}

type BundleV1 struct {
	SchemaVersion              string                              `json:"schema_version"`
	FactualUniverseHash        string                              `json:"factual_universe_hash"`
	AnalysisDate               string                              `json:"analysis_date"`
	AnalysisCutoffAt           string                              `json:"analysis_cutoff_at"`
	CatalogVersion             string                              `json:"catalog_version"`
	ResolverRegistryVersion    string                              `json:"resolver_registry_version"`
	BudgetProfileVersion       string                              `json:"budget_profile_version"`
	FactHandleRegistryVersion  string                              `json:"fact_handle_registry_version"`
	Items                      []EvidenceItem                      `json:"items"`
	ThreadEvidenceRelations    []ThreadRequestRelation             `json:"thread_evidence_relations"`
	ContinuitySupportRelations []ContinuitySupportEvidenceRelation `json:"continuity_support_evidence_relations"`
	Summary                    BundleSummary                       `json:"summary"`
}

func (b BundleV1) Hash() string { return canonicaljson.Hash(b) }

type ExpansionReportV1 struct {
	SchemaVersion        string         `json:"schema_version"`
	RawRequests          int            `json:"raw_requests"`
	UniqueRequests       int            `json:"unique_requests"`
	DedupRatio           string         `json:"dedup_ratio"`
	ItemsComplete        int            `json:"items_complete"`
	ItemsEmptyValid      int            `json:"items_empty_valid"`
	ItemsPartial         int            `json:"items_partial"`
	ItemsFailed          int            `json:"items_failed"`
	ItemsByType          map[string]int `json:"items_by_type"`
	Bytes                int            `json:"bytes"`
	EstimatedTokens      int            `json:"estimated_tokens"`
	TruncationCounts     int            `json:"truncation_counts"`
	ResolverTimingsMS    map[string]int `json:"resolver_timings_ms"`
	FactualAPICallCounts map[string]int `json:"factual_api_call_counts"`
}

type UnifiedHandleRegistryV1 struct {
	SchemaVersion   string          `json:"schema_version"`
	RegistryVersion string          `json:"registry_version"`
	Entries         []UnifiedHandle `json:"entries"`
}

type UnifiedHandle struct {
	Handle         string    `json:"handle"`
	Origin         string    `json:"origin"`
	EvidenceID     string    `json:"evidence_id,omitempty"`
	Entity         EntityRef `json:"entity"`
	SemanticPath   string    `json:"semantic_path"`
	FactRefs       []string  `json:"fact_refs"`
	FormulaKey     string    `json:"formula_key,omitempty"`
	FormulaVersion string    `json:"formula_version,omitempty"`
	InputFactRefs  []string  `json:"input_fact_refs,omitempty"`
}
