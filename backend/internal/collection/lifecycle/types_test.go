package lifecycle

import (
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func postingFixture(status string, quantity string) ozon.CanonicalPosting {
	var p ozon.CanonicalPosting
	p.OrderID = 1
	p.OrderNumber = "ORDER-1"
	p.PostingNumber = "POST-1"
	p.Status = status
	p.CreatedAt = "2026-08-20T10:00:00Z"
	p.DeliveryMethod.Schema = "FBS"
	p.Products = []ozon.PostingProduct{{SKU: 101, OfferID: "offer", Quantity: ozon.Scalar(quantity), Price: ozon.Scalar("9007199254740993.123456"), CurrencyCode: "RUB"}}
	return p
}
func TestPostingNormalizationIdentityMoneyQuantityAndCompositionHash(t *testing.T) {
	a, e := normalizePosting(postingFixture("awaiting_deliver", "3"), "FBS", 10, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if a.Scheme != "FBS" || a.Products[0].UnitPrice.String() != "9007199254740993.123456" || a.Products[0].Quantity != 3 {
		t.Fatalf("candidate=%+v", a)
	}
	bRaw := postingFixture("awaiting_deliver", "2")
	b, e := normalizePosting(bRaw, "FBS", 11, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if a.StateHash == b.StateHash {
		t.Fatal("composition change did not change hash")
	}
	c, e := normalizePosting(postingFixture("delivered", "3"), "FBS", 12, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if a.StateHash == c.StateHash {
		t.Fatal("status change did not change hash")
	}
}
func TestPostingNormalizationRejectsFractionalNegativeAndMissingOrder(t *testing.T) {
	for _, q := range []string{"1.5", "-1"} {
		_, e := normalizePosting(postingFixture("new", q), "FBS", 1, time.Now())
		if !errors.Is(e, ErrInvalidPostingQuantity) {
			t.Fatalf("q=%s err=%v", q, e)
		}
	}
	p := postingFixture("new", "1")
	p.PostingNumber = ""
	if _, e := normalizePosting(p, "FBS", 1, time.Now()); !errors.Is(e, ErrSourceContractMismatch) {
		t.Fatalf("missing posting identity err=%v", e)
	}
}
func TestFulfillmentSchemeUsesExplicitFBSFamilySchema(t *testing.T) {
	p := postingFixture("new", "1")
	p.DeliveryMethod.Schema = "rFBS"
	c, e := normalizePosting(p, "FBS", 1, time.Now())
	if e != nil || c.Scheme != "RFBS" {
		t.Fatalf("candidate=%+v err=%v", c, e)
	}
	p.DeliveryMethod.Schema = "mystery"
	c, e = normalizePosting(p, "FBS", 1, time.Now())
	if e != nil || c.Scheme != "UNKNOWN" {
		t.Fatalf("candidate=%+v err=%v", c, e)
	}
}
func TestReturnNormalizationPreservesProviderSemanticsWithoutClassification(t *testing.T) {
	var raw ozon.CanonicalReturn
	raw.ID = 77
	raw.SourceID = 99
	raw.Type = "some_provider_type"
	raw.Schema = "FBO"
	raw.ReturnReasonName = "provider reason"
	raw.Quantity = "1"
	raw.Price = "12.345678"
	raw.CurrencyCode = "RUB"
	raw.Logistic.ReturnDate = "2026-08-20T12:00:00Z"
	c, e := normalizeReturn(raw, 1, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if c.ProviderReturnID != 77 || c.SourceID != 99 || c.ProviderType != "some_provider_type" || c.ProviderSchema != "FBO" || c.Reason != "provider reason" || c.ProviderReturnDate == nil {
		t.Fatalf("candidate=%+v", c)
	}
}

func TestReturnMissingSKUNeverUsesOtherProductIdentity(t *testing.T) {
	var raw ozon.CanonicalReturn
	raw.ID = 88
	raw.Quantity = "1"
	raw.Price = "1"
	raw.CurrencyCode = "RUB"
	raw.SKU = 0
	c, e := normalizeReturn(raw, 1, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if c.SourceSKU != 0 {
		t.Fatalf("source SKU=%d", c.SourceSKU)
	}
}
func TestTemporalHelpersNeverInventApproximateExactTime(t *testing.T) {
	from := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	same := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	cross := time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)
	if approxBusinessDate(&from, same) == nil {
		t.Fatal("same business day not resolved")
	}
	if approxBusinessDate(&from, cross) != nil {
		t.Fatal("cross-boundary business date invented")
	}
}

func TestStatusEventMappingDoesNotTreatAwaitingDeliveryAsDelivering(t *testing.T) {
	for status, want := range map[string]string{
		"awaiting_deliver": "",
		"delivering":       "POSTING_DELIVERING",
		"delivered":        "POSTING_DELIVERED",
		"cancelled":        "POSTING_CANCELLED_FULL",
	} {
		if got := statusEvent(status); got != want {
			t.Fatalf("status %q: got %q want %q", status, got, want)
		}
	}
}

func TestProductIDIsNeverUsedAsSourceSKU(t *testing.T) {
	raw := postingFixture("new", "1")
	raw.Products[0].SKU = 111
	raw.Products[0].ProductID = 222
	c, e := normalizePosting(raw, "FBS", 1, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if c.Products[0].SourceSKU != 111 {
		t.Fatalf("source SKU=%d", c.Products[0].SourceSKU)
	}
	raw.Products[0].SKU = 0
	c, e = normalizePosting(raw, "FBS", 1, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if c.Products[0].SourceSKU != 0 {
		t.Fatalf("product_id became SKU: %d", c.Products[0].SourceSKU)
	}
}

func TestNeedPostingDetailUsesRequiredLifecycleFields(t *testing.T) {
	p := postingFixture("new", "1")
	if NeedPostingDetail(p) {
		t.Fatal("complete list candidate requested detail")
	}
	p.OrderNumber = ""
	if !NeedPostingDetail(p) {
		t.Fatal("missing required order number did not request detail")
	}
}
