package performance

import "testing"

func TestNormalizeMicroRubles(t *testing.T) {
	t.Parallel()
	if got := NormalizeMicroRubles(5_000_000); got != "5.00" {
		t.Fatalf("expected 5.00, got %s", got)
	}
	if got := NormalizeMicroRubles(0); got != "" {
		t.Fatalf("expected empty for zero, got %s", got)
	}
}

func TestMapCampaignsBudgetMicroConversion(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{
			"id":                       int64(42),
			"title":                    "Test campaign",
			"advObjectType":            "SKU",
			"state":                    "RUNNING",
			"paymentType":              "CPC",
			"budget":                   int64(10_000_000),
			"dailyBudget":              int64(1_500_000),
			"weeklyBudget":             int64(7_000_000),
			"placement":                "search",
			"productCampaignMode":      "AUTO",
			"productAutopilotStrategy": "TARGET_BIDS",
		},
	}
	items := mapCampaigns(rows)
	if len(items) != 1 {
		t.Fatalf("expected 1 campaign, got %d", len(items))
	}
	c := items[0]
	if c.BudgetRawMicro != 10_000_000 || c.BudgetAmount != "10.00" {
		t.Fatalf("budget raw=%d amount=%s", c.BudgetRawMicro, c.BudgetAmount)
	}
	if c.DailyBudgetRawMicro != 1_500_000 || c.BudgetDaily != "1.50" {
		t.Fatalf("daily budget raw=%d amount=%s", c.DailyBudgetRawMicro, c.BudgetDaily)
	}
	if c.WeeklyBudgetRawMicro != 7_000_000 || c.BudgetWeekly != "7.00" {
		t.Fatalf("weekly budget raw=%d amount=%s", c.WeeklyBudgetRawMicro, c.BudgetWeekly)
	}
	if c.ProductCampaignMode != "AUTO" || c.ProductAutopilotStrategy != "TARGET_BIDS" {
		t.Fatalf("unexpected mode/strategy: %s %s", c.ProductCampaignMode, c.ProductAutopilotStrategy)
	}
}

func TestMapCampaignProductsBidMicro(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"sku": int64(123), "title": "Product", "bid": int64(2_500_000)},
	}
	items := mapCampaignProducts(99, rows)
	if len(items) != 1 {
		t.Fatalf("expected 1 product, got %d", len(items))
	}
	if items[0].BidRawMicro != 2_500_000 || items[0].BidAmount != "2.50" {
		t.Fatalf("bid raw=%d amount=%s", items[0].BidRawMicro, items[0].BidAmount)
	}
}

func TestNormalizeMoneyStringNeverRoundsThroughFloat64(t *testing.T) {
	if got := normalizeMoneyString("123456789012345.125"); got != "123456789012345.13" {
		t.Fatalf("exact decimal normalization=%q", got)
	}
}

func TestDailyMappingDistinguishesMissingCountersFromKnownZero(t *testing.T) {
	result, err := ParseDailyStatisticsResponse([]byte(`{"items":[{"campaign_id":1,"date":"2026-08-24"},{"campaign_id":2,"date":"2026-08-24","impressions":0,"clicks":0,"orders":0}]}`))
	if err != nil || len(result.Items) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Items[0].ImpressionsPresent || result.Items[0].ClicksPresent || result.Items[0].OrdersCountPresent {
		t.Fatalf("missing counters became zero: %+v", result.Items[0])
	}
	if !result.Items[1].ImpressionsPresent || !result.Items[1].ClicksPresent || !result.Items[1].OrdersCountPresent {
		t.Fatalf("known zero presence lost: %+v", result.Items[1])
	}
}

func TestMapCampaignObjectsSKUType(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{{"id": "12345"}}
	items := mapCampaignObjects(10, "SKU", rows)
	if len(items) != 1 || items[0].ObjectType != "sku" || items[0].ObjectExternalID != "12345" {
		t.Fatalf("unexpected objects: %+v", items)
	}
}
