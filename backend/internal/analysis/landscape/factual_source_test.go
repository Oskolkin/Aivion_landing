package landscape

import (
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestBuildDomainWindowsMultiFamilySearchUsesSellerSummary(t *testing.T) {
	analysisDate := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	summaryTo := analysisDate.AddDate(0, 0, -2)
	queryTo := analysisDate
	cutoff := analysisDate.Add(26 * time.Hour)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    analysisDate,
		CutoffAt:        cutoff,
		Policies:        factualapi.V1Policies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
		Domains: []factualapi.DomainBinding{
			{
				Domain: "SEARCH", FactFamily: "SEARCH.SELLER_SUMMARY", DomainRunID: 1,
				FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("a", 64), CoverageTo: &summaryTo,
			},
			{
				Domain: "SEARCH", FactFamily: "SEARCH.QUERY_DETAILS", DomainRunID: 2,
				FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("b", 64), CoverageTo: &queryTo,
			},
		},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sealed.DomainBinding("SEARCH"); ok {
		t.Fatal("DomainBinding(SEARCH) must be ambiguous with multiple families")
	}
	windows, err := buildDomainWindows(sealed)
	if err != nil {
		t.Fatal(err)
	}
	var searchWin *DomainWindow
	for i := range windows {
		if windows[i].Domain == "search" {
			searchWin = &windows[i]
			break
		}
	}
	if searchWin == nil {
		t.Fatal("search domain window missing")
	}
	if searchWin.LagDays == nil || *searchWin.LagDays != 2 {
		t.Fatalf("search lag must come from SEARCH.SELLER_SUMMARY binding, got %v", searchWin.LagDays)
	}
	if searchWin.Current == nil || searchWin.Current.To != canonicaljson.Date(summaryTo) {
		t.Fatalf("search window end must align to SELLER_SUMMARY coverage_to, got %v want %v", searchWin.Current.To, summaryTo)
	}
}

func TestBuildSKUSearchBlockErrorsNotReadyEmpty(t *testing.T) {
	curErr := errors.New("search current batch failed")
	block := buildSKUSearchBlock(1, map[int64]factualapi.SKUSearchSummary{}, map[int64]factualapi.SKUSearchSummary{}, curErr, nil)
	if block.State != BlockUnavailable {
		t.Fatalf("state=%s want UNAVAILABLE", block.State)
	}
	if len(block.Limitations) == 0 {
		t.Fatal("expected limitation")
	}
}

func TestBuildSKUSearchBlockProjectsPreviousPosition(t *testing.T) {
	curPos, _ := factualapi.ParseDecimal("12.5")
	prevPos, _ := factualapi.ParseDecimal("18")
	block := buildSKUSearchBlock(1,
		map[int64]factualapi.SKUSearchSummary{1: {Position: factualapi.Known(curPos)}},
		map[int64]factualapi.SKUSearchSummary{1: {Position: factualapi.Known(prevPos)}},
		nil, nil,
	)
	if block.State != BlockReady {
		t.Fatalf("state=%s", block.State)
	}
	if block.ProviderAveragePosition == nil || string(*block.ProviderAveragePosition) != "12.5" {
		t.Fatalf("current position=%v", block.ProviderAveragePosition)
	}
	if block.PreviousProviderAveragePosition == nil || string(*block.PreviousProviderAveragePosition) != "18" {
		t.Fatalf("previous position=%v", block.PreviousProviderAveragePosition)
	}
}

func TestCategorySalesFromDemandErrorsNotReadyEmpty(t *testing.T) {
	curErr := errors.New("category demand unavailable")
	block := categorySalesFromDemand(nil, nil, curErr, nil)
	if block.State != BlockUnavailable {
		t.Fatalf("state=%s want UNAVAILABLE", block.State)
	}
}

func TestBuildSKUSupplyBlockCountsOverdueOnly(t *testing.T) {
	inbound := map[int64]factualapi.SKUInbound{
		1: {
			Supplies: []factualapi.SupplyPlannedItem{
				{SupplyID: 10, CanonicalState: "IN_TRANSIT"},
				{SupplyID: 11, CanonicalState: "OVERDUE"},
				{SupplyID: 11, CanonicalState: "OVERDUE"},
			},
			DeclaredInboundUnits: factualapi.Known(int64(5)),
		},
	}
	block := buildSKUSupplyBlock(1, inbound, nil)
	if block.OverdueSupplyCount == nil || *block.OverdueSupplyCount != 1 {
		t.Fatalf("overdue count=%v want 1", block.OverdueSupplyCount)
	}
	if block.OpenSupplyCount == nil || *block.OpenSupplyCount != 2 {
		t.Fatalf("open supply count=%v want 2 distinct supplies", block.OpenSupplyCount)
	}
}

func TestRollupCategorySupplyAggregatesOverdue(t *testing.T) {
	overdue := int64(2)
	open := int64(1)
	skus := []SKULandscape{
		{
			Identity: SKUIdentity{CanonicalSKUID: 1, CategoryID: int64PtrLocal(99)},
			Supply:   &SKUSupplyBlock{State: BlockReady, OverdueSupplyCount: &overdue, OpenSupplyCount: &open},
		},
	}
	block := rollupCategorySupply(99, skus)
	if block.OverdueSupplyCount == nil || *block.OverdueSupplyCount != 2 {
		t.Fatalf("category overdue=%v", block.OverdueSupplyCount)
	}
}
