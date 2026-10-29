package factual

import (
	"context"
	"errors"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type campaignScanner interface{ Scan(...any) error }

func scanCampaign(row campaignScanner) (factualapi.Campaign, error) {
	var x factualapi.Campaign
	var budget, weekly pgtype.Text
	var currency pgtype.Text
	e := row.Scan(&x.ID, &x.ProviderCampaignID, &x.VersionNo, &x.Title, &x.ProviderState, &x.PaymentType, &x.AdvObjectType, &x.CampaignFamily, &x.TargetScopeType, &budget, &weekly, &currency, &x.ObservedAt)
	if e != nil {
		return x, e
	}
	code := ""
	if currency.Valid {
		code = currency.String
	}
	x.Budget = moneyValue(budget, code, "BUDGET_UNAVAILABLE")
	x.WeeklyBudget = moneyValue(weekly, code, "WEEKLY_BUDGET_UNAVAILABLE")
	return x, nil
}

const campaignSelect = `SELECT c.id,c.provider_campaign_id,v.version_no,COALESCE(v.title,''),v.provider_state,COALESCE(v.payment_type,''),COALESCE(v.adv_object_type,''),v.campaign_family,v.target_scope_type,v.budget::text,v.weekly_budget::text,v.currency_code,v.observed_at FROM advertising_campaigns c JOIN LATERAL(SELECT * FROM advertising_campaign_versions x WHERE x.advertising_campaign_id=c.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true`

func (s *Service) GetCampaign(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Campaign, error) {
	var x factualapi.Campaign
	if e := s.validate(c); e != nil {
		return x, e
	}
	x, e := scanCampaign(s.pool.QueryRow(ctx, campaignSelect+` WHERE c.seller_account_id=$1 AND c.id=$3 AND c.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("campaign")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_CONFIG", "CAMPAIGN", id, nil, factHash(x))
	return x, e
}
func (s *Service) BatchGetCampaigns(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchCampaignResult, error) {
	var out factualapi.BatchCampaignResult
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	rows, e := s.pool.Query(ctx, campaignSelect+` WHERE c.seller_account_id=$1 AND c.id=ANY($3)AND c.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), ids)
	if e != nil {
		return out, dbError(e)
	}
	found := map[int64]factualapi.Campaign{}
	for rows.Next() {
		x, se := scanCampaign(rows)
		if se != nil {
			rows.Close()
			return out, se
		}
		x.Meta, _ = s.meta(c, "ADVERTISING.CAMPAIGN_CONFIG", "CAMPAIGN", x.ID, nil, factHash(x))
		found[x.ID] = x
	}
	rows.Close()
	for _, id := range ids {
		if x, ok := found[id]; ok {
			v := x
			out.Items = append(out.Items, factualapi.BatchCampaignItem{ID: id, Status: factualapi.BatchFound, Campaign: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchCampaignItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, nil
}
func (s *Service) ListCampaigns(ctx context.Context, c factualapi.FactualReadContext, f factualapi.CampaignFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.Campaign], error) {
	var out factualapi.Page[factualapi.Campaign]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListCampaigns", f, p)
	if e != nil {
		return out, e
	}
	var families, states any = f.Families, f.ProviderStates
	if len(f.Families) == 0 {
		families = nil
	}
	if len(f.ProviderStates) == 0 {
		states = nil
	}
	rows, e := s.pool.Query(ctx, campaignSelect+` WHERE c.seller_account_id=$1 AND c.first_observed_at<=$2 AND c.id>$3 AND($4::text[] IS NULL OR v.campaign_family=ANY($4))AND($5::text[] IS NULL OR v.provider_state=ANY($5))ORDER BY c.id LIMIT $6`, c.SellerAccountID(), c.CutoffAt(), last, families, states, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		x, se := scanCampaign(rows)
		if se != nil {
			return out, se
		}
		x.Meta, _ = s.meta(c, "ADVERTISING.CAMPAIGN_CONFIG", "CAMPAIGN", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListCampaigns", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, e
}

func (s *Service) GetCampaignTargetScope(ctx context.Context, c factualapi.FactualReadContext, id int64) (string, error) {
	x, e := s.GetCampaign(ctx, c, id)
	return x.TargetScopeType, e
}
func (s *Service) ListCampaignTargetSKUs(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.CampaignTarget], error) {
	var out factualapi.Page[factualapi.CampaignTarget]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListCampaignTargetSKUs", id, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT id,canonical_sku_id,source_sku,sku_resolution_status,assignment_state,bid::text,COALESCE(bid_unit,''),observed_from,observed_to FROM advertising_target_intervals WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND observed_from<=$3 AND(observed_to IS NULL OR observed_to>$3)AND id>$4 ORDER BY id LIMIT $5`, c.SellerAccountID(), id, c.CutoffAt(), last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	var rowIDs []int64
	for rows.Next() {
		var x factualapi.CampaignTarget
		var canonical, source pgtype.Int8
		var bid pgtype.Text
		var to pgtype.Timestamptz
		var rowID int64
		if e = rows.Scan(&rowID, &canonical, &source, &x.ResolutionStatus, &x.AssignmentState, &bid, &x.Bid.ReasonCode, &x.ObservedFrom, &to); e != nil {
			rows.Close()
			return out, e
		}
		if canonical.Valid {
			x.CanonicalSKUID = &canonical.Int64
		}
		if source.Valid {
			x.SourceSKU = &source.Int64
		}
		if to.Valid {
			x.ObservedTo = &to.Time
		}
		x.Bid = moneyValue(bid, "RUB", "BID_UNAVAILABLE")
		out.Items = append(out.Items, x)
		rowIDs = append(rowIDs, rowID)
	}
	rows.Close()
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
		rowIDs = rowIDs[:n]
	}
	if len(rowIDs) > 0 {
		out.NextPageToken, e = s.next(c, "ListCampaignTargetSKUs", h, rowIDs[len(rowIDs)-1], has)
	}
	return out, e
}

func (s *Service) BatchListCampaignTargetSKUs(ctx context.Context, c factualapi.FactualReadContext, campaignIDs []int64, perCampaignLimit int) (factualapi.BatchCampaignTargetSKUResult, error) {
	var out factualapi.BatchCampaignTargetSKUResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(campaignIDs) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if perCampaignLimit <= 0 || len(campaignIDs) == 0 {
		return out, nil
	}
	fetchLimit := perCampaignLimit + 1
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,camp.id found FROM requested r LEFT JOIN advertising_campaigns camp ON camp.id=r.id AND camp.seller_account_id=$1 AND camp.first_observed_at<=$3),latest AS(SELECT DISTINCT ON(advertising_campaign_id,canonical_sku_id)advertising_campaign_id,canonical_sku_id,source_sku,sku_resolution_status,assignment_state,bid::text bid,COALESCE(bid_unit,'')bid_unit,observed_from,observed_to,id FROM advertising_target_intervals WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND observed_from<=$3 AND(observed_to IS NULL OR observed_to>$3)ORDER BY advertising_campaign_id,canonical_sku_id,observed_from DESC,id DESC),ranked AS(SELECT l.*,ROW_NUMBER()OVER(PARTITION BY advertising_campaign_id ORDER BY canonical_sku_id)rn,COUNT(*)OVER(PARTITION BY advertising_campaign_id)total FROM latest l)SELECT e.id,e.found,r.advertising_campaign_id,r.canonical_sku_id,r.source_sku,r.sku_resolution_status,r.assignment_state,r.bid,r.bid_unit,r.observed_from,r.observed_to,r.id,r.rn,r.total FROM eligible e LEFT JOIN ranked r ON r.advertising_campaign_id=e.found AND r.rn<=$4 ORDER BY e.ord,r.rn`, c.SellerAccountID(), campaignIDs, c.CutoffAt(), fetchLimit)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	type campaignBucket struct {
		status     factualapi.BatchStatus
		items      []factualapi.CampaignTarget
		hasMore    bool
		totalCount int
	}
	buckets := map[int64]*campaignBucket{}
	for _, id := range campaignIDs {
		buckets[id] = &campaignBucket{status: factualapi.BatchNotFound}
	}
	for rows.Next() {
		var id int64
		var found, campaignID, canonicalSKU, sourceSKU, rowID, rn, total pgtype.Int8
		var resolution, assignment, bid, bidUnit pgtype.Text
		var from pgtype.Timestamptz
		var to pgtype.Timestamptz
		if e = rows.Scan(&id, &found, &campaignID, &canonicalSKU, &sourceSKU, &resolution, &assignment, &bid, &bidUnit, &from, &to, &rowID, &rn, &total); e != nil {
			return out, dbError(e)
		}
		b := buckets[id]
		if b == nil {
			continue
		}
		if !found.Valid {
			continue
		}
		if !rowID.Valid {
			b.status = factualapi.BatchFoundEmpty
			continue
		}
		b.status = factualapi.BatchFound
		var x factualapi.CampaignTarget
		if canonicalSKU.Valid {
			v := canonicalSKU.Int64
			x.CanonicalSKUID = &v
		}
		if sourceSKU.Valid {
			v := sourceSKU.Int64
			x.SourceSKU = &v
		}
		x.ResolutionStatus = resolution.String
		x.AssignmentState = assignment.String
		if from.Valid {
			x.ObservedFrom = from.Time
		}
		if to.Valid {
			t := to.Time
			x.ObservedTo = &t
		}
		x.Bid = moneyValue(bid, "RUB", "BID_UNAVAILABLE")
		b.items = append(b.items, x)
		if total.Valid {
			b.totalCount = int(total.Int64)
			if total.Int64 > int64(perCampaignLimit) {
				b.hasMore = true
			}
		}
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, id := range campaignIDs {
		b := buckets[id]
		if b == nil {
			continue
		}
		status := b.status
		if status == factualapi.BatchFound && len(b.items) > perCampaignLimit {
			b.items = b.items[:perCampaignLimit]
			b.hasMore = true
		}
		if status == factualapi.BatchFound && len(b.items) == 0 && !b.hasMore {
			status = factualapi.BatchFoundEmpty
		}
		out.Items = append(out.Items, factualapi.BatchCampaignTargetSKUItem{
			CampaignID: id,
			Status:     status,
			Items:      b.items,
			TotalCount: b.totalCount,
			HasMore:    b.hasMore,
		})
	}
	return out, nil
}

func (s *Service) GetCampaignDailyFacts(ctx context.Context, c factualapi.FactualReadContext, id int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) ([]factualapi.AdvertisingDailyFact, error) {
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `SELECT campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount::text,bonus_spend_amount::text,attributed_order_units,attributed_order_amount::text,attribution_kind,provider_drr::text,currency_code,traffic_state,attribution_state FROM(SELECT DISTINCT ON(business_date,attribution_kind)advertising_campaign_id campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,bonus_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,provider_drr,currency_code,traffic_state,attribution_state FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC)x ORDER BY business_date,attribution_kind`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	var out []factualapi.AdvertisingDailyFact
	for rows.Next() {
		var x factualapi.AdvertisingDailyFact
		var impressions, clicks, orders pgtype.Int8
		var paid, bonus, amount, drr pgtype.Text
		if e = rows.Scan(&x.CampaignID, &x.BusinessDate, &x.RevisionNo, &impressions, &clicks, &paid, &bonus, &orders, &amount, &x.AttributionKind, &drr, &x.PaidSpend.ReasonCode, &x.TrafficState, &x.AttributionState); e != nil {
			return nil, e
		}
		currency := x.PaidSpend.ReasonCode
		x.Impressions = intValue(impressions, "IMPRESSIONS_UNAVAILABLE")
		x.Clicks = intValue(clicks, "CLICKS_UNAVAILABLE")
		x.PaidSpend = moneyValue(paid, currency, "PAID_SPEND_UNAVAILABLE")
		x.BonusSpend = moneyValue(bonus, currency, "BONUS_SPEND_UNAVAILABLE")
		x.AttributedOrderUnits = intValue(orders, "ATTRIBUTED_UNITS_UNAVAILABLE")
		x.AttributedOrderAmount = moneyValue(amount, currency, "ATTRIBUTION_UNAVAILABLE")
		x.ProviderDRR = decimalValue(textValue(drr), "PROVIDER_DRR_UNAVAILABLE")
		out = append(out, x)
	}
	hash := factHash(out)
	for i := range out {
		out[i].Meta, _ = s.meta(c, "ADVERTISING.CAMPAIGN_TRAFFIC", "AD_DAILY", id, &p, hash)
	}
	return out, rows.Err()
}
func textValue(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return ""
}
func (s *Service) GetCampaignSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) ([]factualapi.AdvertisingDailyFact, error) {
	return s.GetCampaignDailyFacts(ctx, c, id, standard, explicit)
}

func (s *Service) GetCampaignDRR(ctx context.Context, c factualapi.FactualReadContext, id int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.CampaignDRR, error) {
	var out factualapi.CampaignDRR
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	out.CampaignID = id
	out.FormulaPolicy = "campaign_drr_formula_v1"
	var paid, amount pgtype.Text
	var currencies int64
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)paid_spend_amount,attributed_order_amount,currency_code FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT sum(paid_spend_amount)::text,sum(attributed_order_amount)::text,count(DISTINCT currency_code)FROM latest`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt()).Scan(&paid, &amount, &currencies)
	if e != nil {
		return out, dbError(e)
	}
	if currencies != 1 {
		out.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
		out.AttributedOrderAmount = out.PaidSpend
		out.CalculatedDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "METRICS_NOT_COMPARABLE")
	} else {
		currency, e := singleCampaignCurrency(ctx, s.pool, c, id, p)
		if e != nil {
			return out, e
		}
		out.PaidSpend = moneyValue(paid, currency, "PAID_SPEND_UNAVAILABLE")
		out.AttributedOrderAmount = moneyValue(amount, currency, "ATTRIBUTION_UNAVAILABLE")
		out.CalculatedDRRPct = ratioMoney(out.PaidSpend, out.AttributedOrderAmount)
	}
	out.ProviderDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "PROVIDER_DRR_RECONCILIATION_NOT_AGGREGATED")
	var providerDRR pgtype.Text
	var providerCount int64
	_ = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)provider_drr FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 AND provider_drr IS NOT NULL ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT avg(provider_drr)::text,count(*)FROM latest`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt()).Scan(&providerDRR, &providerCount)
	if providerCount > 0 && providerDRR.Valid {
		out.ProviderDRRPct = decimalValue(providerDRR.String, "PROVIDER_DRR_UNAVAILABLE")
	}
	out.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_DRR", "CAMPAIGN_DRR", id, &p, factHash(out))
	return out, e
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func singleCampaignCurrency(ctx context.Context, pool queryRower, c factualapi.FactualReadContext, id int64, p factualapi.Period) (string, error) {
	var code string
	e := pool.QueryRow(ctx, `SELECT currency_code FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt()).Scan(&code)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", nil
	}
	return code, e
}
func ratioMoney(num, den factualapi.Value[factualapi.Money]) factualapi.Value[factualapi.Decimal] {
	if num.State != factualapi.ValueKnown || num.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if den.State != factualapi.ValueKnown || den.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if num.Value.CurrencyCode != den.Value.CurrencyCode {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "CURRENCY_NOT_COMPARABLE")
	}
	if den.Value.Amount.IsZero() {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a, _ := new(big.Rat).SetString(num.Value.Amount.String())
	b, _ := new(big.Rat).SetString(den.Value.Amount.String())
	a.Quo(a, b)
	a.Mul(a, big.NewRat(100, 1))
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(d)
}

func ratioMoneyPerCount(spend factualapi.Value[factualapi.Money], count factualapi.Value[int64]) factualapi.Value[factualapi.Decimal] {
	if spend.State != factualapi.ValueKnown || spend.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if count.State != factualapi.ValueKnown || count.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if *count.Value == 0 {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a, _ := new(big.Rat).SetString(spend.Value.Amount.String())
	b := new(big.Rat).SetInt64(*count.Value)
	a.Quo(a, b)
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(d)
}

func ratioMoneyPerThousand(spend factualapi.Value[factualapi.Money], impressions factualapi.Value[int64]) factualapi.Value[factualapi.Decimal] {
	if spend.State != factualapi.ValueKnown || spend.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if impressions.State != factualapi.ValueKnown || impressions.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if *impressions.Value == 0 {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a, _ := new(big.Rat).SetString(spend.Value.Amount.String())
	b := new(big.Rat).SetInt64(*impressions.Value)
	a.Quo(a, b)
	a.Mul(a, big.NewRat(1000, 1))
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(d)
}

func (s *Service) GetAdvertisingAccountSummary(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.AdvertisingAccountSummary, error) {
	var out factualapi.AdvertisingAccountSummary
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	out.Period = p
	var paid pgtype.Text
	var currencies int64
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date)paid_spend_amount,currency_code FROM advertising_daily_facts WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$4 ORDER BY advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC)SELECT sum(paid_spend_amount)::text,count(DISTINCT currency_code)FROM latest`, c.SellerAccountID(), p.From, p.To, c.CutoffAt()).Scan(&paid, &currencies)
	if e != nil {
		return out, dbError(e)
	}
	if currencies == 1 {
		var currency string
		e = s.pool.QueryRow(ctx, `SELECT currency_code FROM advertising_daily_facts WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 LIMIT 1`, c.SellerAccountID(), p.From, p.To, c.CutoffAt()).Scan(&currency)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		out.TotalPaidSpend = moneyValue(paid, currency, "PAID_SPEND_UNAVAILABLE")
	} else if currencies > 1 {
		out.TotalPaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
	} else {
		out.TotalPaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "NO_ADVERTISING_FACTS")
	}
	out.AccountAttributedDRR = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "ATTRIBUTION_AGGREGATION_PENDING_ACCEPTANCE")
	out.Meta, e = s.meta(c, "ADVERTISING.ACCOUNT_SPEND", "AD_ACCOUNT", c.SellerAccountID(), &p, factHash(out))
	return out, e
}
func (s *Service) GetAdvertisingSpendShare(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.AdvertisingSpendShare, error) {
	var out factualapi.AdvertisingSpendShare
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	ads, e := s.GetAdvertisingAccountSummary(ctx, c, "", &p)
	if e != nil {
		return out, e
	}
	sales, e := s.GetAccountDemand(ctx, c, "", &p)
	if e != nil {
		return out, e
	}
	out.Period = p
	out.PaidSpend = ads.TotalPaidSpend
	out.SalesOrderedAmount = sales.OrderedAmount
	out.SpendSharePct = ratioMoney(out.PaidSpend, out.SalesOrderedAmount)
	out.AdvertisingFactRef = ads.Meta.FactRef
	out.SalesFactRef = sales.Meta.FactRef
	out.FormulaPolicy = "advertising_spend_share_formula_v1"
	out.Meta, e = s.meta(c, "ADVERTISING.ACCOUNT_SPEND_SHARE", "AD_SPEND_SHARE", c.SellerAccountID(), &p, factHash(out))
	return out, e
}
func (s *Service) GetSKUAdvertising(ctx context.Context, c factualapi.FactualReadContext, id int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.SKUAdvertising, error) {
	var out factualapi.SKUAdvertising
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	out.CanonicalSKUID = id
	out.Period = p
	rows, e := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date,attribution_kind)attribution_kind,impressions,clicks,paid_spend_amount,attributed_order_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC)SELECT attribution_kind,sum(impressions),sum(clicks),sum(paid_spend_amount)::text,sum(attributed_order_amount)::text,count(DISTINCT currency_code),max(currency_code)FROM latest GROUP BY attribution_kind ORDER BY attribution_kind`, c.SellerAccountID(), id, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	kindCount := 0
	for rows.Next() {
		var slice factualapi.SKUAdvertisingKindSlice
		var impressions, clicks pgtype.Int8
		var paid, amount pgtype.Text
		var currencies int64
		var currency pgtype.Text
		if e = rows.Scan(&slice.AttributionKind, &impressions, &clicks, &paid, &amount, &currencies, &currency); e != nil {
			return out, e
		}
		slice.Impressions = intValue(impressions, "SKU_TRAFFIC_UNAVAILABLE")
		slice.Clicks = intValue(clicks, "SKU_TRAFFIC_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		if currencies == 1 {
			slice.PaidSpend = moneyValue(paid, code, "SKU_SPEND_UNAVAILABLE")
			slice.AttributedOrderAmount = moneyValue(amount, code, "SKU_ATTRIBUTION_UNAVAILABLE")
		} else if currencies > 1 {
			slice.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			slice.AttributedOrderAmount = slice.PaidSpend
		} else {
			slice.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			slice.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		}
		out.ByAttributionKind = append(out.ByAttributionKind, slice)
		kindCount++
		if slice.AttributionKind == "PROVIDER_DEFAULT" {
			out.AttributionKind = "PROVIDER_DEFAULT"
			out.Impressions = slice.Impressions
			out.Clicks = slice.Clicks
			out.PaidSpend = slice.PaidSpend
			out.AttributedOrderAmount = slice.AttributedOrderAmount
		}
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	var limitations []factualapi.Limitation
	if kindCount == 0 {
		out.Impressions = factualapi.Missing[int64](factualapi.ValueUnavailable, "SKU_TRAFFIC_UNAVAILABLE")
		out.Clicks = factualapi.Missing[int64](factualapi.ValueUnavailable, "SKU_TRAFFIC_UNAVAILABLE")
		out.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
		out.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
	} else if out.AttributionKind == "" {
		out.AttributionKind = "UNKNOWN"
		out.Impressions = factualapi.Missing[int64](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
		out.Clicks = factualapi.Missing[int64](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
		out.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
		out.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
		limitations = append(limitations, factualapi.Limitation{Code: "ATTRIBUTION_KINDS_NOT_AGGREGATABLE", Message: "attributed metrics are not summed across attribution kinds"})
	} else if kindCount > 1 {
		limitations = append(limitations, factualapi.Limitation{Code: "ATTRIBUTION_KINDS_NOT_AGGREGATABLE", Message: "summary uses PROVIDER_DEFAULT only; other kinds are exposed in ByAttributionKind"})
	}
	out.Meta, e = s.meta(c, "ADVERTISING.SKU_TRAFFIC", "SKU_ADVERTISING", id, &p, factHash(out))
	if e != nil {
		return out, e
	}
	out.Meta.Limitations = append(out.Meta.Limitations, limitations...)
	return out, nil
}

func (s *Service) BatchGetSKUAdvertising(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchSKUAdvertisingResult, error) {
	var out factualapi.BatchSKUAdvertisingResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(promoted_canonical_sku_id,advertising_campaign_id,business_date,attribution_kind)promoted_canonical_sku_id,attribution_kind,impressions,clicks,paid_spend_amount,attributed_order_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY promoted_canonical_sku_id,advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT promoted_canonical_sku_id,attribution_kind,sum(impressions)impressions,sum(clicks)clicks,sum(paid_spend_amount)::text paid,sum(attributed_order_amount)::text amount,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY promoted_canonical_sku_id,attribution_kind)SELECT e.id,e.found,a.attribution_kind,a.impressions,a.clicks,a.paid,a.amount,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.promoted_canonical_sku_id=e.found ORDER BY e.ord,a.attribution_kind`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	foundSet := map[int64]bool{}
	slicesBySKU := map[int64][]factualapi.SKUAdvertisingKindSlice{}
	seenKind := map[int64]map[string]bool{}
	for rows.Next() {
		var id int64
		var found pgtype.Int8
		var kind pgtype.Text
		var impressions, clicks pgtype.Int8
		var paid, amount, currency pgtype.Text
		var currencies pgtype.Int8
		if e = rows.Scan(&id, &found, &kind, &impressions, &clicks, &paid, &amount, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if found.Valid {
			foundSet[id] = true
		}
		if !found.Valid || !kind.Valid {
			continue
		}
		if seenKind[id] == nil {
			seenKind[id] = map[string]bool{}
		}
		if seenKind[id][kind.String] {
			continue
		}
		seenKind[id][kind.String] = true
		slice := factualapi.SKUAdvertisingKindSlice{AttributionKind: kind.String}
		slice.Impressions = intValue(impressions, "SKU_TRAFFIC_UNAVAILABLE")
		slice.Clicks = intValue(clicks, "SKU_TRAFFIC_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		switch {
		case currencies.Valid && currencies.Int64 == 1:
			slice.PaidSpend = moneyValue(paid, code, "SKU_SPEND_UNAVAILABLE")
			slice.AttributedOrderAmount = moneyValue(amount, code, "SKU_ATTRIBUTION_UNAVAILABLE")
		case currencies.Valid && currencies.Int64 > 1:
			slice.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			slice.AttributedOrderAmount = slice.PaidSpend
		default:
			slice.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			slice.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		}
		slicesBySKU[id] = append(slicesBySKU[id], slice)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, id := range ids {
		if !foundSet[id] {
			out.Items = append(out.Items, factualapi.BatchSKUAdvertisingItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		slices := slicesBySKU[id]
		v := factualapi.SKUAdvertising{CanonicalSKUID: id, Period: p, ByAttributionKind: slices}
		var limitations []factualapi.Limitation
		kindCount := len(slices)
		for _, slice := range slices {
			if slice.AttributionKind == "PROVIDER_DEFAULT" {
				v.AttributionKind = "PROVIDER_DEFAULT"
				v.Impressions = slice.Impressions
				v.Clicks = slice.Clicks
				v.PaidSpend = slice.PaidSpend
				v.AttributedOrderAmount = slice.AttributedOrderAmount
			}
		}
		if kindCount == 0 {
			v.Impressions = factualapi.Missing[int64](factualapi.ValueUnavailable, "SKU_TRAFFIC_UNAVAILABLE")
			v.Clicks = factualapi.Missing[int64](factualapi.ValueUnavailable, "SKU_TRAFFIC_UNAVAILABLE")
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		} else if v.AttributionKind == "" {
			v.AttributionKind = "UNKNOWN"
			v.Impressions = factualapi.Missing[int64](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
			v.Clicks = factualapi.Missing[int64](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "ATTRIBUTION_KINDS_NOT_AGGREGATABLE")
			limitations = append(limitations, factualapi.Limitation{Code: "ATTRIBUTION_KINDS_NOT_AGGREGATABLE", Message: "attributed metrics are not summed across attribution kinds"})
		} else if kindCount > 1 {
			limitations = append(limitations, factualapi.Limitation{Code: "ATTRIBUTION_KINDS_NOT_AGGREGATABLE", Message: "summary uses PROVIDER_DEFAULT only; other kinds are exposed in ByAttributionKind"})
		}
		v.Meta, e = s.meta(c, "ADVERTISING.SKU_TRAFFIC", "SKU_ADVERTISING", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		v.Meta.Limitations = append(v.Meta.Limitations, limitations...)
		status := factualapi.BatchFound
		if kindCount > 0 && v.AttributionKind != "PROVIDER_DEFAULT" {
			status = factualapi.BatchLimited
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchSKUAdvertisingItem{ID: id, Status: status, Advertising: &fact})
	}
	return out, nil
}

func (s *Service) BatchGetCampaignPeriodPerformance(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchCampaignPeriodPerformanceResult, error) {
	var out factualapi.BatchCampaignPeriodPerformanceResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	p, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,c.id found FROM requested r LEFT JOIN advertising_campaigns c ON c.id=r.id AND c.seller_account_id=$1 AND c.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date)advertising_campaign_id,impressions,clicks,paid_spend_amount,bonus_spend_amount,attributed_order_units,attributed_order_amount,currency_code,provider_drr FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT advertising_campaign_id,sum(impressions)::bigint impressions,sum(clicks)::bigint clicks,sum(paid_spend_amount)::text paid,sum(bonus_spend_amount)::text bonus,sum(attributed_order_units)::bigint units,sum(attributed_order_amount)::text amount,avg(provider_drr)::text provider_drr,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY advertising_campaign_id)SELECT e.id,e.found,a.impressions,a.clicks,a.paid,a.bonus,a.units,a.amount,a.provider_drr,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.advertising_campaign_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, impressions, clicks, units, currencies pgtype.Int8
		var paid, bonus, amount, providerDRR, currency pgtype.Text
		if e = rows.Scan(&id, &found, &impressions, &clicks, &paid, &bonus, &units, &amount, &providerDRR, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCampaignPeriodPerformanceItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		v := factualapi.CampaignPeriodPerformance{CampaignID: id, Period: p, FormulaPolicy: "campaign_period_performance_v1"}
		hasFacts := impressions.Valid || clicks.Valid || paid.Valid || bonus.Valid || units.Valid || amount.Valid
		if !hasFacts {
			v.Impressions = factualapi.Missing[int64](factualapi.ValueUnavailable, "IMPRESSIONS_UNAVAILABLE")
			v.Clicks = factualapi.Missing[int64](factualapi.ValueUnavailable, "CLICKS_UNAVAILABLE")
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "PAID_SPEND_UNAVAILABLE")
			v.BonusSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "BONUS_SPEND_UNAVAILABLE")
			v.AttributedOrderUnits = factualapi.Missing[int64](factualapi.ValueUnavailable, "ATTRIBUTED_UNITS_UNAVAILABLE")
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "ATTRIBUTION_UNAVAILABLE")
			v.CTRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "CTR_UNAVAILABLE")
			v.CalculatedDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "DRR_UNAVAILABLE")
			v.CPC = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "CPC_UNAVAILABLE")
			v.CPM = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "CPM_UNAVAILABLE")
			v.ProviderDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "PROVIDER_DRR_UNAVAILABLE")
		} else {
			v.Impressions = intValue(impressions, "IMPRESSIONS_UNAVAILABLE")
			v.Clicks = intValue(clicks, "CLICKS_UNAVAILABLE")
			v.AttributedOrderUnits = intValue(units, "ATTRIBUTED_UNITS_UNAVAILABLE")
			code := ""
			if currency.Valid {
				code = currency.String
			}
			if currencies.Valid && currencies.Int64 == 1 {
				v.PaidSpend = moneyValue(paid, code, "PAID_SPEND_UNAVAILABLE")
				v.BonusSpend = moneyValue(bonus, code, "BONUS_SPEND_UNAVAILABLE")
				v.AttributedOrderAmount = moneyValue(amount, code, "ATTRIBUTION_UNAVAILABLE")
				v.CalculatedDRRPct = ratioMoney(v.PaidSpend, v.AttributedOrderAmount)
				v.CPC = ratioMoneyPerCount(v.PaidSpend, v.Clicks)
				v.CPM = ratioMoneyPerThousand(v.PaidSpend, v.Impressions)
			} else if currencies.Valid && currencies.Int64 > 1 {
				v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
				v.BonusSpend = v.PaidSpend
				v.AttributedOrderAmount = v.PaidSpend
				v.CalculatedDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "METRICS_NOT_COMPARABLE")
				v.CPC = v.CalculatedDRRPct
				v.CPM = v.CalculatedDRRPct
			} else {
				v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "PAID_SPEND_UNAVAILABLE")
				v.BonusSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "BONUS_SPEND_UNAVAILABLE")
				v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "ATTRIBUTION_UNAVAILABLE")
				v.CalculatedDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "DRR_UNAVAILABLE")
				v.CPC = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "CPC_UNAVAILABLE")
				v.CPM = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "CPM_UNAVAILABLE")
			}
			v.CTRPct = ratioCountsPct(v.Clicks, v.Impressions)
			if providerDRR.Valid && providerDRR.String != "" {
				v.ProviderDRRPct = decimalValue(providerDRR.String, "PROVIDER_DRR_UNAVAILABLE")
			} else {
				v.ProviderDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "PROVIDER_DRR_UNAVAILABLE")
			}
		}
		v.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_TRAFFIC", "CAMPAIGN_PERIOD_PERFORMANCE", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCampaignPeriodPerformanceItem{ID: id, Status: factualapi.BatchFound, Performance: &fact})
	}
	return out, rows.Err()
}

type skuCampaignPerfFilter struct {
	SKUID  int64
	Period factualapi.Period
}

func (s *Service) ListSKUCampaignPerformance(ctx context.Context, c factualapi.FactualReadContext, skuID int64, standard factualapi.StandardPeriod, explicit *factualapi.Period, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUCampaignPerformance], error) {
	var out factualapi.Page[factualapi.SKUCampaignPerformance]
	if e := s.validate(c); e != nil {
		return out, e
	}
	period, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	filter := skuCampaignPerfFilter{SKUID: skuID, Period: period}
	skip, h, e := s.cursor(c, "ListSKUCampaignPerformance", filter, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date,attribution_kind)advertising_campaign_id,paid_spend_amount,attributed_order_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=$2 AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT advertising_campaign_id campaign_id,sum(paid_spend_amount)::text paid,sum(attributed_order_amount)::text amount,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY advertising_campaign_id),ranked AS(SELECT campaign_id,paid,amount,currencies,currency,ROW_NUMBER()OVER(ORDER BY NULLIF(paid,'')::numeric DESC NULLS LAST,campaign_id)rn FROM agg)SELECT campaign_id,paid,amount,currencies,currency,rn FROM ranked WHERE rn>$6 ORDER BY rn LIMIT $7`, c.SellerAccountID(), skuID, period.From, period.To, c.CutoffAt(), skip, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	var rowNums []int64
	for rows.Next() {
		var x factualapi.SKUCampaignPerformance
		var paid, amount, currency pgtype.Text
		var currencies pgtype.Int8
		var rn int64
		if e = rows.Scan(&x.CampaignID, &paid, &amount, &currencies, &currency, &rn); e != nil {
			return out, e
		}
		x.Period = period
		code := ""
		if currency.Valid {
			code = currency.String
		}
		switch {
		case currencies.Valid && currencies.Int64 == 1:
			x.PaidSpend = moneyValue(paid, code, "SKU_SPEND_UNAVAILABLE")
			x.AttributedOrderAmount = moneyValue(amount, code, "SKU_ATTRIBUTION_UNAVAILABLE")
		case currencies.Valid && currencies.Int64 > 1:
			x.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			x.AttributedOrderAmount = x.PaidSpend
		default:
			x.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			x.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		}
		x.Meta, _ = s.meta(c, "ADVERTISING.SKU_TRAFFIC", "SKU_CAMPAIGN_PERFORMANCE", x.CampaignID, &period, factHash(x))
		out.Items = append(out.Items, x)
		rowNums = append(rowNums, rn)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
		rowNums = rowNums[:n]
	}
	if len(rowNums) > 0 {
		out.NextPageToken, e = s.next(c, "ListSKUCampaignPerformance", h, rowNums[len(rowNums)-1], has)
	}
	return out, e
}

func (s *Service) BatchListSKUCampaignPerformance(ctx context.Context, c factualapi.FactualReadContext, skuIDs []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period, perSKULimit int) (factualapi.BatchSKUCampaignPerformanceResult, error) {
	var out factualapi.BatchSKUCampaignPerformanceResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(skuIDs) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if perSKULimit <= 0 {
		return out, nil
	}
	if len(skuIDs) == 0 {
		return out, nil
	}
	period, e := factualapi.ResolvePeriod(c, standard, explicit)
	if e != nil {
		return out, e
	}
	fetchLimit := perSKULimit + 1
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date,attribution_kind)advertising_campaign_id,promoted_canonical_sku_id,paid_spend_amount,attributed_order_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT promoted_canonical_sku_id sku_id,advertising_campaign_id campaign_id,sum(paid_spend_amount)::text paid,sum(attributed_order_amount)::text amount,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY promoted_canonical_sku_id,advertising_campaign_id),ranked AS(SELECT sku_id,campaign_id,paid,amount,currencies,currency,ROW_NUMBER()OVER(PARTITION BY sku_id ORDER BY NULLIF(paid,'')::numeric DESC NULLS LAST,campaign_id)rn,COUNT(*)OVER(PARTITION BY sku_id)total FROM agg)SELECT e.id,e.found,r.campaign_id,r.paid,r.amount,r.currencies,r.currency,r.rn,r.total FROM eligible e LEFT JOIN ranked r ON r.sku_id=e.found AND r.rn<=$6 ORDER BY e.ord,r.rn`, c.SellerAccountID(), skuIDs, period.From, period.To, c.CutoffAt(), fetchLimit)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	type skuBucket struct {
		status  factualapi.BatchStatus
		items   []factualapi.SKUCampaignPerformance
		hasMore bool
	}
	buckets := map[int64]*skuBucket{}
	for _, id := range skuIDs {
		buckets[id] = &skuBucket{status: factualapi.BatchNotFound}
	}
	for rows.Next() {
		var id int64
		var found, campaignID, currencies, rn, total pgtype.Int8
		var paid, amount, currency pgtype.Text
		if e = rows.Scan(&id, &found, &campaignID, &paid, &amount, &currencies, &currency, &rn, &total); e != nil {
			return out, dbError(e)
		}
		b := buckets[id]
		if b == nil {
			continue
		}
		if !found.Valid {
			continue
		}
		b.status = factualapi.BatchFound
		if !campaignID.Valid {
			continue
		}
		x := factualapi.SKUCampaignPerformance{CampaignID: campaignID.Int64, Period: period}
		code := ""
		if currency.Valid {
			code = currency.String
		}
		switch {
		case currencies.Valid && currencies.Int64 == 1:
			x.PaidSpend = moneyValue(paid, code, "SKU_SPEND_UNAVAILABLE")
			x.AttributedOrderAmount = moneyValue(amount, code, "SKU_ATTRIBUTION_UNAVAILABLE")
		case currencies.Valid && currencies.Int64 > 1:
			x.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			x.AttributedOrderAmount = x.PaidSpend
		default:
			x.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			x.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		}
		x.Meta, _ = s.meta(c, "ADVERTISING.SKU_TRAFFIC", "SKU_CAMPAIGN_PERFORMANCE", x.CampaignID, &period, factHash(x))
		b.items = append(b.items, x)
		if total.Valid && total.Int64 > int64(perSKULimit) {
			b.hasMore = true
		}
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, id := range skuIDs {
		b := buckets[id]
		if b == nil {
			continue
		}
		out.Items = append(out.Items, factualapi.BatchSKUCampaignPerformanceItem{
			SKUID:   id,
			Status:  b.status,
			Items:   b.items,
			HasMore: b.hasMore,
		})
	}
	return out, nil
}

func ratioCountsPct(num, den factualapi.Value[int64]) factualapi.Value[factualapi.Decimal] {
	if num.State != factualapi.ValueKnown || num.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if den.State != factualapi.ValueKnown || den.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if *den.Value == 0 {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a := new(big.Rat).SetInt64(*num.Value)
	b := new(big.Rat).SetInt64(*den.Value)
	a.Quo(a, b)
	a.Mul(a, big.NewRat(100, 1))
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(d)
}

func (s *Service) ListCPOAttribution(ctx context.Context, c factualapi.FactualReadContext, f factualapi.CPOFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.CPOAttribution], error) {
	var out factualapi.Page[factualapi.CPOAttribution]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListCPOAttribution", f, p)
	if e != nil {
		return out, e
	}
	var from, to any = nil, nil
	if f.Period != nil {
		from, to = f.Period.From, f.Period.To
	}
	rows, e := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(provider_fact_key)id,advertising_campaign_id,business_date,promoted_canonical_sku_id,ordered_canonical_sku_id,canonical_order_id,source_promoted_sku,source_ordered_sku,source_order_id,attribution_kind,attributed_order_amount,currency_code FROM advertising_cpo_facts WHERE seller_account_id=$1 AND observed_at<=$2 AND($3::bigint IS NULL OR advertising_campaign_id=$3)AND($4::date IS NULL OR business_date BETWEEN $4 AND $5)ORDER BY provider_fact_key,observed_at DESC,revision_no DESC,id DESC)SELECT id,advertising_campaign_id,business_date,promoted_canonical_sku_id,ordered_canonical_sku_id,canonical_order_id,source_promoted_sku,source_ordered_sku,source_order_id,attribution_kind,attributed_order_amount::text,currency_code FROM latest WHERE id>$6 ORDER BY id LIMIT $7`, c.SellerAccountID(), c.CutoffAt(), f.CampaignID, from, to, last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	var ids []int64
	for rows.Next() {
		var x factualapi.CPOAttribution
		var promoted, ordered, order, sourcePromoted, sourceOrdered, sourceOrder pgtype.Int8
		var amount, currency pgtype.Text
		var rowID int64
		if e = rows.Scan(&rowID, &x.CampaignID, &x.BusinessDate, &promoted, &ordered, &order, &sourcePromoted, &sourceOrdered, &sourceOrder, &x.AttributionKind, &amount, &currency); e != nil {
			rows.Close()
			return out, e
		}
		if promoted.Valid {
			x.PromotedCanonicalSKUID = &promoted.Int64
		}
		if ordered.Valid {
			x.OrderedCanonicalSKUID = &ordered.Int64
		}
		if order.Valid {
			x.CanonicalOrderID = &order.Int64
		}
		if sourcePromoted.Valid {
			x.SourcePromotedSKU = &sourcePromoted.Int64
		}
		if sourceOrdered.Valid {
			x.SourceOrderedSKU = &sourceOrdered.Int64
		}
		if sourceOrder.Valid {
			x.SourceOrderID = &sourceOrder.Int64
		}
		x.AttributedAmount = moneyValue(amount, textValue(currency), "ATTRIBUTION_UNAVAILABLE")
		x.Meta, _ = s.meta(c, "ADVERTISING.CAMPAIGN_ATTRIBUTION", "CPO_ATTRIBUTION", rowID, nil, factHash(x))
		out.Items = append(out.Items, x)
		ids = append(ids, rowID)
	}
	rows.Close()
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
		ids = ids[:n]
	}
	if len(ids) > 0 {
		out.NextPageToken, e = s.next(c, "ListCPOAttribution", h, ids[len(ids)-1], has)
	}
	return out, e
}

func (s *Service) getCPOAttribution(ctx context.Context, c factualapi.FactualReadContext, rowID int64) (factualapi.CPOAttribution, error) {
	var x factualapi.CPOAttribution
	var promoted, ordered, order, sourcePromoted, sourceOrdered, sourceOrder pgtype.Int8
	var amount, currency pgtype.Text
	e := s.pool.QueryRow(ctx, `SELECT advertising_campaign_id,business_date,promoted_canonical_sku_id,ordered_canonical_sku_id,canonical_order_id,source_promoted_sku,source_ordered_sku,source_order_id,attribution_kind,attributed_order_amount::text,currency_code FROM advertising_cpo_facts WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), rowID, c.CutoffAt()).Scan(&x.CampaignID, &x.BusinessDate, &promoted, &ordered, &order, &sourcePromoted, &sourceOrdered, &sourceOrder, &x.AttributionKind, &amount, &currency)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("CPO attribution")
	}
	if e != nil {
		return x, dbError(e)
	}
	if promoted.Valid {
		x.PromotedCanonicalSKUID = &promoted.Int64
	}
	if ordered.Valid {
		x.OrderedCanonicalSKUID = &ordered.Int64
	}
	if order.Valid {
		x.CanonicalOrderID = &order.Int64
	}
	if sourcePromoted.Valid {
		x.SourcePromotedSKU = &sourcePromoted.Int64
	}
	if sourceOrdered.Valid {
		x.SourceOrderedSKU = &sourceOrdered.Int64
	}
	if sourceOrder.Valid {
		x.SourceOrderID = &sourceOrder.Int64
	}
	x.AttributedAmount = moneyValue(amount, textValue(currency), "ATTRIBUTION_UNAVAILABLE")
	x.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_ATTRIBUTION", "CPO_ATTRIBUTION", rowID, nil, factHash(x))
	return x, e
}
