package actionreconcile

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

type EntityRef = discovery.PackEntity

type PlanV1 struct {
	SchemaVersion         string              `json:"schema_version"`
	IdentityPolicyVersion string              `json:"identity_policy_version"`
	ReconciliationPolicy  string              `json:"reconciliation_policy_version"`
	VisibilityPolicy      string              `json:"visibility_policy_version"`
	UserSuppressionPolicy string              `json:"user_suppression_policy_version"`
	RecurrencePolicy      string              `json:"recurrence_policy_version"`
	GroupingPolicy        string              `json:"grouping_policy_version"`
	SellerAccountID       int64               `json:"seller_account_id"`
	AnalysisRunID         int64               `json:"analysis_run_id"`
	BasePublicationID     *int64              `json:"base_publication_id"`
	ValidatedDeepHash     string              `json:"validated_deep_hash"`
	ContinuityContextHash string              `json:"continuity_context_hash"`
	NewProposalIntents    []NewProposalIntent `json:"new_proposal_intents"`
	PreviousOutcomes      []PreviousOutcome   `json:"previous_outcomes"`
	Groups                []GroupIntent       `json:"groups"`
}

type NewProposalIntent struct {
	LocalRef          string                    `json:"local_ref"`
	DeepOrdinal       int                       `json:"deep_ordinal"`
	ActionType        string                    `json:"action_type"`
	Target            EntityRef                 `json:"target"`
	NormalizedContext map[string]string         `json:"normalized_context"`
	IntentSignature   string                    `json:"intent_signature"`
	SignatureHash     string                    `json:"signature_hash"`
	Parameters        actioncatalog.Parameters  `json:"parameters"`
	RecommendationRef string                    `json:"recommendation_ref"`
	FindingRefs       []string                  `json:"finding_refs"`
	FactHandles       []string                  `json:"fact_handles"`
	Supports          []deep.ActionSupportBasis `json:"supports"`
	Disposition       string                    `json:"disposition"`
	ExistingCaseRef   string                    `json:"existing_case_ref,omitempty"`
	OpenNewOccurrence bool                      `json:"open_new_occurrence"`
	Visible           bool                      `json:"visible"`
	DisplayOrder      int                       `json:"display_order"`
}

type PreviousOutcome struct {
	PreviousActionRef  string                           `json:"previous_action_ref"`
	CaseRef            string                           `json:"case_ref"`
	ProposalRef        string                           `json:"proposal_ref"`
	SignatureHash      string                           `json:"signature_hash"`
	DeepAssessment     string                           `json:"deep_assessment"`
	SystemTransition   string                           `json:"system_transition"`
	OpenNewOccurrence  bool                             `json:"open_new_occurrence"`
	LinkedNewLocalRef  string                           `json:"linked_new_local_ref,omitempty"`
	SupportAssessments []deep.PreviousSupportAssessment `json:"support_assessments"`
	SystemEvent        string                           `json:"system_event"`
}

type GroupIntent struct {
	LocalRef              string   `json:"local_ref"`
	RecommendationRef     string   `json:"recommendation_ref"`
	ActionType            string   `json:"action_type"`
	TargetKind            string   `json:"target_kind"`
	Family                string   `json:"family"`
	NormalizedContextHash string   `json:"normalized_context_hash"`
	MemberLocalRefs       []string `json:"member_local_refs"`
	DisplayOrder          int      `json:"display_order"`
	DisplayTitle          string   `json:"display_title"`
	DisplaySummary        string   `json:"display_summary"`
	PresentationKey       string   `json:"presentation_template_key"`
}

type ReportV1 struct {
	SchemaVersion string   `json:"schema_version"`
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors"`
}

type PersistedSnapshot struct {
	Signatures []PersistedSignature
	Now        time.Time
}

type PersistedSignature struct {
	SignatureHash   string
	ActiveCaseRef   string
	ActiveUserState string
	ActiveSystem    string
	SuppressedUntil *time.Time
	LastOccurrence  int
}

type PlanInput struct {
	SellerID          int64
	RunID             int64
	Validated         deep.ValidatedOutputV1
	ValidatedHash     string
	Menu              actioncatalog.Menu
	Continuity        continuity.ContinuityContextV1
	ContinuityHash    string
	BasePublicationID *int64
	Snapshot          PersistedSnapshot
	Catalog           []actioncatalog.Definition
	Now               time.Time
}
