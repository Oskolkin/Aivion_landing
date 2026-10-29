package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type API interface {
	ListProductQueries(context.Context, int64, ozon.ProductQueriesRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error)
	ListProductQueryDetails(context.Context, int64, ozon.ProductQueryDetailsRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error)
	ListTopSearchQueries(context.Context, int64, ozon.MarketSearchRequest) (*ozon.TypedResponse[ozon.SearchQueryResult], error)
}

type UniverseResolver interface {
	ResolveSearchUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type Runner struct {
	API                     API
	Universe                UniverseResolver
	Collection              PayloadRegistrar
	Payloads                *collection.SourcePayloadStore
	Now                     func() time.Time
	PageSize                int
	LimitBySKU              int
	DetailSKUSelectionLimit int
	EnableMarketSearch      bool
}

func NormalizeQueryKey(query string) string {
	// Deliberately conservative: Unicode-aware case folding plus trim and
	// whitespace collapse only. No stemming, transliteration or punctuation edits.
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(query))), " ")
}

func (r *Runner) Collect(ctx context.Context, seller, run int64, through time.Time) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Universe == nil || r.Collection == nil || r.Payloads == nil {
		return out, errors.New("search runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	if r.PageSize <= 0 {
		r.PageSize = DefaultPageSize
	}
	if r.LimitBySKU <= 0 {
		r.LimitBySKU = LimitBySKUDefault
	}
	if r.LimitBySKU > ozon.ProductQueryDetailsLimitMax {
		r.LimitBySKU = ozon.ProductQueryDetailsLimitMax
	}
	if r.DetailSKUSelectionLimit <= 0 {
		r.DetailSKUSelectionLimit = DetailSKUSelectionLimit
	}
	out.StartedAt = r.Now().UTC()
	universe, err := r.Universe.ResolveSearchUniverse(ctx, seller, out.StartedAt)
	if err != nil {
		return out, err
	}
	sort.Slice(universe, func(i, j int) bool { return universe[i].OzonSKU < universe[j].OzonSKU })
	bySKU := make(map[int64]int64, len(universe))
	ozonSKUs := make([]int64, 0, len(universe))
	for _, sku := range universe {
		bySKU[sku.OzonSKU] = sku.CanonicalSKUID
		ozonSKUs = append(ozonSKUs, sku.OzonSKU)
	}
	end := dateOnly(through)
	periods := []struct {
		kind     string
		from, to time.Time
	}{
		{"CURRENT_7D", end.AddDate(0, 0, -6), end},
		{"PREVIOUS_7D", end.AddDate(0, 0, -13), end.AddDate(0, 0, -7)},
	}
	seq := 0
	if len(ozonSKUs) == 0 {
		out.SellerSummaryStatus = "COMPLETE"
	} else {
		for _, p := range periods {
			for start := 0; start < len(ozonSKUs); start += ozon.ProductQueriesSKULimit {
				batchEnd := start + ozon.ProductQueriesSKULimit
				if batchEnd > len(ozonSKUs) {
					batchEnd = len(ozonSKUs)
				}
				batch := ozonSKUs[start:batchEnd]
				page := 1
				for {
					req := ozon.ProductQueriesRequest{
						DateFrom: p.from.Format("2006-01-02"), DateTo: p.to.Format("2006-01-02"),
						SKUs: batch, Page: page, PageSize: r.PageSize,
						SortBy: DefaultSortBy, SortDir: DefaultSortDir,
					}
					resp, fetchErr := r.API.ListProductQueries(ctx, seller, req)
					if fetchErr != nil {
						return out, fmt.Errorf("seller search summary %s page %d: %w", p.kind, page, fetchErr)
					}
					seq++
					payload, regErr := r.register(ctx, seller, run, SummaryEndpoint, req, SummaryContract, resp.Raw, resp.Meta.StatusCode, seq, map[string]any{
						"period_kind": p.kind, "page": page, "has_next": resp.Data.HasNext, "sku_batch_size": len(batch),
					})
					if regErr != nil {
						return out, regErr
					}
					out.PagesReceived++
					for _, row := range resp.Data.Items {
						sku := int64(row.SKU)
						if sku <= 0 {
							continue
						}
						summary := SKUSummary{
							SourceSKU: sku, SKUResolutionStatus: "UNRESOLVED",
							PeriodStart: p.from, PeriodEnd: p.to, PeriodKind: p.kind,
							ProviderSearchGMV: number(row.ProviderSearchGMV), Orders: number(row.Orders),
							UniqueSearchUsers: optionalNumber(row.UniqueSearchUsers), UniqueViewUsers: optionalNumber(row.UniqueViewUsers),
							Position: optionalNumber(row.Position), ViewConversion: optionalNumber(row.ViewConversion),
							UniqueUsersAggregation: "NON_ADDITIVE",
							SourcePayloadID:        payload.ID, ObservedAt: observed(payload, r.Now), SourceFragment: row.Raw,
						}
						if canonical, ok := bySKU[sku]; ok {
							id := canonical
							summary.CanonicalSKUID = &id
							summary.SKUResolutionStatus = "RESOLVED"
						}
						out.SKUSummaries = append(out.SKUSummaries, summary)
					}
					if !resp.Data.HasNext {
						break
					}
					page++
				}
			}
		}
		out.SellerSummaryStatus = "COMPLETE"
	}

	detailSKUs := append([]int64(nil), ozonSKUs...)
	if len(detailSKUs) > r.DetailSKUSelectionLimit {
		detailSKUs = detailSKUs[:r.DetailSKUSelectionLimit]
	}
	if len(detailSKUs) == 0 {
		out.QueryDetailsStatus = "PARTIAL"
	} else {
		p := periods[0]
		detailsPartial := false
		for start := 0; start < len(detailSKUs); start += ozon.ProductQueryDetailsSKULimit {
			batchEnd := start + ozon.ProductQueryDetailsSKULimit
			if batchEnd > len(detailSKUs) {
				batchEnd = len(detailSKUs)
			}
			batch := detailSKUs[start:batchEnd]
			page := 1
			for {
				req := ozon.ProductQueryDetailsRequest{
					DateFrom: p.from.Format("2006-01-02"), DateTo: p.to.Format("2006-01-02"),
					SKUs: batch, LimitBySKU: r.LimitBySKU, Page: page, PageSize: r.PageSize,
					SortBy: DefaultSortBy, SortDir: DefaultSortDir,
				}
				resp, fetchErr := r.API.ListProductQueryDetails(ctx, seller, req)
				if fetchErr != nil {
					detailsPartial = true
					break
				}
				seq++
				payload, regErr := r.register(ctx, seller, run, DetailsEndpoint, req, DetailsContract, resp.Raw, resp.Meta.StatusCode, seq, map[string]any{
					"retrieval_scope": RetrievalReasonTopN, "retrieval_reason": RetrievalReasonTopN,
					"limit_by_sku": r.LimitBySKU, "detail_sku_selection_limit": r.DetailSKUSelectionLimit,
					"sort_by": DefaultSortBy, "sort_dir": DefaultSortDir, "page": page, "page_size": r.PageSize,
					"policy": DetailCollectionPolicyV1,
				})
				if regErr != nil {
					return out, regErr
				}
				out.PagesReceived++
				for _, row := range resp.Data.Items {
					id := identity(row.Query)
					if id.NormalizedQueryKey == "" {
						continue
					}
					sku := int64(row.SKU)
					detail := Detail{
						Identity: id, SourceSKU: sku, SKUResolutionStatus: "UNRESOLVED",
						ProviderRank: row.Rank, ProviderSearchGMV: number(row.ProviderSearchGMV), Orders: number(row.Orders),
						RetrievalScope: RetrievalReasonTopN, RetrievalReason: RetrievalReasonTopN,
						TopNLimit: r.LimitBySKU, LimitBySKU: r.LimitBySKU, SortBy: DefaultSortBy, SortDir: DefaultSortDir,
						Page: page, PageSize: r.PageSize,
						PeriodStart: p.from, PeriodEnd: p.to, SourcePayloadID: payload.ID,
						ObservedAt: observed(payload, r.Now), SourceFragment: row.Raw,
					}
					if canonical, ok := bySKU[sku]; ok {
						cid := canonical
						detail.CanonicalSKUID = &cid
						detail.SKUResolutionStatus = "RESOLVED"
					}
					out.Details = append(out.Details, detail)
				}
				if !resp.Data.HasNext {
					break
				}
				page++
			}
			if detailsPartial {
				break
			}
		}
		if detailsPartial {
			out.QueryDetailsStatus = "PARTIAL"
		} else {
			out.QueryDetailsStatus = "COMPLETE"
		}
	}

	out.MarketSearchStatus = "UNAVAILABLE"
	if r.EnableMarketSearch {
		p := periods[0]
		req := ozon.MarketSearchRequest{DateFrom: p.from.Format("2006-01-02"), DateTo: p.to.Format("2006-01-02"), Limit: 1000}
		if resp, err := r.API.ListTopSearchQueries(ctx, seller, req); err == nil {
			seq++
			payload, regErr := r.register(ctx, seller, run, MarketEndpoint, req, SummaryContract, resp.Raw, resp.Meta.StatusCode, seq, map[string]any{"premium": true})
			if regErr != nil {
				return out, regErr
			}
			for _, row := range resp.Data.Items {
				id := identity(row.Query)
				if id.NormalizedQueryKey != "" {
					out.MarketFacts = append(out.MarketFacts, MarketFact{Identity: id, ProviderRank: row.Rank, PeriodStart: p.from, PeriodEnd: p.to, SourcePayloadID: payload.ID, ObservedAt: observed(payload, r.Now)})
				}
			}
			out.MarketSearchStatus = "COMPLETE"
			out.PagesReceived++
		}
	}
	out.CompletenessStatus = "COMPLETE"
	if out.QueryDetailsStatus != "COMPLETE" {
		out.CompletenessStatus = "PARTIAL"
	}
	out.CompletedAt = r.Now().UTC()
	return out, nil
}

func identity(query string) QueryIdentity {
	return QueryIdentity{Provider: "OZON", Market: "RU", QueryText: strings.TrimSpace(query), NormalizedQueryKey: NormalizeQueryKey(query)}
}
func number(n ozon.ExactNumber) string {
	if strings.TrimSpace(string(n)) == "" {
		return "0"
	}
	return string(n)
}
func optionalNumber(n ozon.ExactNumber) string {
	return strings.TrimSpace(string(n))
}
func observed(p collection.SourcePayload, now func() time.Time) time.Time {
	if !p.ReceivedAt.IsZero() {
		return p.ReceivedAt
	}
	return now().UTC()
}
func dateOnly(v time.Time) time.Time {
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}

func (r *Runner) register(ctx context.Context, seller, run int64, endpoint string, request any, contract collection.SourceContract, body []byte, statusCode, seq int, metadata map[string]any) (collection.SourcePayload, error) {
	var identity map[string]any
	raw, err := json.Marshal(request)
	if err != nil {
		return collection.SourcePayload{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err = decoder.Decode(&identity); err != nil {
		return collection.SourcePayload{}, err
	}
	for k, v := range map[string]any{"contract_key": contract.ContractKey, "contract_version": contract.ContractVersion, "semantic_mapping_hash": contract.SemanticMappingHash, "redaction_profile": RedactionProfile} {
		metadata[k] = v
	}
	page, status := int64(seq), int32(statusCode)
	return r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: endpoint, RequestIdentity: identity, Metadata: metadata, Body: body, ContentType: "application/json", PageSeq: &page, HTTPStatus: &status})
}
