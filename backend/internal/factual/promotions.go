package factual

import (
	"context"
	"errors"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type promotionScanner interface{ Scan(...any) error }

const promotionSelect = `SELECT p.id,p.provider_action_id,v.version_no,p.promotion_origin,COALESCE(v.title,''),v.provider_state,COALESCE(v.discount_type,''),COALESCE(v.currency_code,''),v.starts_at,v.ends_at,v.discount_value::text,v.observed_at FROM promotions p JOIN LATERAL(SELECT * FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true`

func scanPromotion(row promotionScanner) (factualapi.Promotion, error) {
	var out factualapi.Promotion
	var starts, ends pgtype.Timestamptz
	var discount pgtype.Text
	if err := row.Scan(&out.ID, &out.ProviderActionID, &out.VersionNo, &out.Origin, &out.Title, &out.ProviderState, &out.DiscountType, &out.CurrencyCode, &starts, &ends, &discount, &out.ObservedAt); err != nil {
		return out, err
	}
	if starts.Valid {
		out.StartsAt = &starts.Time
	}
	if ends.Valid {
		out.EndsAt = &ends.Time
	}
	out.DiscountValue = decimalValue(textValue(discount), "DISCOUNT_VALUE_UNAVAILABLE")
	return out, nil
}

func (s *Service) GetPromotion(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Promotion, error) {
	var out factualapi.Promotion
	if err := s.validate(c); err != nil {
		return out, err
	}
	out, err := scanPromotion(s.pool.QueryRow(ctx, promotionSelect+` WHERE p.seller_account_id=$1 AND p.id=$3 AND p.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("promotion")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.Meta, err = s.meta(c, "PROMOTIONS.DEFINITIONS", "PROMOTION", id, nil, factHash(out))
	return out, err
}

func (s *Service) ListPromotions(ctx context.Context, c factualapi.FactualReadContext, filter factualapi.PromotionFilter, page factualapi.PageRequest) (factualapi.Page[factualapi.Promotion], error) {
	var out factualapi.Page[factualapi.Promotion]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListPromotions", filter, page)
	if err != nil {
		return out, err
	}
	var origins, states any = filter.Origins, filter.ProviderStates
	if len(filter.Origins) == 0 {
		origins = nil
	}
	if len(filter.ProviderStates) == 0 {
		states = nil
	}
	rows, err := s.pool.Query(ctx, promotionSelect+` WHERE p.seller_account_id=$1 AND p.first_observed_at<=$2 AND p.id>$3 AND($4::text[] IS NULL OR p.promotion_origin=ANY($4))AND($5::text[] IS NULL OR v.provider_state=ANY($5))ORDER BY p.id LIMIT $6`, c.SellerAccountID(), c.CutoffAt(), last, origins, states, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		item, scanErr := scanPromotion(rows)
		if scanErr != nil {
			return out, scanErr
		}
		item.Meta, _ = s.meta(c, "PROMOTIONS.DEFINITIONS", "PROMOTION", item.ID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListPromotions", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) ListPromotionParticipants(ctx context.Context, c factualapi.FactualReadContext, promotionID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.PromotionParticipant], error) {
	var out factualapi.Page[factualapi.PromotionParticipant]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListPromotionParticipants", promotionID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,canonical_sku_id,source_product_id,source_sku,COALESCE(source_offer_id,''),sku_resolution_status,time_quality,action_price::text,action_stock,known_from,known_to FROM promotion_sku_memberships WHERE seller_account_id=$1 AND promotion_id=$2 AND known_from<=$3 AND(known_to IS NULL OR known_to>$3)AND id>$4 ORDER BY id LIMIT $5`, c.SellerAccountID(), promotionID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	var ids []int64
	for rows.Next() {
		var item factualapi.PromotionParticipant
		var canonical, product, sku, stock pgtype.Int8
		var actionPrice pgtype.Text
		var knownTo pgtype.Timestamptz
		var id int64
		if err = rows.Scan(&id, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &item.TimeQuality, &actionPrice, &stock, &item.KnownFrom, &knownTo); err != nil {
			rows.Close()
			return out, err
		}
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		out.Items = append(out.Items, item)
		ids = append(ids, id)
	}
	rows.Close()
	has := len(out.Items) > n
	if has {
		out.Items, ids = out.Items[:n], ids[:n]
	}
	if len(ids) > 0 {
		out.NextPageToken, err = s.next(c, "ListPromotionParticipants", hash, ids[len(ids)-1], has)
	}
	return out, err
}

func (s *Service) ListPromotionCandidates(ctx context.Context, c factualapi.FactualReadContext, promotionID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.PromotionCandidate], error) {
	var out factualapi.Page[factualapi.PromotionCandidate]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListPromotionCandidates", promotionID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH snapshot AS(SELECT * FROM promotion_eligibility_snapshot_runs WHERE seller_account_id=$1 AND promotion_id=$2 AND observed_at<=$3 ORDER BY observed_at DESC,id DESC LIMIT 1)SELECT e.id,e.canonical_sku_id,e.source_product_id,e.source_sku,COALESCE(e.source_offer_id,''),e.sku_resolution_status,e.action_price::text,e.action_stock,s.observed_at,s.completeness_status FROM snapshot s JOIN promotion_sku_eligibility_snapshots e ON e.snapshot_run_id=s.id WHERE e.id>$4 ORDER BY e.id LIMIT $5`, c.SellerAccountID(), promotionID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	var ids []int64
	for rows.Next() {
		var item factualapi.PromotionCandidate
		var canonical, product, sku, stock pgtype.Int8
		var actionPrice pgtype.Text
		var id int64
		if err = rows.Scan(&id, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &actionPrice, &stock, &item.ObservedAt, &item.CompletenessStatus); err != nil {
			rows.Close()
			return out, err
		}
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		out.Items = append(out.Items, item)
		ids = append(ids, id)
	}
	rows.Close()
	has := len(out.Items) > n
	if has {
		out.Items, ids = out.Items[:n], ids[:n]
	}
	if len(ids) > 0 {
		out.NextPageToken, err = s.next(c, "ListPromotionCandidates", hash, ids[len(ids)-1], has)
	}
	return out, err
}

func (s *Service) GetSKUPromotions(ctx context.Context, c factualapi.FactualReadContext, skuID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.SKUPromotion], error) {
	var out factualapi.Page[factualapi.SKUPromotion]
	if err := s.validate(c); err != nil {
		return out, err
	}
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "GetSKUPromotions", skuID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT m.id,p.id,p.provider_action_id,v.version_no,p.promotion_origin,COALESCE(v.title,''),v.provider_state,COALESCE(v.discount_type,''),COALESCE(v.currency_code,''),v.starts_at,v.ends_at,v.discount_value::text,v.observed_at,m.canonical_sku_id,m.source_product_id,m.source_sku,COALESCE(m.source_offer_id,''),m.sku_resolution_status,m.time_quality,m.action_price::text,m.action_stock,m.known_from,m.known_to FROM promotion_sku_memberships m JOIN promotions p ON p.id=m.promotion_id AND p.seller_account_id=m.seller_account_id JOIN LATERAL(SELECT * FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE m.seller_account_id=$1 AND m.canonical_sku_id=$3 AND m.known_from<=$2 AND(m.known_to IS NULL OR m.known_to>$2)AND p.first_observed_at<=$2 AND m.id>$4 ORDER BY m.id LIMIT $5`, c.SellerAccountID(), c.CutoffAt(), skuID, last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item factualapi.SKUPromotion
		var starts, ends, knownTo pgtype.Timestamptz
		var discount, actionPrice pgtype.Text
		var canonical, product, sku, stock pgtype.Int8
		if err = rows.Scan(&item.MembershipID, &item.ID, &item.ProviderActionID, &item.VersionNo, &item.Origin, &item.Title, &item.ProviderState, &item.DiscountType, &item.CurrencyCode, &starts, &ends, &discount, &item.ObservedAt, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &item.TimeQuality, &actionPrice, &stock, &item.KnownFrom, &knownTo); err != nil {
			return out, err
		}
		if starts.Valid {
			item.StartsAt = &starts.Time
		}
		if ends.Valid {
			item.EndsAt = &ends.Time
		}
		item.DiscountValue = decimalValue(textValue(discount), "DISCOUNT_VALUE_UNAVAILABLE")
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		item.Meta, _ = s.metaWithAux(c, "PROMOTIONS.MEMBERSHIP", "SKU_PROMOTION", skuID, item.MembershipID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "GetSKUPromotions", hash, out.Items[len(out.Items)-1].MembershipID, has)
	}
	return out, err
}

func (s *Service) BatchGetSKUPromotions(ctx context.Context, c factualapi.FactualReadContext, skuIDs []int64, perSKULimit int) (factualapi.BatchSKUPromotionsResult, error) {
	var out factualapi.BatchSKUPromotionsResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(skuIDs) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if perSKULimit <= 0 || len(skuIDs) == 0 {
		return out, nil
	}
	fetchLimit := perSKULimit + 1
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$3),memberships AS(SELECT m.id membership_id,p.id promotion_id,p.provider_action_id,v.version_no,p.promotion_origin,COALESCE(v.title,'')title,v.provider_state,COALESCE(v.discount_type,'')discount_type,COALESCE(v.currency_code,'')currency_code,v.starts_at,v.ends_at,v.discount_value::text discount,v.observed_at,m.canonical_sku_id,m.source_product_id,m.source_sku,COALESCE(m.source_offer_id,'')source_offer_id,m.sku_resolution_status,m.time_quality,m.action_price::text action_price,m.action_stock,m.known_from,m.known_to FROM promotion_sku_memberships m JOIN promotions p ON p.id=m.promotion_id AND p.seller_account_id=m.seller_account_id JOIN LATERAL(SELECT * FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE m.seller_account_id=$1 AND m.canonical_sku_id=ANY($2)AND m.known_from<=$3 AND(m.known_to IS NULL OR m.known_to>$3)AND p.first_observed_at<=$3),ranked AS(SELECT m.*,ROW_NUMBER()OVER(PARTITION BY canonical_sku_id ORDER BY membership_id)rn,COUNT(*)OVER(PARTITION BY canonical_sku_id)total FROM memberships m)SELECT e.id,e.found,r.membership_id,r.promotion_id,r.provider_action_id,r.version_no,r.promotion_origin,r.title,r.provider_state,r.discount_type,r.currency_code,r.starts_at,r.ends_at,r.discount,r.observed_at,r.canonical_sku_id,r.source_product_id,r.source_sku,r.source_offer_id,r.sku_resolution_status,r.time_quality,r.action_price,r.action_stock,r.known_from,r.known_to,r.rn,r.total FROM eligible e LEFT JOIN ranked r ON r.canonical_sku_id=e.found AND r.rn<=$4 ORDER BY e.ord,r.rn`, c.SellerAccountID(), skuIDs, c.CutoffAt(), fetchLimit)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	type skuBucket struct {
		status  factualapi.BatchStatus
		items   []factualapi.SKUPromotion
		hasMore bool
	}
	buckets := map[int64]*skuBucket{}
	for _, id := range skuIDs {
		buckets[id] = &skuBucket{status: factualapi.BatchNotFound}
	}
	for rows.Next() {
		var id int64
		var found, membershipID, promotionID, providerActionID, versionNo, canonical, product, sku, stock, rn, total pgtype.Int8
		var origin, title, state, discountType, currency, offerID, resolution, timeQuality pgtype.Text
		var starts, ends, knownFrom, knownTo, observedAt pgtype.Timestamptz
		var discount, actionPrice pgtype.Text
		if err = rows.Scan(&id, &found, &membershipID, &promotionID, &providerActionID, &versionNo, &origin, &title, &state, &discountType, &currency, &starts, &ends, &discount, &observedAt, &canonical, &product, &sku, &offerID, &resolution, &timeQuality, &actionPrice, &stock, &knownFrom, &knownTo, &rn, &total); err != nil {
			return out, err
		}
		b := buckets[id]
		if b == nil {
			continue
		}
		if !found.Valid {
			continue
		}
		b.status = factualapi.BatchFound
		if !membershipID.Valid {
			continue
		}
		var item factualapi.SKUPromotion
		item.MembershipID = membershipID.Int64
		item.ID = promotionID.Int64
		item.ProviderActionID = providerActionID.Int64
		item.VersionNo = versionNo.Int64
		item.Origin = origin.String
		item.Title = title.String
		item.ProviderState = state.String
		item.DiscountType = discountType.String
		item.CurrencyCode = currency.String
		if starts.Valid {
			item.StartsAt = &starts.Time
		}
		if ends.Valid {
			item.EndsAt = &ends.Time
		}
		item.DiscountValue = decimalValue(textValue(discount), "DISCOUNT_VALUE_UNAVAILABLE")
		if observedAt.Valid {
			item.ObservedAt = observedAt.Time
		}
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.SourceOfferID = offerID.String
		item.ResolutionStatus = resolution.String
		item.TimeQuality = timeQuality.String
		if knownFrom.Valid {
			item.KnownFrom = knownFrom.Time
		}
		item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		item.Meta, _ = s.metaWithAux(c, "PROMOTIONS.MEMBERSHIP", "SKU_PROMOTION", id, item.MembershipID, nil, factHash(item))
		b.items = append(b.items, item)
		if total.Valid && total.Int64 > int64(perSKULimit) {
			b.hasMore = true
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, skuID := range skuIDs {
		b := buckets[skuID]
		if b == nil {
			continue
		}
		out.Items = append(out.Items, factualapi.BatchSKUPromotionsItem{
			SKUID:   skuID,
			Status:  b.status,
			Items:   b.items,
			HasMore: b.hasMore,
		})
	}
	return out, nil
}

func (s *Service) BatchGetSKUPromotionSummary(ctx context.Context, c factualapi.FactualReadContext, skuIDs []int64) (factualapi.BatchSKUPromotionSummaryResult, error) {
	var out factualapi.BatchSKUPromotionSummaryResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(skuIDs) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(skuIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$3),active AS(SELECT m.canonical_sku_id,count(DISTINCT m.promotion_id)::bigint cnt FROM promotion_sku_memberships m JOIN promotions p ON p.id=m.promotion_id AND p.seller_account_id=m.seller_account_id WHERE m.seller_account_id=$1 AND m.canonical_sku_id=ANY($2)AND m.known_from<=$3 AND(m.known_to IS NULL OR m.known_to>$3)AND p.first_observed_at<=$3 GROUP BY m.canonical_sku_id),latest_snap AS(SELECT DISTINCT ON(promotion_id)id,promotion_id FROM promotion_eligibility_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$3 ORDER BY promotion_id,observed_at DESC,id DESC),candidates AS(SELECT e.canonical_sku_id,count(DISTINCT ls.promotion_id)::bigint cnt FROM promotion_sku_eligibility_snapshots e JOIN latest_snap ls ON ls.id=e.snapshot_run_id WHERE e.seller_account_id=$1 AND e.canonical_sku_id=ANY($2)AND e.canonical_sku_id IS NOT NULL GROUP BY e.canonical_sku_id)SELECT e.id,e.found,COALESCE(a.cnt,0),COALESCE(c.cnt,0)FROM eligible e LEFT JOIN active a ON a.canonical_sku_id=e.found LEFT JOIN candidates c ON c.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), skuIDs, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, activeCount, candidateCount pgtype.Int8
		if err = rows.Scan(&id, &found, &activeCount, &candidateCount); err != nil {
			return out, err
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUPromotionSummaryItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		summary := factualapi.SKUPromotionSummary{
			CanonicalSKUID:          id,
			ActivePromotionCount:    intValue(activeCount, "ACTIVE_PROMOTION_COUNT_UNAVAILABLE"),
			CandidatePromotionCount: intValue(candidateCount, "CANDIDATE_PROMOTION_COUNT_UNAVAILABLE"),
		}
		summary.Meta, err = s.meta(c, "PROMOTIONS.MEMBERSHIP", "SKU_PROMOTION_SUMMARY", id, nil, factHash(summary))
		if err != nil {
			return out, err
		}
		fact := summary
		out.Items = append(out.Items, factualapi.BatchSKUPromotionSummaryItem{ID: id, Status: factualapi.BatchFound, Summary: &fact})
	}
	return out, rows.Err()
}

func (s *Service) GetPromotionAccountSummary(ctx context.Context, c factualapi.FactualReadContext) (factualapi.PromotionAccountSummary, error) {
	var out factualapi.PromotionAccountSummary
	if err := s.validate(c); err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, factualapi.PeriodCurrent7D, nil)
	if err != nil {
		return out, err
	}
	var activeProv, activeSeller, participating, candidates pgtype.Int8
	var partAmount, totalAmount pgtype.Text
	var currencies int64
	var currency pgtype.Text
	err = s.pool.QueryRow(ctx, `WITH active_promos AS(SELECT p.id,p.promotion_origin origin FROM promotions p JOIN LATERAL(SELECT provider_state FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE p.seller_account_id=$1 AND p.first_observed_at<=$2 AND upper(v.provider_state)IN('ACTIVE','RUNNING')),participants AS(SELECT DISTINCT m.canonical_sku_id FROM promotion_sku_memberships m WHERE m.seller_account_id=$1 AND m.known_from<=$2 AND(m.known_to IS NULL OR m.known_to>$2)AND m.canonical_sku_id IS NOT NULL),latest_snap AS(SELECT DISTINCT ON(promotion_id)id FROM promotion_eligibility_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY promotion_id,observed_at DESC,id DESC),candidate_skus AS(SELECT DISTINCT e.canonical_sku_id FROM promotion_sku_eligibility_snapshots e JOIN latest_snap ls ON ls.id=e.snapshot_run_id WHERE e.seller_account_id=$1 AND e.canonical_sku_id IS NOT NULL),sales AS(SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,ordered_amount,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND business_date BETWEEN $3 AND $4 AND observed_at<=$2 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC),sales_agg AS(SELECT CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)END total,CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)FILTER(WHERE canonical_sku_id IN(SELECT canonical_sku_id FROM participants))END participating,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM sales)SELECT (SELECT count(*)FROM active_promos WHERE origin<>'SELLER'),(SELECT count(*)FROM active_promos WHERE origin='SELLER'),(SELECT count(*)FROM participants),(SELECT count(*)FROM candidate_skus),sa.participating::text,sa.total::text,sa.currencies,sa.currency FROM sales_agg sa`, c.SellerAccountID(), c.CutoffAt(), p.From, p.To).Scan(&activeProv, &activeSeller, &participating, &candidates, &partAmount, &totalAmount, &currencies, &currency)
	if err != nil {
		return out, dbError(err)
	}
	out.ActiveProviderPromotionCount = intValue(activeProv, "ACTIVE_PROVIDER_PROMOTION_COUNT_UNAVAILABLE")
	out.ActiveSellerPromotionCount = intValue(activeSeller, "ACTIVE_SELLER_PROMOTION_COUNT_UNAVAILABLE")
	out.ParticipatingSKUCount = intValue(participating, "PARTICIPATING_SKU_COUNT_UNAVAILABLE")
	out.CandidateSKUCount = intValue(candidates, "CANDIDATE_SKU_COUNT_UNAVAILABLE")
	if currencies > 1 {
		out.SalesExposureParticipating = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
	} else if !totalAmount.Valid || totalAmount.String == "" {
		out.SalesExposureParticipating = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "SALES_EXPOSURE_UNAVAILABLE")
	} else {
		code := textValue(currency)
		if code == "" {
			code = "RUB"
		}
		out.SalesExposureParticipating = ratioMoney(
			moneyValue(partAmount, code, "SALES_EXPOSURE_UNAVAILABLE"),
			moneyValue(totalAmount, code, "SALES_EXPOSURE_UNAVAILABLE"),
		)
	}
	out.Meta, err = s.meta(c, "PROMOTIONS.DEFINITIONS", "PROMOTION_ACCOUNT_SUMMARY", c.SellerAccountID(), nil, factHash(out))
	return out, err
}

func (s *Service) GetPromotionEligibility(ctx context.Context, c factualapi.FactualReadContext, promotionID int64, page factualapi.PageRequest) (factualapi.PromotionEligibility, error) {
	var out factualapi.PromotionEligibility
	if err := s.validate(c); err != nil {
		return out, err
	}
	out.PromotionID = promotionID
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "GetPromotionEligibility", promotionID, page)
	if err != nil {
		return out, err
	}
	err = s.pool.QueryRow(ctx, `SELECT id,observed_at,completeness_status FROM promotion_eligibility_snapshot_runs WHERE seller_account_id=$1 AND promotion_id=$2 AND observed_at<=$3 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), promotionID, c.CutoffAt()).Scan(&out.SnapshotID, &out.ObservedAt, &out.CompletenessStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		out.Meta, err = s.meta(c, "PROMOTIONS.ELIGIBILITY", "PROMOTION_ELIGIBILITY", promotionID, nil, factHash(out))
		return out, err
	}
	if err != nil {
		return out, dbError(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id,canonical_sku_id,source_product_id,source_sku,COALESCE(source_offer_id,''),sku_resolution_status,action_price::text,action_stock FROM promotion_sku_eligibility_snapshots WHERE seller_account_id=$1 AND snapshot_run_id=$2 AND id>$3 ORDER BY id LIMIT $4`, c.SellerAccountID(), out.SnapshotID, last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	var ids []int64
	for rows.Next() {
		var item factualapi.PromotionCandidate
		var canonical, product, sku, stock pgtype.Int8
		var actionPrice pgtype.Text
		var id int64
		if err = rows.Scan(&id, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &actionPrice, &stock); err != nil {
			rows.Close()
			return out, err
		}
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		item.ObservedAt = out.ObservedAt
		item.CompletenessStatus = out.CompletenessStatus
		out.Candidates = append(out.Candidates, item)
		ids = append(ids, id)
	}
	rows.Close()
	has := len(out.Candidates) > n
	if has {
		out.Candidates, ids = out.Candidates[:n], ids[:n]
	}
	if len(ids) > 0 {
		out.NextPageToken, err = s.next(c, "GetPromotionEligibility", hash, ids[len(ids)-1], has)
		if err != nil {
			return out, err
		}
	}
	out.Meta, err = s.meta(c, "PROMOTIONS.ELIGIBILITY", "PROMOTION_ELIGIBILITY", promotionID, nil, factHash(out))
	return out, err
}

func (s *Service) GetPromotionMembershipHistory(ctx context.Context, c factualapi.FactualReadContext, promotionID int64, filter factualapi.PromotionMembershipFilter, page factualapi.PageRequest) (factualapi.Page[factualapi.PromotionMembershipInterval], error) {
	var out factualapi.Page[factualapi.PromotionMembershipInterval]
	if err := s.validate(c); err != nil {
		return out, err
	}
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "GetPromotionMembershipHistory", struct {
		PromotionID int64
		Filter      factualapi.PromotionMembershipFilter
	}{promotionID, filter}, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,promotion_id,canonical_sku_id,source_product_id,source_sku,COALESCE(source_offer_id,''),sku_resolution_status,time_quality,action_price::text,action_stock,known_from,known_to FROM promotion_sku_memberships WHERE seller_account_id=$1 AND promotion_id=$2 AND known_from<=$3 AND id>$4 AND($5::bigint IS NULL OR canonical_sku_id=$5)ORDER BY id LIMIT $6`, c.SellerAccountID(), promotionID, c.CutoffAt(), last, filter.CanonicalSKUID, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item factualapi.PromotionMembershipInterval
		var canonical, product, sku, stock pgtype.Int8
		var actionPrice pgtype.Text
		var knownTo pgtype.Timestamptz
		if err = rows.Scan(&item.ID, &item.PromotionID, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &item.TimeQuality, &actionPrice, &stock, &item.KnownFrom, &knownTo); err != nil {
			return out, err
		}
		assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
		item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
		item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
		item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
		item.Meta, _ = s.meta(c, "PROMOTIONS.HISTORY", "PROMOTION_MEMBERSHIP_INTERVAL", item.ID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "GetPromotionMembershipHistory", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

// knownToAsOf hides closures observed after the frozen cutoff so AS-KNOWN-AT
// reads never leak future membership ends.
func knownToAsOf(knownTo pgtype.Timestamptz, cutoff time.Time) *time.Time {
	if !knownTo.Valid || knownTo.Time.After(cutoff) {
		return nil
	}
	t := knownTo.Time.UTC()
	return &t
}

func assignIDs(canonicalOut, productOut, skuOut **int64, canonical, product, sku pgtype.Int8) {
	if canonical.Valid {
		v := canonical.Int64
		*canonicalOut = &v
	}
	if product.Valid {
		v := product.Int64
		*productOut = &v
	}
	if sku.Valid {
		v := sku.Int64
		*skuOut = &v
	}
}
