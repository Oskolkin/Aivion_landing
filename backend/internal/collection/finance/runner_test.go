package finance

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type financeAPI struct {
	rows []ozon.FinanceAccrualRow
}

func (a financeAPI) ListFinanceAccrualTypes(context.Context, int64) (*ozon.TypedResponse[ozon.FinanceAccrualTypesResult], error) {
	return &ozon.TypedResponse[ozon.FinanceAccrualTypesResult]{
		Data: ozon.FinanceAccrualTypesResult{Items: []ozon.FinanceAccrualType{{ID: "known", Name: "Known"}}},
		Raw:  []byte(`{"items":[{"id":"known","name":"Known"}]}`),
		Meta: ozon.ResponseMeta{StatusCode: 200},
	}, nil
}

func (a financeAPI) ListFinanceAccrualsByDay(context.Context, int64, ozon.FinanceAccrualRequest) (*ozon.TypedResponse[ozon.FinanceAccrualResult], error) {
	return &ozon.TypedResponse[ozon.FinanceAccrualResult]{
		Data: ozon.FinanceAccrualResult{Items: a.rows},
		Raw:  []byte(`{"items":[]}`),
		Meta: ozon.ResponseMeta{StatusCode: 200},
	}, nil
}

type financeRegistrar struct{ next int64 }

func (r *financeRegistrar) StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	r.next++
	return collection.SourcePayload{ID: r.next}, nil
}

func collectFinance(t *testing.T, rows []ozon.FinanceAccrualRow) Result {
	t.Helper()
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	result, err := (&Runner{
		API: financeAPI{rows: rows}, Collection: &financeRegistrar{},
		Payloads: &collection.SourcePayloadStore{}, Now: func() time.Time { return day.Add(12 * time.Hour) },
	}).Collect(context.Background(), 1, 2, day, day)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAccountNetNeverDoubleCountsChildren(t *testing.T) {
	records := []Record{
		{TotalAmountSigned: "100", Lines: []Line{{SignedAmount: "60"}, {SignedAmount: "40"}}},
		{TotalAmountSigned: "-25", Lines: []Line{{SignedAmount: "-25"}}},
	}
	net, err := AccountNet(records)
	if err != nil {
		t.Fatal(err)
	}
	if net != "75" {
		t.Fatalf("parent net=%s; child lines were likely double-counted", net)
	}
}

func TestUnknownTypeRetainsAmountAndIsUnmapped(t *testing.T) {
	result := collectFinance(t, []ozon.FinanceAccrualRow{{
		ProviderTypeID: "new-provider-type", Amount: ozon.ExactNumber("-12.34"), CurrencyCode: "RUB",
		Components: []ozon.FinanceAccrualComponent{{Path: "fees.delivery", Amount: ozon.ExactNumber("-12.34")}},
	}})
	record := result.Days[0].Records[0]
	if record.ProviderTypeID != "new-provider-type" || record.TotalAmountSigned != "-12.34" || record.Lines[0].MappingStatus != "UNMAPPED" {
		t.Fatalf("unknown type was lost or guessed: %+v", record)
	}
}

func TestIdenticalRowsPreserveMultiplicityByOrdinal(t *testing.T) {
	row := ozon.FinanceAccrualRow{ProviderTypeID: "known", Amount: ozon.ExactNumber("10"), CurrencyCode: "RUB"}
	result := collectFinance(t, []ozon.FinanceAccrualRow{row, row})
	records := result.Days[0].Records
	if len(records) != 2 || records[0].RowOrdinal != 1 || records[1].RowOrdinal != 2 {
		t.Fatalf("identical rows were deduplicated: %+v", records)
	}
	single := DayRevision{BusinessDate: result.Days[0].BusinessDate, CompletenessStatus: "COMPLETE", Records: records[:1]}
	dup := DayRevision{BusinessDate: result.Days[0].BusinessDate, CompletenessStatus: "COMPLETE", Records: records}
	if dayHash(single) == dayHash(dup) {
		t.Fatal("dayHash must preserve multiplicity of identical provider rows")
	}
}

func TestDayHashUsesDereferencedOptionalPointers(t *testing.T) {
	sku1, sku2 := int64(9), int64(9)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	a := DayRevision{BusinessDate: day, CompletenessStatus: "COMPLETE", Records: []Record{{
		RowOrdinal: 1, ProviderTypeID: "op", TotalAmountSigned: "1", CurrencyCode: "RUB",
		Lines: []Line{{Ordinal: 1, SourceComponentPath: "x", SignedAmount: "1", MappingStatus: "UNMAPPED", CanonicalSKUID: &sku1}},
	}}}
	b := DayRevision{BusinessDate: day, CompletenessStatus: "COMPLETE", Records: []Record{{
		RowOrdinal: 1, ProviderTypeID: "op", TotalAmountSigned: "1", CurrencyCode: "RUB",
		Lines: []Line{{Ordinal: 1, SourceComponentPath: "x", SignedAmount: "1", MappingStatus: "UNMAPPED", CanonicalSKUID: &sku2}},
	}}}
	if dayHash(a) != dayHash(b) {
		t.Fatal("separately allocated equal optional pointers must produce the same dayHash")
	}
}

func TestEmptyCompleteDayIsKnownZero(t *testing.T) {
	result := collectFinance(t, nil)
	if result.CompletenessStatus != "COMPLETE" || len(result.Days) != 1 || result.Days[0].CompletenessStatus != "COMPLETE" || len(result.Days[0].Records) != 0 {
		t.Fatalf("empty complete day was not materialized: %+v", result)
	}
	net, err := AccountNet(result.Days[0].Records)
	if err != nil || net != "0" {
		t.Fatalf("empty day net=%q err=%v", net, err)
	}
}
