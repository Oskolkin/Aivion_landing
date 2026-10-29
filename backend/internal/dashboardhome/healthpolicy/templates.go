package healthpolicy

import (
	"fmt"
	"strings"
)

// DomainLabel returns the seller-facing Russian label for a candidate
// Domain. Self-contained (does not depend on dashboardhome's unexported
// category-label table) so this package has no cyclic/private coupling.
func DomainLabel(domain string) string {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "sales":
		return "Продажи"
	case "stock":
		return "Остатки"
	case "sku":
		return "SKU"
	case "surplus", "slow_turnover":
		return "Медленная оборачиваемость"
	case "funnel":
		return "Воронка"
	case "advertising":
		return "Реклама"
	case "data_quality":
		return "Данные"
	default:
		return "Кабинет"
	}
}

// DefaultTitle returns the backend default seller-facing title for a known
// driver_id. Used when the AI has not (yet) supplied an overlay Title for a
// registry candidate that is still shown in the display list.
func DefaultTitle(driverID string) string {
	switch driverID {
	case DriverSalesDecline:
		return "Снижаются заказы"
	case DriverRevenueDecline:
		return "Снижается выручка"
	case DriverKeySKUStockRisk:
		return "Риск дефицита по ключевым товарам"
	case DriverStockout:
		return "Товары закончились на складе"
	case DriverNoSalesWithStock:
		return "Товары в наличии не продаются"
	case DriverSurplus:
		return "Часть остатков оборачивается медленно"
	case DriverViewToCartDecline:
		return "Ослабла конверсия в корзину"
	case DriverCartToOrderDecline:
		return "Ослабла конверсия в заказ"
	case DriverHighDRRCampaign:
		return "Кампании с высокой ДРР"
	case DriverCampaignSpendWithoutOrders:
		return "Реклама тратит бюджет без заказов"
	case DriverAdShareGrowth:
		return "Растёт доля рекламных расходов"
	case DriverDataLimitation:
		return "Часть данных недоступна"
	default:
		return "Риск по кабинету"
	}
}

// DefaultReason renders a generic backend Russian sentence from a
// candidate's ReasonTemplateData. This is only a fallback for registry
// candidates the AI did not choose to explain — it is deliberately simple
// and never invents numbers beyond what ReasonTemplateData carries.
func DefaultReason(c RiskCandidate) string {
	data := c.ReasonTemplateData
	switch c.DriverID {
	case DriverSalesDecline:
		return fmt.Sprintf("Заказы снизились на %s к предыдущему дню.", formatPercentValue(templateFloat(data, "orders_day_delta_pct")))
	case DriverRevenueDecline:
		return fmt.Sprintf("Выручка снизилась на %s к предыдущему дню и на %s к предыдущим 7 дням.",
			formatPercentValue(templateFloat(data, "revenue_day_delta_pct")),
			formatPercentValue(templateFloat(data, "revenue_week_delta_pct")))
	case DriverKeySKUStockRisk:
		return fmt.Sprintf("У %d ключевых SKU критичное покрытие и ещё у %d — короткое покрытие (4–7 дней).",
			templateInt(data, "critical_key_sku_count"), templateInt(data, "shortage_key_sku_count"))
	case DriverStockout:
		return fmt.Sprintf("%d товаров, которые продавались за последние 7 дней, сейчас отсутствуют на складе.", templateInt(data, "stockout_sku_count"))
	case DriverNoSalesWithStock:
		return fmt.Sprintf("%d товаров есть в наличии, но не продаются.", templateInt(data, "no_sales_with_stock_sku_count"))
	case DriverSurplus:
		return fmt.Sprintf("%d товаров оборачиваются медленно или имеют избыточный остаток.", templateInt(data, "surplus_sku_count"))
	case DriverViewToCartDecline:
		return fmt.Sprintf("Этап «%s» просел на %s к предыдущему дню.", templateString(data, "stage_label"), formatPercentValue(absFloat(templateFloat(data, "change_vs_previous_day_pct"))))
	case DriverCartToOrderDecline:
		return fmt.Sprintf("Этап «%s» просел на %s к предыдущему дню.", templateString(data, "stage_label"), formatPercentValue(absFloat(templateFloat(data, "change_vs_previous_day_pct"))))
	case DriverHighDRRCampaign:
		return fmt.Sprintf("%d кампаний работают с высокой ДРР, расход под риском — %s ₽.", templateInt(data, "campaigns_high_drr_count"), formatMoneyValue(templateFloat(data, "high_drr_spend")))
	case DriverCampaignSpendWithoutOrders:
		return fmt.Sprintf("%d кампаний расходуют бюджет без заказов, потрачено %s ₽.", templateInt(data, "campaigns_no_orders_count"), formatMoneyValue(templateFloat(data, "spend_without_orders")))
	case DriverAdShareGrowth:
		return fmt.Sprintf("Доля рекламных расходов выросла на %s п.п.", formatPercentValue(templateFloat(data, "drr_growth_pp")))
	case DriverDataLimitation:
		return "Часть источников данных недоступна или неполна — показатели по этим направлениям не учитываются в оценке."
	default:
		return "Есть отклонение по кабинету, требующее внимания."
	}
}

func templateFloat(data map[string]any, key string) float64 {
	if data == nil {
		return 0
	}
	switch v := data[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func templateInt(data map[string]any, key string) int {
	if data == nil {
		return 0
	}
	switch v := data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func templateString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func formatPercentValue(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

func formatMoneyValue(v float64) string {
	return fmt.Sprintf("%.0f", v)
}
