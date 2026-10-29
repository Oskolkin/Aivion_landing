package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type scriptedAPI struct {
	requests     []ozon.ListProductsRequest
	infoRequests []ozon.ProductInfoListRequest
	list         func(ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error)
	info         func(ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error)
}

func (f *scriptedAPI) ListProducts(_ context.Context, _ int64, request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
	f.requests = append(f.requests, request)
	return f.list(request)
}

func (f *scriptedAPI) ListProductsInfo(_ context.Context, _ int64, request ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
	f.infoRequests = append(f.infoRequests, request)
	if f.info != nil {
		return f.info(request)
	}
	items := make([]ozon.ProductInfoListItem, 0, len(request.ProductID))
	for _, id := range request.ProductID {
		items = append(items, ozon.ProductInfoListItem{ID: id, SKU: id + 1000})
	}
	return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{Items: items}}, nil
}

type fakePayloadRegistrar struct {
	inputs []collection.RegisterPayloadInput
}

func (f *fakePayloadRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, input collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	f.inputs = append(f.inputs, input)
	return collection.SourcePayload{}, nil
}

func productListResponse(items []ozon.ProductItem, lastID string) *ozon.TypedResponse[ozon.ListProductsResult] {
	return &ozon.TypedResponse[ozon.ListProductsResult]{Raw: []byte(`{"result":{}}`), Data: ozon.ListProductsResult{Items: items, LastID: lastID}, Meta: ozon.ResponseMeta{StatusCode: 200}}
}

func TestRunnerBuildsCompleteAllAndArchivedUnion(t *testing.T) {
	skuByProduct := map[int64]int64{101: 1001, 102: 1002, 103: 1003, 104: 1004}
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		switch request.Filter.Visibility + ":" + request.LastID {
		case "ALL:":
			return productListResponse([]ozon.ProductItem{{ProductID: 101, SKU: 1001}, {ProductID: 102, SKU: 1002}}, "all-next"), nil
		case "ALL:all-next":
			return productListResponse([]ozon.ProductItem{{ProductID: 103, SKU: 1003}}, ""), nil
		case "ARCHIVED:":
			return productListResponse([]ozon.ProductItem{{ProductID: 104, SKU: 1004, Archived: true}}, ""), nil
		default:
			return nil, errors.New("unexpected request")
		}
	}
	api.info = func(request ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
		items := make([]ozon.ProductInfoListItem, 0, len(request.ProductID))
		for _, id := range request.ProductID {
			items = append(items, ozon.ProductInfoListItem{ID: id, SKU: skuByProduct[id], Archived: id == 104})
		}
		return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{Items: items}}, nil
	}

	result, err := (&Runner{API: api, TaxonomyReady: true}).RunCatalog(context.Background(), 7, 99)
	if err != nil {
		t.Fatal(err)
	}
	if result.AllPages != 2 || result.ArchivedPages != 1 || result.Pages != 3 || result.AllItems != 3 || result.ArchivedItems != 1 || result.UnionSKUs != 4 {
		t.Fatalf("unexpected counters: %+v", result)
	}
	if !reflect.DeepEqual(result.ObservedSKUs, []int64{1001, 1002, 1003, 1004}) {
		t.Fatalf("union=%v", result.ObservedSKUs)
	}
	if len(result.observations) != 4 || result.observations[3].State != "ARCHIVED" {
		t.Fatalf("observations=%+v", result.observations)
	}
	if len(api.infoRequests) != 1 || !reflect.DeepEqual(api.infoRequests[0].ProductID, []int64{101, 102, 103, 104}) {
		t.Fatalf("product info IDs=%+v", api.infoRequests)
	}
}

func TestRunnerStartsBothStreamsAndEveryNewRunAtEmptyCursor(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		return productListResponse(nil, ""), nil
	}
	runner := &Runner{API: api}
	if _, err := runner.RunCatalog(context.Background(), 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.RunCatalog(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	want := []string{"ALL:", "ARCHIVED:", "ALL:", "ARCHIVED:"}
	got := make([]string, 0, len(api.requests))
	for _, request := range api.requests {
		got = append(got, request.Filter.Visibility+":"+request.LastID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requests=%v", got)
	}
}

func TestRunnerIncompleteArchivedOrAllCannotProduceFinalizableUniverse(t *testing.T) {
	tests := []struct {
		name       string
		failStream string
	}{
		{name: "archived", failStream: "ARCHIVED"},
		{name: "all", failStream: "ALL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &scriptedAPI{}
			api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
				if request.Filter.Visibility == test.failStream && request.LastID == "next" {
					return nil, errors.New("page unavailable")
				}
				if request.Filter.Visibility == test.failStream {
					return productListResponse([]ozon.ProductItem{{ProductID: 1, SKU: 1001}}, "next"), nil
				}
				return productListResponse(nil, ""), nil
			}
			result, err := (&Runner{API: api}).RunCatalog(context.Background(), 1, 1)
			if err == nil || len(result.observations) != 0 || result.MarkedUnobserved != 0 || result.RecordsWritten != 0 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestRunnerDetailFailureCannotProduceFinalizableUniverse(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		if request.Filter.Visibility == "ALL" {
			return productListResponse([]ozon.ProductItem{{ProductID: 1, SKU: 1001}}, ""), nil
		}
		return productListResponse(nil, ""), nil
	}
	api.info = func(ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
		return nil, errors.New("detail unavailable")
	}
	result, err := (&Runner{API: api}).RunCatalog(context.Background(), 1, 1)
	if err == nil || len(result.observations) != 0 || result.MarkedUnobserved != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestRunnerCrossStreamDuplicateIsOnePresentUnknownOnConflict(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		return productListResponse([]ozon.ProductItem{{ProductID: 101, SKU: 1001}}, ""), nil
	}
	api.info = func(ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
		return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{Items: []ozon.ProductInfoListItem{{ID: 101, SKU: 1001}}}}, nil
	}
	result, err := (&Runner{API: api}).RunCatalog(context.Background(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.UnionSKUs != 1 || result.CrossStreamDuplicates != 1 || len(result.observations) != 1 || result.observations[0].State != "UNKNOWN" {
		t.Fatalf("result=%+v observations=%+v", result, result.observations)
	}
}

func TestRunnerRegistersDistinctListStreamProvenance(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		if request.Filter.Visibility == "ALL" && request.LastID == "" {
			return productListResponse([]ozon.ProductItem{{ProductID: 1, SKU: 1001}}, "all-next"), nil
		}
		if request.Filter.Visibility == "ARCHIVED" {
			return productListResponse([]ozon.ProductItem{{ProductID: 2, SKU: 1002, Archived: true}}, ""), nil
		}
		return productListResponse([]ozon.ProductItem{{ProductID: 3, SKU: 1003}}, ""), nil
	}
	api.info = func(request ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
		items := make([]ozon.ProductInfoListItem, 0, len(request.ProductID))
		for _, id := range request.ProductID {
			items = append(items, ozon.ProductInfoListItem{ID: id, SKU: id + 1000, Archived: id == 2})
		}
		return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{Items: items}}, nil
	}
	registrar := &fakePayloadRegistrar{}
	_, err := (&Runner{API: api, Collection: registrar, Payloads: &collection.SourcePayloadStore{}}).RunCatalog(context.Background(), 7, 99)
	if err != nil {
		t.Fatal(err)
	}
	var listInputs []collection.RegisterPayloadInput
	for _, input := range registrar.inputs {
		if input.EndpointKey == "ozon.product.list" {
			listInputs = append(listInputs, input)
		}
	}
	if len(listInputs) != 3 {
		t.Fatalf("list payload count=%d all payloads=%d", len(listInputs), len(registrar.inputs))
	}
	want := []string{"ALL", "ALL", "ARCHIVED"}
	for index, input := range listInputs {
		if input.EndpointKey != "ozon.product.list" || input.RequestIdentity["visibility"] != want[index] || input.Metadata["visibility_stream"] != want[index] {
			t.Fatalf("payload[%d]=%+v", index, input)
		}
	}
	allFingerprint, err := FingerprintForPage("ALL", "")
	if err != nil {
		t.Fatal(err)
	}
	archivedFingerprint, err := FingerprintForPage("ARCHIVED", "")
	if err != nil {
		t.Fatal(err)
	}
	if allFingerprint == archivedFingerprint {
		t.Fatal("ALL and ARCHIVED page 1 request fingerprints collided")
	}
}

func TestRunnerRejectsRepeatedCursorPerStream(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		return productListResponse([]ozon.ProductItem{{ProductID: 1, SKU: 1001}}, "same"), nil
	}
	if _, err := (&Runner{API: api}).RunCatalog(context.Background(), 1, 1); err == nil {
		t.Fatal("non-advancing cursor succeeded")
	}
}

func TestRunnerRejectsSameSKUWithDifferentProductIDs(t *testing.T) {
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		if request.Filter.Visibility == "ALL" {
			return productListResponse([]ozon.ProductItem{{ProductID: 101, SKU: 1001}, {ProductID: 102, SKU: 1001}}, ""), nil
		}
		return productListResponse(nil, ""), nil
	}
	result, err := (&Runner{API: api}).RunCatalog(context.Background(), 1, 1)
	var conflict *SKUIdentityConflictError
	if !errors.As(err, &conflict) || conflict.OzonSKU != 1001 || !reflect.DeepEqual(conflict.ProductIDs, []int64{101, 102}) {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if len(result.observations) != 0 || len(api.infoRequests) != 0 {
		t.Fatal("identity conflict produced finalizable observations")
	}
}
