package discovery

import (
	"encoding/json"
	"os"
	"strings"
)

func NonEmptyFixtureJSON(payloadJSON []byte) string {
	var p PayloadV1
	_ = json.Unmarshal(payloadJSON, &p)
	sku := PackEntity{Kind: KindSKU, ID: "0"}
	handle := "f_0001"
	if len(p.SKUs) > 0 {
		sku = p.SKUs[0].Entity
		if p.SKUs[0].Sales != nil && p.SKUs[0].Sales.OrderedAmount != nil && p.SKUs[0].Sales.OrderedAmount.FactHandle != "" {
			handle = p.SKUs[0].Sales.OrderedAmount.FactHandle
		} else if p.SKUs[0].Price != nil && p.SKUs[0].Price.ObservedPrice != nil && p.SKUs[0].Price.ObservedPrice.FactHandle != "" {
			handle = p.SKUs[0].Price.ObservedPrice.FactHandle
		}
	}
	menu := map[string]bool{}
	for _, m := range p.EvidenceMenu {
		menu[m.Key] = true
	}
	want := []string{"SKU_SALES_HISTORY", "SKU_PRICE_HISTORY", "SKU_SEARCH_QUERIES"}
	types := []string{}
	for _, k := range want {
		if menu[k] {
			types = append(types, k)
		}
	}
	if len(types) == 0 {
		return EmptyThreadsJSON()
	}
	er := func(k string) EvidenceRequest {
		return EvidenceRequest{EvidenceType: k, Subject: sku, InvestigationQuestion: "clarify " + k}
	}
	t1 := []EvidenceRequest{er(types[0])}
	if len(types) > 1 {
		t1 = append(t1, er(types[1]))
	}
	t2 := []EvidenceRequest{}
	if len(types) > 1 {
		t2 = append(t2, er(types[1]))
	}
	if len(types) > 2 {
		t2 = append(t2, er(types[2]))
	}
	if len(t2) == 0 {
		t2 = []EvidenceRequest{er(types[0])}
	}
	out := OutputV1{
		DiscoveryState: StateThreadsFound,
		Threads: []ThreadV1{
			{PrimaryEntity: sku, RelatedEntities: []PackEntity{}, Observation: "SKU sales and price movement", InvestigationFocus: "confirm demand vs price", BasisFactHandles: []string{handle}, EvidenceRequests: t1},
			{PrimaryEntity: sku, RelatedEntities: []PackEntity{}, Observation: "price and search overlap", InvestigationFocus: "confirm search queries", BasisFactHandles: []string{handle}, EvidenceRequests: t2},
		},
	}
	b, err := json.Marshal(out)
	if err != nil {
		return EmptyThreadsJSON()
	}
	return string(b)
}

func fakeNonEmptyEnabled() bool {
	v := strings.TrimSpace(os.Getenv("DISCOVERY_FAKE_NONEMPTY"))
	return v == "1" || strings.EqualFold(v, "true")
}
