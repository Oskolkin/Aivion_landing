package aidatapack

import (
	"fmt"
	"time"
)

// BuildPack assembles SellerAIDataPack from a shared AnalysisDataSnapshot.
func BuildPack(ctx *loadContext) (*SellerAIDataPack, error) {
	windows := ctx.Windows
	b := NewBuilder(windows)

	known := buildKnownSKUSet(ctx)
	knownSKUSet := make(map[string]struct{}, len(known))
	for sku := range known {
		knownSKUSet[sku] = struct{}{}
	}

	selected, err := allSKUsStableOrder(known)
	if err != nil {
		return nil, err
	}

	skuEntities := buildSKUEntities(known)
	skuGroups := buildSKUGroupEntities(ctx, knownSKUSet)
	campaignEntities := buildCampaignEntities(ctx)
	warehouseEntities := buildWarehouseEntities(ctx)
	categoryEntities := buildCategoryEntities(ctx, known)

	accountRollup := buildAccountRollup(b, ctx)
	skuRollups := buildSKURollups(b, ctx, known, selected)
	productEnrichment := buildProductEnrichment(b, ctx, known)
	stockPositions := buildStockPositions(b, ctx, known)
	stockMultiRowSKUs := 0
	for _, row := range stockPositions {
		if row.SourceRowsCount > 1 {
			stockMultiRowSKUs++
		}
	}
	campaignRollups := buildCampaignRollups(b, ctx)
	enrichIdx := enrichmentIndex(productEnrichment)
	stockIdx := stockPositionsIndex(stockPositions)
	campaignLinks := buildCampaignProductLinks(ctx, known, stockIdx, enrichIdx)
	funnelRollup := buildFunnelRollup(b, ctx)
	stockLites := liteStockSnapshots(ctx)
	categoryRollups := buildCategoryRollups(b, ctx, known, categoryEntities, stockLites)
	financeRollup := buildFinanceRollup()
	constraints := buildConstraintsBlock(ctx)

	pack := &SellerAIDataPack{
		SchemaVersion: SchemaVersion,
		SellerContext: SellerContext{
			SellerAccountID: ctx.SellerAccountID,
			Marketplace:     MarketplaceOzon,
			MetricDate:      windows.CurrentDay,
			Timezone:        DefaultTimezone,
		},
		DataWindows: windows,
		DataQuality: buildDataQuality(ctx, len(selected), len(known), stockMultiRowSKUs),
		EntityRegistry: EntityRegistry{
			SKUs:       skuEntities,
			SKUGroups:  skuGroups,
			Campaigns:  campaignEntities,
			Warehouses: warehouseEntities,
			Categories: categoryEntities,
		},
		FactRegistry: b.FactRegistry(),
		Tables: Tables{
			AccountRollup:        accountRollup,
			SKURollups:           skuRollups,
			ProductEnrichment:    productEnrichment,
			StockPositions:       stockPositions,
			CampaignRollups:      campaignRollups,
			CampaignProductLinks: campaignLinks,
			FunnelRollup:         funnelRollup,
			CategoryRollups:      categoryRollups,
			FinanceRollup:        financeRollup,
		},
		Constraints: constraints,
	}

	if err := ValidateDataPack(pack); err != nil {
		return nil, fmt.Errorf("validate data pack: %w", err)
	}
	return pack, nil
}

func pgDateUTC(t time.Time) interface{} {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
