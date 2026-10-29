package deep

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
)

const (
	CodeBindingMismatch           = "DEEP_INPUT_BINDING_MISMATCH"
	CodePackTooLarge              = "DEEP_PACK_TOO_LARGE"
	CodePackInvalid               = "DEEP_PACK_INVALID"
	CodeRelationshipClaimExceeded = "DEEP_RELATIONSHIP_CLAIM_EXCEEDED"
	CodeValidationFailed          = "DEEP_VALIDATION_FAILED"
	CodeProviderRateLimit         = "OPENAI_RATE_LIMIT"
	CodeProviderServerError       = "OPENAI_SERVER_ERROR"
	CodeProviderTimeout           = "OPENAI_TIMEOUT"
	CodeProviderAuth              = "OPENAI_AUTH"
	CodeProviderBadRequest        = "OPENAI_BAD_REQUEST"
	CodeProviderRefused           = "OPENAI_REFUSED"
	CodeProviderIncomplete        = "OPENAI_INCOMPLETE"
	CodeProviderInvalidResponse   = "OPENAI_INVALID_RESPONSE"
	CodeProviderNotConfigured     = "DEEP_PROVIDER_NOT_CONFIGURED"
	CodeCausalEstimate            = "CAUSAL_ESTIMATE"
)

type Error struct {
	Code  string
	Msg   string
	class policy.FailureClass
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Msg == "" {
		return e.Code
	}
	return e.Code + ": " + e.Msg
}

func (e *Error) FailureClass() policy.FailureClass {
	if e == nil {
		return policy.InternalBug
	}
	return e.class
}

func packErr(code, msg string) *Error {
	return &Error{Code: code, Msg: msg, class: policy.ContractViolation}
}

func transientErr(code, msg string) *Error {
	return &Error{Code: code, Msg: msg, class: policy.TransientDependency}
}

func timeoutErr(msg string) *Error {
	return &Error{Code: CodeProviderTimeout, Msg: msg, class: policy.Timeout}
}

func Failed(code, msg string) *Error { return packErr(code, msg) }

func BindingMismatch(msg string) error {
	return packErr(CodeBindingMismatch, msg)
}

func fmtErr(code, msg string) error { return fmt.Errorf("%s: %s", code, msg) }
