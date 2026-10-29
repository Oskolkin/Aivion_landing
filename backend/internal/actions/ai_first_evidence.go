package actions

import (
	"fmt"
	"strings"
)

const (
	pilotIssueNoValidEvidenceAfterDerivation = "no_valid_evidence_after_candidate_derivation"

	evidenceSourceDerivedFromCandidates  = "derived_from_selected_candidates"
	evidenceSourceAISubsetPlusDerivation = "ai_subset_plus_candidate_derivation"
	evidenceSourceAISubset               = "ai_subset"
	evidenceSourceUnavailable            = "unavailable"

	normEventEvidenceDerivedFromCandidates = "evidence_fact_ids_derived_from_source_candidates"
)

// ActionEvidenceDerivation records how canonical action evidence was built.
type ActionEvidenceDerivation struct {
	RawSourceCandidateIDs           []string `json:"raw_source_candidate_ids,omitempty"`
	ValidSourceCandidateIDs         []string `json:"valid_source_candidate_ids,omitempty"`
	UnknownCandidateIDsRemoved      []string `json:"unknown_candidate_ids_removed,omitempty"`
	AIProvidedEvidenceFactIDs       []string `json:"ai_provided_evidence_fact_ids,omitempty"`
	CandidateDerivedEvidenceFactIDs []string `json:"candidate_derived_evidence_fact_ids,omitempty"`
	CanonicalEvidenceFactIDs        []string `json:"canonical_evidence_fact_ids,omitempty"`
	EvidenceSource                  string   `json:"evidence_source,omitempty"`
	EvidenceFactCount               int      `json:"evidence_fact_count,omitempty"`
}

type actionEvidenceDerivationResult struct {
	Canonical  []string
	Derivation ActionEvidenceDerivation
	Dropped    bool
	DropReason string
	Issues     []ValidationIssue
	Events     []AIPipelineNormalizationEvent
}

func deriveAndApplyCanonicalActionEvidence(
	input ActionsAIFirstInput,
	aiProvided []string,
	rawSourceIDs, validSourceIDs, removedSourceIDs []string,
	idx int,
	actionKey string,
	candidateByID map[string]AIFirstActionCandidate,
) actionEvidenceDerivationResult {
	result := actionEvidenceDerivationResult{
		Derivation: ActionEvidenceDerivation{
			RawSourceCandidateIDs:      append([]string(nil), rawSourceIDs...),
			ValidSourceCandidateIDs:    append([]string(nil), validSourceIDs...),
			UnknownCandidateIDsRemoved: append([]string(nil), removedSourceIDs...),
			AIProvidedEvidenceFactIDs:  append([]string(nil), normalizeSourceCandidateIDs(aiProvided)...),
		},
	}

	derived := stableUniqueUnionCandidateEvidenceFactIDs(input, validSourceIDs, candidateByID)
	result.Derivation.CandidateDerivedEvidenceFactIDs = append([]string(nil), derived...)

	allowed := map[string]struct{}{}
	for _, factID := range derived {
		allowed[factID] = struct{}{}
	}

	var sanitizedAI []string
	for _, factID := range result.Derivation.AIProvidedEvidenceFactIDs {
		if !input.factExists(factID) {
			i := idx
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "unknown_evidence_fact_id_removed",
				Message:     fmt.Sprintf("unknown evidence_fact_id %q removed from AI output", factID),
				ActionIndex: &i, Field: "evidence_fact_ids",
			})
			continue
		}
		if _, ok := allowed[factID]; !ok {
			i := idx
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "evidence_fact_not_from_selected_candidates",
				Message:     fmt.Sprintf("evidence_fact_id %q removed because it is not from selected candidates", factID),
				ActionIndex: &i, Field: "evidence_fact_ids",
			})
			continue
		}
		sanitizedAI = append(sanitizedAI, factID)
	}

	canonical := derived
	source := evidenceSourceDerivedFromCandidates
	if len(sanitizedAI) > 0 {
		if len(sanitizedAI) == len(derived) && sameStringSet(sanitizedAI, derived) {
			source = evidenceSourceAISubset
		} else {
			source = evidenceSourceAISubsetPlusDerivation
		}
	} else if len(result.Derivation.AIProvidedEvidenceFactIDs) == 0 {
		source = evidenceSourceDerivedFromCandidates
	} else if len(derived) == 0 {
		source = evidenceSourceUnavailable
	}

	result.Canonical = canonical
	result.Derivation.CanonicalEvidenceFactIDs = append([]string(nil), canonical...)
	result.Derivation.EvidenceSource = source
	result.Derivation.EvidenceFactCount = len(canonical)

	if len(canonical) == 0 {
		result.Dropped = true
		result.DropReason = pilotIssueNoValidEvidenceAfterDerivation
		i := idx
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: IssueSeverityFatal, Code: pilotIssueNoValidEvidenceAfterDerivation,
			Message:     "action dropped: selected candidates contain no valid evidence facts in fact catalog",
			ActionIndex: &i, Field: "evidence_fact_ids",
		})
		return result
	}

	if len(result.Derivation.AIProvidedEvidenceFactIDs) == 0 || len(sanitizedAI) != len(result.Derivation.AIProvidedEvidenceFactIDs) || !sameStringSet(sanitizedAI, derived) {
		appendNormEvent(&result.Events, AIPipelineNormalizationEvent{
			Stage: "compose_evidence_derivation", ActionIndex: idx, ActionKey: actionKey,
			EventCode: normEventEvidenceDerivedFromCandidates, Severity: IssueSeverityWarning,
			FactIDs: append([]string(nil), canonical...),
			Details: map[string]any{
				"evidence_source":   source,
				"ai_provided_count": len(result.Derivation.AIProvidedEvidenceFactIDs),
				"derived_count":     len(derived),
			},
		})
	}

	return result
}

func stableUniqueUnionCandidateEvidenceFactIDs(
	input ActionsAIFirstInput,
	sourceIDs []string,
	candidateByID map[string]AIFirstActionCandidate,
) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, sourceID := range sourceIDs {
		c, ok := candidateByID[strings.TrimSpace(sourceID)]
		if !ok {
			continue
		}
		for _, factID := range c.EvidenceFactIDs {
			factID = strings.TrimSpace(factID)
			if factID == "" || !input.factExists(factID) {
				continue
			}
			if _, ok := seen[factID]; ok {
				continue
			}
			seen[factID] = struct{}{}
			out = append(out, factID)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
