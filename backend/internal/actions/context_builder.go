package actions

import (
	"context"
	"fmt"
	"time"
)

// CandidateBuilder assembles compact action candidate context from store metrics.
type CandidateBuilder struct {
	repo CandidateRepository
	cfg  CandidateBuilderConfig
}

func NewCandidateBuilder(repo CandidateRepository, cfg CandidateBuilderConfig) *CandidateBuilder {
	if cfg.MaxCandidatesPerDomain == 0 {
		cfg = DefaultCandidateBuilderConfig()
	}
	return &CandidateBuilder{repo: repo, cfg: cfg}
}

func (b *CandidateBuilder) BindActionsRunID(runID int64) {
	if r, ok := b.repo.(*SQLCCandidateRepository); ok {
		r.BindActionsRunID(runID)
	}
}

// BuildCandidateContext analyzes store data and returns compact candidates for downstream AI.
// It persists entity groups but does not create action_items or invoke AI.
func (b *CandidateBuilder) BuildCandidateContext(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*ActionCandidateContext, error) {
	metricDate = metricDateUTC(metricDate)
	dateStr := formatMetricDate(metricDate)

	snapshotRow, snapErr := b.repo.GetAccountSnapshot(ctx, sellerAccountID, metricDate)
	dataQualityNotes := make([]string, 0)
	if snapErr != nil {
		dataQualityNotes = append(dataQualityNotes, fmt.Sprintf("Account snapshot: %v", snapErr))
	}

	skuBundle, skuErr := b.repo.LoadSKUMetricsBundle(ctx, sellerAccountID, metricDate)
	if skuErr != nil {
		dataQualityNotes = append(dataQualityNotes, fmt.Sprintf("SKU metrics: %v", skuErr))
	}

	var stockDomain, skuSalesDomain ActionDomainContext
	stockFound, skuSalesFound := 0, 0
	if skuErr == nil {
		stockDomain, stockFound = buildStockDomain(ctx, b.repo, sellerAccountID, skuBundle, b.cfg)
		skuSalesDomain, skuSalesFound = buildSKUSalesDomain(ctx, b.repo, sellerAccountID, skuBundle, b.cfg)
	} else {
		errNote := []string{fmt.Sprintf("SKU домены недоступны: %v", skuErr)}
		stockDomain = ActionDomainContext{Domain: ActionDomainStock, Status: DomainStatusError, Candidates: []ActionCandidate{}, DataQualityNotes: errNote}
		skuSalesDomain = ActionDomainContext{Domain: ActionDomainSKUSales, Status: DomainStatusError, Candidates: []ActionCandidate{}, DataQualityNotes: errNote}
	}

	adDomain, adFound, campaignsScanned := buildAdvertisingDomain(ctx, b.repo, sellerAccountID, metricDate, b.cfg)
	funnelDomain, funnelFound := buildFunnelDomain(ctx, b.repo, sellerAccountID, metricDate, b.cfg)

	domains := []ActionDomainContext{stockDomain, skuSalesDomain, adDomain, funnelDomain}
	for _, d := range domains {
		dataQualityNotes = append(dataQualityNotes, d.DataQualityNotes...)
	}

	totalSKUs := 0
	if skuBundle != nil {
		totalSKUs = skuBundle.TotalScanned
	}

	coverage := CandidateCoverageReport{
		TotalSKUsScanned:           totalSKUs,
		TotalCampaignsScanned:      campaignsScanned,
		StockCandidatesFound:       stockFound,
		SKUSalesCandidatesFound:    skuSalesFound,
		AdvertisingCandidatesFound: adFound,
		FunnelCandidatesFound:      funnelFound,
	}

	return &ActionCandidateContext{
		SellerAccountID:    sellerAccountID,
		MetricDate:         dateStr,
		AccountSnapshot:    buildAccountSnapshot(snapshotRow),
		Domains:            domains,
		AllowedActionTypes: DefaultAllowedActionTypes,
		ForbiddenClaims:    DefaultForbiddenClaims,
		DataQualityNotes:   dataQualityNotes,
		CoverageReport:     coverage,
	}, nil
}
