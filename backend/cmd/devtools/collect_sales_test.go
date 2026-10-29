package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
)

func TestClassifySalesFailureKeepsSupersessionOutOfGenericFailure(t *testing.T) {
	d := classifySalesFailure(&sales.SupersededRunError{DomainRunID: 10, SupersededBy: 11})
	if !d.superseded || d.terminalFailure {
		t.Fatalf("superseded disposition=%+v", d)
	}
	d = classifySalesFailure(context.DeadlineExceeded)
	if d.superseded || d.terminalFailure {
		t.Fatalf("resumable disposition=%+v", d)
	}
	d = classifySalesFailure(errors.New("terminal contract failure"))
	if d.superseded || !d.terminalFailure {
		t.Fatalf("terminal disposition=%+v", d)
	}
}
