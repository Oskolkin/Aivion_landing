package unifiedanalysis

import (
	"errors"
	"fmt"
)

const (
	CodeProviderDisabled           = "provider_disabled"
	CodePackValidationFailed       = "pack_validation_failed"
	CodeFactsValidationFailed      = "facts_validation_failed"
	CodeSourceReconciliationFailed = "source_reconciliation_failed"
	CodeInconsistentRunState       = "inconsistent_run_state"
	CodeSmokeValidationFailed      = "smoke_validation_failed"
	CodeProviderFailed             = "provider_failed"
	CodeProviderPollTimeout        = "provider_poll_timeout"
	CodeRunNotFound                = "run_not_found"
	CodeActiveRunExists            = "active_run_exists"
	CodeCancellationFailed         = "cancellation_failed"

	CodeStructuredOutputMissing         = "structured_output_missing"
	CodeStructuredOutputAmbiguous       = "structured_output_ambiguous"
	CodeStructuredOutputInvalidJSON     = "structured_output_invalid_json"
	CodeStructuredOutputUnknownField    = "structured_output_unknown_field"
	CodeStructuredOutputSchemaMismatch  = "structured_output_schema_mismatch"
	CodeProviderRefusal                 = "provider_refusal"
	CodeProviderResponseIncomplete      = "provider_response_incomplete"
	CodeProviderOutputTruncated         = "provider_output_truncated"
	CodeOutputContractInvalid           = "output_contract_invalid"
	CodeOutputPackIDMismatch            = "output_pack_id_mismatch"
	CodeOutputMetricDateMismatch        = "output_metric_date_mismatch"
	CodeOutputInvalidOutcome            = "output_invalid_outcome"
	CodeOutputActionLimitExceeded       = "output_action_limit_exceeded"
	CodeOutputDuplicateActionID         = "output_duplicate_action_id"
	CodeOutputUnknownActionReference    = "output_unknown_action_reference"
	CodeOutputUnknownSKU                = "output_unknown_sku"
	CodeOutputUnknownCampaign           = "output_unknown_campaign"
	CodeOutputDuplicateSKUAcrossActions = "output_duplicate_sku_across_actions"
	CodeOutputUnknownLimitation         = "output_unknown_limitation_reference"
	CodeOutputMissingScopeEntities      = "output_missing_scope_entities"
	CodeOutputArtifactSaveFailed        = "output_artifact_save_failed"
)

// NewCodedError builds a typed orchestration error with a stable code.
func NewCodedError(code, message string) error {
	return newError(code, message, nil)
}

var (
	ErrProviderDisabled     = errors.New("unified analysis provider is disabled")
	ErrPackValidationFailed = errors.New("analysis pack validation failed")
	ErrInconsistentRunState = errors.New("inconsistent unified analysis run state")
	ErrCASConflict          = errors.New("unified analysis run status conflict")
	ErrRunNotFound          = errors.New("unified analysis run not found")
	ErrActiveRunExists      = errors.New("active unified analysis run already exists")
)

// Error is a typed unified analysis orchestration error.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newError(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func wrapError(code, message string, err error) error {
	if err == nil {
		return nil
	}
	return newError(code, message, err)
}

func asError(err error) (*Error, bool) {
	var ue *Error
	if errors.As(err, &ue) {
		return ue, true
	}
	return nil, false
}

func errorf(code, format string, args ...any) error {
	return newError(code, fmt.Sprintf(format, args...), nil)
}
