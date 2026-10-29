package factual

import (
	"math"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestCoverageValueUsesExactArithmeticAndExplicitStates(t *testing.T) {
	got := coverageValue(factualapi.Known(int64(math.MaxInt64)), factualapi.Known(int64(math.MaxInt64)), 30)
	if got.State != factualapi.ValueKnown || got.Value == nil || got.Value.String() != "30" {
		t.Fatalf("overflow-safe coverage=%+v", got)
	}
	zero := coverageValue(factualapi.Known(int64(12)), factualapi.Known(int64(0)), 7)
	if zero.State != factualapi.ValueNoDenominator || zero.ReasonCode != "NO_DEMAND" {
		t.Fatalf("zero demand state=%+v", zero)
	}
	missing := coverageValue(factualapi.Missing[int64](factualapi.ValueUnknown, "x"), factualapi.Known(int64(1)), 7)
	if missing.State != factualapi.ValueUnknown || missing.ReasonCode != "STOCK_UNAVAILABLE" {
		t.Fatalf("missing stock state=%+v", missing)
	}
}
