package aicontext

const GlobalSemanticContextSchemaVersion = "global_semantic_context_v1"

// GlobalSemanticContext is the shared semantic frame for Dashboard and Actions.
// It is not shown to sellers directly.
type GlobalSemanticContext struct {
	SchemaVersion string `json:"schema_version"`

	SellerAccountID int64  `json:"seller_account_id,omitempty"`
	MetricDate      string `json:"metric_date,omitempty"`

	DataPackHash string `json:"data_pack_hash,omitempty"`

	OverallStory string `json:"overall_story"`

	PrimaryRisks   []SemanticRisk   `json:"primary_risks,omitempty"`
	SecondaryRisks []SemanticRisk   `json:"secondary_risks,omitempty"`
	Opportunities  []SemanticSignal `json:"opportunities,omitempty"`

	PriorityOrder []string `json:"priority_order,omitempty"`

	DataLimitations FlexibleStringSlice `json:"data_limitations,omitempty"`

	Confidence string `json:"confidence,omitempty"`

	SourceFactIDs []string `json:"source_fact_ids,omitempty"`
}

type SemanticRisk struct {
	RiskID string `json:"risk_id"`

	Category string `json:"category"`
	Severity string `json:"severity"`

	Title string `json:"title"`

	Rationale string `json:"rationale,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids,omitempty"`

	Entities []SemanticEntityRef `json:"entities,omitempty"`
}

type SemanticSignal struct {
	SignalID string `json:"signal_id"`

	Category string `json:"category"`

	Title string `json:"title"`

	Rationale string `json:"rationale,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids,omitempty"`

	Entities []SemanticEntityRef `json:"entities,omitempty"`
}

type SemanticEntityRef struct {
	EntityType string   `json:"entity_type"`
	EntityID   string   `json:"entity_id"`
	Name       string   `json:"name,omitempty"`
	FactIDs    []string `json:"fact_ids,omitempty"`
}
