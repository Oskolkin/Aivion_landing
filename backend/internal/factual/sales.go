package factual

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func periodDays(p factualapi.Period) int { return int(p.To.Sub(p.From).Hours()/24) + 1 }
func unavailableDemand(readiness string, p factualapi.Period) factualapi.Demand {
	state := factualapi.ValueUnavailable
	reason := "CAPABILITY_UNAVAILABLE"
	if readiness == "FAILED" {
		state = factualapi.ValueUnknown
		reason = "SOURCE_FAILED"
	} else if readiness == "NOT_APPLICABLE" {
		state = factualapi.ValueNotApplicable
		reason = "CAPABILITY_NOT_APPLICABLE"
	}
	return factualapi.Demand{OrderedUnits: factualapi.Missing[int64](state, reason), OrderedAmount: factualapi.Missing[factualapi.Money](state, reason)}
}
func (s *Service) finishDemand(ctx context.Context, c factualapi.FactualReadContext, key, kind string, entity int64, p factualapi.Period, amount string, units int64, days int, currency string, dq *periodDQAdjust) (factualapi.Demand, error) {
	r, _ := readiness(c, key)
	if r == "UNAVAILABLE" || r == "FAILED" || r == "NOT_APPLICABLE" {
		d := unavailableDemand(r, p)
		m, e := s.meta(c, key, kind, entity, &p, factualapi.StableHash(r, "unavailable"))
		d.Meta = m
		return d, e
	}
	d := factualapi.Demand{}
	if currency == "MIXED" {
		d.OrderedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "MIXED_CURRENCY")
		d.OrderedAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MIXED_CURRENCY")
	} else if days != periodDays(p) {
		d.OrderedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
		d.OrderedAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
	} else {
		a, e := factualapi.ParseDecimal(amount)
		if e != nil {
			return d, factualapi.NewError(factualapi.CodeInternalInvariantViolation, "invalid canonical decimal")
		}
		d.OrderedUnits = factualapi.Known(units)
		d.OrderedAmount = factualapi.Known(factualapi.Money{Amount: a, CurrencyCode: currency})
	}
	m, e := s.meta(c, key, kind, entity, &p, factualapi.StableHash(amount, fmt.Sprint(units), fmt.Sprint(days), currency))
	if e == nil {
		if dq != nil {
			dq.apply(&m)
		} else {
			e = s.applyPeriodDQ(ctx, c, p, &m)
		}
	}
	if currency == "MIXED" {
		m.Readiness = "FAILED"
		m.Limitations = append(m.Limitations, factualapi.Limitation{Code: "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE", Message: "requested facts contain more than one currency and no FX policy exists"})
	}
	d.Meta = m
	return d, e
}

func (s *Service) applyPeriodDQ(ctx context.Context, c factualapi.FactualReadContext, p factualapi.Period, m *factualapi.FactMeta) error {
	rows, e := s.pool.Query(ctx, `WITH days AS(SELECT generate_series($2::date,$3::date,'1 day')::date d),latest AS(SELECT DISTINCT ON(r.business_date)r.business_date,r.overall_status FROM sales_reconciliations r JOIN collection_domain_runs dr ON dr.id=r.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.observed_at<=$4 AND dr.status IN('COMPLETED','PARTIAL') ORDER BY r.business_date,r.observed_at DESC,r.id DESC)SELECT days.d,COALESCE(latest.overall_status,'MISSING')FROM days LEFT JOIN latest ON latest.business_date=days.d ORDER BY days.d`, c.SellerAccountID(), p.From, p.To, c.CutoffAt())
	if e != nil {
		return dbError(e)
	}
	defer rows.Close()
	partial, blocked := false, false
	for rows.Next() {
		var d time.Time
		var status string
		if e = rows.Scan(&d, &status); e != nil {
			return dbError(e)
		}
		switch status {
		case "PASS":
		case "EXPLAINED_GAP":
			partial = true
		case "UNEXPLAINED_GAP", "NOT_COMPARABLE", "MISSING":
			blocked = true
		}
	}
	if e = rows.Err(); e != nil {
		return dbError(e)
	}
	if blocked {
		m.Readiness = "FAILED"
		m.Limitations = append(m.Limitations, factualapi.Limitation{Code: "PERIOD_RECONCILIATION_BLOCKED", Message: "requested period contains missing, unexplained, or non-comparable day"})
	} else if partial && m.Readiness == "READY" {
		m.Readiness = "PARTIAL"
		m.Limitations = append(m.Limitations, factualapi.Limitation{Code: "PERIOD_EXPLAINED_GAP", Message: "requested period contains a policy-allowed explained gap"})
	}
	return nil
}

// periodDQAdjust captures seller-period reconciliation adjustments shared by all facts in one period.
type periodDQAdjust struct {
	readiness   string
	limitations []factualapi.Limitation
}

func (a *periodDQAdjust) apply(m *factualapi.FactMeta) {
	if a == nil || m == nil {
		return
	}
	if a.readiness != "" && a.readiness != "READY" {
		m.Readiness = a.readiness
	}
	if len(a.limitations) > 0 {
		m.Limitations = append(m.Limitations, a.limitations...)
	}
}

func (s *Service) loadPeriodDQAdjust(ctx context.Context, c factualapi.FactualReadContext, p factualapi.Period) (periodDQAdjust, error) {
	m := factualapi.FactMeta{Readiness: "READY"}
	if err := s.applyPeriodDQ(ctx, c, p, &m); err != nil {
		return periodDQAdjust{}, err
	}
	return periodDQAdjust{readiness: m.Readiness, limitations: append([]factualapi.Limitation(nil), m.Limitations...)}, nil
}

func (s *Service) GetAccountDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.Demand, error) {
	if e := s.validate(c); e != nil {
		return factualapi.Demand{}, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return factualapi.Demand{}, e
	}
	var amount, currency string
	var units int64
	var days int
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)business_date,ordered_amount,ordered_units,currency_code FROM sales_account_daily WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT CASE WHEN count(DISTINCT currency_code)>1 THEN 0 ELSE COALESCE(sum(ordered_amount),0)END::text,COALESCE(sum(ordered_units),0)::bigint,CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE COALESCE(min(currency_code),'RUB') END,count(DISTINCT business_date)::int FROM latest`, c.SellerAccountID(), p.From, p.To, c.CutoffAt()).Scan(&amount, &units, &currency, &days)
	if e != nil {
		return factualapi.Demand{}, dbError(e)
	}
	return s.finishDemand(ctx, c, "SALES.ACCOUNT_DEMAND", "ACCOUNT_DEMAND", c.SellerAccountID(), p, amount, units, days, currency, nil)
}

// sourceSalesPeriodDays counts distinct canonical Account Sales business days
// in the requested period. Category (and other rollups) must use this source
// completeness, not the count of category/SKU rows.
func (s *Service) sourceSalesPeriodDays(ctx context.Context, c factualapi.FactualReadContext, p factualapi.Period) (int, error) {
	var days int
	e := s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)business_date FROM sales_account_daily WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT count(*)::int FROM latest`, c.SellerAccountID(), p.From, p.To, c.CutoffAt()).Scan(&days)
	if e != nil {
		return 0, dbError(e)
	}
	return days, nil
}
func (s *Service) GetSKUDemand(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.SKUDemand, error) {
	var out factualapi.SKUDemand
	if e := s.validate(c); e != nil {
		return out, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return out, e
	}
	var amount, currency string
	var units int64
	var days int
	e = s.pool.QueryRow(ctx, `SELECT cs.id,COALESCE(x.amount,0)::text,COALESCE(x.units,0)::bigint,COALESCE(x.currency,'RUB'),COALESCE(x.days,0)::int FROM canonical_skus cs LEFT JOIN LATERAL(SELECT CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)END amount,sum(ordered_units) units,CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE min(currency_code)END currency,count(DISTINCT business_date)days FROM(SELECT DISTINCT ON(business_date)business_date,ordered_amount,ordered_units,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=cs.id AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)f)x ON true WHERE cs.seller_account_id=$1 AND cs.id=$2 AND cs.first_observed_at<=$5`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt()).Scan(&out.CanonicalSKUID, &amount, &units, &currency, &days)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, notFound("SKU")
	}
	if e != nil {
		return out, dbError(e)
	}
	out.Demand, e = s.finishDemand(ctx, c, "SALES.SKU_DEMAND", "SKU_DEMAND", id, p, amount, units, days, currency, nil)
	return out, e
}
func (s *Service) BatchGetSKUDemand(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUDemandResult, error) {
	var out factualapi.BatchSKUDemandResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return out, e
	}
	dqAdj, e := s.loadPeriodDQAdjust(ctx, c, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,business_date,ordered_amount,ordered_units,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT canonical_sku_id,CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)END::text amount,sum(ordered_units)::bigint units,CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE min(currency_code)END currency,count(DISTINCT business_date)::int days FROM latest GROUP BY canonical_sku_id)SELECT e.id,e.found,COALESCE(a.amount,'0'),COALESCE(a.units,0),COALESCE(a.currency,'RUB'),COALESCE(a.days,0)FROM eligible e LEFT JOIN agg a ON a.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found pgtype.Int8
		var amount, currency string
		var units int64
		var days int
		if e = rows.Scan(&id, &found, &amount, &units, &currency, &days); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUDemandItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		d, de := s.finishDemand(ctx, c, "SALES.SKU_DEMAND", "SKU_DEMAND", id, p, amount, units, days, currency, &dqAdj)
		if de != nil {
			return out, de
		}
		v := factualapi.SKUDemand{CanonicalSKUID: id, Demand: d}
		status := factualapi.BatchFound
		if d.OrderedAmount.State != factualapi.ValueKnown {
			status = factualapi.BatchLimited
		}
		out.Items = append(out.Items, factualapi.BatchSKUDemandItem{ID: id, Status: status, Demand: &v})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetSKUTrendContext(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUTrendContextResult, error) {
	var out factualapi.BatchSKUTrendContextResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	p, e := factualapi.ResolvePeriod(c, factualapi.PeriodTrend30D, nil)
	if e != nil {
		return out, e
	}
	dqAdj, e := s.loadPeriodDQAdjust(ctx, c, p)
	if e != nil {
		return out, e
	}
	wantDays := periodDays(p)
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),days AS(SELECT generate_series($3::date,$4::date,'1 day')::date d),latest AS(SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,business_date,ordered_amount,ordered_units,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC),grid AS(SELECT e.id req_id,e.found sku_id,e.ord,d.d FROM eligible e CROSS JOIN days d WHERE e.found IS NOT NULL),filled AS(SELECT g.req_id,g.sku_id,g.ord,g.d,l.ordered_amount,l.ordered_units,l.currency_code,l.business_date IS NOT NULL has_fact FROM grid g LEFT JOIN latest l ON l.canonical_sku_id=g.sku_id AND l.business_date=g.d),day_kind AS(SELECT req_id,sku_id,ord,d,ordered_amount,ordered_units,currency_code,has_fact,CASE WHEN NOT has_fact THEN 1 WHEN ordered_units>0 THEN 2 ELSE 0 END kind,ROW_NUMBER()OVER(PARTITION BY req_id ORDER BY d DESC)-1 dist FROM filled),breaks AS(SELECT req_id,MIN(dist)FILTER(WHERE kind<>0)break_dist,count(*)FILTER(WHERE has_fact AND ordered_units>0)::bigint days_with_orders,count(*)FILTER(WHERE has_fact)::int fact_days,bool_or(NOT has_fact)any_missing FROM day_kind GROUP BY req_id),amounts AS(SELECT req_id,CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)END::text amount,sum(ordered_units)::bigint units,CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE min(currency_code)END currency FROM filled WHERE has_fact GROUP BY req_id)SELECT e.id,e.found,a.amount,a.units,a.currency,COALESCE(b.days_with_orders,0),COALESCE(b.fact_days,0),LEAST(COALESCE(b.break_dist,30),30)::bigint,COALESCE(b.any_missing,false)FROM eligible e LEFT JOIN amounts a ON a.req_id=e.id LEFT JOIN breaks b ON b.req_id=e.id ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found pgtype.Int8
		var amount, currency pgtype.Text
		var units, daysWithOrders, trailing, factDays int64
		var days int
		var anyMissing bool
		if e = rows.Scan(&id, &found, &amount, &units, &currency, &daysWithOrders, &factDays, &trailing, &anyMissing); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUTrendContextItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		days = int(factDays)
		cur := currency.String
		if !currency.Valid {
			cur = "RUB"
		}
		trend := factualapi.SKUTrendContext{
			CanonicalSKUID: id,
			Period:         p,
			FormulaPolicy:  "sku_sales_trend_context_v1",
		}
		d, de := s.finishDemand(ctx, c, "SALES.SKU_DEMAND", "SKU_TREND_CONTEXT", id, p, textValue(amount), units, days, cur, &dqAdj)
		if de != nil {
			return out, de
		}
		trend.OrderedAmount30D = d.OrderedAmount
		trend.OrderedUnits30D = d.OrderedUnits
		if days == wantDays {
			trend.DaysWithOrders30D = factualapi.Known(daysWithOrders)
			trend.TrailingDaysWithoutOrders = factualapi.Known(trailing)
		} else {
			trend.DaysWithOrders30D = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
			trend.TrailingDaysWithoutOrders = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
		}
		trend.Meta = d.Meta
		if anyMissing {
			if trend.Meta.Readiness == "READY" {
				trend.Meta.Readiness = "PARTIAL"
			}
			trend.Meta.Limitations = append(trend.Meta.Limitations, factualapi.Limitation{Code: "MISSING_DAILY_FACT", Message: "missing day in 30d window stops trailing-days-without-orders policy"})
		}
		status := factualapi.BatchFound
		if trend.OrderedAmount30D.State != factualapi.ValueKnown {
			status = factualapi.BatchLimited
		}
		fact := trend
		out.Items = append(out.Items, factualapi.BatchSKUTrendContextItem{ID: id, Status: status, Trend: &fact})
	}
	return out, rows.Err()
}
func (s *Service) ListSKUDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period, req factualapi.PageRequest) (factualapi.Page[factualapi.SKUDemand], error) {
	var out factualapi.Page[factualapi.SKUDemand]
	if e := s.validate(c); e != nil {
		return out, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return out, e
	}
	n, e := pageSize(req, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListSKUDemand", p, req)
	if e != nil {
		return out, e
	}
	dqAdj, e := s.loadPeriodDQAdjust(ctx, c, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT cs.id,COALESCE(x.amount,0)::text,COALESCE(x.units,0)::bigint,COALESCE(x.currency,'RUB'),COALESCE(x.days,0)::int FROM canonical_skus cs LEFT JOIN LATERAL(SELECT CASE WHEN count(DISTINCT currency_code)>1 THEN NULL ELSE sum(ordered_amount)END amount,sum(ordered_units)units,CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE min(currency_code)END currency,count(DISTINCT business_date)days FROM(SELECT DISTINCT ON(business_date)business_date,ordered_amount,ordered_units,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=cs.id AND business_date BETWEEN $4 AND $5 AND observed_at<=$2 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)z)x ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND cs.id>$3 ORDER BY cs.id LIMIT $6`, c.SellerAccountID(), c.CutoffAt(), last, p.From, p.To, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id, units int64
		var days int
		var amount, currency string
		if e = rows.Scan(&id, &amount, &units, &currency, &days); e != nil {
			return out, dbError(e)
		}
		d, de := s.finishDemand(ctx, c, "SALES.SKU_DEMAND", "SKU_DEMAND", id, p, amount, units, days, currency, &dqAdj)
		if de != nil {
			return out, de
		}
		out.Items = append(out.Items, factualapi.SKUDemand{CanonicalSKUID: id, Demand: d})
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListSKUDemand", h, out.Items[len(out.Items)-1].CanonicalSKUID, has)
	}
	return out, e
}
func (s *Service) GetDemandSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.DemandPoint, error) {
	if e := s.validate(c); e != nil {
		return nil, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return nil, e
	}
	if _, e = s.GetSKU(ctx, c, id); e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT ON(business_date)business_date,ordered_amount::text,ordered_units,currency_code,id FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	type raw struct {
		amount, currency string
		units, id        int64
	}
	m := map[string]raw{}
	for rows.Next() {
		var d time.Time
		var x raw
		if e = rows.Scan(&d, &x.amount, &x.units, &x.currency, &x.id); e != nil {
			return nil, dbError(e)
		}
		m[d.Format("2006-01-02")] = x
	}
	var out []factualapi.DemandPoint
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		x, ok := m[d.Format("2006-01-02")]
		point := factualapi.DemandPoint{BusinessDate: d}
		one := factualapi.Period{From: d, To: d}
		if !ok {
			point.OrderedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
			point.OrderedAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
		} else {
			a, _ := factualapi.ParseDecimal(x.amount)
			point.OrderedUnits = factualapi.Known(x.units)
			point.OrderedAmount = factualapi.Known(factualapi.Money{Amount: a, CurrencyCode: x.currency})
		}
		point.Meta, _ = s.meta(c, "SALES.SKU_DEMAND", "SKU_DEMAND_DAY", id, &one, factualapi.StableHash(x.amount, fmt.Sprint(x.units)))
		if e = s.applyPeriodDQ(ctx, c, one, &point.Meta); e != nil {
			return nil, e
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (s *Service) GetAccountDemandSeries(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.DemandPoint, error) {
	if e := s.validate(c); e != nil {
		return nil, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT ON(business_date)business_date,ordered_amount::text,ordered_units,currency_code,id FROM sales_account_daily WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC`,
		c.SellerAccountID(), p.From, p.To, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	return s.scanDemandSeries(ctx, c, "SALES.ACCOUNT_DEMAND", "ACCOUNT_DEMAND_DAY", c.SellerAccountID(), p, rows)
}

func (s *Service) GetCategoryDemandSeries(ctx context.Context, c factualapi.FactualReadContext, categoryID int64, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.DemandPoint, error) {
	if e := s.validate(c); e != nil {
		return nil, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `WITH facts AS(
		SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,business_date,ordered_amount,ordered_units,currency_code,id
		FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id IS NOT NULL AND business_date BETWEEN $3 AND $4 AND observed_at<=$5
		ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC
	), mapped AS(
		SELECT f.business_date,f.ordered_amount,f.ordered_units,f.currency_code,f.id,
			CASE WHEN a.cnt=1 AND a.mapped=1 THEN a.category_id ELSE NULL END category_id
		FROM facts f LEFT JOIN LATERAL(
			SELECT count(*)cnt,count(*)FILTER(WHERE x.mapping_status='MAPPED' AND x.seller_category_id IS NOT NULL)mapped,max(x.seller_category_id)category_id
			FROM sku_category_assignments x
			WHERE x.canonical_sku_id=f.canonical_sku_id
			  AND x.observed_from < ((f.business_date+1)::timestamp AT TIME ZONE 'Europe/Moscow')
			  AND (x.observed_to IS NULL OR x.observed_to>(f.business_date::timestamp AT TIME ZONE 'Europe/Moscow'))
			  AND x.observed_from<=$5
		)a ON true
	)
	SELECT business_date,COALESCE(sum(ordered_amount),0)::text,COALESCE(sum(ordered_units),0)::bigint,
		CASE WHEN count(DISTINCT currency_code)>1 THEN 'MIXED' ELSE COALESCE(min(currency_code),'RUB') END,min(id)
	FROM mapped WHERE category_id=$2
	GROUP BY business_date ORDER BY business_date`,
		c.SellerAccountID(), categoryID, p.From, p.To, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	return s.scanDemandSeries(ctx, c, "SALES.CATEGORY_DEMAND", "CATEGORY_DEMAND_DAY", categoryID, p, rows)
}

func (s *Service) BatchGetSKUDemandSeries(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUDemandSeriesResult, error) {
	var out factualapi.BatchSKUDemandSeriesResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return out, e
	}
	uniq := uniquePositiveInt64(ids)
	out.Items = make([]factualapi.BatchSKUDemandSeriesItem, 0, len(uniq))
	if len(uniq) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,business_date,ordered_amount::text,ordered_units,currency_code,id
		FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2) AND business_date BETWEEN $3 AND $4 AND observed_at<=$5
		ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC`,
		c.SellerAccountID(), uniq, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	type raw struct {
		amount, currency string
		units, id        int64
	}
	bySKU := map[int64]map[string]raw{}
	for rows.Next() {
		var sku int64
		var d time.Time
		var x raw
		if e = rows.Scan(&sku, &d, &x.amount, &x.units, &x.currency, &x.id); e != nil {
			return out, dbError(e)
		}
		if bySKU[sku] == nil {
			bySKU[sku] = map[string]raw{}
		}
		bySKU[sku][d.Format("2006-01-02")] = x
	}
	if e = rows.Err(); e != nil {
		return out, dbError(e)
	}
	for _, id := range uniq {
		series := factualapi.SKUDemandSeries{CanonicalSKUID: id, Points: []factualapi.DemandPoint{}}
		m := bySKU[id]
		for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
			x, ok := m[d.Format("2006-01-02")]
			point := factualapi.DemandPoint{BusinessDate: d}
			one := factualapi.Period{From: d, To: d}
			if !ok {
				point.OrderedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
				point.OrderedAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
			} else {
				a, _ := factualapi.ParseDecimal(x.amount)
				point.OrderedUnits = factualapi.Known(x.units)
				point.OrderedAmount = factualapi.Known(factualapi.Money{Amount: a, CurrencyCode: x.currency})
			}
			point.Meta, _ = s.meta(c, "SALES.SKU_DEMAND", "SKU_DEMAND_DAY", id, &one, factualapi.StableHash(x.amount, fmt.Sprint(x.units)))
			if e = s.applyPeriodDQ(ctx, c, one, &point.Meta); e != nil {
				return out, e
			}
			series.Points = append(series.Points, point)
		}
		cp := series
		status := factualapi.BatchFound
		if len(m) == 0 {
			status = factualapi.BatchFoundEmpty
		}
		out.Items = append(out.Items, factualapi.BatchSKUDemandSeriesItem{ID: id, Status: status, Series: &cp})
	}
	return out, nil
}

func uniquePositiveInt64(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (s *Service) scanDemandSeries(ctx context.Context, c factualapi.FactualReadContext, capKey, kind string, entity int64, p factualapi.Period, rows pgx.Rows) ([]factualapi.DemandPoint, error) {
	type raw struct {
		amount, currency string
		units, id        int64
	}
	m := map[string]raw{}
	for rows.Next() {
		var d time.Time
		var x raw
		if e := rows.Scan(&d, &x.amount, &x.units, &x.currency, &x.id); e != nil {
			return nil, dbError(e)
		}
		m[d.Format("2006-01-02")] = x
	}
	var out []factualapi.DemandPoint
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		x, ok := m[d.Format("2006-01-02")]
		point := factualapi.DemandPoint{BusinessDate: d}
		one := factualapi.Period{From: d, To: d}
		if !ok {
			point.OrderedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
			point.OrderedAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MISSING_DAILY_FACT")
		} else {
			a, _ := factualapi.ParseDecimal(x.amount)
			point.OrderedUnits = factualapi.Known(x.units)
			point.OrderedAmount = factualapi.Known(factualapi.Money{Amount: a, CurrencyCode: x.currency})
		}
		point.Meta, _ = s.meta(c, capKey, kind, entity, &one, factualapi.StableHash(x.amount, fmt.Sprint(x.units)))
		if e := s.applyPeriodDQ(ctx, c, one, &point.Meta); e != nil {
			return nil, e
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

type categoryAggregate struct {
	amount   factualapi.Decimal
	units    int64
	currency string
	count    int
}

func (s *Service) categoryDemand(ctx context.Context, c factualapi.FactualReadContext, p factualapi.Period) (map[string]categoryAggregate, error) {
	rows, e := s.pool.Query(ctx, `WITH facts AS(SELECT DISTINCT ON(canonical_sku_id,business_date)id,canonical_sku_id,business_date,ordered_amount,ordered_units,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id IS NOT NULL AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC)SELECT f.ordered_amount::text,f.ordered_units,f.currency_code,CASE WHEN a.cnt=1 AND a.mapped=1 THEN a.category_id ELSE NULL END FROM facts f LEFT JOIN LATERAL(SELECT count(*)cnt,count(*)FILTER(WHERE x.mapping_status='MAPPED' AND x.seller_category_id IS NOT NULL)mapped,max(x.seller_category_id)category_id FROM sku_category_assignments x WHERE x.canonical_sku_id=f.canonical_sku_id AND x.observed_from < ((f.business_date+1)::timestamp AT TIME ZONE 'Europe/Moscow') AND(x.observed_to IS NULL OR x.observed_to>(f.business_date::timestamp AT TIME ZONE 'Europe/Moscow'))AND x.observed_from<=$4)a ON true`, c.SellerAccountID(), p.From, p.To, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	out := map[string]categoryAggregate{}
	for rows.Next() {
		var amount, currency string
		var units int64
		var cat pgtype.Int8
		if e = rows.Scan(&amount, &units, &currency, &cat); e != nil {
			return nil, dbError(e)
		}
		d, de := factualapi.ParseDecimal(amount)
		if de != nil {
			return nil, de
		}
		key := "UNMAPPED"
		if cat.Valid {
			key = fmt.Sprint(cat.Int64)
		}
		a := out[key]
		if a.count == 0 || a.currency == currency {
			a.amount = a.amount.Add(d)
		}
		a.units += units
		if a.count > 0 && a.currency != currency {
			a.currency = "MIXED"
			a.amount, _ = factualapi.ParseDecimal("0")
		} else if a.count == 0 {
			a.currency = currency
		}
		a.count++
		out[key] = a
	}
	return out, rows.Err()
}
func (s *Service) GetCategoryDemand(ctx context.Context, c factualapi.FactualReadContext, id *int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.CategoryDemand, error) {
	if e := s.validate(c); e != nil {
		return factualapi.CategoryDemand{}, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return factualapi.CategoryDemand{}, e
	}
	m, e := s.categoryDemand(ctx, c, p)
	if e != nil {
		return factualapi.CategoryDemand{}, e
	}
	key := "UNMAPPED"
	bucket := "UNMAPPED"
	if id != nil {
		key = fmt.Sprint(*id)
		bucket = "CATEGORY"
	}
	a, ok := m[key]
	if !ok {
		a.amount, _ = factualapi.ParseDecimal("0")
		a.currency = "RUB"
	}
	sourceDays, e := s.sourceSalesPeriodDays(ctx, c, p)
	if e != nil {
		return factualapi.CategoryDemand{}, e
	}
	d, e := s.finishDemand(ctx, c, "SALES.CATEGORY_DEMAND", "CATEGORY_DEMAND", func() int64 {
		if id == nil {
			return 0
		}
		return *id
	}(), p, a.amount.String(), a.units, sourceDays, a.currency, nil)
	return factualapi.CategoryDemand{CategoryID: id, Bucket: bucket, Demand: d}, e
}
func (s *Service) ListCategoryDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period, req factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error) {
	var out factualapi.Page[factualapi.CategoryDemand]
	if e := s.validate(c); e != nil {
		return out, e
	}
	p, e := factualapi.ResolvePeriod(c, sp, ep)
	if e != nil {
		return out, e
	}
	n, e := pageSize(req, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListCategoryDemand", p, req)
	if e != nil {
		return out, e
	}
	m, e := s.categoryDemand(ctx, c, p)
	if e != nil {
		return out, e
	}
	dqAdj, e := s.loadPeriodDQAdjust(ctx, c, p)
	if e != nil {
		return out, e
	}
	sourceDays, e := s.sourceSalesPeriodDays(ctx, c, p)
	if e != nil {
		return out, e
	}
	var ids []int64
	for k := range m {
		if k == "UNMAPPED" {
			continue
		}
		var id int64
		_, _ = fmt.Sscan(k, &id)
		if id > last {
			ids = append(ids, id)
		}
	}
	sortInt64(ids)
	_, unmappedPending := m["UNMAPPED"]
	has := len(ids) > n || (len(ids) >= n && unmappedPending)
	if len(ids) > n {
		ids = ids[:n]
	}
	for _, id := range ids {
		a := m[fmt.Sprint(id)]
		d, _ := s.finishDemand(ctx, c, "SALES.CATEGORY_DEMAND", "CATEGORY_DEMAND", id, p, a.amount.String(), a.units, sourceDays, a.currency, &dqAdj)
		v := id
		out.Items = append(out.Items, factualapi.CategoryDemand{CategoryID: &v, Bucket: "CATEGORY", Demand: d})
	}
	if !has && len(out.Items) < n {
		if a, ok := m["UNMAPPED"]; ok {
			d, _ := s.finishDemand(ctx, c, "SALES.CATEGORY_DEMAND", "CATEGORY_DEMAND", 0, p, a.amount.String(), a.units, sourceDays, a.currency, &dqAdj)
			out.Items = append(out.Items, factualapi.CategoryDemand{Bucket: "UNMAPPED", Demand: d})
		}
	}
	if len(ids) > 0 {
		out.NextPageToken, e = s.next(c, "ListCategoryDemand", h, ids[len(ids)-1], has)
	}
	return out, e
}
func sortInt64(v []int64) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
