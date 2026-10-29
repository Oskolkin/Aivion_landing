package funnel

import (
	"fmt"
	"testing"
)

func TestObservationStateHashDereferencesCanonicalSKU(t *testing.T) {
	id1, id2 := int64(55), int64(55)
	o := Observation{
		ValueNumeric: "1", Unit: "COUNT", TemporalSemantics: "ADDITIVE_FLOW", EntityAggregationSemantics: "ADDITIVE",
		Surface: "SEARCH_CATALOG", ProviderMetricName: "hits", RegistryVersion: "v", SourceContractHash: fmt.Sprintf("%064x", 1),
		SKUResolutionStatus: "RESOLVED",
	}
	if observationStateHash(o, &id1) != observationStateHash(o, &id2) {
		t.Fatal("separately allocated equal canonical SKU ids must hash identically")
	}
	if observationStateHash(o, nil) == observationStateHash(o, &id1) {
		t.Fatal("NULL canonical SKU must not collide with concrete id")
	}
}
