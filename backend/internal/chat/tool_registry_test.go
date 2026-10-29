package chat

import "testing"

func TestDefaultToolRegistryContainsExpectedTools(t *testing.T) {
	registry := NewDefaultToolRegistry()
	expected := []string{
		ToolGetDashboardSummary,
		ToolGetOpenRecommendations,
		ToolGetRecommendationDetail,
		ToolGetOpenAlerts,
		ToolGetAlertsByGroup,
		ToolGetCriticalSKUs,
		ToolGetStockRisks,
		ToolGetAdvertisingAnalytics,
		ToolGetPriceEconomicsRisks,
		ToolGetSKUMetrics,
		ToolGetSKUContext,
		ToolGetCampaignContext,
		ToolRunABCAnalysis,
	}
	if len(registry.List()) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(registry.List()))
	}
	for _, name := range expected {
		def, ok := registry.Get(name)
		if !ok {
			t.Fatalf("missing default tool: %s", name)
		}
		if def.Name == "" {
			t.Fatalf("empty tool name for %s", name)
		}
		if def.Purpose == "" && def.Description == "" {
			t.Fatalf("tool has no purpose/description: %s", name)
		}
		if len(def.SupportedIntents) == 0 {
			t.Fatalf("tool has no supported intents: %s", name)
		}
		if def.OutputShape == "" {
			t.Fatalf("tool has empty output shape: %s", name)
		}
		if _, exists := def.AllowedArgs["seller_account_id"]; exists {
			t.Fatalf("forbidden arg seller_account_id is present in %s", name)
		}
	}
}

func TestDefaultToolRegistryToolsReadOnly(t *testing.T) {
	registry := NewDefaultToolRegistry()
	for _, def := range registry.List() {
		if !def.ReadOnly {
			t.Fatalf("tool must be read-only: %s", def.Name)
		}
		if arg, ok := def.AllowedArgs["limit"]; ok {
			if arg.Default == nil {
				t.Fatalf("limit arg default is empty: %s", def.Name)
			}
			if arg.MaxInt == nil {
				t.Fatalf("limit arg max is empty: %s", def.Name)
			}
		}
	}
}

func TestRegistrySpecificContracts(t *testing.T) {
	registry := NewDefaultToolRegistry()

	abc, ok := registry.Get(ToolRunABCAnalysis)
	if !ok {
		t.Fatal("missing run_abc_analysis")
	}
	metric := abc.AllowedArgs["metric"]
	if len(metric.AllowedValues) != 2 || metric.AllowedValues[0] != "revenue" || metric.AllowedValues[1] != "orders" {
		t.Fatalf("unexpected metric allowed values: %+v", metric.AllowedValues)
	}

	byGroup, ok := registry.Get(ToolGetAlertsByGroup)
	if !ok {
		t.Fatal("missing get_alerts_by_group")
	}
	group := byGroup.AllowedArgs["group"]
	if len(group.AllowedValues) != 4 {
		t.Fatalf("unexpected group allowed values count: %d", len(group.AllowedValues))
	}

	openRecs, ok := registry.Get(ToolGetOpenRecommendations)
	if !ok {
		t.Fatal("missing get_open_recommendations")
	}
	if len(openRecs.AllowedArgs["priority_levels"].AllowedValues) != 4 {
		t.Fatalf("unexpected priority_levels values count: %d", len(openRecs.AllowedArgs["priority_levels"].AllowedValues))
	}

	skuContext, ok := registry.Get(ToolGetSKUContext)
	if !ok {
		t.Fatal("missing get_sku_context")
	}
	if _, ok := skuContext.AllowedArgs["sku"]; !ok {
		t.Fatal("sku arg is missing for get_sku_context")
	}
	if _, ok := skuContext.AllowedArgs["offer_id"]; !ok {
		t.Fatal("offer_id arg is missing for get_sku_context")
	}

	campaignContext, ok := registry.Get(ToolGetCampaignContext)
	if !ok {
		t.Fatal("missing get_campaign_context")
	}
	if !campaignContext.AllowedArgs["campaign_id"].Required {
		t.Fatal("campaign_id must be required for get_campaign_context")
	}
}
