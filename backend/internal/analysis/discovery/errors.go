package discovery

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
)

const (
	CodePackBudgetExceeded      = "DISCOVERY_PACK_BUDGET_EXCEEDED"
	CodePackInvalid             = "DISCOVERY_PACK_INVALID"
	CodeProviderRateLimit       = "OPENAI_RATE_LIMIT"
	CodeProviderServerError     = "OPENAI_SERVER_ERROR"
	CodeProviderTimeout         = "OPENAI_TIMEOUT"
	CodeProviderAuth            = "OPENAI_AUTH"
	CodeProviderBadRequest      = "OPENAI_BAD_REQUEST"
	CodeProviderRefused         = "OPENAI_REFUSED"
	CodeProviderIncomplete      = "OPENAI_INCOMPLETE"
	CodeProviderInvalidResponse = "OPENAI_INVALID_RESPONSE"
	CodeValidationFailed        = "DISCOVERY_VALIDATION_FAILED"
	CodeProviderNotConfigured   = "DISCOVERY_PROVIDER_NOT_CONFIGURED"
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

func Failed(code, msg string) *Error { return packErr(code, msg) }

func packErr(code, msg string) *Error {
	return &Error{Code: code, Msg: msg, class: policy.ContractViolation}
}

func transientErr(code, msg string) *Error {
	return &Error{Code: code, Msg: msg, class: policy.TransientDependency}
}

func timeoutErr(msg string) *Error {
	return &Error{Code: CodeProviderTimeout, Msg: msg, class: policy.Timeout}
}

func fmtPackErr(code, format string, args ...any) *Error {
	return packErr(code, fmt.Sprintf(format, args...))
}
