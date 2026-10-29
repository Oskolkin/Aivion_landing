import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import {
  buildVolumeVsConversionNote,
  buildWeakSpotInterpretation,
  buildWeakSpotMetricDisplay,
  inferTransitionId,
  messageMatchesTransition,
  pickConversionDeltaForTransition,
  pickWeakestExplanation,
  resolveWeakestTransition,
  weakestStageDeltaMatchesTransition,
} from "@/lib/funnel-weak-spot-display";

export type FunnelTone = "negative" | "warning" | "neutral" | "positive";

export type FunnelDisplayItem = {
  key: string;
  label: string;
  /** Full label for title/tooltip when `label` is compact. */
  titleLabel?: string;
  value?: string;
  delta?: string;
  tone: FunnelTone;
};

export type FunnelComparisonGroupsDisplay = {
  stages: FunnelDisplayItem[];
  conversions: FunnelDisplayItem[];
  business: FunnelDisplayItem[];
};

export type FunnelDropFactorGroup =
  | "funnel_signal"
  | "business_signal"
  | "possible_amplifier"
  | "context_hypothesis";

export type FunnelDropFactorSectionKey = "drop_signals" | "possible_amplifiers";

export type FunnelDropFactorDisplay = {
  key: string;
  label: string;
  metric: string;
  delta: string;
  limitation: string;
  explanation: string;
  evidenceLevel: "confirmed" | "likely" | "hypothesis" | "limited" | "unknown";
  group: FunnelDropFactorGroup;
  tone: FunnelTone;
};

export type FunnelDropFactorSectionDisplay = {
  key: FunnelDropFactorSectionKey;
  title: string;
  factors: FunnelDropFactorDisplay[];
};

const DROP_FACTOR_GROUP_RANK: Record<FunnelDropFactorGroup, number> = {
  funnel_signal: 0,
  business_signal: 1,
  possible_amplifier: 2,
  context_hypothesis: 3,
};

const DROP_FACTOR_CANONICAL_EXPLANATION: Partial<Record<string, string>> = {
  cart_add_decline:
    "Добавления в корзину снизились сильнее других этапов. Это прямой подтверждённый сигнал ослабления перехода от интереса к выбору товара.",
  views_decline:
    "Просмотры карточки снизились сильнее, чем показы. Это уменьшает базу для добавлений в корзину и может усиливать просадку среднего этапа.",
  aov_drop:
    "Средний чек снизился. Это не объясняет падение переходов воронки, но усиливает финансовый эффект дня: даже при близком количестве заказов выручка проседает сильнее.",
  revenue_drop:
    "Выручка снизилась. Это не объясняет падение переходов воронки, но усиливает финансовый эффект дня.",
  stock_pressure:
    "Дефицит по значимым товарам может ограничивать продажи и ухудшать переходы, но текущий срез остатков не доказывает причинность по этапам воронки.",
  inefficient_ads:
    "Часть рекламного трафика расходует бюджет при слабом результате. Это может указывать на менее эффективное привлечение, но без прямой привязки кампаний к этапам воронки причинность не доказана.",
};

const DROP_FACTOR_CANONICAL_LIMITATION: Partial<Record<string, string>> = {
  views_decline: "Факт снижения подтверждён, связь с падением следующего этапа вероятна.",
  aov_drop: "Это подтверждённый финансовый сигнал, а не доказанная причина просадки переходов.",
  revenue_drop: "Это подтверждённый финансовый сигнал, а не доказанная причина просадки переходов.",
  stock_pressure:
    "Остатки доступны текущим срезом, поэтому влияние дефицита на этапы воронки нельзя доказать строго.",
  inefficient_ads:
    "Рекламные данные доступны на уровне кампаний; причинность с просадкой этапов воронки не доказана.",
};

/** Deterministic semantic group by factor key (ignores AI group). */
export function classifyDropFactorGroup(key: string): FunnelDropFactorGroup {
  const k = key.trim().toLowerCase();
  switch (k) {
    case "views_decline":
    case "cart_add_decline":
      return "funnel_signal";
    case "aov_drop":
    case "revenue_drop":
    case "average_order_value":
      return "business_signal";
    case "stock_pressure":
    case "stock_risks":
    case "inventory_pressure":
      return "possible_amplifier";
    case "inefficient_ads":
    case "ad":
    case "ads":
    case "advertising":
      return "context_hypothesis";
    default:
      return "context_hypothesis";
  }
}

function normalizeDropFactorGroup(raw: string | undefined, key: string): FunnelDropFactorGroup {
  void raw;
  return classifyDropFactorGroup(key);
}

function defaultEvidenceLevelForKey(key: string): FunnelDropFactorDisplay["evidenceLevel"] | null {
  switch (key.trim().toLowerCase()) {
    case "cart_add_decline":
      return "confirmed";
    case "views_decline":
      return "likely";
    case "aov_drop":
    case "revenue_drop":
    case "average_order_value":
      return "confirmed";
    case "stock_pressure":
    case "stock_risks":
    case "inventory_pressure":
      return "limited";
    case "inefficient_ads":
    case "ad":
    case "ads":
    case "advertising":
      return "hypothesis";
    default:
      return null;
  }
}

function enrichDropFactorDisplay(factor: FunnelDropFactorDisplay): FunnelDropFactorDisplay {
  const key = factor.key.trim().toLowerCase();
  const defaultEvidence = defaultEvidenceLevelForKey(key);
  let evidenceLevel = factor.evidenceLevel;
  if (defaultEvidence) {
    switch (key) {
      case "cart_add_decline":
      case "aov_drop":
      case "revenue_drop":
      case "average_order_value":
      case "stock_pressure":
      case "stock_risks":
      case "inventory_pressure":
        evidenceLevel = defaultEvidence;
        break;
      case "views_decline":
        if (evidenceLevel === "unknown" || evidenceLevel === "confirmed") {
          evidenceLevel = defaultEvidence;
        }
        break;
      case "inefficient_ads":
      case "ad":
      case "ads":
      case "advertising":
        if (evidenceLevel === "confirmed" || evidenceLevel === "likely" || evidenceLevel === "unknown") {
          evidenceLevel = defaultEvidence;
        }
        break;
      default:
        if (evidenceLevel === "unknown") {
          evidenceLevel = defaultEvidence;
        }
    }
  }
  const explanation = factor.explanation || DROP_FACTOR_CANONICAL_EXPLANATION[key] || "";
  const limitation = factor.limitation || DROP_FACTOR_CANONICAL_LIMITATION[key] || "";
  return {
    ...factor,
    group: classifyDropFactorGroup(key),
    evidenceLevel,
    explanation,
    limitation,
  };
}

export function buildDropFactorSections(factors: FunnelDropFactorDisplay[]): FunnelDropFactorSectionDisplay[] {
  const sorted = [...factors].sort(
    (a, b) => DROP_FACTOR_GROUP_RANK[a.group] - DROP_FACTOR_GROUP_RANK[b.group],
  );
  const signals = sorted.filter((f) => f.group === "funnel_signal");
  const amplifiers = sorted.filter((f) => f.group !== "funnel_signal");
  const sections: FunnelDropFactorSectionDisplay[] = [];
  if (signals.length > 0) {
    sections.push({ key: "drop_signals", title: "Сигналы просадки", factors: signals });
  }
  if (amplifiers.length > 0) {
    sections.push({
      key: "possible_amplifiers",
      title: "Факторы, которые могли усилить эффект",
      factors: amplifiers,
    });
  }
  return sections;
}

export type FunnelDirectionDisplay = {
  key: string;
  label: string;
  text: string;
  priority: number;
  priorityLabel: string;
  reason: string;
  dataNeeded: string;
  tone: FunnelTone;
};

export type FunnelDetailsSectionDisplay = {
  key: "supporting" | "mitigating" | "related" | "limitations" | "conclusion" | "other";
  title: string;
  items: Array<{ text: string; metrics: string[] }>;
};

const WEAKEST_SHORT_LABEL_BY_KEY: Record<string, string> = {
  product_views_to_add_to_cart: "Карточка → корзина",
  views_to_cart: "Карточка → корзина",
};

const STAGE_LABEL_FALLBACK: Record<string, string> = {
  impressions: "Показы",
  product_views: "Просмотры карточки",
  add_to_cart: "Добавления в корзину",
  orders: "Заказанные единицы",
  delivered_units: "Доставленные единицы",
};

/** Unified comparison skeleton — same keys/order for previous and 7-day cards. */
const STAGE_KEYS_ORDER = [
  "impressions",
  "product_views",
  "add_to_cart",
  "orders",
  "delivered_units",
] as const;

const CONVERSION_KEYS_ORDER = [
  "impressions_to_product_views",
  "product_views_to_add_to_cart",
  "add_to_cart_to_orders",
  "orders_to_delivered_units",
] as const;

const BUSINESS_KEYS_ORDER = ["revenue", "average_order_value"] as const;

const CONVERSION_KEY_ALIASES: Record<string, string> = {
  impressions_to_views: "impressions_to_product_views",
  views_to_cart: "product_views_to_add_to_cart",
  cart_to_order: "add_to_cart_to_orders",
  add_to_cart_to_order: "add_to_cart_to_orders",
};

const CONVERSION_LABELS: Record<string, { short: string; full: string }> = {
  impressions_to_product_views: { short: "Показы → Карточка", full: "Показы → Карточка" },
  product_views_to_add_to_cart: { short: "Карточка → Корзина", full: "Карточка → Корзина" },
  add_to_cart_to_orders: { short: "Корзина → Заказанные единицы", full: "Корзина → Заказанные единицы" },
  orders_to_delivered_units: {
    short: "Заказанные → Доставленные",
    full: "Заказанные единицы → Доставленные единицы",
  },
};

const BUSINESS_LABELS: Record<string, string> = {
  revenue: "Выручка",
  average_order_value: "Средний чек",
  gross_margin: "Валовая маржа",
  profit: "Прибыль",
  ad_spend: "Расход на рекламу",
  drr: "ДРР",
};

const BUSINESS_METRIC_KEYS = new Set(["average_order_value", "revenue", "gross_margin", "profit", "ad_spend", "drr"]);

type ComparisonBucket = "stage" | "conversion" | "business";

export function canonicalComparisonKey(key: string): string {
  const k = cleanText(key).toLowerCase();
  return CONVERSION_KEY_ALIASES[k] ?? k;
}

export function classifyComparisonBucket(key: string): ComparisonBucket | "unknown" {
  const k = canonicalComparisonKey(key);
  if ((STAGE_KEYS_ORDER as readonly string[]).includes(k)) return "stage";
  if ((CONVERSION_KEYS_ORDER as readonly string[]).includes(k)) return "conversion";
  if ((BUSINESS_KEYS_ORDER as readonly string[]).includes(k)) return "business";
  if (BUSINESS_METRIC_KEYS.has(k)) return "business";
  if (k.includes("_to_")) return "conversion";
  return "unknown";
}

function labelForComparisonKey(key: string, bucket: ComparisonBucket): { short: string; full: string } {
  const k = canonicalComparisonKey(key);
  if (bucket === "stage") {
    const short = STAGE_LABEL_FALLBACK[k] ?? k;
    return { short, full: short };
  }
  if (bucket === "conversion") {
    const mapped = CONVERSION_LABELS[k];
    if (mapped) return mapped;
    const normalized = normalizeSellerLabel("", k);
    return { short: normalized, full: normalized };
  }
  const short = BUSINESS_LABELS[k] ?? normalizeSellerLabel("", k);
  return { short, full: short };
}

const FORBIDDEN_LABEL_REPLACEMENTS: Array<{ rx: RegExp; value: string }> = [
  { rx: /оформленн[а-я\s]*заказ[а-я]*/gi, value: "Заказанные единицы" },
  { rx: /выкуп(?:ы|или|ленн[а-я]*)?/gi, value: "Доставленные единицы" },
];

const DROP_FACTOR_LABEL_POLISH: Array<{ match: RegExp; label: string }> = [
  { match: /расходы в слабых рекламных/i, label: "Реклама с низкой отдачей" },
  { match: /давление дефицита/i, label: "Дефицит значимых товаров" },
];

const IMPERATIVE_PREFIXES = [
  "срочно",
  "остановите",
  "остановить",
  "сократите",
  "сократить",
  "пополните",
  "пополнить",
  "уберите",
  "убрать",
  "проверьте",
  "проверить",
  "сверьте",
  "сверить",
  "разберите",
  "разобрать",
] as const;

function cleanText(value?: string | null): string {
  return (value ?? "").trim();
}

function normalizeSellerLabel(label?: string, key?: string): string {
  const mapped = STAGE_LABEL_FALLBACK[cleanText(key)];
  let out = cleanText(label) || mapped || cleanText(key);
  for (const repl of FORBIDDEN_LABEL_REPLACEMENTS) {
    out = out.replace(repl.rx, repl.value);
  }
  return out.trim();
}

function formatDeltaRu(input: string): string {
  const src = cleanText(input);
  if (!src) return "";
  return src.replace(/([+-−]?\d+)\.(\d+)/g, "$1,$2").replace(/\s+/g, " ").trim();
}

function normalizeForCompare(value: string): string {
  return value
    .toLowerCase()
    .replace(/[.,!?;:()[\]{}"'`«»\-—–/\\]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function isNearDuplicate(a: string, b: string): boolean {
  const na = normalizeForCompare(a);
  const nb = normalizeForCompare(b);
  if (!na || !nb) return false;
  if (na === nb) return true;
  return na.includes(nb) || nb.includes(na);
}

function tone(value?: string): FunnelTone {
  return value === "negative" || value === "warning" || value === "positive" ? value : "neutral";
}

/** Seller-facing short title for the weakest-stage card. */
export function shortenWeakestStageLabel(key: string, label: string): string {
  const k = key.trim();
  const l = label.trim();
  if (k && WEAKEST_SHORT_LABEL_BY_KEY[k]) {
    return WEAKEST_SHORT_LABEL_BY_KEY[k];
  }
  if (/из\s+просмотров\s+карточки\s+в\s+добавление\s+в\s+корзину/i.test(l)) {
    return "Карточка → корзина";
  }
  if (/из\s+просмотров.*корзин/i.test(l)) {
    return "Карточка → корзина";
  }
  return l;
}

function polishDropFactorLabel(label: string): string {
  const trimmed = label.trim();
  if (!trimmed) return trimmed;
  for (const rule of DROP_FACTOR_LABEL_POLISH) {
    if (rule.match.test(trimmed)) return rule.label;
  }
  return trimmed;
}

function stripImperativeOpenersTs(text: string): string {
  let s = text.trim();
  let changed = true;
  while (changed && s) {
    changed = false;
    const lower = s.toLowerCase();
    for (const p of IMPERATIVE_PREFIXES) {
      const hit = [" ", ",", ":"].some((sep) => lower.startsWith(p + sep));
      if (!hit) continue;
      const pr = [...p];
      const sr = [...s];
      let ok = true;
      for (let i = 0; i < pr.length; i++) {
        if (sr[i]?.toLowerCase() !== pr[i]) {
          ok = false;
          break;
        }
      }
      if (!ok) continue;
      s = sr
        .slice(pr.length)
        .join("")
        .replace(/^[\s,.:]+/, "")
        .trim();
      changed = true;
      break;
    }
  }
  return s.trim();
}

function hasStructuredComparisonMetrics(
  comparisons?: FunnelAIAnalysis["comparisons"],
): boolean {
  if (!comparisons) return false;
  return (
    (comparisons.stage_metrics?.length ?? 0) > 0 ||
    (comparisons.conversion_metrics?.length ?? 0) > 0 ||
    (comparisons.business_metrics?.length ?? 0) > 0
  );
}

type RawComparisonItem = {
  key?: string;
  label?: string;
  value?: string;
  delta?: string;
  tone?: string;
};

function cleanComparisonDelta(value: string): string {
  return value
    .replace(/\s+к\s+\d{1,2}\.\d{1,2}\.\d{4}/gi, "")
    .replace(/\s+к\s+20\.\d{1,2}\.\d{4}/gi, "")
    .replace(/\s+к\s+прошлой\s+сводке/gi, "")
    .replace(/\s+к\s+прошлому\s+дню/gi, "")
    .replace(/\s+к\s+вчера/gi, "")
    .replace(/\s+к\s+среднему\s+за\s+7\s+дней/gi, "")
    .replace(/\s+/g, " ")
    .trim();
}

function rawComparisonToDisplay(item: RawComparisonItem, fallbackKey: string): FunnelDisplayItem | null {
  const rawKey = cleanText(item.key) || fallbackKey;
  const bucket = classifyComparisonBucket(rawKey);
  if (bucket === "unknown") return null;

  const key = canonicalComparisonKey(rawKey);
  const labels = labelForComparisonKey(key, bucket);
  const delta = formatDeltaRu(cleanComparisonDelta(cleanText(item.delta)));
  const value = cleanText(item.value);
  if (!delta && !value) return null;

  return {
    key,
    label: labels.short,
    titleLabel: labels.full !== labels.short ? labels.full : undefined,
    value,
    delta,
    tone: tone(item.tone),
  };
}

function buildGroupsFromSkeleton(items: FunnelDisplayItem[]): FunnelComparisonGroupsDisplay {
  const bySlot = new Map<string, FunnelDisplayItem>();
  for (const item of items) {
    const bucket = classifyComparisonBucket(item.key);
    if (bucket === "unknown") continue;
    const canonical = canonicalComparisonKey(item.key);
    const slot = `${bucket}:${canonical}`;
    const labels = labelForComparisonKey(canonical, bucket);
    const normalized: FunnelDisplayItem = {
      ...item,
      key: canonical,
      label: labels.short,
      titleLabel: labels.full !== labels.short ? labels.full : item.titleLabel ?? undefined,
    };
    const existing = bySlot.get(slot);
    if (!existing || (normalized.delta && !existing.delta)) {
      bySlot.set(slot, normalized);
    }
  }

  const pick = (bucket: ComparisonBucket, keys: readonly string[]) =>
    keys.map((k) => bySlot.get(`${bucket}:${k}`)).filter((x): x is FunnelDisplayItem => Boolean(x));

  return {
    stages: pick("stage", STAGE_KEYS_ORDER),
    conversions: pick("conversion", CONVERSION_KEYS_ORDER),
    business: pick("business", BUSINESS_KEYS_ORDER),
  };
}

function collectPreviousRawItems(comparisons?: FunnelAIAnalysis["comparisons"]): RawComparisonItem[] {
  if (!comparisons) return [];
  if (hasStructuredComparisonMetrics(comparisons)) {
    return [
      ...(comparisons.stage_metrics ?? []),
      ...(comparisons.conversion_metrics ?? []),
      ...(comparisons.business_metrics ?? []),
    ];
  }
  return comparisons.previous_period ?? [];
}

function collectSevenDayRawItems(comparisons?: FunnelAIAnalysis["comparisons"]): RawComparisonItem[] {
  return comparisons?.seven_day_average ?? [];
}

function itemsFromRawList(rawList: RawComparisonItem[], keyPrefix: string): FunnelDisplayItem[] {
  return rawList
    .map((item, index) => rawComparisonToDisplay(item, `${keyPrefix}-${index}`))
    .filter((item): item is FunnelDisplayItem => item !== null);
}

function normalizeItems(
  raw: FunnelAIAnalysis["comparisons"] extends infer T ? T : never,
  key: "previous_period" | "seven_day_average",
): FunnelDisplayItem[] {
  const source = raw?.[key] ?? [];
  return itemsFromRawList(source, key);
}

// Context signals (e.g. history_status) are not funnel metrics, so they bypass the
// bucket classifier (which would drop them). The limited-history note lives here and
// is rendered as a separate InfoNote, never folded into the AI commentary text.
function normalizeContextSignals(items: NonNullable<FunnelAIAnalysis["comparisons"]>["context_signals"]): FunnelDisplayItem[] {
  return (items ?? [])
    .map((item, index): FunnelDisplayItem | null => {
      const key = cleanText(item?.key) || `ctx-${index}`;
      const value = replaceForbiddenLabels(cleanText(item?.value));
      const delta = formatDeltaRu(cleanComparisonDelta(cleanText(item?.delta)));
      if (!value && !delta) return null;
      return { key, label: cleanText(item?.label), value, delta, tone: tone(item?.tone) };
    })
    .filter((item): item is FunnelDisplayItem => item !== null);
}

function replaceForbiddenLabels(text: string): string {
  let t = text;
  for (const repl of FORBIDDEN_LABEL_REPLACEMENTS) {
    t = t.replace(repl.rx, repl.value);
  }
  return t;
}

function normalizeDropFactors(analysis: FunnelAIAnalysis): FunnelDropFactorDisplay[] {
  const primary: FunnelDropFactorDisplay[] = (analysis.drop_factors ?? []).map((item, index) => {
    const key = cleanText(item?.key) || `factor-${index}`;
    return enrichDropFactorDisplay({
      key,
      label: polishDropFactorLabel(normalizeSellerLabel(item?.label, item?.key)),
      metric: cleanText(item?.metric),
      delta: formatDeltaRu(cleanText(item?.delta)),
      limitation: cleanText(item?.limitation),
      explanation: cleanText(item?.description) || cleanText(item?.explanation) || cleanText(item?.text),
      evidenceLevel: normalizeEvidenceLevel(item?.evidence_level),
      group: normalizeDropFactorGroup(cleanText(item?.group), key),
      tone: tone(item?.tone),
    });
  });
  if (primary.length > 0) {
    return primary.filter((f) => f.label || f.explanation || f.metric || f.delta || f.limitation);
  }
  const legacy: FunnelDropFactorDisplay[] = (analysis.influence_factors ?? []).map((item, index) => {
    const key = cleanText(item?.key) || `legacy-${index}`;
    return enrichDropFactorDisplay({
      key,
      label: cleanText(item?.label),
      metric: "",
      delta: "",
      limitation: "",
      explanation: cleanText(item?.text) || cleanText(item?.label),
      evidenceLevel: "unknown",
      group: classifyDropFactorGroup(key),
      tone: tone(item?.tone),
    });
  });
  return legacy.filter((f) => f.label || f.explanation);
}

function normalizeEvidenceLevel(raw?: string): FunnelDropFactorDisplay["evidenceLevel"] {
  const v = cleanText(raw).toLowerCase();
  if (v === "confirmed" || v === "likely" || v === "hypothesis" || v === "limited") return v;
  return "unknown";
}

function normalizeDirections(analysis: FunnelAIAnalysis): FunnelDirectionDisplay[] {
  const out: FunnelDirectionDisplay[] = [];
  for (const [index, item] of (analysis.check_directions ?? []).entries()) {
    let label = stripImperativeOpenersTs(normalizeSellerLabel(item?.label, item?.key));
    const text = stripImperativeOpenersTs(cleanText(item?.text));
    const priority = Number(cleanText(item?.priority) || "999");
    if (!text) continue;
    if (!label) label = text.slice(0, 48);
    out.push({
      key: cleanText(item?.key) || `dir-${index}`,
      label,
      text,
      priority: Number.isFinite(priority) ? priority : 999,
      priorityLabel: Number.isFinite(priority) ? String(priority) : "—",
      reason: cleanText(item?.reason),
      dataNeeded: cleanText(item?.data_needed),
      tone: tone(item?.tone),
    });
  }
  return out.sort((a, b) => a.priority - b.priority);
}

function normalizeDetailsSections(analysis: FunnelAIAnalysis): FunnelDetailsSectionDisplay[] {
  const sections = analysis.details_sections ?? [];
  const orderedKeys: Array<FunnelDetailsSectionDisplay["key"]> = [
    "supporting",
    "mitigating",
    "related",
    "limitations",
    "conclusion",
  ];
  const titleByKey: Record<FunnelDetailsSectionDisplay["key"], string> = {
    supporting: "Что подтверждает вывод",
    mitigating: "Что смягчает вывод",
    related: "Связанные сигналы",
    limitations: "Ограничения анализа",
    conclusion: "Итог",
    other: "",
  };
  const keyByTitle = (title: string): FunnelDetailsSectionDisplay["key"] => {
    const t = title.toLowerCase().trim();
    if (!t) return "other";
    if (/где просадка|supporting_evidence|evidence|confirmation|where_drop|drop_evidence|подтвержда|support/.test(t))
      return "supporting";
    if (/что работает устойчиво|mitigating_evidence|positive_signals|stability|what_is_stable|смягча|mitigat/.test(t))
      return "mitigating";
    if (/что усиливает просадку|related_signals|influence_factors|signals|drop_factors|связанные сигналы|related/.test(t))
      return "related";
    if (/ограничени|limitations?|data_limits|analysis_limits/.test(t)) return "limitations";
    if (/итог|вывод|заключение|conclusion|summary|final_summary|result/.test(t)) return "conclusion";
    return "other";
  };

  const byKey = new Map<FunnelDetailsSectionDisplay["key"], Array<{ text: string; metrics: string[] }>>();
  for (const sec of sections) {
    const key = keyByTitle(cleanText(sec?.title));
    if (key === "other") continue;
    const target = byKey.get(key) ?? [];
    for (const item of sec?.items ?? []) {
      const text = cleanText(item?.text);
      const metrics = (item?.metrics ?? []).map((m) => cleanText(m)).filter(Boolean);
      if (!text && metrics.length === 0) continue;
      if (!target.some((existing) => isNearDuplicate(existing.text, text))) {
        const uniqueMetrics = metrics.filter((m) => !target.some((existing) => existing.metrics.some((em) => isNearDuplicate(em, m))));
        target.push({ text, metrics: uniqueMetrics });
      }
    }
    byKey.set(key, target);
  }

  const result: FunnelDetailsSectionDisplay[] = [];
  for (const key of orderedKeys) {
    const items = byKey.get(key) ?? [];
    if (items.length === 0) continue;
    if (key === "conclusion") {
      const longest = items
        .slice()
        .sort((a, b) => normalizeForCompare(b.text).length - normalizeForCompare(a.text).length)[0];
      if (longest?.text) {
        result.push({ key, title: titleByKey[key], items: [{ text: longest.text, metrics: [] }] });
      }
      continue;
    }
    result.push({ key, title: titleByKey[key], items });
  }
  return result;
}

export function hasFunnelAIAnalysisExpandedContent(analysis: FunnelAIAnalysis): boolean {
  const sections = normalizeDetailsSections(analysis);
  if (sections.length > 0) return true;
  const legacy = cleanText(analysis.details) || cleanText(analysis.full_analysis);
  return legacy.length >= 12;
}

function extractWeakSpotFromPrimaryMessage(primaryDropMessage: string): { previous: string; sevenDay: string } {
  const prev = primaryDropMessage.match(/([+-−]?\d+(?:[.,]\d+)?)\s*п\.п\.\s*к\s*(?:вчера|прошлой\s+сводке)/i)?.[1] ?? "";
  const seven = primaryDropMessage.match(/([+-−]?\d+(?:[.,]\d+)?)\s*п\.п\.\s*к\s*среднему\s*за\s*7\s*дней/i)?.[1] ?? "";
  return {
    previous: prev ? `${prev.replace(".", ",")} п.п.` : "",
    sevenDay: seven ? `${seven.replace(".", ",")} п.п.` : "",
  };
}

/** Metrics for “К прошлой сводке” — structured or legacy `previous_period`, unified skeleton. */
export function buildPreviousComparisonGroups(analysis: FunnelAIAnalysis): FunnelComparisonGroupsDisplay {
  return buildGroupsFromSkeleton(itemsFromRawList(collectPreviousRawItems(analysis.comparisons), "previous"));
}

/** Metrics for “К среднему за 7 дней” — only `seven_day_average`, same skeleton. */
export function buildSevenDayComparisonGroups(analysis: FunnelAIAnalysis): FunnelComparisonGroupsDisplay {
  return buildGroupsFromSkeleton(itemsFromRawList(collectSevenDayRawItems(analysis.comparisons), "seven_day"));
}

function deltaIsNegative(delta?: string): boolean {
  const src = cleanText(delta).replace(",", ".").replace(/−/g, "-");
  const m = src.match(/(-?\d+(?:\.\d+)?)/);
  if (!m) return false;
  const n = parseFloat(m[1]);
  return Number.isFinite(n) && n < 0;
}

/**
 * Deterioration in the FUNNEL itself, measured strictly against the PREVIOUS
 * summary (not the 7-day average): true when at least one funnel stage metric
 * (impressions, product_views, add_to_cart, orders, delivered_units) or funnel
 * conversion metric (the four step transitions) has a negative previous delta.
 *
 * Business metrics (`revenue`, `average_order_value`) are intentionally
 * excluded: a drop in revenue/AOV is a financial signal, not a weak spot of a
 * funnel transition, so it must not, on its own, surface «Слабое место воронки».
 * Those metrics still appear in the comparison cards and drop-factor sections.
 */
export function hasNegativePreviousFunnelDynamics(analysis: FunnelAIAnalysis): boolean {
  const groups = buildPreviousComparisonGroups(analysis);
  const funnelItems = [...groups.stages, ...groups.conversions];
  return funnelItems.some((item) => deltaIsNegative(item.delta));
}

export function hasPreviousComparisonSectionContent(analysis: FunnelAIAnalysis): boolean {
  const commentary = cleanText(
    analysis.comparisons_commentary?.previous_summary || analysis.comparisons_commentary?.previous_period,
  );
  if (commentary) return true;
  const groups = buildPreviousComparisonGroups(analysis);
  return groups.stages.length > 0 || groups.conversions.length > 0 || groups.business.length > 0;
}

export function hasSevenDayComparisonSectionContent(analysis: FunnelAIAnalysis): boolean {
  if (cleanText(analysis.comparisons_commentary?.seven_day_average)) return true;
  const historyNote = (analysis.comparisons?.context_signals ?? [])
    .map((s) => cleanText(s?.value))
    .find((v) => v.length > 0);
  if (historyNote) return true;
  const groups = buildSevenDayComparisonGroups(analysis);
  return groups.stages.length > 0 || groups.conversions.length > 0 || groups.business.length > 0;
}

const LIMITED_HISTORY_NOTE_RX = /Сравнение\s+ограничено:?\s*доступно\s+меньше\s*7\s*дней\s+истории\.?/gi;

function funnelHistoryIsLimited(analysis: FunnelAIAnalysis): boolean {
  return (analysis.comparisons?.context_signals ?? []).some((s) => {
    if (cleanText(s?.key) !== "history_status") return false;
    return /limited/i.test(cleanText(s?.delta)) || /меньше\s*7\s*дн/i.test(cleanText(s?.value));
  });
}

/**
 * Defensive frontend guardrails for the AI comparison commentary. AI text stays
 * the primary source; we only strip the limited-history note (shown separately
 * via InfoNote), replace legacy labels, and rephrase "за 7 дней" wording when
 * history is limited so the summary never claims a full 7-day baseline.
 */
function sanitizeComparisonCommentary(raw: string, limitedHistory: boolean): string {
  let t = cleanText(raw);
  if (!t) return "";
  t = t.replace(LIMITED_HISTORY_NOTE_RX, " ");
  for (const repl of FORBIDDEN_LABEL_REPLACEMENTS) {
    t = t.replace(repl.rx, repl.value);
  }
  if (limitedHistory) {
    t = t
      .replace(/средн(ему|его|ей)\s+за\s+7\s*дн(?:ей|я)?/gi, (_m, end: string) => {
        const map: Record<string, string> = {
          ему: "доступному среднему за период",
          его: "доступного среднего за период",
          ей: "доступной средней за период",
        };
        return map[end.toLowerCase()] ?? "доступному среднему за период";
      })
      .replace(/средн(яя|ий)\s+за\s+7\s*дн(?:ей|я)?/gi, (_m, end: string) =>
        end.toLowerCase() === "яя" ? "доступная средняя за период" : "доступный средний за период",
      )
      .replace(/за\s+последн(?:ие|их)\s+7\s*дн(?:ей|я)?/gi, "по доступной истории")
      .replace(/за\s+7\s*дн(?:ей|я)?/gi, "по доступной истории")
      .replace(/7[-\s]*дневн\w*\s+средн\w*/gi, "доступному среднему за период");
  }
  return t.replace(/\s+/g, " ").trim();
}

export function buildFunnelAnalysisDisplayModel(
  analysis: FunnelAIAnalysis,
  meta?: { primaryDropStage?: string; primaryDropMessage?: string; stages?: FunnelStage[] },
) {
  const limitedHistory = funnelHistoryIsLimited(analysis);
  const previousPeriod = normalizeItems(analysis.comparisons, "previous_period");
  const sevenDayAverage = normalizeItems(analysis.comparisons, "seven_day_average");
  const comparisonGroups = buildPreviousComparisonGroups(analysis);
  const sevenDayComparisonGroups = buildSevenDayComparisonGroups(analysis);
  const ws = analysis.weakest_stage;
  let status = cleanText(ws?.status);
  if (!status) {
    status = cleanText(ws?.label) || cleanText(ws?.metric) ? "available" : "unavailable";
  }
  const previousStageDeltas = comparisonGroups.stages.map((s) => ({ key: s.key, delta: s.delta }));
  const transition = resolveWeakestTransition(
    analysis,
    { ...meta, stageDeltas: previousStageDeltas },
    comparisonGroups.conversions,
  );
  const primaryDropStageId = inferTransitionId("", meta?.primaryDropStage);
  const usePrimaryDropStage =
    primaryDropStageId !== "unknown" &&
    (transition.transitionId === "unknown" || primaryDropStageId === transition.transitionId);

  const weakestKey = cleanText(ws?.key) || transition.key;
  const weakestLabelRaw = normalizeSellerLabel(
    usePrimaryDropStage ? meta?.primaryDropStage : ws?.label || meta?.primaryDropStage,
    ws?.key || transition.key,
  );
  const primaryDropMessage = cleanText(meta?.primaryDropMessage);
  const extracted = extractWeakSpotFromPrimaryMessage(primaryDropMessage);
  const useExtractedFromMessage =
    Boolean(primaryDropMessage) && messageMatchesTransition(primaryDropMessage, transition.transitionId);
  const conversionDeltas = pickConversionDeltaForTransition(transition.transitionId, comparisonGroups.conversions);
  const derivedPrevious = conversionDeltas.previous || (useExtractedFromMessage ? extracted.previous : "");
  const derivedSevenDay = conversionDeltas.sevenDay || (useExtractedFromMessage ? extracted.sevenDay : "");
  const wsDeltaPrevious = weakestStageDeltaMatchesTransition(
    ws?.key,
    ws?.label,
    cleanText(ws?.delta_previous),
    transition.transitionId,
  )
    ? cleanComparisonDelta(cleanText(ws?.delta_previous))
    : "";
  const wsDelta7d = weakestStageDeltaMatchesTransition(ws?.key, ws?.label, cleanText(ws?.delta_7d), transition.transitionId)
    ? cleanComparisonDelta(cleanText(ws?.delta_7d))
    : "";
  const deltaPrevious = formatDeltaRu(derivedPrevious || wsDeltaPrevious);
  const delta7d = formatDeltaRu(derivedSevenDay || wsDelta7d);

  const metricDisplay = buildWeakSpotMetricDisplay(transition.transitionId, meta?.stages, cleanText(ws?.value));
  const interpretation = buildWeakSpotInterpretation(transition.transitionId);
  const explanation = pickWeakestExplanation(
    transition.transitionId,
    cleanText(ws?.explanation) || cleanText(ws?.reason),
    interpretation,
    cleanText(meta?.primaryDropMessage),
  );
  const volumeVsConversionNote = buildVolumeVsConversionNote(transition.transitionId, meta?.stages, deltaPrevious);
  const dropFactors = normalizeDropFactors(analysis);

  // Show the weak-spot block only when the FUNNEL itself deteriorated vs the
  // previous summary (negative stage or conversion delta). A drop in
  // revenue/AOV alone is a financial signal, not a funnel weak spot, so it must
  // not surface this block. If every funnel previous delta is ≥ 0, weakSpot = null.
  const showWeakSpot = hasNegativePreviousFunnelDynamics(analysis);
  const weakestStage = showWeakSpot
    ? {
        status,
        key: weakestKey,
        transitionId: transition.transitionId,
        label: transition.shortLabel || shortenWeakestStageLabel(weakestKey, weakestLabelRaw),
        labelFull: transition.subline || weakestLabelRaw,
        labelSubline: transition.subline,
        metricKind: transition.metricKind,
        metric: cleanText(ws?.metric),
        value: cleanText(ws?.value),
        conversionRate: metricDisplay?.conversionRate ?? "",
        conversionSubline: metricDisplay?.conversionSubline ?? "",
        explanation,
        deltaPrevious,
        delta7d,
        volumeVsConversionNote: volumeVsConversionNote ?? "",
        tone: tone(ws?.tone),
      }
    : null;

  return {
    previousPeriod,
    sevenDayAverage,
    comparisonGroups,
    sevenDayComparisonGroups,
    comparisonsCommentary: {
      previousPeriod: sanitizeComparisonCommentary(
        analysis.comparisons_commentary?.previous_summary || analysis.comparisons_commentary?.previous_period || "",
        limitedHistory,
      ),
      sevenDayAverage: sanitizeComparisonCommentary(analysis.comparisons_commentary?.seven_day_average || "", limitedHistory),
    },
    contextSignals: normalizeContextSignals(analysis.comparisons?.context_signals),
    weakestStage,
    dropFactors,
    dropFactorSections: buildDropFactorSections(dropFactors),
    checkDirections: normalizeDirections(analysis),
    detailsSections: normalizeDetailsSections(analysis),
    legacyDetailsText:
      cleanText(analysis.details) || cleanText(analysis.full_analysis) || cleanText(analysis.trend_vs_7d),
  };
}
