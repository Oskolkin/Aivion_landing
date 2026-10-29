package publication

import (
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/bundlevalidator"
)

// Stage 5 publication error codes. These are stable identifiers surfaced on
// unified_analysis_publications.error_code / ai_pipeline_runs.error_code and
// in the audit trail; keep them frozen once shipped.
const (
	CodeRunNotReady               = "publication_run_not_ready"
	CodeRunNotFound               = "publication_run_not_found"
	CodeAlreadyPublished          = "publication_already_published"
	CodeArtifactMissing           = "publication_artifact_missing"
	CodeArtifactIntegrityFailed   = "publication_artifact_integrity_failed"
	CodeOutputParseFailed         = "publication_output_parse_failed"
	CodePromptNotAllowed          = "publication_prompt_not_allowed"
	CodeSchemaNotAllowed          = "publication_schema_not_allowed"
	CodeAnalysisFailedOutcome     = "publication_analysis_failed_outcome"
	CodeProjectionFailed          = "publication_projection_failed"
	CodeStaleRelativeToCurrent    = "publication_stale_relative_to_current"
	CodeConcurrentPublishConflict = "publication_concurrent_conflict"
	CodePersistDashboardFailed    = "publication_persist_dashboard_failed"
	CodePersistActionsFailed      = "publication_persist_actions_failed"
	CodePersistLinkFailed         = "publication_persist_link_failed"
	CodeSettingsInvalid           = "publication_settings_invalid"
	CodeRollbackNotAllowed        = "publication_rollback_not_allowed"
	CodePromotionNotAllowed       = "publication_promotion_not_allowed"
	CodeSelectorInconsistentState = "publication_selector_inconsistent_state"
	// CodeTestInjectedFailure is only ever returned by Service.maybeFail when
	// a test configured a FailurePoint via WithTestFailurePoint; production
	// callers never see this code.
	CodeTestInjectedFailure = "publication_test_injected_failure"

	// CodeDashboardFactsMissing/CodeDashboardFactsHashMismatch mirror the
	// bundlevalidator codes of the same failure so publication.Error callers
	// can compare against a single stable constant regardless of whether the
	// facts check happened before or inside ValidatePublicationBundle.
	CodeDashboardFactsMissing      = bundlevalidator.CodeDashboardFactsMissing
	CodeDashboardFactsHashMismatch = bundlevalidator.CodeDashboardFactsHashMismatch
	// CodeBundleValidationFailed is the generic Error.Code used for the
	// publication.Error returned to callers when ValidatePublicationBundle
	// rejects a bundle; the specific reason (one of the bundlevalidator
	// Code* constants) is stamped on the publication row's error_code
	// column instead, so downstream tooling can distinguish causes.
	CodeBundleValidationFailed = "publication_bundle_validation_failed"
)

// Error is a typed publication error with a stable code, analogous to
// unifiedanalysis.Error but scoped to the publication package.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Message != "" && e.Err != nil:
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	case e.Message != "":
		return e.Message
	case e.Err != nil:
		return e.Err.Error()
	default:
		return e.Code
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewError builds a typed publication error with a stable code.
func NewError(code, message string) error {
	return &Error{Code: code, Message: message}
}

// WrapError builds a typed publication error wrapping an underlying cause.
// Returns nil when err is nil, so it is safe to use as `return WrapError(..., err)`.
func WrapError(code, message string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: message, Err: err}
}

// Errorf builds a typed publication error with a formatted message.
func Errorf(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeOf extracts the stable code from a publication error, if any.
func CodeOf(err error) (string, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e.Code, true
	}
	return "", false
}

// Sentinel errors for common control-flow branches.
var (
	ErrRunNotFound        = errors.New("unified analysis run not found")
	ErrAlreadyPublished   = errors.New("publication already published")
	ErrConcurrentConflict = errors.New("concurrent publication conflict")
)
