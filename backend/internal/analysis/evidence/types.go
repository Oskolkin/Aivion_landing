package evidence

import (
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
)

type EntityRef = discovery.PackEntity

type ThreadEvidenceRef struct {
	ThreadID               string   `json:"thread_id"`
	InvestigationQuestions []string `json:"investigation_questions"`
}

type EvidenceConsumerRef struct {
	Source              string  `json:"source"`
	ThreadRef           *string `json:"thread_ref,omitempty"`
	PreviousCaseRef     *string `json:"previous_case_ref,omitempty"`
	PreviousProposalRef *string `json:"previous_proposal_ref,omitempty"`
	SupportKey          *string `json:"support_key,omitempty"`
}

type NormalizedEvidenceRequest struct {
	RequestKey      string
	EvidenceType    string
	EvidenceVersion string
	Subject         EntityRef
	Related         *EntityRef
	ThreadRefs      []ThreadEvidenceRef
	ConsumerRefs    []EvidenceConsumerRef
	Source          string
	Ordinal         int
}

type TemporalScope struct {
	ProfileKey string `json:"profile_key"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

type Coverage struct {
	MappedShare *string  `json:"mapped_share,omitempty"`
	Unmapped    *string  `json:"unmapped_context,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type Limitation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type TruncationMeta struct {
	TotalKnown    *int   `json:"total_known,omitempty"`
	Returned      int    `json:"returned"`
	Truncated     bool   `json:"truncated"`
	SortBasis     string `json:"sort_basis,omitempty"`
	SemanticScope string `json:"semantic_scope,omitempty"`
}

type EvidenceItem struct {
	EvidenceID            string          `json:"evidence_id"`
	EvidenceType          string          `json:"evidence_type"`
	EvidenceTypeVersion   string          `json:"evidence_type_version"`
	Subject               EntityRef       `json:"subject"`
	Related               *EntityRef      `json:"related_entity,omitempty"`
	Readiness             string          `json:"readiness"`
	MaterializationStatus string          `json:"materialization_status"`
	TemporalScope         TemporalScope   `json:"temporal_scope"`
	Coverage              Coverage        `json:"coverage"`
	Limitations           []Limitation    `json:"limitations"`
	Truncation            TruncationMeta  `json:"truncation"`
	Data                  json.RawMessage `json:"data"`
	FactHandles           []string        `json:"fact_handles"`
}

type DeclaredFact struct {
	SemanticPath string
	FactRefs     []string
	Derivation   *discovery.Derivation
}

type MaterializationResult struct {
	Request     NormalizedEvidenceRequest
	Status      string
	Readiness   string
	Temporal    TemporalScope
	Coverage    Coverage
	Limitations []Limitation
	Truncation  TruncationMeta
	Data        any
	Facts       []DeclaredFact
	Integrity   error
	APICalls    int
}

func CatalogType(key string) (evidencecatalog.Type, bool) {
	t, ok := evidencecatalog.ByKey()[key]
	return t, ok
}
