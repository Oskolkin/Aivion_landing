package devseed

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	DevOzonEnrichmentSeedSource = "dev_seed"
	OzonEnrichmentSeedScope     = "p0_dev_seed"
	enrichmentStatsLookbackDays = 13
	devRelatedSKUErrorCode      = "DEV_SKU_NOT_FOUND"
)

type OzonEnrichmentSeedScenario string

const (
	OzonEnrichmentScenarioRich    OzonEnrichmentSeedScenario = "rich"
	OzonEnrichmentScenarioMinimal OzonEnrichmentSeedScenario = "minimal"
	OzonEnrichmentScenarioEdge    OzonEnrichmentSeedScenario = "edge"
)

type enrichmentLocalSKU struct {
	OzonProductID         int64
	SKU                   int64
	OfferID               string
	Name                  string
	DescriptionCategoryID int64
	Price                 float64
	OldPrice              float64
	MinPrice              float64
	StockAvailable        int32
	DaysOfCover           float64
	OrdersCount           int32
	Revenue               float64
}

type productInfoVariant string

const (
	productVariantNormal     productInfoVariant = "normal"
	productVariantNoStock    productInfoVariant = "no_stock"
	productVariantValidation productInfoVariant = "validation_warning"
	productVariantPriceIndex productInfoVariant = "price_index"
)

type stockScenario string

const (
	stockScenarioDeficit          stockScenario = "DEFICIT"
	stockScenarioWaitingForSupply stockScenario = "WAITING_FOR_SUPPLY"
	stockScenarioSurplus          stockScenario = "SURPLUS"
	stockScenarioNoSales          stockScenario = "NO_SALES"
	stockScenarioActual           stockScenario = "ACTUAL"
)

type campaignEnrichmentKind string

const (
	campaignKindManualRunning campaignEnrichmentKind = "manual_running"
	campaignKindAutoRunning   campaignEnrichmentKind = "auto_running"
	campaignKindStoppedBudget campaignEnrichmentKind = "stopped_budget"
	campaignKindInactive      campaignEnrichmentKind = "inactive"
)

type adMetricsProfile string

const (
	adMetricsSpendWithoutOrders adMetricsProfile = "spend_without_orders"
	adMetricsHighDRR            adMetricsProfile = "high_drr"
	adMetricsHealthyROAS        adMetricsProfile = "healthy_roas"
	adMetricsStoppedOrInactive  adMetricsProfile = "stopped_or_inactive"
)

func devSeedEnvelope(scenario OzonEnrichmentSeedScenario, extra map[string]any) []byte {
	payload := map[string]any{
		"source":       DevOzonEnrichmentSeedSource,
		"scenario":     string(scenario),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"note":         "Synthetic P0 enrichment data for local AI Data Pack development. Not real Ozon API data.",
	}
	for k, v := range extra {
		payload[k] = v
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func devSeedWarningJSON() []byte {
	raw, _ := json.Marshal([]map[string]string{
		{
			"code":    "DEV_SEED_DATA",
			"message": "Synthetic P0 enrichment generated for local AI Data Pack development. Not real Ozon API data.",
		},
	})
	return raw
}

func hashSeed(parts ...int64) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = fmt.Fprintf(h, "%d|", p)
	}
	return h.Sum64()
}

func deterministicPrice(sku int64, sellerID int64) float64 {
	h := hashSeed(sku, sellerID, 17)
	base := 700.0 + float64(h%2801)
	return math.Round(base*100) / 100
}

func productInfoVariantForIndex(idx int) productInfoVariant {
	switch idx % 10 {
	case 8:
		return productVariantNoStock
	case 9:
		return productVariantValidation
	case 7:
		return productVariantPriceIndex
	default:
		return productVariantNormal
	}
}

func deriveStockScenarioFromFacts(availableStock int32, daysOfCover float64, ordersCount int32, orders7dHint int32) stockScenario {
	orders := orders7dHint
	if orders <= 0 {
		orders = ordersCount // single-day fallback when 7d not loaded
	}
	if availableStock <= 0 {
		return stockScenarioWaitingForSupply
	}
	// DaysOfCover > 0 means analytics saw sales velocity — never label as NO_SALES.
	if orders <= 0 && daysOfCover <= 0 {
		return stockScenarioNoSales
	}
	if daysOfCover > 0 {
		switch {
		case daysOfCover <= 3:
			return stockScenarioDeficit
		case daysOfCover <= 7:
			return stockScenarioDeficit
		case daysOfCover >= 60:
			return stockScenarioSurplus
		default:
			return stockScenarioActual
		}
	}
	// No coverage signal: infer from stock magnitude vs recent orders.
	if orders > 0 && availableStock <= int32(orders)*3 {
		return stockScenarioDeficit
	}
	if orders > 0 && availableStock >= int32(orders)*60 {
		return stockScenarioSurplus
	}
	if orders <= 0 && availableStock >= 30 {
		return stockScenarioNoSales
	}
	return stockScenarioActual
}

// reshapeEnrichmentItemForScenario adjusts cover/orders only so
// deriveStockScenarioFromFacts yields the intended grade (used by rich seed).
// Available stock must remain the stocks-table aggregate — never invent counts.
func reshapeEnrichmentItemForScenario(want stockScenario, item enrichmentLocalSKU) enrichmentLocalSKU {
	switch want {
	case stockScenarioWaitingForSupply:
		// Waiting requires availableStock <= 0; caller clears stocks rows.
	case stockScenarioDeficit:
		item.DaysOfCover = 2
		if item.OrdersCount < 1 {
			item.OrdersCount = 5
		}
	case stockScenarioSurplus:
		item.DaysOfCover = 90
		if item.OrdersCount < 1 {
			item.OrdersCount = 4
		}
	case stockScenarioNoSales:
		item.DaysOfCover = 0
		item.OrdersCount = 0
	case stockScenarioActual:
		item.DaysOfCover = 25
		if item.OrdersCount < 1 {
			item.OrdersCount = 5
		}
	}
	return item
}

// stockScenarioForIndex is retained for tests of historical idx%N distribution;
// production enrichment must call deriveStockScenarioFromFacts.
func stockScenarioForIndex(idx int) stockScenario {
	switch idx % 5 {
	case 0:
		return stockScenarioDeficit
	case 1:
		return stockScenarioWaitingForSupply
	case 2:
		return stockScenarioSurplus
	case 3:
		return stockScenarioNoSales
	default:
		return stockScenarioActual
	}
}

func stockAnalyticsValues(scenario stockScenario, idx int, sku enrichmentLocalSKU, availableStock int32) map[string]any {
	out := map[string]any{
		"turnover_grade":        string(scenario),
		"warehouse_id":          int64(100 + idx%3),
		"warehouse_name":        fmt.Sprintf("Ozon WH %d", 100+idx%3),
		"cluster_id":            int64(10 + idx%2),
		"cluster_name":          fmt.Sprintf("Cluster %d", 10+idx%2),
		"available_stock_count": availableStock,
	}

	// ADS = average daily sales proxy; DaysOfCover on the SKU is coverage (IDC), not ADS.
	ads := 0.0
	if sku.OrdersCount > 0 {
		ads = float64(sku.OrdersCount)
	}
	if ads <= 0 && sku.DaysOfCover > 0 && availableStock > 0 {
		ads = float64(availableStock) / sku.DaysOfCover
	}
	if ads <= 0 {
		ads = 1
	}
	idc := sku.DaysOfCover
	if idc <= 0 && ads > 0 && availableStock > 0 {
		idc = float64(availableStock) / ads
	}

	switch scenario {
	case stockScenarioDeficit:
		out["ads"] = math.Max(ads, 1)
		out["idc"] = math.Min(idc, 3)
		out["requested_stock_count"] = int32(0)
		out["transit_stock_count"] = int32(0)
		out["days_without_sales"] = int32(0)
	case stockScenarioWaitingForSupply:
		out["ads"] = math.Max(ads, 1)
		out["idc"] = 0.0
		out["requested_stock_count"] = int32(20 + idx%40)
		out["transit_stock_count"] = int32(10 + idx%25)
		out["days_without_sales"] = int32(0)
	case stockScenarioSurplus:
		out["ads"] = math.Max(0.5, math.Min(ads, 3))
		out["idc"] = math.Max(idc, 60)
		out["excess_stock_count"] = int32(math.Max(0, float64(availableStock)/2))
		out["days_without_sales"] = int32(0)
	case stockScenarioNoSales:
		out["ads"] = 0.0
		out["idc"] = math.Max(idc, 30)
		out["days_without_sales"] = int32(28 + idx%14)
	default:
		out["ads"] = math.Max(ads, 1)
		out["idc"] = idc
		if out["idc"].(float64) < 8 {
			out["idc"] = 20.0 + float64(idx%20)
		}
		out["days_without_sales"] = int32(0)
	}
	if idx%7 == 0 {
		out["waiting_docs_stock_count"] = int32(3 + idx%5)
	}
	if idx%11 == 0 {
		out["return_from_customer_stock_count"] = int32(2 + idx%4)
	}
	if idx%13 == 0 {
		out["stock_defect_stock_count"] = int32(1 + idx%3)
	}
	if idx%17 == 0 {
		out["expiring_stock_count"] = int32(4 + idx%6)
	}
	out["valid_stock_count"] = availableStock
	return out
}

func campaignKindForIndex(idx int) campaignEnrichmentKind {
	switch idx % 4 {
	case 0:
		return campaignKindManualRunning
	case 1:
		return campaignKindAutoRunning
	case 2:
		return campaignKindStoppedBudget
	default:
		return campaignKindInactive
	}
}

func priceIndexColorForVariant(v productInfoVariant, idx int) string {
	if v == productVariantPriceIndex {
		if idx%2 == 0 {
			return "RED"
		}
		return "YELLOW"
	}
	return "GREEN"
}

func numericFromFloat(v float64) pgtype.Numeric {
	if v == 0 {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", v))
	return n
}

// numericMoneyAmount maps a ruble amount for NOT NULL money columns (0 is valid, not NULL).
func numericMoneyAmount(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", v))
	return n
}

func numericFromString(v string) pgtype.Numeric {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	if err := n.Scan(v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func pgText(v string) pgtype.Text {
	if strings.TrimSpace(v) == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: strings.TrimSpace(v), Valid: true}
}

func pgInt8(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func pgInt4(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}

func pgBool(v bool) pgtype.Bool {
	return pgtype.Bool{Bool: v, Valid: true}
}

func pgDateUTC(t time.Time) pgtype.Date {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: t, Valid: true}
}

func pgTimestamptzUTC(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func microRubles(amount float64) int64 {
	return int64(math.Round(amount * 1_000_000))
}

func fallbackSKU(ozonProductID int64, sellerID int64) int64 {
	if ozonProductID <= 0 {
		return 0
	}
	return 900_000_000_000 + sellerID%1000*1_000_000 + ozonProductID%1_000_000
}

func fallbackOfferID(sku int64) string {
	return fmt.Sprintf("DEV-OFFER-%d", sku)
}

func buildCommissionsJSON(price float64) []byte {
	raw, _ := json.Marshal([]map[string]any{
		{"sale_schema": "FBO", "percent": 15, "value": math.Round(price*0.15*10) / 10, "delivery_amount": 0, "return_amount": 0},
		{"sale_schema": "FBS", "percent": 13, "value": math.Round(price*0.13*10) / 10, "delivery_amount": 0, "return_amount": 0},
	})
	return raw
}

func buildPromotionsJSON(enabled bool) []byte {
	if !enabled {
		return []byte("[]")
	}
	raw, _ := json.Marshal([]map[string]any{{"is_enabled": true, "type": "DISCOUNT"}})
	return raw
}

func buildErrorsJSON(variant productInfoVariant) []byte {
	if variant != productVariantValidation {
		return []byte("[]")
	}
	raw, _ := json.Marshal([]map[string]any{
		{
			"level": "WARNING",
			"code":  "DEV_CONTENT_WARNING",
			"field": "attributes",
			"texts": map[string]string{
				"message":           "Не заполнена часть характеристик",
				"short_description": "Есть незаполненные характеристики",
				"attribute_name":    "Характеристики",
			},
		},
	})
	return raw
}

func buildAvailabilitiesJSON(sku int64, variant productInfoVariant) []byte {
	if variant == productVariantNoStock {
		raw, _ := json.Marshal([]map[string]any{
			{
				"availability": "UNAVAILABLE",
				"source":       DevOzonEnrichmentSeedSource,
				"sku":          sku,
				"reasons": []map[string]any{
					{"id": 1, "human_text": map[string]string{"text": "Нет доступного остатка"}},
				},
			},
		})
		return raw
	}
	raw, _ := json.Marshal([]map[string]any{
		{"availability": "AVAILABLE", "source": DevOzonEnrichmentSeedSource, "sku": sku, "reasons": []any{}},
	})
	return raw
}

func buildPriceIndexes(price float64, color string) (externalMin, externalIdx, ozonMin, ozonIdx, selfMin, selfIdx float64) {
	switch color {
	case "RED":
		externalMin = price * 0.82
		ozonMin = price * 0.85
		selfMin = price * 0.88
		externalIdx = 1.22
		ozonIdx = 1.18
		selfIdx = 1.14
	case "YELLOW":
		externalMin = price * 0.92
		ozonMin = price * 0.94
		selfMin = price * 0.96
		externalIdx = 1.09
		ozonIdx = 1.07
		selfIdx = 1.05
	default:
		externalMin = price * 1.02
		ozonMin = price * 1.01
		selfMin = price * 1.03
		externalIdx = 0.98
		ozonIdx = 0.99
		selfIdx = 0.97
	}
	return
}

func adMetricsProfileForIndex(idx int) adMetricsProfile {
	switch idx % 4 {
	case 0:
		return adMetricsSpendWithoutOrders
	case 1:
		return adMetricsHighDRR
	case 2:
		return adMetricsHealthyROAS
	default:
		return adMetricsStoppedOrInactive
	}
}

// dayAdMetricScales returns deterministic spend/revenue multipliers for a day
// offset from metric_date (0 = metric date). Spend and revenue move
// independently so daily attributed DRR is not flat across the window.
func dayAdMetricScales(daysBeforeMetric int) (spendScale, revenueScale float64) {
	if daysBeforeMetric < 0 {
		daysBeforeMetric = 0
	}
	// Fixed 7-day narrative ending on metric_date (index 0), then repeats for older lookback days.
	type pair struct{ spend, revenue float64 }
	week := []pair{
		{1.00, 1.00}, // metric day — baseline (~91.8% at base profiles)
		{1.08, 0.95}, // d-1 partial recovery from trough
		{1.20, 0.85}, // d-2 worst efficiency (high spend, low attributed rev)
		{1.15, 0.95}, // d-3 worsening
		{1.02, 1.20}, // d-4 strong attributed revenue
		{0.95, 1.08}, // d-5 healthy
		{0.88, 1.15}, // d-6 best attributed efficiency
	}
	p := week[daysBeforeMetric%len(week)]
	return p.spend, p.revenue
}

func dailyAdMetricsForProfile(profile adMetricsProfile) (impressions, clicks int64, spend, revenue float64, orders int32) {
	return dailyAdMetricsForProfileOnDay(profile, 0)
}

func dailyAdMetricsForProfileOnDay(profile adMetricsProfile, daysBeforeMetric int) (impressions, clicks int64, spend, revenue float64, orders int32) {
	spendScale, revenueScale := dayAdMetricScales(daysBeforeMetric)
	// Activity (impressions/clicks) tracks a blend so funnel-like signals stay coherent.
	activityScale := (spendScale + revenueScale) / 2

	var baseImp, baseClicks int64
	var baseSpend, baseRevenue float64
	var baseOrders int32
	switch profile {
	case adMetricsSpendWithoutOrders:
		baseImp, baseClicks, baseSpend, baseRevenue, baseOrders = 1200, 35, 1800, 0, 0
	case adMetricsHighDRR:
		baseImp, baseClicks, baseSpend, baseRevenue, baseOrders = 2000, 60, 5000, 2500, 1
	case adMetricsHealthyROAS:
		baseImp, baseClicks, baseSpend, baseRevenue, baseOrders = 3000, 95, 1500, 9000, 6
	case adMetricsStoppedOrInactive:
		baseImp, baseClicks, baseSpend, baseRevenue, baseOrders = 50, 2, 0, 0, 0
	default:
		baseImp, baseClicks, baseSpend, baseRevenue, baseOrders = 50, 2, 0, 0, 0
	}

	impressions = int64(math.Round(float64(baseImp) * activityScale))
	clicks = int64(math.Round(float64(baseClicks) * activityScale))
	if impressions < 0 {
		impressions = 0
	}
	if clicks < 0 {
		clicks = 0
	}
	spend = roundMoney2(baseSpend * spendScale)
	revenue = roundMoney2(baseRevenue * revenueScale)
	if baseRevenue == 0 {
		revenue = 0
	}
	if baseSpend == 0 {
		spend = 0
	}
	orders = baseOrders
	if baseOrders > 0 {
		scaled := int32(math.Round(float64(baseOrders) * revenueScale))
		if scaled < 1 {
			scaled = 1
		}
		orders = scaled
	}
	return impressions, clicks, spend, revenue, orders
}

func roundMoney2(v float64) float64 {
	return math.Round(v*100) / 100
}

// accountAttributedDRRForDay returns account-level attributed DRR for a synthetic
// 10-campaign enrichment layout on the given day offset (for tests/reconciliation).
func accountAttributedDRRForDay(campaignCount int, daysBeforeMetric int) (spendSum, revenueSum, drrPct float64) {
	for idx := 0; idx < campaignCount; idx++ {
		_, _, spend, revenue, _ := dailyAdMetricsForProfileOnDay(adMetricsProfileForIndex(idx), daysBeforeMetric)
		spendSum += spend
		revenueSum += revenue
	}
	if revenueSum <= 0 {
		return spendSum, revenueSum, 0
	}
	drrPct = math.Round((spendSum/revenueSum*100)*100) / 100
	return spendSum, revenueSum, drrPct
}

func bidAmountForSKU(idx int, kind campaignEnrichmentKind) (float64, bool) {
	return bidAmountForSKUInt64(int64(idx*997+13), kind)
}

func bidAmountForSKUInt64(sku int64, kind campaignEnrichmentKind) (float64, bool) {
	if kind == campaignKindAutoRunning {
		return 0, false
	}
	idx := int(sku % 1000)
	if idx%4 == 0 {
		return 40 + float64(idx%21), true
	}
	return 10 + float64(idx%16), true
}

func mergeDevSeedRawAttributes(existing []byte, scenario OzonEnrichmentSeedScenario, extra map[string]any) []byte {
	base := map[string]any{}
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &base)
	}
	base["source"] = DevOzonEnrichmentSeedSource
	base["scenario"] = string(scenario)
	base["generated_at"] = time.Now().UTC().Format(time.RFC3339)
	base["note"] = "Synthetic P0 enrichment data for local AI Data Pack development. Not real Ozon API data."
	for k, v := range extra {
		base[k] = v
	}
	raw, _ := json.Marshal(base)
	return raw
}

func campaignKindByExternalID(campaigns []dbgen.AdCampaign) map[int64]campaignEnrichmentKind {
	out := make(map[int64]campaignEnrichmentKind, len(campaigns))
	for idx, c := range campaigns {
		out[c.CampaignExternalID] = campaignKindForIndex(idx)
	}
	return out
}

func isAutoCampaign(c dbgen.AdCampaign) bool {
	if c.IsAutocreated.Valid && c.IsAutocreated.Bool {
		return true
	}
	if c.ProductCampaignMode.Valid && strings.Contains(strings.ToUpper(c.ProductCampaignMode.String), "AUTO") {
		return true
	}
	if c.ProductAutopilotStrategy.Valid && strings.TrimSpace(c.ProductAutopilotStrategy.String) != "" {
		return true
	}
	return false
}

type productTitleLookups struct {
	byProductID map[int64]string
	bySKU       map[int64]string
}

func buildProductTitleLookups(skus []enrichmentLocalSKU) productTitleLookups {
	byProduct := make(map[int64]string, len(skus))
	bySKU := make(map[int64]string, len(skus))
	for _, s := range skus {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		byProduct[s.OzonProductID] = name
		bySKU[s.SKU] = name
	}
	return productTitleLookups{byProductID: byProduct, bySKU: bySKU}
}

func resolveCampaignSKUProductTitle(link dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow, titles productTitleLookups) string {
	if link.ProductName.Valid {
		if name := strings.TrimSpace(link.ProductName.String); name != "" {
			return name
		}
	}
	if link.ProductTitle.Valid {
		if name := strings.TrimSpace(link.ProductTitle.String); name != "" {
			return name
		}
	}
	if name := titles.byProductID[link.OzonProductID]; name != "" {
		return name
	}
	if link.Sku.Valid {
		if name := titles.bySKU[link.Sku.Int64]; name != "" {
			return name
		}
		return fmt.Sprintf("SKU %d", link.Sku.Int64)
	}
	return fmt.Sprintf("SKU %d", link.OzonProductID)
}

func parseNumericFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func formatSKU(s int64) string {
	return strconv.FormatInt(s, 10)
}
