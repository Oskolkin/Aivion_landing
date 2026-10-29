package pack

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestClassifySKUAnalysisEligibility(t *testing.T) {
	t.Parallel()
	idx := &Indexes{
		SKUMeta:            map[string]*skuMeta{},
		ProductBySKU:       map[string]dbgen.Product{},
		SKUMetricsByDate:   map[string]map[string]skuMetricDay{},
		StockBySKU:         map[string]stockAgg{},
		ProductInfoBySKU:   map[string]dbgen.OzonProductInfoSnapshot{},
		campaignLinkedSKUs: map[string]struct{}{},
	}

	idx.SKUMeta["10"] = &skuMeta{SKU: "10", OfferID: "OFF-10", Title: "Товар 10"}
	idx.ProductBySKU["10"] = dbgen.Product{Name: "Товар 10", OfferID: pgtype.Text{String: "OFF-10", Valid: true}}
	idx.SKUMetricsByDate["10"] = map[string]skuMetricDay{"2026-07-08": {Orders: 1}}
	if ok, reason := ClassifySKUAnalysisEligibility(idx, "10"); !ok || reason != EligibilityOK {
		t.Fatalf("full sku: ok=%v reason=%s", ok, reason)
	}

	idx.SKUMeta["20"] = &skuMeta{SKU: "20", OfferID: "", Title: "SKU 20"}
	if ok, reason := ClassifySKUAnalysisEligibility(idx, "20"); ok || reason != EligibilityNoProduct {
		t.Fatalf("no product: ok=%v reason=%s", ok, reason)
	}

	idx.SKUMeta["30"] = &skuMeta{SKU: "30", OfferID: "OFF-30", Title: "Archived"}
	idx.ProductBySKU["30"] = dbgen.Product{Name: "Archived", IsArchived: true, OfferID: pgtype.Text{String: "OFF-30", Valid: true}}
	idx.SKUMetricsByDate["30"] = map[string]skuMetricDay{"2026-07-08": {Orders: 0}}
	if ok, reason := ClassifySKUAnalysisEligibility(idx, "30"); ok || reason != EligibilityArchivedOrDeleted {
		t.Fatalf("archived: ok=%v reason=%s", ok, reason)
	}

	idx.SKUMeta["40"] = &skuMeta{SKU: "40", OfferID: "OFF-40", Title: "SKU 40"}
	idx.ProductBySKU["40"] = dbgen.Product{Name: "SKU 40", OfferID: pgtype.Text{String: "OFF-40", Valid: true}}
	idx.SKUMetricsByDate["40"] = map[string]skuMetricDay{"2026-07-08": {}}
	if ok, reason := ClassifySKUAnalysisEligibility(idx, "40"); ok || reason != EligibilityTechnicalTitle {
		t.Fatalf("technical title: ok=%v reason=%s", ok, reason)
	}
}

func TestValidateBusinessConsistency_BlocksContradictions(t *testing.T) {
	t.Parallel()
	metric := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	periods := ComputePackPeriods(metric)
	idx := &Indexes{
		SKUUniverse: []string{"1"},
		SKUMeta: map[string]*skuMeta{
			"1": {SKU: "1", OfferID: "OFF-1", Title: "Good"},
		},
		ProductBySKU: map[string]dbgen.Product{
			"1": {Name: "Good", OfferID: pgtype.Text{String: "OFF-1", Valid: true}},
		},
		SKUMetricsByDate: map[string]map[string]skuMetricDay{
			"1": {
				"2026-07-02": {Orders: 1, Revenue: 100},
				"2026-07-03": {Orders: 1, Revenue: 100},
				"2026-07-04": {Orders: 1, Revenue: 100},
				"2026-07-05": {Orders: 1, Revenue: 100},
				"2026-07-06": {Orders: 1, Revenue: 100},
				"2026-07-07": {Orders: 1, Revenue: 100},
				"2026-07-08": {Orders: 1, Revenue: 100},
			},
		},
		AccountMetricsByDate: map[string]dbgen.DailyAccountMetric{},
		CampaignMeta:         map[int64]*campaignMeta{},
		Eligibility:          SKUEligibilityDiagnostics{TotalDiscovered: 1, AnalysisEligible: 1},
	}
	for d, day := range idx.SKUMetricsByDate["1"] {
		idx.AccountMetricsByDate[d] = dbgen.DailyAccountMetric{
			OrdersCount: int32(day.Orders),
			Revenue:     mustNumeric(t, "100"),
		}
	}

	facts := map[string]skuFacts{
		"1": {
			Orders7D: 7, Revenue7D: 700, DaysWithoutSales: 30, HasStockData: true, AvailableStock: 10,
			StockStatus: stockpolicy.StatusNoSales, Coverage7D: floatPtr(20),
		},
	}
	report := ValidateBusinessConsistency(idx, facts, periods, "2026-07-08")
	if !report.HasErrors() {
		t.Fatalf("expected errors for sales vs no-sales, got warnings=%v", report.Warnings)
	}
}
