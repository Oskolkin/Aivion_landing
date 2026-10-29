import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import { formatFunnelNumber, formatFunnelPercent, getStageDisplayLabel } from "@/lib/sales-funnel-display";

export type ConversionMetricItem = {
  key: string;
  label: string;
  delta?: string;
};

export type FunnelTransitionId =
  | "impressions_to_views"
  | "product_views_to_add_to_cart"
  | "add_to_cart_to_orders"
  | "orders_to_delivered"
  | "unknown";

const TRANSITION_SHORT_LABEL: Record<FunnelTransitionId, string> = {
  impressions_to_views: "Показы → карточка",
  product_views_to_add_to_cart: "Карточка → корзина",
  add_to_cart_to_orders: "Корзина → заказанные единицы",
  orders_to_delivered: "Заказанные единицы → доставленные единицы",
  unknown: "",
};

const TRANSITION_SUBLINE: Record<FunnelTransitionId, string> = {
  impressions_to_views: "Увидели товар → открыли карточку",
  product_views_to_add_to_cart: "Открыли карточку → добавили товар в корзину",
  add_to_cart_to_orders: "Добавили в корзину → заказали товар",
  orders_to_delivered: "Заказали товар → получили доставку",
  unknown: "",
};

/**
 * Universal interpretation for each weak transition. Explains what the drop
 * means for the seller and which groups of causes to check — never repeats
 * the deltas shown above in chips.
 */
const TRANSITION_INTERPRETATION: Record<FunnelTransitionId, string> = {
  impressions_to_views:
    "Покупатели реже открывают карточки после показа товара. Значит, проблема может быть не в наличии спроса, а в привлекательности товара в выдаче: обложке, цене, названии, рейтинге или качестве трафика.",
  product_views_to_add_to_cart:
    "Покупатели доходят до карточек, но хуже переходят к выбору товара. Значит, проблема, скорее всего, не только в охвате, а в привлекательности карточек, цене, наличии товара или качестве трафика.",
  add_to_cart_to_orders:
    "Покупатели добавляют товары в корзину, но реже доходят до заказа. Это может указывать на барьеры на финальном шаге: цена после сравнения, доставка, условия покупки, промо, конкуренты или сомнения перед оформлением.",
  orders_to_delivered:
    "Заказы оформляются, но хуже доходят до доставки. Это уже не проблема интереса к товару, а риск исполнения заказа: отмены, логистика, наличие, обработка или качество выполнения.",
  unknown:
    "В данных видно ухудшение одного из переходов воронки. Проверьте, на каком шаге пользователи теряются сильнее всего, и сопоставьте это с карточками, ценой, остатками, рекламой и условиями покупки.",
};

/** Stage key → transition that ends at that stage (used for volume-based fallback). */
const STAGE_KEY_TO_INCOMING_TRANSITION: Record<string, FunnelTransitionId> = {
  product_views: "impressions_to_views",
  add_to_cart: "product_views_to_add_to_cart",
  orders: "add_to_cart_to_orders",
  delivered_units: "orders_to_delivered",
};

/**
 * Interpretation text for the weak transition. Universal across all transitions
 * and independent of the demo `Карточка → корзина` case.
 */
export function buildWeakSpotInterpretation(transitionId: FunnelTransitionId): string {
  return TRANSITION_INTERPRETATION[transitionId] ?? TRANSITION_INTERPRETATION.unknown;
}

/** Stage keys for each funnel transition (from → to). */
const TRANSITION_STAGES: Record<
  Exclude<FunnelTransitionId, "unknown">,
  { fromKey: string; toKey: string; conversionCaption: string }
> = {
  impressions_to_views: {
    fromKey: "impressions",
    toKey: "product_views",
    conversionCaption: "из показов в карточку",
  },
  product_views_to_add_to_cart: {
    fromKey: "product_views",
    toKey: "add_to_cart",
    conversionCaption: "из просмотров карточки в корзину",
  },
  add_to_cart_to_orders: {
    fromKey: "add_to_cart",
    toKey: "orders",
    conversionCaption: "из корзины в заказанные единицы",
  },
  orders_to_delivered: {
    fromKey: "orders",
    toKey: "delivered_units",
    conversionCaption: "из заказанных в доставленные единицы",
  },
};

const KEY_TO_TRANSITION: Record<string, FunnelTransitionId> = {
  impressions_to_views: "impressions_to_views",
  impressions_to_product_views: "impressions_to_views",
  views_to_cart: "product_views_to_add_to_cart",
  product_views_to_add_to_cart: "product_views_to_add_to_cart",
  add_to_cart_to_orders: "add_to_cart_to_orders",
  cart_to_order: "add_to_cart_to_orders",
  orders_to_delivered: "orders_to_delivered",
  orders_to_delivered_units: "orders_to_delivered",
  order_to_delivered: "orders_to_delivered",
  // Stage key `add_to_cart` = weakness at transition INTO add_to_cart (product_views → add_to_cart).
  add_to_cart: "product_views_to_add_to_cart",
  // Stage key `product_views` = weakness at transition INTO product_views (impressions → product_views).
  product_views: "impressions_to_views",
  impressions: "impressions_to_views",
  orders: "add_to_cart_to_orders",
  delivered_units: "orders_to_delivered",
};

function cleanText(value?: string | null): string {
  return (value ?? "").trim();
}

function normalizeForMatch(value: string): string {
  return value
    .toLowerCase()
    .replace(/[.,!?;:()[\]{}"'`«»\-—–/\\]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export function inferTransitionId(key?: string, label?: string): FunnelTransitionId {
  const k = cleanText(key).toLowerCase();
  if (k && KEY_TO_TRANSITION[k]) return KEY_TO_TRANSITION[k];
  const text = normalizeForMatch(`${key ?? ""} ${label ?? ""}`);
  if (/показ|увидел/.test(text) && /просмотр|карточк|открыл/.test(text) && !/корзин/.test(text)) {
    return "impressions_to_views";
  }
  if (/карточк|просмотр/.test(text) && /корзин|добавлен/.test(text)) {
    return "product_views_to_add_to_cart";
  }
  if (/корзин/.test(text) && /заказ/.test(text) && !/достав/.test(text)) {
    return "add_to_cart_to_orders";
  }
  if (/заказ/.test(text) && /достав/.test(text)) return "orders_to_delivered";
  return "unknown";
}

export function messageMatchesTransition(message: string, transitionId: FunnelTransitionId): boolean {
  const m = normalizeForMatch(message);
  if (!m) return false;
  if (transitionId === "unknown") return true;

  const mentionsImpressionsToViews =
    (/показ|увидел/.test(m) && /просмотр|карточк|открыл/.test(m) && !/корзин|добавлен/.test(m)) ||
    /основн.*потер.*показ|потер.*показ.*просмотр|показ.*просмотр/.test(m);
  const mentionsCardToCart =
    (/карточк|просмотр/.test(m) && /корзин|добавлен/.test(m)) ||
    /карточка.*корзин|из\s+просмотр.*корзин|конверси.*корзин/.test(m);
  const mentionsCartToOrders =
    (/корзин/.test(m) && /заказ/.test(m) && !/достав/.test(m)) ||
    /корзин.*заказанн|из\s+корзин.*заказ/.test(m);
  const mentionsOrdersToDelivered = /заказ/.test(m) && /достав/.test(m);

  switch (transitionId) {
    case "impressions_to_views":
      return mentionsImpressionsToViews && !mentionsCardToCart && !mentionsCartToOrders;
    case "product_views_to_add_to_cart":
      if (mentionsImpressionsToViews && !mentionsCardToCart) return false;
      if (mentionsCartToOrders) return false;
      return mentionsCardToCart || (/п\.п\./.test(m) && !/показ|увидел/.test(m) && !/корзин.*заказ/.test(m));
    case "add_to_cart_to_orders":
      if (mentionsCardToCart && !mentionsCartToOrders) return false;
      return mentionsCartToOrders;
    case "orders_to_delivered":
      return mentionsOrdersToDelivered;
    default:
      return true;
  }
}

function parseSignedNumber(raw?: string): number | null {
  const src = cleanText(raw).replace(",", ".");
  const m = src.match(/([+-−]?\d+(?:\.\d+)?)/);
  if (!m) return null;
  const n = parseFloat(m[1].replace("−", "-"));
  return Number.isFinite(n) ? n : null;
}

function formatPpDelta(value: number): string {
  const sign = value > 0 ? "+" : value < 0 ? "−" : "";
  const abs = Math.abs(value);
  const formatted = new Intl.NumberFormat("ru-RU", { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(abs);
  return `${sign}${formatted} п.п.`;
}

export function formatPctDelta(value: number): string {
  const sign = value > 0 ? "+" : value < 0 ? "−" : "";
  const abs = Math.abs(value);
  const formatted = new Intl.NumberFormat("ru-RU", { minimumFractionDigits: 0, maximumFractionDigits: 1 }).format(abs);
  return `${sign}${formatted}%`;
}

function formatDeltaRu(input: string): string {
  const src = cleanText(input);
  if (!src) return "";
  return src.replace(/([+-−]?\d+)\.(\d+)/g, "$1,$2").replace(/\s+/g, " ").trim();
}

function cleanComparisonDelta(value: string): string {
  return value
    .replace(/\s+к\s+\d{1,2}\.\d{1,2}\.\d{4}/gi, "")
    .replace(/\s+к\s+прошлой\s+сводке/gi, "")
    .replace(/\s+к\s+прошлому\s+дню/gi, "")
    .replace(/\s+к\s+среднему\s+за\s+7\s+дней/gi, "")
    .replace(/\s+к\s+вчера/gi, "")
    .replace(/\s+/g, " ")
    .trim();
}

function normalizePpDelta(raw: string): string {
  const cleaned = cleanComparisonDelta(raw);
  if (!cleaned) return "";
  if (/п\.п\./i.test(cleaned)) return formatDeltaRu(cleaned);
  const n = parseSignedNumber(cleaned);
  if (n == null) return formatDeltaRu(cleaned);
  return formatDeltaRu(formatPpDelta(n));
}

export type ConversionDeltaPair = { previous: string; sevenDay: string };

/** Picks conversion deltas (п.п.) for the selected transition only — never another transition. */
export function pickConversionDeltaForTransition(
  transitionId: FunnelTransitionId,
  conversions: ConversionMetricItem[],
): ConversionDeltaPair {
  if (transitionId === "unknown") return { previous: "", sevenDay: "" };

  const matches = conversions.filter((item) => inferTransitionId(item.key, item.label) === transitionId);
  let previous = "";
  let sevenDay = "";

  for (const item of matches) {
    const delta = cleanText(item.delta);
    if (!delta) continue;
    if (/среднему\s*за\s*7\s*дней/i.test(delta)) {
      sevenDay = normalizePpDelta(delta);
    } else if (/п\.п\./i.test(delta) || parseSignedNumber(delta) != null) {
      previous = normalizePpDelta(delta);
    }
  }

  return { previous, sevenDay };
}

export function pickStrongestNegativeConversion(
  conversions: ConversionMetricItem[],
): { item: ConversionMetricItem; transitionId: FunnelTransitionId } | null {
  let best: { item: ConversionMetricItem; transitionId: FunnelTransitionId; score: number } | null = null;
  for (const item of conversions) {
    const delta = cleanText(item.delta);
    if (!/п\.п\./i.test(delta)) continue;
    const n = parseSignedNumber(delta);
    if (n == null || n >= 0) continue;
    const transitionId = inferTransitionId(item.key, item.label);
    if (!best || n < best.score) {
      best = { item, transitionId, score: n };
    }
  }
  return best ? { item: best.item, transitionId: best.transitionId } : null;
}

function stageByKey(stages: FunnelStage[] | undefined, key: string): FunnelStage | undefined {
  return stages?.find((s) => s.key === key);
}

function parseRateFromValue(value?: string): number | null {
  const m = cleanText(value).replace(",", ".").match(/(\d+(?:\.\d+)?)\s*%/);
  if (!m) return null;
  const n = parseFloat(m[1]);
  return Number.isFinite(n) ? n : null;
}

function resolveTransitionStages(
  transitionId: FunnelTransitionId,
  stages: FunnelStage[] | undefined,
): { from: FunnelStage; to: FunnelStage } | null {
  if (transitionId === "unknown") return null;
  const cfg = TRANSITION_STAGES[transitionId];
  const from = stageByKey(stages, cfg.fromKey);
  const to = stageByKey(stages, cfg.toKey);
  if (!from || !to) return null;
  return { from, to };
}

function conversionRateForTransition(
  from: FunnelStage,
  to: FunnelStage,
  weakestValue?: string,
): string {
  if (typeof to.conversion_from_previous === "number") {
    return formatFunnelPercent(to.conversion_from_previous);
  }
  if (typeof from.value === "number" && typeof to.value === "number" && from.value > 0) {
    return formatFunnelPercent((to.value / from.value) * 100);
  }
  const parsed = parseRateFromValue(weakestValue);
  return parsed != null ? formatFunnelPercent(parsed) : "";
}

export type ResolvedWeakestTransition = {
  transitionId: FunnelTransitionId;
  key: string;
  shortLabel: string;
  subline: string;
  metricKind: "conversion" | "stage_volume" | "unknown";
};

export type StageDeltaItem = { key: string; delta?: string };

function negativePreviousStageDelta(stageKey: string, stageDeltas?: StageDeltaItem[]): number | null {
  const item = stageDeltas?.find((d) => cleanText(d.key) === stageKey);
  if (!item) return null;
  const n = parseSignedNumber(item.delta);
  return n != null && n < 0 ? n : null;
}

/**
 * True if a transition shows a negative previous-summary signal — a negative
 * conversion delta (п.п.), a negative previous stage delta at its end stage, or
 * a negative volume change at its end stage. Prevents selecting a transition
 * whose previous delta is actually ≥ 0.
 */
export function transitionHasNegativePreviousSignal(
  transitionId: FunnelTransitionId,
  conversions: ConversionMetricItem[],
  stages: FunnelStage[] | undefined,
  stageDeltas?: StageDeltaItem[],
): boolean {
  if (transitionId === "unknown") return false;
  const { previous } = pickConversionDeltaForTransition(transitionId, conversions);
  const convN = parseSignedNumber(previous);
  if (convN != null && convN < 0) return true;
  const toKey = TRANSITION_STAGES[transitionId].toKey;
  if (negativePreviousStageDelta(toKey, stageDeltas) != null) return true;
  const pair = resolveTransitionStages(transitionId, stages);
  const toPct = pair?.to?.change_vs_previous_day_pct;
  return typeof toPct === "number" && toPct < 0;
}

/**
 * Strongest negative stage change mapped to the transition feeding that stage.
 * Considers both previous stage deltas and sales-stage volume changes.
 */
export function pickStrongestNegativeStageVolume(
  stages: FunnelStage[] | undefined,
  stageDeltas?: StageDeltaItem[],
): { transitionId: FunnelTransitionId; stageKey: string } | null {
  type Candidate = { transitionId: FunnelTransitionId; stageKey: string; pct: number };
  const candidates: Candidate[] = [];
  const addCandidate = (stageKey: string, pct: number | null | undefined) => {
    const transitionId = STAGE_KEY_TO_INCOMING_TRANSITION[stageKey];
    if (!transitionId) return;
    if (typeof pct !== "number" || pct >= 0) return;
    candidates.push({ transitionId, stageKey, pct });
  };
  for (const stage of stages ?? []) addCandidate(stage.key, stage.change_vs_previous_day_pct);
  for (const item of stageDeltas ?? []) addCandidate(cleanText(item.key), parseSignedNumber(item.delta));
  if (candidates.length === 0) return null;
  const best = candidates.reduce((a, b) => (b.pct < a.pct ? b : a));
  return { transitionId: best.transitionId, stageKey: best.stageKey };
}

function resolvedFor(
  transitionId: FunnelTransitionId,
  key: string,
  metricKind: ResolvedWeakestTransition["metricKind"],
  fallbackLabel = "",
): ResolvedWeakestTransition {
  return {
    transitionId,
    key: key || transitionId,
    shortLabel: TRANSITION_SHORT_LABEL[transitionId] || fallbackLabel,
    subline: TRANSITION_SUBLINE[transitionId] || (transitionId === "unknown" ? "" : fallbackLabel),
    metricKind,
  };
}

export function resolveWeakestTransition(
  analysis: FunnelAIAnalysis,
  meta?: { primaryDropStage?: string; stages?: FunnelStage[]; stageDeltas?: StageDeltaItem[] },
  conversions: ConversionMetricItem[] = [],
): ResolvedWeakestTransition {
  const stages = meta?.stages;
  const stageDeltas = meta?.stageDeltas;
  const ws = analysis.weakest_stage;
  const wsStatus = cleanText(ws?.status);
  const wsKey = cleanText(ws?.key);
  const wsLabel = cleanText(ws?.label);

  // 1. Valid weakest_stage — only if it maps to a transition with a negative previous signal.
  if (wsStatus === "available" && (wsKey || wsLabel)) {
    const transitionId = inferTransitionId(wsKey, wsLabel);
    if (transitionId !== "unknown" && transitionHasNegativePreviousSignal(transitionId, conversions, stages, stageDeltas)) {
      return resolvedFor(transitionId, wsKey, "conversion", wsLabel);
    }
  }

  // 2. Strongest negative conversion metric to the previous summary.
  const fromConversion = pickStrongestNegativeConversion(conversions);
  if (fromConversion) {
    return resolvedFor(fromConversion.transitionId, fromConversion.item.key, "conversion", fromConversion.item.label);
  }

  // 3. primary_drop_stage — only if it maps to a transition with a negative previous signal.
  const primaryStage = cleanText(meta?.primaryDropStage);
  if (primaryStage) {
    const transitionId = inferTransitionId("", primaryStage);
    if (transitionId !== "unknown" && transitionHasNegativePreviousSignal(transitionId, conversions, stages, stageDeltas)) {
      return resolvedFor(transitionId, transitionId, "conversion");
    }
  }

  // 4. Fallback by strongest negative stage change (previous deltas or volume).
  const stageFallback = pickStrongestNegativeStageVolume(stages, stageDeltas);
  if (stageFallback) {
    return resolvedFor(stageFallback.transitionId, stageFallback.stageKey, "stage_volume");
  }

  return {
    transitionId: "unknown",
    key: "",
    shortLabel: "",
    subline: "",
    metricKind: "unknown",
  };
}

export function weakestStageDeltaMatchesTransition(
  wsKey: string | undefined,
  wsLabel: string | undefined,
  wsDelta: string | undefined,
  transitionId: FunnelTransitionId,
): boolean {
  if (!wsDelta?.trim()) return false;
  const wsTransition = inferTransitionId(wsKey, wsLabel);
  if (wsTransition !== transitionId) return false;
  // Conversion weak spot uses п.п.; stage-volume fallback may be %.
  if (/%/.test(wsDelta) && !/п\.п\./i.test(wsDelta)) return false;
  return true;
}

export type WeakSpotMetricDisplay = {
  conversionRate: string;
  conversionSubline: string;
  volumeCountLabel: string;
  volumeSourceLabel: string;
};

export function buildWeakSpotMetricDisplay(
  transitionId: FunnelTransitionId,
  stages: FunnelStage[] | undefined,
  weakestValue?: string,
): WeakSpotMetricDisplay | null {
  if (transitionId === "unknown") return null;
  const pair = resolveTransitionStages(transitionId, stages);
  if (!pair) return null;

  const rate = conversionRateForTransition(pair.from, pair.to, weakestValue);
  if (!rate) return null;

  const cfg = TRANSITION_STAGES[transitionId];
  return {
    conversionRate: rate,
    conversionSubline: cfg.conversionCaption,
    volumeCountLabel: typeof pair.to.value === "number" ? formatFunnelNumber(pair.to.value) : "",
    volumeSourceLabel: typeof pair.from.value === "number" ? formatFunnelNumber(pair.from.value) : "",
  };
}

/**
 * True if the explanation just restates the deltas/values already shown above
 * (chips, conversion rate) instead of interpreting what the drop means.
 * Simple guardrails, no NLP.
 */
export function isMetricRepeatExplanation(text: string): boolean {
  const t = cleanText(text);
  if (!t) return true;
  const lower = t.toLowerCase();
  const hasDeltaToken = /п\.п\.|%/.test(lower);
  const hasComparisonRef = /к\s+прошл|к\s+среднему|к\s+вчера|к\s+прошлой\s+сводке/.test(lower);
  if (hasDeltaToken && hasComparisonRef) return true;
  // Strip numbers + the most common "metric narration" words; if almost nothing
  // meaningful is left, the text is just a delta/value retelling.
  const meaningful = lower
    .replace(/[+-−]?\d+(?:[.,]\d+)?\s*(?:п\.п\.|%|₽|шт)?/g, " ")
    .replace(
      /\b(конверси\w*|снизил\w*|упал\w*|вырос\w*|изменил\w*|перехода?|переход\w*|сводк\w*|среднему|прошл\w*|вчера|дн\w*|составля\w*|сейчас|это|на|до|за|и|к)\b/g,
      " ",
    )
    .replace(/[^а-яё ]/gi, " ")
    .replace(/\s+/g, " ")
    .trim();
  return meaningful.length < 12;
}

function normalizeForCompare(value: string): string {
  return value
    .toLowerCase()
    .replace(/[.,!?;:()[\]{}"'`«»\-—–/\\]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function essentiallySame(a: string, b: string): boolean {
  const na = normalizeForCompare(a);
  const nb = normalizeForCompare(b);
  if (!na || !nb) return false;
  return na === nb || na.includes(nb) || nb.includes(na);
}

/**
 * Picks the "Что это значит" interpretation. Prefers the derived universal
 * interpretation and only keeps the AI explanation when it genuinely
 * interprets the selected transition (matches it, isn't a number retelling,
 * and doesn't conflict with the primary-drop message).
 */
export function pickWeakestExplanation(
  transitionId: FunnelTransitionId,
  aiExplanation: string,
  derivedInterpretation: string,
  primaryDropMessage = "",
): string {
  const ai = cleanText(aiExplanation);
  if (
    ai &&
    messageMatchesTransition(ai, transitionId) &&
    !isMetricRepeatExplanation(ai) &&
    !essentiallySame(ai, primaryDropMessage)
  ) {
    return ai;
  }
  return derivedInterpretation;
}

/**
 * Short, universal note clarifying that the funnel stage delta is a volume
 * change while the card delta is a conversion change. Only shown when both a
 * stage delta and a conversion delta are available — never hardcoded labels.
 */
export function buildVolumeVsConversionNote(
  transitionId: FunnelTransitionId,
  stages: FunnelStage[] | undefined,
  deltaPreviousPp: string,
): string | null {
  if (transitionId === "unknown") return null;

  const pair = resolveTransitionStages(transitionId, stages);
  const toStage = pair?.to ?? stageByKey(stages, TRANSITION_STAGES[transitionId].toKey);
  const volumePct = toStage?.change_vs_previous_day_pct;

  const conversionDelta =
    deltaPreviousPp && /п\.п\./i.test(deltaPreviousPp)
      ? formatDeltaRu(deltaPreviousPp)
      : deltaPreviousPp
        ? `${formatDeltaRu(deltaPreviousPp)} п.п.`
        : "";

  if (typeof volumePct !== "number" || !conversionDelta) return null;

  const stageDelta = formatPctDelta(volumePct);
  const toStageLabel = toStage ? getStageDisplayLabel(toStage.key, toStage.label) : "";
  const transitionLabel = TRANSITION_SHORT_LABEL[transitionId];
  if (!toStageLabel || !transitionLabel) return null;

  return `${stageDelta} в воронке — это изменение количества на этапе «${toStageLabel}». ${conversionDelta} здесь — изменение конверсии перехода «${transitionLabel}».`;
}

export function getStageVolumeDeltaLabel(stage: FunnelStage | undefined): string | null {
  if (!stage || typeof stage.change_vs_previous_day_pct !== "number") return null;
  return formatPctDelta(stage.change_vs_previous_day_pct);
}
