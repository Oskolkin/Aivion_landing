package performance

import (
	"encoding/json"
	"testing"
)

func TestMapCampaignProductAndExpenseStatistics(t *testing.T) {
	product := mapCampaignProductStatistics([]map[string]any{{
		"campaignId": json.Number("11"),
		"sku":        json.Number("101"),
		"date":       "2026-08-24",
		"views":      json.Number("5"),
		"clicks":     json.Number("1"),
		"expense":    "12.5",
		"bonusSpend": "1.5",
		"orders":     json.Number("2"),
		"sales":      "40",
		"drr":        json.Number("31.25"),
	}})
	if len(product) != 1 || product[0].SKU != 101 || product[0].Spend != "12.50" || product[0].BonusSpend != "1.50" || product[0].ProviderDRR != "31.25" {
		t.Fatalf("product=%+v", product)
	}
	expense := mapExpenseStatistics([]map[string]any{{
		"date":       "2026-08-24",
		"moneySpent": "12.5",
		"bonusSpend": "1.5",
	}})
	if len(expense) != 1 || expense[0].PaidSpend != "12.50" || expense[0].BonusSpend != "1.50" {
		t.Fatalf("expense=%+v", expense)
	}
}
