package factualapi

import "fmt"

type ErrorCode string

const (
	CodeInvalidArgument            ErrorCode = "INVALID_ARGUMENT"
	CodeNotFound                   ErrorCode = "NOT_FOUND"
	CodeContextInvalid             ErrorCode = "CONTEXT_INVALID"
	CodeContextSellerMismatch      ErrorCode = "CONTEXT_SELLER_MISMATCH"
	CodeUniverseIntegrityFailure   ErrorCode = "UNIVERSE_INTEGRITY_FAILURE"
	CodePeriodUnsupported          ErrorCode = "PERIOD_UNSUPPORTED"
	CodePageTokenInvalid           ErrorCode = "PAGE_TOKEN_INVALID"
	CodePageTokenContextMismatch   ErrorCode = "PAGE_TOKEN_CONTEXT_MISMATCH"
	CodePageTokenFilterMismatch    ErrorCode = "PAGE_TOKEN_FILTER_MISMATCH"
	CodeFactRefInvalid             ErrorCode = "FACT_REF_INVALID"
	CodeFactRefContextMismatch     ErrorCode = "FACT_REF_CONTEXT_MISMATCH"
	CodeFactTraceResultMismatch    ErrorCode = "FACT_TRACE_RESULT_MISMATCH"
	CodeInternalInvariantViolation ErrorCode = "INTERNAL_INVARIANT_VIOLATION"
)

type Error struct {
	Code        ErrorCode
	SafeMessage string
}

func (e *Error) Error() string                      { return fmt.Sprintf("%s: %s", e.Code, e.SafeMessage) }
func NewError(code ErrorCode, message string) error { return &Error{Code: code, SafeMessage: message} }
