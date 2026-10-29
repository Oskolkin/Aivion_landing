package aipipeline

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const PublicationGuardDecisionSchemaVersion = "publication_guard_decision_v1"

const PublicationBlockedByFinalSemanticValidation = "publication_blocked_by_final_semantic_validation"

// PublicationGuardInput captures pre-publish validation inputs.
type PublicationGuardInput struct {
	RequestedDashboard    bool
	RequestedActions      bool
	Pack                  *aidatapack.SellerAIDataPack
	DashboardPayload      []byte
	Actions               []actions.ActionItem
	ComposeTruncated      bool
	GlobalContextBlocking bool
}

// PublicationGuardDecision is the publication guard outcome.
type PublicationGuardDecision struct {
	SchemaVersion                  string                        `json:"schema_version"`
	RequestedOutputs               map[string]bool               `json:"requested_outputs"`
	DashboardValidation            string                        `json:"dashboard_validation"`
	ActionsValidation              string                        `json:"actions_validation"`
	FinalSemanticValidation        FinalOutputSemanticValidation `json:"final_semantic_validation"`
	BlockingIssueCodes             []string                      `json:"blocking_issue_codes,omitempty"`
	PublicationAllowed             bool                          `json:"publication_allowed"`
	PreviousPublishedPipelineRunID int64                         `json:"previous_published_pipeline_run_id,omitempty"`
	NewPipelineRunID               int64                         `json:"new_pipeline_run_id,omitempty"`
	Decision                       string                        `json:"decision"`
	DecisionReason                 string                        `json:"decision_reason,omitempty"`
}

// RunPublicationGuard validates final outputs and returns publication decision.
func RunPublicationGuard(input PublicationGuardInput) PublicationGuardDecision {
	decision := PublicationGuardDecision{
		SchemaVersion: PublicationGuardDecisionSchemaVersion,
		RequestedOutputs: map[string]bool{
			"dashboard": input.RequestedDashboard,
			"actions":   input.RequestedActions,
		},
		DashboardValidation: "skipped",
		ActionsValidation:   "skipped",
		Decision:            "allowed",
		PublicationAllowed:  true,
	}
	final := FinalOutputSemanticValidation{
		SchemaVersion:      FinalOutputSemanticValidationSchemaVersion,
		PublicationAllowed: true,
	}
	if input.RequestedDashboard {
		final.Dashboard = ValidateFinalDashboardOutput(input.Pack, input.DashboardPayload)
		decision.DashboardValidation = final.Dashboard.Status
	}
	if input.RequestedActions {
		final.Actions = ValidateFinalActionsOutput(input.Pack, input.Actions)
		decision.ActionsValidation = final.Actions.Status
	}
	if input.ComposeTruncated {
		appendBlockingCode(&decision, "actions_compose_output_truncated")
	}
	if input.GlobalContextBlocking {
		appendBlockingCode(&decision, GlobalContextUnresolvedNumericPlaceholderCode)
	}
	for _, issue := range final.Dashboard.Issues {
		if issue.Severity == "blocking" {
			appendBlockingCode(&decision, issue.Code)
		}
	}
	for _, issue := range final.Actions.Issues {
		if issue.Severity == "blocking" {
			appendBlockingCode(&decision, issue.Code)
		}
	}
	final.PublicationAllowed = len(decision.BlockingIssueCodes) == 0
	decision.FinalSemanticValidation = final
	if !final.PublicationAllowed {
		decision.PublicationAllowed = false
		decision.Decision = "blocked"
		decision.DecisionReason = PublicationBlockedByFinalSemanticValidation
	}
	return decision
}

func appendBlockingCode(decision *PublicationGuardDecision, code string) {
	if decision == nil || code == "" {
		return
	}
	for _, existing := range decision.BlockingIssueCodes {
		if existing == code {
			return
		}
	}
	decision.BlockingIssueCodes = append(decision.BlockingIssueCodes, code)
}
