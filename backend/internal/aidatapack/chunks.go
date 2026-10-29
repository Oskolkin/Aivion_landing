package aidatapack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// BuildChunks splits a SellerAIDataPack into deterministic technical chunks.
func BuildChunks(pack *SellerAIDataPack, opts ChunkOptions) (*ChunkSet, error) {
	if pack == nil {
		return nil, fmt.Errorf("pack is nil")
	}
	if opts.MaxRowsPerChunk <= 0 {
		opts.MaxRowsPerChunk = 50
	}
	if opts.MaxApproxTokensPerChunk <= 0 {
		opts.MaxApproxTokensPerChunk = 12000
	}
	if !opts.IncludeLocalFactCatalog {
		opts.IncludeLocalFactCatalog = true
	}
	if !opts.IncludeLocalEntityCatalog {
		opts.IncludeLocalEntityCatalog = true
	}

	base := &chunkBase{
		pack: pack,
		opts: opts,
	}
	var chunks []DataPackChunk

	manifest, err := base.buildManifestChunk()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, manifest)

	account, err := base.buildAccountFunnelQualityChunk()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, account)

	entityCore, err := base.buildEntityRegistryCoreChunk()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, entityCore)

	skuEntityChunks, err := base.buildSKUEntityChunks()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, skuEntityChunks...)

	skuGroupChunks, err := base.buildSKUGroupChunks()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, skuGroupChunks...)

	skuRollupChunks, err := buildTableRowChunks(base, "sku_rollups", pack.Tables.SKURollups, 20, skuRollupKey)
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, skuRollupChunks...)

	stockChunks, err := buildTableRowChunks(base, "stock_positions", pack.Tables.StockPositions, 20, stockPositionKey)
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, stockChunks...)

	enrichChunks, err := buildTableRowChunks(base, "product_enrichment", pack.Tables.ProductEnrichment, 15, productEnrichmentKey)
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, enrichChunks...)

	campaignChunks, err := base.buildCampaignRollupChunks()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, campaignChunks...)

	linkChunks, err := base.buildCampaignProductLinkChunks()
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, linkChunks...)

	categoryChunks, err := buildTableRowChunks(base, "category_rollups", pack.Tables.CategoryRollups, 50, categoryRollupKey)
	if err != nil {
		return nil, err
	}
	chunks = append(chunks, categoryChunks...)

	coverage := computeChunkCoverage(pack, chunks)
	chunkIDs := make([]string, len(chunks))
	totalTokens := 0
	for i, c := range chunks {
		chunkIDs[i] = c.ChunkID
		totalTokens += c.ApproxTokens
	}

	set := &ChunkSet{
		SchemaVersion:         ChunkSetSchemaVersion,
		DataPackSchemaVersion: pack.SchemaVersion,
		SellerAccountID:       pack.SellerContext.SellerAccountID,
		MetricDate:            pack.SellerContext.MetricDate,
		ChunkingPolicy: ChunkingPolicy{
			Policy:                   ChunkingPolicyName,
			SelectionPolicy:          SelectionPolicyTechnical,
			OrderingPolicy:           OrderingPolicyStableAsc,
			BusinessFilteringAllowed: false,
		},
		Manifest: ChunkManifestSummary{
			ChunkCount:        len(chunks),
			DatasetCounts:     datasetCounts(pack),
			ChunkIDs:          chunkIDs,
			TotalApproxTokens: totalTokens,
		},
		Chunks:   chunks,
		Coverage: coverage,
	}
	return set, nil
}

type chunkBase struct {
	pack *SellerAIDataPack
	opts ChunkOptions
}

func (b *chunkBase) sellerID() int64 {
	return b.pack.SellerContext.SellerAccountID
}

func (b *chunkBase) metricDate() string {
	return b.pack.SellerContext.MetricDate
}

func (b *chunkBase) newChunk(chunkID, chunkType, sourcePath, sourceTable string, rowCount int) DataPackChunk {
	return DataPackChunk{
		ChunkID:         chunkID,
		ChunkType:       chunkType,
		SourcePath:      sourcePath,
		SourceTable:     sourceTable,
		SchemaVersion:   ChunkSetSchemaVersion,
		SellerAccountID: b.sellerID(),
		MetricDate:      b.metricDate(),
		Ordering:        OrderingPolicyStableAsc,
		SelectionPolicy: SelectionPolicyTechnical,
		RowCount:        rowCount,
	}
}

func (b *chunkBase) finalizeChunk(c *DataPackChunk, factIDs, entityIDs []string) error {
	if b.opts.IncludeLocalFactCatalog {
		c.LocalFactCatalog = buildLocalFactCatalog(b.pack, factIDs)
	}
	if b.opts.IncludeLocalEntityCatalog {
		c.LocalEntityCatalog = buildLocalEntityCatalog(b.pack, entityIDs)
	}
	hash, tokens, err := hashAndTokenizeChunk(c)
	if err != nil {
		return err
	}
	c.ChunkHash = hash
	c.ApproxTokens = tokens
	if tokens > b.opts.MaxApproxTokensPerChunk {
		c.Limitations = append(c.Limitations,
			fmt.Sprintf("chunk approx_tokens=%d exceeds budget=%d; split further with smaller MaxRowsPerChunk", tokens, b.opts.MaxApproxTokensPerChunk))
		hash, tokens, err = hashAndTokenizeChunk(c)
		if err != nil {
			return err
		}
		c.ChunkHash = hash
		c.ApproxTokens = tokens
	}
	return nil
}

func (b *chunkBase) buildManifestChunk() (DataPackChunk, error) {
	counts := datasetCounts(b.pack)
	payload := map[string]any{
		"schema_version": b.pack.SchemaVersion,
		"seller_context": b.pack.SellerContext,
		"data_windows":   b.pack.DataWindows,
		"data_quality":   b.pack.DataQuality,
		"dataset_counts": counts,
		"entity_registry_counts": map[string]int{
			"skus":       len(b.pack.EntityRegistry.SKUs),
			"sku_groups": len(b.pack.EntityRegistry.SKUGroups),
			"campaigns":  len(b.pack.EntityRegistry.Campaigns),
			"warehouses": len(b.pack.EntityRegistry.Warehouses),
			"categories": len(b.pack.EntityRegistry.Categories),
		},
		"fact_registry_count": len(b.pack.FactRegistry),
		"limitations":         b.pack.DataQuality.Limitations,
		"constraints":         b.pack.Constraints,
		"finance_rollup":      b.pack.Tables.FinanceRollup,
	}
	c := b.newChunk("manifest_context", "manifest", "pack", "", 0)
	c.Payload = payload
	c.Limitations = append([]string(nil), b.pack.DataQuality.Limitations...)
	if err := b.finalizeChunk(&c, collectManifestFactIDs(b.pack), nil); err != nil {
		return DataPackChunk{}, err
	}
	return c, nil
}

func collectManifestFactIDs(pack *SellerAIDataPack) []string {
	var ids []string
	ids = append(ids, pack.Tables.AccountRollup.FactIDs...)
	ids = append(ids, pack.Tables.FunnelRollup.FactIDs...)
	return compactStringIDs(ids)
}

func (b *chunkBase) buildAccountFunnelQualityChunk() (DataPackChunk, error) {
	payload := map[string]any{
		"account_rollup": b.pack.Tables.AccountRollup,
		"funnel_rollup":  b.pack.Tables.FunnelRollup,
		"finance_rollup": b.pack.Tables.FinanceRollup,
		"constraints":    b.pack.Constraints,
		"data_quality":   b.pack.DataQuality,
	}
	c := b.newChunk("account_funnel_quality_constraints", "account_level_tables", "tables", "account_funnel_finance", 0)
	c.Payload = payload
	var factIDs []string
	factIDs = append(factIDs, b.pack.Tables.AccountRollup.FactIDs...)
	factIDs = append(factIDs, b.pack.Tables.FunnelRollup.FactIDs...)
	factIDs = compactStringIDs(factIDs)
	if err := b.finalizeChunk(&c, factIDs, nil); err != nil {
		return DataPackChunk{}, err
	}
	return c, nil
}

func (b *chunkBase) buildEntityRegistryCoreChunk() (DataPackChunk, error) {
	payload := map[string]any{
		"campaigns":  b.pack.EntityRegistry.Campaigns,
		"categories": b.pack.EntityRegistry.Categories,
		"warehouses": b.pack.EntityRegistry.Warehouses,
		"counts": map[string]int{
			"skus":       len(b.pack.EntityRegistry.SKUs),
			"sku_groups": len(b.pack.EntityRegistry.SKUGroups),
			"campaigns":  len(b.pack.EntityRegistry.Campaigns),
			"warehouses": len(b.pack.EntityRegistry.Warehouses),
			"categories": len(b.pack.EntityRegistry.Categories),
		},
	}
	c := b.newChunk("entity_registry_core", "entity_dictionary", "entity_registry", "campaigns_categories_warehouses", 0)
	c.Payload = payload
	var entityIDs []string
	for id := range b.pack.EntityRegistry.Campaigns {
		entityIDs = append(entityIDs, id)
	}
	for id := range b.pack.EntityRegistry.Categories {
		entityIDs = append(entityIDs, id)
	}
	for id := range b.pack.EntityRegistry.Warehouses {
		entityIDs = append(entityIDs, id)
	}
	sort.Strings(entityIDs)
	if err := b.finalizeChunk(&c, nil, entityIDs); err != nil {
		return DataPackChunk{}, err
	}
	return c, nil
}

func (b *chunkBase) buildSKUEntityChunks() ([]DataPackChunk, error) {
	entities := make([]SKUEntity, 0, len(b.pack.EntityRegistry.SKUs))
	for _, e := range b.pack.EntityRegistry.SKUs {
		entities = append(entities, e)
	}
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].EntityID < entities[j].EntityID
	})
	return buildEntityRowChunks(b, "sku_entities", "entity_registry.skus", entities, 50, func(e SKUEntity) string {
		return e.EntityID
	}, func(e SKUEntity) string { return e.EntityID })
}

func (b *chunkBase) buildSKUGroupChunks() ([]DataPackChunk, error) {
	groups := make([]SKUGroupEntity, 0, len(b.pack.EntityRegistry.SKUGroups))
	for _, g := range b.pack.EntityRegistry.SKUGroups {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].EntityID < groups[j].EntityID
	})
	return buildEntityRowChunks(b, "sku_groups", "entity_registry.sku_groups", groups, 50, func(g SKUGroupEntity) string {
		return g.EntityID
	}, func(g SKUGroupEntity) string { return g.EntityID })
}

func buildEntityRowChunks[T any](
	b *chunkBase,
	prefix, sourcePath string,
	rows []T,
	pageSize int,
	keyFn func(T) string,
	entityFn func(T) string,
) ([]DataPackChunk, error) {
	return buildGenericRowChunks(b, prefix, "entity_dictionary", sourcePath, prefix, rows, pageSize, keyFn, entityFn, nil)
}

func buildTableRowChunks[T any](
	b *chunkBase,
	tableName string,
	rows []T,
	pageSize int,
	keyFn func(T) string,
) ([]DataPackChunk, error) {
	return buildGenericRowChunks(b, tableName, "table_rows", "tables."+tableName, tableName, rows, pageSize, keyFn, entityIDFromRow[T], factIDsFromRow[T])
}

func entityIDFromRow[T any](row T) string {
	switch v := any(row).(type) {
	case SKURollupRow:
		return v.EntityID
	case StockPositionRow:
		return v.EntityID
	case ProductEnrichmentRow:
		return v.EntityID
	case CategoryRollupRow:
		return v.EntityID
	default:
		return ""
	}
}

func factIDsFromRow[T any](row T) []string {
	switch v := any(row).(type) {
	case SKURollupRow:
		return v.FactIDs
	case StockPositionRow:
		return v.FactIDs
	case ProductEnrichmentRow:
		return v.FactIDs
	case CampaignRollupRow:
		return v.FactIDs
	case CategoryRollupRow:
		return v.FactIDs
	default:
		return nil
	}
}

func buildGenericRowChunks[T any](
	b *chunkBase,
	idPrefix, chunkType, sourcePath, sourceTable string,
	rows []T,
	pageSize int,
	keyFn func(T) string,
	entityFn func(T) string,
	factFn func(T) []string,
) ([]DataPackChunk, error) {
	if pageSize <= 0 {
		pageSize = b.opts.MaxRowsPerChunk
	}
	sorted := append([]T(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool {
		return keyFn(sorted[i]) < keyFn(sorted[j])
	})
	var out []DataPackChunk
	for start := 0; start < len(sorted); start += pageSize {
		end := start + pageSize
		if end > len(sorted) {
			end = len(sorted)
		}
		page := sorted[start:end]
		seq := start/pageSize + 1
		chunkID := fmt.Sprintf("%s_%03d", idPrefix, seq)
		c := b.newChunk(chunkID, chunkType, sourcePath, sourceTable, len(page))
		c.Rows = page
		var factIDs, entityIDs []string
		for _, row := range page {
			if factFn != nil {
				factIDs = append(factIDs, factFn(row)...)
			}
			if eid := entityFn(row); eid != "" {
				entityIDs = append(entityIDs, eid)
			}
		}
		factIDs = compactStringIDs(factIDs)
		entityIDs = compactStringIDs(entityIDs)
		if err := b.finalizeChunk(&c, factIDs, entityIDs); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (b *chunkBase) buildCampaignRollupChunks() ([]DataPackChunk, error) {
	rows := append([]CampaignRollupRow(nil), b.pack.Tables.CampaignRollups...)
	for i := range rows {
		// Daily series is synthesis-only; including it in chunks exceeds token budgets.
		rows[i].DailySeries = nil
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CampaignExternalID < rows[j].CampaignExternalID
	})
	chunks, err := buildGenericRowChunks(b, "campaign_rollups", "table_rows", "tables.campaign_rollups", "campaign_rollups", rows, 25,
		func(r CampaignRollupRow) string { return r.CampaignExternalID },
		func(r CampaignRollupRow) string { return r.EntityID },
		func(r CampaignRollupRow) []string { return r.FactIDs },
	)
	if err != nil {
		return nil, err
	}
	limitation := "Advertising metrics are campaign-level only. SKU-level ad spend attribution is unavailable."
	for i := range chunks {
		chunks[i].Limitations = append(chunks[i].Limitations, limitation)
		hash, tokens, err := hashAndTokenizeChunk(&chunks[i])
		if err != nil {
			return nil, err
		}
		chunks[i].ChunkHash = hash
		chunks[i].ApproxTokens = tokens
	}
	return chunks, nil
}

func (b *chunkBase) buildCampaignProductLinkChunks() ([]DataPackChunk, error) {
	rows := append([]CampaignProductLinkRow(nil), b.pack.Tables.CampaignProductLinks...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CampaignExternalID != rows[j].CampaignExternalID {
			return rows[i].CampaignExternalID < rows[j].CampaignExternalID
		}
		return rows[i].SKU < rows[j].SKU
	})
	chunks, err := buildGenericRowChunks(b, "campaign_product_links", "table_rows", "tables.campaign_product_links", "campaign_product_links", rows, 60,
		func(r CampaignProductLinkRow) string { return r.CampaignExternalID + ":" + r.SKU },
		func(r CampaignProductLinkRow) string {
			if r.SKUEntityID != "" {
				return r.SKUEntityID
			}
			return r.CampaignEntityID
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	limitation := "Campaign-product links do not provide SKU-level ad spend attribution."
	for i := range chunks {
		chunks[i].Limitations = append(chunks[i].Limitations, limitation)
		// Include both campaign and sku entities
		var entityIDs []string
		for _, row := range chunks[i].Rows.([]CampaignProductLinkRow) {
			if row.CampaignEntityID != "" {
				entityIDs = append(entityIDs, row.CampaignEntityID)
			}
			if row.SKUEntityID != "" {
				entityIDs = append(entityIDs, row.SKUEntityID)
			}
		}
		entityIDs = compactStringIDs(entityIDs)
		chunks[i].LocalEntityCatalog = buildLocalEntityCatalog(b.pack, entityIDs)
		hash, tokens, err := hashAndTokenizeChunk(&chunks[i])
		if err != nil {
			return nil, err
		}
		chunks[i].ChunkHash = hash
		chunks[i].ApproxTokens = tokens
	}
	return chunks, nil
}

func skuRollupKey(r SKURollupRow) string                 { return r.EntityID }
func stockPositionKey(r StockPositionRow) string         { return r.EntityID }
func productEnrichmentKey(r ProductEnrichmentRow) string { return r.EntityID }
func categoryRollupKey(r CategoryRollupRow) string       { return r.EntityID }

func datasetCounts(pack *SellerAIDataPack) map[string]int {
	return map[string]int{
		"sku_rollups":            len(pack.Tables.SKURollups),
		"stock_positions":        len(pack.Tables.StockPositions),
		"product_enrichment":     len(pack.Tables.ProductEnrichment),
		"campaign_rollups":       len(pack.Tables.CampaignRollups),
		"campaign_product_links": len(pack.Tables.CampaignProductLinks),
		"category_rollups":       len(pack.Tables.CategoryRollups),
		"facts_total":            len(pack.FactRegistry),
	}
}

func buildLocalFactCatalog(pack *SellerAIDataPack, factIDs []string) map[string]CompactFact {
	out := map[string]CompactFact{}
	for _, id := range factIDs {
		f, ok := pack.FactRegistry[id]
		if !ok {
			continue
		}
		fv := f.FormattedValue
		if fv == "" {
			fv = FormatValue(f.Unit, f.Value)
		}
		out[id] = CompactFact{
			FactID:         f.FactID,
			EntityID:       f.EntityID,
			EntityType:     f.EntityType,
			Metric:         f.Metric,
			Window:         f.Window,
			Value:          f.Value,
			Unit:           f.Unit,
			FormattedValue: fv,
		}
	}
	return out
}

func buildLocalEntityCatalog(pack *SellerAIDataPack, entityIDs []string) map[string]CompactEntity {
	out := map[string]CompactEntity{}
	for _, id := range entityIDs {
		if e, ok := pack.EntityRegistry.SKUs[id]; ok {
			out[id] = CompactEntity{
				EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
				SKU: e.SKU, OfferID: e.OfferID, ProductID: e.OzonProductID, CategoryID: e.CategoryID,
			}
			continue
		}
		if e, ok := pack.EntityRegistry.Campaigns[id]; ok {
			out[id] = CompactEntity{
				EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
				CampaignExternalID: e.CampaignExternalID,
			}
			continue
		}
		if e, ok := pack.EntityRegistry.Categories[id]; ok {
			out[id] = CompactEntity{
				EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name, CategoryID: e.CategoryID,
			}
			continue
		}
		if e, ok := pack.EntityRegistry.Warehouses[id]; ok {
			out[id] = CompactEntity{
				EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
			}
			continue
		}
		if e, ok := pack.EntityRegistry.SKUGroups[id]; ok {
			out[id] = CompactEntity{
				EntityID: e.EntityID, EntityType: e.EntityType, ProductID: e.ProductID,
			}
		}
	}
	return out
}

func hashAndTokenizeChunk(c *DataPackChunk) (string, int, error) {
	copy := *c
	copy.ChunkHash = ""
	copy.ApproxTokens = 0
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), (len(raw) + 3) / 4, nil
}

func compactStringIDs(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func computeChunkCoverage(pack *SellerAIDataPack, chunks []DataPackChunk) ChunkCoverage {
	return ChunkCoverage{
		SKURollups:           tableCoverage(pack.Tables.SKURollups, chunks, "sku_rollups", skuRollupKey),
		StockPositions:       tableCoverage(pack.Tables.StockPositions, chunks, "stock_positions", stockPositionKey),
		ProductEnrichment:    tableCoverage(pack.Tables.ProductEnrichment, chunks, "product_enrichment", productEnrichmentKey),
		CampaignRollups:      tableCoverage(pack.Tables.CampaignRollups, chunks, "campaign_rollups", func(r CampaignRollupRow) string { return r.CampaignExternalID }),
		CampaignProductLinks: tableCoverage(pack.Tables.CampaignProductLinks, chunks, "campaign_product_links", func(r CampaignProductLinkRow) string { return r.CampaignExternalID + ":" + r.SKU }),
		CategoryRollups:      tableCoverage(pack.Tables.CategoryRollups, chunks, "category_rollups", categoryRollupKey),
		SKUEntities:          skuEntityCoverage(pack, chunks),
		SKUGroups:            skuGroupCoverage(pack, chunks),
	}
}

func skuEntityCoverage(pack *SellerAIDataPack, chunks []DataPackChunk) ChunkTableCoverage {
	entities := make([]SKUEntity, 0, len(pack.EntityRegistry.SKUs))
	for _, e := range pack.EntityRegistry.SKUs {
		entities = append(entities, e)
	}
	return tableCoverage(entities, chunks, "sku_entities", func(e SKUEntity) string { return e.EntityID })
}

func skuGroupCoverage(pack *SellerAIDataPack, chunks []DataPackChunk) ChunkTableCoverage {
	groups := make([]SKUGroupEntity, 0, len(pack.EntityRegistry.SKUGroups))
	for _, g := range pack.EntityRegistry.SKUGroups {
		groups = append(groups, g)
	}
	return tableCoverage(groups, chunks, "sku_groups", func(g SKUGroupEntity) string { return g.EntityID })
}

func tableCoverage[T any](source []T, chunks []DataPackChunk, prefix string, keyFn func(T) string) ChunkTableCoverage {
	sourceKeys := map[string]int{}
	for _, row := range source {
		sourceKeys[keyFn(row)]++
	}
	chunkedKeys := map[string]int{}
	for _, ch := range chunks {
		if !strings.HasPrefix(ch.ChunkID, prefix+"_") {
			continue
		}
		rows, ok := ch.Rows.([]T)
		if !ok {
			continue
		}
		for _, row := range rows {
			chunkedKeys[keyFn(row)]++
		}
	}
	return buildCoverageCounts(len(source), sourceKeys, chunkedKeys)
}

func buildCoverageCounts(sourceCount int, sourceKeys, chunkedKeys map[string]int) ChunkTableCoverage {
	missing := 0
	duplicates := 0
	for key, want := range sourceKeys {
		got := chunkedKeys[key]
		if got == 0 {
			missing += want
		} else if got > want {
			duplicates += got - want
		}
	}
	chunkedCount := sourceCount - missing
	if chunkedCount < 0 {
		chunkedCount = 0
	}
	return ChunkTableCoverage{
		SourceCount:    sourceCount,
		ChunkedCount:   chunkedCount,
		MissingCount:   missing,
		DuplicateCount: duplicates,
	}
}
