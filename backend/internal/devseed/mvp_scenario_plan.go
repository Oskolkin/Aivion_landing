package devseed

import (
	"fmt"
	"math"
	"time"
)

// MVPScenario labels the demo membership of a product within the scenario plan.
// The scenario is the SOURCE of truth: it decides how many orders/sales/stock rows
// SeedMVPCommerce writes so that daily_sku_metrics rebuilt from source reproduces the
// SKU-focus demo tabs WITHOUT any post-rebuild UPDATE of daily_sku_metrics.
type MVPScenario string

const (
	scenarioBase      MVPScenario = "base"
	scenarioLongTail  MVPScenario = "long_tail"
	scenarioOrderDrop MVPScenario = "order_drop"
	scenarioStockRisk MVPScenario = "stock_risk"
	scenarioNoSales   MVPScenario = "no_sales"
	scenarioKeySku    MVPScenario = "key_sku"
)

// keySkuAccountRevenueShare is the target combined share of total CURRENT 7-day
// account revenue that curated key SKUs own (Pareto for SKU-focus key tab).
// Target demo band is ~55–75% so the cabinet is not a pure key-SKU monopoly.
const keySkuAccountRevenueShare = 0.65

// demoCurrentVsPreviousGrowth is the target (current_7d / previous_7d - 1) for
// account revenue/orders. Previous window is written from the same SKU mix so
// growth is realistic (+15–35%), not a cliff vs base-commerce previous week.
const demoCurrentVsPreviousGrowth = 0.25

const (
	demoCurrentWindowLen  = 7
	demoPreviousWindowLen = 7
)

// MVPWindowDaySKU is the plan for one product on one calendar day inside the demo window.
type MVPWindowDaySKU struct {
	DayIndex    int
	Date        time.Time
	OrdersCount int
	Units       int
	UnitPrice   float64
	Revenue     float64
}

// MVPProductPlan is the per-product portion of the scenario plan.
type MVPProductPlan struct {
	ProductIndex int
	Scenario     MVPScenario
	// StockOverride, when >= 0, is the total current availability written to stocks.
	StockOverride int
	// ReferencePriceOverride, when > 0, replaces the catalog reference_price.
	ReferencePriceOverride float64
	// Window covers previous_7d then current_7d (14 days when days allow).
	Window []MVPWindowDaySKU
}

// MVPScenarioPlan is the full deterministic day×SKU plan for previous+current weeks.
type MVPScenarioPlan struct {
	Anchor           time.Time
	Days             int
	WindowLen        int
	WindowStartIndex int
	WindowDates      []time.Time
	Products         []MVPProductPlan
}

// PlanForProduct returns the plan for products[i].
func (p *MVPScenarioPlan) PlanForProduct(index int) *MVPProductPlan {
	if index < 0 || index >= len(p.Products) {
		return nil
	}
	return &p.Products[index]
}

// InWindow reports whether dayIndex falls inside the trailing demo window.
func (p *MVPScenarioPlan) InWindow(dayIndex int) bool {
	return dayIndex >= p.WindowStartIndex && dayIndex <= p.WindowStartIndex+p.WindowLen-1
}

// BuildMVPScenarioPlan classifies products into demo scenarios and generates a
// deterministic plan for previous_7d + current_7d so weekly growth stays realistic.
func BuildMVPScenarioPlan(opts MVPSeedOptions, products []mvpProduct) (*MVPScenarioPlan, error) {
	anchor := MVPDateUTC(opts.AnchorDate)
	days := opts.Days
	if days <= 0 {
		days = 1
	}
	windowLen := demoPreviousWindowLen + demoCurrentWindowLen
	if days < windowLen {
		windowLen = days
	}
	windowStart := days - windowLen
	windowDates := make([]time.Time, windowLen)
	for j := 0; j < windowLen; j++ {
		windowDates[j] = anchor.AddDate(0, 0, -(windowLen - 1 - j))
	}
	// Offset of current_7d inside Window slice.
	curOffset := windowLen - demoCurrentWindowLen
	if curOffset < 0 {
		curOffset = 0
	}

	plan := &MVPScenarioPlan{
		Anchor:           anchor,
		Days:             days,
		WindowLen:        windowLen,
		WindowStartIndex: windowStart,
		WindowDates:      windowDates,
		Products:         make([]MVPProductPlan, len(products)),
	}
	for i := range plan.Products {
		plan.Products[i] = MVPProductPlan{ProductIndex: i, Scenario: scenarioLongTail, StockOverride: -1}
	}

	byName := make(map[string]int, len(products))
	for i, p := range products {
		if _, dup := byName[p.Name]; !dup {
			byName[p.Name] = i
		}
	}
	claimed := make(map[int]bool, len(products))
	claim := func(index int, scenario MVPScenario) error {
		if claimed[index] {
			return fmt.Errorf("product index %d claimed by two demo scenarios (last=%s)", index, scenario)
		}
		claimed[index] = true
		plan.Products[index].Scenario = scenario
		return nil
	}
	resolve := func(name, scenario string) (int, error) {
		idx, ok := byName[name]
		if !ok {
			return 0, fmt.Errorf("%s demo product not found: %q (increase --products)", scenario, name)
		}
		return idx, nil
	}

	// --- Order-drop overlay (declining into the anchor day) ---
	for _, sc := range mvpOrderDropDemoSKUs {
		idx, err := resolve(sc.ProductName, "order-drop")
		if err != nil {
			return nil, err
		}
		if err := claim(idx, scenarioOrderDrop); err != nil {
			return nil, err
		}
		pp := &plan.Products[idx]
		current := make([]MVPWindowDaySKU, demoCurrentWindowLen)
		for j := 0; j < demoCurrentWindowLen && j < windowLen; j++ {
			si := 7 - demoCurrentWindowLen + j
			orders := int(sc.Orders7D[si])
			unit := mvpOrderDropDemoUnitPrice(sc.BasePrice, sc.PriceFactors[si])
			wj := curOffset + j
			current[j] = newWindowDay(windowStart+wj, windowDates[wj], orders, unit)
		}
		// Previous week must be HIGHER than current so WoW order-drop tab has candidates
		// (intra-week decline alone is not enough when previous is scaled down by growth).
		pp.Window = prependElevatedPreviousWindow(current, windowStart, windowDates, curOffset, sc.BasePrice, 1.45)
	}

	// --- Stock-risk overlay ---
	for _, sc := range mvpStockRiskDemoSKUs {
		idx, err := resolve(sc.productName(), "stock-risk")
		if err != nil {
			return nil, err
		}
		if err := claim(idx, scenarioStockRisk); err != nil {
			return nil, err
		}
		pp := &plan.Products[idx]
		pp.StockOverride = int(sc.Stock7D[6])
		current := make([]MVPWindowDaySKU, demoCurrentWindowLen)
		for j := 0; j < demoCurrentWindowLen && j < windowLen; j++ {
			wj := curOffset + j
			current[j] = newWindowDay(windowStart+wj, windowDates[wj], int(sc.OrdersPerDay), sc.BasePrice)
		}
		pp.Window = prependPreviousWindow(current, windowStart, windowDates, curOffset, sc.BasePrice, 1+demoCurrentVsPreviousGrowth)
	}

	// --- No-sales overlay ---
	for _, sc := range mvpNoSalesDemoSKUs {
		idx, err := resolve(sc.productName(), "no-sales")
		if err != nil {
			return nil, err
		}
		if err := claim(idx, scenarioNoSales); err != nil {
			return nil, err
		}
		pp := &plan.Products[idx]
		pp.StockOverride = int(sc.Stock)
		pp.ReferencePriceOverride = sc.BasePrice
		products[idx].ReferencePrice = sc.BasePrice
		current := make([]MVPWindowDaySKU, demoCurrentWindowLen)
		for j := 0; j < demoCurrentWindowLen && j < windowLen; j++ {
			wj := curOffset + j
			// Entire current week has zero sales so days_without_sales reaches
			// PersistentNoSalesMinDays together with a quiet previous week.
			current[j] = newWindowDay(windowStart+wj, windowDates[wj], 0, sc.BasePrice)
		}
		pp.Window = make([]MVPWindowDaySKU, 0, windowLen)
		for j := 0; j < curOffset; j++ {
			// Quiet previous week (no sales) so the no-sales streak is ≥14 days.
			pp.Window = append(pp.Window, newWindowDay(windowStart+j, windowDates[j], 0, sc.BasePrice))
		}
		pp.Window = append(pp.Window, current...)
	}

	// --- Long-tail current window ---
	longTailWeekly := 0.0
	longTailIdx := 0
	for i := range products {
		if claimed[i] {
			continue
		}
		pp := &plan.Products[i]
		pp.Scenario = scenarioLongTail
		daily := mvpLongTailDailyOrders(products[i])
		price := products[i].ReferencePrice

		// Realistic exclusive stock-band mix for long-tail (~50 SKU after demos).
		// Target overall (with overlays): oos 2–4, critical 4–7, short 6–10,
		// planning 8–13, healthy 43–50, surplus 3–6, no_sales 3–5.
		switch longTailIdx % 20 {
		case 0:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = 0 // out_of_stock (~1/20)
		case 1:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = daily * 2 // critical (~1/20)
		case 2:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = daily * 5 // short (~1/20)
		case 3:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = daily * 10 // planning (~1/20)
		case 4:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = daily * 90 // surplus (~1/20)
		default:
			if daily < 1 {
				daily = 1
			}
			pp.StockOverride = daily * 28 // healthy (~15/20)
		}
		longTailIdx++

		current := make([]MVPWindowDaySKU, demoCurrentWindowLen)
		for j := 0; j < demoCurrentWindowLen && j < windowLen; j++ {
			wj := curOffset + j
			// Mild end-of-week dip on last 2 days without reversing weekly growth.
			dayOrders := daily
			if j >= demoCurrentWindowLen-2 && dayOrders > 1 {
				dayOrders = dayOrders - 1
			}
			current[j] = newWindowDay(windowStart+wj, windowDates[wj], dayOrders, price)
			longTailWeekly += current[j].Revenue
		}
		pp.Window = prependPreviousWindow(current, windowStart, windowDates, curOffset, price, 1+demoCurrentVsPreviousGrowth)
	}

	scenarioWeekly := 0.0
	for i := range plan.Products {
		switch plan.Products[i].Scenario {
		case scenarioOrderDrop, scenarioStockRisk, scenarioNoSales:
			for j := curOffset; j < len(plan.Products[i].Window); j++ {
				scenarioWeekly += plan.Products[i].Window[j].Revenue
			}
		}
	}

	otherWeekly := longTailWeekly + scenarioWeekly
	keyTotalWeekly := (keySkuAccountRevenueShare / (1 - keySkuAccountRevenueShare)) * otherWeekly
	weightSum := 0.0
	for _, sc := range mvpKeySkuDemoSKUs {
		weightSum += sc.RevenueWeight
	}
	if weightSum <= 0 {
		return nil, fmt.Errorf("key-sku demo weights must sum > 0")
	}
	for _, sc := range mvpKeySkuDemoSKUs {
		idx, err := resolve(sc.ProductName, "key-sku")
		if err != nil {
			return nil, err
		}
		if err := claim(idx, scenarioKeySku); err != nil {
			return nil, err
		}
		pp := &plan.Products[idx]
		pp.ReferencePriceOverride = sc.BasePrice
		products[idx].ReferencePrice = sc.BasePrice
		products[idx].Segment = segLeaders

		weekly := (sc.RevenueWeight / weightSum) * keyTotalWeekly
		totalOrders := int(math.Round(weekly / sc.BasePrice))
		if totalOrders < demoCurrentWindowLen {
			totalOrders = demoCurrentWindowLen
		}
		perDay := mvpDistributeOrders(totalOrders, sc.OrdersWeight7D, demoCurrentWindowLen)
		current := make([]MVPWindowDaySKU, demoCurrentWindowLen)
		var orderSum int
		for j := 0; j < demoCurrentWindowLen && j < windowLen; j++ {
			wj := curOffset + j
			current[j] = newWindowDay(windowStart+wj, windowDates[wj], perDay[j], sc.BasePrice)
			orderSum += perDay[j]
		}
		// Stock = average daily orders × CoverageDays so key SKUs land in the
		// intended band instead of collapsing into critical under Pareto volume.
		avgDaily := float64(orderSum) / float64(demoCurrentWindowLen)
		if avgDaily < 1 {
			avgDaily = 1
		}
		cover := sc.CoverageDays
		if cover <= 0 {
			cover = 28
		}
		pp.StockOverride = int(math.Round(avgDaily * cover))
		if pp.StockOverride < 1 {
			pp.StockOverride = 1
		}
		pp.Window = prependPreviousWindow(current, windowStart, windowDates, curOffset, sc.BasePrice, 1+demoCurrentVsPreviousGrowth)
	}

	return plan, nil
}

// prependPreviousWindow builds previous_7d days as a scaled copy of current so
// account current/previous deltas land near demoCurrentVsPreviousGrowth.
func prependPreviousWindow(current []MVPWindowDaySKU, windowStart int, windowDates []time.Time, curOffset int, unitPrice float64, growthFactor float64) []MVPWindowDaySKU {
	if growthFactor < 1.05 {
		growthFactor = 1.05
	}
	if growthFactor > 1.40 {
		growthFactor = 1.40
	}
	prevLen := curOffset
	if prevLen <= 0 {
		return current
	}
	out := make([]MVPWindowDaySKU, 0, prevLen+len(current))
	for j := 0; j < prevLen; j++ {
		src := current[j%len(current)]
		prevOrders := int(math.Round(float64(src.OrdersCount) / growthFactor))
		if src.OrdersCount > 0 && prevOrders < 1 {
			prevOrders = 1
		}
		out = append(out, newWindowDay(windowStart+j, windowDates[j], prevOrders, unitPrice))
	}
	return append(out, current...)
}

// prependElevatedPreviousWindow writes a previous week that is elevateFactor× higher
// than the current week (for intentional WoW order-drop demo SKUs).
func prependElevatedPreviousWindow(current []MVPWindowDaySKU, windowStart int, windowDates []time.Time, curOffset int, unitPrice float64, elevateFactor float64) []MVPWindowDaySKU {
	if elevateFactor < 1.2 {
		elevateFactor = 1.2
	}
	if elevateFactor > 2.0 {
		elevateFactor = 2.0
	}
	prevLen := curOffset
	if prevLen <= 0 {
		return current
	}
	out := make([]MVPWindowDaySKU, 0, prevLen+len(current))
	for j := 0; j < prevLen; j++ {
		src := current[j%len(current)]
		prevOrders := int(math.Round(float64(src.OrdersCount) * elevateFactor))
		if src.OrdersCount > 0 && prevOrders <= src.OrdersCount {
			prevOrders = src.OrdersCount + 2
		}
		if prevOrders < 1 {
			prevOrders = 1
		}
		out = append(out, newWindowDay(windowStart+j, windowDates[j], prevOrders, unitPrice))
	}
	return append(out, current...)
}

func newWindowDay(dayIndex int, date time.Time, orders int, unitPrice float64) MVPWindowDaySKU {
	if orders < 0 {
		orders = 0
	}
	units := orders
	revenue := 0.0
	if orders > 0 {
		revenue = mvpRoundMoney(float64(orders) * unitPrice)
	}
	return MVPWindowDaySKU{
		DayIndex:    dayIndex,
		Date:        date,
		OrdersCount: orders,
		Units:       units,
		UnitPrice:   mvpRoundMoney(unitPrice),
		Revenue:     revenue,
	}
}

func mvpLongTailDailyOrders(p mvpProduct) int {
	expected := 0.55 * segmentScale(p.Segment)
	orders := int(math.Round(expected))
	if orders < 0 {
		orders = 0
	}
	if orders > 12 {
		orders = 12
	}
	return orders
}

func mvpDistributeOrders(total int, weights [7]float64, windowLen int) []int {
	out := make([]int, windowLen)
	if windowLen <= 0 {
		return out
	}
	sum := 0.0
	for j := 0; j < windowLen; j++ {
		sum += weights[7-windowLen+j]
	}
	if sum <= 0 {
		for j := range out {
			out[j] = total / windowLen
		}
		out[windowLen-1] += total - (total/windowLen)*windowLen
		return out
	}
	assigned := 0
	for j := 0; j < windowLen-1; j++ {
		w := weights[7-windowLen+j] / sum
		d := int(math.Round(float64(total) * w))
		if d < 1 {
			d = 1
		}
		out[j] = d
		assigned += d
	}
	last := total - assigned
	if last < 1 {
		last = 1
	}
	out[windowLen-1] = last
	return out
}
