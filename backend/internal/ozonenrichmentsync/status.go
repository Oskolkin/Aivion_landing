package ozonenrichmentsync

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatusResponse struct {
	SellerID      int64               `json:"seller_id"`
	MetricDate    string              `json:"metric_date"`
	LastRun       *StatusLastRun      `json:"last_run,omitempty"`
	DataAvailable StatusDataAvailable `json:"data_available"`
}

type StatusLastRun struct {
	ID                      int64    `json:"id"`
	Status                  string   `json:"status"`
	Scope                   string   `json:"scope,omitempty"`
	Source                  string   `json:"source,omitempty"`
	ProductInfoCount        int32    `json:"product_info_count"`
	RelatedSKUCount         int32    `json:"related_sku_count"`
	StockAnalyticsCount     int32    `json:"stock_analytics_count"`
	CampaignCount           int32    `json:"campaign_count"`
	CampaignDailyStatsCount int32    `json:"campaign_daily_stats_count"`
	CampaignObjectsCount    int32    `json:"campaign_objects_count"`
	CampaignProductsCount   int32    `json:"campaign_products_count"`
	Warnings                []string `json:"warnings"`
}

type StatusDataAvailable struct {
	APICapabilities  bool `json:"api_capabilities"`
	ProductInfo      bool `json:"product_info"`
	RelatedSKUs      bool `json:"related_skus"`
	StockAnalytics   bool `json:"stock_analytics"`
	Campaigns        bool `json:"campaigns"`
	CampaignStats    bool `json:"campaign_stats"`
	CampaignObjects  bool `json:"campaign_objects"`
	CampaignProducts bool `json:"campaign_products"`
}

func (s *Service) GetEnrichmentStatus(ctx context.Context, sellerAccountID int64, metricDate time.Time, scope string) (StatusResponse, error) {
	scope = normalizeScope(scope)
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	resp := StatusResponse{
		SellerID:   sellerAccountID,
		MetricDate: metricDate.Format("2006-01-02"),
	}

	run, err := s.getLatestRunForStatus(ctx, sellerAccountID, metricDate, scope)
	if err == nil {
		var warnings []string
		_ = json.Unmarshal(run.WarningsJson, &warnings)
		source := extractRunSource(run.RawSummaryJson)
		resp.LastRun = &StatusLastRun{
			ID:                      run.ID,
			Status:                  run.Status,
			Scope:                   run.Scope,
			Source:                  source,
			ProductInfoCount:        run.ProductInfoCount,
			RelatedSKUCount:         run.RelatedSkuCount,
			StockAnalyticsCount:     run.StockAnalyticsCount,
			CampaignCount:           run.CampaignCount,
			CampaignDailyStatsCount: run.CampaignDailyStatsCount,
			CampaignObjectsCount:    run.CampaignObjectsCount,
			CampaignProductsCount:   run.CampaignProductsCount,
			Warnings:                warnings,
		}
	}

	capabilities, _ := s.queries.GetOzonAPICapabilitiesBySeller(ctx, sellerAccountID)
	for _, cap := range capabilities {
		if cap.Status == statusAvailable {
			resp.DataAvailable.APICapabilities = true
			break
		}
	}

	productInfoCount, err := s.queries.CountOzonProductInfoSnapshotsBySellerAndDate(ctx, dbgen.CountOzonProductInfoSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    pgDate(metricDate),
	})
	if err == nil && productInfoCount > 0 {
		resp.DataAvailable.ProductInfo = true
	}

	relatedCount, err := s.queries.CountOzonRelatedSKUsBySeller(ctx, sellerAccountID)
	if err == nil && relatedCount > 0 {
		resp.DataAvailable.RelatedSKUs = true
	}

	stockRows, err := s.queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    pgDate(metricDate),
	})
	if err == nil && len(stockRows) > 0 {
		resp.DataAvailable.StockAnalytics = true
	}

	campaigns, err := s.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        pgtype.Date{Time: metricDate.AddDate(0, 0, -statsLookbackDays), Valid: true},
		DateTo:          pgDate(metricDate),
	})
	if err == nil && len(campaigns) > 0 {
		resp.DataAvailable.Campaigns = true
	}

	metrics, err := s.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        pgtype.Date{Time: metricDate.AddDate(0, 0, -statsLookbackDays), Valid: true},
		DateTo:          pgDate(metricDate),
	})
	if err == nil && len(metrics) > 0 {
		resp.DataAvailable.CampaignStats = true
	}

	objectsCount, err := s.queries.CountAdCampaignObjectsBySeller(ctx, sellerAccountID)
	if err == nil && objectsCount > 0 {
		resp.DataAvailable.CampaignObjects = true
	}

	skuLinks, err := s.queries.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	if err == nil && len(skuLinks) > 0 {
		resp.DataAvailable.CampaignProducts = true
	}

	return resp, nil
}

func (s *Service) getLatestRunForStatus(ctx context.Context, sellerAccountID int64, metricDate time.Time, scope string) (dbgen.OzonEnrichmentSyncRun, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == scopeP0 {
		return s.queries.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      pgDate(metricDate),
		})
	}
	return s.queries.GetLatestOzonEnrichmentSyncRun(ctx, dbgen.GetLatestOzonEnrichmentSyncRunParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(metricDate),
		Scope:           normalizeScope(scope),
	})
}

func extractRunSource(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		return ""
	}
	if v, ok := summary["source"].(string); ok {
		return v
	}
	return ""
}

func NewStatusService(db *pgxpool.Pool) *Service {
	return NewService(db, nil)
}
