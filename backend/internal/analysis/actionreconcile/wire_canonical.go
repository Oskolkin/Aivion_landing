package actionreconcile

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type newProposalIntentWire struct {
	LocalRef          string                       `json:"local_ref"`
	DeepOrdinal       int                          `json:"deep_ordinal"`
	ActionType        string                       `json:"action_type"`
	Target            EntityRef                    `json:"target"`
	NormalizedContext map[string]string            `json:"normalized_context"`
	IntentSignature   string                       `json:"intent_signature"`
	SignatureHash     string                       `json:"signature_hash"`
	Parameters        actioncatalog.ParametersWire `json:"parameters"`
	RecommendationRef string                       `json:"recommendation_ref"`
	FindingRefs       []string                     `json:"finding_refs"`
	FactHandles       []string                     `json:"fact_handles"`
	Supports          []deep.ActionSupportBasis    `json:"supports"`
	Disposition       string                       `json:"disposition"`
	ExistingCaseRef   string                       `json:"existing_case_ref,omitempty"`
	OpenNewOccurrence bool                         `json:"open_new_occurrence"`
	Visible           bool                         `json:"visible"`
	DisplayOrder      int                          `json:"display_order"`
}

type planWireV1 struct {
	SchemaVersion         string                  `json:"schema_version"`
	IdentityPolicyVersion string                  `json:"identity_policy_version"`
	ReconciliationPolicy  string                  `json:"reconciliation_policy_version"`
	VisibilityPolicy      string                  `json:"visibility_policy_version"`
	UserSuppressionPolicy string                  `json:"user_suppression_policy_version"`
	RecurrencePolicy      string                  `json:"recurrence_policy_version"`
	GroupingPolicy        string                  `json:"grouping_policy_version"`
	SellerAccountID       int64                   `json:"seller_account_id"`
	AnalysisRunID         int64                   `json:"analysis_run_id"`
	BasePublicationID     *int64                  `json:"base_publication_id"`
	ValidatedDeepHash     string                  `json:"validated_deep_hash"`
	ContinuityContextHash string                  `json:"continuity_context_hash"`
	NewProposalIntents    []newProposalIntentWire `json:"new_proposal_intents"`
	PreviousOutcomes      []PreviousOutcome       `json:"previous_outcomes"`
	Groups                []GroupIntent           `json:"groups"`
}

// MarshalPlanCanonical serializes a reconciliation plan for artifact storage.
func MarshalPlanCanonical(p PlanV1) ([]byte, error) {
	intents := make([]newProposalIntentWire, len(p.NewProposalIntents))
	for i, np := range p.NewProposalIntents {
		intents[i] = newProposalIntentWire{
			LocalRef:          np.LocalRef,
			DeepOrdinal:       np.DeepOrdinal,
			ActionType:        np.ActionType,
			Target:            np.Target,
			NormalizedContext: np.NormalizedContext,
			IntentSignature:   np.IntentSignature,
			SignatureHash:     np.SignatureHash,
			Parameters:        actioncatalog.ParametersWireFrom(np.Parameters),
			RecommendationRef: np.RecommendationRef,
			FindingRefs:       append([]string(nil), np.FindingRefs...),
			FactHandles:       append([]string(nil), np.FactHandles...),
			Supports:          np.Supports,
			Disposition:       np.Disposition,
			ExistingCaseRef:   np.ExistingCaseRef,
			OpenNewOccurrence: np.OpenNewOccurrence,
			Visible:           np.Visible,
			DisplayOrder:      np.DisplayOrder,
		}
	}
	w := planWireV1{
		SchemaVersion:         p.SchemaVersion,
		IdentityPolicyVersion: p.IdentityPolicyVersion,
		ReconciliationPolicy:  p.ReconciliationPolicy,
		VisibilityPolicy:      p.VisibilityPolicy,
		UserSuppressionPolicy: p.UserSuppressionPolicy,
		RecurrencePolicy:      p.RecurrencePolicy,
		GroupingPolicy:        p.GroupingPolicy,
		SellerAccountID:       p.SellerAccountID,
		AnalysisRunID:         p.AnalysisRunID,
		BasePublicationID:     p.BasePublicationID,
		ValidatedDeepHash:     p.ValidatedDeepHash,
		ContinuityContextHash: p.ContinuityContextHash,
		NewProposalIntents:    intents,
		PreviousOutcomes:      p.PreviousOutcomes,
		Groups:                p.Groups,
	}
	return canonicaljson.Marshal(w)
}
