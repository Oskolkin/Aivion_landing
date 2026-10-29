package aidatapack_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func compactCatalogPack() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		FactRegistry: map[string]aidatapack.Fact{
			"fact_1": {FactID: "fact_1", EntityID: "sku:1", EntityType: "sku", Metric: "revenue", Value: 1},
			"fact_2": {FactID: "fact_2", EntityID: "sku:2", EntityType: "sku", Metric: "orders", Value: 2},
			"fact_3": {FactID: "fact_3", EntityID: "campaign:1", EntityType: "campaign", Metric: "spend", Value: 3},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs: map[string]aidatapack.SKUEntity{
				"sku:1": {EntityID: "sku:1", EntityType: "sku", SKU: "1", Name: "One"},
				"sku:2": {EntityID: "sku:2", EntityType: "sku", SKU: "2", Name: "Two"},
			},
			Campaigns: map[string]aidatapack.CampaignEntity{
				"campaign:1": {EntityID: "campaign:1", EntityType: "campaign", Name: "Camp"},
			},
		},
	}
}

func TestBuildCompactFactCatalogOnlyRequestedFacts(t *testing.T) {
	pack := compactCatalogPack()
	catalog, err := aidatapack.BuildCompactFactCatalog(pack, []string{"fact_1", "fact_3", "fact_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("catalog=%v", catalog)
	}
	if _, ok := catalog["fact_2"]; ok {
		t.Fatal("fact_2 must be absent")
	}
}

func TestBuildCompactFactCatalogUnknownFactErrors(t *testing.T) {
	_, err := aidatapack.BuildCompactFactCatalog(compactCatalogPack(), []string{"missing"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildCompactFactCatalogEmptyList(t *testing.T) {
	catalog, err := aidatapack.BuildCompactFactCatalog(compactCatalogPack(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if catalog == nil || len(catalog) != 0 {
		t.Fatalf("catalog=%v", catalog)
	}
}

func TestBuildCompactEntityCatalogOnlyRequestedEntities(t *testing.T) {
	pack := compactCatalogPack()
	catalog, err := aidatapack.BuildCompactEntityCatalog(pack, []string{"sku:1", "campaign:1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("catalog=%v", catalog)
	}
	if _, ok := catalog["sku:2"]; ok {
		t.Fatal("sku:2 must be absent")
	}
}

func TestBuildCompactEntityCatalogUnknownEntityErrors(t *testing.T) {
	_, err := aidatapack.BuildCompactEntityCatalog(compactCatalogPack(), []string{"sku:missing"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildCompactEntityCatalogSystemEntities(t *testing.T) {
	catalog, err := aidatapack.BuildCompactEntityCatalog(compactCatalogPack(), []string{"account:seller"})
	if err != nil {
		t.Fatal(err)
	}
	if catalog["account:seller"].EntityType != "account" {
		t.Fatalf("entity=%+v", catalog["account:seller"])
	}
}

func TestCompactCatalogHelpersDoNotMutatePack(t *testing.T) {
	pack := compactCatalogPack()
	beforeFacts := len(pack.FactRegistry)
	beforeSKUs := len(pack.EntityRegistry.SKUs)
	if _, err := aidatapack.BuildCompactFactCatalog(pack, []string{"fact_1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := aidatapack.BuildCompactEntityCatalog(pack, []string{"sku:1"}); err != nil {
		t.Fatal(err)
	}
	if len(pack.FactRegistry) != beforeFacts || len(pack.EntityRegistry.SKUs) != beforeSKUs {
		t.Fatal("pack mutated")
	}
}
