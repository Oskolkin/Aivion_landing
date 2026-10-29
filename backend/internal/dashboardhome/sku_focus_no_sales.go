package dashboardhome

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

const noSalesHistoryLookbackDays = 60

type noSalesDayMetric struct {
	orders  int32
	revenue float64
	hasData bool
}

type noSalesFocusMetrics struct {
	ordersByDate map[string]noSalesDayMetric
}

type noSalesFocusCandidate struct {
	ozonProductID           int64
	sku                     *int64
	offerID                 *string
	productName             string
	currentStock            int32
	noSalesDays             int32
	lastCompletedMetricDate string
	unitPrice               *float64
	frozenStockValue        *float64
	previousDayOrders       int32
}

func qualifiesForNoSalesFocusTab(stockAvailable int32, ordersOnCheckDay int32, hasConfirmedCheckDay bool) bool {
	if !hasConfirmedCheckDay {
		return false
	}
	if stockAvailable <= 0 {
		return false
	}
	return ordersOnCheckDay == 0
}

func countConsecutiveNoSalesDays(metricDate time.Time, ordersByDate map[string]noSalesDayMetric) (int32, bool) {
	if len(ordersByDate) == 0 {
		return 0, false
	}
	checkKey := metricDate.Format("2006-01-02")
	checkDay, ok := ordersByDate[checkKey]
	if !ok || !checkDay.hasData {
		return 0, false
	}
	if checkDay.orders != 0 {
		return 0, false
	}

	days := int32(0)
	for d := metricDate; ; d = d.AddDate(0, 0, -1) {
		key := d.Format("2006-01-02")
		day, ok := ordersByDate[key]
		if !ok || !day.hasData {
			break
		}
		if day.orders > 0 {
			break
		}
		days++
	}
	if days < 1 {
		return 0, false
	}
	return days, true
}

func resolveNoSalesUnitPrice(referencePrice *float64, ordersByDate map[string]noSalesDayMetric) *float64 {
	if referencePrice != nil && *referencePrice > 0 {
		v := math.Round(*referencePrice*100) / 100
		return &v
	}

	var revenueSum float64
	var ordersSum int64
	for _, day := range ordersByDate {
		if !day.hasData || day.orders <= 0 || day.revenue <= 0 {
			continue
		}
		revenueSum += day.revenue
		ordersSum += int64(day.orders)
	}
	if ordersSum <= 0 {
		return nil
	}
	price := math.Round((revenueSum/float64(ordersSum))*100) / 100
	if price <= 0 {
		return nil
	}
	return &price
}

func calcFrozenStockValue(stock int32, unitPrice *float64) *float64 {
	if stock <= 0 || unitPrice == nil || *unitPrice <= 0 {
		return nil
	}
	v := math.Round(float64(stock)**unitPrice*100) / 100
	if v <= 0 {
		return nil
	}
	return &v
}

func buildNoSalesFocusCandidates(
	skus []analytics.DashboardSKUMetricsItem,
	metricDate time.Time,
	metricsByProduct map[int64]noSalesFocusMetrics,
	referencePrices map[int64]float64,
) []noSalesFocusCandidate {
	checkKey := metricDate.Format("2006-01-02")
	out := make([]noSalesFocusCandidate, 0)
	for _, sku := range skus {
		if sku.StockAvailable <= 0 || sku.OrdersCount != 0 {
			continue
		}
		metrics := metricsByProduct[sku.OzonProductID]
		if metrics.ordersByDate == nil {
			continue
		}
		checkDay, ok := metrics.ordersByDate[checkKey]
		if !ok || !checkDay.hasData {
			continue
		}
		if !qualifiesForNoSalesFocusTab(sku.StockAvailable, checkDay.orders, true) {
			continue
		}
		noSalesDays, ok := countConsecutiveNoSalesDays(metricDate, metrics.ordersByDate)
		if !ok {
			continue
		}
		var refPrice *float64
		if price, ok := referencePrices[sku.OzonProductID]; ok && price > 0 {
			refPrice = &price
		}
		unitPrice := resolveNoSalesUnitPrice(refPrice, metrics.ordersByDate)
		frozen := calcFrozenStockValue(sku.StockAvailable, unitPrice)
		out = append(out, noSalesFocusCandidate{
			ozonProductID:           sku.OzonProductID,
			sku:                     sku.SKU,
			offerID:                 sku.OfferID,
			productName:             derefStr(sku.ProductName, "Товар"),
			currentStock:            sku.StockAvailable,
			noSalesDays:             noSalesDays,
			lastCompletedMetricDate: checkKey,
			unitPrice:               unitPrice,
			frozenStockValue:        frozen,
			previousDayOrders:       0,
		})
	}
	return out
}

func sortNoSalesFocusCandidates(candidates []noSalesFocusCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		aFrozen := a.frozenStockValue
		bFrozen := b.frozenStockValue
		aHasFrozen := aFrozen != nil && *aFrozen > 0
		bHasFrozen := bFrozen != nil && *bFrozen > 0
		if aHasFrozen && bHasFrozen && *aFrozen != *bFrozen {
			return *aFrozen > *bFrozen
		}
		if aHasFrozen && !bHasFrozen {
			return true
		}
		if !aHasFrozen && bHasFrozen {
			return false
		}
		if a.currentStock != b.currentStock {
			return a.currentStock > b.currentStock
		}
		if a.noSalesDays != b.noSalesDays {
			return a.noSalesDays > b.noSalesDays
		}
		if a.productName != b.productName {
			return a.productName < b.productName
		}
		return a.ozonProductID < b.ozonProductID
	})
}

func noSalesFocusCardFromCandidate(c noSalesFocusCandidate) SKUFocusCard {
	stock := c.currentStock
	noSalesDays := c.noSalesDays
	prevOrders := c.previousDayOrders
	metricDate := c.lastCompletedMetricDate
	return SKUFocusCard{
		OzonProductID:           c.ozonProductID,
		SKU:                     c.sku,
		OfferID:                 c.offerID,
		ProductName:             c.productName,
		CurrentStock:            &stock,
		NoSalesDays:             &noSalesDays,
		LastCompletedMetricDate: &metricDate,
		UnitPrice:               c.unitPrice,
		FrozenStockValue:        c.frozenStockValue,
		PreviousDayOrders:       &prevOrders,
		KeyMetric:               fmt.Sprintf("Остаток %d", stock),
	}
}
