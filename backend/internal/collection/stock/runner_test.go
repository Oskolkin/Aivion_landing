package stock

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type stockUniverse []FrozenSKU

func (u stockUniverse) ResolveStockUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error) {
	return u, nil
}

type stockAPI struct{ requests []ozon.CanonicalStockRequest }

func (a *stockAPI) ListCanonicalStocks(_ context.Context, _ int64, req ozon.CanonicalStockRequest) (*ozon.TypedResponse[[]ozon.CanonicalStockRow], error) {
	a.requests = append(a.requests, req)
	var rows []ozon.CanonicalStockRow
	for _, sku := range req.SKUs {
		var row ozon.CanonicalStockRow
		_ = json.Unmarshal([]byte(`{"sku":`+sku+`,"warehouse_id":9,"valid_stock_count":5,"ads":0.25,"idc":4}`), &row)
		rows = append(rows, row)
	}
	return &ozon.TypedResponse[[]ozon.CanonicalStockRow]{Data: rows, Raw: []byte(`{"items":[]}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}

type stockRegistrar struct{ next int64 }

func (r *stockRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, _ collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	r.next++
	return collection.SourcePayload{ID: r.next, ReceivedAt: time.Unix(r.next, 0).UTC()}, nil
}

func TestRunnerFreezesUniverseBatchesAndDoesNotRenameProviderStock(t *testing.T) {
	u := make(stockUniverse, ozon.StockAnalyticsBatchLimit+1)
	for i := range u {
		u[i] = FrozenSKU{CanonicalSKUID: int64(i + 1), OzonSKU: int64(1000 + i)}
	}
	api := &stockAPI{}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	result, err := (&Runner{API: api, Universe: u, Collection: &stockRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return now }}).Collect(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 2 || len(api.requests[0].SKUs) != ozon.StockAnalyticsBatchLimit || len(api.requests[1].SKUs) != 1 {
		t.Fatalf("batches=%+v", api.requests)
	}
	if result.UniverseHash == "" || result.ExpectedSKUCount != len(u) || result.CompletenessStatus != "COMPLETE" {
		t.Fatalf("result=%+v", result)
	}
	for _, p := range result.Positions {
		if p.IsAbsence() {
			continue
		}
		if p.SellableUnits != nil || p.ProviderValidStockCount == nil || *p.ProviderValidStockCount != 5 || p.QuantitySemanticStatus != "PENDING_ACCEPTANCE" || p.PoolIndependenceStatus != "UNVERIFIED" {
			t.Fatalf("provider field was guessed as canonical sellable stock: %+v", p)
		}
		if p.SourcePayloadID == nil || *p.SourcePayloadID == 0 {
			t.Fatalf("provider position missing payload: %+v", p)
		}
	}
}

func TestCanonicalStockJSONDistinguishesMissingFromKnownZero(t *testing.T) {
	var missing, zero ozon.CanonicalStockRow
	if err := json.Unmarshal([]byte(`{"sku":1}`), &missing); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"sku":1,"valid_stock_count":0,"days_without_sales":0}`), &zero); err != nil {
		t.Fatal(err)
	}
	if missing.ValidStockCount != nil || missing.ProviderDaysWithoutSales != nil {
		t.Fatalf("missing provider values became zero: %+v", missing)
	}
	if zero.ValidStockCount == nil || *zero.ValidStockCount != 0 || zero.ProviderDaysWithoutSales == nil || *zero.ProviderDaysWithoutSales != 0 {
		t.Fatalf("known zero was lost: %+v", zero)
	}
}

func TestRunnerMaterializesMissingResponseSKUAsUnknownQuantity(t *testing.T) {
	u := stockUniverse{{CanonicalSKUID: 1, OzonSKU: 101}, {CanonicalSKUID: 2, OzonSKU: 102}}
	api := &stockAPISparse{present: map[string]bool{"101": true}}
	result, err := (&Runner{API: api, Universe: u, Collection: &stockRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC) }}).Collect(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReceivedSKUCount != 1 || result.MissingResponseSKUCount != 1 || result.CompletenessStatus != "PARTIAL" || len(result.Positions) != 2 {
		t.Fatalf("result=%+v", result)
	}
	var missing Position
	for _, p := range result.Positions {
		if p.SourceSKU == 102 {
			missing = p
		}
	}
	if missing.SellableUnits != nil || missing.ProviderValidStockCount != nil || missing.AbsenceReason != "MISSING_PROVIDER_RESPONSE_ROW" || missing.SourcePayloadID != nil {
		t.Fatalf("missing response became zero or fake payload: %+v", missing)
	}
}

func TestRunnerFailedBatchIsUnavailableNotMissing(t *testing.T) {
	u := stockUniverse{{CanonicalSKUID: 1, OzonSKU: 101}, {CanonicalSKUID: 2, OzonSKU: 102}}
	api := &stockAPIFail{}
	result, err := (&Runner{API: api, Universe: u, Collection: &stockRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC) }}).Collect(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.BatchesFailed != 1 || result.BatchUnavailableSKUCount != 2 || result.MissingResponseSKUCount != 0 || result.CompletenessStatus != "PARTIAL" {
		t.Fatalf("failed batch semantics wrong: %+v", result)
	}
	for _, p := range result.Positions {
		if p.AbsenceReason != "SOURCE_BATCH_UNAVAILABLE" {
			t.Fatalf("expected unavailable absence: %+v", p)
		}
	}
}

type stockAPISparse struct{ present map[string]bool }

func (a *stockAPISparse) ListCanonicalStocks(_ context.Context, _ int64, req ozon.CanonicalStockRequest) (*ozon.TypedResponse[[]ozon.CanonicalStockRow], error) {
	var rows []ozon.CanonicalStockRow
	for _, sku := range req.SKUs {
		if !a.present[sku] {
			continue
		}
		var row ozon.CanonicalStockRow
		_ = json.Unmarshal([]byte(`{"sku":`+sku+`,"warehouse_id":9,"valid_stock_count":0}`), &row)
		rows = append(rows, row)
	}
	return &ozon.TypedResponse[[]ozon.CanonicalStockRow]{Data: rows, Raw: []byte(`{"items":[]}`), Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}

type stockAPIFail struct{}

func (a *stockAPIFail) ListCanonicalStocks(context.Context, int64, ozon.CanonicalStockRequest) (*ozon.TypedResponse[[]ozon.CanonicalStockRow], error) {
	return nil, errors.New("provider unavailable")
}

func TestVerifiedIndependentPoolsAreDistinctKeys(t *testing.T) {
	a := Position{ProviderPoolKey: "FBO:WAREHOUSE:1", FulfillmentType: "FBO", PoolIndependenceStatus: "VERIFIED_INDEPENDENT"}
	b := Position{ProviderPoolKey: "FBS:WAREHOUSE:1", FulfillmentType: "FBS", PoolIndependenceStatus: "UNVERIFIED"}
	if a.ProviderPoolKey == b.ProviderPoolKey {
		t.Fatal("FBO/FBS must not share pool key")
	}
}
