package landscape

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"

func indexSKUDemandBatch(items []factualapi.BatchSKUDemandItem) map[int64]factualapi.SKUDemand {
	out := map[int64]factualapi.SKUDemand{}
	for _, it := range items {
		if it.Demand == nil {
			continue
		}
		if it.Status == factualapi.BatchFound || it.Status == factualapi.BatchLimited {
			out[it.ID] = *it.Demand
		}
	}
	return out
}

func degradeSalesBlock(state *BlockState) {
	if state == nil {
		return
	}
	if *state == BlockReady || *state == BlockReadyEmpty || *state == "" {
		*state = BlockPartial
	}
}

func appendUniqueLimitation(lims *[]Limitation, lim Limitation) {
	if lims == nil || lim.Code == "" {
		return
	}
	for _, existing := range *lims {
		if existing.Code == lim.Code && existing.Message == lim.Message {
			return
		}
	}
	*lims = append(*lims, lim)
}

func applySalesDemandMeta(state *BlockState, lims *[]Limitation, d factualapi.Demand, periodCode string) {
	unknown := d.OrderedAmount.State != factualapi.ValueKnown || d.OrderedUnits.State != factualapi.ValueKnown
	if unknown {
		degradeSalesBlock(state)
		reason := d.OrderedAmount.ReasonCode
		if reason == "" {
			reason = d.OrderedUnits.ReasonCode
		}
		if reason == "" {
			reason = "SALES_PERIOD_UNKNOWN"
		}
		appendUniqueLimitation(lims, Limitation{Code: periodCode, Message: reason})
	}
	switch d.Meta.Readiness {
	case "PARTIAL", "FAILED":
		degradeSalesBlock(state)
		if len(d.Meta.Limitations) == 0 {
			appendUniqueLimitation(lims, Limitation{Code: periodCode, Message: d.Meta.Readiness})
			return
		}
		for _, l := range d.Meta.Limitations {
			code := l.Code
			if code == "" {
				code = periodCode
			}
			appendUniqueLimitation(lims, Limitation{Code: code, Message: l.Message})
		}
	}
}

func skuSalesBlockFromDemand(cur, prev, trend *factualapi.SKUDemand, curOK, prevOK, trendOK bool) *SKUSalesBlock {
	if !curOK && !prevOK && !trendOK {
		return &SKUSalesBlock{State: BlockReadyEmpty}
	}
	block := &SKUSalesBlock{State: BlockReady}
	var curAmt, prevAmt *MoneyAmount
	var curU, prevU *int64
	if curOK && cur != nil {
		curAmt = moneyFromValue(cur.Demand.OrderedAmount)
		curU = intFromValue(cur.Demand.OrderedUnits)
		applySalesDemandMeta(&block.State, &block.Limitations, cur.Demand, "SALES_CURRENT_7D")
	}
	if prevOK && prev != nil {
		prevAmt = moneyFromValue(prev.Demand.OrderedAmount)
		prevU = intFromValue(prev.Demand.OrderedUnits)
		applySalesDemandMeta(&block.State, &block.Limitations, prev.Demand, "SALES_PREVIOUS_7D")
	}
	block.OrderedAmount = BuildMoneyComparison(curAmt, prevAmt)
	block.OrderedUnits = BuildCountComparison(curU, prevU)
	if trendOK && trend != nil {
		block.Trend30DAmount = moneyFromValue(trend.Demand.OrderedAmount)
		block.Trend30DUnits = intFromValue(trend.Demand.OrderedUnits)
		applySalesDemandMeta(&block.State, &block.Limitations, trend.Demand, "SALES_TREND_30D")
	}
	return block
}

func accountSalesFromDemand(cur, prev, day, prevDay, trend *factualapi.Demand, prevErr, dayErr, prevDayErr, trendErr error, includeTrend bool) *AccountSalesBlock {
	block := &AccountSalesBlock{State: BlockReady}
	var curAmt, prevAmt *MoneyAmount
	var curU, prevU *int64
	if cur != nil {
		curAmt = moneyFromValue(cur.OrderedAmount)
		curU = intFromValue(cur.OrderedUnits)
		applySalesDemandMeta(&block.State, &block.Limitations, *cur, "SALES_CURRENT_7D")
	}
	if prevErr != nil {
		degradeSalesBlock(&block.State)
		appendUniqueLimitation(&block.Limitations, Limitation{Code: "SALES_PREV_UNAVAILABLE", Message: prevErr.Error()})
	} else if prev != nil {
		prevAmt = moneyFromValue(prev.OrderedAmount)
		prevU = intFromValue(prev.OrderedUnits)
		applySalesDemandMeta(&block.State, &block.Limitations, *prev, "SALES_PREVIOUS_7D")
	}
	block.OrderedAmount = BuildMoneyComparison(curAmt, prevAmt)
	block.OrderedUnits = BuildCountComparison(curU, prevU)

	if dayErr != nil {
		degradeSalesBlock(&block.State)
		appendUniqueLimitation(&block.Limitations, Limitation{Code: "SALES_ANALYSIS_DAY_UNAVAILABLE", Message: dayErr.Error()})
	} else if day != nil {
		applySalesDemandMeta(&block.State, &block.Limitations, *day, "SALES_ANALYSIS_DAY")
		var prevDayAmt *MoneyAmount
		var prevDayUnits *int64
		if prevDayErr != nil {
			degradeSalesBlock(&block.State)
			appendUniqueLimitation(&block.Limitations, Limitation{Code: "SALES_PREVIOUS_DAY_UNAVAILABLE", Message: prevDayErr.Error()})
		} else if prevDay != nil {
			prevDayAmt = moneyFromValue(prevDay.OrderedAmount)
			prevDayUnits = intFromValue(prevDay.OrderedUnits)
			applySalesDemandMeta(&block.State, &block.Limitations, *prevDay, "SALES_PREVIOUS_DAY")
		}
		dayAmt := moneyFromValue(day.OrderedAmount)
		dayUnits := intFromValue(day.OrderedUnits)
		if dayAmt != nil || prevDayAmt != nil {
			cmpAmt := BuildMoneyComparison(dayAmt, prevDayAmt)
			cmpUnits := BuildCountComparison(dayUnits, prevDayUnits)
			block.AnalysisDayAmount = &cmpAmt
			block.AnalysisDayUnits = &cmpUnits
		}
	}

	if includeTrend {
		if trendErr != nil {
			degradeSalesBlock(&block.State)
			appendUniqueLimitation(&block.Limitations, Limitation{Code: "SALES_30D_UNAVAILABLE", Message: trendErr.Error()})
		} else if trend != nil {
			block.Trend30DAmount = moneyFromValue(trend.OrderedAmount)
			block.Trend30DUnits = intFromValue(trend.OrderedUnits)
			applySalesDemandMeta(&block.State, &block.Limitations, *trend, "SALES_TREND_30D")
		}
	}
	return block
}
