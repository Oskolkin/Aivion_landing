package actioncatalog

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type moneyWire struct {
	Amount   canonicaljson.Decimal `json:"amount"`
	Currency string                `json:"currency"`
}

// ParametersWire is the canonical JSON wire form of action parameters.
type ParametersWire struct {
	RecommendedPrice            *moneyWire `json:"recommended_price"`
	RecommendedUnits            *int       `json:"recommended_units"`
	RecommendedUnitsReduction   *int       `json:"recommended_units_reduction"`
	CampaignRef                 *string    `json:"campaign_ref"`
	RecommendedBid              *moneyWire `json:"recommended_bid"`
	DesiredState                *string    `json:"desired_state"`
	PromotionRef                *string    `json:"promotion_ref"`
	RecommendedActionPrice      *moneyWire `json:"recommended_action_price"`
	PrimaryRatingKey            *string    `json:"primary_rating_key"`
	RelatedRatingKeys           []string   `json:"related_rating_keys"`
	ProviderErrorTypes          []string   `json:"provider_error_types"`
	Direction                   *string    `json:"direction"`
	AffectedMaterialSKURefs     []string   `json:"affected_material_sku_refs"`
	Intent                      *string    `json:"intent"`
	RelatedMaterialCampaignRefs []string   `json:"related_material_campaign_refs"`
	RecommendedBudget           *moneyWire `json:"recommended_budget"`
	BudgetPeriod                *string    `json:"budget_period"`
	DesiredStrategy             *string    `json:"desired_strategy"`
	AddSKURefs                  []string   `json:"add_sku_refs"`
	RemoveSKURefs               []string   `json:"remove_sku_refs"`
}

func moneyToWire(m *Money) *moneyWire {
	if m == nil {
		return nil
	}
	return &moneyWire{Amount: decimalFromFloat(m.Amount), Currency: m.Currency}
}

func decimalFromFloat(f float64) canonicaljson.Decimal {
	return canonicaljson.Decimal(strconv.FormatFloat(f, 'f', -1, 64))
}

// ParametersWireFrom converts catalog parameters for canonical JSON storage.
func ParametersWireFrom(p Parameters) ParametersWire {
	return ParametersWire{
		RecommendedPrice:            moneyToWire(p.RecommendedPrice),
		RecommendedUnits:            p.RecommendedUnits,
		RecommendedUnitsReduction:   p.RecommendedUnitsReduction,
		CampaignRef:                 p.CampaignRef,
		RecommendedBid:              moneyToWire(p.RecommendedBid),
		DesiredState:                p.DesiredState,
		PromotionRef:                p.PromotionRef,
		RecommendedActionPrice:      moneyToWire(p.RecommendedActionPrice),
		PrimaryRatingKey:            p.PrimaryRatingKey,
		RelatedRatingKeys:           append([]string(nil), p.RelatedRatingKeys...),
		ProviderErrorTypes:          append([]string(nil), p.ProviderErrorTypes...),
		Direction:                   p.Direction,
		AffectedMaterialSKURefs:     append([]string(nil), p.AffectedMaterialSKURefs...),
		Intent:                      p.Intent,
		RelatedMaterialCampaignRefs: append([]string(nil), p.RelatedMaterialCampaignRefs...),
		RecommendedBudget:           moneyToWire(p.RecommendedBudget),
		BudgetPeriod:                p.BudgetPeriod,
		DesiredStrategy:             p.DesiredStrategy,
		AddSKURefs:                  append([]string(nil), p.AddSKURefs...),
		RemoveSKURefs:               append([]string(nil), p.RemoveSKURefs...),
	}
}

func hasParameters(p Parameters) bool {
	return p.RecommendedPrice != nil || p.RecommendedUnits != nil || p.RecommendedUnitsReduction != nil ||
		p.CampaignRef != nil || p.RecommendedBid != nil || p.DesiredState != nil || p.PromotionRef != nil ||
		p.RecommendedActionPrice != nil || p.PrimaryRatingKey != nil || len(p.RelatedRatingKeys) > 0 ||
		len(p.ProviderErrorTypes) > 0 || p.Direction != nil || len(p.AffectedMaterialSKURefs) > 0 ||
		p.Intent != nil || len(p.RelatedMaterialCampaignRefs) > 0 || p.RecommendedBudget != nil ||
		p.BudgetPeriod != nil || p.DesiredStrategy != nil || len(p.AddSKURefs) > 0 || len(p.RemoveSKURefs) > 0
}

// ParametersFromJSON decodes proposal parameters from DB JSON (wire or legacy float form).
func ParametersFromJSON(b []byte) (Parameters, error) {
	var p Parameters
	if err := json.Unmarshal(b, &p); err == nil && hasParameters(p) {
		return p, nil
	}
	var aux map[string]json.RawMessage
	if err := json.Unmarshal(b, &aux); err != nil {
		return Parameters{}, err
	}
	out := Parameters{}
	if raw, ok := aux["recommended_price"]; ok {
		if m, err := moneyFromJSON(raw); err == nil {
			out.RecommendedPrice = m
		}
	}
	if raw, ok := aux["recommended_bid"]; ok {
		if m, err := moneyFromJSON(raw); err == nil {
			out.RecommendedBid = m
		}
	}
	if raw, ok := aux["recommended_action_price"]; ok {
		if m, err := moneyFromJSON(raw); err == nil {
			out.RecommendedActionPrice = m
		}
	}
	if raw, ok := aux["recommended_budget"]; ok {
		if m, err := moneyFromJSON(raw); err == nil {
			out.RecommendedBudget = m
		}
	}
	if raw, ok := aux["recommended_units"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err == nil {
			out.RecommendedUnits = &n
		}
	}
	if raw, ok := aux["recommended_units_reduction"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err == nil {
			out.RecommendedUnitsReduction = &n
		}
	}
	if raw, ok := aux["campaign_ref"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			out.CampaignRef = &s
		}
	}
	if hasParameters(out) {
		return out, nil
	}
	var wire ParametersWire
	if err := json.Unmarshal(b, &wire); err != nil {
		return Parameters{}, err
	}
	return ParametersFromWire(wire), nil
}

func moneyFromJSON(raw json.RawMessage) (*Money, error) {
	var wire moneyWire
	if err := json.Unmarshal(raw, &wire); err == nil && string(wire.Amount) != "" {
		if m := wireToMoney(&wire); m != nil {
			return m, nil
		}
	}
	var legacy struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	if legacy.Currency == "" {
		return nil, fmt.Errorf("missing currency")
	}
	return &Money{Amount: legacy.Amount, Currency: legacy.Currency}, nil
}

func wireToMoney(w *moneyWire) *Money {
	if w == nil {
		return nil
	}
	amt, err := strconv.ParseFloat(string(w.Amount), 64)
	if err != nil {
		return nil
	}
	return &Money{Amount: amt, Currency: w.Currency}
}

// ParametersFromWire decodes canonical wire parameters into catalog Parameters.
func ParametersFromWire(w ParametersWire) Parameters {
	return Parameters{
		RecommendedPrice:            wireToMoney(w.RecommendedPrice),
		RecommendedUnits:            w.RecommendedUnits,
		RecommendedUnitsReduction:   w.RecommendedUnitsReduction,
		CampaignRef:                 w.CampaignRef,
		RecommendedBid:              wireToMoney(w.RecommendedBid),
		DesiredState:                w.DesiredState,
		PromotionRef:                w.PromotionRef,
		RecommendedActionPrice:      wireToMoney(w.RecommendedActionPrice),
		PrimaryRatingKey:            w.PrimaryRatingKey,
		RelatedRatingKeys:           append([]string(nil), w.RelatedRatingKeys...),
		ProviderErrorTypes:          append([]string(nil), w.ProviderErrorTypes...),
		Direction:                   w.Direction,
		AffectedMaterialSKURefs:     append([]string(nil), w.AffectedMaterialSKURefs...),
		Intent:                      w.Intent,
		RelatedMaterialCampaignRefs: append([]string(nil), w.RelatedMaterialCampaignRefs...),
		RecommendedBudget:           wireToMoney(w.RecommendedBudget),
		BudgetPeriod:                w.BudgetPeriod,
		DesiredStrategy:             w.DesiredStrategy,
		AddSKURefs:                  append([]string(nil), w.AddSKURefs...),
		RemoveSKURefs:               append([]string(nil), w.RemoveSKURefs...),
	}
}
