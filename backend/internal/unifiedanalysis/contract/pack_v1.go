package contract

// ContractDateRange is an inclusive YYYY-MM-DD range used in pack JSON docs.
type ContractDateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// AnalysisPackPeriodsV1 describes the analysis periods in manifest.json.
type AnalysisPackPeriodsV1 struct {
	Current7D  ContractDateRange `json:"current_7d"`
	Previous7D ContractDateRange `json:"previous_7d"`
	Trend      ContractDateRange `json:"trend"`
}

// AnalysisPackEntityCountsV1 summarizes entity coverage in the pack.
type AnalysisPackEntityCountsV1 struct {
	SKUCount      int `json:"sku_count"`
	CampaignCount int `json:"campaign_count"`
	CategoryCount int `json:"category_count"`
}

// AnalysisPackDiagnosticsV1 carries optional internal diagnostics for demos/debug.
// Omitted from older packs; never required by technical validator.
type AnalysisPackDiagnosticsV1 struct {
	TotalDiscoveredSKU    int  `json:"total_discovered_sku"`
	AnalysisEligibleSKU   int  `json:"analysis_eligible_sku"`
	ExcludedTechnicalSKU  int  `json:"excluded_technical_sku"`
	ExcludedHistoricalSKU int  `json:"excluded_historical_sku"`
	ExcludedIncompleteSKU int  `json:"excluded_incomplete_sku"`
	ExcludedArchivedSKU   int  `json:"excluded_archived_sku"`
	BusinessConsistencyOK bool `json:"business_consistency_ok"`
	BusinessErrorCount    int  `json:"business_error_count"`
	BusinessWarningCount  int  `json:"business_warning_count"`
}

// AnalysisPackTableManifestV1 describes one table/file entry in manifest.json.
type AnalysisPackTableManifestV1 struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Required bool     `json:"required"`
	RowCount int      `json:"row_count"`
	Columns  []string `json:"columns"`
	SHA256   string   `json:"sha256"`
}

// AnalysisPackManifestV1 is the contract for future pack manifest.json.
// Hash computation and ZIP writing are deferred to later stages.
type AnalysisPackManifestV1 struct {
	SchemaVersion   string                        `json:"schema_version"`
	PackID          string                        `json:"pack_id"`
	SellerAccountID int64                         `json:"seller_account_id"`
	MetricDate      string                        `json:"metric_date"`
	Timezone        string                        `json:"timezone"`
	Currency        string                        `json:"currency"`
	Periods         AnalysisPackPeriodsV1         `json:"periods"`
	Tables          []AnalysisPackTableManifestV1 `json:"tables"`
	EntityCounts    AnalysisPackEntityCountsV1    `json:"entity_counts"`
	DataStatus      string                        `json:"data_status"`
	Diagnostics     *AnalysisPackDiagnosticsV1    `json:"diagnostics,omitempty"`
}

// AnalysisPolicyV1 is the contract for analysis_policy.json.
type AnalysisPolicyV1 struct {
	SchemaVersion string `json:"schema_version"`

	MaxActions    int `json:"max_actions"`
	MaxTopActions int `json:"max_top_actions"`

	AllowedActionTypes      []string `json:"allowed_action_types"`
	AllowedActionCategories []string `json:"allowed_action_categories"`
	AllowedPriorities       []string `json:"allowed_priorities"`

	GroupSimilarObjects     bool `json:"group_similar_objects"`
	AvoidSemanticDuplicates bool `json:"avoid_semantic_duplicates"`
	DoNotFillActionQuota    bool `json:"do_not_fill_action_quota"`
	DashboardActionsLinked  bool `json:"dashboard_actions_linked"`
	UseOnlyPackData         bool `json:"use_only_pack_data"`
}

// DefaultAnalysisPolicyV1 returns the stage-1 policy defaults.
func DefaultAnalysisPolicyV1() AnalysisPolicyV1 {
	return AnalysisPolicyV1{
		SchemaVersion:           AnalysisPackSchemaVersionV1,
		MaxActions:              MaxActionsV1,
		MaxTopActions:           MaxTopActionsV1,
		AllowedActionTypes:      append([]string(nil), AllowedActionTypesV1...),
		AllowedActionCategories: append([]string(nil), AllowedActionCategoriesV1...),
		AllowedPriorities:       append([]string(nil), AllowedActionPrioritiesV1...),
		GroupSimilarObjects:     true,
		AvoidSemanticDuplicates: true,
		DoNotFillActionQuota:    true,
		DashboardActionsLinked:  true,
		UseOnlyPackData:         true,
	}
}

// DataDictionaryFieldV1 describes one metric/field in data_dictionary.json.
type DataDictionaryFieldV1 struct {
	Type          string `json:"type"`
	Unit          string `json:"unit,omitempty"`
	Description   string `json:"description"`
	Formula       string `json:"formula,omitempty"`
	Period        string `json:"period,omitempty"`
	Nullable      bool   `json:"nullable"`
	NullSemantics string `json:"null_semantics,omitempty"`
	Source        string `json:"source,omitempty"`
}

// DataDictionaryV1 is the contract for data_dictionary.json.
// Full field catalog is filled in stage 2; stage 1 ships representative examples.
type DataDictionaryV1 struct {
	SchemaVersion string                           `json:"schema_version"`
	Fields        map[string]DataDictionaryFieldV1 `json:"fields"`
}

// ExampleDataDictionaryV1 returns representative dictionary entries for fixtures.
func ExampleDataDictionaryV1() DataDictionaryV1 {
	return DataDictionaryV1{
		SchemaVersion: AnalysisPackSchemaVersionV1,
		Fields: map[string]DataDictionaryFieldV1{
			"orders_7d": {
				Type:          "integer",
				Unit:          "count",
				Description:   "Orders in the current 7-day window inclusive of metric_date.",
				Period:        "last_7d",
				Nullable:      false,
				NullSemantics: "Never null; zero means no orders observed.",
				Source:        "daily_account_metrics / daily_sku_metrics",
			},
			"revenue_7d": {
				Type:        "number",
				Unit:        "RUB",
				Description: "Revenue in the current 7-day window.",
				Period:      "last_7d",
				Nullable:    false,
				Source:      "daily_account_metrics / daily_sku_metrics",
			},
			"drr_pct": {
				Type:          "number",
				Unit:          "percent",
				Description:   "Ad spend / ad revenue * 100 for the period.",
				Formula:       "spend / revenue * 100 when revenue > 0",
				Nullable:      true,
				NullSemantics: "Null when revenue is zero or unavailable.",
				Source:        "ad_metrics_daily",
			},
			"stock_coverage_days_30d": {
				Type:          "number",
				Unit:          "days",
				Description:   "Stock coverage using 30-day average daily sales.",
				Nullable:      true,
				NullSemantics: "May be unavailable while snapshot history is only 14 days (stage-2 open question).",
				Source:        "stock_analytics_snapshots + sales history",
			},
		},
	}
}

// LimitationV1 describes one limitation entry.
type LimitationV1 struct {
	Code           string   `json:"code"`
	Severity       string   `json:"severity"`
	Description    string   `json:"description"`
	AffectedTables []string `json:"affected_tables,omitempty"`
}

// DatasetCoverageV1 describes presence gaps for one Analysis Pack dataset.
type DatasetCoverageV1 struct {
	Dataset   string   `json:"dataset"`
	Status    string   `json:"status"` // present|missing|partial|sparse
	DatesWith int      `json:"dates_with_data,omitempty"`
	GapDates  []string `json:"gap_dates,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// LimitationsDocumentV1 is the contract for limitations.json.
type LimitationsDocumentV1 struct {
	SchemaVersion string              `json:"schema_version"`
	Limitations   []LimitationV1      `json:"limitations"`
	Coverage      []DatasetCoverageV1 `json:"coverage,omitempty"`
}
