package alerts

import "testing"

func TestFingerprintDiffersByEntityIdentity(t *testing.T) {
	entityA := "product-100"
	entityB := "product-101"

	f1 := BuildFingerprint(1, AlertTypePriceBelowMinConstraint, EntityTypeProduct, BuildEntityIdentity(EntityTypeProduct, &entityA, nil, nil))
	f2 := BuildFingerprint(1, AlertTypePriceBelowMinConstraint, EntityTypeProduct, BuildEntityIdentity(EntityTypeProduct, &entityB, nil, nil))

	if f1 == f2 {
		t.Fatalf("expected different fingerprints for different entities, got same=%s", f1)
	}
}

func TestRuleResultAutoFingerprintIgnoresPresentationFields(t *testing.T) {
	productID := "12345"
	sku := int64(100500)
	offer := "offer-1"

	base := RuleResult{
		AlertType:     AlertTypeSKURevenueDrop,
		AlertGroup:    AlertGroupSales,
		EntityType:    EntityTypeSKU,
		EntityID:      &productID,
		EntitySKU:     &sku,
		EntityOfferID: &offer,
		Title:         "old title",
		Message:       "old message",
		Severity:      SeverityMedium,
		Urgency:       UrgencyMedium,
		EvidencePayload: EvidencePayload{
			"any": "value",
		},
	}
	changed := base
	changed.Title = "new title"
	changed.Message = "new message"
	changed.Severity = SeverityCritical
	changed.Urgency = UrgencyImmediate
	changed.EvidencePayload = EvidencePayload{"any": "another"}

	inA := RuleResultToUpsertInput(77, base)
	inB := RuleResultToUpsertInput(77, changed)

	if inA.Fingerprint == "" || inB.Fingerprint == "" {
		t.Fatal("auto-generated fingerprint must not be empty")
	}
	if inA.Fingerprint != inB.Fingerprint {
		t.Fatalf("fingerprint must not depend on title/message/evidence/severity: a=%s b=%s", inA.Fingerprint, inB.Fingerprint)
	}
}

func TestRuleResultWithExplicitFingerprintKeepsIt(t *testing.T) {
	explicit := "manual-fingerprint-123"
	result := RuleResult{
		AlertType:   AlertTypeStockOOSRisk,
		AlertGroup:  AlertGroupStock,
		EntityType:  EntityTypeSKU,
		Fingerprint: explicit,
		Title:       "x",
		Message:     "y",
		Severity:    SeverityHigh,
		Urgency:     UrgencyHigh,
	}

	in := RuleResultToUpsertInput(10, result)
	if in.Fingerprint != explicit {
		t.Fatalf("expected explicit fingerprint to be preserved, got=%s", in.Fingerprint)
	}
}
