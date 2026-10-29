package factual

import (
	"context"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *Service) SearchEntities(ctx context.Context, c factualapi.FactualReadContext, req factualapi.EntitySearchRequest) (factualapi.EntitySearchResult, error) {
	var out factualapi.EntitySearchResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	tokens := factualapi.NormalizeEntitySearchQuery(req.Query)
	if tokens.Empty() {
		return out, nil
	}
	limit := factualapi.NormalizeEntitySearchLimit(req.Limit)
	kinds := factualapi.DefaultEntitySearchKinds(req.Kinds)
	seen := map[string]struct{}{}
	for _, kind := range kinds {
		var items []factualapi.EntitySearchMatch
		var err error
		switch kind {
		case factualapi.EntitySearchSKU:
			items, err = s.searchSKUs(ctx, c, tokens, limit)
		case factualapi.EntitySearchCampaign:
			items, err = s.searchCampaigns(ctx, c, tokens, limit)
		case factualapi.EntitySearchCategory:
			items, err = s.searchCategories(ctx, c, tokens, limit)
		default:
			continue
		}
		if err != nil {
			return factualapi.EntitySearchResult{}, err
		}
		for _, it := range items {
			key := string(it.Kind) + ":" + fmt.Sprintf("%d", it.CanonicalID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out.Items = append(out.Items, it)
			if len(out.Items) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

func (s *Service) searchSKUs(ctx context.Context, c factualapi.FactualReadContext, tokens factualapi.EntitySearchTokens, limit int) ([]factualapi.EntitySearchMatch, error) {
	nums := tokens.Numerics
	if nums == nil {
		nums = []int64{}
	}
	numStrs := make([]string, 0, len(nums))
	for _, n := range nums {
		numStrs = append(numStrs, fmt.Sprintf("%d", n))
	}
	pats := make([]string, 0, len(tokens.Texts))
	for _, t := range tokens.Texts {
		pats = append(pats, "%"+likeEscape(t)+"%")
	}
	exact := tokens.Exact
	rows, err := s.pool.Query(ctx, `
		SELECT cs.id, cs.ozon_sku, v.provider_product_id, COALESCE(v.offer_id,''), COALESCE(v.name,''),
			CASE
				WHEN cs.id = ANY($3::bigint[]) THEN 0
				WHEN cs.ozon_sku = ANY($3::bigint[]) THEN 1
				WHEN lower(COALESCE(v.offer_id,'')) = ANY($4::text[]) THEN 2
				WHEN lower(COALESCE(v.name,'')) = $5 THEN 3
				ELSE 4
			END AS rank_ord,
			CASE
				WHEN cs.id = ANY($3::bigint[]) THEN 'canonical_id'
				WHEN cs.ozon_sku = ANY($3::bigint[]) THEN 'ozon_sku'
				WHEN lower(COALESCE(v.offer_id,'')) = ANY($4::text[]) THEN 'offer_id'
				WHEN lower(COALESCE(v.name,'')) = $5 THEN 'exact_name'
				ELSE 'token'
			END AS reason,
			COALESCE((SELECT count(*) FROM unnest($6::text[]) p WHERE lower(COALESCE(v.name,'') || ' ' || COALESCE(v.offer_id,'')) LIKE p ESCAPE '\' ), 0) AS token_hits
		FROM canonical_skus cs
		JOIN LATERAL (
			SELECT * FROM canonical_sku_versions x
			WHERE x.canonical_sku_id=cs.id AND x.observed_at<=$2
			ORDER BY x.observed_at DESC, x.id DESC LIMIT 1
		) v ON true
		WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2
		  AND (
			cs.id = ANY($3::bigint[])
			OR cs.ozon_sku = ANY($3::bigint[])
			OR lower(COALESCE(v.offer_id,'')) = ANY($4::text[])
			OR ($4::text[] <> '{}'::text[] AND (
				SELECT bool_or(lower(COALESCE(v.name,'') || ' ' || COALESCE(v.offer_id,'')) LIKE ('%' || x || '%'))
				FROM unnest($4::text[]) x
			))
			OR ($6::text[] <> '{}'::text[] AND (
				SELECT bool_or(lower(COALESCE(v.name,'') || ' ' || COALESCE(v.offer_id,'')) LIKE p ESCAPE '\' )
				FROM unnest($6::text[]) p
			))
		  )
		ORDER BY token_hits DESC, rank_ord, cs.id
		LIMIT $7`,
		c.SellerAccountID(), c.CutoffAt(), nums, numStrs, exact, pats, limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.EntitySearchMatch
	for rows.Next() {
		var m factualapi.EntitySearchMatch
		var ozon int64
		var provider *int64
		var hits int
		if err := rows.Scan(&m.CanonicalID, &ozon, &provider, &m.OfferID, &m.Name, &m.Rank, &m.MatchReason, &hits); err != nil {
			return nil, dbError(err)
		}
		m.Kind = factualapi.EntitySearchSKU
		m.OzonSKU = &ozon
		m.ProviderID = provider
		m.Meta, _ = s.meta(c, "CATALOG.SKU_UNIVERSE", "SKU", m.CanonicalID, nil, factHash(m.CanonicalID))
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Service) searchCampaigns(ctx context.Context, c factualapi.FactualReadContext, tokens factualapi.EntitySearchTokens, limit int) ([]factualapi.EntitySearchMatch, error) {
	nums := tokens.Numerics
	if nums == nil {
		nums = []int64{}
	}
	pats := make([]string, 0, len(tokens.Texts))
	for _, t := range tokens.Texts {
		pats = append(pats, "%"+likeEscape(t)+"%")
	}
	exact := tokens.Exact
	rows, err := s.pool.Query(ctx, campaignSelect+`
		WHERE c.seller_account_id=$1 AND c.first_observed_at<=$2
		  AND (
			c.id = ANY($3::bigint[])
			OR c.provider_campaign_id = ANY($3::bigint[])
			OR lower(COALESCE(v.title,'')) = $4
			OR ($5::text[] <> '{}'::text[] AND (
				SELECT bool_or(lower(COALESCE(v.title,'')) LIKE p ESCAPE '\' )
				FROM unnest($5::text[]) p
			))
		  )
		ORDER BY
			COALESCE((SELECT count(*) FROM unnest($5::text[]) p WHERE lower(COALESCE(v.title,'')) LIKE p ESCAPE '\' ), 0) DESC,
			CASE
				WHEN c.id = ANY($3::bigint[]) THEN 0
				WHEN c.provider_campaign_id = ANY($3::bigint[]) THEN 1
				WHEN lower(COALESCE(v.title,'')) = $4 THEN 2
				ELSE 3
			END, c.id
		LIMIT $6`,
		c.SellerAccountID(), c.CutoffAt(), nums, exact, pats, limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.EntitySearchMatch
	for rows.Next() {
		x, se := scanCampaign(rows)
		if se != nil {
			return nil, dbError(se)
		}
		reason := "token"
		rank := 3
		for _, n := range nums {
			if x.ID == n {
				reason, rank = "canonical_id", 0
				break
			}
			if x.ProviderCampaignID == n {
				reason, rank = "provider_id", 1
			}
		}
		if rank > 1 && strings.ToLower(x.Title) == exact {
			reason, rank = "exact_name", 2
		}
		x.Meta, _ = s.meta(c, "ADVERTISING.CAMPAIGN_CONFIG", "CAMPAIGN", x.ID, nil, factHash(x))
		pid := x.ProviderCampaignID
		out = append(out, factualapi.EntitySearchMatch{
			Kind: factualapi.EntitySearchCampaign, CanonicalID: x.ID, ProviderID: &pid,
			Name: x.Title, MatchReason: reason, Rank: rank, Meta: x.Meta,
		})
	}
	return out, rows.Err()
}

func (s *Service) searchCategories(ctx context.Context, c factualapi.FactualReadContext, tokens factualapi.EntitySearchTokens, limit int) ([]factualapi.EntitySearchMatch, error) {
	nums := tokens.Numerics
	if nums == nil {
		nums = []int64{}
	}
	pats := make([]string, 0, len(tokens.Texts))
	for _, t := range tokens.Texts {
		pats = append(pats, "%"+likeEscape(t)+"%")
	}
	exact := tokens.Exact
	rows, err := s.pool.Query(ctx, `
		SELECT sc.id, pc.provider_category_id, COALESCE(v.name,''),
			CASE
				WHEN sc.id = ANY($3::bigint[]) THEN 0
				WHEN pc.provider_category_id = ANY($3::bigint[]) THEN 1
				WHEN lower(COALESCE(v.name,'')) = $4 THEN 2
				ELSE 3
			END AS rank_ord,
			CASE
				WHEN sc.id = ANY($3::bigint[]) THEN 'canonical_id'
				WHEN pc.provider_category_id = ANY($3::bigint[]) THEN 'provider_id'
				WHEN lower(COALESCE(v.name,'')) = $4 THEN 'exact_name'
				ELSE 'token'
			END AS reason
		FROM seller_categories sc
		JOIN provider_categories pc ON pc.id=sc.provider_category_id
		JOIN LATERAL (
			SELECT * FROM provider_category_versions x
			WHERE x.provider_category_id=pc.id AND x.observed_at<=$2
			ORDER BY x.observed_at DESC, x.id DESC LIMIT 1
		) v ON true
		WHERE sc.seller_account_id=$1 AND sc.first_observed_at<=$2
		  AND (
			sc.id = ANY($3::bigint[])
			OR pc.provider_category_id = ANY($3::bigint[])
			OR lower(COALESCE(v.name,'')) = $4
			OR ($5::text[] <> '{}'::text[] AND (
				SELECT bool_or(lower(COALESCE(v.name,'')) LIKE p ESCAPE '\' )
				FROM unnest($5::text[]) p
			))
		  )
		ORDER BY
			COALESCE((SELECT count(*) FROM unnest($5::text[]) p WHERE lower(COALESCE(v.name,'')) LIKE p ESCAPE '\' ), 0) DESC,
			rank_ord, sc.id
		LIMIT $6`,
		c.SellerAccountID(), c.CutoffAt(), nums, exact, pats, limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.EntitySearchMatch
	for rows.Next() {
		var m factualapi.EntitySearchMatch
		var provider int64
		if err := rows.Scan(&m.CanonicalID, &provider, &m.Name, &m.Rank, &m.MatchReason); err != nil {
			return nil, dbError(err)
		}
		m.Kind = factualapi.EntitySearchCategory
		m.ProviderID = &provider
		m.Meta, _ = s.meta(c, "CATALOG.CATEGORY_ATTRIBUTION", "CATEGORY", m.CanonicalID, nil, factHash(m.CanonicalID))
		out = append(out, m)
	}
	return out, rows.Err()
}
