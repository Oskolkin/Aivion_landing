package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func persistSKUEntityGroup(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	groupKey string,
	rows []SKUMetricRow,
	summary map[string]any,
) (*ActionEntityGroupRef, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, skuEntityID(row.OzonProductID))
	}
	rec, err := repo.UpsertActionEntityGroup(ctx, sellerAccountID, metricDate, groupKey, "sku", ids, summary)
	if err != nil {
		return nil, err
	}
	return entityGroupRefFromRecord(rec, summary), nil
}

func persistCampaignEntityGroup(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	groupKey string,
	rows []AdCampaignMetricRow,
	summary map[string]any,
) (*ActionEntityGroupRef, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.CampaignExternalID)
	}
	rec, err := repo.UpsertActionEntityGroup(ctx, sellerAccountID, metricDate, groupKey, "campaign", ids, summary)
	if err != nil {
		return nil, err
	}
	return entityGroupRefFromRecord(rec, summary), nil
}

func entityGroupRefFromRecord(rec *ActionEntityGroupRecord, summary map[string]any) *ActionEntityGroupRef {
	if rec == nil {
		return nil
	}
	count := 0
	var ids []string
	if len(rec.EntityIDsJSON) > 0 {
		_ = json.Unmarshal(rec.EntityIDsJSON, &ids)
		count = len(ids)
	}
	return &ActionEntityGroupRef{
		GroupKey:   rec.GroupKey,
		EntityType: rec.EntityType,
		Count:      count,
		Summary:    summary,
	}
}

func stockReplenishmentSKURows(rows []StockReplenishmentRow) []SKUMetricRow {
	out := make([]SKUMetricRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SKUMetricRow)
	}
	return out
}

func noSalesSKURows(rows []NoSalesWithStockRow) []SKUMetricRow {
	out := make([]SKUMetricRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SKUMetricRow)
	}
	return out
}

func ordersDropSKURows(rows []SKUOrdersDropRow) []SKUMetricRow {
	out := make([]SKUMetricRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SKUMetricRow)
	}
	return out
}

func keyRiskSKURows(rows []KeySKURiskRow) []SKUMetricRow {
	out := make([]SKUMetricRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SKUMetricRow)
	}
	return out
}

func formatCountFact(candidateID, suffix, label, value, unit string) ActionCandidateFact {
	return ActionCandidateFact{
		FactID: factID(candidateID, suffix),
		Kind:   "metric",
		Label:  label,
		Value:  value,
		Unit:   unit,
	}
}

func buildAccountSnapshot(row AccountSnapshotRow) AccountSnapshot {
	var snap AccountSnapshot
	if row.Revenue != nil {
		v := round2(*row.Revenue)
		snap.Revenue = &v
	}
	if row.Orders != nil {
		snap.Orders = row.Orders
	}
	if snap.Revenue != nil && snap.Orders != nil && *snap.Orders > 0 {
		aov := round2(*snap.Revenue / float64(*snap.Orders))
		snap.AOV = &aov
	}
	if row.RevenuePrev != nil && row.Revenue != nil {
		if delta := analyticsPercentDelta(*row.Revenue, *row.RevenuePrev); delta != nil {
			snap.RevenueDeltaPct = delta
		}
	}
	if row.OrdersPrev != nil && row.Orders != nil {
		if delta := analyticsPercentDelta(float64(*row.Orders), float64(*row.OrdersPrev)); delta != nil {
			snap.OrdersDeltaPct = delta
		}
	}
	if row.AdSpend7d != nil && row.Revenue7d != nil && *row.Revenue7d > 0 {
		drr := round2((*row.AdSpend7d / *row.Revenue7d) * 100)
		snap.DRR = &drr
	}
	return snap
}

func analyticsPercentDelta(current, previous float64) *float64 {
	if previous <= 0 {
		return nil
	}
	v := round2((current - previous) / previous * 100)
	return &v
}

func metricDateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func validateGroupKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty group key")
	}
	for _, ch := range key {
		if ch == '/' {
			return fmt.Errorf("group key must not contain '/'")
		}
	}
	return nil
}
