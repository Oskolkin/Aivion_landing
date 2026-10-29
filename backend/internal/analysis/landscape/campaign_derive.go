package landscape

import (
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func deriveCampaignCoreBlocks(feed *CoreFeed, profile ExecutableProfileV3) {
	if feed == nil {
		return
	}
	if profile.HasBlock("campaign.exposure") {
		deriveCampaignExposure(feed)
	}
}

func deriveCampaignExposure(feed *CoreFeed) {
	if feed.Account.Advertising == nil || (feed.Account.Advertising.Traffic == nil && feed.Account.Advertising.Attribution == nil) {
		for i := range feed.Campaigns {
			if feed.Campaigns[i].Exposure != nil {
				continue
			}
			feed.Campaigns[i].Exposure = &CampaignExposureBlock{
				State: BlockUnavailable,
				Limitations: []Limitation{{
					Code: "CAMPAIGN_EXPOSURE_DENOMINATOR_UNKNOWN", Message: "account advertising block unavailable",
				}},
			}
		}
		return
	}
	var accountSpendCur, accountSpendPrev, accountAttrCur *MoneyAmount
	if feed.Account.Advertising.Traffic != nil {
		accountSpendCur = moneyComparisonCurrent(feed.Account.Advertising.Traffic.PaidSpend)
		accountSpendPrev = moneyComparisonPrevious(feed.Account.Advertising.Traffic.PaidSpend)
	}
	if feed.Account.Advertising.Attribution != nil {
		accountAttrCur = moneyComparisonCurrent(feed.Account.Advertising.Attribution.AttributedOrderAmount)
	}

	for i := range feed.Campaigns {
		if feed.Campaigns[i].Exposure != nil {
			continue
		}
		block := &CampaignExposureBlock{State: BlockReady}
		computed := false
		if feed.Campaigns[i].Performance != nil && feed.Campaigns[i].Performance.Traffic != nil {
			traffic := feed.Campaigns[i].Performance.Traffic
			if share, lim := shareOfAccount(traffic.PaidSpend.Current, accountSpendCur); share != nil {
				block.PaidSpendShareOfAccountAds = share
				computed = true
			} else if lim != nil {
				block.Limitations = append(block.Limitations, *lim)
				block.State = BlockPartial
			}
		}
		if feed.Campaigns[i].Performance != nil && feed.Campaigns[i].Performance.Attribution != nil {
			attr := feed.Campaigns[i].Performance.Attribution
			if share, lim := shareOfAccount(attr.AttributedOrderAmount.Current, accountAttrCur); share != nil {
				block.AttributedShareOfAccountAds = share
				computed = true
			} else if lim != nil {
				block.Limitations = append(block.Limitations, *lim)
				if block.State == BlockReady {
					block.State = BlockPartial
				}
			}
		}
		if !computed && len(block.Limitations) == 0 {
			block.State = BlockReadyEmpty
		}
		_ = accountSpendPrev
		feed.Campaigns[i].Exposure = block
	}
}

func deriveCampaignDetailBlocks(campaigns []CampaignLandscape, extras map[int64]CampaignDetailExtras, profile ExecutableProfileV3) {
	if !profile.HasBlock("campaign.stock") {
		return
	}
	for i := range campaigns {
		if campaigns[i].Stock != nil {
			continue
		}
		ex, ok := extras[campaigns[i].Identity.CampaignID]
		if !ok {
			campaigns[i].Stock = &CampaignStockBlock{State: BlockReadyEmpty}
			continue
		}
		campaigns[i].Stock = buildCampaignStockBlock(ex.AssignedTotal, ex.AssignedSKUs, ex.ContextTruncated, ex.Stocks)
	}
}

func buildCampaignStockBlock(assignedTotal int64, contextSKUs []int64, truncated bool, stocks map[int64]factualapi.SKUStock) *CampaignStockBlock {
	if assignedTotal <= 0 && len(contextSKUs) == 0 {
		return &CampaignStockBlock{State: BlockReadyEmpty}
	}
	if assignedTotal <= 0 {
		assignedTotal = int64(len(contextSKUs))
	}
	contextN := int64(len(contextSKUs))
	knownStock := int64(0)
	zeroStock := int64(0)
	block := &CampaignStockBlock{
		State:                         BlockReady,
		AssignedSKUCount:              &assignedTotal,
		ContextSKUCount:               &contextN,
		ContextTruncated:              truncated,
		ContextSKUWithKnownStockCount: &knownStock,
		ContextSKUWithZeroStockCount:  &zeroStock,
	}
	for _, skuID := range contextSKUs {
		st, ok := stocks[skuID]
		if !ok {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{
				Code: "CAMPAIGN_STOCK_SKU_MISSING", Message: "context SKU stock row not found",
			})
			continue
		}
		u := intFromValue(st.SellableUnits)
		if u == nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{
				Code: "CAMPAIGN_STOCK_SKU_UNKNOWN", Message: "context SKU sellable units unknown",
			})
			continue
		}
		knownStock++
		if *u == 0 {
			zeroStock++
		}
	}
	block.ContextSKUWithKnownStockCount = &knownStock
	block.ContextSKUWithZeroStockCount = &zeroStock
	if truncated {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{
			Code: "CAMPAIGN_STOCK_BOUNDED_CONTEXT", Message: "stock is computed on bounded SKU context, not the full assigned set",
		})
	}
	if assignedTotal > 0 && knownStock == 0 {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{
			Code: "CAMPAIGN_STOCK_NO_KNOWN_SKU_STOCK", Message: "assigned SKUs exist but no known stock facts",
		})
	}
	return block
}

func shareOfAccount(numerator, denominator *MoneyAmount) (*canonicaljson.Decimal, *Limitation) {
	if numerator == nil {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_NUMERATOR_UNKNOWN", Message: "campaign amount unknown"}
	}
	if denominator == nil {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_DENOMINATOR_UNKNOWN", Message: "account denominator unknown"}
	}
	num, err := parseDecLocal(numerator.Amount)
	if err != nil {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_NUMERATOR_INVALID", Message: "campaign amount invalid"}
	}
	den, err := parseDecLocal(denominator.Amount)
	if err != nil {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_DENOMINATOR_INVALID", Message: "account denominator invalid"}
	}
	if numerator.CurrencyCode != "" && denominator.CurrencyCode != "" && numerator.CurrencyCode != denominator.CurrencyCode {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_CURRENCY_MISMATCH", Message: "campaign/account currency mismatch"}
	}
	if den.Sign() == 0 {
		return nil, &Limitation{Code: "CAMPAIGN_EXPOSURE_NO_DENOMINATOR", Message: "account denominator is zero"}
	}
	share := new(big.Rat).Quo(num, den)
	share.Mul(share, big.NewRat(100, 1))
	d := canonicaljson.Decimal(formatRatLocal(share))
	return &d, nil
}

func moneyComparisonCurrent(m MoneyComparison) *MoneyAmount {
	return m.Current
}

func moneyComparisonPrevious(m MoneyComparison) *MoneyAmount {
	return m.Previous
}
