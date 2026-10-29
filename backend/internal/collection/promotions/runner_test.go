package promotions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type recordingResolver struct {
	productID, sku *int64
	offer          string
	cutoff         time.Time
}

func (r *recordingResolver) ResolvePromotionSKU(_ context.Context, _ int64, productID, sku *int64, offer string, cutoff time.Time) (*int64, string, error) {
	r.productID, r.sku, r.offer, r.cutoff = productID, sku, offer, cutoff
	id := int64(77)
	return &id, "RESOLVED", nil
}

func TestProductIDIsNotTreatedAsSKUIdentity(t *testing.T) {
	productID := ozon.FlexibleInt64(1001)
	resolver := &recordingResolver{}
	cutoff := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	runner := Runner{SKUs: resolver}
	got, err := runner.normalizeProduct(context.Background(), 1, "OZON", 9, ozon.PromotionProduct{ProductID: productID, OfferID: "offer"}, 4, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceProductID == nil || *got.SourceProductID != 1001 || got.SourceSKU != nil {
		t.Fatalf("product identity collapsed into SKU: %+v", got)
	}
	if resolver.productID == nil || resolver.sku != nil || !resolver.cutoff.Equal(cutoff) {
		t.Fatalf("resolver did not receive AS KNOWN AT identifiers: %+v", resolver)
	}
}

func TestCandidateAndParticipantRemainDifferentFactKinds(t *testing.T) {
	candidate := SKUObservation{ProviderActionID: 5, SourceOfferID: "x"}
	result := Result{Eligibility: []EligibilitySnapshot{{ProviderActionID: 5, CompletenessStatus: "COMPLETE", Rows: []SKUObservation{candidate}}}}
	if len(result.Participants) != 0 {
		t.Fatal("candidate created participant membership")
	}
	if len(result.CompleteParticipantKeys) != 0 {
		t.Fatal("candidate enumeration became authoritative membership")
	}
}

func TestDefinitionPreservesRulesAndObservationDoesNotBackdateMembership(t *testing.T) {
	action := ozon.PromotionAction{ID: 12, DateStart: "2026-01-01", Raw: json.RawMessage(`{"rules":[{"type":"minimum_quantity","value":2}]}`)}
	def, err := normalizeDefinition(action, "OZON", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(def.RuleComponents) != 1 {
		t.Fatalf("rules=%s", def.RuleComponents)
	}
	observed := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if def.StartsAt == nil || !def.StartsAt.Before(observed) {
		t.Fatal("test requires promotion start before observation")
	}
	// Repository.Finalize uses Result.StartedAt for known_from, never StartsAt.
	result := Result{StartedAt: observed, Definitions: []DefinitionObservation{def}}
	if !result.StartedAt.Equal(observed) {
		t.Fatal("observation time was altered")
	}
}

func TestMembershipHashKeepsActionPriceAndStockVerticalLocal(t *testing.T) {
	base := SKUObservation{ProviderActionID: 1, SourceOfferID: "a", SKUResolutionStatus: "UNRESOLVED"}
	withStock := base
	stock := int64(3)
	withStock.ActionStock = &stock
	if membershipHash(base) == membershipHash(withStock) {
		t.Fatal("promotion-local stock change was compressed")
	}
}

func TestMembershipHashUsesDereferencedOptionalPointers(t *testing.T) {
	sku1, sku2 := int64(9), int64(9)
	stock1, stock2 := int64(4), int64(4)
	a := SKUObservation{ProviderActionID: 1, SourceOfferID: "a", CanonicalSKUID: &sku1, ActionStock: &stock1, SKUResolutionStatus: "RESOLVED"}
	b := SKUObservation{ProviderActionID: 1, SourceOfferID: "a", CanonicalSKUID: &sku2, ActionStock: &stock2, SKUResolutionStatus: "RESOLVED"}
	if membershipHash(a) != membershipHash(b) {
		t.Fatal("separately allocated equal pointers changed membership hash")
	}
	start1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d1 := DefinitionObservation{Origin: "OZON", ProviderActionID: 1, ProviderState: "ACTIVE", StartsAt: &start1}
	d2 := DefinitionObservation{Origin: "OZON", ProviderActionID: 1, ProviderState: "ACTIVE", StartsAt: &start2}
	if definitionHash(d1) != definitionHash(d2) {
		t.Fatal("separately allocated equal starts_at pointers changed definition hash")
	}
}
