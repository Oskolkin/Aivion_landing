package dashboardhome

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

// ChunkCandidateRejection records a single rejected candidate with structured diagnostics.
type ChunkCandidateRejection struct {
	ChunkID            string   `json:"chunk_id"`
	CandidateIndex     int      `json:"candidate_index"`
	CandidateID        string   `json:"candidate_id,omitempty"`
	CandidateType      string   `json:"candidate_type,omitempty"`
	SourceSignalRefs   []string `json:"source_signal_refs,omitempty"`
	EvidenceFactRefs   []string `json:"evidence_fact_refs,omitempty"`
	AffectedEntityRefs []string `json:"affected_entity_refs,omitempty"`
	RejectCode         string   `json:"reject_code"`
	RejectField        string   `json:"reject_field,omitempty"`
	RawValue           string   `json:"raw_value,omitempty"`
	AllowedValues      string   `json:"allowed_values_or_refs,omitempty"`
	Repairable         bool     `json:"repairable_reference_error"`
	Category           string   `json:"category"`
}

// ChunkCandidateRejectionReport aggregates candidate accept/reject statistics.
type ChunkCandidateRejectionReport struct {
	CandidateTotal    int                       `json:"candidate_total"`
	Accepted          int                       `json:"accepted"`
	Rejected          int                       `json:"rejected"`
	Repaired          int                       `json:"repaired"`
	RejectionsByCode  map[string]int            `json:"rejections_by_code"`
	RejectionsByChunk map[string]int            `json:"rejections_by_chunk"`
	Rejections        []ChunkCandidateRejection `json:"rejections,omitempty"`
}

const (
	candidateRejectCategoryLegitimate      = "legitimate_technical_rejection"
	candidateRejectCategoryRepairable      = "repairable_reference_error"
	candidateRejectCategoryPromptMismatch  = "prompt_contract_mismatch"
	candidateRejectCategoryLocalRef        = "local_ref_mapping_error"
	candidateRejectCategoryFalsePositive   = "validator_false_positive"
	candidateRejectCategorySemanticWarning = "semantic_warning_only"
	candidateRejectCategoryLegacySchema    = "legacy_schema_mismatch"
	candidateRejectCategoryUnknown         = "unknown"
)

func classifyCandidateRejectCategory(code string, repairable bool) string {
	switch code {
	case "candidate_unknown_fact_id", "candidate_unknown_entity_id", "candidate_unknown_source_signal_id":
		if repairable {
			return candidateRejectCategoryRepairable
		}
		return candidateRejectCategoryLegitimate
	case "candidate_entity_type_mismatch":
		return candidateRejectCategoryRepairable
	case "candidate_no_affected_entities":
		return candidateRejectCategoryPromptMismatch
	case "candidate_no_evidence_facts", "no_valid_evidence_after_normalization", "no_valid_source_signals_after_normalization":
		return candidateRejectCategoryLegitimate
	case "candidate_unknown_action_type", "candidate_invalid_local_priority", "candidate_problem_empty",
		"candidate_recommended_action_empty", "candidate_why_actionable_empty":
		return candidateRejectCategoryLegitimate
	case "candidate_exceeds_chunk_limit", "candidate_too_many_entities", "candidate_too_many_facts":
		return candidateRejectCategoryLegitimate
	case "metric_semantic_mismatch", "unsupported_placeholder_claim":
		return candidateRejectCategorySemanticWarning
	default:
		if repairable {
			return candidateRejectCategoryRepairable
		}
		return candidateRejectCategoryUnknown
	}
}

func buildCandidateRejectionReport(
	chunkID string,
	raw []actions.AIFirstActionCandidate,
	outcomes []chunkCandidateOutcome,
) ChunkCandidateRejectionReport {
	report := ChunkCandidateRejectionReport{
		CandidateTotal:    len(raw),
		RejectionsByCode:  map[string]int{},
		RejectionsByChunk: map[string]int{},
	}
	for i, o := range outcomes {
		if o.Accepted {
			report.Accepted++
			if o.Repaired {
				report.Repaired++
			}
			continue
		}
		report.Rejected++
		report.RejectionsByCode[o.RejectCode]++
		report.RejectionsByChunk[chunkID]++
		rawCandidate := actions.AIFirstActionCandidate{}
		if i < len(raw) {
			rawCandidate = raw[i]
		}
		report.Rejections = append(report.Rejections, ChunkCandidateRejection{
			ChunkID:            chunkID,
			CandidateIndex:     i + 1,
			CandidateID:        rawCandidate.CandidateID,
			CandidateType:      string(rawCandidate.Type),
			SourceSignalRefs:   append([]string(nil), rawCandidate.SourceSignalIDs...),
			EvidenceFactRefs:   append([]string(nil), rawCandidate.EvidenceFactIDs...),
			AffectedEntityRefs: entityIDsFromCandidate(rawCandidate),
			RejectCode:         o.RejectCode,
			RejectField:        o.RejectField,
			RawValue:           o.RawValue,
			AllowedValues:      o.AllowedValues,
			Repairable:         o.Repairable,
			Category:           classifyCandidateRejectCategory(o.RejectCode, o.Repairable),
		})
	}
	return report
}

func entityIDsFromCandidate(c actions.AIFirstActionCandidate) []string {
	out := make([]string, 0, len(c.AffectedEntities))
	for _, ent := range c.AffectedEntities {
		if id := strings.TrimSpace(ent.EntityID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

type chunkCandidateOutcome struct {
	Accepted      bool
	Repaired      bool
	Candidate     actions.AIFirstActionCandidate
	RejectCode    string
	RejectField   string
	RawValue      string
	AllowedValues string
	Repairable    bool
	Warnings      []string
}

func isAccountLevelEntityID(entityID string) bool {
	switch strings.TrimSpace(entityID) {
	case "account:seller", "funnel:account":
		return true
	default:
		return false
	}
}

func isAccountLevelCandidateType(t actions.ActionType) bool {
	switch t {
	case actions.ActionTypeSalesDropReview:
		return true
	default:
		return false
	}
}
