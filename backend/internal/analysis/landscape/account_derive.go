package landscape

import (
	"math/big"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func deriveAccountBlocks(feed *CoreFeed, profile ExecutableProfileV3, sealed factualapi.FactualReadContext) {
	if feed == nil {
		return
	}
	deriveAccountIdentity(feed)

	if profile.HasBlock("account.assortment") {
		deriveAccountAssortment(feed)
	}
	if spec, ok := profile.Block("account.stock"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) {
		deriveAccountStock(feed)
	}
	if spec, ok := profile.Block("account.price"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) {
		deriveAccountPrice(feed)
	}
	if spec, ok := profile.Block("account.search"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) {
		deriveAccountSearch(feed)
	}
	if spec, ok := profile.Block("account.advertising.traffic"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) {
		deriveAccountAdvertising(feed)
	} else if spec, ok := profile.Block("account.advertising.attribution"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) {
		deriveAccountAdvertising(feed)
	}
	if profile.HasBlock("account.concentration") {
		deriveAccountConcentration(feed)
	}
	if spec, ok := profile.Block("account.supply"); ok && capabilityReadyV3(feed.GlobalCapabilities, spec) && feed.Account.Supply == nil {
		deriveAccountSupply(feed)
	}
}

func deriveAccountIdentity(feed *CoreFeed) {
	var active, archived int64
	catSet := map[int64]struct{}{}
	for _, sk := range feed.SKUs {
		if catalogStateActive(sk.Identity.CatalogState) {
			active++
		} else if strings.EqualFold(sk.Identity.CatalogState, "ARCHIVED") || strings.EqualFold(sk.Identity.CatalogState, "DELETED") {
			archived++
		}
		if sk.Identity.CategoryID != nil {
			catSet[*sk.Identity.CategoryID] = struct{}{}
		}
	}
	for _, c := range feed.Categories {
		if c.Identity.Unmapped {
			continue
		}
		catSet[c.Identity.CategoryID] = struct{}{}
	}
	catCount := int64(len(catSet))
	campN := int64(len(feed.Campaigns))
	feed.Account.Identity.ActiveSKUCount = &active
	feed.Account.Identity.ArchivedSKUCount = &archived
	feed.Account.Identity.CategoryCount = &catCount
	feed.Account.Identity.CampaignCurrentCount = &campN
}

func deriveAccountAssortment(feed *CoreFeed) {
	var active, archived, withSales, withoutSales, unknownSales, unmapped int64
	catSet := map[int64]struct{}{}
	var unmappedSalesNum, totalSalesDenom big.Rat

	for _, sk := range feed.SKUs {
		if catalogStateActive(sk.Identity.CatalogState) {
			active++
		} else if strings.EqualFold(sk.Identity.CatalogState, "ARCHIVED") || strings.EqualFold(sk.Identity.CatalogState, "DELETED") {
			archived++
		}
		if sk.Identity.Unmapped {
			unmapped++
		}
		if sk.Identity.CategoryID != nil {
			catSet[*sk.Identity.CategoryID] = struct{}{}
		}
		switch classifySKUCurrentDemand(sk) {
		case DemandPositive:
			withSales++
			if sk.Sales != nil && sk.Sales.OrderedAmount.Current != nil {
				if r, err := parseDecLocal(sk.Sales.OrderedAmount.Current.Amount); err == nil {
					totalSalesDenom.Add(&totalSalesDenom, r)
					if sk.Identity.Unmapped {
						unmappedSalesNum.Add(&unmappedSalesNum, r)
					}
				}
			}
		case DemandZero:
			withoutSales++
		default:
			unknownSales++
		}
	}
	for _, c := range feed.Categories {
		if !c.Identity.Unmapped {
			catSet[c.Identity.CategoryID] = struct{}{}
		}
	}
	catCount := int64(len(catSet))
	block := &AccountAssortmentBlock{
		State:                       BlockReady,
		ActiveSKUCount:              &active,
		ArchivedSKUCount:            &archived,
		SKUWithSalesCurrent7:        &withSales,
		SKUWithoutSalesCurrent7:     &withoutSales,
		SKUWithUnknownSalesCurrent7: &unknownSales,
		CategoryCount:               &catCount,
		UnmappedSKUCount:            &unmapped,
	}
	if totalSalesDenom.Sign() > 0 {
		share := new(big.Rat).Quo(&unmappedSalesNum, &totalSalesDenom)
		share.Mul(share, big.NewRat(100, 1))
		d := canonicaljson.Decimal(formatRatLocal(share))
		block.UnmappedSalesShare = &d
	}
	feed.Account.Assortment = block
}

func deriveAccountStock(feed *CoreFeed) {
	var sellableTotal, demandPos, zeroSellDemand, noDemand, demandUnknown int64
	var coverageIncomplete bool
	var coverages []*big.Rat
	zeroNum := MoneyAccumulator{}
	zeroDen := MoneyAccumulator{}

	for _, sk := range feed.SKUs {
		demand := classifySKUCurrentDemand(sk)
		switch demand {
		case DemandPositive:
			demandPos++
		case DemandZero:
			noDemand++
		default:
			demandUnknown++
		}
		zeroSellable := false
		if sk.Stock != nil && sk.Stock.SellableUnits != nil {
			sellableTotal += *sk.Stock.SellableUnits
			if *sk.Stock.SellableUnits == 0 {
				zeroSellable = true
			}
			if sk.Stock.CoverageDays7D != nil {
				if r, err := parseDecLocal(*sk.Stock.CoverageDays7D); err == nil {
					coverages = append(coverages, r)
				}
			} else if demand == DemandPositive {
				coverageIncomplete = true
			}
		} else if sk.Stock != nil && sk.Stock.State != BlockReadyEmpty {
			coverageIncomplete = true
		}
		if sk.Sales != nil && sk.Sales.OrderedAmount.Current != nil {
			if r, err := parseDecLocal(sk.Sales.OrderedAmount.Current.Amount); err == nil {
				zeroDen.Add(sk.Sales.OrderedAmount.Current)
				if zeroSellable && demand == DemandPositive {
					zeroSellDemand++
					zeroNum.Add(sk.Sales.OrderedAmount.Current)
				}
				_ = r
			}
		} else if demand == DemandUnknown {
			coverageIncomplete = true
		}
	}
	block := &AccountStockBlock{
		State:                       BlockReady,
		TotalSellableUnits:          &sellableTotal,
		DemandPositiveSKUCount:      &demandPos,
		ZeroSellableWithDemandCount: &zeroSellDemand,
		NoDemandSKUCount:            &noDemand,
		DemandUnknownSKUCount:       &demandUnknown,
	}
	if len(coverages) > 0 {
		block.CoverageP25 = nearestRankQuantileV1(coverages, 1, 4)
		block.CoverageP50 = nearestRankQuantileV1(coverages, 1, 2)
		block.CoverageP75 = nearestRankQuantileV1(coverages, 3, 4)
	}
	if zeroDen.KnownCount > 0 {
		if share, lim := shareOfKnown(zeroNum.Result(), zeroDen.Result()); share != nil {
			block.SalesExposureZeroStockPct = share
		} else if lim != nil {
			block.Limitations = append(block.Limitations, *lim)
			block.State = BlockPartial
		}
	}
	if demandUnknown > 0 {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{
			Code: "SALES_DEMAND_INCOMPLETE", Message: "unknown current sales demand SKUs excluded from zero-stock exposure denominator",
		})
	}
	if coverageIncomplete {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{
			Code: "STOCK_COVERAGE_INCOMPLETE", Message: "coverage percentiles omit SKUs without known coverage",
		})
	}
	feed.Account.Stock = block
}

func deriveAccountPrice(feed *CoreFeed) {
	var observed, increased, decreased, unchanged, withHistory int64
	for _, sk := range feed.SKUs {
		if sk.Price == nil || sk.Price.CurrentPrice == nil {
			continue
		}
		observed++
		if sk.Price.LastObservedChange == nil {
			continue
		}
		withHistory++
		switch sk.Price.LastObservedChange.ChangeState {
		case ChangeIncreased, ChangeFromZero:
			increased++
		case ChangeDecreased, ChangeToZero:
			decreased++
		case ChangeUnchanged:
			unchanged++
		}
	}
	block := &AccountPriceBlock{
		State:                     BlockReady,
		SKUWithObservedPriceCount: &observed,
		PriceIncreasedSKUCount:    &increased,
		PriceDecreasedSKUCount:    &decreased,
		PriceUnchangedSKUCount:    &unchanged,
	}
	if observed > 0 {
		block.PriceChangeCoverage = coverageRatio(withHistory, observed)
	}
	feed.Account.Price = block
}

func deriveAccountSearch(feed *CoreFeed) {
	var searchWith, posUp, posDown int64
	var positions []*big.Rat
	for _, sk := range feed.SKUs {
		if sk.Search == nil || sk.Search.State != BlockReady {
			continue
		}
		if sk.Search.ProviderAveragePosition == nil {
			continue
		}
		searchWith++
		if r, err := parseDecLocal(*sk.Search.ProviderAveragePosition); err == nil {
			positions = append(positions, r)
		}
		if sk.Search.ProviderAveragePosition != nil && sk.Search.PreviousProviderAveragePosition != nil {
			cur, err1 := parseDecLocal(*sk.Search.ProviderAveragePosition)
			prev, err2 := parseDecLocal(*sk.Search.PreviousProviderAveragePosition)
			if err1 == nil && err2 == nil {
				cmp := cur.Cmp(prev)
				if cmp < 0 {
					posUp++
				} else if cmp > 0 {
					posDown++
				}
			}
		}
	}
	block := &AccountSearchBlock{
		State:                BlockReady,
		SKUWithSearchData:    &searchWith,
		SKUPositionUpCount:   &posUp,
		SKUPositionDownCount: &posDown,
	}
	if len(positions) > 0 {
		block.PositionP25 = nearestRankQuantileV1(positions, 1, 4)
		block.PositionP50 = nearestRankQuantileV1(positions, 1, 2)
		block.PositionP75 = nearestRankQuantileV1(positions, 3, 4)
	}
	feed.Account.Search = block
}

func deriveAccountAdvertising(feed *CoreFeed) {
	var trafficCur, trafficPrev, bonusCur, bonusPrev, attrCur, attrPrev MoneyAccumulator
	var curImp, prevImp, curClk, prevClk, curUnits, prevUnits CountAccumulator
	var withSpend, running int64
	var trafficPeriodCur, trafficPeriodPrev, attrPeriodCur, attrPeriodPrev *DateRange
	trafficKnown := false
	attrKnown := false

	for _, camp := range feed.Campaigns {
		if camp.Identity.ProviderState != "" && strings.EqualFold(camp.Identity.ProviderState, "RUNNING") {
			running++
		}
		if camp.Performance != nil && camp.Performance.Traffic != nil {
			trafficKnown = true
			p := camp.Performance.Traffic
			if trafficPeriodCur == nil {
				trafficPeriodCur = cloneDateRange(p.CurrentPeriod)
				trafficPeriodPrev = cloneDateRange(p.PreviousPeriod)
			}
			trafficCur.Add(p.PaidSpend.Current)
			if moneyIsKnownPositive(p.PaidSpend.Current) {
				withSpend++
			}
			trafficPrev.Add(p.PaidSpend.Previous)
			bonusCur.Add(moneyPtrCurrent(p.BonusSpend))
			bonusPrev.Add(moneyPtrPrevious(p.BonusSpend))
			curImp.Observe(p.Impressions.Current)
			prevImp.Observe(p.Impressions.Previous)
			curClk.Observe(p.Clicks.Current)
			prevClk.Observe(p.Clicks.Previous)
		}
		if camp.Performance != nil && camp.Performance.Attribution != nil {
			attrKnown = true
			a := camp.Performance.Attribution
			if attrPeriodCur == nil {
				attrPeriodCur = cloneDateRange(a.CurrentPeriod)
				attrPeriodPrev = cloneDateRange(a.PreviousPeriod)
			}
			attrCur.Add(a.AttributedOrderAmount.Current)
			attrPrev.Add(a.AttributedOrderAmount.Previous)
			curUnits.Observe(a.AttributedOrderUnits.Current)
			prevUnits.Observe(a.AttributedOrderUnits.Previous)
		}
	}

	block := &AccountAdsBlock{State: BlockReady}
	if trafficKnown {
		tr := &AccountAdsTrafficBlock{
			State:          BlockReady,
			CurrentPeriod:  trafficPeriodCur,
			PreviousPeriod: trafficPeriodPrev,
			PaidSpend:      BuildMoneyComparison(trafficCur.Result(), trafficPrev.Result()),
			Impressions:    BuildCountComparison(curImp.Result(), prevImp.Result()),
			Clicks:         BuildCountComparison(curClk.Result(), prevClk.Result()),
		}
		if bonusCur.KnownCount > 0 || bonusPrev.KnownCount > 0 {
			b := BuildMoneyComparison(bonusCur.Result(), bonusPrev.Result())
			tr.BonusSpend = &b
		}
		if curImp.Incomplete() || prevImp.Incomplete() || curClk.Incomplete() || prevClk.Incomplete() {
			tr.State = BlockPartial
			tr.Limitations = append(tr.Limitations, Limitation{
				Code: "ACCOUNT_AD_COUNTS_INCOMPLETE", Message: "some campaign traffic counts are unknown",
			})
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{
				Code: "ACCOUNT_AD_COUNTS_INCOMPLETE", Message: "some campaign traffic counts are unknown",
			})
		}
		if curImp.UnknownCount == 0 && prevImp.UnknownCount == 0 && curClk.UnknownCount == 0 && prevClk.UnknownCount == 0 {
			curImpV, prevImpV := curImp.Result(), prevImp.Result()
			curClkV, prevClkV := curClk.Result(), prevClk.Result()
			if (curImpV != nil && *curImpV > 0) || (prevImpV != nil && *prevImpV > 0) {
				var curCTR, prevCTR *canonicaljson.Decimal
				if curImpV != nil && *curImpV > 0 && curClkV != nil {
					d := canonicaljson.Decimal(formatRatLocal(new(big.Rat).SetFrac64(*curClkV*100, *curImpV)))
					curCTR = &d
				}
				if prevImpV != nil && *prevImpV > 0 && prevClkV != nil {
					d := canonicaljson.Decimal(formatRatLocal(new(big.Rat).SetFrac64(*prevClkV*100, *prevImpV)))
					prevCTR = &d
				}
				cmp := BuildPctPointComparison(curCTR, prevCTR)
				tr.CTR = &cmp
			}
			if cpc := cpcFromSpendClicks(trafficCur.Result(), curClkV); cpc != nil || cpcFromSpendClicks(trafficPrev.Result(), prevClkV) != nil {
				cmp := BuildMoneyComparison(cpcFromSpendClicks(trafficCur.Result(), curClkV), cpcFromSpendClicks(trafficPrev.Result(), prevClkV))
				tr.CPC = &cmp
			}
			if cpm := cpmFromSpendImpressions(trafficCur.Result(), curImpV); cpm != nil || cpmFromSpendImpressions(trafficPrev.Result(), prevImpV) != nil {
				cmp := BuildMoneyComparison(cpmFromSpendImpressions(trafficCur.Result(), curImpV), cpmFromSpendImpressions(trafficPrev.Result(), prevImpV))
				tr.CPM = &cmp
			}
		}
		block.Traffic = tr
	}
	if attrKnown {
		at := &AccountAdsAttributionBlock{
			State:                 BlockReady,
			CurrentPeriod:         attrPeriodCur,
			PreviousPeriod:        attrPeriodPrev,
			AttributedOrderAmount: BuildMoneyComparison(attrCur.Result(), attrPrev.Result()),
			AttributedOrderUnits:  BuildCountComparison(curUnits.Result(), prevUnits.Result()),
		}
		if curUnits.Incomplete() || prevUnits.Incomplete() {
			at.State = BlockPartial
			at.Limitations = append(at.Limitations, Limitation{
				Code: "ACCOUNT_AD_COUNTS_INCOMPLETE", Message: "some campaign attribution units are unknown",
			})
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{
				Code: "ACCOUNT_AD_COUNTS_INCOMPLETE", Message: "some campaign attribution units are unknown",
			})
		}
		if dateRangesEqual(trafficPeriodCur, attrPeriodCur) && dateRangesEqual(trafficPeriodPrev, attrPeriodPrev) {
			if drr := calculatedDRRComparison(trafficCur.Result(), attrCur.Result(), trafficPrev.Result(), attrPrev.Result()); drr != nil {
				at.CalculatedDRR = drr
			}
		} else if trafficKnown {
			at.Limitations = append(at.Limitations, Limitation{Code: "DRR_PERIOD_MISMATCH", Message: "traffic and attribution periods are not comparable"})
		}
		block.Attribution = at
	}
	block.CampaignCountRunning = &running
	block.CampaignCountWithSpend = &withSpend
	if capabilityReadyV3(feed.GlobalCapabilities, BlockSpecV3{CapabilityKeys: []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}, CapabilityMatch: CapabilityAll}) && block.Traffic == nil {
		block.Traffic = &AccountAdsTrafficBlock{State: BlockReadyEmpty}
	}
	if capabilityReadyV3(feed.GlobalCapabilities, BlockSpecV3{CapabilityKeys: []string{"ADVERTISING.CAMPAIGN_ATTRIBUTION"}, CapabilityMatch: CapabilityAll}) && block.Attribution == nil {
		block.Attribution = &AccountAdsAttributionBlock{State: BlockReadyEmpty}
	}
	if !trafficKnown && !attrKnown {
		block.State = BlockReadyEmpty
	}
	feed.Account.Advertising = block
}

func deriveAccountConcentration(feed *CoreFeed) {
	block := &AccountConcentrationBlock{State: BlockReady}

	skuAmounts := make([]amountEntry, 0, len(feed.SKUs))
	for _, sk := range feed.SKUs {
		if sk.Sales == nil || sk.Sales.OrderedAmount.Current == nil {
			continue
		}
		if r, err := parseDecLocal(sk.Sales.OrderedAmount.Current.Amount); err == nil && r.Sign() > 0 {
			skuAmounts = append(skuAmounts, amountEntry{id: sk.Identity.CanonicalSKUID, amount: r})
		}
	}
	if share, lim := topNShare(skuAmounts, 5); share != nil {
		block.Top5SKUOrderedAmountShare = share
	} else if lim != nil {
		block.Limitations = append(block.Limitations, *lim)
		block.State = BlockPartial
	}

	catAmounts := make([]amountEntry, 0, len(feed.Categories))
	for _, c := range feed.Categories {
		if c.Identity.Unmapped || c.Sales == nil || c.Sales.OrderedAmount.Current == nil {
			continue
		}
		if r, err := parseDecLocal(c.Sales.OrderedAmount.Current.Amount); err == nil && r.Sign() > 0 {
			catAmounts = append(catAmounts, amountEntry{id: c.Identity.CategoryID, amount: r})
		}
	}
	if share, lim := topNShare(catAmounts, 3); share != nil {
		block.Top3CategoryOrderedAmountShare = share
	} else if lim != nil {
		block.Limitations = append(block.Limitations, *lim)
		block.State = BlockPartial
	}

	campAmounts := make([]amountEntry, 0, len(feed.Campaigns))
	for _, c := range feed.Campaigns {
		if c.Performance == nil || c.Performance.Traffic == nil || c.Performance.Traffic.PaidSpend.Current == nil {
			continue
		}
		if r, err := parseDecLocal(c.Performance.Traffic.PaidSpend.Current.Amount); err == nil && r.Sign() > 0 {
			campAmounts = append(campAmounts, amountEntry{id: c.Identity.CampaignID, amount: r})
		}
	}
	if share, lim := topNShare(campAmounts, 5); share != nil {
		block.Top5CampaignSpendShare = share
	} else if lim != nil {
		block.Limitations = append(block.Limitations, *lim)
		block.State = BlockPartial
	}
	feed.Account.Concentration = block
}

func deriveAccountSupply(feed *CoreFeed) {
	var inboundSKU, overdue int64
	for _, sk := range feed.SKUs {
		if sk.Supply == nil {
			continue
		}
		if sk.Supply.OpenSupplyCount != nil && *sk.Supply.OpenSupplyCount > 0 {
			inboundSKU++
		}
		if sk.Supply.OverdueSupplyCount != nil {
			overdue += *sk.Supply.OverdueSupplyCount
		}
	}
	block := &AccountSupplyBlock{State: BlockReady, SKUWithOpenInboundCount: &inboundSKU, OverdueSupplyCount: &overdue}
	if inboundSKU == 0 && overdue == 0 {
		block.State = BlockReadyEmpty
	}
	feed.Account.Supply = block
}

type amountEntry struct {
	id     int64
	amount *big.Rat
}

func topNShare(entries []amountEntry, n int) (*canonicaljson.Decimal, *Limitation) {
	if len(entries) == 0 {
		return nil, &Limitation{Code: "CONCENTRATION_DENOMINATOR_UNAVAILABLE", Message: "no known positive amounts for concentration share"}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].amount.Cmp(entries[j].amount) > 0
	})
	var total, top big.Rat
	for _, e := range entries {
		total.Add(&total, e.amount)
	}
	limit := n
	if limit > len(entries) {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		top.Add(&top, entries[i].amount)
	}
	if total.Sign() == 0 {
		return nil, &Limitation{Code: "CONCENTRATION_DENOMINATOR_UNAVAILABLE", Message: "denominator is zero"}
	}
	share := new(big.Rat).Quo(&top, &total)
	share.Mul(share, big.NewRat(100, 1))
	d := canonicaljson.Decimal(formatRatLocal(share))
	return &d, nil
}

func skuHasKnownPositiveSales(sk SKULandscape) bool {
	return sk.Sales != nil && sk.Sales.State == BlockReady &&
		((sk.Sales.OrderedUnits.Current != nil && *sk.Sales.OrderedUnits.Current > 0) ||
			(sk.Sales.OrderedAmount.Current != nil && sk.Sales.OrderedAmount.Current.Amount != ""))
}

func addMoneyRat(acc *big.Rat, currency *string, m *MoneyAmount) bool {
	if m == nil || m.Amount == "" {
		return false
	}
	r, err := parseDecLocal(m.Amount)
	if err != nil || r.Sign() <= 0 {
		return false
	}
	if *currency == "" {
		*currency = m.CurrencyCode
	}
	if *currency != "" && m.CurrencyCode != "" && *currency != m.CurrencyCode {
		return false
	}
	acc.Add(acc, r)
	return true
}

func moneyFromRat(r *big.Rat, currency string) *MoneyAmount {
	if r == nil || r.Sign() == 0 {
		return nil
	}
	return &MoneyAmount{Amount: canonicaljson.Decimal(formatRatLocal(r)), CurrencyCode: currency}
}

func moneyPtrCurrent(m *MoneyComparison) *MoneyAmount {
	if m == nil {
		return nil
	}
	return m.Current
}

func moneyPtrPrevious(m *MoneyComparison) *MoneyAmount {
	if m == nil {
		return nil
	}
	return m.Previous
}

func int64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	x := v
	return &x
}
