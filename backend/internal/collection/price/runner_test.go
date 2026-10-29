package price

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestNormalizePricePreservesExactSemantics(t *testing.T) {
	var row ozon.CanonicalPriceRow
	if err := json.Unmarshal([]byte(`{"product_id":10,"currency_code":"RUB","price":{"price":"123456789012345.123456","old_price":"0","marketing_seller_price":"119.990001","marketing_price":"1.25","min_price":"0","retail_price":"125","vat":"20","auto_action_enabled":true},"price_indexes":{"color_index":"WITHOUT_INDEX"},"marketing_actions":[{"title":"promo"}]}`), &row); err != nil {
		t.Fatal(err)
	}
	id, sku, product := int64(1), int64(2), int64(10)
	got, err := normalize(row, &id, &sku, &product, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCurrentPrice == nil || got.ProviderCurrentPrice.String() != "123456789012345.123456" {
		t.Fatalf("current price lost precision: %v", got.ProviderCurrentPrice)
	}
	if got.OldReferencePrice != nil {
		t.Fatalf("zero old_price must mean not set: %v", got.OldReferencePrice)
	}
	if got.MinimumAllowedPrice != nil {
		t.Fatalf("zero min_price must mean not set: %v", got.MinimumAllowedPrice)
	}
	if got.CustomerPrice != nil {
		t.Fatalf("marketing_price must not become customer price: %v", got.CustomerPrice)
	}
	if got.SellerPromoPrice == nil || got.SellerPromoPrice.String() != "119.990001" || got.CurrencyCode != "RUB" {
		t.Fatalf("promo/currency=%v/%s", got.SellerPromoPrice, got.CurrencyCode)
	}
	if len(got.PromotionContextJSON) == 0 || string(got.ProviderFields) == "" || got.SemanticMappingStatus != "PENDING_ACCEPTANCE" {
		t.Fatalf("promotion/raw/semantic evidence missing: %+v", got)
	}
	if !bytes.Contains(got.ProviderFields, []byte(`"marketing_price"`)) {
		t.Fatalf("marketing_price must stay in provider context only")
	}
}

func TestNormalizeZeroCurrentPriceIsInvalidNotZeroFact(t *testing.T) {
	var row ozon.CanonicalPriceRow
	if err := json.Unmarshal([]byte(`{"product_id":10,"price":{"price":"0"}}`), &row); err != nil {
		t.Fatal(err)
	}
	got, err := normalize(row, nil, nil, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCurrentPrice != nil || got.CurrentPriceState != "INVALID" {
		t.Fatalf("zero current price normalized as a fact: %+v", got)
	}
}

func TestFieldSemanticRegistryHasIndependentZeroPolicies(t *testing.T) {
	if FieldSemanticsRegistryV1["price.price"].ZeroMeaning != "INVALID_CURRENT_PRICE" || FieldSemanticsRegistryV1["price.old_price"].ZeroMeaning != "NOT_SET" || FieldSemanticsRegistryV1["price.marketing_price"].CanonicalRole != "RAW_PROVIDER_CONTEXT_ONLY" {
		t.Fatalf("field semantic registry=%+v", FieldSemanticsRegistryV1)
	}
}

type priceReplayObjects struct{ bodies map[string][]byte }

func (o *priceReplayObjects) Put(_ context.Context, bucket, key string, body []byte, _ string) error {
	if o.bodies == nil {
		o.bodies = map[string][]byte{}
	}
	o.bodies[bucket+"/"+key] = append([]byte(nil), body...)
	return nil
}
func (o *priceReplayObjects) Get(_ context.Context, bucket, key string) ([]byte, error) {
	b, ok := o.bodies[bucket+"/"+key]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), b...), nil
}

type priceReplayHarness struct {
	store    *collection.SourcePayloadStore
	payloads []collection.SourcePayload
}

func newPriceReplayHarness() *priceReplayHarness {
	objects := &priceReplayObjects{}
	return &priceReplayHarness{store: collection.NewSourcePayloadStoreWithObjects(objects, "raw")}
}
func (h *priceReplayHarness) StoreAndRegisterPayload(ctx context.Context, store *collection.SourcePayloadStore, in collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	stored, err := store.Put(ctx, in.SellerAccountID, collection.DomainPrice, in.Body, in.ContentType)
	if err != nil {
		return collection.SourcePayload{}, err
	}
	identity := map[string]any{"collection_domain": string(collection.DomainPrice)}
	for k, v := range in.RequestIdentity {
		identity[k] = v
	}
	fingerprint, err := collection.RequestFingerprint(in.Provider, in.EndpointKey, identity)
	if err != nil {
		return collection.SourcePayload{}, err
	}
	metadata, _ := json.Marshal(in.Metadata)
	id := int64(len(h.payloads) + 1)
	p := collection.SourcePayload{ID: id, SellerAccountID: in.SellerAccountID, DomainRunID: in.DomainRunID, Provider: in.Provider, EndpointKey: in.EndpointKey, RequestFingerprint: fingerprint, Bucket: stored.Bucket, ObjectKey: stored.ObjectKey, SHA256: stored.SHA256, ReceivedAt: time.Unix(id, 0).UTC(), Metadata: metadata, RedactionProfile: RedactionProfile, PageSeq: in.PageSeq, HTTPStatus: in.HTTPStatus}
	h.payloads = append(h.payloads, p)
	return p, nil
}
func (h *priceReplayHarness) ListPayloads(_ context.Context, seller, run int64) ([]collection.SourcePayload, error) {
	var out []collection.SourcePayload
	for _, p := range h.payloads {
		if p.SellerAccountID == seller && p.DomainRunID == run {
			out = append(out, p)
		}
	}
	return out, nil
}

type fixedPriceUniverse []FrozenSKU

func (u fixedPriceUniverse) ResolvePriceUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error) {
	return u, nil
}

type priceReplayAPI struct {
	calls int
	fail  bool
}

func (a *priceReplayAPI) ListCanonicalPrices(_ context.Context, _ int64, _ ozon.CanonicalPriceRequest) (*ozon.TypedResponse[ozon.CanonicalPriceResult], error) {
	a.calls++
	if a.fail {
		return nil, errors.New("provider must not be called")
	}
	raw := []byte(`{"items":[{"product_id":10,"sku":101,"currency_code":"RUB","price":{"price":"10.25","old_price":"0"}}],"cursor":"","total":1}`)
	var result ozon.CanonicalPriceResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &ozon.TypedResponse[ozon.CanonicalPriceResult]{Raw: raw, Data: result, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func TestSameRunReplayUsesImmutablePriceArtifactAndSkipsProvider(t *testing.T) {
	h := newPriceReplayHarness()
	universe := fixedPriceUniverse{{CanonicalSKUID: 1, OzonSKU: 101, ProviderProductID: 10}}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	firstAPI := &priceReplayAPI{}
	first, err := (&Runner{API: firstAPI, Universe: universe, Collection: h, Payloads: h.store, Recovery: h, Now: func() time.Time { return now }}).Collect(context.Background(), 7, 9)
	if err != nil {
		t.Fatal(err)
	}
	secondAPI := &priceReplayAPI{fail: true}
	second, err := (&Runner{API: secondAPI, Universe: universe, Collection: h, Payloads: h.store, Recovery: h, Now: func() time.Time { return now }}).Collect(context.Background(), 7, 9)
	if err != nil {
		t.Fatal(err)
	}
	if firstAPI.calls != 1 || secondAPI.calls != 0 || len(h.payloads) != 1 || len(second.Observations) != 1 || first.UniverseHash != second.UniverseHash || second.Observations[0].ProviderCurrentPrice.String() != "10.25" {
		t.Fatalf("first=%+v second=%+v calls=%d/%d payloads=%d", first, second, firstAPI.calls, secondAPI.calls, len(h.payloads))
	}
}
