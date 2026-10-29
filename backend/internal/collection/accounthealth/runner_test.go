package accounthealth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func decodeDocument(t *testing.T, raw string) ozon.AccountHealthDocument {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var document ozon.AccountHealthDocument
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestUnknownRatingKeyPersistsUnmappedAndGroupIsNotIdentity(t *testing.T) {
	first, _, _ := normalizeSummary(decodeDocument(t, `{"ratings":[{"rating_key":"new_provider_metric","group_name":"Delivery","value":95}]}`), 1)
	second, _, _ := normalizeSummary(decodeDocument(t, `{"ratings":[{"rating_key":"new_provider_metric","group_name":"Quality","value":96}]}`), 2)
	if len(first) != 1 || first[0].MappingStatus != "UNMAPPED" || first[0].CanonicalMetricKey != "UNMAPPED" {
		t.Fatalf("unknown key was not retained conservatively: %+v", first)
	}
	if first[0].ProviderRatingKey != second[0].ProviderRatingKey {
		t.Fatal("group movement changed metric identity")
	}
}

func TestSummaryGroupIsInheritedOnlyAsMetadata(t *testing.T) {
	metrics, _, _ := normalizeSummary(decodeDocument(t, `{"result":{"groups":[{"name":"Fulfillment","items":[{"rating_key":"r","value":1}]}]}}`), 1)
	if len(metrics) != 1 || metrics[0].ProviderRatingKey != "r" || metrics[0].GroupName != "Fulfillment" {
		t.Fatalf("grouped summary normalization failed: %+v", metrics)
	}
}

func TestPastValueRemainsProviderPreviousAndStatusIsProviderFact(t *testing.T) {
	metrics, _, _ := normalizeSummary(decodeDocument(t, `{"ratings":[{"rating_key":"r","value":95,"past_value":92,"status":"CRITICAL"}]}`), 1)
	if len(metrics) != 1 || metrics[0].Value != "95" || metrics[0].ProviderPreviousValue != "92" {
		t.Fatalf("summary values were remapped: %+v", metrics)
	}
	if metrics[0].ProviderStatus != "CRITICAL" {
		t.Fatal("provider judgement was not preserved")
	}
}

func TestHistoryProviderPeriodIsNotExploded(t *testing.T) {
	facts := normalizeHistory(decodeDocument(t, `{"ratings":[{"rating_key":"r","periods":[{"date_from":"2026-08-01","date_to":"2026-08-07","value":91,"status":"WARNING"}]}]}`), 1)
	if len(facts) != 1 || facts[0].PeriodFrom.Format("2006-01-02") != "2026-08-01" || facts[0].PeriodTo.Format("2006-01-02") != "2026-08-07" {
		t.Fatalf("provider period was not retained as one fact: %+v", facts)
	}
}

func TestUnresolvedPostingErrorDoesNotCreatePosting(t *testing.T) {
	event := normalizeFBSError(map[string]any{"id": json.Number("9007199254740993"), "posting_number": "FBS-1", "error_code": "SELLER_DELAY"}, 4)
	if event.CanonicalPostingID != nil || event.ResolutionStatus != "UNRESOLVED" {
		t.Fatalf("fake posting relation created: %+v", event)
	}
	if event.ProviderEventKey != "9007199254740993" {
		t.Fatalf("large provider id lost precision: %q", event.ProviderEventKey)
	}
}

func TestMetricDefinitionVersionChangesWithoutChangingIdentity(t *testing.T) {
	base := MetricObservation{ProviderRatingKey: "r", MappingStatus: "UNMAPPED", CanonicalMetricKey: "UNMAPPED", GroupName: "A"}
	changed := base
	changed.GroupName = "B"
	if metricHash(base) == metricHash(changed) {
		t.Fatal("group metadata change did not create a new metric version")
	}
	if base.ProviderRatingKey != changed.ProviderRatingKey {
		t.Fatal("definition version changed stable identity")
	}
}

func TestOptionalPointerHashesAreAllocationStable(t *testing.T) {
	id1, id2 := int64(42), int64(42)
	at1 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	true1, true2 := true, true

	metricA := MetricObservation{ProviderRatingKey: "r", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", HistorySupported: &true1}
	metricB := MetricObservation{ProviderRatingKey: "r", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", HistorySupported: &true2}
	if metricHash(metricA) != metricHash(metricB) {
		t.Fatal("HistorySupported pointer allocation changed metric hash")
	}

	penaltyA := PenaltyEvent{ProviderEventKey: "p1", Amount: "1", Currency: "RUB", OccurredAt: &at1}
	penaltyB := PenaltyEvent{ProviderEventKey: "p1", Amount: "1", Currency: "RUB", OccurredAt: &at2}
	if penaltyHash(penaltyA, &id1) != penaltyHash(penaltyB, &id2) {
		t.Fatal("separately allocated equal pointers changed penalty hash")
	}
	if penaltyHash(penaltyA, nil) == penaltyHash(penaltyA, &id1) {
		t.Fatal("nil metric id must not collide with concrete id")
	}

	errA := FBSErrorEvent{ProviderEventKey: "e1", SourcePostingNumber: "P", ResolutionStatus: "RESOLVED", CanonicalPostingID: &id1, OccurredAt: &at1}
	errB := FBSErrorEvent{ProviderEventKey: "e1", SourcePostingNumber: "P", ResolutionStatus: "RESOLVED", CanonicalPostingID: &id2, OccurredAt: &at2}
	if fbsErrorHash(errA) != fbsErrorHash(errB) {
		t.Fatal("separately allocated equal pointers changed FBS error hash")
	}
	if optionalInt64(nil) != "NULL" || optionalTime(nil) != "NULL" {
		t.Fatal("nil optional markers must be NULL")
	}
}
