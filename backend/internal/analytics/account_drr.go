package analytics

import "time"

// AccountDRRPct is the share of advertising spend in total store (cabinet) revenue:
//
//	total ad spend (all campaigns) / total_store_revenue * 100
//
// Used by Seller Pulse («ДРР в общей выручке»).
// Returns nil when accountRevenue <= 0.
//
// Rules:
//   - Numerator includes every campaign (effective and not, with and without orders).
//   - Denominator is cabinet revenue from daily_account_metrics — never attributed ad revenue.
func AccountDRRPct(adSpend, accountRevenue float64) *float64 {
	return CampaignDRRPct(adSpend, accountRevenue)
}

// AdSpendShareOfAttributedRevenuePct is the share of advertising spend in
// campaign-attributed revenue:
//
//	total ad spend (all campaigns) / attributed_ad_revenue * 100
//
// Used by the Advertising efficiency block.
// Returns nil when attributedRevenue <= 0.
func AdSpendShareOfAttributedRevenuePct(adSpend, attributedRevenue float64) *float64 {
	return CampaignDRRPct(adSpend, attributedRevenue)
}

// AccountDRRRatio is the 0–1 form of AccountDRRPct (spend / cabinet revenue).
func AccountDRRRatio(adSpend, accountRevenue float64) *float64 {
	return RatioOrNil(adSpend, accountRevenue)
}

// AccountDRRDayPoint is one calendar day of a spend/revenue share series.
type AccountDRRDayPoint struct {
	Date    string
	Spend   float64
	Revenue float64
	DRRPct  *float64
	HasData bool
}

// BuildAccountDRRDailySeries computes per-day spend/revenue shares for [from..to].
// spendByDate must already sum all campaigns; revenueByDate is the chosen denominator
// (store revenue for Pulse, attributed ad revenue for the Advertising block).
func BuildAccountDRRDailySeries(
	from, to time.Time,
	spendByDate map[string]float64,
	revenueByDate map[string]float64,
) []AccountDRRDayPoint {
	out := make([]AccountDRRDayPoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := AccountDRRDayPoint{Date: key}
		spend, hasSpend := spendByDate[key]
		rev, hasRev := revenueByDate[key]
		if hasSpend {
			pt.Spend = spend
		}
		if hasRev {
			pt.Revenue = rev
		}
		if hasSpend && hasRev {
			pt.HasData = true
			if rev > 0 {
				pt.DRRPct = AccountDRRPct(spend, rev)
			}
		}
		out = append(out, pt)
	}
	return out
}

// AccountDRRPeriodPct is spend_sum / revenue_sum * 100 over a window.
// Only days present in both maps with revenue > 0 contribute to both sums.
func AccountDRRPeriodPct(spendByDate, revenueByDate map[string]float64, from, to time.Time) *float64 {
	spendSum, revSum := SumWindowTotals(spendByDate, revenueByDate, from, to)
	return AccountDRRPct(spendSum, revSum)
}

// SumWindowTotals returns (spendSum, revenueSum) for days present in both maps with revenue > 0.
func SumWindowTotals(spendByDate, revenueByDate map[string]float64, from, to time.Time) (spendSum, revSum float64) {
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		rev, hasRev := revenueByDate[key]
		spend, hasSpend := spendByDate[key]
		if !hasRev || !hasSpend || rev <= 0 {
			continue
		}
		spendSum += spend
		revSum += rev
	}
	return spendSum, revSum
}
