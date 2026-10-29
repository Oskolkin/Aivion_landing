export type ActionFamily =
  | "INVENTORY"
  | "PRICING"
  | "ADVERTISING"
  | "PROMOTION"
  | "ASSORTMENT"
  | "ACCOUNT_HEALTH";

export type ActionIconRole =
  | "account"
  | "price"
  | "stock"
  | "ads"
  | "promo"
  | "catalog"
  | "campaign";

export type ParameterFormatterId =
  | "none"
  | "sku_price"
  | "sku_replenishment"
  | "sku_promotion"
  | "campaign_budget"
  | "campaign_product_set"
  | "category_plan"
  | "account_rating";

export type ActionPresentation = {
  action_type: CanonicalActionType;
  family: ActionFamily;
  presentation_template_key: string;
  template: string;
  label: string;
  icon: ActionIconRole;
  detail: ParameterFormatterId;
  columns: string[];
  parameter_formatter: ParameterFormatterId;
};

export const CANONICAL_ACTION_TYPES = [
  "ACCOUNT_RATING_REMEDIATION",
  "CATEGORY_REPRICING_PLAN",
  "CATEGORY_REPLENISHMENT_PLAN",
  "CATEGORY_ADVERTISING_REALLOCATION",
  "CATEGORY_PROMOTION_PLAN",
  "CATEGORY_ASSORTMENT_RATIONALIZATION",
  "SKU_ADJUST_PRICE",
  "SKU_INCREASE_REPLENISHMENT",
  "SKU_REDUCE_REPLENISHMENT",
  "SKU_ADJUST_AD_BID",
  "SKU_CHANGE_AD_PARTICIPATION",
  "SKU_JOIN_PROMOTION",
  "SKU_LEAVE_PROMOTION",
  "SKU_ADJUST_PROMOTION_PRICE",
  "SKU_CHANGE_CATALOG_STATE",
  "CAMPAIGN_ADJUST_BUDGET",
  "CAMPAIGN_CHANGE_STATE",
  "CAMPAIGN_ADJUST_STRATEGY",
  "CAMPAIGN_ADJUST_PRODUCT_SET",
] as const;

export type CanonicalActionType = (typeof CANONICAL_ACTION_TYPES)[number];

const FAMILY_BY_TYPE: Record<CanonicalActionType, ActionFamily> = {
  ACCOUNT_RATING_REMEDIATION: "ACCOUNT_HEALTH",
  CATEGORY_REPRICING_PLAN: "PRICING",
  CATEGORY_REPLENISHMENT_PLAN: "INVENTORY",
  CATEGORY_ADVERTISING_REALLOCATION: "ADVERTISING",
  CATEGORY_PROMOTION_PLAN: "PROMOTION",
  CATEGORY_ASSORTMENT_RATIONALIZATION: "ASSORTMENT",
  SKU_ADJUST_PRICE: "PRICING",
  SKU_INCREASE_REPLENISHMENT: "INVENTORY",
  SKU_REDUCE_REPLENISHMENT: "INVENTORY",
  SKU_ADJUST_AD_BID: "ADVERTISING",
  SKU_CHANGE_AD_PARTICIPATION: "ADVERTISING",
  SKU_JOIN_PROMOTION: "PROMOTION",
  SKU_LEAVE_PROMOTION: "PROMOTION",
  SKU_ADJUST_PROMOTION_PRICE: "PROMOTION",
  SKU_CHANGE_CATALOG_STATE: "ASSORTMENT",
  CAMPAIGN_ADJUST_BUDGET: "ADVERTISING",
  CAMPAIGN_CHANGE_STATE: "ADVERTISING",
  CAMPAIGN_ADJUST_STRATEGY: "ADVERTISING",
  CAMPAIGN_ADJUST_PRODUCT_SET: "ADVERTISING",
};

const LABEL: Record<CanonicalActionType, string> = {
  ACCOUNT_RATING_REMEDIATION: "Рейтинг кабинета",
  CATEGORY_REPRICING_PLAN: "План цен категории",
  CATEGORY_REPLENISHMENT_PLAN: "Пополнение категории",
  CATEGORY_ADVERTISING_REALLOCATION: "Перераспределение рекламы",
  CATEGORY_PROMOTION_PLAN: "План акций категории",
  CATEGORY_ASSORTMENT_RATIONALIZATION: "Ассортимент категории",
  SKU_ADJUST_PRICE: "Изменить цену",
  SKU_INCREASE_REPLENISHMENT: "Пополнить запас",
  SKU_REDUCE_REPLENISHMENT: "Снизить поставку",
  SKU_ADJUST_AD_BID: "Изменить ставку",
  SKU_CHANGE_AD_PARTICIPATION: "Участие в рекламе",
  SKU_JOIN_PROMOTION: "Добавить в акцию",
  SKU_LEAVE_PROMOTION: "Выйти из акции",
  SKU_ADJUST_PROMOTION_PRICE: "Цена в акции",
  SKU_CHANGE_CATALOG_STATE: "Состояние в каталоге",
  CAMPAIGN_ADJUST_BUDGET: "Бюджет кампании",
  CAMPAIGN_CHANGE_STATE: "Состояние кампании",
  CAMPAIGN_ADJUST_STRATEGY: "Стратегия кампании",
  CAMPAIGN_ADJUST_PRODUCT_SET: "Состав кампании",
};

const ICON: Record<CanonicalActionType, ActionIconRole> = {
  ACCOUNT_RATING_REMEDIATION: "account",
  CATEGORY_REPRICING_PLAN: "price",
  CATEGORY_REPLENISHMENT_PLAN: "stock",
  CATEGORY_ADVERTISING_REALLOCATION: "ads",
  CATEGORY_PROMOTION_PLAN: "promo",
  CATEGORY_ASSORTMENT_RATIONALIZATION: "catalog",
  SKU_ADJUST_PRICE: "price",
  SKU_INCREASE_REPLENISHMENT: "stock",
  SKU_REDUCE_REPLENISHMENT: "stock",
  SKU_ADJUST_AD_BID: "ads",
  SKU_CHANGE_AD_PARTICIPATION: "ads",
  SKU_JOIN_PROMOTION: "promo",
  SKU_LEAVE_PROMOTION: "promo",
  SKU_ADJUST_PROMOTION_PRICE: "promo",
  SKU_CHANGE_CATALOG_STATE: "catalog",
  CAMPAIGN_ADJUST_BUDGET: "campaign",
  CAMPAIGN_CHANGE_STATE: "campaign",
  CAMPAIGN_ADJUST_STRATEGY: "campaign",
  CAMPAIGN_ADJUST_PRODUCT_SET: "campaign",
};

const DETAIL: Record<CanonicalActionType, ParameterFormatterId> = {
  ACCOUNT_RATING_REMEDIATION: "account_rating",
  CATEGORY_REPRICING_PLAN: "category_plan",
  CATEGORY_REPLENISHMENT_PLAN: "category_plan",
  CATEGORY_ADVERTISING_REALLOCATION: "category_plan",
  CATEGORY_PROMOTION_PLAN: "category_plan",
  CATEGORY_ASSORTMENT_RATIONALIZATION: "category_plan",
  SKU_ADJUST_PRICE: "sku_price",
  SKU_INCREASE_REPLENISHMENT: "sku_replenishment",
  SKU_REDUCE_REPLENISHMENT: "sku_replenishment",
  SKU_ADJUST_AD_BID: "campaign_budget",
  SKU_CHANGE_AD_PARTICIPATION: "none",
  SKU_JOIN_PROMOTION: "sku_promotion",
  SKU_LEAVE_PROMOTION: "sku_promotion",
  SKU_ADJUST_PROMOTION_PRICE: "sku_price",
  SKU_CHANGE_CATALOG_STATE: "none",
  CAMPAIGN_ADJUST_BUDGET: "campaign_budget",
  CAMPAIGN_CHANGE_STATE: "none",
  CAMPAIGN_ADJUST_STRATEGY: "none",
  CAMPAIGN_ADJUST_PRODUCT_SET: "campaign_product_set",
};

const COLUMNS: Record<CanonicalActionType, string[]> = {
  ACCOUNT_RATING_REMEDIATION: ["rating"],
  CATEGORY_REPRICING_PLAN: ["category"],
  CATEGORY_REPLENISHMENT_PLAN: ["category"],
  CATEGORY_ADVERTISING_REALLOCATION: ["category"],
  CATEGORY_PROMOTION_PLAN: ["category"],
  CATEGORY_ASSORTMENT_RATIONALIZATION: ["category"],
  SKU_ADJUST_PRICE: ["current_price", "recommended_price", "delta_amount", "delta_percent"],
  SKU_INCREASE_REPLENISHMENT: ["sellable_units", "coverage", "ordered_units_7d", "inbound", "recommended_units"],
  SKU_REDUCE_REPLENISHMENT: ["sellable_units", "coverage", "recommended_units"],
  SKU_ADJUST_AD_BID: ["bid"],
  SKU_CHANGE_AD_PARTICIPATION: ["participation"],
  SKU_JOIN_PROMOTION: ["promotion"],
  SKU_LEAVE_PROMOTION: ["promotion"],
  SKU_ADJUST_PROMOTION_PRICE: ["current_price", "recommended_price"],
  SKU_CHANGE_CATALOG_STATE: ["catalog_state"],
  CAMPAIGN_ADJUST_BUDGET: ["campaign", "current_budget", "recommended_budget", "campaign_drr", "paid_spend"],
  CAMPAIGN_CHANGE_STATE: ["campaign", "state"],
  CAMPAIGN_ADJUST_STRATEGY: ["campaign", "strategy"],
  CAMPAIGN_ADJUST_PRODUCT_SET: ["add_skus", "exclude_skus"],
};

export const ACTION_PRESENTATION_REGISTRY: Record<CanonicalActionType, ActionPresentation> = Object.fromEntries(
  CANONICAL_ACTION_TYPES.map((action_type) => [
    action_type,
    {
      action_type,
      family: FAMILY_BY_TYPE[action_type],
      presentation_template_key: action_type,
      template: action_type,
      label: LABEL[action_type],
      icon: ICON[action_type],
      detail: DETAIL[action_type],
      columns: COLUMNS[action_type],
      parameter_formatter: DETAIL[action_type],
    },
  ]),
) as Record<CanonicalActionType, ActionPresentation>;

export function presentationFor(actionType: string): ActionPresentation | null {
  if ((CANONICAL_ACTION_TYPES as readonly string[]).includes(actionType)) {
    return ACTION_PRESENTATION_REGISTRY[actionType as CanonicalActionType];
  }
  return null;
}

export function presentationOrFallback(actionType: string): ActionPresentation | { fallback: true; action_type: string } {
  const hit = presentationFor(actionType);
  if (hit) return hit;
  if (typeof console !== "undefined") {
    console.warn("unknown_action_type", actionType);
  }
  return { fallback: true, action_type: actionType };
}

export function formatActionParameters(
  actionType: string,
  params: Record<string, unknown> | undefined,
): { label: string; value: string }[] {
  const spec = presentationFor(actionType);
  const p = params ?? {};
  const money = (v: unknown) => (v == null || v === "" ? "—" : String(v));
  if (!spec || spec.parameter_formatter === "none") {
    return [];
  }
  switch (spec.parameter_formatter) {
    case "sku_price":
      return [
        { label: "Текущая цена", value: money(p.current_price ?? p.CurrentPrice) },
        { label: "Рекомендуемая цена", value: money(p.recommended_price ?? p.RecommendedPrice) },
        { label: "Изменение ₽", value: money(p.delta_amount) },
        { label: "Изменение %", value: money(p.delta_percent) },
      ];
    case "sku_replenishment": {
      const qty = p.recommended_units ?? p.RecommendedUnits;
      return [
        { label: "Остаток", value: money(p.sellable_units) },
        { label: "Покрытие", value: money(p.coverage_days) },
        { label: "Заказано товаров за 7 дней", value: money(p.ordered_units_7d) },
        { label: "Inbound", value: money(p.inbound) },
        { label: "Пополнение", value: qty == null || qty === "" ? "Пополнить запас" : String(qty) },
      ];
    }
    case "sku_promotion":
      return [{ label: "Акция", value: money(p.promotion_ref ?? p.promotion) }];
    case "campaign_budget":
      return [
        { label: "Кампания", value: money(p.campaign_id ?? p.campaign) },
        { label: "Текущий бюджет", value: money(p.current_budget ?? p.budget) },
        { label: "Рекомендуемый бюджет", value: money(p.recommended_budget) },
        { label: "DRR кампании", value: money(p.campaign_drr) },
        { label: "Paid spend", value: money(p.paid_spend) },
      ];
    case "campaign_product_set": {
      const add = Array.isArray(p.add_skus) ? (p.add_skus as unknown[]).map(String).join(", ") : "";
      const drop = Array.isArray(p.exclude_skus) ? (p.exclude_skus as unknown[]).map(String).join(", ") : "";
      return [
        { label: "Добавить", value: add || "—" },
        { label: "Исключить", value: drop || "—" },
      ];
    }
    case "category_plan":
      return [{ label: "Категория", value: money(p.category_id ?? p.category) }];
    case "account_rating":
      return [{ label: "Рейтинг", value: money(p.rating) }];
    default:
      return [];
  }
}
