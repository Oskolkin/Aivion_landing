package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type fakeAPI struct {
	marketErr       error
	queryCalls      []ozon.ProductQueriesRequest
	detailCalls     []ozon.ProductQueryDetailsRequest
	queryPages      map[string][]*ozon.TypedResponse[ozon.SearchQueryResult]
	detailPages     []*ozon.TypedResponse[ozon.SearchQueryResult]
	detailPageIndex int
}

func response(rows []ozon.SearchQueryRow, hasNext bool) *ozon.TypedResponse[ozon.SearchQueryResult] {
	return &ozon.TypedResponse[ozon.SearchQueryResult]{Data: ozon.SearchQueryResult{Items: rows, HasNext: hasNext}, Raw: []byte(`{"items":[]}`), Meta: ozon.ResponseMeta{StatusCode: 200}}
}

func (f *fakeAPI) ListProductQueries(_ context.Context, _ int64, r ozon.ProductQueriesRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error) {
	f.queryCalls = append(f.queryCalls, r)
	if len(r.SKUs) == 0 {
		return nil, errors.New("skus required")
	}
	key := r.DateFrom + "|" + r.DateTo
	pages := f.queryPages[key]
	if len(pages) == 0 {
		return response([]ozon.SearchQueryRow{{SKU: ozon.FlexibleInt64(r.SKUs[0]), ProviderSearchGMV: "100", Orders: "2", UniqueSearchUsers: "7"}}, false), nil
	}
	idx := r.Page - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(pages) {
		return response(nil, false), nil
	}
	return pages[idx], nil
}

func (f *fakeAPI) ListProductQueryDetails(_ context.Context, _ int64, r ozon.ProductQueryDetailsRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error) {
	f.detailCalls = append(f.detailCalls, r)
	if r.LimitBySKU <= 0 || r.LimitBySKU > 15 {
		return nil, errors.New("limit_by_sku invalid")
	}
	if len(f.detailPages) == 0 {
		return response([]ozon.SearchQueryRow{{Query: "телефон чехол", SKU: 1, ProviderSearchGMV: "60", Rank: 1}}, false), nil
	}
	if f.detailPageIndex >= len(f.detailPages) {
		return response(nil, false), nil
	}
	out := f.detailPages[f.detailPageIndex]
	f.detailPageIndex++
	return out, nil
}

func (f *fakeAPI) ListTopSearchQueries(context.Context, int64, ozon.MarketSearchRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error) {
	if f.marketErr != nil {
		return nil, f.marketErr
	}
	return response(nil, false), nil
}

type registrar struct{ next int64 }

func (r *registrar) StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	r.next++
	return collection.SourcePayload{ID: r.next, ReceivedAt: time.Now().UTC()}, nil
}

type fixedUniverse []FrozenSKU

func (u fixedUniverse) ResolveSearchUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error) {
	return append([]FrozenSKU(nil), u...), nil
}

func TestNormalizeQueryKeyIsConservative(t *testing.T) {
	if got := NormalizeQueryKey("  ТЕЛЕФОН \t Чехол  "); got != "телефон чехол" {
		t.Fatalf("key=%q", got)
	}
	if got := NormalizeQueryKey("машины"); got != "машины" {
		t.Fatalf("stemming unexpectedly changed key: %q", got)
	}
}

func TestProductQueriesRequiresSKUsAndPaginates(t *testing.T) {
	api := &fakeAPI{queryPages: map[string][]*ozon.TypedResponse[ozon.SearchQueryResult]{
		"2026-08-19|2026-08-25": {
			response([]ozon.SearchQueryRow{{SKU: 1, ProviderSearchGMV: "10", Orders: "1"}}, true),
			response([]ozon.SearchQueryRow{{SKU: 1, ProviderSearchGMV: "20", Orders: "2"}}, false),
		},
		"2026-08-12|2026-08-18": {
			response([]ozon.SearchQueryRow{{SKU: 1, ProviderSearchGMV: "5", Orders: "1"}}, false),
		},
	}}
	result, err := (&Runner{
		API: api, Universe: fixedUniverse{{CanonicalSKUID: 9, OzonSKU: 1}},
		Collection: &registrar{}, Payloads: &collection.SourcePayloadStore{},
	}).Collect(context.Background(), 1, 2, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.SellerSummaryStatus != "COMPLETE" {
		t.Fatalf("status=%s", result.SellerSummaryStatus)
	}
	currentPages := 0
	for _, c := range api.queryCalls {
		if c.DateFrom == "2026-08-19" {
			currentPages++
			if len(c.SKUs) == 0 {
				t.Fatal("skus missing")
			}
		}
	}
	if currentPages != 2 {
		t.Fatalf("expected 2 current pages, got %d calls=%+v", currentPages, api.queryCalls)
	}
	if len(result.SKUSummaries) < 3 {
		t.Fatalf("summaries=%+v", result.SKUSummaries)
	}
}

func TestDetailsLimitBySKUIsPerSKUNotSKUCount(t *testing.T) {
	api := &fakeAPI{}
	universe := make(fixedUniverse, 20)
	for i := range universe {
		universe[i] = FrozenSKU{CanonicalSKUID: int64(i + 1), OzonSKU: int64(1000 + i)}
	}
	_, err := (&Runner{
		API: api, Universe: universe, Collection: &registrar{}, Payloads: &collection.SourcePayloadStore{},
		DetailSKUSelectionLimit: 20, LimitBySKU: 15,
	}).Collect(context.Background(), 1, 2, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(api.detailCalls) == 0 {
		t.Fatal("expected details call")
	}
	call := api.detailCalls[0]
	if call.LimitBySKU != 15 {
		t.Fatalf("limit_by_sku=%d", call.LimitBySKU)
	}
	if len(call.SKUs) != 20 {
		t.Fatalf("requested SKUs=%d (selection limit), limit_by_sku must stay independent", len(call.SKUs))
	}
}

func TestMarketFailureDoesNotFailSellerSearch(t *testing.T) {
	api := &fakeAPI{marketErr: errors.New("premium unavailable")}
	result, err := (&Runner{
		API: api, Universe: fixedUniverse{{CanonicalSKUID: 1, OzonSKU: 1}},
		Collection: &registrar{}, Payloads: &collection.SourcePayloadStore{}, EnableMarketSearch: true,
	}).Collect(context.Background(), 1, 2, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.SellerSummaryStatus != "COMPLETE" || result.MarketSearchStatus != "UNAVAILABLE" {
		t.Fatalf("statuses=%+v", result)
	}
	if len(result.Details) != 1 || result.Details[0].RetrievalReason != RetrievalReasonTopN || result.Details[0].LimitBySKU != 15 {
		t.Fatalf("details=%+v", result.Details)
	}
}

func TestSKUSummaryHashIgnoresPointerIdentity(t *testing.T) {
	a := int64(42)
	b := int64(42)
	dayFrom := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	dayTo := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	h1 := skuSummaryHash(SKUSummary{CanonicalSKUID: &a, SourceSKU: 7, SKUResolutionStatus: "RESOLVED", PeriodStart: dayFrom, PeriodEnd: dayTo, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "100", Orders: "2", UniqueSearchUsers: "7", UniqueUsersAggregation: "NON_ADDITIVE"})
	h2 := skuSummaryHash(SKUSummary{CanonicalSKUID: &b, SourceSKU: 7, SKUResolutionStatus: "RESOLVED", PeriodStart: dayFrom, PeriodEnd: dayTo, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "100", Orders: "2", UniqueSearchUsers: "7", UniqueUsersAggregation: "NON_ADDITIVE"})
	if h1 != h2 {
		t.Fatalf("hash changed without semantic change: %s vs %s", h1, h2)
	}
	p1, p2 := &a, &b
	if optionalInt64(p1) != optionalInt64(p2) || optionalInt64(p1) != "42" {
		t.Fatalf("pointer deref broken: %q %q", optionalInt64(p1), optionalInt64(p2))
	}
	if optionalInt64(nil) != "NULL" {
		t.Fatal("nil pointer marker")
	}
}

func TestSummaryAndDetailHashSupportABACycle(t *testing.T) {
	dayFrom := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	dayTo := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	a := skuSummaryHash(SKUSummary{SourceSKU: 1, SKUResolutionStatus: "UNRESOLVED", PeriodStart: dayFrom, PeriodEnd: dayTo, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "100", Orders: "1", UniqueUsersAggregation: "NON_ADDITIVE"})
	b := skuSummaryHash(SKUSummary{SourceSKU: 1, SKUResolutionStatus: "UNRESOLVED", PeriodStart: dayFrom, PeriodEnd: dayTo, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "200", Orders: "2", UniqueUsersAggregation: "NON_ADDITIVE"})
	a2 := skuSummaryHash(SKUSummary{SourceSKU: 1, SKUResolutionStatus: "UNRESOLVED", PeriodStart: dayFrom, PeriodEnd: dayTo, PeriodKind: "CURRENT_7D", ProviderSearchGMV: "100", Orders: "1", UniqueUsersAggregation: "NON_ADDITIVE"})
	if a == b || a != a2 {
		t.Fatalf("summary A→B→A hashes broken: %s %s %s", a, b, a2)
	}
	dA := detailHash(Detail{SourceSKU: 1, ProviderRank: 1, PeriodStart: dayFrom, PeriodEnd: dayTo, ProviderSearchGMV: "10", Orders: "1", RetrievalScope: RetrievalReasonTopN, RetrievalReason: RetrievalReasonTopN, TopNLimit: 15, LimitBySKU: 15}, 9)
	dB := detailHash(Detail{SourceSKU: 1, ProviderRank: 1, PeriodStart: dayFrom, PeriodEnd: dayTo, ProviderSearchGMV: "20", Orders: "2", RetrievalScope: RetrievalReasonTopN, RetrievalReason: RetrievalReasonTopN, TopNLimit: 15, LimitBySKU: 15}, 9)
	dA2 := detailHash(Detail{SourceSKU: 1, ProviderRank: 1, PeriodStart: dayFrom, PeriodEnd: dayTo, ProviderSearchGMV: "10", Orders: "1", RetrievalScope: RetrievalReasonTopN, RetrievalReason: RetrievalReasonTopN, TopNLimit: 15, LimitBySKU: 15}, 9)
	if dA == dB || dA != dA2 {
		t.Fatalf("detail A→B→A hashes broken: %s %s %s", dA, dB, dA2)
	}
}
