package promotions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type PromotionsAPI interface {
	ListPromotionActions(context.Context, int64) (*ozon.TypedResponse[[]ozon.PromotionAction], error)
	ListPromotionCandidates(context.Context, int64, ozon.PromotionProductRequest) (*ozon.TypedResponse[ozon.PromotionProductsResult], error)
	ListPromotionProducts(context.Context, int64, ozon.PromotionProductRequest) (*ozon.TypedResponse[ozon.PromotionProductsResult], error)
}

type SellerPromotionsAPI interface {
	ListSellerPromotionActions(context.Context, int64) (*ozon.TypedResponse[[]ozon.PromotionAction], error)
	ListSellerPromotionCandidates(context.Context, int64, ozon.PromotionProductRequest) (*ozon.TypedResponse[ozon.PromotionProductsResult], error)
	ListSellerPromotionProducts(context.Context, int64, ozon.PromotionProductRequest) (*ozon.TypedResponse[ozon.PromotionProductsResult], error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type SKUResolver interface {
	ResolvePromotionSKU(ctx context.Context, seller int64, productID, sourceSKU *int64, offerID string, cutoff time.Time) (*int64, string, error)
}

type Runner struct {
	API        PromotionsAPI
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	SKUs       SKUResolver
	Now        func() time.Time
}

func (r *Runner) Collect(ctx context.Context, seller, run int64) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("promotions runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	out.StartedAt = r.Now().UTC()
	if err := r.collectOrigin(ctx, seller, run, "OZON", r.API.ListPromotionActions, r.API.ListPromotionProducts, r.API.ListPromotionCandidates, &out); err != nil {
		return out, err
	}
	if sellerAPI, ok := r.API.(SellerPromotionsAPI); ok {
		if err := r.collectOrigin(ctx, seller, run, "SELLER", sellerAPI.ListSellerPromotionActions, sellerAPI.ListSellerPromotionProducts, sellerAPI.ListSellerPromotionCandidates, &out); err != nil {
			out.CompletenessStatus = "PARTIAL"
		}
	}
	out.CompletedAt = r.Now().UTC()
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	return out, nil
}

type listActions func(context.Context, int64) (*ozon.TypedResponse[[]ozon.PromotionAction], error)
type listProducts func(context.Context, int64, ozon.PromotionProductRequest) (*ozon.TypedResponse[ozon.PromotionProductsResult], error)

func (r *Runner) collectOrigin(ctx context.Context, seller, run int64, origin string, actions listActions, products, candidates listProducts, out *Result) error {
	response, err := actions(ctx, seller)
	if err != nil {
		return err
	}
	endpoint := ActionsEndpoint
	contract := ActionsContract
	if origin == "SELLER" {
		endpoint = SellerActionsEndpoint
	}
	payload, err := r.register(ctx, seller, run, endpoint, contract, map[string]any{"origin": origin}, response.Raw, 1, response.Meta.StatusCode, len(response.Data))
	if err != nil {
		return err
	}
	out.DefinitionRequests++
	for _, action := range response.Data {
		def, err := normalizeDefinition(action, origin, payload.ID)
		if err != nil {
			return err
		}
		out.Definitions = append(out.Definitions, def)
		participantRows, participantComplete := r.collectProductPages(ctx, seller, run, origin, actionID(action), products, false, out)
		out.Participants = append(out.Participants, participantRows...)
		if participantComplete {
			out.CompleteParticipantKeys = append(out.CompleteParticipantKeys, PromotionKey{Origin: origin, ProviderActionID: actionID(action)})
		} else {
			out.CompletenessStatus = "PARTIAL"
		}
		candidateRows, candidateComplete := r.collectProductPages(ctx, seller, run, origin, actionID(action), candidates, true, out)
		status := "PARTIAL"
		if candidateComplete {
			status = "COMPLETE"
		} else {
			out.CompletenessStatus = "PARTIAL"
		}
		var sourcePayload *int64
		if len(candidateRows) > 0 {
			id := candidateRows[len(candidateRows)-1].SourcePayloadID
			sourcePayload = &id
		}
		out.Eligibility = append(out.Eligibility, EligibilitySnapshot{ProviderActionID: actionID(action), Origin: origin, CompletenessStatus: status, Rows: candidateRows, SourcePayloadID: sourcePayload})
	}
	return nil
}

func (r *Runner) collectProductPages(ctx context.Context, seller, run int64, origin string, actionID int64, call listProducts, candidate bool, out *Result) ([]SKUObservation, bool) {
	var rows []SKUObservation
	lastID := int64(0)
	for page := int64(1); ; page++ {
		response, err := call(ctx, seller, ozon.PromotionProductRequest{ActionID: actionID, Limit: ozon.PromotionProductsPageLimit, LastID: lastID})
		if err != nil {
			return rows, false
		}
		endpoint, contract := ProductsEndpoint, ProductsContract
		if candidate {
			endpoint, contract = CandidatesEndpoint, CandidatesContract
		}
		if origin == "SELLER" {
			if candidate {
				endpoint = SellerCandidatesEndpoint
			} else {
				endpoint = SellerProductsEndpoint
			}
		}
		got := response.Data.Rows()
		payload, err := r.register(ctx, seller, run, endpoint, contract, map[string]any{"origin": origin, "action_id": actionID, "last_id": lastID, "limit": ozon.PromotionProductsPageLimit}, response.Raw, page, response.Meta.StatusCode, len(got))
		if err != nil {
			return rows, false
		}
		if candidate {
			out.CandidatePages++
		} else {
			out.ParticipantPages++
		}
		for _, product := range got {
			obs, err := r.normalizeProduct(ctx, seller, origin, actionID, product, payload.ID, out.StartedAt)
			if err != nil {
				return rows, false
			}
			rows = append(rows, obs)
		}
		if response.Data.Total > 0 && int64(len(rows)) >= response.Data.Total {
			return rows, true
		}
		if len(got) < ozon.PromotionProductsPageLimit {
			return rows, true
		}
		next := response.Data.LastID
		if next <= lastID {
			return rows, false
		}
		lastID = next
	}
}

func (r *Runner) register(ctx context.Context, seller, run int64, endpoint string, contract collection.SourceContract, identity map[string]any, body []byte, seq int64, status, returned int) (collection.SourcePayload, error) {
	httpStatus := int32(status)
	meta := map[string]any{"contract_key": contract.ContractKey, "contract_version": contract.ContractVersion, "semantic_mapping_hash": contract.SemanticMappingHash, "redaction_profile": RedactionProfile, "returned_rows": returned}
	return r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: endpoint, RequestIdentity: identity, Metadata: meta, Body: body, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &httpStatus})
}

func normalizeDefinition(action ozon.PromotionAction, origin string, payload int64) (DefinitionObservation, error) {
	value, err := decimal(action.DiscountValue.String())
	if err != nil {
		return DefinitionObservation{}, err
	}
	var raw struct {
		Rules      []json.RawMessage `json:"rules"`
		Conditions []json.RawMessage `json:"conditions"`
	}
	_ = json.Unmarshal(action.Raw, &raw)
	rules := raw.Rules
	if len(rules) == 0 {
		rules = raw.Conditions
	}
	return DefinitionObservation{Origin: origin, ProviderActionID: actionID(action), Title: action.Title, ProviderState: strings.ToUpper(action.State), StartsAt: parseTime(action.DateStart), EndsAt: parseTime(action.DateEnd), DiscountType: action.DiscountType, DiscountValue: value, CurrencyCode: strings.ToUpper(action.CurrencyCode), ProviderFields: jsonOrEmpty(action.Raw), RuleComponents: rules, SourcePayloadID: payload}, nil
}

func (r *Runner) normalizeProduct(ctx context.Context, seller int64, origin string, actionID int64, product ozon.PromotionProduct, payload int64, cutoff time.Time) (SKUObservation, error) {
	obs := SKUObservation{Origin: origin, ProviderActionID: actionID, SourceOfferID: product.OfferID, SKUResolutionStatus: "UNRESOLVED", ActionStock: product.Stock, ProviderFields: jsonOrEmpty(product.Raw), SourcePayloadID: payload}
	if id := int64(product.ProductID); id > 0 {
		obs.SourceProductID = &id
	}
	if sku := int64(product.SKU); sku > 0 {
		obs.SourceSKU = &sku
	}
	price, err := decimal(product.ActionPrice.String())
	if err != nil {
		return obs, err
	}
	obs.ActionPrice = price
	if r.SKUs != nil {
		obs.CanonicalSKUID, obs.SKUResolutionStatus, err = r.SKUs.ResolvePromotionSKU(ctx, seller, obs.SourceProductID, obs.SourceSKU, obs.SourceOfferID, cutoff)
		if err != nil {
			return obs, err
		}
	}
	return obs, nil
}

func actionID(a ozon.PromotionAction) int64 { return int64(a.ID) }
func decimal(raw string) (*sales.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := sales.ParseAmount(json.Number(raw))
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func parseTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if v, err := time.Parse(layout, raw); err == nil {
			v = v.UTC()
			return &v
		}
	}
	return nil
}
func jsonOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
