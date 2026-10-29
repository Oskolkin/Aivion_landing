package ozon

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestPreserveConnectionOnTemporaryValidationFailure(t *testing.T) {
	t.Parallel()

	ready := dbgen.OzonConnection{
		SellerApiValid:      true,
		PerformanceApiValid: true,
		ConnectionStatus:    "connected",
	}

	t.Run("temporary errors preserve", func(t *testing.T) {
		if !preserveConnectionOnTemporaryValidationFailure(ready, []ConnectionError{
			{Source: "seller_api", Code: "temporary_unavailable", Message: "x"},
		}) {
			t.Fatal("expected preserve on temporary seller error")
		}
	})

	t.Run("credential errors do not preserve", func(t *testing.T) {
		if preserveConnectionOnTemporaryValidationFailure(ready, []ConnectionError{
			{Source: "performance_api", Code: "invalid_credentials", Message: "x"},
		}) {
			t.Fatal("expected no preserve on invalid credentials")
		}
	})

	t.Run("not ready does not preserve", func(t *testing.T) {
		notReady := ready
		notReady.ConnectionStatus = "connection_error"
		if preserveConnectionOnTemporaryValidationFailure(notReady, []ConnectionError{
			{Source: "seller_api", Code: "temporary_unavailable", Message: "x"},
		}) {
			t.Fatal("expected no preserve when not connected")
		}
	})
}
