package dashboardhome

import (
	"regexp"
	"strings"
)

var (
	actionMalformedWhatChangedStart = regexp.MustCompile(`^\d+\s*,`)
	actionGenericSignalPattern      = regexp.MustCompile(`(?i)сигнал продолжает сохраняться`)
)

type semanticActionCategory struct {
	Category string
	DriverID string
	Label    string
}

func inferSemanticActionCategory(title, reason, nextStep string) semanticActionCategory {
	text := strings.ToLower(strings.TrimSpace(title + " " + reason + " " + nextStep))
	switch {
	case matchesAdvertisingActionText(text):
		return semanticActionCategory{Category: "advertising", DriverID: driverAdvertising, Label: topActionCategoryLabel("advertising")}
	case matchesFunnelActionText(text):
		return semanticActionCategory{Category: "funnel", DriverID: driverFunnel, Label: topActionCategoryLabel("funnel")}
	case matchesStockActionText(text):
		return semanticActionCategory{Category: "stock", DriverID: driverStock, Label: topActionCategoryLabel("stock")}
	case matchesSalesActionText(text):
		return semanticActionCategory{Category: "sales", DriverID: driverAccountSales, Label: topActionCategoryLabel("sales")}
	default:
		return semanticActionCategory{}
	}
}

func matchesAdvertisingActionText(text string) bool {
	if strings.Contains(text, "реклам") || strings.Contains(text, "дрр") || strings.Contains(text, "кампани") {
		return true
	}
	for _, token := range []string{"ставк", "бюджет", "roas", "cpo", "cpc", "ctr кампани", "рекламн"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	if strings.Contains(text, "расход") && (strings.Contains(text, "реклам") || strings.Contains(text, "кампани") || strings.Contains(text, "дрр")) {
		return true
	}
	return false
}

func matchesStockActionText(text string) bool {
	for _, token := range []string{"остат", "дефицит", "пополн", "out-of-stock", "покрыти", "законч", "поставк", "наличи"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func matchesFunnelActionText(text string) bool {
	for _, token := range []string{"воронк", "корзин", "просмотр", "показ", "ctr кабинет", "конверси"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func matchesSalesActionText(text string) bool {
	for _, token := range []string{"выручк", "заказ", "оборот", "спрос"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func repairNarrativeTopActionSemantic(action *dashboardNarrativeTopAction) bool {
	if action == nil {
		return false
	}
	semantic := inferSemanticActionCategory(action.Title, action.Reason, action.NextStep)
	if semantic.Category == "" {
		return false
	}
	changed := false
	if action.Category != semantic.Category {
		action.Category = semantic.Category
		changed = true
	}
	if semantic.DriverID != "" && action.DriverID != semantic.DriverID {
		action.DriverID = semantic.DriverID
		changed = true
	}
	return changed
}

func repairTopActionItemSemantic(item *TopActionItem) bool {
	if item == nil {
		return false
	}
	semantic := inferSemanticActionCategory(item.Title, item.Reason, item.NextStep)
	if semantic.Category == "" {
		return false
	}
	changed := false
	if item.Category != semantic.Category {
		item.Category = semantic.Category
		item.CategoryLabel = semantic.Label
		changed = true
	}
	if semantic.DriverID != "" && item.DriverID != semantic.DriverID {
		item.DriverID = semantic.DriverID
		changed = true
	}
	return changed
}

func repairTopActionsSemantic(items []TopActionItem) {
	for i := range items {
		repairTopActionItemSemantic(&items[i])
	}
}

func actionCategoryConsistentWithText(item TopActionItem) bool {
	if dashboardAIOnlyEnabled() {
		if expected := categoryForDriverID(item.DriverID); expected != "" {
			return strings.EqualFold(strings.TrimSpace(item.Category), expected)
		}
	}
	semantic := inferSemanticActionCategory(item.Title, item.Reason, item.NextStep)
	if semantic.Category == "" {
		return true
	}
	return item.Category == semantic.Category && item.DriverID == semantic.DriverID
}

func categoryForDriverID(driverID string) string {
	switch strings.TrimSpace(driverID) {
	case driverStock:
		return "stock"
	case driverSlowTurnover:
		return "slow_turnover"
	case driverAdvertising:
		return "advertising"
	case driverFunnel:
		return "funnel"
	case driverAccountSales, "driver-sales-revenue":
		return "sales"
	default:
		return ""
	}
}

func quickActionKindMatchesCategory(kind, category string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	category = strings.ToLower(strings.TrimSpace(category))
	if kind == "" || category == "" {
		return true
	}
	switch category {
	case "advertising":
		return kind == "advertising"
	case "stock":
		return kind == "stock"
	case "funnel":
		return kind == "funnel"
	case "sales", "products":
		return kind == "products" || kind == "sales"
	case "slow_turnover":
		return kind == "slow_turnover" || kind == "stock" || kind == "products"
	default:
		return true
	}
}
