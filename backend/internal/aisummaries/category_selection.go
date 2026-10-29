package aisummaries

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type categoryPickStats struct {
	id        int64
	revenue   float64
	orders    int64
	withStock int64
	active    int64
	critN     int
	highN     int
	stockRisk bool
	lowCover  bool
	pressure  int
}

func alertSuggestsStockRisk(a dbgen.Alert) bool {
	t := strings.ToLower(a.AlertType + " " + a.AlertGroup + " " + a.Title + " " + a.Message)
	return strings.Contains(t, "stock") ||
		strings.Contains(t, "остат") ||
		strings.Contains(t, "склад") ||
		strings.Contains(t, "cover") ||
		strings.Contains(t, "избыт") ||
		strings.Contains(t, "fbo") ||
		strings.Contains(t, "оборачиваем")
}

func priorityCategory(st *categoryPickStats) bool {
	if st == nil {
		return false
	}
	if st.critN > 0 || st.highN > 0 {
		return true
	}
	if st.stockRisk {
		return true
	}
	if st.lowCover {
		return true
	}
	if st.pressure >= 2 {
		return true
	}
	return false
}

func lessCatAgg(a, b *categoryPickStats, priorityTier bool) bool {
	if priorityTier {
		scoreI := a.critN*1000 + a.highN*100 + a.pressure*10
		scoreJ := b.critN*1000 + b.highN*100 + b.pressure*10
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		if a.stockRisk != b.stockRisk {
			return a.stockRisk && !b.stockRisk
		}
		if a.lowCover != b.lowCover {
			return a.lowCover && !b.lowCover
		}
	}
	if a.revenue != b.revenue {
		return a.revenue > b.revenue
	}
	if a.orders != b.orders {
		return a.orders > b.orders
	}
	if a.withStock != b.withStock {
		return a.withStock > b.withStock
	}
	if a.active != b.active {
		return a.active > b.active
	}
	return a.id < b.id
}

func buildCategoryPickStatsMap(
	rollup []dbgen.ListDescriptionCategoryRollupForAISummaryRow,
	lowCover []pgtype.Int8,
	alerts []dbgen.Alert,
	recs []dbgen.Recommendation,
	skuToCat map[int64]int64,
) map[int64]*categoryPickStats {
	stats := make(map[int64]*categoryPickStats)

	touch := func(id int64) *categoryPickStats {
		st := stats[id]
		if st == nil {
			st = &categoryPickStats{id: id}
			stats[id] = st
		}
		return st
	}

	for _, r := range rollup {
		if !r.DescriptionCategoryID.Valid {
			continue
		}
		id := r.DescriptionCategoryID.Int64
		st := touch(id)
		st.revenue = numericToFloat(r.RevenueSum)
		st.orders = r.OrdersSum
		st.withStock = r.ProductsWithStockCount
		st.active = r.ActiveProductCount
	}

	for _, cid := range lowCover {
		if !cid.Valid {
			continue
		}
		touch(cid.Int64).lowCover = true
	}

	for _, a := range alerts {
		if !a.EntitySku.Valid {
			continue
		}
		cid, ok := skuToCat[a.EntitySku.Int64]
		if !ok {
			continue
		}
		st := touch(cid)
		st.pressure++
		switch strings.ToLower(strings.TrimSpace(a.Severity)) {
		case "critical":
			st.critN++
		case "high":
			st.highN++
		}
		if alertSuggestsStockRisk(a) {
			st.stockRisk = true
		}
	}

	for _, r := range recs {
		if !r.EntitySku.Valid {
			continue
		}
		cid, ok := skuToCat[r.EntitySku.Int64]
		if !ok {
			continue
		}
		touch(cid).pressure++
	}

	return stats
}

func sortPickedCategoryIDs(stats map[int64]*categoryPickStats, limit int, notes *[]string) []int64 {
	if len(stats) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(stats))
	for id := range stats {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ai, aj := stats[ids[i]], stats[ids[j]]
		pri, prj := priorityCategory(ai), priorityCategory(aj)
		if pri != prj {
			return pri && !prj
		}
		return lessCatAgg(ai, aj, pri)
	})
	if len(ids) > limit {
		if notes != nil {
			*notes = append(*notes, fmt.Sprintf("category_truncated_from_%d_to_%d", len(ids), limit))
		}
		ids = ids[:limit]
	}
	return ids
}

// selectCategoryIDsForAISummary returns up to limit category IDs with deterministic priority:
// high/critical alert categories, stock-risk signals, low days-of-cover, alert+recommendation pressure,
// then top categories by revenue / orders / in-stock product counts.
func selectCategoryIDsForAISummary(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate pgtype.Date,
	rollup []dbgen.ListDescriptionCategoryRollupForAISummaryRow,
	limit int,
	notes *[]string,
) ([]int64, error) {
	skuRows, err := q.ListProductSkuCategoryBySellerForAISummary(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	skuToCat := make(map[int64]int64, len(skuRows))
	for _, row := range skuRows {
		if !row.Sku.Valid || !row.DescriptionCategoryID.Valid {
			continue
		}
		skuToCat[row.Sku.Int64] = row.DescriptionCategoryID.Int64
	}

	lowCoverRows, err := q.ListLowCoverCategoryIdsForAISummary(ctx, dbgen.ListLowCoverCategoryIdsForAISummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return nil, err
	}

	alerts, err := q.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           500,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}
	recs, err := q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           500,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}

	stats := buildCategoryPickStatsMap(rollup, lowCoverRows, alerts, recs, skuToCat)
	return sortPickedCategoryIDs(stats, limit, notes), nil
}
