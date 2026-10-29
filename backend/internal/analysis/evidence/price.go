package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type priceObs struct {
	ObservedAt            string  `json:"observed_at"`
	ProviderCurrentPrice  *string `json:"provider_current_price,omitempty"`
	CustomerPrice         *string `json:"customer_price,omitempty"`
	SellerPromoPrice      *string `json:"seller_promo_price,omitempty"`
	OldReferencePrice     *string `json:"old_reference_price,omitempty"`
	CurrencyCode          string  `json:"currency_code,omitempty"`
	FactRef               string  `json:"fact_ref,omitempty"`
	NotForwardFilledDaily bool    `json:"not_forward_filled_daily"`
}

func registerPrice(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_PRICE_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_PRICE_HISTORY")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.GetSKUPriceHistory(ctx, rc, id, factualapi.PageRequest{PageSize: b.MaxObservations})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			obs := []priceObs{}
			facts := []DeclaredFact{}
			for _, p := range page.Items {
				o := priceObs{
					ObservedAt: p.ObservedAt.UTC().Format("2006-01-02T15:04:05Z"), ProviderCurrentPrice: knownMoney(p.ProviderCurrentPrice),
					CustomerPrice: knownMoney(p.CustomerPrice), SellerPromoPrice: knownMoney(p.SellerPromoPrice),
					OldReferencePrice: knownMoney(p.OldReferencePrice), CurrencyCode: p.CurrencyCode, FactRef: p.Meta.FactRef,
					NotForwardFilledDaily: true,
				}
				obs = append(obs, o)
				if p.Meta.FactRef != "" {
					facts = append(facts, DeclaredFact{SemanticPath: "price.observed." + o.ObservedAt, FactRefs: []string{p.Meta.FactRef}})
				}
			}
			obs, tr := boundList(obs, b.MaxObservations, "observed_at_desc", "OBSERVATION_HISTORY", nil)
			if page.NextPageToken != "" {
				tr.Truncated = true
			}
			st := StatusComplete
			if len(obs) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_PRICE_HISTORY"), rc), Truncation: tr, Data: map[string]any{"observations": obs, "promotion_action_price_does_not_replace_observed_price": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
}
