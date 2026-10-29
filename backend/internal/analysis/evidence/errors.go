package evidence

import (
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
)

var ErrIntegrity = errors.New(CodeIntegrityFailure)

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

func (e *Error) Unwrap() error {
	if e != nil && e.Code == CodeIntegrityFailure {
		return ErrIntegrity
	}
	return nil
}

func Integrity(msg string) *Error {
	return &Error{Code: CodeIntegrityFailure, Msg: msg, class: policy.ContractViolation}
}

func BudgetExceeded(msg string) *Error {
	return &Error{Code: CodeBudgetExceeded, Msg: msg, class: policy.ContractViolation}
}

func Contract(code, msg string) *Error {
	return &Error{Code: code, Msg: msg, class: policy.ContractViolation}
}

func FmtIntegrity(format string, args ...any) *Error {
	return Integrity(fmt.Sprintf(format, args...))
}
