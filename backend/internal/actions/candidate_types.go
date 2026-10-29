package actions

import "time"

const (
	ActionDomainStock       = "stock"
	ActionDomainSKUSales    = "sku_sales"
	ActionDomainAdvertising = "advertising"
	ActionDomainFunnel      = "funnel"

	DomainStatusAvailable          = "available"
	DomainStatusEmpty              = "empty"
	DomainStatusPartial            = "partial"
	DomainStatusSourceNotAvailable = "source_not_available"
	DomainStatusError              = "error"
)

// DefaultForbiddenClaims are guardrails passed to downstream AI composers.
var DefaultForbiddenClaims = []string{
	"Не давать ценовые рекомендации.",
	"Не считать маржинальность.",
	"Не делать SKU-level рекламные выводы.",
	"Не обещать точный финансовый эффект.",
	"Не добавлять SKU, кампании или метрики, которых нет в context.",
	"Не использовать отсутствующие строки метрик как нулевые продажи.",
}

// DefaultAllowedActionTypes lists candidate types eligible for generation on step 2.
var DefaultAllowedActionTypes = LegacyCandidateActionTypes

type ActionCandidateContext struct {
	SellerAccountID    int64                   `json:"seller_account_id"`
	MetricDate         string                  `json:"metric_date"`
	AccountSnapshot    AccountSnapshot         `json:"account_snapshot"`
	Domains            []ActionDomainContext   `json:"domains"`
	AllowedActionTypes []ActionType            `json:"allowed_action_types"`
	ForbiddenClaims    []string                `json:"forbidden_claims"`
	DataQualityNotes   []string                `json:"data_quality_notes,omitempty"`
	CoverageReport     CandidateCoverageReport `json:"coverage_report"`
}

type AccountSnapshot struct {
	Revenue         *float64 `json:"revenue,omitempty"`
	Orders          *int64   `json:"orders,omitempty"`
	AOV             *float64 `json:"aov,omitempty"`
	DRR             *float64 `json:"drr,omitempty"`
	RevenueDeltaPct *float64 `json:"revenue_delta_pct,omitempty"`
	OrdersDeltaPct  *float64 `json:"orders_delta_pct,omitempty"`
	HealthScore     *int     `json:"health_score,omitempty"`
}

type ActionDomainContext struct {
	Domain           string            `json:"domain"`
	Status           string            `json:"status"`
	Candidates       []ActionCandidate `json:"candidates"`
	DataQualityNotes []string          `json:"data_quality_notes,omitempty"`
}

type ActionCandidate struct {
	CandidateID  string         `json:"candidate_id"`
	Domain       string         `json:"domain"`
	Type         ActionType     `json:"type"`
	Category     ActionCategory `json:"category"`
	PriorityHint ActionPriority `json:"priority_hint"`

	Summary     string                   `json:"summary"`
	Aggregates  map[string]any           `json:"aggregates"`
	Facts       []ActionCandidateFact    `json:"facts"`
	TopExamples []ActionCandidateExample `json:"top_examples"`

	EntityGroupRef *ActionEntityGroupRef `json:"entity_group_ref,omitempty"`

	AllowedCTAs []string `json:"allowed_ctas"`
	Limitations []string `json:"limitations,omitempty"`
	SourceRefs  []string `json:"source_refs,omitempty"`
}

type ActionCandidateFact struct {
	FactID      string `json:"fact_id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
}

type ActionCandidateExample struct {
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Name       string         `json:"name,omitempty"`
	Metrics    map[string]any `json:"metrics,omitempty"`
}

type ActionEntityGroupRef struct {
	GroupKey   string         `json:"group_key"`
	EntityType string         `json:"entity_type"`
	Count      int            `json:"count"`
	Summary    map[string]any `json:"summary,omitempty"`
}

type CandidateCoverageReport struct {
	TotalSKUsScanned           int `json:"total_skus_scanned"`
	TotalCampaignsScanned      int `json:"total_campaigns_scanned"`
	StockCandidatesFound       int `json:"stock_candidates_found"`
	SKUSalesCandidatesFound    int `json:"sku_sales_candidates_found"`
	AdvertisingCandidatesFound int `json:"advertising_candidates_found"`
	FunnelCandidatesFound      int `json:"funnel_candidates_found"`
}

type ActionEntityGroupRecord struct {
	ID              int64
	SellerAccountID int64
	MetricDate      *time.Time
	GroupKey        string
	EntityType      string
	EntityIDsJSON   []byte
	SummaryJSON     []byte
}
