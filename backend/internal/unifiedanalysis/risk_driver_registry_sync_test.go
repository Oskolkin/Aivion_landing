package unifiedanalysis_test

import (
	"reflect"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/healthpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func TestRiskDriverRegistry_MatchesHealthPolicy(t *testing.T) {
	if !reflect.DeepEqual(outputvalidator.CanonicalRiskDriverIDs, healthpolicy.KnownDriverIDs) {
		t.Fatalf("outputvalidator.CanonicalRiskDriverIDs drifted from healthpolicy.KnownDriverIDs:\n got=%v\nwant=%v",
			outputvalidator.CanonicalRiskDriverIDs, healthpolicy.KnownDriverIDs)
	}
	for _, id := range healthpolicy.KnownDriverIDs {
		if !outputvalidator.IsCanonicalRiskDriverID(id) {
			t.Fatalf("IsCanonicalRiskDriverID(%q)=false", id)
		}
	}
	if outputvalidator.IsCanonicalRiskDriverID("high_drr_882000021528") {
		t.Fatal("composite id must not be canonical")
	}
}
