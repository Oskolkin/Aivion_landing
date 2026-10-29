package aicost

import (
	"math"
	"testing"
)

func TestEstimateUSDKnownModel(t *testing.T) {
	e := EstimateUSD("gpt-4.1-mini", 1_000_000, 500_000)
	if !e.Known {
		t.Fatalf("expected known pricing")
	}
	if e.CostUSD <= 0 || math.IsNaN(e.CostUSD) {
		t.Fatalf("expected positive cost, got %v", e.CostUSD)
	}
}

func TestEstimateUSDUnknownModel(t *testing.T) {
	e := EstimateUSD("unknown-model-xyz", 100, 50)
	if e.Known {
		t.Fatalf("expected unknown pricing")
	}
	if e.CostUSD != 0 || e.Reason != "unknown_price" {
		t.Fatalf("unexpected estimate: %+v", e)
	}
}
