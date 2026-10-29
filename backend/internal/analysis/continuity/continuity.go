// Package continuity implements PREPARE_CONTINUITY: first-run no-base and
// publication-anchored previous Action snapshots (Prompt №13).
package continuity

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

const (
	SchemaVersionV1           = "CONTINUITY_CONTEXT_V1"
	SchemaVersionV2           = "CONTINUITY_CONTEXT_V2"
	ModeNoBasePublication     = "NO_BASE_PUBLICATION"
	ModeFromPublication       = "FROM_PUBLICATION"
	ContractInitialNoBaseV1   = "CONTINUITY_INITIAL_NO_BASE_V1"
	ContractFromPublicationV1 = "CONTINUITY_FROM_PUBLICATION_V1"
	PolicyV1                  = "action_continuity_policy_v1"
	PolicyV2                  = "action_continuity_policy_v2"
	ErrReaderNotImplemented   = "CONTINUITY_READER_NOT_IMPLEMENTED"
	ErrCapacityExceeded       = "CONTINUITY_CAPACITY_EXCEEDED"
	ErrSellerMismatch         = "CONTINUITY_SELLER_MISMATCH"
	// CapacityV1 is the approved Deep Action proposal budget (must not silently drop).
	CapacityV1 = 40
)

type EntityRef = discovery.PackEntity

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

type PreviousAtomicAction struct {
	PreviousActionRef string                   `json:"previous_action_ref"`
	CaseRef           string                   `json:"case_ref"`
	ProposalRef       string                   `json:"proposal_ref"`
	SignatureHash     string                   `json:"signature_hash"`
	ActionType        string                   `json:"action_type"`
	Target            EntityRef                `json:"target"`
	NormalizedContext map[string]string        `json:"normalized_context"`
	IntentSignature   string                   `json:"intent_signature"`
	Parameters        actioncatalog.Parameters `json:"parameters"`
	RequiredSupports  []PackedSupport          `json:"required_supports"`
	ContextSupports   []PackedSupport          `json:"context_supports"`
	CounterSupports   []PackedSupport          `json:"counterevidence_supports"`
	SystemState       string                   `json:"system_state"`
	UserState         string                   `json:"user_state"`
	SuppressedUntil   string                   `json:"suppressed_until,omitempty"`
	PublicationID     int64                    `json:"publication_id"`
	Generation        int64                    `json:"publication_generation"`
	SelectionReason   string                   `json:"selection_reason"`
}

type ContinuityContextV1 struct {
	SchemaVersion             string                 `json:"schema_version"`
	Mode                      string                 `json:"mode"`
	BasePublicationID         *int64                 `json:"base_publication_id"`
	MandatoryActionCount      int                    `json:"mandatory_action_count"`
	MandatoryEntityCount      int                    `json:"mandatory_entity_count"`
	ContinuityContractVersion string                 `json:"continuity_contract_version"`
	ContinuityPolicyVersion   string                 `json:"continuity_policy_version,omitempty"`
	PreviousActions           []PreviousAtomicAction `json:"previous_actions"`
	Limitations               []map[string]any       `json:"limitations"`
}

type SnapshotReader interface {
	LoadPreviousActions(sellerID int64, publicationID int64) ([]PreviousAtomicAction, error)
}

// Prepare returns the no-base continuity artifact, or fails closed when a base is requested
// without a reader.
func Prepare(basePublicationID *int64) (body []byte, hash string, err error) {
	return PrepareWithReader(0, basePublicationID, nil)
}

func PrepareWithReader(sellerID int64, basePublicationID *int64, reader SnapshotReader) (body []byte, hash string, err error) {
	if basePublicationID == nil || *basePublicationID <= 0 {
		ctx := ContinuityContextV1{
			SchemaVersion:             SchemaVersionV2,
			Mode:                      ModeNoBasePublication,
			BasePublicationID:         nil,
			MandatoryActionCount:      0,
			MandatoryEntityCount:      0,
			ContinuityContractVersion: ContractInitialNoBaseV1,
			ContinuityPolicyVersion:   PolicyV2,
			PreviousActions:           []PreviousAtomicAction{},
			Limitations:               []map[string]any{},
		}
		body, err = canonicaljson.Marshal(ctx)
		if err != nil {
			return nil, "", err
		}
		return body, canonicaljson.Hash(ctx), nil
	}
	if reader == nil {
		return nil, "", fmt.Errorf("%s", ErrReaderNotImplemented)
	}
	actions, err := reader.LoadPreviousActions(sellerID, *basePublicationID)
	if err != nil {
		return nil, "", err
	}
	if len(actions) > CapacityV1 {
		return nil, "", fmt.Errorf("%s: mandatory previous actions %d exceed capacity %d", ErrCapacityExceeded, len(actions), CapacityV1)
	}
	ents := map[string]struct{}{}
	for i := range actions {
		if actions[i].SelectionReason == "" {
			actions[i].SelectionReason = "ACTION_CONTINUITY_RECHECK"
		}
		ents[string(actions[i].Target.Kind)+"|"+actions[i].Target.ID] = struct{}{}
	}
	ctx := ContinuityContextV1{
		SchemaVersion:             SchemaVersionV2,
		Mode:                      ModeFromPublication,
		BasePublicationID:         basePublicationID,
		MandatoryActionCount:      len(actions),
		MandatoryEntityCount:      len(ents),
		ContinuityContractVersion: ContractFromPublicationV1,
		ContinuityPolicyVersion:   PolicyV2,
		PreviousActions:           actions,
		Limitations:               []map[string]any{},
	}
	body, hash, err = MarshalContextCanonical(ctx)
	if err != nil {
		return nil, "", err
	}
	return body, hash, nil
}
