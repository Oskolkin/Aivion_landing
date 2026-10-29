package supply

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestOrderAndSupplyIdentitiesNeverCollapse(t *testing.T) {
	order := normalizeOrder(ozon.SupplyOrder{OrderID: "42", State: "CREATED"}, 1)
	supply := normalizeSupply(ozon.Supply{OrderID: "42", SupplyID: "99", State: "IN_TRANSIT"}, 2)
	if order.ProviderOrderID != "42" || supply.ProviderSupplyID != "99" || supply.ProviderOrderID != "42" {
		t.Fatalf("identities collapsed: order=%+v supply=%+v", order, supply)
	}
	if order.ProviderOrderID == supply.ProviderSupplyID {
		t.Fatal("test requires distinct provider identities")
	}
}

func TestBundleIdentityDoesNotBecomeSupplyIdentity(t *testing.T) {
	bundle := BundleObservation{ProviderSupplyID: "s-1", ProviderBundleID: "B1"}
	next := bundle
	next.ProviderBundleID = "B2"
	if bundle.ProviderSupplyID != next.ProviderSupplyID {
		t.Fatal("bundle revision changed supply identity")
	}
	if bundle.ProviderBundleID == next.ProviderBundleID {
		t.Fatal("test requires plan revision")
	}
}

func TestUnknownStateIsRetained(t *testing.T) {
	canonical, status := MapState("NEW_PROVIDER_STATE")
	if canonical != "UNKNOWN" || status != "UNKNOWN" {
		t.Fatalf("unknown state was synthesized: %s/%s", canonical, status)
	}
	obs := normalizeSupply(ozon.Supply{OrderID: "o", SupplyID: "s", State: "NEW_PROVIDER_STATE", Raw: json.RawMessage(`{"state":"NEW_PROVIDER_STATE"}`)}, 1)
	if obs.ProviderState != "NEW_PROVIDER_STATE" || string(obs.ProviderFields) == "" {
		t.Fatalf("provider state not retained: %+v", obs)
	}
}

func TestPartialBundleDoesNotClaimCompleteComposition(t *testing.T) {
	bundle := BundleObservation{ProviderSupplyID: "s", CompletenessStatus: "PARTIAL", Items: []PlannedItem{{SourceOfferID: "a"}}}
	if bundle.CompletenessStatus == "COMPLETE" {
		t.Fatal("partial pagination closed composition")
	}
}

func TestTimeslotIsNotExpectedStockAt(t *testing.T) {
	order := normalizeOrder(ozon.SupplyOrder{OrderID: "o", TimeslotFrom: "2026-08-27T10:00:00Z"}, 1)
	if order.TimeslotFrom == nil {
		t.Fatal("timeslot was not preserved")
	}
}

func TestStateObservationChangeCompression(t *testing.T) {
	// poll1 READY → write; poll2 READY → skip; poll3 IN_TRANSIT → write; poll4 READY → write (A→B→A).
	type step struct {
		provider, canonical string
		expectChange        bool
	}
	var prevProvider, prevCanonical, prevStatus string
	havePrev := false
	written := 0
	for i, s := range []step{
		{"READY_TO_SUPPLY", "READY", true},
		{"READY_TO_SUPPLY", "READY", false},
		{"IN_TRANSIT", "IN_TRANSIT", true},
		{"READY_TO_SUPPLY", "READY", true},
	} {
		changed := !havePrev || StateObservationChanged(prevProvider, prevCanonical, prevStatus, s.provider, s.canonical, "MAPPED")
		if changed != s.expectChange {
			t.Fatalf("poll%d: changed=%v want %v", i+1, changed, s.expectChange)
		}
		if changed {
			written++
			prevProvider, prevCanonical, prevStatus = s.provider, s.canonical, "MAPPED"
			havePrev = true
		}
	}
	if written != 3 {
		t.Fatalf("expected 3 observations for READY→READY→IN_TRANSIT→READY, got %d", written)
	}
}

func TestPtrHashMarkersAreDeterministic(t *testing.T) {
	var nilID *int64
	id := int64(42)
	if ptrInt64Marker(nilID) != "NULL" || ptrInt64Marker(&id) != "42" {
		t.Fatalf("ptrInt64Marker failed: nil=%q id=%q", ptrInt64Marker(nilID), ptrInt64Marker(&id))
	}
	var nilTime *time.Time
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if ptrTimeMarker(nilTime) != "NULL" || ptrTimeMarker(&ts) != "2026-08-26T12:00:00Z" {
		t.Fatalf("ptrTimeMarker failed: nil=%q ts=%q", ptrTimeMarker(nilTime), ptrTimeMarker(&ts))
	}
	a := factualapi.StableHash(ptrInt64Marker(&id), ptrTimeMarker(&ts), ptrInt64Marker(nilID))
	b := factualapi.StableHash(ptrInt64Marker(&id), ptrTimeMarker(&ts), ptrInt64Marker(nilID))
	if a != b || a == "" {
		t.Fatal("hash markers must be deterministic")
	}
}

type fakeSupplyAPI struct {
	list       ozon.SupplyOrderListResult
	get        ozon.SupplyOrder
	details    ozon.SupplyOrder
	bundle     ozon.SupplyBundleResult
	summary    *ozon.TypedResponse[ozon.SupplyActSummary]
	summaryErr error
	products   *ozon.TypedResponse[ozon.SupplyActProductResult]
	productErr error
}

func (f *fakeSupplyAPI) ListSupplyOrders(context.Context, int64, ozon.SupplyOrderListRequest) (*ozon.TypedResponse[ozon.SupplyOrderListResult], error) {
	return &ozon.TypedResponse[ozon.SupplyOrderListResult]{Data: f.list, Raw: []byte(`{"orders":[]}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeSupplyAPI) GetSupplyOrder(context.Context, int64, ozon.SupplyOrderRequest) (*ozon.TypedResponse[ozon.SupplyOrder], error) {
	return &ozon.TypedResponse[ozon.SupplyOrder]{Data: f.get, Raw: []byte(`{}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeSupplyAPI) GetSupplyOrderDetails(context.Context, int64, ozon.SupplyOrderRequest) (*ozon.TypedResponse[ozon.SupplyOrder], error) {
	return &ozon.TypedResponse[ozon.SupplyOrder]{Data: f.details, Raw: []byte(`{}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeSupplyAPI) GetSupplyBundle(context.Context, int64, ozon.SupplyBundleRequest) (*ozon.TypedResponse[ozon.SupplyBundleResult], error) {
	return &ozon.TypedResponse[ozon.SupplyBundleResult]{Data: f.bundle, Raw: []byte(`{}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeSupplyAPI) GetSupplyActSummary(context.Context, int64, ozon.SupplyActRequest) (*ozon.TypedResponse[ozon.SupplyActSummary], error) {
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	return f.summary, nil
}
func (f *fakeSupplyAPI) GetSupplyActProducts(context.Context, int64, ozon.SupplyActRequest) (*ozon.TypedResponse[ozon.SupplyActProductResult], error) {
	if f.productErr != nil {
		return nil, f.productErr
	}
	return f.products, nil
}

type fakeRegistrar struct{ next int64 }

func (r *fakeRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, _ collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	r.next++
	return collection.SourcePayload{ID: r.next, ReceivedAt: time.Unix(r.next, 0).UTC()}, nil
}

func baseFixture() *fakeSupplyAPI {
	order := ozon.SupplyOrder{
		OrderID: "ORDER-1", State: "READY_TO_SUPPLY",
		Supplies: []ozon.Supply{{OrderID: "ORDER-1", SupplyID: "SUPPLY-1", State: "READY_TO_SUPPLY"}},
	}
	return &fakeSupplyAPI{
		list:    ozon.SupplyOrderListResult{Orders: []ozon.SupplyOrder{{OrderID: "ORDER-1", State: "READY_TO_SUPPLY"}}},
		get:     order,
		details: order,
		bundle:  ozon.SupplyBundleResult{BundleID: "B1", Items: []ozon.SupplyBundleItem{{ID: "1", OfferID: "offer"}}},
	}
}

func TestAcceptanceCollectionPersistsIndependentFacts(t *testing.T) {
	api := baseFixture()
	accepted, shortage, surplus := int64(8), int64(1), int64(2)
	api.summary = &ozon.TypedResponse[ozon.SupplyActSummary]{
		Data: ozon.SupplyActSummary{ActID: "ACT-1", SupplyID: "SUPPLY-1", State: "DONE", TotalAmount: "10.5", CurrencyCode: "RUB"},
		Raw:  []byte(`{"act_id":"ACT-1"}`), Meta: ozon.ResponseMeta{StatusCode: 200},
	}
	api.products = &ozon.TypedResponse[ozon.SupplyActProductResult]{
		Data: ozon.SupplyActProductResult{Items: []ozon.SupplyActItem{{
			SKU: 101, OfferID: "offer", Accepted: &accepted, Shortage: &shortage, Surplus: &surplus, Amount: "10.5", CurrencyCode: "RUB",
			Raw: json.RawMessage(`{"sku":101,"accepted":8,"shortage":1,"surplus":2}`),
		}}},
		Raw: []byte(`{"items":[]}`), Meta: ozon.ResponseMeta{StatusCode: 200},
	}
	runner := &Runner{API: api, Collection: &fakeRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC) }}
	result, err := runner.Collect(context.Background(), 1, 9)
	if err != nil {
		t.Fatal(err)
	}
	if result.CompletenessStatus != "COMPLETE" {
		t.Fatalf("core supply failed: %+v", result)
	}
	if result.AcceptanceSummaryStatus != "COMPLETE" || result.AcceptanceProductsStatus != "COMPLETE" {
		t.Fatalf("acceptance status=%s/%s", result.AcceptanceSummaryStatus, result.AcceptanceProductsStatus)
	}
	if len(result.Acceptances) != 1 || len(result.Acceptances[0].Items) != 1 {
		t.Fatalf("acceptances=%+v", result.Acceptances)
	}
	item := result.Acceptances[0].Items[0]
	if item.AcceptedUnits == nil || *item.AcceptedUnits != 8 || item.ShortageUnits == nil || *item.ShortageUnits != 1 || item.SurplusUnits == nil || *item.SurplusUnits != 2 {
		t.Fatalf("independent units lost: %+v", item)
	}
}

func TestAcceptanceUnavailableDoesNotFailCoreSupply(t *testing.T) {
	api := baseFixture()
	api.summaryErr = &ozon.ProviderError{StatusCode: http.StatusForbidden, Body: "forbidden"}
	runner := &Runner{API: api, Collection: &fakeRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC) }}
	result, err := runner.Collect(context.Background(), 1, 9)
	if err != nil {
		t.Fatal(err)
	}
	if result.CompletenessStatus != "COMPLETE" {
		t.Fatalf("core supply must continue: %+v", result)
	}
	if len(result.Orders) != 1 || len(result.Supplies) != 1 {
		t.Fatalf("core facts missing: %+v", result)
	}
	if result.AcceptanceSummaryStatus != "FORBIDDEN" || result.AcceptanceProductsStatus != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN acceptance status, got %s/%s", result.AcceptanceSummaryStatus, result.AcceptanceProductsStatus)
	}
	if len(result.Acceptances) != 0 {
		t.Fatalf("forbidden acceptance must not invent rows: %+v", result.Acceptances)
	}
}

func TestAcceptanceUnavailableClassifiesSoftErrors(t *testing.T) {
	if got := classifyAcceptanceErr(&ozon.ProviderError{StatusCode: 404}); got != "UNAVAILABLE" {
		t.Fatalf("404 → %s", got)
	}
	if got := classifyAcceptanceErr(errors.New("pending acceptance")); got != "PENDING_ACCEPTANCE" {
		t.Fatalf("pending → %s", got)
	}
}
