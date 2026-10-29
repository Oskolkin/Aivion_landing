package actions

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type fakeCandidateRepo struct {
	mu sync.Mutex

	accountSnapshot AccountSnapshotRow
	skuBundle       *SKUMetricsBundle
	adCampaigns     []AdCampaignMetricRow
	adCampaignCount int64
	funnelData      *FunnelDataBundle

	entityGroups map[string]*ActionEntityGroupRecord

	accountSnapshotErr error
	skuBundleErr       error
	adListErr          error
	adCountErr         error
	funnelErr          error
	entityGroupErr     error

	actionItemsCreated int
}

func newFakeCandidateRepo() *fakeCandidateRepo {
	return &fakeCandidateRepo{
		entityGroups: make(map[string]*ActionEntityGroupRecord),
	}
}

func (f *fakeCandidateRepo) GetAccountSnapshot(ctx context.Context, sellerAccountID int64, metricDate time.Time) (AccountSnapshotRow, error) {
	if f.accountSnapshotErr != nil {
		return AccountSnapshotRow{}, f.accountSnapshotErr
	}
	return f.accountSnapshot, nil
}

func (f *fakeCandidateRepo) LoadSKUMetricsBundle(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*SKUMetricsBundle, error) {
	if f.skuBundleErr != nil {
		return nil, f.skuBundleErr
	}
	return f.skuBundle, nil
}

func (f *fakeCandidateRepo) ListStockReplenishmentCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]StockReplenishmentRow, error) {
	bundle, err := f.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterStockReplenishmentRows(bundle, cfg), nil
}

func (f *fakeCandidateRepo) ListNoSalesWithStockCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]NoSalesWithStockRow, error) {
	bundle, err := f.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterNoSalesWithStockRows(bundle), nil
}

func (f *fakeCandidateRepo) ListSKUOrdersDropCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUOrdersDropRow, error) {
	bundle, err := f.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return filterSKUOrdersDropRows(bundle), nil
}

func (f *fakeCandidateRepo) ListKeySKURiskCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]KeySKURiskRow, error) {
	bundle, err := f.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	replenish := filterStockReplenishmentRows(bundle, cfg)
	replenishSet := make(map[int64]bool)
	for _, row := range replenish {
		replenishSet[row.OzonProductID] = true
	}
	drops := filterSKUOrdersDropRows(bundle)
	dropSet := make(map[int64]bool)
	for _, row := range drops {
		dropSet[row.OzonProductID] = true
	}
	return filterKeySKURiskRows(bundle, cfg, replenishSet, dropSet), nil
}

func (f *fakeCandidateRepo) ListAdCampaignMetrics7d(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]AdCampaignMetricRow, error) {
	if f.adListErr != nil {
		return nil, f.adListErr
	}
	return f.adCampaigns, nil
}

func (f *fakeCandidateRepo) CountAdCampaigns(ctx context.Context, sellerAccountID int64) (int64, error) {
	if f.adCountErr != nil {
		return 0, f.adCountErr
	}
	return f.adCampaignCount, nil
}

func (f *fakeCandidateRepo) GetFunnelData(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*FunnelDataBundle, error) {
	if f.funnelErr != nil {
		return nil, f.funnelErr
	}
	return f.funnelData, nil
}

func (f *fakeCandidateRepo) UpsertActionEntityGroup(ctx context.Context, sellerAccountID int64, metricDate time.Time, groupKey, entityType string, entityIDs []string, summary map[string]any) (*ActionEntityGroupRecord, error) {
	if f.entityGroupErr != nil {
		return nil, f.entityGroupErr
	}
	idsJSON, _ := json.Marshal(entityIDs)
	summaryJSON, _ := json.Marshal(summary)
	rec := &ActionEntityGroupRecord{
		ID:              int64(len(f.entityGroups) + 1),
		SellerAccountID: sellerAccountID,
		MetricDate:      &metricDate,
		GroupKey:        groupKey,
		EntityType:      entityType,
		EntityIDsJSON:   idsJSON,
		SummaryJSON:     summaryJSON,
	}
	f.mu.Lock()
	f.entityGroups[groupKey] = rec
	f.mu.Unlock()
	return rec, nil
}

func testMetricDate() time.Time {
	return time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
}

func ptrCoverage(v float64) *float64 {
	return &v
}

func makeSKURow(id int64, opts func(*SKUMetricRow)) SKUMetricRow {
	row := SKUMetricRow{OzonProductID: id, ProductName: "SKU " + skuEntityID(id)}
	if opts != nil {
		opts(&row)
	}
	return row
}

func buildTestBundle(rows []SKUMetricRow) *SKUMetricsBundle {
	var storeRev float64
	for i := range rows {
		storeRev += rows[i].Revenue7d
	}
	keySet := selectKeySKUIDs(rows, storeRev)
	for i := range rows {
		if keySet[rows[i].OzonProductID] {
			rows[i].IsKeySKU = true
		}
		if storeRev > 0 && rows[i].Revenue7d > 0 {
			share := round1((rows[i].Revenue7d / storeRev) * 100)
			rows[i].RevenueSharePct = &share
		}
	}
	return &SKUMetricsBundle{
		MetricDate:     testMetricDate(),
		Rows:           rows,
		StoreRevenue7d: storeRev,
		TotalScanned:   len(rows),
	}
}

func findCandidate(domains []ActionDomainContext, domain string, typ ActionType) *ActionCandidate {
	for _, d := range domains {
		if d.Domain != domain {
			continue
		}
		for i := range d.Candidates {
			if d.Candidates[i].Type == typ {
				return &d.Candidates[i]
			}
		}
	}
	return nil
}

func TestStockReplenishment_CoverageThresholds(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	rows := []SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 10
			r.DaysOfCover = ptrCoverage(5)
			r.Revenue7d = 7000
			r.Orders7d = 7
		}),
		makeSKURow(2, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 5
			r.DaysOfCover = ptrCoverage(12)
			r.Revenue7d = 1400
			r.Orders7d = 2
		}),
		makeSKURow(3, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 20
			r.DaysOfCover = ptrCoverage(20)
			r.Revenue7d = 5000
			r.Orders7d = 5
		}),
	}
	filtered := filterStockReplenishmentRows(buildTestBundle(rows), cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 replenishment rows, got %d", len(filtered))
	}
}

func TestNoSalesWithStock_RequiresConfirmedMetricRow(t *testing.T) {
	rows := []SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.OrdersCount = 0
			r.StockAvailable = 5
		}),
		makeSKURow(2, func(r *SKUMetricRow) {
			r.HasMetricRow = false
			r.OrdersCount = 0
			r.StockAvailable = 10
		}),
	}
	filtered := filterNoSalesWithStockRows(buildTestBundle(rows))
	if len(filtered) != 1 || filtered[0].OzonProductID != 1 {
		t.Fatalf("only confirmed metric row should qualify, got %+v", filtered)
	}
}

func TestBuildCandidateContext_StockCandidates(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle([]SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.OrdersCount = 0
			r.StockAvailable = 8
			r.DaysOfCover = ptrCoverage(6)
			r.Revenue7d = 700
			r.Orders7d = 7
		}),
		makeSKURow(2, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.OrdersCount = 0
			r.StockAvailable = 3
			r.Revenue = 0
			r.Revenue7d = 0
		}),
	})
	repo.adCampaignCount = 0
	repo.funnelData = &FunnelDataBundle{HasAccountScope: false}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}

	replenish := findCandidate(ctx.Domains, ActionDomainStock, ActionTypeStockReplenishment)
	if replenish == nil {
		t.Fatal("expected stock_replenishment candidate")
	}
	if len(replenish.TopExamples) > cfg.MaxTopExamplesPerCandidate {
		t.Fatalf("top_examples=%d exceeds max", len(replenish.TopExamples))
	}
	if replenish.EntityGroupRef == nil || replenish.EntityGroupRef.Count != 1 {
		t.Fatalf("entity_group_ref count want 1, got %+v", replenish.EntityGroupRef)
	}
	if len(replenish.Limitations) == 0 {
		t.Fatal("expected limitations")
	}

	noSales := findCandidate(ctx.Domains, ActionDomainStock, ActionTypeNoSalesWithStock)
	if noSales == nil {
		t.Fatal("expected no_sales_with_stock candidate")
	}
}

func TestSKUOrdersDrop_NegativeDelta(t *testing.T) {
	rows := []SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.HasPrevMetricRow = true
			r.OrdersCount = 2
			r.PrevOrdersCount = 10
			r.OrdersDelta = -8
			r.RevenueDelta = -5000
		}),
	}
	filtered := filterSKUOrdersDropRows(buildTestBundle(rows))
	if len(filtered) != 1 {
		t.Fatalf("expected 1 drop row, got %d", len(filtered))
	}
}

func TestKeySKURisk_MultiRiskNotDuplicate(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	rows := []SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.HasPrevMetricRow = true
			r.IsKeySKU = true
			r.Revenue7d = 8000
			r.StockAvailable = 3
			r.DaysOfCover = ptrCoverage(5)
			r.OrdersCount = 0
			r.OrdersDelta = -2
			r.PrevOrdersCount = 2
		}),
		makeSKURow(2, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.IsKeySKU = true
			r.Revenue7d = 100
			r.Orders7d = 1
			r.OrdersCount = 1
			r.StockAvailable = 2
			r.DaysOfCover = ptrCoverage(5)
		}),
	}
	bundle := buildTestBundle(rows)
	bundle.Rows[0].IsKeySKU = true
	bundle.Rows[1].IsKeySKU = true
	replenishSet := map[int64]bool{1: true}
	dropSet := map[int64]bool{}
	risks := filterKeySKURiskRows(bundle, cfg, replenishSet, dropSet)
	if len(risks) != 1 {
		t.Fatalf("expected 1 key_sku_risk, got %d", len(risks))
	}
	if len(risks[0].RiskTypes) < 2 {
		t.Fatalf("expected multiple risk types, got %v", risks[0].RiskTypes)
	}
}

func TestAdvertising_HighDRRAndNoOrders(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	drr := 150.0
	repo := newFakeCandidateRepo()
	repo.adCampaignCount = 2
	repo.adCampaigns = []AdCampaignMetricRow{
		{CampaignExternalID: "101", CampaignName: "High DRR", Spend7d: 2000, Revenue7d: 1000, Orders7d: 5, DRRPct: &drr, HasMetricData: true},
		{CampaignExternalID: "102", CampaignName: "No orders", Spend7d: 1500, Revenue7d: 0, Orders7d: 0, HasMetricData: true},
	}
	repo.skuBundle = buildTestBundle(nil)
	repo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}

	highDRR := findCandidate(ctx.Domains, ActionDomainAdvertising, ActionTypeAdHighDRR)
	if highDRR == nil {
		t.Fatal("expected ad_high_drr")
	}
	noOrders := findCandidate(ctx.Domains, ActionDomainAdvertising, ActionTypeAdSpendNoOrders)
	if noOrders == nil {
		t.Fatal("expected ad_spend_no_orders")
	}
	for _, lim := range highDRR.Limitations {
		if lim == "" {
			t.Fatal("empty limitation")
		}
	}
	for _, ex := range highDRR.TopExamples {
		if ex.EntityType != "campaign" {
			t.Fatalf("expected campaign-level examples, got %s", ex.EntityType)
		}
	}
}

func TestAdvertising_MissingAdDataNotZeroSpend(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	repo := newFakeCandidateRepo()
	repo.adCampaignCount = 3
	repo.adCampaigns = []AdCampaignMetricRow{
		{CampaignExternalID: "1", Spend7d: 0, HasMetricData: false},
	}
	repo.skuBundle = buildTestBundle(nil)
	repo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	adDomain := ctx.Domains[2]
	if adDomain.Status != DomainStatusSourceNotAvailable {
		t.Fatalf("status=%q want source_not_available", adDomain.Status)
	}
	if len(adDomain.Candidates) != 0 {
		t.Fatal("should not create candidates without metric data")
	}
	if ctx.CoverageReport.TotalCampaignsScanned != 0 {
		t.Fatalf("total_campaigns_scanned=%d want 0", ctx.CoverageReport.TotalCampaignsScanned)
	}
}

func TestCoverageReport_TotalCampaignsScanned_7dPeriod(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	drr := 150.0
	repo := newFakeCandidateRepo()
	repo.adCampaignCount = 5
	repo.adCampaigns = []AdCampaignMetricRow{
		{CampaignExternalID: "1", CampaignName: "With metrics A", Spend7d: 2000, Revenue7d: 1000, Orders7d: 5, DRRPct: &drr, HasMetricData: true},
		{CampaignExternalID: "2", CampaignName: "With metrics B", Spend7d: 1500, Revenue7d: 0, Orders7d: 0, HasMetricData: true},
		{CampaignExternalID: "3", CampaignName: "Historical only", Spend7d: 0, HasMetricData: false},
		{CampaignExternalID: "4", CampaignName: "Historical only 2", Spend7d: 0, HasMetricData: false},
		{CampaignExternalID: "5", CampaignName: "Historical only 3", Spend7d: 0, HasMetricData: false},
	}
	repo.skuBundle = buildTestBundle(nil)
	repo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	if ctx.CoverageReport.TotalCampaignsScanned != 2 {
		t.Fatalf("total_campaigns_scanned=%d want 2 (only campaigns with 7d metrics)", ctx.CoverageReport.TotalCampaignsScanned)
	}
	if findCandidate(ctx.Domains, ActionDomainAdvertising, ActionTypeAdHighDRR) == nil {
		t.Fatal("expected ad_high_drr from scanned campaign with metrics")
	}
	if findCandidate(ctx.Domains, ActionDomainAdvertising, ActionTypeAdSpendNoOrders) == nil {
		t.Fatal("expected ad_spend_no_orders from scanned campaign with metrics")
	}
}

func TestFunnelWeakTransition_AndMissingData(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	impressions := int64(1000)
	views := int64(100)
	prevImp := int64(1000)
	prevViews := int64(200)
	cart := int64(20)
	prevCart := int64(30)

	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle(nil)
	repo.adCampaignCount = 0
	repo.funnelData = &FunnelDataBundle{
		HasAccountScope: true,
		Current: &FunnelDayMetrics{
			Impressions:  &impressions,
			ProductViews: &views,
			AddToCart:    &cart,
			Orders:       ptrInt64(10),
		},
		Previous: &FunnelDayMetrics{
			Impressions:  &prevImp,
			ProductViews: &prevViews,
			AddToCart:    &prevCart,
			Orders:       ptrInt64(12),
		},
	}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	funnel := findCandidate(ctx.Domains, ActionDomainFunnel, ActionTypeFunnelWeakTransition)
	if funnel == nil {
		t.Fatal("expected funnel_weak_transition")
	}
	if funnel.EntityGroupRef != nil {
		t.Fatal("entity_group_ref should not be created without SKU contributors")
	}

	repo.funnelData = nil
	ctx2, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	if ctx2.Domains[3].Status != DomainStatusSourceNotAvailable {
		t.Fatalf("funnel status=%q", ctx2.Domains[3].Status)
	}
}

func TestCompactContext_100SKU(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	rows := make([]SKUMetricRow, 0, 100)
	for i := int64(1); i <= 100; i++ {
		id := i
		rows = append(rows, makeSKURow(id, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 5
			r.DaysOfCover = ptrCoverage(5)
			r.Revenue7d = float64(101-id) * 100
			r.Orders7d = 1
		}))
	}
	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle(rows)
	repo.adCampaignCount = 0
	repo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	cand := findCandidate(ctx.Domains, ActionDomainStock, ActionTypeStockReplenishment)
	if cand == nil {
		t.Fatal("expected replenishment candidate")
	}
	if len(cand.TopExamples) > cfg.MaxTopExamplesPerCandidate {
		t.Fatalf("top_examples=%d", len(cand.TopExamples))
	}
	if cand.EntityGroupRef == nil || cand.EntityGroupRef.Count != 100 {
		t.Fatalf("entity group count=%v want 100", cand.EntityGroupRef)
	}
	raw, _ := json.Marshal(cand)
	if len(raw) > 50000 {
		t.Fatalf("candidate json suspiciously large: %d bytes", len(raw))
	}
}

func TestDeterministicGroupKey(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle([]SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 4
			r.DaysOfCover = ptrCoverage(4)
			r.Revenue7d = 700
			r.Orders7d = 7
		}),
	})
	repo.adCampaignCount = 0
	repo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(repo, cfg)
	_, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	key1 := repo.entityGroups
	_, err = builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if len(key1) != len(repo.entityGroups) {
		t.Fatalf("entity group keys changed between builds")
	}
	for k := range key1 {
		if _, ok := repo.entityGroups[k]; !ok {
			t.Fatalf("missing key %q on second build", k)
		}
	}
}

func TestDomainIsolation_AdvertisingErrorDoesNotBreakStock(t *testing.T) {
	cfg := DefaultCandidateBuilderConfig()
	repo := newFakeCandidateRepo()
	repo.skuBundle = buildTestBundle([]SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 2
			r.DaysOfCover = ptrCoverage(3)
			r.Revenue7d = 700
			r.Orders7d = 7
		}),
	})
	repo.adCampaignCount = 1
	repo.adListErr = context.Canceled
	repo.funnelData = &FunnelDataBundle{HasAccountScope: false}

	builder := NewCandidateBuilder(repo, cfg)
	ctx, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate())
	if err != nil {
		t.Fatalf("BuildCandidateContext should not fail entirely: %v", err)
	}
	if findCandidate(ctx.Domains, ActionDomainStock, ActionTypeStockReplenishment) == nil {
		t.Fatal("stock candidate missing")
	}
	if ctx.Domains[2].Status != DomainStatusError {
		t.Fatalf("advertising status=%q", ctx.Domains[2].Status)
	}
	if len(ctx.DataQualityNotes) == 0 {
		t.Fatal("expected data quality notes")
	}
}

func TestBuildCandidateContext_DoesNotCreateActionItems(t *testing.T) {
	readRepo := &mockRepo{}
	svc := NewService(readRepo)
	if len(readRepo.items) != 0 {
		t.Fatal("precondition: no action items")
	}

	cfg := DefaultCandidateBuilderConfig()
	candRepo := newFakeCandidateRepo()
	candRepo.skuBundle = buildTestBundle([]SKUMetricRow{
		makeSKURow(1, func(r *SKUMetricRow) {
			r.HasMetricRow = true
			r.StockAvailable = 2
			r.DaysOfCover = ptrCoverage(3)
			r.Revenue7d = 500
			r.Orders7d = 3
		}),
	})
	candRepo.adCampaignCount = 0
	candRepo.funnelData = &FunnelDataBundle{}

	builder := NewCandidateBuilder(candRepo, cfg)
	if _, err := builder.BuildCandidateContext(context.Background(), 1, testMetricDate()); err != nil {
		t.Fatalf("BuildCandidateContext: %v", err)
	}
	resp, err := svc.ListActions(context.Background(), 1, ListActionsFilters{MetricDate: "latest"})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if resp.Status != string(ActionResponseStatusNotGenerated) {
		t.Fatalf("ListActions status=%q want not_generated", resp.Status)
	}
}

var _ CandidateRepository = (*fakeCandidateRepo)(nil)
