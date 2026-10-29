package evidence

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type TypeBudget struct {
	MaxRows            int    `json:"max_rows"`
	MaxObservations    int    `json:"max_observations"`
	MaxRelatedEntities int    `json:"max_related_entities"`
	MaxTextLength      int    `json:"max_text_length"`
	MaxSerializedBytes int    `json:"max_serialized_bytes"`
	CostClass          string `json:"cost_class"`
}

type BudgetProfile struct {
	Version            string                `json:"version"`
	MaxBundleBytes     int                   `json:"max_bundle_bytes"`
	MaxEstimatedTokens int                   `json:"max_estimated_tokens"`
	MaxUniqueRequests  int                   `json:"max_unique_requests"`
	ByType             map[string]TypeBudget `json:"by_type"`
}

func CurrentBudget() BudgetProfile {
	b := BudgetProfile{
		Version:            BudgetProfileV1,
		MaxBundleBytes:     2_000_000,
		MaxEstimatedTokens: 500_000,
		MaxUniqueRequests:  24,
		ByType:             map[string]TypeBudget{},
	}
	for _, t := range evidencecatalog.Catalog() {
		tb := TypeBudget{MaxRows: 40, MaxObservations: 90, MaxRelatedEntities: 40, MaxTextLength: 512, MaxSerializedBytes: 80_000, CostClass: "standard"}
		switch t.TemporalProfileKey {
		case evidencecatalog.TemporalFlowHistory30D90D:
			tb.MaxObservations = 90
			tb.MaxRows = 40
			tb.CostClass = "flow"
		case evidencecatalog.TemporalObservationHistory:
			tb.MaxObservations = 60
			tb.CostClass = "observation"
		case evidencecatalog.TemporalCurrentSnapshot:
			tb.MaxObservations = 1
			tb.MaxRows = 40
			tb.CostClass = "snapshot"
		case evidencecatalog.TemporalEventHistory30D:
			tb.MaxRows = 40
			tb.CostClass = "event"
		case evidencecatalog.TemporalProviderPeriod:
			tb.MaxObservations = 12
			tb.CostClass = "provider_period"
		}
		b.ByType[t.Key] = tb
	}
	return b
}

func (p BudgetProfile) Hash() string { return canonicaljson.Hash(p) }

func (p BudgetProfile) For(t string) TypeBudget {
	if b, ok := p.ByType[t]; ok {
		return b
	}
	return TypeBudget{MaxRows: 20, MaxObservations: 30, MaxRelatedEntities: 20, MaxTextLength: 256, MaxSerializedBytes: 40_000, CostClass: "standard"}
}
