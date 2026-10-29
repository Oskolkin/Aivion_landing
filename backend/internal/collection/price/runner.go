package price

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type PriceAPI interface {
	ListCanonicalPrices(context.Context, int64, ozon.CanonicalPriceRequest) (*ozon.TypedResponse[ozon.CanonicalPriceResult], error)
}
type UniverseResolver interface {
	ResolvePriceUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error)
}
type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type Runner struct {
	API        PriceAPI
	Universe   UniverseResolver
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Recovery   collection.ReplayRepository
	Now        func() time.Time
}

func (r *Runner) Collect(ctx context.Context, seller, run int64) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Universe == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("price runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	out.StartedAt = r.Now().UTC()
	universe, err := r.Universe.ResolvePriceUniverse(ctx, seller, out.StartedAt)
	if err != nil {
		return out, err
	}
	sort.Slice(universe, func(i, j int) bool { return universe[i].CanonicalSKUID < universe[j].CanonicalSKUID })
	parts := make([]string, 0, len(universe))
	byProduct := map[int64]FrozenSKU{}
	for _, x := range universe {
		parts = append(parts, fmt.Sprintf("%d:%d:%d:%s", x.CanonicalSKUID, x.OzonSKU, x.ProviderProductID, x.OfferID))
		if x.ProviderProductID > 0 {
			byProduct[x.ProviderProductID] = x
		}
	}
	out.UniverseHash = factualapi.StableHash(parts...)
	out.ExpectedSKUCount = len(universe)
	out.RequestedSKUCount = len(universe)
	limit := ozon.PriceListPageLimit
	if len(universe) > 0 {
		out.BatchesExpected = (len(universe) + limit - 1) / limit
	}
	replay, err := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, CoreEndpoint, RedactionProfile)
	if err != nil {
		return out, err
	}
	sequence := 0
	seen := map[int64]struct{}{}
	for start := 0; start < len(universe); start += limit {
		end := start + limit
		if end > len(universe) {
			end = len(universe)
		}
		ids := make([]int64, 0, end-start)
		for _, x := range universe[start:end] {
			if x.ProviderProductID > 0 {
				ids = append(ids, x.ProviderProductID)
			}
		}
		if len(ids) == 0 {
			out.BatchesFailed++
			continue
		}
		cursor := ""
		for page := 1; ; page++ {
			sequence++
			identity := map[string]any{"product_ids": ids, "cursor": cursor, "limit": limit}
			var resp *ozon.TypedResponse[ozon.CanonicalPriceResult]
			var payload collection.SourcePayload
			if sequence <= len(replay) {
				artifact := replay[sequence-1]
				if err = validatePriceReplay(artifact, identity, sequence); err != nil {
					return out, err
				}
				var result ozon.CanonicalPriceResult
				if err = json.Unmarshal(artifact.Body, &result); err != nil {
					return out, fmt.Errorf("decode price replay: %w", err)
				}
				resp = &ozon.TypedResponse[ozon.CanonicalPriceResult]{Data: result, Raw: artifact.Body}
				payload = artifact.Payload
			} else {
				var fetchErr error
				resp, fetchErr = r.API.ListCanonicalPrices(ctx, seller, ozon.CanonicalPriceRequest{Cursor: cursor, Filter: ozon.PriceFilter{ProductIDs: ids, Visibility: "ALL"}, Limit: limit})
				if fetchErr != nil {
					out.BatchesFailed++
					return out, fmt.Errorf("price batch %d page %d: %w", start/limit+1, page, fetchErr)
				}
				seq := int64(sequence)
				status := int32(resp.Meta.StatusCode)
				payload, err = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: CoreEndpoint, RequestIdentity: identity, Metadata: map[string]any{"contract_key": CoreContract.ContractKey, "contract_version": CoreContract.ContractVersion, "semantic_mapping_hash": CoreContract.SemanticMappingHash, "redaction_profile": RedactionProfile, "cursor": cursor, "next_cursor": resp.Data.Cursor, "returned_rows": len(resp.Data.Items)}, Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
				if err != nil {
					return out, err
				}
			}
			for _, row := range resp.Data.Items {
				product := int64(row.ProductID)
				u, ok := byProduct[product]
				var canonical, sourceSKU, provider *int64
				resolution := "UNRESOLVED"
				if product > 0 {
					v := product
					provider = &v
				}
				if ok {
					v := u.CanonicalSKUID
					canonical = &v
					s := u.OzonSKU
					sourceSKU = &s
					resolution = "RESOLVED"
				}
				seen[product] = struct{}{}
				obs, e := normalize(row, canonical, sourceSKU, provider, payload.ID)
				if e != nil {
					return out, e
				}
				obs.SKUResolutionStatus = resolution
				out.Observations = append(out.Observations, obs)
			}
			if resp.Data.Cursor == "" {
				break
			}
			if resp.Data.Cursor == cursor {
				return out, fmt.Errorf("price cursor did not advance")
			}
			cursor = resp.Data.Cursor
		}
		out.BatchesCompleted++
	}
	if sequence < len(replay) {
		return out, fmt.Errorf("price replay contains artifacts outside the frozen run plan")
	}
	out.ReceivedSKUCount = len(seen)
	out.CompletedAt = r.Now().UTC()
	out.CompletenessStatus = "COMPLETE"
	if out.BatchesFailed > 0 || out.ReceivedSKUCount < out.ExpectedSKUCount {
		out.CompletenessStatus = "PARTIAL"
	}
	return out, nil
}

func validatePriceReplay(a collection.ReplayArtifact, identity map[string]any, page int) error {
	if a.Payload.PageSeq == nil || *a.Payload.PageSeq != int64(page) {
		return fmt.Errorf("price replay sequence mismatch")
	}
	request := map[string]any{"collection_domain": string(collection.DomainPrice)}
	for k, v := range identity {
		request[k] = v
	}
	fingerprint, err := collection.RequestFingerprint(collection.ProviderOzon, CoreEndpoint, request)
	if err != nil || fingerprint != a.Payload.RequestFingerprint {
		return fmt.Errorf("price replay request fingerprint mismatch")
	}
	var metadata map[string]any
	if json.Unmarshal(a.Payload.Metadata, &metadata) != nil || metadata["semantic_mapping_hash"] != CoreContract.SemanticMappingHash {
		return fmt.Errorf("price replay source contract mismatch")
	}
	return nil
}

func normalize(row ozon.CanonicalPriceRow, canonical, sourceSKU, provider *int64, payloadID int64) (Observation, error) {
	current, e := moneyPtr(row.Price.Price, false)
	if e != nil {
		return Observation{}, e
	}
	state := "KNOWN"
	if current == nil || current.IsZero() {
		current = nil
		state = "INVALID"
	}
	old, e := moneyPtr(row.Price.OldPrice, true)
	if e != nil {
		return Observation{}, e
	}
	minimum, e := moneyPtr(row.Price.MinPrice, true)
	if e != nil {
		return Observation{}, e
	}
	promo, e := moneyPtr(row.Price.MarketingSellerPrice, true)
	if e != nil {
		return Observation{}, e
	}
	retail, e := moneyPtr(row.Price.RetailPrice, true)
	if e != nil {
		return Observation{}, e
	}
	vat, e := moneyPtr(row.Price.VAT, true)
	if e != nil {
		return Observation{}, e
	}
	index, _ := json.Marshal(row.PriceIndexes)
	promotion := row.MarketingActions
	if len(promotion) == 0 {
		promotion = json.RawMessage(`{}`)
	}
	raw := row.Raw
	if len(raw) == 0 {
		raw, _ = json.Marshal(row)
	}
	id := payloadID
	auto := row.Price.AutoActionEnabled
	currency := strings.ToUpper(strings.TrimSpace(row.CurrencyCode))
	if currency == "" {
		currency, e = sales.ResolveCurrency("OZON", "RU")
		if e != nil {
			return Observation{}, e
		}
	}
	return Observation{CanonicalSKUID: canonical, SourceSKU: sourceSKU, ProviderProductID: provider, CurrencyCode: currency, ProviderCurrentPrice: current, SellerPromoPrice: promo, OldReferencePrice: old, MinimumAllowedPrice: minimum, RetailPrice: retail, VATRate: vat, AutoActionEnabled: &auto, CurrentPriceState: state, PriceIndexColor: row.PriceIndexes.Color, PriceIndexJSON: index, PromotionContextJSON: promotion, ProviderFields: raw, SemanticMappingStatus: "PENDING_ACCEPTANCE", ReconciliationStatus: "NOT_APPLICABLE", SourcePayloadID: &id}, nil
}
func moneyPtr(raw string, zeroIsUnset bool) (*sales.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, e := sales.ParseAmount(json.Number(raw))
	if e != nil {
		return nil, e
	}
	if zeroIsUnset && v.IsZero() {
		return nil, nil
	}
	return &v, nil
}
