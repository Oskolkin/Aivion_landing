package landscape

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// UnmappedCategoryID is the Landscape Category identity for the UNMAPPED reconciliation bucket.
const UnmappedCategoryID int64 = 0

// FactualAPISource is a thin CoreDataSource that reads only through factualapi.API.
type FactualAPISource struct {
	API factualapi.API
}

func (s FactualAPISource) LoadCoreFeed(ctx context.Context, sealed factualapi.FactualReadContext, universeID, universeHash string, profiles CoreProfileBundleV3) (CoreFeed, error) {
	if s.API == nil {
		return CoreFeed{}, fmt.Errorf("landscape: FactualAPISource requires factualapi.API")
	}
	if sealed.Mode() != factualapi.ModeSealedUniverse {
		return CoreFeed{}, fmt.Errorf("landscape: sealed FactualReadContext required")
	}
	if profiles.IsZero() {
		profiles = DefaultCoreProfilesV5()
	}
	if err := ValidateCoreProfileCapabilitiesV3(sealed, profiles); err != nil {
		return CoreFeed{}, err
	}
	_ = universeID
	_ = universeHash

	feed := CoreFeed{
		GlobalCapabilities: mapCapabilityFlags(sealed.CapabilityBindings()),
		Account: AccountLandscape{
			Profile:  profiles.Account.Name,
			Identity: AccountIdentity{SellerAccountID: sealed.SellerAccountID()},
		},
	}

	acct, err := s.API.GetAccount(ctx, sealed)
	if err != nil {
		return CoreFeed{}, err
	}
	feed.Account.Identity.SellerAccountID = acct.SellerAccountID

	windows, err := buildDomainWindows(sealed)
	if err != nil {
		return CoreFeed{}, err
	}
	feed.DomainWindows = windows

	cats, err := s.listAllCategories(ctx, sealed)
	if err != nil {
		return CoreFeed{}, err
	}
	var catDemandCur []factualapi.CategoryDemand
	var catDemandCurErr error
	if profiles.Category.HasBlock("category.sales") {
		catDemandCur, catDemandCurErr = s.listAllCategoryDemand(ctx, sealed, factualapi.PeriodCurrent7D)
		if catDemandCurErr != nil {
			feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "CATEGORY_DEMAND_CUR_UNAVAILABLE", Message: catDemandCurErr.Error()})
			catDemandCur = nil
		}
	}
	var catDemandPrev []factualapi.CategoryDemand
	var catDemandPrevErr error
	if profiles.Category.HasBlock("category.sales") {
		catDemandPrev, catDemandPrevErr = s.listAllCategoryDemand(ctx, sealed, factualapi.PeriodPrevious7D)
		if catDemandPrevErr != nil {
			feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "CATEGORY_DEMAND_PREV_UNAVAILABLE", Message: catDemandPrevErr.Error()})
			catDemandPrev = nil
		}
	}

	skus, err := s.listAllSKUs(ctx, sealed)
	if err != nil {
		return CoreFeed{}, err
	}
	ids := make([]int64, 0, len(skus))
	skuByID := map[int64]factualapi.SKU{}
	for _, sk := range skus {
		ids = append(ids, sk.ID)
		skuByID[sk.ID] = sk
	}

	skuProf := profiles.SKU

	var demandCur, demandPrev, demand30 map[int64]factualapi.SKUDemand
	if skuProf.HasBlock("sku.sales") {
		var err error
		demandCur, err = s.batchSKUDemand(ctx, sealed, ids, factualapi.PeriodCurrent7D)
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: BatchGetSKUDemand current: %w", err)
		}
		demandPrev, err = s.batchSKUDemand(ctx, sealed, ids, factualapi.PeriodPrevious7D)
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: BatchGetSKUDemand previous: %w", err)
		}
		if profileHasPeriodV3(skuProf, "sku.sales", "TREND_30D") {
			demand30, err = s.batchSKUDemand(ctx, sealed, ids, factualapi.PeriodTrend30D)
			if err != nil {
				feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "SKU_DEMAND_30D_UNAVAILABLE", Message: err.Error()})
				demand30 = nil
			}
		}
	}

	var stocks map[int64]factualapi.SKUStock
	if skuProf.HasBlock("sku.stock") {
		var err error
		stocks, err = s.batchSKUStock(ctx, sealed, ids)
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: BatchGetSKUStock: %w", err)
		}
	}

	var prices map[int64]factualapi.SKUPrice
	if skuProf.HasBlock("sku.price") {
		var err error
		prices, err = s.batchSKUPrice(ctx, sealed, ids)
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: BatchGetSKUPrice: %w", err)
		}
	}

	var priceChanges map[int64]factualapi.SKUPriceLastObservedChange
	var priceChangeErr error
	priceHistoryFetch, err := profileBlockExecutes(sealed, skuProf, "sku.price.history")
	if err != nil {
		return CoreFeed{}, err
	}
	if priceHistoryFetch {
		priceChanges, priceChangeErr = s.batchSKUPriceLastChange(ctx, sealed, ids)
	}

	var searchCur, searchPrev map[int64]factualapi.SKUSearchSummary
	var searchCurErr, searchPrevErr error
	searchFetch, err := profileBlockExecutes(sealed, skuProf, "sku.search")
	if err != nil {
		return CoreFeed{}, err
	}
	if searchFetch {
		searchPair, pairErr := ResolveFactFamilyPeriodPair(sealed, "SEARCH", "SEARCH.SELLER_SUMMARY")
		if pairErr != nil {
			feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "SKU_SEARCH_PERIOD_UNAVAILABLE", Message: pairErr.Error()})
			searchCur = map[int64]factualapi.SKUSearchSummary{}
			searchPrev = map[int64]factualapi.SKUSearchSummary{}
		} else {
			searchCur, searchCurErr = s.batchSKUSearch(ctx, sealed, ids, "", &searchPair.Current)
			if searchCurErr != nil {
				feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "SKU_SEARCH_CUR_UNAVAILABLE", Message: searchCurErr.Error()})
				searchCur = map[int64]factualapi.SKUSearchSummary{}
			}
			searchPrev, searchPrevErr = s.batchSKUSearch(ctx, sealed, ids, "", &searchPair.Previous)
			if searchPrevErr != nil {
				feed.GlobalLimitations = append(feed.GlobalLimitations, Limitation{Code: "SKU_SEARCH_PREV_UNAVAILABLE", Message: searchPrevErr.Error()})
				searchPrev = map[int64]factualapi.SKUSearchSummary{}
			}
		}
	}

	var skuTrafficCur, skuTrafficPrev map[int64]factualapi.SKUAdvertisingTraffic
	var skuTrafficCurErr, skuTrafficPrevErr error
	trafficSKUSpec, trafficSKUDecision, trafficSKUPresent, err := ProfileBlockDecision(sealed, skuProf, "sku.advertising.traffic")
	if err != nil {
		return CoreFeed{}, err
	}
	if trafficSKUPresent && trafficSKUDecision.Execute {
		curPeriod, perr := periodFromBlock(sealed, trafficSKUSpec, "CURRENT_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: sku traffic current period: %w", perr)
		}
		prevPeriod, perr := periodFromBlock(sealed, trafficSKUSpec, "PREVIOUS_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: sku traffic previous period: %w", perr)
		}
		skuTrafficCur, skuTrafficCurErr = s.batchSKUTraffic(ctx, sealed, ids, "", &curPeriod)
		skuTrafficPrev, skuTrafficPrevErr = s.batchSKUTraffic(ctx, sealed, ids, "", &prevPeriod)
	}

	var skuAttrCur, skuAttrPrev map[int64]factualapi.SKUAdvertisingAttribution
	var skuAttrCurErr, skuAttrPrevErr error
	attrSKUSpec, attrSKUDecision, attrSKUPresent, err := ProfileBlockDecision(sealed, skuProf, "sku.advertising.attribution")
	if err != nil {
		return CoreFeed{}, err
	}
	if attrSKUPresent && attrSKUDecision.Execute {
		curPeriod, perr := periodFromBlock(sealed, attrSKUSpec, "CURRENT_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: sku attribution current period: %w", perr)
		}
		prevPeriod, perr := periodFromBlock(sealed, attrSKUSpec, "PREVIOUS_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: sku attribution previous period: %w", perr)
		}
		skuAttrCur, skuAttrCurErr = s.batchSKUAttribution(ctx, sealed, ids, "", &curPeriod)
		skuAttrPrev, skuAttrPrevErr = s.batchSKUAttribution(ctx, sealed, ids, "", &prevPeriod)
	}
	adsFetch := (trafficSKUPresent && trafficSKUDecision.Execute) || (attrSKUPresent && attrSKUDecision.Execute)

	var inbound map[int64]factualapi.SKUInbound
	var inboundErr error
	supplyFetch, err := profileBlockExecutes(sealed, skuProf, "sku.supply")
	if err != nil {
		return CoreFeed{}, err
	}
	if supplyFetch {
		inbound, inboundErr = s.batchSKUInbound(ctx, sealed, ids)
	}

	var coverage map[int64]factualapi.StockCoverage
	var coverageErr error
	coverageFetch, err := profileBlockExecutes(sealed, skuProf, "sku.stock.coverage")
	if err != nil {
		return CoreFeed{}, err
	}
	if coverageFetch {
		coverage, coverageErr = s.batchStockCoverage(ctx, sealed, ids)
	}

	var promoSummaries map[int64]factualapi.SKUPromotionSummary
	var promoErr error
	promoFetch, err := profileBlockExecutes(sealed, skuProf, "sku.promotions")
	if err != nil {
		return CoreFeed{}, err
	}
	if promoFetch {
		promoSummaries, promoErr = s.batchSKUPromotionSummary(ctx, sealed, ids)
	}

	camps, err := s.listAllCampaigns(ctx, sealed)
	if err != nil {
		return CoreFeed{}, err
	}
	campIDs := make([]int64, 0, len(camps))
	for _, c := range camps {
		campIDs = append(campIDs, c.ID)
	}
	campProf := profiles.Campaign
	var campaignTrafficCur, campaignTrafficPrev map[int64]factualapi.CampaignTrafficPerformance
	var campTrafficCurErr, campTrafficPrevErr error
	trafficSpec, trafficDecision, trafficPresent, err := ProfileBlockDecision(sealed, campProf, "campaign.traffic")
	if err != nil {
		return CoreFeed{}, err
	}
	if trafficPresent && trafficDecision.Execute {
		trafficCurPeriod, perr := periodFromBlock(sealed, trafficSpec, "CURRENT_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: campaign traffic current period: %w", perr)
		}
		trafficPrevPeriod, perr := periodFromBlock(sealed, trafficSpec, "PREVIOUS_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: campaign traffic previous period: %w", perr)
		}
		campaignTrafficCur, campTrafficCurErr = s.batchCampaignTraffic(ctx, sealed, campIDs, "", &trafficCurPeriod)
		campaignTrafficPrev, campTrafficPrevErr = s.batchCampaignTraffic(ctx, sealed, campIDs, "", &trafficPrevPeriod)
	}
	var campaignAttrCur, campaignAttrPrev map[int64]factualapi.CampaignAttributionPerformance
	var campAttrCurErr, campAttrPrevErr error
	attrSpec, attrDecision, attrPresent, err := ProfileBlockDecision(sealed, campProf, "campaign.attribution")
	if err != nil {
		return CoreFeed{}, err
	}
	if attrPresent && attrDecision.Execute {
		attrCurPeriod, perr := periodFromBlock(sealed, attrSpec, "CURRENT_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: campaign attribution current period: %w", perr)
		}
		attrPrevPeriod, perr := periodFromBlock(sealed, attrSpec, "PREVIOUS_7D")
		if perr != nil {
			return CoreFeed{}, fmt.Errorf("landscape: campaign attribution previous period: %w", perr)
		}
		campaignAttrCur, campAttrCurErr = s.batchCampaignAttribution(ctx, sealed, campIDs, "", &attrCurPeriod)
		campaignAttrPrev, campAttrPrevErr = s.batchCampaignAttribution(ctx, sealed, campIDs, "", &attrPrevPeriod)
	}
	var campTargeting map[int64]factualapi.CampaignTargetingSummary
	if targetingSpec, targetingDecision, targetingPresent, err := ProfileBlockDecision(sealed, campProf, "campaign.targeting"); err != nil {
		return CoreFeed{}, err
	} else if targetingPresent && targetingDecision.Execute {
		var targetingPeriod factualapi.Period
		targetingPeriod, err = periodFromBlock(sealed, targetingSpec, "CURRENT_7D")
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: campaign targeting period: %w", err)
		}
		campTargeting, err = s.batchCampaignTargetingSummary(ctx, sealed, campIDs, "", &targetingPeriod)
		if err != nil {
			return CoreFeed{}, fmt.Errorf("landscape: BatchGetCampaignTargetingSummary: %w", err)
		}
	}

	// Build SKU landscapes then filter by eligibility.
	type skuFacts struct {
		sl SKULandscape
		in SKUEligibilityInput
	}
	skuFactsList := make([]skuFacts, 0, len(ids))
	for _, id := range ids {
		sk := skuByID[id]
		sl := SKULandscape{
			Profile: profiles.SKU.Name,
			Identity: SKUIdentity{
				CanonicalSKUID: sk.ID, ProviderSKU: sk.OzonSKU, OfferID: sk.OfferID, Name: sk.Name,
				CatalogState: sk.CatalogState, CategoryID: sk.Category.CategoryID,
				Unmapped: sk.Category.MappingStatus == "UNMAPPED" || sk.Category.CategoryID == nil,
			},
		}
		elig := SKUEligibilityInput{CatalogState: sk.CatalogState}

		if skuProf.HasBlock("sku.sales") {
			var curPtr, prevPtr, trendPtr *factualapi.SKUDemand
			cur, curOK := demandCur[id]
			if curOK {
				cp := cur
				curPtr = &cp
				elig.SalesUnitsCur = intFromValue(cp.Demand.OrderedUnits)
				elig.SalesAmountCurKnown = moneyFromValue(cp.Demand.OrderedAmount) != nil
			}
			prev, prevOK := demandPrev[id]
			if prevOK {
				pp := prev
				prevPtr = &pp
				elig.SalesUnitsPrev = intFromValue(pp.Demand.OrderedUnits)
				elig.SalesAmountPrevKnown = moneyFromValue(pp.Demand.OrderedAmount) != nil
			}
			trend, trendOK := demand30[id]
			if trendOK {
				tp := trend
				trendPtr = &tp
			}
			sl.Sales = skuSalesBlockFromDemand(curPtr, prevPtr, trendPtr, curOK, prevOK, trendOK)
		}

		if skuProf.HasBlock("sku.stock") {
			if st, ok := stocks[id]; ok {
				if u := intFromValue(st.SellableUnits); u != nil {
					zero := *u == 0
					sl.Stock = &SKUStockBlock{
						State: BlockReady, SellableUnits: u, SellableStockIsZero: &zero,
						StockSnapshotAsOf: timePtr(st.SnapshotAt),
					}
					elig.SellableUnits = u
				} else {
					sl.Stock = &SKUStockBlock{State: BlockPartial, Limitations: []Limitation{{
						Code: "SELLABLE_UNITS_UNKNOWN", Message: "stock row found but sellable units not known",
					}}}
				}
			} else {
				sl.Stock = &SKUStockBlock{State: BlockReadyEmpty}
			}
		}

		if skuProf.HasBlock("sku.stock.coverage") {
			if cov, ok := coverage[id]; ok {
				if sl.Stock == nil {
					sl.Stock = &SKUStockBlock{State: BlockReady}
				}
				sl.Stock.CoverageDays7D = decFromValue(cov.CoverageDays7D)
				sl.Stock.CoverageDays30D = decFromValue(cov.CoverageDays30D)
				if coverageErr != nil {
					sl.Stock.Limitations = append(sl.Stock.Limitations, Limitation{Code: "STOCK_COVERAGE_PARTIAL", Message: coverageErr.Error()})
				}
			} else if coverageErr != nil {
				if sl.Stock == nil {
					sl.Stock = &SKUStockBlock{State: BlockUnavailable, Limitations: []Limitation{{
						Code: "STOCK_COVERAGE_UNAVAILABLE", Message: coverageErr.Error(),
					}}}
				} else {
					sl.Stock.Limitations = append(sl.Stock.Limitations, Limitation{Code: "STOCK_COVERAGE_UNAVAILABLE", Message: coverageErr.Error()})
				}
			} else {
				if sl.Stock == nil {
					sl.Stock = &SKUStockBlock{State: BlockPartial}
				} else if sl.Stock.State == BlockReady || sl.Stock.State == BlockReadyEmpty {
					sl.Stock.State = BlockPartial
				}
				sl.Stock.Limitations = append(sl.Stock.Limitations, Limitation{
					Code: "STOCK_COVERAGE_MISSING", Message: "stock coverage row not found for SKU",
				})
			}
		}

		if skuProf.HasBlock("sku.price") || skuProf.HasBlock("sku.price.history") {
			sl.Price = buildSKUPriceBlock(id, prices, priceChanges, priceChangeErr)
		}

		if adsFetch {
			sl.Advertising = buildSKUAdsBlockV4(
				id,
				skuTrafficCur, skuTrafficPrev, skuTrafficCurErr, skuTrafficPrevErr, trafficSKUPresent && trafficSKUDecision.Execute,
				skuAttrCur, skuAttrPrev, skuAttrCurErr, skuAttrPrevErr, attrSKUPresent && attrSKUDecision.Execute,
			)
			if skuTrafficCurErr == nil {
				if a, ok := skuTrafficCur[id]; ok {
					k, _ := moneyKnownPositive(a.PaidSpend)
					elig.AdsSpendCurKnown = k
				}
			}
			if skuTrafficPrevErr == nil {
				if a, ok := skuTrafficPrev[id]; ok {
					k, _ := moneyKnownPositive(a.PaidSpend)
					elig.AdsSpendPrevKnown = k
				}
			}
			if skuAttrCurErr == nil {
				if a, ok := skuAttrCur[id]; ok {
					k, _ := moneyKnownPositive(a.AttributedOrderAmount)
					elig.AdsAttrCurKnown = k
				}
			}
			if skuAttrPrevErr == nil {
				if a, ok := skuAttrPrev[id]; ok {
					k, _ := moneyKnownPositive(a.AttributedOrderAmount)
					elig.AdsAttrPrevKnown = k
				}
			}
		}

		if searchFetch {
			sl.Search = buildSKUSearchBlock(id, searchCur, searchPrev, searchCurErr, searchPrevErr)
		}

		if supplyFetch {
			sl.Supply = buildSKUSupplyBlock(id, inbound, inboundErr)
			if ib, ok := inbound[id]; ok {
				elig.OpenInboundUnits = intFromValue(ib.DeclaredInboundUnits)
				n := int64(len(ib.Supplies))
				elig.OpenSupplyCount = &n
			}
		}

		if promoFetch {
			sl.Promotions = buildSKUPromotionsBlock(id, promoSummaries, promoErr)
		}

		skuFactsList = append(skuFactsList, skuFacts{sl: sl, in: elig})
	}

	eligibleSKUs := make([]SKULandscape, 0, len(skuFactsList))
	for _, sf := range skuFactsList {
		if SKUEligible(sf.in) {
			eligibleSKUs = append(eligibleSKUs, sf.sl)
		}
	}
	feed.SKUs = eligibleSKUs

	// Categories: leaf + UNMAPPED reconciliation bucket.
	feed.Categories = buildCategoryLandscapes(profiles.Category.Name, cats, catDemandCur, catDemandPrev, catDemandCurErr, catDemandPrevErr, feed.SKUs, stocks, inbound, profiles.Category)

	// Campaigns with eligibility + performance.
	eligibleCamps := make([]CampaignLandscape, 0, len(camps))
	for _, c := range camps {
		cl := CampaignLandscape{
			Profile: profiles.Campaign.Name,
			Identity: CampaignIdentity{
				CampaignID: c.ID, Title: c.Title, ProviderState: c.ProviderState,
				PaymentType: c.PaymentType, AdvObjectType: c.AdvObjectType, TargetScope: c.TargetScopeType,
			},
		}
		cl.Performance = buildCampaignPerfBlockV4(
			c.ID,
			campaignTrafficCur, campaignTrafficPrev, campTrafficCurErr, campTrafficPrevErr,
			trafficPresent, trafficDecision,
			campaignAttrCur, campaignAttrPrev, campAttrCurErr, campAttrPrevErr,
			attrPresent, attrDecision,
		)
		if _, targetingDecision, targetingPresent, err := ProfileBlockDecision(sealed, campProf, "campaign.targeting"); err != nil {
			return CoreFeed{}, err
		} else if targetingPresent && targetingDecision.Execute {
			sum := campTargeting[c.ID]
			cl.Targeting = buildCampaignTargetingFromSummary(&sum, c.TargetScopeType)
			if _, ok := campTargeting[c.ID]; !ok {
				cl.Targeting = buildCampaignTargetingFromSummary(nil, c.TargetScopeType)
			}
		}
		if campProf.HasBlock("campaign.exposure") {
			cl.Exposure = nil
		}
		ein := CampaignEligibilityInput{ProviderState: c.ProviderState}
		if p, ok := campaignTrafficCur[c.ID]; ok {
			k, pos := moneyKnownPositive(p.PaidSpend)
			ein.SpendCurKnown, ein.SpendCurPositive = k, pos
		}
		if p, ok := campaignTrafficPrev[c.ID]; ok {
			k, pos := moneyKnownPositive(p.PaidSpend)
			ein.SpendPrevKnown, ein.SpendPrevPositive = k, pos
		}
		if p, ok := campaignAttrCur[c.ID]; ok {
			k, pos := moneyKnownPositive(p.AttributedOrderAmount)
			ein.AttrCurKnown, ein.AttrCurPositive = k, pos
		}
		if p, ok := campaignAttrPrev[c.ID]; ok {
			k, pos := moneyKnownPositive(p.AttributedOrderAmount)
			ein.AttrPrevKnown, ein.AttrPrevPositive = k, pos
		}
		if CampaignEligible(ein) {
			eligibleCamps = append(eligibleCamps, cl)
		}
	}
	feed.Campaigns = eligibleCamps

	if err := s.loadAccountDirectBlocks(ctx, sealed, &feed, profiles.Account); err != nil {
		return CoreFeed{}, err
	}
	deriveAccountBlocks(&feed, profiles.Account, sealed)
	deriveCampaignCoreBlocks(&feed, profiles.Campaign)
	stampAdvertisingPeriodsFromWindows(&feed)
	return feed, nil
}

// BuildCore assembles a Core LandscapeSet using executable V3 profiles (Prompt №9.4).
func (s FactualAPISource) BuildCore(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	universeID, universeHash, contractBundleHash string,
	profiles CoreProfileBundleV3,
) (LandscapeSet, []byte, string, error) {
	if profiles.IsZero() {
		profiles = DefaultCoreProfilesV5()
	}
	return CoreBuilder{Source: s, Profiles: profiles}.BuildCore(ctx, sealed, universeID, universeHash, contractBundleHash)
}

func profileHasPeriodV3(profile ExecutableProfileV3, blockID, period string) bool {
	spec, ok := profile.blockByID(blockID)
	if !ok {
		return false
	}
	for _, p := range spec.Periods {
		if p.Name == period {
			return true
		}
	}
	return false
}

func blockLimitationsFromDecision(d BlockExecutionDecision) []Limitation {
	if d.Limitation == nil {
		return nil
	}
	return []Limitation{*d.Limitation}
}

func blockStateFromDecision(d BlockExecutionDecision) BlockState {
	if d.State != "" {
		return d.State
	}
	return BlockUnavailable
}

func periodFromBlock(sealed factualapi.FactualReadContext, spec BlockSpecV3, name string) (factualapi.Period, error) {
	periods, err := ResolveBlockPeriodsV3(sealed, spec)
	if err != nil {
		return factualapi.Period{}, err
	}
	p, ok := periods[name]
	if !ok {
		return factualapi.Period{}, fmt.Errorf("period %s not configured for block %s", name, spec.BlockID)
	}
	return p, nil
}

func mapCapabilityFlags(bindings []factualapi.CapabilityBinding) []CapabilityFlag {
	out := make([]CapabilityFlag, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, CapabilityFlag{
			Key: b.CapabilityKey, Scope: b.Scope, FinalState: b.FinalState, ReasonCode: b.ReasonCode,
		})
	}
	return out
}

func buildDomainWindows(sealed factualapi.FactualReadContext) ([]DomainWindow, error) {
	salesCur, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodCurrent7D, nil)
	if err != nil {
		return nil, err
	}
	salesPrev, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodPrevious7D, nil)
	if err != nil {
		return nil, err
	}
	out := []DomainWindow{{
		Domain: "sales",
		Current: &DateRange{
			From: canonicaljson.Date(salesCur.From), To: canonicaljson.Date(salesCur.To),
		},
		Previous: &DateRange{
			From: canonicaljson.Date(salesPrev.From), To: canonicaljson.Date(salesPrev.To),
		},
		AlignedToAnalysisDate: true,
	}}

	if trafficPair, err := ResolveFactFamilyPeriodPair(sealed, "ADVERTISING", "ADVERTISING.TRAFFIC"); err == nil {
		lag := lagDaysForFactFamily(sealed, "ADVERTISING", "ADVERTISING.TRAFFIC")
		out = append(out, DomainWindow{
			Domain:                "advertising",
			FactFamily:            "ADVERTISING.TRAFFIC",
			Current:               &DateRange{From: canonicaljson.Date(trafficPair.Current.From), To: canonicaljson.Date(trafficPair.Current.To)},
			Previous:              &DateRange{From: canonicaljson.Date(trafficPair.Previous.From), To: canonicaljson.Date(trafficPair.Previous.To)},
			AlignedToAnalysisDate: lag == nil || *lag == 0,
			LagDays:               lag,
		})
	}
	if attrPair, err := ResolveFactFamilyPeriodPair(sealed, "ADVERTISING", "ADVERTISING.ATTRIBUTION"); err == nil {
		lag := lagDaysForFactFamily(sealed, "ADVERTISING", "ADVERTISING.ATTRIBUTION")
		out = append(out, DomainWindow{
			Domain:                "advertising",
			FactFamily:            "ADVERTISING.ATTRIBUTION",
			Current:               &DateRange{From: canonicaljson.Date(attrPair.Current.From), To: canonicaljson.Date(attrPair.Current.To)},
			Previous:              &DateRange{From: canonicaljson.Date(attrPair.Previous.From), To: canonicaljson.Date(attrPair.Previous.To)},
			AlignedToAnalysisDate: lag == nil || *lag == 0,
			LagDays:               lag,
		})
	}

	searchWin, searchLag := laggedPeriodWindow(sealed, "SEARCH", "SEARCH.SELLER_SUMMARY")
	out = append(out, DomainWindow{
		Domain:                "search",
		Current:               searchWin.Current,
		Previous:              searchWin.Previous,
		AlignedToAnalysisDate: searchLag == nil || *searchLag == 0,
		LagDays:               searchLag,
	})

	cutoffDay := sealed.CutoffAt().UTC()
	asOf := canonicaljson.Date(time.Date(cutoffDay.Year(), cutoffDay.Month(), cutoffDay.Day(), 0, 0, 0, 0, time.UTC))
	out = append(out, DomainWindow{
		Domain:                "stock",
		Current:               &DateRange{From: asOf, To: asOf},
		AlignedToAnalysisDate: true,
	})

	if _, ok := sealed.Binding("FINANCE", "FINANCE.DAILY_LEDGER"); ok || capabilityPresent(sealed, "FINANCE.ACCOUNT_NET_ACCRUAL") {
		finWin, finLag := laggedPeriodWindow(sealed, "FINANCE", "FINANCE.DAILY_LEDGER")
		out = append(out, DomainWindow{
			Domain:                "finance",
			Current:               finWin.Current,
			Previous:              finWin.Previous,
			AlignedToAnalysisDate: finLag == nil || *finLag == 0,
			LagDays:               finLag,
		})
	}
	return out, nil
}

type periodPair struct {
	Current, Previous *DateRange
}

func laggedPeriodWindow(sealed factualapi.FactualReadContext, domain, factFamily string) (periodPair, *int64) {
	lag := lagDaysForFactFamily(sealed, domain, factFamily)
	anchor := sealed.AnalysisDate()
	if lag != nil && *lag > 0 {
		anchor = anchor.AddDate(0, 0, -int(*lag))
	}
	curFrom, curTo := anchor.AddDate(0, 0, -6), anchor
	prevFrom, prevTo := anchor.AddDate(0, 0, -13), anchor.AddDate(0, 0, -7)
	return periodPair{
		Current:  &DateRange{From: canonicaljson.Date(curFrom), To: canonicaljson.Date(curTo)},
		Previous: &DateRange{From: canonicaljson.Date(prevFrom), To: canonicaljson.Date(prevTo)},
	}, lag
}

func lagDaysForFactFamily(sealed factualapi.FactualReadContext, domain, factFamily string) *int64 {
	b, ok := sealed.Binding(domain, factFamily)
	if !ok || b.CoverageTo == nil {
		return nil
	}
	through := time.Date(b.CoverageTo.Year(), b.CoverageTo.Month(), b.CoverageTo.Day(), 0, 0, 0, 0, time.UTC)
	d := sealed.AnalysisDate()
	if through.Before(d) {
		days := int64(d.Sub(through).Hours() / 24)
		if days > 0 {
			return &days
		}
	}
	return nil
}

func capabilityPresent(sealed factualapi.FactualReadContext, key string) bool {
	_, ok := sealed.CapabilityBinding(key, "HISTORICAL_QUERY")
	return ok
}

func profileBlockExecutes(sealed factualapi.FactualReadContext, profile ExecutableProfileV3, blockID string) (bool, error) {
	_, dec, present, err := ProfileBlockDecision(sealed, profile, blockID)
	if err != nil || !present {
		return false, err
	}
	return dec.Execute, nil
}

func capabilityUsable(sealed factualapi.FactualReadContext, key string) (usable bool, state string, reason string) {
	b, ok := sealed.CapabilityBinding(key, "HISTORICAL_QUERY")
	if !ok {
		return false, "MISSING", "CAPABILITY_NOT_BOUND"
	}
	switch b.FinalState {
	case "READY", "PARTIAL":
		return true, b.FinalState, b.ReasonCode
	case "STALE":
		return false, b.FinalState, nz(b.ReasonCode, "CAPABILITY_STALE")
	default:
		return false, b.FinalState, nz(b.ReasonCode, "CAPABILITY_NOT_USABLE")
	}
}

func accountSalesCapability(sealed factualapi.FactualReadContext) (bool, string, string) {
	if ok, state, reason := capabilityUsable(sealed, "SALES.ACCOUNT_DEMAND"); ok {
		return true, state, reason
	}
	if b, ok := sealed.CapabilityBinding("SALES.ACCOUNT_DEMAND", "HISTORICAL_QUERY"); ok {
		return false, b.FinalState, nz(b.ReasonCode, "CAPABILITY_NOT_USABLE")
	}
	return false, "MISSING", "CAPABILITY_NOT_BOUND"
}

func unavailableBlock(code, msg string) []Limitation {
	return []Limitation{{Code: code, Message: msg}}
}

func (s FactualAPISource) loadAccountDirectBlocks(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed, profile ExecutableProfileV3) error {
	if _, decision, present, err := ProfileBlockDecision(sealed, profile, "account.sales"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountSales(ctx, sealed, feed, profile); err != nil {
			feed.Account.Sales = &AccountSalesBlock{State: BlockUnavailable, Limitations: unavailableBlock("SALES_UNAVAILABLE", err.Error())}
		}
	}

	if spec, decision, present, err := ProfileBlockDecision(sealed, profile, "account.finance"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountFinance(ctx, sealed, feed, spec); err != nil {
			feed.Account.Finance = &AccountFinanceBlock{State: BlockUnavailable, Limitations: unavailableBlock("FINANCE_UNAVAILABLE", err.Error())}
		}
	}

	if _, decision, present, err := ProfileBlockDecision(sealed, profile, "account.health"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountHealth(ctx, sealed, feed); err != nil {
			feed.Account.Health = &AccountHealthBlock{State: BlockUnavailable, Limitations: unavailableBlock("HEALTH_UNAVAILABLE", err.Error())}
		}
	}

	if _, decision, present, err := ProfileBlockDecision(sealed, profile, "account.supply"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountSupply(ctx, sealed, feed); err != nil {
			feed.Account.Supply = &AccountSupplyBlock{State: BlockUnavailable, Limitations: unavailableBlock("SUPPLY_UNAVAILABLE", err.Error())}
		}
	}

	if _, decision, present, err := ProfileBlockDecision(sealed, profile, "account.promotions"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountPromotions(ctx, sealed, feed); err != nil {
			feed.Account.Promotions = &AccountPromotionsBlock{State: BlockUnavailable, Limitations: unavailableBlock("PROMOTIONS_UNAVAILABLE", err.Error())}
		}
	}

	if spec, decision, present, err := ProfileBlockDecision(sealed, profile, "account.funnel"); err != nil {
		return err
	} else if present && decision.Execute {
		if err := s.fillAccountFunnel(ctx, sealed, feed, spec); err != nil {
			feed.Account.Funnel = &AccountFunnelBlock{State: BlockUnavailable, Limitations: unavailableBlock("FUNNEL_UNAVAILABLE", err.Error())}
		}
	}

	return nil
}

func capabilityUsableFirst(sealed factualapi.FactualReadContext, keys ...string) bool {
	for _, k := range keys {
		if ok, _, _ := capabilityUsable(sealed, k); ok {
			return true
		}
	}
	return false
}

func (s FactualAPISource) fillAccountSales(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed, profile ExecutableProfileV3) error {
	demand, err := s.API.GetAccountDemand(ctx, sealed, factualapi.PeriodCurrent7D, nil)
	if err != nil {
		return err
	}
	prev, prevErr := s.API.GetAccountDemand(ctx, sealed, factualapi.PeriodPrevious7D, nil)
	var prevPtr *factualapi.Demand
	if prevErr == nil {
		prevPtr = &prev
	}
	day, dayErr := s.API.GetAccountDemand(ctx, sealed, factualapi.PeriodAnalysisDay, nil)
	var dayPtr *factualapi.Demand
	if dayErr == nil {
		dayPtr = &day
	}
	prevDay, prevDayErr := s.API.GetAccountDemand(ctx, sealed, factualapi.PeriodPreviousDay, nil)
	var prevDayPtr *factualapi.Demand
	if prevDayErr == nil {
		prevDayPtr = &prevDay
	}
	includeTrend := profileHasPeriodV3(profile, "account.sales", "TREND_30D")
	var trendPtr *factualapi.Demand
	var trendErr error
	if includeTrend {
		trend, errTrend := s.API.GetAccountDemand(ctx, sealed, factualapi.PeriodTrend30D, nil)
		trendErr = errTrend
		if trendErr == nil {
			trendPtr = &trend
		}
	}
	feed.Account.Sales = accountSalesFromDemand(&demand, prevPtr, dayPtr, prevDayPtr, trendPtr, prevErr, dayErr, prevDayErr, trendErr, includeTrend)
	return nil
}

func (s FactualAPISource) fillAccountFinance(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed, spec BlockSpecV3) error {
	curPeriod, err := periodFromBlock(sealed, spec, "CURRENT_7D")
	if err != nil {
		return err
	}
	prevPeriod, err := periodFromBlock(sealed, spec, "PREVIOUS_7D")
	if err != nil {
		return err
	}
	cur, err := s.API.GetFinanceAccountSummary(ctx, sealed, "", &curPeriod)
	if err != nil {
		return err
	}
	prev, prevErr := s.API.GetFinanceAccountSummary(ctx, sealed, "", &prevPeriod)
	breakdown, breakdownErr := s.API.GetFinanceAccountBreakdown(ctx, sealed, "", &curPeriod)
	block := &AccountFinanceBlock{State: BlockReady}
	curAmt := moneyFromSigned(cur.AccountNet)
	var prevAmt *MoneyAmount
	if prevErr == nil {
		prevAmt = moneyFromSigned(prev.AccountNet)
	} else {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "FINANCE_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	block.NetAccrual = BuildMoneyComparison(curAmt, prevAmt)
	if breakdownErr == nil {
		block.MappingCoverage = decFromValue(breakdown.MappingCoverage)
	} else {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "FINANCE_BREAKDOWN_UNAVAILABLE", Message: breakdownErr.Error()})
	}
	feed.Account.Finance = block
	return nil
}

func (s FactualAPISource) fillAccountHealth(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed) error {
	h, err := s.API.GetAccountHealthSummary(ctx, sealed)
	if err != nil {
		return err
	}
	var okN, warnN, critN, badN, goodN int64
	for _, r := range h.Ratings {
		switch strings.ToUpper(r.ProviderStatus) {
		case "OK", "GREEN", "GOOD":
			okN++
		case "WARNING", "YELLOW", "WARN":
			warnN++
		case "CRITICAL", "RED", "BAD":
			critN++
		}
		switch strings.ToUpper(r.ProviderChangeMeaning) {
		case "BAD", "WORSE", "NEGATIVE":
			badN++
		case "GOOD", "BETTER", "POSITIVE":
			goodN++
		}
	}
	feed.Account.Health = &AccountHealthBlock{
		State: BlockReady, ProviderOKCount: &okN, ProviderWarningCount: &warnN,
		ProviderCriticalCount: &critN, ProviderBadChangesCount: &badN, ProviderGoodChangesCount: &goodN,
	}
	return nil
}

func (s FactualAPISource) fillAccountSupply(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed) error {
	summary, err := s.API.GetSupplyAccountSummary(ctx, sealed)
	if err != nil {
		return err
	}
	block := &AccountSupplyBlock{State: BlockReady}
	block.OpenSupplyOrderCount = intFromValue(summary.OpenSupplyOrderCount)
	block.SKUWithOpenInboundCount = intFromValue(summary.SKUWithOpenInboundCount)
	block.PendingAcceptanceActCount = intFromValue(summary.PendingAcceptanceActCount)
	block.OverdueSupplyCount = intFromValue(summary.OverdueSupplyCount)
	feed.Account.Supply = block
	return nil
}

func (s FactualAPISource) fillAccountPromotions(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed) error {
	summary, err := s.API.GetPromotionAccountSummary(ctx, sealed)
	if err != nil {
		return err
	}
	block := &AccountPromotionsBlock{State: BlockReady}
	block.ActiveProviderPromotionCount = intFromValue(summary.ActiveProviderPromotionCount)
	block.ActiveSellerPromotionCount = intFromValue(summary.ActiveSellerPromotionCount)
	block.ParticipatingSKUCount = intFromValue(summary.ParticipatingSKUCount)
	block.CandidateSKUCount = intFromValue(summary.CandidateSKUCount)
	block.SalesExposureParticipating = decFromValue(summary.SalesExposureParticipating)
	feed.Account.Promotions = block
	return nil
}

func (s FactualAPISource) fillAccountFunnel(ctx context.Context, sealed factualapi.FactualReadContext, feed *CoreFeed, spec BlockSpecV3) error {
	curPeriod, err := periodFromBlock(sealed, spec, "CURRENT_7D")
	if err != nil {
		return err
	}
	prevPeriod, err := periodFromBlock(sealed, spec, "PREVIOUS_7D")
	if err != nil {
		return err
	}
	search, err := s.buildAccountFunnelSurface(ctx, sealed, "SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION", curPeriod, prevPeriod)
	if err != nil {
		return err
	}
	pdp, err := s.buildAccountFunnelSurface(ctx, sealed, "PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION", curPeriod, prevPeriod)
	if err != nil {
		return err
	}
	var st FunnelBuildState
	collectAccountFunnelSurface(&st, search)
	collectAccountFunnelSurface(&st, pdp)
	state, lims := finalizeFunnelState(st)
	block := &AccountFunnelBlock{State: state, SearchSurface: search, PDPSurface: pdp, Limitations: lims}
	feed.Account.Funnel = block
	return nil
}

func collectAccountFunnelSurface(st *FunnelBuildState, surface *FunnelSurfaceBlock) {
	if surface == nil {
		st.MissingMetricCount += 3
		return
	}
	if len(surface.Limitations) > 0 {
		st.LimitedMetricCount += len(surface.Limitations)
		st.Limitations = append(st.Limitations, surface.Limitations...)
		return
	}
	if surface.Impressions != nil || surface.CartAdds != nil || surface.Conversion != nil {
		st.KnownMetricCount++
		return
	}
	st.MissingMetricCount++
}

func (s FactualAPISource) buildAccountFunnelSurface(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	impressionsKey, cartAddsKey, conversionKey string,
	curPeriod, prevPeriod factualapi.Period,
) (*FunnelSurfaceBlock, error) {
	surface := &FunnelSurfaceBlock{}
	var partial bool

	if impCur, impPrev, lim := s.loadAccountFunnelMetricPair(ctx, sealed, impressionsKey, curPeriod, prevPeriod); lim != nil {
		surface.Limitations = append(surface.Limitations, *lim)
		partial = true
	} else {
		cmp := BuildCountComparison(impCur, impPrev)
		surface.Impressions = &cmp
	}
	if cartCur, cartPrev, lim := s.loadAccountFunnelMetricPair(ctx, sealed, cartAddsKey, curPeriod, prevPeriod); lim != nil {
		surface.Limitations = append(surface.Limitations, *lim)
		partial = true
	} else {
		cmp := BuildCountComparison(cartCur, cartPrev)
		surface.CartAdds = &cmp
	}
	if convCur, convPrev, lim := s.loadAccountFunnelConversionPair(ctx, sealed, conversionKey, curPeriod, prevPeriod); lim != nil {
		surface.Limitations = append(surface.Limitations, *lim)
		partial = true
	} else if convCur != nil || convPrev != nil {
		cmp := BuildPctPointComparison(convCur, convPrev)
		surface.Conversion = &cmp
	}
	if surface.Impressions == nil && surface.CartAdds == nil && surface.Conversion == nil && len(surface.Limitations) == 0 {
		return nil, nil
	}
	if partial && len(surface.Limitations) > 0 {
		_ = partial
	}
	return surface, nil
}

func (s FactualAPISource) loadAccountFunnelMetricPair(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	metric string,
	curPeriod, prevPeriod factualapi.Period,
) (cur, prev *int64, lim *Limitation) {
	curRow, curErr := s.API.GetAccountFunnelPeriod(ctx, sealed, metric, "", &curPeriod)
	prevRow, prevErr := s.API.GetAccountFunnelPeriod(ctx, sealed, metric, "", &prevPeriod)
	if curErr != nil && prevErr != nil {
		return nil, nil, &Limitation{Code: "FUNNEL_METRIC_UNAVAILABLE", Message: curErr.Error()}
	}
	if curErr == nil {
		cur = decIntFromValue(curRow.Metric.Value)
	} else {
		lim = &Limitation{Code: "FUNNEL_CUR_UNAVAILABLE", Message: curErr.Error()}
	}
	if prevErr == nil {
		prev = decIntFromValue(prevRow.Metric.Value)
	} else if lim == nil {
		lim = &Limitation{Code: "FUNNEL_PREV_UNAVAILABLE", Message: prevErr.Error()}
	}
	return cur, prev, lim
}

func (s FactualAPISource) loadAccountFunnelConversionPair(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	metric string,
	curPeriod, prevPeriod factualapi.Period,
) (cur, prev *canonicaljson.Decimal, lim *Limitation) {
	curRow, curErr := s.API.GetAccountFunnelPeriod(ctx, sealed, metric, "", &curPeriod)
	prevRow, prevErr := s.API.GetAccountFunnelPeriod(ctx, sealed, metric, "", &prevPeriod)
	if curErr != nil && prevErr != nil {
		return nil, nil, &Limitation{Code: "FUNNEL_CONVERSION_UNAVAILABLE", Message: curErr.Error()}
	}
	if curErr == nil {
		cur = decFromValue(curRow.Metric.Value)
	} else {
		lim = &Limitation{Code: "FUNNEL_CONVERSION_CUR_UNAVAILABLE", Message: curErr.Error()}
	}
	if prevErr == nil {
		prev = decFromValue(prevRow.Metric.Value)
	} else if lim == nil {
		lim = &Limitation{Code: "FUNNEL_CONVERSION_PREV_UNAVAILABLE", Message: prevErr.Error()}
	}
	return cur, prev, lim
}

func buildSKUSearchBlock(
	id int64,
	cur, prev map[int64]factualapi.SKUSearchSummary,
	curErr, prevErr error,
) *SKUSearchBlock {
	if curErr != nil && prevErr != nil {
		return &SKUSearchBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_SEARCH_UNAVAILABLE", curErr.Error())}
	}
	ssCur, hasCur := cur[id]
	ssPrev, hasPrev := prev[id]
	if !hasCur && !hasPrev {
		block := &SKUSearchBlock{State: BlockReadyEmpty}
		if curErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "SKU_SEARCH_CUR_UNAVAILABLE", Message: curErr.Error()})
		}
		if prevErr != nil {
			block.State = BlockPartial
			block.Limitations = append(block.Limitations, Limitation{Code: "SKU_SEARCH_PREV_UNAVAILABLE", Message: prevErr.Error()})
		}
		if curErr != nil && prevErr == nil {
			block.State = BlockUnavailable
		}
		return block
	}
	block := &SKUSearchBlock{State: BlockReady}
	if hasCur {
		block.ProviderAveragePosition = decFromValue(ssCur.Position)
		block.ViewConversion = decFromValue(ssCur.ViewConversion)
		if u := decIntFromValue(ssCur.UniqueSearchUsers); u != nil {
			block.UniqueSearchUsers = u
		}
		if u := decIntFromValue(ssCur.UniqueViewUsers); u != nil {
			block.UniqueViewUsers = u
		}
	}
	if hasPrev {
		block.PreviousProviderAveragePosition = decFromValue(ssPrev.Position)
	}
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_SEARCH_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_SEARCH_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func buildSKUPriceBlock(
	id int64,
	prices map[int64]factualapi.SKUPrice,
	changes map[int64]factualapi.SKUPriceLastObservedChange,
	changeErr error,
) *SKUPriceBlock {
	pr, hasPrice := prices[id]
	if !hasPrice && changeErr != nil {
		return &SKUPriceBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_PRICE_UNAVAILABLE", changeErr.Error())}
	}
	if !hasPrice {
		return &SKUPriceBlock{State: BlockReadyEmpty}
	}
	block := &SKUPriceBlock{State: BlockReady, CurrentPrice: moneyFromValue(pr.ProviderCurrentPrice)}
	if block.CurrentPrice != nil && pr.CurrencyCode != "" {
		block.CurrentPrice.CurrencyCode = pr.CurrencyCode
	}
	if ch, ok := changes[id]; ok {
		curAmt := moneyFromValue(ch.CurrentPrice)
		prevAmt := moneyFromValue(ch.PreviousPrice)
		cmp := BuildMoneyComparison(curAmt, prevAmt)
		block.LastObservedChange = &cmp
		block.LastObservedChangeAt = timePtr(ch.ObservedAt)
	} else if changeErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "SKU_PRICE_HISTORY_PARTIAL", Message: changeErr.Error()})
	}
	return block
}

func buildSKUSupplyBlock(id int64, inbound map[int64]factualapi.SKUInbound, inboundErr error) *SKUSupplyBlock {
	if inboundErr != nil {
		return &SKUSupplyBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_INBOUND_UNAVAILABLE", inboundErr.Error())}
	}
	ib, ok := inbound[id]
	if !ok {
		return &SKUSupplyBlock{State: BlockReadyEmpty}
	}
	u := intFromValue(ib.DeclaredInboundUnits)
	openSupplies := int64(0)
	overdueSupplies := int64(0)
	seenSupply := map[int64]struct{}{}
	for _, item := range ib.Supplies {
		if _, dup := seenSupply[item.SupplyID]; dup {
			continue
		}
		seenSupply[item.SupplyID] = struct{}{}
		openSupplies++
		if strings.EqualFold(item.CanonicalState, "OVERDUE") {
			overdueSupplies++
		}
	}
	return &SKUSupplyBlock{
		State: BlockReady, OpenInboundDeclaredUnits: u, OpenSupplyCount: &openSupplies, OverdueSupplyCount: &overdueSupplies,
	}
}

func buildSKUPromotionsBlock(id int64, summaries map[int64]factualapi.SKUPromotionSummary, fetchErr error) *SKUPromotionsBlock {
	if fetchErr != nil {
		return &SKUPromotionsBlock{State: BlockUnavailable, Limitations: unavailableBlock("SKU_PROMOTIONS_UNAVAILABLE", fetchErr.Error())}
	}
	sum, ok := summaries[id]
	if !ok {
		return &SKUPromotionsBlock{State: BlockReadyEmpty}
	}
	return &SKUPromotionsBlock{
		State:                   BlockReady,
		ActivePromotionCount:    intFromValue(sum.ActivePromotionCount),
		CandidatePromotionCount: intFromValue(sum.CandidatePromotionCount),
	}
}

func aggregateCampaignPerfState(traffic *CampaignTrafficBlock, attribution *CampaignAttributionBlock) BlockState {
	states := []BlockState{}
	if traffic != nil && traffic.State != "" {
		states = append(states, traffic.State)
	}
	if attribution != nil && attribution.State != "" {
		states = append(states, attribution.State)
	}
	if len(states) == 0 {
		return BlockReadyEmpty
	}
	worst := states[0]
	for _, st := range states[1:] {
		if blockStateRank(st) > blockStateRank(worst) {
			worst = st
		}
	}
	return worst
}

func blockStateRank(st BlockState) int {
	switch st {
	case BlockUnavailable:
		return 4
	case BlockPartial:
		return 3
	case BlockReadyEmpty:
		return 2
	case BlockReady:
		return 1
	default:
		return 0
	}
}

func buildCategoryLandscapes(
	profileName string,
	cats []factualapi.Category,
	curDemand, prevDemand []factualapi.CategoryDemand,
	curDemandErr, prevDemandErr error,
	skus []SKULandscape,
	stocks map[int64]factualapi.SKUStock,
	inbound map[int64]factualapi.SKUInbound,
	profile ExecutableProfileV3,
) []CategoryLandscape {
	prevByID := map[int64]factualapi.CategoryDemand{}
	curByID := map[int64]factualapi.CategoryDemand{}
	var unmappedCur, unmappedPrev *factualapi.CategoryDemand
	for i := range curDemand {
		d := curDemand[i]
		if d.CategoryID == nil || d.Bucket == "UNMAPPED" {
			cp := d
			unmappedCur = &cp
			continue
		}
		curByID[*d.CategoryID] = d
	}
	for i := range prevDemand {
		d := prevDemand[i]
		if d.CategoryID == nil || d.Bucket == "UNMAPPED" {
			cp := d
			unmappedPrev = &cp
			continue
		}
		prevByID[*d.CategoryID] = d
	}

	out := make([]CategoryLandscape, 0, len(cats)+1)
	seenUnmapped := false
	for _, c := range cats {
		if c.State == "UNMAPPED" {
			seenUnmapped = true
			cl := CategoryLandscape{
				Profile: profileName,
				Identity: CategoryIdentity{
					CategoryID: c.ID, Name: nz(c.Name, "UNMAPPED"), IsLeaf: true, Unmapped: true,
				},
			}
			if profile.HasBlock("category.sales") {
				cl.Sales = categorySalesFromDemand(unmappedCur, unmappedPrev, curDemandErr, prevDemandErr)
			}
			attachCategoryDerivedBlocks(&cl, UnmappedCategoryID, skus, profile, stocks, inbound)
			out = append(out, cl)
			continue
		}
		if !c.IsLeaf {
			continue
		}
		cl := CategoryLandscape{
			Profile:  profileName,
			Identity: CategoryIdentity{CategoryID: c.ID, Name: c.Name, IsLeaf: c.IsLeaf, Unmapped: false},
		}
		var curD, prevD *factualapi.CategoryDemand
		if d, ok := curByID[c.ID]; ok {
			curD = &d
		}
		if d, ok := prevByID[c.ID]; ok {
			prevD = &d
		}
		if profile.HasBlock("category.sales") {
			cl.Sales = categorySalesFromDemand(curD, prevD, curDemandErr, prevDemandErr)
		}
		attachCategoryDerivedBlocks(&cl, c.ID, skus, profile, stocks, inbound)
		out = append(out, cl)
	}

	needUnmapped := unmappedCur != nil || unmappedPrev != nil
	if !needUnmapped {
		for _, sk := range skus {
			if sk.Identity.Unmapped {
				needUnmapped = true
				break
			}
		}
	}
	if needUnmapped && !seenUnmapped {
		cl := CategoryLandscape{
			Profile: profileName,
			Identity: CategoryIdentity{
				CategoryID: UnmappedCategoryID, Name: "UNMAPPED", IsLeaf: true, Unmapped: true,
			},
		}
		if profile.HasBlock("category.sales") {
			cl.Sales = categorySalesFromDemand(unmappedCur, unmappedPrev, curDemandErr, prevDemandErr)
		}
		attachCategoryDerivedBlocks(&cl, UnmappedCategoryID, skus, profile, stocks, inbound)
		out = append(out, cl)
	}
	return out
}

func attachCategoryDerivedBlocks(
	cl *CategoryLandscape,
	catID int64,
	skus []SKULandscape,
	profile ExecutableProfileV3,
	stocks map[int64]factualapi.SKUStock,
	inbound map[int64]factualapi.SKUInbound,
) {
	if profile.HasBlock("category.advertising.traffic") || profile.HasBlock("category.advertising.attribution") || profile.HasBlock("category.advertising") {
		cl.Advertising = rollupCategoryAdsV4(catID, skus)
	}
	if profile.HasBlock("category.stock") {
		cl.Stock = rollupCategoryStock(catID, skus, stocks, inbound)
	}
	if profile.HasBlock("category.supply") {
		cl.Supply = rollupCategorySupply(catID, skus)
	}
	if profile.HasBlock("category.price") {
		cl.Price = rollupCategoryPrice(catID, skus)
	}
}

func rollupCategoryPrice(catID int64, skus []SKULandscape) *CategoryPriceBlock {
	var mapped, increased, decreased, unchanged, withHistory int64
	for _, sk := range skus {
		if !skuMatchesCategory(sk, catID) {
			continue
		}
		mapped++
		if sk.Price == nil || sk.Price.CurrentPrice == nil {
			continue
		}
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
	if mapped == 0 {
		return &CategoryPriceBlock{State: BlockReadyEmpty}
	}
	block := &CategoryPriceBlock{
		State:                  BlockReady,
		SKUPriceIncreasedCount: &increased,
		SKUPriceDecreasedCount: &decreased,
		SKUPriceUnchangedCount: &unchanged,
	}
	block.PriceChangeCoverage = coverageRatio(withHistory, mapped)
	return block
}

func categorySalesFromDemand(cur, prev *factualapi.CategoryDemand, curErr, prevErr error) *CategorySalesBlock {
	if curErr != nil && prevErr != nil {
		return &CategorySalesBlock{
			State: BlockUnavailable, Limitations: unavailableBlock("CATEGORY_DEMAND_UNAVAILABLE", curErr.Error()),
		}
	}
	if cur == nil && prev == nil {
		if curErr != nil || prevErr != nil {
			block := &CategorySalesBlock{State: BlockPartial}
			if curErr != nil {
				block.Limitations = append(block.Limitations, Limitation{Code: "CATEGORY_DEMAND_CUR_UNAVAILABLE", Message: curErr.Error()})
			}
			if prevErr != nil {
				block.Limitations = append(block.Limitations, Limitation{Code: "CATEGORY_DEMAND_PREV_UNAVAILABLE", Message: prevErr.Error()})
			}
			if curErr != nil && prevErr == nil {
				block.State = BlockUnavailable
			}
			return block
		}
		return &CategorySalesBlock{State: BlockReadyEmpty}
	}
	var curAmt, prevAmt *MoneyAmount
	var curU, prevU *int64
	if cur != nil {
		curAmt = moneyFromValue(cur.Demand.OrderedAmount)
		curU = intFromValue(cur.Demand.OrderedUnits)
	}
	if prev != nil {
		prevAmt = moneyFromValue(prev.Demand.OrderedAmount)
		prevU = intFromValue(prev.Demand.OrderedUnits)
	}
	block := &CategorySalesBlock{
		State:         BlockReady,
		OrderedAmount: BuildMoneyComparison(curAmt, prevAmt),
		OrderedUnits:  BuildCountComparison(curU, prevU),
	}
	if cur != nil {
		applySalesDemandMeta(&block.State, &block.Limitations, cur.Demand, "SALES_CURRENT_7D")
	}
	if prev != nil {
		applySalesDemandMeta(&block.State, &block.Limitations, prev.Demand, "SALES_PREVIOUS_7D")
	}
	if curErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CATEGORY_DEMAND_CUR_UNAVAILABLE", Message: curErr.Error()})
	}
	if prevErr != nil {
		block.State = BlockPartial
		block.Limitations = append(block.Limitations, Limitation{Code: "CATEGORY_DEMAND_PREV_UNAVAILABLE", Message: prevErr.Error()})
	}
	return block
}

func skuMatchesCategory(sk SKULandscape, catID int64) bool {
	if catID == UnmappedCategoryID {
		return sk.Identity.Unmapped
	}
	if sk.Identity.CategoryID == nil {
		return false
	}
	return *sk.Identity.CategoryID == catID
}

func rollupCategorySupply(catID int64, skus []SKULandscape) *CategorySupplyBlock {
	var openInbound, overdue int64
	for _, sk := range skus {
		if !skuMatchesCategory(sk, catID) {
			continue
		}
		if sk.Supply == nil {
			continue
		}
		if sk.Supply.OpenSupplyCount != nil && *sk.Supply.OpenSupplyCount > 0 {
			openInbound++
		}
		if sk.Supply.OverdueSupplyCount != nil {
			overdue += *sk.Supply.OverdueSupplyCount
		}
	}
	if openInbound == 0 && overdue == 0 {
		return &CategorySupplyBlock{State: BlockReadyEmpty}
	}
	return &CategorySupplyBlock{
		State: BlockReady, SKUWithOpenInboundCount: &openInbound, OverdueSupplyCount: &overdue,
	}
}

func rollupCategoryStock(
	catID int64,
	skus []SKULandscape,
	stocks map[int64]factualapi.SKUStock,
	inbound map[int64]factualapi.SKUInbound,
) *CategoryStockBlock {
	var skuCount, demandPos, zeroDemand, inboundN int64
	var zeroStockExposure big.Rat
	var exposureCurrency string
	for _, sk := range skus {
		if !skuMatchesCategory(sk, catID) {
			continue
		}
		skuCount++
		hasSales := sk.Sales != nil && sk.Sales.OrderedUnits.Current != nil && *sk.Sales.OrderedUnits.Current > 0
		if hasSales {
			demandPos++
		}
		zeroSellable := false
		if st, ok := stocks[sk.Identity.CanonicalSKUID]; ok {
			if u := intFromValue(st.SellableUnits); u != nil && *u == 0 {
				zeroSellable = true
			}
		} else if sk.Stock != nil && sk.Stock.SellableUnits != nil && *sk.Stock.SellableUnits == 0 {
			zeroSellable = true
		}
		if zeroSellable && hasSales {
			zeroDemand++
			if sk.Sales != nil && sk.Sales.OrderedAmount.Current != nil {
				if exposureCurrency == "" {
					exposureCurrency = sk.Sales.OrderedAmount.Current.CurrencyCode
				}
				if exposureCurrency == sk.Sales.OrderedAmount.Current.CurrencyCode {
					if r, err := parseDecLocal(sk.Sales.OrderedAmount.Current.Amount); err == nil {
						zeroStockExposure.Add(&zeroStockExposure, r)
					}
				}
			}
		}
		if _, ok := inbound[sk.Identity.CanonicalSKUID]; ok {
			inboundN++
		}
	}
	if skuCount == 0 {
		return &CategoryStockBlock{State: BlockReadyEmpty}
	}
	block := &CategoryStockBlock{
		State: BlockReady, SKUCount: &skuCount, DemandPositiveSKUCount: &demandPos,
		ZeroStockWithDemandCount: &zeroDemand, SKUWithOpenInboundCount: &inboundN,
	}
	if zeroStockExposure.Sign() > 0 {
		d := canonicaljson.Decimal(formatRatLocal(&zeroStockExposure))
		block.SalesExposureZeroStock = &d
	}
	return block
}

func coverageRatio(num, den int64) *canonicaljson.Decimal {
	if den <= 0 {
		return nil
	}
	r := new(big.Rat).SetFrac64(num, den)
	d := canonicaljson.Decimal(formatRatLocal(r))
	return &d
}

func ctrFromTraffic(clicks, impressions factualapi.Value[int64]) *canonicaljson.Decimal {
	if clicks.State != factualapi.ValueKnown || clicks.Value == nil {
		return nil
	}
	if impressions.State != factualapi.ValueKnown || impressions.Value == nil || *impressions.Value == 0 {
		return nil
	}
	r := new(big.Rat).SetFrac64(*clicks.Value*100, *impressions.Value)
	d := canonicaljson.Decimal(formatRatLocal(r))
	return &d
}

// --- factual batch/list helpers ---

func (s FactualAPISource) listAllCategories(ctx context.Context, sealed factualapi.FactualReadContext) ([]factualapi.Category, error) {
	var out []factualapi.Category
	token := ""
	for {
		page, err := s.API.ListCategories(ctx, sealed, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return out, nil
}

func (s FactualAPISource) listAllSKUs(ctx context.Context, sealed factualapi.FactualReadContext) ([]factualapi.SKU, error) {
	var out []factualapi.SKU
	token := ""
	for {
		page, err := s.API.ListSKUs(ctx, sealed, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return out, nil
}

func (s FactualAPISource) listAllCampaigns(ctx context.Context, sealed factualapi.FactualReadContext) ([]factualapi.Campaign, error) {
	var out []factualapi.Campaign
	token := ""
	for {
		page, err := s.API.ListCampaigns(ctx, sealed, factualapi.CampaignFilter{}, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return out, nil
}

func (s FactualAPISource) listAllCategoryDemand(ctx context.Context, sealed factualapi.FactualReadContext, period factualapi.StandardPeriod) ([]factualapi.CategoryDemand, error) {
	var out []factualapi.CategoryDemand
	token := ""
	for {
		page, err := s.API.ListCategoryDemand(ctx, sealed, period, nil, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return out, nil
}

func (s FactualAPISource) batchSKUDemand(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod) (map[int64]factualapi.SKUDemand, error) {
	out := map[int64]factualapi.SKUDemand{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUDemand(ctx, sealed, chunk, period, nil)
		if err != nil {
			return out, err
		}
		for id, d := range indexSKUDemandBatch(res.Items) {
			out[id] = d
		}
	}
	return out, nil
}

func chunkInt64IDs(ids []int64, size int) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	if size <= 0 || len(ids) <= size {
		return [][]int64{ids}
	}
	out := make([][]int64, 0, (len(ids)+size-1)/size)
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}

func (s FactualAPISource) batchSKUStock(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUStock, error) {
	out := map[int64]factualapi.SKUStock{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUStock(ctx, sealed, chunk)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Stock != nil {
				out[it.ID] = *it.Stock
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUPrice(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUPrice, error) {
	out := map[int64]factualapi.SKUPrice{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUPrice(ctx, sealed, chunk)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Price != nil {
				out[it.ID] = *it.Price
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUSearch(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64, period factualapi.StandardPeriod, explicit *factualapi.Period) (map[int64]factualapi.SKUSearchSummary, error) {
	out := map[int64]factualapi.SKUSearchSummary{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUSearchSummary(ctx, sealed, chunk, period, explicit)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Fact != nil {
				out[it.ID] = *it.Fact
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUPromotionSummary(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUPromotionSummary, error) {
	out := map[int64]factualapi.SKUPromotionSummary{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUPromotionSummary(ctx, sealed, chunk)
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

func (s FactualAPISource) batchSKUInbound(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUInbound, error) {
	out := map[int64]factualapi.SKUInbound{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUInbound(ctx, sealed, chunk)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Inbound != nil {
				out[it.ID] = *it.Inbound
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchStockCoverage(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.StockCoverage, error) {
	out := map[int64]factualapi.StockCoverage{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetStockCoverage(ctx, sealed, chunk)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Coverage != nil {
				out[it.ID] = *it.Coverage
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchSKUPriceLastChange(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUPriceLastObservedChange, error) {
	out := map[int64]factualapi.SKUPriceLastObservedChange{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkInt64IDs(ids, 1000) {
		res, err := s.API.BatchGetSKUPriceLastObservedChange(ctx, sealed, chunk)
		if err != nil {
			return out, err
		}
		for _, it := range res.Items {
			if it.Status == factualapi.BatchFound && it.Change != nil {
				out[it.ID] = *it.Change
			}
		}
	}
	return out, nil
}

func (s FactualAPISource) batchCampaignTargets(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64][]factualapi.CampaignTarget, error) {
	out := make(map[int64][]factualapi.CampaignTarget, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	res, err := s.API.BatchListCampaignTargetSKUs(ctx, sealed, ids, 0)
	if err != nil {
		return out, err
	}
	for _, item := range res.Items {
		if item.Status == factualapi.BatchFound {
			out[item.CampaignID] = append([]factualapi.CampaignTarget(nil), item.Items...)
		}
	}
	return out, nil
}

func moneyFromValue(v factualapi.Value[factualapi.Money]) *MoneyAmount {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	return &MoneyAmount{
		Amount:       canonicaljson.Decimal(v.Value.Amount.String()),
		CurrencyCode: v.Value.CurrencyCode,
	}
}

func moneyFromSigned(v factualapi.Value[factualapi.SignedMoney]) *MoneyAmount {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	return &MoneyAmount{Amount: canonicaljson.Decimal(v.Value.Amount), CurrencyCode: v.Value.CurrencyCode}
}

func intFromValue(v factualapi.Value[int64]) *int64 {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	x := *v.Value
	return &x
}

func decFromValue(v factualapi.Value[factualapi.Decimal]) *canonicaljson.Decimal {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	d := canonicaljson.Decimal(v.Value.String())
	return &d
}

func decIntFromValue(v factualapi.Value[factualapi.Decimal]) *int64 {
	d := decFromValue(v)
	if d == nil {
		return nil
	}
	r, ok := new(big.Rat).SetString(string(*d))
	if !ok || !r.IsInt() {
		return nil
	}
	n := r.Num().Int64()
	return &n
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

func parseDecLocal(d canonicaljson.Decimal) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(string(d))
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", d)
	}
	return r, nil
}

func formatRatLocal(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	s := r.FloatString(12)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}
