package pack

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// skuMetricDay is one day of daily_sku_metrics for a single SKU.
type skuMetricDay struct {
	Revenue        float64
	Orders         int64
	UnitsSold      int64
	StockAvailable int32
	HasStock       bool
}

// campaignMetricDay is one day of ad_metrics_daily for a single campaign.
type campaignMetricDay struct {
	Impressions int64
	Clicks      int64
	Spend       float64
	Orders      int64
	Revenue     float64
}

// stockAgg is the aggregated stock_analytics_snapshots position for a single SKU
// on the (single) loaded snapshot date.
type stockAgg struct {
	HasData             bool
	AvailableStockCount int64
	TransitStockCount   int64
	DaysWithoutSales    int32
	HasDaysWithoutSales bool
	TurnoverGrade       string
	SourceRowsCount     int
}

// skuMeta is normalized SKU identity/catalog metadata merged across all sources.
type skuMeta struct {
	SKU        string
	ProductID  string
	OfferID    string
	Title      string
	CategoryID string
}

// campaignMeta is normalized campaign identity metadata.
type campaignMeta struct {
	ExternalID int64
	Name       string
	Type       string
	Status     string
}

// campaignSKULink identifies one distinct (campaign, sku) pairing.
type campaignSKULink struct {
	CampaignExternalID int64
	SKU                string
}

// Indexes are the normalized, pre-aggregated lookups derived once from an
// AnalysisDataSnapshot and shared across all table builders.
type Indexes struct {
	SKUMeta     map[string]*skuMeta
	SKUUniverse []string // analysis-eligible SKUs, sorted ascending by numeric SKU value

	// DiscoveredSKUUniverse is the raw union before eligibility filtering (diagnostics).
	DiscoveredSKUUniverse []string
	Eligibility           SKUEligibilityDiagnostics

	SKUMetricsByDate map[string]map[string]skuMetricDay // sku -> date -> day

	StockBySKU map[string]stockAgg

	ProductInfoBySKU map[string]dbgen.OzonProductInfoSnapshot
	ProductBySKU     map[string]dbgen.Product

	CategoryByID map[int64]dbgen.DescriptionCategory

	CampaignMeta     map[int64]*campaignMeta
	CampaignUniverse []int64 // sorted ascending

	CampaignMetricsByDate map[int64]map[string]campaignMetricDay // campaign -> date -> day

	CampaignSKULinks []campaignSKULink // sorted (campaign, sku); eligible SKUs only
	CampaignSKUCount map[int64]int     // campaign -> distinct linked SKU count

	// campaignLinkedSKUs is the raw set of SKUs appearing in campaign mappings
	// (before eligibility filtering). Used only for eligibility business-source checks.
	campaignLinkedSKUs map[string]struct{}
	relatedOnlySKUs    map[string]struct{}

	AccountMetricsByDate map[string]dbgen.DailyAccountMetric
	FunnelByDate         map[string]dbgen.ProductAnalyticsDaily
}

// BuildIndexes derives all shared lookups from a loaded AnalysisDataSnapshot.
func BuildIndexes(ctx *analysisdata.AnalysisDataSnapshot) *Indexes {
	idx := &Indexes{
		SKUMeta:               map[string]*skuMeta{},
		SKUMetricsByDate:      map[string]map[string]skuMetricDay{},
		StockBySKU:            map[string]stockAgg{},
		ProductInfoBySKU:      map[string]dbgen.OzonProductInfoSnapshot{},
		ProductBySKU:          map[string]dbgen.Product{},
		CategoryByID:          map[int64]dbgen.DescriptionCategory{},
		CampaignMeta:          map[int64]*campaignMeta{},
		CampaignMetricsByDate: map[int64]map[string]campaignMetricDay{},
		CampaignSKUCount:      map[int64]int{},
		campaignLinkedSKUs:    map[string]struct{}{},
		relatedOnlySKUs:       map[string]struct{}{},
		AccountMetricsByDate:  map[string]dbgen.DailyAccountMetric{},
		FunnelByDate:          map[string]dbgen.ProductAnalyticsDaily{},
	}

	meta := func(sku string) *skuMeta {
		if sku == "" {
			return nil
		}
		m, ok := idx.SKUMeta[sku]
		if !ok {
			m = &skuMeta{SKU: sku}
			idx.SKUMeta[sku] = m
		}
		return m
	}

	for _, p := range ctx.Products {
		if !p.Sku.Valid {
			continue
		}
		sku := strconv.FormatInt(p.Sku.Int64, 10)
		m := meta(sku)
		if p.OzonProductID > 0 {
			m.ProductID = strconv.FormatInt(p.OzonProductID, 10)
		}
		if p.OfferID.Valid && m.OfferID == "" {
			m.OfferID = p.OfferID.String
		}
		if p.Name != "" && m.Title == "" {
			m.Title = p.Name
		}
		if p.DescriptionCategoryID.Valid && m.CategoryID == "" {
			m.CategoryID = strconv.FormatInt(p.DescriptionCategoryID.Int64, 10)
		}
	}

	for _, row := range ctx.SKUMetrics {
		if !row.Sku.Valid {
			continue
		}
		sku := strconv.FormatInt(row.Sku.Int64, 10)
		m := meta(sku)
		if row.OzonProductID > 0 && m.ProductID == "" {
			m.ProductID = strconv.FormatInt(row.OzonProductID, 10)
		}
		if row.OfferID.Valid && m.OfferID == "" {
			m.OfferID = row.OfferID.String
		}
		if row.ProductName.Valid && m.Title == "" {
			m.Title = row.ProductName.String
		}
		d := analysisdata.FormatDate(row.MetricDate.Time)
		if idx.SKUMetricsByDate[sku] == nil {
			idx.SKUMetricsByDate[sku] = map[string]skuMetricDay{}
		}
		day := idx.SKUMetricsByDate[sku][d]
		day.Revenue += numericToFloat(row.Revenue)
		day.Orders += int64(row.OrdersCount)
		if row.UnitsSold.Valid {
			day.UnitsSold += int64(row.UnitsSold.Int32)
		}
		day.StockAvailable = row.StockAvailable
		day.HasStock = true
		idx.SKUMetricsByDate[sku][d] = day
	}

	for _, snap := range ctx.ProductInfoSnapshots {
		if !snap.Sku.Valid {
			continue
		}
		sku := strconv.FormatInt(snap.Sku.Int64, 10)
		meta(sku)
		idx.ProductInfoBySKU[sku] = snap
		m := idx.SKUMeta[sku]
		if snap.Name.Valid && m.Title == "" {
			m.Title = snap.Name.String
		}
		if snap.OfferID.Valid && m.OfferID == "" {
			m.OfferID = snap.OfferID.String
		}
		if snap.ProductID.Valid && m.ProductID == "" {
			m.ProductID = strconv.FormatInt(snap.ProductID.Int64, 10)
		}
		if snap.DescriptionCategoryID.Valid && m.CategoryID == "" {
			m.CategoryID = strconv.FormatInt(snap.DescriptionCategoryID.Int64, 10)
		}
	}

	for _, p := range ctx.Products {
		if p.Sku.Valid {
			sku := strconv.FormatInt(p.Sku.Int64, 10)
			idx.ProductBySKU[sku] = p
		}
	}

	for _, row := range ctx.StockSnapshots {
		sku := strconv.FormatInt(row.Sku, 10)
		meta(sku)
		m := idx.SKUMeta[sku]
		if row.OfferID.Valid && m.OfferID == "" {
			m.OfferID = row.OfferID.String
		}
		if row.ProductName.Valid && m.Title == "" {
			m.Title = row.ProductName.String
		}
		agg := idx.StockBySKU[sku]
		agg.HasData = true
		agg.AvailableStockCount += int64(row.AvailableStockCount)
		if row.TransitStockCount.Valid {
			agg.TransitStockCount += int64(row.TransitStockCount.Int32)
		}
		if row.DaysWithoutSales.Valid {
			if !agg.HasDaysWithoutSales || row.DaysWithoutSales.Int32 > agg.DaysWithoutSales {
				agg.DaysWithoutSales = row.DaysWithoutSales.Int32
				agg.HasDaysWithoutSales = true
			}
		}
		agg.SourceRowsCount++
		grade := strings.TrimSpace(row.TurnoverGrade.String)
		if grade != "" {
			agg.TurnoverGrade = mostCriticalGrade(agg.TurnoverGrade, grade)
		}
		idx.StockBySKU[sku] = agg
	}

	for _, link := range ctx.CampaignSKUs {
		if !link.Sku.Valid {
			continue
		}
		sku := strconv.FormatInt(link.Sku.Int64, 10)
		meta(sku)
		idx.campaignLinkedSKUs[sku] = struct{}{}
		m := idx.SKUMeta[sku]
		if link.OfferID.Valid && m.OfferID == "" {
			m.OfferID = link.OfferID.String
		}
		if link.ProductTitle.Valid && m.Title == "" {
			m.Title = link.ProductTitle.String
		}
	}

	for _, row := range ctx.RelatedSKUs {
		req := strconv.FormatInt(row.RequestedSku, 10)
		rel := strconv.FormatInt(row.RelatedSku, 10)
		meta(req)
		meta(rel)
		// Mark as related-only until another source proves otherwise.
		idx.relatedOnlySKUs[req] = struct{}{}
		idx.relatedOnlySKUs[rel] = struct{}{}
	}

	for sku, m := range idx.SKUMeta {
		if m.Title == "" {
			m.Title = "SKU " + sku
		}
	}

	idx.DiscoveredSKUUniverse = sortedSKUs(idx.SKUMeta)
	idx.SKUUniverse, idx.Eligibility = BuildActionableSKUUniverse(idx, idx.DiscoveredSKUUniverse)
	eligibleSet := make(map[string]struct{}, len(idx.SKUUniverse))
	for _, sku := range idx.SKUUniverse {
		eligibleSet[sku] = struct{}{}
	}

	for _, c := range ctx.Categories {
		idx.CategoryByID[c.DescriptionCategoryID] = c
	}

	cmeta := func(id int64) *campaignMeta {
		m, ok := idx.CampaignMeta[id]
		if !ok {
			m = &campaignMeta{ExternalID: id}
			idx.CampaignMeta[id] = m
		}
		return m
	}
	for _, c := range ctx.Campaigns {
		m := cmeta(c.CampaignExternalID)
		if c.CampaignName != "" {
			m.Name = c.CampaignName
		}
		if c.CampaignType.Valid {
			m.Type = c.CampaignType.String
		}
		if c.Status.Valid {
			m.Status = c.Status.String
		}
	}
	for _, row := range ctx.AdMetrics {
		cmeta(row.CampaignExternalID)
		d := analysisdata.FormatDate(row.MetricDate.Time)
		if idx.CampaignMetricsByDate[row.CampaignExternalID] == nil {
			idx.CampaignMetricsByDate[row.CampaignExternalID] = map[string]campaignMetricDay{}
		}
		day := idx.CampaignMetricsByDate[row.CampaignExternalID][d]
		day.Impressions += row.Impressions
		day.Clicks += row.Clicks
		day.Spend += numericToFloat(row.Spend)
		day.Orders += int64(row.OrdersCount)
		day.Revenue += numericToFloat(row.Revenue)
		idx.CampaignMetricsByDate[row.CampaignExternalID][d] = day
	}

	linkSeen := map[string]struct{}{}
	skuSeenByCampaign := map[int64]map[string]struct{}{}
	for _, link := range ctx.CampaignSKUs {
		cmeta(link.CampaignExternalID)
		sku := ""
		if link.Sku.Valid {
			sku = strconv.FormatInt(link.Sku.Int64, 10)
		}
		// Keep AI pack referential integrity: drop links to non-eligible SKUs.
		if sku != "" {
			if _, ok := eligibleSet[sku]; !ok {
				continue
			}
		}
		key := strconv.FormatInt(link.CampaignExternalID, 10) + ":" + sku
		if _, dup := linkSeen[key]; dup {
			continue
		}
		linkSeen[key] = struct{}{}
		idx.CampaignSKULinks = append(idx.CampaignSKULinks, campaignSKULink{CampaignExternalID: link.CampaignExternalID, SKU: sku})
		if sku != "" {
			if skuSeenByCampaign[link.CampaignExternalID] == nil {
				skuSeenByCampaign[link.CampaignExternalID] = map[string]struct{}{}
			}
			skuSeenByCampaign[link.CampaignExternalID][sku] = struct{}{}
		}
	}
	for cid, skus := range skuSeenByCampaign {
		idx.CampaignSKUCount[cid] = len(skus)
	}

	sort.Slice(idx.CampaignSKULinks, func(i, j int) bool {
		a, b := idx.CampaignSKULinks[i], idx.CampaignSKULinks[j]
		if a.CampaignExternalID != b.CampaignExternalID {
			return a.CampaignExternalID < b.CampaignExternalID
		}
		return a.SKU < b.SKU
	})

	idx.CampaignUniverse = make([]int64, 0, len(idx.CampaignMeta))
	for id := range idx.CampaignMeta {
		idx.CampaignUniverse = append(idx.CampaignUniverse, id)
	}
	sort.Slice(idx.CampaignUniverse, func(i, j int) bool { return idx.CampaignUniverse[i] < idx.CampaignUniverse[j] })

	for _, row := range ctx.AccountMetrics {
		idx.AccountMetricsByDate[analysisdata.FormatDate(row.MetricDate.Time)] = row
	}
	for _, row := range ctx.FunnelRows {
		idx.FunnelByDate[analysisdata.FormatDate(row.MetricDate.Time)] = row
	}

	return idx
}

// sortedSKUs returns SKU keys sorted ascending by numeric value (fallback to string sort).
func sortedSKUs(m map[string]*skuMeta) []string {
	out := make([]string, 0, len(m))
	for sku := range m {
		out = append(out, sku)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aerr := strconv.ParseInt(out[i], 10, 64)
		bi, berr := strconv.ParseInt(out[j], 10, 64)
		if aerr == nil && berr == nil {
			return ai < bi
		}
		return out[i] < out[j]
	})
	return out
}

// turnoverGradePriority mirrors aidatapack's most-critical-first ordering.
var turnoverGradePriority = []string{
	"DEFICIT",
	"WAITING_FOR_SUPPLY",
	"NO_SALES",
	"RESTRICTED_NO_SALES",
	"SURPLUS",
	"POPULAR",
	"ACTUAL",
	"COLLECTING_DATA",
	"TURNOVER_GRADE_NONE",
	"UNSPECIFIED",
}

func gradeRank(g string) int {
	for i, candidate := range turnoverGradePriority {
		if candidate == g {
			return i
		}
	}
	return len(turnoverGradePriority)
}

func mostCriticalGrade(current, candidate string) string {
	if current == "" {
		return candidate
	}
	if gradeRank(candidate) < gradeRank(current) {
		return candidate
	}
	return current
}
