package chatv2

import "errors"

var (
	ErrQuestionRequired         = errors.New("chat question is required")
	ErrIdempotencyConflict      = errors.New("idempotency key was reused with a different request")
	ErrRequestInProgress        = errors.New("chat request is already in progress")
	ErrSessionNotFound          = errors.New("chat session not found")
	ErrActionViewNotFound       = errors.New("action view not found")
	ErrActionViewForbidden      = errors.New("action view not found")
	ErrAttachmentNotFound       = errors.New("attachment not found")
	ErrProviderFailed           = errors.New("chat provider failed")
	ErrValidationFailed         = errors.New("chat answer validation failed")
	ErrCancelled                = errors.New("chat turn cancelled")
	ErrBudgetExceeded           = errors.New("chat tool budget exceeded")
	ErrPeriodLimited            = errors.New("requested period exceeds 90 days")
	ErrMalformedToolArgs        = errors.New("malformed tool arguments")
	ErrInvalidToolCall          = errors.New("invalid provider tool call")
	ErrInvalidToolArgs          = errors.New("invalid tool arguments")
	ErrForbiddenToolArg         = errors.New("tool argument is not allowed")
	ErrExportFailed             = errors.New("chat export failed")
	ErrExportForbidden          = errors.New("export attachment not found")
	ErrIdempotencyOwnershipLost = errors.New("chat idempotency ownership lost")
	ErrUnsupportedExportPeriod  = errors.New("unsupported export period")
	ErrUnsupportedExportDataset = errors.New("unsupported export dataset")
	ErrFinalizationFailed       = errors.New("chat turn finalization failed")
)
