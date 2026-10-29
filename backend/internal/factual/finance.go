package factual

import (
	"context"
	"errors"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) GetFinanceDailyLedger(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period) ([]factualapi.FinanceDailyNet, error) {
	if err := s.validate(c); err != nil {
		return nil, err
	}
	period, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(
		SELECT DISTINCT ON(business_date) id,business_date,revision_no,completeness_status
		FROM finance_accrual_day_revisions
		WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4
		ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)
		SELECT l.id,l.business_date,l.revision_no,l.completeness_status,
		       COALESCE(sum(r.total_amount_signed),0)::text,
		       count(DISTINCT r.currency_code),max(r.currency_code)
		FROM latest l LEFT JOIN finance_accrual_records r ON r.day_revision_id=l.id
		GROUP BY l.id,l.business_date,l.revision_no,l.completeness_status
		ORDER BY l.business_date`, c.SellerAccountID(), period.From, period.To, c.CutoffAt())
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.FinanceDailyNet
	for rows.Next() {
		var x factualapi.FinanceDailyNet
		var revisionID, currencies int64
		var amount string
		var currency pgtype.Text
		if err = rows.Scan(&revisionID, &x.BusinessDate, &x.RevisionNo, &x.CompletenessStatus, &amount, &currencies, &currency); err != nil {
			return nil, err
		}
		switch {
		case currencies == 0:
			x.Net = factualapi.Known(factualapi.SignedMoney{Amount: "0", CurrencyCode: "RUB"})
		case currencies == 1:
			x.Net = factualapi.Known(factualapi.SignedMoney{Amount: amount, CurrencyCode: textValue(currency)})
		default:
			x.Net = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
		}
		x.Meta, _ = s.meta(c, "FINANCE.DAILY_LEDGER", "FINANCE_DAY", revisionID, &period, factHash(x))
		if x.CompletenessStatus == "PARTIAL" {
			x.Meta.Limitations = append(x.Meta.Limitations, factualapi.Limitation{Code: "FINANCE_DAY_PARTIAL", Message: "one or more source pages are missing"})
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) GetFinanceAccountSummary(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.FinanceAccountSummary, error) {
	var out factualapi.FinanceAccountSummary
	period, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	days, err := s.GetFinanceDailyLedger(ctx, c, "", &period)
	if err != nil {
		return out, err
	}
	out.Period = period
	out.FormulaPolicy = "finance_account_net_parent_totals_only_v1"
	currency := ""
	for _, day := range days {
		if day.CompletenessStatus == "COMPLETE" {
			out.DaysComplete++
		} else {
			out.DaysPartial++
		}
		if day.Net.State != factualapi.ValueKnown || day.Net.Value == nil {
			out.AccountNet = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnknown, "FINANCE_DAYS_NOT_AGGREGATABLE")
			break
		}
		if currency == "" {
			currency = day.Net.Value.CurrencyCode
		}
		if currency != day.Net.Value.CurrencyCode {
			out.AccountNet = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			break
		}
	}
	if out.AccountNet.State == "" {
		var amount string
		var currencies int64
		var code pgtype.Text
		err = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)id FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT COALESCE(sum(r.total_amount_signed),0)::text,count(DISTINCT r.currency_code),max(r.currency_code)FROM latest l LEFT JOIN finance_accrual_records r ON r.day_revision_id=l.id`, c.SellerAccountID(), period.From, period.To, c.CutoffAt()).Scan(&amount, &currencies, &code)
		if err != nil {
			return out, dbError(err)
		}
		if currencies > 1 {
			out.AccountNet = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
		} else {
			if code.Valid {
				currency = code.String
			}
			if currency == "" {
				currency = "RUB"
			}
			out.AccountNet = factualapi.Known(factualapi.SignedMoney{Amount: amount, CurrencyCode: currency})
		}
	}
	out.Meta, err = s.meta(c, "FINANCE.ACCOUNT_NET", "FINANCE_ACCOUNT", c.SellerAccountID(), &period, factHash(out))
	return out, err
}

func (s *Service) GetFinanceAccountBreakdown(ctx context.Context, c factualapi.FactualReadContext, standard factualapi.StandardPeriod, explicit *factualapi.Period) (factualapi.FinanceAccountBreakdown, error) {
	var out factualapi.FinanceAccountBreakdown
	if err := s.validate(c); err != nil {
		return out, err
	}
	period, err := factualapi.ResolvePeriod(c, standard, explicit)
	if err != nil {
		return out, err
	}
	out.Period = period
	out.FormulaPolicy = "finance_account_breakdown_v1"
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)id FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT r.provider_type_id,COALESCE(r.provider_accrued_category,''),COALESCE(v.mapping_status,'UNMAPPED'),COALESCE(sum(r.total_amount_signed),0)::text,count(DISTINCT r.currency_code),max(r.currency_code)FROM latest l JOIN finance_accrual_records r ON r.day_revision_id=l.id LEFT JOIN finance_provider_type_versions v ON v.finance_provider_type_id=r.finance_provider_type_id AND v.valid_from<=$4 AND(v.valid_to IS NULL OR v.valid_to>$4)GROUP BY r.provider_type_id,r.provider_accrued_category,COALESCE(v.mapping_status,'UNMAPPED')ORDER BY r.provider_type_id,r.provider_accrued_category`, c.SellerAccountID(), period.From, period.To, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	currency := ""
	var mappedTotal, unmappedTotal string
	mappedSet, unmappedSet := false, false
	for rows.Next() {
		var comp factualapi.FinanceComponentAmount
		var amount string
		var currencies int64
		var code pgtype.Text
		if err = rows.Scan(&comp.ProviderTypeID, &comp.Category, &comp.MappingStatus, &amount, &currencies, &code); err != nil {
			return out, err
		}
		if currencies > 1 {
			out.MappedAmount = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			out.UnmappedAmount = out.MappedAmount
			out.MappingCoverage = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE")
			break
		}
		if code.Valid {
			currency = code.String
		}
		comp.Amount = factualapi.Known(factualapi.SignedMoney{Amount: amount, CurrencyCode: textValue(code)})
		out.Components = append(out.Components, comp)
		switch comp.MappingStatus {
		case "MAPPED":
			mappedSet = true
			if mappedTotal == "" {
				mappedTotal = amount
			} else {
				mappedTotal = addSignedDecimalStrings(mappedTotal, amount)
			}
		default:
			unmappedSet = true
			if unmappedTotal == "" {
				unmappedTotal = amount
			} else {
				unmappedTotal = addSignedDecimalStrings(unmappedTotal, amount)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if out.MappedAmount.State == "" {
		if currency == "" {
			currency = "RUB"
		}
		if mappedSet {
			out.MappedAmount = factualapi.Known(factualapi.SignedMoney{Amount: mappedTotal, CurrencyCode: currency})
		} else {
			out.MappedAmount = factualapi.Known(factualapi.SignedMoney{Amount: "0", CurrencyCode: currency})
		}
		if unmappedSet {
			out.UnmappedAmount = factualapi.Known(factualapi.SignedMoney{Amount: unmappedTotal, CurrencyCode: currency})
		} else {
			out.UnmappedAmount = factualapi.Known(factualapi.SignedMoney{Amount: "0", CurrencyCode: currency})
		}
		out.MappingCoverage = mappingCoverageAmount(out.MappedAmount, out.UnmappedAmount)
	}
	out.Meta, err = s.meta(c, "FINANCE.ACCRUAL_BREAKDOWN", "FINANCE_ACCOUNT_BREAKDOWN", c.SellerAccountID(), &period, factHash(out))
	return out, err
}

func addSignedDecimalStrings(a, b string) string {
	ra, _ := new(big.Rat).SetString(a)
	rb, _ := new(big.Rat).SetString(b)
	if ra == nil {
		ra = new(big.Rat)
	}
	if rb == nil {
		rb = new(big.Rat)
	}
	ra.Add(ra, rb)
	return ra.FloatString(6)
}

func mappingCoverageAmount(mapped, unmapped factualapi.Value[factualapi.SignedMoney]) factualapi.Value[factualapi.Decimal] {
	if mapped.State != factualapi.ValueKnown || mapped.Value == nil || unmapped.State != factualapi.ValueKnown || unmapped.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MAPPING_COVERAGE_UNAVAILABLE")
	}
	if mapped.Value.CurrencyCode != unmapped.Value.CurrencyCode {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "CURRENCY_NOT_COMPARABLE")
	}
	num, _ := new(big.Rat).SetString(mapped.Value.Amount)
	den, _ := new(big.Rat).SetString(unmapped.Value.Amount)
	if num == nil {
		num = new(big.Rat)
	}
	if den == nil {
		den = new(big.Rat)
	}
	numAbs := new(big.Rat).Abs(num)
	denAbs := new(big.Rat).Abs(den)
	total := new(big.Rat).Add(numAbs, denAbs)
	if total.Sign() == 0 {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DENOMINATOR")
	}
	numAbs.Quo(numAbs, total)
	numAbs.Mul(numAbs, big.NewRat(100, 1))
	d, err := factualapi.ParseDecimal(numAbs.FloatString(6))
	if err != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "MAPPING_COVERAGE_ARITHMETIC_FAILED")
	}
	return factualapi.Known(d)
}

func (s *Service) ListFinanceAccruals(ctx context.Context, c factualapi.FactualReadContext, filter factualapi.FinanceAccrualFilter, page factualapi.PageRequest) (factualapi.Page[factualapi.FinanceAccrual], error) {
	var out factualapi.Page[factualapi.FinanceAccrual]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListFinanceAccruals", filter, page)
	if err != nil {
		return out, err
	}
	var from, to any
	if filter.Period != nil {
		from, to = filter.Period.From, filter.Period.To
	}
	var typeIDs any = filter.ProviderTypeIDs
	if len(filter.ProviderTypeIDs) == 0 {
		typeIDs = nil
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)id,business_date FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND observed_at<=$2 AND($3::date IS NULL OR business_date BETWEEN $3 AND $4)ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)
		SELECT r.id,r.day_revision_id,l.business_date,r.row_ordinal,r.provider_type_id,COALESCE(r.provider_accrued_category,''),COALESCE(r.unit_number,''),r.total_amount_signed::text,r.currency_code,COALESCE(v.mapping_status,'UNMAPPED')
		FROM latest l JOIN finance_accrual_records r ON r.day_revision_id=l.id
		LEFT JOIN finance_provider_type_versions v ON v.finance_provider_type_id=r.finance_provider_type_id AND v.valid_from<=$2 AND(v.valid_to IS NULL OR v.valid_to>$2)
		WHERE r.id>$5 AND($6::text[] IS NULL OR r.provider_type_id=ANY($6))
		ORDER BY r.id LIMIT $7`, c.SellerAccountID(), c.CutoffAt(), from, to, last, typeIDs, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.FinanceAccrual
		var amount, currency string
		if err = rows.Scan(&x.ID, &x.DayRevisionID, &x.BusinessDate, &x.RowOrdinal, &x.ProviderTypeID, &x.ProviderAccruedCategory, &x.UnitNumber, &amount, &currency, &x.MappingStatus); err != nil {
			return out, err
		}
		x.Total = factualapi.SignedMoney{Amount: amount, CurrencyCode: currency}
		x.Meta, _ = s.meta(c, "FINANCE.ACCRUAL_BREAKDOWN", "FINANCE_ACCRUAL", x.ID, filter.Period, factHash(x))
		out.Items = append(out.Items, x)
	}
	hasMore := len(out.Items) > n
	if hasMore {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListFinanceAccruals", hash, out.Items[len(out.Items)-1].ID, hasMore)
	}
	return out, err
}

func (s *Service) GetFinanceBalance(ctx context.Context, c factualapi.FactualReadContext) (factualapi.FinanceBalance, error) {
	var out factualapi.FinanceBalance
	var amount, currency string
	err := s.pool.QueryRow(ctx, `SELECT balance_amount_signed::text,currency_code,observed_at FROM finance_balance_snapshots WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), c.CutoffAt()).Scan(&amount, &currency, &out.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		out.Balance = factualapi.Missing[factualapi.SignedMoney](factualapi.ValueUnavailable, "NO_FINANCE_BALANCE_SNAPSHOT")
		out.Meta, err = s.meta(c, "FINANCE.BALANCE", "FINANCE_BALANCE", c.SellerAccountID(), nil, factHash(out))
		return out, err
	}
	if err != nil {
		return out, dbError(err)
	}
	out.Balance = factualapi.Known(factualapi.SignedMoney{Amount: amount, CurrencyCode: currency})
	out.Meta, err = s.meta(c, "FINANCE.BALANCE", "FINANCE_BALANCE", c.SellerAccountID(), nil, factHash(out))
	return out, err
}
