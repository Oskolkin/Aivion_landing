package continuity

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type previousActionWire struct {
	PreviousActionRef string                       `json:"previous_action_ref"`
	CaseRef           string                       `json:"case_ref"`
	ProposalRef       string                       `json:"proposal_ref"`
	SignatureHash     string                       `json:"signature_hash"`
	ActionType        string                       `json:"action_type"`
	Target            EntityRef                    `json:"target"`
	NormalizedContext map[string]string            `json:"normalized_context"`
	IntentSignature   string                       `json:"intent_signature"`
	Parameters        actioncatalog.ParametersWire `json:"parameters"`
	RequiredSupports  []PackedSupport              `json:"required_supports"`
	ContextSupports   []PackedSupport              `json:"context_supports"`
	CounterSupports   []PackedSupport              `json:"counterevidence_supports"`
	SystemState       string                       `json:"system_state"`
	UserState         string                       `json:"user_state"`
	SuppressedUntil   string                       `json:"suppressed_until,omitempty"`
	PublicationID     int64                        `json:"publication_id"`
	Generation        int64                        `json:"publication_generation"`
	SelectionReason   string                       `json:"selection_reason"`
}

type contextWireV1 struct {
	SchemaVersion             string               `json:"schema_version"`
	Mode                      string               `json:"mode"`
	BasePublicationID         *int64               `json:"base_publication_id"`
	MandatoryActionCount      int                  `json:"mandatory_action_count"`
	MandatoryEntityCount      int                  `json:"mandatory_entity_count"`
	ContinuityContractVersion string               `json:"continuity_contract_version"`
	ContinuityPolicyVersion   string               `json:"continuity_policy_version,omitempty"`
	PreviousActions           []previousActionWire `json:"previous_actions"`
	Limitations               []map[string]any     `json:"limitations"`
}

func MarshalContextCanonical(c ContinuityContextV1) ([]byte, string, error) {
	prev := make([]previousActionWire, len(c.PreviousActions))
	for i, a := range c.PreviousActions {
		prev[i] = previousActionWire{
			PreviousActionRef: a.PreviousActionRef,
			CaseRef:           a.CaseRef,
			ProposalRef:       a.ProposalRef,
			SignatureHash:     a.SignatureHash,
			ActionType:        a.ActionType,
			Target:            a.Target,
			NormalizedContext: a.NormalizedContext,
			IntentSignature:   a.IntentSignature,
			Parameters:        actioncatalog.ParametersWireFrom(a.Parameters),
			RequiredSupports:  append([]PackedSupport(nil), a.RequiredSupports...),
			ContextSupports:   append([]PackedSupport(nil), a.ContextSupports...),
			CounterSupports:   append([]PackedSupport(nil), a.CounterSupports...),
			SystemState:       a.SystemState,
			UserState:         a.UserState,
			SuppressedUntil:   a.SuppressedUntil,
			PublicationID:     a.PublicationID,
			Generation:        a.Generation,
			SelectionReason:   a.SelectionReason,
		}
	}
	w := contextWireV1{
		SchemaVersion:             c.SchemaVersion,
		Mode:                      c.Mode,
		BasePublicationID:         c.BasePublicationID,
		MandatoryActionCount:      c.MandatoryActionCount,
		MandatoryEntityCount:      c.MandatoryEntityCount,
		ContinuityContractVersion: c.ContinuityContractVersion,
		ContinuityPolicyVersion:   c.ContinuityPolicyVersion,
		PreviousActions:           prev,
		Limitations:               c.Limitations,
	}
	body, err := canonicaljson.Marshal(w)
	if err != nil {
		return nil, "", err
	}
	return body, canonicaljson.Hash(w), nil
}
