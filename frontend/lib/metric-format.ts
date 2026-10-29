export type MetricKind = "MONEY" | "COUNT" | "PERCENT" | "PERCENTAGE_POINTS" | "DAYS" | "RATIO";

const UNAVAILABLE = "—";

export function formatMetric(kind: MetricKind, amount?: string | null, unit?: string): string {
  if (amount == null || amount === "") return UNAVAILABLE;
  const n = Number(amount);
  if (!Number.isFinite(n)) return UNAVAILABLE;
  switch (kind) {
    case "MONEY":
      return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(n) + (unit ? ` ${unit}` : " ₽");
    case "COUNT":
      return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(n);
    case "PERCENT":
    case "RATIO":
      return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(n * (kind === "RATIO" && n <= 1 ? 100 : 1))} %`;
    case "PERCENTAGE_POINTS":
      return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(n)} п.п.`;
    case "DAYS":
      return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(n)} дн.`;
    default:
      return amount;
  }
}

export function formatDelta(kind: MetricKind, delta?: string | null): string {
  if (delta == null || delta === "") return "";
  const n = Number(delta);
  if (!Number.isFinite(n)) return "";
  const sign = n > 0 ? "+" : "";
  return sign + formatMetric(kind, delta);
}

export function kpiKind(key: string, unit?: string): MetricKind {
  if (key === "ORDERED_UNITS") return "COUNT";
  if (key === "AD_SHARE" || key.includes("ad_spend_share") || unit === "RATIO") return "RATIO";
  if (unit === "UNITS") return "COUNT";
  if (unit === "DAYS") return "DAYS";
  return "MONEY";
}
