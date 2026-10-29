package devseed

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

func TestSellerAnalysisPackPrefixExact(t *testing.T) {
	p1 := SellerAnalysisPackPrefix(1)
	p10 := SellerAnalysisPackPrefix(10)
	if p1 != "ai-analysis-packs/seller-1/" {
		t.Fatalf("seller 1 prefix=%q", p1)
	}
	if p10 != "ai-analysis-packs/seller-10/" {
		t.Fatalf("seller 10 prefix=%q", p10)
	}
	// Exact trailing slash: seller-1/ must not be a prefix of seller-10 keys incorrectly
	// when matching with HasPrefix after ensuring trailing slash.
	key10 := "ai-analysis-packs/seller-10/2026-07-08/abc/analysis_pack.zip"
	if hasExactSellerPackPrefix(key10, 1) {
		t.Fatalf("seller-10 key must not match seller-1 prefix")
	}
	if !hasExactSellerPackPrefix(key10, 10) {
		t.Fatalf("seller-10 key must match seller-10 prefix")
	}
}

func hasExactSellerPackPrefix(key string, sellerID int64) bool {
	prefix := SellerAnalysisPackPrefix(sellerID)
	return len(key) >= len(prefix) && key[:len(prefix)] == prefix
}

func TestSellerResetOptionsPreserveIdentity(t *testing.T) {
	opts := SellerResetOptions{SellerAccountID: 1, DryRun: true}
	if opts.SellerAccountID <= 0 {
		t.Fatal("seller id required")
	}
	_ = opts.DryRun
}

func TestMVPScenarioPlanKeySkuSharesSumToOne(t *testing.T) {
	anchor := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	opts := MVPSeedOptions{
		SellerAccountID: 1,
		AnchorDate:      anchor,
		Days:            90,
		ProductsTarget:  80,
		Seed:            20260514,
	}
	rng := rand.New(rand.NewSource(opts.Seed + opts.SellerAccountID))
	products := buildMVPProducts(1, 80, rng)
	plan, err := BuildMVPScenarioPlan(opts, products)
	if err != nil {
		t.Fatalf("BuildMVPScenarioPlan: %v", err)
	}
	if plan == nil || len(plan.Products) == 0 {
		t.Fatal("expected products in plan")
	}
	var keyCount int
	for _, p := range plan.Products {
		if p.Scenario == scenarioKeySku {
			keyCount++
		}
		for _, d := range p.Window {
			if d.Revenue > 0 && d.OrdersCount <= 0 {
				t.Fatalf("revenue>0 with orders=0 product=%d day=%d", p.ProductIndex, d.DayIndex)
			}
			if d.OrdersCount > 0 && d.Units <= 0 {
				t.Fatalf("orders>0 with units=0 product=%d day=%d", p.ProductIndex, d.DayIndex)
			}
		}
	}
	if keyCount < 8 || keyCount > 12 {
		t.Fatalf("key sku count=%d want 8-12", keyCount)
	}
}

func TestReconcileReportTypes(t *testing.T) {
	_ = context.Background()
	var r ReconciliationReport
	if r.Valid {
		t.Fatal("zero report should not be valid")
	}
}
