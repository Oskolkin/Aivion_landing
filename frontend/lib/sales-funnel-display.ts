import type { FunnelStage, WidgetStatus } from "@/lib/dashboard-home-types";
import {
  formatCompactStageCountDelta,
  formatSignedPercent,
  formatSignedPp,
  formatStageCountDelta,
} from "@/lib/funnel-stage-copy";

/** Design constant: минимальная ширина нижнего сегмента (%). Плавное сужение воронки. */
export const FUNNEL_MIN_STAGE_WIDTH_PERCENT = 58;
const MIN_STAGE_WIDTH_PERCENT = FUNNEL_MIN_STAGE_WIDTH_PERCENT;
const MAX_STAGE_WIDTH_PERCENT = 100;

const LEGACY_STAGE_KEY_ALIASES: Record<string, string> = {
  hits_view: "impressions",
  hits_view_pdp: "product_views",
  hits_tocart: "add_to_cart",
  ordered_units: "orders",
};

/** Короткая подпись «от …» по ключу предыдущего этапа. */
const CONVERSION_GENITIVE_SHORT: Record<string, string> = {
  impressions: "показов",
  product_views: "карточек",
  add_to_cart: "корзины",
  orders: "заказанных",
  delivered_units: "доставленных",
};

/** Полная формулировка для title/tooltip. */
const CONVERSION_FROM_PREVIOUS_TITLE: Record<string, string> = {
  product_views: "от показов",
  add_to_cart: "от просмотров карточки",
  orders: "от добавлений в корзину",
  delivered_units: "от заказанных единиц",
};

/** @deprecated Use stage-specific count delta titles from formatStageCountDelta. */
export const VOLUME_DELTA_FULL_LABEL = "Изменение количества событий этапа к предыдущему периоду";

export type FunnelStageDisplay = {
  key: string;
  label: string;
  value: number;
  valueLabel: string;
  widthPercent: number;
  /** Процент перехода с предыдущего этапа (или «н/д»). */
  conversionLabel: string;
  /** Короткая подпись, например «от карточек»; у первого этапа null. */
  conversionCaption: string | null;
  /** Подсказка с полной формулировкой перехода. */
  conversionTitle: string | null;
  previousDayValueLabel: string | null;
  previousDayChangeLabel: string | null;
  /** Full sentence for accessibility. */
  previousDayChangeText: string;
  /** Primary badge line: ±N% / Без изменений / Нет данных. */
  previousDayChangePrimary: string;
  /** Secondary badge line: event + period; null when unavailable. */
  previousDayChangeSecondary: string | null;
  /** @deprecated Prefer previousDayChangePrimary (+ secondary). Kept for older tests. */
  previousDayChangeCompactText: string;
  previousDayChangeTone: "positive" | "negative" | "neutral" | "unavailable";
  /** Самый сильный отрицательный delta среди этапов (для лёгкого визуального акцента на бейдже). */
  deltaBadgeEmphasis: "normal" | "strong";
  accessibleSummary: string;
  /** Фон сегмента воронки: спокойная единая палитра без «радуги». */
  surfaceClassName: string;
  iconName: FunnelStageIconName;
};

const STAGE_LABELS: Record<string, string> = {
  impressions: "Увидели товар",
  product_views: "Открыли карточку",
  add_to_cart: "Добавили в корзину",
  orders: "Заказанные единицы",
  delivered_units: "Доставленные единицы",
};

/** Короткие подписи для header chain и переходов. */
const STAGE_COMPACT_LABELS: Record<string, string> = {
  impressions: "Показы",
  product_views: "карточка",
  add_to_cart: "корзина",
  orders: "заказанные",
  delivered_units: "доставленные",
};

/** Длиннее для header chain на последних этапах стандартной воронки. */
const STAGE_HEADER_CHAIN_LABELS: Record<string, string> = {
  impressions: "Показы",
  product_views: "карточка",
  add_to_cart: "корзина",
  orders: "заказанные единицы",
  delivered_units: "доставленные единицы",
};

export type FunnelStageIconName = "eye" | "file-text" | "shopping-cart" | "package" | "truck" | "circle";

const LEGACY_LABEL_REPLACEMENTS: Array<{ rx: RegExp; value: string }> = [
  { rx: /выкуп(?:ы|или|ленн[а-я]*)?/gi, value: "Доставленные единицы" },
  { rx: /выкупили товар/gi, value: "Доставленные единицы" },
];

function canonicalStageKey(stageKey: string): string {
  const key = stageKey.trim().toLowerCase();
  return LEGACY_STAGE_KEY_ALIASES[key] ?? key;
}

function humanizeUnknownStageKey(stageKey: string): string {
  const cleaned = stageKey.trim().replace(/_/g, " ");
  if (!cleaned) return "Этап";
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
}

export function normalizeFunnelStageLabel(stageKey: string, fallbackLabel?: string): string {
  const canonical = canonicalStageKey(stageKey);
  const mapped = STAGE_LABELS[canonical];
  if (mapped) return mapped;

  let label = (fallbackLabel ?? "").trim();
  for (const repl of LEGACY_LABEL_REPLACEMENTS) {
    label = label.replace(repl.rx, repl.value);
  }
  if (label) return label;

  if (stageKey.trim()) return humanizeUnknownStageKey(stageKey);
  return "Этап";
}

/** Сверху вниз: мягкий blue → teal, один визуальный ряд без «радуги». */
const STAGE_SURFACES = [
  "bg-[#6B93F5] text-white ring-1 ring-inset ring-white/20",
  "bg-[#7BA3F0] text-white ring-1 ring-inset ring-white/20",
  "bg-[#6BB8E8] text-white ring-1 ring-inset ring-white/20",
  "bg-[#55C4D4] text-white ring-1 ring-inset ring-white/20",
  "bg-[#4EC9B8] text-white ring-1 ring-inset ring-white/20",
];

export const FUNNEL_WEAK_STAGE_SURFACE =
  "bg-rose-50/95 text-slate-800 ring-2 ring-inset ring-rose-300 shadow-[0_1px_4px_rgba(244,63,94,0.08)]";

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function isFiniteNumber(value: number | null | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export function formatFunnelNumber(value: number | null | undefined): string {
  if (!isFiniteNumber(value)) {
    return "—";
  }
  return new Intl.NumberFormat("ru-RU", {
    maximumFractionDigits: 0,
  }).format(value);
}

export function formatFunnelPercent(value: number | null | undefined): string {
  if (!isFiniteNumber(value)) {
    return "—";
  }
  return new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(value) + "%";
}

export function formatFunnelDeltaPct(value: number | null | undefined): string {
  if (!isFiniteNumber(value)) {
    return "—";
  }
  return formatSignedPercent(value);
}

/** Percentage-point delta for conversion changes (never use `%` here). */
export function formatFunnelPpDelta(value: number | null | undefined): string | null {
  if (!isFiniteNumber(value)) {
    return null;
  }
  return formatSignedPp(value);
}

export type FunnelDeltaTextOptions = {
  /** Compact badge: just ±N% (or «нет сравнения» / «без изменений»). */
  compact?: boolean;
  /** @deprecated Period hint is now stage-aware via formatStageCountDelta. */
  withPeriodHint?: boolean;
  /** Stage key for event-specific wording. Required for meaningful labels. */
  stageKey?: string;
  periodType?: string | null;
  comparisonLabel?: string | null;
};

/**
 * @deprecated Prefer formatStageCountDelta / formatCompactStageCountDelta.
 * Kept for transitional call sites; never emits «по объёму».
 */
export function formatFunnelDeltaText(
  value: number | null | undefined,
  compactOrOptions: boolean | FunnelDeltaTextOptions = false,
): string {
  const options: FunnelDeltaTextOptions =
    typeof compactOrOptions === "boolean" ? { compact: compactOrOptions } : compactOrOptions;
  const stageKey = options.stageKey ?? "product_views";
  const labels = options.compact
    ? formatCompactStageCountDelta({
        stageKey,
        deltaPercent: value,
        periodType: options.periodType,
        comparisonLabel: options.comparisonLabel,
      })
    : formatStageCountDelta({
        stageKey,
        deltaPercent: value,
        periodType: options.periodType,
        comparisonLabel: options.comparisonLabel,
      });

  if (options.compact) {
    if (!labels.secondary) return labels.primary.toLowerCase() === "нет данных для сравнения" ? "нет сравнения" : labels.primary;
    if (labels.primary === "Без изменений") return "без изменений";
    return options.withPeriodHint ? `${labels.primary} ${labels.secondary}` : labels.primary;
  }
  return labels.full;
}

export function getStageDisplayLabel(stageKey: string, fallbackLabel?: string): string {
  return normalizeFunnelStageLabel(stageKey, fallbackLabel);
}

export function getFunnelStageCompactLabel(stageKey: string, fallbackLabel?: string): string {
  const canonical = canonicalStageKey(stageKey);
  if (STAGE_COMPACT_LABELS[canonical]) return STAGE_COMPACT_LABELS[canonical];
  const humanized = normalizeFunnelStageLabel(stageKey, fallbackLabel);
  if (humanized.length <= 24) return humanized.toLowerCase();
  return humanizeUnknownStageKey(stageKey).toLowerCase();
}

export function getFunnelStageHeaderChainLabel(stageKey: string, fallbackLabel?: string): string {
  const canonical = canonicalStageKey(stageKey);
  if (STAGE_HEADER_CHAIN_LABELS[canonical]) return STAGE_HEADER_CHAIN_LABELS[canonical];
  return getFunnelStageCompactLabel(stageKey, fallbackLabel);
}

export function buildFunnelStageCompactChainSubtitle(stages: FunnelStage[]): string {
  if (stages.length === 0) {
    return "Этапы воронки появятся, когда будут доступны данные.";
  }
  return stages.map((stage) => getFunnelStageHeaderChainLabel(stage.key, stage.label)).join(" → ");
}

export function getFunnelStageIconName(stageKey: string): FunnelStageIconName {
  switch (canonicalStageKey(stageKey)) {
    case "impressions":
      return "eye";
    case "product_views":
      return "file-text";
    case "add_to_cart":
      return "shopping-cart";
    case "orders":
      return "package";
    case "delivered_units":
      return "truck";
    default:
      return "circle";
  }
}

export function getFixedFunnelStageWidth(index: number, totalStages: number): number {
  if (!Number.isFinite(index) || index < 0) {
    return MIN_STAGE_WIDTH_PERCENT;
  }
  if (totalStages <= 0) {
    return MAX_STAGE_WIDTH_PERCENT;
  }
  if (totalStages <= 1 || index <= 0) {
    return MAX_STAGE_WIDTH_PERCENT;
  }
  const width =
    MAX_STAGE_WIDTH_PERCENT -
    ((MAX_STAGE_WIDTH_PERCENT - MIN_STAGE_WIDTH_PERCENT) * index) / (totalStages - 1);
  return clamp(Math.round(width * 10) / 10, MIN_STAGE_WIDTH_PERCENT, MAX_STAGE_WIDTH_PERCENT);
}

export function resolveConversionFromPreviousPct(
  stage: FunnelStage,
  index: number,
  stages: FunnelStage[],
): number | null {
  if (index === 0) {
    return 100;
  }
  if (isFiniteNumber(stage.conversion_from_previous)) {
    return stage.conversion_from_previous;
  }
  const prev = stages[index - 1];
  const prevVal = isFiniteNumber(prev?.value) ? prev.value : null;
  const curVal = isFiniteNumber(stage.value) ? stage.value : null;
  if (prevVal === null || curVal === null || prevVal <= 0) {
    return null;
  }
  return Math.round((curVal / prevVal) * 10000) / 100;
}

export function getConversionFromPreviousCaption(previousStageKey: string, previousStageLabel?: string): string {
  const canonical = canonicalStageKey(previousStageKey);
  const short = CONVERSION_GENITIVE_SHORT[canonical];
  if (short) return `от ${short}`;
  const label = normalizeFunnelStageLabel(previousStageKey, previousStageLabel);
  if (label && label !== "Этап") return `от «${label.toLowerCase()}»`;
  return "от предыдущего этапа";
}

export function getFunnelDeltaTone(value: number | null | undefined): "positive" | "negative" | "neutral" | "unavailable" {
  if (!isFiniteNumber(value)) {
    return "unavailable";
  }
  if (value > 0) {
    return "positive";
  }
  if (value < 0) {
    return "negative";
  }
  return "neutral";
}

export function buildSalesFunnelDisplay(
  status: WidgetStatus | string | undefined,
  stages: FunnelStage[],
): FunnelStageDisplay[] {
  if (status && status !== "available") {
    return [];
  }
  if (stages.length === 0) {
    return [];
  }

  let worstNegativeIndex = -1;
  let worstNegativeValue = 0;
  for (let i = 0; i < stages.length; i++) {
    const v = stages[i]?.change_vs_previous_day_pct;
    if (isFiniteNumber(v) && v < 0) {
      if (worstNegativeIndex < 0 || v < worstNegativeValue) {
        worstNegativeValue = v;
        worstNegativeIndex = i;
      }
    }
  }

  return stages.map((stage, index) => {
    const value = isFiniteNumber(stage.value) ? stage.value : 0;
    const label = getStageDisplayLabel(stage.key, stage.label);
    const conversionPct = resolveConversionFromPreviousPct(stage, index, stages);
    const prevStage = index > 0 ? stages[index - 1] : undefined;
    const conversionCaption =
      index === 0 || conversionPct === null
        ? null
        : getConversionFromPreviousCaption(prevStage?.key ?? "", prevStage?.label);
    const conversionLabel = conversionPct !== null ? formatFunnelPercent(conversionPct) : "н/д";
    const conversionTitle =
      index === 0
        ? null
        : conversionPct !== null
          ? `${conversionLabel} ${CONVERSION_FROM_PREVIOUS_TITLE[stage.key] ?? conversionCaption ?? "от предыдущего этапа"}`
          : "Недостаточно данных для расчёта перехода от предыдущего этапа.";
    const previousDayValueLabel = isFiniteNumber(stage.previous_day_value) ? formatFunnelNumber(stage.previous_day_value) : null;
    const countDelta = formatCompactStageCountDelta({
      stageKey: stage.key,
      deltaPercent: stage.change_vs_previous_day_pct,
      periodType: stage.period_type,
    });
    const previousDayChangeLabel = isFiniteNumber(stage.change_vs_previous_day_pct)
      ? formatFunnelDeltaPct(stage.change_vs_previous_day_pct)
      : null;
    const previousDayChangeText = countDelta.full;
    const previousDayChangePrimary = countDelta.primary;
    const previousDayChangeSecondary = countDelta.secondary;
    const previousDayChangeCompactText = countDelta.secondary
      ? `${countDelta.primary} ${countDelta.secondary}`
      : countDelta.primary;
    const conversionSummary =
      index === 0
        ? conversionLabel
        : conversionCaption
          ? `${conversionLabel} ${conversionCaption}`
          : conversionLabel;
    const parts = [label, `${formatFunnelNumber(value)} значений`, conversionSummary, previousDayChangeText];

    return {
      key: stage.key,
      label,
      value,
      valueLabel: formatFunnelNumber(value),
      widthPercent: getFixedFunnelStageWidth(index, stages.length),
      conversionLabel,
      conversionCaption,
      conversionTitle,
      previousDayValueLabel,
      previousDayChangeLabel,
      previousDayChangeText,
      previousDayChangePrimary,
      previousDayChangeSecondary,
      previousDayChangeCompactText,
      previousDayChangeTone: getFunnelDeltaTone(stage.change_vs_previous_day_pct),
      deltaBadgeEmphasis:
        index === worstNegativeIndex &&
        worstNegativeIndex >= 0 &&
        getFunnelDeltaTone(stage.change_vs_previous_day_pct) === "negative"
          ? "strong"
          : "normal",
      accessibleSummary: parts.join(", "),
      surfaceClassName: STAGE_SURFACES[index % STAGE_SURFACES.length],
      iconName: getFunnelStageIconName(stage.key),
    };
  });
}
