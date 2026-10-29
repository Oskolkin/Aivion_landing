package actions

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

func formatMetricDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func candidateID(actionType ActionType, metricDate time.Time, suffix string) string {
	return fmt.Sprintf("%s_%s_%s", actionType, formatMetricDate(metricDate), suffix)
}

func factID(candidateID, suffix string) string {
	return candidateID + "." + suffix
}

func entityGroupKey(candidateID string) string {
	return candidateID
}

func skuEntityID(ozonProductID int64) string {
	return strconv.FormatInt(ozonProductID, 10)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func ptrFloat(v float64) *float64 {
	return &v
}

func ptrInt64(v int64) *int64 {
	return &v
}

func derefStr(s *string, fallback string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return fallback
	}
	return *s
}

func limitFacts(facts []ActionCandidateFact, max int) []ActionCandidateFact {
	if len(facts) <= max {
		return facts
	}
	return facts[:max]
}

func limitExamples(examples []ActionCandidateExample, max int) []ActionCandidateExample {
	if len(examples) <= max {
		return examples
	}
	return examples[:max]
}

func limitCandidates(candidates []ActionCandidate, max int) []ActionCandidate {
	if len(candidates) <= max {
		return candidates
	}
	return candidates[:max]
}

func skuExample(row SKUMetricRow, metrics map[string]any) ActionCandidateExample {
	name := row.ProductName
	if strings.TrimSpace(name) == "" {
		name = "Товар"
	}
	return ActionCandidateExample{
		EntityType: "sku",
		EntityID:   skuEntityID(row.OzonProductID),
		Name:       name,
		Metrics:    metrics,
	}
}

func campaignExample(row AdCampaignMetricRow, metrics map[string]any) ActionCandidateExample {
	return ActionCandidateExample{
		EntityType: "campaign",
		EntityID:   row.CampaignExternalID,
		Name:       row.CampaignName,
		Metrics:    metrics,
	}
}

func sortStockReplenishmentRows(rows []StockReplenishmentRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.RevenueAtRisk7d != b.RevenueAtRisk7d {
			return a.RevenueAtRisk7d > b.RevenueAtRisk7d
		}
		if a.Revenue7d != b.Revenue7d {
			return a.Revenue7d > b.Revenue7d
		}
		return a.Orders7d > b.Orders7d
	})
}

func sortNoSalesRows(rows []NoSalesWithStockRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		af, bf := 0.0, 0.0
		if a.FrozenStockValue != nil {
			af = *a.FrozenStockValue
		}
		if b.FrozenStockValue != nil {
			bf = *b.FrozenStockValue
		}
		if af != bf {
			return af > bf
		}
		return a.StockAvailable > b.StockAvailable
	})
}

func sortOrdersDropRows(rows []SKUOrdersDropRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.OrdersDelta != b.OrdersDelta {
			return a.OrdersDelta < b.OrdersDelta
		}
		if a.RevenueDelta != b.RevenueDelta {
			return a.RevenueDelta < b.RevenueDelta
		}
		return a.OzonProductID < b.OzonProductID
	})
}

func sortKeySKURiskRows(rows []KeySKURiskRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.RiskScore != b.RiskScore {
			return a.RiskScore > b.RiskScore
		}
		ash, bsh := 0.0, 0.0
		if a.RevenueSharePct != nil {
			ash = *a.RevenueSharePct
		}
		if b.RevenueSharePct != nil {
			bsh = *b.RevenueSharePct
		}
		return ash > bsh
	})
}

func sortHighDRRRows(rows []AdCampaignMetricRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Spend7d != b.Spend7d {
			return a.Spend7d > b.Spend7d
		}
		ad, bd := 0.0, 0.0
		if a.DRRPct != nil {
			ad = *a.DRRPct
		}
		if b.DRRPct != nil {
			bd = *b.DRRPct
		}
		return ad > bd
	})
}

func sortNoOrderCampaignRows(rows []AdCampaignMetricRow) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Spend7d > rows[j].Spend7d
	})
}

func calcRevenueAtRisk7d(avgDailyRevenue float64, coverageDays float64) float64 {
	if avgDailyRevenue <= 0 || coverageDays >= 7 {
		return 0
	}
	return round2(avgDailyRevenue * math.Max(0, 7-coverageDays))
}

func avgDailyRevenue7d(revenue7d float64) float64 {
	if revenue7d <= 0 {
		return 0
	}
	return revenue7d / 7
}

func frozenStockValue(stock int32, unitPrice float64) *float64 {
	if stock <= 0 || unitPrice <= 0 {
		return nil
	}
	v := round2(float64(stock) * unitPrice)
	return &v
}

func unitPriceFromRow(row SKUMetricRow) float64 {
	if row.OrdersCount > 0 && row.Revenue > 0 {
		return row.Revenue / float64(row.OrdersCount)
	}
	if row.Orders7d > 0 && row.Revenue7d > 0 {
		return row.Revenue7d / float64(row.Orders7d)
	}
	return 0
}

const keySkuRevenueShareThresholdPct = 80.0

func selectKeySKUIDs(rows []SKUMetricRow, storeRevenue7d float64) map[int64]bool {
	out := make(map[int64]bool)
	if storeRevenue7d <= 0 {
		return out
	}
	type candidate struct {
		id       int64
		revenue7 float64
	}
	candidates := make([]candidate, 0)
	for _, row := range rows {
		if row.Revenue7d <= 0 {
			continue
		}
		candidates = append(candidates, candidate{id: row.OzonProductID, revenue7: row.Revenue7d})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].revenue7 == candidates[j].revenue7 {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].revenue7 > candidates[j].revenue7
	})
	var cumulative float64
	for _, c := range candidates {
		share := (c.revenue7 / storeRevenue7d) * 100
		cumulative += share
		out[c.id] = true
		if cumulative >= keySkuRevenueShareThresholdPct {
			break
		}
	}
	return out
}
