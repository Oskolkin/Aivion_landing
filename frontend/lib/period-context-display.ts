/**
 * Formats structured backend `period_context` into seller-facing period groups.
 * Dates always come from ISO fields — never from AI narrative text or caption strings.
 */

export type PeriodContextFields = {
  value_period?: string | null;
  comparison_period?: string | null;
  trend_period?: string | null;
  current_range?: string | null;
  previous_range?: string | null;
  caption?: string | null;
  caption_primary?: string | null;
  caption_secondary?: string | null;
} | null | undefined;

const MONTHS_GENITIVE = [
  "",
  "января",
  "февраля",
  "марта",
  "апреля",
  "мая",
  "июня",
  "июля",
  "августа",
  "сентября",
  "октября",
  "ноября",
  "декабря",
];

function parseISODate(iso: string | null | undefined): Date | null {
  const text = (iso ?? "").trim();
  if (!/^\d{4}-\d{2}-\d{2}$/.test(text)) return null;
  const [y, m, d] = text.split("-").map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d));
  if (Number.isNaN(dt.getTime())) return null;
  return dt;
}

function formatDayMonth(dt: Date, includeMonth: boolean): string {
  const day = dt.getUTCDate();
  if (!includeMonth) return String(day);
  return `${day} ${MONTHS_GENITIVE[dt.getUTCMonth() + 1]}`;
}

/** Compact range: "2–8 июля" or "25 июня–1 июля". */
export function formatRUDateRange(startISO: string, endISO: string): string | null {
  const start = parseISODate(startISO);
  const end = parseISODate(endISO);
  if (!start || !end) return null;
  const sameMonth =
    start.getUTCFullYear() === end.getUTCFullYear() && start.getUTCMonth() === end.getUTCMonth();
  if (sameMonth) {
    return `${formatDayMonth(start, false)}–${formatDayMonth(end, true)}`;
  }
  return `${formatDayMonth(start, true)}–${formatDayMonth(end, true)}`;
}

/** Range with year: "2–8 июля 2026" or "25 июня–1 июля 2026". */
export function formatRUDateRangeWithYear(startISO: string, endISO: string): string | null {
  const start = parseISODate(startISO);
  const end = parseISODate(endISO);
  if (!start || !end) return null;
  const base = formatRUDateRange(startISO, endISO);
  if (!base) return null;
  const startYear = start.getUTCFullYear();
  const endYear = end.getUTCFullYear();
  if (startYear === endYear) {
    return `${base} ${endYear}`;
  }
  return `${formatDayMonth(start, true)} ${startYear}–${formatDayMonth(end, true)} ${endYear}`;
}

export function formatRUDayMonth(iso: string | null | undefined): string | null {
  const dt = parseISODate(iso);
  if (!dt) return null;
	return formatDayMonth(dt, true);
}

export function formatRUDayMonthYear(iso: string | null | undefined): string | null {
  const dt = parseISODate(iso);
  if (!dt) return null;
  return `${formatDayMonth(dt, true)} ${dt.getUTCFullYear()}`;
}

function splitRange(range: string | null | undefined): { start: string; end: string } | null {
  const text = (range ?? "").trim();
  const parts = text.split("/");
  if (parts.length !== 2) return null;
  const start = parts[0]?.trim() ?? "";
  const end = parts[1]?.trim() ?? "";
  if (!start || !end) return null;
  return { start, end };
}

function formatTrendPeriodLabel(trendPeriod: string | null | undefined): string {
  const text = (trendPeriod ?? "7d").trim().toLowerCase();
  if (text === "7d" || text === "7") return "последние 7 дней";
  return text;
}

export type PeriodContextItem = { label: string; value: string };

export type PeriodContextDisplay = {
  /** Three semantic groups: analysis period, comparison, data freshness. */
  groups: PeriodContextItem[];
};

/**
 * Human-readable period context for «Пульс продавца».
 * Groups: Период анализа / Сравнение / Данные обновлены.
 * Day-over-day delta ("Изменение к …") is intentionally omitted — it belongs on KPI cards.
 */
export function formatPeriodContextDisplay(ctx: PeriodContextFields): PeriodContextDisplay {
  if (!ctx) return { groups: [] };

  const valueDay = formatRUDayMonthYear(ctx.value_period);
  const current = splitRange(ctx.current_range);
  const previous = splitRange(ctx.previous_range);
  const currentRange = current ? formatRUDateRangeWithYear(current.start, current.end) : null;
  const previousRange = previous
    ? formatRUDateRangeWithYear(previous.start, previous.end)
    : null;
  const trendFallback = formatTrendPeriodLabel(ctx.trend_period);

  const groups: PeriodContextItem[] = [];

  if (currentRange) {
    groups.push({ label: "Период анализа", value: currentRange });
  } else if (ctx.trend_period) {
    groups.push({ label: "Период анализа", value: trendFallback });
  }

  if (previousRange) {
    groups.push({ label: "Сравнение", value: previousRange });
  }

  if (valueDay) {
    groups.push({ label: "Данные обновлены", value: valueDay });
  }

  return { groups };
}

export type PeriodContextLines = {
  primary: string | null;
  secondary: string | null;
};

/** @deprecated Prefer formatPeriodContextDisplay for semantic label/value groups. */
export function formatPeriodContextLines(ctx: PeriodContextFields): PeriodContextLines {
  const display = formatPeriodContextDisplay(ctx);
  if (display.groups.length === 0) {
    return { primary: null, secondary: null };
  }
  const compact = display.groups.map((item) => `${item.label}: ${item.value}`).join(" · ");
  return { primary: compact, secondary: null };
}
