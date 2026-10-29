package discovery

type OutputV1 struct {
	DiscoveryState DiscoveryState `json:"discovery_state"`
	Threads        []ThreadV1     `json:"threads"`
}

type ThreadV1 struct {
	PrimaryEntity      PackEntity        `json:"primary_entity"`
	RelatedEntities    []PackEntity      `json:"related_entities"`
	Observation        string            `json:"observation"`
	InvestigationFocus string            `json:"investigation_focus"`
	BasisFactHandles   []string          `json:"basis_fact_handles"`
	EvidenceRequests   []EvidenceRequest `json:"evidence_requests"`
}

type EvidenceRequest struct {
	EvidenceType          string      `json:"evidence_type"`
	Subject               PackEntity  `json:"subject"`
	RelatedEntity         *PackEntity `json:"related_entity"`
	InvestigationQuestion string      `json:"investigation_question"`
}

type ValidatedOutputV1 struct {
	SchemaVersion  string            `json:"schema_version"`
	DiscoveryState DiscoveryState    `json:"discovery_state"`
	Threads        []ValidatedThread `json:"threads"`
}

type ValidatedThread struct {
	ThreadID           string            `json:"thread_id"`
	Ordinal            int               `json:"ordinal"`
	PrimaryEntity      PackEntity        `json:"primary_entity"`
	RelatedEntities    []PackEntity      `json:"related_entities"`
	Observation        string            `json:"observation"`
	InvestigationFocus string            `json:"investigation_focus"`
	BasisFactHandles   []string          `json:"basis_fact_handles"`
	EvidenceRequests   []EvidenceRequest `json:"evidence_requests"`
}

type ValidationReportV1 struct {
	SchemaVersion    string   `json:"schema_version"`
	Valid            bool     `json:"valid"`
	Errors           []string `json:"errors"`
	ThreadCount      int      `json:"thread_count"`
	EvidenceRequests int      `json:"evidence_requests"`
}

func AssignThreadIDs(out OutputV1) ValidatedOutputV1 {
	v := ValidatedOutputV1{SchemaVersion: ValidatedOutputSchemaV1, DiscoveryState: out.DiscoveryState, Threads: []ValidatedThread{}}
	for i, t := range out.Threads {
		v.Threads = append(v.Threads, ValidatedThread{
			ThreadID: formatThreadID(i + 1), Ordinal: i + 1,
			PrimaryEntity: t.PrimaryEntity, RelatedEntities: t.RelatedEntities,
			Observation: t.Observation, InvestigationFocus: t.InvestigationFocus,
			BasisFactHandles: t.BasisFactHandles, EvidenceRequests: t.EvidenceRequests,
		})
	}
	return v
}

func formatThreadID(n int) string {
	return "T" + pad3(n)
}

func pad3(n int) string {
	s := []byte{'0', '0', '0'}
	for i := 2; n > 0 && i >= 0; i-- {
		s[i] = byte('0' + n%10)
		n /= 10
	}
	return string(s)
}
