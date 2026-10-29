package dashboardhome

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"

const CategorySummariesSelectionPolicyAllCatalog = "all_catalog_categories"

const DashboardSynthesisInputSchemaVersion = "dashboard_synthesis_input_v1"

type DashboardSynthesisInput struct {
	SchemaVersion             string                          `json:"schema_version"`
	MetricDate                string                          `json:"metric_date"`
	ContextSource             string                          `json:"context_source"`
	ManifestSummary           map[string]any                  `json:"manifest_summary"`
	DataQualityLimitations    []string                        `json:"data_quality_limitations"`
	ValidatedSignals          []DashboardChunkSignal          `json:"validated_signals"`
	AllowedFactIDs            []string                        `json:"allowed_fact_ids"`
	AllowedEntityIDs          []string                        `json:"allowed_entity_ids"`
	OutputSchemaHint          map[string]any                  `json:"output_schema_hint"`
	FinanceAvailable          bool                            `json:"finance_available"`
	PriceConstraintsAvailable bool                            `json:"price_constraints_available"`
	CategoryCatalog           []DashboardCategoryCatalogEntry `json:"category_catalog,omitempty"`
	CategorySelectionPolicy   string                          `json:"category_selection_policy,omitempty"`
	CategoryCatalogCount      int                             `json:"category_catalog_count,omitempty"`
	FunnelSemantics           map[string]any                  `json:"funnel_semantics,omitempty"`
	OrderSemantics            map[string]any                  `json:"order_semantics,omitempty"`
	AdvertisingMetrics        map[string]any                  `json:"advertising_metrics,omitempty"`
	PeriodComparison          map[string]any                  `json:"period_comparison,omitempty"`

	GlobalSemanticContext *aicontext.GlobalSemanticContext `json:"global_semantic_context,omitempty"`
	DataPackHash          string                           `json:"data_pack_hash,omitempty"`
	GlobalContextHash     string                           `json:"global_context_hash,omitempty"`
	PipelineRunID         *int64                           `json:"pipeline_run_id,omitempty"`
}

type DashboardCategoryCatalogEntry struct {
	EntityID     string  `json:"entity_id"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Revenue7d    float64 `json:"revenue_7d,omitempty"`
	Orders7d     int64   `json:"orders_7d,omitempty"`
}

type DashboardSynthesisValidationResult struct {
	Valid            bool
	HardErrors       []string
	RepairableErrors []string
	Repaired         bool
}
