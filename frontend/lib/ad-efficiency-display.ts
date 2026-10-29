import type {
  AdEfficiencyNarrative,
  AdEfficiencySummary,
  AdEfficiencyTrendPoint,
  DashboardHomeResponse,
} from "@/lib/dashboard-home-types";
import { asArray } from "@/lib/as-array";

export type AdEfficiencyWidget = DashboardHomeResponse["ai_diagnostics"]["ad_efficiency"];

export type DrrDeltaTone = "warning" | "success" | "neutral";

export const AD_EFFICIENCY_PENDING_SUMMARY = "AI-вывод по рекламе формируется.";
export const AD_EFFICIENCY_UNAVAILABLE_SUMMARY = "AI-вывод по рекламе пока недоступен.";
export const AD_EFFICIENCY_PENDING_CHECKLIST = "AI-чеклист по рекламе формируется.";
export const AD_EFFICIENCY_UNAVAILABLE_CHECKLIST = "AI-чеклист по рекламе пока недоступен.";

export const AD_EFFICIENCY_METRIC_LABEL =
  "ДРР в выручке с рекламных кампаний";

export const AD_EFFICIENCY_DENOMINATOR_NOTE =
  "В «Пульсе продавца» рекламные расходы сравниваются со всей выручкой кабинета. Здесь — только с выручкой, полученной с рекламы.";

export const AD_EFFICIENCY_TODAY_TOOLTIP_BASE =
  "Показывает, какая доля выручки, полученной с рекламных кампаний, была потрачена на рекламу.\n\nФормула:\nрасходы по всем рекламным кампаниям ÷ выручка, атрибутированная рекламным кампаниям, × 100%.";

export const AD_EFFICIENCY_PERIOD_TOOLTIP_BASE =
  "Показывает, какая доля выручки, полученной с рекламных кампаний, была потрачена на рекламу за период.\n\nФормула:\nсумма расходов по всем рекламным кампаниям ÷ сумма выручки, атрибутированной рекламным кампаниям, × 100%.";

const MONEY_FORMATTER = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
  minimumFractionDigits: 0,
});

const ORDERS_FORMATTER = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
});

const MONTHS_GENITIVE = [
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

export function formatDrrPercent(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return "—";
  const rounded = Math.round(value * 10) / 10;
  const isInteger = Math.abs(rounded - Math.round(rounded)) < 0.05;
  const formatted = isInteger
    ? Math.round(rounded).toLocaleString("ru-RU")
    : rounded.toLocaleString("ru-RU", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  return `${formatted}%`;
}

export function formatMoneyRub(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return "—";
  return `${MONEY_FORMATTER.format(Math.round(value)).replace(/\u00A0/g, " ")} ₽`;
}

export function formatOrders(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return "—";
  return ORDERS_FORMATTER.format(value);
}

export function formatPpDelta(value?: number | null): string | null {
  if (value == null || Number.isNaN(value)) return null;
  const rounded = Math.round(value * 10) / 10;
  const isInteger = Math.abs(rounded - Math.round(rounded)) < 0.05;
  const absFormatted = isInteger
    ? Math.round(Math.abs(rounded)).toLocaleString("ru-RU")
    : Math.abs(rounded).toLocaleString("ru-RU", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  if (rounded > 0) return `+${absFormatted} п.п.`;
  if (rounded < 0) return `−${absFormatted} п.п.`;
  return "0 п.п.";
}

export function getDrrDeltaTone(delta?: number | null): DrrDeltaTone {
  if (delta == null || Number.isNaN(delta) || delta === 0) return "neutral";
  return delta > 0 ? "warning" : "success";
}

export function formatPeriodLabel(sampleDays?: number | null): string | null {
  if (sampleDays == null || sampleDays <= 0) return null;
  if (sampleDays >= 7) return "за последние 7 дней";
  return `за доступные ${sampleDays} ${daysWord(sampleDays)}`;
}

export function formatAverageDeltaCaption(
  sampleDays?: number | null,
  averageDrrPct?: number | null,
): string | null {
  if (sampleDays == null || sampleDays <= 1 || averageDrrPct == null) return null;
  if (sampleDays >= 7) return "к среднему за 7 дней";
  return `к среднему за доступные ${sampleDays} ${daysWord(sampleDays)}`;
}

export function formatFooterNote(sampleDays?: number | null): string {
  if (sampleDays == null || sampleDays <= 0) {
    return "Данные по рекламе обновляются ежедневно.";
  }
  if (sampleDays >= 7) {
    return "Данные по рекламе обновляются ежедневно. Показаны кампании и расходы за последние 7 дней.";
  }
  return `Данные по рекламе обновляются ежедневно. Показаны кампании и расходы за доступные ${sampleDays} ${daysWord(sampleDays)}.`;
}

export const AD_EFFICIENCY_ACTIONS_HREF = "/app/actions?filter=advertising";

export function formatAverageLineLabel(sampleDays?: number | null): string | null {
  if (sampleDays == null || sampleDays <= 0) return null;
  if (sampleDays >= 7) return "За 7 дней";
  return `За доступные ${sampleDays} ${daysWord(sampleDays)}`;
}

export function formatAverageChartLabel(
  sampleDays: number | null | undefined,
  averageDrrPct: number | null | undefined,
): string | null {
  if (averageDrrPct == null || Number.isNaN(averageDrrPct)) return null;
  const period =
    sampleDays != null && sampleDays > 0 && sampleDays < 7
      ? `За доступные ${sampleDays} ${daysWord(sampleDays)}`
      : "За 7 дней";
  return `${period}: ${formatDrrPercent(averageDrrPct)}`;
}

function appendDynamicShareCalc(
  base: string,
  spend?: number | null,
  revenue?: number | null,
): string {
  if (spend == null || revenue == null || Number.isNaN(spend) || Number.isNaN(revenue)) {
    return base;
  }
  if (spend <= 0 || revenue <= 0) return base;
  const pct = (spend / revenue) * 100;
  return `${base}\n\nРасчёт: ${formatMoneyRub(spend)} ÷ ${formatMoneyRub(revenue)} = ${formatDrrPercent(pct)}.`;
}

export function buildAdEfficiencyTodayTooltip(summary?: AdEfficiencySummary | null): string {
  return appendDynamicShareCalc(
    AD_EFFICIENCY_TODAY_TOOLTIP_BASE,
    summary?.total_spend_today,
    summary?.total_revenue_today,
  );
}

export function buildAdEfficiencyPeriodTooltip(
  summary?: AdEfficiencySummary | null,
  trend?: Array<{ spend?: number; revenue?: number; data_status?: string }> | null,
): string {
  let spend = summary?.total_spend_7d;
  let revenue = summary?.total_attributed_revenue_7d;
  if ((spend == null || spend <= 0 || revenue == null || revenue <= 0) && Array.isArray(trend)) {
    let spendSum = 0;
    let revenueSum = 0;
    for (const pt of trend) {
      if (pt?.data_status && pt.data_status !== "available") continue;
      spendSum += Number(pt.spend) || 0;
      revenueSum += Number(pt.revenue) || 0;
    }
    if (spendSum > 0 && revenueSum > 0) {
      spend = spendSum;
      revenue = revenueSum;
    }
  }
  return appendDynamicShareCalc(AD_EFFICIENCY_PERIOD_TOOLTIP_BASE, spend, revenue);
}

export type TrendPointDisplay = {
  date: string;
  label: string;
  drrValue: number | null;
  drrLabel: string | null;
  deltaLabel: string | null;
  deltaTone: DrrDeltaTone;
  isToday: boolean;
  isYesterday: boolean;
  hasDrr: boolean;
};

export type AdEfficiencyChecklistItem = {
  title: string;
  text: string;
};

export type AdEfficiencyAIConclusion = {
  kind: "ai" | "pending" | "unavailable";
  title: string | null;
  body: string;
};

export type AdEfficiencyDisplay = {
  status: string;
  partialMessage: string | null;
  hasBody: boolean;
  sampleDays: number | null;
  currentDrr: string;
  deltaVsPrevious: string | null;
  deltaVsPreviousTone: DrrDeltaTone;
  weeklyDelta: string | null;
  previousCompletedDateLabel: string | null;
  aiConclusion: AdEfficiencyAIConclusion;
  highDrrCount: number;
  highDrrThresholdPct: number;
  noOrdersCount: number;
  averageDrrPct: number | null;
  averageDrrPeriodLabel: string | null;
  averageLineLabel: string | null;
  averageChartLabel: string | null;
  todayTooltip: string;
  periodTooltip: string;
  trendPoints: TrendPointDisplay[];
  hasTrendData: boolean;
  footerNote: string;
  checklist: AdEfficiencyChecklistItem[];
  checklistState: "items" | "pending" | "unavailable";
  actionsHref: string;
};

function daysWord(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return "день";
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return "дня";
  return "дней";
}

function parseISODate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value.trim());
  if (!match) return null;
  const year = Number(match[1]);
  const month = Number(match[2]) - 1;
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month, day));
  if (Number.isNaN(date.getTime())) return null;
  return date;
}

export function formatAdEfficiencyDate(value?: string | null): string | null {
  if (!value) return null;
  const date = parseISODate(value);
  if (!date) return value;
  return `${date.getUTCDate()} ${MONTHS_GENITIVE[date.getUTCMonth()]}`;
}

export function formatTrendAxisLabel(point: AdEfficiencyTrendPoint): string {
  const raw = point.label?.trim();
  if (raw) {
    return raw.replace(/\s*\(сегодня\)\s*$/i, "").replace(/\s*\(вчера\)\s*$/i, "").trim();
  }
  const date = parseISODate(point.date);
  if (!date) return point.date;
  const day = date.getUTCDate();
  const month = MONTHS_GENITIVE[date.getUTCMonth()] ?? "";
  return `${day} ${month}`;
}

function isNarrativeReady(status?: string): boolean {
  return status === "ready" || status === "available";
}

function isNarrativePending(status?: string): boolean {
  return status === "pending" || status === "source_pending";
}

function sanitizeAIText(text?: string | null): string {
  return (text ?? "").trim().replace(/\s+/g, " ");
}

function isSubstantiveAIText(text?: string | null): boolean {
  const s = sanitizeAIText(text);
  return s.length >= 16;
}

export function getAdEfficiencyAISummary(narrative?: AdEfficiencyNarrative | null): {
  kind: "ai" | "pending" | "unavailable";
  text: string;
  headline: string;
} {
  const headline = sanitizeAIText(narrative?.headline);
  if (
    isNarrativeReady(narrative?.status) &&
    (isSubstantiveAIText(narrative?.summary) || isSubstantiveAIText(headline))
  ) {
    return {
      kind: "ai",
      text: sanitizeAIText(narrative?.summary),
      headline,
    };
  }
  if (isNarrativePending(narrative?.status)) {
    return { kind: "pending", text: AD_EFFICIENCY_PENDING_SUMMARY, headline: "" };
  }
  return { kind: "unavailable", text: AD_EFFICIENCY_UNAVAILABLE_SUMMARY, headline: "" };
}

/**
 * Builds the AI conclusion card from separate headline + summary fields.
 * New publications must supply both; never invent a title by splitting on '.'.
 *
 * Legacy publications (summary only, no headline): show the full summary as
 * body with no title. Do NOT split on '.' — decimals like 19.4% must stay intact.
 */
export function parseAdEfficiencyAIConclusion(
  summary: string,
  kind: "ai" | "pending" | "unavailable",
  headline?: string | null,
): AdEfficiencyAIConclusion {
  if (kind !== "ai") {
    const body = kind === "pending" ? AD_EFFICIENCY_PENDING_SUMMARY : AD_EFFICIENCY_UNAVAILABLE_SUMMARY;
    return { kind, title: null, body };
  }

  const title = sanitizeAIText(headline);
  const body = sanitizeAIText(summary);

  if (!body && !title) {
    return { kind: "unavailable", title: null, body: AD_EFFICIENCY_UNAVAILABLE_SUMMARY };
  }

  // Prefer explicit headline; never derive it from body by splitting on '.'.
  if (title && body) {
    return { kind: "ai", title, body };
  }
  if (title && !body) {
    return { kind: "ai", title, body: title };
  }
  // Legacy: summary-only narrative — show full text as body, no invented title.
  return { kind: "ai", title: null, body };
}

export function getAdEfficiencyChecklist(narrative?: AdEfficiencyNarrative | null): {
  state: "items" | "pending" | "unavailable";
  items: AdEfficiencyChecklistItem[];
} {
  const raw = asArray(narrative?.checklist);
  if (isNarrativeReady(narrative?.status)) {
    const items: AdEfficiencyChecklistItem[] = [];
    for (const item of raw) {
      const title = sanitizeAIText(item.title);
      const text = sanitizeAIText(item.text);
      if (!title && !text) continue;
      items.push({ title: title || text, text: title ? text : "" });
      if (items.length >= 3) break;
    }
    if (items.length > 0) return { state: "items", items };
  }
  if (isNarrativePending(narrative?.status)) {
    return { state: "pending", items: [] };
  }
  return { state: "unavailable", items: [] };
}

function buildTrendDisplay(points: AdEfficiencyTrendPoint[]): TrendPointDisplay[] {
  const out: TrendPointDisplay[] = [];
  let prevDrr: number | null = null;

  for (const point of points) {
    const hasDrr = point.drr_pct != null && !Number.isNaN(point.drr_pct);
    let deltaLabel: string | null = null;
    let deltaTone: DrrDeltaTone = "neutral";

    if (hasDrr && prevDrr != null) {
      const delta = point.drr_pct! - prevDrr;
      deltaLabel = formatPpDelta(delta);
      deltaTone = getDrrDeltaTone(delta);
    }

    if (hasDrr) {
      prevDrr = point.drr_pct!;
    }

    out.push({
      date: point.date,
      label: formatTrendAxisLabel(point),
      drrValue: hasDrr ? point.drr_pct! : null,
      drrLabel: hasDrr ? formatDrrPercent(point.drr_pct) : null,
      deltaLabel,
      deltaTone,
      isToday: Boolean(point.is_today),
      isYesterday: Boolean(point.is_yesterday),
      hasDrr,
    });
  }

  return out;
}

function hasRenderableTrend(points: AdEfficiencyTrendPoint[]): boolean {
  return points.some((p) => p.drr_pct != null && !Number.isNaN(p.drr_pct));
}

export function buildAdEfficiencyChartSummary(
  points: TrendPointDisplay[],
  averageDrrPct: number | null,
): string {
  const values = points
    .map((p) => p.drrValue)
    .filter((v): v is number => v != null && !Number.isNaN(v));
  if (values.length === 0) {
    return "Недостаточно данных для динамики доли рекламных расходов в выручке с кампаний.";
  }
  const min = Math.min(...values);
  const max = Math.max(...values);
  const current = values[values.length - 1];
  const avgPart =
    averageDrrPct != null && !Number.isNaN(averageDrrPct)
      ? `, за 7 дней — ${formatDrrPercent(averageDrrPct)}`
      : "";
  return `За последние ${values.length} ${daysWord(values.length)} доля расходов в выручке с рекламных кампаний изменялась от ${formatDrrPercent(min)} до ${formatDrrPercent(max)}. Текущее значение — ${formatDrrPercent(current)}${avgPart}.`;
}

export function buildAdEfficiencyDisplay(widget: AdEfficiencyWidget): AdEfficiencyDisplay {
  const summary = widget.summary;
  const period = widget.period;
  const sampleDays = period?.sample_days ?? null;
  const narrative = widget.ad_efficiency_narrative;
  const trendRaw = asArray(widget.trend_7d);
  const checklistResult = getAdEfficiencyChecklist(narrative);

  const aiSummaryCandidate = getAdEfficiencyAISummary(narrative);
  const aiSummaryDirectionConflict = (() => {
    if (aiSummaryCandidate.kind !== "ai") return false;
    const delta = summary?.delta_vs_previous_pp;
    if (delta == null || Number.isNaN(delta) || delta === 0) return false;
    const text = aiSummaryCandidate.text.toLowerCase();
    const hasGrowthWords =
      text.includes("рост дрр") ||
      text.includes("рост drr") ||
      text.includes("дrr вырос") ||
      text.includes("drr вырос") ||
      text.includes("дрр вырос") ||
      text.includes("увеличение дрр") ||
      text.includes("повышение дрр");
    const hasDeclineWords =
      text.includes("снижение дрр") ||
      text.includes("снижение drr") ||
      text.includes("дрр сниз") ||
      text.includes("drr сниз") ||
      text.includes("уменьшение дрр") ||
      text.includes("улучшение дрр");
    if (delta < 0 && hasGrowthWords) return true;
    if (delta > 0 && hasDeclineWords) return true;
    return false;
  })();

  const aiConclusionKind = aiSummaryDirectionConflict ? "unavailable" : aiSummaryCandidate.kind;
  const aiConclusionSummary =
    aiSummaryDirectionConflict ? "" : aiSummaryCandidate.kind === "ai" ? aiSummaryCandidate.text : "";
  const aiConclusionHeadline =
    aiSummaryDirectionConflict ? "" : aiSummaryCandidate.kind === "ai" ? aiSummaryCandidate.headline : "";

  return {
    status: widget.status ?? "available",
    partialMessage:
      widget.status === "partial"
        ? widget.message?.trim() || "Данные частично доступны. Часть показателей может быть ограничена."
        : null,
    hasBody: Boolean(summary),
    sampleDays,
    currentDrr: formatDrrPercent(summary?.current_drr_pct),
    deltaVsPrevious: formatPpDelta(summary?.delta_vs_previous_pp),
    deltaVsPreviousTone: getDrrDeltaTone(summary?.delta_vs_previous_pp),
    weeklyDelta: formatPpDelta(summary?.delta_vs_previous_7d_pp),
    previousCompletedDateLabel: formatAdEfficiencyDate(period?.previous_completed_date),
    aiConclusion: parseAdEfficiencyAIConclusion(
      aiConclusionSummary,
      aiConclusionKind,
      aiConclusionHeadline,
    ),
    highDrrCount: summary?.campaigns_high_drr_count ?? 0,
    highDrrThresholdPct: summary?.high_drr_threshold_pct ?? 100,
    noOrdersCount: summary?.campaigns_no_orders_count ?? 0,
    averageDrrPct: summary?.average_drr_7d_pct ?? null,
    averageDrrPeriodLabel: formatPeriodLabel(sampleDays),
    averageLineLabel: formatAverageLineLabel(sampleDays),
    averageChartLabel: formatAverageChartLabel(sampleDays, summary?.average_drr_7d_pct ?? null),
    todayTooltip: buildAdEfficiencyTodayTooltip(summary),
    periodTooltip: buildAdEfficiencyPeriodTooltip(summary, trendRaw),
    trendPoints: buildTrendDisplay(trendRaw),
    hasTrendData: hasRenderableTrend(trendRaw),
    footerNote: formatFooterNote(sampleDays),
    checklist: checklistResult.items,
    checklistState: checklistResult.state,
    actionsHref: AD_EFFICIENCY_ACTIONS_HREF,
  };
}
