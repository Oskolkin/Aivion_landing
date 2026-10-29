/** User-facing labels for recommendation type codes (internal identifiers unchanged). */

const LEGACY_RECOMMENDATION_TYPES = new Set([
  "review_price_margin",
  "review_price_floor",
  "review_price_below_min",
  "review_price_above_max",
  "review_margin_risk",
  "add_pricing_constraints_for_key_sku",
  "review_price_and_ads_for_sku",
]);

export const ACTIVE_RECOMMENDATION_TYPES = [
  "investigate_sales_drop",
  "replenish_sku",
  "review_ad_spend",
  "pause_or_reduce_ads",
  "avoid_ads_for_low_stock_sku",
  "discount_overstock",
  "monitor_sku",
  "account_priority_review",
] as const;

export const RECOMMENDATION_TYPE_LABELS: Record<string, string> = {
  investigate_sales_drop: "Разобрать падение продаж",
  investigate_sku_drop: "Разобрать просадку SKU",
  focus_on_negative_contributor_sku: "SKU с негативным вкладом",
  replenish_sku: "Пополнить SKU",
  prioritize_key_sku_replenishment: "Приоритет пополнения ключевых SKU",
  reduce_ads_until_stock_recovers: "Снизить рекламу до восстановления остатков",
  review_campaign_without_result: "Проверить кампанию без результата",
  reduce_or_pause_inefficient_campaign: "Снизить или остановить неэффективную кампанию",
  redirect_ad_budget_from_low_stock_sku: "Перенаправить бюджет с SKU с низким остатком",
  rebalance_ads_and_stock: "Сбалансировать рекламу и остатки",
  prioritize_sku_recovery_plan: "План восстановления SKU",
  review_ad_spend: "Проверить рекламные расходы",
  pause_or_reduce_ads: "Снизить или остановить рекламу",
  avoid_ads_for_low_stock_sku: "Не рекламировать SKU с низким остатком",
  discount_overstock: "Разобрать излишки",
  monitor_sku: "Наблюдать за SKU",
  account_priority_review: "Приоритеты аккаунта",
};

export function isLegacyRecommendationType(type: string): boolean {
  return LEGACY_RECOMMENDATION_TYPES.has(type);
}

export function recommendationTypeLabel(type: string): string {
  if (LEGACY_RECOMMENDATION_TYPES.has(type)) {
    return "Устаревший тип";
  }
  return RECOMMENDATION_TYPE_LABELS[type] ?? type.replaceAll("_", " ");
}
