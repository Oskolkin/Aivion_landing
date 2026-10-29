package universe

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type memEntitySource struct {
	skus  []factualapi.SKU
	camps []factualapi.Campaign
	cats  []factualapi.Category
}

func (m memEntitySource) ListSKUs(_ context.Context, _ factualapi.FactualReadContext, _ factualapi.SKUFilter, _ factualapi.PageRequest) (factualapi.Page[factualapi.SKU], error) {
	return factualapi.Page[factualapi.SKU]{Items: m.skus}, nil
}
func (m memEntitySource) ListCampaigns(_ context.Context, _ factualapi.FactualReadContext, _ factualapi.CampaignFilter, _ factualapi.PageRequest) (factualapi.Page[factualapi.Campaign], error) {
	return factualapi.Page[factualapi.Campaign]{Items: m.camps}, nil
}
func (m memEntitySource) ListCategories(_ context.Context, _ factualapi.FactualReadContext, _ factualapi.PageRequest) (factualapi.Page[factualapi.Category], error) {
	return factualapi.Page[factualapi.Category]{Items: m.cats}, nil
}

func TestBuildEntitySetsUsesSemanticMembersNotDBPKs(t *testing.T) {
	catID := int64(77) // seller_category_id DB PK — must not appear in members
	src := memEntitySource{
		cats: []factualapi.Category{{ID: 77, SellerCategoryID: 77, ProviderCategoryID: 555001, Name: "Leaf"}},
		skus: []factualapi.SKU{
			{ID: 9001, OzonSKU: 111, Category: factualapi.CategoryRelation{CategoryID: &catID, MappingStatus: "MAPPED"}},
			{ID: 9002, OzonSKU: 222, Category: factualapi.CategoryRelation{MappingStatus: "UNMAPPED"}},
		},
		camps: []factualapi.Campaign{
			{ID: 8001, ProviderCampaignID: 4242},
		},
	}
	rc := testReadContext(t)
	sets, err := buildEntitySets(context.Background(), src, rc)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string]EntitySet{}
	for _, s := range sets {
		byKind[s.EntityKind] = s
	}
	if got := byKind[entityKindSKU].Members; len(got) != 2 || got[0] != "111" || got[1] != "222" {
		t.Fatalf("SKU members=%v", got)
	}
	if got := byKind[entityKindCampaign].Members; len(got) != 1 || got[0] != "4242" {
		t.Fatalf("CAMPAIGN members=%v", got)
	}
	wantMap := []string{"555001:111", "UNMAPPED:222"}
	if got := byKind[entityKindCategoryMapping].Members; len(got) != 2 || got[0] != wantMap[0] || got[1] != wantMap[1] {
		t.Fatalf("CATEGORY_MAPPING members=%v want %v", got, wantMap)
	}
	for _, m := range byKind[entityKindCategoryMapping].Members {
		if contains(m, "77") || contains(m, "9001") || contains(m, "9002") {
			t.Fatalf("member leaked DB PK: %s", m)
		}
	}
}

func TestBuildEntitySetsHashIndependentOfDBPrimaryKeys(t *testing.T) {
	catA := int64(1)
	catB := int64(99999)
	a := memEntitySource{
		cats:  []factualapi.Category{{ID: 1, ProviderCategoryID: 42}},
		skus:  []factualapi.SKU{{ID: 10, OzonSKU: 7, Category: factualapi.CategoryRelation{CategoryID: &catA, MappingStatus: "MAPPED"}}},
		camps: []factualapi.Campaign{{ID: 3, ProviderCampaignID: 100}},
	}
	b := memEntitySource{
		cats:  []factualapi.Category{{ID: 99999, ProviderCategoryID: 42}},
		skus:  []factualapi.SKU{{ID: 888, OzonSKU: 7, Category: factualapi.CategoryRelation{CategoryID: &catB, MappingStatus: "MAPPED"}}},
		camps: []factualapi.Campaign{{ID: 444, ProviderCampaignID: 100}},
	}
	rc := testReadContext(t)
	sa, err := buildEntitySets(context.Background(), a, rc)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := buildEntitySets(context.Background(), b, rc)
	if err != nil {
		t.Fatal(err)
	}
	for i := range sa {
		if sa[i].SetHash != sb[i].SetHash || sa[i].MemberCount != sb[i].MemberCount {
			t.Fatalf("kind %s hash/count differ under different DB PKs: %+v vs %+v", sa[i].EntityKind, sa[i], sb[i])
		}
	}
}

func TestReadContextFromResolvedUsesCutoffBindings(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	runID, aid := int64(11), int64(22)
	obs := cut.Add(-time.Hour)
	m := resolve.Manifest{
		SellerAccountID: 1,
		AnalysisDate:    canonicaljson.Date(cut.AddDate(0, 0, -1)),
		CutoffAt:        cut,
		Policies:        factualapi.CurrentPolicies(),
		Sources: []resolve.SourceBinding{{
			Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", LastAcceptedDomainRunID: &runID,
			LatestObservedAt: &obs, SourceContractKey: "CATALOG", SourceContractVersion: "v1",
			SourceContractHash: "h", WatermarkHash: "wm",
		}},
		Capabilities: []resolve.CapabilitySnapshot{{
			Key: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY",
			AssessmentID: &aid, AssessedAt: obs,
		}},
	}
	rc, err := ReadContextFromResolved(m)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Mode() != factualapi.ModeLatestStable {
		t.Fatalf("mode=%s", rc.Mode())
	}
	if !rc.CutoffAt().Equal(cut.UTC()) {
		t.Fatalf("cutoff=%v", rc.CutoffAt())
	}
	b, ok := rc.DomainBinding("CATALOG")
	if !ok || b.DomainRunID != runID || b.WatermarkHash != "wm" || b.FactFamily != "CATALOG.SKU_UNIVERSE" {
		t.Fatalf("domain binding=%+v ok=%v", b, ok)
	}
}

func testReadContext(t *testing.T) factualapi.FactualReadContext {
	t.Helper()
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	rc, err := factualapi.NewReadContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    cut.AddDate(0, 0, -1),
		CutoffAt:        cut,
		Mode:            factualapi.ModeLatestStable,
		Policies:        factualapi.CurrentPolicies(),
		Domains:         []factualapi.DomainBinding{{Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", DomainRunID: 1, FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "h"}},
		Capabilities:    []factualapi.CapabilityBinding{{CapabilityKey: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut.Add(-time.Hour)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rc
}
