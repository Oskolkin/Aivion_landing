package advertising

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

type memReports struct {
	mu   sync.Mutex
	byFP map[string]ReportRequestObservation
}

func reportKey(run int64, fingerprint string) string {
	return fmt.Sprintf("%d:%s", run, fingerprint)
}

func (m *memReports) LookupReportRequest(_ context.Context, _, run int64, fingerprint string) (ReportRequestObservation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.byFP[reportKey(run, fingerprint)]
	return x, ok, nil
}
func (m *memReports) UpsertReportRequest(_ context.Context, _, run int64, rr ReportRequestObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byFP == nil {
		m.byFP = map[string]ReportRequestObservation{}
	}
	m.byFP[reportKey(run, rr.RequestFingerprint)] = rr
	return nil
}

type cpoAPI struct {
	submits int
	uuid    string
}

func (a *cpoAPI) ListPerformanceCampaigns(context.Context, int64, performance.ListCampaignsRequest) (*performance.TypedResponse[performance.ListCampaignsResult], error) {
	return &performance.TypedResponse[performance.ListCampaignsResult]{Data: performance.ListCampaignsResult{}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) GetPerformanceDailyStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.CampaignStatisticsResult], error) {
	return &performance.TypedResponse[performance.CampaignStatisticsResult]{Data: performance.CampaignStatisticsResult{}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) GetPerformanceCampaignProducts(context.Context, int64, performance.CampaignProductsRequest) (*performance.TypedResponse[performance.CampaignProductsResult], error) {
	return &performance.TypedResponse[performance.CampaignProductsResult]{Data: performance.CampaignProductsResult{}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) GetPerformanceCampaignProductStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.CampaignProductStatisticsResult], error) {
	return &performance.TypedResponse[performance.CampaignProductStatisticsResult]{Data: performance.CampaignProductStatisticsResult{}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) GetPerformanceExpenseStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.ExpenseStatisticsResult], error) {
	return &performance.TypedResponse[performance.ExpenseStatisticsResult]{Data: performance.ExpenseStatisticsResult{}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) SubmitPerformanceCPOOrdersReport(context.Context, int64, time.Time, time.Time) (*performance.TypedResponse[performance.ReportSubmitResult], error) {
	a.submits++
	a.uuid = "report-uuid-1"
	return &performance.TypedResponse[performance.ReportSubmitResult]{Data: performance.ReportSubmitResult{UUID: a.uuid}, Raw: []byte(`{"UUID":"report-uuid-1"}`), Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) GetPerformanceStatisticsReportStatus(context.Context, int64, string) (*performance.TypedResponse[performance.ReportStatus], error) {
	return &performance.TypedResponse[performance.ReportStatus]{Data: performance.ReportStatus{UUID: a.uuid, State: "READY"}, Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}
func (a *cpoAPI) DownloadPerformanceStatisticsReport(context.Context, int64, string) (*performance.RawResponse, error) {
	return &performance.RawResponse{Body: []byte(`[]`), Meta: performance.ResponseMeta{StatusCode: 200}}, nil
}

type cpoRegistrar struct{ next int64 }

func (r *cpoRegistrar) StoreAndRegisterPayload(_ context.Context, _ *collection.SourcePayloadStore, _ collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	r.next++
	return collection.SourcePayload{ID: r.next, ReceivedAt: time.Unix(r.next, 0).UTC()}, nil
}

func TestCPOSameRunResumeDoesNotResubmit(t *testing.T) {
	api := &cpoAPI{}
	reports := &memReports{}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	runner := &Runner{API: api, Collection: &cpoRegistrar{}, Payloads: &collection.SourcePayloadStore{}, Reports: reports, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}}
	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	out := &Result{RequestedFrom: day, RequestedTo: day, StartedAt: now}
	if err := runner.collectCPO(context.Background(), 1, 99, out); err != nil {
		t.Fatal(err)
	}
	if api.submits != 1 {
		t.Fatalf("first submit count=%d", api.submits)
	}
	out2 := &Result{RequestedFrom: day, RequestedTo: day, StartedAt: now}
	if err := runner.collectCPO(context.Background(), 1, 99, out2); err != nil {
		t.Fatal(err)
	}
	if api.submits != 1 {
		t.Fatalf("same-run resume submitted again: %d", api.submits)
	}
	out3 := &Result{RequestedFrom: day, RequestedTo: day, StartedAt: now}
	if err := runner.collectCPO(context.Background(), 1, 100, out3); err != nil {
		t.Fatal(err)
	}
	if api.submits != 2 {
		t.Fatalf("new domain run should submit again: %d", api.submits)
	}
}
