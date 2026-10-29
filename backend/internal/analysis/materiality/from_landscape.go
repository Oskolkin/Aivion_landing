package materiality

import (
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// CoreFactsFromLandscape converts a Landscape Core set into ranking CoreFacts.
// Money/price Decimals are preserved as Exact strings (no ParseFloat). UNMAPPED categories are skipped.
// Parse/validation errors propagate; missing amounts are never coerced to zero.
func CoreFactsFromLandscape(set landscape.LandscapeSet) (CoreFacts, error) {
	out := CoreFacts{
		Account:      AccountFeatures{SemanticKey: "account", CanonicalRefID: strconv.FormatInt(set.Account.Identity.SellerAccountID, 10)},
		Capabilities: mapCapabilities(set.GlobalCapabilities),
	}
	var err error
	if out.Account, err = accountFeatures(set.Account); err != nil {
		return CoreFacts{}, err
	}
	for _, c := range set.Categories {
		if c.Identity.Unmapped {
			continue
		}
		cf, e := categoryFeatures(c)
		if e != nil {
			return CoreFacts{}, e
		}
		out.Categories = append(out.Categories, cf)
	}
	for _, s := range set.SKUs {
		sf, e := skuFeatures(s)
		if e != nil {
			return CoreFacts{}, e
		}
		out.SKUs = append(out.SKUs, sf)
	}
	for _, c := range set.Campaigns {
		cf, e := campaignFeatures(c)
		if e != nil {
			return CoreFacts{}, e
		}
		out.Campaigns = append(out.Campaigns, cf)
	}
	return out, nil
}

// SelectionEntityRefs maps a Materiality Selection into Landscape Detail EntityRefs.
func SelectionEntityRefs(sel Selection) []landscape.EntityRef {
	out := make([]landscape.EntityRef, 0, 1+len(sel.Categories)+len(sel.SKUs)+len(sel.Campaigns))
	if id, ok := parseRefID(sel.Account.CanonicalRefID); ok {
		out = append(out, landscape.EntityRef{Kind: landscape.EntityAccount, ID: id})
	}
	for _, e := range sel.Categories {
		if id, ok := parseRefID(e.CanonicalRefID); ok {
			out = append(out, landscape.EntityRef{Kind: landscape.EntityCategory, ID: id})
		}
	}
	for _, e := range sel.SKUs {
		if id, ok := parseRefID(e.CanonicalRefID); ok {
			out = append(out, landscape.EntityRef{Kind: landscape.EntitySKU, ID: id})
		}
	}
	for _, e := range sel.Campaigns {
		if id, ok := parseRefID(e.CanonicalRefID); ok {
			out = append(out, landscape.EntityRef{Kind: landscape.EntityCampaign, ID: id})
		}
	}
	return out
}

func mapCapabilities(flags []landscape.CapabilityFlag) map[string]string {
	out := make(map[string]string, len(flags))
	for _, f := range flags {
		if f.Key == "" {
			continue
		}
		out[f.Key] = f.FinalState
	}
	return out
}

func accountFeatures(a landscape.AccountLandscape) (AccountFeatures, error) {
	f := AccountFeatures{
		SemanticKey:    "account",
		CanonicalRefID: strconv.FormatInt(a.Identity.SellerAccountID, 10),
	}
	if a.Sales != nil {
		st := a.Sales.State
		var err error
		if f.OrderedAmountCurrent7, err = moneyCurrent(st, a.Sales.OrderedAmount); err != nil {
			return AccountFeatures{}, err
		}
		if f.OrderedAmountPrevious7, err = moneyPrevious(st, a.Sales.OrderedAmount); err != nil {
			return AccountFeatures{}, err
		}
		if a.Sales.Trend30DAmount != nil {
			if f.OrderedAmount30D, err = moneySingle(st, a.Sales.Trend30DAmount.Amount); err != nil {
				return AccountFeatures{}, err
			}
		}
	}
	if a.Advertising != nil {
		st := a.Advertising.State
		var err error
		if a.Advertising.Traffic != nil {
			st = a.Advertising.Traffic.State
			if f.PaidSpendCurrent7, err = moneyCurrent(st, a.Advertising.Traffic.PaidSpend); err != nil {
				return AccountFeatures{}, err
			}
			if f.PaidSpendPrevious7, err = moneyPrevious(st, a.Advertising.Traffic.PaidSpend); err != nil {
				return AccountFeatures{}, err
			}
		}
		if a.Advertising.Attribution != nil {
			st = a.Advertising.Attribution.State
			if f.AttributedAmountCurrent7, err = moneyCurrent(st, a.Advertising.Attribution.AttributedOrderAmount); err != nil {
				return AccountFeatures{}, err
			}
		}
	}
	return f, nil
}

func categoryFeatures(c landscape.CategoryLandscape) (CategoryFeatures, error) {
	f := CategoryFeatures{
		SemanticKey:    fmt.Sprintf("category:%d", c.Identity.CategoryID),
		CanonicalRefID: strconv.FormatInt(c.Identity.CategoryID, 10),
	}
	if c.Sales != nil {
		st := c.Sales.State
		var err error
		if f.OrderedAmountCurrent7, err = moneyCurrent(st, c.Sales.OrderedAmount); err != nil {
			return CategoryFeatures{}, err
		}
		if f.OrderedAmountPrevious7, err = moneyPrevious(st, c.Sales.OrderedAmount); err != nil {
			return CategoryFeatures{}, err
		}
	}
	if c.Advertising != nil && c.Advertising.Traffic != nil {
		st := c.Advertising.Traffic.State
		var err error
		if f.PaidSpendCurrent7, err = moneyCurrent(st, c.Advertising.Traffic.PaidSpend); err != nil {
			return CategoryFeatures{}, err
		}
		if f.PaidSpendPrevious7, err = moneyPrevious(st, c.Advertising.Traffic.PaidSpend); err != nil {
			return CategoryFeatures{}, err
		}
	}
	if c.Stock != nil && c.Stock.SalesExposureZeroStock != nil {
		ex, err := parseExactDecimal(*c.Stock.SalesExposureZeroStock)
		if err != nil {
			return CategoryFeatures{}, err
		}
		f.ZeroStockSalesAmount = KnownExact(ex)
	}
	if c.Supply != nil {
		overdue := c.Supply.OverdueSupplyCount != nil && *c.Supply.OverdueSupplyCount > 0
		f.HasSupplyOverdue = overdue
		if overdue {
			f.SupplyOverdueSales = f.OrderedAmountCurrent7
		}
	}
	return f, nil
}

func skuFeatures(s landscape.SKULandscape) (SKUFeatures, error) {
	f := SKUFeatures{
		SemanticKey:    fmt.Sprintf("sku:%d", s.Identity.CanonicalSKUID),
		CanonicalRefID: strconv.FormatInt(s.Identity.CanonicalSKUID, 10),
	}
	if s.Identity.CategoryID != nil {
		f.CategorySemanticKey = fmt.Sprintf("category:%d", *s.Identity.CategoryID)
	}
	if s.Sales != nil {
		st := s.Sales.State
		f.SalesAvailable = blockUsable(st)
		var err error
		if f.OrderedAmountCurrent7, err = moneyCurrent(st, s.Sales.OrderedAmount); err != nil {
			return SKUFeatures{}, err
		}
		if f.OrderedAmountPrevious7, err = moneyPrevious(st, s.Sales.OrderedAmount); err != nil {
			return SKUFeatures{}, err
		}
		if f.OrderedUnitsCurrent7, err = countCurrent(st, s.Sales.OrderedUnits); err != nil {
			return SKUFeatures{}, err
		}
		if f.OrderedUnitsPrevious7, err = countPrevious(st, s.Sales.OrderedUnits); err != nil {
			return SKUFeatures{}, err
		}
		if s.Sales.Trend30DAmount != nil {
			if f.OrderedAmount30D, err = moneySingle(st, s.Sales.Trend30DAmount.Amount); err != nil {
				return SKUFeatures{}, err
			}
		}
	}
	if s.Stock != nil {
		st := s.Stock.State
		f.StockAvailable = blockUsable(st)
		if s.Stock.SellableUnits != nil {
			f.SellableStock = KnownExact(exactInt(*s.Stock.SellableUnits))
		} else if !blockUsable(st) {
			f.SellableStock = MoneyFeature{State: blockFeatureState(st)}
		}
		if s.Stock.CoverageDays7D != nil {
			cov, err := decimalFeature(st, s.Stock.CoverageDays7D)
			if err != nil {
				return SKUFeatures{}, err
			}
			f.CoverageDays = cov
		}
	}
	if s.Advertising != nil && s.Advertising.Traffic != nil {
		st := s.Advertising.Traffic.State
		f.AdsAvailable = blockUsable(st)
		var err error
		if f.PaidSpendCurrent7, err = moneyCurrent(st, s.Advertising.Traffic.PaidSpend); err != nil {
			return SKUFeatures{}, err
		}
		if f.PaidSpendPrevious7, err = moneyPrevious(st, s.Advertising.Traffic.PaidSpend); err != nil {
			return SKUFeatures{}, err
		}
	}
	if s.Price != nil {
		st := s.Price.State
		f.PriceAvailable = blockUsable(st)
		if s.Price.CurrentPrice != nil {
			ex, err := parseExactDecimal(s.Price.CurrentPrice.Amount)
			if err != nil {
				return SKUFeatures{}, err
			}
			f.PriceCurrent = KnownExact(ex)
		} else if !blockUsable(st) {
			f.PriceCurrent = MoneyFeature{State: blockFeatureState(st)}
		}
		if s.Price.LastObservedChange != nil {
			if s.Price.LastObservedChange.Previous != nil {
				ex, err := parseExactDecimal(s.Price.LastObservedChange.Previous.Amount)
				if err != nil {
					return SKUFeatures{}, err
				}
				f.PricePrevious = KnownExact(ex)
			} else if s.Price.LastObservedChange.Current != nil && s.Price.CurrentPrice == nil {
				ex, err := parseExactDecimal(s.Price.LastObservedChange.Current.Amount)
				if err != nil {
					return SKUFeatures{}, err
				}
				f.PriceCurrent = KnownExact(ex)
			}
		}
	}
	if s.Search != nil {
		st := s.Search.State
		f.SearchAvailable = blockUsable(st)
		if s.Search.ProviderAveragePosition != nil {
			pos, err := decimalFeature(st, s.Search.ProviderAveragePosition)
			if err != nil {
				return SKUFeatures{}, err
			}
			f.SearchPositionCurrent = pos
		}
		if s.Search.PreviousProviderAveragePosition != nil {
			pos, err := decimalFeature(st, s.Search.PreviousProviderAveragePosition)
			if err != nil {
				return SKUFeatures{}, err
			}
			f.SearchPositionPrevious = pos
		}
	}
	if s.Supply != nil {
		f.HasSupplyOverdue = s.Supply.OverdueSupplyCount != nil && *s.Supply.OverdueSupplyCount > 0
	}
	return f, nil
}

func campaignFeatures(c landscape.CampaignLandscape) (CampaignFeatures, error) {
	f := CampaignFeatures{
		SemanticKey:    fmt.Sprintf("campaign:%d", c.Identity.CampaignID),
		CanonicalRefID: strconv.FormatInt(c.Identity.CampaignID, 10),
	}
	if c.Performance != nil {
		var err error
		if c.Performance.Traffic != nil {
			st := c.Performance.Traffic.State
			f.TrafficAvailable = blockUsable(st)
			if f.PaidSpendCurrent7, err = moneyCurrent(st, c.Performance.Traffic.PaidSpend); err != nil {
				return CampaignFeatures{}, err
			}
			if f.PaidSpendPrevious7, err = moneyPrevious(st, c.Performance.Traffic.PaidSpend); err != nil {
				return CampaignFeatures{}, err
			}
			if c.Performance.Traffic.CTR != nil {
				if c.Performance.Traffic.CTR.CurrentPct != nil {
					if f.CTRCurrentPP, err = decimalFeature(st, c.Performance.Traffic.CTR.CurrentPct); err != nil {
						return CampaignFeatures{}, err
					}
				}
				if c.Performance.Traffic.CTR.PreviousPct != nil {
					if f.CTRPreviousPP, err = decimalFeature(st, c.Performance.Traffic.CTR.PreviousPct); err != nil {
						return CampaignFeatures{}, err
					}
				}
			}
		}
		if c.Performance.Attribution != nil {
			st := c.Performance.Attribution.State
			f.AttributionAvailable = blockUsable(st)
			if f.AttributedAmountCurrent7, err = moneyCurrent(st, c.Performance.Attribution.AttributedOrderAmount); err != nil {
				return CampaignFeatures{}, err
			}
			if f.AttributedAmountPrevious7, err = moneyPrevious(st, c.Performance.Attribution.AttributedOrderAmount); err != nil {
				return CampaignFeatures{}, err
			}
			if c.Performance.Attribution.CalculatedDRR != nil {
				f.DRRAvailable = true
				if c.Performance.Attribution.CalculatedDRR.CurrentPct != nil {
					if f.DRRCurrentPP, err = decimalFeature(st, c.Performance.Attribution.CalculatedDRR.CurrentPct); err != nil {
						return CampaignFeatures{}, err
					}
				}
				if c.Performance.Attribution.CalculatedDRR.PreviousPct != nil {
					if f.DRRPreviousPP, err = decimalFeature(st, c.Performance.Attribution.CalculatedDRR.PreviousPct); err != nil {
						return CampaignFeatures{}, err
					}
				}
			}
		}
	}
	return f, nil
}

func blockUsable(s landscape.BlockState) bool {
	return s == landscape.BlockReady || s == landscape.BlockPartial || s == landscape.BlockReadyEmpty
}

func blockFeatureState(s landscape.BlockState) FeatureState {
	switch s {
	case landscape.BlockUnavailable:
		return FeatureUnavailable
	case landscape.BlockStale:
		return FeatureStale
	case landscape.BlockNotApplicable:
		return FeatureNotApplicable
	default:
		return FeatureUnknown
	}
}

func moneyCurrent(blockState landscape.BlockState, m landscape.MoneyComparison) (MoneyFeature, error) {
	if !blockUsable(blockState) {
		return MoneyFeature{State: blockFeatureState(blockState)}, nil
	}
	if m.Current == nil {
		return MoneyFeature{State: FeatureUnknown}, nil
	}
	ex, err := parseExactDecimal(m.Current.Amount)
	if err != nil {
		return MoneyFeature{}, fmt.Errorf("current money amount: %w", err)
	}
	return KnownExact(ex), nil
}

func moneyPrevious(blockState landscape.BlockState, m landscape.MoneyComparison) (MoneyFeature, error) {
	if !blockUsable(blockState) {
		return MoneyFeature{State: blockFeatureState(blockState)}, nil
	}
	if m.Previous == nil {
		return MoneyFeature{State: FeatureUnknown}, nil
	}
	ex, err := parseExactDecimal(m.Previous.Amount)
	if err != nil {
		return MoneyFeature{}, fmt.Errorf("previous money amount: %w", err)
	}
	return KnownExact(ex), nil
}

func moneySingle(blockState landscape.BlockState, d canonicaljson.Decimal) (MoneyFeature, error) {
	if !blockUsable(blockState) {
		return MoneyFeature{State: blockFeatureState(blockState)}, nil
	}
	ex, err := parseExactDecimal(d)
	if err != nil {
		return MoneyFeature{}, err
	}
	return KnownExact(ex), nil
}

func decimalFeature(blockState landscape.BlockState, d *canonicaljson.Decimal) (MoneyFeature, error) {
	if d == nil {
		return MoneyFeature{State: FeatureUnknown}, nil
	}
	if !blockUsable(blockState) {
		return MoneyFeature{State: blockFeatureState(blockState)}, nil
	}
	ex, err := parseExactDecimal(*d)
	if err != nil {
		return MoneyFeature{}, err
	}
	return KnownExact(ex), nil
}

func countCurrent(blockState landscape.BlockState, c landscape.CountComparison) (CountFeature, error) {
	if !blockUsable(blockState) {
		return CountFeature{State: blockFeatureState(blockState)}, nil
	}
	if c.Current == nil {
		return CountFeature{State: FeatureUnknown}, nil
	}
	return KnownCount(*c.Current), nil
}

func countPrevious(blockState landscape.BlockState, c landscape.CountComparison) (CountFeature, error) {
	if !blockUsable(blockState) {
		return CountFeature{State: blockFeatureState(blockState)}, nil
	}
	if c.Previous == nil {
		return CountFeature{State: FeatureUnknown}, nil
	}
	return KnownCount(*c.Previous), nil
}

func parseRefID(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
