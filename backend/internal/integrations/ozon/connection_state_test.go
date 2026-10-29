package ozon

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	ozonperf "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestConnectionReady(t *testing.T) {
	t.Parallel()

	base := dbgen.OzonConnection{
		SellerApiValid:      true,
		PerformanceApiValid: true,
		ConnectionStatus:    "connected",
	}

	if !ConnectionReady(base) {
		t.Fatal("expected ready for connected valid connection")
	}

	disconnected := base
	disconnected.ConnectionStatus = "disconnected_retention"
	if ConnectionReady(disconnected) {
		t.Fatal("expected not ready when disconnected_retention")
	}

	missingPerf := base
	missingPerf.PerformanceApiValid = false
	if ConnectionReady(missingPerf) {
		t.Fatal("expected not ready without performance")
	}

	withDiscAt := base
	withDiscAt.DisconnectedAt = pgtype.Timestamptz{Valid: true}
	if ConnectionReady(withDiscAt) {
		t.Fatal("expected not ready when disconnected_at set")
	}
}

func TestMapSellerCheckError(t *testing.T) {
	t.Parallel()

	got := mapSellerCheckError(ErrInvalidCredentials)
	if got.Code != "invalid_credentials" {
		t.Fatalf("code=%s", got.Code)
	}
	if got.Message == "" {
		t.Fatal("expected message")
	}
}

func TestMapPerformanceCheckError(t *testing.T) {
	t.Parallel()

	got := mapPerformanceCheckError(ozonperf.ErrInvalidCredentials)
	if got.Code != "invalid_credentials" {
		t.Fatalf("code=%s", got.Code)
	}
}

func TestIsCredentialOrAccessProblem(t *testing.T) {
	t.Parallel()
	if !IsCredentialOrAccessProblem("invalid_credentials") {
		t.Fatal("invalid_credentials should be credential problem")
	}
	if IsCredentialOrAccessProblem("temporary_unavailable") {
		t.Fatal("temporary should not be credential problem")
	}
}
