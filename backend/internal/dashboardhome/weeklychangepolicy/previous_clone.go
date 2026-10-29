package weeklychangepolicy

import (
	"sort"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

// BuildPreviousPeriodFactsClone builds a previous-period facts snapshot from
// current facts for continuity classification when a prior publication/facts
// artifact is unavailable.
//
// Contract (explicit, not invented numbers):
//   - KPI previous values still come from WoW deltas on current facts;
//   - problem continuity uses a reduced entity set derived from the SAME
//     current entities (drop newest half of stockout / high-DRR / no-order
//     campaigns) so worsened/new_risk/active stay mutually exclusive and
//     grounded in real entity IDs that exist today.
//
// Callers must only use this under demo/dev seed paths and must record
// Limitations=["previous_period_facts_cloned_from_current"] on the registry.
func BuildPreviousPeriodFactsClone(current dashboardfacts.DashboardFactsV1) dashboardfacts.DashboardFactsV1 {
	prev := current
	prev.MetricDate = current.MetricDate + "_prev"
	prev.FactsHash = ""
	prev.SKUFactsIndex.ByOzonProductID = cloneSKUIndex(current.SKUFactsIndex.ByOzonProductID)

	// Stockout: keep older half (by id) as previous stockouts; newer IDs become "new"/worsened.
	stockoutIDs := stockoutEntityIDs(current)
	sort.Strings(stockoutIDs)
	keep := len(stockoutIDs) / 2
	if keep < 1 && len(stockoutIDs) > 1 {
		keep = len(stockoutIDs) - 1
	}
	if keep > len(stockoutIDs) {
		keep = len(stockoutIDs)
	}
	keepSet := map[string]struct{}{}
	for _, id := range stockoutIDs[:keep] {
		keepSet[id] = struct{}{}
	}
	for id, rec := range prev.SKUFactsIndex.ByOzonProductID {
		if rec.StockAvailable > 0 || rec.Orders7d <= 0 {
			continue
		}
		if _, ok := keepSet[id]; ok {
			continue
		}
		// Restore stock so condition is absent in previous period.
		rec.StockAvailable = 5
		if rec.DaysOfCover == nil {
			d := 10.0
			rec.DaysOfCover = &d
		}
		prev.SKUFactsIndex.ByOzonProductID[id] = rec
	}

	// High DRR: keep first campaign only as previous.
	if len(current.AdEfficiency.HighDRRCampaigns) > 0 {
		prev.AdEfficiency.HighDRRCampaigns = append([]dashboardfacts.AdCampaignDisplayRow{}, current.AdEfficiency.HighDRRCampaigns[:1]...)
		if prev.AdEfficiency.Summary != nil {
			summary := *prev.AdEfficiency.Summary
			summary.CampaignsHighDRRCount = 1
			if summary.HighDRRSpend > 0 && current.AdEfficiency.Summary != nil && current.AdEfficiency.Summary.CampaignsHighDRRCount > 0 {
				summary.HighDRRSpend = summary.HighDRRSpend / float64(current.AdEfficiency.Summary.CampaignsHighDRRCount)
			}
			prev.AdEfficiency.Summary = &summary
		}
	}

	// Spend without orders: absent in previous → becomes new_risk when present now.
	prev.AdEfficiency.NoOrderCampaigns = nil
	if prev.AdEfficiency.Summary != nil {
		summary := *prev.AdEfficiency.Summary
		summary.CampaignsNoOrdersCount = 0
		summary.SpendWithoutOrders = 0
		prev.AdEfficiency.Summary = &summary
	}

	// Sales KPIs: use explicit previous-week revenue when present so clone does
	// not freeze current WoW deltas into "unchanged" improved classification.
	acc := prev.DashboardMetrics.Account
	if acc.PreviousWeekTotal > 0 {
		acc.RevenueLast7Days = acc.PreviousWeekTotal
		acc.RevenueWeekDelta = analytics.DashboardDelta{}
		prev.DashboardMetrics.Account = acc
	}
	for id, rec := range prev.SKUFactsIndex.ByOzonProductID {
		if rec.OrdersPrev7d > 0 {
			rec.Orders7d = rec.OrdersPrev7d
			prev.SKUFactsIndex.ByOzonProductID[id] = rec
		}
	}

	// No-sales / surplus / key-SKU critical: keep as-is so they classify as active.
	return prev
}

func cloneSKUIndex(in map[string]dashboardfacts.SKUFactRecord) map[string]dashboardfacts.SKUFactRecord {
	out := make(map[string]dashboardfacts.SKUFactRecord, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stockoutEntityIDs(facts dashboardfacts.DashboardFactsV1) []string {
	var ids []string
	for key, rec := range facts.SKUFactsIndex.ByOzonProductID {
		if rec.StockAvailable > 0 || rec.Orders7d <= 0 {
			continue
		}
		ids = append(ids, key)
	}
	if len(ids) == 0 {
		// Fallback to dashboard metrics SKUs with zero stock.
		for _, sku := range facts.DashboardMetrics.SKUs {
			if sku.StockAvailable == 0 && sku.OrdersLast7Days > 0 {
				ids = append(ids, strconv.FormatInt(sku.OzonProductID, 10))
			}
		}
	}
	return ids
}
