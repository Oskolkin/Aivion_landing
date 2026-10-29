package factual

import (
	"context"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgtype"
)

func ratioSpendPerCount(spend factualapi.Value[factualapi.Money], count factualapi.Value[int64]) factualapi.Value[factualapi.Money] {
	if spend.State != factualapi.ValueKnown || spend.Value == nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if count.State != factualapi.ValueKnown || count.Value == nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if *count.Value == 0 {
		return factualapi.Missing[factualapi.Money](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a, ok := new(big.Rat).SetString(spend.Value.Amount.String())
	if !ok {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	a.Quo(a, new(big.Rat).SetInt64(*count.Value))
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(factualapi.Money{Amount: d, CurrencyCode: spend.Value.CurrencyCode})
}

func ratioSpendPerThousand(spend factualapi.Value[factualapi.Money], impressions factualapi.Value[int64]) factualapi.Value[factualapi.Money] {
	if spend.State != factualapi.ValueKnown || spend.Value == nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "NUMERATOR_UNAVAILABLE")
	}
	if impressions.State != factualapi.ValueKnown || impressions.Value == nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "DENOMINATOR_UNAVAILABLE")
	}
	if *impressions.Value == 0 {
		return factualapi.Missing[factualapi.Money](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	a, ok := new(big.Rat).SetString(spend.Value.Amount.String())
	if !ok {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	a.Quo(a, new(big.Rat).SetInt64(*impressions.Value))
	a.Mul(a, big.NewRat(1000, 1))
	d, e := factualapi.ParseDecimal(a.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "RATIO_ARITHMETIC_FAILED")
	}
	return factualapi.Known(factualapi.Money{Amount: d, CurrencyCode: spend.Value.CurrencyCode})
}

func (s *Service) BatchGetCampaignTrafficPerformance(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchCampaignTrafficResult, error) {
	var out factualapi.BatchCampaignTrafficResult
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
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,c.id found FROM requested r LEFT JOIN advertising_campaigns c ON c.id=r.id AND c.seller_account_id=$1 AND c.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date)advertising_campaign_id,impressions,clicks,paid_spend_amount,bonus_spend_amount,currency_code FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT advertising_campaign_id,sum(impressions)::bigint impressions,sum(clicks)::bigint clicks,sum(paid_spend_amount)::text paid,sum(bonus_spend_amount)::text bonus,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY advertising_campaign_id)SELECT e.id,e.found,a.impressions,a.clicks,a.paid,a.bonus,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.advertising_campaign_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, impressions, clicks, currencies pgtype.Int8
		var paid, bonus, currency pgtype.Text
		if e = rows.Scan(&id, &found, &impressions, &clicks, &paid, &bonus, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCampaignTrafficItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		hasFacts := impressions.Valid || clicks.Valid || paid.Valid || bonus.Valid
		if !hasFacts {
			out.Items = append(out.Items, factualapi.BatchCampaignTrafficItem{ID: id, Status: factualapi.BatchFoundEmpty})
			continue
		}
		v := factualapi.CampaignTrafficPerformance{CampaignID: id, Period: p, FormulaPolicy: "campaign_traffic_cpc_cpm_ctr_v4"}
		v.Impressions = intValue(impressions, "IMPRESSIONS_UNAVAILABLE")
		v.Clicks = intValue(clicks, "CLICKS_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		if currencies.Valid && currencies.Int64 == 1 {
			v.PaidSpend = moneyValue(paid, code, "PAID_SPEND_UNAVAILABLE")
			v.BonusSpend = moneyValue(bonus, code, "BONUS_SPEND_UNAVAILABLE")
			v.CPC = ratioSpendPerCount(v.PaidSpend, v.Clicks)
			v.CPM = ratioSpendPerThousand(v.PaidSpend, v.Impressions)
		} else if currencies.Valid && currencies.Int64 > 1 {
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			v.BonusSpend = v.PaidSpend
			v.CPC = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "METRICS_NOT_COMPARABLE")
			v.CPM = v.CPC
		} else {
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "PAID_SPEND_UNAVAILABLE")
			v.BonusSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "BONUS_SPEND_UNAVAILABLE")
			v.CPC = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "CPC_UNAVAILABLE")
			v.CPM = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "CPM_UNAVAILABLE")
		}
		v.CTRPct = ratioCountsPct(v.Clicks, v.Impressions)
		v.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_TRAFFIC", "CAMPAIGN_TRAFFIC_PERIOD", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCampaignTrafficItem{ID: id, Status: factualapi.BatchFound, Traffic: &fact})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetCampaignAttributionPerformance(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchCampaignAttributionResult, error) {
	var out factualapi.BatchCampaignAttributionResult
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
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,c.id found FROM requested r LEFT JOIN advertising_campaigns c ON c.id=r.id AND c.seller_account_id=$1 AND c.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date)advertising_campaign_id,business_date,attributed_order_units,attributed_order_amount,currency_code,provider_drr FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT advertising_campaign_id,sum(attributed_order_units)::bigint units,sum(attributed_order_amount)::text amount,count(DISTINCT currency_code)currencies,max(currency_code)currency,(array_agg(provider_drr ORDER BY business_date DESC)FILTER(WHERE provider_drr IS NOT NULL))[1]::text provider_drr FROM latest GROUP BY advertising_campaign_id)SELECT e.id,e.found,a.units,a.amount,a.provider_drr,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.advertising_campaign_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, units, currencies pgtype.Int8
		var amount, providerDRR, currency pgtype.Text
		if e = rows.Scan(&id, &found, &units, &amount, &providerDRR, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCampaignAttributionItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		hasFacts := units.Valid || amount.Valid || providerDRR.Valid
		if !hasFacts {
			out.Items = append(out.Items, factualapi.BatchCampaignAttributionItem{ID: id, Status: factualapi.BatchFoundEmpty})
			continue
		}
		v := factualapi.CampaignAttributionPerformance{CampaignID: id, Period: p}
		v.AttributedOrderUnits = intValue(units, "ATTRIBUTED_UNITS_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		if currencies.Valid && currencies.Int64 == 1 {
			v.AttributedOrderAmount = moneyValue(amount, code, "ATTRIBUTION_UNAVAILABLE")
		} else if currencies.Valid && currencies.Int64 > 1 {
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
		} else {
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "ATTRIBUTION_UNAVAILABLE")
		}
		if providerDRR.Valid && providerDRR.String != "" {
			v.ProviderDRRPct = decimalValue(providerDRR.String, "PROVIDER_DRR_UNAVAILABLE")
		} else {
			v.ProviderDRRPct = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnavailable, "PROVIDER_DRR_UNAVAILABLE")
		}
		v.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_ATTRIBUTION", "CAMPAIGN_ATTRIBUTION_PERIOD", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCampaignAttributionItem{ID: id, Status: factualapi.BatchFound, Attribution: &fact})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetSKUAdvertisingTraffic(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchSKUAdvertisingTrafficResult, error) {
	var out factualapi.BatchSKUAdvertisingTrafficResult
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
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(promoted_canonical_sku_id,advertising_campaign_id,business_date)promoted_canonical_sku_id,impressions,clicks,paid_spend_amount,bonus_spend_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY promoted_canonical_sku_id,advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT promoted_canonical_sku_id,sum(impressions)::bigint impressions,sum(clicks)::bigint clicks,sum(paid_spend_amount)::text paid,sum(bonus_spend_amount)::text bonus,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY promoted_canonical_sku_id)SELECT e.id,e.found,a.impressions,a.clicks,a.paid,a.bonus,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.promoted_canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, impressions, clicks, currencies pgtype.Int8
		var paid, bonus, currency pgtype.Text
		if e = rows.Scan(&id, &found, &impressions, &clicks, &paid, &bonus, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUAdvertisingTrafficItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		hasFacts := impressions.Valid || clicks.Valid || paid.Valid || bonus.Valid
		if !hasFacts {
			out.Items = append(out.Items, factualapi.BatchSKUAdvertisingTrafficItem{ID: id, Status: factualapi.BatchFoundEmpty})
			continue
		}
		v := factualapi.SKUAdvertisingTraffic{CanonicalSKUID: id, Period: p}
		v.Impressions = intValue(impressions, "SKU_TRAFFIC_UNAVAILABLE")
		v.Clicks = intValue(clicks, "SKU_TRAFFIC_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		if currencies.Valid && currencies.Int64 == 1 {
			v.PaidSpend = moneyValue(paid, code, "SKU_SPEND_UNAVAILABLE")
			v.BonusSpend = moneyValue(bonus, code, "SKU_BONUS_UNAVAILABLE")
		} else if currencies.Valid && currencies.Int64 > 1 {
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			v.BonusSpend = v.PaidSpend
		} else {
			v.PaidSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_SPEND_UNAVAILABLE")
			v.BonusSpend = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_BONUS_UNAVAILABLE")
		}
		v.Meta, e = s.meta(c, "ADVERTISING.SKU_TRAFFIC", "SKU_ADVERTISING_TRAFFIC", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchSKUAdvertisingTrafficItem{ID: id, Status: factualapi.BatchFound, Traffic: &fact})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetSKUAdvertisingAttribution(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchSKUAdvertisingAttributionResult, error) {
	var out factualapi.BatchSKUAdvertisingAttributionResult
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
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$5),latest AS(SELECT DISTINCT ON(promoted_canonical_sku_id,advertising_campaign_id,business_date)promoted_canonical_sku_id,attributed_order_units,attributed_order_amount,currency_code FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=ANY($2)AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY promoted_canonical_sku_id,advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC),agg AS(SELECT promoted_canonical_sku_id,sum(attributed_order_units)::bigint units,sum(attributed_order_amount)::text amount,count(DISTINCT currency_code)currencies,max(currency_code)currency FROM latest GROUP BY promoted_canonical_sku_id)SELECT e.id,e.found,a.units,a.amount,a.currencies,a.currency FROM eligible e LEFT JOIN agg a ON a.promoted_canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, units, currencies pgtype.Int8
		var amount, currency pgtype.Text
		if e = rows.Scan(&id, &found, &units, &amount, &currencies, &currency); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchSKUAdvertisingAttributionItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		hasFacts := units.Valid || amount.Valid
		if !hasFacts {
			out.Items = append(out.Items, factualapi.BatchSKUAdvertisingAttributionItem{ID: id, Status: factualapi.BatchFoundEmpty})
			continue
		}
		v := factualapi.SKUAdvertisingAttribution{CanonicalSKUID: id, Period: p}
		v.AttributedOrderUnits = intValue(units, "SKU_ATTRIBUTED_UNITS_UNAVAILABLE")
		code := ""
		if currency.Valid {
			code = currency.String
		}
		if currencies.Valid && currencies.Int64 == 1 {
			v.AttributedOrderAmount = moneyValue(amount, code, "SKU_ATTRIBUTION_UNAVAILABLE")
		} else if currencies.Valid && currencies.Int64 > 1 {
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
		} else {
			v.AttributedOrderAmount = factualapi.Missing[factualapi.Money](factualapi.ValueUnavailable, "SKU_ATTRIBUTION_UNAVAILABLE")
		}
		v.Meta, e = s.meta(c, "ADVERTISING.SKU_ATTRIBUTION", "SKU_ADVERTISING_ATTRIBUTION", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchSKUAdvertisingAttributionItem{ID: id, Status: factualapi.BatchFound, Attribution: &fact})
	}
	return out, rows.Err()
}

func (s *Service) BatchGetCampaignTargetingSummary(ctx context.Context, c factualapi.FactualReadContext, ids []int64, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.BatchCampaignTargetingSummaryResult, error) {
	var out factualapi.BatchCampaignTargetingSummaryResult
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
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,c.id found FROM requested r LEFT JOIN advertising_campaigns c ON c.id=r.id AND c.seller_account_id=$1 AND c.first_observed_at<=$5),scope AS(SELECT DISTINCT ON(advertising_campaign_id)advertising_campaign_id,COALESCE(target_scope_type,'UNKNOWN')target_scope_type FROM advertising_campaign_versions WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND observed_at<=$5 ORDER BY advertising_campaign_id,observed_at DESC,version_no DESC,id DESC),assigned AS(SELECT advertising_campaign_id,count(DISTINCT canonical_sku_id)FILTER(WHERE canonical_sku_id IS NOT NULL)::bigint assigned FROM advertising_target_intervals WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND observed_from<=$5 AND(observed_to IS NULL OR observed_to>$5)GROUP BY advertising_campaign_id),latest_perf AS(SELECT DISTINCT ON(advertising_campaign_id,promoted_canonical_sku_id,business_date)advertising_campaign_id,promoted_canonical_sku_id,impressions,clicks,paid_spend_amount FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=ANY($2)AND promoted_canonical_sku_id IS NOT NULL AND business_date BETWEEN $3 AND $4 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$5 ORDER BY advertising_campaign_id,promoted_canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC),observed AS(SELECT advertising_campaign_id,count(DISTINCT promoted_canonical_sku_id)::bigint observed FROM latest_perf WHERE COALESCE(impressions,0)>0 OR COALESCE(clicks,0)>0 OR COALESCE(paid_spend_amount,0)>0 GROUP BY advertising_campaign_id)SELECT e.id,e.found,sc.target_scope_type,a.assigned,o.observed FROM eligible e LEFT JOIN scope sc ON sc.advertising_campaign_id=e.found LEFT JOIN assigned a ON a.advertising_campaign_id=e.found LEFT JOIN observed o ON o.advertising_campaign_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, p.From, p.To, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var found, assigned, observed pgtype.Int8
		var scope pgtype.Text
		if e = rows.Scan(&id, &found, &scope, &assigned, &observed); e != nil {
			return out, dbError(e)
		}
		if !found.Valid {
			out.Items = append(out.Items, factualapi.BatchCampaignTargetingSummaryItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		v := factualapi.CampaignTargetingSummary{CampaignID: id, TargetScope: scope.String}
		if v.TargetScope == "EXPLICIT_SKU_SET" {
			if assigned.Valid {
				v.ExplicitAssignedSKUCount = factualapi.Known(assigned.Int64)
			} else {
				v.ExplicitAssignedSKUCount = factualapi.Known(int64(0))
			}
		} else {
			v.ExplicitAssignedSKUCount = factualapi.Missing[int64](factualapi.ValueNotApplicable, "EXPLICIT_ASSIGNED_SKU_COUNT_NOT_APPLICABLE")
		}
		if observed.Valid {
			v.SKUWithObservedPerformanceCount = factualapi.Known(observed.Int64)
		} else {
			v.SKUWithObservedPerformanceCount = factualapi.Known(int64(0))
		}
		v.Meta, e = s.meta(c, "ADVERTISING.CAMPAIGN_TRAFFIC", "CAMPAIGN_TARGETING_SUMMARY", id, &p, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchCampaignTargetingSummaryItem{ID: id, Status: factualapi.BatchFound, Summary: &fact})
	}
	return out, rows.Err()
}
