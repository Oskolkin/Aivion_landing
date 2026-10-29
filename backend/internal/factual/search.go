package factual

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func searchMoney(raw string) (factualapi.Value[factualapi.Money], error) {
	d, err := factualapi.ParseDecimal(raw)
	if err != nil {
		return factualapi.Value[factualapi.Money]{}, err
	}
	return factualapi.Known(factualapi.Money{Amount: d, CurrencyCode: "RUB"}), nil
}

func searchDecimal(raw string) (factualapi.Value[factualapi.Decimal], error) {
	if raw == "" {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "PROVIDER_FIELD_UNAVAILABLE"), nil
	}
	d, err := factualapi.ParseDecimal(raw)
	if err != nil {
		return factualapi.Value[factualapi.Decimal]{}, err
	}
	return factualapi.Known(d), nil
}

func (s *Service) mapSKUSearchSummary(c factualapi.FactualReadContext, summaryID int64, canonical *int64, sourceSKU int64, resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion string, p factualapi.Period) (factualapi.SKUSearchSummary, error) {
	out := factualapi.SKUSearchSummary{
		ID: summaryID, SellerAccountID: c.SellerAccountID(), SourceSKU: sourceSKU,
		CanonicalSKUID: canonical, SKUResolutionStatus: resolution, Period: p, PeriodKind: periodKind,
		UniqueUsersAggregation: "NON_ADDITIVE",
	}
	var err error
	out.ProviderSearchGMV, err = searchMoney(gmv)
	if err != nil {
		return out, err
	}
	out.Orders, err = searchDecimal(orders)
	if err != nil {
		return out, err
	}
	out.UniqueSearchUsers, err = searchDecimal(searchUsers)
	if err != nil {
		return out, err
	}
	out.UniqueViewUsers, err = searchDecimal(viewUsers)
	if err != nil {
		return out, err
	}
	out.Position, err = searchDecimal(position)
	if err != nil {
		return out, err
	}
	out.ViewConversion, err = searchDecimal(conversion)
	if err != nil {
		return out, err
	}
	entity := sourceSKU
	if canonical != nil {
		entity = *canonical
	}
	out.Meta, _ = s.meta(c, "SEARCH.SELLER_SUMMARY", "SKU_SEARCH_SUMMARY", entity, &p, factHash(out))
	return out, nil
}

func (s *Service) GetSKUSearchSummary(ctx context.Context, c factualapi.FactualReadContext, skuID int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.SKUSearchSummary, error) {
	var out factualapi.SKUSearchSummary
	if err := s.validate(c); err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	var summaryID, sourceSKU int64
	var canonical pgtype.Int8
	var resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion string
	err = s.pool.QueryRow(ctx, `SELECT x.id,x.canonical_sku_id,x.source_sku,x.sku_resolution_status,x.period_kind,x.provider_search_gmv::text,x.orders::text,COALESCE(x.unique_search_users::text,''),COALESCE(x.unique_view_users::text,''),COALESCE(x.position::text,''),COALESCE(x.view_conversion::text,''),x.period_start,x.period_end FROM search_sku_summaries x WHERE x.seller_account_id=$1 AND x.canonical_sku_id=$2 AND x.period_start=$3 AND x.period_end=$4 AND x.observed_at<=$5 ORDER BY x.observed_at DESC,x.revision_no DESC,x.id DESC LIMIT 1`, c.SellerAccountID(), skuID, p.From, p.To, c.CutoffAt()).Scan(&summaryID, &canonical, &sourceSKU, &resolution, &periodKind, &gmv, &orders, &searchUsers, &viewUsers, &position, &conversion, &p.From, &p.To)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("SKU search summary")
	}
	if err != nil {
		return out, dbError(err)
	}
	var cid *int64
	if canonical.Valid {
		v := canonical.Int64
		cid = &v
	}
	return s.mapSKUSearchSummary(c, summaryID, cid, sourceSKU, resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion, p)
}

func (s *Service) BatchGetSKUSearchSummary(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchSKUSearchSummaryResult, error) {
	var out factualapi.BatchSKUSearchSummaryResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(canonical_sku_id)id,canonical_sku_id,source_sku,sku_resolution_status,period_kind,provider_search_gmv::text gmv,orders::text orders,COALESCE(unique_search_users::text,'') search_users,COALESCE(unique_view_users::text,'') view_users,COALESCE(position::text,'') position,COALESCE(view_conversion::text,'') conversion,period_start,period_end FROM search_sku_summaries WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND period_start=$3 AND period_end=$4 AND observed_at<=$5 ORDER BY canonical_sku_id,observed_at DESC,revision_no DESC,id DESC)SELECT e.id,e.found,l.id,l.source_sku,l.sku_resolution_status,l.period_kind,l.gmv,l.orders,l.search_users,l.view_users,l.position,l.conversion,l.period_start,l.period_end FROM eligible e LEFT JOIN latest l ON l.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, summaryID pgtype.Int8
		var sourceSKU pgtype.Int8
		var resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion pgtype.Text
		var from, to pgtype.Date
		if err = rows.Scan(&id, &found, &summaryID, &sourceSKU, &resolution, &periodKind, &gmv, &orders, &searchUsers, &viewUsers, &position, &conversion, &from, &to); err != nil {
			return out, dbError(err)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUSearchSummaryItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		if !summaryID.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUSearchSummaryItem{ID: id, Status: factualapi.BatchLimited})
			continue
		}
		cid := found.Int64
		period := factualapi.Period{From: from.Time, To: to.Time}
		fact, mapErr := s.mapSKUSearchSummary(c, summaryID.Int64, &cid, sourceSKU.Int64, resolution.String, periodKind.String, gmv.String, orders.String, searchUsers.String, viewUsers.String, position.String, conversion.String, period)
		if mapErr != nil {
			return out, mapErr
		}
		v := fact
		out.Items = append(out.Items, factualapi.BatchSKUSearchSummaryItem{ID: id, Status: factualapi.BatchFound, Fact: &v})
	}
	return out, rows.Err()
}

func (s *Service) ListSKUSearchSummary(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period, page factualapi.PageRequest) (factualapi.Page[factualapi.SKUSearchSummary], error) {
	var out factualapi.Page[factualapi.SKUSearchSummary]
	if err := s.validate(c); err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, h, err := s.cursor(c, "ListSKUSearchSummary", p, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(source_sku,period_start,period_end)* FROM search_sku_summaries WHERE seller_account_id=$1 AND period_start=$2 AND period_end=$3 AND observed_at<=$4 ORDER BY source_sku,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)SELECT id,canonical_sku_id,source_sku,sku_resolution_status,period_kind,provider_search_gmv::text,orders::text,COALESCE(unique_search_users::text,''),COALESCE(unique_view_users::text,''),COALESCE(position::text,''),COALESCE(view_conversion::text,''),period_start,period_end FROM latest WHERE id>$5 ORDER BY id LIMIT $6`, c.SellerAccountID(), p.From, p.To, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var summaryID, sourceSKU int64
		var canonical pgtype.Int8
		var resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion string
		var period factualapi.Period
		if err = rows.Scan(&summaryID, &canonical, &sourceSKU, &resolution, &periodKind, &gmv, &orders, &searchUsers, &viewUsers, &position, &conversion, &period.From, &period.To); err != nil {
			return out, err
		}
		var cid *int64
		if canonical.Valid {
			v := canonical.Int64
			cid = &v
		}
		fact, mapErr := s.mapSKUSearchSummary(c, summaryID, cid, sourceSKU, resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion, period)
		if mapErr != nil {
			return out, mapErr
		}
		out.Items = append(out.Items, fact)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListSKUSearchSummary", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) GetSearchSummarySeries(ctx context.Context, c factualapi.FactualReadContext, skuID int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) ([]factualapi.SKUSearchSummary, error) {
	if err := s.validate(c); err != nil {
		return nil, err
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(period_start,period_end)* FROM search_sku_summaries WHERE seller_account_id=$1 AND canonical_sku_id=$2 AND period_start>=$3 AND period_end<=$4 AND observed_at<=$5 ORDER BY period_start,period_end,observed_at DESC,revision_no DESC,id DESC)SELECT id,canonical_sku_id,source_sku,sku_resolution_status,period_kind,provider_search_gmv::text,orders::text,COALESCE(unique_search_users::text,''),COALESCE(unique_view_users::text,''),COALESCE(position::text,''),COALESCE(view_conversion::text,''),period_start,period_end FROM latest ORDER BY period_start,period_end`, c.SellerAccountID(), skuID, p.From, p.To, c.CutoffAt())
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.SKUSearchSummary
	for rows.Next() {
		var summaryID, sourceSKU int64
		var canonical pgtype.Int8
		var resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion string
		var period factualapi.Period
		if err = rows.Scan(&summaryID, &canonical, &sourceSKU, &resolution, &periodKind, &gmv, &orders, &searchUsers, &viewUsers, &position, &conversion, &period.From, &period.To); err != nil {
			return nil, err
		}
		var cid *int64
		if canonical.Valid {
			v := canonical.Int64
			cid = &v
		}
		fact, mapErr := s.mapSKUSearchSummary(c, summaryID, cid, sourceSKU, resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion, period)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, fact)
	}
	return out, rows.Err()
}

func (s *Service) GetSKUQueryDetails(ctx context.Context, c factualapi.FactualReadContext, skuID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.SearchQueryDetail], error) {
	var out factualapi.Page[factualapi.SearchQueryDetail]
	if err := s.validate(c); err != nil {
		return out, err
	}
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, h, err := s.cursor(c, "GetSKUQueryDetails", skuID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(search_query_id,source_sku,period_start,period_end)* FROM search_query_details WHERE seller_account_id=$1 AND canonical_sku_id=$2 AND observed_at<=$3 ORDER BY search_query_id,source_sku,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)SELECT id,search_query_id,canonical_sku_id,source_sku,sku_resolution_status,provider_rank,provider_search_gmv::text,orders::text,retrieval_scope,retrieval_reason,top_n_limit,COALESCE(limit_by_sku,top_n_limit),COALESCE(sort_by,'orders'),COALESCE(sort_dir,'DESC'),COALESCE(page,1),COALESCE(page_size,100),collection_policy,period_start,period_end FROM latest WHERE id>$4 ORDER BY id LIMIT $5`, c.SellerAccountID(), skuID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.SearchQueryDetail
		var gmv, orders string
		if err = rows.Scan(&x.ID, &x.SearchQueryID, &x.CanonicalSKUID, &x.SourceSKU, &x.SKUResolutionStatus, &x.ProviderRank, &gmv, &orders, &x.RetrievalScope, &x.RetrievalReason, &x.TopNLimit, &x.LimitBySKU, &x.SortBy, &x.SortDir, &x.Page, &x.PageSize, &x.CollectionPolicy, &x.Period.From, &x.Period.To); err != nil {
			return out, err
		}
		x.ProviderSearchGMV, err = searchMoney(gmv)
		if err != nil {
			return out, err
		}
		x.Orders, err = searchDecimal(orders)
		if err != nil {
			return out, err
		}
		x.Meta, _ = s.meta(c, "SEARCH.QUERY_DETAILS", "SEARCH_DETAIL", x.ID, &x.Period, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "GetSKUQueryDetails", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) BatchGetSKUQueryDetails(ctx context.Context, c factualapi.FactualReadContext, skuIDs []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period, perSKULimit int) (factualapi.BatchSKUQueryDetailsResult, error) {
	var out factualapi.BatchSKUQueryDetailsResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(skuIDs) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	if perSKULimit <= 0 || len(skuIDs) == 0 {
		return out, nil
	}
	fetchLimit := perSKULimit + 1
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(search_query_id,source_sku)id,search_query_id,canonical_sku_id,source_sku,sku_resolution_status,provider_rank,provider_search_gmv::text gmv,orders::text orders,retrieval_scope,retrieval_reason,top_n_limit,COALESCE(limit_by_sku,top_n_limit)limit_by_sku,COALESCE(sort_by,'orders')sort_by,COALESCE(sort_dir,'DESC')sort_dir,COALESCE(page,1)page,COALESCE(page_size,100)page_size,collection_policy,period_start,period_end FROM search_query_details WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND period_start=$3 AND period_end=$4 AND observed_at<=$5 ORDER BY search_query_id,source_sku,observed_at DESC,revision_no DESC,id DESC),ranked AS(SELECT l.*,ROW_NUMBER()OVER(PARTITION BY canonical_sku_id ORDER BY provider_rank ASC NULLS LAST,id ASC)rn,COUNT(*)OVER(PARTITION BY canonical_sku_id)total FROM latest l)SELECT e.id,e.found,r.id,r.search_query_id,r.canonical_sku_id,r.source_sku,r.sku_resolution_status,r.provider_rank,r.gmv,r.orders,r.retrieval_scope,r.retrieval_reason,r.top_n_limit,r.limit_by_sku,r.sort_by,r.sort_dir,r.page,r.page_size,r.collection_policy,r.period_start,r.period_end,r.rn,r.total FROM eligible e LEFT JOIN ranked r ON r.canonical_sku_id=e.found AND r.rn<=$6 ORDER BY e.ord,r.rn`, c.SellerAccountID(), skuIDs, p.From, p.To, c.CutoffAt(), fetchLimit)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	type skuBucket struct {
		status  factualapi.BatchStatus
		items   []factualapi.SearchQueryDetail
		hasMore bool
	}
	buckets := map[int64]*skuBucket{}
	for _, id := range skuIDs {
		buckets[id] = &skuBucket{status: factualapi.BatchNotFound}
	}
	for rows.Next() {
		var id int64
		var found, rowID, searchQueryID, canonicalSKU, sourceSKU, providerRank, topN, limitBySKU, page, pageSize, rn, total pgtype.Int8
		var resolution, scope, reason, sortBy, sortDir, policy, gmv, orders pgtype.Text
		var from, to pgtype.Date
		if err = rows.Scan(&id, &found, &rowID, &searchQueryID, &canonicalSKU, &sourceSKU, &resolution, &providerRank, &gmv, &orders, &scope, &reason, &topN, &limitBySKU, &sortBy, &sortDir, &page, &pageSize, &policy, &from, &to, &rn, &total); err != nil {
			return out, dbError(err)
		}
		b := buckets[id]
		if b == nil {
			continue
		}
		if !found.Valid {
			continue
		}
		b.status = factualapi.BatchFound
		if !rowID.Valid {
			continue
		}
		var x factualapi.SearchQueryDetail
		if canonicalSKU.Valid {
			v := canonicalSKU.Int64
			x.CanonicalSKUID = &v
		}
		x.ID = rowID.Int64
		x.SearchQueryID = searchQueryID.Int64
		x.SourceSKU = sourceSKU.Int64
		x.SKUResolutionStatus = resolution.String
		if providerRank.Valid {
			x.ProviderRank = int(providerRank.Int64)
		}
		x.RetrievalScope = scope.String
		x.RetrievalReason = reason.String
		if topN.Valid {
			x.TopNLimit = int(topN.Int64)
		}
		if limitBySKU.Valid {
			x.LimitBySKU = int(limitBySKU.Int64)
		}
		x.SortBy = sortBy.String
		x.SortDir = sortDir.String
		if page.Valid {
			x.Page = int(page.Int64)
		}
		if pageSize.Valid {
			x.PageSize = int(pageSize.Int64)
		}
		x.CollectionPolicy = policy.String
		if from.Valid {
			x.Period.From = from.Time
		}
		if to.Valid {
			x.Period.To = to.Time
		}
		x.ProviderSearchGMV, err = searchMoney(gmv.String)
		if err != nil {
			return out, err
		}
		x.Orders, err = searchDecimal(orders.String)
		if err != nil {
			return out, err
		}
		x.Meta, _ = s.meta(c, "SEARCH.QUERY_DETAILS", "SEARCH_DETAIL", x.ID, &x.Period, factHash(x))
		b.items = append(b.items, x)
		if total.Valid && total.Int64 > int64(perSKULimit) {
			b.hasMore = true
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, id := range skuIDs {
		b := buckets[id]
		if b == nil {
			continue
		}
		out.Items = append(out.Items, factualapi.BatchSKUQueryDetailsItem{
			SKUID:   id,
			Status:  b.status,
			Items:   b.items,
			HasMore: b.hasMore,
		})
	}
	return out, nil
}

func (s *Service) GetSearchQueryFact(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SearchQueryFact, error) {
	var out factualapi.SearchQueryFact
	if err := s.validate(c); err != nil {
		return out, err
	}
	// Display as-known-at: latest search_query_versions with observed_at<=cutoff, else seeded search_queries.display_query.
	err := s.pool.QueryRow(ctx, `SELECT q.id,q.provider,q.market,q.normalized_query_key,COALESCE(v.display_query,q.display_query),COALESCE(v.version_no,1) FROM search_queries q LEFT JOIN LATERAL(SELECT display_query,version_no FROM search_query_versions x WHERE x.search_query_id=q.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE q.id=$1`, id, c.CutoffAt()).Scan(&out.ID, &out.Provider, &out.Market, &out.NormalizedKey, &out.QueryText, &out.DisplayVersionNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("search query")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.SellerAccountID = c.SellerAccountID()
	out.Meta, _ = s.meta(c, "SEARCH.SELLER_SUMMARY", "SEARCH_QUERY", id, nil, factHash(out))
	return out, nil
}

func (s *Service) GetCategorySearch(ctx context.Context, c factualapi.FactualReadContext, categoryID int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.SearchCategorySummary, error) {
	out := factualapi.SearchCategorySummary{CategoryID: categoryID}
	if err := s.validate(c); err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	out.Period = p
	// Roll up only when every SKU with search facts has an unambiguous category assignment for the entire period.
	var ambiguous int
	err = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(source_sku)canonical_sku_id FROM search_sku_summaries WHERE seller_account_id=$1 AND period_start=$3 AND period_end=$4 AND observed_at<=$2 AND canonical_sku_id IS NOT NULL ORDER BY source_sku,observed_at DESC,revision_no DESC,id DESC)SELECT count(*) FROM latest l WHERE(SELECT count(*) FROM sku_category_assignments a WHERE a.canonical_sku_id=l.canonical_sku_id AND a.observed_from<=(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(a.observed_to IS NULL OR a.observed_to>($3::date::timestamp AT TIME ZONE 'Europe/Moscow')))<>1`, c.SellerAccountID(), c.CutoffAt(), p.From, p.To).Scan(&ambiguous)
	if err != nil {
		return out, dbError(err)
	}
	if ambiguous > 0 {
		out.ProviderSearchGMV = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "CATEGORY_PERIOD_MAPPING_AMBIGUOUS")
		out.Orders = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "CATEGORY_PERIOD_MAPPING_AMBIGUOUS")
		out.Meta, _ = s.meta(c, "SEARCH.CATEGORY_SUMMARY", "CATEGORY_SEARCH", categoryID, &p, factHash(out))
		return out, nil
	}
	var gmv, orders string
	err = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(source_sku)canonical_sku_id,provider_search_gmv,orders FROM search_sku_summaries WHERE seller_account_id=$1 AND period_start=$3 AND period_end=$4 AND observed_at<=$2 AND canonical_sku_id IS NOT NULL ORDER BY source_sku,observed_at DESC,revision_no DESC,id DESC),in_cat AS(SELECT l.* FROM latest l JOIN sku_category_assignments a ON a.canonical_sku_id=l.canonical_sku_id AND a.seller_category_id=$5 AND a.observed_from<=(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(a.observed_to IS NULL OR a.observed_to>($3::date::timestamp AT TIME ZONE 'Europe/Moscow')))SELECT COALESCE(sum(provider_search_gmv),0)::text,COALESCE(sum(orders),0)::text FROM in_cat`, c.SellerAccountID(), c.CutoffAt(), p.From, p.To, categoryID).Scan(&gmv, &orders)
	if err != nil {
		return out, dbError(err)
	}
	out.ProviderSearchGMV, err = searchMoney(gmv)
	if err != nil {
		return out, err
	}
	out.Orders, err = searchDecimal(orders)
	if err != nil {
		return out, err
	}
	out.Meta, _ = s.meta(c, "SEARCH.CATEGORY_SUMMARY", "CATEGORY_SEARCH", categoryID, &p, factHash(out))
	return out, nil
}

func (s *Service) GetCategorySearchSummary(ctx context.Context, c factualapi.FactualReadContext, categoryID int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.SearchCategorySummary, error) {
	return s.GetCategorySearch(ctx, c, categoryID, standard, explicit)
}

func (s *Service) ListMarketSearchFacts(ctx context.Context, c factualapi.FactualReadContext, page factualapi.PageRequest) (factualapi.Page[factualapi.MarketSearchFact], error) {
	var out factualapi.Page[factualapi.MarketSearchFact]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, h, err := s.cursor(c, "ListMarketSearchFacts", struct{}{}, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,search_query_id,provider_rank,period_start,period_end FROM search_market_facts WHERE seller_account_id=$1 AND observed_at<=$2 AND id>$3 ORDER BY id LIMIT $4`, c.SellerAccountID(), c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		var x factualapi.MarketSearchFact
		if err = rows.Scan(&id, &x.SearchQueryID, &x.ProviderRank, &x.Period.From, &x.Period.To); err != nil {
			return out, err
		}
		x.Meta, _ = s.meta(c, "SEARCH.MARKET_SEARCH", "MARKET_SEARCH", x.SearchQueryID, &x.Period, factHash(x))
		out.Items = append(out.Items, x)
		ids = append(ids, id)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
		ids = ids[:n]
	}
	if len(ids) > 0 {
		out.NextPageToken, err = s.next(c, "ListMarketSearchFacts", h, ids[len(ids)-1], has)
	}
	return out, err
}
