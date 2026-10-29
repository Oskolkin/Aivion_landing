import type {
  ActionCategory,
  ActionCompletionCondition,
  ActionEvidence,
  ActionItem,
  ActionPriority,
  ActionScopeType,
  ActionStatus,
  ActionType,
  ActionsFilterKey,
  OperationalObjectAction,
  OperationalDetail,
} from "@/lib/actions-types";

const METRIC_LABEL_MAP: Record<string, string> = {
  stock_available: "Остаток",
  "stock_available/current_day": "Остаток",
  "stock_available (current_day)": "Остаток",
  orders: "Заказы",
  "orders/last_7d": "Заказы за 7 дней",
  "orders (last_7d)": "Заказы за 7 дней",
  revenue: "Выручка",
  "revenue/last_7d": "Выручка за 7 дней",
  "revenue (last_7d)": "Выручка за 7 дней",
  spend_7d: "Расход 7д",
  revenue_7d: "Выручка 7д",
  orders_7d: "Заказы 7д",
  stock_units: "Остаток",
  drr_pct: "ДРР",
  coverage_days: "Покрытие",
  idc_days: "Покрытие запасом",
  "idc_days/current_day": "Покрытие запасом",
  days_without_sales: "Дней без продаж",
  spend: "Расход",
  "spend/last_7d": "Расход за 7 дней",
  roas: "ROAS",
  count: "шт",
};

const RAW_LABEL_PATTERN =
  /\b(slow_turnover|stock_available|current_day|last_7d|orders|revenue|count|RUB)\b/gi;

export function actionPriorityLabel(priority: ActionPriority): string {
  const map: Record<ActionPriority, string> = {
    critical: "Критично",
    important: "Важно",
    watch: "Наблюдение",
  };
  return map[priority] ?? priority;
}

// User-facing statuses: open -> Открыто, checked -> Выполнено, dismissed -> Неактуально.
// Legacy internal statuses (snoozed) are not shown in UI and fall back to "Открыто".
export function actionStatusLabel(status: ActionStatus): string {
  switch (status) {
    case "checked":
      return "Выполнено";
    case "dismissed":
      return "Неактуально";
    default:
      return "Открыто";
  }
}

// Urgency markers are in-action badges only (never page filters).
export function actionUrgencyLabel(marker?: string): string | null {
  switch (marker) {
    case "urgent":
      return "Срочно";
    case "watch":
      return "Наблюдать";
    default:
      return null;
  }
}

export function formatCompletionOperator(operator: string): string {
  switch (operator.toLowerCase()) {
    case "eq":
      return "=";
    case "ne":
      return "≠";
    case "lt":
      return "<";
    case "lte":
      return "≤";
    case "gt":
      return ">";
    case "gte":
      return "≥";
    case "between":
      return "между";
    case "decision_recorded":
      return "решение зафиксировано";
    default:
      return operator;
  }
}

export function formatCompletionConditionLine(condition: ActionCompletionCondition): string {
  if (condition.display_text?.trim()) return condition.display_text.trim();
  const metricLabel = formatMetricLabel(condition.metric_code);
  const op = formatCompletionOperator(condition.operator);
  if (condition.operator === "between" && condition.target_value != null && condition.target_value_max != null) {
    return `${metricLabel} ${op} ${condition.target_value}–${condition.target_value_max} (${condition.scope})`;
  }
  if (condition.operator === "decision_recorded") {
    return `${metricLabel || "Критерий"}: ${op} (${condition.scope})`;
  }
  if (condition.target_value != null) {
    const period = condition.period_days ? `, ${condition.period_days} дн.` : "";
    return `${metricLabel} ${op} ${condition.target_value} (${condition.scope}${period})`;
  }
  return metricLabel || condition.metric_code;
}

export function formatActionCategoryLabel(category: ActionCategory | string): string {
  return actionCategoryLabel(category as ActionCategory);
}

export function actionCategoryLabel(category: ActionCategory): string {
  return getActionCategoryLabel(category);
}

export function getActionCategoryLabel(category: ActionCategory | string): string {
  const map: Record<ActionCategory, string> = {
    stock: "Остатки и поставки",
    slow_turnover: "Оборачиваемость",
    sku: "SKU",
    sales: "Продажи",
    advertising: "Реклама",
    funnel: "Воронка и карточки",
    category: "Категории",
    data_quality: "Качество данных",
  };
  return map[category as ActionCategory] ?? String(category);
}

export type ActionTone = { bg: string; text: string; ring: string };

export function getActionCategoryTone(category: ActionCategory | string): ActionTone {
  switch (category) {
    case "stock":
      return { bg: "bg-amber-50", text: "text-amber-800", ring: "ring-amber-200" };
    case "slow_turnover":
      return { bg: "bg-sky-50", text: "text-sky-800", ring: "ring-sky-200" };
    case "advertising":
      return { bg: "bg-violet-50", text: "text-violet-800", ring: "ring-violet-200" };
    case "funnel":
      return { bg: "bg-blue-50", text: "text-blue-800", ring: "ring-blue-200" };
    case "sales":
    case "sku":
      return { bg: "bg-emerald-50", text: "text-emerald-800", ring: "ring-emerald-200" };
    case "category":
      return { bg: "bg-indigo-50", text: "text-indigo-800", ring: "ring-indigo-200" };
    default:
      return { bg: "bg-slate-50", text: "text-slate-700", ring: "ring-slate-200" };
  }
}

export function getActionStatusLabel(status: ActionStatus): string {
  return actionStatusLabel(status);
}

export function getActionStatusTone(status: ActionStatus): ActionTone {
  switch (status) {
    case "checked":
      return { bg: "bg-emerald-50", text: "text-emerald-800", ring: "ring-emerald-200" };
    case "dismissed":
      return { bg: "bg-slate-100", text: "text-slate-600", ring: "ring-slate-200" };
    default:
      return { bg: "bg-blue-50", text: "text-blue-800", ring: "ring-blue-200" };
  }
}

export function formatActionTypeLabel(type: ActionType | string): string {
  return actionTypeLabel(type as ActionType);
}

export function actionTypeLabel(type: ActionType): string {
  const map: Record<ActionType, string> = {
    stock_replenishment: "Пополнение",
    stock_redistribution: "Перераспределение остатков",
    slow_turnover_review: "Проверка оборачиваемости",
    no_sales_with_stock: "Остаток без продаж",
    sku_orders_drop: "Падение заказов",
    key_sku_risk: "Ключевые SKU",
    key_sku_protection: "Защита ключевых SKU",
    ad_high_drr: "Высокий ДРР",
    ad_high_drr_review: "Высокий ДРР",
    ad_spend_no_orders: "Расход без заказов",
    ad_budget_reallocation: "Перераспределение бюджета",
    funnel_weak_transition: "Просадка воронки",
    card_conversion_review: "Конверсия карточки",
    category_underperformance: "Просадка категории",
    sales_drop_review: "Просадка продаж",
    data_quality_issue: "Качество данных",
  };
  return map[type] ?? type;
}

export function actionsFilterLabel(filter: ActionsFilterKey): string {
  const map: Record<ActionsFilterKey, string> = {
    open: "Все открытые",
    stock: "Остатки и поставки",
    slow_turnover: "Оборачиваемость",
    sku_sales: "SKU и продажи",
    advertising: "Реклама",
    funnel: "Воронка и карточки",
    category: "Категории",
    checked: "Выполненные",
    dismissed: "Неактуальные",
  };
  return map[filter];
}

export function categoryFilterLabel(filter: ActionsFilterKey): string {
  if (filter === "open") return "Все категории";
  return actionsFilterLabel(filter);
}

export function formatMetricDate(metricDate?: string): string {
  if (!metricDate) return "";
  const [y, m, d] = metricDate.split("-");
  if (!y || !m || !d) return metricDate;
  return `${d}.${m}.${y}`;
}

export function formatCurrency(value: number): string {
  return new Intl.NumberFormat("ru-RU", { style: "currency", currency: "RUB", maximumFractionDigits: 0 }).format(value);
}

export const formatCurrencyRub = formatCurrency;

export function formatDate(metricDate?: string): string {
  return formatMetricDate(metricDate);
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 2 }).format(value);
}

function parseNumeric(value: string): number | null {
  const normalized = value.replace(",", ".");
  const num = Number(normalized);
  return Number.isFinite(num) ? num : null;
}

export function formatMetricLabel(label: string): string {
  const trimmed = label.trim();
  if (!trimmed) return "";
  const lower = trimmed.toLowerCase();
  if (METRIC_LABEL_MAP[lower]) return METRIC_LABEL_MAP[lower];
  if (METRIC_LABEL_MAP[trimmed]) return METRIC_LABEL_MAP[trimmed];
  if (lower.includes("stock_available")) return "Остаток";
  if (lower.includes("idc_days") || lower.includes("coverage_days")) return "Покрытие запасом";
  if (lower.includes("days_without_sales")) return "Дней без продаж";
  if (lower.includes("orders") && lower.includes("7")) return "Заказы за 7 дней";
  if (lower.includes("revenue") && lower.includes("7")) return "Выручка за 7 дней";
  if (lower === "count") return "шт";
  if (RAW_LABEL_PATTERN.test(trimmed)) {
    return trimmed
      .replace(/\bstock_available\b/gi, "Остаток")
      .replace(/\bcurrent_day\b/gi, "")
      .replace(/\blast_7d\b/gi, "7 дней")
      .replace(/\borders\b/gi, "Заказы")
      .replace(/\brevenue\b/gi, "Выручка")
      .replace(/\bcount\b/gi, "шт")
      .replace(/\bRUB\b/gi, "₽")
      .replace(/\s*\/\s*/g, " ")
      .replace(/\s*\(\s*/g, " ")
      .replace(/\s*\)\s*/g, "")
      .replace(/\s{2,}/g, " ")
      .trim();
  }
  return trimmed;
}

function stripEmbeddedUnitFromValue(value: string): string {
  return value
    .replace(/\s*(RUB|₽|rub|rubles)\s*₽?/gi, "")
    .replace(/\s*(count|pcs|units|piece|шт\.?)\s*$/gi, "")
    .trim();
}

const UNIT_ALIASES: Record<string, string> = {
  count: "count",
  units: "count",
  unit: "count",
  pcs: "count",
  piece: "count",
  шт: "count",
  rub: "rub",
  rubles: "rub",
  "₽": "rub",
  pct: "pct",
  percent: "pct",
  "%": "pct",
  pp: "pp",
  "п.п.": "pp",
};

function normalizeRawUnit(unit?: string): string {
  if (!unit) return "";
  const lower = unit.trim().toLowerCase();
  return UNIT_ALIASES[lower] ?? lower;
}

function resolveMetricUnit(metricKey: string | undefined, rawUnit: string | undefined, value: string): string {
  const lower = (metricKey ?? "").toLowerCase();
  if (lower.includes("заказ") || lower.includes("order")) return "orders";
  if (lower.includes("остаток") || lower.includes("stock")) return "count";
  if (lower.includes("выручк") || lower.includes("revenue") || lower.includes("расход")) return "rub";
  const normalized = normalizeRawUnit(rawUnit);
  if (normalized) return normalized;
  if (value.includes("₽")) return "rub";
  if (value.includes("%")) return "pct";
  return "";
}

export type ActionMetricFormatContext = "default" | "orders_metric";

export function formatActionMetricValue({
  metricKey,
  value,
  rawUnit,
  context = "default",
}: {
  metricKey?: string;
  value: string;
  rawUnit?: string;
  context?: ActionMetricFormatContext;
}): string {
  const cleaned = stripEmbeddedUnitFromValue(value);
  if (!cleaned) return "—";
  if (cleaned.includes("₽") && !rawUnit) {
    return cleaned.replace(/\s*RUB\s*/gi, " ").replace(/\s+/g, " ").trim();
  }
  if (cleaned.includes("%")) return cleaned;

  const unit = resolveMetricUnit(metricKey, rawUnit, value);
  const parsed = parseNumeric(cleaned);

  if (parsed == null) {
    if (unit === "count" && context !== "orders_metric") return `${cleaned} шт.`;
    if (unit === "rub") return cleaned.includes("₽") ? cleaned : `${cleaned} ₽`;
    return cleaned;
  }

  if (unit === "rub") return formatCurrency(parsed);
  if (context === "orders_metric" || unit === "orders") return formatNumber(parsed);
  if (unit === "count") return `${formatNumber(parsed)} шт.`;
  if (unit === "pct") return `${formatNumber(parsed)}%`;
  if (unit === "pp") return `${formatNumber(parsed)} п.п.`;
  if (unit === "days" || unit === "дни" || unit === "дн.") return `${formatNumber(parsed)} дн.`;
  if (rawUnit) return `${formatNumber(parsed)} ${formatMetricLabel(rawUnit)}`;
  return formatNumber(parsed);
}

export function formatMetricValue(value: string, unit?: string): string {
  return formatActionMetricValue({ value, rawUnit: unit });
}

export function formatEvidenceValue(evidence: ActionEvidence): string {
  return formatMetricValue(evidence.value, evidence.unit);
}

export function formatStockRange(min: number, max: number): string {
  if (min === max) return `${formatNumber(min)} шт`;
  return `${formatNumber(min)}–${formatNumber(max)} шт`;
}

export type ObjectMetricColumns = {
  stock: string;
  orders: string;
  revenue: string;
};

function metricLabelKind(label: string): "stock" | "orders" | "revenue" | null {
  const lower = label.toLowerCase();
  if (lower.includes("остаток") || lower.includes("stock")) return "stock";
  if (lower.includes("заказ") || lower.includes("order")) return "orders";
  if (lower.includes("выручк") || lower.includes("revenue")) return "revenue";
  return null;
}

export function formatObjectMetric(label: string, value: string, rawUnit?: string): string {
  const kind = metricLabelKind(formatMetricLabel(label));
  return formatActionMetricValue({
    metricKey: label,
    value,
    rawUnit,
    context: kind === "orders" ? "orders_metric" : "default",
  });
}

export const formatActionMetric = formatObjectMetric;

export function formatActionDate(metricDate?: string): string {
  return formatMetricDate(metricDate);
}

export function parseObjectMetricColumns(
  metrics?: Array<{ label: string; value: string }>,
): ObjectMetricColumns {
  const columns: ObjectMetricColumns = { stock: "—", orders: "—", revenue: "—" };
  if (!metrics?.length) return columns;
  for (const metric of metrics) {
    const label = formatMetricLabel(metric.label);
    const kind = metricLabelKind(label);
    const formatted = formatObjectMetric(label, metric.value);
    if (kind === "stock") columns.stock = formatted;
    else if (kind === "orders") columns.orders = formatted;
    else if (kind === "revenue") columns.revenue = formatted;
  }
  return columns;
}

export type MetricChip = { label: string; value: string };

export function actionCardMetricChips(action: ActionItem): MetricChip[] {
  const chips: MetricChip[] = [];
  for (const item of action.evidence.slice(0, 3)) {
    chips.push({
      label: formatMetricLabel(item.label),
      value: formatEvidenceValue(item),
    });
  }
  return chips.slice(0, 2);
}

export function actionCardFooterParts(action: ActionItem): string[] {
  const parts: string[] = [];
  const statusDate = actionStatusDateLabel(action);
  if (statusDate) parts.push(statusDate);
  return parts;
}

export function actionStatusDateLabel(action: ActionItem): string {
  const status = actionStatusLabel(action.status);
  const date = action.checked_at
    ? formatMetricDate(action.checked_at.split("T")[0])
    : action.metric_date
      ? formatMetricDate(action.metric_date)
      : "";
  return date ? `${status} ${date}` : status;
}

export function getActionObjectCountLabel(action: ActionItem): string | null {
  return actionObjectsSummary(action);
}

export function actionCardFooter(action: ActionItem): string {
  return actionCardFooterParts(action).join(" · ");
}

export function focusCategoryLabel(focus?: { top_categories?: string[]; priority_order?: string[] }): string | null {
  const raw = focus?.top_categories?.[0] ?? focus?.priority_order?.[0];
  if (!raw) return null;
  return formatActionCategoryLabel(raw);
}

export function cardPriorityBadge(action: ActionItem): string | null {
  const fromMarker = actionUrgencyLabel(action.urgency_marker);
  if (fromMarker) return fromMarker;
  if (action.priority === "critical" || action.priority === "important") return "Важно";
  if (action.priority === "watch") return "Наблюдать";
  return null;
}

export function actionCardDescription(action: ActionItem): string {
  return action.summary || action.reason || "Подробности доступны в карточке действия.";
}

// "Объекты: 4 SKU · 2 кампании" summary for the list card.
export function countAffectedEntityObjects(action: ActionItem): number {
  const entities = action.affected_entities;
  if (!entities) return 0;
  return (
    (entities.skus?.length ?? 0) +
    (entities.campaigns?.length ?? 0) +
    (entities.funnel ? 1 : 0)
  );
}

export function resolveActionScopeType(action: ActionItem): ActionScopeType | "unknown" {
  const explicit = action.scope_type;
  if (explicit === "account" || explicit === "entity" || explicit === "mixed") {
    return explicit;
  }
  if (countAffectedEntityObjects(action) > 0 || (action.affected_objects_total ?? 0) > 0) {
    return "entity";
  }
  return "account";
}

export function formatActionScopeLevelLabel(scope: ActionScopeType | "unknown"): string {
  switch (scope) {
    case "account":
      return "Весь кабинет";
    case "mixed":
      return "Весь кабинет";
    case "entity":
      return "";
    default:
      return "Весь кабинет";
  }
}

export function resolveAffectedObjectCounts(action: ActionItem): {
  total: number;
  displayed: number;
  truncated: boolean;
  entityType: string;
  scopeType: ActionScopeType | "unknown";
} {
  const scopeType = resolveActionScopeType(action);
  if (scopeType === "account") {
    return { total: 0, displayed: 0, truncated: false, entityType: "sku", scopeType };
  }
  const objectActions = action.operational_detail?.object_actions ?? [];
  const entityType = dominantEntityType(objectActions);
  const fallbackTotal = countAffectedEntityObjects(action);
  // Authoritative count = validated entity list length. Prefer the longer of
  // affected_entities vs object_actions so a partial object_actions projection
  // cannot shrink a complete entity list, and a stale affected_objects_total
  // cannot inflate either list.
  const listTotal = Math.max(objectActions.length, fallbackTotal);
  const total =
    listTotal > 0
      ? listTotal
      : action.affected_objects_total && action.affected_objects_total > 0
        ? action.affected_objects_total
        : 0;
  const displayed =
    objectActions.length > 0
      ? objectActions.length
      : listTotal > 0
        ? listTotal
        : action.affected_objects_displayed != null && action.affected_objects_displayed >= 0
          ? action.affected_objects_displayed
          : 0;
  const truncated =
    action.affected_objects_truncated === true ||
    (total > 0 && displayed > 0 && displayed < total);
  return { total, displayed, truncated, entityType, scopeType };
}

export function formatMissingObjectActionsMessage(): string {
  return "Для всех позиций применяется общий план действий.";
}

export function formatAffectedObjectsHeader(total: number, entityType: string): string {
  if (total <= 0) return "";
  switch (entityType) {
    case "sku":
      return `${total} SKU`;
    case "campaign":
      return pluralizeRu(total, "рекламная кампания", "рекламные кампании", "рекламных кампаний");
    default:
      return formatObjectCountLabel(total, entityType);
  }
}

export function formatAffectedObjectsDetailCaption(
  total: number,
  displayed: number,
  truncated: boolean,
  entityType = "sku",
  coverageMode?: string,
): string | null {
  if (total <= 0) return null;
  if (displayed === 0 && total > 0) {
    return `Затронуто: ${formatAffectedObjectsHeader(total, entityType)}. Индивидуальные рекомендации временно не сформированы. Общий план относится ко всем затронутым объектам.`;
  }
  if (!truncated && coverageMode !== "partial" && displayed >= total) {
    return `Затронуто: ${formatAffectedObjectsHeader(total, entityType)}. Подробные рекомендации сформированы для всех ${total}.`;
  }
  if (coverageMode === "partial" || truncated) {
    return `Затронуто: ${formatAffectedObjectsHeader(total, entityType)}. Подробно показано: ${displayed} из ${total}. Для ${total - displayed} объектов индивидуальные рекомендации не сформированы. Общий план относится ко всем ${total} SKU.`;
  }
  return `Показано ${displayed} из ${total}`;
}

export function actionObjectsSummary(action: ActionItem): string | null {
  const { total, entityType } = resolveAffectedObjectCounts(action);
  if (total <= 0) {
    const entities = action.affected_entities;
    if (!entities) return null;
    const parts: string[] = [];
    const skuCount = entities.skus?.length ?? 0;
    const campaignCount = entities.campaigns?.length ?? 0;
    if (skuCount > 0) parts.push(`${skuCount} SKU`);
    if (campaignCount > 0) parts.push(pluralizeRu(campaignCount, "кампания", "кампании", "кампаний"));
    if (entities.funnel) parts.push("переход воронки");
    return parts.length ? parts.join(" · ") : null;
  }
  return formatAffectedObjectsHeader(total, entityType);
}

export function entityTypeLabel(entityType: string): string {
  switch (entityType) {
    case "sku":
      return "SKU";
    case "campaign":
      return "Кампания";
    case "category":
      return "Категория";
    case "funnel":
      return "Воронка";
    default:
      return entityType;
  }
}

function pluralizeRu(count: number, one: string, few: string, many: string): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} ${one}`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} ${few}`;
  return `${count} ${many}`;
}

export function encodeActionKey(value: string): string {
  return encodeURIComponent(value);
}

export function removeRepeatedLeadingTitle(reason: string, title: string): string {
  if (!reason.trim() || !title.trim()) return reason;
  const normalize = (value: string) =>
    value.trim().replace(/\s+/g, " ").replace(/[.!?…]+$/u, "").toLowerCase();
  const normalizedReason = normalize(reason);
  const normalizedTitle = normalize(title);
  if (!normalizedReason.startsWith(normalizedTitle)) return reason;
  const trimmed = reason.slice(title.length).replace(/^[\s.:;—–-]+/u, "").trim();
  return trimmed || reason;
}

export type ActionAffectedObjectDisplay = {
  key: string;
  primaryLabel: string;
  secondaryLabel?: string;
  entityType: string;
  metrics: Array<{ key: string; label: string; formattedValue: string }>;
  recommendedAction?: string;
  compactRecommendation?: string;
  reason?: string;
};

export function getCompactObjectRecommendation(fullText?: string): string | undefined {
  if (!fullText?.trim()) return undefined;
  const trimmed = fullText.trim();
  const sentences = trimmed.match(/[^.!?…]+[.!?…]+(?:\s|$)|[^.!?…]+$/g) ?? [trimmed];
  if (sentences.length <= 2) return trimmed;
  return sentences.slice(0, 2).join(" ").trim();
}

export function normalizeAffectedObjects(
  objects: OperationalObjectAction[],
): ActionAffectedObjectDisplay[] {
  return objects.map((obj) => {
    const recommendedAction = obj.recommended_action || undefined;
    return {
      key: `${obj.entity_type}-${obj.entity_id}`,
      primaryLabel: obj.name || normalizeEntityId(obj.entity_id),
      secondaryLabel: formatEntitySecondaryLabel(obj),
      entityType: obj.entity_type,
      metrics: (obj.metrics ?? []).map((metric) => {
        const label = formatMetricLabel(metric.display_label || metric.label);
        const rawValue = metric.formatted_value || metric.value;
        return {
          key: `${obj.entity_type}-${metric.metric_code || metric.label}`,
          label,
          formattedValue: formatObjectMetric(label, rawValue),
        };
      }),
      recommendedAction,
      compactRecommendation: getCompactObjectRecommendation(recommendedAction),
      reason: obj.reason || undefined,
    };
  });
}

function formatObjectCountLabel(count: number, entityType: string): string {
  switch (entityType) {
    case "sku":
      return `${count} SKU`;
    case "campaign":
      return pluralizeRu(count, "кампания", "кампании", "кампаний");
    case "category":
      return pluralizeRu(count, "категория", "категории", "категорий");
    default:
      return pluralizeRu(count, "объект", "объекта", "объектов");
  }
}

function collectKnownEntityTokens(objects: OperationalObjectAction[]): Set<string> {
  const tokens = new Set<string>();
  for (const obj of objects) {
    tokens.add(normalizeEntityId(obj.entity_id).toLowerCase());
    tokens.add(obj.entity_id.toLowerCase());
  }
  return tokens;
}

export function collapseLeadingEntityIdList(
  text: string,
  objects: OperationalObjectAction[],
): string {
  if (!text.trim() || objects.length < 2) return text;

  const listMatch = text.match(
    /^((?:SKU\s*)?(?:[A-Za-z0-9:_-]+)(?:\s*,\s*(?:SKU\s*)?[A-Za-z0-9:_-]+)+)\s*[,;.\-—–]?\s*/i,
  );
  if (!listMatch) return text;

  const known = collectKnownEntityTokens(objects);
  const idsInSegment = listMatch[1]
    .split(/\s*,\s*/)
    .map((part) => normalizeEntityId(part.replace(/^SKU\s*/i, "").trim()).toLowerCase())
    .filter(Boolean);

  if (idsInSegment.length < 2) return text;

  const knownCount = idsInSegment.filter((id) => known.has(id)).length;
  if (knownCount < Math.min(2, Math.ceil(idsInSegment.length * 0.6))) return text;

  const countLabel = formatObjectCountLabel(
    Math.max(objects.length, idsInSegment.length),
    dominantEntityType(objects),
  );
  const remainder = text.slice(listMatch[0].length).replace(/^[,;.\-—–\s]+/u, "");
  return remainder ? `${countLabel} ${remainder}` : countLabel;
}

export function normalizeTextForComparison(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s]/gu, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export function areActionTextsNearDuplicate(a: string, b: string): boolean {
  const left = normalizeTextForComparison(a);
  const right = normalizeTextForComparison(b);
  if (!left || !right) return false;
  if (left === right) return true;
  if (left.length > 24 && right.length > 24) {
    if (left.includes(right) || right.includes(left)) return true;
  }
  return false;
}

export function normalizeActionReasonForDisplay({
  reason,
  title,
  summary,
  affectedObjects = [],
}: {
  reason: string;
  title: string;
  summary?: string;
  affectedObjects?: OperationalObjectAction[];
}): string | null {
  let text = removeRepeatedLeadingTitle(reason, title);
  if (!text.trim()) return null;
  text = collapseLeadingEntityIdList(text, affectedObjects);
  if (summary && areActionTextsNearDuplicate(summary, text)) return null;
  return text.trim() || null;
}

export function getActionFocusDisplayTitle(action: Pick<ActionItem, "title" | "summary">): string {
  const summary = action.summary?.trim();
  const title = action.title?.trim() ?? "";
  if (summary && summary.length > 0 && summary.length <= Math.max(48, title.length * 0.75)) {
    return summary;
  }
  return title;
}

export function getActionFocusHeadline(
  focusHeadline: string | undefined,
  action?: Pick<ActionItem, "title" | "summary">,
): string {
  const raw = focusHeadline?.trim() || (action ? getActionFocusDisplayTitle(action) : "");
  if (!raw) return "Критических действий на сегодня нет.";
  return raw.replace(/^Главное действие:\s*/i, "").replace(/^Фокус:\s*/i, "");
}

export function normalizeEntityId(entityId: string): string {
  return entityId.replace(/^(sku|campaign|category):\s*/i, "").trim();
}

export function formatActionObjectIdentifier(entityType: string, entityId: string): string {
  const normalized = normalizeEntityId(entityId);
  switch (entityType) {
    case "sku":
      return `SKU ${normalized}`;
    case "campaign":
      return `ID ${normalized}`;
    case "category":
      return `Категория ${normalized}`;
    case "funnel":
    case "funnel_transition":
      return normalized;
    default:
      return `${entityTypeLabel(entityType)} ${normalized}`;
  }
}

export function formatEntitySecondaryLabel(obj: OperationalObjectAction): string | undefined {
  if (obj.entity_type === "account") return undefined;
  return formatActionObjectIdentifier(obj.entity_type, obj.entity_id);
}

export type ActionCommand = "ask_ai" | "complete" | "irrelevant" | "return_to_work" | "copy_link";

export function getAvailableActionCommands(action: ActionItem): ActionCommand[] {
  const commands: ActionCommand[] = ["ask_ai", "copy_link"];
  switch (action.status) {
    case "checked":
    case "dismissed":
      commands.push("return_to_work");
      break;
    default:
      commands.push("complete", "irrelevant");
      break;
  }
  return commands;
}

export type ObjectTableColumn = { id: string; label: string };

export function getObjectTableColumns(
  entityType: string,
  sampleMetrics?: Array<{ label: string }>,
): ObjectTableColumn[] {
  if (entityType === "sku") {
    return [
      { id: "object", label: "Товар / SKU" },
      { id: "stock", label: "Остаток" },
      { id: "orders", label: "Заказы 7д" },
      { id: "revenue", label: "Выручка 7д" },
      { id: "action", label: "Что сделать" },
    ];
  }
  if (entityType === "campaign") {
    return [
      { id: "object", label: "Кампания" },
      { id: "metrics", label: "Показатели" },
      { id: "action", label: "Что сделать" },
    ];
  }
  const dynamicMetrics = (sampleMetrics ?? []).map((metric) => ({
    id: `metric-${formatMetricLabel(metric.label)}`,
    label: formatMetricLabel(metric.label),
  }));
  return [
    { id: "object", label: "Объект" },
    ...dynamicMetrics,
    { id: "action", label: "Что сделать" },
  ];
}

export function dominantEntityType(objects: OperationalObjectAction[]): string {
  if (!objects.length) return "other";
  const counts = new Map<string, number>();
  for (const obj of objects) {
    counts.set(obj.entity_type, (counts.get(obj.entity_type) ?? 0) + 1);
  }
  let best = objects[0].entity_type;
  let bestCount = 0;
  for (const [type, count] of counts) {
    if (count > bestCount) {
      best = type;
      bestCount = count;
    }
  }
  return best;
}

export function buildDisplaySteps(
  action: ActionItem,
  detail?: OperationalDetail,
): Array<{ title: string; instruction: string; expectedResult?: string }> {
  const steps = detail?.action_plan?.steps ?? [];
  if (steps.length > 0) {
    return steps.map((step, index) => ({
      title: step.title || `Шаг ${index + 1}`,
      instruction: step.instruction,
      expectedResult: step.expected_result || undefined,
    }));
  }
  if (action.next_step) {
    return [{ title: "Следующий шаг", instruction: action.next_step }];
  }
  return [];
}

export function focusHeadlineText(headline?: string, fallbackTitle?: string): string {
  return getActionFocusHeadline(headline, fallbackTitle ? { title: fallbackTitle, summary: undefined } : undefined);
}
