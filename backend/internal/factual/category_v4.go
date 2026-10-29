package factual

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/funnelregistry"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) BatchGetCategoryFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, ids []int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchCategoryFunnelPeriodResult, error) {
	var out factualapi.BatchCategoryFunnelPeriodResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	entry, ok := funnelregistry.LookupByCanonicalKey(metric)
	if !ok {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "unknown funnel metric")
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	if entry.TemporalSemantics != funnelregistry.AdditiveFlow {
		for _, id := range ids {
			item := factualapi.CategoryFunnelPeriod{CategoryID: id}
			item.Metric = factualapi.FunnelMetricValue{
				MetricKey: metric, Unit: entry.Unit, TemporalSemantics: string(entry.TemporalSemantics),
				EntityAggregationSemantics: string(entry.EntityAggregationSemantics), Surface: string(entry.Surface),
				ProviderMetricName: entry.ProviderMetric,
				Value:              factualapi.Missing[factualapi.Decimal](factualapi.ValueNotApplicable, "NON_ADDITIVE_CATEGORY_ROLLUP_FORBIDDEN"),
			}
			item.Metric.Meta, _ = s.meta(c, "FUNNEL.CATEGORY_ADDITIVE", "CATEGORY_FUNNEL", id, &p, factHash(item))
			out.Items = append(out.Items, factualapi.BatchCategoryFunnelPeriodItem{ID: id, Status: factualapi.BatchFound, Funnel: &item})
		}
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($6::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,sc.id found FROM requested r LEFT JOIN seller_categories sc ON sc.id=r.id AND sc.seller_account_id=$1 AND sc.first_observed_at<=$5),facts AS(SELECT DISTINCT ON(canonical_sku_id,period_start)canonical_sku_id,period_start,value_numeric FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND metric_key=$2 AND period_start BETWEEN $3 AND $4 AND period_end=period_start AND observed_at<=$5 ORDER BY canonical_sku_id,period_start,observed_at DESC,revision_no DESC,id DESC),classified AS(SELECT f.*,a.category_id,a.stable FROM facts f LEFT JOIN LATERAL(SELECT max(seller_category_id)category_id,(count(*)=1 AND bool_and(mapping_status='MAPPED')AND min(observed_from)<=($3::date::timestamp AT TIME ZONE 'Europe/Moscow')AND bool_and(observed_to IS NULL OR observed_to>(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')))stable FROM sku_category_assignments x WHERE x.canonical_sku_id=f.canonical_sku_id AND x.observed_from<=(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(x.observed_to IS NULL OR x.observed_to>($3::date::timestamp AT TIME ZONE 'Europe/Moscow'))AND x.observed_from<=$5)a ON true),agg AS(SELECT category_id,COALESCE(sum(value_numeric)FILTER(WHERE stable),0)::text value,count(DISTINCT period_start)FILTER(WHERE stable)::int days,count(*)FILTER(WHERE NOT COALESCE(stable,false))::int ambiguous FROM classified WHERE category_id=ANY($6)GROUP BY category_id)SELECT e.id,e.found,a.value,a.days,a.ambiguous FROM eligible e LEFT JOIN agg a ON a.category_id=e.found ORDER BY e.ord`, c.SellerAccountID(), metric, p.From, p.To, c.CutoffAt(), ids)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	wantDays := periodDays(p)
	for rows.Next() {
		var id int64
		var found pgtype.Int8
		var value pgtype.Text
		var days, ambiguous pgtype.Int4
		if err = rows.Scan(&id, &found, &value, &days, &ambiguous); err != nil {
			return out, dbError(err)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCategoryFunnelPeriodItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		item := factualapi.CategoryFunnelPeriod{CategoryID: id}
		item.Metric = factualapi.FunnelMetricValue{
			MetricKey: metric, Unit: entry.Unit, TemporalSemantics: string(entry.TemporalSemantics),
			EntityAggregationSemantics: string(entry.EntityAggregationSemantics), Surface: string(entry.Surface),
			ProviderMetricName: entry.ProviderMetric,
		}
		status := factualapi.BatchFound
		if ambiguous.Valid && ambiguous.Int32 > 0 {
			item.Metric.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "CATEGORY_PERIOD_MAPPING_AMBIGUOUS")
			status = factualapi.BatchLimited
		} else if !days.Valid || days.Int32 != int32(wantDays) {
			item.Metric.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MISSING_DAILY_FUNNEL_FACT")
			status = factualapi.BatchLimited
		} else {
			d, parseErr := factualapi.ParseDecimal(value.String)
			if parseErr != nil {
				return out, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid funnel decimal")
			}
			item.Metric.Value = factualapi.Known(d)
		}
		item.Metric.Meta, _ = s.meta(c, "FUNNEL.CATEGORY_ADDITIVE", "CATEGORY_FUNNEL", id, &p, factHash(item))
		funnel := item
		out.Items = append(out.Items, factualapi.BatchCategoryFunnelPeriodItem{ID: id, Status: status, Funnel: &funnel})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetCategorySearchDistribution(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchCategorySearchDistributionResult, error) {
	var out factualapi.BatchCategorySearchDistributionResult
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
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,sc.id found FROM requested r LEFT JOIN seller_categories sc ON sc.id=r.id AND sc.seller_account_id=$1 AND sc.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(canonical_sku_id)canonical_sku_id,position FROM search_sku_summaries WHERE seller_account_id=$1 AND period_start=$3 AND period_end=$4 AND observed_at<=$5 AND canonical_sku_id IS NOT NULL ORDER BY canonical_sku_id,observed_at DESC,revision_no DESC,id DESC),mapped AS(SELECT l.canonical_sku_id,l.position,a.category_id,a.stable,a.overlap_ids FROM latest l LEFT JOIN LATERAL(SELECT max(seller_category_id)FILTER(WHERE mapping_status='MAPPED')category_id,(count(*)=1 AND bool_and(mapping_status='MAPPED')AND min(observed_from)<=($3::date::timestamp AT TIME ZONE 'Europe/Moscow')AND bool_and(observed_to IS NULL OR observed_to>(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')))stable,array_agg(DISTINCT seller_category_id)overlap_ids FROM sku_category_assignments x WHERE x.canonical_sku_id=l.canonical_sku_id AND x.observed_from<=(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(x.observed_to IS NULL OR x.observed_to>($3::date::timestamp AT TIME ZONE 'Europe/Moscow'))AND x.observed_from<=$5)a ON true)SELECT e.id,e.found,m.canonical_sku_id,m.position::text,m.stable,m.category_id,m.overlap_ids FROM eligible e LEFT JOIN mapped m ON(m.stable AND m.category_id=e.found)OR(NOT COALESCE(m.stable,false) AND e.found=ANY(m.overlap_ids)) ORDER BY e.ord,m.canonical_sku_id`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	type catAcc struct {
		found    bool
		stable   []exactPosition
		excluded int64
		skuCount int64
	}
	acc := map[int64]*catAcc{}
	order := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := acc[id]; !ok {
			acc[id] = &catAcc{}
			order = append(order, id)
		}
	}
	for rows.Next() {
		var id int64
		var found, skuID, categoryID pgtype.Int8
		var position pgtype.Text
		var stable pgtype.Bool
		var overlap []int64
		if err = rows.Scan(&id, &found, &skuID, &position, &stable, &categoryID, &overlap); err != nil {
			return out, dbError(err)
		}
		a := acc[id]
		if a == nil {
			a = &catAcc{}
			acc[id] = a
			order = append(order, id)
		}
		if found.Valid {
			a.found = true
		}
		if !found.Valid || !skuID.Valid {
			continue
		}
		if stable.Valid && stable.Bool {
			a.skuCount++
			if position.Valid && position.String != "" {
				if d, parseErr := factualapi.ParseDecimal(position.String); parseErr == nil {
					if pos, posErr := parseExactPosition(d); posErr == nil {
						a.stable = append(a.stable, pos)
					}
				}
			}
			continue
		}
		a.excluded++
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, id := range order {
		a := acc[id]
		if a == nil || !a.found {
			out.Items = append(out.Items, factualapi.BatchCategorySearchDistributionItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		v := factualapi.CategorySearchDistribution{
			CategoryID:    id,
			Period:        p,
			FormulaPolicy: "nearest_rank_quantile_v1",
		}
		v.SKUWithSearchSummaryCount = factualapi.Known(a.skuCount)
		v.EligibleStableSKUCount = factualapi.Known(int64(len(a.stable)))
		v.MappingExcludedSKUCount = factualapi.Known(a.excluded)
		if len(a.stable) == 0 && a.skuCount == 0 && a.excluded == 0 {
			v.Meta, err = s.meta(c, "SEARCH.CATEGORY_SUMMARY", "CATEGORY_SEARCH_DISTRIBUTION", id, &p, factHash(v))
			if err != nil {
				return out, err
			}
			fact := v
			out.Items = append(out.Items, factualapi.BatchCategorySearchDistributionItem{ID: id, Status: factualapi.BatchFoundEmpty, Distribution: &fact})
			continue
		}
		if p25 := nearestRankExact(a.stable, 1, 4); p25 != nil {
			v.PositionP25 = factualapi.Known(*p25)
		} else {
			v.PositionP25 = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "POSITION_QUANTILE_UNAVAILABLE")
		}
		if p50 := nearestRankExact(a.stable, 1, 2); p50 != nil {
			v.PositionP50 = factualapi.Known(*p50)
		} else {
			v.PositionP50 = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "POSITION_QUANTILE_UNAVAILABLE")
		}
		if p75 := nearestRankExact(a.stable, 3, 4); p75 != nil {
			v.PositionP75 = factualapi.Known(*p75)
		} else {
			v.PositionP75 = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "POSITION_QUANTILE_UNAVAILABLE")
		}
		den := a.skuCount + a.excluded
		if den > 0 {
			cov := new(big.Rat).SetFrac64(a.skuCount*100, den)
			d, parseErr := factualapi.ParseDecimal(cov.FloatString(6))
			if parseErr != nil {
				v.Coverage = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "COVERAGE_ARITHMETIC_FAILED")
			} else {
				v.Coverage = factualapi.Known(d)
			}
		} else {
			v.Coverage = factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
		}
		v.Meta, err = s.meta(c, "SEARCH.CATEGORY_SUMMARY", "CATEGORY_SEARCH_DISTRIBUTION", id, &p, factHash(v))
		if err != nil {
			return out, err
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCategorySearchDistributionItem{ID: id, Status: factualapi.BatchFound, Distribution: &fact})
	}
	return out, nil
}

func parseExactPosition(d factualapi.Decimal) (exactPosition, error) {
	r, ok := new(big.Rat).SetString(d.String())
	if !ok {
		return exactPosition{}, fmt.Errorf("invalid search position %q", d.String())
	}
	return exactPosition{Raw: d, Rat: r}, nil
}

type exactPosition struct {
	Raw factualapi.Decimal
	Rat *big.Rat
}

func nearestRankExact(values []exactPosition, numerator, denominator int) *factualapi.Decimal {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]exactPosition(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Rat.Cmp(sorted[j].Rat) < 0
	})
	n := len(sorted)
	rank := (numerator*n + denominator - 1) / denominator
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	raw := sorted[rank-1].Raw
	return &raw
}

func (s *Service) BatchGetCategoryPromotionSummary(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchCategoryPromotionSummaryResult, error) {
	var out factualapi.BatchCategoryPromotionSummaryResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,sc.id found FROM requested r LEFT JOIN seller_categories sc ON sc.id=r.id AND sc.seller_account_id=$1 AND sc.first_observed_at<=$3),stable_sku AS(SELECT canonical_sku_id,seller_category_id FROM sku_category_assignments WHERE observed_from<=$3 AND(observed_to IS NULL OR observed_to>$3)AND mapping_status='MAPPED' AND seller_category_id=ANY($2)),participants AS(SELECT DISTINCT m.canonical_sku_id,s.seller_category_id,m.promotion_id FROM promotion_sku_memberships m JOIN stable_sku s ON s.canonical_sku_id=m.canonical_sku_id WHERE m.seller_account_id=$1 AND m.known_from<=$3 AND(m.known_to IS NULL OR m.known_to>$3)AND m.canonical_sku_id IS NOT NULL),active_promos AS(SELECT p.id FROM promotions p JOIN LATERAL(SELECT provider_state FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE p.seller_account_id=$1 AND p.first_observed_at<=$3 AND upper(v.provider_state)IN('ACTIVE','RUNNING')),latest_snap AS(SELECT DISTINCT ON(promotion_id)id,promotion_id FROM promotion_eligibility_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$3 ORDER BY promotion_id,observed_at DESC,id DESC),candidates AS(SELECT DISTINCT e.canonical_sku_id,s.seller_category_id FROM promotion_sku_eligibility_snapshots e JOIN latest_snap ls ON ls.id=e.snapshot_run_id JOIN stable_sku s ON s.canonical_sku_id=e.canonical_sku_id WHERE e.seller_account_id=$1 AND e.canonical_sku_id IS NOT NULL),agg AS(SELECT e.id category_id,(SELECT count(DISTINCT p.promotion_id)FROM participants p JOIN active_promos ap ON ap.id=p.promotion_id WHERE p.seller_category_id=e.id)active_promos,(SELECT count(DISTINCT p.canonical_sku_id)FROM participants p JOIN active_promos ap ON ap.id=p.promotion_id WHERE p.seller_category_id=e.id)participating,(SELECT count(DISTINCT c.canonical_sku_id)FROM candidates c WHERE c.seller_category_id=e.id)candidates FROM eligible e WHERE e.found IS NOT NULL)SELECT e.id,e.found,a.active_promos,a.participating,a.candidates FROM eligible e LEFT JOIN agg a ON a.category_id=e.id ORDER BY e.ord`, c.SellerAccountID(), ids, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, active, participating, candidates pgtype.Int8
		if err = rows.Scan(&id, &found, &active, &participating, &candidates); err != nil {
			return out, dbError(err)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCategoryPromotionSummaryItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		v := factualapi.CategoryPromotionSummary{CategoryID: id}
		v.ActivePromotionCount = intValue(active, "ACTIVE_PROMOTION_COUNT_UNAVAILABLE")
		if !active.Valid {
			v.ActivePromotionCount = factualapi.Known(int64(0))
		}
		v.ParticipatingSKUCount = intValue(participating, "PARTICIPATING_SKU_COUNT_UNAVAILABLE")
		if !participating.Valid {
			v.ParticipatingSKUCount = factualapi.Known(int64(0))
		}
		v.CandidateSKUCount = intValue(candidates, "CANDIDATE_SKU_COUNT_UNAVAILABLE")
		if !candidates.Valid {
			v.CandidateSKUCount = factualapi.Known(int64(0))
		}
		v.Meta, err = s.meta(c, "PROMOTIONS.MEMBERSHIP", "CATEGORY_PROMOTION_SUMMARY", id, nil, factHash(v))
		if err != nil {
			return out, err
		}
		empty := (!active.Valid || active.Int64 == 0) && (!participating.Valid || participating.Int64 == 0) && (!candidates.Valid || candidates.Int64 == 0)
		status := factualapi.BatchFound
		if empty {
			status = factualapi.BatchFoundEmpty
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCategoryPromotionSummaryItem{ID: id, Status: status, Summary: &fact})
	}
	return out, rows.Err()
}
