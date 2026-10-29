package actions

type ActionType string

const (
	ActionTypeStockReplenishment   ActionType = "stock_replenishment"
	ActionTypeStockRedistribution  ActionType = "stock_redistribution"
	ActionTypeSlowTurnoverReview   ActionType = "slow_turnover_review"
	ActionTypeNoSalesWithStock     ActionType = "no_sales_with_stock"
	ActionTypeSKUOrdersDrop        ActionType = "sku_orders_drop"
	ActionTypeKeySKURisk           ActionType = "key_sku_risk"
	ActionTypeKeySKUProtection     ActionType = "key_sku_protection"
	ActionTypeAdHighDRR            ActionType = "ad_high_drr"
	ActionTypeAdHighDRRReview      ActionType = "ad_high_drr_review"
	ActionTypeAdSpendNoOrders      ActionType = "ad_spend_no_orders"
	ActionTypeAdBudgetReallocation ActionType = "ad_budget_reallocation"
	ActionTypeFunnelWeakTransition ActionType = "funnel_weak_transition"
	ActionTypeCardConversionReview ActionType = "card_conversion_review"
	ActionTypeCategoryUnderperform ActionType = "category_underperformance"
	ActionTypeSalesDropReview      ActionType = "sales_drop_review"
	ActionTypeDataQualityIssue     ActionType = "data_quality_issue"
)

// CTA keys referenced in action candidate context (handlers not implemented on step 2).
const (
	ActionCTAOpenSKUList              = "open_sku_list"
	ActionCTAOpenCampaignList         = "open_campaign_list"
	ActionCTAExportStockReplenishment = "export_stock_replenishment"
	ActionCTAExportNoSalesSKUs        = "export_no_sales_skus"
	ActionCTAExportSKUOrdersDrop      = "export_sku_orders_drop"
	ActionCTAExportHighDRRCampaigns   = "export_high_drr_campaigns"
	ActionCTAExportNoOrderCampaigns   = "export_no_order_campaigns"
	ActionCTAAskAI                    = "ask_ai"
	ActionCTAMarkChecked              = "mark_checked"
)

type ActionCategory string

const (
	ActionCategoryStock        ActionCategory = "stock"
	ActionCategorySlowTurnover ActionCategory = "slow_turnover"
	ActionCategorySKU          ActionCategory = "sku"
	ActionCategorySales        ActionCategory = "sales"
	ActionCategoryAdvertising  ActionCategory = "advertising"
	ActionCategoryFunnel       ActionCategory = "funnel"
	ActionCategoryCategory     ActionCategory = "category"
	ActionCategoryDataQuality  ActionCategory = "data_quality"
)

var LegacyCandidateActionTypes = []ActionType{
	ActionTypeStockReplenishment,
	ActionTypeNoSalesWithStock,
	ActionTypeSKUOrdersDrop,
	ActionTypeKeySKURisk,
	ActionTypeAdHighDRR,
	ActionTypeAdSpendNoOrders,
	ActionTypeFunnelWeakTransition,
}

var AIFirstAllowedActionTypes = []ActionType{
	ActionTypeStockReplenishment,
	ActionTypeStockRedistribution,
	ActionTypeSlowTurnoverReview,
	ActionTypeNoSalesWithStock,
	ActionTypeSKUOrdersDrop,
	ActionTypeKeySKUProtection,
	ActionTypeAdHighDRRReview,
	ActionTypeAdSpendNoOrders,
	ActionTypeAdBudgetReallocation,
	ActionTypeFunnelWeakTransition,
	ActionTypeCardConversionReview,
	ActionTypeCategoryUnderperform,
	ActionTypeSalesDropReview,
	ActionTypeDataQualityIssue,
}

var AllKnownActionTypes = []ActionType{
	ActionTypeStockReplenishment,
	ActionTypeStockRedistribution,
	ActionTypeSlowTurnoverReview,
	ActionTypeNoSalesWithStock,
	ActionTypeSKUOrdersDrop,
	ActionTypeKeySKURisk,
	ActionTypeKeySKUProtection,
	ActionTypeAdHighDRR,
	ActionTypeAdHighDRRReview,
	ActionTypeAdSpendNoOrders,
	ActionTypeAdBudgetReallocation,
	ActionTypeFunnelWeakTransition,
	ActionTypeCardConversionReview,
	ActionTypeCategoryUnderperform,
	ActionTypeSalesDropReview,
	ActionTypeDataQualityIssue,
}

// ActionsAIFirstActiveActionTypes is the Stage 2 active user-facing taxonomy
// for AI-first generation. It intentionally excludes:
//   - data_quality_issue (internal/future, not user-facing),
//   - key_sku_risk and ad_high_drr (legacy candidate-first only).
var ActionsAIFirstActiveActionTypes = []ActionType{
	ActionTypeStockReplenishment,
	ActionTypeStockRedistribution,
	ActionTypeSlowTurnoverReview,
	ActionTypeNoSalesWithStock,
	ActionTypeSKUOrdersDrop,
	ActionTypeKeySKUProtection,
	ActionTypeAdHighDRRReview,
	ActionTypeAdSpendNoOrders,
	ActionTypeAdBudgetReallocation,
	ActionTypeFunnelWeakTransition,
	ActionTypeCardConversionReview,
	ActionTypeCategoryUnderperform,
	ActionTypeSalesDropReview,
}

// ActionsAIFirstActiveCategories is the Stage 2 active category set for
// AI-first generation. data_quality stays internal/future only.
var ActionsAIFirstActiveCategories = []ActionCategory{
	ActionCategoryStock,
	ActionCategorySlowTurnover,
	ActionCategorySKU,
	ActionCategorySales,
	ActionCategoryAdvertising,
	ActionCategoryFunnel,
	ActionCategoryCategory,
}

var AIFirstAllowedCategories = []ActionCategory{
	ActionCategoryStock,
	ActionCategorySlowTurnover,
	ActionCategorySKU,
	ActionCategorySales,
	ActionCategoryAdvertising,
	ActionCategoryFunnel,
	ActionCategoryCategory,
	ActionCategoryDataQuality,
}

var AllKnownActionCategories = []ActionCategory{
	ActionCategoryStock,
	ActionCategorySlowTurnover,
	ActionCategorySKU,
	ActionCategorySales,
	ActionCategoryAdvertising,
	ActionCategoryFunnel,
	ActionCategoryCategory,
	ActionCategoryDataQuality,
}

type ActionPriority string

const (
	ActionPriorityCritical  ActionPriority = "critical"
	ActionPriorityImportant ActionPriority = "important"
	ActionPriorityWatch     ActionPriority = "watch"
)

type ActionStatus string

const (
	ActionStatusOpen      ActionStatus = "open"
	ActionStatusChecked   ActionStatus = "checked"
	ActionStatusSnoozed   ActionStatus = "snoozed"
	ActionStatusDismissed ActionStatus = "dismissed"
)

type ActionResponseStatus string

const (
	ActionResponseStatusAvailable    ActionResponseStatus = "available"
	ActionResponseStatusEmpty        ActionResponseStatus = "empty"
	ActionResponseStatusNotGenerated ActionResponseStatus = "not_generated"
	ActionResponseStatusPartial      ActionResponseStatus = "partial"
	ActionResponseStatusError        ActionResponseStatus = "error"
)

type ActionsResponse struct {
	MetricDate       string          `json:"metric_date,omitempty"`
	Status           string          `json:"status"`
	AIFocus          *ActionsAIFocus `json:"ai_focus,omitempty"`
	Counters         ActionCounters  `json:"counters"`
	Actions          []ActionItem    `json:"actions"`
	DataQualityNotes []string        `json:"data_quality_notes,omitempty"`
	// AnalysisSource / PublicationID / PipelineRunID are safe read-path diagnostics.
	AnalysisSource string `json:"analysis_source,omitempty"`
	PublicationID  int64  `json:"publication_id,omitempty"`
	PipelineRunID  int64  `json:"pipeline_run_id,omitempty"`
	FallbackUsed   bool   `json:"fallback_used,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

type ActionsAIFocus struct {
	Status        string   `json:"status"`
	Source        string   `json:"source"`
	Headline      string   `json:"headline"`
	Summary       string   `json:"summary"`
	TopCategories []string `json:"top_categories,omitempty"`
	Limitations   []string `json:"limitations,omitempty"`

	// Optional AI-first fields (Stage 2). Backward-compatible: absent for
	// candidate-first runs and ignored by old clients.
	StartWith     string   `json:"start_with,omitempty"`
	PriorityOrder []string `json:"priority_order,omitempty"`
}

type ActionCounters struct {
	TotalOpen   int `json:"total_open"`
	Critical    int `json:"critical"`
	Important   int `json:"important"`
	Watch       int `json:"watch"`
	Stock       int `json:"stock"`
	SKU         int `json:"sku"`
	Advertising int `json:"advertising"`
	Funnel      int `json:"funnel"`
	Sales       int `json:"sales"`
	Checked     int `json:"checked"`

	// Additive Stage 2 counters. Old clients ignore unknown fields.
	SlowTurnover  int `json:"slow_turnover"`
	CategoryCount int `json:"category_count"`
	Dismissed     int `json:"dismissed"`
}

type ActionItem struct {
	ID        int64  `json:"id"`
	ActionKey string `json:"action_key"`

	Type     ActionType     `json:"type"`
	Category ActionCategory `json:"category"`
	Priority ActionPriority `json:"priority"`
	Status   ActionStatus   `json:"status"`
	// ExecutionTier is required|recommended from unified analysis (optional for legacy).
	ExecutionTier string `json:"execution_tier,omitempty"`

	Title          string `json:"title"`
	Summary        string `json:"summary,omitempty"`
	Reason         string `json:"reason"`
	NextStep       string `json:"next_step"`
	ExpectedEffect string `json:"expected_effect,omitempty"`

	Evidence    []ActionEvidence `json:"evidence"`
	Limitations []string         `json:"limitations,omitempty"`
	CTAs        []ActionCTA      `json:"ctas"`

	AffectedEntities ActionAffectedEntities `json:"affected_entities,omitempty"`

	SourceCandidateIDs []string `json:"source_candidate_ids,omitempty"`
	EvidenceFactIDs    []string `json:"evidence_fact_ids,omitempty"`

	EntityGroupRef string `json:"entity_group_ref,omitempty"`

	OperationalDetail *OperationalDetail `json:"operational_detail,omitempty"`

	// AI-first Stage 2 fields (optional, persisted in payload_json).
	UrgencyMarker            string                       `json:"urgency_marker,omitempty"`
	AffectedObjectsSummary   string                       `json:"affected_objects_summary,omitempty"`
	AffectedObjectsTotal     int                          `json:"affected_objects_total,omitempty"`
	AffectedObjectsDisplayed int                          `json:"affected_objects_displayed,omitempty"`
	AffectedObjectsTruncated bool                         `json:"affected_objects_truncated,omitempty"`
	OperationalGoal          string                       `json:"operational_goal,omitempty"`
	ScopeType                string                       `json:"scope_type,omitempty"`
	ObjectActionsCoverage    *ActionObjectActionsCoverage `json:"object_actions_coverage,omitempty"`
	CompletionCriteria       string                       `json:"completion_criteria,omitempty"`
	CompletionConditions     []ActionCompletionCondition  `json:"completion_conditions,omitempty"`
	VisibleLimitations       []string                     `json:"visible_limitations,omitempty"`

	MetricDate string             `json:"metric_date,omitempty"`
	RunID      int64              `json:"run_id,omitempty"`
	Source     *ActionSourceTrace `json:"source,omitempty"`
	CreatedAt  string             `json:"created_at,omitempty"`
	UpdatedAt  string             `json:"updated_at,omitempty"`
	CheckedAt  *string            `json:"checked_at,omitempty"`
}

type ActionSourceTrace struct {
	PublicationID     int64  `json:"publication_id,omitempty"`
	PipelineRunID     int64  `json:"pipeline_run_id,omitempty"`
	ActionsRunID      int64  `json:"actions_run_id,omitempty"`
	DataPackHash      string `json:"data_pack_hash,omitempty"`
	GlobalContextHash string `json:"global_context_hash,omitempty"`
	PackHash          string `json:"pack_hash,omitempty"`
	MetricDate        string `json:"metric_date,omitempty"`
}

type ActionEvidence struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Unit        string `json:"unit,omitempty"`
	Direction   string `json:"direction,omitempty"`
	Description string `json:"description,omitempty"`
}

type ActionCTA struct {
	Type       string            `json:"type"`
	Label      string            `json:"label"`
	Href       string            `json:"href,omitempty"`
	ExportType string            `json:"export_type,omitempty"`
	Prompt     string            `json:"prompt,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

type ActionAffectedEntities struct {
	SKUs      []ActionSKURef      `json:"skus,omitempty"`
	Campaigns []ActionCampaignRef `json:"campaigns,omitempty"`
	Funnel    *ActionFunnelRef    `json:"funnel,omitempty"`
}

type ActionSKURef struct {
	SKU      string `json:"sku"`
	OfferID  string `json:"offer_id,omitempty"`
	Name     string `json:"name,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type ActionCampaignRef struct {
	CampaignID string `json:"campaign_id"`
	Name       string `json:"name,omitempty"`
}

type ActionFunnelRef struct {
	TransitionKey string `json:"transition_key"`
	FromStage     string `json:"from_stage"`
	ToStage       string `json:"to_stage"`
}

// ActionEntityGroup is a persisted full object list for an action.
type ActionEntityGroup struct {
	GroupKey   string
	EntityType string
	EntityIDs  []string
	Summary    map[string]any
}

// ExportEntityContext carries resolved entity group data for export renderers.
type ExportEntityContext struct {
	EntityType      string
	EntityIDs       []string
	Summary         map[string]any
	Source          string
	MissingFullList bool
	Warning         string
}

type ListActionsFilters struct {
	Status     string
	Category   *ActionCategory
	Priority   *ActionPriority
	MetricDate string
	Limit      int
	Offset     int
}

type AdminListGenerationRunsFilters struct {
	SellerAccountID *int64
	Status          string
	FinalStatus     string
	MetricDate      string
	Limit           int
	Offset          int
}

type AdminGenerationRun struct {
	RunID                int64  `json:"run_id"`
	SellerAccountID      int64  `json:"seller_account_id"`
	MetricDate           string `json:"metric_date,omitempty"`
	Status               string `json:"status"`
	FinalStatus          string `json:"final_status,omitempty"`
	ContextHash          string `json:"context_hash,omitempty"`
	AcceptedCount        int    `json:"accepted_count"`
	RepairedCount        int    `json:"repaired_count"`
	DroppedCount         int    `json:"dropped_count"`
	FallbackCount        int    `json:"fallback_count"`
	CreatedAt            string `json:"created_at,omitempty"`
	UpdatedAt            string `json:"updated_at,omitempty"`
	AIFocusJSON          []byte `json:"ai_focus_json,omitempty"`
	TopActionKeysJSON    []byte `json:"top_action_keys_json,omitempty"`
	DataQualityNotesJSON []byte `json:"data_quality_notes_json,omitempty"`
	AiPipelineRunID      int64  `json:"ai_pipeline_run_id,omitempty"`
	DataPackHash         string `json:"data_pack_hash,omitempty"`
	GlobalContextHash    string `json:"global_context_hash,omitempty"`
}

type AdminGenerationCategoryRun struct {
	Domain              string `json:"domain"`
	Status              string `json:"status"`
	Model               string `json:"model,omitempty"`
	AcceptedCount       int    `json:"accepted_count"`
	RepairedCount       int    `json:"repaired_count"`
	DroppedCount        int    `json:"dropped_count"`
	ValidationJSON      []byte `json:"validation_json,omitempty"`
	FinalValidationJSON []byte `json:"final_validation_json,omitempty"`
	RawResponseJSON     []byte `json:"raw_response_json,omitempty"`
	RepairResponseJSON  []byte `json:"repair_response_json,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}

const emptyActionsNote = "Критичных действий по доступным данным на эту дату не найдено."
const notGeneratedActionsNote = "Действия ещё не сформированы."
