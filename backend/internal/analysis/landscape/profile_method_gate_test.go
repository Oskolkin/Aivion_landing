package landscape

import (
	"reflect"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestV3ProfileFactualMethodsExistOnAPI(t *testing.T) {
	typ := reflect.TypeOf((*factualapi.API)(nil)).Elem()
	methods := map[string]struct{}{}
	for i := 0; i < typ.NumMethod(); i++ {
		methods[typ.Method(i).Name] = struct{}{}
	}
	bundles := []struct {
		name string
		fn   func() ExecutableProfileV3
	}{
		{"account", func() ExecutableProfileV3 { return AccountDiscoveryV3() }},
		{"category_core", func() ExecutableProfileV3 { return CategoryCoreV3() }},
		{"category_detail", func() ExecutableProfileV3 { return CategoryDetailV3() }},
		{"sku_core", func() ExecutableProfileV3 { return SKUCoreV3() }},
		{"sku_detail", func() ExecutableProfileV3 { return SKUDetailV3() }},
		{"campaign_core", func() ExecutableProfileV3 { return CampaignCoreV3() }},
		{"campaign_detail", func() ExecutableProfileV3 { return CampaignDetailV3() }},
	}
	for _, b := range bundles {
		p := b.fn()
		for _, block := range p.Blocks {
			for _, method := range block.FactualMethods {
				if _, ok := methods[method]; !ok {
					t.Fatalf("profile %s block %s references missing factual API method %s", p.Name, block.BlockID, method)
				}
			}
		}
	}
}

func TestV4ProfileFactualMethodsExistOnAPI(t *testing.T) {
	typ := reflect.TypeOf((*factualapi.API)(nil)).Elem()
	methods := map[string]struct{}{}
	for i := 0; i < typ.NumMethod(); i++ {
		methods[typ.Method(i).Name] = struct{}{}
	}
	for _, p := range []ExecutableProfileV4{
		AccountDiscoveryV4(),
		CategoryCoreV4(),
		CategoryDetailV4(),
		SKUCoreV4(),
		SKUDetailV4(),
		CampaignCoreV4(),
		CampaignDetailV4(),
	} {
		if len(p.CallPlan()) == 0 {
			t.Fatalf("profile %s CallPlan empty", p.Name)
		}
		for _, block := range p.Blocks {
			for _, method := range block.FactualMethods {
				if _, ok := methods[method]; !ok {
					t.Fatalf("profile %s block %s references missing factual API method %s", p.Name, block.BlockID, method)
				}
			}
		}
	}
}

func TestV3ProfileCallPlanNonEmpty(t *testing.T) {
	for _, p := range []ExecutableProfileV3{
		AccountDiscoveryV3(),
		CategoryCoreV3(),
		CategoryDetailV3(),
		SKUCoreV3(),
		SKUDetailV3(),
		CampaignCoreV3(),
		CampaignDetailV3(),
	} {
		plan := p.CallPlan()
		if len(plan) == 0 {
			t.Fatalf("profile %s CallPlan empty", p.Name)
		}
	}
}
