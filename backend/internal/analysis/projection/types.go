package projection

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type Input struct {
	SellerID     int64
	AnalysisDate time.Time
	CutoffAt     time.Time
	UniverseHash string
	Sealed       factualapi.FactualReadContext
	API          factualapi.API
	Validated    deep.ValidatedOutputV1
	Plan         actionreconcile.PlanV1
}

type Built struct {
	Manifest     Manifest
	Overview     OverviewV1DTO
	Funnel       FunnelV1DTO
	Stock        DomainPageDTO
	Advertising  DomainPageDTO
	Actions      ActionsSnapshotDTO
	Chat         ChatContextDTO
	StockRows    []map[string]any
	CampaignRows []map[string]any
	FocusRows    []map[string]any
	Bytes        map[string][]byte
	QueryCounts  map[string]int
}

const (
	PolicyV1           = "projection_policy_v1"
	PolicyV2           = "projection_policy_v2"
	OverviewV1         = "overview_projection_v1"
	OverviewV2         = "overview_projection_v2"
	FunnelV1           = "funnel_projection_v1"
	FunnelV2           = "funnel_projection_v2"
	StockV1            = "stock_projection_v1"
	StockV2            = "stock_projection_v2"
	AdvertisingV1      = "advertising_projection_v1"
	AdvertisingV2      = "advertising_projection_v2"
	ActionsSnapshotV1  = "actions_snapshot_projection_v1"
	ActionsSnapshotV2  = "actions_snapshot_projection_v2"
	ChatContextV1      = "chat_context_projection_v1"
	ManifestV1         = "publication_projection_manifest_v1"
	ManifestV2         = "publication_projection_manifest_v2"
	StockSKURowsV1     = "stock_sku_rows_v1"
	StockSKURowsV2     = "stock_sku_rows_v2"
	AdCampaignRowsV1   = "ad_campaign_rows_v1"
	AdCampaignRowsV2   = "ad_campaign_rows_v2"
	SKUFocusRowsV1     = "sku_focus_rows_v1"
	SKUFocusRowsV2     = "sku_focus_rows_v2"
	KeyOverview        = "OVERVIEW"
	KeyFunnel          = "FUNNEL"
	KeyStock           = "STOCK"
	KeyAdvertising     = "ADVERTISING"
	KeyActionsSnapshot = "ACTIONS_SNAPSHOT"
	KeyChatContext     = "CHAT_CONTEXT"
	KeyStockSKURows    = "STOCK_SKU_ROWS"
	KeyAdCampaignRows  = "AD_CAMPAIGN_ROWS"
	KeySKUFocusRows    = "SKU_FOCUS_ROWS"
)

type Manifest struct {
	SchemaVersion string            `json:"schema_version"`
	PolicyVersion string            `json:"policy_version"`
	Hashes        map[string]string `json:"hashes"`
	QueryCounts   map[string]int    `json:"query_counts"`
}

type KPI struct {
	Key        string       `json:"key"`
	Status     string       `json:"status"`
	Amount     *string      `json:"amount"`
	Unit       string       `json:"unit,omitempty"`
	Current    *string      `json:"current,omitempty"`
	Previous   *string      `json:"previous,omitempty"`
	Delta      *string      `json:"delta,omitempty"`
	DeltaUnit  string       `json:"delta_unit,omitempty"`
	ReasonCode string       `json:"reason_code,omitempty"`
	Trend      []TrendPoint `json:"trend,omitempty"`
}

type TrendPoint struct {
	Date   string  `json:"date"`
	Amount *string `json:"amount"`
	State  string  `json:"state"`
}

type PeriodWindow struct {
	Key  string `json:"key"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type FindingView struct {
	Ref            string `json:"ref,omitempty"`
	Title          string `json:"title"`
	Statement      string `json:"statement"`
	PrimaryDomain  string `json:"primary_domain"`
	OverviewBucket string `json:"overview_bucket"`
	EntityKind     string `json:"entity_kind,omitempty"`
	EntityID       string `json:"entity_id,omitempty"`
}

type TakeawayView struct {
	Statement     string `json:"statement"`
	PrimaryDomain string `json:"primary_domain"`
}

type OverviewV1DTO struct {
	SchemaVersion       string           `json:"schema_version"`
	SellerAccountID     int64            `json:"seller_account_id"`
	AnalysisDate        string           `json:"analysis_date"`
	AnalysisCutoffAt    string           `json:"analysis_cutoff_at"`
	FactualUniverseHash string           `json:"factual_universe_hash"`
	Periods             []PeriodWindow   `json:"periods,omitempty"`
	KPIs                []KPI            `json:"kpis"`
	ExecutiveHeadline   string           `json:"executive_headline,omitempty"`
	ExecutiveSummary    string           `json:"executive_summary"`
	Takeaways           []TakeawayView   `json:"takeaways,omitempty"`
	Findings            []FindingView    `json:"findings,omitempty"`
	Recommendations     []string         `json:"recommendation_refs"`
	TopActions          []string         `json:"top_action_local_refs"`
	SKUFocusPreview     []map[string]any `json:"sku_focus_preview,omitempty"`
	Limitations         []string         `json:"limitations,omitempty"`
}

type FunnelV1DTO struct {
	SchemaVersion  string             `json:"schema_version"`
	State          string             `json:"state"`
	ReasonCode     string             `json:"reason_code,omitempty"`
	Status         string             `json:"status"`
	Period         FunnelPeriods      `json:"period"`
	DataAsOf       string             `json:"data_as_of,omitempty"`
	Stages         []FunnelStage      `json:"stages"`
	Transitions    []FunnelTransition `json:"transitions"`
	Insight        *FunnelInsight     `json:"insight,omitempty"`
	Limitations    []string           `json:"limitations"`
	RelatedActions []string           `json:"related_action_local_refs"`
}

type FunnelPeriods struct {
	Current  PeriodWindow `json:"current"`
	Previous PeriodWindow `json:"previous"`
}

type FunnelStage struct {
	Key            string  `json:"key"`
	Status         string  `json:"status"`
	Amount         *string `json:"amount"`
	PreviousAmount *string `json:"previous_amount,omitempty"`
	Delta          *string `json:"delta,omitempty"`
	ReasonCode     string  `json:"reason_code,omitempty"`
}

type FunnelTransition struct {
	FromKey string  `json:"from_key"`
	ToKey   string  `json:"to_key"`
	Key     string  `json:"key"`
	Status  string  `json:"status"`
	Amount  *string `json:"amount,omitempty"`
}

type FunnelInsight struct {
	Headline      string `json:"headline"`
	Statement     string `json:"statement"`
	PrimaryDomain string `json:"primary_domain"`
}

type CoverageBandCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	State string `json:"state"`
}

type DomainPageDTO struct {
	SchemaVersion         string              `json:"schema_version"`
	State                 string              `json:"state,omitempty"`
	ReasonCode            string              `json:"reason_code,omitempty"`
	DataAsOf              string              `json:"data_as_of,omitempty"`
	DataThrough           string              `json:"data_through,omitempty"`
	KPIs                  []KPI               `json:"kpis,omitempty"`
	CoverageBands         []CoverageBandCount `json:"coverage_bands,omitempty"`
	AccountAdSpendShare   *KPI                `json:"account_ad_spend_share,omitempty"`
	YesterdayAdSpendShare *KPI                `json:"yesterday_ad_spend_share,omitempty"`
	FindingRefs           []string            `json:"finding_refs"`
	Findings              []FindingView       `json:"findings,omitempty"`
	RecommendationRefs    []string            `json:"recommendation_refs"`
	ActionLocalRefs       []string            `json:"action_local_refs"`
	GroupLocalRefs        []string            `json:"group_local_refs"`
	Limitations           []string            `json:"limitations,omitempty"`
}

type ActionsSnapshotDTO struct {
	SchemaVersion string                     `json:"schema_version"`
	Groups        []actionreconcileGroupView `json:"groups"`
	Singles       []string                   `json:"single_action_local_refs"`
}

type actionreconcileGroupView struct {
	LocalRef                string   `json:"local_ref"`
	MemberRefs              []string `json:"member_local_refs"`
	DisplayOrder            int      `json:"display_order"`
	DisplayTitle            string   `json:"display_title,omitempty"`
	DisplaySummary          string   `json:"display_summary,omitempty"`
	ActionType              string   `json:"action_type,omitempty"`
	Family                  string   `json:"family,omitempty"`
	PresentationTemplateKey string   `json:"presentation_template_key,omitempty"`
}

type ChatContextDTO struct {
	SchemaVersion      string   `json:"schema_version"`
	ExecutiveSummary   string   `json:"executive_summary"`
	FindingRefs        []string `json:"finding_refs"`
	RecommendationRefs []string `json:"recommendation_refs"`
	ActionLocalRefs    []string `json:"action_local_refs"`
	Limitations        []string `json:"limitations"`
	FactHandles        []string `json:"fact_handles"`
	AnalysisDate       string   `json:"analysis_date"`
}
