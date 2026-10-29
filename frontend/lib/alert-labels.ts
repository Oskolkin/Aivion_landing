/** Seller-facing labels and active/legacy classification for alert groups and types. */

/** Active seller-facing alert groups in MVP. */
export const ACTIVE_ALERT_GROUPS = ["sales", "stock", "advertising"] as const;
export type ActiveAlertGroup = (typeof ACTIVE_ALERT_GROUPS)[number];

/** Legacy groups kept for API/history compatibility (not active seller filters). */
export const LEGACY_ALERT_GROUPS = ["price_economics"] as const;
export type LegacyAlertGroup = (typeof LEGACY_ALERT_GROUPS)[number];

export type AlertGroup = ActiveAlertGroup | LegacyAlertGroup;

const LEGACY_ALERT_TYPES = new Set([
  "sku_revenue_drop",
  "stock_advertised_sku_low_stock",
  "ad_spend_without_result",
  "ad_budget_on_weak_sku",
  "ad_budget_on_low_stock_sku",
  "price_below_min_constraint",
  "price_above_max_constraint",
  "margin_risk_at_current_price",
  "margin_risk_at_min_price",
  "missing_pricing_constraints_for_key_sku",
]);

const ACTIVE_ALERT_TYPES = new Set([
  "sales_revenue_drop",
  "sales_orders_drop",
  "sku_orders_drop",
  "sku_negative_contribution",
  "stock_low_coverage",
  "stock_oos_risk",
  "stock_critical_sku_low_stock",
  "fbo_surplus_storage_risk",
  "ad_weak_campaign_efficiency",
]);

const ALERT_TYPE_LABELS: Record<string, string> = {
  sales_revenue_drop: "Падение выручки (аккаунт)",
  sales_orders_drop: "Падение заказов (аккаунт)",
  sku_orders_drop: "Просадка заказов по SKU",
  sku_negative_contribution: "Негативный вклад SKU",
  stock_low_coverage: "Низкое покрытие остатков",
  stock_oos_risk: "Риск отсутствия на складе",
  stock_critical_sku_low_stock: "Критический остаток ключевого SKU",
  fbo_surplus_storage_risk: "Риск излишков FBO",
  ad_weak_campaign_efficiency: "Слабая эффективность кампании",
};

const ALERT_GROUP_LABELS: Record<string, string> = {
  sales: "Продажи",
  stock: "Склад",
  advertising: "Реклама",
};

export function isActiveAlertType(type: string): boolean {
  return ACTIVE_ALERT_TYPES.has(type);
}

export function isLegacyAlertType(type: string): boolean {
  return LEGACY_ALERT_TYPES.has(type);
}

export function isActiveAlertGroup(group: string): boolean {
  return (ACTIVE_ALERT_GROUPS as readonly string[]).includes(group);
}

export function isLegacyAlertGroup(group: string): boolean {
  return (LEGACY_ALERT_GROUPS as readonly string[]).includes(group);
}

/** Seller-facing filters and default UI should not treat legacy groups as active signals. */
export function isSellerFacingAlertGroup(group: string): boolean {
  return isActiveAlertGroup(group);
}

export function isSellerHiddenAlertGroup(group: string): boolean {
  return isLegacyAlertGroup(group);
}

export function alertTypeLabel(type: string): string {
  if (isLegacyAlertType(type)) {
    return "Устаревший тип";
  }
  return ALERT_TYPE_LABELS[type] ?? type.replaceAll("_", " ");
}

export function alertGroupLabel(group: string): string {
  if (isLegacyAlertGroup(group)) {
    return "Устаревший тип";
  }
  return ALERT_GROUP_LABELS[group] ?? group.replaceAll("_", " ");
}

/** Active seller-facing alert group filter options (excludes price_economics). */
export const SELLER_ALERT_GROUP_OPTIONS: [string, string][] = [
  ["", "Все"],
  ["sales", "Продажи"],
  ["stock", "Склад"],
  ["advertising", "Реклама"],
];
