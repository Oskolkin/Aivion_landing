package landscape

import (
	"context"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func (s FactualAPISource) batchCampaignTraffic(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.CampaignTrafficPerformance, error) {
	out := map[int64]factualapi.CampaignTrafficPerformance{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetCampaignTrafficPerformance(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Traffic != nil {
				out[it.ID] = *it.Traffic
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchCampaignAttribution(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.CampaignAttributionPerformance, error) {
	out := map[int64]factualapi.CampaignAttributionPerformance{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetCampaignAttributionPerformance(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Attribution != nil {
				out[it.ID] = *it.Attribution
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUTraffic(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.SKUAdvertisingTraffic, error) {
	out := map[int64]factualapi.SKUAdvertisingTraffic{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUAdvertisingTraffic(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Traffic != nil {
				out[it.ID] = *it.Traffic
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUAttribution(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.SKUAdvertisingAttribution, error) {
	out := map[int64]factualapi.SKUAdvertisingAttribution{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUAdvertisingAttribution(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Attribution != nil {
				out[it.ID] = *it.Attribution
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchCampaignTargetingSummary(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.CampaignTargetingSummary, error) {
	out := map[int64]factualapi.CampaignTargetingSummary{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetCampaignTargetingSummary(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Summary != nil {
				out[it.ID] = *it.Summary
			}
		}
	}
	return out, nil
}

func buildSKUAdsBlockV4(
	id int64,
	trafficCur, trafficPrev map[int64]factualapi.SKUAdvertisingTraffic,
	trafficCurErr, trafficPrevErr error,
	trafficExecute bool,
	attrCur, attrPrev map[int64]factualapi.SKUAdvertisingAttribution,
	attrCurErr, attrPrevErr error,
	attrExecute bool,
) *SKUAdsBlock {
	if !trafficExecute && !attrExecute {
		return nil
	}
	block := &SKUAdsBlock{State: BlockReady}
	if trafficExecute {
		block.Traffic = buildSKUAdsTrafficBlock(id, trafficCur, trafficPrev, trafficCurErr, trafficPrevErr)
	}
	if attrExecute {
		block.Attribution = buildSKUAdsAttributionBlock(id, attrCur, attrPrev, attrCurErr, attrPrevErr)
	}
	if block.Traffic == nil && block.Attribution == nil {
		return nil
	}
	if block.Traffic != nil && block.Attribution != nil {
		if dateRangesEqual(block.Traffic.CurrentPeriod, block.Attribution.CurrentPeriod) &&
			dateRangesEqual(block.Traffic.PreviousPeriod, block.Attribution.PreviousPeriod) {
			if drr := calculatedDRRComparison(
				block.Traffic.PaidSpend.Current, block.Attribution.AttributedOrderAmount.Current,
				block.Traffic.PaidSpend.Previous, block.Attribution.AttributedOrderAmount.Previous,
			); drr != nil {
				block.Attribution.CalculatedDRR = drr
			}
		} else {
			block.Attribution.CalculatedDRR = nil
			block.Attribution.Limitations = append(block.Attribution.Limitations, Limitation{
				Code: "DRR_PERIOD_MISMATCH", Message: "traffic and attribution periods are not comparable",
			})
			if block.Attribution.State == BlockReady {
				block.Attribution.State = BlockPartial
			}
		}
	}
	block.State = aggregateCampaignPerfState(
		(*CampaignTrafficBlock)(nil),
		(*CampaignAttributionBlock)(nil),
	)
	states := []BlockState{}
	if block.Traffic != nil {
		states = append(states, block.Traffic.State)
	}
	if block.Attribution != nil {
		states = append(states, block.Attribution.State)
	}
	if len(states) > 0 {
		worst := states[0]
		for _, st := range states[1:] {
			if blockStateRank(st) > blockStateRank(worst) {
				worst = st
			}
		}
		block.State = worst
	}
	return block
}

func buildSKUAdsTrafficBlock(
	id int64,
	cur, prev map[int64]factualapi.SKUAdvertisingTraffic,
	curErr, prevErr error,
) *SKUAdsTrafficBlock {
	if curErr != nil && prevErr != nil {
		return &SKUAdsTrafficBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_AD_TRAFFIC_UNAVAILABLE", curErr.Error())}
	}
	var curA, prevA *factualapi.SKUAdvertisingTraffic
	if a, ok := cur[id]; ok {
		cp := a
		curA = &cp
	}
	if a, ok := prev[id]; ok {
		cp := a
		prevA = &cp
	}
	if curA == nil && prevA == nil {
		block := &SKUAdsTrafficBlock{State: BlockReadyEmpty}
		if curErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_TRAFFIC_CUR_UNAVAILABLE", Message: curErr.Error()})
		}
		return block
	}
	block := &SKUAdsTrafficBlock{State: BlockReady}
	if curA != nil {
		block.CurrentPeriod = periodToDateRange(curA.Period)
	}
	if prevA != nil {
		block.PreviousPeriod = periodToDateRange(prevA.Period)
	}
	var curSpend, prevSpend, curBonus, prevBonus *MoneyAmount
	var curImp, prevImp, curClk, prevClk *int64
	if curA != nil {
		curSpend = moneyFromValue(curA.PaidSpend)
		curBonus = moneyFromValue(curA.BonusSpend)
		curImp = intFromValue(curA.Impressions)
		curClk = intFromValue(curA.Clicks)
	}
	if prevA != nil {
		prevSpend = moneyFromValue(prevA.PaidSpend)
		prevBonus = moneyFromValue(prevA.BonusSpend)
		prevImp = intFromValue(prevA.Impressions)
		prevClk = intFromValue(prevA.Clicks)
	}
	block.PaidSpend = BuildMoneyComparison(curSpend, prevSpend)
	block.Impressions = BuildCountComparison(curImp, prevImp)
	block.Clicks = BuildCountComparison(curClk, prevClk)
	if curBonus != nil || prevBonus != nil {
		b := BuildMoneyComparison(curBonus, prevBonus)
		block.BonusSpend = &b
	}
	if ctr := ctrFromCounts(curClk, curImp); ctr != nil || ctrFromCounts(prevClk, prevImp) != nil {
		cmp := BuildPctPointComparison(ctrFromCounts(curClk, curImp), ctrFromCounts(prevClk, prevImp))
		block.CTR = &cmp
	}
	if cpc := cpcFromSpendClicks(curSpend, curClk); cpc != nil || cpcFromSpendClicks(prevSpend, prevClk) != nil {
		cmp := BuildMoneyComparison(cpcFromSpendClicks(curSpend, curClk), cpcFromSpendClicks(prevSpend, prevClk))
		block.CPC = &cmp
	}
	if cpm := cpmFromSpendImpressions(curSpend, curImp); cpm != nil || cpmFromSpendImpressions(prevSpend, prevImp) != nil {
		cmp := BuildMoneyComparison(cpmFromSpendImpressions(curSpend, curImp), cpmFromSpendImpressions(prevSpend, prevImp))
		block.CPM = &cmp
	}
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_TRAFFIC_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_TRAFFIC_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func buildSKUAdsAttributionBlock(
	id int64,
	cur, prev map[int64]factualapi.SKUAdvertisingAttribution,
	curErr, prevErr error,
) *SKUAdsAttributionBlock {
	if curErr != nil && prevErr != nil {
		return &SKUAdsAttributionBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_AD_ATTRIBUTION_UNAVAILABLE", curErr.Error())}
	}
	var curA, prevA *factualapi.SKUAdvertisingAttribution
	if a, ok := cur[id]; ok {
		cp := a
		curA = &cp
	}
	if a, ok := prev[id]; ok {
		cp := a
		prevA = &cp
	}
	if curA == nil && prevA == nil {
		block := &SKUAdsAttributionBlock{State: BlockReadyEmpty}
		if curErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_ATTRIBUTION_CUR_UNAVAILABLE", Message: curErr.Error()})
		}
		return block
	}
	block := &SKUAdsAttributionBlock{State: BlockReady}
	if curA != nil {
		block.CurrentPeriod = periodToDateRange(curA.Period)
	}
	if prevA != nil {
		block.PreviousPeriod = periodToDateRange(prevA.Period)
	}
	var curAmt, prevAmt *MoneyAmount
	var curU, prevU *int64
	if curA != nil {
		curAmt = moneyFromValue(curA.AttributedOrderAmount)
		curU = intFromValue(curA.AttributedOrderUnits)
	}
	if prevA != nil {
		prevAmt = moneyFromValue(prevA.AttributedOrderAmount)
		prevU = intFromValue(prevA.AttributedOrderUnits)
	}
	block.AttributedOrderAmount = BuildMoneyComparison(curAmt, prevAmt)
	block.AttributedOrderUnits = BuildCountComparison(curU, prevU)
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_ATTRIBUTION_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_AD_ATTRIBUTION_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func buildCampaignPerfBlockV4(
	id int64,
	trafficCur, trafficPrev map[int64]factualapi.CampaignTrafficPerformance,
	trafficCurErr, trafficPrevErr error,
	trafficPresent bool, trafficDecision BlockExecutionDecision,
	attrCur, attrPrev map[int64]factualapi.CampaignAttributionPerformance,
	attrCurErr, attrPrevErr error,
	attrPresent bool, attrDecision BlockExecutionDecision,
) *CampaignPerfBlock {
	if !trafficPresent && !attrPresent {
		return nil
	}
	block := &CampaignPerfBlock{}
	if trafficPresent && trafficDecision.Execute {
		block.Traffic = buildCampaignTrafficBlockV4(id, trafficCur, trafficPrev, trafficCurErr, trafficPrevErr)
	}
	if attrPresent && attrDecision.Execute {
		block.Attribution = buildCampaignAttributionBlockV4(id, attrCur, attrPrev, attrCurErr, attrPrevErr)
	}
	if block.Traffic == nil && block.Attribution == nil {
		return nil
	}
	if block.Traffic != nil && block.Attribution != nil {
		if dateRangesEqual(block.Traffic.CurrentPeriod, block.Attribution.CurrentPeriod) &&
			dateRangesEqual(block.Traffic.PreviousPeriod, block.Attribution.PreviousPeriod) {
			if drr := calculatedDRRComparison(
				block.Traffic.PaidSpend.Current, block.Attribution.AttributedOrderAmount.Current,
				block.Traffic.PaidSpend.Previous, block.Attribution.AttributedOrderAmount.Previous,
			); drr != nil {
				block.Attribution.CalculatedDRR = drr
			}
		} else {
			block.Attribution.CalculatedDRR = nil
			block.Attribution.Limitations = append(block.Attribution.Limitations, Limitation{
				Code: "DRR_PERIOD_MISMATCH", Message: "traffic and attribution periods are not comparable",
			})
			if block.Attribution.State == BlockReady {
				block.Attribution.State = BlockPartial
			}
		}
	}
	block.State = aggregateCampaignPerfState(block.Traffic, block.Attribution)
	return block
}

func buildCampaignTrafficBlockV4(
	id int64,
	cur, prev map[int64]factualapi.CampaignTrafficPerformance,
	curErr, prevErr error,
) *CampaignTrafficBlock {
	if curErr != nil && prevErr != nil {
		return &CampaignTrafficBlock{State: BlockUnavailable, Limitations: unavailableBlock("CAMPAIGN_TRAFFIC_UNAVAILABLE", curErr.Error())}
	}
	var curP, prevP *factualapi.CampaignTrafficPerformance
	if p, ok := cur[id]; ok {
		cp := p
		curP = &cp
	}
	if p, ok := prev[id]; ok {
		cp := p
		prevP = &cp
	}
	if curP == nil && prevP == nil {
		block := &CampaignTrafficBlock{State: BlockReadyEmpty}
		if curErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_TRAFFIC_CUR_UNAVAILABLE", Message: curErr.Error()})
		}
		return block
	}
	block := &CampaignTrafficBlock{State: BlockReady}
	if curP != nil {
		block.CurrentPeriod = periodToDateRange(curP.Period)
	}
	if prevP != nil {
		block.PreviousPeriod = periodToDateRange(prevP.Period)
	}
	var curSpend, prevSpend, curBonus, prevBonus *MoneyAmount
	var curImp, prevImp, curClk, prevClk *int64
	var curCTR, prevCTR *canonicaljson.Decimal
	var curCPC, prevCPC, curCPM, prevCPM *MoneyAmount
	if curP != nil {
		curSpend = moneyFromValue(curP.PaidSpend)
		curBonus = moneyFromValue(curP.BonusSpend)
		curImp = intFromValue(curP.Impressions)
		curClk = intFromValue(curP.Clicks)
		curCTR = decFromValue(curP.CTRPct)
		curCPC = moneyFromValue(curP.CPC)
		curCPM = moneyFromValue(curP.CPM)
	}
	if prevP != nil {
		prevSpend = moneyFromValue(prevP.PaidSpend)
		prevBonus = moneyFromValue(prevP.BonusSpend)
		prevImp = intFromValue(prevP.Impressions)
		prevClk = intFromValue(prevP.Clicks)
		prevCTR = decFromValue(prevP.CTRPct)
		prevCPC = moneyFromValue(prevP.CPC)
		prevCPM = moneyFromValue(prevP.CPM)
	}
	block.PaidSpend = BuildMoneyComparison(curSpend, prevSpend)
	block.Impressions = BuildCountComparison(curImp, prevImp)
	block.Clicks = BuildCountComparison(curClk, prevClk)
	if curBonus != nil || prevBonus != nil {
		b := BuildMoneyComparison(curBonus, prevBonus)
		block.BonusSpend = &b
	}
	if curCTR != nil || prevCTR != nil {
		p := BuildPctPointComparison(curCTR, prevCTR)
		block.CTR = &p
	}
	if curCPC != nil || prevCPC != nil {
		p := BuildMoneyComparison(curCPC, prevCPC)
		block.CPC = &p
	}
	if curCPM != nil || prevCPM != nil {
		p := BuildMoneyComparison(curCPM, prevCPM)
		block.CPM = &p
	}
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_TRAFFIC_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_TRAFFIC_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func buildCampaignAttributionBlockV4(
	id int64,
	cur, prev map[int64]factualapi.CampaignAttributionPerformance,
	curErr, prevErr error,
) *CampaignAttributionBlock {
	if curErr != nil && prevErr != nil {
		return &CampaignAttributionBlock{State: BlockUnavailable, Limitations: unavailableBlock("CAMPAIGN_ATTRIBUTION_UNAVAILABLE", curErr.Error())}
	}
	var curP, prevP *factualapi.CampaignAttributionPerformance
	if p, ok := cur[id]; ok {
		cp := p
		curP = &cp
	}
	if p, ok := prev[id]; ok {
		cp := p
		prevP = &cp
	}
	if curP == nil && prevP == nil {
		block := &CampaignAttributionBlock{State: BlockReadyEmpty}
		if curErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_ATTRIBUTION_CUR_UNAVAILABLE", Message: curErr.Error()})
		}
		return block
	}
	block := &CampaignAttributionBlock{State: BlockReady}
	if curP != nil {
		block.CurrentPeriod = periodToDateRange(curP.Period)
	}
	if prevP != nil {
		block.PreviousPeriod = periodToDateRange(prevP.Period)
	}
	var curAttr, prevAttr *MoneyAmount
	var curU, prevU *int64
	var curDRR, prevDRR *canonicaljson.Decimal
	if curP != nil {
		curAttr = moneyFromValue(curP.AttributedOrderAmount)
		curU = intFromValue(curP.AttributedOrderUnits)
		curDRR = decFromValue(curP.ProviderDRRPct)
	}
	if prevP != nil {
		prevAttr = moneyFromValue(prevP.AttributedOrderAmount)
		prevU = intFromValue(prevP.AttributedOrderUnits)
		prevDRR = decFromValue(prevP.ProviderDRRPct)
	}
	block.AttributedOrderAmount = BuildMoneyComparison(curAttr, prevAttr)
	block.AttributedOrderUnits = BuildCountComparison(curU, prevU)
	if curDRR != nil || prevDRR != nil {
		p := BuildPctPointComparison(curDRR, prevDRR)
		block.ProviderDRR = &p
	}
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_ATTRIBUTION_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CAMPAIGN_ATTRIBUTION_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func buildCampaignTargetingFromSummary(sum *factualapi.CampaignTargetingSummary, targetScope string) *CampaignTargetBlock {
	if sum == nil {
		return &CampaignTargetBlock{State: BlockReadyEmpty, TargetScope: targetScope}
	}
	block := &CampaignTargetBlock{State: BlockReady, TargetScope: nz(sum.TargetScope, targetScope)}
	if sum.ExplicitAssignedSKUCount.State == factualapi.ValueNotApplicable {
		block.AssignedSKUCount = nil
	} else {
		block.AssignedSKUCount = intFromValue(sum.ExplicitAssignedSKUCount)
	}
	block.SKUWithObservedPerformance = intFromValue(sum.SKUWithObservedPerformanceCount)
	if block.AssignedSKUCount != nil && *block.AssignedSKUCount == 0 {
		block.State = BlockReadyEmpty
	}
	return block
}

func rollupCategoryAdsV4(catID int64, skus []SKULandscape) *CategoryAdsBlock {
	var mapped, trafficActivity, attrActivity int64
	var trafficCur, trafficPrev, attrCur, attrPrev MoneyAccumulator
	var curUnits, prevUnits int64
	var knownCurUnits, knownPrevUnits bool
	var trafficPeriodCur, trafficPeriodPrev, attrPeriodCur, attrPeriodPrev *DateRange
	trafficKnown := false
	attrKnown := false

	for _, sk := range skus {
		if !skuMatchesCategory(sk, catID) {
			continue
		}
		mapped++
		if sk.Advertising == nil {
			continue
		}
		if sk.Advertising.Traffic != nil {
			trafficKnown = true
			tr := sk.Advertising.Traffic
			if trafficPeriodCur == nil {
				trafficPeriodCur = cloneDateRange(tr.CurrentPeriod)
				trafficPeriodPrev = cloneDateRange(tr.PreviousPeriod)
			}
			addedCur := trafficCur.Add(tr.PaidSpend.Current)
			addedPrev := trafficPrev.Add(tr.PaidSpend.Previous)
			if addedCur || addedPrev || tr.PaidSpend.Current != nil || tr.PaidSpend.Previous != nil {
				trafficActivity++
			}
		}
		if sk.Advertising.Attribution != nil {
			attrKnown = true
			at := sk.Advertising.Attribution
			if attrPeriodCur == nil {
				attrPeriodCur = cloneDateRange(at.CurrentPeriod)
				attrPeriodPrev = cloneDateRange(at.PreviousPeriod)
			}
			added := attrCur.Add(at.AttributedOrderAmount.Current)
			attrPrev.Add(at.AttributedOrderAmount.Previous)
			if added || at.AttributedOrderAmount.Current != nil || at.AttributedOrderAmount.Previous != nil {
				attrActivity++
			}
			if at.AttributedOrderUnits.Current != nil {
				knownCurUnits = true
				curUnits += *at.AttributedOrderUnits.Current
			}
			if at.AttributedOrderUnits.Previous != nil {
				knownPrevUnits = true
				prevUnits += *at.AttributedOrderUnits.Previous
			}
		}
	}

	if !trafficKnown && !attrKnown {
		return nil
	}
	block := &CategoryAdsBlock{State: BlockReady}
	if trafficKnown {
		tr := &CategoryAdsTrafficBlock{
			State:          BlockReady,
			CurrentPeriod:  trafficPeriodCur,
			PreviousPeriod: trafficPeriodPrev,
			PaidSpend:      BuildMoneyComparison(trafficCur.Result(), trafficPrev.Result()),
			SKUWithTrafficActivity: func() *int64 {
				v := trafficActivity
				return &v
			}(),
			MappingCoverage: coverageRatio(trafficActivity, mapped),
		}
		block.Traffic = tr
	}
	if attrKnown {
		at := &CategoryAdsAttributionBlock{
			State:                      BlockReady,
			CurrentPeriod:              attrPeriodCur,
			PreviousPeriod:             attrPeriodPrev,
			AttributedOrderAmount:      BuildMoneyComparison(attrCur.Result(), attrPrev.Result()),
			AttributedOrderUnits:       BuildCountComparison(int64PtrKnown(curUnits, knownCurUnits), int64PtrKnown(prevUnits, knownPrevUnits)),
			SKUWithAttributionActivity: func() *int64 { v := attrActivity; return &v }(),
			MappingCoverage:            coverageRatio(attrActivity, mapped),
		}
		block.Attribution = at
	}
	return block
}

func ctrFromCounts(clicks, impressions *int64) *canonicaljson.Decimal {
	if clicks == nil || impressions == nil || *impressions == 0 {
		return nil
	}
	r := new(big.Rat).SetFrac64(*clicks*100, *impressions)
	d := canonicaljson.Decimal(formatRatLocal(r))
	return &d
}

func cpcFromSpendClicks(spend *MoneyAmount, clicks *int64) *MoneyAmount {
	if spend == nil || clicks == nil || *clicks == 0 {
		return nil
	}
	num, err := parseDecLocal(spend.Amount)
	if err != nil {
		return nil
	}
	r := new(big.Rat).Quo(num, new(big.Rat).SetInt64(*clicks))
	return &MoneyAmount{Amount: canonicaljson.Decimal(formatRatLocal(r)), CurrencyCode: spend.CurrencyCode}
}

func advertisingDomainWindow(windows []DomainWindow, family string) *DomainWindow {
	for i := range windows {
		w := &windows[i]
		if w.Domain == "advertising" && w.FactFamily == family {
			return w
		}
	}
	return nil
}

func stampAdPeriodPair(cur, prev **DateRange, win *DomainWindow) {
	if win == nil || cur == nil || prev == nil {
		return
	}
	if *cur == nil {
		*cur = cloneDateRange(win.Current)
	}
	if *prev == nil {
		*prev = cloneDateRange(win.Previous)
	}
}

func stampUsableAdPeriods(state BlockState, cur, prev **DateRange, win *DomainWindow) {
	if state != BlockReady && state != BlockPartial {
		return
	}
	stampAdPeriodPair(cur, prev, win)
}

func stampAdvertisingPeriodsFromWindows(feed *CoreFeed) {
	if feed == nil {
		return
	}
	trafficWin := advertisingDomainWindow(feed.DomainWindows, "ADVERTISING.TRAFFIC")
	attrWin := advertisingDomainWindow(feed.DomainWindows, "ADVERTISING.ATTRIBUTION")
	if feed.Account.Advertising != nil {
		if tr := feed.Account.Advertising.Traffic; tr != nil {
			stampUsableAdPeriods(tr.State, &tr.CurrentPeriod, &tr.PreviousPeriod, trafficWin)
		}
		if at := feed.Account.Advertising.Attribution; at != nil {
			stampUsableAdPeriods(at.State, &at.CurrentPeriod, &at.PreviousPeriod, attrWin)
		}
	}
	for i := range feed.Campaigns {
		p := feed.Campaigns[i].Performance
		if p == nil {
			continue
		}
		if p.Traffic != nil {
			stampUsableAdPeriods(p.Traffic.State, &p.Traffic.CurrentPeriod, &p.Traffic.PreviousPeriod, trafficWin)
		}
		if p.Attribution != nil {
			stampUsableAdPeriods(p.Attribution.State, &p.Attribution.CurrentPeriod, &p.Attribution.PreviousPeriod, attrWin)
		}
	}
	for i := range feed.SKUs {
		ads := feed.SKUs[i].Advertising
		if ads == nil {
			continue
		}
		if ads.Traffic != nil {
			stampUsableAdPeriods(ads.Traffic.State, &ads.Traffic.CurrentPeriod, &ads.Traffic.PreviousPeriod, trafficWin)
		}
		if ads.Attribution != nil {
			stampUsableAdPeriods(ads.Attribution.State, &ads.Attribution.CurrentPeriod, &ads.Attribution.PreviousPeriod, attrWin)
		}
	}
	for i := range feed.Categories {
		ads := feed.Categories[i].Advertising
		if ads == nil {
			continue
		}
		if ads.Traffic != nil {
			stampUsableAdPeriods(ads.Traffic.State, &ads.Traffic.CurrentPeriod, &ads.Traffic.PreviousPeriod, trafficWin)
		}
		if ads.Attribution != nil {
			stampUsableAdPeriods(ads.Attribution.State, &ads.Attribution.CurrentPeriod, &ads.Attribution.PreviousPeriod, attrWin)
		}
	}
}

func cpmFromSpendImpressions(spend *MoneyAmount, impressions *int64) *MoneyAmount {
	if spend == nil || impressions == nil || *impressions == 0 {
		return nil
	}
	num, err := parseDecLocal(spend.Amount)
	if err != nil {
		return nil
	}
	r := new(big.Rat).Quo(num, new(big.Rat).SetInt64(*impressions))
	r.Mul(r, big.NewRat(1000, 1))
	return &MoneyAmount{Amount: canonicaljson.Decimal(formatRatLocal(r)), CurrencyCode: spend.CurrencyCode}
}
