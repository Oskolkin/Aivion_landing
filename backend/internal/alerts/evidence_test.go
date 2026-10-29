package alerts

import (
	"encoding/json"
	"testing"
)

func TestEvidenceBuildersAreMarshalFriendly(t *testing.T) {
	min := 100.0
	max := 150.0
	stock := BuildStockEvidence(2.5, 12, 4.8, EvidencePayload{"sku": int64(123)})
	price := BuildPriceEvidence(95.0, &min, &max, nil, nil, EvidencePayload{"offer_id": "abc"})
	sales := BuildSalesEvidence(7, 1000, 1300, 10, 14, nil)
	ad := BuildAdvertisingEvidence(500, 120, 2, 600, 83.3, nil)

	payloads := []EvidencePayload{stock, price, sales, ad}
	for i, payload := range payloads {
		if _, err := json.Marshal(payload); err != nil {
			t.Fatalf("payload %d marshal failed: %v", i, err)
		}
	}
}

func TestBuildFingerprintIsStable(t *testing.T) {
	entityID := "product-42"
	entityIdentity := BuildEntityIdentity(EntityTypeProduct, &entityID, nil, nil)

	a := BuildFingerprint(10, AlertTypeSKURevenueDrop, EntityTypeProduct, entityIdentity)
	b := BuildFingerprint(10, AlertTypeSKURevenueDrop, EntityTypeProduct, entityIdentity)
	c := BuildFingerprint(10, AlertTypeSKUOrdersDrop, EntityTypeProduct, entityIdentity)

	if a == "" {
		t.Fatal("fingerprint is empty")
	}
	if a != b {
		t.Fatalf("fingerprint must be stable: a=%s b=%s", a, b)
	}
	if a == c {
		t.Fatalf("fingerprint must change when key fields change: a=%s c=%s", a, c)
	}
}
