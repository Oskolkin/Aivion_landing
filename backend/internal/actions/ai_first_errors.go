package actions

import "errors"

const (
	ActionsErrorInputBuildFailed                  = "actions_input_build_failed"
	ActionsErrorInputDuplicateCandidateID         = "actions_input_duplicate_candidate_id"
	ActionsErrorInputMissingDataPack              = "actions_input_missing_data_pack"
	ActionsErrorInputTokenEstimationFailed        = "actions_input_token_estimation_failed"
	ActionsErrorInputTooLarge                     = "actions_input_too_large"
	ActionsErrorAIDisabled                        = "actions_ai_disabled"
	ActionsErrorAIClientUnavailable               = "actions_ai_client_unavailable"
	ActionsErrorAIFirstUnsupported                = "actions_ai_first_unsupported"
	ActionsErrorProviderFailed                    = "actions_provider_failed"
	ActionsErrorInvalidJSON                       = "actions_invalid_json"
	ActionsErrorValidationFailed                  = "actions_validation_failed"
	ActionsErrorAllInvalid                        = "actions_all_invalid"
	ActionsErrorAllDropped                        = "actions_all_dropped"
	ActionsErrorEmptyOutputFromNonEmptyCandidates = "actions_empty_output_from_non_empty_candidates"
	ActionsErrorNoValidActions                    = "actions_no_valid_actions"
	ActionsErrorUnknownSourceCandidateID          = "unknown_source_candidate_id"
	ActionsErrorDuplicateSourceCandidateUsage     = "duplicate_source_candidate_usage"
	ActionsErrorTooManyActions                    = "too_many_actions"
	ActionsErrorFactNotGroundedInCandidates       = "action_fact_not_grounded_in_source_candidates"
	ActionsErrorEntityNotGroundedInCandidates     = "action_entity_not_grounded_in_source_candidates"
	ActionsErrorIncompatibleSourceCandidateTypes  = "incompatible_source_candidate_types"
)

// ActionsGenerationError is a controlled Actions generation failure with a stable code.
type ActionsGenerationError struct {
	Code string
	Err  error
}

func (e *ActionsGenerationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *ActionsGenerationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ActionsErrorCode extracts a stable Actions error code when present.
func ActionsErrorCode(err error) string {
	var coded *ActionsGenerationError
	if errors.As(err, &coded) && coded != nil && coded.Code != "" {
		return coded.Code
	}
	return ""
}
