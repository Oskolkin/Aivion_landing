package weeklychangepolicy

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

// Related health driver IDs (mirrors healthpolicy.KnownDriverIDs — duplicated
// here to avoid an import cycle through dashboardhome).
const (
	driverStockout                   = "stockout"
	driverKeySKUStockRisk            = "key_sku_stock_risk"
	driverNoSalesWithStock           = "no_sales_with_stock"
	driverSurplus                    = "surplus"
	driverHighDRRCampaign            = "high_drr_campaign"
	driverCampaignSpendWithoutOrders = "campaign_spend_without_orders"
	driverViewToCartDecline          = "view_to_cart_decline"
	driverCartToOrderDecline         = "cart_to_order_decline"
)

// conditionSnapshot is the current-period measurable state used for classification.
type conditionSnapshot struct {
	metricCode      string
	domain          string
	entityType      string
	value           float64
	present         bool // condition is "on" (problem exists / metric material)
	entityIDs       []string
	entityNames     []string
	financialImpact float64
	factIDs         []string
	relatedDriver   string
	severity        string
	period          string
	unit            string // "money", "count", "pct", "pp"
	isProblem       bool   // problem-shaped (vs plain KPI)
	higherIsWorse   bool   // for worsened/improved direction on continuous metrics
	// inherentlyPersistent is true when the fact itself proves multi-week
	// duration (e.g. days_without_sales >= 14). Such problems may be classified
	// as active_problem even without a previous publication.
	inherentlyPersistent bool
}

// BuildCandidateRegistry builds the closed weekly-change candidate set.
// previous may be nil. prevCtx.Available controls active_problem eligibility.
func BuildCandidateRegistry(
	current dashboardfacts.DashboardFactsV1,
	previous *dashboardfacts.DashboardFactsV1,
	prevCtx PreviousContext,
) RegistryResult {
	if previous != nil && !prevCtx.HasPreviousFacts {
		fromFacts := BuildPreviousContextFromFacts(*previous)
		prevCtx = MergePreviousContext(prevCtx, fromFacts)
	}
	// Always allow KPI comparisons from implied WoW deltas without claiming
	// problem-history continuity.
	prevCtx = EnrichPreviousContextWithImpliedDeltas(prevCtx, current)

	currentPeriod, previousPeriod := periodLabels(current, prevCtx)
	conditions := extractConditions(current)

	var candidates []ChangeCandidate
	seen := map[string]struct{}{}
	add := func(c ChangeCandidate) {
		if c.ChangeID == "" {
			return
		}
		if _, ok := seen[c.ChangeID]; ok {
			return
		}
		seen[c.ChangeID] = struct{}{}
		candidates = append(candidates, c)
	}

	for _, cond := range conditions {
		classified := classifyCondition(cond, prevCtx, currentPeriod, previousPeriod)
		for _, c := range classified {
			add(c)
		}
	}

	candidates = SortCandidates(candidates)
	byType := map[ChangeType][]ChangeCandidate{
		ChangeWorsened:      {},
		ChangeNewRisk:       {},
		ChangeActiveProblem: {},
		ChangeImproved:      {},
	}
	for _, c := range candidates {
		byType[c.ChangeType] = append(byType[c.ChangeType], c)
	}

	history := prevCtx.HistoryAvailable || prevCtx.HasPreviousPublication || prevCtx.HasPreviousFacts
	empty := map[string]string{
		GroupKeyWorsened:       EmptyTextForGroup(GroupKeyWorsened, history),
		GroupKeyNewRisks:       EmptyTextForGroup(GroupKeyNewRisks, history),
		GroupKeyUnchangedRisks: EmptyTextForGroup(GroupKeyUnchangedRisks, history),
		GroupKeyImproved:       EmptyTextForGroup(GroupKeyImproved, history),
	}

	var limitations []string
	if previous != nil && strings.HasSuffix(strings.TrimSpace(previous.MetricDate), "_prev") {
		limitations = append(limitations, "previous_period_facts_cloned_from_current")
	}

	return RegistryResult{
		PolicyVersion:    PolicyVersion,
		HistoryAvailable: history,
		CurrentPeriod:    currentPeriod,
		PreviousPeriod:   previousPeriod,
		Candidates:       candidates,
		ByChangeType:     byType,
		EmptyTextByGroup: empty,
		Limitations:      limitations,
	}
}

func periodLabels(current dashboardfacts.DashboardFactsV1, prevCtx PreviousContext) (string, string) {
	cur := strings.TrimSpace(current.MetricDate)
	if p := current.AdEfficiency.Period; p != nil && strings.TrimSpace(p.DateFrom) != "" && strings.TrimSpace(p.DateTo) != "" {
		cur = fmt.Sprintf("%s–%s", p.DateFrom, p.DateTo)
	}
	prev := strings.TrimSpace(prevCtx.PeriodLabel)
	if prev == "" && prevCtx.HasPreviousFacts {
		prev = "предыдущий период"
	}
	return cur, prev
}

func extractConditions(facts dashboardfacts.DashboardFactsV1) []conditionSnapshot {
	out := make([]conditionSnapshot, 0, 16)
	account := facts.DashboardMetrics.Account
	period := facts.MetricDate
	if p := facts.AdEfficiency.Period; p != nil && p.DateFrom != "" && p.DateTo != "" {
		period = fmt.Sprintf("%s–%s", p.DateFrom, p.DateTo)
	}

	// Sales: week-over-week from account deltas (previous implied).
	if account.RevenueLast7Days > 0 || (account.RevenueWeekDelta.Pct != nil && *account.RevenueWeekDelta.Pct != 0) {
		cur := account.RevenueLast7Days
		prev := 0.0
		pct := 0.0
		if account.RevenueWeekDelta.Pct != nil {
			pct = *account.RevenueWeekDelta.Pct * 100
			if pct != -100 {
				prev = cur / (1 + *account.RevenueWeekDelta.Pct)
			}
		}
		if account.RevenueWeekDelta.Abs != 0 && prev == 0 {
			prev = cur - account.RevenueWeekDelta.Abs
		}
		out = append(out, conditionSnapshot{
			metricCode: MetricRevenue7d, domain: "sales", value: cur, present: true,
			financialImpact: math.Abs(cur - prev), factIDs: []string{"dashboard_metrics.account.revenue_last_7d", "dashboard_metrics.account.revenue_week_to_week_delta"},
			period: period, unit: "money", isProblem: false, higherIsWorse: false,
			entityType: "account",
		})
		_ = pct
	}

	orders7d := int64(0)
	for _, kpi := range facts.SellerPulse.KPIs {
		if kpi.Key == "orders" || kpi.Key == "orders_count" || kpi.Key == "orders_7d" {
			if kpi.Delta != nil {
				// Value is display string; use account if available.
			}
		}
	}
	// Prefer summing SKU orders_7d when account lacks explicit orders_last_7d.
	for _, sku := range facts.DashboardMetrics.SKUs {
		orders7d += sku.OrdersLast7Days
	}
	if orders7d == 0 {
		// Fall back to today's orders only when 7d aggregate is unavailable —
		// still emit if seller pulse has an orders delta.
		for _, kpi := range facts.SellerPulse.KPIs {
			key := strings.ToLower(strings.TrimSpace(kpi.Key))
			if (key == "orders" || key == "orders_count" || strings.Contains(key, "order")) && kpi.Delta != nil && kpi.Delta.Pct != nil {
				cur := float64(account.OrdersToday)
				if cur <= 0 {
					cur = math.Abs(kpi.Delta.Abs)
				}
				out = append(out, conditionSnapshot{
					metricCode: MetricOrders7d, domain: "sales", value: cur, present: true,
					financialImpact: 0, factIDs: []string{"seller_pulse.kpis[orders]"},
					period: period, unit: "count", isProblem: false, higherIsWorse: false,
					entityType: "account",
				})
				break
			}
		}
	} else {
		out = append(out, conditionSnapshot{
			metricCode: MetricOrders7d, domain: "sales", value: float64(orders7d), present: true,
			factIDs: []string{"dashboard_metrics.skus[].orders_last_7d"},
			period:  period, unit: "count", isProblem: false, higherIsWorse: false,
			entityType: "account",
		})
	}

	// Stockout with recent demand.
	{
		var ids, names []string
		var revenueAtRisk float64
		for key, rec := range facts.SKUFactsIndex.ByOzonProductID {
			if rec.StockAvailable > 0 || rec.Orders7d <= 0 {
				continue
			}
			ids = append(ids, key)
			if rec.ProductName != nil && strings.TrimSpace(*rec.ProductName) != "" {
				names = append(names, strings.TrimSpace(*rec.ProductName))
			}
			revenueAtRisk += rec.Revenue7d
		}
		sort.Strings(ids)
		names = uniquePreserveOrder(names)
		count := float64(len(ids))
		out = append(out, conditionSnapshot{
			metricCode: MetricStockoutSKUCount, domain: "stock", entityType: "sku",
			value: count, present: count > 0, entityIDs: ids, entityNames: names,
			financialImpact: revenueAtRisk,
			factIDs:         []string{"sku_facts_index.by_ozon_product_id[].stock_available", "sku_facts_index.by_ozon_product_id[].orders_7d"},
			relatedDriver:   driverStockout, severity: severityFromCount(int(count), 5, 3, 1),
			period: period, unit: "count", isProblem: true, higherIsWorse: true,
		})
	}

	// Critical / shortage key SKUs.
	if strings.EqualFold(strings.TrimSpace(facts.KeySKUCohort.Status), "available") {
		critical, shortage := 0, 0
		var ids, names []string
		var revenueAtRisk float64
		for _, item := range facts.KeySKUCohort.Items {
			rec, ok := facts.SKUFactsIndex.ByOzonProductID[strconv.FormatInt(item.OzonProductID, 10)]
			if !ok || rec.DaysOfCover == nil {
				continue
			}
			days := *rec.DaysOfCover
			if days <= 3 {
				critical++
				ids = append(ids, strconv.FormatInt(item.OzonProductID, 10))
				if item.ProductName != nil {
					names = append(names, strings.TrimSpace(*item.ProductName))
				}
				revenueAtRisk += item.Revenue7d
			} else if days <= 7 {
				shortage++
				ids = append(ids, strconv.FormatInt(item.OzonProductID, 10))
				if item.ProductName != nil {
					names = append(names, strings.TrimSpace(*item.ProductName))
				}
				revenueAtRisk += item.Revenue7d
			}
		}
		if critical > 0 {
			out = append(out, conditionSnapshot{
				metricCode: MetricCriticalSKUCount, domain: "stock", entityType: "sku",
				value: float64(critical), present: true, entityIDs: ids, entityNames: uniquePreserveOrder(names),
				financialImpact: revenueAtRisk, factIDs: []string{"key_sku_cohort.items[].days_of_cover"},
				relatedDriver: driverKeySKUStockRisk, severity: severityFromCount(critical, 3, 2, 1),
				period: period, unit: "count", isProblem: true, higherIsWorse: true,
			})
		}
		if shortage > 0 && critical == 0 {
			out = append(out, conditionSnapshot{
				metricCode: MetricShortageSKUCount, domain: "stock", entityType: "sku",
				value: float64(shortage), present: true, entityIDs: ids, entityNames: uniquePreserveOrder(names),
				financialImpact: revenueAtRisk, factIDs: []string{"key_sku_cohort.items[].days_of_cover"},
				relatedDriver: driverKeySKUStockRisk, severity: "medium",
				period: period, unit: "count", isProblem: true, higherIsWorse: true,
			})
		}
	}

	// No sales with stock.
	{
		var ids, names []string
		persistent := false
		for key, rec := range facts.SKUFactsIndex.ByOzonProductID {
			if rec.StockAvailable <= 0 {
				continue
			}
			if rec.Orders7d > 0 || rec.Orders > 0 {
				continue
			}
			ids = append(ids, key)
			if rec.ProductName != nil && strings.TrimSpace(*rec.ProductName) != "" {
				names = append(names, strings.TrimSpace(*rec.ProductName))
			}
			if rec.DaysWithoutSales != nil && int64(*rec.DaysWithoutSales) >= stockpolicy.PersistentNoSalesMinDays {
				persistent = true
			}
		}
		sort.Strings(ids)
		count := float64(len(ids))
		out = append(out, conditionSnapshot{
			metricCode: MetricNoSalesWithStock, domain: "no_sales", entityType: "sku",
			value: count, present: count > 0, entityIDs: ids, entityNames: uniquePreserveOrder(names),
			factIDs:       []string{"sku_facts_index.by_ozon_product_id[].orders_7d", "sku_facts_index.by_ozon_product_id[].stock_available"},
			relatedDriver: driverNoSalesWithStock, severity: severityFromCount(int(count), 6, 3, 1),
			period: period, unit: "count", isProblem: true, higherIsWorse: true,
			inherentlyPersistent: persistent,
		})
	}

	// Surplus / slow stock.
	{
		var ids, names []string
		var frozen float64
		persistent := false
		for _, sku := range facts.DashboardMetrics.SKUs {
			slow := sku.StockTurnoverGrade != nil && strings.EqualFold(strings.TrimSpace(*sku.StockTurnoverGrade), "slow")
			excess := sku.ExcessStockShare != nil && *sku.ExcessStockShare >= 0.3
			stale := sku.DaysWithoutSales != nil && int64(*sku.DaysWithoutSales) >= stockpolicy.PersistentNoSalesMinDays && sku.StockAvailable > 0
			if !slow && !excess && !stale {
				continue
			}
			ids = append(ids, strconv.FormatInt(sku.OzonProductID, 10))
			if sku.ProductName != nil {
				names = append(names, strings.TrimSpace(*sku.ProductName))
			}
			frozen += float64(sku.StockAvailable)
			if stale {
				persistent = true
			}
		}
		count := float64(len(ids))
		out = append(out, conditionSnapshot{
			metricCode: MetricSurplusSKUCount, domain: "surplus", entityType: "sku",
			value: count, present: count > 0, entityIDs: ids, entityNames: uniquePreserveOrder(names),
			financialImpact: frozen, factIDs: []string{"dashboard_metrics.skus[].stock_turnover_grade"},
			relatedDriver: driverSurplus, severity: severityFromCount(int(count), 5, 3, 1),
			period: period, unit: "count", isProblem: true, higherIsWorse: true,
			inherentlyPersistent: persistent,
		})
	}

	// Advertising: high DRR campaigns.
	if summary := facts.AdEfficiency.Summary; summary != nil {
		ids, names := campaignEntities(facts.AdEfficiency.HighDRRCampaigns)
		count := float64(summary.CampaignsHighDRRCount)
		if count == 0 {
			count = float64(len(ids))
		}
		out = append(out, conditionSnapshot{
			metricCode: MetricHighDRRCampaigns, domain: "advertising", entityType: "campaign",
			value: count, present: count > 0, entityIDs: ids, entityNames: names,
			financialImpact: summary.HighDRRSpend,
			factIDs:         []string{"ad_efficiency.summary.campaigns_high_drr_count", "ad_efficiency.summary.high_drr_spend"},
			relatedDriver:   driverHighDRRCampaign,
			severity:        severityFromCount(int(count), 3, 2, 1),
			period:          period, unit: "count", isProblem: true, higherIsWorse: true,
		})

		ids2, names2 := campaignEntities(facts.AdEfficiency.NoOrderCampaigns)
		noOrders := float64(summary.CampaignsNoOrdersCount)
		if noOrders == 0 {
			noOrders = float64(len(ids2))
		}
		out = append(out, conditionSnapshot{
			metricCode: MetricSpendWithoutOrders, domain: "advertising", entityType: "campaign",
			value: summary.SpendWithoutOrders, present: noOrders > 0 && summary.SpendWithoutOrders > 0,
			entityIDs: ids2, entityNames: names2, financialImpact: summary.SpendWithoutOrders,
			factIDs:       []string{"ad_efficiency.summary.campaigns_no_orders_count", "ad_efficiency.summary.spend_without_orders"},
			relatedDriver: driverCampaignSpendWithoutOrders,
			severity:      severityFromMoney(summary.SpendWithoutOrders, 20000, 8000, 2000),
			period:        period, unit: "money", isProblem: true, higherIsWorse: true,
		})

		// Ad share of store revenue — improvement when stable/down, worsen when up.
		if summary.TotalStoreRevenue7d > 0 && summary.TotalSpend7d >= 0 {
			share := (summary.TotalSpend7d / summary.TotalStoreRevenue7d) * 100
			out = append(out, conditionSnapshot{
				metricCode: MetricAdShareOfRevenue, domain: "advertising",
				value: share, present: true,
				factIDs: []string{"ad_efficiency.summary.total_spend_7d", "ad_efficiency.summary.total_store_revenue_7d"},
				period:  period, unit: "pct", isProblem: false, higherIsWorse: true,
				entityType: "account",
			})
		}
	}

	// Funnel conversions.
	for _, stage := range facts.Funnel.Stages {
		key := strings.ToLower(strings.TrimSpace(stage.Key))
		var metricCode, driver string
		switch key {
		case "add_to_cart", "cart", "view_to_cart":
			metricCode, driver = MetricFunnelViewToCart, driverViewToCartDecline
		case "orders", "order", "cart_to_order":
			metricCode, driver = MetricFunnelCartToOrder, driverCartToOrderDecline
		default:
			continue
		}
		if stage.ConversionFromPrevious == nil {
			continue
		}
		out = append(out, conditionSnapshot{
			metricCode: metricCode, domain: "funnel",
			value: *stage.ConversionFromPrevious, present: true,
			factIDs:       []string{fmt.Sprintf("funnel.stages[%s].conversion_from_previous", key)},
			relatedDriver: driver, period: period, unit: "pct", isProblem: false, higherIsWorse: false,
			entityType: "account",
		})
	}

	return out
}

func classifyCondition(cond conditionSnapshot, prevCtx PreviousContext, currentPeriod, previousPeriod string) []ChangeCandidate {
	prevVal, hasPrevMetric := prevCtx.Metrics[cond.metricCode]
	prevProblem, hasPrevProblem := prevCtx.Problems[cond.metricCode]
	history := prevCtx.HistoryAvailable || prevCtx.HasPreviousPublication || prevCtx.HasPreviousFacts

	// Continuous KPI deltas (sales / funnel / ad share).
	if !cond.isProblem {
		return classifyKPI(cond, prevVal, hasPrevMetric, history, currentPeriod, previousPeriod)
	}

	// Problem-shaped conditions.
	if !cond.present {
		// Problem gone → improved, only when we know it existed before.
		if history && ((hasPrevProblem && prevProblem.Present) || (hasPrevMetric && prevVal > 0)) {
			prev := prevVal
			if hasPrevProblem && prevProblem.Value > 0 {
				prev = prevProblem.Value
			}
			return []ChangeCandidate{buildCandidate(cond, ChangeImproved, &prev, currentPeriod, previousPeriod, history)}
		}
		return nil
	}

	// Problem present now.
	if !history {
		// Without problem-history we cannot classify new vs active. Previous
		// facts metric values (HasPreviousFacts) still allow worsened/active.
		if prevCtx.HasPreviousFacts && hasPrevMetric {
			return classifyProblemWithPrevious(cond, prevVal, prevVal > 0, currentPeriod, previousPeriod, history)
		}
		return nil
	}

	prevPresent := (hasPrevProblem && prevProblem.Present) || (hasPrevMetric && prevVal > 0)
	if !prevPresent {
		return []ChangeCandidate{buildCandidate(cond, ChangeNewRisk, nil, currentPeriod, previousPeriod, history)}
	}
	return classifyProblemWithPrevious(cond, preferFloat(prevProblem.Value, prevVal), true, currentPeriod, previousPeriod, history)
}

func classifyProblemWithPrevious(cond conditionSnapshot, prev float64, prevPresent bool, currentPeriod, previousPeriod string, history bool) []ChangeCandidate {
	if !prevPresent {
		return []ChangeCandidate{buildCandidate(cond, ChangeNewRisk, nil, currentPeriod, previousPeriod, history)}
	}
	cur := cond.value
	// Substantial worsening threshold: +1 entity or +15% relative, or money +10%.
	worsened := false
	switch cond.unit {
	case "count":
		worsened = cur >= prev+1 && (prev == 0 || (cur-prev)/math.Max(prev, 1) >= 0.15 || cur-prev >= 2)
		if cur > prev && int(cur) > int(prev) {
			// Even +1 campaign/SKU counts as worsened when material.
			worsened = true
		}
	case "money":
		worsened = cur > prev*1.1 && cur-prev >= 1000
	default:
		worsened = cur > prev
	}
	if worsened {
		return []ChangeCandidate{buildCandidate(cond, ChangeWorsened, &prev, currentPeriod, previousPeriod, history)}
	}
	// Persists without substantial worsening → active problem.
	return []ChangeCandidate{buildCandidate(cond, ChangeActiveProblem, &prev, currentPeriod, previousPeriod, history)}
}

func classifyKPI(cond conditionSnapshot, prev float64, hasPrev bool, history bool, currentPeriod, previousPeriod string) []ChangeCandidate {
	// Try SellerPulse / week delta embedded via Metrics map population by caller.
	// If no previous, attempt reverse from facts is already in Metrics when
	// BuildCandidateRegistry was given previous facts. For revenue we also
	// accept relative interpretation via financialImpact as abs delta.
	if !hasPrev {
		// Without previous KPI value we cannot claim improved/worsened.
		return nil
	}
	cur := cond.value
	if almostEqual(cur, prev) {
		// Stable ad share can be an "improved"/support signal when not rising.
		if cond.metricCode == MetricAdShareOfRevenue && !cond.higherIsWorse {
			return nil
		}
		if cond.metricCode == MetricAdShareOfRevenue {
			// Stable (not up) advertising load → improved support point.
			c := buildCandidate(cond, ChangeImproved, &prev, currentPeriod, previousPeriod, history)
			c.TitleCandidate = "Рекламная нагрузка кабинета стабильна"
			c.DescriptionCandidate = fmt.Sprintf(
				"Доля рекламных расходов к выручке осталась около %s за %s. Существенного роста рекламной нагрузки не произошло.",
				formatPct(cur), currentPeriod,
			)
			c.MaterialityScore = clampInt(c.MaterialityScore-10, 5, 90)
			return []ChangeCandidate{c}
		}
		return nil
	}
	delta := cur - prev
	rel := 0.0
	if prev != 0 {
		rel = (delta / math.Abs(prev)) * 100
	}

	worse := false
	better := false
	if cond.higherIsWorse {
		worse = delta > 0 && (math.Abs(rel) >= 3 || math.Abs(delta) >= meaningfulAbs(cond))
		better = delta < 0 && (math.Abs(rel) >= 3 || math.Abs(delta) >= meaningfulAbs(cond))
	} else {
		worse = delta < 0 && (math.Abs(rel) >= 5 || math.Abs(delta) >= meaningfulAbs(cond))
		better = delta > 0 && (math.Abs(rel) >= 5 || math.Abs(delta) >= meaningfulAbs(cond))
	}
	// Funnel: use pp thresholds (~1pp).
	if strings.HasPrefix(cond.metricCode, "funnel.") {
		worse = delta < -1
		better = delta > 1
	}

	switch {
	case worse:
		return []ChangeCandidate{buildCandidate(cond, ChangeWorsened, &prev, currentPeriod, previousPeriod, history)}
	case better:
		return []ChangeCandidate{buildCandidate(cond, ChangeImproved, &prev, currentPeriod, previousPeriod, history)}
	default:
		return nil
	}
}

func meaningfulAbs(cond conditionSnapshot) float64 {
	switch cond.unit {
	case "money":
		return 1000
	case "count":
		return 1
	case "pct", "pp":
		return 1
	default:
		return 0
	}
}

func buildCandidate(
	cond conditionSnapshot,
	changeType ChangeType,
	prev *float64,
	currentPeriod, previousPeriod string,
	history bool,
) ChangeCandidate {
	cur := cond.value
	var absDelta, relDelta *float64
	if prev != nil {
		d := cur - *prev
		absDelta = &d
		if *prev != 0 {
			r := (d / math.Abs(*prev)) * 100
			relDelta = &r
		}
	}

	id := fmt.Sprintf("%s:%s", changeType, cond.metricCode)
	c := ChangeCandidate{
		ChangeID:         id,
		Domain:           cond.domain,
		ChangeType:       changeType,
		MetricCode:       cond.metricCode,
		CurrentValue:     f64(cur),
		PreviousValue:    prev,
		AbsoluteDelta:    absDelta,
		RelativeDeltaPct: relDelta,
		CurrentPeriod:    currentPeriod,
		PreviousPeriod:   previousPeriod,
		EntityType:       cond.entityType,
		EntityIDs:        append([]string{}, cond.entityIDs...),
		EntityNames:      append([]string{}, cond.entityNames...),
		EntityCount:      len(cond.entityIDs),
		EvidenceFactIDs:  append([]string{}, cond.factIDs...),
		RelatedDriverID:  cond.relatedDriver,
		Severity:         cond.severity,
		Confidence:       "high",
		HistoryRequired:  changeType == ChangeActiveProblem || changeType == ChangeNewRisk,
	}
	if cond.financialImpact > 0 {
		c.FinancialImpact = f64(cond.financialImpact)
	}
	if changeType == ChangeActiveProblem && !history {
		c.Limitations = append(c.Limitations, "previous_publication_absent")
	}

	c.CurrentValueText = formatValue(cur, cond.unit, cond)
	if prev != nil {
		c.PreviousValueText = formatValue(*prev, cond.unit, cond)
		c.DeltaText = formatDelta(cur, *prev, cond)
	}
	c.ImpactText = formatImpact(cond)
	c.TitleCandidate, c.DescriptionCandidate = buildCopy(c, cond)
	c.MaterialityScore = scoreMateriality(c, cond)
	c.ActionLinkLabel = defaultActionLinkLabel(c)
	if c.EntityCount > 0 && c.EntityType == "sku" {
		c.DeepLink = "/app/actions"
	}
	return c
}

func scoreMateriality(c ChangeCandidate, cond conditionSnapshot) int {
	score := 20
	score += sevRank(c.Severity) * 12
	if c.FinancialImpact != nil {
		score += clampInt(int(math.Round(math.Abs(*c.FinancialImpact)/5000)), 0, 40)
	}
	score += clampInt(c.EntityCount*3, 0, 24)
	if c.RelativeDeltaPct != nil {
		score += clampInt(int(math.Round(math.Abs(*c.RelativeDeltaPct)/2)), 0, 20)
	}
	if cond.metricCode == MetricRevenue7d || cond.metricCode == MetricOrders7d {
		score += 15
	}
	return clampInt(score, 1, 100)
}

func preferFloat(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func severityFromCount(n, critical, high, medium int) string {
	switch {
	case n >= critical:
		return "critical"
	case n >= high:
		return "high"
	case n >= medium:
		return "medium"
	default:
		return "low"
	}
}

func severityFromMoney(v, critical, high, medium float64) string {
	switch {
	case v >= critical:
		return "critical"
	case v >= high:
		return "high"
	case v >= medium:
		return "medium"
	default:
		return "low"
	}
}

func campaignEntities(rows []dashboardfacts.AdCampaignDisplayRow) ([]string, []string) {
	ids := make([]string, 0, len(rows))
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, strconv.FormatInt(row.CampaignExternalID, 10))
		name := strings.TrimSpace(row.CampaignName)
		if name == "" || looksLikeNumericID(name) {
			name = "Кампания без названия"
		}
		names = append(names, name)
	}
	return ids, uniquePreserveOrder(names)
}

func uniquePreserveOrder(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// CandidatesByGroup returns sorted candidates for a home group key.
func (r RegistryResult) CandidatesByGroup(groupKey string) []ChangeCandidate {
	var t ChangeType
	switch groupKey {
	case GroupKeyWorsened:
		t = ChangeWorsened
	case GroupKeyNewRisks:
		t = ChangeNewRisk
	case GroupKeyUnchangedRisks:
		t = ChangeActiveProblem
	case GroupKeyImproved:
		t = ChangeImproved
	default:
		return nil
	}
	return SortCandidates(r.ByChangeType[t])
}

// TopN returns up to n candidates for a group.
func (r RegistryResult) TopN(groupKey string, n int) []ChangeCandidate {
	all := r.CandidatesByGroup(groupKey)
	if n <= 0 || len(all) <= n {
		return all
	}
	return all[:n]
}
