package deep

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type actionProposalWire struct {
	Ordinal               int                          `json:"ordinal"`
	ActionType            string                       `json:"action_type"`
	Target                EntityRef                    `json:"target"`
	Parameters            actioncatalog.ParametersWire `json:"parameters"`
	RecommendationOrdinal int                          `json:"recommendation_ordinal"`
	FindingOrdinals       []int                        `json:"finding_ordinals"`
	FactHandles           []string                     `json:"fact_handles"`
	Supports              []ActionSupportBasis         `json:"supports"`
}

type outputWireV1 struct {
	ExecutiveSummary          ExecutiveSummary           `json:"executive_summary"`
	ThreadResolutions         []ThreadResolution         `json:"thread_resolutions"`
	Findings                  []Finding                  `json:"findings"`
	Recommendations           []Recommendation           `json:"recommendations"`
	ActionProposals           []actionProposalWire       `json:"action_proposals"`
	PreviousActionAssessments []PreviousActionAssessment `json:"previous_action_assessments"`
}

type validatedOutputWireV1 struct {
	SchemaVersion      string       `json:"schema_version"`
	Output             outputWireV1 `json:"output"`
	FindingRefs        []string     `json:"finding_refs"`
	RecommendationRefs []string     `json:"recommendation_refs"`
	ActionProposalRefs []string     `json:"action_proposal_refs"`
	SupportRefs        []string     `json:"support_refs,omitempty"`
}

func outputToWire(o OutputV1) outputWireV1 {
	w := outputWireV1{
		ExecutiveSummary:          o.ExecutiveSummary,
		ThreadResolutions:         o.ThreadResolutions,
		Findings:                  o.Findings,
		Recommendations:           o.Recommendations,
		PreviousActionAssessments: o.PreviousActionAssessments,
		ActionProposals:           make([]actionProposalWire, len(o.ActionProposals)),
	}
	for i, a := range o.ActionProposals {
		w.ActionProposals[i] = actionProposalWire{
			Ordinal:               a.Ordinal,
			ActionType:            a.ActionType,
			Target:                a.Target,
			Parameters:            actioncatalog.ParametersWireFrom(a.Parameters),
			RecommendationOrdinal: a.RecommendationOrdinal,
			FindingOrdinals:       append([]int(nil), a.FindingOrdinals...),
			FactHandles:           append([]string(nil), a.FactHandles...),
			Supports:              a.Supports,
		}
	}
	return w
}

// MarshalOutputCanonical serializes Deep output for artifact storage and hashing.
func MarshalOutputCanonical(o OutputV1) ([]byte, error) {
	return canonicaljson.Marshal(outputToWire(o))
}

// MarshalValidatedOutputCanonical serializes validated Deep output for artifact storage.
func MarshalValidatedOutputCanonical(v ValidatedOutputV1) ([]byte, error) {
	w := validatedOutputWireV1{
		SchemaVersion:      v.SchemaVersion,
		Output:             outputToWire(v.Output),
		FindingRefs:        append([]string(nil), v.FindingRefs...),
		RecommendationRefs: append([]string(nil), v.RecommendationRefs...),
		ActionProposalRefs: append([]string(nil), v.ActionProposalRefs...),
		SupportRefs:        append([]string(nil), v.SupportRefs...),
	}
	return canonicaljson.Marshal(w)
}
