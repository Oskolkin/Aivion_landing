package actions

import "strings"

// BuildActionLifecycleDiagnostics constructs per-action lifecycle records and summary counts.
func BuildActionLifecycleDiagnostics(
	composeOutput *ActionsAIFirstComposeOutput,
	baseActions []AIFirstBaseAction,
	pilot PilotActionsValidationResult,
	persist aiFirstPersistGuardResult,
) (ActionsLifecycleSummary, []ActionLifecycleDiagnostic) {
	summary := ActionsLifecycleSummary{}
	lifecycles := make([]ActionLifecycleDiagnostic, 0)

	if composeOutput != nil {
		summary.ProviderReturnedCount = len(composeOutput.Actions)
		summary.ParsedCount = len(composeOutput.Actions)
	}

	persistedKeys := map[string]bool{}
	for _, item := range persist.Items {
		if item.ActionKey != "" {
			persistedKeys[item.ActionKey] = true
		}
	}
	droppedByIndex := map[int]string{}
	for _, base := range baseActions {
		if base.Dropped {
			droppedByIndex[base.AIIndex] = base.DropReason
		}
	}

	for i, action := range composeOutputActions(composeOutput) {
		lc := ActionLifecycleDiagnostic{
			RawActionIndex:  i,
			ActionKey:       action.ActionKey,
			ParseOutcome:    "parsed",
			RawCandidateIDs: append([]string(nil), action.SourceCandidateIDs...),
		}
		if base, ok := baseActionByIndex(baseActions, i); ok {
			lc.ValidCandidateIDs = append([]string(nil), base.ValidSourceCandidates...)
			lc.UnknownCandidateIDs = append([]string(nil), base.RemovedCandidateIDs...)
			lc.AuthoritativeEntityIDs = entityIDsFromActionRefs(base.AuthoritativeEntities)
			lc.RawSourceCandidateIDs = append([]string(nil), base.EvidenceDerivation.RawSourceCandidateIDs...)
			if len(lc.RawSourceCandidateIDs) == 0 {
				lc.RawSourceCandidateIDs = append([]string(nil), lc.RawCandidateIDs...)
			}
			lc.ValidSourceCandidateIDs = append([]string(nil), base.EvidenceDerivation.ValidSourceCandidateIDs...)
			if len(lc.ValidSourceCandidateIDs) == 0 {
				lc.ValidSourceCandidateIDs = append([]string(nil), base.ValidSourceCandidates...)
			}
			lc.UnknownCandidateIDsRemoved = append([]string(nil), base.EvidenceDerivation.UnknownCandidateIDsRemoved...)
			lc.AIProvidedEvidenceFactIDs = append([]string(nil), base.EvidenceDerivation.AIProvidedEvidenceFactIDs...)
			lc.CandidateDerivedEvidenceFactIDs = append([]string(nil), base.EvidenceDerivation.CandidateDerivedEvidenceFactIDs...)
			lc.CanonicalEvidenceFactIDs = append([]string(nil), base.EvidenceDerivation.CanonicalEvidenceFactIDs...)
			lc.EvidenceSource = base.EvidenceDerivation.EvidenceSource
			lc.EvidenceFactCount = base.EvidenceDerivation.EvidenceFactCount
			if base.Dropped {
				lc.NormalizationOutcome = "dropped"
				lc.DropStage = "normalization"
				lc.DropCode = base.DropReason
				lc.DropReason = base.DropReason
				summary.DroppedCount++
			} else {
				lc.NormalizationOutcome = "normalized"
				summary.NormalizedCount++
			}
		} else if reason, dropped := droppedByIndex[i]; dropped {
			lc.NormalizationOutcome = "dropped"
			lc.DropStage = "normalization"
			lc.DropCode = reason
			lc.DropReason = reason
			summary.DroppedCount++
		} else {
			lc.NormalizationOutcome = "normalized"
			summary.NormalizedCount++
		}

		if lc.NormalizationOutcome != "dropped" {
			summary.CandidateValidCount++
			if isActionAcceptedAtIndex(pilot, i) {
				lc.ValidationOutcome = "accepted"
				summary.ValidatedCount++
			} else {
				lc.ValidationOutcome = "dropped"
				lc.DropStage = "validation"
				lc.DropCode = "validation_dropped"
				summary.DroppedCount++
			}
		}

		if lc.ValidationOutcome == "accepted" {
			summary.AfterLimitCount++
		}

		if lc.ActionKey != "" && persistedKeys[lc.ActionKey] {
			lc.PersistOutcome = "persisted"
			summary.PersistedCount++
		} else if lc.NormalizationOutcome != "dropped" && lc.ValidationOutcome == "accepted" {
			lc.PersistOutcome = "dropped"
			lc.DropStage = "persist_guard"
			lc.DropCode = "persist_guard_rejected"
			summary.DroppedCount++
		} else if lc.NormalizationOutcome == "dropped" || lc.ValidationOutcome == "dropped" {
			lc.PersistOutcome = "dropped"
		}

		lifecycles = append(lifecycles, lc)
	}

	accounted := summary.PersistedCount + countDroppedDuringNormalization(lifecycles) +
		countDroppedDuringValidation(lifecycles) + countDroppedByPersistGuard(lifecycles)
	if summary.ParsedCount > 0 && accounted != summary.ParsedCount {
		summary.CountMismatch = true
		summary.CountMismatchCode = "actions_lifecycle_count_mismatch"
	}
	return summary, lifecycles
}

func composeOutputActions(compose *ActionsAIFirstComposeOutput) []AIFirstComposeAction {
	if compose == nil {
		return nil
	}
	return compose.Actions
}

func baseActionByIndex(baseActions []AIFirstBaseAction, idx int) (AIFirstBaseAction, bool) {
	for _, b := range baseActions {
		if b.AIIndex == idx {
			return b, true
		}
	}
	return AIFirstBaseAction{}, false
}

func entityIDsFromActionRefs(refs []AIFirstAffectedEntityRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.EntityID != "" {
			out = append(out, r.EntityID)
		}
	}
	return out
}

func isActionAcceptedAtIndex(pilot PilotActionsValidationResult, idx int) bool {
	for _, d := range pilot.Dropped {
		if d.OriginalIndex == idx {
			return false
		}
	}
	return idx < pilot.ReturnedCount
}

func countDroppedDuringNormalization(lcs []ActionLifecycleDiagnostic) int {
	n := 0
	for _, lc := range lcs {
		if lc.DropStage == "normalization" {
			n++
		}
	}
	return n
}

func countDroppedDuringValidation(lcs []ActionLifecycleDiagnostic) int {
	n := 0
	for _, lc := range lcs {
		if lc.DropStage == "validation" {
			n++
		}
	}
	return n
}

func countDroppedByPersistGuard(lcs []ActionLifecycleDiagnostic) int {
	n := 0
	for _, lc := range lcs {
		if lc.DropStage == "persist_guard" {
			n++
		}
	}
	return n
}

// ActionLifecycleDiagnostic tracks one Action through pipeline stages.
type ActionLifecycleDiagnostic struct {
	RawActionIndex                  int      `json:"raw_action_index"`
	ActionKey                       string   `json:"action_key,omitempty"`
	ParseOutcome                    string   `json:"parse_outcome"`
	NormalizationOutcome            string   `json:"normalization_outcome"`
	ValidationOutcome               string   `json:"validation_outcome"`
	PersistOutcome                  string   `json:"persist_outcome"`
	RawCandidateIDs                 []string `json:"raw_candidate_ids,omitempty"`
	ValidCandidateIDs               []string `json:"valid_candidate_ids,omitempty"`
	UnknownCandidateIDs             []string `json:"unknown_candidate_ids,omitempty"`
	RawSourceCandidateIDs           []string `json:"raw_source_candidate_ids,omitempty"`
	ValidSourceCandidateIDs         []string `json:"valid_source_candidate_ids,omitempty"`
	UnknownCandidateIDsRemoved      []string `json:"unknown_candidate_ids_removed,omitempty"`
	AIProvidedEvidenceFactIDs       []string `json:"ai_provided_evidence_fact_ids,omitempty"`
	CandidateDerivedEvidenceFactIDs []string `json:"candidate_derived_evidence_fact_ids,omitempty"`
	CanonicalEvidenceFactIDs        []string `json:"canonical_evidence_fact_ids,omitempty"`
	EvidenceSource                  string   `json:"evidence_source,omitempty"`
	EvidenceFactCount               int      `json:"evidence_fact_count,omitempty"`
	RawEntityIDs                    []string `json:"raw_entity_ids,omitempty"`
	AuthoritativeEntityIDs          []string `json:"authoritative_entity_ids,omitempty"`
	FinalEntityIDs                  []string `json:"final_entity_ids,omitempty"`
	DropStage                       string   `json:"drop_stage,omitempty"`
	DropCode                        string   `json:"drop_code,omitempty"`
	DropReason                      string   `json:"drop_reason,omitempty"`
}

// ActionsLifecycleSummary aggregates action counts across stages.
type ActionsLifecycleSummary struct {
	ProviderReturnedCount int    `json:"provider_returned_count"`
	ParsedCount           int    `json:"parsed_count"`
	CandidateValidCount   int    `json:"candidate_valid_count"`
	NormalizedCount       int    `json:"normalized_count"`
	ValidatedCount        int    `json:"validated_count"`
	AfterLimitCount       int    `json:"after_limit_count"`
	PersistedCount        int    `json:"persisted_count"`
	DroppedCount          int    `json:"dropped_count"`
	CountMismatch         bool   `json:"count_mismatch,omitempty"`
	CountMismatchCode     string `json:"count_mismatch_code,omitempty"`
}

// RecordRawCandidateReferences extracts candidate IDs from parsed compose output before normalization.
func RecordRawCandidateReferences(compose *ActionsAIFirstComposeOutput) [][]string {
	if compose == nil {
		return nil
	}
	out := make([][]string, len(compose.Actions))
	for i, a := range compose.Actions {
		for _, id := range a.SourceCandidateIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				out[i] = append(out[i], id)
			}
		}
	}
	return out
}
