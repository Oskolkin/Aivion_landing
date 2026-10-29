package productanalyticssync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type mockAnalyticsClient struct {
	resp *ozon.AnalyticsDataResponse
	err  error
}

func (m *mockAnalyticsClient) FetchAnalyticsData(_ context.Context, _, _ string, _ ozon.AnalyticsDataRequest) (*ozon.AnalyticsDataResponse, error) {
	return m.resp, m.err
}

func TestOutcomeFromProviderError_accessDenied(t *testing.T) {
	out, ok := outcomeFromProviderError(ozon.ErrInvalidCredentials)
	if !ok || out.SourceStatus != SourceStatusSourceAccessDenied {
		t.Fatalf("got %+v ok=%v", out, ok)
	}
}

func TestOutcomeFromProviderError_premiumRequired(t *testing.T) {
	out, ok := outcomeFromProviderError(&ozon.ProviderError{StatusCode: 402, Body: "premium"})
	if !ok || out.SourceStatus != SourceStatusPremiumRequired {
		t.Fatalf("got %+v", out)
	}
}

func TestSyncAccountFunnelForDate_mapsAvailableMetrics(t *testing.T) {
	impressions := int64(1000)
	views := int64(400)
	cart := int64(80)
	orders := int64(20)
	delivered := int64(18)
	rev := 500000.0
	client := &mockAnalyticsClient{
		resp: &ozon.AnalyticsDataResponse{
			Totals: []float64{float64(impressions), float64(views), float64(cart), float64(orders), float64(delivered), rev},
		},
	}
	svc := &Service{client: client}
	out, err := svc.syncAccountFunnelForDate(context.Background(), "cid", "key", time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if out.SourceStatus != SourceStatusAvailable {
		t.Fatalf("status=%q", out.SourceStatus)
	}
	if out.Impressions == nil || *out.Impressions != impressions {
		t.Fatalf("impressions=%v", out.Impressions)
	}
	if out.Completed == nil || *out.Completed != delivered {
		t.Fatalf("completed=%v", out.Completed)
	}
}

func TestSyncAccountFunnelForDate_notEnoughData(t *testing.T) {
	client := &mockAnalyticsClient{resp: &ozon.AnalyticsDataResponse{Totals: []float64{}}}
	svc := &Service{client: client}
	out, err := svc.syncAccountFunnelForDate(context.Background(), "cid", "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if out.SourceStatus != SourceStatusNotEnoughData {
		t.Fatalf("status=%q", out.SourceStatus)
	}
}

func TestSyncAccountFunnelForDate_doesNotFailOn403(t *testing.T) {
	client := &mockAnalyticsClient{err: ozon.ErrInvalidCredentials}
	svc := &Service{client: client}
	out, err := svc.syncAccountFunnelForDate(context.Background(), "cid", "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if out.SourceStatus != SourceStatusSourceAccessDenied {
		t.Fatalf("status=%q", out.SourceStatus)
	}
}

func TestSyncAccountFunnelForDate_doesNotFailOn402(t *testing.T) {
	client := &mockAnalyticsClient{err: &ozon.ProviderError{StatusCode: 402}}
	svc := &Service{client: client}
	out, err := svc.syncAccountFunnelForDate(context.Background(), "cid", "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if out.SourceStatus != SourceStatusPremiumRequired {
		t.Fatalf("status=%q", out.SourceStatus)
	}
}

func TestSyncAccountFunnelForDate_mapsNetworkErrorToSourceError(t *testing.T) {
	client := &mockAnalyticsClient{err: errors.New("network down")}
	svc := &Service{client: client}
	out, err := svc.syncAccountFunnelForDate(context.Background(), "cid", "key", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.SourceStatus != SourceStatusError {
		t.Fatalf("status=%q", out.SourceStatus)
	}
	if out.SourceMessage == "" {
		t.Fatal("expected source message")
	}
}

func TestSyncFunnelForSellerRange_callsEveryDateInclusive(t *testing.T) {
	var days []string
	err := syncFunnelDateRange(context.Background(), time.Date(2026, 5, 10, 15, 0, 0, 0, time.UTC), time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC), func(day time.Time) error {
		days = append(days, day.Format("2006-01-02"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(days, ",")
	want := "2026-05-10,2026-05-11,2026-05-12"
	if got != want {
		t.Fatalf("days=%s want=%s", got, want)
	}
}

func TestSyncFunnelForSellerRange_stopsOnPersistErrors(t *testing.T) {
	calls := 0
	err := syncFunnelDateRange(context.Background(), time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), func(day time.Time) error {
		calls++
		if day.Format("2006-01-02") == "2026-05-11" {
			return errors.New("db down")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Fatalf("calls=%d want 2", calls)
	}
}

func TestHasAnyFunnelMetric(t *testing.T) {
	if hasAnyFunnelMetric(nil, nil, nil, nil, nil) {
		t.Fatal("expected false")
	}
	v := int64(1)
	if !hasAnyFunnelMetric(nil, &v, nil, nil, nil) {
		t.Fatal("expected true")
	}
}
