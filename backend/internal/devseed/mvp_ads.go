package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adCampaignArchetype string

const (
	adArchetypeGoodROAS      adCampaignArchetype = "good_roas"
	adArchetypeWeakROAS      adCampaignArchetype = "weak_roas"
	adArchetypeSpendNoResult adCampaignArchetype = "spend_no_result"
	adArchetypeLowStockPush  adCampaignArchetype = "low_stock_push"
	adArchetypePausedHistory adCampaignArchetype = "paused_history"
	adArchetypeAdWasteHeavy  adCampaignArchetype = "ad_waste_heavy"
	adArchetypeGoodLeaders   adCampaignArchetype = "good_leaders"
	adArchetypeMixedSearch   adCampaignArchetype = "mixed_search"
)

type adCampaignDef struct {
	ExternalID   int64
	Name         string
	Status       string // running | paused
	CampaignType string
	Placement    string
	Archetype    adCampaignArchetype
	Strategy     string
	Audience     string
	BidMode      string
}

func campaignExternalID(seed, sellerID int64, slot int) int64 {
	return 882_000_000_000 + sellerID%90_000*1_000 + seed%10_000 + int64(slot)*10_007
}

func productSegmentFromRaw(raw []byte) mvpCommerceSegment {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m["segment"].(string)
	return mvpCommerceSegment(s)
}

func productCategoryFromRaw(raw []byte) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m["category"].(string)
	return s
}

func stockAvailableByProduct(stocks []dbgen.Stock) map[int64]int64 {
	out := make(map[int64]int64)
	for _, st := range stocks {
		pid, err := parseProductExternalIDInt(st.ProductExternalID)
		if err != nil {
			continue
		}
		if st.QuantityAvailable.Valid {
			out[pid] += int64(st.QuantityAvailable.Int32)
		}
	}
	return out
}

func parseProductExternalIDInt(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func buildAdCampaignDefs(seed, sellerID int64) []adCampaignDef {
	return []adCampaignDef{
		{campaignExternalID(seed, sellerID, 0), "Поиск — товары для кухни", "running", "search_promo", "search", adArchetypeGoodROAS, "max_visibility", "kitchen_intenders", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 1), "Продвижение лидеров продаж", "running", "sku_promo", "search", adArchetypeGoodLeaders, "target_roas", "high_intent_search", "cpc_manual"},
		{campaignExternalID(seed, sellerID, 2), "Рекомендации — домашний декор", "running", "display", "recommendations", adArchetypeWeakROAS, "awareness", "decor_lookalike", "cpm"},
		{campaignExternalID(seed, sellerID, 3), "Поиск — органайзеры и хранение", "running", "search_promo", "search", adArchetypeMixedSearch, "traffic", "broad_match", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 4), "Холодный трафик — без заказов", "running", "search_promo", "search", adArchetypeSpendNoResult, "exploration", "cold_audience", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 5), "Реклама при низком остатке", "running", "sku_promo", "search", adArchetypeGoodROAS, "stock_clearance", "remarketing_views", "cpc_max"},
		{campaignExternalID(seed, sellerID, 6), "Пауза — сезонное промо", "paused", "search_promo", "search", adArchetypePausedHistory, "seasonal", "holiday_shoppers", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 7), "Агрессивный поиск — слабая отдача", "running", "search_promo", "search", adArchetypeAdWasteHeavy, "aggressive", "category_in_market", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 8), "Универсальное продвижение", "running", "display", "recommendations", adArchetypeMixedSearch, "balanced", "mixed_segments", "cpc_auto"},
		{campaignExternalID(seed, sellerID, 9), "Поиск — текстиль для дома", "running", "search_promo", "search", adArchetypeGoodROAS, "profit", "textile_buyers", "cpc_manual"},
	}
}

// SeedMVPAds writes Performance-style campaigns, daily metrics, and SKU links.
func SeedMVPAds(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, opts MVPSeedOptions) (*AdsSeedStats, error) {
	stats := &AdsSeedStats{}
	rng := rand.New(rand.NewSource(opts.Seed + sellerAccountID*31 + 701))
	anchor := MVPDateUTC(opts.AnchorDate)

	products, err := q.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products for ads: %w", err)
	}
	if len(products) == 0 {
		return stats, nil
	}

	stocks, err := q.ListStocksBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list stocks for ads: %w", err)
	}
	stockByProduct := stockAvailableByProduct(stocks)

	defs := buildAdCampaignDefs(opts.Seed, sellerAccountID)
	metricDays := 30 + int((opts.Seed+int64(sellerAccountID))%16)
	if metricDays > 45 {
		metricDays = 45
	}
	if opts.Days > 0 && metricDays > opts.Days {
		metricDays = opts.Days
	}
	metricDates, err := SeedDateRange(anchor, metricDays)
	if err != nil {
		return nil, err
	}

	for _, def := range defs {
		campRaw, _ := json.Marshal(map[string]any{
			"strategy":    def.Strategy,
			"audience":    def.Audience,
			"bid_mode":    def.BidMode,
			"seeded_by":   "dev-seed-mvp",
			"archetype":   string(def.Archetype),
			"campaign_id": def.ExternalID,
		})
		_, err := q.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:    sellerAccountID,
			CampaignExternalID: def.ExternalID,
			CampaignName:       def.Name,
			CampaignType:       textNV(def.CampaignType),
			PlacementType:      textNV(def.Placement),
			Status:             textNV(def.Status),
			BudgetAmount:       moneyNV(120_000 + float64(def.ExternalID%50_000)),
			BudgetDaily:        moneyNV(3500 + float64(def.ExternalID%4000)),
			RawAttributes:      campRaw,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert campaign %d: %w", def.ExternalID, err)
		}
		stats.Campaigns++

		for d, day := range metricDates {
			impr, clicks, spend, orders, revenue := adDailyMetricsForArchetype(
				def.Archetype, d, len(metricDates), rng, products, stockByProduct,
			)
			if clicks > impr {
				clicks = impr
			}
			if int64(orders) > clicks {
				orders = int32(clicks)
			}
			mRaw, _ := json.Marshal(map[string]any{
				"strategy":  def.Strategy,
				"audience":  def.Audience,
				"bid_mode":  def.BidMode,
				"seeded_by": "dev-seed-mvp",
				"day_index": d,
			})
			_, err := q.UpsertAdMetricDaily(ctx, dbgen.UpsertAdMetricDailyParams{
				SellerAccountID:    sellerAccountID,
				CampaignExternalID: def.ExternalID,
				MetricDate:         pgtype.Date{Time: day, Valid: true},
				Impressions:        impr,
				Clicks:             clicks,
				Spend:              moneyNV(spend),
				OrdersCount:        orders,
				Revenue:            moneyNV(revenue),
				RawAttributes:      mRaw,
			})
			if err != nil {
				return nil, fmt.Errorf("ad metric %d %s: %w", def.ExternalID, day.Format("2006-01-02"), err)
			}
			stats.MetricRows++
		}
	}

	if err := seedAdCampaignSKULinks(ctx, q, sellerAccountID, defs, products, stockByProduct, rng, stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// scaleDemoAdSpendToStoreShare keeps demo advertising load realistic after commerce
// metrics exist: store ad spend share over the last 7 days lands in ~20–34%,
// attributed ad revenue does not exceed store revenue, and attributed DRR stays
// near ~40% when possible.
func scaleDemoAdSpendToStoreShare(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, anchor time.Time) error {
	const (
		targetShare = 0.28
		maxShare    = 0.34
		minShare    = 0.20
		targetDRR   = 0.40 // attributed_drr = spend / attributed_revenue
	)
	from := anchor.AddDate(0, 0, -6)
	var storeRev, adSpend, adRev float64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT SUM(revenue::float8) FROM daily_sku_metrics
			WHERE seller_account_id = $1 AND metric_date BETWEEN $2::date AND $3::date
		), 0),
		COALESCE((
			SELECT SUM(spend::float8) FROM ad_metrics_daily
			WHERE seller_account_id = $1 AND metric_date BETWEEN $2::date AND $3::date
		), 0),
		COALESCE((
			SELECT SUM(revenue::float8) FROM ad_metrics_daily
			WHERE seller_account_id = $1 AND metric_date BETWEEN $2::date AND $3::date
		), 0)
	`, sellerAccountID, from, anchor).Scan(&storeRev, &adSpend, &adRev)
	if err != nil {
		return err
	}
	if storeRev <= 0 || adSpend <= 0 {
		return nil
	}
	spendFactor := 1.0
	share := adSpend / storeRev
	if share < minShare || share > maxShare {
		spendFactor = targetShare / share
	}
	scaledSpend := adSpend * spendFactor
	targetAttrRev := scaledSpend / targetDRR
	if targetAttrRev > storeRev {
		targetAttrRev = storeRev
	}
	revFactor := 1.0
	if adRev > 0 {
		revFactor = targetAttrRev / adRev
	}
	if math.Abs(spendFactor-1) < 0.02 && math.Abs(revFactor-1) < 0.02 {
		return nil
	}
	_, err = pool.Exec(ctx, `
		UPDATE ad_metrics_daily
		SET spend = ROUND((spend::numeric * $4::numeric), 2),
		    revenue = ROUND((revenue::numeric * $5::numeric), 2),
		    updated_at = NOW()
		WHERE seller_account_id = $1
		  AND metric_date BETWEEN $2::date AND $3::date
	`, sellerAccountID, from, anchor, spendFactor, revFactor)
	return err
}

func adDailyMetricsForArchetype(
	arche adCampaignArchetype,
	dayIdx, metricDays int,
	rng *rand.Rand,
	products []dbgen.Product,
	stockBy map[int64]int64,
) (impr int64, clicks int64, spend float64, orders int32, revenue float64) {
	avgPrice := 1200.0
	if len(products) > 0 {
		s := 0.0
		n := 0
		for _, p := range products {
			if p.ReferencePrice.Valid {
				s += numericToFloat64(p.ReferencePrice)
				n++
			}
		}
		if n > 0 {
			avgPrice = s / float64(n)
		}
	}

	impr = int64(1000 + rng.Intn(149_000))
	ctr := 0.004 + rng.Float64()*(0.05-0.004)
	clicks = int64(math.Round(float64(impr) * ctr))
	if clicks < 1 {
		clicks = 1
	}
	if clicks > impr {
		clicks = impr
	}

	switch arche {
	case adArchetypeSpendNoResult:
		clicks = int64(5 + rng.Intn(46))
		if clicks > impr {
			impr = clicks + int64(200+rng.Intn(2000))
		}
		spend = mvpRoundMoney(300 + float64(rng.Intn(5000)) + float64(clicks)*12)
		return impr, clicks, spend, 0, 0

	case adArchetypeGoodROAS, adArchetypeGoodLeaders:
		roas := 3.0 + rng.Float64()*2.0
		spend = mvpRoundMoney(400 + float64(rng.Intn(10_000)) + float64(clicks)*float64(8+rng.Intn(8)))
		revenue = mvpRoundMoney(spend * roas)
		orders = int32(math.Max(1, math.Round(revenue/(avgPrice*0.9))))
		if int64(orders) > clicks {
			orders = int32(clicks)
		}
		if orders < 1 {
			orders = 1
		}
		revenue = mvpRoundMoney(float64(orders) * avgPrice * (0.95 + rng.Float64()*0.2))
		spend = mvpRoundMoney(revenue / roas)
		return impr, clicks, spend, orders, revenue

	case adArchetypeWeakROAS:
		spend = mvpRoundMoney(500 + float64(rng.Intn(8000)) + float64(clicks)*10)
		roas := 0.35 + rng.Float64()*0.55
		revenue = mvpRoundMoney(spend * roas)
		orders = int32(math.Max(0, math.Min(float64(clicks), math.Round(revenue/(avgPrice*1.1)))))
		if revenue > 0 && orders < 1 {
			orders = 1
			revenue = mvpRoundMoney(math.Min(spend*0.9, float64(orders)*avgPrice*0.5))
		}
		return impr, clicks, spend, orders, revenue

	case adArchetypeAdWasteHeavy:
		impr = int64(40_000 + rng.Intn(110_000))
		ctr = 0.012 + rng.Float64()*0.02
		clicks = int64(math.Round(float64(impr) * ctr))
		if clicks < 20 {
			clicks = 20
		}
		if clicks > impr {
			impr = clicks + 1000
		}
		spend = mvpRoundMoney(2000 + float64(rng.Intn(12_000)) + float64(clicks)*18)
		revenue = mvpRoundMoney(spend * (0.08 + rng.Float64()*0.12))
		orders = int32(math.Max(1, math.Min(float64(clicks), math.Round(revenue/(avgPrice*1.2)))))
		if int64(orders) > clicks {
			orders = int32(clicks)
		}
		return impr, clicks, spend, orders, revenue

	case adArchetypeLowStockPush:
		spend = mvpRoundMoney(600 + float64(rng.Intn(6000)) + float64(clicks)*14)
		orders = int32(1 + rng.Intn(int(math.Min(float64(clicks), 8))))
		revenue = mvpRoundMoney(float64(orders) * avgPrice * (0.85 + rng.Float64()*0.15))
		return impr, clicks, spend, orders, revenue

	case adArchetypePausedHistory:
		spend = mvpRoundMoney(400 + float64(rng.Intn(7000)) + float64(clicks)*9)
		revenue = mvpRoundMoney(spend * (0.5 + rng.Float64()*0.4))
		orders = int32(math.Max(0, math.Min(float64(clicks), math.Round(revenue/(avgPrice*1.05)))))
		if orders < 1 && revenue > 200 {
			orders = 1
		}
		return impr, clicks, spend, orders, revenue

	case adArchetypeMixedSearch:
		roas := 1.4 + rng.Float64()*1.8
		spend = mvpRoundMoney(350 + float64(rng.Intn(9000)) + float64(clicks)*7)
		revenue = mvpRoundMoney(spend * roas * (0.9 + 0.1*math.Sin(float64(dayIdx))))
		orders = int32(math.Max(0, math.Min(float64(clicks), math.Round(revenue/(avgPrice*1.0)))))
		if orders < 1 && revenue > spend {
			orders = 1
		}
		return impr, clicks, spend, orders, revenue

	default:
		spend = mvpRoundMoney(float64(clicks) * 10)
		revenue = mvpRoundMoney(spend * 1.2)
		orders = int32(math.Min(float64(clicks), math.Max(1, math.Round(revenue/avgPrice))))
		return impr, clicks, spend, orders, revenue
	}
}

func seedAdCampaignSKULinks(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	defs []adCampaignDef,
	products []dbgen.Product,
	stockBy map[int64]int64,
	rng *rand.Rand,
	stats *AdsSeedStats,
) error {
	bySeg := make(map[mvpCommerceSegment][]dbgen.Product)
	var kitchen, decor, organizers, adWaste, lowStock, leadersLowStock []dbgen.Product
	for _, p := range products {
		seg := productSegmentFromRaw(p.RawAttributes)
		bySeg[seg] = append(bySeg[seg], p)
		cat := productCategoryFromRaw(p.RawAttributes)
		if cat == string(catKitchenStorage) {
			kitchen = append(kitchen, p)
		}
		if cat == string(catLightDecor) {
			decor = append(decor, p)
		}
		if cat == string(catHomeOrganization) || cat == string(catKitchenStorage) {
			organizers = append(organizers, p)
		}
		if seg == segAdWaste {
			adWaste = append(adWaste, p)
		}
		if seg == segLowStock {
			lowStock = append(lowStock, p)
		}
		if seg == segLeaders {
			st := stockBy[p.OzonProductID]
			if st <= 12 {
				leadersLowStock = append(leadersLowStock, p)
			}
		}
	}

	link := func(defIdx int, plist []dbgen.Product, limit int, active bool) error {
		if len(plist) == 0 || defIdx >= len(defs) {
			return nil
		}
		if limit > len(plist) {
			limit = len(plist)
		}
		def := defs[defIdx]
		for i := 0; i < limit; i++ {
			p := plist[i%len(plist)]
			raw, _ := json.Marshal(map[string]any{
				"strategy":  def.Strategy,
				"audience":  def.Audience,
				"bid_mode":  def.BidMode,
				"seeded_by": "dev-seed-mvp",
				"campaign":  def.Name,
				"segment":   string(productSegmentFromRaw(p.RawAttributes)),
			})
			st := textNV("active")
			if !active {
				st = textNV("paused")
			}
			_, err := q.UpsertAdCampaignSKU(ctx, dbgen.UpsertAdCampaignSKUParams{
				SellerAccountID:    sellerID,
				CampaignExternalID: def.ExternalID,
				OzonProductID:      p.OzonProductID,
				OfferID:            p.OfferID,
				Sku:                p.Sku,
				IsActive:           active,
				Status:             st,
				RawAttributes:      raw,
			})
			if err != nil {
				return fmt.Errorf("campaign sku %d product %d: %w", def.ExternalID, p.OzonProductID, err)
			}
			stats.SKULinks++
		}
		return nil
	}

	// 0: kitchen search — real kitchen SKUs
	if err := link(0, kitchen, 10, true); err != nil {
		return err
	}
	// 1: leaders
	if err := link(1, bySeg[segLeaders], 12, true); err != nil {
		return err
	}
	// 2: decor weak
	if err := link(2, decor, 6, true); err != nil {
		return err
	}
	// 3: organizers weak
	if err := link(3, organizers, 8, true); err != nil {
		return err
	}
	// 4: spend no result — random in-stock products
	mix := pickMixedProducts(products, 6, rng)
	if err := link(4, mix, len(mix), true); err != nil {
		return err
	}
	// 5: low stock advertised — low_stock segment + leaders with low inventory
	ls := append([]dbgen.Product{}, lowStock...)
	ls = append(ls, leadersLowStock...)
	if len(ls) == 0 {
		ls = pickLowStockCandidates(products, stockBy, 6)
	}
	if err := link(5, ls, 8, true); err != nil {
		return err
	}
	// 6: paused — still link SKUs (historical)
	if err := link(6, pickMixedProducts(products, 5, rng), 5, false); err != nil {
		return err
	}
	// 7: ad_waste heavy
	wasteList := adWaste
	if len(wasteList) < 6 {
		wasteList = pickMixedProducts(products, minInt(14, len(products)), rng)
	}
	if err := link(7, wasteList, minInt(14, maxInt(len(wasteList), 6)), true); err != nil {
		return err
	}
	// 8: mixed
	if err := link(8, pickMixedProducts(products, 10, rng), 10, true); err != nil {
		return err
	}
	// 9: textile good
	textile := filterCategory(products, string(catHomeTextile))
	if err := link(9, textile, 9, true); err != nil {
		return err
	}

	return nil
}

func filterCategory(products []dbgen.Product, cat string) []dbgen.Product {
	var out []dbgen.Product
	for _, p := range products {
		if productCategoryFromRaw(p.RawAttributes) == cat {
			out = append(out, p)
		}
	}
	return out
}

func pickMixedProducts(products []dbgen.Product, n int, rng *rand.Rand) []dbgen.Product {
	if n > len(products) {
		n = len(products)
	}
	out := make([]dbgen.Product, 0, n)
	perm := rng.Perm(len(products))
	for i := 0; i < n; i++ {
		out = append(out, products[perm[i]])
	}
	return out
}

func pickLowStockCandidates(products []dbgen.Product, stockBy map[int64]int64, n int) []dbgen.Product {
	var out []dbgen.Product
	for _, p := range products {
		if stockBy[p.OzonProductID] <= 8 {
			out = append(out, p)
			if len(out) >= n {
				break
			}
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
