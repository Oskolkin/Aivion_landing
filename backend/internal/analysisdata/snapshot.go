package analysisdata

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// AnalysisDataSnapshot is the shared raw cabinet data snapshot used by both the
// legacy SellerAIDataPack builder and the Unified Analysis Pack builder.
//
// It contains only loaded database rows and derived technical metadata
// (windows, enrichment source, loaded history). It must not contain AI conclusions,
// signals, candidates, dashboard/actions payloads, or user-facing formatting.
type AnalysisDataSnapshot struct {
	SellerAccountID int64
	MetricDate      pgtype.Date
	Windows         DataWindows
	// LoadedHistory is the actual SQL range that was fetched (14 or 30 days).
	// Legacy BuildPack ignores this field and continues to use Windows (14d semantics).
	LoadedHistory LoadedHistory

	EnrichmentSource string

	AccountMetrics       []dbgen.DailyAccountMetric
	SKUMetrics           []dbgen.DailySkuMetric
	Products             []dbgen.Product
	ProductInfoSnapshots []dbgen.OzonProductInfoSnapshot
	RelatedSKUs          []dbgen.OzonRelatedSku
	StockSnapshots       []dbgen.StockAnalyticsSnapshot
	Campaigns            []dbgen.AdCampaign
	AdMetrics            []dbgen.AdMetricsDaily
	CampaignSKUs         []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow
	FunnelRows           []dbgen.ProductAnalyticsDaily
	Categories           []dbgen.DescriptionCategory
	PriceConstraints     []dbgen.SkuEffectiveConstraint
	EnrichmentRun        *dbgen.OzonEnrichmentSyncRun
	APICapabilities      []dbgen.OzonApiCapability
}
