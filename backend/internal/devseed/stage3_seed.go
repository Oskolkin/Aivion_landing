package devseed

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	productCount      = 50
	historyDays       = 60
	baseRandomSeed    = int64(2026031401)
	anchorDateISO8601 = "2026-03-31"
)

type Result struct {
	SellerAccountID int64
	Products        int
	Orders          int
	Sales           int
	Stocks          int
}

type productSegment string

const (
	segmentLeader    productSegment = "leader"
	segmentRising    productSegment = "rising"
	segmentDeclining productSegment = "declining"
	segmentTail      productSegment = "tail"
	segmentZeroSales productSegment = "zero_sales"
)

type seededProduct struct {
	OzonProductID int64
	OfferID       string
	SKU           int64
	Name          string
	Status        string
	UnitPrice     float64
	Segment       productSegment
}

// SeedStage3ForSeller creates reproducible synthetic source data for a single seller account.
// It never touches other seller accounts and is intended for local/dev use only.
func SeedStage3ForSeller(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64) (*Result, error) {
	if sellerAccountID <= 0 {
		return nil, fmt.Errorf("seller_account_id must be greater than 0")
	}

	anchorDate, err := time.Parse("2006-01-02", anchorDateISO8601)
	if err != nil {
		return nil, fmt.Errorf("parse anchor date: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := dbgen.New(tx)
	if _, err := q.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		return nil, fmt.Errorf("seller account %d not found: %w", sellerAccountID, err)
	}

	if err := cleanupSellerData(ctx, tx, sellerAccountID); err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(baseRandomSeed + sellerAccountID))
	products := buildProducts(sellerAccountID)

	result := &Result{SellerAccountID: sellerAccountID}

	for _, p := range products {
		if err := upsertProduct(ctx, q, sellerAccountID, p, anchorDate); err != nil {
			return nil, err
		}
		result.Products++
	}

	for i, p := range products {
		if err := upsertStock(ctx, q, sellerAccountID, p, i, anchorDate); err != nil {
			return nil, err
		}
		result.Stocks++
	}

	orderSeq := 1
	saleSeq := 1
	for day := 0; day < historyDays; day++ {
		dayTime := anchorDate.AddDate(0, 0, -(historyDays - 1 - day))
		dayDemandFactor := dayDemandMultiplier(day)
		for _, p := range products {
			units := unitsForDay(rng, p, day, dayDemandFactor)
			if units <= 0 {
				continue
			}

			chunks := splitUnitsIntoOrders(rng, units)
			for _, quantity := range chunks {
				orderID := fmt.Sprintf("OZ-ORD-%d-%05d", sellerAccountID, orderSeq)
				postingNumber := fmt.Sprintf("POST-%d-%05d", sellerAccountID, orderSeq)
				orderSeq++

				statusRoll := rng.Intn(100)
				status := "delivered"
				switch {
				case statusRoll < 8:
					status = "cancelled"
				case statusRoll < 13:
					status = "returned"
				}

				createdAt := dayTime.Add(time.Duration(rng.Intn(20)) * time.Hour)
				processedAt := createdAt.Add(time.Duration(2+rng.Intn(24)) * time.Hour)
				orderAmount := roundMoney(float64(quantity) * p.UnitPrice)

				if err := upsertOrder(
					ctx,
					q,
					sellerAccountID,
					orderID,
					postingNumber,
					status,
					createdAt,
					processedAt,
					orderAmount,
					p,
				); err != nil {
					return nil, err
				}
				result.Orders++

				if status == "cancelled" {
					continue
				}

				saleID := fmt.Sprintf("OZ-SALE-%d-%05d", sellerAccountID, saleSeq)
				saleSeq++
				saleAmount := orderAmount
				if status == "returned" {
					saleAmount = roundMoney(orderAmount * -1)
				}

				saleAt := processedAt.Add(time.Duration(rng.Intn(4)) * time.Hour)
				if err := upsertSale(
					ctx,
					q,
					sellerAccountID,
					saleID,
					orderID,
					postingNumber,
					quantity,
					saleAmount,
					saleAt,
					p,
				); err != nil {
					return nil, err
				}
				result.Sales++
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func cleanupSellerData(ctx context.Context, tx dbgen.DBTX, sellerAccountID int64) error {
	if _, err := tx.Exec(ctx, "DELETE FROM sales WHERE seller_account_id = $1", sellerAccountID); err != nil {
		return fmt.Errorf("cleanup sales: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM orders WHERE seller_account_id = $1", sellerAccountID); err != nil {
		return fmt.Errorf("cleanup orders: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM stocks WHERE seller_account_id = $1", sellerAccountID); err != nil {
		return fmt.Errorf("cleanup stocks: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM products WHERE seller_account_id = $1", sellerAccountID); err != nil {
		return fmt.Errorf("cleanup products: %w", err)
	}
	return nil
}

func buildProducts(sellerAccountID int64) []seededProduct {
	products := make([]seededProduct, 0, productCount)
	for i := 0; i < productCount; i++ {
		segment := segmentTail
		switch {
		case i < 8:
			segment = segmentLeader
		case i < 14:
			segment = segmentRising
		case i < 20:
			segment = segmentDeclining
		case i < 26:
			segment = segmentZeroSales
		}

		status := "active"
		if i%17 == 0 {
			status = "archived"
		}

		productID := 8000000000 + sellerAccountID*1000 + int64(i+1)
		sku := 7000000000 + sellerAccountID*1000 + int64(i+1)
		offerID := fmt.Sprintf("OFFER-%02d-%d", i+1, sellerAccountID)
		name := fmt.Sprintf("Synthetic Product %02d", i+1)

		price := 700 + float64((i%9)*120)
		if segment == segmentLeader {
			price += 650
		}

		products = append(products, seededProduct{
			OzonProductID: productID,
			OfferID:       offerID,
			SKU:           sku,
			Name:          name,
			Status:        status,
			UnitPrice:     price,
			Segment:       segment,
		})
	}
	return products
}

func upsertProduct(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, p seededProduct, sourceUpdatedAt time.Time) error {
	_, err := q.UpsertProduct(ctx, dbgen.UpsertProductParams{
		SellerAccountID: sellerAccountID,
		OzonProductID:   p.OzonProductID,
		OfferID:         textValue(p.OfferID),
		Sku:             int8Value(p.SKU),
		Name:            p.Name,
		Status:          textValue(p.Status),
		IsArchived:      p.Status == "archived",
		RawAttributes: []byte(fmt.Sprintf(
			`{"seed":"stage3","synthetic":true,"segment":"%s","unit_price":%.2f}`,
			p.Segment,
			p.UnitPrice,
		)),
		SourceUpdatedAt: timestamptzValue(sourceUpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert product %d: %w", p.OzonProductID, err)
	}
	return nil
}

func upsertStock(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, p seededProduct, index int, snapshotAt time.Time) error {
	available := 0
	// Explicit requirements: exactly two out-of-stock and two low-stock SKUs.
	switch index {
	case 0, 1:
		available = 0
	case 2, 3:
		available = 4
	default:
		switch p.Segment {
		case segmentLeader:
			available = 120
		case segmentRising:
			available = 80
		case segmentDeclining:
			available = 45
		case segmentZeroSales:
			available = 65
		default:
			available = 30 + (index % 90)
		}
	}

	reserved := 0
	if available > 5 {
		reserved = 2 + (index % 7)
	}
	total := available + reserved

	_, err := q.UpsertStock(ctx, dbgen.UpsertStockParams{
		SellerAccountID:     sellerAccountID,
		ProductExternalID:   fmt.Sprintf("%d", p.OzonProductID),
		WarehouseExternalID: "WH-DEFAULT",
		QuantityTotal:       int4Value(total),
		QuantityReserved:    int4Value(reserved),
		QuantityAvailable:   int4Value(available),
		SnapshotAt:          timestamptzValue(snapshotAt),
		RawAttributes: []byte(fmt.Sprintf(
			`{"seed":"stage3","synthetic":true,"segment":"%s","stock_band":"%s"}`,
			p.Segment,
			stockBand(available),
		)),
	})
	if err != nil {
		return fmt.Errorf("upsert stock for product %d: %w", p.OzonProductID, err)
	}
	return nil
}

func upsertOrder(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	orderID string,
	postingNumber string,
	status string,
	createdAt time.Time,
	processedAt time.Time,
	totalAmount float64,
	p seededProduct,
) error {
	_, err := q.UpsertOrder(ctx, dbgen.UpsertOrderParams{
		SellerAccountID:   sellerAccountID,
		OzonOrderID:       orderID,
		PostingNumber:     textValue(postingNumber),
		Status:            textValue(status),
		CreatedAtSource:   timestamptzValue(createdAt),
		ProcessedAtSource: timestamptzValue(processedAt),
		TotalAmount:       moneyNumeric(totalAmount),
		CurrencyCode:      textValue("RUB"),
		RawAttributes: []byte(fmt.Sprintf(
			`{"seed":"stage3","synthetic":true,"product_id":%d,"offer_id":"%s","segment":"%s","status":"%s"}`,
			p.OzonProductID,
			p.OfferID,
			p.Segment,
			status,
		)),
	})
	if err != nil {
		return fmt.Errorf("upsert order %s: %w", orderID, err)
	}
	return nil
}

func upsertSale(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	saleID string,
	orderID string,
	postingNumber string,
	quantity int,
	amount float64,
	saleAt time.Time,
	p seededProduct,
) error {
	_, err := q.UpsertSale(ctx, dbgen.UpsertSaleParams{
		SellerAccountID: sellerAccountID,
		OzonSaleID:      saleID,
		OzonOrderID:     textValue(orderID),
		PostingNumber:   textValue(postingNumber),
		Quantity:        int4Value(quantity),
		Amount:          moneyNumeric(amount),
		CurrencyCode:    textValue("RUB"),
		SaleDate:        timestamptzValue(saleAt),
		RawAttributes: []byte(fmt.Sprintf(
			`{"seed":"stage3","synthetic":true,"product_id":%d,"offer_id":"%s","segment":"%s"}`,
			p.OzonProductID,
			p.OfferID,
			p.Segment,
		)),
	})
	if err != nil {
		return fmt.Errorf("upsert sale %s: %w", saleID, err)
	}
	return nil
}

func dayDemandMultiplier(dayIndex int) float64 {
	weeklyWave := 1.0 + 0.18*math.Sin(float64(dayIndex)*2*math.Pi/7)
	trend := 0.92 + float64(dayIndex)*0.003
	spike := 1.0
	switch dayIndex {
	case 9, 26, 43, 52:
		spike = 2.15
	}
	return weeklyWave * trend * spike
}

func unitsForDay(rng *rand.Rand, p seededProduct, dayIndex int, dayFactor float64) int {
	if p.Segment == segmentZeroSales {
		return 0
	}

	base := 0.0
	switch p.Segment {
	case segmentLeader:
		base = 4.6
	case segmentRising:
		base = 2.4
	case segmentDeclining:
		base = 2.8
	default:
		base = 1.1
	}

	segmentTrend := 1.0
	switch p.Segment {
	case segmentRising:
		segmentTrend = 0.55 + float64(dayIndex)*0.018
	case segmentDeclining:
		segmentTrend = math.Max(0.25, 1.6-float64(dayIndex)*0.022)
	}

	noise := 0.8 + rng.Float64()*0.5
	raw := base * segmentTrend * dayFactor * noise
	if raw < 0.85 {
		if rng.Intn(100) < 70 {
			return 0
		}
		return 1
	}

	units := int(math.Round(raw))
	if units < 0 {
		return 0
	}
	return units
}

func splitUnitsIntoOrders(rng *rand.Rand, units int) []int {
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

func stockBand(available int) string {
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

func textValue(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: true}
}

func int8Value(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func int4Value(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func timestamptzValue(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func moneyNumeric(v float64) pgtype.Numeric {
	formatted := fmt.Sprintf("%.2f", roundMoney(v))
	var n pgtype.Numeric
	_ = n.Scan(formatted)
	return n
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
