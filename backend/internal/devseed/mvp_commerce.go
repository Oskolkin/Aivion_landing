package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	whMoscow = "WH-MOSCOW"
	whSPB    = "WH-SPB"
	whKazan  = "WH-KAZAN"
)

type mvpProduct struct {
	OzonProductID         int64
	SKU                   int64
	OfferID               string
	Name                  string
	Status                string
	ReferencePrice        float64
	OldPrice              float64
	OzonMinPrice          float64
	DescriptionCategoryID int64
	Segment               mvpCommerceSegment
	Category              mvpCommerceCategory
	CategoryNameRU        string
	Brand                 string
	PackageSize           string
	ColorMaterial         string
}

// SeedMVPCommerce inserts products, multi-warehouse stocks, orders, and sales.
func SeedMVPCommerce(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, opts MVPSeedOptions) (*CommerceSeedStats, error) {
	rng := rand.New(rand.NewSource(opts.Seed + sellerAccountID))
	anchor := MVPDateUTC(opts.AnchorDate)
	days := opts.Days
	dateRange, err := SeedDateRange(anchor, days)
	if err != nil {
		return nil, err
	}
	products := buildMVPProducts(sellerAccountID, opts.ProductsTarget, rng)

	// The scenario plan is the SOURCE of truth for the trailing demo window: it decides
	// per product/day orders, units, unit price, revenue, stock and demo membership, and
	// mutates products (reference_price / segment overrides) before they are persisted.
	plan, err := BuildMVPScenarioPlan(opts, products)
	if err != nil {
		return nil, err
	}

	stats := &CommerceSeedStats{}
	if err := SeedMVPDescriptionCategories(ctx, q, sellerAccountID); err != nil {
		return nil, err
	}
	for i, p := range products {
		if err := upsertMVPProduct(ctx, q, sellerAccountID, p, anchor, i); err != nil {
			return nil, err
		}
		stats.Products++
	}

	for i, p := range products {
		if err := upsertMVPStocksAllWarehouses(ctx, q, sellerAccountID, p, i, anchor, rng, plan.Products[i].StockOverride); err != nil {
			return nil, err
		}
		stats.Stocks += 3
	}

	dailyQty := make([]int, len(dateRange))
	orderSeq := 1
	saleSeq := 1

	for day, dayTime := range dateRange {
		if plan.InWindow(day) {
			if err := writeMVPWindowDay(ctx, q, sellerAccountID, opts, products, plan, day, dayTime, &orderSeq, &saleSeq, stats); err != nil {
				return nil, err
			}
			continue
		}

		dayMult := commerceDayMultiplier(dayTime, day, days, rng, opts.Seed)

		for pi := range products {
			p := &products[pi]
			units := commerceUnitsForDay(rng, p, day, days, dayMult)
			if units <= 0 {
				continue
			}
			for _, quantity := range mvpSplitUnitsIntoOrders(rng, units) {
				status, saleMult := commerceOrderOutcome(rng)
				orderID := commerceOrderID(opts.Seed, sellerAccountID, dayTime, orderSeq)
				postingNumber := commercePostingNumber(opts.Seed, sellerAccountID, orderSeq)
				orderSeq++

				createdAt, processedAt := mvpCommerceOrderTimestamps(dayTime, opts.Seed, orderSeq)
				orderAmount := mvpRoundMoney(float64(quantity) * p.ReferencePrice * categoryTicketMult(p.Category))

				if err := upsertMVPOrder(ctx, q, sellerAccountID, orderID, postingNumber, status, createdAt, processedAt, orderAmount, *p); err != nil {
					return nil, err
				}
				stats.Orders++
				dailyQty[day] += quantity

				if status == "cancelled" {
					continue
				}

				saleID := fmt.Sprintf("MVP-SALE-%d-%d-%05d", opts.Seed, sellerAccountID, saleSeq)
				saleSeq++
				saleAmount := mvpRoundMoney(orderAmount * saleMult)
				if quantity > 0 && saleAmount <= 0 {
					saleAmount = mvpRoundMoney(float64(quantity) * p.ReferencePrice * categoryTicketMult(p.Category) * 0.9)
				}
				saleAt := mvpCommerceSaleTimestamp(dayTime, processedAt, opts.Seed, saleSeq)
				if err := upsertMVPSale(ctx, q, sellerAccountID, saleID, orderID, postingNumber, quantity, saleAmount, saleAt, *p); err != nil {
					return nil, err
				}
				stats.Sales++
			}
		}

		if day >= days-14 && day < plan.WindowStartIndex && dailyQty[day] < 28 {
			need := 28 - dailyQty[day]
			for need > 0 && len(products) > 0 {
				p := commercePickBoostProduct(products, rng)
				qty := need
				if qty > 4 {
					qty = 2 + rng.Intn(3)
				}
				if qty > need {
					qty = need
				}
				status, saleMult := commerceOrderOutcome(rng)
				if status == "cancelled" {
					status = "delivered"
					saleMult = 1
				}
				orderID := commerceOrderID(opts.Seed, sellerAccountID, dayTime, orderSeq)
				postingNumber := commercePostingNumber(opts.Seed, sellerAccountID, orderSeq)
				orderSeq++
				createdAt := mvpTimestampOnDay(dayTime, 10)
				processedAt := mvpTimestampOnDay(dayTime, 13)
				orderAmount := mvpRoundMoney(float64(qty) * p.ReferencePrice * categoryTicketMult(p.Category))
				if err := upsertMVPOrder(ctx, q, sellerAccountID, orderID, postingNumber, status, createdAt, processedAt, orderAmount, *p); err != nil {
					return nil, err
				}
				stats.Orders++
				dailyQty[day] += qty
				if status != "cancelled" {
					saleID := fmt.Sprintf("MVP-SALE-%d-%d-%05d", opts.Seed, sellerAccountID, saleSeq)
					saleSeq++
					saleAmount := mvpRoundMoney(orderAmount * saleMult)
					if qty > 0 && saleAmount <= 0 {
						saleAmount = mvpRoundMoney(float64(qty) * p.ReferencePrice * categoryTicketMult(p.Category) * 0.9)
					}
					saleAt := mvpCommerceSaleTimestamp(dayTime, processedAt, opts.Seed, saleSeq)
					if err := upsertMVPSale(ctx, q, sellerAccountID, saleID, orderID, postingNumber, qty, saleAmount, saleAt, *p); err != nil {
						return nil, err
					}
					stats.Sales++
				}
				need -= qty
			}
		}
	}

	return stats, nil
}

// writeMVPWindowDay writes the SOURCE orders/sales for one trailing-window day directly
// from the scenario plan. Every planned order is one non-cancelled sale of one unit, so
// SUM(sku orders) reconciles 1:1 with account orders and revenue>0 implies orders>0.
func writeMVPWindowDay(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	opts MVPSeedOptions,
	products []mvpProduct,
	plan *MVPScenarioPlan,
	dayIndex int,
	dayTime time.Time,
	orderSeq, saleSeq *int,
	stats *CommerceSeedStats,
) error {
	wj := dayIndex - plan.WindowStartIndex
	for pi := range products {
		p := &products[pi]
		pp := plan.PlanForProduct(pi)
		if pp == nil || wj < 0 || wj >= len(pp.Window) {
			continue
		}
		entry := pp.Window[wj]
		if entry.OrdersCount <= 0 {
			continue
		}
		for k := 0; k < entry.OrdersCount; k++ {
			orderID := commerceOrderID(opts.Seed, sellerAccountID, dayTime, *orderSeq)
			postingNumber := commercePostingNumber(opts.Seed, sellerAccountID, *orderSeq)
			*orderSeq++
			createdAt, processedAt := mvpCommerceOrderTimestamps(dayTime, opts.Seed, *orderSeq)
			orderAmount := mvpRoundMoney(entry.UnitPrice)

			if err := upsertMVPOrder(ctx, q, sellerAccountID, orderID, postingNumber, "delivered", createdAt, processedAt, orderAmount, *p); err != nil {
				return err
			}
			stats.Orders++

			saleID := fmt.Sprintf("MVP-SALE-%d-%d-%05d", opts.Seed, sellerAccountID, *saleSeq)
			*saleSeq++
			saleAt := mvpCommerceSaleTimestamp(dayTime, processedAt, opts.Seed, *saleSeq)
			if err := upsertMVPSale(ctx, q, sellerAccountID, saleID, orderID, postingNumber, 1, orderAmount, saleAt, *p); err != nil {
				return err
			}
			stats.Sales++
		}
	}
	return nil
}

func commercePickBoostProduct(products []mvpProduct, rng *rand.Rand) *mvpProduct {
	leaders := make([]int, 0)
	for i := range products {
		if products[i].Segment == segLeaders {
			leaders = append(leaders, i)
		}
	}
	if len(leaders) == 0 {
		return &products[rng.Intn(len(products))]
	}
	return &products[leaders[rng.Intn(len(leaders))]]
}

func mvpCommerceOrderTimestamps(day time.Time, seed int64, orderSeq int) (createdAt, processedAt time.Time) {
	createdHour := hashHour(seed, orderSeq, 18)         // 1..18
	processedDelta := 2 + hashHour(seed, orderSeq+7, 6) // 2..7
	createdAt = mvpTimestampOnDay(day, createdHour)
	maxProcessedHour := 23 - createdHour
	if processedDelta > maxProcessedHour {
		processedDelta = maxProcessedHour
	}
	processedAt = mvpTimestampOnDay(day, createdHour+processedDelta)
	return MVPClampToCalendarDayUTC(createdAt, day), MVPClampToCalendarDayUTC(processedAt, day)
}

func mvpCommerceSaleTimestamp(day, processedAt time.Time, seed int64, saleSeq int) time.Time {
	saleHourDelta := hashHour(seed, saleSeq, 4) // 1..4
	dayStart := MVPDateUTC(day)
	processedHour := int(processedAt.Sub(dayStart) / time.Hour)
	saleHour := processedHour + saleHourDelta
	if saleHour > 23 {
		saleHour = 23
	}
	return MVPClampToCalendarDayUTC(mvpTimestampOnDay(day, saleHour), day)
}

func hashHour(seed int64, salt int, mod int) int {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%d-%d", seed, salt)
	return int(h.Sum32()%uint32(mod)) + 1
}

func commerceOrderID(seed, seller int64, day time.Time, seq int) string {
	return fmt.Sprintf("OZN-%s-%d-%05d", day.Format("20060102"), (seed+int64(seller))%90000+10000, seq)
}

func commercePostingNumber(seed, seller int64, seq int) string {
	return fmt.Sprintf("FBS-%d-%05d", (seed*7+seller)%900000+100000, seq)
}

func commerceOrderOutcome(rng *rand.Rand) (status string, saleMult float64) {
	r := rng.Intn(100)
	cancelCut := 6 + rng.Intn(3)
	retCut := cancelCut + 3 + rng.Intn(3)
	switch {
	case r < cancelCut:
		return "cancelled", 0
	case r < retCut:
		return "returned", -1
	case r < retCut+18:
		return "shipped", 1
	default:
		return "delivered", 1
	}
}

func categoryTicketMult(cat mvpCommerceCategory) float64 {
	switch cat {
	case catLightDecor:
		return 1.25
	case catKitchenStorage:
		return 1.05
	case catCleaning:
		return 0.95
	case catBathroom:
		return 1.1
	case catHomeTextile:
		return 1.15
	default:
		return 1.0
	}
}

func commerceDayMultiplier(dayTime time.Time, dayIndex, days int, rng *rand.Rand, seed int64) float64 {
	wd := dayTime.Weekday()
	wk := 1.0
	switch wd {
	case time.Saturday, time.Sunday:
		wk = 1.14
	case time.Monday, time.Tuesday:
		wk = 0.96
	case time.Wednesday:
		wk = 0.92
	default:
		wk = 1.0
	}

	anchorIdx := days - 1
	spike := 1.0
	for _, off := range []int{70, 55, 33, 12} {
		if anchorIdx-off >= 0 && dayIndex == anchorIdx-off {
			spike *= 1.75 + 0.15*float64((seed+int64(off))%5)/5.0
		}
	}

	noise := 0.92 + 0.16*(float64((seed+int64(dayIndex)*31)%100)/100.0)
	return wk * spike * noise * mvpDayDemandTrend(dayIndex, days)
}

func mvpDayDemandTrend(dayIndex, days int) float64 {
	weeklyWave := 1.0 + 0.16*math.Sin(float64(dayIndex)*2*math.Pi/7)
	trend := 0.88 + 0.24*float64(dayIndex)/float64(max(1, days-1))
	return weeklyWave * trend
}

func segmentScale(seg mvpCommerceSegment) float64 {
	switch seg {
	case segLeaders:
		return 5.2
	case segRising:
		return 2.1
	case segDeclining:
		return 2.0
	case segLowStock:
		return 3.8
	case segOverstock:
		return 0.42
	case segAdWaste:
		return 0.32
	case segPriceRisk:
		return 1.35
	case segStableTail:
		return 1.05
	default:
		return 1.0
	}
}

func commerceUnitsForDay(rng *rand.Rand, p *mvpProduct, dayIndex, days int, dayMult float64) int {
	recentStart := days - 14
	segTrend := 1.0
	if dayIndex >= recentStart {
		rd := float64(dayIndex - recentStart)
		switch p.Segment {
		case segRising:
			segTrend = 0.75 + rd*0.045
		case segDeclining:
			segTrend = math.Max(0.35, 1.15-rd*0.055)
		case segAdWaste:
			segTrend = 0.55 + 0.02*math.Sin(rd)
		default:
			segTrend = 1.0
		}
	}

	base := 0.55 * segmentScale(p.Segment) * segTrend * dayMult
	noise := 0.75 + rng.Float64()*0.45
	raw := base * noise
	if raw < 0.35 {
		if rng.Intn(100) < 65 {
			return 0
		}
		return 1
	}
	u := int(math.Round(raw))
	if u < 0 {
		return 0
	}
	if u > 48 {
		u = 48
	}
	return u
}

func buildMVPProducts(sellerAccountID int64, n int, rng *rand.Rand) []mvpProduct {
	products := make([]mvpProduct, 0, n)
	catLen := len(mvpProductCatalog)
	for i := 0; i < n; i++ {
		entry := mvpProductCatalog[i%catLen]
		seg := commerceSegmentForIndex(i, n)

		status := "active"
		// Main catalog products stay analysis-eligible so Dashboard/Pack totals match
		// (eligible == products). Historical/technical SKUs come from related-SKU seed.

		productID := 901_000_000_000 + sellerAccountID*100_000 + int64(i+1)
		sku := 801_000_000_000 + sellerAccountID*100_000 + int64(i+1)
		offerID := fmt.Sprintf("%s-%03d", entry.OfferPrefix, i+1)

		price := entry.BasePrice * (0.95 + 0.1*float64(rng.Intn(10))/10.0)
		if seg == segLeaders {
			price *= 1.08
		}
		if seg == segPriceRisk {
			price *= 1.02
		}
		price = mvpRoundMoney(price)

		old := mvpRoundMoney(price * (1.05 + 0.03*float64(rng.Intn(5))))
		ozonMin := mvpRoundMoney(price * 0.88)
		if seg == segPriceRisk {
			ozonMin = mvpRoundMoney(price * 0.94)
		}

		descID := categoryDescriptionID(entry.Category, sellerAccountID)

		products = append(products, mvpProduct{
			OzonProductID:         productID,
			SKU:                   sku,
			OfferID:               offerID,
			Name:                  entry.NameRU,
			Status:                status,
			ReferencePrice:        price,
			OldPrice:              old,
			OzonMinPrice:          ozonMin,
			DescriptionCategoryID: descID,
			Segment:               seg,
			Category:              entry.Category,
			CategoryNameRU:        entry.CategoryNameRU,
			Brand:                 entry.Brand,
			PackageSize:           entry.PackageSize,
			ColorMaterial:         entry.ColorMaterial,
		})
	}
	return products
}

func upsertMVPProduct(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, p mvpProduct, sourceUpdatedAt time.Time, imageIndex int) error {
	raw, _ := json.Marshal(map[string]any{
		"category_name":  p.CategoryNameRU,
		"brand":          p.Brand,
		"segment":        string(p.Segment),
		"package_size":   p.PackageSize,
		"color_material": p.ColorMaterial,
		"seeded_by":      "dev-seed-mvp",
		"category":       string(p.Category),
	})
	primaryURL, galleryJSON := mvpProductDemoImages(p, imageIndex)
	_, err := q.UpsertProduct(ctx, dbgen.UpsertProductParams{
		SellerAccountID:       sellerAccountID,
		OzonProductID:         p.OzonProductID,
		OfferID:               textNV(p.OfferID),
		Sku:                   int8NV(p.SKU),
		Name:                  p.Name,
		Status:                textNV(p.Status),
		ReferencePrice:        moneyNV(p.ReferencePrice),
		OldPrice:              moneyNV(p.OldPrice),
		OzonMinPrice:          moneyNV(p.OzonMinPrice),
		DescriptionCategoryID: pgtype.Int8{Int64: p.DescriptionCategoryID, Valid: true},
		IsArchived:            p.Status == "archived",
		RawAttributes:         raw,
		PrimaryImageUrl:       textNV(primaryURL),
		ImageUrls:             galleryJSON,
		SourceUpdatedAt:       tsNV(sourceUpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert product %d: %w", p.OzonProductID, err)
	}
	return nil
}

func upsertMVPStocksAllWarehouses(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, p mvpProduct, index int, snapshotAt time.Time, rng *rand.Rand, stockOverride int) error {
	totalAvail := commerceTotalAvailability(p, index, rng)
	if stockOverride >= 0 {
		totalAvail = stockOverride
	}
	weights := warehouseWeights(sellerAccountID, index)
	wIDs := []string{whMoscow, whSPB, whKazan}
	sumW := weights[0] + weights[1] + weights[2]
	avail := make([]int, 3)
	rem := totalAvail
	for wi := 0; wi < 3; wi++ {
		share := int(float64(totalAvail) * float64(weights[wi]) / float64(sumW))
		if share > rem {
			share = rem
		}
		if wi == 2 {
			share = rem
		}
		avail[wi] = share
		rem -= share
	}

	for wi, wh := range wIDs {
		a := avail[wi]
		res := 0
		if a > 4 {
			res = 1 + (index+wi)%5
		}
		tot := a + res
		raw, _ := json.Marshal(map[string]any{
			"mvp_seed":    true,
			"segment":     string(p.Segment),
			"stock_band":  mvpStockBand(a),
			"seed_source": "dev-seed-mvp",
			"warehouse":   wh,
		})
		_, err := q.UpsertStock(ctx, dbgen.UpsertStockParams{
			SellerAccountID:     sellerAccountID,
			ProductExternalID:   fmt.Sprintf("%d", p.OzonProductID),
			WarehouseExternalID: wh,
			QuantityTotal:       int4NV(tot),
			QuantityReserved:    int4NV(res),
			QuantityAvailable:   int4NV(a),
			SnapshotAt:          tsNV(snapshotAt),
			RawAttributes:       raw,
		})
		if err != nil {
			return fmt.Errorf("upsert stock product %d wh %s: %w", p.OzonProductID, wh, err)
		}
	}
	return nil
}

func warehouseWeights(sellerID int64, idx int) [3]int {
	h := int(sellerID)%3 + idx%3
	switch h % 3 {
	case 0:
		return [3]int{50, 32, 18}
	case 1:
		return [3]int{45, 35, 20}
	default:
		return [3]int{48, 30, 22}
	}
}

func commerceTotalAvailability(p mvpProduct, index int, rng *rand.Rand) int {
	switch p.Segment {
	case segLowStock:
		return rng.Intn(6)
	case segOverstock:
		return 150 + rng.Intn(251)
	case segLeaders:
		if index%5 == 0 {
			return 2 + rng.Intn(7)
		}
		if index%7 == 1 {
			return 25 + rng.Intn(40)
		}
		return 40 + rng.Intn(50)
	case segAdWaste, segDeclining:
		return 35 + rng.Intn(60)
	case segStableTail:
		return 20 + rng.Intn(101)
	default:
		return 15 + rng.Intn(90)
	}
}

func upsertMVPOrder(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	orderID, postingNumber, status string,
	createdAt, processedAt time.Time,
	totalAmount float64,
	p mvpProduct,
) error {
	raw, _ := json.Marshal(map[string]any{
		"mvp_seed":    true,
		"product_id":  p.OzonProductID,
		"offer_id":    p.OfferID,
		"sku":         p.SKU,
		"segment":     string(p.Segment),
		"status":      status,
		"category":    string(p.Category),
		"seed_source": "dev-seed-mvp",
	})
	_, err := q.UpsertOrder(ctx, dbgen.UpsertOrderParams{
		SellerAccountID:   sellerAccountID,
		OzonOrderID:       orderID,
		PostingNumber:     textNV(postingNumber),
		Status:            textNV(status),
		CreatedAtSource:   tsNV(createdAt),
		ProcessedAtSource: tsNV(processedAt),
		TotalAmount:       moneyNV(totalAmount),
		CurrencyCode:      textNV("RUB"),
		RawAttributes:     raw,
	})
	if err != nil {
		return fmt.Errorf("upsert order %s: %w", orderID, err)
	}
	return nil
}

func upsertMVPSale(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	saleID, orderID, postingNumber string,
	quantity int,
	amount float64,
	saleAt time.Time,
	p mvpProduct,
) error {
	raw, _ := json.Marshal(map[string]any{
		"mvp_seed":    true,
		"product_id":  p.OzonProductID,
		"offer_id":    p.OfferID,
		"sku":         p.SKU,
		"segment":     string(p.Segment),
		"category":    string(p.Category),
		"seed_source": "dev-seed-mvp",
	})
	_, err := q.UpsertSale(ctx, dbgen.UpsertSaleParams{
		SellerAccountID: sellerAccountID,
		OzonSaleID:      saleID,
		OzonOrderID:     textNV(orderID),
		PostingNumber:   textNV(postingNumber),
		Quantity:        int4NV(quantity),
		Amount:          moneyNV(amount),
		CurrencyCode:    textNV("RUB"),
		SaleDate:        tsNV(saleAt),
		RawAttributes:   raw,
	})
	if err != nil {
		return fmt.Errorf("upsert sale %s: %w", saleID, err)
	}
	return nil
}

func mvpSplitUnitsIntoOrders(rng *rand.Rand, units int) []int {
	if units <= 0 {
		return nil
	}
	if units == 1 {
		return []int{1}
	}
	chunks := make([]int, 0, units)
	remaining := units
	for remaining > 0 {
		maxChunk := 3
		if remaining == 2 {
			maxChunk = 2
		}
		chunk := 1 + rng.Intn(maxChunk)
		if chunk > remaining {
			chunk = remaining
		}
		chunks = append(chunks, chunk)
		remaining -= chunk
	}
	return chunks
}

func mvpStockBand(available int) string {
	switch {
	case available <= 0:
		return "out_of_stock"
	case available <= 5:
		return "low_stock"
	case available >= 100:
		return "high_stock"
	default:
		return "normal_stock"
	}
}

func textNV(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: true}
}

func int8NV(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func int4NV(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func tsNV(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func moneyNV(v float64) pgtype.Numeric {
	formatted := fmt.Sprintf("%.2f", mvpRoundMoney(v))
	var n pgtype.Numeric
	_ = n.Scan(formatted)
	return n
}

func mvpRoundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
