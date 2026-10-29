const KEY_LABELS: Record<string, string> = {
  metric: "Метрика",
  current_value: "Текущее значение",
  previous_value: "Предыдущее значение",
  delta: "Изменение",
  delta_pct: "Изменение, %",
  threshold: "Порог",
  sku: "SKU",
  offer_id: "Артикул",
  campaign_id: "Кампания",
  stock_available: "Остаток",
  days_of_cover: "Покрытие, дн.",
  revenue: "Выручка",
  orders_count: "Заказы",
  current_orders: "Текущие заказы",
  comparison_orders: "Заказы за сравниваемый период",
  orders_delta: "Изменение заказов",
  orders_delta_percent: "Изменение заказов, %",
  sku_orders_delta_percent: "Изменение заказов по SKU, %",
  metric_date: "Дата метрики",
  comparison_date: "Дата сравнения",
  comparison_period: "Период сравнения",
  minimum_previous_orders: "Мин. заказов в базовом периоде",
  reason: "Причина",
  ad_spend: "Расход на рекламу",
  roas: "ROAS",
  price: "Цена",
  min_price: "Мин. цена",
  max_price: "Макс. цена",
};

function humanizeKey(key: string): string {
  return KEY_LABELS[key] ?? key.replaceAll("_", " ");
}

function formatValue(value: unknown): string {
  if (value == null) return "—";
  if (typeof value === "number") {
    return Number.isInteger(value)
      ? new Intl.NumberFormat("ru-RU").format(value)
      : new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 2 }).format(value);
  }
  if (typeof value === "boolean") return value ? "Да" : "Нет";
  if (typeof value === "string") return value.trim() === "" ? "—" : value;
  if (Array.isArray(value)) {
    return value.map(formatValue).join(", ");
  }
  if (typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) return "—";
    if (entries.length <= 4) {
      return entries.map(([k, v]) => `${humanizeKey(k)}: ${formatValue(v)}`).join("; ");
    }
    return `${entries.length} параметров`;
  }
  return String(value);
}

export type EvidenceEntry = { label: string; value: string };

/** Human-readable key/value rows from backend evidence/metrics payloads. */
export function evidenceEntries(payload: unknown): EvidenceEntry[] {
  if (payload == null || typeof payload !== "object" || Array.isArray(payload)) {
    return [];
  }
  return Object.entries(payload as Record<string, unknown>)
    .filter(([, v]) => v != null && v !== "")
    .map(([key, value]) => ({
      label: humanizeKey(key),
      value: formatValue(value),
    }));
}
