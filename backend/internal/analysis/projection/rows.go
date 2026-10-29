package projection

import (
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

const (
	bandOutOfStock    = "out_of_stock"
	bandCoverage0To3  = "coverage_0_3"
	bandCoverage4To7  = "coverage_4_7"
	bandCoverage8To14 = "coverage_8_14"
)

func stockRowFromSKU(it factualapi.SKUStock, cov *factualapi.StockCoverage, dem *factualapi.SKUDemand, displayOrder int64) map[string]any {
	sku := strconv.FormatInt(it.CanonicalSKUID, 10)
	row := map[string]any{
		"sku_id":                          sku,
		"display_order":                   displayOrder,
		"known_subtotal_units":            it.KnownSubtotalUnits,
		"known_subtotal_is_authoritative": false,
		"sellable_units_state":            string(it.SellableUnits.State),
	}
	if it.SellableUnits.ReasonCode != "" {
		row["sellable_units_reason"] = it.SellableUnits.ReasonCode
	}
	if it.SellableUnits.State == factualapi.ValueKnown && it.SellableUnits.Value != nil {
		row["sellable_units"] = *it.SellableUnits.Value
	}
	band, covState, covReason, days := classifyCoverageBand(it.SellableUnits, cov)
	row["coverage_state"] = covState
	if covReason != "" {
		row["coverage_reason"] = covReason
	}
	if days != nil {
		row["coverage_days"] = *days
	}
	if band != nil {
		row["coverage_band"] = *band
	}
	if dem != nil {
		row["ordered_units_7d_state"] = string(dem.Demand.OrderedUnits.State)
		if dem.Demand.OrderedUnits.ReasonCode != "" {
			row["ordered_units_7d_reason"] = dem.Demand.OrderedUnits.ReasonCode
		}
		if dem.Demand.OrderedUnits.State == factualapi.ValueKnown && dem.Demand.OrderedUnits.Value != nil {
			row["ordered_units_7d"] = *dem.Demand.OrderedUnits.Value
			row["no_sales"] = *dem.Demand.OrderedUnits.Value == 0
		}
	}
	return row
}

func classifyCoverageBand(sellable factualapi.Value[int64], cov *factualapi.StockCoverage) (band *string, covState, covReason string, days *string) {
	if sellable.State == factualapi.ValueKnown && sellable.Value != nil && *sellable.Value == 0 {
		b := bandOutOfStock
		return &b, string(factualapi.ValueKnown), "", nil
	}
	if cov == nil {
		return nil, string(factualapi.ValueUnknown), "COVERAGE_UNAVAILABLE", nil
	}
	cv := cov.CoverageDays7D
	covState = string(cv.State)
	covReason = cv.ReasonCode
	if cv.State != factualapi.ValueKnown || cv.Value == nil {
		return nil, covState, covReason, nil
	}
	s := cv.Value.String()
	days = &s
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, string(factualapi.ValueUnknown), "COVERAGE_PARSE", days
	}
	var b string
	switch {
	case n <= 3:
		b = bandCoverage0To3
	case n <= 7:
		b = bandCoverage4To7
	case n <= 14:
		b = bandCoverage8To14
	default:
		return nil, covState, covReason, days
	}
	return &b, covState, covReason, days
}

func campaignRowFrom(it factualapi.Campaign, perf *factualapi.CampaignPeriodPerformance) map[string]any {
	row := map[string]any{
		"campaign_id":    strconv.FormatInt(it.ID, 10),
		"title":          it.Title,
		"provider_state": it.ProviderState,
		"budget_state":   string(it.Budget.State),
	}
	if it.Budget.ReasonCode != "" {
		row["budget_reason"] = it.Budget.ReasonCode
	}
	if it.Budget.State == factualapi.ValueKnown && it.Budget.Value != nil {
		row["budget_amount"] = it.Budget.Value.Amount.String()
		row["budget_currency"] = it.Budget.Value.CurrencyCode
	}
	if perf == nil {
		row["paid_spend_state"] = string(factualapi.ValueUnknown)
		row["attributed_order_amount_state"] = string(factualapi.ValueUnknown)
		row["campaign_drr_state"] = string(factualapi.ValueUnknown)
		return row
	}
	putMoney(row, "paid_spend", perf.PaidSpend)
	putMoney(row, "attributed_order_amount", perf.AttributedOrderAmount)
	putInt(row, "attributed_orders", perf.AttributedOrderUnits)
	putDec(row, "campaign_drr", perf.CalculatedDRRPct)
	row["review_cohort"] = campaignReviewCohort(row)
	return row
}

func campaignReviewCohort(row map[string]any) string {
	spendState, _ := row["paid_spend_state"].(string)
	if spendState == string(factualapi.ValueKnown) {
		if s, ok := row["paid_spend_amount"].(string); ok && (s == "0" || s == "0.0") {
			return "ZERO_SPEND"
		}
		drrState, _ := row["campaign_drr_state"].(string)
		if drrState != string(factualapi.ValueKnown) {
			return "UNKNOWN_EFFICIENCY"
		}
	}
	return ""
}

func putMoney(row map[string]any, prefix string, v factualapi.Value[factualapi.Money]) {
	row[prefix+"_state"] = string(v.State)
	if v.ReasonCode != "" {
		row[prefix+"_reason"] = v.ReasonCode
	}
	if v.State == factualapi.ValueKnown && v.Value != nil {
		row[prefix+"_amount"] = v.Value.Amount.String()
		row[prefix+"_currency"] = v.Value.CurrencyCode
	}
}

func putInt(row map[string]any, prefix string, v factualapi.Value[int64]) {
	row[prefix+"_state"] = string(v.State)
	if v.ReasonCode != "" {
		row[prefix+"_reason"] = v.ReasonCode
	}
	if v.State == factualapi.ValueKnown && v.Value != nil {
		row[prefix] = *v.Value
	}
}

func putDec(row map[string]any, prefix string, v factualapi.Value[factualapi.Decimal]) {
	row[prefix+"_state"] = string(v.State)
	if v.ReasonCode != "" {
		row[prefix+"_reason"] = v.ReasonCode
	}
	if v.State == factualapi.ValueKnown && v.Value != nil {
		row[prefix] = v.Value.String()
	}
}

func coverageBandCounts(rows []map[string]any) []CoverageBandCount {
	keys := []string{bandOutOfStock, bandCoverage0To3, bandCoverage4To7, bandCoverage8To14}
	n := map[string]int{}
	for _, r := range rows {
		if b, ok := r["coverage_band"].(string); ok {
			n[b]++
		}
	}
	out := make([]CoverageBandCount, 0, len(keys))
	for _, k := range keys {
		out = append(out, CoverageBandCount{Key: k, Count: n[k], State: string(factualapi.ValueKnown)})
	}
	return out
}
