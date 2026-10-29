import { asArray } from "@/lib/as-array";
import type {
  DashboardHomeResponse,
  GlobalStatus,
  KPICard,
  SellerPulseNarrative,
  SellerPulseQuickAction,
  SparklinePoint,
} from "@/lib/dashboard-home-types";
import type { StatusPillVariant } from "@/components/ui/status-pill";

export type PulseMetricKey =
  | "revenue"
  | "orders"
  | "average_order_value"
  | "ad_spend_share_total_revenue"
  | "ctr"
  | "drr"
  | "risks";

export type PulseMetricState =
  | "positive"
  | "negative"
  | "neutral"
  | "warning"
  | "critical"
  | "no_data"
  | "unavailable";

export type PulseSummaryTone = "positive" | "negative" | "warning" | "critical" | "neutral" | "info";

export type PulseMetricView = {
  key: PulseMetricKey;
  label: string;
  value: string | null;
  valueDisplay: string;
  deltaText: string | null;
  statusLabel: string;
  state: PulseMetricState;
  pillVariant: StatusPillVariant;
  sparkline: SparklinePoint[];
  hasSparkline: boolean;
  sparklineStroke: string;
  cardAccentClass: string;
  deltaClass: string;
  sparklineEmptyLabel: string;
  /** Info tooltip (formula / explanation); from API hint when present. */
  hint?: string | null;
};

export type PulseSummarySignal = {
  label: string;
  variant: StatusPillVariant;
};

export type PulseSummaryHighlight = {
  title: string;
  value: string;
  variant: StatusPillVariant;
};

export type PulseSummaryView = {
  headline: string;
  summary?: string;
  tone: PulseSummaryTone;
  signals: PulseSummarySignal[];
  highlights: PulseSummaryHighlight[];
};

export type PulseKnowledgeItem = {
  title?: string;
  text: string;
  tone?: string;
};

export type PulseQuickAction = {
  label: string;
  description?: string;
  kind?: string;
  priority?: string;
  href?: string;
  actionId?: string;
};

export type SellerPulseViewModel = {
  metrics: PulseMetricView[];
  summary: PulseSummaryView;
  /** Bullet points for AI-резюме (from narrative.key_points). */
  keyPoints: PulseKnowledgeItem[];
  /** Items for «Что важно знать» (from narrative.important_notes only). */
  knowledge: PulseKnowledgeItem[];
  quickActions: PulseQuickAction[];
  narrativeStatus?: SellerPulseNarrative["status"] | "source_not_available";
  dataQualityNote?: string;
};

export type SellerPulseBuildInput = {
  data: DashboardHomeResponse["seller_pulse"];
  dashboardStatus?: GlobalStatus;
  recommendedActionsCount?: number;
  /** 7-day totals for the ad_spend_share_total_revenue (weekly share) tooltip calculation — matches the backend's 7-day aggregation window. */
  adSpend7d?: number | null;
  storeRevenue7d?: number | null;
};

/** KPI cards shown in the seller pulse row (risks stay in narrative/fallback only). */
export const METRIC_CARD_ORDER: PulseMetricKey[] = [
  "revenue",
  "orders",
  "average_order_value",
  "ad_spend_share_total_revenue",
];

const METRIC_ORDER: PulseMetricKey[] = [...METRIC_CARD_ORDER, "risks"];

const METRIC_LABELS: Record<PulseMetricKey, string> = {
  revenue: "Выручка",
  orders: "Заказы",
  average_order_value: "Средний чек",
  ad_spend_share_total_revenue: "Реклама в общей выручке (7 дней)",
  ctr: "Кликабельность карточек",
  drr: "Реклама в общей выручке",
  risks: "Риски",
};

export const PULSE_CTR_TOOLTIP =
  "Показывает долю переходов в карточки товаров от общего количества их показов по всему кабинету.\n\nФормула:\nклики по всем карточкам ÷ показы всех карточек × 100%.";

export const PULSE_AOV_TOOLTIP =
  "Выручка отчётного дня ÷ число заказов. При нуле заказов показатель не рассчитывается.";

export const PULSE_AD_SPEND_SHARE_TOOLTIP =
  "Недельная доля: показывает, какую долю общей выручки кабинета за последние 7 дней составляют рекламные расходы за тот же период.\n\nФормула:\nрасходы по всем рекламным кампаниям за 7 дней ÷ общая выручка кабинета за 7 дней × 100%.\n\nНе путать с ДРР кампаний (расход ÷ атрибутированная рекламная выручка).";

const NEUTRAL_PCT_THRESHOLD = 2;
const CTR_NEUTRAL_PP_THRESHOLD = 0.15;
const DRR_WARNING_THRESHOLD = 20;
const DRR_CRITICAL_THRESHOLD = 35;

const moneyFormatter = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
});

const pctFormatter = new Intl.NumberFormat("ru-RU", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
  signDisplay: "exceptZero",
});

const STATE_PILL: Record<PulseMetricState, StatusPillVariant> = {
  positive: "success",
  negative: "danger",
  neutral: "neutral",
  warning: "warning",
  critical: "danger",
  no_data: "neutral",
  unavailable: "neutral",
};

const STATE_SPARKLINE: Record<PulseMetricState, string> = {
  positive: "#12b76a",
  negative: "#f04438",
  neutral: "#98a2b3",
  warning: "#f79009",
  critical: "#f04438",
  no_data: "#d0d5dd",
  unavailable: "#d0d5dd",
};

const STATE_CARD_ACCENT: Record<PulseMetricState, string> = {
  positive: "border-t-[3px] border-t-[#12b76a]",
  negative: "border-t-[3px] border-t-[#f04438]",
  neutral: "border-t-[3px] border-t-slate-200",
  warning: "border-t-[3px] border-t-[#f79009]",
  critical: "border-t-[3px] border-t-[#b42318]",
  no_data: "border-t-[3px] border-t-slate-200",
  unavailable: "border-t-[3px] border-t-slate-200",
};

const STATE_DELTA_CLASS: Record<PulseMetricState, string> = {
  positive: "text-[#027a48]",
  negative: "text-[#b42318]",
  neutral: "text-aivion-muted",
  warning: "text-[#b54708]",
  critical: "text-[#b42318]",
  no_data: "text-aivion-muted-soft",
  unavailable: "text-aivion-muted-soft",
};

const SPARKLINE_EMPTY_LABEL: Record<PulseMetricState, string> = {
  positive: "Нет истории за период",
  negative: "Нет истории за период",
  neutral: "Нет истории за период",
  warning: "Нет истории за период",
  critical: "Нет истории за период",
  no_data: "Появится после синхронизации",
  unavailable: "Динамика недоступна",
};

const SUMMARY_TONE_CLASS: Record<PulseSummaryTone, string> = {
  positive: "border-[#abefc6] bg-[#ecfdf3]",
  negative: "border-[#fecdca] bg-[#fef3f2]",
  warning: "border-[#fedf89] bg-[#fffaeb]",
  critical: "border-[#fecdca] bg-[#fef3f2]",
  neutral: "border-aivion-border bg-aivion-surface-soft",
  info: "border-[#b2ddff] bg-[#eff8ff]",
};

const SUMMARY_ICON_CLASS: Record<PulseSummaryTone, string> = {
  positive: "text-[#027a48]",
  negative: "text-[#b42318]",
  warning: "text-[#b54708]",
  critical: "text-[#b42318]",
  neutral: "text-aivion-muted",
  info: "text-[#175cd3]",
};

export function getPulseSummaryStripClass(tone: PulseSummaryTone): string {
  return SUMMARY_TONE_CLASS[tone];
}

export function getPulseSummaryIconClass(tone: PulseSummaryTone): string {
  return SUMMARY_ICON_CLASS[tone];
}

export function getPulseMetricDeltaClass(state: PulseMetricState): string {
  return STATE_DELTA_CLASS[state];
}

export function getPulseKnowledgeToneClass(tone?: string): string {
  switch ((tone ?? "").trim().toLowerCase()) {
    case "positive":
      return "bg-[#12b76a]";
    case "warning":
      return "bg-[#f79009]";
    case "critical":
      return "bg-[#f04438]";
    default:
      return "bg-[#0873f6]";
  }
}

export function getPulseKnowledgeIconToneClass(tone?: string): string {
  switch ((tone ?? "").trim().toLowerCase()) {
    case "positive":
      return "text-[#12b76a]";
    case "warning":
      return "text-[#f79009]";
    case "critical":
      return "text-[#f04438]";
    case "negative":
      return "text-[#b42318]";
    default:
      return "text-[#0873f6]";
  }
}

const HIGHLIGHT_VARIANT_CLASS: Record<StatusPillVariant, string> = {
  success: "border-[#abefc6] bg-white/95",
  danger: "border-[#fecdca] bg-white/95",
  warning: "border-[#fedf89] bg-white/95",
  neutral: "border-aivion-border bg-white/95",
  info: "border-[#b2ddff] bg-white/95",
};

export function getPulseSummaryHighlightClass(variant: StatusPillVariant): string {
  return HIGHLIGHT_VARIANT_CLASS[variant];
}

const NARRATIVE_SENTENCE_END_RE = /[.!?…:;]$/u;

export function formatNarrativeSentence(text?: string | null): string {
  const trimmed = (text ?? "").trim().replace(/\s+/g, " ");
  if (!trimmed) {
    return "";
  }
  if (NARRATIVE_SENTENCE_END_RE.test(trimmed)) {
    return trimmed;
  }
  return `${trimmed}.`;
}

export function formatPulseReportDate(isoDate?: string): string | null {
  const trimmed = isoDate?.trim();
  if (!trimmed) return null;
  const isoMatch = trimmed.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (isoMatch) {
    return `${isoMatch[3]}.${isoMatch[2]}.${isoMatch[1]}`;
  }
  if (/^\d{2}\.\d{2}\.\d{4}$/.test(trimmed)) {
    return trimmed;
  }
  return null;
}

export function formatPulseChartDate(date?: string): string {
  const formatted = formatPulseReportDate(date);
  return formatted ?? date?.trim() ?? "";
}

const chartPctFormatter = new Intl.NumberFormat("ru-RU", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

export function formatPulseChartAxisValue(metricKey: PulseMetricKey, value: number): string {
  switch (metricKey) {
    case "revenue":
    case "average_order_value":
      if (Math.abs(value) >= 1_000_000) {
        return `${Math.round(value / 1_000_000)}М`;
      }
      if (Math.abs(value) >= 1_000) {
        return `${Math.round(value / 1_000)}к`;
      }
      return moneyFormatter.format(Math.round(value));
    case "orders":
      return moneyFormatter.format(Math.round(value));
    case "ctr":
    case "drr":
    case "ad_spend_share_total_revenue":
      return `${chartPctFormatter.format(value)}%`;
    default:
      return moneyFormatter.format(Math.round(value));
  }
}

export function formatPulseChartTooltipValue(metricKey: PulseMetricKey, value: number): string {
  switch (metricKey) {
    case "revenue":
    case "average_order_value":
      return `${moneyFormatter.format(Math.round(value))} ₽`;
    case "orders":
      return moneyFormatter.format(Math.round(value));
    case "ctr":
    case "drr":
    case "ad_spend_share_total_revenue":
      return `${chartPctFormatter.format(value)}%`;
    default:
      return moneyFormatter.format(Math.round(value));
  }
}

function normalizeHighlightTitle(metric: string): string {
  const lower = metric.toLowerCase();
  if (lower.includes("дрр")) return "Главный риск дня";
  if (lower.includes("риск")) return "Открытых рисков";
  return metric;
}

export function parseSummaryHighlightLabel(label: string): { title: string; value: string } {
  const trimmed = label.trim();
  if (!trimmed) {
    return { title: "", value: "" };
  }

  const colonIdx = trimmed.indexOf(":");
  if (colonIdx > 0) {
    const left = trimmed.slice(0, colonIdx).trim();
    const right = trimmed.slice(colonIdx + 1).trim();
    return {
      title: normalizeHighlightTitle(left),
      value: right,
    };
  }

  const metricValueMatch = trimmed.match(/^([^\d]+?)\s+(.+)$/);
  if (metricValueMatch) {
    return {
      title: normalizeHighlightTitle(metricValueMatch[1].trim()),
      value: metricValueMatch[2].trim(),
    };
  }

  return { title: "Сигнал", value: trimmed };
}

export function mapSignalToHighlight(signal: PulseSummarySignal): PulseSummaryHighlight {
  const parsed = parseSummaryHighlightLabel(signal.label);
  return {
    title: parsed.title,
    value: parsed.value,
    variant: signal.variant,
  };
}

export function mapSignalsToHighlights(signals: PulseSummarySignal[]): PulseSummaryHighlight[] {
  return signals
    .map(mapSignalToHighlight)
    .filter((highlight) => highlight.title.length > 0 || highlight.value.length > 0)
    .slice(0, 3);
}

function withSummaryHighlights(
  summary: Omit<PulseSummaryView, "highlights">,
): PulseSummaryView {
  return {
    ...summary,
    highlights: mapSignalsToHighlights(summary.signals),
  };
}

export function getSparklineEmptyLabel(state: PulseMetricState): string {
  return SPARKLINE_EMPTY_LABEL[state];
}

export function mapNarrativeStatusToSummaryTone(
  status: SellerPulseNarrative["status"] | undefined,
  tone?: string,
): PulseSummaryTone {
  if (status && status !== "ready") {
    return "info";
  }
  return mapNarrativeTone(tone);
}

function parseNumericValue(value?: string | null): number | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed || trimmed === "—" || trimmed === "-") return null;
  const normalized = trimmed
    .replace(/\s/g, "")
    .replace(/₽/g, "")
    .replace(/%/g, "")
    .replace(",", ".");
  const parsed = Number.parseFloat(normalized);
  return Number.isFinite(parsed) ? parsed : null;
}

function hasMeaningfulValue(value?: string | null): boolean {
  if (!value) return false;
  const trimmed = value.trim();
  return trimmed.length > 0 && trimmed !== "—" && trimmed !== "-";
}

function isSignificantDelta(key: PulseMetricKey, delta: NonNullable<KPICard["delta"]>): boolean {
  if (delta.pct != null && Math.abs(delta.pct) >= NEUTRAL_PCT_THRESHOLD) {
    return true;
  }
  if (key === "orders" || key === "risks") {
    return Math.abs(delta.abs) >= 1;
  }
  if (key === "ctr") {
    return Math.abs(delta.abs) >= CTR_NEUTRAL_PP_THRESHOLD;
  }
  if (key === "revenue" || key === "average_order_value") {
    return Math.abs(delta.abs) >= 1;
  }
  return Math.abs(delta.abs) > 0;
}

function formatMoneyDelta(value: number): string {
  const prefix = value >= 0 ? "+" : "";
  return `${prefix}${moneyFormatter.format(Math.round(value))} ₽`;
}

function formatPctDelta(value: number): string {
  return `${pctFormatter.format(value)}%`;
}

function formatDeltaText(key: PulseMetricKey, delta?: KPICard["delta"] | null): string | null {
  if (!delta) return null;
  if (delta.display?.trim()) {
    const display = delta.display.trim();
    return delta.label ? `${display} ${delta.label}`.trim() : display;
  }
  const parts: string[] = [];
  if (key === "revenue" || key === "average_order_value") {
    parts.push(formatMoneyDelta(delta.abs));
  } else if (key === "ctr") {
    const prefix = delta.abs >= 0 ? "+" : "";
    parts.push(`${prefix}${pctFormatter.format(delta.abs)} п.п.`);
  } else if (key === "ad_spend_share_total_revenue" || key === "drr") {
    const prefix = delta.abs >= 0 ? "+" : "";
    parts.push(`${prefix}${pctFormatter.format(delta.abs)} п.п.`);
  } else {
    const prefix = delta.abs >= 0 ? "+" : "";
    parts.push(`${prefix}${Math.round(delta.abs)}`);
  }
  if (delta.pct != null) {
    parts.push(`(${formatPctDelta(delta.pct)})`);
  }
  if (delta.label) {
    parts.push(delta.label);
  }
  return parts.join(" ");
}

function sparklineHasHistory(points: SparklinePoint[]): boolean {
  const withDataFlag = points.filter((p) => p.has_data !== false && p.value != null && Number.isFinite(p.value));
  if (withDataFlag.length >= 2) {
    return true;
  }
  const values = points
    .map((p) => p.value)
    .filter((v): v is number => v != null && Number.isFinite(v));
  return values.length >= 2;
}

function resolveSparklineTrendLabel(kpi: KPICard | undefined, hasSparkline: boolean): string | null {
  if (!hasSparkline) {
    return null;
  }
  const trendLabel = (kpi?.trend?.label ?? "").trim();
  if (trendLabel && trendLabel.toLowerCase() !== "динамика недоступна") {
    return trendLabel;
  }
  return "Динамика за 7 дней";
}

function resolveGrowthMetricState(
  key: PulseMetricKey,
  kpi: KPICard | undefined,
  present: boolean,
): Pick<PulseMetricView, "state" | "statusLabel" | "value" | "valueDisplay" | "deltaText" | "sparkline" | "hasSparkline"> {
  if (!present) {
    const emptyLabel =
      key === "ctr" ? "Нет данных по показам" : "Метрика появится после синхронизации";
    return {
      state: "no_data",
      statusLabel: "Недостаточно данных",
      value: null,
      valueDisplay: emptyLabel,
      deltaText: null,
      sparkline: [],
      hasSparkline: false,
    };
  }

  const sparkline = asArray(kpi?.sparkline_7d);
  const hasSparkline = sparklineHasHistory(sparkline);
  const hasValue = hasMeaningfulValue(kpi?.value);

  if (!hasValue) {
    return {
      state: "no_data",
      statusLabel: "Недостаточно данных",
      value: null,
      valueDisplay: "Метрика появится после синхронизации",
      deltaText: null,
      sparkline,
      hasSparkline,
    };
  }

  const delta = kpi?.delta ?? null;
  if (!delta) {
    return {
      state: "neutral",
      statusLabel: hasSparkline ? "За 7 дней" : "Динамика недоступна",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: resolveSparklineTrendLabel(kpi, hasSparkline),
      sparkline,
      hasSparkline,
    };
  }

  if (!isSignificantDelta(key, delta)) {
    return {
      state: "neutral",
      statusLabel: "Стабильно",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText(key, delta),
      sparkline,
      hasSparkline,
    };
  }

  const improved = delta.abs > 0 || (delta.pct != null && delta.pct > 0);
  return {
    state: improved ? "positive" : "negative",
    statusLabel: improved ? "Рост" : "Снижение",
    value: kpi!.value,
    valueDisplay: kpi!.value,
    deltaText: formatDeltaText(key, delta),
    sparkline,
    hasSparkline,
  };
}

function resolveDrrState(
  kpi: KPICard | undefined,
  present: boolean,
): Pick<PulseMetricView, "state" | "statusLabel" | "value" | "valueDisplay" | "deltaText" | "sparkline" | "hasSparkline"> {
  if (!present) {
    return {
      state: "no_data",
      statusLabel: "Нет рекламных данных",
      value: null,
      valueDisplay: "Метрика появится после синхронизации",
      deltaText: null,
      sparkline: [],
      hasSparkline: false,
    };
  }

  const sparkline = asArray(kpi?.sparkline_7d);
  const hasSparkline = sparklineHasHistory(sparkline);
  const hasValue = hasMeaningfulValue(kpi?.value);
  const numeric = parseNumericValue(kpi?.value);

  if (!hasValue || numeric == null) {
    return {
      state: "no_data",
      statusLabel: "Недостаточно данных",
      value: null,
      valueDisplay: "Данные за период пока недоступны",
      deltaText: null,
      sparkline,
      hasSparkline,
    };
  }

  const delta = kpi?.delta ?? null;
  if (delta && isSignificantDelta("drr", delta)) {
    const worsened = delta.abs > 0 || (delta.pct != null && delta.pct > 0);
    return {
      state: worsened ? (numeric >= DRR_CRITICAL_THRESHOLD ? "critical" : "warning") : "positive",
      statusLabel: worsened ? "Рост доли" : "Снижение доли",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText("drr", delta),
      sparkline,
      hasSparkline,
    };
  }

  if (numeric >= DRR_CRITICAL_THRESHOLD) {
    return {
      state: "critical",
      statusLabel: "Критично высокий",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText("drr", delta) ?? resolveSparklineTrendLabel(kpi, hasSparkline),
      sparkline,
      hasSparkline,
    };
  }
  if (numeric >= DRR_WARNING_THRESHOLD) {
    return {
      state: "warning",
      statusLabel: "Требует внимания",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText("drr", delta) ?? resolveSparklineTrendLabel(kpi, hasSparkline),
      sparkline,
      hasSparkline,
    };
  }
  return {
    state: "positive",
    statusLabel: "В норме",
    value: kpi!.value,
    valueDisplay: kpi!.value,
    deltaText: formatDeltaText("drr", delta) ?? resolveSparklineTrendLabel(kpi, hasSparkline),
    sparkline,
    hasSparkline,
  };
}

function resolveRisksState(
  kpi: KPICard | undefined,
  present: boolean,
): Pick<PulseMetricView, "state" | "statusLabel" | "value" | "valueDisplay" | "deltaText" | "sparkline" | "hasSparkline"> {
  if (!present) {
    return {
      state: "unavailable",
      statusLabel: "Источник временно недоступен",
      value: null,
      valueDisplay: "Источник временно недоступен",
      deltaText: null,
      sparkline: [],
      hasSparkline: false,
    };
  }

  const sparkline = asArray(kpi?.sparkline_7d);
  const hasSparkline = sparklineHasHistory(sparkline);
  const count = parseNumericValue(kpi?.value);

  if (count == null) {
    return {
      state: "no_data",
      statusLabel: "Недостаточно данных",
      value: null,
      valueDisplay: "Метрика появится после синхронизации",
      deltaText: null,
      sparkline,
      hasSparkline,
    };
  }

  const delta = kpi?.delta ?? null;
  if (count === 0) {
    return {
      state: "positive",
      statusLabel: "Без критичных рисков",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText("risks", delta),
      sparkline,
      hasSparkline,
    };
  }
  if (count >= 3) {
    return {
      state: "critical",
      statusLabel: "Высокий уровень рисков",
      value: kpi!.value,
      valueDisplay: kpi!.value,
      deltaText: formatDeltaText("risks", delta),
      sparkline,
      hasSparkline,
    };
  }
  return {
    state: "warning",
    statusLabel: "Есть риски",
    value: kpi!.value,
    valueDisplay: kpi!.value,
    deltaText: formatDeltaText("risks", delta),
    sparkline,
    hasSparkline,
  };
}

function resolveMetricHint(
  key: PulseMetricKey,
  kpi: KPICard | undefined,
  shareCalc?: { spend?: number | null; revenue?: number | null },
): string | null {
  const fromApi = kpi?.hint?.trim();
  if (key === "average_order_value") {
    return fromApi || PULSE_AOV_TOOLTIP;
  }
  if (key === "ctr") {
    return fromApi || PULSE_CTR_TOOLTIP;
  }
  if (key !== "drr" && key !== "ad_spend_share_total_revenue") {
    return fromApi || null;
  }
  const base = fromApi || PULSE_AD_SPEND_SHARE_TOOLTIP;
  const spend = shareCalc?.spend;
  const revenue = shareCalc?.revenue;
  if (
    spend == null ||
    revenue == null ||
    !Number.isFinite(spend) ||
    !Number.isFinite(revenue) ||
    spend <= 0 ||
    revenue <= 0
  ) {
    return base;
  }
  const pct = (spend / revenue) * 100;
  const chartPct = new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(Math.round(pct * 10) / 10);
  return `${base}\n\nРасчёт за 7 дней: ${moneyFormatter.format(Math.round(spend))} ₽ ÷ ${moneyFormatter.format(Math.round(revenue))} ₽ = ${chartPct}%.`;
}

export function normalizePulseMetric(
  key: PulseMetricKey,
  kpi: KPICard | undefined,
  present: boolean,
  shareCalc?: { spend?: number | null; revenue?: number | null },
): PulseMetricView {
  const base =
    key === "drr" || key === "ad_spend_share_total_revenue"
      ? resolveDrrState(kpi, present)
      : key === "risks"
        ? resolveRisksState(kpi, present)
        : resolveGrowthMetricState(key, kpi, present);

  const state = base.state;
  return {
    key,
    label: METRIC_LABELS[key],
    ...base,
    hint: resolveMetricHint(key, kpi, shareCalc),
    pillVariant: STATE_PILL[state],
    sparklineStroke: STATE_SPARKLINE[state],
    cardAccentClass: STATE_CARD_ACCENT[state],
    deltaClass: STATE_DELTA_CLASS[state],
    sparklineEmptyLabel: SPARKLINE_EMPTY_LABEL[state],
  };
}

function countActionableMetrics(metrics: PulseMetricView[]): {
  critical: number;
  negative: number;
  warning: number;
  positive: number;
  noData: number;
  unavailable: number;
} {
  return metrics.reduce(
    (acc, metric) => {
      if (metric.state === "critical") acc.critical += 1;
      if (metric.state === "negative") acc.negative += 1;
      if (metric.state === "warning") acc.warning += 1;
      if (metric.state === "positive") acc.positive += 1;
      if (metric.state === "no_data") acc.noData += 1;
      if (metric.state === "unavailable") acc.unavailable += 1;
      return acc;
    },
    { critical: 0, negative: 0, warning: 0, positive: 0, noData: 0, unavailable: 0 },
  );
}

function metricByKey(metrics: PulseMetricView[], key: PulseMetricKey): PulseMetricView | undefined {
  return metrics.find((m) => m.key === key);
}

export function buildPulseSummary(metrics: PulseMetricView[], dashboardStatus?: GlobalStatus): PulseSummaryView {
  const stats = countActionableMetrics(metrics);
  const revenue = metricByKey(metrics, "revenue");
  const orders = metricByKey(metrics, "orders");
  const adShare = metricByKey(metrics, "ad_spend_share_total_revenue");
  const risks = metricByKey(metrics, "risks");

  const signals: PulseSummarySignal[] = [];
  const pushSignal = (metric: PulseMetricView | undefined) => {
    if (!metric) return;
    if (metric.deltaText && (metric.state === "negative" || metric.state === "warning" || metric.state === "critical")) {
      signals.push({ label: `${metric.label}: ${metric.deltaText}`, variant: metric.pillVariant });
    } else if (metric.state === "critical" || metric.state === "warning") {
      signals.push({ label: `${metric.label}: ${metric.statusLabel}`, variant: metric.pillVariant });
    }
  };

  if (dashboardStatus === "not_enough_data" || stats.noData + stats.unavailable >= 4) {
    return withSummaryHighlights({
      headline: "Данных пока недостаточно для полноценной оценки динамики. Блок обновится после синхронизации.",
      tone: "neutral",
      signals: [],
    });
  }

  if (stats.critical > 0) {
    pushSignal(adShare);
    pushSignal(risks);
    if (revenue?.state === "negative") pushSignal(revenue);
    if (orders?.state === "negative") pushSignal(orders);
    return withSummaryHighlights({
      headline: "Есть критичные зоны: проверьте показатели с наибольшим влиянием на результат дня.",
      tone: "critical",
      signals: signals.slice(0, 2),
    });
  }

  const salesPressure = revenue?.state === "negative" || orders?.state === "negative";
  const drrPressure = adShare?.state === "warning" || adShare?.state === "critical";
  const riskPressure = risks?.state === "warning" || risks?.state === "critical";

  if (salesPressure) {
    if (revenue?.state === "negative") pushSignal(revenue);
    if (orders?.state === "negative") pushSignal(orders);
    return withSummaryHighlights({
      headline:
        "Есть ухудшение по продажам: проверьте динамику заказов, воронку и товары с наибольшим вкладом в просадку.",
      tone: "negative",
      signals: signals.slice(0, 2),
    });
  }

  if (drrPressure) {
    pushSignal(adShare);
    return withSummaryHighlights({
      headline:
        "Рекламная эффективность требует внимания: проверьте кампании с высокой долей расходов и расходом без заказов.",
      tone: "warning",
      signals: signals.slice(0, 2),
    });
  }

  if (riskPressure) {
    pushSignal(risks);
    return withSummaryHighlights({
      headline: "Есть товары с повышенным риском: стоит проверить SKU с дефицитом или критичными сигналами.",
      tone: "warning",
      signals: signals.slice(0, 2),
    });
  }

  if (stats.positive >= 2 && stats.negative === 0 && stats.warning === 0) {
    return withSummaryHighlights({
      headline: "Кабинет выглядит стабильно: ключевые показатели без критичных ухудшений.",
      tone: "positive",
      signals: [],
    });
  }

  if (stats.noData >= 2) {
    return withSummaryHighlights({
      headline: "Часть метрик ещё формируется. Доступные показатели можно использовать для первичного контроля.",
      tone: "info",
      signals: [],
    });
  }

  return withSummaryHighlights({
    headline: "Ключевые показатели в пределах ожидаемой динамики. Продолжайте мониторинг продаж и рисков.",
    tone: "neutral",
    signals: [],
  });
}

export function buildPulseKnowledge(metrics: PulseMetricView[], dashboardStatus?: GlobalStatus): PulseKnowledgeItem[] {
  const items: PulseKnowledgeItem[] = [];
  const revenue = metricByKey(metrics, "revenue");
  const orders = metricByKey(metrics, "orders");
  const aov = metricByKey(metrics, "average_order_value");
  const adShare = metricByKey(metrics, "ad_spend_share_total_revenue");
  const risks = metricByKey(metrics, "risks");
  const stats = countActionableMetrics(metrics);

  const push = (text: string) => {
    if (items.length < 4 && !items.some((item) => item.text === text)) {
      items.push({ text });
    }
  };

  if (dashboardStatus === "not_enough_data" || stats.noData + stats.unavailable >= 4) {
    push("Данных пока недостаточно для полноценного анализа динамики.");
    push("Блок обновится после завершения синхронизации метрик.");
    return items;
  }

  if (revenue?.state === "negative") {
    push("Выручка снизилась относительно предыдущего дня — стоит проверить товары с наибольшим вкладом.");
  } else if (revenue?.state === "positive") {
    push("Выручка растёт относительно предыдущего дня.");
  }

  if (orders?.state === "negative") {
    push("Заказов стало меньше — имеет смысл проверить спрос и воронку продаж.");
  } else if (orders?.state === "positive") {
    push("Количество заказов увеличилось.");
  }

  if (aov?.state === "negative") {
    push("Средний чек снизился — проверьте структуру заказов и цены.");
  } else if (aov?.state === "positive") {
    push("Средний чек вырос относительно предыдущего дня.");
  }

  if (adShare?.state === "warning" || adShare?.state === "critical") {
    push("Доля рекламы в общей выручке повышена — проверьте кампании и расход без заказов.");
  } else if (adShare?.state === "positive") {
    push("Рекламная нагрузка на общую выручку в комфортном диапазоне.");
  } else if (adShare?.state === "no_data") {
    push("Рекламные данные пока недоступны для оценки доли расходов в выручке.");
  }

  if (risks?.state === "warning" || risks?.state === "critical") {
    push("Есть открытые риски высокой важности — стоит разобрать приоритетные сигналы.");
  } else if (risks?.state === "positive") {
    push("Критичных рисков по открытым сигналам не обнаружено.");
  }

  if (items.length === 0) {
    push("Критичных ухудшений по доступным метрикам не видно.");
  }

  if (stats.noData > 0 || stats.unavailable > 0) {
    push("Часть метрик ограничена по данным — выводы основаны только на доступном срезе.");
  }

  return items.slice(0, 4);
}

export function buildPulseQuickActions(
  metrics: PulseMetricView[],
  dashboardStatus?: GlobalStatus,
  recommendedActionsCount = 0,
): PulseQuickAction[] {
  const actions: PulseQuickAction[] = [];
  const stats = countActionableMetrics(metrics);

  const push = (label: string, href?: string) => {
    if (actions.length < 4 && !actions.some((action) => action.label === label)) {
      actions.push({ label, href });
    }
  };

  if (dashboardStatus === "not_enough_data" || stats.noData + stats.unavailable >= 4) {
    push("Дождаться завершения синхронизации");
    push("Обновить главную после загрузки данных", "/app/dashboard");
    return actions;
  }

  if (recommendedActionsCount > 0) {
    push("Открыть действия", "/app/actions");
  }

  if (actions.length === 0) {
    push("Открыть действия", "/app/actions");
  }

  return actions.slice(0, 4);
}

export function mapNarrativeTone(tone?: string): PulseSummaryTone {
  switch ((tone ?? "").trim().toLowerCase()) {
    case "stable":
      return "positive";
    case "attention":
      return "warning";
    case "critical":
      return "critical";
    case "neutral":
      return "neutral";
    default:
      return "neutral";
  }
}

function mapChipVariant(tone?: string): StatusPillVariant {
  switch ((tone ?? "").trim().toLowerCase()) {
    case "positive":
      return "success";
    case "warning":
      return "warning";
    case "critical":
      return "danger";
    default:
      return "neutral";
  }
}

export function mapNarrativeSummary(narrative: SellerPulseNarrative): PulseSummaryView {
  const signals = asArray(narrative.chips).map((chip) => ({
    label: chip.label,
    variant: mapChipVariant(chip.tone),
  }));
  const summary = narrative.summary?.trim();
  return withSummaryHighlights({
    headline: narrative.headline,
    summary: summary || undefined,
    tone: mapNarrativeStatusToSummaryTone(narrative.status, narrative.tone),
    signals: signals.slice(0, 3),
  });
}

export function mapNarrativeKeyPoints(narrative: SellerPulseNarrative): PulseKnowledgeItem[] {
  return asArray(narrative.key_points)
    .map((note) => ({
      title: note.title?.trim() || undefined,
      text: note.text.trim(),
      tone: note.tone?.trim() || undefined,
    }))
    .filter((note) => note.text.length > 0)
    .slice(0, 5);
}

export function mapNarrativeKnowledge(narrative: SellerPulseNarrative): PulseKnowledgeItem[] {
  return asArray(narrative.important_notes)
    .map((note) => ({
      title: note.title?.trim() || undefined,
      text: note.text.trim(),
      tone: note.tone?.trim() || undefined,
    }))
    .filter((note) => note.text.length > 0)
    .slice(0, 5);
}

export function mapNarrativeQuickActions(narrative: SellerPulseNarrative): PulseQuickAction[] {
  return asArray(narrative.quick_actions)
    .map((action: SellerPulseQuickAction) => {
      const actionId = (action.action_id ?? action.analysis_action_id ?? "").trim();
      const href =
        action.href?.trim() ||
        (actionId ? `/app/actions?action=${encodeURIComponent(actionId)}` : undefined);
      return {
        label: action.label.trim(),
        description: action.description?.trim() || undefined,
        kind: action.kind?.trim() || undefined,
        priority: action.priority?.trim() || undefined,
        href,
        actionId: actionId || undefined,
      };
    })
    .filter((action) => action.label.length > 0)
    .slice(0, 4);
}

function buildFrontendNarrativeFallback(
  metrics: PulseMetricView[],
  input: SellerPulseBuildInput,
): Pick<SellerPulseViewModel, "summary" | "keyPoints" | "knowledge" | "quickActions" | "narrativeStatus" | "dataQualityNote"> {
  return {
    summary: buildPulseSummary(metrics, input.dashboardStatus),
    keyPoints: [],
    knowledge: buildPulseKnowledge(metrics, input.dashboardStatus),
    quickActions: buildPulseQuickActions(metrics, input.dashboardStatus, input.recommendedActionsCount ?? 0),
    narrativeStatus: "fallback",
    dataQualityNote: "Показана базовая сводка по метрикам без AI-интерпретации.",
  };
}

function buildPulseMetrics(
  keys: PulseMetricKey[],
  kpiByKey: Map<string, KPICard>,
  shareCalc?: { spend?: number | null; revenue?: number | null },
): PulseMetricView[] {
  return keys.map((key) => {
    const kpi = resolveKpiForPulseKey(key, kpiByKey);
    return normalizePulseMetric(key, kpi, kpi != null, shareCalc);
  });
}

/** Maps canonical pulse keys onto legacy frozen KPI keys (ctr/drr). */
function resolveKpiForPulseKey(key: PulseMetricKey, kpiByKey: Map<string, KPICard>): KPICard | undefined {
  if (kpiByKey.has(key)) {
    return kpiByKey.get(key);
  }
  if (key === "average_order_value") {
    return kpiByKey.get("aov");
  }
  if (key === "ad_spend_share_total_revenue") {
    return kpiByKey.get("drr");
  }
  return undefined;
}

export function buildPulseViewModel(input: SellerPulseBuildInput): SellerPulseViewModel {
  const kpis = asArray(input.data?.kpis);
  const kpiByKey = new Map(kpis.map((kpi) => [kpi.key, kpi]));
  const shareCalc = {
    spend: input.adSpend7d,
    revenue: input.storeRevenue7d,
  };
  const displayMetrics = buildPulseMetrics(METRIC_CARD_ORDER, kpiByKey, shareCalc);
  const narrative = input.data?.narrative;

  if (narrative?.headline) {
    return {
      metrics: displayMetrics,
      summary: mapNarrativeSummary(narrative),
      keyPoints: mapNarrativeKeyPoints(narrative),
      knowledge: mapNarrativeKnowledge(narrative),
      quickActions: mapNarrativeQuickActions(narrative),
      narrativeStatus: narrative.status,
      dataQualityNote: narrative.data_quality_note?.trim() || undefined,
    };
  }

  const missingNarrativeSummary: PulseSummaryView = {
    headline: "AI-вывод для блока появится после генерации.",
    tone: "info",
    signals: [],
    highlights: [],
  };
  return {
    metrics: displayMetrics,
    summary: missingNarrativeSummary,
    keyPoints: [],
    knowledge: [],
    quickActions: [],
    narrativeStatus: "source_not_available",
  };
}
