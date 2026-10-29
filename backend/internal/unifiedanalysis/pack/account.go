package pack

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// accountWindowSums aggregates daily_account_metrics over a date range.
type accountWindowSums struct {
	Revenue   float64
	Orders    int64
	Returns   int64
	Cancelled int64
}

func sumAccountWindow(idx *Indexes, from, to string) accountWindowSums {
	var out accountWindowSums
	for _, d := range dateRangeInclusive(from, to) {
		row, ok := idx.AccountMetricsByDate[d]
		if !ok {
			continue
		}
		out.Revenue += numericToFloat(row.Revenue)
		out.Orders += int64(row.OrdersCount)
		out.Returns += int64(row.ReturnsCount)
		out.Cancelled += int64(row.CancelCount)
	}
	return out
}

// funnelWindowSums aggregates product_analytics_daily (account scope) over a date range.
type funnelWindowSums struct {
	Impressions     int64
	ProductViews    int64
	AddToCart       int64
	Orders          int64
	CompletedOrders int64
}

func sumFunnelWindow(idx *Indexes, from, to string) funnelWindowSums {
	var out funnelWindowSums
	for _, d := range dateRangeInclusive(from, to) {
		row, ok := idx.FunnelByDate[d]
		if !ok {
			continue
		}
		out.Impressions += pgInt8(row.Impressions)
		out.ProductViews += pgInt8(row.ProductViews)
		out.AddToCart += pgInt8(row.AddToCart)
		out.Orders += pgInt8(row.OrdersCount)
		out.CompletedOrders += pgInt8(row.CompletedOrders)
	}
	return out
}

// adWindowSums aggregates ad_metrics_daily across all campaigns over a date range.
type adWindowSums struct {
	Spend   float64
	Revenue float64
	Orders  int64
}

func sumAdWindow(idx *Indexes, from, to string) adWindowSums {
	var out adWindowSums
	for _, campaignID := range idx.CampaignUniverse {
		byDate := idx.CampaignMetricsByDate[campaignID]
		for _, d := range dateRangeInclusive(from, to) {
			day, ok := byDate[d]
			if !ok {
				continue
			}
			out.Spend += day.Spend
			out.Revenue += day.Revenue
			out.Orders += day.Orders
		}
	}
	return out
}

// skuHealthCounters summarizes account-level SKU health signals derived from skuFacts.
type skuHealthCounters struct {
	Active       int
	Stockout     int
	Deficit      int
	Surplus      int
	NoSalesStock int
}

func computeSKUHealthCounters(idx *Indexes, facts map[string]skuFacts) skuHealthCounters {
	var out skuHealthCounters
	for _, sku := range idx.SKUUniverse {
		f := facts[sku]
		if f.Revenue7D > 0 || f.Orders7D > 0 {
			out.Active++
		}
		if f.HasStockData && f.AvailableStock <= 0 {
			out.Stockout++
		}
		// Deficit/surplus counters use canonical Aivion stock_status, not raw Ozon grade.
		if stockpolicy.IsDeficitStatus(f.StockStatus) {
			out.Deficit++
		}
		if stockpolicy.IsSurplusStatus(f.StockStatus) {
			out.Surplus++
		}
		if f.HasStockData && f.AvailableStock > 0 && f.Orders7D == 0 {
			out.NoSalesStock++
		}
	}
	return out
}

// BuildAccountSummaryTable renders the single-row account_summary.csv.
func BuildAccountSummaryTable(
	idx *Indexes,
	periods Periods,
	facts map[string]skuFacts,
	sellerAccountID int64,
	metricDate string,
	limitations *limitationCollector,
) csvTable {
	if len(idx.AccountMetricsByDate) == 0 {
		limitations.Add(LimitationAccountDataUnavailable)
	}
	if len(idx.FunnelByDate) == 0 {
		limitations.Add(LimitationFunnelDataUnavailable)
	}

	current := sumAccountWindow(idx, periods.Current7D.From, periods.Current7D.To)
	previous := sumAccountWindow(idx, periods.Previous7D.From, periods.Previous7D.To)
	funnel := sumFunnelWindow(idx, periods.Current7D.From, periods.Current7D.To)
	ads := sumAdWindow(idx, periods.Current7D.From, periods.Current7D.To)
	health := computeSKUHealthCounters(idx, facts)

	delivered := current.Orders - current.Cancelled - current.Returns
	if delivered < 0 {
		delivered = 0
	}

	ordersAbs := DeltaAbs(float64(current.Orders), float64(previous.Orders))
	ordersPct := DeltaPct(float64(current.Orders), float64(previous.Orders))
	revAbs := DeltaAbs(current.Revenue, previous.Revenue)
	revPct := DeltaPct(current.Revenue, previous.Revenue)
	aov := AOV(current.Revenue, current.Orders)

	impressionToView := ConversionPct(float64(funnel.Impressions), float64(funnel.ProductViews))
	viewToCart := ConversionPct(float64(funnel.ProductViews), float64(funnel.AddToCart))
	cartToOrder := ConversionPct(float64(funnel.AddToCart), float64(funnel.Orders))
	orderToDelivered := ConversionPct(float64(funnel.Orders), float64(funnel.CompletedOrders))
	accountDRR := DRRPct(ads.Spend, current.Revenue)
	attributedShare := DRRPct(ads.Spend, ads.Revenue)

	row := []string{
		"account:" + FormatIntCSV(sellerAccountID) + ":" + metricDate,
		FormatIntCSV(sellerAccountID),
		metricDate,
		FormatIntCSV(current.Orders),
		FormatIntCSV(previous.Orders),
		FormatFloatCSV(&ordersAbs),
		FormatPctCSV(ordersPct),
		FormatMoneyCSV(current.Revenue),
		FormatMoneyCSV(previous.Revenue),
		FormatMoneyCSV(revAbs),
		FormatPctCSV(revPct),
		FormatIntCSV(delivered),
		FormatIntCSV(current.Cancelled),
		FormatIntCSV(current.Returns),
		FormatMoneyCSVPtr(aov),
		FormatIntCSV(funnel.Impressions),
		FormatIntCSV(funnel.ProductViews),
		FormatIntCSV(funnel.AddToCart),
		FormatIntCSV(funnel.Orders),
		FormatPctCSV(impressionToView),
		FormatPctCSV(viewToCart),
		FormatPctCSV(cartToOrder),
		FormatPctCSV(orderToDelivered),
		FormatIntCSV(int64(health.Active)),
		FormatIntCSV(int64(health.Stockout)),
		FormatIntCSV(int64(health.Deficit)),
		FormatIntCSV(int64(health.Surplus)),
		FormatIntCSV(int64(health.NoSalesStock)),
		FormatMoneyCSV(ads.Spend),
		FormatMoneyCSV(ads.Revenue),
		FormatIntCSV(ads.Orders),
		FormatPctCSV(accountDRR),
		FormatPctCSV(accountDRR), // ad_spend_share_of_total_revenue_pct (alias of account_drr_pct)
		FormatPctCSV(attributedShare),
	}

	return csvTable{Name: "account_summary.csv", Columns: contract.AccountSummaryColumnsV1, Rows: [][]string{row}}
}

// BuildAccountDailyTable renders account_daily.csv rows for observed days only.
// Missing calendar days are omitted (never synthesized as revenue/orders/views = 0).
func BuildAccountDailyTable(idx *Indexes, periods Periods, sellerAccountID int64) csvTable {
	dates := dateRangeInclusive(periods.Trend.From, periods.Trend.To)
	rows := make([][]string, 0, len(dates))
	for _, d := range dates {
		account, hasAccount := idx.AccountMetricsByDate[d]
		funnel, hasFunnel := idx.FunnelByDate[d]
		hasAds := false
		var adSpend, adRevenue float64
		var adOrders int64
		for _, campaignID := range idx.CampaignUniverse {
			if day, ok := idx.CampaignMetricsByDate[campaignID][d]; ok {
				hasAds = true
				adSpend += day.Spend
				adRevenue += day.Revenue
				adOrders += day.Orders
			}
		}
		// Sparse series: emit only days with at least one observed source row.
		if !hasAccount && !hasFunnel && !hasAds {
			continue
		}

		var revenue float64
		var orders, cancelled, returns, delivered int64
		if hasAccount {
			revenue = numericToFloat(account.Revenue)
			orders = int64(account.OrdersCount)
			cancelled = int64(account.CancelCount)
			returns = int64(account.ReturnsCount)
			delivered = orders - cancelled - returns
			if delivered < 0 {
				delivered = 0
			}
		}

		impressions, views, addToCart := "", "", ""
		if hasFunnel {
			impressions = FormatIntCSV(pgInt8(funnel.Impressions))
			views = FormatIntCSV(pgInt8(funnel.ProductViews))
			addToCart = FormatIntCSV(pgInt8(funnel.AddToCart))
		}

		ordersCell, revenueCell, deliveredCell, cancelledCell, returnsCell := "", "", "", "", ""
		if hasAccount {
			ordersCell = FormatIntCSV(orders)
			revenueCell = FormatMoneyCSV(revenue)
			deliveredCell = FormatIntCSV(delivered)
			cancelledCell = FormatIntCSV(cancelled)
			returnsCell = FormatIntCSV(returns)
		}

		adSpendCell, adRevenueCell, adOrdersCell := "", "", ""
		shareOfStoreCell, shareOfAttrCell := "", ""
		if hasAds {
			adSpendCell = FormatMoneyCSV(adSpend)
			adRevenueCell = FormatMoneyCSV(adRevenue)
			adOrdersCell = FormatIntCSV(adOrders)
			if hasAccount {
				shareOfStoreCell = FormatPctCSV(DRRPct(adSpend, revenue))
			}
			shareOfAttrCell = FormatPctCSV(DRRPct(adSpend, adRevenue))
		}

		rows = append(rows, []string{
			"account:" + FormatIntCSV(sellerAccountID) + ":" + d,
			FormatIntCSV(sellerAccountID),
			d,
			ordersCell,
			revenueCell,
			deliveredCell,
			cancelledCell,
			returnsCell,
			impressions,
			views,
			addToCart,
			adSpendCell,
			adRevenueCell,
			adOrdersCell,
			shareOfStoreCell,
			shareOfAttrCell,
		})
	}
	return csvTable{Name: "account_daily.csv", Columns: contract.AccountDailyColumnsV1, Rows: rows}
}
