package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type fakeAPI struct {
	requests []ozon.AnalyticsDataRequest
	fn       func(ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error)
}

func (f *fakeAPI) FetchCanonicalAnalyticsData(_ context.Context, _ int64, r ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
	f.requests = append(f.requests, r)
	return f.fn(r)
}

type fakeRegistrar struct {
	inputs []collection.RegisterPayloadInput
}
type fakeCursors struct {
	keys   []string
	values [][]byte
}

func (f *fakeCursors) UpsertCursor(_ context.Context, _ int64, _ collection.CollectionDomain, key string, value []byte) (collection.Cursor, error) {
	f.keys = append(f.keys, key)
	f.values = append(f.values, append([]byte(nil), value...))
	return collection.Cursor{}, nil
}

func (f *fakeRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, in collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	f.inputs = append(f.inputs, in)
	return collection.SourcePayload{ID: int64(len(f.inputs)), ReceivedAt: time.Now()}, nil
}
func response(rows []ozon.CanonicalAnalyticsRow) *ozon.CanonicalAnalyticsResponse {
	raw, _ := json.Marshal(map[string]any{"result": map[string]any{"data": rows}})
	return &ozon.CanonicalAnalyticsResponse{Data: rows, Raw: raw, Meta: ozon.ResponseMeta{StatusCode: 200}}
}
func row(dims []string, amount, units string) ozon.CanonicalAnalyticsRow {
	d := make([]ozon.AnalyticsDataDimension, len(dims))
	for i, v := range dims {
		d[i] = ozon.AnalyticsDataDimension{ID: v}
	}
	return ozon.CanonicalAnalyticsRow{Dimensions: d, Metrics: []json.Number{json.Number(amount), json.Number(units)}}
}

func TestCollectUsesExactIndependentContractsAndPersistsEveryPayload(t *testing.T) {
	api := &fakeAPI{fn: func(r ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if reflect.DeepEqual(r.Dimension, []string{"sku", "day"}) {
			return response([]ozon.CanonicalAnalyticsRow{row([]string{"123", "2026-08-01"}, "10.25", "2.0")}), nil
		}
		return response([]ozon.CanonicalAnalyticsRow{row([]string{"2026-08-01"}, "10.25", "2")}), nil
	}}
	reg := &fakeRegistrar{}
	cursors := &fakeCursors{}
	from, _ := time.Parse("2006-01-02", "2026-08-01")
	result, err := (&Runner{API: api, Collection: reg, Cursors: cursors, Now: func() time.Time { return time.Unix(1, 0) }}).Collect(context.Background(), 7, 9, from, from)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 2 || len(reg.inputs) != 2 || result.SKURecords != 1 || result.AccountRecords != 1 {
		t.Fatalf("requests=%d payloads=%d result=%+v", len(api.requests), len(reg.inputs), result)
	}
	for _, r := range api.requests {
		if !reflect.DeepEqual(r.Metrics, []string{"revenue", "ordered_units"}) || r.Limit != 1000 || r.Offset != 0 || r.DateFrom != "2026-08-01" || r.DateTo != "2026-08-01" {
			t.Fatalf("bad request: %+v", r)
		}
	}
	if reg.inputs[0].EndpointKey == reg.inputs[1].EndpointKey {
		t.Fatal("logical sources collapsed")
	}
	if len(cursors.keys) != 2 || cursors.keys[0] != "run:9:sku_daily" || cursors.keys[1] != "run:9:account_daily" {
		t.Fatalf("cursor keys=%v", cursors.keys)
	}
}

func TestCollectRequestsExactBoundaryNextPageAndNewRunRestartsAtZero(t *testing.T) {
	api := &fakeAPI{fn: func(r ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if reflect.DeepEqual(r.Dimension, []string{"sku", "day"}) && r.Offset == 0 {
			rows := make([]ozon.CanonicalAnalyticsRow, PageSize)
			for i := range rows {
				rows[i] = row([]string{fmt.Sprint(i + 1), "2026-08-01"}, "0", "0")
			}
			return response(rows), nil
		}
		return response(nil), nil
	}}
	reg := &fakeRegistrar{}
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	runner := &Runner{API: api, Collection: reg}
	if _, err := runner.Collect(context.Background(), 1, 1, d, d); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Collect(context.Background(), 1, 2, d, d); err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1000, 0, 0, 1000, 0}
	got := make([]int, len(api.requests))
	for i, r := range api.requests {
		got[i] = r.Offset
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("offsets=%v want=%v", got, want)
	}
}

func TestCollectRejectsContractMismatchDuplicateFractionalAndNegative(t *testing.T) {
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	tests := []struct {
		name string
		rows []ozon.CanonicalAnalyticsRow
	}{
		{"dimension-count", []ozon.CanonicalAnalyticsRow{row([]string{"123"}, "1", "1")}},
		{"duplicate", []ozon.CanonicalAnalyticsRow{row([]string{"123", "2026-08-01"}, "1", "1"), row([]string{"123", "2026-08-01"}, "1", "1")}},
		{"fractional-units", []ozon.CanonicalAnalyticsRow{row([]string{"123", "2026-08-01"}, "1", "1.5")}},
		{"negative", []ozon.CanonicalAnalyticsRow{row([]string{"123", "2026-08-01"}, "-1", "1")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{fn: func(r ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
				if len(r.Dimension) == 2 {
					return response(tt.rows), nil
				}
				return response(nil), nil
			}}
			_, err := (&Runner{API: api, Collection: &fakeRegistrar{}}).Collect(context.Background(), 1, 1, d, d)
			if !errors.Is(err, ErrSourceContractMismatch) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestDecimalExactnessAndInitialMoscowRange(t *testing.T) {
	a, err := ParseAmount(json.Number("9007199254740993.123456"))
	if err != nil || a.String() != "9007199254740993.123456" {
		t.Fatalf("amount=%s err=%v", a.String(), err)
	}
	if u, err := ParseUnits(json.Number("10.0")); err != nil || u != 10 {
		t.Fatalf("units=%d err=%v", u, err)
	}
	if _, err := ParseUnits(json.Number("10.01")); err == nil {
		t.Fatal("fractional units accepted")
	}
	from, to := Initial90DayRange(time.Date(2026, 8, 24, 0, 30, 0, 0, time.UTC))
	if to.Format("2006-01-02") != "2026-08-23" || daysInclusive(from, to) != 90 {
		t.Fatalf("range=%s..%s", from, to)
	}
}

func TestReconciliationStatusPrecedence(t *testing.T) {
	if measureStatus(true, false) != "PASS" || measureStatus(true, true) != "EXPLAINED_GAP" || measureStatus(false, true) != "UNEXPLAINED_GAP" {
		t.Fatal("measure status semantics changed")
	}
	if overallStatus("NOT_COMPARABLE", "UNEXPLAINED_GAP") != "UNEXPLAINED_GAP" || overallStatus("PASS", "EXPLAINED_GAP") != "EXPLAINED_GAP" {
		t.Fatal("precedence changed")
	}
}

func TestAutomatedInitial90DayExplicitZeroFixture(t *testing.T) {
	from, to := Initial90DayRange(time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	api := &fakeAPI{fn: func(r ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(r.Dimension) == 2 {
			return response(nil), nil
		}
		rows := make([]ozon.CanonicalAnalyticsRow, 0, 90)
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			rows = append(rows, row([]string{d.Format("2006-01-02")}, "0", "0"))
		}
		return response(rows), nil
	}}
	result, err := (&Runner{API: api, Collection: &fakeRegistrar{}}).Collect(context.Background(), 1, 1, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccountRecords != 90 || result.SKURecords != 0 || result.Currency != "RUB" {
		t.Fatalf("90d fixture=%+v", result)
	}
}

func TestCurrencyPolicyIsProviderMarketScoped(t *testing.T) {
	if c, err := ResolveCurrency("OZON", "RU"); err != nil || c != "RUB" {
		t.Fatalf("currency=%q err=%v", c, err)
	}
	if _, err := ResolveCurrency("OZON", "UNKNOWN"); err == nil {
		t.Fatal("unknown market received currency")
	}
}

func TestAmountAndUnitsStatusesRemainIndependent(t *testing.T) {
	amount := measureStatus(true, false)
	units := measureStatus(false, false)
	if amount != "PASS" || units != "UNEXPLAINED_GAP" || overallStatus(amount, units) != "UNEXPLAINED_GAP" {
		t.Fatalf("amount=%s units=%s overall=%s", amount, units, overallStatus(amount, units))
	}
}
