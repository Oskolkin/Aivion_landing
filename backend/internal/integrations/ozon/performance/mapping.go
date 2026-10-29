package performance

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func mapCampaigns(data []map[string]any) []Campaign {
	items := make([]Campaign, 0, len(data))
	for _, row := range data {
		id := firstInt64(row, "campaign_id", "id", "campaignId")
		if id == 0 {
			continue
		}
		raw, _ := json.Marshal(row)
		budgetRaw := ParseMicroRublesFromAny(row, "budget")
		dailyRaw := ParseMicroRublesFromAny(row, "dailyBudget", "daily_budget")
		weeklyRaw := ParseMicroRublesFromAny(row, "weeklyBudget", "weekly_budget")
		placementRaw, _ := json.Marshal(row["placement"])
		autopilotRaw, _ := json.Marshal(row["autopilot"])
		isAutocreated, isAutocreatedPresent := firstBoolPresent(row, "isAutocreated", "is_autocreated")
		items = append(items, Campaign{
			CampaignExternalID:       id,
			CampaignName:             firstString(row, "title", "name"),
			CampaignType:             normalizeCampaignType(firstString(row, "advObjectType", "adv_object_type")),
			PlacementType:            firstString(row, "placement"),
			PlacementJSON:            placementRaw,
			Status:                   firstString(row, "state", "status"),
			PaymentType:              firstString(row, "paymentType", "payment_type"),
			BudgetRawMicro:           budgetRaw,
			DailyBudgetRawMicro:      dailyRaw,
			WeeklyBudgetRawMicro:     weeklyRaw,
			BudgetAmount:             normalizeBudgetAmount(row, budgetRaw, "budget"),
			BudgetDaily:              normalizeBudgetAmount(row, dailyRaw, "dailyBudget", "daily_budget"),
			BudgetWeekly:             normalizeBudgetAmount(row, weeklyRaw, "weeklyBudget", "weekly_budget"),
			FromDate:                 parseDateFlexible(firstString(row, "fromDate", "from_date")),
			ToDate:                   parseDateFlexible(firstString(row, "toDate", "to_date")),
			AutostopStatus:           firstString(row, "autostopStatus", "autostop_status"),
			IsAutocreated:            isAutocreated,
			IsAutocreatedPresent:     isAutocreatedPresent,
			ProductAutopilotStrategy: firstString(row, "productAutopilotStrategy", "product_autopilot_strategy"),
			ProductCampaignMode:      firstString(row, "productCampaignMode", "product_campaign_mode"),
			Autopilot:                autopilotRaw,
			CreatedAt:                parseDateFlexible(firstString(row, "createdAt", "created_at")),
			UpdatedAt:                parseDateFlexible(firstString(row, "updatedAt", "updated_at")),
			Raw:                      raw,
		})
	}
	return items
}

func normalizeBudgetAmount(row map[string]any, rawMicro int64, keys ...string) string {
	if rawMicro > 0 {
		return NormalizeMicroRubles(rawMicro)
	}
	return normalizeMoneyString(firstString(row, keys...))
}

func mapCampaignStatistics(data []map[string]any) []CampaignDailyMetric {
	items := make([]CampaignDailyMetric, 0, len(data))
	for _, row := range data {
		campaignID := firstInt64(row, "campaign_id", "id", "campaignId")
		if campaignID == 0 {
			continue
		}
		date := parseDateFlexible(firstString(row, "date", "day", "metric_date"))
		if date.IsZero() {
			continue
		}
		raw, _ := json.Marshal(row)
		impressions, impressionsPresent := firstInt64Present(row, "impressions")
		clicks, clicksPresent := firstInt64Present(row, "clicks")
		orders, ordersPresent := firstInt64Present(row, "orders", "ordersCount", "orders_count")
		spendRaw := ParseMicroRublesFromAny(row, "spend", "moneySpent", "expense")
		bonusRaw := ParseMicroRublesFromAny(row, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		revenueRaw := ParseMicroRublesFromAny(row, "ordersInRubles", "revenue", "ordersMoney", "sales")
		spend := normalizeBudgetAmount(row, spendRaw, "spend", "spendInRubles", "expense")
		bonus := normalizeBudgetAmount(row, bonusRaw, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		revenue := normalizeBudgetAmount(row, revenueRaw, "ordersInRubles", "revenue", "sales")
		drr := firstString(row, "drr", "DRR")
		if drr == "" {
			if v, ok := firstFloatString(row, "drr", "DRR"); ok {
				drr = v
			}
		}
		items = append(items, CampaignDailyMetric{
			CampaignExternalID: campaignID,
			CampaignName:       firstString(row, "campaignName", "campaign_name", "title", "name"),
			MetricDate:         date,
			Impressions:        impressions,
			ImpressionsPresent: impressionsPresent,
			Clicks:             clicks,
			ClicksPresent:      clicksPresent,
			Spend:              spend,
			BonusSpend:         bonus,
			OrdersCount:        orders,
			OrdersCountPresent: ordersPresent,
			Revenue:            revenue,
			ProviderDRR:        drr,
			Raw:                raw,
		})
	}
	return items
}

func firstFloatString(m map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case json.Number:
				return val.String(), true
			case float64:
				return strconv.FormatFloat(val, 'f', -1, 64), true
			case int64:
				return strconv.FormatInt(val, 10), true
			case int:
				return strconv.Itoa(val), true
			case string:
				if s := strings.TrimSpace(val); s != "" {
					return s, true
				}
			}
		}
	}
	return "", false
}

func mapCampaignProducts(campaignID int64, data []map[string]any) []CampaignPromotedProduct {
	items := make([]CampaignPromotedProduct, 0, len(data))
	for _, row := range data {
		// Contract: campaign products SKU identity is provider field "sku" only (never arbitrary "id").
		sku := firstInt64(row, "sku")
		if sku == 0 {
			continue
		}
		raw, _ := json.Marshal(row)
		bidRaw := ParseMicroRublesFromAny(row, "bid")
		bidAmount := ""
		bidUnit := ""
		if bidRaw > 0 {
			bidAmount = NormalizeMicroRubles(bidRaw)
			bidUnit = "micro_rubles"
		}
		isActive, _ := firstBoolPresent(row, "isActive", "is_active", "active")
		items = append(items, CampaignPromotedProduct{
			CampaignExternalID: campaignID,
			OzonProductID:      sku,
			SKU:                sku,
			Title:              firstString(row, "title"),
			IsActive:           isActive,
			Status:             firstString(row, "status", "state"),
			BidRawMicro:        bidRaw,
			BidAmount:          bidAmount,
			BidUnitAssumption:  bidUnit,
			Raw:                raw,
		})
	}
	return items
}

func firstBool(m map[string]any, keys ...string) bool {
	v, _ := firstBoolPresent(m, keys...)
	return v
}

func mapCampaignObjects(campaignID int64, advObjectType string, data []map[string]any) []CampaignObject {
	items := make([]CampaignObject, 0, len(data))
	objectType := inferObjectType(advObjectType)
	for _, row := range data {
		objectID := firstString(row, "id", "objectId", "object_id", "sku")
		if objectID == "" {
			if id := firstInt64(row, "id", "objectId", "object_id", "sku"); id > 0 {
				objectID = strconv.FormatInt(id, 10)
			}
		}
		if objectID == "" {
			continue
		}
		raw, _ := json.Marshal(row)
		items = append(items, CampaignObject{
			CampaignExternalID: campaignID,
			ObjectExternalID:   objectID,
			ObjectType:         objectType,
			Raw:                raw,
		})
	}
	return items
}

func inferObjectType(advObjectType string) string {
	switch normalizeCampaignType(advObjectType) {
	case "SKU":
		return "sku"
	case "BANNER", "VIDEO_BANNER":
		return "banner_object"
	default:
		return "unknown"
	}
}

func mapCampaignProductStatistics(data []map[string]any) []CampaignProductDailyMetric {
	items := make([]CampaignProductDailyMetric, 0, len(data))
	for _, row := range data {
		campaignID := firstInt64(row, "campaignId", "campaign_id", "id")
		sku := firstInt64(row, "sku", "SKU")
		date := parseDateFlexible(firstString(row, "date", "day", "metric_date"))
		if campaignID == 0 || sku == 0 || date.IsZero() {
			continue
		}
		raw, _ := json.Marshal(row)
		impressions, impressionsPresent := firstInt64Present(row, "views", "impressions")
		clicks, clicksPresent := firstInt64Present(row, "clicks")
		orders, ordersPresent := firstInt64Present(row, "orders", "ordersCount", "orders_count")
		spendRaw := ParseMicroRublesFromAny(row, "expense", "spend", "moneySpent")
		bonusRaw := ParseMicroRublesFromAny(row, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		revenueRaw := ParseMicroRublesFromAny(row, "sales", "ordersInRubles", "revenue", "ordersMoney")
		spend := normalizeBudgetAmount(row, spendRaw, "expense", "spend", "spendInRubles")
		bonus := normalizeBudgetAmount(row, bonusRaw, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		revenue := normalizeBudgetAmount(row, revenueRaw, "sales", "ordersInRubles", "revenue")
		drr := firstString(row, "drr", "DRR")
		if drr == "" {
			if v, ok := firstFloatString(row, "drr", "DRR"); ok {
				drr = v
			}
		}
		base := CampaignProductDailyMetric{
			CampaignExternalID: campaignID,
			SKU:                sku,
			MetricDate:         date,
			Impressions:        impressions,
			ImpressionsPresent: impressionsPresent,
			Clicks:             clicks,
			ClicksPresent:      clicksPresent,
			Spend:              spend,
			BonusSpend:         bonus,
			OrdersCount:        orders,
			OrdersCountPresent: ordersPresent,
			Revenue:            revenue,
			ProviderDRR:        drr,
			AttributionKind:    "PROVIDER_DEFAULT",
			Raw:                raw,
		}
		items = append(items, base)
		if modelOrders, ok := firstInt64Present(row, "modelOrders", "model_orders"); ok {
			model := base
			model.OrdersCount = modelOrders
			model.OrdersCountPresent = true
			model.AttributionKind = "MODEL"
			// MODEL view is attribution-only; do not reuse traffic/spend as a second traffic fact.
			model.ImpressionsPresent = false
			model.ClicksPresent = false
			model.Impressions = 0
			model.Clicks = 0
			model.Spend = ""
			model.BonusSpend = ""
			items = append(items, model)
		}
		if viewOrders, ok := firstInt64Present(row, "viewOrders", "view_orders"); ok {
			view := base
			view.OrdersCount = viewOrders
			view.OrdersCountPresent = true
			view.AttributionKind = "VIEW"
			view.ImpressionsPresent = false
			view.ClicksPresent = false
			view.Impressions = 0
			view.Clicks = 0
			view.Spend = ""
			view.BonusSpend = ""
			items = append(items, view)
		}
	}
	return items
}

func mapExpenseStatistics(data []map[string]any) []ExpenseDailyMetric {
	items := make([]ExpenseDailyMetric, 0, len(data))
	for _, row := range data {
		date := parseDateFlexible(firstString(row, "date", "day", "metric_date"))
		if date.IsZero() {
			continue
		}
		raw, _ := json.Marshal(row)
		paidRaw := ParseMicroRublesFromAny(row, "moneySpent", "spend", "expense", "paidSpend", "paid_spend")
		bonusRaw := ParseMicroRublesFromAny(row, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		paid := normalizeBudgetAmount(row, paidRaw, "moneySpent", "spend", "expense", "paidSpend", "paid_spend")
		bonus := normalizeBudgetAmount(row, bonusRaw, "bonusSpend", "bonus_spend", "moneyBonus", "bonus")
		items = append(items, ExpenseDailyMetric{
			MetricDate:   date,
			PaidSpend:    paid,
			BonusSpend:   bonus,
			CurrencyHint: firstString(row, "currency", "currencyCode", "currency_code"),
			Raw:          raw,
		})
	}
	return items
}

func mapSearchPromoProducts(data []map[string]any) []CampaignPromotedProduct {
	items := make([]CampaignPromotedProduct, 0, len(data))
	for _, row := range data {
		sku := firstInt64(row, "sku")
		if sku == 0 {
			continue
		}
		hintCampaignID := int64(0)
		if hint, ok := row["hint"].(map[string]any); ok {
			hintCampaignID = firstInt64(hint, "campaignId", "campaign_id")
		}
		if hintCampaignID == 0 {
			continue
		}
		raw, _ := json.Marshal(row)
		items = append(items, CampaignPromotedProduct{
			CampaignExternalID: hintCampaignID,
			OzonProductID:      sku,
			SKU:                sku,
			Title:              firstString(row, "title"),
			IsActive:           true,
			Status:             firstString(row, "searchPromoStatus", "status"),
			Raw:                raw,
		})
	}
	return items
}

func parseDateFlexible(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	layouts := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case string:
				return strings.TrimSpace(val)
			case json.Number:
				return val.String()
			case float64:
				return strconv.FormatFloat(val, 'f', -1, 64)
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	v, _ := firstInt64Present(m, keys...)
	return v
}
func firstInt64Present(m map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case int64:
				return val, true
			case int32:
				return int64(val), true
			case int:
				return int64(val), true
			case float64:
				return int64(val), true
			case json.Number:
				if i, err := val.Int64(); err == nil {
					return i, true
				}
			case string:
				if i, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
					return i, true
				}
			}
		}
	}
	return 0, false
}

func firstBoolPresent(m map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case bool:
				return val, true
			case string:
				v := strings.ToLower(strings.TrimSpace(val))
				return v == "true" || v == "1" || v == "active", true
			case float64:
				return val != 0, true
			case json.Number:
				return val.String() != "0", true
			}
		}
	}
	return false, false
}

func extractRows(raw map[string]any) []map[string]any {
	candidates := []string{"list", "products", "result", "items", "campaigns", "rows", "statistics"}
	for _, key := range candidates {
		if value, ok := raw[key]; ok {
			if rows := toMapSlice(value); len(rows) > 0 {
				return rows
			}
			// Some APIs wrap arrays one level deeper.
			if nested, ok := value.(map[string]any); ok {
				for _, inner := range candidates {
					if innerValue, exists := nested[inner]; exists {
						if rows := toMapSlice(innerValue); len(rows) > 0 {
							return rows
						}
					}
				}
			}
		}
	}
	return nil
}

func toMapSlice(v any) []map[string]any {
	array, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(array))
	for _, item := range array {
		if row, ok := item.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}

func normalizeCampaignType(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

func normalizeMoneyString(v string) string {
	normalized := strings.TrimSpace(v)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	normalized = strings.ReplaceAll(normalized, "₽", "")
	normalized = strings.ReplaceAll(normalized, "RUB", "")
	normalized = strings.ReplaceAll(normalized, "rub", "")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}
	if parsed, ok := new(big.Rat).SetString(normalized); ok {
		return parsed.FloatString(2)
	}
	return normalized
}
