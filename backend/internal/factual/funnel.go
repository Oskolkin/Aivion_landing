package factual

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/funnelregistry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func funnelCapability(e funnelregistry.Entry) string {
	if e.TemporalSemantics == funnelregistry.AdditiveFlow {
		return "FUNNEL.DAILY_ADDITIVE"
	}
	return "FUNNEL.DIRECT_PERIOD"
}

func (s *Service) funnelValue(ctx context.Context, c factualapi.FactualReadContext, scope string, sku int64, metric string, p factualapi.Period) (factualapi.FunnelMetricValue, error) {
	var out factualapi.FunnelMetricValue
	entry, ok := funnelregistry.LookupByCanonicalKey(metric)
	if !ok || entry.SemanticStatus != funnelregistry.Available || entry.OptionalProviderCandidate {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "unknown or unavailable funnel metric")
	}
	out.MetricKey, out.Unit = entry.CanonicalKey, entry.Unit
	out.TemporalSemantics, out.EntityAggregationSemantics, out.Surface, out.ProviderMetricName = string(entry.TemporalSemantics), string(entry.EntityAggregationSemantics), string(entry.Surface), entry.ProviderMetric
	var value string
	var count int
	if entry.TemporalSemantics == funnelregistry.AdditiveFlow {
		err := s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(scope_kind,COALESCE(source_sku,0),metric_key,period_start,period_end)value_numeric,period_start FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind=$2 AND($2='ACCOUNT' OR canonical_sku_id=$3)AND metric_key=$4 AND period_start BETWEEN $5 AND $6 AND period_end=period_start AND observed_at<=$7 ORDER BY scope_kind,COALESCE(source_sku,0),metric_key,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)SELECT COALESCE(sum(value_numeric),0)::text,count(DISTINCT period_start)::int FROM latest`, c.SellerAccountID(), scope, sku, metric, p.From, p.To, c.CutoffAt()).Scan(&value, &count)
		if err != nil {
			return out, dbError(err)
		}
		if count != periodDays(p) {
			out.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MISSING_DAILY_FUNNEL_FACT")
		} else {
			d, err := factualapi.ParseDecimal(value)
			if err != nil {
				return out, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid funnel decimal")
			}
			out.Value = factualapi.Known(d)
		}
	} else {
		err := s.pool.QueryRow(ctx, `SELECT value_numeric::text FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind=$2 AND($2='ACCOUNT' OR canonical_sku_id=$3)AND metric_key=$4 AND period_start=$5 AND period_end=$6 AND observed_at<=$7 ORDER BY observed_at DESC,revision_no DESC,id DESC LIMIT 1`, c.SellerAccountID(), scope, sku, metric, p.From, p.To, c.CutoffAt()).Scan(&value)
		if errors.Is(err, pgx.ErrNoRows) {
			out.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DIRECT_PERIOD_FACT_UNAVAILABLE")
		} else if err != nil {
			return out, dbError(err)
		} else {
			d, err := factualapi.ParseDecimal(value)
			if err != nil {
				return out, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid funnel decimal")
			}
			out.Value = factualapi.Known(d)
		}
	}
	entity := sku
	if scope == "ACCOUNT" {
		entity = c.SellerAccountID()
	}
	out.Meta, _ = s.meta(c, funnelCapability(entry), scope+"_FUNNEL", entity, &p, factHash(out))
	return out, nil
}

func (s *Service) GetAccountFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.AccountFunnelPeriod, error) {
	out := factualapi.AccountFunnelPeriod{SellerAccountID: c.SellerAccountID()}
	if err := s.validate(c); err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	out.Metric, err = s.funnelValue(ctx, c, "ACCOUNT", 0, metric, p)
	return out, err
}

func (s *Service) GetSKUFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, id int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.SKUFunnelPeriod, error) {
	out := factualapi.SKUFunnelPeriod{CanonicalSKUID: id}
	if err := s.validate(c); err != nil {
		return out, err
	}
	var exists int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM canonical_skus WHERE seller_account_id=$1 AND id=$2 AND first_observed_at<=$3`, c.SellerAccountID(), id, c.CutoffAt()).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("SKU")
	}
	if err != nil {
		return out, dbError(err)
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	out.Metric, err = s.funnelValue(ctx, c, "SKU", id, metric, p)
	return out, err
}

func (s *Service) BatchGetSKUFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, ids []int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUFunnelPeriodResult, error) {
	var out factualapi.BatchSKUFunnelPeriodResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	entry, ok := funnelregistry.LookupByCanonicalKey(metric)
	if !ok || entry.SemanticStatus != funnelregistry.Available || entry.OptionalProviderCandidate {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "unknown or unavailable funnel metric")
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	capability := funnelCapability(entry)
	entityKind := "SKU_FUNNEL"
	if entry.TemporalSemantics == funnelregistry.AdditiveFlow {
		rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$6),latest AS(SELECT DISTINCT ON(canonical_sku_id,period_start)canonical_sku_id,period_start,value_numeric FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND canonical_sku_id=ANY($2)AND metric_key=$3 AND period_start BETWEEN $4 AND $5 AND period_end=period_start AND observed_at<=$6 ORDER BY canonical_sku_id,period_start,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT canonical_sku_id,COALESCE(sum(value_numeric),0)::text value,count(DISTINCT period_start)::int days FROM latest GROUP BY canonical_sku_id)SELECT e.id,e.found,a.value,a.days FROM eligible e LEFT JOIN agg a ON a.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, metric, p.From, p.To, c.CutoffAt())
		if err != nil {
			return out, dbError(err)
		}
		defer rows.Close()
		wantDays := periodDays(p)
		for rows.Next() {
			var id int64
			var found pgtype.Int8
			var value pgtype.Text
			var days pgtype.Int4
			if err = rows.Scan(&id, &found, &value, &days); err != nil {
				return out, dbError(err)
			}
			if !found.Valid {
				out.Items = append(out.Items, factualapi.BatchSKUFunnelPeriodItem{ID: id, Status: factualapi.BatchNotFound})
				continue
			}
			metricVal := factualapi.FunnelMetricValue{
				MetricKey:                  entry.CanonicalKey,
				Unit:                       entry.Unit,
				TemporalSemantics:          string(entry.TemporalSemantics),
				EntityAggregationSemantics: string(entry.EntityAggregationSemantics),
				Surface:                    string(entry.Surface),
				ProviderMetricName:         entry.ProviderMetric,
			}
			status := factualapi.BatchFound
			if !days.Valid || days.Int32 != int32(wantDays) {
				metricVal.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MISSING_DAILY_FUNNEL_FACT")
				status = factualapi.BatchLimited
			} else {
				d, parseErr := factualapi.ParseDecimal(value.String)
				if parseErr != nil {
					return out, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid funnel decimal")
				}
				metricVal.Value = factualapi.Known(d)
			}
			metricVal.Meta, _ = s.meta(c, capability, entityKind, id, &p, factHash(metricVal))
			v := factualapi.SKUFunnelPeriod{CanonicalSKUID: id, Metric: metricVal}
			out.Items = append(out.Items, factualapi.BatchSKUFunnelPeriodItem{ID: id, Status: status, Funnel: &v})
		}
		return out, rows.Err()
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$6),latest AS(SELECT DISTINCT ON(canonical_sku_id)canonical_sku_id,value_numeric::text value FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND canonical_sku_id=ANY($2)AND metric_key=$3 AND period_start=$4 AND period_end=$5 AND observed_at<=$6 ORDER BY canonical_sku_id,observed_at DESC,revision_no DESC,id DESC)SELECT e.id,e.found,l.value FROM eligible e LEFT JOIN latest l ON l.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, metric, p.From, p.To, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found pgtype.Int8
		var value pgtype.Text
		if err = rows.Scan(&id, &found, &value); err != nil {
			return out, dbError(err)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUFunnelPeriodItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		metricVal := factualapi.FunnelMetricValue{
			MetricKey:                  entry.CanonicalKey,
			Unit:                       entry.Unit,
			TemporalSemantics:          string(entry.TemporalSemantics),
			EntityAggregationSemantics: string(entry.EntityAggregationSemantics),
			Surface:                    string(entry.Surface),
			ProviderMetricName:         entry.ProviderMetric,
		}
		status := factualapi.BatchFound
		if !value.Valid {
			metricVal.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DIRECT_PERIOD_FACT_UNAVAILABLE")
			status = factualapi.BatchLimited
		} else {
			d, parseErr := factualapi.ParseDecimal(value.String)
			if parseErr != nil {
				return out, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid funnel decimal")
			}
			metricVal.Value = factualapi.Known(d)
		}
		metricVal.Meta, _ = s.meta(c, capability, entityKind, id, &p, factHash(metricVal))
		v := factualapi.SKUFunnelPeriod{CanonicalSKUID: id, Metric: metricVal}
		out.Items = append(out.Items, factualapi.BatchSKUFunnelPeriodItem{ID: id, Status: status, Funnel: &v})
	}
	return out, rows.Err()
}

func (s *Service) ListSKUFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period, req factualapi.PageRequest) (factualapi.Page[factualapi.SKUFunnelPeriod], error) {
	var out factualapi.Page[factualapi.SKUFunnelPeriod]
	n, err := pageSize(req, s.maxPage)
	if err != nil {
		return out, err
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	last, h, err := s.cursor(c, "ListSKUFunnelPeriod", struct {
		Metric string
		Period factualapi.Period
	}{metric, p}, req)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND first_observed_at<=$2 AND id>$3 ORDER BY id LIMIT $4`, c.SellerAccountID(), c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return out, dbError(err)
		}
		ids = append(ids, id)
	}
	has := len(ids) > n
	if has {
		ids = ids[:n]
	}
	for _, id := range ids {
		value, e := s.funnelValue(ctx, c, "SKU", id, metric, p)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, factualapi.SKUFunnelPeriod{CanonicalSKUID: id, Metric: value})
	}
	if len(ids) > 0 {
		out.NextPageToken, err = s.next(c, "ListSKUFunnelPeriod", h, ids[len(ids)-1], has)
	}
	return out, err
}

func (s *Service) GetSKUFunnelSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.FunnelPoint, error) {
	entry, ok := funnelregistry.LookupByCanonicalKey(metric)
	if !ok {
		return nil, factualapi.NewError(factualapi.CodeInvalidArgument, "unknown funnel metric")
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return nil, err
	}
	if entry.TemporalSemantics != funnelregistry.AdditiveFlow {
		v, e := s.funnelValue(ctx, c, "SKU", id, metric, p)
		return []factualapi.FunnelPoint{{PeriodStart: p.From, PeriodEnd: p.To, Metric: v}}, e
	}
	var out []factualapi.FunnelPoint
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		one := factualapi.Period{From: d, To: d}
		v, e := s.funnelValue(ctx, c, "SKU", id, metric, one)
		if e != nil {
			return nil, e
		}
		out = append(out, factualapi.FunnelPoint{PeriodStart: d, PeriodEnd: d, Metric: v})
	}
	return out, nil
}

func (s *Service) GetCategoryFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, categoryID int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.CategoryFunnelPeriod, error) {
	out := factualapi.CategoryFunnelPeriod{CategoryID: categoryID}
	entry, ok := funnelregistry.LookupByCanonicalKey(metric)
	if !ok {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "unknown funnel metric")
	}
	p, err := factualapi.ResolvePeriod(c, sp, ep)
	if err != nil {
		return out, err
	}
	out.Metric = factualapi.FunnelMetricValue{MetricKey: metric, Unit: entry.Unit, TemporalSemantics: string(entry.TemporalSemantics), EntityAggregationSemantics: string(entry.EntityAggregationSemantics), Surface: string(entry.Surface), ProviderMetricName: entry.ProviderMetric}
	if entry.TemporalSemantics != funnelregistry.AdditiveFlow {
		out.Metric.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueNotApplicable, "NON_ADDITIVE_CATEGORY_ROLLUP_FORBIDDEN")
		out.Metric.Meta, _ = s.meta(c, "FUNNEL.CATEGORY_ADDITIVE", "CATEGORY_FUNNEL", categoryID, &p, factHash(out))
		return out, nil
	}
	var value string
	var days, ambiguous int
	err = s.pool.QueryRow(ctx, `WITH facts AS(SELECT DISTINCT ON(canonical_sku_id,period_start)canonical_sku_id,period_start,value_numeric FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND metric_key=$2 AND period_start BETWEEN $3 AND $4 AND period_end=period_start AND observed_at<=$5 ORDER BY canonical_sku_id,period_start,observed_at DESC,revision_no DESC,id DESC),classified AS(SELECT f.*,a.category_id,a.stable FROM facts f LEFT JOIN LATERAL(SELECT max(seller_category_id)category_id,(count(*)=1 AND bool_and(mapping_status='MAPPED')AND min(observed_from)<=($3::date::timestamp AT TIME ZONE 'Europe/Moscow')AND bool_and(observed_to IS NULL OR observed_to>(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')))stable FROM sku_category_assignments x WHERE x.canonical_sku_id=f.canonical_sku_id AND x.observed_from<=(($4::date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(x.observed_to IS NULL OR x.observed_to>($3::date::timestamp AT TIME ZONE 'Europe/Moscow'))AND x.observed_from<=$5)a ON true)SELECT COALESCE(sum(value_numeric)FILTER(WHERE stable AND category_id=$6),0)::text,count(DISTINCT period_start)FILTER(WHERE stable AND category_id=$6)::int,count(*)FILTER(WHERE NOT COALESCE(stable,false))::int FROM classified`, c.SellerAccountID(), metric, p.From, p.To, c.CutoffAt(), categoryID).Scan(&value, &days, &ambiguous)
	if err != nil {
		return out, dbError(err)
	}
	if ambiguous > 0 {
		out.Metric.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "CATEGORY_PERIOD_MAPPING_AMBIGUOUS")
	} else if days != periodDays(p) {
		out.Metric.Value = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MISSING_DAILY_FUNNEL_FACT")
	} else {
		d, e := factualapi.ParseDecimal(value)
		if e != nil {
			return out, e
		}
		out.Metric.Value = factualapi.Known(d)
	}
	out.Metric.Meta, _ = s.meta(c, "FUNNEL.CATEGORY_ADDITIVE", "CATEGORY_FUNNEL", categoryID, &p, factHash(out))
	return out, nil
}

func (s *Service) GetFunnelMetric(ctx context.Context, c factualapi.FactualReadContext, q factualapi.FunnelMetricQuery) (factualapi.FunnelMetricValue, error) {
	if q.ScopeKind == "ACCOUNT" {
		x, e := s.GetAccountFunnelPeriod(ctx, c, q.MetricKey, q.StandardPeriod, q.Period)
		return x.Metric, e
	}
	if q.ScopeKind == "SKU" {
		x, e := s.GetSKUFunnelPeriod(ctx, c, q.CanonicalSKUID, q.MetricKey, q.StandardPeriod, q.Period)
		return x.Metric, e
	}
	return factualapi.FunnelMetricValue{}, factualapi.NewError(factualapi.CodeInvalidArgument, "scope must be ACCOUNT or SKU")
}

func (s *Service) GetFunnelCapabilities(_ context.Context, c factualapi.FactualReadContext) (factualapi.FunnelCapabilities, error) {
	if err := s.validate(c); err != nil {
		return factualapi.FunnelCapabilities{}, err
	}
	out := factualapi.FunnelCapabilities{RegistryVersion: funnelregistry.Version()}
	for _, e := range funnelregistry.Entries() {
		out.Metrics = append(out.Metrics, factualapi.FunnelMetricCapability{MetricKey: e.CanonicalKey, ProviderMetricName: e.ProviderMetric, TemporalSemantics: string(e.TemporalSemantics), EntityAggregationSemantics: string(e.EntityAggregationSemantics), Surface: string(e.Surface), SemanticStatus: string(e.SemanticStatus)})
	}
	return out, nil
}

var _ = fmt.Sprint
var _ = time.Time{}
