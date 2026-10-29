package provider

import (
	"errors"
	"fmt"
	"net/http"
)

// Error codes normalized for unified analysis provider failures.
const (
	CodeProviderContainerCreateFailed   = "provider_container_create_failed"
	CodeProviderContainerRetrieveFailed = "provider_container_retrieve_failed"
	CodeProviderContainerDeleteFailed   = "provider_container_delete_failed"
	CodeProviderContainerExpired        = "provider_container_expired"

	CodeProviderFileUploadFailed = "provider_file_upload_failed"
	CodeProviderFileDeleteFailed = "provider_file_delete_failed"

	CodeProviderResponseStartFailed    = "provider_response_start_failed"
	CodeProviderResponseRetrieveFailed = "provider_response_retrieve_failed"
	CodeProviderResponseCancelFailed   = "provider_response_cancel_failed"

	CodeProviderCleanupFailed = "provider_cleanup_failed"

	CodeProviderAPIKeyMissing = "provider_api_key_missing"
	CodeProviderNotFound      = "provider_not_found"
)

// ErrNotFound indicates the requested provider resource does not exist.
var ErrNotFound = errors.New("provider resource not found")

// Error is a typed provider error with diagnostic fields.
type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	RequestID  string
	Err        error
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

// Is reports whether target matches this error or its wrapped cause.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e.Err, target) {
		return true
	}
	if target == ErrNotFound && e.HTTPStatus == http.StatusNotFound {
		return true
	}
	return false
}

func newProviderError(code, message string, httpStatus int, requestID string, err error) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		RequestID:  requestID,
		Err:        err,
	}
}

func notFoundError(code, message, requestID string) *Error {
	return newProviderError(code, message, http.StatusNotFound, requestID, ErrNotFound)
}

// AsError extracts a provider Error from err.
func AsError(err error) (*Error, bool) {
	var pe *Error
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

// IsNotFound reports whether err represents a missing provider resource.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	if pe, ok := AsError(err); ok && pe.HTTPStatus == http.StatusNotFound {
		return true
	}
	return false
}

func formatHTTPMessage(operation string, status int, body string) string {
	if body == "" {
		body = "<empty>"
	}
	return fmt.Sprintf("%s: status=%d body=%s", operation, status, body)
}
