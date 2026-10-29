package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildTopActions_NormalizesAdvertisingZeroResult(t *testing.T) {
	items := buildTopActions(
		nil,
		[]dbgen.Recommendation{
			{
				ID:                 101,
				RecommendationType: "review_ad_spend",
				PriorityLevel:      "critical",
				EntityType:         "campaign",
				EntityID:           pgtype.Text{String: "cmp-77", Valid: true},
				Title:              "Pause campaign high ad_waste",
				WhatHappened:       "ROAS 0.12x, campaign spent money without result",
				WhyItMatters:       "critical",
				RecommendedAction:  "pause campaign",
				SupportingMetricsPayload: mustJSON(t, map[string]any{
					"campaign_name": "Поиск — бренд",
					"spend":         1250.0,
					"revenue":       0.0,
					"orders_count":  0.0,
				}),
			},
		},
		nil,
		nil,
		3,
	)

	if len(items) != 1 {
		t.Fatalf("len=%d want 1", len(items))
	}
	item := items[0]
	if item.Title != "Остановите неэффективную рекламную кампанию" {
		t.Fatalf("title=%q", item.Title)
	}
	if item.PriorityLabel != "Критический" {
		t.Fatalf("priority_label=%q", item.PriorityLabel)
	}
	if item.CategoryLabel != "Реклама" {
		t.Fatalf("category_label=%q", item.CategoryLabel)
	}
	if !strings.Contains(item.Reason, "1 250 ₽") {
		t.Fatalf("reason=%q", item.Reason)
	}
	if !strings.Contains(item.Reason, "не принесла заказов и выручки") {
		t.Fatalf("reason=%q", item.Reason)
	}
	if !strings.Contains(item.NextStep, "остановите кампанию") {
		t.Fatalf("next_step=%q", item.NextStep)
	}
	if len(item.SupportingMetrics) != 3 {
		t.Fatalf("supporting_metrics=%v", item.SupportingMetrics)
	}

	raw := strings.Join([]string{
		item.Title,
		item.Reason,
		item.NextStep,
		marshalSupportingMetrics(t, item.SupportingMetrics),
	}, " ")
	for _, forbidden := range []string{"ad_waste", "ad_weak_sku", "campaign", "ROAS", "CPO", "high", "critical", "0.12x"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("raw top action still contains %q: %s", forbidden, raw)
		}
	}
}

func TestBuildTopActions_NormalizesAdvertisingLowReturn(t *testing.T) {
	items := buildTopActions(
		nil,
		[]dbgen.Recommendation{
			{
				ID:                 102,
				RecommendationType: "review_ad_spend",
				PriorityLevel:      "critical",
				EntityType:         "campaign",
				EntityID:           pgtype.Text{String: "555", Valid: true},
				SupportingMetricsPayload: mustJSON(t, map[string]any{
					"roas": 0.12,
				}),
			},
		},
		nil,
		[]AdEfficiencyPoint{
			{CampaignExternalID: 555, CampaignName: "Перегрев бюджета", Spend: 29596, Revenue: 3473, Orders: 3},
		},
		3,
	)

	if len(items) != 1 {
		t.Fatalf("len=%d want 1", len(items))
	}
	item := items[0]
	if item.Title != "Сократите бюджет кампании с низкой отдачей" {
		t.Fatalf("title=%q", item.Title)
	}
	if !strings.Contains(item.Reason, "29 596 ₽") || !strings.Contains(item.Reason, "3 473 ₽") {
		t.Fatalf("reason=%q", item.Reason)
	}
	if !strings.Contains(item.Reason, "0,12 ₽ выручки") {
		t.Fatalf("reason=%q", item.Reason)
	}
	if strings.Contains(item.Reason, "0.12x") || strings.Contains(item.Reason, "0,12x") {
		t.Fatalf("raw x remained in reason=%q", item.Reason)
	}
	if !hasSupportingMetric(item.SupportingMetrics, "Окупаемость рекламы", "0,12 ₽ выручки на 1 ₽ расходов") {
		t.Fatalf("supporting_metrics=%v", item.SupportingMetrics)
	}
}

func TestBuildTopActions_NormalizesAdvertisingMonitorScenario(t *testing.T) {
	items := buildTopActions(
		nil,
		[]dbgen.Recommendation{
			{
				ID:                 103,
				RecommendationType: "pause_or_reduce_ads",
				PriorityLevel:      "high",
				EntityType:         "campaign",
				EntityID:           pgtype.Text{String: "556", Valid: true},
				WhatHappened:       "Кампания слабее лидирующих кампаний с 3,75x–4,56x",
			},
		},
		nil,
		[]AdEfficiencyPoint{
			{CampaignExternalID: 556, CampaignName: "Проверка отдачи", Spend: 51600, Revenue: 40266, Orders: 12},
		},
		3,
	)

	if len(items) != 1 {
		t.Fatalf("len=%d want 1", len(items))
	}
	item := items[0]
	if item.Title != "Поставьте рекламную кампанию на контроль" {
		t.Fatalf("title=%q", item.Title)
	}
	if !strings.Contains(item.Reason, "51 600 ₽") || !strings.Contains(item.Reason, "40 266 ₽") {
		t.Fatalf("reason=%q", item.Reason)
	}
	if !strings.Contains(item.Reason, "0,78 ₽ выручки") {
		t.Fatalf("reason=%q", item.Reason)
	}
	for _, forbidden := range []string{"0.78x", "0,78x", "3,75x", "4,56x", "campaign"} {
		if strings.Contains(item.Reason+" "+item.NextStep, forbidden) {
			t.Fatalf("forbidden token %q remained in item=%+v", forbidden, item)
		}
	}
}

func TestBuildTopActions_NormalizesStockSalesAndSurplus(t *testing.T) {
	items := buildTopActions(
		nil,
		[]dbgen.Recommendation{
			{
				ID:                 201,
				RecommendationType: "replenish_sku",
				PriorityLevel:      "high",
				EntityType:         "sku",
				EntityID:           pgtype.Text{String: "sku-1", Valid: true},
				SupportingMetricsPayload: mustJSON(t, map[string]any{
					"days_of_cover":   3.2,
					"stock_available": 14.0,
				}),
			},
			{
				ID:                 202,
				RecommendationType: "investigate_sales_drop",
				PriorityLevel:      "high",
				EntityType:         "account",
				SupportingMetricsPayload: mustJSON(t, map[string]any{
					"delta_percent":   -37.4,
					"current_orders":  23.0,
					"previous_orders": 37.0,
				}),
			},
		},
		[]dbgen.Alert{
			{
				ID:         301,
				AlertType:  "fbo_surplus_storage_risk",
				Severity:   "high",
				EntityType: "sku",
				EntityID:   pgtype.Text{String: "sku-2", Valid: true},
				EvidencePayload: mustJSON(t, map[string]any{
					"available_stock_count": 180.0,
					"idc":                   93.0,
				}),
			},
		},
		nil,
		3,
	)

	if len(items) != 3 {
		t.Fatalf("len=%d want 3", len(items))
	}

	stock := findTopActionByCategory(items, "stock", "Проверьте товар с риском дефицита")
	if stock == nil {
		t.Fatalf("stock action not found: %+v", items)
	}
	if !strings.Contains(stock.Reason, "примерно на 3,2") {
		t.Fatalf("stock reason=%q", stock.Reason)
	}
	if !strings.Contains(stock.NextStep, "запланируйте поставку") {
		t.Fatalf("stock next_step=%q", stock.NextStep)
	}

	sales := findTopActionByCategory(items, "sales", "Разберите причину падения продаж")
	if sales == nil {
		t.Fatalf("sales action not found: %+v", items)
	}
	if !strings.Contains(sales.NextStep, "товары с наибольшей просадкой") {
		t.Fatalf("sales next_step=%q", sales.NextStep)
	}

	surplus := findTopActionByCategory(items, "stock", "Проверьте товар с избыточным остатком")
	if surplus == nil {
		t.Fatalf("surplus action not found: %+v", items)
	}
	if !strings.Contains(surplus.Reason, "избыточного хранения") {
		t.Fatalf("surplus reason=%q", surplus.Reason)
	}
}

func TestBuildTopActions_AppliesNormalizationForAllSources(t *testing.T) {
	tests := []struct {
		name       string
		build      func() []TopActionItem
		wantSource string
	}{
		{
			name: "recommendation",
			build: func() []TopActionItem {
				return buildTopActions(nil, []dbgen.Recommendation{
					{
						ID:                 501,
						RecommendationType: "replenish_sku",
						PriorityLevel:      "high",
						EntityType:         "sku",
						EntityID:           pgtype.Text{String: "sku-77", Valid: true},
						SupportingMetricsPayload: mustJSON(t, map[string]any{
							"days_of_cover": 2.5,
						}),
					},
				}, nil, nil, 1)
			},
			wantSource: "recommendation",
		},
		{
			name: "ai_summary",
			build: func() []TopActionItem {
				return buildTopActions([]ActionItem{
					{
						Priority:          "high",
						Action:            "Проверьте товар с риском дефицита",
						Reason:            "SKU low stock",
						RelatedEntityType: "sku",
						RelatedEntityID:   strPtr("sku-77"),
						Source:            "ai_summary",
					},
				}, nil, nil, nil, 1)
			},
			wantSource: "ai_summary",
		},
		{
			name: "alert",
			build: func() []TopActionItem {
				return buildTopActions(nil, nil, []dbgen.Alert{
					{
						ID:         601,
						AlertType:  "stock_oos_risk",
						Severity:   "critical",
						EntityType: "sku",
						EntityID:   pgtype.Text{String: "sku-88", Valid: true},
						EvidencePayload: mustJSON(t, map[string]any{
							"days_of_cover": 2.5,
						}),
					},
				}, nil, 1)
			},
			wantSource: "alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := tt.build()
			if len(items) != 1 {
				t.Fatalf("len=%d want 1", len(items))
			}
			if items[0].Source != tt.wantSource {
				t.Fatalf("source=%q want %q", items[0].Source, tt.wantSource)
			}
			assertTopActionFieldsFilled(t, items[0])
		})
	}
}

func TestBuildTopActions_RecommendationWinsOverAISummaryAndAlert(t *testing.T) {
	items := buildTopActions(
		[]ActionItem{
			{
				Priority:          "high",
				Action:            "Проверьте товар с риском дефицита",
				Reason:            "SKU low stock",
				RelatedEntityType: "sku",
				RelatedEntityID:   strPtr("sku-77"),
				Source:            "ai_summary",
			},
		},
		[]dbgen.Recommendation{
			{
				ID:                 501,
				RecommendationType: "replenish_sku",
				PriorityLevel:      "high",
				EntityType:         "sku",
				EntityID:           pgtype.Text{String: "sku-77", Valid: true},
				SupportingMetricsPayload: mustJSON(t, map[string]any{
					"days_of_cover": 2.5,
				}),
			},
		},
		[]dbgen.Alert{
			{
				ID:         601,
				AlertType:  "stock_oos_risk",
				Severity:   "critical",
				EntityType: "sku",
				EntityID:   pgtype.Text{String: "sku-77", Valid: true},
				EvidencePayload: mustJSON(t, map[string]any{
					"days_of_cover": 2.5,
				}),
			},
		},
		nil,
		1,
	)

	if len(items) != 1 {
		t.Fatalf("len=%d want 1", len(items))
	}
	if items[0].Source != "recommendation" {
		t.Fatalf("source=%q want recommendation", items[0].Source)
	}
}

func TestNormalizeTopActionCandidate_EmptyFieldGuardAndTechnicalTokens(t *testing.T) {
	item := normalizeTopActionCandidate(topActionCandidate{
		Source:            "ai_summary",
		Priority:          "high",
		RelatedEntityType: "campaign",
		RelatedEntityID:   strPtr("cmp-12"),
		Title:             "",
		WhatHappened:      "ROAS 0.12x, ad_weak_sku и high severity",
		NextStep:          "",
	}, topActionContext{})

	assertTopActionFieldsFilled(t, item)
	joined := item.Title + " " + item.Reason + " " + item.NextStep
	for _, forbidden := range []string{"ad_waste", "ad_weak_sku", "campaign", "ROAS", "CPO", "0.12x", "0,12x", "high", "critical"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("joined text still contains %q: %s", forbidden, joined)
		}
	}
}

func mustJSON(t *testing.T, value map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func marshalSupportingMetrics(t *testing.T, metrics []TopActionSupportingMetric) string {
	t.Helper()
	raw, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("marshal supporting metrics: %v", err)
	}
	return string(raw)
}

func findTopActionByCategory(items []TopActionItem, category string, title string) *TopActionItem {
	for i := range items {
		if items[i].Category == category && items[i].Title == title {
			return &items[i]
		}
	}
	return nil
}

func hasSupportingMetric(metrics []TopActionSupportingMetric, label string, value string) bool {
	for _, metric := range metrics {
		if metric.Label == label && metric.Value == value {
			return true
		}
	}
	return false
}

func assertTopActionFieldsFilled(t *testing.T, item TopActionItem) {
	t.Helper()
	for _, value := range []string{item.ID, item.Source, item.Priority, item.PriorityLabel, item.Category, item.CategoryLabel, item.Title, item.Reason, item.NextStep} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("top action has empty required field: %+v", item)
		}
	}
}
