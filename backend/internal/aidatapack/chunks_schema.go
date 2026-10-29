package aidatapack

const ChunkSetSchemaVersion = "ai_data_pack_chunks_v1"

const (
	SelectionPolicyTechnical = "technical_chunking_only_no_business_filtering"
	OrderingPolicyStableAsc  = "stable_key_asc"
	ChunkingPolicyName       = "deterministic_table_split"
)

// ChunkOptions controls deterministic technical chunking.
type ChunkOptions struct {
	MaxRowsPerChunk           int
	MaxApproxTokensPerChunk   int
	IncludeLocalFactCatalog   bool
	IncludeLocalEntityCatalog bool
}

// DefaultChunkOptions returns MVP chunking defaults.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		MaxRowsPerChunk:           50,
		MaxApproxTokensPerChunk:   12000,
		IncludeLocalFactCatalog:   true,
		IncludeLocalEntityCatalog: true,
	}
}

type ChunkSet struct {
	SchemaVersion         string               `json:"schema_version"`
	DataPackSchemaVersion string               `json:"data_pack_schema_version"`
	SellerAccountID       int64                `json:"seller_account_id"`
	MetricDate            string               `json:"metric_date"`
	ChunkingPolicy        ChunkingPolicy       `json:"chunking_policy"`
	Manifest              ChunkManifestSummary `json:"manifest"`
	Chunks                []DataPackChunk      `json:"chunks"`
	Coverage              ChunkCoverage        `json:"coverage"`
}

type ChunkingPolicy struct {
	Policy                   string `json:"policy"`
	SelectionPolicy          string `json:"selection_policy"`
	OrderingPolicy           string `json:"ordering_policy"`
	BusinessFilteringAllowed bool   `json:"business_filtering_allowed"`
}

type ChunkManifestSummary struct {
	ChunkCount        int            `json:"chunk_count"`
	DatasetCounts     map[string]int `json:"dataset_counts"`
	ChunkIDs          []string       `json:"chunk_ids"`
	TotalApproxTokens int            `json:"total_approx_tokens,omitempty"`
}

type DataPackChunk struct {
	ChunkID            string                   `json:"chunk_id"`
	ChunkType          string                   `json:"chunk_type"`
	SourcePath         string                   `json:"source_path"`
	SourceTable        string                   `json:"source_table,omitempty"`
	SchemaVersion      string                   `json:"schema_version"`
	SellerAccountID    int64                    `json:"seller_account_id"`
	MetricDate         string                   `json:"metric_date"`
	Ordering           string                   `json:"ordering"`
	SelectionPolicy    string                   `json:"selection_policy"`
	RowCount           int                      `json:"row_count"`
	Rows               any                      `json:"rows,omitempty"`
	Payload            any                      `json:"payload,omitempty"`
	LocalFactCatalog   map[string]CompactFact   `json:"local_fact_catalog,omitempty"`
	LocalEntityCatalog map[string]CompactEntity `json:"local_entity_catalog,omitempty"`
	Limitations        []string                 `json:"limitations,omitempty"`
	ChunkHash          string                   `json:"chunk_hash"`
	ApproxTokens       int                      `json:"approx_tokens,omitempty"`
}

type CompactFact struct {
	FactID         string `json:"fact_id"`
	EntityID       string `json:"entity_id,omitempty"`
	EntityType     string `json:"entity_type,omitempty"`
	Metric         string `json:"metric,omitempty"`
	Window         string `json:"window,omitempty"`
	Value          any    `json:"value,omitempty"`
	Unit           string `json:"unit,omitempty"`
	FormattedValue string `json:"formatted_value,omitempty"`
}

type CompactEntity struct {
	EntityID           string `json:"entity_id"`
	EntityType         string `json:"entity_type"`
	Name               string `json:"name,omitempty"`
	SKU                string `json:"sku,omitempty"`
	OfferID            string `json:"offer_id,omitempty"`
	ProductID          string `json:"product_id,omitempty"`
	CampaignExternalID string `json:"campaign_external_id,omitempty"`
	CategoryID         string `json:"category_id,omitempty"`
}

type ChunkCoverage struct {
	SKURollups           ChunkTableCoverage `json:"sku_rollups"`
	StockPositions       ChunkTableCoverage `json:"stock_positions"`
	ProductEnrichment    ChunkTableCoverage `json:"product_enrichment"`
	CampaignRollups      ChunkTableCoverage `json:"campaign_rollups"`
	CampaignProductLinks ChunkTableCoverage `json:"campaign_product_links"`
	CategoryRollups      ChunkTableCoverage `json:"category_rollups"`
	SKUEntities          ChunkTableCoverage `json:"sku_entities"`
	SKUGroups            ChunkTableCoverage `json:"sku_groups"`
}

type ChunkTableCoverage struct {
	SourceCount    int `json:"source_count"`
	ChunkedCount   int `json:"chunked_count"`
	MissingCount   int `json:"missing_count"`
	DuplicateCount int `json:"duplicate_count"`
}
