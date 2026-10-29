package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
)

type replayObjects struct{ bodies map[string][]byte }

func (o *replayObjects) Put(_ context.Context, bucket, key string, body []byte, _ string) error {
	if o.bodies == nil {
		o.bodies = map[string][]byte{}
	}
	o.bodies[bucket+"/"+key] = append([]byte(nil), body...)
	return nil
}
func (o *replayObjects) Get(_ context.Context, bucket, key string) ([]byte, error) {
	body, ok := o.bodies[bucket+"/"+key]
	if !ok {
		return nil, errors.New("artifact missing")
	}
	return append([]byte(nil), body...), nil
}

type replayHarness struct {
	store    *collection.SourcePayloadStore
	objects  *replayObjects
	payloads []collection.SourcePayload
	cursors  map[string]collection.Cursor
	nextID   int64
}

func newReplayHarness() *replayHarness {
	objects := &replayObjects{}
	return &replayHarness{store: collection.NewSourcePayloadStoreWithObjects(objects, "raw"), objects: objects, cursors: map[string]collection.Cursor{}}
}
func (h *replayHarness) StoreAndRegisterPayload(ctx context.Context, store *collection.SourcePayloadStore, in collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	stored, err := store.Put(ctx, in.SellerAccountID, collection.DomainSalesDemand, in.Body, in.ContentType)
	if err != nil {
		return collection.SourcePayload{}, err
	}
	h.nextID++
	metadata, _ := json.Marshal(in.Metadata)
	fingerprint, err := collection.RequestFingerprint(in.Provider, in.EndpointKey, withDomain(in.RequestIdentity))
	if err != nil {
		return collection.SourcePayload{}, err
	}
	received := time.Date(2026, 8, 24, 10, 0, int(h.nextID), 0, time.UTC)
	payload := collection.SourcePayload{ID: h.nextID, SellerAccountID: in.SellerAccountID, DomainRunID: in.DomainRunID, Provider: in.Provider, EndpointKey: in.EndpointKey, RequestFingerprint: fingerprint, Bucket: stored.Bucket, ObjectKey: stored.ObjectKey, SHA256: stored.SHA256, RedactionProfile: "OZON_SALES_DEMAND_REDACTION_V1", ReceivedAt: received, Metadata: metadata, PageSeq: in.PageSeq, HTTPStatus: in.HTTPStatus}
	h.payloads = append(h.payloads, payload)
	return payload, nil
}
func (h *replayHarness) UpsertCursor(_ context.Context, sellerID int64, domain collection.CollectionDomain, key string, value []byte) (collection.Cursor, error) {
	cursor := collection.Cursor{ID: int64(len(h.cursors) + 1), SellerAccountID: sellerID, Domain: domain, Key: key, Value: append([]byte(nil), value...)}
	h.cursors[key] = cursor
	return cursor, nil
}
func (h *replayHarness) GetCursor(_ context.Context, sellerID int64, domain collection.CollectionDomain, key string) (collection.Cursor, error) {
	cursor, ok := h.cursors[key]
	if !ok || cursor.SellerAccountID != sellerID || cursor.Domain != domain {
		return collection.Cursor{}, pgx.ErrNoRows
	}
	return cursor, nil
}
func (h *replayHarness) ListPayloads(_ context.Context, sellerID, domainRunID int64) ([]collection.SourcePayload, error) {
	var out []collection.SourcePayload
	for _, payload := range h.payloads {
		if payload.SellerAccountID == sellerID && payload.DomainRunID == domainRunID {
			out = append(out, payload)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func skuPage(offset, count int) []ozon.CanonicalAnalyticsRow {
	rows := make([]ozon.CanonicalAnalyticsRow, count)
	for i := range rows {
		rows[i] = row([]string{fmt.Sprint(offset + i + 1), "2026-08-01"}, fmt.Sprint(offset+i+1), "1")
	}
	return rows
}
func interruptedPrefix(t *testing.T, h *replayHarness, runID int64) {
	t.Helper()
	api := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 && req.Offset < 3000 {
			return response(skuPage(req.Offset, PageSize)), nil
		}
		return nil, ozon.ErrNetworkError
	}}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	_, err := (&Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, runID, d, d)
	if !errors.Is(err, ozon.ErrNetworkError) {
		t.Fatalf("interruption=%v", err)
	}
	if len(h.payloads) != 3 {
		t.Fatalf("prefix payloads=%d", len(h.payloads))
	}
}

func TestSameRunReplayResumeRestoresPrefixAndSkipsProviderPages(t *testing.T) {
	h := newReplayHarness()
	interruptedPrefix(t, h, 77)
	api := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 {
			return response(skuPage(req.Offset, 1)), nil
		}
		return response([]ozon.CanonicalAnalyticsRow{row([]string{"2026-08-01"}, "4504501", "3001")}), nil
	}}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	result, err := (&Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 77, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if got := []int{api.requests[0].Offset, api.requests[1].Offset}; !reflect.DeepEqual(got, []int{3000, 0}) {
		t.Fatalf("provider offsets=%v", got)
	}
	if result.SKURecords != 3001 || result.AccountRecords != 1 || result.Pages != 5 {
		t.Fatalf("resumed result=%+v", result)
	}
	if len(h.payloads) != 5 {
		t.Fatalf("payload metadata duplicated: %d", len(h.payloads))
	}
	api2 := &fakeAPI{fn: func(ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		return nil, errors.New("terminal replay called provider")
	}}
	again, err := (&Runner{API: api2, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 77, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(api2.requests) != 0 || again.SKURecords != result.SKURecords || len(h.payloads) != 5 {
		t.Fatalf("idempotent resume requests=%d result=%+v payloads=%d", len(api2.requests), again, len(h.payloads))
	}
}

func TestSameRunReplayResumeContinuesIndependentAccountStream(t *testing.T) {
	h := newReplayHarness()
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	first := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 {
			return response(nil), nil
		}
		if req.Offset < 3000 {
			rows := make([]ozon.CanonicalAnalyticsRow, PageSize)
			for i := range rows {
				rows[i] = row([]string{d.AddDate(0, 0, req.Offset+i).Format("2006-01-02")}, "1", "1")
			}
			return response(rows), nil
		}
		return nil, ozon.ErrNetworkError
	}}
	_, err := (&Runner{API: first, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 78, d, d.AddDate(0, 0, 3000))
	if !errors.Is(err, ozon.ErrNetworkError) {
		t.Fatalf("interruption=%v", err)
	}
	continuation := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		return response([]ozon.CanonicalAnalyticsRow{row([]string{d.AddDate(0, 0, req.Offset).Format("2006-01-02")}, "1", "1")}), nil
	}}
	result, err := (&Runner{API: continuation, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 78, d, d.AddDate(0, 0, 3000))
	if err != nil {
		t.Fatal(err)
	}
	if len(continuation.requests) != 1 || continuation.requests[0].Offset != 3000 || len(continuation.requests[0].Dimension) != 1 {
		t.Fatalf("provider requests=%+v", continuation.requests)
	}
	if result.AccountRecords != 3001 || result.Pages != 5 || len(h.payloads) != 5 {
		t.Fatalf("result=%+v payloads=%d", result, len(h.payloads))
	}
}

func TestResumedUniverseEqualsUninterruptedUniverse(t *testing.T) {
	h := newReplayHarness()
	interruptedPrefix(t, h, 88)
	continuation := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 {
			return response(skuPage(req.Offset, 1)), nil
		}
		return response([]ozon.CanonicalAnalyticsRow{row([]string{"2026-08-01"}, "4504501", "3001")}), nil
	}}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	resumed, err := (&Runner{API: continuation, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 88, d, d)
	if err != nil {
		t.Fatal(err)
	}
	freshHarness := newReplayHarness()
	uninterruptedAPI := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 {
			count := PageSize
			if req.Offset == 3000 {
				count = 1
			}
			return response(skuPage(req.Offset, count)), nil
		}
		return response([]ozon.CanonicalAnalyticsRow{row([]string{"2026-08-01"}, "4504501", "3001")}), nil
	}}
	fresh, err := (&Runner{API: uninterruptedAPI, Collection: freshHarness, Payloads: freshHarness.store, Cursors: freshHarness, Recovery: freshHarness}).Collect(context.Background(), 1, 89, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.sku) != len(fresh.sku) || len(resumed.account) != len(fresh.account) {
		t.Fatalf("sizes resumed=%d/%d fresh=%d/%d", len(resumed.sku), len(resumed.account), len(fresh.sku), len(fresh.account))
	}
	for i := range resumed.sku {
		a, b := resumed.sku[i], fresh.sku[i]
		if a.SourceSKU != b.SourceSKU || !a.Date.Equal(b.Date) || !a.Amount.Equal(b.Amount) || a.Units != b.Units {
			t.Fatalf("candidate %d differs", i)
		}
	}
}

func TestReplayIntegrityFailuresAreFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*replayHarness)
	}{
		{"missing-artifact", func(h *replayHarness) { delete(h.objects.bodies, h.payloads[1].Bucket+"/"+h.payloads[1].ObjectKey) }},
		{"corrupt-artifact", func(h *replayHarness) {
			h.objects.bodies[h.payloads[1].Bucket+"/"+h.payloads[1].ObjectKey] = []byte(`{"corrupt":true}`)
		}},
		{"wrong-contract", func(h *replayHarness) {
			var m map[string]any
			_ = json.Unmarshal(h.payloads[1].Metadata, &m)
			m["logical_contract_version"] = "wrong"
			h.payloads[1].Metadata, _ = json.Marshal(m)
		}},
		{"page-hole", func(h *replayHarness) { v := int64(4); h.payloads[1].PageSeq = &v }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newReplayHarness()
			interruptedPrefix(t, h, 100)
			tt.mutate(h)
			api := &fakeAPI{fn: func(ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
				return nil, errors.New("provider must not be called")
			}}
			d, _ := time.Parse("2006-01-02", "2026-08-01")
			_, err := (&Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 100, d, d)
			if !errors.Is(err, ErrResumeArtifactIntegrity) || len(api.requests) != 0 {
				t.Fatalf("err=%v requests=%d", err, len(api.requests))
			}
		})
	}
}

func TestNewAuthoritativeRunNeverConsumesOldRunCursor(t *testing.T) {
	h := newReplayHarness()
	interruptedPrefix(t, h, 200)
	api := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		return response(nil), nil
	}}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	_, err := (&Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(context.Background(), 1, 201, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 2 || api.requests[0].Offset != 0 || api.requests[1].Offset != 0 {
		t.Fatalf("new run requests=%+v", api.requests)
	}
}

func TestReplayHonorsContextCancellation(t *testing.T) {
	h := newReplayHarness()
	interruptedPrefix(t, h, 300)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := &fakeAPI{fn: func(ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		return nil, errors.New("provider called")
	}}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	_, err := (&Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}).Collect(ctx, 1, 300, d, d)
	if !errors.Is(err, context.Canceled) || len(api.requests) != 0 {
		t.Fatalf("err=%v requests=%d", err, len(api.requests))
	}
}
