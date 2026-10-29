package outputvalidator

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// AccountMetricFact is one Pack-derived numeric fact used to ground AI claims.
type AccountMetricFact struct {
	FactID     string
	MetricCode string
	Value      float64
	PeriodType string
}

// LookupAccountMetric finds a fact by canonical fact_id or metric_code+period_type.
func (idx *PackReferenceIndex) LookupAccountMetric(factID, metricCode, periodType string) (AccountMetricFact, bool) {
	if idx == nil || idx.accountMetrics == nil {
		return AccountMetricFact{}, false
	}
	factID = strings.TrimSpace(factID)
	if factID != "" {
		if f, ok := idx.accountMetrics[factID]; ok {
			return f, true
		}
	}
	metricCode = strings.TrimSpace(metricCode)
	periodType = strings.TrimSpace(periodType)
	if metricCode != "" && periodType != "" {
		key := "account:" + metricCode + ":" + periodType
		if f, ok := idx.accountMetrics[key]; ok {
			return f, true
		}
	}
	return AccountMetricFact{}, false
}

// SellerPulseGroundingLines returns canonical account_summary facts the model
// must copy into seller_pulse metric_claims (excluded from prompt hash).
func (idx *PackReferenceIndex) SellerPulseGroundingLines() []string {
	if idx == nil || len(idx.accountMetrics) == 0 {
		return nil
	}
	keys := []string{
		"account:revenue_7d:current_7d",
		"account:revenue_7d:previous_7d",
		"account:orders_7d:current_7d",
		"account:orders_7d:previous_7d",
		"account:revenue_delta_pct:current_7d",
		"account:orders_delta_pct:current_7d",
		"account:cart_to_order_pct:current_7d",
		"account:ad_spend_7d:current_7d",
		"account:drr:current_7d",
		"account:ad_spend_share_of_total_revenue_pct:current_7d",
		"account:account_drr_pct:current_7d",
		"account:no_sales_with_stock_count:current_7d",
		"account:deficit_sku_count:current_7d",
		"account:stockout_sku_count:current_7d",
		// Canonical ad_efficiency.narrative.metric_claims codes (Wave 2).
		"account:ad_spend:current_7d",
		"account:store_total_revenue:current_7d",
		"account:ad_spend_share_of_total_store_revenue_pct:current_7d",
		"account:attributed_ad_revenue:current_7d",
		"account:attributed_campaign_drr_pct:current_7d",
		"account:ad_orders:current_7d",
	}
	var lines []string
	for _, k := range keys {
		f, ok := idx.accountMetrics[k]
		if !ok {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%g", k, f.Value))
	}
	return lines
}

func indexAccountSummaryFromCSV(idx *PackReferenceIndex, raw []byte) {
	if idx == nil || len(raw) == 0 {
		return
	}
	if idx.accountMetrics == nil {
		idx.accountMetrics = make(map[string]AccountMetricFact)
	}
	r := csv.NewReader(bytes.NewReader(raw))
	header, err := r.Read()
	if err != nil {
		return
	}
	// Copy header names before the next Read: csv.Reader may reuse the
	// record buffer, which would otherwise replace header cells with row values
	// and leave accountMetrics empty (claim grounding silently no-ops).
	headerCopy := append([]string(nil), header...)
	row, err := r.Read()
	if err != nil {
		return
	}
	col := map[string]int{}
	for i, h := range headerCopy {
		col[strings.TrimSpace(h)] = i
	}
	get := func(name string) (float64, bool) {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return 0, false
		}
		v := strings.TrimSpace(row[i])
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	put := func(metricCode, periodType string, value float64) {
		id := "account:" + metricCode + ":" + periodType
		idx.accountMetrics[id] = AccountMetricFact{
			FactID:     id,
			MetricCode: metricCode,
			Value:      value,
			PeriodType: periodType,
		}
	}
	// Index every numeric account_summary column as current_7d so AI claims
	// that cite Pack fields (cart_to_order_pct, no_sales_with_stock_count, …)
	// can be grounded without a brittle allow-list.
	skipCols := map[string]struct{}{
		"row_id": {}, "seller_account_id": {}, "metric_date": {},
	}
	for name, i := range col {
		if _, skip := skipCols[name]; skip {
			continue
		}
		if i >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[i])
		if v == "" {
			continue
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil {
			continue
		}
		period := "current_7d"
		if strings.Contains(name, "previous_7d") || strings.HasSuffix(name, "_previous") {
			period = "previous_7d"
		}
		put(name, period, f)
	}
	// Canonical aliases used by seller_pulse metric_claims.
	if v, ok := get("revenue_7d"); ok {
		put("revenue_7d", "current_7d", v)
	}
	if v, ok := get("revenue_previous_7d"); ok {
		put("revenue_7d", "previous_7d", v)
		put("revenue_previous_7d", "previous_7d", v)
	}
	if v, ok := get("orders_7d"); ok {
		put("orders_7d", "current_7d", v)
	}
	if v, ok := get("orders_previous_7d"); ok {
		put("orders_7d", "previous_7d", v)
		put("orders_previous_7d", "previous_7d", v)
	}
	// Store-level ad load = ad_spend / store revenue (not attributed campaign DRR).
	if spend, okSpend := get("ad_spend_7d"); okSpend {
		if rev, okRev := get("revenue_7d"); okRev && rev != 0 {
			share := spend / rev * 100
			put("ad_spend_share_of_total_revenue_pct", "current_7d", share)
			put("drr", "current_7d", share)
		}
	}
	// Canonical ad_efficiency.narrative.metric_claims codes (claim-based
	// grounding, Wave 2): raw values + the two percentage shares, keyed by
	// the exact metric_code the AI must copy into its claims. These are
	// derived facts (not extra CSV columns), so they are put()-ed directly
	// rather than via the generic per-column indexing loop above.
	if spend, okSpend := get("ad_spend_7d"); okSpend {
		put("ad_spend", "current_7d", spend)
		if rev, okRev := get("revenue_7d"); okRev {
			put("store_total_revenue", "current_7d", rev)
			if rev != 0 {
				put("ad_spend_share_of_total_store_revenue_pct", "current_7d", spend/rev*100)
			}
		}
		if adRev, okAdRev := get("ad_revenue_7d"); okAdRev {
			put("attributed_ad_revenue", "current_7d", adRev)
			if adRev != 0 {
				put("attributed_campaign_drr_pct", "current_7d", spend/adRev*100)
			}
		}
	}
	if adOrders, ok := get("ad_orders_7d"); ok {
		put("ad_orders", "current_7d", adOrders)
	}
}
