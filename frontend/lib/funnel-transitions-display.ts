import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import {
  formatConversionCurrentCaption,
  formatConversionDeltaLabels,
  formatSignedPp,
  formatStageCountDelta,
} from "@/lib/funnel-stage-copy";
import {
  buildFunnelStageCompactChainSubtitle,
  formatFunnelNumber,
  formatFunnelPercent,
  getFunnelStageCompactLabel,
  getStageDisplayLabel,
  normalizeFunnelStageLabel,
  resolveConversionFromPreviousPct,
} from "@/lib/sales-funnel-display";

export type FunnelTransitionTone = "negative" | "warning" | "positive" | "neutral" | "unavailable";

export type FunnelTransitionHighlightKind = "weak_spot" | "bottleneck" | null;

export type FunnelTransitionDisplay = {
  key: string;
  /** Полная подпись для accessibility. */
  label: string;
  /** Компактная подпись для UI. */
  compactLabel: string;
  /** Подпись конверсии для этого перехода (напр. «Конверсия в карточку»). */
  conversionCaption: string;
  fromStageKey: string;
  toStageKey: string;
  fromValue: number | null;
  toValue: number | null;
  valuesSubline: string;
  conversionPct: number | null;
  conversionLabel: string;
  volumeDeltaPct: number | null;
  /** Primary line for count delta (±N% / Без изменений). */
  volumeDeltaPrimary: string | null;
  /** Secondary line: event + period. */
  volumeDeltaSecondary: string | null;
  /** Full seller sentence for count delta. */
  volumeDeltaLabel: string;
  volumeDeltaCompactLabel: string;
  volumeDeltaFullLabel: string;
  conversionDeltaPp: number | null;
  conversionDeltaLabel: string | null;
  conversionDeltaSecondary: string | null;
  progressWidthPercent: number | null;
  tone: FunnelTransitionTone;
  highlightKind: FunnelTransitionHighlightKind;
};

/** Explicit conversion captions for transitions whose generic "from → to" label reads awkwardly. */
const TRANSITION_CONVERSION_CAPTION: Record<string, string> = {
  add_to_cart_to_orders: "Конверсия из корзины в заказ",
  orders_to_delivered: "Доля доставленных единиц от заказанных",
};

function buildConversionCaption(canonicalKey: string, toStageKey: string): string {
  return TRANSITION_CONVERSION_CAPTION[canonicalKey] ?? formatConversionCurrentCaption(toStageKey);
}

const TRANSITION_KEY_ALIASES: Record<string, string> = {
  impressions_to_product_views: "impressions_to_product_views",
  impressions_to_views: "impressions_to_product_views",
  views_to_cart: "product_views_to_add_to_cart",
  product_views_to_add_to_cart: "product_views_to_add_to_cart",
  cart_to_order: "add_to_cart_to_orders",
  add_to_cart_to_orders: "add_to_cart_to_orders",
  orders_to_delivered_units: "orders_to_delivered",
  orders_to_delivered: "orders_to_delivered",
};

function isFiniteNumber(value: number | null | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function cleanText(value?: string | null): string {
  return (value ?? "").trim();
}

/** Normalizes transition/stage labels for fuzzy matching. */
export function normalizeTransitionText(value: string): string {
  return value
    .toLowerCase()
    .replace(/[→➔➜>]/g, " ")
    .replace(/[.,!?;:()[\]{}"'`«»\-—–/\\]/g, " ")
    .replace(/\s+/g, " ")
    .replace(/карточки/g, "карточка")
    .replace(/открыли карточку/g, "карточка")
    .replace(/добавления в корзину/g, "корзина")
    .replace(/добавили в корзину/g, "корзина")
    .replace(/\bзаказанные единицы\b/g, "заказы")
    .replace(/\bзаказали товар\b/g, "заказы")
    .replace(/\bдоставленные единицы\b/g, "доставка")
    .replace(/\bувидели товар\b/g, "показы")
    .replace(/\bоткрыли карточку\b/g, "карточка")
    .trim();
}

function canonicalTransitionKey(key: string): string {
  const k = cleanText(key).toLowerCase();
  return TRANSITION_KEY_ALIASES[k] ?? k;
}

function buildTransitionKey(fromKey: string, toKey: string): string {
  return `${fromKey}_to_${toKey}`;
}

function buildTransitionLabel(fromStage: FunnelStage, toStage: FunnelStage): string {
  const fromLabel = getStageDisplayLabel(fromStage.key, fromStage.label);
  const toLabel = getStageDisplayLabel(toStage.key, toStage.label);
  return `${fromLabel} → ${toLabel}`;
}

function buildTransitionCompactLabel(fromStage: FunnelStage, toStage: FunnelStage): string {
  const fromCanonical = canonicalStageKey(fromStage.key);
  const fromCompact = getFunnelStageCompactLabel(fromStage.key, fromStage.label);
  const toCompact = getFunnelStageCompactLabel(toStage.key, toStage.label);
  const fromLabel =
    fromCanonical === "impressions"
      ? fromCompact
      : fromCompact.charAt(0).toUpperCase() + fromCompact.slice(1);
  return `${fromLabel} → ${toCompact}`;
}

function canonicalStageKey(stageKey: string): string {
  const key = stageKey.trim().toLowerCase();
  const LEGACY: Record<string, string> = {
    hits_view: "impressions",
    hits_view_pdp: "product_views",
    hits_tocart: "add_to_cart",
    ordered_units: "orders",
    delivered_units: "delivered_units",
  };
  return LEGACY[key] ?? key;
}

function parseSignedNumber(raw?: string | null): number | null {
  const src = cleanText(raw).replace(",", ".");
  const m = src.match(/([+-−]?\d+(?:\.\d+)?)/);
  if (!m) return null;
  const n = parseFloat(m[1].replace("−", "-"));
  return Number.isFinite(n) ? n : null;
}

function formatPpDelta(value: number): string {
  return formatSignedPp(value);
}

function pickConversionDeltaPp(
  transitionKey: string,
  transitionLabel: string,
  conversionMetrics?: NonNullable<FunnelAIAnalysis["comparisons"]>["conversion_metrics"],
): number | null {
  if (!conversionMetrics?.length) return null;
  const normalizedLabel = normalizeTransitionText(transitionLabel);
  const canonicalKey = canonicalTransitionKey(transitionKey);

  for (const item of conversionMetrics) {
    const itemKey = canonicalTransitionKey(cleanText(item.key));
    const itemLabel = normalizeTransitionText(cleanText(item.label));
    const matchesKey = itemKey && (itemKey === canonicalKey || itemKey === transitionKey);
    const matchesLabel =
      itemLabel &&
      normalizedLabel &&
      (itemLabel === normalizedLabel || normalizedLabel.includes(itemLabel) || itemLabel.includes(normalizedLabel));
    if (!matchesKey && !matchesLabel) continue;

    const delta = cleanText(item.delta);
    if (!delta) continue;
    if (/п\.п\./i.test(delta)) {
      const n = parseSignedNumber(delta);
      if (n != null) return n;
    }
  }
  return null;
}

/** Prefers explicit stage-level conversion_delta_pp / previous_conversion_from_previous over AI-derived metrics. */
function resolveStageConversionDeltaPp(toStage: FunnelStage): number | null {
  if (isFiniteNumber(toStage.conversion_delta_pp)) {
    return toStage.conversion_delta_pp;
  }
  if (
    isFiniteNumber(toStage.conversion_from_previous) &&
    isFiniteNumber(toStage.previous_conversion_from_previous)
  ) {
    const delta = toStage.conversion_from_previous - toStage.previous_conversion_from_previous;
    return Math.round(delta * 10) / 10;
  }
  return null;
}

function resolveTransitionTone(
  conversionDeltaPp: number | null,
  volumeDeltaPct: number | null,
): FunnelTransitionTone {
  if (conversionDeltaPp != null) {
    if (conversionDeltaPp <= -5) return "negative";
    if (conversionDeltaPp < 0) return "warning";
    if (conversionDeltaPp > 0) return "positive";
    return "neutral";
  }
  if (volumeDeltaPct != null) {
    if (volumeDeltaPct <= -15) return "negative";
    if (volumeDeltaPct < 0) return "warning";
    if (volumeDeltaPct > 0) return "positive";
    return "neutral";
  }
  return "unavailable";
}

function hasNegativeSignal(transition: FunnelTransitionDisplay): boolean {
  return (
    (transition.conversionDeltaPp != null && transition.conversionDeltaPp < 0) ||
    (transition.volumeDeltaPct != null && transition.volumeDeltaPct < 0)
  );
}

function applyTransitionHighlights(
  transitions: FunnelTransitionDisplay[],
  highlight: { key: string; kind: Exclude<FunnelTransitionHighlightKind, null> } | null,
): FunnelTransitionDisplay[] {
  return transitions.map((transition) => ({
    ...transition,
    highlightKind: highlight != null && transition.key === highlight.key ? highlight.kind : null,
  }));
}

function valuesSubline(fromValue: number | null, toValue: number | null): string {
  const from = fromValue != null ? formatFunnelNumber(fromValue) : "—";
  const to = toValue != null ? formatFunnelNumber(toValue) : "—";
  return `${from} → ${to}`;
}

export function buildFunnelTransitions(
  stages: FunnelStage[],
  funnelAnalysis?: FunnelAIAnalysis | null,
  options?: { comparisonMode?: "default" | "day_over_day" },
): FunnelTransitionDisplay[] {
  if (stages.length < 2) return [];

  const conversionMetrics = funnelAnalysis?.comparisons?.conversion_metrics;
  const transitions: FunnelTransitionDisplay[] = [];

  for (let index = 1; index < stages.length; index++) {
    const fromStage = stages[index - 1];
    const toStage = stages[index];
    const fromValue = isFiniteNumber(fromStage.value) ? fromStage.value : null;
    const toValue = isFiniteNumber(toStage.value) ? toStage.value : null;
    const key = buildTransitionKey(fromStage.key, toStage.key);
    const label = buildTransitionLabel(fromStage, toStage);
    const compactLabel = buildTransitionCompactLabel(fromStage, toStage);
    const conversionCaption = buildConversionCaption(canonicalTransitionKey(key), toStage.key);
    const conversionPct = resolveConversionFromPreviousPct(toStage, index, stages);
    const volumeDeltaPct = isFiniteNumber(toStage.change_vs_previous_day_pct)
      ? toStage.change_vs_previous_day_pct
      : null;
    const conversionDeltaPp =
      resolveStageConversionDeltaPp(toStage) ??
      (options?.comparisonMode === "day_over_day" ? null : pickConversionDeltaPp(key, label, conversionMetrics));
    const countDelta = formatStageCountDelta({
      stageKey: toStage.key,
      deltaPercent: volumeDeltaPct,
      periodType: toStage.period_type,
    });
    const conversionDelta = formatConversionDeltaLabels({
      conversionDeltaPp,
      periodType: toStage.period_type,
    });
    const progressWidthPercent =
      conversionPct != null ? clamp(Math.round(conversionPct * 10) / 10, 0, 100) : null;

    transitions.push({
      key,
      label,
      compactLabel,
      conversionCaption,
      fromStageKey: fromStage.key,
      toStageKey: toStage.key,
      fromValue,
      toValue,
      valuesSubline: valuesSubline(fromValue, toValue),
      conversionPct,
      conversionLabel: conversionPct != null ? formatFunnelPercent(conversionPct) : "н/д",
      volumeDeltaPct,
      volumeDeltaPrimary: volumeDeltaPct != null ? countDelta.primary : null,
      volumeDeltaSecondary: volumeDeltaPct != null ? countDelta.secondary : null,
      volumeDeltaLabel: volumeDeltaPct != null ? countDelta.full : "Нет данных для сравнения",
      volumeDeltaCompactLabel:
        volumeDeltaPct != null
          ? countDelta.secondary
            ? `${countDelta.primary} ${countDelta.secondary}`
            : countDelta.primary
          : "нет сравнения",
      volumeDeltaFullLabel: countDelta.title,
      conversionDeltaPp,
      conversionDeltaLabel: conversionDelta.primary ?? (conversionDeltaPp != null ? formatPpDelta(conversionDeltaPp) : null),
      conversionDeltaSecondary: conversionDelta.secondary,
      progressWidthPercent,
      tone: resolveTransitionTone(conversionDeltaPp, volumeDeltaPct),
      highlightKind: null,
    });
  }

  return applyTransitionHighlights(transitions, detectHighlightedTransition(transitions, funnelAnalysis));
}

export function markWeakestTransition(
  transitions: FunnelTransitionDisplay[],
  funnelAnalysis?: FunnelAIAnalysis | null,
  primaryDropStage?: string,
  stages?: FunnelStage[],
): FunnelTransitionDisplay[] {
  const built =
    transitions.length > 0
      ? transitions
      : stages && stages.length > 1
        ? buildFunnelTransitions(stages, funnelAnalysis)
        : [];
  const highlight = detectHighlightedTransition(built, funnelAnalysis, primaryDropStage);
  return applyTransitionHighlights(built, highlight);
}

function transitionMatchesReference(
  transition: FunnelTransitionDisplay,
  referenceKey?: string,
  referenceLabel?: string,
): boolean {
  const refKey = cleanText(referenceKey);
  const refLabel = cleanText(referenceLabel);

  if (refKey) {
    const canonicalRef = canonicalTransitionKey(refKey);
    if (canonicalRef === canonicalTransitionKey(transition.key)) return true;
    if (refKey === transition.toStageKey || refKey === transition.fromStageKey) return true;
    if (refKey.includes("_to_") && canonicalTransitionKey(refKey) === canonicalTransitionKey(transition.key)) {
      return true;
    }
  }

  if (refLabel) {
    const normalizedRef = normalizeTransitionText(refLabel);
    const normalizedTransition = normalizeTransitionText(transition.label);
    if (normalizedRef && normalizedTransition) {
      if (normalizedRef === normalizedTransition) return true;
      if (normalizedTransition.includes(normalizedRef) || normalizedRef.includes(normalizedTransition)) return true;
    }
    const normalizedStage = normalizeTransitionText(normalizeFunnelStageLabel("", refLabel));
    if (normalizedStage && normalizedTransition.includes(normalizedStage)) return true;
  }

  return false;
}

export function detectHighlightedTransition(
  transitions: FunnelTransitionDisplay[],
  funnelAnalysis?: FunnelAIAnalysis | null,
  primaryDropStage?: string,
): { key: string; kind: Exclude<FunnelTransitionHighlightKind, null> } | null {
  if (transitions.length === 0) return null;

  const ws = funnelAnalysis?.weakest_stage;
  if (cleanText(ws?.key)) {
    const match = transitions.find((t) => transitionMatchesReference(t, ws?.key, ws?.label));
    if (match && hasNegativeSignal(match)) return { key: match.key, kind: "weak_spot" };
  }
  if (cleanText(ws?.label)) {
    const match = transitions.find((t) => transitionMatchesReference(t, undefined, ws?.label));
    if (match && hasNegativeSignal(match)) return { key: match.key, kind: "weak_spot" };
  }

  if (cleanText(primaryDropStage)) {
    const match = transitions.find((t) => transitionMatchesReference(t, undefined, primaryDropStage));
    if (match && hasNegativeSignal(match)) return { key: match.key, kind: "weak_spot" };
  }

  // Fallback: prefer the most negative conversion Δ in percentage points.
  // Do not promote a weak_spot from volume drop alone — traffic can fall across the whole cabinet.
  const withConversionDelta = transitions.filter((t) => t.conversionDeltaPp != null);
  const negativeConversion = withConversionDelta.filter((t) => (t.conversionDeltaPp ?? 0) < 0);
  if (negativeConversion.length > 0) {
    const worst = negativeConversion.reduce((a, b) =>
      (a.conversionDeltaPp ?? 0) < (b.conversionDeltaPp ?? 0) ? a : b,
    );
    return { key: worst.key, kind: "weak_spot" };
  }

  // Without conversion Δpp: pick the narrowest transition. Upgrade to weak_spot only when
  // that same step also carries a negative signal (volume), so we do not flag a healthy narrow step.
  const withConversion = transitions.filter((t) => t.conversionPct != null);
  if (withConversion.length > 0) {
    const narrowest = withConversion.reduce((a, b) =>
      (a.conversionPct ?? Infinity) < (b.conversionPct ?? Infinity) ? a : b,
    );
    if (hasNegativeSignal(narrowest)) {
      return { key: narrowest.key, kind: "weak_spot" };
    }
    return { key: narrowest.key, kind: "bottleneck" };
  }

  return null;
}

/** @deprecated use detectHighlightedTransition */
export function detectWeakestTransition(
  transitions: FunnelTransitionDisplay[],
  funnelAnalysis?: FunnelAIAnalysis | null,
  primaryDropStage?: string,
  stages?: FunnelStage[],
): string | null {
  const built =
    transitions.length > 0
      ? transitions
      : stages && stages.length > 1
        ? buildFunnelTransitions(stages, funnelAnalysis)
        : [];
  return detectHighlightedTransition(built, funnelAnalysis, primaryDropStage)?.key ?? null;
}

const FUNNEL_MAIN_CONCLUSION_MAX_SENTENCES = 7;

function truncateSentences(text: string, maxSentences: number): string {
  const parts = text.split(/(?<=[.!?])\s+/).filter(Boolean);
  if (parts.length <= maxSentences) return text.trim();
  return parts.slice(0, maxSentences).join(" ").trim();
}

function truncateMainConclusion(text: string): string {
  return truncateSentences(text, FUNNEL_MAIN_CONCLUSION_MAX_SENTENCES);
}

/** Этап воронки для подсветки в пирамиде: `toStage` weak transition. */
export function resolveHighlightedStageKey(
  transitions: FunnelTransitionDisplay[],
): { stageKey: string; kind: "weak_spot" } | null {
  const weak = transitions.find((t) => t.highlightKind === "weak_spot");
  if (!weak) return null;
  return { stageKey: weak.toStageKey, kind: "weak_spot" };
}

export type FunnelFallbackDisplay = {
  lead: string;
  detail: string;
  metricChips: string[];
};

/** Разбирает fallback-текст: короткий lead, detail и chips с п.п. */
export function formatFunnelFallbackDisplay(message: string): FunnelFallbackDisplay {
  const src = message.trim();
  if (!src) return { lead: "", detail: "", metricChips: [] };

  const metricChips: string[] = [];
  const withoutMetrics = src.replace(/([−+-]?\d+(?:[.,]\d+)?)\s*п\.п\./gi, (match) => {
    metricChips.push(match.replace(/\s+/g, " ").trim());
    return " ";
  });

  const sentences = withoutMetrics
    .replace(/\s+/g, " ")
    .trim()
    .split(/(?<=[.!?])\s+/)
    .filter(Boolean);

  return {
    lead: sentences[0] ?? withoutMetrics.trim(),
    detail: sentences.slice(1, 2).join(" ").trim(),
    metricChips,
  };
}

export type FunnelMainConclusionSource = "ai" | "ai_repaired" | "fallback" | "empty";

export type FunnelMainConclusionDisplay = {
  text: string;
  source: FunnelMainConclusionSource;
};

const FUNNEL_MAIN_CONCLUSION_EMPTY_TEXT =
  "Пока недостаточно данных, чтобы сформировать краткий вывод по воронке.";

function isFunnelAIAvailable(funnelAnalysis?: FunnelAIAnalysis | null): boolean {
  return cleanText(funnelAnalysis?.status).toLowerCase() === "available";
}

function extractConclusionFromDetailsSections(analysis: FunnelAIAnalysis): string {
  for (const section of analysis.details_sections ?? []) {
    const title = cleanText(section?.title).toLowerCase();
    if (!/итог|вывод|заключение|conclusion|summary|final_summary|result|main_insight/.test(title)) {
      continue;
    }
    for (const item of section?.items ?? []) {
      const text = cleanText(item?.text);
      if (text) return text;
    }
  }
  return "";
}

/** Извлекает AI main_conclusion для «Главного вывода» (без legacy fallback-полей). */
export function extractFunnelAIMainConclusionText(funnelAnalysis: FunnelAIAnalysis): string {
  return cleanText(funnelAnalysis.main_conclusion);
}

/** Извлекает первый непустой AI-текст для «Главного вывода». */
export function extractFunnelAIConclusionText(funnelAnalysis: FunnelAIAnalysis): string {
  const main = extractFunnelAIMainConclusionText(funnelAnalysis);
  if (main) return main;

  const mainInsight = cleanText((funnelAnalysis as FunnelAIAnalysis & { main_insight?: string }).main_insight);
  const candidates = [
    cleanText(funnelAnalysis.weakest_stage?.explanation),
    cleanText(funnelAnalysis.comparisons_commentary?.previous_period),
    cleanText(funnelAnalysis.comparisons_commentary?.previous_summary),
    cleanText(funnelAnalysis.summary),
    cleanText(funnelAnalysis.short_summary),
    extractConclusionFromDetailsSections(funnelAnalysis),
    mainInsight,
  ];
  return candidates.find(Boolean) ?? "";
}

function buildFunnelFallbackConclusion(primaryDropStage?: string, primaryDropMessage?: string): string {
  const dropMessage = cleanText(primaryDropMessage);
  const dropStage = cleanText(primaryDropStage);
  if (dropMessage && dropStage) {
    const stageLabel = normalizeFunnelStageLabel("", dropStage) || dropStage;
    return truncateSentences(`По текущим данным сильнее всего просел этап: ${stageLabel}. ${dropMessage}`, 2);
  }
  if (dropMessage) return truncateSentences(dropMessage, 2);
  return "";
}

export function buildFunnelMainConclusionDisplay(input: {
  funnelAnalysis?: FunnelAIAnalysis | null;
  primaryDropStage?: string;
  primaryDropMessage?: string;
}): FunnelMainConclusionDisplay {
  const { funnelAnalysis, primaryDropStage, primaryDropMessage } = input;

  if (isFunnelAIAvailable(funnelAnalysis) && funnelAnalysis) {
    const mainConclusion = extractFunnelAIMainConclusionText(funnelAnalysis);
    if (mainConclusion) {
      return { text: truncateMainConclusion(mainConclusion), source: "ai" };
    }
    const legacyAiText = extractFunnelAIConclusionText(funnelAnalysis);
    if (legacyAiText) {
      return { text: truncateMainConclusion(legacyAiText), source: "ai" };
    }
  }

  const fallbackText = buildFunnelFallbackConclusion(primaryDropStage, primaryDropMessage);
  if (fallbackText) {
    return { text: fallbackText, source: "fallback" };
  }

  return { text: FUNNEL_MAIN_CONCLUSION_EMPTY_TEXT, source: "empty" };
}

export function buildFunnelMainConclusion(input: {
  funnelAnalysis?: FunnelAIAnalysis | null;
  primaryDropStage?: string;
  primaryDropMessage?: string;
}): string {
  return buildFunnelMainConclusionDisplay(input).text;
}

export function resolveFunnelAIEmptyMessage(input: {
  aiStatus?: string;
  funnelAnalysisStatus?: string;
  message?: string;
  primaryDropStage?: string;
  primaryDropMessage?: string;
}): string {
  const aiStatus = cleanText(input.aiStatus).toLowerCase();
  const funnelStatus = cleanText(input.funnelAnalysisStatus).toLowerCase();
  const message = cleanText(input.message);

  if (aiStatus === "available") return "";

  if (aiStatus === "source_pending" || funnelStatus === "source_pending") {
    return "AI-анализ появится после обновления дневной сводки.";
  }

  if (aiStatus === "not_enough_data" || funnelStatus === "not_enough_data") {
    const dropStage = cleanText(input.primaryDropStage);
    const dropMessage = cleanText(input.primaryDropMessage);
    if (dropMessage && dropStage) {
      const stageLabel = normalizeFunnelStageLabel("", dropStage) || dropStage;
      return `По текущим данным сильнее всего просел этап: ${stageLabel}. ${dropMessage}`;
    }
    if (dropMessage) return dropMessage;
    return "Недостаточно данных, чтобы надёжно определить слабое место воронки.";
  }

  if (dropMessageFromInput(input)) {
    return dropMessageFromInput(input)!;
  }

  if (message && !isTechnicalFunnelMessage(message)) {
    return message;
  }

  return "Пока недостаточно данных для анализа воронки.";
}

function dropMessageFromInput(input: {
  primaryDropStage?: string;
  primaryDropMessage?: string;
}): string | null {
  const dropStage = cleanText(input.primaryDropStage);
  const dropMessage = cleanText(input.primaryDropMessage);
  if (!dropMessage) return null;
  if (dropStage) {
    const stageLabel = normalizeFunnelStageLabel("", dropStage) || dropStage;
    return `По текущим данным сильнее всего просел этап: ${stageLabel}. ${dropMessage}`;
  }
  return dropMessage;
}

function isTechnicalFunnelMessage(message: string): boolean {
  const lower = message.toLowerCase();
  return (
    lower.includes("запустите генерацию") ||
    lower.includes("не сформирован") ||
    lower.includes("ai-сводк")
  );
}

export function buildFunnelStageChainSubtitle(stages: FunnelStage[]): string {
  return buildFunnelStageCompactChainSubtitle(stages);
}
