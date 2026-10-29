package actioncatalog

import (
	"encoding/json"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

type EntityRef = discovery.PackEntity

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

func (m *Money) UnmarshalJSON(b []byte) error {
	type wire struct {
		Amount   json.RawMessage `json:"amount"`
		Currency string          `json:"currency"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	m.Currency = w.Currency
	if len(w.Amount) == 0 {
		return nil
	}
	if err := json.Unmarshal(w.Amount, &m.Amount); err == nil {
		return nil
	}
	var s string
	if err := json.Unmarshal(w.Amount, &s); err != nil {
		return err
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	m.Amount = f
	return nil
}

// Parameters is the closed Action Catalog V1 parameter superset.
// Unknown JSON keys are rejected by Structured Output; per-type validators
// require the relevant fields and forbid the rest.
type Parameters struct {
	RecommendedPrice            *Money   `json:"recommended_price"`
	RecommendedUnits            *int     `json:"recommended_units"`
	RecommendedUnitsReduction   *int     `json:"recommended_units_reduction"`
	CampaignRef                 *string  `json:"campaign_ref"`
	RecommendedBid              *Money   `json:"recommended_bid"`
	DesiredState                *string  `json:"desired_state"`
	PromotionRef                *string  `json:"promotion_ref"`
	RecommendedActionPrice      *Money   `json:"recommended_action_price"`
	PrimaryRatingKey            *string  `json:"primary_rating_key"`
	RelatedRatingKeys           []string `json:"related_rating_keys"`
	ProviderErrorTypes          []string `json:"provider_error_types"`
	Direction                   *string  `json:"direction"`
	AffectedMaterialSKURefs     []string `json:"affected_material_sku_refs"`
	Intent                      *string  `json:"intent"`
	RelatedMaterialCampaignRefs []string `json:"related_material_campaign_refs"`
	RecommendedBudget           *Money   `json:"recommended_budget"`
	BudgetPeriod                *string  `json:"budget_period"`
	DesiredStrategy             *string  `json:"desired_strategy"`
	AddSKURefs                  []string `json:"add_sku_refs"`
	RemoveSKURefs               []string `json:"remove_sku_refs"`
}

type Definition struct {
	ActionTypeKey          string
	Version                string
	TargetKind             discovery.EntityKind
	ActionFamily           string
	SourceDomain           string
	AllowedRelatedEntities []string
	RequiredCapabilities   []string
	RequiredFactualContext []string
	ConflictGroup          string
	ExecutionSupport       string
	ExecutionRisk          string
	ValidationPolicy       string
}

type Proposal struct {
	ActionType string
	Target     EntityRef
	Parameters Parameters
}

type MenuItem struct {
	ActionType               string   `json:"action_type"`
	TargetKind               string   `json:"target_kind"`
	EligibleTargetIDs        []string `json:"eligible_target_ids"`
	ParameterContract        string   `json:"parameter_contract"`
	AllowedRelatedEntities   []string `json:"allowed_related_entities"`
	EligibleRelatedEntityIDs []string `json:"eligible_related_entity_ids,omitempty"`
	HardConstraints          []string `json:"hard_constraints"`
	OmitReason               string   `json:"-"`
}

type Menu struct {
	Version string     `json:"version"`
	Catalog string     `json:"action_catalog_version"`
	Items   []MenuItem `json:"items"`
}

type MainEntity struct {
	Ref EntityRef
}

// Snapshot is run-specific factual eligibility (not a recommendation).
type Snapshot struct {
	Capabilities map[string]string // key → readiness
	SKU          map[string]SKUState
	Category     map[string]CategoryState
	Campaign     map[string]CampaignState
	Account      AccountState
	MainIDs      map[string]bool // kind|id
}

type SKUState struct {
	CurrentPrice            *Money
	MinAllowedPrice         *float64
	HasPrice                bool
	HasStockContext         bool
	HasDemandContext        bool
	HasReplenishmentContext bool
	CatalogState            string // ACTIVE | ARCHIVED
	CampaignRefs            []string
	BidKnownForCampaign     map[string]bool
	PromotionCandidates     []string
	PromotionParticipants   []string
	PromotionActionPrice    map[string]*Money
}

type CategoryState struct {
	MaterialSKURefs      []string
	MaterialCampaignRefs []string
}

type CampaignState struct {
	State             string // ACTIVE | PAUSED
	Budget            *Money
	AllowedStrategies []string
	CurrentStrategy   string
	ProductSKURefs    []string
}

type AccountState struct {
	RatingKeys []string
	ErrorTypes []string
}
