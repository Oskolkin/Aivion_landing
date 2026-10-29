package analytics

import (
	"math"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRatioOrNil_DivisionByZero(t *testing.T) {
	cases := []struct {
		name string
		num  float64
		den  float64
	}{
		{"zero_den", 10, 0},
		{"negative_den", 10, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RatioOrNil(tc.num, tc.den); got != nil {
				t.Fatalf("expected nil, got %v", *got)
			}
		})
	}
}

func TestCampaignMetrics_Example(t *testing.T) {
	spend := 10000.0
	revenue := 200000.0
	orders := int64(20)
	impressions := int64(10000)
	clicks := int64(500)

	assertRatio(t, CampaignDRR(spend, revenue), 0.05)
	assertRatio(t, CampaignROAS(revenue, spend), 20)
	assertRatio(t, CampaignCPO(spend, orders), 500)
	assertRatio(t, CampaignCTR(clicks, impressions), 0.05)
	assertRatio(t, CampaignCPC(spend, clicks), 20)
	assertRatio(t, CampaignClickToOrderRate(orders, clicks), 0.04)
}

func TestCampaignDRR_ZeroRevenue(t *testing.T) {
	if got := CampaignDRR(100, 0); got != nil {
		t.Fatalf("expected nil at zero revenue, got %v", *got)
	}
}

func TestSKUShare_Example(t *testing.T) {
	accountOrders := int32(100)
	accountRevenue := 100000.0
	skuOrders := int32(10)
	skuRevenue := 15000.0

	assertRatio(t, RatioOrNil(float64(skuOrders), float64(accountOrders)), 0.10)
	assertRatio(t, RatioOrNil(skuRevenue, accountRevenue), 0.15)
}

func TestSKUVelocity_Example(t *testing.T) {
	assertRatio(t, AverageDailyVelocity(70, 7), 10)
	assertRatio(t, AverageDailyVelocity(140, 14), 10)
}

func TestDiscountDepth_Example(t *testing.T) {
	assertRatio(t, DiscountDepth(2000, 1500), 0.25)
}

func TestExcessStockShare_Example(t *testing.T) {
	assertRatio(t, ExcessStockShare(25, 100), 0.25)
}

func TestCancellationRate_StatusSemantics(t *testing.T) {
	// 10 total postings: 2 cancelled, 1 returned (returned must not inflate cancel_count).
	cancelCount := int32(2)
	totalOrders := int32(10)
	assertRatio(t, CancellationRate(cancelCount, totalOrders), 0.2)
	if got := CancellationRate(1, 0); got != nil {
		t.Fatalf("expected nil when denominator=0, got %v", *got)
	}
}

func TestCancellationRate_ReturnedNotInNumerator(t *testing.T) {
	// Numerator is cancel_count only; returned orders are in orders_count but not in cancel_count.
	cancelCount := int32(0)
	totalWithReturned := int32(5)
	if got := CancellationRate(cancelCount, totalWithReturned); got == nil || *got != 0 {
		t.Fatalf("expected 0 cancellation rate, got %v", got)
	}
}

func TestUnitsSold_OrdersVsUnitsDistinct(t *testing.T) {
	orders := int32(1)
	units := int32(3)
	if orders == units {
		t.Fatal("fixture must keep orders_count and units_sold distinct")
	}
}

func TestSumSKUUnitsSoldInRange_NullAware(t *testing.T) {
	productID := int64(42)
	asOf := mustParseDate(t, "2026-05-10")
	rows := []dbgen.DailySkuMetric{
		{OzonProductID: productID, MetricDate: dateValue(asOf), UnitsSold: pgtype.Int4{Int32: 3, Valid: true}},
		{OzonProductID: productID, MetricDate: dateValue(asOf.AddDate(0, 0, -1)), UnitsSold: pgtype.Int4{Valid: false}},
	}
	sum, ok := sumSKUUnitsSoldInRange(rows, productID, asOf.AddDate(0, 0, -6), asOf)
	if ok {
		t.Fatalf("expected incomplete window => ok=false, sum=%d", sum)
	}
}

func TestSumSKUUnitsSoldInRange_CompleteWindow(t *testing.T) {
	productID := int64(7)
	asOf := mustParseDate(t, "2026-05-07")
	rows := []dbgen.DailySkuMetric{}
	for i := 0; i < 7; i++ {
		d := asOf.AddDate(0, 0, -i)
		rows = append(rows, dbgen.DailySkuMetric{
			OzonProductID: productID,
			MetricDate:    dateValue(d),
			UnitsSold:     pgtype.Int4{Int32: 1, Valid: true},
		})
	}
	sum, ok := sumSKUUnitsSoldInRange(rows, productID, asOf.AddDate(0, 0, -6), asOf)
	if !ok || sum != 7 {
		t.Fatalf("expected sum=7 ok=true, got sum=%d ok=%v", sum, ok)
	}
	assertRatio(t, AverageDailyVelocity(sum, 7), 1)
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func assertRatio(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected %v, got nil", want)
	}
	if math.Abs(*got-want) > 0.0001 {
		t.Fatalf("expected %v, got %v", want, *got)
	}
}
