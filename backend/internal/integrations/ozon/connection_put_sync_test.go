package ozon

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestInitialSyncStartedOnlyOnFirstSuccessfulConnect(t *testing.T) {
	t.Parallel()

	connected := dbgen.OzonConnection{
		SellerApiValid:      true,
		PerformanceApiValid: true,
		ConnectionStatus:    "connected",
	}

	t.Run("first_connect", func(t *testing.T) {
		wasReady := false
		if !initialSyncStarted(wasReady, ConnectionReady(connected)) {
			t.Fatal("expected initial sync on first successful connect")
		}
	})

	t.Run("re_save_existing_ready", func(t *testing.T) {
		wasReady := true
		if initialSyncStarted(wasReady, ConnectionReady(connected)) {
			t.Fatal("must not start initial sync when connection was already ready")
		}
	})

	t.Run("not_ready", func(t *testing.T) {
		broken := connected
		broken.SellerApiValid = false
		if initialSyncStarted(false, ConnectionReady(broken)) {
			t.Fatal("expected no initial sync when connection is not ready")
		}
	})
}
